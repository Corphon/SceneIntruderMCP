// internal/services/scene_service.go
package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/storage"
)

// SceneData 包含场景及其相关数据
type SceneData struct {
	Scene      models.Scene
	Context    models.SceneContext
	Settings   models.SceneSettings
	Characters []*models.Character
	Items      []*models.Item
}

// SceneService 处理场景相关的业务逻辑
type SceneService struct {
	BasePath    string
	FileCache   *storage.FileCacheService
	ItemService *ItemService
}

// LLMServicer 定义LLM服务接口
type LLMServicer interface {
	AnalyzeText(text, title string) (*models.AnalysisResult, error)
	AnalyzeContent(text string) (*ContentAnalysis, error)
}

// NewSceneService 创建场景服务
func NewSceneService(basePath string) *SceneService {
	if basePath == "" {
		basePath = "data/scenes"
	}

	// 创建基础目录
	if err := os.MkdirAll(basePath, 0755); err != nil {
		fmt.Printf("警告: 创建场景目录失败: %v\n", err)
	}

	// 创建文件缓存服务
	fileCache := storage.NewFileCacheService(200, 10*time.Minute)
	/*
		// 获取或创建物品服务 - 添加防止死锁的保护
		container := di.GetContainer()
		var itemService *ItemService

		// 首先尝试获取已注册的服务
		if itemObj := container.Get("item"); itemObj != nil {
			if is, ok := itemObj.(*ItemService); ok {
				itemService = is
			}
		}

		// 如果获取失败，则创建新服务
		if itemService == nil {
			itemService = NewItemService()
			// 只有创建成功才注册到容器
			if itemService != nil {
				container.Register("item", itemService)
			}
		}
	*/
	return &SceneService{
		BasePath:    basePath,
		FileCache:   fileCache,
		ItemService: nil, // ItemService 先设为 nil，在需要时再获取
	}
}

// CreateScene 创建新场景
func (s *SceneService) CreateScene(title, description, era, theme string) (*models.Scene, error) {
	// 生成场景ID
	sceneID := fmt.Sprintf("scene_%d", time.Now().UnixNano())

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
		Title:       title,
		Description: description,
		Era:         era,
		Themes:      themes,
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
	}

	// 验证基础路径存在
	if _, err := os.Stat(s.BasePath); os.IsNotExist(err) {
		// 尝试创建基础路径
		if err := os.MkdirAll(s.BasePath, 0755); err != nil {
			return nil, fmt.Errorf("基础路径不存在且无法创建: %w", err)
		}
	}

	// 创建场景目录
	scenePath := filepath.Join(s.BasePath, sceneID)
	fmt.Printf("DEBUG: 创建场景目录: %s\n", scenePath)
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

	// 初始化上下文
	context := models.SceneContext{
		SceneID:       sceneID,
		Conversations: []models.Conversation{},
		LastUpdated:   time.Now(),
	}

	if err := s.UpdateContext(sceneID, &context); err != nil {
		return nil, fmt.Errorf("初始化场景上下文失败: %w", err)
	}

	return scene, nil
}

// 懒加载方法
func (s *SceneService) getItemService() *ItemService {
	if s.ItemService == nil {
		container := di.GetContainer()
		if itemObj := container.Get("item"); itemObj != nil {
			if is, ok := itemObj.(*ItemService); ok {
				s.ItemService = is
			}
		}

		// 如果还是获取不到，创建一个新的
		if s.ItemService == nil {
			s.ItemService = NewItemService()
		}
	}
	return s.ItemService
}

// LoadScene 加载场景数据
func (s *SceneService) LoadScene(sceneID string) (*SceneData, error) {
	// 构建场景文件路径
	sceneFilePath := filepath.Join(s.BasePath, sceneID, "scene.json")

	var scene models.Scene
	// 使用文件缓存读取场景数据
	if err := s.FileCache.ReadFile(sceneFilePath, &scene); err != nil {
		return nil, fmt.Errorf("加载场景数据失败: %w", err)
	}

	// 加载角色
	characters, err := s.loadCharacters(sceneID)
	if err != nil {
		fmt.Printf("警告: 加载场景角色失败: %v\n", err)
	}

	// 加载物品
	items, err := s.getItemService().GetAllItems(sceneID)
	if err != nil {
		fmt.Printf("警告: 加载场景物品失败: %v\n", err)
	}

	// 构建场景数据
	sceneData := &SceneData{
		Scene:      scene,
		Characters: characters,
		Items:      items,
	}

	return sceneData, nil
}

