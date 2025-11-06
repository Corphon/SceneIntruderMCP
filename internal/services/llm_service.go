// internal/services/llm_service.go
package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/llm"
)

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// LLMService 提供统一的大语言模型调用接口
type LLMService struct {
	providerMutex sync.RWMutex
	provider      llm.Provider
	providerName  string
	cache         *LLMCache
	isReady       bool
	readyState    string
}
type LLMCache struct {
	cache      map[string]*CacheEntry
	mutex      sync.RWMutex
	expiration time.Duration
}

type CacheEntry struct {
	Response  interface{}
	CreatedAt time.Time
}

// ChatCompletionRequest 兼容旧的请求格式
type ChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	Temperature float64                 `json:"temperature"`
	MaxTokens   int                     `json:"max_tokens"`
	ExtraParams map[string]interface{}  `json:"extra_params,omitempty"`
}

// ChatCompletionMessage 兼容旧的消息格式
type ChatCompletionMessage struct {
	Role    string
	Content string
}

// ChatCompletionResponse 兼容旧的响应格式
type ChatCompletionResponse struct {
	ID      string
	Choices []ChatCompletionChoice
	Usage   Usage
}

// ChatCompletionChoice 兼容旧的选择格式
type ChatCompletionChoice struct {
	Message      ChatCompletionMessage
	FinishReason string
}

// Usage 兼容旧的用量统计
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// 提取语言检测辅助方法
type LanguageContext struct {
	IsEnglish bool
	MainText  string
}

// 以下是为各种服务定义的结构化输出类型-------------------
// CharacterInfo 角色信息
type CharacterInfo struct {
	Name          string            `json:"name"`
	Role          string            `json:"role"`
	Description   string            `json:"description"`
	Personality   string            `json:"personality"`
	Background    string            `json:"background"`
	SpeechStyle   string            `json:"speech_style"`
	Relationships map[string]string `json:"relationships"`
	Knowledge     []string          `json:"knowledge"`
}

// SceneInfo 场景信息
type SceneInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Atmosphere  string   `json:"atmosphere"`
	Era         string   `json:"era"`
	Themes      []string `json:"themes"`
	Items       []string `json:"items"`
	Importance  string   `json:"importance"`
}

// ExplorationResult 探索结果
type ExplorationResult struct {
	DiscoveredItem *struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
	} `json:"discovered_item,omitempty"`
	StoryEvent *struct {
		Content string `json:"content"`
		Type    string `json:"type"`
		Choices []struct {
			Text        string `json:"text"`
			Consequence string `json:"consequence"`
		} `json:"choices,omitempty"`
	} `json:"story_event,omitempty"`
	NewClue string `json:"new_clue,omitempty"`
}

// -----------------------------------------
// NewLLMService 创建一个新的LLM服务
func NewLLMService() (*LLMService, error) {
	service := createBaseLLMService()

	// 尝试从配置初始化
	cfg := config.GetCurrentConfig()
	if cfg == nil {
		service.readyState = "无法获取配置"
		return service, nil
	}

	if cfg.LLMProvider == "" || (cfg.LLMConfig != nil && cfg.LLMConfig["api_key"] == "") {
		service.readyState = "未配置API密钥"
		return service, nil
	}

	// 尝试初始化提供商
	provider, err := llm.GetProvider(cfg.LLMProvider, cfg.LLMConfig)
	if err != nil {
		service.readyState = fmt.Sprintf("初始化失败: %v", err)
		return service, nil // 返回未就绪服务而不是错误
	}

	// 初始化成功
	service.provider = provider
	service.providerName = cfg.LLMProvider
	service.isReady = true
	service.readyState = "已就绪"

	return service, nil
}

// NewEmptyLLMService 创建一个空的LLM服务实例作为后备方案
func NewEmptyLLMService() *LLMService {
	service := createBaseLLMService()
	service.providerName = "empty"
	service.readyState = "后备服务模式 - 请在设置中配置API密钥"
	return service
}

// createBaseLLMService 创建基础LLM服务实例
func createBaseLLMService() *LLMService {
	return &LLMService{
		provider:     nil,
		providerName: "",
		isReady:      false,
		readyState:   "未初始化",
		cache: &LLMCache{
			cache:      make(map[string]*CacheEntry),
			mutex:      sync.RWMutex{},
			expiration: 30 * time.Minute,
		},
	}
}

// IsReady 返回服务是否已就绪
func (s *LLMService) IsReady() bool {
	s.providerMutex.RLock()
	defer s.providerMutex.RUnlock()
	return s.isReady
}

