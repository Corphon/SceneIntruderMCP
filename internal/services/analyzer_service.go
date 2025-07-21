// internal/services/analyzer_service.go
package services

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/llm"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

// AnalyzerService 分析和提取文本中的各种信息
type AnalyzerService struct {
	LLMService    *LLMService
	semaphore     chan struct{}
	analysisCache *AnalysisCache
}

// 分析结果缓存
type AnalysisCache struct {
	cache      map[string]*CachedAnalysis
	mutex      sync.RWMutex
	expiration time.Duration
}

type CachedAnalysis struct {
	Result    *models.AnalysisResult
	Timestamp time.Time
}

// NewAnalyzerService 创建分析服务
func NewAnalyzerService() (*AnalyzerService, error) {
	llmService, err := NewLLMService()
	if err != nil {
		return nil, err
	}

	return &AnalyzerService{
		LLMService: llmService,
		semaphore:  make(chan struct{}, 3), // 限制并发数量为3
		analysisCache: &AnalysisCache{
			cache:      make(map[string]*CachedAnalysis),
			expiration: 30 * time.Minute,
		},
	}, nil
}

// NewAnalyzerServiceWithProvider 使用指定的LLM Provider创建分析服务
func NewAnalyzerServiceWithProvider(provider llm.Provider) *AnalyzerService {
	// 添加对nil提供商的处理
	if provider == nil {
		return &AnalyzerService{
			LLMService: &LLMService{
				provider:     nil,
				providerName: "empty",
				isReady:      false,
				readyState:   "提供商未初始化",
				cache: &LLMCache{
					cache:      make(map[string]*CacheEntry),
					mutex:      sync.RWMutex{},
					expiration: 30 * time.Minute,
				},
			},
			// 使用信号量限制并发数量
			semaphore: make(chan struct{}, 3),
			analysisCache: &AnalysisCache{
				cache:      make(map[string]*CachedAnalysis),
				expiration: 30 * time.Minute,
			},
		}
	}

	// 原有逻辑（提供商不为nil时）
	return &AnalyzerService{
		LLMService: &LLMService{
			provider:     provider,
			providerName: provider.GetName(),
			isReady:      true,
			readyState:   "已就绪",
			cache: &LLMCache{
				cache:      make(map[string]*CacheEntry),
				mutex:      sync.RWMutex{},
				expiration: 30 * time.Minute,
			},
		},
		// 使用信号量限制并发数量
		semaphore: make(chan struct{}, 3),
		analysisCache: &AnalysisCache{
			cache:      make(map[string]*CachedAnalysis),
			expiration: 30 * time.Minute,
		},
	}
}

// AnalyzeText 分析文本，提取场景、角色、物品等信息
// 🔧 优化后的 AnalyzeText 方法
func (s *AnalyzerService) AnalyzeText(text, title string) (*models.AnalysisResult, error) {
	// 获取并发许可
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	// 检查LLM提供商是否就绪
	if s.LLMService == nil || !s.LLMService.IsReady() {
		return nil, errors.New("LLM服务未配置或未就绪，请先在设置页面配置API密钥")
	}

	// 检查缓存
	cacheKey := s.generateCacheKey(text, title)
	if cachedResult := s.checkAnalysisCache(cacheKey); cachedResult != nil {
		return cachedResult, nil
	}

	// 一次性预处理
	isEnglish := isEnglishText(text + " " + title)

	result := &models.AnalysisResult{
		Title: title,
		Metadata: map[string]interface{}{
			"is_english":  isEnglish,
			"text_length": len(text),
		},
	}

	// 并行提取（使用 goroutine）
	var wg sync.WaitGroup
	var sceneErr, charErr, itemErr, summaryErr error
	var scenes []models.Scene
	var characters []models.Character
	var items []models.Item
	var summary string

	// 提取场景
	wg.Add(1)
	go func() {
		defer wg.Done()
		s, err := s.extractScenes(text, title)
		if err != nil {
			sceneErr = err
			return
		}
		scenes = s
	}()

	// 提取角色
	wg.Add(1)
	go func() {
		defer wg.Done()
		characters, err := s.extractCharacters(text, title)
		if err != nil {
			charErr = err
			return
		}
		result.Characters = characters
	}()

	// 提取角色
	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := s.extractCharacters(text, title)
		if err != nil {
			charErr = err
			return
		}
		characters = c
	}()

	// 提取物品
	wg.Add(1)
	go func() {
		defer wg.Done()
		i, err := s.extractItems(text, title)
		if err != nil {
			itemErr = err
			return
		}
		items = i
	}()

	// 生成摘要
	wg.Add(1)
	go func() {
		defer wg.Done()
		sum, err := s.generateSummary(text, title)
		if err != nil {
			summaryErr = err
			return
		}
		summary = sum
	}()

	// 等待所有任务完成
	wg.Wait()

	// 检查错误
	if sceneErr != nil {
		return nil, fmt.Errorf("提取场景失败: %w", sceneErr)
	}
	if charErr != nil {
		return nil, fmt.Errorf("提取角色失败: %w", charErr)
	}
	if itemErr != nil {
		return nil, fmt.Errorf("提取物品失败: %w", itemErr)
	}
	if summaryErr != nil {
		// 摘要生成失败不是致命错误
		result.Summary = "无法生成摘要。"
	}

	// 安全地设置结果
	result.Scenes = scenes
	result.Characters = characters
	result.Items = items
	result.Summary = summary

	// 添加到缓存
	s.addToAnalysisCache(cacheKey, result)

	return result, nil
}

