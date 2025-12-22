// internal/services/story_service.go
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/storage"
	"github.com/Corphon/SceneIntruderMCP/internal/utils"
)

// 角色互动触发条件常量
const (
	TriggerTypeCharacterInteraction = "character_interaction"
	maxInitialSegmentNodes          = 12
	defaultSegmentRuneLimit         = 900
)

// StoryService 管理故事进展和剧情分支
type StoryService struct {
	SceneService     *SceneService
	ContextService   *ContextService
	LLMService       *LLMService
	FileStorage      *storage.FileStorage
	ItemService      *ItemService
	CharacterService *CharacterService
	BasePath         string
	lockManager      *LockManager // 使用统一的锁管理器

	// 缓存机制
	cacheMutex  sync.RWMutex
	storyCache  map[string]*CachedStoryData
	cacheExpiry time.Duration
}

// CachedStoryData 缓存的故事数据
type CachedStoryData struct {
	Data      *models.StoryData
	Timestamp time.Time
	Loading   *sync.Once // 用于确保只加载一次
}

// NewStoryService 创建故事服务
func NewStoryService(llmService *LLMService) *StoryService {
	// 创建基础路径
	basePath := "data/stories"
	if err := os.MkdirAll(basePath, 0755); err != nil {
		utils.GetLogger().Error("failed to create story data directory", map[string]interface{}{
			"base_path": basePath,
			"err":       err.Error(),
		})
	}

	// 创建文件存储
	fileStorage, err := storage.NewFileStorage(basePath)
	if err != nil {
		utils.GetLogger().Error("failed to create story file storage", map[string]interface{}{
			"base_path": basePath,
			"err":       err.Error(),
		})
		return nil
	}

	// 创建场景服务(如果需要)
	scenesPath := "data/scenes"
	sceneService := NewSceneService(scenesPath)
	contextService := NewContextService(sceneService)

	// 创建物品服务(如果需要)
	itemService := NewItemService("data/items")

	// 🔧 获取角色服务并缓存
	var characterService *CharacterService
	if container := di.GetContainer(); container != nil {
		if charServiceObj := container.Get("character"); charServiceObj != nil {
			characterService = charServiceObj.(*CharacterService)
		}
	}

	service := &StoryService{
		SceneService:     sceneService,
		ContextService:   contextService,
		LLMService:       llmService,
		FileStorage:      fileStorage,
		ItemService:      itemService,
		CharacterService: characterService,
		BasePath:         basePath,
		lockManager:      NewLockManager(),
		storyCache:       make(map[string]*CachedStoryData),
		cacheExpiry:      5 * time.Minute, // 5分钟缓存
	}

	// 启动缓存清理
	service.startCacheCleanup()

	return service
}

// startCacheCleanup 启动缓存清理定时器
func (s *StoryService) startCacheCleanup() {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			s.cleanupExpiredCache()
		}
	}()
}

// cleanupExpiredCache 清理过期的缓存数据
func (s *StoryService) cleanupExpiredCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	now := time.Now()
	for sceneID, cached := range s.storyCache {
		// 不清理正在加载的数据
		if cached.Loading != nil {
			continue
		}
		if now.Sub(cached.Timestamp) > s.cacheExpiry {
			delete(s.storyCache, sceneID)
		}
	}
}

// GetStoryNode 根据场景和节点ID获取对应的故事节点
func (s *StoryService) GetStoryNode(sceneID, nodeID string) (*models.StoryNode, error) {
	if sceneID == "" {
		return nil, fmt.Errorf("scene_id 不能为空")
	}
	if nodeID == "" {
		return nil, fmt.Errorf("node_id 不能为空")
	}

	storyData, err := s.loadStoryDataSafe(sceneID)
	if err != nil {
		return nil, fmt.Errorf("加载故事数据失败: %w", err)
	}

	for i := range storyData.Nodes {
		node := storyData.Nodes[i]
		if node.ID == nodeID {
			result := node
			return &result, nil
		}
	}

	return nil, fmt.Errorf("未找到指定的故事节点: %s", nodeID)
}

// InitializeStoryForScene 初始化场景的故事线
func (s *StoryService) InitializeStoryForScene(sceneID string, preferences *models.UserPreferences) (*models.StoryData, error) {
	storyData, err := s.initializeStoryForSceneFast(sceneID, preferences)
	if err == nil {
		return storyData, nil
	}
	utils.GetLogger().Warn("fast story initialization failed; falling back to legacy", map[string]interface{}{
		"scene_id": sceneID,
		"err":      err.Error(),
	})
	return s.initializeStoryForSceneLegacy(sceneID, preferences)
}

// InitializeStoryForSceneFull 强制使用完整分析流程（保留旧实现）
func (s *StoryService) InitializeStoryForSceneFull(sceneID string, preferences *models.UserPreferences) (*models.StoryData, error) {
	return s.initializeStoryForSceneLegacy(sceneID, preferences)
}

func (s *StoryService) initializeStoryForSceneLegacy(sceneID string, preferences *models.UserPreferences) (*models.StoryData, error) {
	sceneData, err := s.SceneService.LoadSceneNoCache(sceneID)
	if err != nil {
		return nil, fmt.Errorf("加载场景失败: %w", err)
	}

	storyData, err := s.extractInitialStoryFromText(sceneData, preferences)
	if err != nil {
		utils.GetLogger().Warn("story initialization via llm failed; using fallback synthesis", map[string]interface{}{
			"scene_id": sceneID,
			"err":      err.Error(),
		})
		storyData, err = s.buildFallbackStoryData(sceneData)
		if err != nil {
			return nil, fmt.Errorf("提取故事信息失败: %w", err)
		}
	}

	if err := s.saveStoryData(sceneID, storyData); err != nil {
		return nil, fmt.Errorf("保存故事数据失败: %w", err)
	}

	return storyData, nil
}

func (s *StoryService) initializeStoryForSceneFast(sceneID string, preferences *models.UserPreferences) (*models.StoryData, error) {
	sceneData, err := s.SceneService.LoadSceneNoCache(sceneID)
	if err != nil {
		return nil, fmt.Errorf("加载场景失败: %w", err)
	}

	isEnglish := isEnglishText(sceneData.Scene.Name + " " + sceneData.Scene.Description)
	segments := s.prepareInitialSegments(sceneData, maxInitialSegmentNodes, isEnglish)
	if len(segments) == 0 {
		return nil, fmt.Errorf("无法生成原文片段用于快速初始化")
	}
	if len(segments) > maxInitialSegmentNodes {
		segments = segments[:maxInitialSegmentNodes]
	}
	if len(sceneData.OriginalSegments) == 0 {
		s.persistOriginalSegmentsForScene(sceneData, segments)
	}

	intro := strings.TrimSpace(sceneData.Scene.Description)
	if intro == "" {
		intro = pickLocale(isEnglish, "An urgent tale is unfolding.", "一段紧张剧情正在展开。")
	}
	mainObjective := strings.TrimSpace(sceneData.Scene.Summary)
	if mainObjective == "" {
		mainObjective = pickLocale(isEnglish, "Guide your allies through the crisis.", "引导同伴走出当前危局。")
	}

	storyData := &models.StoryData{
		SceneID:       sceneID,
		Intro:         intro,
		MainObjective: mainObjective,
		CurrentState:  pickLocale(isEnglish, "Initial", "初始"),
		Progress:      0,
		Nodes:         make([]models.StoryNode, 0, len(segments)),
		Tasks:         s.buildFallbackTasks(sceneData, isEnglish),
		Locations:     s.buildFallbackLocations(sceneData),
		LastUpdated:   time.Now(),
	}

	storyContext := s.buildQuickStoryContext(sceneData, storyData, isEnglish)

	for i, segment := range segments {
		original := strings.TrimSpace(segment.OriginalText)
		if original == "" {
			continue
		}
		node := models.StoryNode{
			ID:              fmt.Sprintf("node_%s_%d", sceneID, i+1),
			SceneID:         sceneID,
			OriginalContent: original,
			Content:         "",
			Type:            inferNodeType(i, len(segments)),
			Choices:         nil,
			IsRevealed:      i == 0,
			CreatedAt:       time.Now(),
			Source:          models.SourceSystem,
			Metadata: map[string]interface{}{
				"segment_strategy": "quick_init",
				"analyzed":         false,
			},
		}
		node.Choices = s.buildFallbackChoicesForSegment(&node, isEnglish)
		storyData.Nodes = append(storyData.Nodes, node)
	}

	if len(storyData.Nodes) == 0 {
		return nil, fmt.Errorf("快速初始化未能生成任何故事节点")
	}

	immediateNodes := 2
	if immediateNodes > len(storyData.Nodes) {
		immediateNodes = len(storyData.Nodes)
	}

	for i := 0; i < immediateNodes; i++ {
		s.enrichNodeFromSegment(&storyData.Nodes[i], storyContext, preferences, isEnglish)
		if storyData.Nodes[i].Metadata == nil {
			storyData.Nodes[i].Metadata = map[string]interface{}{}
		}
		storyData.Nodes[i].Metadata["analyzed"] = true
		delete(storyData.Nodes[i].Metadata, "pending_analysis")
	}

	for i := immediateNodes; i < len(storyData.Nodes); i++ {
		node := &storyData.Nodes[i]
		if strings.TrimSpace(node.Content) == "" {
			node.Content = s.synthesizeNarrationFromOriginal(node.OriginalContent, isEnglish)
		}
		if node.Metadata == nil {
			node.Metadata = map[string]interface{}{}
		}
		node.Metadata["pending_analysis"] = true
		node.Metadata["analyzed"] = false
	}

	if err := s.saveStoryData(sceneID, storyData); err != nil {
		return nil, fmt.Errorf("保存故事数据失败: %w", err)
	}

	go s.analyzeRemainingNodesAsync(sceneID, preferences)

	return storyData, nil
}

func (s *StoryService) buildQuickStoryContext(sceneData *SceneData, storyData *models.StoryData, isEnglish bool) string {
	var contextParts []string
	if sceneData != nil {
		title := strings.TrimSpace(sceneData.Scene.Name)
		if title == "" {
			title = strings.TrimSpace(sceneData.Scene.Title)
		}
		if title != "" {
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Scene", "场景"), title))
		}
		if desc := strings.TrimSpace(sceneData.Scene.Description); desc != "" {
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Description", "描述"), desc))
		}
		if era := strings.TrimSpace(sceneData.Scene.Era); era != "" {
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Era", "时代"), era))
		}
		if len(sceneData.Scene.Locations) > 0 {
			var locNames []string
			for _, loc := range sceneData.Scene.Locations {
				if name := strings.TrimSpace(loc.Name); name != "" {
					locNames = append(locNames, name)
				}
			}
			if len(locNames) > 0 {
				contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Locations", "关键地点"), strings.Join(locNames, ", ")))
			}
		}
		if len(sceneData.Characters) > 0 {
			var charNames []string
			for _, ch := range sceneData.Characters {
				if ch != nil {
					if name := strings.TrimSpace(ch.Name); name != "" {
						charNames = append(charNames, name)
					}
				}
			}
			if len(charNames) > 0 {
				contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Characters", "角色"), strings.Join(charNames, ", ")))
			}
		}
	}
	if storyData != nil {
		if intro := strings.TrimSpace(storyData.Intro); intro != "" {
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Intro", "开场"), intro))
		}
		if objective := strings.TrimSpace(storyData.MainObjective); objective != "" {
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Objective", "目标"), objective))
		}
	}
	return strings.Join(contextParts, "\n")
}

func (s *StoryService) analyzeRemainingNodesAsync(sceneID string, preferences *models.UserPreferences) {
	defer func() {
		if r := recover(); r != nil {
			utils.GetLogger().Error("background analyze crashed", map[string]interface{}{
				"scene_id": sceneID,
				"err":      fmt.Sprint(r),
			})
		}
	}()

	storyData, err := s.loadStoryDataSafe(sceneID)
	if err != nil {
		utils.GetLogger().Warn("background analyze skipped (load story failed)", map[string]interface{}{
			"scene_id": sceneID,
			"err":      err.Error(),
		})
		return
	}
	raw, err := json.Marshal(storyData)
	if err != nil {
		utils.GetLogger().Warn("background analyze skipped (marshal failed)", map[string]interface{}{
			"scene_id": sceneID,
			"err":      err.Error(),
		})
		return
	}
	var working models.StoryData
	if err := json.Unmarshal(raw, &working); err != nil {
		utils.GetLogger().Warn("background analyze skipped (unmarshal failed)", map[string]interface{}{
			"scene_id": sceneID,
			"err":      err.Error(),
		})
		return
	}

	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		utils.GetLogger().Warn("background analyze skipped (load scene failed)", map[string]interface{}{
			"scene_id": sceneID,
			"err":      err.Error(),
		})
		return
	}
	isEnglish := isEnglishText(sceneData.Scene.Name + " " + sceneData.Scene.Description)
	storyContext := s.buildQuickStoryContext(sceneData, &working, isEnglish)
	updated := false

	for i := range working.Nodes {
		node := &working.Nodes[i]
		if node.Metadata != nil {
			if analyzed, ok := node.Metadata["analyzed"].(bool); ok && analyzed {
				continue
			}
		}
		s.enrichNodeFromSegment(node, storyContext, preferences, isEnglish)
		if node.Metadata == nil {
			node.Metadata = map[string]interface{}{}
		}
		node.Metadata["analyzed"] = true
		delete(node.Metadata, "pending_analysis")
		updated = true
		working.LastUpdated = time.Now()
		if err := s.saveStoryData(sceneID, &working); err != nil {
			utils.GetLogger().Warn("background analyze save failed", map[string]interface{}{
				"scene_id": sceneID,
				"err":      err.Error(),
			})
			return
		}
		s.invalidateStoryCache(sceneID)
		time.Sleep(2 * time.Second)
	}

	if updated {
		utils.GetLogger().Info("background node analysis completed", map[string]interface{}{
			"scene_id": sceneID,
		})
	}
}

func (s *StoryService) buildFallbackStoryData(sceneData *SceneData) (*models.StoryData, error) {
	if sceneData == nil || sceneData.Scene.ID == "" {
		return nil, fmt.Errorf("缺少有效的场景数据")
	}
	isEnglish := isEnglishText(sceneData.Scene.Name + " " + sceneData.Scene.Description)
	segments := s.fallbackSegmentsFromScene(sceneData, maxInitialSegmentNodes)
	if len(segments) == 0 {
		return nil, fmt.Errorf("场景缺少可用的剧情片段")
	}
	intro := strings.TrimSpace(sceneData.Scene.Description)
	if intro == "" {
		intro = pickLocale(isEnglish, "An urgent tale is unfolding.", "一段紧张剧情正在展开。")
	}
	mainObjective := strings.TrimSpace(sceneData.Scene.Summary)
	if mainObjective == "" {
		mainObjective = pickLocale(isEnglish, "Guide your allies through the crisis.", "引导同伴走出当前危局。")
	}
	storyData := &models.StoryData{
		SceneID:       sceneData.Scene.ID,
		Intro:         intro,
		MainObjective: mainObjective,
		CurrentState:  pickLocale(isEnglish, "Initial", "初始"),
		Progress:      0,
		Nodes:         make([]models.StoryNode, 0, len(segments)),
		Tasks:         s.buildFallbackTasks(sceneData, isEnglish),
		Locations:     s.buildFallbackLocations(sceneData),
		LastUpdated:   time.Now(),
	}

	for i, segment := range segments {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" {
			continue
		}
		narrative := s.synthesizeNarrationFromOriginal(trimmed, isEnglish)
		node := models.StoryNode{
			ID:              fmt.Sprintf("node_%s_%d", sceneData.Scene.ID, i+1),
			SceneID:         sceneData.Scene.ID,
			OriginalContent: trimmed,
			Content:         narrative,
			Type:            inferNodeType(i, len(segments)),
			Choices:         nil,
			IsRevealed:      i == 0,
			CreatedAt:       time.Now(),
			Source:          models.SourceSystem,
			Metadata: map[string]interface{}{
				"fallback_generated": true,
			},
		}
		node.Choices = s.buildFallbackChoicesForSegment(&node, isEnglish)
		storyData.Nodes = append(storyData.Nodes, node)
	}

	if len(storyData.Nodes) == 0 {
		return nil, fmt.Errorf("无法构建任何故事节点")
	}

	return storyData, nil
}

func (s *StoryService) fallbackSegmentsFromScene(sceneData *SceneData, limit int) []string {
	segments := make([]string, 0, limit)
	for _, seg := range sceneData.OriginalSegments {
		text := strings.TrimSpace(seg.OriginalText)
		if text == "" {
			continue
		}
		segments = append(segments, text)
		if limit > 0 && len(segments) >= limit {
			return segments
		}
	}

	if len(segments) == 0 && strings.TrimSpace(sceneData.OriginalText) != "" {
		chunks := splitTextIntoSegments(sceneData.OriginalText, defaultSegmentRuneLimit)
		for _, chunk := range chunks {
			trimmed := strings.TrimSpace(chunk)
			if trimmed == "" {
				continue
			}
			segments = append(segments, trimmed)
			if limit > 0 && len(segments) >= limit {
				return segments
			}
		}
	}

	if len(segments) == 0 && strings.TrimSpace(sceneData.Scene.Description) != "" {
		segments = append(segments, strings.TrimSpace(sceneData.Scene.Description))
	}

	return segments
}

func (s *StoryService) buildFallbackLocations(sceneData *SceneData) []models.StoryLocation {
	if sceneData == nil {
		return nil
	}
	baseLocations := sceneData.Scene.Locations
	if len(baseLocations) == 0 {
		if strings.TrimSpace(sceneData.Scene.Title) != "" {
			baseLocations = []models.Location{{
				Name:        sceneData.Scene.Title,
				Description: sceneData.Scene.Description,
			}}
		}
	}
	locations := make([]models.StoryLocation, 0, len(baseLocations))
	for i, loc := range baseLocations {
		name := strings.TrimSpace(loc.Name)
		if name == "" {
			name = fmt.Sprintf("Location %d", i+1)
		}
		description := strings.TrimSpace(loc.Description)
		locations = append(locations, models.StoryLocation{
			ID:           fmt.Sprintf("loc_%s_%d", sceneData.Scene.ID, i+1),
			SceneID:      sceneData.Scene.ID,
			Name:         name,
			Description:  description,
			Accessible:   i == 0,
			RequiresItem: "",
			Source:       models.SourceSystem,
		})
	}
	return locations
}