// GetReadyState 返回服务就绪状态描述
func (s *LLMService) GetReadyState() string {
	s.providerMutex.RLock()
	defer s.providerMutex.RUnlock()
	return s.readyState
}

// UpdateProvider 更新LLM服务的提供商
func (s *LLMService) UpdateProvider(providerName string, config map[string]string) error {
	provider, err := llm.GetProvider(providerName, config)
	if err != nil {
		s.providerMutex.Lock()
		s.isReady = false
		s.readyState = fmt.Sprintf("配置失败: %v", err)
		s.providerMutex.Unlock()
		return err
	}

	s.providerMutex.Lock()
	defer s.providerMutex.Unlock()

	s.provider = provider
	s.providerName = providerName
	s.isReady = true
	s.readyState = "已就绪"

	// 清理缓存
	s.cache = &LLMCache{
		cache:      make(map[string]*CacheEntry),
		mutex:      sync.RWMutex{},
		expiration: 30 * time.Minute,
	}

	return nil
}

// generateCacheKey 生成缓存键
func (s *LLMService) generateCacheKey(prompt, systemPrompt, model string) string {
	s.providerMutex.RLock()
	providerName := s.providerName
	s.providerMutex.RUnlock()

	hashInput := fmt.Sprintf("%s:::%s:::%s:::%s",
		prompt, systemPrompt, model, providerName)
	h := md5.New()
	h.Write([]byte(hashInput))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// getFromCache 从缓存中获取结果
func (c *LLMCache) getFromCache(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Since(entry.CreatedAt) > c.expiration {
		return nil, false
	}

	return entry.Response, true
}

// saveToCache 保存结果到缓存
func (c *LLMCache) saveToCache(key string, response interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache[key] = &CacheEntry{
		Response:  response,
		CreatedAt: time.Now(),
	}

	// 如果缓存太大，可以考虑清理最旧的条目
	if len(c.cache) > 1000 {
		c.cleanupOldest(100) // 清理100个最旧的条目
	}
}

// cleanupOldest 清理最旧的缓存条目
func (c *LLMCache) cleanupOldest(count int) {
	type keyAge struct {
		key string
		age time.Time
	}

	entries := make([]keyAge, 0, len(c.cache))
	for k, v := range c.cache {
		entries = append(entries, keyAge{k, v.CreatedAt})
	}

	// 按创建时间排序
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].age.Before(entries[j].age)
	})

	// 删除最旧的条目
	maxToDelete := min(count, len(entries))
	for i := 0; i < maxToDelete; i++ {
		delete(c.cache, entries[i].key)
	}
}

// 兼容旧的CreateChatCompletion方法
func (s *LLMService) CreateChatCompletion(ctx context.Context, request ChatCompletionRequest) (ChatCompletionResponse, error) {
	// 构建系统和用户提示
	var systemContent, userContent string
	var assistantMessages []string
	for _, msg := range request.Messages {
		switch msg.Role {
		case RoleSystem:
			systemContent = msg.Content
		case RoleUser:
			userContent = msg.Content
		case RoleAssistant:
			assistantMessages = append(assistantMessages, msg.Content)
		default:
			log.Printf("警告: 未知的消息角色类型: %s", msg.Role)
		}
	}

	// 助手消息历史，将其整合到用户提示中
	if len(assistantMessages) > 0 {
		conversationHistory := strings.Join(assistantMessages, "\n\n")
		userContent = fmt.Sprintf("对话历史:\n%s\n\n当前用户输入: %s",
			conversationHistory, userContent)
	}

	// 生成缓存键
	cacheKey := s.generateCacheKey(userContent, systemContent, request.Model)

	// 检查缓存
	if s.cache != nil {
		var cachedResult ChatCompletionResponse
		if s.checkAndUseCache(cacheKey, &cachedResult) {
			log.Printf("DEBUG:LLM聊天缓存命中: %s", cacheKey[:8])
			return cachedResult, nil
		}
	}

	// 转换请求格式
	req := llm.CompletionRequest{
		Model:       request.Model,
		Temperature: float32(request.Temperature),
		MaxTokens:   request.MaxTokens,
	}

	req.SystemPrompt = systemContent
	req.Prompt = userContent

	// 调用实际Provider
	resp, err := s.provider.CompleteText(ctx, req)
	if err != nil {
		return ChatCompletionResponse{}, err
	}

	// 转换为旧格式的响应
	result := ChatCompletionResponse{
		ID: resp.ModelName + "-" + s.providerName,
		Choices: []ChatCompletionChoice{
			{
				Message: ChatCompletionMessage{
					Role:    "assistant",
					Content: resp.Text,
				},
				FinishReason: resp.FinishReason,
			},
		},
		Usage: Usage{
			PromptTokens:     resp.PromptTokens,
			CompletionTokens: resp.OutputTokens,
			TotalTokens:      resp.TokensUsed,
		},
	}

	// 保存到缓存
	if s.cache != nil {
		s.saveToCache(cacheKey, result)
		log.Printf("DEBUG:保存到LLM聊天缓存: %s", cacheKey[:8])
	}

	return result, nil
}