// 提取场景信息
func (s *AnalyzerService) extractScenes(text, title string) ([]models.Scene, error) {
	// 使用LLMService的结构化输出功能
	sceneInfos, err := s.LLMService.ExtractScenes(context.Background(), text, title)
	if err != nil {
		return nil, err
	}

	// 转换为模型格式
	var scenes []models.Scene
	for _, info := range sceneInfos {
		scene := models.Scene{
			Name:        info.Name,
			Description: info.Description,
			Atmosphere:  info.Atmosphere,
			Era:         info.Era,
			Themes:      info.Themes,
		}

		// 处理物品列表 - 确保 info.Items 是字符串数组
		var items []models.Item
		if info.Items != nil {
			for _, itemName := range info.Items {
				// 确保 itemName 是有效字符串
				if itemName != "" {
					items = append(items, models.Item{
						Name:        itemName,
						Description: "场景中的物品",
					})
				}
			}
		}
		scene.Items = items

		scenes = append(scenes, scene)
	}

	return scenes, nil
}

// 提取角色信息
func (s *AnalyzerService) extractCharacters(text, title string) ([]models.Character, error) {
	// 使用LLMService的结构化输出功能
	characterInfos, err := s.LLMService.ExtractCharacters(context.Background(), text, title)
	if err != nil {
		return nil, err
	}

	// 转换为模型格式
	var characters []models.Character
	for _, info := range characterInfos {
		character := models.Character{
			Name:        info.Name,
			Role:        info.Role,
			Description: info.Description,
			Personality: info.Personality,
			Background:  info.Background,
			SpeechStyle: info.SpeechStyle,
			Knowledge:   info.Knowledge,
		}

		// 处理关系
		relationships := make(map[string]string)
		for name, relation := range info.Relationships {
			relationships[name] = relation
		}
		character.Relationships = relationships

		characters = append(characters, character)
	}

	return characters, nil
}

