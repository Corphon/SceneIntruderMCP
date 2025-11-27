// internal/services/scene_service.go
package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/storage"
)

// SceneData 包含场景及其相关数据
type SceneData struct {
	Scene      models.Scene         `json:"scene"`
	Context    models.SceneContext  `json:"context"`
	Settings   models.SceneSettings `json:"settings"`
	Characters []*models.Character  `json:"characters"`
	Items      []*models.Item       `json:"items"`
}

// SceneService 处理场景相关的业务逻辑
type SceneService struct {
	BasePath    string
	FileCache   *storage.FileStorage
	ItemService *ItemService

	// 并发控制
	sceneLocks   sync.Map // sceneID -> *sync.RWMutex
	cacheMutex   sync.RWMutex
	sceneCache   map[string]*CachedSceneData
	listCache    *CachedSceneList
	cacheExpiry  time.Duration
	maxCacheSize int // Maximum number of cached scenes
}

// CachedSceneList 缓存的场景列表
type CachedSceneList struct {
	Scenes    []models.Scene
	Timestamp time.Time
}

// LLMServicer 定义LLM服务接口
type LLMServicer interface {
	AnalyzeText(text, title string) (*models.AnalysisResult, error)
	AnalyzeContent(text string) (*ContentAnalysis, error)
}

// ---------------------------------------------------
// NewSceneService 创建场景服务
func NewSceneService(basePath string) *SceneService {
	if basePath == "" {
		basePath = "data/scenes"
	}

	// 创建基础目录
	if err := os.MkdirAll(basePath, 0755); err != nil {
		fmt.Printf("警告: 创建场景目录失败: %v\n", err)
	}

	// 初始化 FileStorage
	fileStorage, err := storage.NewFileStorage(basePath)
	if err != nil {
		fmt.Printf("警告: 创建文件存储失败: %v\n", err)
		fileStorage = nil
	}

	service := &SceneService{
		BasePath:     basePath,
		FileCache:    fileStorage,
		sceneCache:   make(map[string]*CachedSceneData),
		cacheExpiry:  5 * time.Minute,
		maxCacheSize: 100, // Default to 100 cached scenes
	}

	// 启动缓存清理
	service.startCacheCleanup()

	return service
}

// 获取场景锁
func (s *SceneService) getSceneLock(sceneID string) *sync.RWMutex {
	value, _ := s.sceneLocks.LoadOrStore(sceneID, &sync.RWMutex{})
	return value.(*sync.RWMutex)
}