// 增强版ChatCompletion: 支持结构化输出
// 🔧 优化后的 CreateStructuredCompletion
func (s *LLMService) CreateStructuredCompletion(ctx context.Context, prompt string, systemPrompt string, outputSchema interface{}) error {
	// 获取默认模型（线程安全）
	s.providerMutex.RLock()
	if !s.isReady || s.provider == nil {
		s.providerMutex.RUnlock()
		return fmt.Errorf("LLM服务未就绪: %s", s.readyState)
	}

	models := s.provider.GetSupportedModels()
	model := "gpt-4o" // 默认兜底值
	if len(models) > 0 {
		model = models[0]
	}
	provider := s.provider
	s.providerMutex.RUnlock()

	// 生成缓存键
	cacheKey := s.generateCacheKey(prompt, systemPrompt, model)

	// 检查缓存
	if s.checkAndUseCache(cacheKey, outputSchema) {
		return nil
	}

	// 修改系统提示以请求特定格式
	structuredSystemPrompt := systemPrompt
	if systemPrompt != "" {
		structuredSystemPrompt += "\n\n"
	}
	structuredSystemPrompt += "以有效的JSON格式返回您的响应，遵循提供的输出架构，不要添加解释或前言。"

	req := llm.CompletionRequest{
		Prompt:       prompt,
		SystemPrompt: structuredSystemPrompt,
		Temperature:  0.3,
		Model:        model,
	}

	// 调用实际Provider
	resp, err := provider.CompleteText(ctx, req)
	if err != nil {
		return err
	}

	// 尝试解析结构化输出
	text := cleanJSONString(resp.Text)

	// 解析JSON到提供的结构中
	err = json.Unmarshal([]byte(text), outputSchema)
	if err != nil {
		return fmt.Errorf("解析AI响应为结构化数据失败: %w\nAI返回: %s", err, text)
	}

	// 保存到缓存
	s.saveToCache(cacheKey, outputSchema)

	return nil
}

// 清理JSON字符串，去除前后非JSON内容
func cleanJSONString(s string) string {
	// 查找第一个{
	start := strings.Index(s, "{")
	if start == -1 {
		start = strings.Index(s, "[")
		if start == -1 {
			return s // 没有找到JSON开始标记
		}
	}

	// 查找最后一个}
	end := strings.LastIndex(s, "}")
	if end == -1 {
		end = strings.LastIndex(s, "]")
		if end == -1 {
			return s // 没有找到JSON结束标记
		}
		end++
	} else {
		end++
	}

	// 提取JSON部分
	if start < end {
		return s[start:end]
	}
	return s
}

