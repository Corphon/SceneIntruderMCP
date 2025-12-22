// internal/services/llm_service.go
package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/llm"
	"github.com/Corphon/SceneIntruderMCP/internal/utils"
)

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

var ErrLLMNotReady = errors.New("llm service not ready")

var providerDefaultModels = map[string]string{
	"openai":       "gpt-4.1",
	"anthropic":    "claude-haiku-4.5",
	"mistral":      "mistral-large-latest",
	"deepseek":     "deepseek-chat",
	"glm":          "glm-4.5-air",
	"google":       "gemini-2.5-flash",
	"qwen":         "qwen3-max",
	"githubmodels": "gpt-4.1-mini",
	"grok":         "grok-4.1-fast",
	"openrouter":   "x-ai/grok-4.1-fast:free",
}

// LLMService 提供统一的大语言模型调用接口
type LLMService struct {
	providerMutex      sync.RWMutex
	provider           llm.Provider
	providerName       string
	cache              *LLMCache
	isReady            bool
	readyState         string
	activeDefaultModel string
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
		service.readyState = "Failed to retrieve configuration"
		return service, nil
	}

	if cfg.LLMProvider == "" || (cfg.LLMConfig != nil && cfg.LLMConfig["api_key"] == "") {
		service.readyState = "API key not configured"
		return service, nil
	}

	// 尝试初始化提供商
	provider, err := llm.GetProvider(cfg.LLMProvider, cfg.LLMConfig)
	if err != nil {
		service.readyState = fmt.Sprintf("Initialization failed: %v", err)
		return service, nil // 返回未就绪服务而不是错误
	}

	// 初始化成功
	service.provider = provider
	service.providerName = cfg.LLMProvider
	service.activeDefaultModel = extractDefaultModel(cfg.LLMConfig)
	service.isReady = true
	service.readyState = "Ready"

	return service, nil
}

// NewEmptyLLMService 创建一个空的LLM服务实例作为后备方案
func NewEmptyLLMService() *LLMService {
	service := createBaseLLMService()
	service.providerName = "empty"
	service.readyState = "Standby Service Mode – Please configure the API key in settings"
	return service
}