// 线程安全的场景创建
func (s *SceneService) CreateScene(userID, title, description, content, source string) (*models.Scene, error) {
	// 验证输入参数
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("场景标题不能为空")
	}

	if strings.TrimSpace(description) == "" {
		return nil, fmt.Errorf("场景描述不能为空")
	}

	// 线程安全的ID生成
	sceneID := s.generateUniqueSceneID()

	// 获取场景锁
	lock := s.getSceneLock(sceneID)
	lock.Lock()
	defer lock.Unlock()

	// 创建场景对象
	scene := &models.Scene{
		ID:          sceneID,
		UserID:      userID,
		Title:       title,
		Description: description,
		Source:      source,
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
	}

	// 如果提供了内容，将其添加到场景对象中（如果模型支持的话）
	// 注意：在当前模型中，场景内容不是直接字段，但我们可以通过其他方式处理
	if content != "" {
		scene.Summary = content // 将内容暂时存储在Summary字段中
	}

	// 使用 FileStorage 保存场景数据
	if s.FileCache != nil {
		if err := s.FileCache.SaveJSONFile(sceneID, "scene.json", scene); err != nil {
			return nil, fmt.Errorf("保存场景数据失败: %w", err)
		}
	} else {
		// 降级到直接文件操作（如果 FileStorage 初始化失败）
		scenePath := filepath.Join(s.BasePath, sceneID)
		if err := os.MkdirAll(scenePath, 0755); err != nil {
			return nil, fmt.Errorf("创建场景目录失败: %w", err)
		}

		sceneDataJSON, err := json.MarshalIndent(scene, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("序列化场景数据失败: %w", err)
		}

		sceneFilePath := filepath.Join(scenePath, "scene.json")
		tempPath := sceneFilePath + ".tmp"

		if err := os.WriteFile(tempPath, sceneDataJSON, 0644); err != nil {
			return nil, fmt.Errorf("保存场景文件失败: %w", err)
		}

		if err := os.Rename(tempPath, sceneFilePath); err != nil {
			os.Remove(tempPath)
			return nil, fmt.Errorf("保存场景文件失败: %w", err)
		}
	}

	// 初始化场景上下文
	context := models.SceneContext{
		SceneID:       sceneID,
		Conversations: []models.Conversation{},
		LastUpdated:   time.Now(),
	}

	if err := s.UpdateContext(sceneID, &context); err != nil {
		log.Printf("警告: 初始化场景上下文失败: %v", err)
		// 不要让上下文初始化失败阻断场景创建
	}

	// 初始化场景设置
	settings := models.SceneSettings{
		SceneID:     sceneID,
		UserID:      userID,
		LastUpdated: time.Now(),
	}

	if err := s.UpdateSettings(sceneID, &settings); err != nil {
		log.Printf("警告: 初始化场景设置失败: %v", err)
		// 不要让设置初始化失败阻断场景创建
	}

	// 清除列表缓存
	s.invalidateListCache()

	return scene, nil
}

// 清除场景缓存
func (s *SceneService) invalidateSceneCache(sceneID string) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	delete(s.sceneCache, sceneID)
	s.listCache = nil // 清除列表缓存
}

// 清除列表缓存
func (s *SceneService) invalidateListCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.listCache = nil
}

// 清理过期缓存
func (s *SceneService) cleanupExpiredCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	now := time.Now()
	for sceneID, cached := range s.sceneCache {
		if now.Sub(cached.Timestamp) > s.cacheExpiry {
			delete(s.sceneCache, sceneID)
		}
	}

	if s.listCache != nil && now.Sub(s.listCache.Timestamp) > s.cacheExpiry {
		s.listCache = nil
	}
}

// 启动后台缓存清理
func (s *SceneService) startCacheCleanup() {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			s.cleanupExpiredCache()
			s.enforceMaxCacheSize()
		}
	}()
}

// enforceMaxCacheSize enforces the maximum cache size by removing oldest entries
func (s *SceneService) enforceMaxCacheSize() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// Check if cache size exceeds maximum
	if len(s.sceneCache) <= s.maxCacheSize {
		return
	}

	// Find oldest entries to remove
	type cacheEntryWithTime struct {
		key       string
		timestamp time.Time
	}

	var entries []cacheEntryWithTime
	for key, entry := range s.sceneCache {
		entries = append(entries, cacheEntryWithTime{key: key, timestamp: entry.Timestamp})
	}

	// Sort by timestamp (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].timestamp.Before(entries[j].timestamp)
	})

	// Remove excess entries
	removeCount := len(entries) - s.maxCacheSize
	if removeCount > 0 {
		for i := 0; i < removeCount; i++ {
			delete(s.sceneCache, entries[i].key)
		}
		log.Printf("场景服务缓存大小限制执行: 移除了 %d 个最旧的缓存条目", removeCount)
	}
}

// 生成唯一场景ID
func (s *SceneService) generateUniqueSceneID() string {
	for {
		id := fmt.Sprintf("scene_%d", time.Now().UnixNano())
		scenePath := filepath.Join(s.BasePath, id)

		if _, err := os.Stat(scenePath); os.IsNotExist(err) {
			return id
		}

		// 如果ID冲突，稍微等待后重试
		time.Sleep(time.Microsecond)
	}
}

