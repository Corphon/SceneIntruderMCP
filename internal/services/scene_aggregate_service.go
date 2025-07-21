// internal/services/scene_aggregate_service.go
package services

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

// AggregateOptions 聚合选项
type AggregateOptions struct {
	IncludeConversations bool
	ConversationLimit    int
	IncludeStoryData     bool
	IncludeUIState       bool
	IncludeProgress      bool
	UserPreferences      *models.UserPreferences
}

// AggregateStats 聚合统计信息
type AggregateStats struct {
	TotalCharacters    int     `json:"total_characters"`
	TotalConversations int     `json:"total_conversations"`
	StoryProgress      float64 `json:"story_progress"`
	ActiveNodes        int     `json:"active_nodes"`
	AvailableChoices   int     `json:"available_choices"`
}

// UINotification UI通知
type UINotification struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // info, warning, error, success
	Message   string    `json:"message"`
	Actions   []string  `json:"actions"`
	Timestamp time.Time `json:"timestamp"`
}

// SceneAggregateService 聚合场景服务
type SceneAggregateService struct {
	SceneService     *SceneService
	CharacterService *CharacterService
	ContextService   *ContextService
	StoryService     *StoryService
	ProgressService  *ProgressService

	// 缓存和并发控制
	cacheMutex     sync.RWMutex
	aggregateCache map[string]*CachedAggregateData
	cacheExpiry    time.Duration
}

// CachedAggregateData 缓存的聚合数据
type CachedAggregateData struct {
	Data      *SceneAggregateData
	Timestamp time.Time
}

// SceneAggregateData 聚合的场景数据
type SceneAggregateData struct {
	// 基础信息
	Scene      *models.Scene       `json:"scene"`
	Characters []*models.Character `json:"characters"`

	// 故事相关
	Story            *models.StoryData     `json:"story"`
	CurrentNode      *models.StoryNode     `json:"current_node"`
	AvailableChoices []*models.StoryChoice `json:"available_choices"`

	// 交互历史
	RecentConversations []models.Conversation `json:"recent_conversations"`

	// UI状态
	UIState *SceneUIState `json:"ui_state"`

	// 进度和统计
	Progress *SceneProgress `json:"progress"`

	// 元数据
	LastUpdated time.Time `json:"last_updated"`
	Version     string    `json:"version"`
}

// SceneUIState UI状态信息
type SceneUIState struct {
	SelectedCharacterIds []string         `json:"selected_character_ids"`
	CurrentTab           string           `json:"current_tab"`
	StoryTreeExpanded    bool             `json:"story_tree_expanded"`
	ChatViewMode         string           `json:"chat_view_mode"`
	Notifications        []UINotification `json:"notifications"`
}

// SceneProgress 场景进度信息
type SceneProgress struct {
	StoryCompletion       float64  `json:"story_completion"`
	CharacterInteractions int      `json:"character_interactions"`
	UnlockedContent       []string `json:"unlocked_content"`
	Achievements          []string `json:"achievements"`
}

// ------------------------------------------------------------
// NewSceneAggregateService 创建场景聚合服务
func NewSceneAggregateService(
	sceneService *SceneService,
	characterService *CharacterService,
	contextService *ContextService,
	storyService *StoryService,
	progressService *ProgressService) *SceneAggregateService {

	// 添加关键依赖检查
	if sceneService == nil {
		panic("SceneService cannot be nil")
	}
	if characterService == nil {
		panic("CharacterService cannot be nil")
	}
	if contextService == nil {
		panic("ContextService cannot be nil")
	}
	if storyService == nil {
		panic("StoryService cannot be nil")
	}
	if progressService == nil {
		panic("ProgressService cannot be nil")
	}

	service := &SceneAggregateService{
		SceneService:     sceneService,
		CharacterService: characterService,
		ContextService:   contextService,
		StoryService:     storyService,
		ProgressService:  progressService,

		// 初始化缓存
		aggregateCache: make(map[string]*CachedAggregateData),
		cacheExpiry:    2 * time.Minute, // 聚合数据缓存时间较短
	}

	// 启动缓存清理
	service.startCacheCleanup()

	return service
}