// createBaseLLMService 创建基础LLM服务实例
func createBaseLLMService() *LLMService {
	return &LLMService{
		provider:           nil,
		providerName:       "",
		isReady:            false,
		readyState:         "Uninitialized",
		activeDefaultModel: "",
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

	// Check if provider exists and is set
	if s.provider != nil && s.isReady {
		return true
	}

	// Check current config to see if service should be ready
	cfg := config.GetCurrentConfig()
	if cfg == nil {
		return false
	}

	// Check if provider and API key are properly configured
	if cfg.LLMProvider == "" {
		return false
	}

	// Check if API key is available in the current config
	// The LLMConfig from GetCurrentConfig should already include decrypted API key
	if cfg.LLMConfig == nil || cfg.LLMConfig["api_key"] == "" {
		return false
	}

	return true
}

// GetReadyState 返回服务就绪状态描述
func (s *LLMService) GetReadyState() string {
	s.providerMutex.RLock()
	defer s.providerMutex.RUnlock()

	// Return real-time status based on current config
	cfg := config.GetCurrentConfig()
	if cfg == nil {
		return "Cannot get configuration"
	}

	if cfg.LLMProvider == "" {
		return "LLM provider not configured"
	}

	// Check if API key is available in the current config
	// The LLMConfig from GetCurrentConfig should already include decrypted API key
	if cfg.LLMConfig == nil || cfg.LLMConfig["api_key"] == "" {
		return "API key not configured"
	}

	// If we have a provider set and it's marked as ready, return "已就绪"
	if s.provider != nil && s.isReady {
		return "Ready"
	}

	// Otherwise, the provider might need to be initialized
	return "Waiting for initialization"
}

// GetProviderStatus 返回服务是否就绪以及可读描述
func (s *LLMService) GetProviderStatus() (bool, string) {
	if s == nil {
		return false, "LLM服务实例未初始化"
	}
	if s.IsReady() {
		return true, "Ready"
	}
	return false, s.GetReadyState()
}

// UpdateProvider 更新LLM服务的提供商
func (s *LLMService) UpdateProvider(providerName string, config map[string]string) error {
	provider, err := llm.GetProvider(providerName, config)
	if err != nil {
		s.providerMutex.Lock()
		s.isReady = false
		s.readyState = fmt.Sprintf("Configuration failed: %v", err)
		s.providerMutex.Unlock()
		return err
	}

	s.providerMutex.Lock()
	defer s.providerMutex.Unlock()

	s.provider = provider
	s.providerName = providerName
	s.activeDefaultModel = extractDefaultModel(config)
	s.isReady = true
	s.readyState = "Ready"

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
			utils.GetLogger().Warn("Unknown message role type", map[string]interface{}{"role": msg.Role})
		}
	}

	// 助手消息历史，将其整合到用户提示中
	if len(assistantMessages) > 0 {
		conversationHistory := strings.Join(assistantMessages, "\n\n")
		userContent = fmt.Sprintf("Conversation history:\n%s\n\nCurrent user input: %s",
			conversationHistory, userContent)
	}

	// 解析需要使用的模型
	resolvedModel := s.resolveModel(request.Model)

	// 生成缓存键
	cacheKey := s.generateCacheKey(userContent, systemContent, resolvedModel)

	// 检查缓存
	if s.cache != nil {
		var cachedResult ChatCompletionResponse
		if s.checkAndUseCache(cacheKey, &cachedResult) {
			utils.GetLogger().Info("DEBUG:LLM Chat cache hit", map[string]interface{}{"cache_key_prefix": cacheKey[:8]})
			return cachedResult, nil
		}
	}

	// 转换请求格式
	req := llm.CompletionRequest{
		Model:       resolvedModel,
		Temperature: float32(request.Temperature),
		MaxTokens:   request.MaxTokens,
		ExtraParams: request.ExtraParams,
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
		utils.GetLogger().Info("DEBUG:Save to LLM chat cache", map[string]interface{}{"cache_key_prefix": cacheKey[:8]})
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
		return fmt.Errorf("LLM service not ready: %s", s.readyState)
	}
	provider := s.provider
	s.providerMutex.RUnlock()

	model := s.resolveModel("")

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
	structuredSystemPrompt += "Return your response in valid JSON format, following the provided output schema, without adding explanations or preambles."

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
		return fmt.Errorf("failed to parse AI response into structured data: %w\nAI return: %s", err, text)
	}

	// 保存到缓存
	s.saveToCache(cacheKey, outputSchema)

	return nil
}

// 清理JSON字符串，去除前后非JSON内容
var jsonNoiseReplacer = strings.NewReplacer(
	"```json", "",
	"```", "",
	"\ufeff", "",
	"\u00a0", " ",
	"\u2028", "\n",
	"\u2029", "\n",
)

var structuralPunctuationMap = map[rune]rune{
	'：': ':',
	'﹕': ':',
	'，': ',',
	'﹐': ',',
	'；': ';',
	'﹔': ';',
	'【': '[',
	'】': ']',
	'［': '[',
	'］': ']',
	'｛': '{',
	'｝': '}',
	'（': '(',
	'）': ')',
}

var quotePairs = map[rune]rune{
	'"': '"',
	'“': '”',
	'”': '”',
	'„': '”',
	'‟': '”',
	'「': '」',
	'」': '」',
	'『': '』',
	'﹁': '﹂',
	'﹂': '﹂',
}