// GenerateScenarioIdeas 根据初始概念生成场景创意
func (s *LLMService) GenerateScenarioIdeas(ctx context.Context, concept string, genre string, complexity string) (*ScenarioIdeas, error) {
	result := &ScenarioIdeas{}

	// 检测输入是否主要是英文
	isEnglish := isEnglishText(concept) || isEnglishText(genre)

	var systemPrompt, prompt string

	if isEnglish {
		// ✅ 优化后的英文提示词
		systemPrompt = `You are a creative story concept specialist and world-building expert, skilled at crafting compelling, immersive scenarios for interactive stories and games.
Your scenarios should balance originality with player accessibility, offering multiple narrative paths and meaningful choices that reflect the intended genre and complexity level.`

		prompt = fmt.Sprintf(`Generate diverse scenario ideas for an interactive game or story based on the following parameters:

Concept: %s
Genre: %s
Complexity: %s

Create several distinct scenario concepts, each including:
1. Core premise and unique selling proposition
2. Primary character archetypes and their motivations
3. Central conflicts and tension sources
4. Key branching decision points that affect story outcomes
5. Atmospheric elements that reinforce the genre
6. Scalable complexity appropriate to the specified level
7. Potential for player agency and meaningful choices

Ensure each scenario offers rich possibilities for character development, plot progression, and player engagement while staying true to the specified genre conventions.`,
			concept, genre, complexity)
	} else {
		// ✅ 优化后的中文提示词
		systemPrompt = `你是一个创意故事构思专家和世界构建专家，擅长为交互式故事和游戏创造引人入胜、沉浸感强的场景。
你的场景应该平衡原创性与玩家可接受性，提供多条叙事路径和反映预期类型和复杂度的有意义选择。`

		prompt = fmt.Sprintf(`基于以下参数为交互式游戏或故事生成多样化的场景创意:

概念: %s
类型: %s
复杂度: %s

创造几个不同的场景概念，每个包括：
1. 核心前提和独特卖点
2. 主要角色原型及其动机驱动
3. 中心冲突和张力来源
4. 影响故事结局的关键分支决策点
5. 强化类型特色的氛围要素
6. 适应指定水平的可扩展复杂性
7. 玩家能动性和有意义选择的潜力

确保每个场景都为角色发展、情节推进和玩家参与提供丰富可能性，同时忠于指定类型的惯例。`,
			concept, genre, complexity)
	}

	err := s.CreateStructuredCompletion(ctx, prompt, systemPrompt, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// AnalyzeLocationContext 分析场景中的位置关系
func (s *LLMService) AnalyzeLocationContext(ctx context.Context, sceneName string, locations []LocationInfo) (*LocationAnalysis, error) {
	result := &LocationAnalysis{}

	// 将位置信息转换为JSON字符串
	locationsJSON, err := json.Marshal(locations)
	if err != nil {
		return nil, err
	}

	// 检测语言
	isEnglish := isEnglishText(sceneName)

	// 如果场景名称检测不确定，尝试从位置描述中检测
	if !isEnglish && len(locations) > 0 {
		// 合并所有位置名称和描述用于语言检测
		var locationTexts []string
		for _, loc := range locations {
			if loc.Name != "" {
				locationTexts = append(locationTexts, loc.Name)
			}
			if loc.Description != "" {
				locationTexts = append(locationTexts, loc.Description)
			}
		}

		combinedText := strings.Join(locationTexts, " ")
		if combinedText != "" {
			isEnglish = isEnglishText(combinedText)
		}
	}

	var systemPrompt, prompt string

	if isEnglish {
		// 英文提示词
		systemPrompt = `You are a spatial planning and story world building expert. Analyze the provided location information, infer spatial relationships between them, possible paths, and story potential.`

		prompt = fmt.Sprintf(`Analyze the following location information in the scene "%s":

%s

Please analyze:
1. Spatial relationships between locations and possible paths
2. Story function and importance of each location
3. Suggested exploration routes
4. Story flow and pacing recommendations`, sceneName, string(locationsJSON))
	} else {
		// 中文提示词
		systemPrompt = `你是一个空间规划和故事世界构建专家。分析提供的位置信息，推断它们之间的空间关系、可能的路径和故事潜力。`

		prompt = fmt.Sprintf(`分析以下场景"%s"中的位置信息:

%s

请分析:
1. 位置之间的空间关系和可能的路径
2. 每个位置的故事功能和重要性
3. 可能的探索路线建议
4. 故事流动和节奏建议`, sceneName, string(locationsJSON))
	}

	err = s.CreateStructuredCompletion(ctx, prompt, systemPrompt, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GenerateCharacterInteraction 生成角色互动内容
func (s *LLMService) GenerateCharacterInteraction(ctx context.Context, character1 CharacterInfo, character2 CharacterInfo, situation string) (*CharacterInteraction, error) {
	result := &CharacterInteraction{}

	// 将角色信息转换为JSON
	char1JSON, _ := json.Marshal(character1)
	char2JSON, _ := json.Marshal(character2)

	// 检测输入语言
	isEnglish := isEnglishText(character1.Name + " " + character2.Name + " " + situation)

	var systemPrompt, prompt string

	if isEnglish {
		// 英文提示词
		systemPrompt = `You are a dialogue and character interaction expert. Based on the provided character information and situation, create authentic, engaging interactions that match character traits.
Ensure the dialogue reflects each character's personality, speech style, and motivations.`

		prompt = fmt.Sprintf(`Create an interaction between the following two characters in the given situation:

Character 1: %s

Character 2: %s

Situation: %s

Generate a dialogue sequence with the following requirements:
1. Each character's dialogue should reflect their unique personality traits and speech patterns
2. The interaction should advance the plot or reveal character development
3. Include appropriate emotional expressions and subtle body language descriptions
4. Limit the conversation to 3-5 exchanges between characters
5. Ensure the dialogue maintains logical consistency with the given situation
6. Consider the relationship dynamics between the characters
7. If conflict arises, show how each character would realistically respond

Format the output to show clear speaker attribution and any relevant actions or emotional states.`, string(char1JSON), string(char2JSON), situation)
	} else {
		// 中文提示词（原有逻辑）
		systemPrompt = `你是一个对话和角色互动专家。根据提供的角色信息和情境，创造真实、有趣且符合角色特点的互动。
确保对话反映角色的性格、说话风格和动机。`

		prompt = fmt.Sprintf(`创建以下两个角色在给定情境下的互动:

角色1: %s

角色2: %s

情境: %s

请生成一段对话序列，要求如下：
1. 每个角色的对话应体现其独特的性格特征和说话方式
2. 互动应推进情节发展或揭示角色成长
3. 包含适当的情绪表达和细微的肢体语言描述
4. 角色间的对话交流限制在3-5轮以内
5. 确保对话与给定情境保持逻辑一致性
6. 考虑角色之间的关系动态
7. 如果出现冲突，展示每个角色会如何真实地回应

输出格式要显示清晰的说话者归属和任何相关的动作或情绪状态。`, string(char1JSON), string(char2JSON), situation)
	}

	err := s.CreateStructuredCompletion(ctx, prompt, systemPrompt, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetProvider 返回内部的Provider实例
func (s *LLMService) GetProvider() llm.Provider {
	s.providerMutex.RLock()
	defer s.providerMutex.RUnlock()
	return s.provider
}

// GetProviderName 返回当前LLM提供商名称
func (s *LLMService) GetProviderName() string {
	s.providerMutex.RLock()
	defer s.providerMutex.RUnlock()
	return s.providerName
}

// isEnglishText 检测文本是否为英文
func isEnglishText(text string) bool {
	if len(text) == 0 {
		return false
	}

	// 计数
	letterCount := 0
	chineseCount := 0
	totalValidChars := 0 // 有效字符总数

	for _, r := range text {
		// 英文字母
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			letterCount++
			totalValidChars++
		}
		// 检测中文字符
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseCount++
			totalValidChars++
		}
		// 数字也算有效字符
		if r >= '0' && r <= '9' {
			totalValidChars++
		}
	}

	// 判定规则：
	// 1. 如果没有有效字符，返回 false
	if totalValidChars == 0 {
		return false
	}

	// 2. 计算英文字母占有效字符的比例
	englishRatio := float64(letterCount) / float64(totalValidChars)

	// 3. 如果英文字母比例超过50%，认为是英文文本
	// 这样 "Mixed 中英文" 中的 "Mixed" 占主导，会被判定为英文
	return englishRatio > 0.5
}

// 用于结构化输出时抽取角色信息
func (s *LLMService) ExtractCharacters(ctx context.Context, text, title string) ([]CharacterInfo, error) {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// 继续执行
	}

	// 检测文本语言
	isEnglish := isEnglishText(text)

	var prompt, systemPrompt string
	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`Analyze the following text titled "%s" and extract all character information:

%s

Identify all possible characters from the text, including names, personality traits, appearance descriptions, etc. For each character, provide as detailed information as possible.`, title, text)

		systemPrompt = `You are a professional literary character analyst, skilled at extracting character information from texts. Extract all characters that appear in the text, including both main and supporting characters.`
	} else {
		// 中文提示词（原有逻辑）
		prompt = fmt.Sprintf(`分析以下标题为《%s》的文本，提取所有角色信息:

%s

从文本中识别所有可能的角色，包括名字、性格特点、外表描述等。对于每个角色，提供尽可能详细的信息。`, title, text)

		systemPrompt = `你是一个专业的文学角色分析专家，擅长提取文本中的人物信息。提取文本中出现的所有角色，包括主要和次要角色。`
	}

	// 使用结构化输出API
	request := llm.CompletionRequest{
		Model:        s.GetDefaultModel(),
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
		MaxTokens:    2000,
		Temperature:  0.2,
	}

	cacheKey := s.GenerateCacheKey(request)
	if cachedResp := s.CheckCache(cacheKey); cachedResp != nil {
		// 尝试解析为数组格式
		var characters []CharacterInfo
		err := json.Unmarshal([]byte(cachedResp.Text), &characters)
		if err == nil {
			return characters, nil
		}

		// 如果解析数组失败，尝试解析为单个对象
		var singleCharacter CharacterInfo
		err = json.Unmarshal([]byte(cachedResp.Text), &singleCharacter)
		if err != nil {
			return nil, fmt.Errorf("解析缓存的AI响应为结构化数据失败: %w\nAI返回: %s",
				err, truncateText(cachedResp.Text, 120))
		}

		// 将单个对象添加到数组中
		return []CharacterInfo{singleCharacter}, nil
	}

	response, err := s.provider.CompleteText(ctx, request)
	if err != nil {
		return nil, err
	}
	// 添加到缓存
	s.AddToCache(cacheKey, response)

	// 尝试解析为数组格式
	var characters []CharacterInfo
	err = json.Unmarshal([]byte(response.Text), &characters)
	if err == nil {
		return characters, nil
	}

	// 如果解析数组失败，尝试解析为单个对象
	var singleCharacter CharacterInfo
	err = json.Unmarshal([]byte(response.Text), &singleCharacter)
	if err != nil {
		return nil, fmt.Errorf("解析AI响应为结构化数据失败: %w\nAI返回: %s",
			err, truncateText(response.Text, 120))
	}

	// 将单个对象添加到数组中
	return []CharacterInfo{singleCharacter}, nil
}

// 用于提取场景信息
func (s *LLMService) ExtractScenes(ctx context.Context, text, title string) ([]SceneInfo, error) {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// 继续执行
	}

	// 检测文本语言
	isEnglish := isEnglishText(text)

	var prompt, systemPrompt string
	if isEnglish {
		prompt = fmt.Sprintf(`Analyze the following text titled "%s" and extract all scene information:

%s

Identify the main scenes in the text, including:
1. Scene name
2. Detailed description
3. Scene atmosphere
4. Time period/era
5. Thematic elements
6. Major items or props present in the scene`,
			title, truncateText(text, 5000))

		systemPrompt = `You are a professional scene analysis expert. Please extract key scene information from the text. Return in JSON format, including scene name, description, atmosphere, era, themes, and a list of items.`
	} else {
		// 原有中文提示词
		prompt = fmt.Sprintf(`分析以下标题为《%s》的文本，提取所有场景信息:

%s

识别文中的主要场景，包括:
1. 场景名称
2. 详细描述
3. 场景氛围
4. 时代背景
5. 主题元素
6. 场景中出现的主要物品`,
			title, truncateText(text, 5000))

		systemPrompt = `你是一个专业的场景分析专家，请从文本中提取关键场景信息。返回JSON格式，包含场景名称(name)、描述(description)、氛围(atmosphere)、时代背景(era)、主题(themes)和物品列表(items)。`
	}

	// 使用结构化输出API
	request := llm.CompletionRequest{
		Model:        s.GetDefaultModel(),
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
		MaxTokens:    2000,
		Temperature:  0.2,
	}

	cacheKey := s.GenerateCacheKey(request)
	if cachedResp := s.CheckCache(cacheKey); cachedResp != nil {
		// 尝试解析为数组格式
		var scenes []SceneInfo
		err := json.Unmarshal([]byte(cachedResp.Text), &scenes)
		if err == nil {
			return scenes, nil
		}

		// 如果解析数组失败，尝试解析为单个对象
		var singleScene SceneInfo
		err = json.Unmarshal([]byte(cachedResp.Text), &singleScene)
		if err != nil {
			return nil, fmt.Errorf("解析缓存的AI响应为结构化数据失败: %w\nAI返回: %s",
				err, truncateText(cachedResp.Text, 120))
		}

		// 将单个对象添加到数组中
		return []SceneInfo{singleScene}, nil
	}

	response, err := s.provider.CompleteText(ctx, request)
	if err != nil {
		return nil, err
	}
	// 添加到缓存
	s.AddToCache(cacheKey, response)

	// 尝试解析为数组格式
	var scenes []SceneInfo
	err = json.Unmarshal([]byte(response.Text), &scenes)
	if err == nil {
		return scenes, nil
	}

	// 如果解析数组失败，尝试解析为单个对象
	var singleScene SceneInfo
	err = json.Unmarshal([]byte(response.Text), &singleScene)
	if err != nil {
		return nil, fmt.Errorf("解析AI响应为结构化数据失败: %w\nAI返回: %s",
			err, truncateText(response.Text, 120))
	}

	// 将单个对象添加到数组中
	return []SceneInfo{singleScene}, nil
}