// 提取物品信息
func (s *AnalyzerService) extractItems(text, title string) ([]models.Item, error) {
	type ItemInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Importance  string `json:"importance"`
		Location    string `json:"location"`
		Usage       string `json:"usage,omitempty"`
	}

	// 检测文本语言
	isEnglish := isEnglishText(text + " " + title)

	var prompt, systemPrompt string

	if isEnglish {
		prompt = fmt.Sprintf(`Analyze the following text titled "%s" and extract all important item information:
	
	%s
	
	Please identify all significant items mentioned in the text, providing:
	1. Item name and physical description
	2. Function or purpose within the story
	3. Current location or where it was found
	4. Importance level (critical/important/minor)
	5. Associated characters (who owns/uses it)
	6. Any special properties or abilities
	7. Historical or cultural significance
	
	Focus on items that are plot-relevant, have symbolic meaning, or play a role in character development.`, title, truncateText(text, 5000))

		systemPrompt = `You are a professional item analysis expert specializing in identifying story-relevant objects and artifacts. Extract detailed information about each item's role in the narrative, its symbolic meaning, and practical importance.`
	} else {
		prompt = fmt.Sprintf(`分析以下标题为《%s》的文本，提取所有重要物品信息:
	
	%s
	
	请识别文本中提到的所有重要物品，提供以下信息：
	1. 物品名称和外观描述
	2. 在故事中的功能或用途
	3. 当前位置或发现地点
	4. 重要性等级（关键/重要/次要）
	5. 相关角色（谁拥有/使用它）
	6. 任何特殊属性或能力
	7. 历史或文化意义
	
	重点关注与情节相关、具有象征意义或在角色发展中发挥作用的物品。`, title, truncateText(text, 5000))

		systemPrompt = `你是一个专业的物品分析专家，专门识别故事相关的物体和文物。提取每个物品在叙事中的作用、象征意义和实际重要性的详细信息。`
	}

	// 使用结构化输出API获取响应
	request := llm.CompletionRequest{
		Model:        s.LLMService.GetDefaultModel(),
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
		MaxTokens:    2000,
		Temperature:  0.2,
	}

	// 尝试从缓存获取
	cacheKey := s.LLMService.GenerateCacheKey(request)
	var response *llm.CompletionResponse
	if cachedResp := s.LLMService.CheckCache(cacheKey); cachedResp != nil {
		response = cachedResp
	} else {
		// 执行API调用
		var err error
		response, err = s.LLMService.provider.CompleteText(context.Background(), request)
		if err != nil {
			return nil, err
		}
		// 添加到缓存
		s.LLMService.AddToCache(cacheKey, response)
	}
	// 尝试解析为数组格式
	var itemInfos []ItemInfo
	err := json.Unmarshal([]byte(response.Text), &itemInfos)
	if err == nil {
		// 数组解析成功
		var items []models.Item
		for _, info := range itemInfos {
			item := models.Item{
				Name:        info.Name,
				Description: info.Description,
				Location:    info.Location,
			}
			items = append(items, item)
		}
		return items, nil
	}

	// 如果解析数组失败，尝试解析为单个对象
	var singleItem ItemInfo
	err = json.Unmarshal([]byte(response.Text), &singleItem)
	if err != nil {
		return nil, fmt.Errorf("解析AI响应为结构化数据失败: %w\nAI返回: %s",
			err, truncateText(response.Text, 120))
	}

	// 将单个对象添加到数组中
	return []models.Item{
		{
			Name:        singleItem.Name,
			Description: singleItem.Description,
			Location:    singleItem.Location,
		},
	}, nil
}

// 生成故事摘要
func (s *AnalyzerService) generateSummary(text, title string) (string, error) {
	type SummaryResponse struct {
		Summary string `json:"summary"`
	}

	var response SummaryResponse

	// 检测文本语言
	isEnglish := isEnglishText(text + " " + title)

	var prompt, systemPrompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`Create a concise summary for the following text titled "%s":

%s

The summary should be brief and capture the main plot, characters, and themes of the story.`, title, truncateText(text, 5000))

		systemPrompt = `You are a professional literary summary expert, skilled at creating concise yet comprehensive summaries for stories.`
	} else {
		// 中文提示词（原有逻辑）
		prompt = fmt.Sprintf(`为以下标题为《%s》的文本创建一个简洁的摘要:

%s

摘要应该简明扼要，捕捉故事的主要情节、角色和主题。`, title, truncateText(text, 5000))

		systemPrompt = `你是一个专业的文学摘要专家，擅长为故事创建简明而全面的摘要。`
	}

	err := s.LLMService.CreateStructuredCompletion(context.Background(), prompt, systemPrompt, &response)
	if err != nil {
		return "", err
	}

	return response.Summary, nil
}

// 辅助函数，保持文本长度在限制范围内
func truncateText(text string, maxLength int) string {
	// 处理边界情况
	if maxLength <= 0 {
		return "..."
	}

	if len(text) == 0 {
		return ""
	}

	// 将字符串转换为符文(rune)数组，以正确处理中文等多字节字符
	runes := []rune(text)
	if len(runes) <= maxLength {
		return text
	}

	// 确保截断长度不会超出范围
	if maxLength > len(runes) {
		maxLength = len(runes)
	}

	// 截取指定长度的符文，然后添加省略号
	return string(runes[:maxLength]) + "..."
}