func (s *StoryService) buildFallbackTasks(sceneData *SceneData, isEnglish bool) []models.Task {
	if sceneData == nil {
		return nil
	}
	taskDescription := strings.TrimSpace(sceneData.Scene.Summary)
	if taskDescription == "" {
		taskDescription = pickLocale(isEnglish, "Stabilize the retreat and locate key allies.", "稳住撤退阵线并寻找关键同伴。")
	}
	objectives := []models.Objective{
		{
			ID:          fmt.Sprintf("obj_%s_1", sceneData.Scene.ID),
			Description: pickLocale(isEnglish, "Assess the battlefield and calm the ranks.", "侦查战场、稳住军心。"),
			Completed:   false,
		},
		{
			ID:          fmt.Sprintf("obj_%s_2", sceneData.Scene.ID),
			Description: pickLocale(isEnglish, "Reunite scattered companions or protect civilians.", "寻回失散同伴或保护百姓。"),
			Completed:   false,
		},
	}

	return []models.Task{
		{
			ID:          fmt.Sprintf("task_%s_primary", sceneData.Scene.ID),
			SceneID:     sceneData.Scene.ID,
			Title:       pickLocale(isEnglish, "Stabilize the Situation", "稳住危局"),
			Description: taskDescription,
			Objectives:  objectives,
			Reward:      pickLocale(isEnglish, "Unlock new strategic opportunities.", "解锁新的行动机会。"),
			IsRevealed:  true,
			Source:      models.SourceSystem,
			Type:        "main",
			Status:      "active",
			Priority:    "high",
		},
	}
}

// 统一的故事数据加载方法
func (s *StoryService) loadStoryDataSafe(sceneID string) (*models.StoryData, error) {
	// 检查缓存
	s.cacheMutex.RLock()
	if cached, exists := s.storyCache[sceneID]; exists {
		if time.Since(cached.Timestamp) < s.cacheExpiry {
			s.cacheMutex.RUnlock()
			return cached.Data, nil
		} else if cached.Loading != nil {
			// 如果数据过期但正在加载，等待加载完成
			loading := cached.Loading
			s.cacheMutex.RUnlock()
			loading.Do(func() {}) // 等待加载完成
			// 重新检查缓存
			s.cacheMutex.RLock()
			if cached, exists := s.storyCache[sceneID]; exists && cached.Data != nil {
				s.cacheMutex.RUnlock()
				return cached.Data, nil
			}
			s.cacheMutex.RUnlock()
		} else {
			s.cacheMutex.RUnlock()
		}
	} else {
		s.cacheMutex.RUnlock()
	}

	// 缓存过期或不存在，需要重新加载
	storyPath := filepath.Join(s.BasePath, sceneID, "story.json")

	// 检查文件是否存在
	if _, err := os.Stat(storyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("故事数据不存在")
	}

	// 实现加载操作，确保只进行一次
	var loadedData *models.StoryData
	var loadErr error

	// 获取或创建加载标记
	s.cacheMutex.Lock()
	cached, exists := s.storyCache[sceneID]
	if !exists || cached.Loading == nil {
		// 创建新的加载标记
		s.storyCache[sceneID] = &CachedStoryData{
			Loading: &sync.Once{},
		}
		cached = s.storyCache[sceneID]
	}
	loading := cached.Loading
	s.cacheMutex.Unlock()

	// 使用 sync.Once 确保只加载一次
	loading.Do(func() {
		// 读取文件
		storyDataBytes, err := os.ReadFile(storyPath)
		if err != nil {
			loadErr = fmt.Errorf("读取故事数据失败: %w", err)
			return
		}

		var storyData models.StoryData
		if err := json.Unmarshal(storyDataBytes, &storyData); err != nil {
			loadErr = fmt.Errorf("解析故事数据失败: %w", err)
			return
		}

		s.ensureNodeRelatedItems(sceneID, &storyData, nil)
		s.ensureNodeLocations(&storyData)

		loadedData = &storyData

		// 更新缓存
		s.cacheMutex.Lock()
		s.storyCache[sceneID] = &CachedStoryData{
			Data:      &storyData,
			Timestamp: time.Now(),
			Loading:   nil, // 加载完成，清除加载标记
		}
		s.cacheMutex.Unlock()
	})

	if loadErr != nil {
		return nil, loadErr
	}

	return loadedData, nil
}

// 缓存失效方法
func (s *StoryService) invalidateStoryCache(sceneID string) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	delete(s.storyCache, sceneID)
}

// 从文本中提取初始故事节点和任务
func (s *StoryService) extractInitialStoryFromText(sceneData *SceneData, preferences *models.UserPreferences) (*models.StoryData, error) {
	// 检测场景语言
	isEnglish := isEnglishText(sceneData.Scene.Name + " " + sceneData.Scene.Description)

	// 如果场景名称和描述不能确定，尝试检查角色名称
	if !isEnglish && len(sceneData.Characters) > 0 {
		characterNames := ""
		for _, char := range sceneData.Characters {
			characterNames += char.Name + " "
		}
		isEnglish = isEnglishText(characterNames)
	}

	// 准备提示词
	var prompt, systemPrompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`Analyze the following scene and character information to create an initial setting for an interactive story:

Scene Title: %s
Scene Description: %s
Era/Period: %s
Main Locations: %s
Main Themes: %s

Key Characters:
%s

Based on this information, please create an engaging interactive story setup including:
1. Story background introduction
2. Main quests and objectives
3. Exploration locations
4. Initial story nodes
5. Possible story branches and decision points

Return in JSON format:
{
  "intro": "Overall story introduction",
  "main_objective": "Main story goal",
  "locations": [
    {
      "name": "Location name",
      "description": "Location description",
      "accessible": true/false,
      "requires_item": "Optional, ID of required item"
    }
  ],
  "initial_nodes": [
    {
      "content": "Story node content",
      "type": "main/side/hidden",
      "choices": [
        {
          "text": "Choice text",
          "consequence": "Choice consequence description",
          "next_node_hint": "Next node content hint"
        }
      ]
    }
  ],
  "tasks": [
    {
      "title": "Task title",
      "description": "Task description",
      "objectives": ["Objective 1", "Objective 2"],
      "reward": "Task reward description"
    }
  ]
}`,
			sceneData.Scene.Name,
			sceneData.Scene.Description,
			sceneData.Scene.Era,
			formatLocations(sceneData.Scene.Locations),
			formatThemes(sceneData.Scene.Themes),
			formatCharacters(sceneData.Characters),
		)

		systemPrompt = "You are a creative story designer responsible for creating engaging interactive stories."
	} else {
		// 中文提示词（原有逻辑）
		prompt = fmt.Sprintf(`分析以下场景和角色信息，创建一个交互式故事的初始设置：

场景标题: %s
场景描述: %s
时代背景: %s
主要地点: %s
主要主题: %s

主要角色:
%s

请根据这些信息创建一个有趣的交互式故事初始设置，包括：
1. 故事背景介绍
2. 主要任务和目标
3. 探索地点
4. 初始故事节点
5. 可能的故事分支和决策点

返回JSON格式:
{
  "intro": "故事总体介绍",
  "main_objective": "主要故事目标",
  "locations": [
    {
      "name": "地点名称",
      "description": "地点描述",
      "accessible": true/false,
      "requires_item": "可选，需要的物品ID"
    }
  ],
  "initial_nodes": [
    {
      "content": "故事节点内容",
      "type": "main/side/hidden",
      "choices": [
        {
          "text": "选择文本",
          "consequence": "选择后果描述",
          "next_node_hint": "下一个节点内容提示"
        }
      ]
    }
  ],
  "tasks": [
    {
      "title": "任务标题",
      "description": "任务描述",
      "objectives": ["目标1", "目标2"],
      "reward": "任务奖励描述"
    }
  ]
}`,
			sceneData.Scene.Name,
			sceneData.Scene.Description,
			sceneData.Scene.Era,
			formatLocations(sceneData.Scene.Locations),
			formatThemes(sceneData.Scene.Themes),
			formatCharacters(sceneData.Characters),
		)

		systemPrompt = "你是一个创意故事设计师，负责创建引人入胜的交互式故事。"
	}

	if s.LLMService == nil {
		if isEnglish {
			return nil, fmt.Errorf("LLM service is not configured")
		}
		return nil, fmt.Errorf("LLM服务未配置")
	}
	if !s.LLMService.IsReady() {
		state := s.LLMService.GetReadyState()
		if isEnglish {
			return nil, fmt.Errorf("LLM service not ready: %s", state)
		}
		return nil, fmt.Errorf("LLM服务未就绪: %s", state)
	}

	// Create a context with timeout for the LLM call
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	resp, err := s.LLMService.CreateChatCompletion(
		ctx,
		ChatCompletionRequest{
			Model: s.getLLMModel(preferences),
			Messages: []ChatCompletionMessage{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: prompt,
				},
			},
			// 请求JSON格式输出
			ExtraParams: map[string]interface{}{
				"response_format": map[string]string{
					"type": "json_object",
				},
			},
		},
	)

	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to generate story data: %w", err)
		} else {
			return nil, fmt.Errorf("生成故事数据失败: %w", err)
		}
	}

	jsonStr := SanitizeLLMJSONResponse(resp.Choices[0].Message.Content)

	// 解析返回的JSON
	var storySetup struct {
		Intro         string `json:"intro"`
		MainObjective string `json:"main_objective"`
		Locations     []struct {
			Name         string `json:"name"`
			Description  string `json:"description"`
			Accessible   bool   `json:"accessible"`
			RequiresItem string `json:"requires_item,omitempty"`
		} `json:"locations"`
		InitialNodes []struct {
			Content string `json:"content"`
			Type    string `json:"type"`
			Choices []struct {
				Text         string `json:"text"`
				Consequence  string `json:"consequence"`
				NextNodeHint string `json:"next_node_hint"`
			} `json:"choices"`
		} `json:"initial_nodes"`
		Tasks []struct {
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Objectives  []string `json:"objectives"`
			Reward      string   `json:"reward"`
		} `json:"tasks"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &storySetup); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to parse story data: %w", err)
		} else {
			return nil, fmt.Errorf("解析故事数据失败: %w", err)
		}
	}

	segmentsCache := s.prepareInitialSegments(sceneData, len(storySetup.InitialNodes), isEnglish)
	if len(sceneData.OriginalSegments) == 0 && len(segmentsCache) > 0 {
		s.persistOriginalSegmentsForScene(sceneData, segmentsCache)
	}
	if len(segmentsCache) == 0 {
		segmentsCache = s.resolveOriginalSegments(sceneData, len(storySetup.InitialNodes))
	}

	// 转换为故事数据模型
	storyData := &models.StoryData{
		SceneID:       sceneData.Scene.ID,
		Intro:         storySetup.Intro,
		MainObjective: storySetup.MainObjective,
		CurrentState: func() string {
			if isEnglish {
				return "Initial"
			}
			return "初始"
		}(),
		Progress:    0,
		Nodes:       []models.StoryNode{},
		Tasks:       []models.Task{},
		Locations:   []models.StoryLocation{},
		LastUpdated: time.Now(),
	}

	// 添加地点
	for _, loc := range storySetup.Locations {
		storyData.Locations = append(storyData.Locations, models.StoryLocation{
			ID:           fmt.Sprintf("loc_%s_%d", sceneData.Scene.ID, len(storyData.Locations)+1),
			SceneID:      sceneData.Scene.ID,
			Name:         loc.Name,
			Description:  loc.Description,
			Accessible:   loc.Accessible,
			RequiresItem: loc.RequiresItem,
			Source:       models.SourceExplicit,
		})
	}

	if len(segmentsCache) > 0 {
		effectiveSegments := segmentsCache
		if len(effectiveSegments) > maxInitialSegmentNodes {
			effectiveSegments = effectiveSegments[:maxInitialSegmentNodes]
		}

		var contextParts []string
		if sceneData != nil {
			title := strings.TrimSpace(sceneData.Scene.Name)
			if title == "" {
				title = strings.TrimSpace(sceneData.Scene.Title)
			}
			if title != "" {
				contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Scene", "场景"), title))
			}
			if desc := strings.TrimSpace(sceneData.Scene.Description); desc != "" {
				contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Description", "描述"), desc))
			}
			if era := strings.TrimSpace(sceneData.Scene.Era); era != "" {
				contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Era", "时代"), era))
			}
		}
		if intro := strings.TrimSpace(storySetup.Intro); intro != "" {
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Intro", "开场"), intro))
		}
		if objective := strings.TrimSpace(storySetup.MainObjective); objective != "" {
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Objective", "目标"), objective))
		}
		if len(storySetup.Locations) > 0 {
			var locNames []string
			for _, loc := range storySetup.Locations {
				if strings.TrimSpace(loc.Name) != "" {
					locNames = append(locNames, strings.TrimSpace(loc.Name))
				}
			}
			if len(locNames) > 0 {
				contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Locations", "关键地点"), strings.Join(locNames, ", ")))
			}
		}
		if sceneData != nil && len(sceneData.Characters) > 0 {
			var charNames []string
			for _, ch := range sceneData.Characters {
				if ch != nil && strings.TrimSpace(ch.Name) != "" {
					charNames = append(charNames, strings.TrimSpace(ch.Name))
				}
			}
			if len(charNames) > 0 {
				contextParts = append(contextParts, fmt.Sprintf("%s: %s", pickLocale(isEnglish, "Characters", "角色"), strings.Join(charNames, ", ")))
			}
		}
		storyContext := strings.Join(contextParts, "\n")

		for i := 0; i < len(effectiveSegments); i++ {
			storyNode := models.StoryNode{
				ID:         fmt.Sprintf("node_%s_%d", sceneData.Scene.ID, i+1),
				SceneID:    sceneData.Scene.ID,
				Content:    "",
				Type:       inferNodeType(i, len(effectiveSegments)),
				Choices:    []models.StoryChoice{},
				IsRevealed: i == 0,
				CreatedAt:  time.Now(),
				Source:     models.SourceStory,
				Metadata:   map[string]interface{}{"segment_strategy": "natural_split"},
			}

			s.assignOriginalSegmentToNode(sceneData.Scene.ID, sceneData, storyData, &storyNode, effectiveSegments)
			s.enrichNodeFromSegment(&storyNode, storyContext, preferences, isEnglish)

			if strings.TrimSpace(storyNode.Content) == "" {
				storyNode.Content = storyNode.OriginalContent
			}
			if len(storyNode.Choices) == 0 {
				storyNode.Choices = s.buildFallbackChoicesForSegment(&storyNode, isEnglish)
			}

			storyData.Nodes = append(storyData.Nodes, storyNode)
		}
	} else {
		for i, node := range storySetup.InitialNodes {
			var choices []models.StoryChoice
			for j, choice := range node.Choices {
				choices = append(choices, models.StoryChoice{
					ID:           fmt.Sprintf("choice_%s_%d_%d", sceneData.Scene.ID, i+1, j+1),
					Text:         choice.Text,
					Consequence:  choice.Consequence,
					NextNodeHint: choice.NextNodeHint,
					Selected:     false,
				})
			}

			storyNode := models.StoryNode{
				ID:              fmt.Sprintf("node_%s_%d", sceneData.Scene.ID, i+1),
				SceneID:         sceneData.Scene.ID,
				Content:         node.Content,
				OriginalContent: node.Content,
				Type:            node.Type,
				Choices:         choices,
				IsRevealed:      i == 0,
				CreatedAt:       time.Now(),
				Source:          models.SourceExplicit,
				Metadata:        map[string]interface{}{},
			}

			s.assignOriginalSegmentToNode(sceneData.Scene.ID, sceneData, storyData, &storyNode, segmentsCache)

			storyData.Nodes = append(storyData.Nodes, storyNode)
		}
	}

	// 添加任务
	for i, task := range storySetup.Tasks {
		objectives := make([]models.Objective, 0, len(task.Objectives))
		for j, obj := range task.Objectives {
			objectives = append(objectives, models.Objective{
				ID:          fmt.Sprintf("obj_%s_%d_%d", sceneData.Scene.ID, i+1, j+1),
				Description: obj,
				Completed:   false,
			})
		}

		storyData.Tasks = append(storyData.Tasks, models.Task{
			ID:          fmt.Sprintf("task_%s_%d", sceneData.Scene.ID, i+1),
			SceneID:     sceneData.Scene.ID,
			Title:       task.Title,
			Description: task.Description,
			Objectives:  objectives,
			Reward:      task.Reward,
			Completed:   false,
			IsRevealed:  i == 0, // 只有第一个任务默认显示
			Source:      models.SourceExplicit,
		})
	}

	return storyData, nil
}

// getLLMModel 根据用户偏好和可用配置获取合适的LLM模型名称
func (s *StoryService) getLLMModel(preferences *models.UserPreferences) string {
	// 如果提供了用户偏好设置，并且用户有指定模型
	if preferences != nil && preferences.PreferredModel != "" {
		return preferences.PreferredModel
	}

	// 使用LLMService的GetDefaultModel方法获取默认模型
	if s.LLMService != nil {
		defaultModel := s.LLMService.GetDefaultModel()
		if defaultModel != "" {
			return defaultModel
		}
	}

	// 极少数情况下如果仍然无法获取模型，使用通用默认值
	return "gpt-4o"
}

// 保存故事数据到文件
func (s *StoryService) saveStoryData(sceneID string, storyData *models.StoryData) error {
	storyDataJSON, err := json.MarshalIndent(storyData, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化故事数据失败: %w", err)
	}

	// 确保目录存在
	storyDir := filepath.Join(s.BasePath, sceneID)
	if err := os.MkdirAll(storyDir, 0755); err != nil {
		return fmt.Errorf("创建故事目录失败: %w", err)
	}

	storyPath := filepath.Join(storyDir, "story.json")

	// 🔧 原子性文件写入
	tempPath := storyPath + ".tmp"

	if err := os.WriteFile(tempPath, storyDataJSON, 0644); err != nil {
		return fmt.Errorf("保存故事数据失败: %w", err)
	}

	if err := os.Rename(tempPath, storyPath); err != nil {
		os.Remove(tempPath) // 清理临时文件
		return fmt.Errorf("保存故事数据失败: %w", err)
	}

	return nil
}

func (s *StoryService) resolveOriginalSegments(sceneData *SceneData, expectedCount int) []models.OriginalSegment {
	if sceneData != nil && len(sceneData.OriginalSegments) > 0 {
		segments := make([]models.OriginalSegment, len(sceneData.OriginalSegments))
		copy(segments, sceneData.OriginalSegments)
		if expectedCount > 0 && len(segments) > 0 && len(segments) < expectedCount {
			last := segments[len(segments)-1]
			for len(segments) < expectedCount {
				duplicated := last
				duplicated.Index = len(segments)
				segments = append(segments, duplicated)
			}
		}
		return segments
	}

	if sceneData == nil {
		return nil
	}
	fallbackText := strings.TrimSpace(sceneData.OriginalText)
	if fallbackText == "" {
		fallbackText = strings.TrimSpace(sceneData.Scene.Summary)
	}
	if fallbackText == "" {
		fallbackText = strings.TrimSpace(sceneData.Scene.Description)
	}
	if fallbackText == "" {
		return nil
	}
	isEnglish := isEnglishText(sceneData.Scene.Title + " " + fallbackText)
	segment := models.OriginalSegment{
		Index:        0,
		Title:        pickLocale(isEnglish, "Original Segment", "原文片段"),
		Summary:      "",
		OriginalText: fallbackText,
	}
	return []models.OriginalSegment{segment}
}

func (s *StoryService) assignOriginalSegmentToNode(sceneID string, sceneData *SceneData, storyData *models.StoryData, node *models.StoryNode, cachedSegments []models.OriginalSegment) {
	if node == nil || storyData == nil {
		return
	}
	if strings.TrimSpace(node.OriginalContent) == "" {
		node.OriginalContent = node.Content
	}

	segments := cachedSegments
	if len(segments) == 0 {
		if sceneData == nil {
			loadedScene, err := s.SceneService.LoadScene(sceneID)
			if err != nil {
				return
			}
			sceneData = loadedScene
		}
		segments = s.resolveOriginalSegments(sceneData, 0)
	}
	if len(segments) == 0 {
		return
	}

	nextIdx := determineNextOriginalSegmentIndex(storyData.Nodes, segments)
	if nextIdx >= len(segments) {
		nextIdx = len(segments) - 1
	}
	if nextIdx < 0 {
		nextIdx = 0
	}

	segment := segments[nextIdx]
	node.OriginalContent = segment.OriginalText
	if node.Metadata == nil {
		node.Metadata = map[string]interface{}{}
	}
	node.Metadata["original_segment_index"] = nextIdx
	if segment.Title != "" {
		node.Metadata["original_segment_title"] = segment.Title
	}
}

func determineNextOriginalSegmentIndex(nodes []models.StoryNode, segments []models.OriginalSegment) int {
	if len(segments) == 0 {
		return 0
	}
	maxIdx := -1
	for _, node := range nodes {
		if idx, ok := extractSegmentIndex(node.Metadata); ok {
			if idx > maxIdx {
				maxIdx = idx
			}
			continue
		}
		if idx := matchSegmentIndex(node.OriginalContent, segments); idx >= 0 {
			if idx > maxIdx {
				maxIdx = idx
			}
		}
	}
	return maxIdx + 1
}

func extractSegmentIndex(metadata map[string]interface{}) (int, bool) {
	if metadata == nil {
		return 0, false
	}
	value, ok := metadata["original_segment_index"]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed), true
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func matchSegmentIndex(content string, segments []models.OriginalSegment) int {
	if len(segments) == 0 {
		return -1
	}
	normalizedContent := normalizeComparisonText(content)
	if normalizedContent == "" {
		return -1
	}
	for idx, segment := range segments {
		if normalizeComparisonText(segment.OriginalText) == normalizedContent {
			return idx
		}
	}
	return -1
}

func normalizeComparisonText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n", "")
	text = strings.ReplaceAll(text, " ", "")
	return text
}

func findNextUnrevealedNode(storyData *models.StoryData) *models.StoryNode {
	if storyData == nil {
		return nil
	}
	for i := range storyData.Nodes {
		if !storyData.Nodes[i].IsRevealed {
			return &storyData.Nodes[i]
		}
	}
	return nil
}

// markRevealedFromConversations 根据 context.json 中的 story_original / story_console 记录，标记节点为已揭示（用于内存态，保持 story.json 不变）
func markRevealedFromConversations(convs []models.Conversation, storyData *models.StoryData) {
	if storyData == nil || len(convs) == 0 {
		return
	}

	revealed := make(map[string]struct{})
	for _, conv := range convs {
		convType := ""
		if conv.Metadata != nil {
			if val, ok := conv.Metadata["conversation_type"].(string); ok {
				convType = strings.ToLower(strings.TrimSpace(val))
			}
		}
		speaker := strings.ToLower(strings.TrimSpace(conv.SpeakerID))
		if convType == "" && speaker == "story" {
			convType = "story_original"
		}
		if convType != "story_original" && convType != "story_console" {
			continue
		}
		nodeID := resolveConversationNodeID(conv)
		if nodeID == "" {
			continue
		}
		revealed[nodeID] = struct{}{}
	}

	if len(revealed) == 0 {
		return
	}

	for i := range storyData.Nodes {
		if _, ok := revealed[storyData.Nodes[i].ID]; ok {
			storyData.Nodes[i].IsRevealed = true
		}
	}
}

func findLatestRevealedNode(storyData *models.StoryData) *models.StoryNode {
	if storyData == nil {
		return nil
	}
	for i := len(storyData.Nodes) - 1; i >= 0; i-- {
		if storyData.Nodes[i].IsRevealed {
			return &storyData.Nodes[i]
		}
	}
	return nil
}

func (s *StoryService) generateContinuationNode(sceneID string, storyData *models.StoryData, preferences *models.UserPreferences, isEnglish bool) (*models.StoryNode, error) {
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, err
	}
	segments := s.resolveOriginalSegments(sceneData, 0)
	summary := buildStorySummary(storyData)
	consoleEntries := s.fetchRecentConsoleStoryEntries(sceneID, 5)
	consoleSection := formatConsoleStoryPromptSection(consoleEntries, isEnglish)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var systemPrompt, prompt string
	if isEnglish {
		systemPrompt = "You are a creative story designer who continues the narrative seamlessly."
		prompt = fmt.Sprintf(`Story so far:
%s

%s

Please continue the story by creating the next narrative node. Keep characters consistent and introduce new tension or resolution naturally.
Return JSON:
{
  "content": "Rich description",
  "type": "main/side/continuation",
  "choices": [
    {"text": "choice", "consequence": "result", "next_node_hint": "hint"}
  ]
}`, summary, consoleSection)
	} else {
		systemPrompt = "你是一个负责延续剧情的创意叙事设计师。"
		prompt = fmt.Sprintf(`目前剧情摘要：
%s

%s

请继续创作下一段剧情，保持角色性格与世界观一致，适度制造新的矛盾或推进。
返回JSON：
{
  "content": "详细描述",
  "type": "main/side/continuation",
  "choices": [
    {"text": "选项", "consequence": "后果", "next_node_hint": "提示"}
  ]
}`, summary, consoleSection)
	}

	resp, err := s.LLMService.CreateChatCompletion(ctx, ChatCompletionRequest{
		Model: s.getLLMModel(preferences),
		Messages: []ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
		ExtraParams: map[string]interface{}{
			"response_format": map[string]string{"type": "json_object"},
		},
	})
	if err != nil {
		utils.GetLogger().Warn("generate continuation node failed; using fallback", map[string]interface{}{
			"scene_id": sceneID,
			"err":      err.Error(),
		})
		return s.generateFallbackContinuationNode(sceneID, sceneData, storyData, segments, isEnglish), nil
	}

	jsonStr := SanitizeLLMJSONResponse(resp.Choices[0].Message.Content)
	var payload struct {
		Content string `json:"content"`
		Type    string `json:"type"`
		Choices []struct {
			Text        string `json:"text"`
			Consequence string `json:"consequence"`
			Hint        string `json:"next_node_hint"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		utils.GetLogger().Warn("parse continuation node failed; using fallback", map[string]interface{}{
			"scene_id": sceneID,
			"err":      err.Error(),
		})
		return s.generateFallbackContinuationNode(sceneID, sceneData, storyData, segments, isEnglish), nil
	}

	nodeID := fmt.Sprintf("node_%s_%d", sceneID, time.Now().UnixNano())
	storyNode := models.StoryNode{
		ID:              nodeID,
		SceneID:         sceneID,
		Content:         payload.Content,
		OriginalContent: payload.Content,
		Type:            payload.Type,
		IsRevealed:      false,
		CreatedAt:       time.Now(),
		Source:          models.SourceGenerated,
		Choices:         []models.StoryChoice{},
		Metadata:        map[string]interface{}{},
	}
	if storyNode.Type == "" {
		storyNode.Type = "continuation"
	}
	for i, choice := range payload.Choices {
		storyNode.Choices = append(storyNode.Choices, models.StoryChoice{
			ID:           fmt.Sprintf("choice_%s_%d", nodeID, i+1),
			Text:         choice.Text,
			Consequence:  choice.Consequence,
			NextNodeHint: choice.Hint,
			CreatedAt:    time.Now(),
		})
	}

	s.assignOriginalSegmentToNode(sceneID, sceneData, storyData, &storyNode, segments)
	storyData.Nodes = append(storyData.Nodes, storyNode)
	return &storyData.Nodes[len(storyData.Nodes)-1], nil
}