// GenerateCacheKey 为请求生成缓存键
func (s *LLMService) GenerateCacheKey(req llm.CompletionRequest) string {
	return s.generateCacheKey(req.Prompt, req.SystemPrompt, req.Model)
}

// CheckCache 检查并返回缓存的响应
func (s *LLMService) CheckCache(key string) *llm.CompletionResponse {
	if s.cache == nil {
		return nil
	}

	if entry, found := s.cache.getFromCache(key); found {
		if response, ok := entry.(*llm.CompletionResponse); ok {
			return response
		}
	}
	return nil
}

// AddToCache 添加响应到缓存
func (s *LLMService) AddToCache(key string, response *llm.CompletionResponse) {
	if s.cache != nil {
		s.cache.saveToCache(key, response)
	}
}

// AnalyzeContent 分析文本内容，提取关键信息
func (s *LLMService) AnalyzeContent(ctx context.Context, text string) (*ContentAnalysis, error) {
	result := &ContentAnalysis{}

	// 检测文本语言
	isEnglish := isEnglishText(text)

	var systemPrompt, prompt string
	if isEnglish {
		systemPrompt = `You are a professional literary analyst who extracts key information from texts, including characters, scenes, important props, and major plot points. Provide detailed and precise analysis, ensuring the result format meets requirements. Do not add explanations or preambles.`

		prompt = fmt.Sprintf(`Please analyze the following text and extract all key information:

%s

Please extract information in the following categories:
1. Characters: All characters that appear, including names, traits, and relationships
2. Scenes: All locations and scenes that appear, including descriptions and atmosphere
3. Props: Important items or props mentioned in the text, including usage methods and effects
4. Plot: Major plot points and events
5. Themes: Core themes or ideas the text may express`, text)
	} else {
		// 原有中文提示词
		systemPrompt = `你是一个专业的文学分析专家，需要从文本中提取关键信息，包括角色、场景、重要道具和主要情节点。
提供详细而精确的分析，确保结果格式符合要求。不要添加解释或前言。`

		prompt = fmt.Sprintf(`请分析以下文本，提取所有关键信息:

%s

请提取以下类别的信息:
1. 角色: 所有出现的角色，包括名称、特征和关系
2. 场景: 所有出现的地点和场景，包括描述和氛围
3. 道具: 文中提到的重要物品或道具，以及使用方法、效果
4. 情节: 主要情节点和事件
5. 主题: 文本可能表达的核心主题或思想`, text)
	}

	err := s.CreateStructuredCompletion(ctx, prompt, systemPrompt, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GenerateExplorationResult 用于探索地点时的结果分析
func (s *LLMService) GenerateExplorationResult(ctx context.Context, sceneName, locationName, locationDesc, sceneDesc, creativityLevel string, allowPlotTwists bool) (*ExplorationResult, error) {
	var result ExplorationResult

	// 检测场景描述语言
	isEnglish := isEnglishText(sceneDesc)

	var prompt, systemPrompt string
	if isEnglish {
		// ✅ 优化后的英文提示词
		systemPrompt = `You are a creative story designer and game master specializing in interactive fiction experiences.
Your role is to generate meaningful, contextually appropriate exploration results that enhance player engagement and narrative progression while maintaining consistency with the established world and atmosphere.`

		prompt = fmt.Sprintf(`In the scene "%s", the player is exploring the location "%s".

Location Description: %s
Scene Background: %s
Creativity Level: %s
Allow Plot Twists: %t

Generate exploration results following these guidelines:
1. If creativity level is "high", introduce unexpected discoveries or hidden elements that surprise the player
2. If plot twists are allowed, introduce new story threads, mysteries, or conflicts that deepen the narrative
3. Results must align with the location's characteristics and the overall scene atmosphere
4. Priority hierarchy: Significant story events > Useful items/tools > Background lore/clues
5. Ensure exploration results contribute to overall story progression and player agency
6. Consider environmental storytelling - what would realistically be found in this specific location?
7. Balance immediate rewards with long-term narrative payoffs

Generate 1-2 specific, detailed exploration results that feel organic to the world and situation.`,
			sceneName, locationName, locationDesc, sceneDesc, creativityLevel, allowPlotTwists)
	} else {
		// ✅ 优化后的中文提示词
		systemPrompt = `你是一个创意故事设计师和游戏主持人，专门设计互动小说体验。
你的任务是生成有意义、符合情境的探索结果，提升玩家参与度和叙事推进，同时保持与既定世界观和氛围的一致性。`

		prompt = fmt.Sprintf(`在《%s》这个场景中，玩家正在探索地点"%s"。

地点描述: %s
场景背景: %s
创意水平: %s
允许剧情转折: %t

根据以下准则生成探索结果：
1. 如果创意水平为"高"，引入令玩家惊喜的意外发现或隐藏要素
2. 如果允许剧情转折，引入深化叙事的新故事线索、谜团或冲突
3. 结果必须与地点特征和整体场景氛围保持一致
4. 优先级层次：重要故事事件 > 有用道具/工具 > 背景传说/线索
5. 确保探索结果能促进整体故事推进和玩家能动性
6. 考虑环境叙事——在这个特定地点现实中会发现什么？
7. 平衡即时奖励与长期叙事回报

生成1-2个具体、详细的探索结果，让它们感觉是这个世界和情境的有机组成部分。`,
			sceneName, locationName, locationDesc, sceneDesc, creativityLevel, allowPlotTwists)
	}

	// 使用CreateStructuredCompletion
	err := s.CreateStructuredCompletion(ctx, prompt, systemPrompt, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetDefaultModel 获取当前配置的默认模型
func (s *LLMService) GetDefaultModel() string {
	s.providerMutex.RLock()
	defer s.providerMutex.RUnlock()

	// 如果LLM服务不可用或未就绪，返回默认值
	if !s.isReady || s.provider == nil {
		return "gpt-4.1"
	}

	// 获取提供商支持的模型列表
	models := s.provider.GetSupportedModels()
	if len(models) > 0 {
		return models[0]
	}

	// 根据提供商名称返回默认模型
	defaultModels := map[string]string{
		"OpenAI":           "gpt-4.1",
		"Anthropic Claude": "claude-3.5-sonnet",
		"Mistral":          "mistral-large-latest",
		"DeepSeek":         "deepseek-chat",
		"GLM":              "glm-4",
		"google gemini":    "gemini-2.0-flash",
		"Qwen":             "qwen2.5-max",
		"GitHub Models":    "gpt-4.1",
		"Grok":             "grok-3",
		"openrouter":       "google/gemma-3-27b-it:free",
	}

	if model, exists := defaultModels[s.providerName]; exists {
		return model
	}

	return "gpt-4.1" // 兜底默认值
}

// 统一的缓存操作方法
func (s *LLMService) checkAndUseCache(cacheKey string, outputSchema interface{}) bool {
	if s.cache == nil {
		return false
	}

	if cachedResponse, found := s.cache.getFromCache(cacheKey); found {
		// 直接将缓存响应作为 JSON 字节处理
		if responseBytes, ok := cachedResponse.([]byte); ok {
			if outputSchema != nil {
				// 尝试将缓存的 JSON 字节反序列化到输出结构
				err := json.Unmarshal(responseBytes, outputSchema)
				if err == nil {
					log.Printf("DEBUG:LLM缓存命中: %s", cacheKey[:8])
					return true
				}
			}
		}
		// 如果缓存项不是字节切片，则尝试其他类型转换（向后兼容）
		if resp, ok := cachedResponse.(ChatCompletionResponse); ok {
			if outputSchema != nil {
				// 对于结构化输出，尝试 JSON 转换
				responseJSON, err := json.Marshal(resp)
				if err == nil {
					err = json.Unmarshal(responseJSON, outputSchema)
					if err == nil {
						log.Printf("DEBUG:LLM缓存命中: %s", cacheKey[:8])
						return true
					}
				}
			} else {
				// 对于普通响应，直接返回
				if chatResp, ok := outputSchema.(*ChatCompletionResponse); ok {
					*chatResp = resp
					log.Printf("DEBUG:LLM缓存命中: %s", cacheKey[:8])
					return true
				}
			}
		}

		// 尝试直接转换为 CompletionResponse
		if resp, ok := cachedResponse.(*llm.CompletionResponse); ok {
			if outputSchema != nil {
				err := json.Unmarshal([]byte(resp.Text), outputSchema)
				if err == nil {
					log.Printf("DEBUG:LLM缓存命中: %s", cacheKey[:8])
					return true
				}
			}
		}
	}

	return false
}

// 统一的缓存保存方法
func (s *LLMService) saveToCache(cacheKey string, response interface{}) {
	if s.cache != nil {
		// 总是将响应序列化为JSON字节存储，以确保一致的类型处理
		responseBytes, err := json.Marshal(response)
		if err != nil {
			log.Printf("ERROR:序列化缓存响应失败: %v", err)
			// 仍然尝试保存原始响应以向后兼容
			s.cache.saveToCache(cacheKey, response)
		} else {
			s.cache.saveToCache(cacheKey, responseBytes)
		}
		log.Printf("DEBUG:保存到LLM缓存: %s", cacheKey[:8])
	}
}