// LoadScene 带缓存的加载场景数据
func (s *SceneService) LoadScene(sceneID string) (*SceneData, error) {
	// 第1次缓存检查
	s.cacheMutex.RLock()
	if cached, exists := s.sceneCache[sceneID]; exists {
		if time.Since(cached.Timestamp) < s.cacheExpiry {
			s.cacheMutex.RUnlock()
			return cached.SceneData, nil
		}
	}
	s.cacheMutex.RUnlock()

	// 获取场景锁
	lock := s.getSceneLock(sceneID)
	lock.RLock()
	defer lock.RUnlock()

	// 双重检查缓存
	s.cacheMutex.RLock()
	if cached, exists := s.sceneCache[sceneID]; exists {
		if time.Since(cached.Timestamp) < s.cacheExpiry {
			s.cacheMutex.RUnlock()
			return cached.SceneData, nil
		}
	}
	s.cacheMutex.RUnlock()

	// 使用 FileStorage 读取场景数据
	var scene models.Scene
	if err := s.FileCache.LoadJSONFile(sceneID, "scene.json", &scene); err != nil {
		return nil, err
	}

	// 加载角色数据
	characters, err := s.loadCharactersCached(sceneID)
	if err != nil {
		// 角色加载失败不应该阻断场景加载
		fmt.Printf("警告: 加载角色失败: %v\n", err)
		characters = make([]*models.Character, 0)
	}

	// 加载物品数据（如果需要）
	items := make([]*models.Item, 0)
	if s.ItemService != nil {
		loadedItems, err := s.ItemService.GetAllItems(sceneID)
		if err != nil {
			fmt.Printf("警告: 加载物品失败: %v\n", err)
		} else {
			items = loadedItems
		}
	}

	// 加载上下文和设置
	context := models.SceneContext{
		SceneID:       sceneID,
		Conversations: []models.Conversation{},
		LastUpdated:   time.Now(),
	}

	if s.FileCache != nil {
		if err := s.FileCache.LoadJSONFile(sceneID, "context.json", &context); err != nil {
			// 如果 context 不存在，则保持默认结构
			context.SceneID = sceneID
		}
	}

	settings := models.SceneSettings{
		SceneID:     sceneID,
		UserID:      scene.UserID,
		LastUpdated: time.Now(),
	}

	if s.FileCache != nil {
		if err := s.FileCache.LoadJSONFile(sceneID, "settings.json", &settings); err != nil {
			settings.SceneID = sceneID
		}
	}

	// 更新元数据计数
	scene.CharacterCount = len(characters)
	scene.ItemCount = len(items)

	// 构建完整的 SceneData
	sceneData := &SceneData{
		Scene:      scene,
		Context:    context,
		Settings:   settings,
		Characters: characters,
		Items:      items,
	}

	// 更新缓存
	s.cacheMutex.Lock()
	s.sceneCache[sceneID] = &CachedSceneData{
		SceneData: sceneData,
		Timestamp: time.Now(),
	}
	s.cacheMutex.Unlock()

	defer func() {
		// 异步预加载，不影响当前响应时间
		s.preloadCharacters(sceneID)
	}()

	return sceneData, nil
}

// 带缓存的角色加载
func (s *SceneService) loadCharactersCached(sceneID string) ([]*models.Character, error) {
	if s.FileCache == nil {
		return nil, fmt.Errorf("文件存储服务未初始化")
	}

	charactersDir := filepath.Join(s.BasePath, sceneID, "characters")

	if _, err := os.Stat(charactersDir); os.IsNotExist(err) {
		return []*models.Character{}, nil
	}

	files, err := os.ReadDir(charactersDir)
	if err != nil {
		return nil, fmt.Errorf("读取角色目录失败: %w", err)
	}

	characters := make([]*models.Character, 0, len(files))

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			var character models.Character

			// 🔧 关键修复：使用相对路径而不是绝对路径
			characterPath := filepath.Join("characters", file.Name())
			if err := s.FileCache.LoadJSONFile(sceneID, characterPath, &character); err != nil {
				fmt.Printf("警告: 读取角色数据失败: %v\n", err)
				continue
			}

			// 将加载的角色添加到切片中
			characters = append(characters, &character)
		}
	}

	return characters, nil
}