// loadCharacters 从文件系统加载场景角色
func (s *SceneService) loadCharacters(sceneID string) ([]*models.Character, error) {
	charactersDir := filepath.Join(s.BasePath, sceneID, "characters")

	// 检查角色目录是否存在
	if _, err := os.Stat(charactersDir); os.IsNotExist(err) {
		return []*models.Character{}, nil // 目录不存在返回空数组
	}

	// 读取目录中的所有文件
	files, err := os.ReadDir(charactersDir)
	if err != nil {
		return nil, fmt.Errorf("读取角色目录失败: %w", err)
	}

	// 创建指针切片存储角色
	characters := make([]*models.Character, 0, len(files))

	// 遍历所有JSON文件
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			charPath := filepath.Join(charactersDir, file.Name())

			var character models.Character
			// 使用文件缓存读取角色数据
			if err := s.FileCache.ReadFile(charPath, &character); err != nil {
				fmt.Printf("警告: 读取角色数据失败: %v\n", err)
				continue
			}

			characters = append(characters, &character)
		}
	}

	return characters, nil
}

// UpdateContext 更新场景上下文
func (s *SceneService) UpdateContext(sceneID string, context *models.SceneContext) error {
	context.LastUpdated = time.Now()

	contextDataJSON, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化上下文数据失败: %w", err)
	}

	contextPath := filepath.Join(s.BasePath, sceneID, "context.json")
	if err := os.WriteFile(contextPath, contextDataJSON, 0644); err != nil {
		return fmt.Errorf("保存上下文数据失败: %w", err)
	}

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

	return nil
}

// GetAllScenes 获取所有场景列表
func (s *SceneService) GetAllScenes() ([]models.Scene, error) {
	// 读取场景目录中的所有子目录
	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		return nil, fmt.Errorf("读取场景目录失败: %w", err)
	}

	scenes := make([]models.Scene, 0, len(entries))

	// 遍历所有可能的场景目录
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // 跳过非目录项
		}

		sceneID := entry.Name()
		scenePath := filepath.Join(s.BasePath, sceneID, "scene.json")

		// 检查场景文件是否存在
		if _, err := os.Stat(scenePath); os.IsNotExist(err) {
			continue // 跳过不包含scene.json的目录
		}

		// 读取场景数据
		sceneDataBytes, err := os.ReadFile(scenePath)
		if err != nil {
			log.Printf("警告: 无法读取场景 %s: %v", sceneID, err)
			continue
		}

		var scene models.Scene
		if err := json.Unmarshal(sceneDataBytes, &scene); err != nil {
			log.Printf("警告: 无法解析场景 %s: %v", sceneID, err)
			continue
		}

		// 添加到结果中
		scenes = append(scenes, scene)
	}

	return scenes, nil
}

// CreateSceneFromText 从文本创建新场景
func (s *SceneService) CreateSceneFromText(text, title string) (*models.Scene, error) {
	// 检查参数有效性
	if text == "" || title == "" {
		return nil, fmt.Errorf("文本和标题不能为空")
	}

	// 创建分析器服务（需要注入LLMService）
	container := di.GetContainer()
	llmService, ok := container.Get("llm").(LLMServicer)
	if !ok || llmService == nil {
		return nil, fmt.Errorf("LLM服务未初始化，无法分析文本")
	}

	analysisResult, err := llmService.AnalyzeText(text, title)
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

	return scene, nil
}

// CreateSceneWithCharacters 创建带有角色的场景
func (s *SceneService) CreateSceneWithCharacters(scene *models.Scene, characters []models.Character) error {
	// 创建场景目录
	scenePath := filepath.Join(s.BasePath, scene.ID)
	if err := os.MkdirAll(scenePath, 0755); err != nil {
		return fmt.Errorf("创建场景目录失败: %w", err)
	}

	// 保存场景数据
	sceneDataJSON, err := json.MarshalIndent(scene, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化场景数据失败: %w", err)
	}

	sceneFilePath := filepath.Join(scenePath, "scene.json")
	if err := os.WriteFile(sceneFilePath, sceneDataJSON, 0644); err != nil {
		return fmt.Errorf("保存场景数据失败: %w", err)
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
		LastUpdated: time.Now(),
	}

	if err := s.UpdateSettings(scene.ID, &settings); err != nil {
		return fmt.Errorf("初始化场景设置失败: %w", err)
	}

	// 创建角色目录
	charactersDir := filepath.Join(scenePath, "characters")
	if err := os.MkdirAll(charactersDir, 0755); err != nil {
		return fmt.Errorf("创建角色目录失败: %w", err)
	}

	// 保存角色数据
	for i, character := range characters {
		// 确保每个角色都有ID
		if character.ID == "" {
			character.ID = fmt.Sprintf("char_%d", time.Now().UnixNano()+int64(i))
		}
		character.SceneID = scene.ID

		// 序列化角色数据
		charDataJSON, err := json.MarshalIndent(character, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化角色数据失败: %w", err)
		}

		// 保存角色数据文件
		charPath := filepath.Join(charactersDir, character.ID+".json")
		if err := os.WriteFile(charPath, charDataJSON, 0644); err != nil {
			return fmt.Errorf("保存角色数据失败: %w", err)
		}
	}

	return nil
}