// AnalyzeTextWithProgress 带进度反馈和超时控制的文本分析
func (s *AnalyzerService) AnalyzeTextWithProgress(ctx context.Context, text string, tracker *ProgressTracker) (*models.AnalysisResult, error) {
	// 获取并发许可
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	// 检查context是否已经取消
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// 使用子context和timeout
	analyzeCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	result := &models.AnalysisResult{
		Title:      "分析中...",
		Characters: make([]models.Character, 0), // 与AnalyzeText函数保持一致的类型
		Scenes:     []models.Scene{},            // 使用正确的字段名称
		Items:      []models.Item{},
	}

	// 步骤1: 初步文本分析 (10%)
	tracker.UpdateProgress(10, "初步分析文本内容...")
	if err := s.preliminaryAnalysis(analyzeCtx, text, result); err != nil {
		return nil, fmt.Errorf("初步文本分析失败: %w", err)
	}

	// 检查是否已取消
	if analyzeCtx.Err() != nil {
		return nil, analyzeCtx.Err()
	}

	// 步骤2: 提取场景信息 (30%)
	tracker.UpdateProgress(30, "提取场景信息...")
	if err := s.extractSceneInfo(analyzeCtx, text, result); err != nil {
		return nil, fmt.Errorf("提取场景信息失败: %w", err)
	}

	// 检查是否已取消
	if analyzeCtx.Err() != nil {
		return nil, analyzeCtx.Err()
	}

	// 步骤3: 角色识别与分析 (60%)
	tracker.UpdateProgress(60, "识别和分析角色...")
	if err := s.extractCharacterInfo(analyzeCtx, text, result); err != nil {
		return nil, fmt.Errorf("角色分析失败: %w", err)
	}

	// 检查是否已取消
	if analyzeCtx.Err() != nil {
		return nil, analyzeCtx.Err()
	}

	// 步骤4: 构建角色关系 (80%)
	tracker.UpdateProgress(80, "构建角色关系网络...")
	if err := s.buildCharacterRelationships(analyzeCtx, result.Characters); err != nil {
		return nil, fmt.Errorf("构建角色关系失败: %w", err)
	}

	// 步骤5: 完成分析 (95%)
	tracker.UpdateProgress(95, "完成分析，准备结果...")

	// 执行最终处理...

	// 任务完成
	tracker.Complete("分析成功完成")

	return result, nil
}