func normalizeJSONStructure(s string) string {
	if s == "" {
		return s
	}

	var builder strings.Builder
	builder.Grow(len(s))
	inString := false
	escaped := false
	currentClosing := '"'

	for _, r := range s {
		if inString {
			if !escaped && r == '\\' {
				escaped = true
				builder.WriteRune(r)
				continue
			}

			if escaped {
				escaped = false
				builder.WriteRune(r)
				continue
			}

			if r == currentClosing || r == '"' {
				inString = false
				currentClosing = '"'
				builder.WriteRune('"')
				continue
			}

			builder.WriteRune(r)
			continue
		}

		if replacement, ok := structuralPunctuationMap[r]; ok {
			r = replacement
		} else if closing, ok := quotePairs[r]; ok {
			inString = true
			currentClosing = closing
			builder.WriteRune('"')
			continue
		} else if r == '"' {
			inString = true
			currentClosing = '"'
			builder.WriteRune(r)
			continue
		} else if r > unicode.MaxASCII && !unicode.IsSpace(r) {
			// 丢弃出现在字符串外的异常Unicode字符（例如 æ、• 等）
			continue
		}

		builder.WriteRune(r)
	}

	return builder.String()
}

func cleanJSONString(s string) string {
	if s == "" {
		return s
	}

	// 统一替换常见的噪声、全角符号以及Markdown标记
	s = jsonNoiseReplacer.Replace(s)
	s = strings.TrimSpace(s)

	// 移除零宽字符及除换行/制表符外的控制字符
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\u200b', '\u200c', '\u200d', '\u2060', '\ufeff':
			return -1
		}
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, s)

	// 查找第一个 { 或 [，将其之前的内容全部丢弃
	start := strings.IndexAny(s, "[{")
	if start == -1 {
		return s
	}

	s = strings.TrimSpace(s[start:])
	if s == "" {
		return s
	}

	// 规范化JSON结构所需的标点符号，移除字符串外的异常字符
	s = normalizeJSONStructure(s)

	isArray := len(s) > 0 && s[0] == '['

	// 简单的括号计数匹配
	balance := 0
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		char := s[i]

		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			if isArray {
				if char == '[' {
					balance++
				} else if char == ']' {
					balance--
				}
			} else {
				if char == '{' {
					balance++
				} else if char == '}' {
					balance--
				}
			}

			if balance == 0 {
				// 找到了匹配的结束符
				return strings.TrimSpace(s[:i+1])
			}
		}
	}

	// 如果没找到匹配的结束符，尝试回退到旧逻辑（找最后一个）
	end := -1
	if isArray {
		end = strings.LastIndex(s, "]")
	} else {
		end = strings.LastIndex(s, "}")
	}

	if end != -1 && end >= 0 {
		return strings.TrimSpace(s[:end+1])
	}

	return strings.TrimSpace(s)
}