// GetSceneAggregate 获取完整的场景聚合数据
func (s *SceneAggregateService) GetSceneAggregate(ctx context.Context, sceneID string, options *AggregateOptions) (*SceneAggregateData, error) {
	// 生成缓存键
	cacheKey := s.generateCacheKey(sceneID, options)

	// 检查缓存
	s.cacheMutex.RLock()
	if cached, exists := s.aggregateCache[cacheKey]; exists {
		if time.Since(cached.Timestamp) < s.cacheExpiry {
			s.cacheMutex.RUnlock()
			return cached.Data, nil
		}
	}
	s.cacheMutex.RUnlock()

	// 缓存过期或不存在，重新生成
	aggregateData, err := s.generateAggregateData(sceneID, options)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	s.cacheMutex.Lock()
	s.aggregateCache[cacheKey] = &CachedAggregateData{
		Data:      aggregateData,
		Timestamp: time.Now(),
	}
	s.cacheMutex.Unlock()

	return aggregateData, nil
}

// 生成缓存键
func (s *SceneAggregateService) generateCacheKey(sceneID string, options *AggregateOptions) string {
	if options == nil {
		return sceneID + "_default"
	}

	return fmt.Sprintf("%s_%t_%d_%t_%t_%t",
		sceneID,
		options.IncludeConversations,
		options.ConversationLimit,
		options.IncludeStoryData,
		options.IncludeUIState,
		options.IncludeProgress,
	)
}

// 实际的数据生成逻辑
func (s *SceneAggregateService) generateAggregateData(sceneID string, options *AggregateOptions) (*SceneAggregateData, error) {
	// 输入验证
	if sceneID == "" || strings.TrimSpace(sceneID) == "" {
		return nil, fmt.Errorf("场景ID不能为空")
	}

	if options == nil {
		options = &AggregateOptions{
			IncludeConversations: true,
			ConversationLimit:    20,
			IncludeStoryData:     true,
			IncludeUIState:       true,
			IncludeProgress:      true,
		}
	}

	// 验证选项
	if err := s.validateAggregateOptions(options); err != nil {
		return nil, fmt.Errorf("选项验证失败: %w", err)
	}

	// 并行数据获取
	var (
		scene         *models.Scene
		characters    []*models.Character
		storyData     *models.StoryData
		conversations []models.Conversation

		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	// 并行获取基础数据
	wg.Add(1)

	// 获取场景和角色信息（错误处理完善）
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("获取场景数据时发生panic: %v", r))
				mu.Unlock()
			}
		}()

		sceneData, err := s.SceneService.LoadScene(sceneID)
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("加载场景失败: %w", err))
			mu.Unlock()
			return
		}
		if sceneData == nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("场景数据为空"))
			mu.Unlock()
			return
		}

		scene = &sceneData.Scene
		characters = sceneData.Characters
	}()

	// 获取故事数据（条件执行，错误处理完善）
	if options.IncludeStoryData && s.StoryService != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("获取故事数据时发生panic: %v", r))
					mu.Unlock()
				}
			}()

			story, err := s.StoryService.GetStoryData(sceneID, nil)
			if err != nil {
				// 故事数据获取失败记录但不阻断
				fmt.Printf("警告: 获取故事数据失败: %v\n", err)
				return
			}
			storyData = story
		}()
	}

	wg.Wait()

	// 检查致命错误
	if len(errs) > 0 {
		var errMsg strings.Builder
		errMsg.WriteString("获取聚合数据失败: ")
		for i, err := range errs {
			if i > 0 {
				errMsg.WriteString("; ")
			}
			errMsg.WriteString(err.Error())
		}
		return nil, fmt.Errorf("%s", errMsg.String())
	}

	// 串行获取对话历史（减少并发复杂性）
	if options.IncludeConversations && s.ContextService != nil {
		convs, err := s.ContextService.GetRecentConversations(sceneID, options.ConversationLimit)
		if err != nil {
			fmt.Printf("警告: 获取对话历史失败: %v\n", err)
			conversations = []models.Conversation{}
		} else {
			conversations = convs
		}
	}

	// 构建聚合数据
	aggregateData := &SceneAggregateData{
		Scene:               scene,
		Characters:          characters,
		Story:               storyData,
		CurrentNode:         s.findCurrentStoryNode(storyData),
		AvailableChoices:    s.getAvailableChoices(s.findCurrentStoryNode(storyData)),
		RecentConversations: conversations,
		LastUpdated:         time.Now(),
		Version:             "1.0",
	}

	// 根据选项添加额外数据
	if options.IncludeProgress {
		aggregateData.Progress = s.calculateSceneProgress(scene, storyData, conversations)
	}

	if options.IncludeUIState {
		aggregateData.UIState = s.buildUIState(scene, storyData, characters)
	}

	return aggregateData, nil
}