// preliminaryAnalysis 执行文本的初步分析，设置基本属性
func (s *AnalyzerService) preliminaryAnalysis(ctx context.Context, text string, result *models.AnalysisResult) error {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 继续执行
	}

	// 分析文本长度和复杂度
	textLength := len(text)
	result.TextLength = textLength

	// 设置初步标题（如果为空）
	if result.Title == "分析中..." && textLength > 0 {
		// 截取前30个字符作为临时标题基础
		titleBase := text
		if len(text) > 30 {
			titleBase = text[:30]
		}

		// 根据文本语言设置不同的标题格式
		if isEnglishText(titleBase) {
			result.Title = "Analysis of \"" + strings.TrimSpace(titleBase) + "...\""
		} else {
			result.Title = "《" + strings.TrimSpace(titleBase) + "...》的分析"
		}
	}

	// 检测文本语言
	isEnglish := isEnglishText(text)

	// 扩展类型分析结构，增加更多信息
	type EnhancedTypeAnalysis struct {
		Type             string   `json:"type"`              // 文本类型
		Themes           []string `json:"themes"`            // 主要主题
		GenreAttributes  []string `json:"genre_attributes"`  // 体裁特性
		Mood             string   `json:"mood"`              // 整体情感基调
		WritingStyle     string   `json:"writing_style"`     // 写作风格
		TargetAudience   string   `json:"target_audience"`   // 目标受众
		EstimatedEra     string   `json:"estimated_era"`     // 估计创作年代/时期
		KeyElements      []string `json:"key_elements"`      // 关键元素
		LanguageFeatures string   `json:"language_features"` // 语言特点
	}

	var typeInfo EnhancedTypeAnalysis
	// 使用较短的超时，因为这只是初步分析
	typeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var textTypePrompt, systemPrompt string

	if isEnglish {
		textTypePrompt = fmt.Sprintf(`Conduct a comprehensive literary analysis of the following text excerpt:
	
	%s
	
	Provide detailed analysis including:
	1. **Text Type**: Specific genre classification (e.g., "gothic novel excerpt", "science fiction short story", "historical drama", "contemporary romance")
	2. **Primary Themes**: Core thematic elements and their significance
	3. **Genre Attributes**: Distinctive features that define the genre
	4. **Mood/Tone**: Emotional atmosphere and authorial attitude
	5. **Writing Style**: Narrative techniques, prose style, and literary devices
	6. **Target Audience**: Intended readership demographics and interests
	7. **Estimated Era**: Historical period of writing or setting, with reasoning
	8. **Key Elements**: Notable plot devices, character archetypes, or structural features
	9. **Language Features**: Distinctive vocabulary, syntax, or stylistic choices
	
	Provide specific examples from the text to support your analysis.`, truncateText(text, 1000))

		systemPrompt = `You are a distinguished literary scholar with expertise in genre analysis, stylistic criticism, and textual interpretation. Your analysis should be academically rigorous yet accessible, providing specific textual evidence for your conclusions.`
	} else {
		textTypePrompt = fmt.Sprintf(`对以下文本片段进行全面的文学分析:
	
	%s
	
	请提供详细分析，包括：
	1. **文本类型**：具体的体裁分类（如"哥特式小说片段"、"科幻短篇小说"、"历史剧本"、"当代言情小说"）
	2. **主要主题**：核心主题元素及其意义
	3. **体裁特性**：定义该体裁的独特特征
	4. **情感基调**：情感氛围和作者态度
	5. **写作风格**：叙事技巧、散文风格和文学手法
	6. **目标受众**：预期读者群体和兴趣偏好
	7. **估计年代**：写作或背景的历史时期，并说明理由
	8. **关键元素**：值得注意的情节设计、角色原型或结构特征
	9. **语言特点**：独特的词汇、句法或风格选择
	
	请从文本中提供具体例证来支持您的分析。`, truncateText(text, 1000))

		systemPrompt = `你是一位杰出的文学学者，在体裁分析、风格批评和文本解读方面具有专业知识。你的分析应该既具有学术严谨性又通俗易懂，为你的结论提供具体的文本证据。`
	}

	err := s.LLMService.CreateStructuredCompletion(
		typeCtx,
		textTypePrompt,
		systemPrompt,
		&typeInfo,
	)

	if err != nil {
		// 初步分析失败不是致命错误，记录错误但继续
		fmt.Printf("文本类型识别失败: %v\n", err)
	} else {
		// 设置文本类型和主题
		result.TextType = typeInfo.Type
		result.Themes = typeInfo.Themes

		// 扩展结果对象以存储更多分析数据
		if result.Metadata == nil {
			result.Metadata = make(map[string]interface{})
		}

		// 存储增强的分析结果
		result.Metadata["genre_attributes"] = typeInfo.GenreAttributes
		result.Metadata["mood"] = typeInfo.Mood
		result.Metadata["writing_style"] = typeInfo.WritingStyle
		result.Metadata["target_audience"] = typeInfo.TargetAudience
		result.Metadata["estimated_era"] = typeInfo.EstimatedEra
		result.Metadata["key_elements"] = typeInfo.KeyElements
		result.Metadata["language_features"] = typeInfo.LanguageFeatures
	}

	// 添加语言检测结果
	result.Metadata["is_english"] = isEnglish

	// 进行简单情感分析，确定文本的主要情感色彩
	// 这可以作为单独函数或集成到上述分析中
	if len(text) > 0 && typeInfo.Mood == "" {
		moodCtx, moodCancel := context.WithTimeout(ctx, 15*time.Second)
		defer moodCancel()

		type MoodAnalysis struct {
			PrimaryMood    string   `json:"primary_mood"`
			SecondaryMoods []string `json:"secondary_moods"`
			EmotionalTone  string   `json:"emotional_tone"`
		}

		var moodInfo MoodAnalysis
		var moodPrompt string

		if isEnglish {
			moodPrompt = fmt.Sprintf(`Analyze the emotional tone and mood of the following text:

%s

Identify the primary mood, any secondary moods, and overall emotional tone.`, truncateText(text, 800))
		} else {
			moodPrompt = fmt.Sprintf(`分析以下文本的情感基调和氛围:

%s

识别主要情感氛围、次要情感以及整体情感基调。`, truncateText(text, 800))
		}

		// 使用短超时进行情感分析
		if err := s.LLMService.CreateStructuredCompletion(
			moodCtx,
			moodPrompt,
			systemPrompt,
			&moodInfo,
		); err == nil {
			// 存储情感分析结果
			result.Metadata["primary_mood"] = moodInfo.PrimaryMood
			result.Metadata["secondary_moods"] = moodInfo.SecondaryMoods
			result.Metadata["emotional_tone"] = moodInfo.EmotionalTone
		}
	}

	// 如果文本较长，尝试提取重要人名和地点
	if len(text) > 1000 {
		namesCtx, namesCancel := context.WithTimeout(ctx, 15*time.Second)
		defer namesCancel()

		type NamedEntities struct {
			Characters []string `json:"characters"`
			Locations  []string `json:"locations"`
			TimeFrames []string `json:"time_frames"`
		}

		var entities NamedEntities
		var entitiesPrompt string

		if isEnglish {
			entitiesPrompt = fmt.Sprintf(`Extract key named entities from the following text:

%s

List main character names, locations, and any time frames or periods mentioned.`, truncateText(text, 800))
		} else {
			entitiesPrompt = fmt.Sprintf(`从以下文本中提取关键命名实体:

%s

列出主要角色名称、地点以及提到的任何时间框架或时期。`, truncateText(text, 800))
		}

		// 实体提取
		if err := s.LLMService.CreateStructuredCompletion(
			namesCtx,
			entitiesPrompt,
			systemPrompt,
			&entities,
		); err == nil {
			// 存储实体提取结果
			result.Metadata["preliminary_characters"] = entities.Characters
			result.Metadata["preliminary_locations"] = entities.Locations
			result.Metadata["time_frames"] = entities.TimeFrames
		}
	}

	return nil
}