// 异步预加载角色数据
func (s *SceneService) preloadCharacters(sceneID string) {
	go func() {
		// 异步预加载角色数据
		s.loadCharactersCached(sceneID)
	}()
}

// AddCharacter 添加新角色到场景
func (s *SceneService) AddCharacter(sceneID string, character *models.Character) error {
	// 验证输入参数
	if sceneID == "" {
		return fmt.Errorf("场景ID不能为空")
	}
	if character == nil {
		return fmt.Errorf("角色数据不能为空")
	}

	// 获取场景锁
	lock := s.getSceneLock(sceneID)
	lock.Lock()
	defer lock.Unlock()

	// 检查场景是否存在
	sceneDir := filepath.Join(s.BasePath, sceneID)
	if _, err := os.Stat(sceneDir); os.IsNotExist(err) {
		return fmt.Errorf("场景不存在: %s", sceneID)
	}

	// 生成唯一角色ID（如果没有）
	if character.ID == "" {
		character.ID = s.generateUniqueCharacterID(sceneID)
	}

	// 设置必要字段
	character.SceneID = sceneID
	character.CreatedAt = time.Now()
	character.LastUpdated = time.Now()

	// 统一使用 FileStorage
	if s.FileCache != nil {
		// 修复路径格式
		characterDir := filepath.Join(sceneID, "characters")
		characterFile := character.ID + ".json"

		if err := s.FileCache.SaveJSONFile(characterDir, characterFile, character); err != nil {
			return fmt.Errorf("保存角色数据失败: %w", err)
		}
	} else {
		return fmt.Errorf("文件存储服务未初始化")
	}

	// 清除场景缓存
	s.invalidateSceneCache(sceneID)

	return nil
}

// generateUniqueCharacterID 生成唯一角色ID
func (s *SceneService) generateUniqueCharacterID(sceneID string) string {
	charactersDir := filepath.Join(s.BasePath, sceneID, "characters")

	for {
		id := fmt.Sprintf("char_%d", time.Now().UnixNano())
		characterPath := filepath.Join(charactersDir, id+".json")

		if _, err := os.Stat(characterPath); os.IsNotExist(err) {
			return id
		}

		// 如果ID冲突，稍微等待后重试
		time.Sleep(time.Microsecond)
	}
}

// DeleteCharacter 删除场景中的角色
func (s *SceneService) DeleteCharacter(sceneID, characterID string) error {
	// 验证输入参数
	if sceneID == "" || characterID == "" {
		return fmt.Errorf("场景ID和角色ID不能为空")
	}

	// 获取场景锁
	lock := s.getSceneLock(sceneID)
	lock.Lock()
	defer lock.Unlock()

	// 检查文件存储服务是否初始化
	if s.FileCache == nil {
		// Fallback to direct file operation if FileCache is not available
		characterDirPath := filepath.Join(s.BasePath, sceneID, "characters")
		characterFilePath := filepath.Join(characterDirPath, characterID+".json")

		// 检查角色文件是否存在
		if _, err := os.Stat(characterFilePath); os.IsNotExist(err) {
			return fmt.Errorf("角色不存在: %s", characterID)
		}

		// 删除角色文件
		if err := os.Remove(characterFilePath); err != nil {
			return fmt.Errorf("删除角色文件失败: %w", err)
		}
	} else {
		// Use FileCache DeleteFile method
		characterDir := filepath.Join(sceneID, "characters")
		characterFile := characterID + ".json"

		// First check if file exists by trying to load it
		var existingCharacter models.Character
		if err := s.FileCache.LoadJSONFile(characterDir, characterFile, &existingCharacter); err != nil {
			return fmt.Errorf("角色不存在: %s", characterID)
		}

		// Delete the file using FileCache
		if err := s.FileCache.DeleteFile(characterDir, characterFile); err != nil {
			return fmt.Errorf("删除角色文件失败: %w", err)
		}
	}

	// 清除场景缓存
	s.invalidateSceneCache(sceneID)

	return nil
}