func (s *StoryService) generateContinuationStory(sceneID string, storyData *models.StoryData, preferences *models.UserPreferences, isEnglish bool, reason string) (*models.StoryNode, error) {
	node, err := s.generateContinuationNode(sceneID, storyData, preferences, isEnglish)
	if err != nil {
		return nil, err
	}
	if node != nil {
		if node.Metadata == nil {
			node.Metadata = make(map[string]interface{})
		}
		if strings.TrimSpace(reason) != "" {
			node.Metadata["continuation_reason"] = reason
		}
		node.Metadata["continuation_generated_at"] = time.Now().Format(time.RFC3339Nano)
	}
	return node, nil
}

func (s *StoryService) generateFallbackContinuationNode(sceneID string, sceneData *SceneData, storyData *models.StoryData, segments []models.OriginalSegment, isEnglish bool) *models.StoryNode {
	if len(segments) == 0 {
		segments = s.resolveOriginalSegments(sceneData, 0)
	}
	marker := fmt.Sprintf("node_%s_auto_%d", sceneID, time.Now().UnixNano())
	lastSnippet := strings.TrimSpace(extractLastSnippet(storyData, isEnglish))
	var content string
	if isEnglish {
		if lastSnippet != "" {
			content = fmt.Sprintf("[Auto-progress]\n%s\n\nThe system advances the story conservatively until AI services become available.", lastSnippet)
		} else {
			content = "[Auto-progress] The system advances the story while the AI writer is unavailable."
		}
	} else {
		if lastSnippet != "" {
			content = fmt.Sprintf("【系统临时续写】\n%s\n\n当前AI写作暂不可用，系统根据既有剧情小幅推进，请稍后再试。", lastSnippet)
		} else {
			content = "【系统临时续写】AI 剧情生成暂不可用，系统以安全策略自动推进一小段剧情。"
		}
	}

	choices := []models.StoryChoice{
		{
			ID:           fmt.Sprintf("choice_%s_keep_momentum", marker),
			Text:         pickLocale(isEnglish, "Hold position and regroup", "暂缓推进，整理队伍"),
			Consequence:  pickLocale(isEnglish, "Stabilize the team while waiting for new intel.", "稳定军心，等待新的灵感。"),
			NextNodeHint: pickLocale(isEnglish, "Try generating again once AI is back.", "AI 恢复后可再次推进剧情。"),
		},
		{
			ID:           fmt.Sprintf("choice_%s_push_forward", marker),
			Text:         pickLocale(isEnglish, "Improvise a bold maneuver", "强行推进剧情"),
			Consequence:  pickLocale(isEnglish, "Takes a risky step without AI guidance.", "在缺少AI的情况下冒险出招，风险自负。"),
			NextNodeHint: pickLocale(isEnglish, "Use manual commands to influence the story.", "尝试通过手动指令影响剧情方向。"),
		},
	}

	fallback := models.StoryNode{
		ID:              marker,
		SceneID:         sceneID,
		Content:         content,
		OriginalContent: content,
		Type:            "system",
		IsRevealed:      false,
		CreatedAt:       time.Now(),
		Source:          models.SourceSystem,
		Choices:         choices,
		Metadata:        map[string]interface{}{},
	}

	s.assignOriginalSegmentToNode(sceneID, sceneData, storyData, &fallback, segments)
	storyData.Nodes = append(storyData.Nodes, fallback)
	return &storyData.Nodes[len(storyData.Nodes)-1]
}

func extractLastSnippet(storyData *models.StoryData, isEnglish bool) string {
	if storyData == nil {
		return ""
	}
	for i := len(storyData.Nodes) - 1; i >= 0; i-- {
		node := storyData.Nodes[i]
		if node.IsRevealed && strings.TrimSpace(node.Content) != "" {
			return node.Content
		}
		if node.IsRevealed && strings.TrimSpace(node.OriginalContent) != "" {
			return node.OriginalContent
		}
	}
	if len(storyData.Nodes) > 0 {
		last := storyData.Nodes[len(storyData.Nodes)-1]
		if strings.TrimSpace(last.Content) != "" {
			return last.Content
		}
		return last.OriginalContent
	}
	if isEnglish {
		return "The army pauses to catch its breath."
	}
	return "队伍暂时止步，整理现状。"
}

func pickLocale(isEnglish bool, en, zh string) string {
	if isEnglish {
		return en
	}
	return zh
}

func (s *StoryService) prepareInitialSegments(sceneData *SceneData, expectedCount int, isEnglish bool) []models.OriginalSegment {
	if sceneData == nil {
		return nil
	}
	if len(sceneData.OriginalSegments) > 0 {
		segments := make([]models.OriginalSegment, len(sceneData.OriginalSegments))
		copy(segments, sceneData.OriginalSegments)
		return segments
	}
	baseText := strings.TrimSpace(sceneData.OriginalText)
	if baseText == "" {
		baseText = strings.TrimSpace(sceneData.Scene.Summary)
	}
	if baseText == "" {
		baseText = strings.TrimSpace(sceneData.Scene.Description)
	}
	if baseText == "" {
		return nil
	}
	segments := generateSegmentsFromText(baseText, isEnglish)
	if len(segments) == 0 {
		return nil
	}
	if expectedCount > 0 && len(segments) > 0 && len(segments) < expectedCount {
		last := segments[len(segments)-1]
		for len(segments) < expectedCount {
			dup := last
			dup.Index = len(segments)
			segments = append(segments, dup)
		}
	}
	return segments
}

func (s *StoryService) persistOriginalSegmentsForScene(sceneData *SceneData, segments []models.OriginalSegment) {
	if s == nil || s.SceneService == nil || sceneData == nil || len(segments) == 0 {
		return
	}
	sceneID := strings.TrimSpace(sceneData.Scene.ID)
	if sceneID == "" {
		return
	}
	sceneDir := filepath.Join(s.SceneService.BasePath, sceneID)
	if err := os.MkdirAll(sceneDir, 0755); err != nil {
		utils.GetLogger().Warn("failed to create scene directory for original segments", map[string]interface{}{
			"scene_id": sceneID,
			"err":      err.Error(),
		})
		return
	}
	if err := s.SceneService.saveOriginalSegments(sceneDir, segments); err != nil {
		utils.GetLogger().Warn("failed to save original segments", map[string]interface{}{
			"scene_id": sceneID,
			"err":      err.Error(),
		})
		return
	}
	sceneData.OriginalSegments = make([]models.OriginalSegment, len(segments))
	copy(sceneData.OriginalSegments, segments)
}