// validateAggregateOptions 验证聚合选项
func (s *SceneAggregateService) validateAggregateOptions(options *AggregateOptions) error {
	if options == nil {
		return fmt.Errorf("选项不能为空")
	}

	if options.ConversationLimit < 0 {
		return fmt.Errorf("对话限制不能为负数")
	}

	if options.ConversationLimit > 1000 {
		return fmt.Errorf("对话限制不能超过1000")
	}

	return nil
}

// findCurrentStoryNode 查找当前故事节点（添加空值检查）
func (s *SceneAggregateService) findCurrentStoryNode(storyData *models.StoryData) *models.StoryNode {
	if storyData == nil || storyData.Nodes == nil || len(storyData.Nodes) == 0 {
		return nil
	}

	// 找到最新的已揭示节点
	var currentNode *models.StoryNode
	var latestTime time.Time

	for i := range storyData.Nodes {
		node := &storyData.Nodes[i]
		if node.IsRevealed && (currentNode == nil || node.CreatedAt.After(latestTime)) {
			currentNode = node
			latestTime = node.CreatedAt
		}
	}

	return currentNode
}

// getAvailableChoices 获取可用选择（添加空值检查）
func (s *SceneAggregateService) getAvailableChoices(node *models.StoryNode) []*models.StoryChoice {
	if node == nil {
		return []*models.StoryChoice{}
	}

	// 获取未选择的选择
	var available []*models.StoryChoice
	for i := range node.Choices {
		choice := &node.Choices[i]
		if !choice.Selected {
			available = append(available, choice)
		}
	}

	return available
}