// UpdateContext 更新场景上下文
func (s *SceneService) UpdateContext(sceneID string, context *models.SceneContext) error {
	context.LastUpdated = time.Now()

	// 使用 FileStorage 保存上下文数据
	if s.FileCache != nil {
		if err := s.FileCache.SaveJSONFile(sceneID, "context.json", context); err != nil {
			return fmt.Errorf("保存上下文数据失败: %w", err)
		}
	} else {
		// 降级到直接文件操作
		contextDataJSON, err := json.MarshalIndent(context, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化上下文数据失败: %w", err)
		}

		contextPath := filepath.Join(s.BasePath, sceneID, "context.json")
		if err := os.WriteFile(contextPath, contextDataJSON, 0644); err != nil {
			return fmt.Errorf("保存上下文数据失败: %w", err)
		}
	}

	// 缓存清除
	s.invalidateSceneCache(sceneID)

	return nil
}

// UpdateSettings 更新场景设置
func (s *SceneService) UpdateSettings(sceneID string, settings *models.SceneSettings) error {
	settings.LastUpdated = time.Now()

	settingsDataJSON, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化设置数据失败: %w", err)
	}

	settingsPath := filepath.Join(s.BasePath, sceneID, "settings.json")
	if err := os.WriteFile(settingsPath, settingsDataJSON, 0644); err != nil {
		return fmt.Errorf("保存设置数据失败: %w", err)
	}

	// 缓存清除
	s.invalidateSceneCache(sceneID)

	return nil
}

// UpdateCharacter 更新角色
func (s *SceneService) UpdateCharacter(sceneID, characterID string, character *models.Character) error {
	// 验证输入参数
	if sceneID == "" || characterID == "" {
		return fmt.Errorf("场景ID和角色ID不能为空")
	}

	if character == nil {
		return fmt.Errorf("角色数据不能为空")
	}

	// 获取场景锁
	lock := s.getSceneLock(sceneID)
	lock.Lock()
	defer lock.Unlock()

	// 检查文件存储服务是否初始化
	if s.FileCache == nil {
		return fmt.Errorf("文件存储服务未初始化")
	}

	// 检查角色文件是否存在 by loading it first
	characterDir := filepath.Join(sceneID, "characters")
	characterFile := characterID + ".json"

	var existingCharacter models.Character
	if err := s.FileCache.LoadJSONFile(characterDir, characterFile, &existingCharacter); err != nil {
		return fmt.Errorf("角色不存在: %s", characterID)
	}

	// 确保角色ID和场景ID正确设置
	character.ID = characterID
	character.SceneID = sceneID
	character.LastUpdated = time.Now()

	// Use the existing character's data to preserve fields that might not be in the update
	if character.Name == "" {
		character.Name = existingCharacter.Name
	}
	if character.Description == "" {
		character.Description = existingCharacter.Description
	}
	if character.Personality == "" {
		character.Personality = existingCharacter.Personality
	}

	// 使用 FileStorage 保存更新后的角色数据
	if err := s.FileCache.SaveJSONFile(characterDir, characterFile, character); err != nil {
		return fmt.Errorf("保存角色数据失败: %w", err)
	}

	// 清除场景缓存
	s.invalidateSceneCache(sceneID)

	return nil
}