func splitTextIntoSegments(text string, maxLength int) []string {
	if maxLength <= 0 {
		maxLength = defaultSegmentRuneLimit
	}
	clean := strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if clean == "" {
		return nil
	}
	paragraphs := strings.Split(clean, "\n\n")
	filtered := make([]string, 0, len(paragraphs))
	for _, para := range paragraphs {
		if trimmed := strings.TrimSpace(para); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	if len(filtered) == 0 {
		filtered = []string{clean}
	}

	var segments []string
	var builder strings.Builder
	appendSegment := func() {
		segment := strings.TrimSpace(builder.String())
		if segment != "" {
			segments = append(segments, segment)
		}
		builder.Reset()
	}

	for _, para := range filtered {
		chunks := splitParagraphByLength(para, maxLength)
		for _, chunk := range chunks {
			if builder.Len() == 0 {
				builder.WriteString(chunk)
			} else {
				runeLen := utf8.RuneCountInString(builder.String())
				chunkLen := utf8.RuneCountInString(chunk)
				if runeLen+2+chunkLen > maxLength {
					appendSegment()
				}
				if builder.Len() > 0 {
					builder.WriteString("\n\n")
				}
				builder.WriteString(chunk)
			}
			if utf8.RuneCountInString(builder.String()) >= maxLength {
				appendSegment()
			}
		}
	}
	appendSegment()
	if len(segments) == 0 && clean != "" {
		segments = []string{clean}
	}
	return segments
}

func generateSegmentsFromText(baseText string, isEnglish bool) []models.OriginalSegment {
	cleanText := strings.TrimSpace(baseText)
	if cleanText == "" {
		return nil
	}
	rawSegments := splitTextIntoSegments(cleanText, defaultSegmentRuneLimit)
	if len(rawSegments) == 0 {
		rawSegments = []string{cleanText}
	}
	segments := make([]models.OriginalSegment, len(rawSegments))
	for i, seg := range rawSegments {
		segments[i] = models.OriginalSegment{
			Index:        i,
			Title:        fmt.Sprintf(pickLocale(isEnglish, "Original Segment %d", "原文片段 %d"), i+1),
			OriginalText: seg,
		}
	}
	return segments
}

func splitParagraphByLength(paragraph string, maxLength int) []string {
	if utf8.RuneCountInString(paragraph) <= maxLength {
		return []string{paragraph}
	}
	runes := []rune(paragraph)
	var chunks []string
	start := 0
	for start < len(runes) {
		end := start + maxLength
		if end > len(runes) {
			end = len(runes)
		} else {
			cursor := end
			for cursor > start+200 && cursor < len(runes) && !isPreferredBoundary(runes[cursor-1]) {
				cursor--
			}
			if cursor > start+200 {
				end = cursor
			}
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		start = end
	}
	return chunks
}

func isPreferredBoundary(r rune) bool {
	switch r {
	case '。', '！', '？', '.', '!', '?', '；', ';', '”', '"':
		return true
	default:
		return false
	}
}

func inferNodeType(position, total int) string {
	if total <= 2 {
		return "main"
	}
	switch {
	case position == 0:
		return "main"
	case position == total-1:
		return "main"
	case position%3 == 0:
		return "branch"
	default:
		return "side"
	}
}

func (s *StoryService) enrichNodeFromSegment(node *models.StoryNode, storyContext string, preferences *models.UserPreferences, isEnglish bool) {
	if node == nil {
		return
	}
	if node.Metadata == nil {
		node.Metadata = map[string]interface{}{}
	}
	originalText := strings.TrimSpace(node.OriginalContent)
	if originalText == "" {
		node.Content = ""
		if len(node.Choices) == 0 {
			node.Choices = s.buildFallbackChoicesForSegment(node, isEnglish)
		}
		return
	}
	if s.LLMService == nil || !s.LLMService.IsReady() {
		node.Content = s.synthesizeNarrationFromOriginal(originalText, isEnglish)
		node.Metadata["segment_enhanced"] = false
		if len(node.Choices) == 0 {
			node.Choices = s.buildFallbackChoicesForSegment(node, isEnglish)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	type segmentResponse struct {
		Content string `json:"content"`
		Type    string `json:"type"`
		Choices []struct {
			Text        string `json:"text"`
			Consequence string `json:"consequence"`
			Hint        string `json:"next_node_hint"`
		} `json:"choices"`
	}

	systemPrompt, userPrompt := buildSegmentPrompts(storyContext, node.OriginalContent, node.Type, isEnglish)
	resp, err := s.LLMService.CreateChatCompletion(ctx, ChatCompletionRequest{
		Model: s.getLLMModel(preferences),
		Messages: []ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		ExtraParams: map[string]interface{}{
			"response_format": map[string]string{"type": "json_object"},
		},
	})
	if err != nil {
		node.Content = s.synthesizeNarrationFromOriginal(originalText, isEnglish)
		node.Choices = s.buildFallbackChoicesForSegment(node, isEnglish)
		node.Metadata["segment_enhanced"] = false
		return
	}

	jsonStr := SanitizeLLMJSONResponse(resp.Choices[0].Message.Content)
	var payload segmentResponse
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		node.Content = s.synthesizeNarrationFromOriginal(originalText, isEnglish)
		node.Choices = s.buildFallbackChoicesForSegment(node, isEnglish)
		node.Metadata["segment_enhanced"] = false
		return
	}

	if strings.TrimSpace(payload.Content) == "" {
		payload.Content = s.synthesizeNarrationFromOriginal(originalText, isEnglish)
	}
	node.Content = strings.TrimSpace(payload.Content)
	if strings.TrimSpace(payload.Type) != "" {
		node.Type = strings.TrimSpace(payload.Type)
	}

	compiled := make([]models.StoryChoice, 0, len(payload.Choices))
	for i, choice := range payload.Choices {
		if strings.TrimSpace(choice.Text) == "" {
			continue
		}
		compiled = append(compiled, models.StoryChoice{
			ID:           fmt.Sprintf("choice_%s_%d", node.ID, i+1),
			Text:         strings.TrimSpace(choice.Text),
			Consequence:  strings.TrimSpace(choice.Consequence),
			NextNodeHint: strings.TrimSpace(choice.Hint),
			CreatedAt:    time.Now(),
			Type:         "branch",
		})
	}
	if len(compiled) == 0 {
		compiled = s.buildFallbackChoicesForSegment(node, isEnglish)
	}
	node.Choices = compiled
	node.Metadata["segment_enhanced"] = true
}

func (s *StoryService) synthesizeNarrationFromOriginal(original string, isEnglish bool) string {
	trimmed := strings.TrimSpace(original)
	if trimmed == "" {
		return pickLocale(isEnglish, "(No narrative available)", "（暂无剧情内容）")
	}
	summary := summarizeTextForNarration(trimmed, 360)
	hooks := generateActionHooks(isEnglish)
	if isEnglish {
		return fmt.Sprintf("Narrative focus:\n%s\n\nSuggested actions: %s.", summary, strings.Join(hooks, "; "))
	}
	return fmt.Sprintf("【剧情演绎】%s\n\n【行动提示】%s", summary, strings.Join(hooks, "、"))
}

func summarizeTextForNarration(text string, limit int) string {
	sentences := splitIntoSentencesForNarration(text)
	var builder strings.Builder
	for _, sentence := range sentences {
		trimmed := strings.TrimSpace(sentence)
		if trimmed == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteRune(' ')
		}
		builder.WriteString(trimmed)
		if utf8.RuneCountInString(builder.String()) >= limit {
			break
		}
	}
	summary := strings.TrimSpace(builder.String())
	if summary == "" {
		summary = truncateRunes(text, limit)
	} else if utf8.RuneCountInString(summary) > limit {
		summary = truncateRunes(summary, limit)
	}
	return summary
}

func splitIntoSentencesForNarration(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var sentences []string
	var current strings.Builder
	for _, r := range text {
		current.WriteRune(r)
		if strings.ContainsRune("。！？!?;；\n", r) {
			sentences = append(sentences, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		sentences = append(sentences, current.String())
	}
	return sentences
}

func generateActionHooks(isEnglish bool) []string {
	if isEnglish {
		return []string{"steady the line", "search for key allies", "stage a bold diversion"}
	}
	return []string{"稳住阵线", "寻回关键同伴", "策动佯攻突围"}
}

func buildSegmentPrompts(storyContext, originalText, nodeType string, isEnglish bool) (string, string) {
	contextSnippet := truncateRunes(strings.TrimSpace(storyContext), 1200)
	originalSnippet := truncateRunes(strings.TrimSpace(originalText), 1500)
	if contextSnippet == "" {
		contextSnippet = pickLocale(isEnglish, "An interactive drama set in a turbulent era.", "一个发生在动荡时代的互动剧情背景。")
	}
	if originalSnippet == "" {
		originalSnippet = pickLocale(isEnglish, "Original text missing", "原文缺失")
	}
	if nodeType == "" {
		nodeType = "main"
	}
	if isEnglish {
		systemPrompt := "You are an interactive fiction writer who enhances raw passages into immersive narrative nodes while preserving canonical plot beats."
		userPrompt := fmt.Sprintf(`Story context:
%s

Original passage:
%s

Node type: %s

Tasks:
1. Preserve core events while enriching description and atmosphere.
2. Maintain continuity with the story context and character motivations.
3. Provide 2-3 interactive choices with clear consequences and hints.
Return JSON: {"content":"...","type":"...","choices":[{"text":"...","consequence":"...","next_node_hint":"..."}]}`, contextSnippet, originalSnippet, nodeType)
		return systemPrompt, userPrompt
	}
	systemPrompt := "你是一位资深互动小说作者，需要在保留原剧情核心的基础上进行润色和扩写。"
	userPrompt := fmt.Sprintf(`剧情背景：
	%s

	原文片段：
	%s

	节点类型：%s

	任务：
	1. 保留关键事件，增强画面与情感。
	2. 保持与背景设定及角色动机的一致性。
	3. 生成2-3个可互动的选择，并附上后果与提示。
	仅返回 JSON：{"content":"...","type":"...","choices":[{"text":"...","consequence":"...","next_node_hint":"..."}]}`,
		contextSnippet, originalSnippet, nodeType)
	return systemPrompt, userPrompt

}

func truncateRunes(text string, max int) string {
	if max <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	if max > len(runes) {
		max = len(runes)
	}
	return string(runes[:max]) + "..."
}

func (s *StoryService) buildFallbackChoicesForSegment(node *models.StoryNode, isEnglish bool) []models.StoryChoice {
	if node == nil {
		return nil
	}
	now := time.Now()
	return []models.StoryChoice{
		{
			ID:           fmt.Sprintf("choice_%s_reflect", node.ID),
			Text:         pickLocale(isEnglish, "Reflect and assess", "整理局势"),
			Consequence:  pickLocale(isEnglish, "Gain insight into the current development.", "梳理现状，寻找新的线索。"),
			NextNodeHint: pickLocale(isEnglish, "Stabilize before the next move.", "稳住阵脚后再做决策。"),
			CreatedAt:    now,
			Type:         "main",
		},
		{
			ID:           fmt.Sprintf("choice_%s_act", node.ID),
			Text:         pickLocale(isEnglish, "Take bold action", "立即行动"),
			Consequence:  pickLocale(isEnglish, "Push the narrative with a decisive move.", "以果断举动推动剧情发展。"),
			NextNodeHint: pickLocale(isEnglish, "Expect consequences shaped by this risk.", "此举会引发新的变数。"),
			CreatedAt:    now,
			Type:         "branch",
		},
	}
}

func (s *StoryService) fetchRecentConsoleStoryEntries(sceneID string, limit int) []models.Conversation {
	if limit <= 0 {
		limit = 3
	}
	var source []models.Conversation
	if s.ContextService != nil {
		target := limit * 3
		if target < limit {
			target = limit
		}
		if entries, err := s.ContextService.GetRecentConsoleStoryEntries(sceneID, target); err == nil {
			source = entries
		} else {
			utils.GetLogger().Warn("context fetch failed", map[string]interface{}{
				"scene_id": sceneID,
				"err":      err.Error(),
			})
		}
	}
	if len(source) == 0 {
		sceneData, err := s.SceneService.LoadSceneNoCache(sceneID)
		if err != nil {
			utils.GetLogger().Warn("scene load failed for context fallback", map[string]interface{}{
				"scene_id": sceneID,
				"err":      err.Error(),
			})
			return nil
		}
		source = sceneData.Context.Conversations
	}
	return filterConsoleStoryEntries(source, limit)
}

func filterConsoleStoryEntries(conversations []models.Conversation, limit int) []models.Conversation {
	if len(conversations) == 0 || limit <= 0 {
		return nil
	}
	filtered := make([]models.Conversation, 0, limit)
	for i := len(conversations) - 1; i >= 0 && len(filtered) < limit; i-- {
		conv := conversations[i]
		if !isConsoleStorySpeaker(conv.SpeakerID) {
			continue
		}
		if content := resolveConversationContent(conv); content == "" {
			continue
		}
		filtered = append(filtered, conv)
	}
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
	return filtered
}

func isConsoleStorySpeaker(speakerID string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(speakerID))
	if trimmed == "" {
		return false
	}
	if trimmed == "story" {
		return true
	}
	return strings.HasPrefix(trimmed, "console_story")
}

func formatConsoleStoryPromptSection(entries []models.Conversation, isEnglish bool) string {
	if len(entries) == 0 {
		return pickLocale(isEnglish,
			"\nRecent console directives: None provided. Continue from current narrative state.\n",
			"\n最近没有控制台指令，可直接根据当前剧情继续。\n",
		)
	}
	var builder strings.Builder
	index := 1
	for _, conv := range entries {
		content := resolveConversationContent(conv)
		if content == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("%d. %s\n", index, content))
		index++
	}
	if builder.Len() == 0 {
		return pickLocale(isEnglish,
			"\nRecent console directives: None provided. Continue from current narrative state.\n",
			"\n最近没有控制台指令，可直接根据当前剧情继续。\n",
		)
	}
	if isEnglish {
		return fmt.Sprintf("\nRecent console directives (oldest → newest):\n%s", builder.String())
	}
	return fmt.Sprintf("\n最近的控制台指令（按时间顺序）：\n%s", builder.String())
}

func summarizeConversations(entries []models.Conversation) string {
	if len(entries) == 0 {
		return ""
	}
	var builder strings.Builder
	row := 1
	for _, conv := range entries {
		content := resolveConversationContent(conv)
		if content == "" {
			continue
		}
		label := resolveConversationLabel(conv)
		builder.WriteString(fmt.Sprintf("[%d|%s] %s\n", row, label, content))
		row++
	}
	return strings.TrimSpace(builder.String())
}

func resolveConversationLabel(conv models.Conversation) string {
	if conv.Metadata != nil {
		if speaker, ok := conv.Metadata["speaker_name"].(string); ok && strings.TrimSpace(speaker) != "" {
			return speaker
		}
		if channel, ok := conv.Metadata["channel"].(string); ok && channel == "user" {
			return "玩家"
		}
	}
	id := strings.ToLower(conv.SpeakerID)
	switch {
	case strings.HasPrefix(id, "console_character"):
		return "角色"
	case strings.HasPrefix(id, "console_group"):
		return "群戏"
	case strings.HasPrefix(id, "console_story"):
		return "旁白"
	case id == "web_user" || id == "user":
		return "玩家"
	default:
		if conv.SpeakerID != "" {
			return conv.SpeakerID
		}
		return "旁白"
	}
}

func resolveConversationContent(conv models.Conversation) string {
	if trimmed := strings.TrimSpace(conv.Content); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(conv.Message); trimmed != "" {
		return trimmed
	}
	if conv.Metadata != nil {
		if raw, ok := conv.Metadata["content"].(string); ok {
			if trimmed := strings.TrimSpace(raw); trimmed != "" {
				return trimmed
			}
		}
		if raw, ok := conv.Metadata["message"].(string); ok {
			if trimmed := strings.TrimSpace(raw); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func (s *StoryService) rewriteNodeWithContext(node *models.StoryNode, contextSummary string, preferences *models.UserPreferences, isEnglish bool) (string, error) {
	if s.LLMService == nil {
		return "", fmt.Errorf("LLM service unavailable")
	}
	if node == nil {
		return "", fmt.Errorf("story node is nil")
	}
	original := strings.TrimSpace(node.OriginalContent)
	if original == "" {
		return "", fmt.Errorf("node original content is empty")
	}
	summary := strings.TrimSpace(contextSummary)
	if summary == "" {
		summary = "暂无新的上下文变化"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var systemPrompt, userPrompt string
	if isEnglish {
		systemPrompt = "You are a senior narrative editor who adapts original passages according to new context."
		userPrompt = fmt.Sprintf(`context:
%s

original:
%s

Task:
1. Preserve the core plot beats from the original passage.
2. Adapt details and dialogue so they reflect the new context.
3. Keep tone and characterization consistent; output rewritten narrative only.`, summary, original)
	} else {
		systemPrompt = "你是一位资深剧情编辑，需要根据最新上下文改写原文。"
		userPrompt = fmt.Sprintf(`上下文：
%s

原文：
%s

任务：
1. 保留原文的关键情节；
2. 根据上下文调整细节与语气；
3. 保持人物设定一致，仅输出改写后的正文。
`, summary, original)
	}

	resp, err := s.LLMService.CreateChatCompletion(ctx, ChatCompletionRequest{
		Model: s.getLLMModel(preferences),
		Messages: []ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func (s *StoryService) resolvePendingCommand(sceneID string, currentNode *models.StoryNode) (*models.Conversation, error) {
	if s.ContextService == nil || currentNode == nil {
		return nil, nil
	}
	commands, err := s.ContextService.GetUserStoryCommandsByNode(sceneID, currentNode.ID, 1)
	if err != nil {
		return nil, err
	}
	if len(commands) == 0 {
		return nil, nil
	}
	latest := commands[len(commands)-1]
	latestID := strings.TrimSpace(latest.ID)
	lastConsumed := getLastConsumedCommandID(currentNode)
	if latestID != "" && lastConsumed != "" && latestID == lastConsumed {
		return nil, nil
	}
	return &latest, nil
}

func getLastConsumedCommandID(node *models.StoryNode) string {
	if node == nil || node.Metadata == nil {
		return ""
	}
	if raw, ok := node.Metadata["last_consumed_command_id"].(string); ok {
		return strings.TrimSpace(raw)
	}
	return ""
}

func markCommandAsConsumed(node *models.StoryNode, commandID string) {
	commandID = strings.TrimSpace(commandID)
	if node == nil || commandID == "" {
		return
	}
	if node.Metadata == nil {
		node.Metadata = make(map[string]interface{})
	}
	node.Metadata["last_consumed_command_id"] = commandID
	node.Metadata["last_consumed_command_at"] = time.Now().Format(time.RFC3339Nano)
}

func formatStoryProcessingEntries(entries []models.Conversation) string {
	if len(entries) == 0 {
		return "（暂无旁白加工）"
	}
	var builder strings.Builder
	index := 1
	for _, entry := range entries {
		content := resolveConversationContent(entry)
		if content == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("[%d] %s\n", index, content))
		index++
	}
	formatted := strings.TrimSpace(builder.String())
	if formatted == "" {
		return "（暂无旁白加工）"
	}
	return formatted
}

func (s *StoryService) rewriteNodeWithProcessing(originalSegment, processingSummary, userMessage string, preferences *models.UserPreferences, isEnglish bool) (string, error) {
	if s.LLMService == nil {
		return "", fmt.Errorf("LLM service unavailable")
	}
	original := strings.TrimSpace(originalSegment)
	if original == "" {
		return "", fmt.Errorf("original segment is empty")
	}
	if strings.TrimSpace(processingSummary) == "" {
		processingSummary = pickLocale(isEnglish, "(No processing entries available)", "（暂无旁白加工）")
	}
	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		userMessage = pickLocale(isEnglish, "(Player provided no additional directive.)", "（玩家未提供新的指令。）")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var systemPrompt, userPrompt string
	if isEnglish {
		systemPrompt = "You are a narrative host who continues the story based on the original and processing sections."
		userPrompt = fmt.Sprintf(`original:
%s

processing:
%s

user_new_message:
%s

Task:
1. Continue the story faithfully, incorporating the processing details and the player's latest intent.
2. Use at most 300 English words.
3. Keep tension without resolving the main conflict; output narrative only.`, original, processingSummary, userMessage)
	} else {
		systemPrompt = "你是互动故事的旁白主持人，需要根据 original 与 processing 顺序续写下一段剧情。"
		userPrompt = fmt.Sprintf(`original:
%s

processing:
%s

user_new_message:
%s

任务：
1. 在 original 基础上结合 processing 以及玩家最新指令，续写一段不超过400个汉字的剧情；
2. 推动事件发展但不要直接结束主要矛盾；
3. 保持人物语气与世界观一致，仅输出正文。
`, original, processingSummary, userMessage)
	}

	resp, err := s.LLMService.CreateChatCompletion(ctx, ChatCompletionRequest{
		Model: s.getLLMModel(preferences),
		Messages: []ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func updateStoryState(storyData *models.StoryData, isEnglish bool) {
	if storyData == nil {
		return
	}
	switch {
	case storyData.Progress >= 100:
		if isEnglish {
			storyData.CurrentState = "Ending"
		} else {
			storyData.CurrentState = "结局"
		}
	case storyData.Progress >= 75:
		if isEnglish {
			storyData.CurrentState = "Climax"
		} else {
			storyData.CurrentState = "高潮"
		}
	case storyData.Progress >= 50:
		if isEnglish {
			storyData.CurrentState = "Development"
		} else {
			storyData.CurrentState = "发展"
		}
	case storyData.Progress >= 25:
		if isEnglish {
			storyData.CurrentState = "Conflict"
		} else {
			storyData.CurrentState = "冲突"
		}
	default:
		if isEnglish {
			storyData.CurrentState = "Initial"
		} else {
			storyData.CurrentState = "初始"
		}
	}
}

func buildStorySummary(storyData *models.StoryData) string {
	if storyData == nil {
		return ""
	}
	var parts []string
	for _, node := range storyData.Nodes {
		if node.IsRevealed {
			if strings.TrimSpace(node.Content) != "" {
				parts = append(parts, node.Content)
			} else if strings.TrimSpace(node.OriginalContent) != "" {
				parts = append(parts, node.OriginalContent)
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// GetStoryDataSafe 获取场景的故事数据，线程安全
func (s *StoryService) GetStoryData(sceneID string, preferences *models.UserPreferences) (*models.StoryData, error) {
	var storyData *models.StoryData

	err := s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		storyPath := filepath.Join(s.BasePath, sceneID, "story.json")

		// 在锁内检查和读取
		if _, err := os.Stat(storyPath); os.IsNotExist(err) {
			// 如果不存在，创建初始故事数据
			data, err := s.InitializeStoryForScene(sceneID, preferences)
			if err != nil {
				return err
			}
			storyData = data
			// 初始化后也补全物品关联
			s.ensureNodeRelatedItems(sceneID, storyData, nil)
			s.ensureNodeLocations(storyData)
			return nil
		}

		// 读取故事数据
		storyDataBytes, err := os.ReadFile(storyPath)
		if err != nil {
			return fmt.Errorf("读取故事数据失败: %w", err)
		}

		var tempData models.StoryData
		if err := json.Unmarshal(storyDataBytes, &tempData); err != nil {
			return fmt.Errorf("解析故事数据失败: %w", err)
		}

		storyData = &tempData
		s.ensureNodeRelatedItems(sceneID, storyData, nil)
		s.ensureNodeLocations(storyData)
		s.applyContextReveals(sceneID, storyData)
		return nil
	})

	return storyData, err
}

// applyContextReveals 根据 context.json 中的 story_original / story_console 记录，标记对应节点为已揭示（仅返回给客户端，不写回文件）
func (s *StoryService) applyContextReveals(sceneID string, storyData *models.StoryData) {
	if s.SceneService == nil || storyData == nil {
		return
	}

	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		utils.GetLogger().Warn("load scene failed when applying context reveals", map[string]interface{}{
			"scene_id": sceneID,
			"err":      err.Error(),
		})
		return
	}

	revealed := make(map[string]struct{})
	for _, conv := range sceneData.Context.Conversations {
		convType := ""
		if conv.Metadata != nil {
			if val, ok := conv.Metadata["conversation_type"].(string); ok {
				convType = strings.ToLower(strings.TrimSpace(val))
			}
		}
		speaker := strings.ToLower(strings.TrimSpace(conv.SpeakerID))
		if convType == "" && speaker == "story" {
			convType = "story_original"
		}

		if convType != "story_original" && convType != "story_console" {
			continue
		}
		nodeID := resolveConversationNodeID(conv)
		if nodeID == "" {
			continue
		}
		revealed[nodeID] = struct{}{}
	}

	if len(revealed) == 0 {
		return
	}

	for i := range storyData.Nodes {
		if _, ok := revealed[storyData.Nodes[i].ID]; ok {
			storyData.Nodes[i].IsRevealed = true
		}
	}
}

// GetStoryForScene 获取指定场景的故事数据
func (s *StoryService) GetStoryForScene(sceneID string) (*models.StoryData, error) {
	storyData, err := s.loadStoryDataSafe(sceneID)
	if err != nil {
		return nil, fmt.Errorf("加载故事数据失败: %w", err)
	}
	s.applyContextReveals(sceneID, storyData)
	return storyData, nil
}

// MakeChoice 处理玩家做出的故事选择
func (s *StoryService) MakeChoice(sceneID, nodeID, choiceID string, preferences *models.UserPreferences) (*models.StoryNode, error) {
	var result *models.StoryNode

	err := s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		// 使用缓存加载数据
		storyData, err := s.loadStoryDataSafe(sceneID)
		if err != nil {
			return err
		}

		// 创建副本避免直接修改缓存数据
		storyDataCopy := *storyData

		// 查找节点和选择
		var currentNode *models.StoryNode
		var selectedChoice *models.StoryChoice

		for i, node := range storyDataCopy.Nodes {
			if node.ID == nodeID {
				currentNode = &storyDataCopy.Nodes[i]
				for j, choice := range node.Choices {
					if choice.ID == choiceID {
						if choice.Selected {
							return fmt.Errorf("选择已被选中")
						}
						selectedChoice = &currentNode.Choices[j]
						currentNode.Choices[j].Selected = true
						break
					}
				}
				break
			}
		}

		if currentNode == nil || selectedChoice == nil {
			return fmt.Errorf("无效的节点或选择")
		}

		// 生成下一个故事节点
		nextNode, err := s.generateNextStoryNodeWithData(sceneID, currentNode, selectedChoice, preferences, &storyDataCopy)
		if err != nil {
			selectedChoice.Selected = false
			return err
		}

		// 添加新节点
		storyDataCopy.Nodes = append(storyDataCopy.Nodes, *nextNode)

		// 更新进度
		storyDataCopy.Progress += 5
		if storyDataCopy.Progress > 100 {
			storyDataCopy.Progress = 100
		}

		// 更新状态
		s.updateStoryState(&storyDataCopy)

		// 保存数据
		if err := s.saveStoryData(sceneID, &storyDataCopy); err != nil {
			return err
		}

		// 清除缓存
		s.invalidateStoryCache(sceneID)

		result = nextNode
		return nil
	})

	return result, err
}

// 提取状态更新逻辑
func (s *StoryService) updateStoryState(storyData *models.StoryData) {
	storyData.LastUpdated = time.Now()

	if storyData.Progress >= 100 {
		storyData.CurrentState = "结局"
	} else if storyData.Progress >= 75 {
		storyData.CurrentState = "高潮"
	} else if storyData.Progress >= 50 {
		storyData.CurrentState = "发展"
	} else if storyData.Progress >= 25 {
		storyData.CurrentState = "冲突"
	}
}

// generateNextStoryNodeWithData 根据当前节点和选择生成下一个故事节点（接受已读取的数据）
func (s *StoryService) generateNextStoryNodeWithData(sceneID string, currentNode *models.StoryNode, selectedChoice *models.StoryChoice, preferences *models.UserPreferences, storyData *models.StoryData) (*models.StoryNode, error) {
	// 加载场景数据
	sceneData, err := s.SceneService.LoadSceneNoCache(sceneID)
	if err != nil {
		return nil, err
	}
	segments := s.resolveOriginalSegments(sceneData, 0)

	// 检测语言
	isEnglish := isEnglishText(sceneData.Scene.Name + " " + currentNode.Content + " " + selectedChoice.Text)

	// 获取创造性级别文本表示
	var creativityStr string
	var allowPlotTwists bool
	if preferences != nil {
		switch preferences.CreativityLevel {
		case models.CreativityStrict:
			if isEnglish {
				creativityStr = "Low"
			} else {
				creativityStr = "低"
			}
			allowPlotTwists = false
		case models.CreativityBalanced:
			if isEnglish {
				creativityStr = "Medium"
			} else {
				creativityStr = "中"
			}
			allowPlotTwists = true
		case models.CreativityExpansive:
			if isEnglish {
				creativityStr = "High"
			} else {
				creativityStr = "高"
			}
			allowPlotTwists = true
		default:
			if isEnglish {
				creativityStr = "Medium"
			} else {
				creativityStr = "中"
			}
			allowPlotTwists = true
		}
	} else {
		if isEnglish {
			creativityStr = "Medium"
		} else {
			creativityStr = "中"
		}
		allowPlotTwists = true
	}

	var prompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`In the world of "%s", the player encounters the following situation:

%s

Player's choice: %s
Choice consequence: %s
Hint for next node: %s

Current story progress: %d%%
Current state: %s

Based on this choice, create a new story node that advances the plot.
Creativity level: %s
Allow plot twists: %v

Respond with a JSON object in the following format:
{
  "content": "Detailed description of the story node",
  "type": "event/choice/interaction",
  "choices": [
    {
      "text": "Choice text",
      "consequence": "Brief description of possible consequences",
      "next_node_hint": "Hint for the next node content"
    }
  ],
  "new_task": {
    "title": "Optional, if there's a new task",
    "description": "Task description",
    "objectives": ["Objective 1", "Objective 2"],
    "reward": "Completion reward"
  },
  "new_location": {
    "name": "Optional, if there's a new location",
    "description": "Location description",
    "accessible": true
  },
  "new_item": {
    "name": "Optional, if there's a new item",
    "description": "Item description",
    "type": "Item type"
  },
  "character_interactions": [
    {
      "trigger_condition": "Condition when this interaction should happen",
      "character_ids": ["character1_id", "character2_id"],
      "topic": "Topic of interaction",
      "context_description": "Brief context for the interaction"
    }
  ]
}`,
			sceneData.Scene.Name,
			currentNode.Content,
			selectedChoice.Text,
			selectedChoice.Consequence,
			selectedChoice.NextNodeHint,
			storyData.Progress,
			storyData.CurrentState,
			creativityStr,
			allowPlotTwists)
	} else {
		// 中文提示词
		prompt = fmt.Sprintf(`在《%s》的世界中，玩家遇到了以下情况:

%s

玩家选择: %s
选择后果: %s
下一节点提示: %s

当前故事进度: %d%%
当前状态: %s

根据这个选择，创建一个新的故事节点来推进剧情。
创造性级别: %s
允许剧情转折: %v

返回JSON格式:
{
  "content": "详细的故事节点描述",
  "type": "event/choice/interaction",
  "choices": [
    {
      "text": "选项文本",
      "consequence": "可能的后果简述",
      "next_node_hint": "下一节点的内容提示"
    }
  ],
  "new_task": {
    "title": "可选，如果有新任务",
    "description": "任务描述",
    "objectives": ["目标1", "目标2"],
    "reward": "完成奖励"
  },
  "new_location": {
    "name": "可选，如果有新地点",
    "description": "地点描述",
    "accessible": true
  },
  "new_item": {
    "name": "可选，如果有新物品",
    "description": "物品描述",
    "type": "物品类型"
  },
  "character_interactions": [
    {
      "trigger_condition": "何时触发此互动的条件",
      "character_ids": ["角色1_id", "角色2_id"],
      "topic": "互动主题",
      "context_description": "互动的简要背景"
    }
  ]
}`,
			sceneData.Scene.Name,
			currentNode.Content,
			selectedChoice.Text,
			selectedChoice.Consequence,
			selectedChoice.NextNodeHint,
			storyData.Progress,
			storyData.CurrentState,
			creativityStr,
			allowPlotTwists)
	}

	// 根据语言选择系统提示词
	var systemPrompt string
	if isEnglish {
		systemPrompt = "You are a creative story designer responsible for creating engaging interactive stories with character interactions."
	} else {
		systemPrompt = "你是一个创意故事设计师，负责创建引人入胜的交互式故事，包括角色之间的互动。"
	}

	// Create a context with timeout for the LLM call
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	resp, err := s.LLMService.CreateChatCompletion(
		ctx,
		ChatCompletionRequest{
			Model: s.getLLMModel(preferences),
			Messages: []ChatCompletionMessage{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: prompt,
				},
			},
			// 请求JSON格式输出
			ExtraParams: map[string]interface{}{
				"response_format": map[string]string{
					"type": "json_object",
				},
			},
		},
	)

	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to generate next node: %w", err)
		} else {
			return nil, fmt.Errorf("生成下一个节点失败: %w", err)
		}
	}

	jsonStr := SanitizeLLMJSONResponse(resp.Choices[0].Message.Content)

	// 解析返回的JSON
	var nodeData struct {
		Content string `json:"content"`
		Type    string `json:"type"`
		Choices []struct {
			Text         string `json:"text"`
			Consequence  string `json:"consequence"`
			NextNodeHint string `json:"next_node_hint"`
		} `json:"choices"`
		NewTask *struct {
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Objectives  []string `json:"objectives"`
			Reward      string   `json:"reward"`
		} `json:"new_task,omitempty"`
		NewLocation *struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Accessible  bool   `json:"accessible"`
		} `json:"new_location,omitempty"`
		NewItem *struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Type        string `json:"type"`
		} `json:"new_item,omitempty"`
		CharacterInteractions []struct {
			TriggerCondition   string   `json:"trigger_condition"`
			CharacterIDs       []string `json:"character_ids"`
			Topic              string   `json:"topic"`
			ContextDescription string   `json:"context_description"`
		} `json:"character_interactions,omitempty"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &nodeData); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to parse node data: %w", err)
		} else {
			return nil, fmt.Errorf("解析节点数据失败: %w", err)
		}
	}

	// 创建新节点
	nodeID := fmt.Sprintf("node_%s_%d", sceneID, time.Now().UnixNano())
	newNode := &models.StoryNode{
		ID:         nodeID,
		SceneID:    sceneID,
		ParentID:   currentNode.ID,
		Content:    nodeData.Content,
		Type:       nodeData.Type,
		Choices:    []models.StoryChoice{},
		IsRevealed: true,
		CreatedAt:  time.Now(),
		Source:     models.SourceGenerated,
		Metadata:   map[string]interface{}{},
	}

	// 添加选择
	for i, choice := range nodeData.Choices {
		newNode.Choices = append(newNode.Choices, models.StoryChoice{
			ID:           fmt.Sprintf("choice_%s_%d", nodeID, i+1),
			Text:         choice.Text,
			Consequence:  choice.Consequence,
			NextNodeHint: choice.NextNodeHint,
			Selected:     false,
			CreatedAt:    time.Now(),
		})
	}

	// 处理角色互动触发器
	if len(nodeData.CharacterInteractions) > 0 {
		interactionTriggers := make([]models.InteractionTrigger, 0, len(nodeData.CharacterInteractions))
		for i, interaction := range nodeData.CharacterInteractions {
			trigger := models.InteractionTrigger{
				ID:                 fmt.Sprintf("trigger_%s_%d", nodeID, i+1),
				Condition:          interaction.TriggerCondition,
				CharacterIDs:       interaction.CharacterIDs,
				Topic:              interaction.Topic,
				ContextDescription: interaction.ContextDescription,
				Triggered:          false,
				CreatedAt:          time.Now(),
			}
			interactionTriggers = append(interactionTriggers, trigger)
		}
		newNode.Metadata["interaction_triggers"] = interactionTriggers
	}

	// 🔧 直接修改传入的 storyData，而不是重新读取
	// 添加新任务
	if nodeData.NewTask != nil {
		taskID := fmt.Sprintf("task_%s_%d", sceneID, time.Now().UnixNano())
		objectives := make([]models.Objective, 0, len(nodeData.NewTask.Objectives))
		for i, obj := range nodeData.NewTask.Objectives {
			objectives = append(objectives, models.Objective{
				ID:          fmt.Sprintf("obj_%s_%d", taskID, i+1),
				Description: obj,
				Completed:   false,
			})
		}

		task := models.Task{
			ID:          taskID,
			SceneID:     sceneID,
			Title:       nodeData.NewTask.Title,
			Description: nodeData.NewTask.Description,
			Objectives:  objectives,
			Reward:      nodeData.NewTask.Reward,
			Completed:   false,
			IsRevealed:  true,
			Source:      models.SourceGenerated,
		}

		storyData.Tasks = append(storyData.Tasks, task)
	}

	var createdLocation *models.StoryLocation
	// 添加新地点
	if nodeData.NewLocation != nil {
		locationID := fmt.Sprintf("loc_%s_%d", sceneID, time.Now().UnixNano())
		location := models.StoryLocation{
			ID:          locationID,
			SceneID:     sceneID,
			Name:        strings.TrimSpace(nodeData.NewLocation.Name),
			Description: nodeData.NewLocation.Description,
			Accessible:  nodeData.NewLocation.Accessible,
			Source:      models.SourceGenerated,
		}

		storyData.Locations = append(storyData.Locations, location)
		createdLocation = &storyData.Locations[len(storyData.Locations)-1]
	}

	// 🔧 异步处理新物品，避免在锁内调用外部服务
	if nodeData.NewItem != nil && s.ItemService != nil {
		item := &models.Item{
			ID:          fmt.Sprintf("item_%s_%d", sceneID, time.Now().UnixNano()),
			SceneID:     sceneID,
			Name:        nodeData.NewItem.Name,
			Description: nodeData.NewItem.Description,
			Type:        nodeData.NewItem.Type,
			IsOwned:     true,
			FoundAt:     time.Now(),
			Source:      models.SourceStory,
		}

		// 异步保存物品
		go func() {
			if err := s.ItemService.AddItem(sceneID, item); err != nil {
				utils.GetLogger().Warn("failed to save new item", map[string]interface{}{
					"scene_id":  sceneID,
					"item_id":   item.ID,
					"item_name": item.Name,
					"err":       err.Error(),
				})
			}
		}()
	}

	if len(newNode.RelatedItemIDs) == 0 {
		if related := s.detectRelatedItems(sceneID, newNode, sceneData); len(related) > 0 {
			newNode.RelatedItemIDs = related
		}
	}

	s.assignNodeLocationMetadata(storyData, newNode, selectedChoice, createdLocation)
	s.assignOriginalSegmentToNode(sceneID, sceneData, storyData, newNode, segments)

	return newNode, nil
}

// CompleteObjective 完成任务目标
func (s *StoryService) CompleteObjective(sceneID, taskID, objectiveID string) error {
	return s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		// 使用缓存加载
		storyData, err := s.loadStoryDataSafe(sceneID)
		if err != nil {
			return err
		}

		// 创建副本
		storyDataCopy := *storyData

		// 处理目标完成逻辑
		taskFound, objectiveFound := s.processObjectiveCompletion(&storyDataCopy, taskID, objectiveID)

		if !taskFound || !objectiveFound {
			return fmt.Errorf("无效的任务或目标")
		}

		// 保存和清除缓存
		if err := s.saveStoryData(sceneID, &storyDataCopy); err != nil {
			return err
		}

		s.invalidateStoryCache(sceneID)
		return nil
	})
}

// 提取目标完成逻辑
func (s *StoryService) processObjectiveCompletion(storyData *models.StoryData, taskID, objectiveID string) (bool, bool) {
	taskFound := false
	objectiveFound := false
	allObjectivesCompleted := true

	for i, task := range storyData.Tasks {
		if task.ID == taskID {
			taskFound = true
			for j, objective := range task.Objectives {
				if objective.ID == objectiveID {
					objectiveFound = true
					storyData.Tasks[i].Objectives[j].Completed = true
				}
				if !storyData.Tasks[i].Objectives[j].Completed {
					allObjectivesCompleted = false
				}
			}

			if allObjectivesCompleted {
				storyData.Tasks[i].Completed = true
				storyData.Progress += 10
				if storyData.Progress > 100 {
					storyData.Progress = 100
				}
				s.updateStoryState(storyData)
			}
			break
		}
	}

	return taskFound, objectiveFound
}

// UnlockLocation 解锁场景地点
func (s *StoryService) UnlockLocation(sceneID, locationID string) error {
	return s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		// 使用缓存加载
		storyData, err := s.loadStoryDataSafe(sceneID)
		if err != nil {
			return err
		}

		// 创建副本
		storyDataCopy := *storyData

		// 查找地点
		for i, location := range storyDataCopy.Locations {
			if location.ID == locationID {
				storyDataCopy.Locations[i].Accessible = true
				break
			}
		}

		// 保存并清除缓存
		if err := s.saveStoryData(sceneID, &storyDataCopy); err != nil {
			return err
		}

		s.invalidateStoryCache(sceneID)
		return nil
	})
}

// ExploreLocation 探索地点，可能触发新的故事节点或发现物品
func (s *StoryService) ExploreLocation(sceneID, locationID string, preferences *models.UserPreferences) (*models.ExplorationResult, error) {
	var result *models.ExplorationResult

	err := s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		// 直接读取文件
		storyPath := filepath.Join(s.BasePath, sceneID, "story.json")
		if _, err := os.Stat(storyPath); os.IsNotExist(err) {
			return fmt.Errorf("故事数据不存在")
		}

		storyDataBytes, err := os.ReadFile(storyPath)
		if err != nil {
			return fmt.Errorf("读取故事数据失败: %w", err)
		}

		var storyData models.StoryData
		if err := json.Unmarshal(storyDataBytes, &storyData); err != nil {
			return fmt.Errorf("解析故事数据失败: %w", err)
		}

		// 查找地点
		var location *models.StoryLocation
		for i, loc := range storyData.Locations {
			if loc.ID == locationID {
				location = &storyData.Locations[i]
				break
			}
		}

		if location == nil {
			return fmt.Errorf("地点不存在")
		}

		if !location.Accessible {
			return fmt.Errorf("此地点尚未解锁")
		}

		// 加载场景数据
		sceneData, err := s.SceneService.LoadSceneNoCache(sceneID)
		if err != nil {
			return err
		}

		// 检测语言
		isEnglish := isEnglishText(sceneData.Scene.Name + " " + location.Name + " " + location.Description)

		// 准备提示词和系统提示词
		var prompt, systemPrompt string

		if isEnglish {
			// 英文提示词
			prompt = fmt.Sprintf(`In the world of "%s", the player is exploring the location: %s
Location description: %s

Scene background: %s

Creativity level: %s
Allow plot twists: %v

Please describe what the player discovers while exploring this location, which may include:
1. Detailed environment description
2. Possible items found
3. Possible story events triggered
4. Hidden clues

Return in JSON format:
{
  "description": "Detailed exploration description",
  "found_item": {
    "name": "Item name",
    "description": "Item description",
    "type": "Item type"
  },
  "story_event": {
    "content": "Story event description",
    "type": "discovery/encounter/revelation",
    "choices": [
      {
        "text": "Choice text",
        "consequence": "Choice consequence"
      }
    ]
  },
  "new_clue": "Discovered clue"
}`,
				sceneData.Scene.Name,
				location.Name,
				location.Description,
				sceneData.Scene.Description,
				string(preferences.CreativityLevel),
				preferences.AllowPlotTwists)

			systemPrompt = "You are a creative story designer responsible for creating engaging interactive stories."
		} else {
			// 中文提示词
			prompt = fmt.Sprintf(`在《%s》的世界中，玩家正在探索地点: %s
地点描述: %s

场景背景: %s

创造性级别: %s
允许剧情转折: %v

请描述玩家探索这个地点的发现，可能包括:
1. 详细的环境描述
2. 可能发现的物品
3. 可能触发的故事事件
4. 隐藏的线索

返回JSON格式:
{
  "description": "详细的探索描述",
  "found_item": {
    "name": "物品名称",
    "description": "物品描述",
    "type": "物品类型"
  },
  "story_event": {
    "content": "故事事件描述",
    "type": "discovery/encounter/revelation",
    "choices": [
      {
        "text": "选择文本",
        "consequence": "选择后果"
      }
    ]
  },
  "new_clue": "发现的线索"
}`,
				sceneData.Scene.Name,
				location.Name,
				location.Description,
				sceneData.Scene.Description,
				string(preferences.CreativityLevel),
				preferences.AllowPlotTwists)

			systemPrompt = "你是一个创意故事设计师，负责创建引人入胜的交互式故事。"
		}

		// Create a context with timeout for the LLM call
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		resp, err := s.LLMService.CreateChatCompletion(
			ctx,
			ChatCompletionRequest{
				Model: s.getLLMModel(preferences),
				Messages: []ChatCompletionMessage{
					{
						Role:    "system",
						Content: systemPrompt,
					},
					{
						Role:    "user",
						Content: prompt,
					},
				},
				// 请求JSON格式输出
				ExtraParams: map[string]interface{}{
					"response_format": map[string]string{
						"type": "json_object",
					},
				},
			},
		)

		if err != nil {
			if isEnglish {
				return fmt.Errorf("failed to generate exploration result: %w", err)
			} else {
				return fmt.Errorf("生成探索结果失败: %w", err)
			}
		}

		jsonStr := SanitizeLLMJSONResponse(resp.Choices[0].Message.Content)

		// 解析返回的JSON
		var explorationData struct {
			Description string `json:"description"`
			FoundItem   *struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Type        string `json:"type"`
			} `json:"found_item,omitempty"`
			StoryEvent *struct {
				Content string `json:"content"`
				Type    string `json:"type"`
				Choices []struct {
					Text        string `json:"text"`
					Consequence string `json:"consequence"`
				} `json:"choices"`
			} `json:"story_event,omitempty"`
			NewClue string `json:"new_clue,omitempty"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &explorationData); err != nil {
			if isEnglish {
				return fmt.Errorf("failed to parse exploration data: %w", err)
			} else {
				return fmt.Errorf("解析探索数据失败: %w", err)
			}
		}

		// 构建探索结果
		result = &models.ExplorationResult{
			LocationID:   locationID,
			Description:  explorationData.Description,
			NewClue:      explorationData.NewClue,
			ExploredTime: time.Now(),
		}

		// 处理发现的物品
		if explorationData.FoundItem != nil {
			item := models.Item{
				ID:          fmt.Sprintf("item_%s_%d", sceneID, time.Now().UnixNano()),
				SceneID:     sceneID,
				Name:        explorationData.FoundItem.Name,
				Description: explorationData.FoundItem.Description,
				Type:        explorationData.FoundItem.Type,
				IsOwned:     true,
				FoundAt:     time.Now(),
				Source:      models.SourceExploration,
			}

			result.FoundItem = &item

			// 🔧 异步保存发现的物品，避免在锁内调用可能阻塞的外部服务
			if s.ItemService != nil {
				go func() {
					if err := s.ItemService.AddItem(sceneID, &item); err != nil {
						utils.GetLogger().Warn("failed to save discovered item", map[string]interface{}{
							"scene_id":  sceneID,
							"item_id":   item.ID,
							"item_name": item.Name,
							"err":       err.Error(),
						})
					}
				}()
			} else {
				// 记录日志：ItemService未初始化，物品仅返回但未持久化
				utils.GetLogger().Warn("item service not initialized; item not persisted", map[string]interface{}{
					"scene_id":  sceneID,
					"item_id":   item.ID,
					"item_name": item.Name,
				})
			}
		}

		// 处理故事事件
		if explorationData.StoryEvent != nil {
			// 创建新的故事节点
			nodeID := fmt.Sprintf("node_%s_%d", sceneID, time.Now().UnixNano())
			storyNode := models.StoryNode{
				ID:         nodeID,
				SceneID:    sceneID,
				Content:    explorationData.StoryEvent.Content,
				Type:       explorationData.StoryEvent.Type,
				CreatedAt:  time.Now(),
				IsRevealed: true,
				Source:     models.SourceExploration,
				Choices:    []models.StoryChoice{},
			}

			// 添加选择
			for i, choice := range explorationData.StoryEvent.Choices {
				storyNode.Choices = append(storyNode.Choices, models.StoryChoice{
					ID:          fmt.Sprintf("choice_%s_%d", nodeID, i+1),
					Text:        choice.Text,
					Consequence: choice.Consequence,
					Selected:    false,
					CreatedAt:   time.Now(),
				})
			}

			// 将节点添加到故事数据
			storyData.Nodes = append(storyData.Nodes, storyNode)
			result.StoryNode = &storyNode

			// 🔧 在锁内保存更新后的故事数据
			if err := s.saveStoryData(sceneID, &storyData); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// ensureNodeRelatedItems 为缺少关联物品的节点推断关联
func (s *StoryService) ensureNodeRelatedItems(sceneID string, storyData *models.StoryData, sceneData *SceneData) {
	if storyData == nil || len(storyData.Nodes) == 0 {
		return
	}

	missing := false
	for i := range storyData.Nodes {
		if len(storyData.Nodes[i].RelatedItemIDs) == 0 {
			missing = true
			break
		}
	}

	if !missing {
		return
	}

	items := s.resolveSceneItems(sceneID, sceneData)
	if len(items) == 0 {
		return
	}

	for i := range storyData.Nodes {
		if len(storyData.Nodes[i].RelatedItemIDs) > 0 {
			continue
		}
		if related := matchItemsByText(items, storyData.Nodes[i].Content, storyData.Nodes[i].OriginalContent); len(related) > 0 {
			storyData.Nodes[i].RelatedItemIDs = related
		}
	}
}

func (s *StoryService) ensureNodeLocations(storyData *models.StoryData) {
	if storyData == nil || len(storyData.Nodes) == 0 {
		return
	}

	currentLocation := s.defaultLocationCandidate(storyData)
	for i := range storyData.Nodes {
		node := &storyData.Nodes[i]
		if node.Metadata != nil {
			if raw, ok := node.Metadata["current_location_id"].(string); ok && strings.TrimSpace(raw) != "" {
				if loc := findStoryLocationByID(storyData.Locations, raw); loc != nil {
					currentLocation = loc
					if _, exists := node.Metadata["current_location_name"]; !exists && strings.TrimSpace(loc.Name) != "" {
						node.Metadata["current_location_name"] = loc.Name
					}
					continue
				}
			}
		}

		// 尝试根据地点名称匹配节点文本，推断更准确的位置
		if match := matchStoryLocationByContent(storyData.Locations, node.Content, node.OriginalContent); match != nil {
			currentLocation = match
			if node.Metadata == nil {
				node.Metadata = make(map[string]interface{})
			}
			node.Metadata["current_location_id"] = match.ID
			if name := strings.TrimSpace(match.Name); name != "" {
				node.Metadata["current_location_name"] = name
			}
			continue
		}
		if currentLocation == nil {
			currentLocation = s.defaultLocationCandidate(storyData)
			if currentLocation == nil {
				return
			}
		}
		if node.Metadata == nil {
			node.Metadata = make(map[string]interface{})
		}
		node.Metadata["current_location_id"] = currentLocation.ID
		if name := strings.TrimSpace(currentLocation.Name); name != "" {
			node.Metadata["current_location_name"] = name
		}
	}
}

func (s *StoryService) assignNodeLocationMetadata(storyData *models.StoryData, node *models.StoryNode, selectedChoice *models.StoryChoice, createdLocation *models.StoryLocation) {
	if storyData == nil || node == nil {
		return
	}

	location := createdLocation
	if location == nil && selectedChoice != nil {
		if loc := matchStoryLocationByHint(storyData.Locations, selectedChoice.NextNodeHint); loc != nil {
			location = loc
		}
	}
	if location == nil {
		location = s.inferLocationFromHistory(storyData)
	}
	if location == nil {
		return
	}

	location.Accessible = true
	if node.Metadata == nil {
		node.Metadata = make(map[string]interface{})
	}
	node.Metadata["current_location_id"] = location.ID
	if name := strings.TrimSpace(location.Name); name != "" {
		node.Metadata["current_location_name"] = name
	}
}

func (s *StoryService) inferLocationFromHistory(storyData *models.StoryData) *models.StoryLocation {
	if storyData == nil {
		return nil
	}
	for i := len(storyData.Nodes) - 1; i >= 0; i-- {
		node := storyData.Nodes[i]
		if node.Metadata == nil {
			continue
		}
		if raw, ok := node.Metadata["current_location_id"].(string); ok && strings.TrimSpace(raw) != "" {
			if loc := findStoryLocationByID(storyData.Locations, raw); loc != nil {
				return loc
			}
		}
	}
	return s.defaultLocationCandidate(storyData)
}

func (s *StoryService) defaultLocationCandidate(storyData *models.StoryData) *models.StoryLocation {
	if storyData == nil || len(storyData.Locations) == 0 {
		return nil
	}
	for i := range storyData.Locations {
		if storyData.Locations[i].Accessible {
			return &storyData.Locations[i]
		}
	}
	return &storyData.Locations[0]
}

func matchStoryLocationByHint(locations []models.StoryLocation, hint string) *models.StoryLocation {
	normalized := strings.ToLower(strings.TrimSpace(hint))
	if normalized == "" {
		return nil
	}
	for i := range locations {
		name := strings.ToLower(strings.TrimSpace(locations[i].Name))
		if name == "" {
			continue
		}
		if name == normalized || strings.Contains(name, normalized) || strings.Contains(normalized, name) {
			return &locations[i]
		}
	}
	return nil
}

func matchStoryLocationByContent(locations []models.StoryLocation, contents ...string) *models.StoryLocation {
	if len(locations) == 0 {
		return nil
	}

	var merged strings.Builder
	for _, content := range contents {
		trimmed := strings.ToLower(strings.TrimSpace(content))
		if trimmed == "" {
			continue
		}
		if merged.Len() > 0 {
			merged.WriteByte(' ')
		}
		merged.WriteString(trimmed)
	}

	total := merged.String()
	if total == "" {
		return nil
	}

	for i := range locations {
		name := strings.ToLower(strings.TrimSpace(locations[i].Name))
		if name == "" {
			continue
		}
		if strings.Contains(total, name) {
			return &locations[i]
		}
	}

	return nil
}

func findStoryLocationByID(locations []models.StoryLocation, id string) *models.StoryLocation {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	for i := range locations {
		if strings.EqualFold(locations[i].ID, trimmed) {
			return &locations[i]
		}
	}
	return nil
}

// detectRelatedItems 返回节点文本涉及的物品ID列表
func (s *StoryService) detectRelatedItems(sceneID string, node *models.StoryNode, sceneData *SceneData) []string {
	if node == nil {
		return nil
	}

	items := s.resolveSceneItems(sceneID, sceneData)
	if len(items) == 0 {
		return nil
	}

	return matchItemsByText(items, node.Content, node.OriginalContent)
}

func (s *StoryService) resolveSceneItems(sceneID string, sceneData *SceneData) []*models.Item {
	if sceneData != nil && len(sceneData.Items) > 0 {
		return sceneData.Items
	}

	if s.SceneService != nil {
		if data, err := s.SceneService.LoadScene(sceneID); err == nil && len(data.Items) > 0 {
			return data.Items
		}
	}

	if s.ItemService != nil {
		if items, err := s.ItemService.GetAllItems(sceneID); err == nil {
			return items
		}
	}

	return nil
}

func matchItemsByText(items []*models.Item, contents ...string) []string {
	var builder strings.Builder
	for _, content := range contents {
		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(trimmed)
	}

	totalText := strings.ToLower(builder.String())
	if totalText == "" {
		return nil
	}

	related := make([]string, 0, 3)
	seen := make(map[string]struct{})

	for _, item := range items {
		if item == nil || item.ID == "" {
			continue
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}

		if containsItemKeyword(totalText, collectItemKeywords(item)) {
			related = append(related, item.ID)
			seen[item.ID] = struct{}{}
		}

		if len(related) >= 5 {
			break
		}
	}

	return related
}

func collectItemKeywords(item *models.Item) []string {
	keywords := make([]string, 0, 4)
	if name := strings.TrimSpace(item.Name); name != "" {
		keywords = append(keywords, name)
	}
	if item.Properties != nil {
		if aliases := normalizeStringSlice(item.Properties["aliases"]); len(aliases) > 0 {
			keywords = append(keywords, aliases...)
		}
		if alias := normalizeStringSlice(item.Properties["alias"]); len(alias) > 0 {
			keywords = append(keywords, alias...)
		}
		if alt := normalizeStringSlice(item.Properties["alt_names"]); len(alt) > 0 {
			keywords = append(keywords, alt...)
		}
	}
	return keywords
}

func normalizeStringSlice(value interface{}) []string {
	var result []string
	switch v := value.(type) {
	case []string:
		result = append(result, v...)
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
	case string:
		parts := strings.Split(v, ",")
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	return result
}

func containsItemKeyword(content string, keywords []string) bool {
	for _, keyword := range keywords {
		candidate := strings.ToLower(strings.TrimSpace(keyword))
		if len(candidate) < 2 {
			continue
		}
		if strings.Contains(content, candidate) {
			return true
		}
	}
	return false
}

// GetAvailableChoices 获取当前可用的剧情选择
func (s *StoryService) GetAvailableChoices(sceneID string) ([]models.StoryChoice, error) {
	var availableChoices []models.StoryChoice

	err := s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		// 🔧 在锁内直接读取文件
		storyPath := filepath.Join(s.BasePath, sceneID, "story.json")
		if _, err := os.Stat(storyPath); os.IsNotExist(err) {
			return fmt.Errorf("故事数据不存在")
		}

		storyDataBytes, err := os.ReadFile(storyPath)
		if err != nil {
			return fmt.Errorf("读取故事数据失败: %w", err)
		}

		var storyData models.StoryData
		if err := json.Unmarshal(storyDataBytes, &storyData); err != nil {
			return fmt.Errorf("解析故事数据失败: %w", err)
		}

		// 查找最新的、已显示的、未选择的故事节点
		var latestRevealedNode *models.StoryNode
		latestTime := time.Time{}

		for i := range storyData.Nodes {
			node := &storyData.Nodes[i]
			if node.IsRevealed && node.CreatedAt.After(latestTime) {
				hasUnselectedChoices := false
				for _, choice := range node.Choices {
					if !choice.Selected {
						hasUnselectedChoices = true
						break
					}
				}

				if hasUnselectedChoices {
					latestRevealedNode = node
					latestTime = node.CreatedAt
				}
			}
		}

		// 收集未选择的选项
		if latestRevealedNode != nil {
			for _, choice := range latestRevealedNode.Choices {
				if !choice.Selected {
					availableChoices = append(availableChoices, choice)
				}
			}
		}

		return nil
	})

	return availableChoices, err
}

// UpdateNodeChoices 用新的候选选项覆盖指定节点的可选项
func (s *StoryService) UpdateNodeChoices(sceneID, nodeID string, choices []models.StoryChoice) error {
	if strings.TrimSpace(sceneID) == "" || strings.TrimSpace(nodeID) == "" {
		return fmt.Errorf("scene_id 或 node_id 不能为空")
	}

	return s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		storyPath := filepath.Join(s.BasePath, sceneID, "story.json")
		if _, err := os.Stat(storyPath); os.IsNotExist(err) {
			return fmt.Errorf("故事数据不存在")
		}

		storyDataBytes, err := os.ReadFile(storyPath)
		if err != nil {
			return fmt.Errorf("读取故事数据失败: %w", err)
		}

		var storyData models.StoryData
		if err := json.Unmarshal(storyDataBytes, &storyData); err != nil {
			return fmt.Errorf("解析故事数据失败: %w", err)
		}

		var targetNode *models.StoryNode
		for i := range storyData.Nodes {
			if storyData.Nodes[i].ID == nodeID {
				targetNode = &storyData.Nodes[i]
				break
			}
		}
		if targetNode == nil {
			return fmt.Errorf("未找到指定的故事节点: %s", nodeID)
		}

		now := time.Now()
		normalized := make([]models.StoryChoice, 0, len(choices))
		for idx, choice := range choices {
			if strings.TrimSpace(choice.Text) == "" {
				continue
			}
			clone := choice
			if clone.ID == "" {
				clone.ID = fmt.Sprintf("choice_%s_custom_%d_%d", nodeID, now.Unix(), idx+1)
			}
			clone.Selected = false
			clone.IsSelected = false
			if clone.CreatedAt.IsZero() {
				clone.CreatedAt = now
			}
			if clone.Type == "" {
				clone.Type = "branch"
			}
			if clone.Metadata == nil {
				clone.Metadata = make(map[string]interface{})
			}
			clone.Metadata["source"] = "story_command"
			normalized = append(normalized, clone)
		}

		if len(normalized) == 0 {
			return fmt.Errorf("未提供有效的候选选项")
		}

		targetNode.Choices = normalized
		if targetNode.Metadata == nil {
			targetNode.Metadata = make(map[string]interface{})
		}
		targetNode.Metadata["choices_updated_at"] = now.Format(time.RFC3339Nano)
		targetNode.Metadata["choices_source"] = "story_command"
		storyData.LastUpdated = now

		if err := s.saveStoryData(sceneID, &storyData); err != nil {
			return err
		}

		s.invalidateStoryCache(sceneID)
		return nil
	})
}

// AdvanceStory 推进故事情节
func (s *StoryService) AdvanceStory(sceneID string, preferences *models.UserPreferences) (*models.StoryUpdate, error) {
	var storyUpdate *models.StoryUpdate

	err := s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		// 只读加载 story.json
		storyPath := filepath.Join(s.BasePath, sceneID, "story.json")
		if _, err := os.Stat(storyPath); os.IsNotExist(err) {
			return fmt.Errorf("故事数据不存在")
		}

		storyDataBytes, err := os.ReadFile(storyPath)
		if err != nil {
			return fmt.Errorf("读取故事数据失败: %w", err)
		}

		var storyData models.StoryData
		if err := json.Unmarshal(storyDataBytes, &storyData); err != nil {
			return fmt.Errorf("解析故事数据失败: %w", err)
		}

		// 加载场景与上下文（禁用缓存，确保读取最新 context.json 防止覆盖）
		sceneData, err := s.SceneService.LoadSceneNoCache(sceneID)
		if err != nil {
			return err
		}

		// 根据已记录的 story_original/story_console 先行标记已揭示节点，防止重复推进同一节点
		markRevealedFromConversations(sceneData.Context.Conversations, &storyData)

		// 找到下一个未揭示节点（仅用于标识，不立即修改 story.json）
		nextNode := findNextUnrevealedNode(&storyData)

		// 若存在下一个节点，先将其原文追加到 context.json（speaker: story，conversation_type: story_original）
		var nextNodeID string
		if nextNode != nil {
			nextNodeID = nextNode.ID
			snippet := strings.TrimSpace(nextNode.Content)
			if snippet == "" {
				snippet = strings.TrimSpace(nextNode.OriginalContent)
			}
			if snippet == "" {
				snippet = fmt.Sprintf("[node %s]", nextNode.ID)
			}
			if snippet != "" && s.ContextService != nil {
				meta := map[string]interface{}{"conversation_type": "story_original", "source": "advance_story"}
				if err := s.ContextService.AddConversation(sceneID, "story", snippet, meta, nextNode.ID); err != nil {
					utils.GetLogger().Warn("append original node to context failed", map[string]interface{}{
						"scene_id": sceneID,
						"node_id":  nextNode.ID,
						"err":      err.Error(),
					})
				}
			}
		}

		// 构建 original_story：串联 story.json 中各节点内容（Content 优先，fallback 为 OriginalContent）
		var origBuilder strings.Builder
		for i := range storyData.Nodes {
			node := storyData.Nodes[i]
			part := strings.TrimSpace(node.Content) // 只取 content 作为 original_story
			if part == "" {
				continue
			}
			origBuilder.WriteString(fmt.Sprintf("[NODE %s]\n%s\n\n", node.ID, part))
		}
		originalStoryText := strings.TrimSpace(origBuilder.String())

		// 构建 user_process：从 context.json 收集玩家指令与 console_story（排除 speaker=="story" 的原文条目）
		var procBuilder strings.Builder
		if sceneData != nil {
			for _, conv := range sceneData.Context.Conversations {
				if strings.ToLower(strings.TrimSpace(conv.SpeakerID)) == "story" {
					continue
				}
				if isStoryConsoleConversation(conv) || isStoryConsoleUserConversation(conv) {
					content := resolveConversationContent(conv)
					if content != "" {
						procBuilder.WriteString(fmt.Sprintf("[%s %s]\n%s\n\n", conv.SpeakerID, resolveConversationNodeID(conv), content))
					}
				}
			}
		}
		userProcessText := strings.TrimSpace(procBuilder.String())

		// 组织提示词并调用 LLM 续写
		var systemPrompt, userPrompt string
		isEnglish := isEnglishText(sceneData.Scene.Name + " " + storyData.Intro + " " + storyData.MainObjective)
		if isEnglish {
			systemPrompt = "You are an assistant who continues stories using the original passages and recent user/process logs. Keep continuity and follow user intent."
			userPrompt = fmt.Sprintf("Original_story:\n%s\n\nUser_process:\n%s\n\nTask: Using the Original_story as canonical material and the User_process as recent directives/processing, write the next short narrative paragraph that moves the plot forward and respects player intent. Output narrative only, at most 300 words.", originalStoryText, userProcessText)
		} else {
			systemPrompt = "你是一个剧情续写助手，需要使用 original_story 与 user_process 按顺序续写，保持连贯并遵循玩家意图。"
			userPrompt = fmt.Sprintf("original_story:\n%s\n\nuser_process:\n%s\n\n任务：以 original_story 为正典，结合 user_process 中的玩家指令与旁白加工，续写下一段剧情，推动事件发展但保持人物设定一致。仅输出正文，最多400字。", originalStoryText, userProcessText)
		}

		if s.LLMService == nil || !s.LLMService.IsReady() {
			return fmt.Errorf("LLM服务未就绪，无法续写剧情")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		resp, err := s.LLMService.CreateChatCompletion(ctx, ChatCompletionRequest{
			Model:    s.getLLMModel(preferences),
			Messages: []ChatCompletionMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userPrompt}},
		})
		if err != nil {
			return fmt.Errorf("LLM 续写失败: %w", err)
		}
		llmResp := strings.TrimSpace(resp.Choices[0].Message.Content)

		// 将 LLM 返回追加到 context.json，speaker 使用 console_story（绿色标签）
		if s.ContextService != nil && llmResp != "" {
			meta := map[string]interface{}{"conversation_type": "story_console", "channel": "ai", "mode": "console_story", "source": "advance_story"}
			if err := s.ContextService.AddConversation(sceneID, "console_story", llmResp, meta, nextNodeID); err != nil {
				utils.GetLogger().Warn("append llm response to context failed", map[string]interface{}{
					"scene_id": sceneID,
					"node_id":  nextNodeID,
					"err":      err.Error(),
				})
			}
		}

		// 返回 StoryUpdate，story.json 保持不变
		storyUpdate = &models.StoryUpdate{
			ID:      fmt.Sprintf("update_%s_%d", sceneID, time.Now().UnixNano()),
			SceneID: sceneID,
			Title:   nextNodeID,
			Content: llmResp,
			Type: func() string {
				if nextNode != nil {
					return nextNode.Type
				}
				return "continuation"
			}(),
			CreatedAt: time.Now(),
			Source:    models.SourceGenerated,
		}

		return nil
	})

	return storyUpdate, err
}