// extractSceneInfo 提取场景信息，支持上下文控制和进度反馈
func (s *AnalyzerService) extractSceneInfo(ctx context.Context, text string, result *models.AnalysisResult) error {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 继续执行
	}

	// 使用现有的场景提取功能，但传入上下文
	sceneInfos, err := s.LLMService.ExtractScenes(ctx, text, result.Title)
	if err != nil {
		return err
	}

	// 转换为模型格式
	var scenes []models.Scene
	for _, info := range sceneInfos {
		scene := models.Scene{
			Name:        info.Name,
			Description: info.Description,
			Atmosphere:  info.Atmosphere,
			Era:         info.Era,    // 确保包含时代信息
			Themes:      info.Themes, // 确保包含主题信息
		}

		// 处理物品列表
		var items []models.Item
		if info.Items != nil {
			for _, itemName := range info.Items {
				if itemName != "" {
					items = append(items, models.Item{
						Name:        itemName,
						Description: "场景中的物品",
					})
				}
			}
		}
		scene.Items = items

		scenes = append(scenes, scene)
	}

	// 更新结果对象
	result.Scenes = scenes

	// 如果有场景，更新标题
	if len(scenes) > 0 && result.Title == "分析中..." {
		result.Title = scenes[0].Name
	}

	return nil
}

// extractCharacterInfo 提取角色信息，支持上下文控制和进度反馈
func (s *AnalyzerService) extractCharacterInfo(ctx context.Context, text string, result *models.AnalysisResult) error {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 继续执行
	}

	// 使用LLMService的结构化输出功能，但传入上下文
	characterInfos, err := s.LLMService.ExtractCharacters(ctx, text, result.Title)
	if err != nil {
		return err
	}

	// 转换为模型格式
	var characters []models.Character
	for _, info := range characterInfos {
		character := models.Character{
			Name:        info.Name,
			Role:        info.Role,
			Description: info.Description,
			Personality: info.Personality,
			Background:  info.Background,
			SpeechStyle: info.SpeechStyle,
			Knowledge:   info.Knowledge,
		}

		// 处理关系
		relationships := make(map[string]string)
		for name, relation := range info.Relationships {
			relationships[name] = relation
		}
		character.Relationships = relationships

		characters = append(characters, character)
	}

	// 保存到结果对象
	result.Characters = characters

	return nil
}