// GetAllScenes 带缓存的获取所有场景列表
func (s *SceneService) GetAllScenes() ([]models.Scene, error) {
	// 检查列表缓存
	s.cacheMutex.RLock()
	if s.listCache != nil && time.Since(s.listCache.Timestamp) < s.cacheExpiry {
		scenes := make([]models.Scene, len(s.listCache.Scenes))
		copy(scenes, s.listCache.Scenes)
		s.cacheMutex.RUnlock()
		return scenes, nil
	}
	s.cacheMutex.RUnlock()

	// 加载场景列表
	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		return nil, fmt.Errorf("读取场景目录失败: %w", err)
	}

	scenes := make([]models.Scene, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sceneID := entry.Name()
		scenePath := filepath.Join(s.BasePath, sceneID, "scene.json")

		if _, err := os.Stat(scenePath); os.IsNotExist(err) {
			continue
		}

		var scene models.Scene
		if s.FileCache != nil {
			if err := s.FileCache.LoadJSONFile(sceneID, "scene.json", &scene); err != nil {
				log.Printf("警告: 无法读取场景 %s: %v", sceneID, err)
				continue
			}
		} else {
			// 降级到直接文件读取
			sceneData, err := os.ReadFile(scenePath)
			if err != nil {
				log.Printf("警告: 无法读取场景文件 %s: %v", scenePath, err)
				continue
			}

			if err := json.Unmarshal(sceneData, &scene); err != nil {
				log.Printf("警告: 无法解析场景数据 %s: %v", sceneID, err)
				continue
			}
		}

		// 计算角色/物品数量，便于前端展示
		s.enrichSceneSummary(sceneID, &scene)

		scenes = append(scenes, scene)
	}

	// 更新列表缓存
	s.cacheMutex.Lock()
	s.listCache = &CachedSceneList{
		Scenes:    scenes,
		Timestamp: time.Now(),
	}
	s.cacheMutex.Unlock()

	return scenes, nil
}

// enrichSceneSummary 补充场景的角色和物品数量等元数据
func (s *SceneService) enrichSceneSummary(sceneID string, scene *models.Scene) {
	if scene == nil {
		return
	}

	characterDir := filepath.Join(s.BasePath, sceneID, "characters")
	if entries, err := os.ReadDir(characterDir); err == nil {
		count := 0
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			count++
		}
		scene.CharacterCount = count
	}

	itemsDir := filepath.Join(s.BasePath, sceneID, "items")
	if entries, err := os.ReadDir(itemsDir); err == nil {
		count := 0
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			count++
		}
		scene.ItemCount = count
	}
}