// CleanLLMJSONResponse 提供给外部调用的JSON清洗助手
func CleanLLMJSONResponse(raw string) string {
	return cleanJSONString(raw)
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
		prompt = fmt.Sprintf(`Analyze the following text titled "%s" and extract all character information.
Return the result as a JSON array of objects with the exact schema described in the system prompt.
If a field is unknown, use an empty string or empty array.

Text:
%s`, title, text)

		systemPrompt = `You are a professional literary character analyst. Respond ONLY with valid JSON that matches the following schema:
[
	{
		"name": "string",
		"role": "string",
		"description": "string",
		"personality": "string",
		"background": "string",
		"speech_style": "string",
		"relationships": {"string": "string"},
		"knowledge": ["string"]
	}
]
Formatting requirements:
1. The entire response must be a single JSON array (use [] when no characters are found).
2. Use standard ASCII characters for quotes, commas, and colons. Do NOT use Chinese punctuation or Markdown fences.
3. Do not add explanations, comments, or prose outside the JSON array.`
	} else {
		// 中文提示词（原有逻辑）
		prompt = fmt.Sprintf(`分析以下标题为《%s》的文本，提取所有角色信息。
结果必须符合系统提示中描述的JSON数组结构。如某字段未知请使用空字符串或空数组。

文本内容:
%s`, title, text)

		systemPrompt = `你是专业的文学角色分析专家。回答时只能输出有效的JSON，并且严格符合以下数组结构：
[
	{
		"name": "",
		"role": "",
		"description": "",
		"personality": "",
		"background": "",
		"speech_style": "",
		"relationships": {"": ""},
		"knowledge": [""]
	}
]
格式要求：
1. 整个回答必须是一个JSON数组，没有角色时返回[]。
2. 必须使用半角的双引号、冒号、逗号，不得使用全角符号或Markdown代码块。
3. 禁止在JSON前后添加任何说明文字。`
	}

	// 使用结构化输出API
	request := llm.CompletionRequest{
		Model:        s.GetDefaultModel(),
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
		MaxTokens:    4000, // 增加token限制以容纳完整的JSON
		Temperature:  0.2,
	}

	cacheKey := s.GenerateCacheKey(request)
	if cachedResp := s.CheckCache(cacheKey); cachedResp != nil {
		cleanedText := cleanJSONString(cachedResp.Text)
		// 尝试解析为数组格式
		var characters []CharacterInfo
		err := json.Unmarshal([]byte(cleanedText), &characters)
		if err == nil {
			return characters, nil
		}

		// 如果解析数组失败，尝试解析为单个对象
		var singleCharacter CharacterInfo
		err = json.Unmarshal([]byte(cleanedText), &singleCharacter)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cached AI response into structured data: %w\nAI return: %s",
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

	cleanedText := cleanJSONString(response.Text)
	// 尝试解析为数组格式
	var characters []CharacterInfo
	err = json.Unmarshal([]byte(cleanedText), &characters)
	if err == nil {
		return characters, nil
	}

	// 如果解析数组失败，尝试解析为单个对象
	var singleCharacter CharacterInfo
	err = json.Unmarshal([]byte(cleanedText), &singleCharacter)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response into structured data: %w\nAI return: %s",
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
		prompt = fmt.Sprintf(`Analyze the following text titled "%s" and extract all scene information.
Return the result as a JSON array of objects with the exact schema described in the system prompt.
If a field is unknown, use an empty string or empty array.

Text:
%s`,
			title, truncateText(text, 5000))

		systemPrompt = `You are a professional scene analysis expert. Respond ONLY with valid JSON that matches the following schema:
[
	{
		"name": "string",
		"description": "string",
		"atmosphere": "string",
		"era": "string",
		"themes": ["string"],
		"items": ["string"],
		"importance": "string"
	}
]
Formatting requirements:
1. Output must be a single JSON array (return [] when no scenes are found).
2. Use ASCII double quotes, commas, and colons. Do NOT use Chinese punctuation or Markdown code fences.
3. Provide no commentary outside the JSON array.`
	} else {
		// 原有中文提示词
		prompt = fmt.Sprintf(`分析以下标题为《%s》的文本，提取所有场景信息。
结果必须符合系统提示中提供的JSON数组结构，如无数据请返回[]。

文本内容:
%s`,
			title, truncateText(text, 5000))

		systemPrompt = `你是专业的场景分析专家。你只能输出符合以下结构的JSON数组：
[
	{
		"name": "",
		"description": "",
		"atmosphere": "",
		"era": "",
		"themes": [""],
		"items": [""],
		"importance": ""
	}
]
格式要求：
1. 仅输出JSON数组；没有场景时返回[]。
2. 必须使用半角双引号、逗号、冒号，不得使用全角符号或Markdown代码块。
3. JSON前后不能添加任何解释性文字。`
	}

	// 使用结构化输出API
	request := llm.CompletionRequest{
		Model:        s.GetDefaultModel(),
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
		MaxTokens:    4000,
		Temperature:  0.2,
	}

	cacheKey := s.GenerateCacheKey(request)
	if cachedResp := s.CheckCache(cacheKey); cachedResp != nil {
		cleanedText := cleanJSONString(cachedResp.Text)
		// 尝试解析为数组格式
		var scenes []SceneInfo
		err := json.Unmarshal([]byte(cleanedText), &scenes)
		if err == nil {
			return scenes, nil
		}

		// 如果解析数组失败，尝试解析为单个对象
		var singleScene SceneInfo
		err = json.Unmarshal([]byte(cleanedText), &singleScene)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cached AI response into structured data: %w\nAI return: %s",
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

	cleanedText := cleanJSONString(response.Text)
	// 尝试解析为数组格式
	var scenes []SceneInfo
	err = json.Unmarshal([]byte(cleanedText), &scenes)
	if err == nil {
		return scenes, nil
	}

	// 如果解析数组失败，尝试解析为单个对象
	var singleScene SceneInfo
	err = json.Unmarshal([]byte(cleanedText), &singleScene)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response into structured data: %w\nAI return: %s",
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
	return s.resolveModel("")
}