// calculateSceneProgress 计算场景进度
// 🔧 修复 calculateSceneProgress 方法
func (s *SceneAggregateService) calculateSceneProgress(
	scene *models.Scene,
	story *models.StoryData,
	conversations []models.Conversation) *SceneProgress {

	progress := &SceneProgress{
		StoryCompletion:       0.0,
		CharacterInteractions: len(conversations),
		UnlockedContent:       []string{},
		Achievements:          []string{},
	}

	// 只使用确定存在的字段
	if scene != nil {
		// 1. 基于场景主题计算主题探索进度（确定存在）
		if len(scene.Themes) > 0 {
			exploredThemes := make(map[string]bool)
			for _, conv := range conversations {
				messageContent := strings.ToLower(conv.Content)
				for _, theme := range scene.Themes {
					themeName := strings.ToLower(theme)
					if strings.Contains(messageContent, themeName) {
						exploredThemes[theme] = true
					}
				}
			}

			// 基于主题探索度添加成就
			themeExplorationRate := float64(len(exploredThemes)) / float64(len(scene.Themes))
			if themeExplorationRate >= 0.5 {
				progress.Achievements = append(progress.Achievements, "theme_explorer")
			}
			if themeExplorationRate >= 1.0 {
				progress.Achievements = append(progress.Achievements, "narrative_master")
			}
		}

		// 2. 基于场景时代背景添加历史感知成就（确定存在）
		if scene.Era != "" {
			eraBonus := fmt.Sprintf("深入了解了%s时代", scene.Era)
			progress.UnlockedContent = append(progress.UnlockedContent, eraBonus)

			// 如果对话中多次提及时代背景，添加成就
			eraReferences := 0
			eraKeyword := strings.ToLower(scene.Era)
			for _, conv := range conversations {
				if strings.Contains(strings.ToLower(conv.Content), eraKeyword) {
					eraReferences++
				}
			}
			if eraReferences >= 3 {
				progress.Achievements = append(progress.Achievements, "history_enthusiast")
			}
		}

		// 3. 基于场景创建时间计算资深玩家状态（确定存在）
		sceneAge := time.Since(scene.CreatedAt)
		if sceneAge > 7*24*time.Hour {
			progress.Achievements = append(progress.Achievements, "veteran_player")
		}
	}

	// 计算故事完成度（保留原有逻辑）
	if story != nil && len(story.Nodes) > 0 {
		revealedCount := 0
		for _, node := range story.Nodes {
			if node.IsRevealed {
				revealedCount++
			}
			if node.IsRevealed && node.Type == "unlock" {
				progress.UnlockedContent = append(progress.UnlockedContent, node.Content)
			}
		}
		progress.StoryCompletion = float64(revealedCount) / float64(len(story.Nodes))
	}

	// 检查基础成就
	if progress.StoryCompletion >= 0.5 {
		progress.Achievements = append(progress.Achievements, "story_explorer")
	}
	if progress.CharacterInteractions >= 10 {
		progress.Achievements = append(progress.Achievements, "social_butterfly")
	}

	// 简化完成度计算
	finalCompletion := progress.StoryCompletion

	// 交互活跃度加成
	interactionBonus := math.Min(float64(progress.CharacterInteractions)/50.0, 0.2) // 最多20%加成
	finalCompletion = math.Min(finalCompletion+interactionBonus, 1.0)

	progress.StoryCompletion = finalCompletion

	return progress
}

// buildUIState 构建UI状态
func (s *SceneAggregateService) buildUIState(
	scene *models.Scene,
	story *models.StoryData,
	characters []*models.Character) *SceneUIState {

	uiState := &SceneUIState{
		SelectedCharacterIds: []string{},
		CurrentTab:           "chat",
		StoryTreeExpanded:    false,
		ChatViewMode:         "normal",
		Notifications:        []UINotification{},
	}

	// 选择第一个角色
	if len(characters) > 0 {
		uiState.SelectedCharacterIds = []string{characters[0].ID}
	}

	// 生成欢迎通知
	if scene != nil && story != nil && len(story.Nodes) > 0 {
		welcomeNotification := UINotification{
			ID:        "welcome",
			Type:      "info",
			Message:   "欢迎来到" + scene.Title,
			Actions:   []string{"确定"},
			Timestamp: time.Now(),
		}
		uiState.Notifications = append(uiState.Notifications, welcomeNotification)
	}

	return uiState
}

// 缓存清理
func (s *SceneAggregateService) startCacheCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			s.cleanupExpiredCache()
		}
	}()
}

// cleanupExpiredCache 清理过期缓存
func (s *SceneAggregateService) cleanupExpiredCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	now := time.Now()
	for cacheKey, cached := range s.aggregateCache {
		if now.Sub(cached.Timestamp) > s.cacheExpiry {
			delete(s.aggregateCache, cacheKey)
		}
	}
}

// 当相关数据更新时清除缓存
func (s *SceneAggregateService) InvalidateSceneCache(sceneID string) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	scenePrefix := sceneID + "_"
	var keysToDelete []string

	for key := range s.aggregateCache {
		// 检查键是否以 "sceneID_" 开头，或者就是 "sceneID_default"
		if strings.HasPrefix(key, scenePrefix) || key == sceneID+"_default" {
			keysToDelete = append(keysToDelete, key)
		}
	}

	// 删除匹配的缓存项
	for _, key := range keysToDelete {
		delete(s.aggregateCache, key)
	}
}