// CreateSceneFromText 从文本创建新场景
func (s *SceneService) CreateSceneFromText(userID, text, title string) (*models.Scene, error) {
	// 检查参数有效性
	if text == "" || title == "" {
		return nil, fmt.Errorf("文本和标题不能为空")
	}

	// 创建分析器服务（需要注入AnalyzerService）
	container := di.GetContainer()
	analyzerService, ok := container.Get("analyzer").(*AnalyzerService)
	if !ok || analyzerService == nil {
		return nil, fmt.Errorf("分析服务未初始化，无法分析文本")
	}

	analysisResult, err := analyzerService.AnalyzeText(text, title)
	if err != nil {
		return nil, fmt.Errorf("分析文本失败: %w", err)
	}

	// 生成场景ID
	sceneID := fmt.Sprintf("scene_%d", time.Now().UnixNano())

	// 提取主题和时代（默认值）
	era := "现代"
	theme := "未指定"

	// 如果分析结果包含场景信息，使用第一个场景的信息
	var description string
	if len(analysisResult.Scenes) > 0 {
		mainScene := analysisResult.Scenes[0]
		description = mainScene.Description
		if mainScene.Era != "" {
			era = mainScene.Era
		}
		if len(mainScene.Themes) > 0 {
			theme = strings.Join(mainScene.Themes, ", ")
		}
	} else {
		// 使用摘要作为描述
		description = analysisResult.Summary
		if description == "" {
			description = "从文本中提取的场景"
		}
	}
	// 将主题字符串转换为切片
	var themes []string
	if theme != "" {
		// 如果主题包含逗号，按逗号分割成多个主题
		if strings.Contains(theme, ",") {
			themes = strings.Split(theme, ",")
			// 清理每个主题字符串前后的空格
			for i := range themes {
				themes[i] = strings.TrimSpace(themes[i])
			}
		} else {
			// 单个主题
			themes = []string{theme}
		}
	}
	// 创建场景对象
	scene := &models.Scene{
		ID:          sceneID,
		UserID:      userID,
		Title:       title,
		Description: description,
		Era:         era,
		Themes:      themes,
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
	}

	// 创建场景目录
	scenePath := filepath.Join(s.BasePath, sceneID)
	if err := os.MkdirAll(scenePath, 0755); err != nil {
		return nil, fmt.Errorf("创建场景目录失败: %w", err)
	}

	// 保存场景数据
	sceneDataJSON, err := json.MarshalIndent(scene, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化场景数据失败: %w", err)
	}

	scenePath = filepath.Join(s.BasePath, sceneID, "scene.json")
	if err := os.WriteFile(scenePath, sceneDataJSON, 0644); err != nil {
		return nil, fmt.Errorf("保存场景数据失败: %w", err)
	}

	// 保存角色数据
	if len(analysisResult.Characters) > 0 {
		charactersDir := filepath.Join(s.BasePath, sceneID, "characters")
		if err := os.MkdirAll(charactersDir, 0755); err != nil {
			log.Printf("警告: 创建角色目录失败: %v", err)
		} else {
			for i, character := range analysisResult.Characters {
				// 创建角色ID
				charID := fmt.Sprintf("char_%d_%d", time.Now().UnixNano(), i)
				character.ID = charID
				character.SceneID = sceneID

				charDataJSON, err := json.MarshalIndent(character, "", "  ")
				if err != nil {
					log.Printf("警告: 无法序列化角色数据: %v", err)
					continue
				}

				charPath := filepath.Join(charactersDir, charID+".json")
				if err := os.WriteFile(charPath, charDataJSON, 0644); err != nil {
					log.Printf("警告: 保存角色数据失败: %v", err)
				}
			}
		}
	}

	// 保存物品数据
	if len(analysisResult.Items) > 0 {
		itemsDir := filepath.Join(s.BasePath, sceneID, "items")
		if err := os.MkdirAll(itemsDir, 0755); err != nil {
			log.Printf("警告: 创建物品目录失败: %v", err)
		} else {
			for i, item := range analysisResult.Items {
				// 创建物品ID
				itemID := fmt.Sprintf("item_%d_%d", time.Now().UnixNano(), i)
				item.ID = itemID
				item.SceneID = sceneID

				itemDataJSON, err := json.MarshalIndent(item, "", "  ")
				if err != nil {
					log.Printf("警告: 无法序列化物品数据: %v", err)
					continue
				}

				itemPath := filepath.Join(itemsDir, itemID+".json")
				if err := os.WriteFile(itemPath, itemDataJSON, 0644); err != nil {
					log.Printf("警告: 保存物品数据失败: %v", err)
				}
			}
		}
	}

	// 初始化上下文
	context := models.SceneContext{
		SceneID:       sceneID,
		Conversations: []models.Conversation{},
		LastUpdated:   time.Now(),
	}

	if err := s.UpdateContext(sceneID, &context); err != nil {
		return nil, fmt.Errorf("初始化场景上下文失败: %w", err)
	}

	// 缓存清除
	s.invalidateListCache()

	return scene, nil
}