// resolveModel 根据请求和配置确定应使用的模型
func (s *LLMService) resolveModel(requestedModel string) string {
	if trimmed := strings.TrimSpace(requestedModel); trimmed != "" {
		return trimmed
	}

	s.providerMutex.RLock()
	provider := s.provider
	providerName := s.providerName
	activeDefault := s.activeDefaultModel
	s.providerMutex.RUnlock()

	if activeDefault != "" {
		return activeDefault
	}

	if provider != nil {
		if models := provider.GetSupportedModels(); len(models) > 0 {
			if model := strings.TrimSpace(models[0]); model != "" {
				return model
			}
		}
	}

	if cfg := config.GetCurrentConfig(); cfg != nil && cfg.LLMProvider == providerName {
		if cfg.LLMConfig != nil {
			if model := strings.TrimSpace(cfg.LLMConfig["default_model"]); model != "" {
				return model
			}
			if model := strings.TrimSpace(cfg.LLMConfig["model"]); model != "" {
				return model
			}
		}
	}

	if model, exists := providerDefaultModels[providerName]; exists {
		if trimmed := strings.TrimSpace(model); trimmed != "" {
			return trimmed
		}
	}

	return "gpt-4.1"
}

func extractDefaultModel(cfg map[string]string) string {
	if cfg == nil {
		return ""
	}
	if model := strings.TrimSpace(cfg["default_model"]); model != "" {
		return model
	}
	if model := strings.TrimSpace(cfg["model"]); model != "" {
		return model
	}
	return ""
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
					utils.GetLogger().Info("DEBUG:LLM cache hit", map[string]interface{}{"cache_key_prefix": cacheKey[:8]})
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
						utils.GetLogger().Info("DEBUG:LLM cache hit", map[string]interface{}{"cache_key_prefix": cacheKey[:8]})
						return true
					}
				}
			} else {
				// 对于普通响应，直接返回
				if chatResp, ok := outputSchema.(*ChatCompletionResponse); ok {
					*chatResp = resp
					utils.GetLogger().Info("DEBUG:LLM cache hit", map[string]interface{}{"cache_key_prefix": cacheKey[:8]})
					return true
				}
			}
		}

		// 尝试直接转换为 CompletionResponse
		if resp, ok := cachedResponse.(*llm.CompletionResponse); ok {
			if outputSchema != nil {
				err := json.Unmarshal([]byte(resp.Text), outputSchema)
				if err == nil {
					utils.GetLogger().Info("DEBUG:LLM cache hit", map[string]interface{}{"cache_key_prefix": cacheKey[:8]})
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
			utils.GetLogger().Error("Failed to serialize cached response", map[string]interface{}{"err": err})
			// 仍然尝试保存原始响应以向后兼容
			s.cache.saveToCache(cacheKey, response)
		} else {
			s.cache.saveToCache(cacheKey, responseBytes)
		}
		utils.GetLogger().Info("DEBUG:Save to LLM cache", map[string]interface{}{"cache_key_prefix": cacheKey[:8]})
	}
}

// SanitizeLLMJSONResponse 移除LLM响应中的Markdown代码块或反引号，确保可以解析为JSON
func SanitizeLLMJSONResponse(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return cleaned
	}

	if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
		lower := strings.ToLower(cleaned)
		if strings.HasPrefix(lower, "json") {
			cleaned = strings.TrimSpace(cleaned[4:])
		}
		if idx := strings.LastIndex(cleaned, "```"); idx != -1 {
			cleaned = cleaned[:idx]
		}
	}

	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.Trim(cleaned, "`")
	return strings.TrimSpace(cleaned)
}