// buildCharacterRelationships 构建和增强角色之间的关系网络
func (s *AnalyzerService) buildCharacterRelationships(ctx context.Context, characters []models.Character) error {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 继续执行
	}

	// 如果角色太少，不需要额外处理
	if len(characters) <= 1 {
		return nil
	}

	// 准备角色关系网络分析的输入数据
	type RelationshipInput struct {
		Name        string            `json:"name"`
		Role        string            `json:"role"`
		Description string            `json:"description,omitempty"`
		Relations   map[string]string `json:"relations"`
	}

	inputs := make([]RelationshipInput, len(characters))
	for i, char := range characters {
		inputs[i] = RelationshipInput{
			Name:        char.Name,
			Role:        char.Role,
			Description: char.Description,
			Relations:   char.Relationships,
		}
	}

	// 创建关系网络分析的请求
	type RelationshipOutput struct {
		Characters []struct {
			Name      string            `json:"name"`
			Relations map[string]string `json:"relations"`
		} `json:"characters"`
	}

	// 准备提示词
	inputJSON, _ := json.Marshal(inputs)

	// 检测输入语言（基于角色名称和描述）
	var textSample strings.Builder
	for _, char := range characters {
		textSample.WriteString(char.Name + " " + char.Description + " ")
	}
	isEnglish := isEnglishText(textSample.String())

	var prompt, systemPrompt string

	if isEnglish {
		prompt = fmt.Sprintf(`Analyze the following character information and enhance the relationship network:
	
	%s
	
	Please ensure comprehensive relationship mapping with the following requirements:
	1. **Bidirectional Consistency**: If A is B's father, then B should be A's child
	2. **Relationship Depth**: Specify the nature and quality of relationships (close/distant, positive/negative/neutral)
	3. **Inferred Relationships**: Based on character descriptions, infer logical relationships between characters who haven't directly interacted
	4. **Relationship Evolution**: Consider how relationships might change throughout the story
	5. **Power Dynamics**: Identify hierarchical relationships and social positions
	6. **Emotional Bonds**: Distinguish between formal relationships and emotional connections
	7. **Conflict Relationships**: Identify antagonistic or competitive relationships
	
	For each character, provide their relationship with ALL other characters, even if it's "stranger" or "no direct relationship".`, string(inputJSON))

		systemPrompt = `You are an expert in character relationship analysis and social network mapping for literature. Your analysis should create a comprehensive, psychologically realistic relationship matrix that enhances story understanding and character development potential.`
	} else {
		prompt = fmt.Sprintf(`分析以下角色信息，完善角色之间的关系网络:
	
	%s
	
	请确保全面的关系映射，满足以下要求：
	1. **双向一致性**：如果A是B的父亲，那么B应该是A的孩子
	2. **关系深度**：指明关系的性质和质量（亲密/疏远，正面/负面/中性）
	3. **推断关系**：基于角色描述，推断没有直接互动的角色之间的逻辑关系
	4. **关系演变**：考虑关系在故事中可能的变化
	5. **权力动态**：识别等级关系和社会地位
	6. **情感纽带**：区分正式关系和情感联系
	7. **冲突关系**：识别对抗性或竞争性关系
	
	对于每个角色，提供他们与所有其他角色的关系，即使是"陌生人"或"无直接关系"。`, string(inputJSON))

		systemPrompt = `你是文学中角色关系分析和社交网络映射的专家。你的分析应该创建一个全面的、心理学上现实的关系矩阵，增强故事理解和角色发展潜力。`
	}
	var output RelationshipOutput
	if err := s.LLMService.CreateStructuredCompletion(ctx, prompt, systemPrompt, &output); err != nil {
		return fmt.Errorf("分析角色关系失败: %w", err)
	}

	// 更新角色关系
	for i := range characters {
		for _, enhancedChar := range output.Characters {
			if characters[i].Name == enhancedChar.Name {
				// 只更新关系，保持其他字段不变
				if len(enhancedChar.Relations) > 0 {
					characters[i].Relationships = enhancedChar.Relations
				}
				break
			}
		}
	}

	return nil
}

// 🔧 生成缓存键
func (s *AnalyzerService) generateCacheKey(text, title string) string {
	h := md5.New()
	h.Write([]byte(text + "|" + title))
	return hex.EncodeToString(h.Sum(nil))
}

// 🔧 检查缓存
func (s *AnalyzerService) checkAnalysisCache(cacheKey string) *models.AnalysisResult {
	s.analysisCache.mutex.RLock()
	defer s.analysisCache.mutex.RUnlock()

	if cached, exists := s.analysisCache.cache[cacheKey]; exists {
		if time.Since(cached.Timestamp) < s.analysisCache.expiration {
			return cached.Result
		}
		// 过期，删除
		delete(s.analysisCache.cache, cacheKey)
	}

	return nil
}

// 🔧 添加到缓存
func (s *AnalyzerService) addToAnalysisCache(cacheKey string, result *models.AnalysisResult) {
	s.analysisCache.mutex.Lock()
	defer s.analysisCache.mutex.Unlock()

	s.analysisCache.cache[cacheKey] = &CachedAnalysis{
		Result:    result,
		Timestamp: time.Now(),
	}
}