// CreateStoryBranch 创建故事分支
func (s *StoryService) CreateStoryBranch(sceneID string, triggerType string, triggerID string, preferences *models.UserPreferences) (*models.StoryNode, error) {
	var storyNode *models.StoryNode

	err := s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		// 直接读取文件
		storyPath := filepath.Join(s.BasePath, sceneID, "story.json")
		if _, err := os.Stat(storyPath); os.IsNotExist(err) {
			return fmt.Errorf("故事数据不存在")
		}

		storyDataBytes, err := os.ReadFile(storyPath)
		if err != nil {
			return fmt.Errorf("读取故事数据失败: %w", err)
		}

		var storyData models.StoryData
		if err := json.Unmarshal(storyDataBytes, &storyData); err != nil {
			return fmt.Errorf("解析故事数据失败: %w", err)
		}

		// 加载场景数据
		sceneData, err := s.SceneService.LoadScene(sceneID)
		if err != nil {
			return err
		}

		// 检测语言
		isEnglish := isEnglishText(sceneData.Scene.Name + " " + storyData.Intro)

		// 准备分支创建的提示
		creativityStr := string(preferences.CreativityLevel)
		allowPlotTwists := preferences.AllowPlotTwists

		var triggerDescription string

		// 根据触发类型获取相应的描述
		switch triggerType {
		case "item":
			// 查找物品描述 - 这里应该调用ItemService
			if isEnglish {
				triggerDescription = "(Item triggered)"
			} else {
				triggerDescription = "（物品触发）"
			}
		case "location":
			// 查找地点描述
			for _, loc := range storyData.Locations {
				if loc.ID == triggerID {
					if isEnglish {
						triggerDescription = fmt.Sprintf("Location: %s - %s", loc.Name, loc.Description)
					} else {
						triggerDescription = fmt.Sprintf("地点：%s - %s", loc.Name, loc.Description)
					}
					break
				}
			}
		case "task":
			// 查找任务描述
			for _, task := range storyData.Tasks {
				if task.ID == triggerID {
					if isEnglish {
						triggerDescription = fmt.Sprintf("Task: %s - %s", task.Title, task.Description)
					} else {
						triggerDescription = fmt.Sprintf("任务：%s - %s", task.Title, task.Description)
					}
					break
				}
			}
		case "character":
			// 查找角色描述
			for _, char := range sceneData.Characters {
				if char.ID == triggerID {
					if isEnglish {
						triggerDescription = fmt.Sprintf("Character: %s - %s", char.Name, char.Personality)
					} else {
						triggerDescription = fmt.Sprintf("角色：%s - %s", char.Name, char.Personality)
					}
					break
				}
			}
		default:
			if isEnglish {
				triggerDescription = "Unknown trigger"
			} else {
				triggerDescription = "未知触发器"
			}
		}

		var prompt string
		var systemPrompt string

		if isEnglish {
			// 英文提示词
			prompt = fmt.Sprintf(`In the world of "%s", the player has encountered the following situation:

%s

Current story state: %s
Story progress: %d%%

Based on this trigger, create a new branch story node that provides multiple choices for the player.
This branch should coordinate with the main storyline, but add depth to the story or provide additional content.

Creativity level: %s
Allow plot twists: %v

Return in JSON format:
{
  "content": "Detailed story node description",
  "type": "branch/side/optional",
  "choices": [
    {
      "text": "Choice text",
      "consequence": "Description of choice consequence",
      "next_node_hint": "Subsequent development hint"
    }
  ]
}`,
				sceneData.Scene.Name,
				triggerDescription,
				storyData.CurrentState,
				storyData.Progress,
				creativityStr,
				allowPlotTwists)

			systemPrompt = "You are a creative story designer responsible for creating engaging interactive stories."
		} else {
			// 中文提示词
			prompt = fmt.Sprintf(`在《%s》的世界中，玩家遇到了以下情况:

%s

当前故事状态: %s
故事进展: %d%%

根据这个触发因素，创建一个新的分支故事节点，提供玩家多个选择。
这个分支应该与主线故事协调，但能增加故事的深度或提供额外的内容。

创造性级别: %s
允许剧情转折: %v

返回JSON格式:
{
  "content": "详细的故事节点描述",
  "type": "branch/side/optional",
  "choices": [
    {
      "text": "选择文本",
      "consequence": "选择后果描述",
      "next_node_hint": "后续发展提示"
    }
  ]
}`,
				sceneData.Scene.Name,
				triggerDescription,
				storyData.CurrentState,
				storyData.Progress,
				creativityStr,
				allowPlotTwists)

			systemPrompt = "你是一个创意故事设计师，负责创建引人入胜的交互式故事。"
		}

		// Create a context with timeout for the LLM call
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		resp, err := s.LLMService.CreateChatCompletion(
			ctx,
			ChatCompletionRequest{
				Model: s.getLLMModel(preferences),
				Messages: []ChatCompletionMessage{
					{
						Role:    "system",
						Content: systemPrompt,
					},
					{
						Role:    "user",
						Content: prompt,
					},
				},
				// 请求JSON格式输出
				ExtraParams: map[string]interface{}{
					"response_format": map[string]string{
						"type": "json_object",
					},
				},
			},
		)

		if err != nil {
			if isEnglish {
				return fmt.Errorf("failed to generate story branch: %w", err)
			} else {
				return fmt.Errorf("生成故事分支失败: %w", err)
			}
		}

		jsonStr := SanitizeLLMJSONResponse(resp.Choices[0].Message.Content)

		// 解析返回的JSON
		var branchData struct {
			Content string `json:"content"`
			Type    string `json:"type"`
			Choices []struct {
				Text         string `json:"text"`
				Consequence  string `json:"consequence"`
				NextNodeHint string `json:"next_node_hint"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &branchData); err != nil {
			if isEnglish {
				return fmt.Errorf("failed to parse story branch data: %w", err)
			} else {
				return fmt.Errorf("解析故事分支数据失败: %w", err)
			}
		}

		// 创建新的故事节点
		nodeID := fmt.Sprintf("branch_%s_%d", sceneID, time.Now().UnixNano())
		newStoryNode := &models.StoryNode{
			ID:         nodeID,
			SceneID:    sceneID,
			Content:    branchData.Content,
			Type:       branchData.Type,
			IsRevealed: true,
			CreatedAt:  time.Now(),
			Source:     models.SourceBranch,
			Choices:    []models.StoryChoice{},
			Metadata: map[string]interface{}{
				"trigger_type": triggerType,
				"trigger_id":   triggerID,
			},
		}

		// 添加选择
		for i, choice := range branchData.Choices {
			newStoryNode.Choices = append(newStoryNode.Choices, models.StoryChoice{
				ID:           fmt.Sprintf("choice_%s_%d", nodeID, i+1),
				Text:         choice.Text,
				Consequence:  choice.Consequence,
				NextNodeHint: choice.NextNodeHint,
				Selected:     false,
				CreatedAt:    time.Now(),
			})
		}

		// 将节点添加到故事数据
		storyData.Nodes = append(storyData.Nodes, *newStoryNode)

		// 保存更新后的故事数据
		if err := s.saveStoryData(sceneID, &storyData); err != nil {
			return err
		}

		// 设置返回结果
		storyNode = newStoryNode

		return nil
	})

	if err != nil {
		return nil, err
	}

	return storyNode, nil
}

// EvaluateStoryProgress 评估故事进展状态
// 🔧 修复后的版本
func (s *StoryService) EvaluateStoryProgress(sceneID string) (*models.StoryProgressStatus, error) {
	var status *models.StoryProgressStatus

	err := s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		// 在锁内直接读取文件
		storyPath := filepath.Join(s.BasePath, sceneID, "story.json")
		if _, err := os.Stat(storyPath); os.IsNotExist(err) {
			return fmt.Errorf("故事数据不存在")
		}

		storyDataBytes, err := os.ReadFile(storyPath)
		if err != nil {
			return fmt.Errorf("读取故事数据失败: %w", err)
		}

		var storyData models.StoryData
		if err := json.Unmarshal(storyDataBytes, &storyData); err != nil {
			return fmt.Errorf("解析故事数据失败: %w", err)
		}

		// 计算任务完成情况
		totalTasks := len(storyData.Tasks)
		completedTasks := 0
		for _, task := range storyData.Tasks {
			if task.Completed {
				completedTasks++
			}
		}

		// 计算地点探索情况
		totalLocations := len(storyData.Locations)
		accessibleLocations := 0
		for _, loc := range storyData.Locations {
			if loc.Accessible {
				accessibleLocations++
			}
		}

		// 计算故事节点情况
		totalNodes := len(storyData.Nodes)

		// 创建进展状态
		status = &models.StoryProgressStatus{
			SceneID:             sceneID,
			Progress:            storyData.Progress,
			CurrentState:        storyData.CurrentState,
			CompletedTasks:      completedTasks,
			TotalTasks:          totalTasks,
			TaskCompletionRate:  calculateSafeRate(completedTasks, totalTasks),
			AccessibleLocations: accessibleLocations,
			TotalLocations:      totalLocations,
			LocationAccessRate:  calculateSafeRate(accessibleLocations, totalLocations),
			TotalStoryNodes:     totalNodes,
			EstimatedCompletion: s.calculateEstimatedCompletion(&storyData),
			IsMainObjectiveMet:  storyData.Progress >= 100,
		}

		return nil
	})

	return status, err
}

// 🔧 辅助函数：安全计算比率
func calculateSafeRate(completed, total int) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(completed) / float64(total) * 100
}

// 🔧 提取估算计算逻辑
func (s *StoryService) calculateEstimatedCompletion(storyData *models.StoryData) time.Duration {
	if storyData == nil {
		return 0
	}

	// 如果故事已完成，返回0
	if storyData.Progress >= 100 {
		return 0
	}

	// 检测语言（用于不同的估算逻辑）
	isEnglish := isEnglishText(storyData.Intro + " " + storyData.MainObjective)

	// 基础估算参数
	var baseTimePerNode time.Duration
	var taskComplexityFactor float64
	var progressFactor float64

	if isEnglish {
		// 英文故事的估算参数
		baseTimePerNode = 3 * time.Minute // 每个节点平均3分钟
		taskComplexityFactor = 1.2        // 任务复杂度系数
		progressFactor = 1.1              // 进度影响系数
	} else {
		// 中文故事的估算参数
		baseTimePerNode = 4 * time.Minute // 每个节点平均4分钟（考虑阅读速度差异）
		taskComplexityFactor = 1.3        // 任务复杂度系数
		progressFactor = 1.15             // 进度影响系数
	}

	// 计算剩余节点数（估算）
	revealedNodes := 0
	for _, node := range storyData.Nodes {
		if node.IsRevealed {
			revealedNodes++
		}
	}

	// 估算总节点数（基于当前进度）
	var estimatedTotalNodes int
	if storyData.Progress > 0 && revealedNodes > 0 {
		// 基于当前进度估算总节点数
		estimatedTotalNodes = int(float64(revealedNodes) * 100.0 / float64(storyData.Progress))
	} else {
		// 默认估算（基于故事复杂度）
		estimatedTotalNodes = 15 // 默认15个节点

		// 根据任务数量调整估算
		taskCount := len(storyData.Tasks)
		if taskCount > 5 {
			estimatedTotalNodes += (taskCount - 5) * 2 // 每增加一个任务增加2个节点
		}

		// 根据地点数量调整估算
		locationCount := len(storyData.Locations)
		if locationCount > 3 {
			estimatedTotalNodes += (locationCount - 3) // 每增加一个地点增加1个节点
		}
	}

	// 计算剩余节点数
	remainingNodes := max(estimatedTotalNodes-revealedNodes, 0)

	// 计算未完成任务的影响
	uncompletedTasks := 0
	totalObjectives := 0
	completedObjectives := 0

	for _, task := range storyData.Tasks {
		if !task.Completed {
			uncompletedTasks++
		}

		totalObjectives += len(task.Objectives)
		for _, objective := range task.Objectives {
			if objective.Completed {
				completedObjectives++
			}
		}
	}

	// 任务复杂度影响时间估算
	taskTimeMultiplier := 1.0
	if uncompletedTasks > 0 {
		taskTimeMultiplier = taskComplexityFactor

		// 根据目标完成率调整
		if totalObjectives > 0 {
			objectiveCompletionRate := float64(completedObjectives) / float64(totalObjectives)
			if objectiveCompletionRate < 0.5 {
				taskTimeMultiplier *= 1.2 // 目标完成率低，增加时间
			}
		}
	}

	// 根据当前故事状态调整时间估算
	var stateMultiplier float64
	switch storyData.CurrentState {
	case "初始", "Initial":
		stateMultiplier = 1.3 // 初始阶段通常节奏较慢
	case "冲突", "Conflict":
		stateMultiplier = 1.1 // 冲突阶段节奏适中
	case "发展", "Development":
		stateMultiplier = 1.0 // 发展阶段标准节奏
	case "高潮", "Climax":
		stateMultiplier = 0.8 // 高潮阶段节奏较快
	case "结局", "Ending":
		stateMultiplier = 0.6 // 结局阶段很快
	default:
		stateMultiplier = 1.0
	}

	// 根据剩余进度调整
	remainingProgress := 100 - storyData.Progress
	progressMultiplier := progressFactor * float64(remainingProgress) / 100.0

	// 计算基础估算时间
	baseEstimatedTime := time.Duration(remainingNodes) * baseTimePerNode

	// 应用所有系数
	finalEstimatedTime := time.Duration(
		float64(baseEstimatedTime) *
			taskTimeMultiplier *
			stateMultiplier *
			progressMultiplier,
	)

	// 添加一些随机性和不确定性
	uncertaintyFactor := 1.2 // 20%的不确定性
	finalEstimatedTime = time.Duration(float64(finalEstimatedTime) * uncertaintyFactor)

	// 设置最小和最大时间限制
	minTime := 5 * time.Minute
	maxTime := 4 * time.Hour

	if finalEstimatedTime < minTime {
		finalEstimatedTime = minTime
	}
	if finalEstimatedTime > maxTime {
		finalEstimatedTime = maxTime
	}

	// 考虑用户的平均游戏速度（如果有历史数据）
	// 这里可以根据实际需求添加个性化调整

	return finalEstimatedTime
}

// 辅助函数：格式化位置信息
func formatLocations(locations []models.Location) string {
	var locationNames []string
	for _, loc := range locations {
		locationNames = append(locationNames, loc.Name)
	}
	return strings.Join(locationNames, ", ")
}

// 辅助函数：格式化主题信息
func formatThemes(themes []string) string {
	return strings.Join(themes, ", ")
}

// 辅助函数：格式化角色信息
func formatCharacters(characters []*models.Character) string {
	var result strings.Builder
	for _, char := range characters {
		result.WriteString(fmt.Sprintf("- %s: %s\n", char.Name, char.Personality))
	}
	return result.String()
}

// RewindToNode 回溯故事到指定节点
func (s *StoryService) RewindToNode(sceneID, nodeID string) (*models.StoryData, error) {
	var storyData *models.StoryData

	err := s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		// 直接读取文件
		storyPath := filepath.Join(s.BasePath, sceneID, "story.json")
		if _, err := os.Stat(storyPath); os.IsNotExist(err) {
			return fmt.Errorf("故事数据不存在")
		}

		storyDataBytes, err := os.ReadFile(storyPath)
		if err != nil {
			return fmt.Errorf("读取故事数据失败: %w", err)
		}

		var tempStoryData models.StoryData
		if err := json.Unmarshal(storyDataBytes, &tempStoryData); err != nil {
			return fmt.Errorf("解析故事数据失败: %w", err)
		}

		// 查找目标节点索引
		var targetNode *models.StoryNode
		targetIndex := -1
		for i := range tempStoryData.Nodes {
			if tempStoryData.Nodes[i].ID == nodeID {
				targetNode = &tempStoryData.Nodes[i]
				targetIndex = i
				break
			}
		}

		if targetNode == nil || targetIndex < 0 {
			return fmt.Errorf("节点不存在或不可回溯")
		}

		// 按节点顺序回溯：目标之后的节点标记为未揭示并重置选择，之前的保持揭示
		for i := range tempStoryData.Nodes {
			node := &tempStoryData.Nodes[i]
			if i > targetIndex {
				node.IsRevealed = false
				for j := range node.Choices {
					node.Choices[j].Selected = false
				}
			} else {
				node.IsRevealed = true
			}
		}

		// 重置目标节点的选择状态
		for i := range targetNode.Choices {
			targetNode.Choices[i].Selected = false
		}

		// 重新计算故事进度
		newProgress := calculateProgress(&tempStoryData, targetNode)
		if newProgress >= 0 {
			tempStoryData.Progress = newProgress
		}

		// 更新当前状态
		if tempStoryData.Progress >= 100 {
			tempStoryData.CurrentState = "结局"
		} else if tempStoryData.Progress >= 75 {
			tempStoryData.CurrentState = "高潮"
		} else if tempStoryData.Progress >= 50 {
			tempStoryData.CurrentState = "发展"
		} else if tempStoryData.Progress >= 25 {
			tempStoryData.CurrentState = "冲突"
		} else {
			tempStoryData.CurrentState = "初始"
		}

		// 更新最后修改时间
		tempStoryData.LastUpdated = time.Now()

		// 保存更新后的故事数据
		if err := s.saveStoryData(sceneID, &tempStoryData); err != nil {
			return err
		}

		storyData = &tempStoryData
		return nil
	})

	if err != nil {
		return nil, err
	}

	return storyData, nil
}

// 计算基于指定节点的故事进度
func calculateProgress(storyData *models.StoryData, referenceNode *models.StoryNode) int {
	_ = referenceNode
	// 计算节点总数和已揭示节点数（按当前揭示状态，无需依赖时间戳）
	totalNodes := len(storyData.Nodes)
	revealedNodes := 0

	for _, node := range storyData.Nodes {
		if node.IsRevealed {
			revealedNodes++
		}
	}

	// 如果没有节点，返回0进度
	if totalNodes == 0 {
		return 0
	}

	// 基于已揭示节点百分比计算进度
	progress := (revealedNodes * 100) / totalNodes

	// 考虑完成的任务
	completedTasks := 0
	totalTasks := len(storyData.Tasks)
	for _, task := range storyData.Tasks {
		if task.IsRevealed && task.Completed {
			completedTasks++
		}
	}

	// 任务进度
	taskProgress := 0
	if totalTasks > 0 {
		taskProgress = (completedTasks * 100) / totalTasks
	}

	// 综合节点进度和任务进度
	return (progress*3 + taskProgress) / 4
}

// ProcessCharacterInteractionTriggers 方法，处理故事节点中的角色互动触发器
func (s *StoryService) ProcessCharacterInteractionTriggers(sceneID string, nodeID string, preferences *models.UserPreferences) ([]*models.CharacterInteraction, error) {
	var generatedInteractions []*models.CharacterInteraction

	err := s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		// 在锁内读取所需的数据
		storyPath := filepath.Join(s.BasePath, sceneID, "story.json")
		if _, err := os.Stat(storyPath); os.IsNotExist(err) {
			return fmt.Errorf("故事数据不存在")
		}

		storyDataBytes, err := os.ReadFile(storyPath)
		if err != nil {
			return fmt.Errorf("读取故事数据失败: %w", err)
		}

		var storyData models.StoryData
		if err := json.Unmarshal(storyDataBytes, &storyData); err != nil {
			return fmt.Errorf("解析故事数据失败: %w", err)
		}

		// 加载场景数据
		sceneData, err := s.SceneService.LoadScene(sceneID)
		if err != nil {
			return err
		}

		// 🔧 调用修改后的方法，传递数据
		interactions, err := s.processCharacterInteractionTriggersUnsafe(
			sceneID, nodeID, preferences, &storyData, sceneData)
		if err != nil {
			return err
		}

		// 如果有互动被触发，保存数据
		if len(interactions) > 0 {
			if err := s.saveStoryData(sceneID, &storyData); err != nil {
				return err
			}
			// 清除缓存以确保一致性
			s.invalidateStoryCache(sceneID)
		}
		generatedInteractions = interactions
		return nil
	})

	return generatedInteractions, err
}

// processCharacterInteractionTriggersUnsafe 内部方法，处理故事节点中的角色互动触发器（不加锁）
func (s *StoryService) processCharacterInteractionTriggersUnsafe(sceneID string, nodeID string, preferences *models.UserPreferences, storyData *models.StoryData, sceneData *SceneData) ([]*models.CharacterInteraction, error) {
	// 查找节点
	var node *models.StoryNode
	for i := range storyData.Nodes {
		if storyData.Nodes[i].ID == nodeID {
			node = &storyData.Nodes[i]
			break
		}
	}

	if node == nil {
		return nil, fmt.Errorf("节点不存在: %s", nodeID)
	}

	// 检测语言
	isEnglish := isEnglishText(sceneData.Scene.Name)

	// 获取节点的交互触发器
	if node.Metadata == nil || node.Metadata["interaction_triggers"] == nil {
		return nil, nil // 没有触发器，直接返回
	}

	// 获取交互触发器列表
	var triggers []models.InteractionTrigger
	triggersData, err := json.Marshal(node.Metadata["interaction_triggers"])
	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to process interaction triggers: %w", err)
		} else {
			return nil, fmt.Errorf("处理互动触发器失败: %w", err)
		}
	}

	if err := json.Unmarshal(triggersData, &triggers); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to parse interaction triggers: %w", err)
		} else {
			return nil, fmt.Errorf("解析互动触发器失败: %w", err)
		}
	}

	// 🔧 使用缓存的角色服务
	if s.CharacterService == nil {
		if isEnglish {
			return nil, fmt.Errorf("character service not available")
		} else {
			return nil, fmt.Errorf("角色服务不可用")
		}
	}

	// 处理每个触发器
	var generatedInteractions []*models.CharacterInteraction
	for i := range triggers {
		// 跳过已触发的
		if triggers[i].Triggered {
			continue
		}

		// 🔧 检查触发条件是否满足
		shouldTrigger := s.evaluateTriggerCondition(triggers[i].Condition, storyData, preferences)
		if !shouldTrigger {
			continue
		}

		// 生成角色互动
		interaction, err := s.CharacterService.GenerateCharacterInteraction(
			sceneID,
			triggers[i].CharacterIDs,
			triggers[i].Topic,
			triggers[i].ContextDescription,
		)

		if err != nil {
			utils.GetLogger().Warn("failed to trigger character interaction", map[string]interface{}{
				"scene_id": sceneID,
				"node_id":  nodeID,
				"err":      err.Error(),
			})
			continue
		}

		// 标记为已触发
		triggers[i].Triggered = true
		generatedInteractions = append(generatedInteractions, interaction)
	}

	// 更新触发器状态
	node.Metadata["interaction_triggers"] = triggers

	return generatedInteractions, nil
}

// SaveStoryData 保存故事数据到文件（公开方法）
func (s *StoryService) SaveStoryData(sceneID string, storyData *models.StoryData) error {
	if s == nil {
		return fmt.Errorf("故事服务未初始化")
	}

	// 调用内部的保存方法
	return s.saveStoryData(sceneID, storyData)
}

// DeleteStoryData 删除指定场景的故事数据目录
func (s *StoryService) DeleteStoryData(sceneID string) error {
	if s == nil {
		return fmt.Errorf("故事服务未初始化")
	}

	if sceneID == "" {
		return fmt.Errorf("场景ID不能为空")
	}

	return s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		storyDir := filepath.Join(s.BasePath, sceneID)

		// 优先使用文件存储接口，确保缓存一致
		if s.FileStorage != nil {
			if !s.FileStorage.DirExists(sceneID) {
				s.invalidateStoryCache(sceneID)
				return nil
			}

			if err := s.FileStorage.DeleteDir(sceneID); err != nil {
				return fmt.Errorf("删除故事数据失败: %w", err)
			}
		} else {
			if _, err := os.Stat(storyDir); os.IsNotExist(err) {
				s.invalidateStoryCache(sceneID)
				return nil
			}

			if err := os.RemoveAll(storyDir); err != nil {
				return fmt.Errorf("删除故事数据失败: %w", err)
			}
		}

		s.invalidateStoryCache(sceneID)
		return nil
	})
}

// ExecuteBatchOperation 批量执行故事操作
func (s *StoryService) ExecuteBatchOperation(sceneID string, operation func(*models.StoryData) error) error {
	return s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
		// 直接读取文件，避免死锁
		storyPath := filepath.Join(s.BasePath, sceneID, "story.json")
		if _, err := os.Stat(storyPath); os.IsNotExist(err) {
			return fmt.Errorf("故事数据不存在")
		}

		storyDataBytes, err := os.ReadFile(storyPath)
		if err != nil {
			return fmt.Errorf("读取故事数据失败: %w", err)
		}

		var storyData models.StoryData
		if err := json.Unmarshal(storyDataBytes, &storyData); err != nil {
			return fmt.Errorf("解析故事数据失败: %w", err)
		}

		// 执行批量操作
		if err := operation(&storyData); err != nil {
			return err
		}

		// 保存更新后的数据
		return s.saveStoryData(sceneID, &storyData)
	})
}

// 基础评估方法
func (s *StoryService) evaluateTriggerCondition(condition string, storyData *models.StoryData, preferences *models.UserPreferences) bool {
	if condition == "" {
		return s.evaluateDefaultTriggerCondition(storyData, preferences)
	}

	condition = strings.TrimSpace(strings.ToLower(condition))

	// 🔧 只保留最基本的几种条件
	switch condition {
	case "always", "总是":
		return true
	case "never", "从不":
		return false
	case "random", "随机":
		return rand.Float64() < 0.5 // 50% 概率
	default:
		// 简单的进度检查
		if strings.Contains(condition, "progress") || strings.Contains(condition, "进度") {
			return storyData.Progress > 30
		}
		return s.evaluateDefaultTriggerCondition(storyData, preferences)
	}
}

// 默认触发条件评估（原逻辑）
func (s *StoryService) evaluateDefaultTriggerCondition(storyData *models.StoryData, preferences *models.UserPreferences) bool {
	// 基于用户偏好的触发概率
	if preferences != nil {
		switch preferences.CreativityLevel {
		case models.CreativityExpansive:
			// 高创造性模式下，更容易触发互动
			return true
		case models.CreativityBalanced:
			// 平衡模式下，有条件触发
			return storyData.Progress >= 25 // 进度超过25%时触发
		case models.CreativityStrict:
			// 严格模式下，较少触发
			return storyData.Progress >= 50 // 进度超过50%时触发
		}
	}

	// 默认触发条件
	return storyData.Progress >= 30
}