// CreateSceneWithCharacters 创建带有角色的场景
func (s *SceneService) CreateSceneWithCharacters(scene *models.Scene, characters []models.Character) error {
	// 使用 FileStorage 保存场景数据
	if s.FileCache != nil {
		if err := s.FileCache.SaveJSONFile(scene.ID, "scene.json", scene); err != nil {
			return fmt.Errorf("保存场景数据失败: %w", err)
		}
	} else {
		// 降级到直接文件操作
		scenePath := filepath.Join(s.BasePath, scene.ID)
		if err := os.MkdirAll(scenePath, 0755); err != nil {
			return fmt.Errorf("创建场景目录失败: %w", err)
		}

		sceneDataJSON, err := json.MarshalIndent(scene, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化场景数据失败: %w", err)
		}

		sceneFilePath := filepath.Join(scenePath, "scene.json")
		tempPath := sceneFilePath + ".tmp"

		if err := os.WriteFile(tempPath, sceneDataJSON, 0644); err != nil {
			return fmt.Errorf("保存场景文件失败: %w", err)
		}

		if err := os.Rename(tempPath, sceneFilePath); err != nil {
			os.Remove(tempPath)
			return fmt.Errorf("保存场景文件失败: %w", err)
		}
	}

	// 初始化上下文
	context := models.SceneContext{
		SceneID:       scene.ID,
		Conversations: []models.Conversation{},
		LastUpdated:   time.Now(),
	}

	if err := s.UpdateContext(scene.ID, &context); err != nil {
		return fmt.Errorf("初始化场景上下文失败: %w", err)
	}

	// 初始化设置
	settings := models.SceneSettings{
		SceneID:     scene.ID,
		UserID:      scene.UserID, // Use the scene's UserID if available
		LastUpdated: time.Now(),
	}

	if err := s.UpdateSettings(scene.ID, &settings); err != nil {
		return fmt.Errorf("初始化场景设置失败: %w", err)
	}

	// 保存角色数据 using FileStorage
	for i, character := range characters {
		// 确保每个角色都有ID
		if character.ID == "" {
			character.ID = fmt.Sprintf("char_%d", time.Now().UnixNano()+int64(i))
		}
		character.SceneID = scene.ID

		// 使用 FileStorage 保存角色数据
		characterDir := filepath.Join(scene.ID, "characters")
		characterFile := character.ID + ".json"

		if s.FileCache != nil {
			if err := s.FileCache.SaveJSONFile(characterDir, characterFile, &character); err != nil {
				return fmt.Errorf("保存角色数据失败: %w", err)
			}
		} else {
			// 降级到直接文件操作
			charactersDir := filepath.Join(s.BasePath, scene.ID, "characters")
			if err := os.MkdirAll(charactersDir, 0755); err != nil {
				return fmt.Errorf("创建角色目录失败: %w", err)
			}

			charDataJSON, err := json.MarshalIndent(character, "", "  ")
			if err != nil {
				return fmt.Errorf("序列化角色数据失败: %w", err)
			}

			charPath := filepath.Join(charactersDir, character.ID+".json")
			if err := os.WriteFile(charPath, charDataJSON, 0644); err != nil {
				return fmt.Errorf("保存角色数据失败: %w", err)
			}
		}
	}

	// 清除缓存
	s.invalidateListCache()

	return nil
}

// GetCharactersByScene 获取指定场景的所有角色
func (s *SceneService) GetCharactersByScene(sceneID string) ([]*models.Character, error) {
	// 检查场景是否存在
	sceneDir := filepath.Join(s.BasePath, sceneID)
	if _, err := os.Stat(sceneDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("场景不存在: %s", sceneID)
	}

	// 使用缓存的方法加载角色
	characters, err := s.loadCharactersCached(sceneID)
	if err != nil {
		return nil, fmt.Errorf("加载角色失败: %w", err)
	}

	return characters, nil
}

// DeleteScene 删除场景及其所有相关数据
func (s *SceneService) DeleteScene(sceneID string) error {
	// 验证输入参数
	if sceneID == "" {
		return fmt.Errorf("场景ID不能为空")
	}

	// 获取场景锁
	lock := s.getSceneLock(sceneID)
	lock.Lock()
	defer lock.Unlock()

	// 检查场景是否存在
	sceneDir := filepath.Join(s.BasePath, sceneID)
	if _, err := os.Stat(sceneDir); os.IsNotExist(err) {
		return fmt.Errorf("场景不存在: %s", sceneID)
	}

	// 删除场景目录及其所有内容
	if err := os.RemoveAll(sceneDir); err != nil {
		return fmt.Errorf("删除场景目录失败: %w", err)
	}

	// 清除缓存
	s.invalidateSceneCache(sceneID)
	s.invalidateListCache()

	return nil
}
