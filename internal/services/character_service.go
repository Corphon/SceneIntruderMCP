// internal/services/character_service.go
package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

// CharacterService 处理角色相关的业务逻辑
type CharacterService struct {
	LLMService     *LLMService
	ContextService *ContextService
}

// EmotionData 提取的情绪数据
type EmotionData struct {
	Text              string   `json:"text"`
	Emotion           string   `json:"emotion"`
	Action            string   `json:"action"`
	Intensity         int      `json:"intensity"`
	BodyLanguage      string   `json:"body_language"`
	FacialExpression  string   `json:"facial_expression"`
	VoiceTone         string   `json:"voice_tone"`
	SecondaryEmotions []string `json:"secondary_emotions"`
}

// ChatResponseWithEmotion 包含情绪和动作的聊天响应
type ChatResponseWithEmotion struct {
	CharacterID string `json:"character_id"`
	Message     string `json:"message"`
	Emotion     string `json:"emotion"`            // 情绪标识
	Action      string `json:"action"`             // 行为描述
	Original    string `json:"original,omitempty"` // 原始回复（调试用）
}

// NewCharacterService 创建角色服务
func NewCharacterService() *CharacterService {
	// 从DI容器获取服务
	container := di.GetContainer()

	// 获取LLM服务
	var llmService *LLMService
	if llmObj := container.Get("llm"); llmObj != nil {
		llmService = llmObj.(*LLMService)
	}

	// 获取或创建上下文服务
	var contextService *ContextService
	if ctxObj := container.Get("context"); ctxObj != nil {
		contextService = ctxObj.(*ContextService)
	} else {
		// 获取场景服务
		var sceneService *SceneService
		if sceneObj := container.Get("scene"); sceneObj != nil {
			sceneService = sceneObj.(*SceneService)
		} else {
			scenesPath := "data/scenes"
			sceneService = NewSceneService(scenesPath)
			container.Register("scene", sceneService)
		}

		// 创建上下文服务
		contextService = NewContextService(sceneService)
		container.Register("context", contextService)
	}

	return &CharacterService{
		LLMService:     llmService,
		ContextService: contextService,
	}
}

// GenerateResponse 生成角色回应
func (s *CharacterService) GenerateResponse(sceneID, characterID, userMessage string) (*models.ChatResponse, error) {
	// 加载场景数据
	sceneData, err := s.ContextService.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, err
	}
	// 获取角色
	character, err := s.GetCharacter(sceneID, characterID)
	if err != nil {
		return nil, err
	}

	if character == nil {
		return nil, fmt.Errorf("角色不存在: %s", characterID)
	}

	// 获取角色记忆
	memory, err := s.ContextService.BuildCharacterMemory(sceneID, characterID)
	if err != nil {
		// 记忆构建失败不是致命错误，可以继续
		memory = "我没有之前的记忆。"
	}

	// 获取近期对话
	recentConversations, err := s.ContextService.GetRecentConversations(sceneID, 10)
	if err != nil {
		recentConversations = []models.Conversation{}
	}

	// 构建提示词
	prompt := buildCharacterPrompt(character, sceneData.Scene, memory, recentConversations)

	// 使用LLMService替代直接调用OpenAI
	var characterResponse string
	if s.LLMService != nil {
		// 创建聊天请求
		resp, err := s.LLMService.CreateChatCompletion(
			context.Background(),
			ChatCompletionRequest{
				Model: "gpt-4", // 使用配置或默认模型
				Messages: []ChatCompletionMessage{
					{
						Role:    "system",
						Content: prompt,
					},
					{
						Role:    "user",
						Content: userMessage,
					},
				},
				Temperature: 0.7,
				MaxTokens:   800,
			},
		)

		if err != nil {
			return nil, fmt.Errorf("AI服务错误: %w", err)
		}

		// 提取响应内容
		characterResponse = resp.Choices[0].Message.Content
	} else {
		// LLM服务未配置时的回退方案
		characterResponse = fmt.Sprintf(
			"[系统消息] AI服务未配置。%s可能会说: 很抱歉，我现在无法回应。",
			character.Name)
	}

	// 添加到对话记录
	err = s.ContextService.AddConversation(sceneID, "user", userMessage, nil)
	if err != nil {
		// 记录失败不影响响应
		fmt.Printf("记录用户对话失败: %v\n", err)
	}

	err = s.ContextService.AddConversation(sceneID, characterID, characterResponse, nil)
	if err != nil {
		fmt.Printf("记录角色回应失败: %v\n", err)
	}

	// 返回角色回应
	return &models.ChatResponse{
		Character: character.Name,
		Response:  characterResponse,
	}, nil
}

// buildCharacterPrompt 构建角色提示词
func buildCharacterPrompt(character *models.Character, scene models.Scene, memory string, conversations []models.Conversation) string {
	// 检测语言
	isEnglish := isEnglishText(character.Name + " " + character.Description + " " + scene.Title)

	// 构建基础提示词
	var prompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`You will roleplay as a character named "%s" in the scene "%s".
Character description: %s
Personality: %s

Your memory:
%s

Current scene:
%s

Stay in character at all times. Don't break the fourth wall or mention you're an AI. Responses should reflect the character's personality, knowledge, and background.
`, character.Name, scene.Title, character.Description, character.Personality, memory, scene.Description)

		// 添加近期对话历史
		if len(conversations) > 0 {
			prompt += "\nRecent conversation history:\n"
			for _, conv := range conversations {
				speaker := "User"
				if conv.SpeakerID != "user" {
					speaker = character.Name
				}
				prompt += fmt.Sprintf("%s: %s\n", speaker, conv.Content)
			}
		}
	} else {
		// 中文提示词（原有逻辑）
		prompt = fmt.Sprintf(`你将扮演一个名为"%s"的角色，在场景"%s"中。
角色描述：%s
个性：%s

你的记忆：
%s

当前场景：
%s

对话必须保持在角色内，不要打破第四面墙或提到你是AI。回应应该反映角色的个性、知识和背景。
`, character.Name, scene.Title, character.Description, character.Personality, memory, scene.Description)

		// 添加近期对话历史
		if len(conversations) > 0 {
			prompt += "\n近期对话历史：\n"
			for _, conv := range conversations {
				speaker := "用户"
				if conv.SpeakerID != "user" {
					speaker = character.Name
				}
				prompt += fmt.Sprintf("%s: %s\n", speaker, conv.Content)
			}
		}
	}

	return prompt
}

// 扩展GenerateResponseWithEmotion方法，返回更丰富的情绪数据
func (s *CharacterService) GenerateResponseWithEmotion(sceneID, characterID, message string) (*models.EmotionalResponse, error) {
	// 加载角色数据
	character, err := s.GetCharacter(sceneID, characterID)
	if err != nil {
		return nil, err
	}

	// 检测语言
	isEnglish := isEnglishText(character.Name + " " + character.Description + " " + message)

	var systemPrompt, userPrompt string

	if isEnglish {
		// 英文提示词
		systemPrompt = fmt.Sprintf(
			"You are an emotional analyst simulating character '%s'. Analyze emotions and generate character responses, including emotion type, intensity, and body language."+
				"Character description: %s\n"+
				"Speech style: %s\n"+
				"Personality traits: %s\n",
			character.Name,
			character.Description,
			character.SpeechStyle,
			character.Personality,
		)

		userPrompt = fmt.Sprintf(
			"User message: %s\n\n"+
				"Please respond as the character and analyze the emotions contained in the response."+
				"Return in JSON format, including:\n"+
				"- response (complete response text)\n"+
				"- emotion (core emotion, such as: joy/anger/sadness/fear/surprise/contempt/disgust/neutral)\n"+
				"- intensity (emotional intensity 1-10)\n"+
				"- body_language (description of character's body language and actions)\n"+
				"- facial_expression (facial expression description)\n"+
				"- voice_tone (tone of voice description)\n"+
				"- secondary_emotions (secondary emotions, maximum of two)",
			message,
		)
	} else {
		// 中文提示词（原有逻辑）
		systemPrompt = fmt.Sprintf(
			"你是模拟角色'%s'的情感分析师。分析情绪并生成角色回复，包括情感类型、强度和身体语言。"+
				"角色描述：%s\n"+
				"说话风格：%s\n"+
				"性格特点：%s\n",
			character.Name,
			character.Description,
			character.SpeechStyle,
			character.Personality,
		)

		userPrompt = fmt.Sprintf(
			"用户消息：%s\n\n"+
				"请以角色的身份回复，并分析回复中包含的情感。"+
				"返回JSON格式，包含：\n"+
				"- response (完整回复文本)\n"+
				"- emotion (核心情感，如：喜悦/愤怒/悲伤/恐惧/惊讶/鄙视/厌恶/中性)\n"+
				"- intensity (情感强度1-10)\n"+
				"- body_language (角色的肢体语言和动作描述)\n"+
				"- facial_expression (面部表情描述)\n"+
				"- voice_tone (语调描述)\n"+
				"- secondary_emotions (次要情绪，最多两种）",
			message,
		)
	}

	// 调用LLM服务
	var emotionalData models.EmotionalResponse
	err = s.LLMService.CreateStructuredCompletion(
		context.Background(),
		userPrompt,
		systemPrompt,
		&emotionalData,
	)
	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("emotion analysis failed: %w", err)
		} else {
			return nil, fmt.Errorf("情感分析失败: %w", err)
		}
	}

	// 设置角色信息
	emotionalData.CharacterID = characterID
	emotionalData.CharacterName = character.Name
	emotionalData.Timestamp = time.Now()

	// 存储对话历史
	metadata := map[string]interface{}{
		"emotion":            emotionalData.Emotion,
		"intensity":          emotionalData.Intensity,
		"body_language":      emotionalData.BodyLanguage,
		"facial_expression":  emotionalData.FacialExpression,
		"voice_tone":         emotionalData.VoiceTone,
		"secondary_emotions": emotionalData.SecondaryEmotions,
	}

	// 先添加用户消息
	err = s.ContextService.AddConversation(
		sceneID,
		"user",  // 用户作为发言者
		message, // 用户消息内容
		nil,     // 用户消息无需情感数据
	)
	if err != nil {
		if isEnglish {
			fmt.Printf("Failed to record user message: %v\n", err)
		} else {
			fmt.Printf("记录用户消息失败: %v\n", err)
		}
	}

	// 添加角色回应
	err = s.ContextService.AddConversation(
		sceneID,
		characterID,            // 角色作为发言者
		emotionalData.Response, // 角色回应内容
		metadata,               // 情感相关元数据
	)
	if err != nil {
		if isEnglish {
			fmt.Printf("Failed to record character response: %v\n", err)
		} else {
			fmt.Printf("记录角色回应失败: %v\n", err)
		}
	}

	return &emotionalData, nil
}

// GetCharacter 根据ID获取指定场景中的角色
func (s *CharacterService) GetCharacter(sceneID, characterID string) (*models.Character, error) {
	// 加载场景数据
	sceneData, err := s.ContextService.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, fmt.Errorf("加载场景数据失败: %w", err)
	}

	// 查找角色
	var character *models.Character
	for _, c := range sceneData.Characters {
		if c.ID == characterID {
			character = c
			break
		}
	}

	// 如果未找到角色，返回错误
	if character == nil {
		return nil, fmt.Errorf("角色不存在: %s", characterID)
	}

	return character, nil
}

// buildCharacterPromptWithEmotion 构建包含情绪指导的角色提示词
func buildCharacterPromptWithEmotion(character *models.Character, scene models.Scene, memory string, conversations []models.Conversation) string {
	// 检测语言
	isEnglish := isEnglishText(character.Name + " " + character.Description + " " + scene.Title)

	// 构建基础提示词
	var prompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`You will roleplay as a character named "%s" in the scene "%s".
Character description: %s
Personality: %s

Your memory:
%s

Current scene:
%s

When responding, please follow this format:
[Emotion: (brief emotion description, such as "happy", "fearful", "angry", "confused", etc.)]
[Action: (brief body language or behavior description, such as "smiles", "frowns", "takes a step back", etc.)]
Your actual dialogue content

Stay in character at all times. Don't break the fourth wall or mention you're an AI. Responses should reflect the character's personality, knowledge, and background.
`, character.Name, scene.Title, character.Description, character.Personality, memory, scene.Description)

		// 添加近期对话历史
		if len(conversations) > 0 {
			prompt += "\nRecent conversation history:\n"
			for _, conv := range conversations {
				speaker := "User"
				if conv.SpeakerID != "user" {
					speaker = character.Name
				}
				prompt += fmt.Sprintf("%s: %s\n", speaker, conv.Content)
			}
		}
	} else {
		// 中文提示词
		prompt = fmt.Sprintf(`你将扮演一个名为"%s"的角色，在场景"%s"中。
角色描述：%s
个性：%s

你的记忆：
%s

当前场景：
%s

回复时，请遵循以下格式：
[情绪: (简短的情绪描述，如"高兴"、"恐惧"、"愤怒"、"困惑"等)]
[动作: (简短的肢体语言或行为描述，如"微笑"、"皱眉"、"后退一步"等)]
你的实际对话内容

对话必须保持在角色内，不要打破第四面墙或提到你是AI。回应应该反映角色的个性、知识和背景。
`, character.Name, scene.Title, character.Description, character.Personality, memory, scene.Description)

		// 添加近期对话历史
		if len(conversations) > 0 {
			prompt += "\n近期对话历史：\n"
			for _, conv := range conversations {
				speaker := "用户"
				if conv.SpeakerID != "user" {
					speaker = character.Name
				}
				prompt += fmt.Sprintf("%s: %s\n", speaker, conv.Content)
			}
		}
	}

	return prompt
}

// extractEmotionAndAction 从回应中提取情绪和行为
func extractEmotionAndAction(response string) EmotionData {
	result := EmotionData{
		Text:    response,
		Emotion: "中性", // 默认情绪
		Action:  "",   // 默认无动作
	}

	// 提取情绪
	emotionRegex := regexp.MustCompile(`\[情绪:\s*(.*?)\]`)
	if matches := emotionRegex.FindStringSubmatch(response); len(matches) > 1 {
		result.Emotion = strings.TrimSpace(matches[1])
		result.Text = emotionRegex.ReplaceAllString(result.Text, "")
	}

	// 提取动作
	actionRegex := regexp.MustCompile(`\[动作:\s*(.*?)\]`)
	if matches := actionRegex.FindStringSubmatch(response); len(matches) > 1 {
		result.Action = strings.TrimSpace(matches[1])
		result.Text = actionRegex.ReplaceAllString(result.Text, "")
	}

	// 清理文本
	result.Text = strings.TrimSpace(result.Text)

	return result
}

// GenerateResponseWithEmotionFormat 生成带有标准情绪格式的角色回应
// 使用 [情绪: ...][动作: ...] 格式
func (s *CharacterService) GenerateResponseWithEmotionFormat(sceneID, characterID, userMessage string) (*ChatResponseWithEmotion, error) {
	// 加载场景数据
	sceneData, err := s.ContextService.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, err
	}

	// 获取角色
	character, err := s.GetCharacter(sceneID, characterID)
	if err != nil {
		return nil, err
	}

	// 获取角色记忆
	memory, err := s.ContextService.BuildCharacterMemory(sceneID, characterID)
	if err != nil {
		memory = "我没有之前的记忆。"
	}

	// 获取近期对话
	recentConversations, err := s.ContextService.GetRecentConversations(sceneID, 10)
	if err != nil {
		recentConversations = []models.Conversation{}
	}

	// 构建带情绪指导的提示词
	prompt := buildCharacterPromptWithEmotion(character, sceneData.Scene, memory, recentConversations)

	// 调用LLM服务
	var characterResponse string
	if s.LLMService != nil {
		// 从配置或提供商获取默认模型
		modelName := s.LLMService.GetDefaultModel()
		resp, err := s.LLMService.CreateChatCompletion(
			context.Background(),
			ChatCompletionRequest{
				Model: modelName,
				Messages: []ChatCompletionMessage{
					{
						Role:    "system",
						Content: prompt,
					},
					{
						Role:    "user",
						Content: userMessage,
					},
				},
				Temperature: 0.7,
				MaxTokens:   800,
			},
		)

		if err != nil {
			return nil, fmt.Errorf("AI服务错误: %w", err)
		}

		characterResponse = resp.Choices[0].Message.Content
	} else {
		characterResponse = fmt.Sprintf(
			"[情绪: 中性][动作: 无表情] [系统消息] AI服务未配置。%s可能会说: 很抱歉，我现在无法回应。",
			character.Name)
	}

	// 提取情绪和动作
	emotionData := extractEmotionAndAction(characterResponse)

	// 添加到对话记录
	metadata := map[string]interface{}{
		"emotion": emotionData.Emotion,
		"action":  emotionData.Action,
	}

	// 先添加用户消息
	s.ContextService.AddConversation(sceneID, "user", userMessage, nil)

	// 添加角色回应
	s.ContextService.AddConversation(sceneID, characterID, emotionData.Text, metadata)

	// 返回带情绪的回应
	return &ChatResponseWithEmotion{
		CharacterID: character.ID,
		Message:     emotionData.Text,
		Emotion:     emotionData.Emotion,
		Action:      emotionData.Action,
		Original:    characterResponse,
	}, nil
}

// buildCharacterInteractionPrompt 构建角色互动提示词
func (s *CharacterService) buildCharacterInteractionPrompt(characters []*models.Character, topic string, contextDescription string) string {
	// 检测主要语言
	var isEnglish bool
	if len(characters) > 0 {
		isEnglish = isEnglishText(characters[0].Name + characters[0].Description)
	}

	var prompt strings.Builder

	if isEnglish {
		// 英文提示词
		prompt.WriteString(`You are directing a sophisticated character interaction scene. Create a compelling, multi-layered dialogue that showcases each character's unique voice while advancing the narrative.

Key Requirements:
1. **Character Authenticity**: Each character must speak in their established voice and personality
2. **Dynamic Interaction**: Characters should react authentically to each other's words and emotions
3. **Emotional Depth**: Include subtext, emotional undercurrents, and realistic character motivations
4. **Natural Flow**: Ensure conversation rhythm feels organic and engaging
5. **Conflict/Tension**: Introduce appropriate dramatic tension based on character relationships
6. **Story Progression**: The dialogue should either reveal character depth or advance plot elements

`)
		prompt.WriteString("Characters participating in this conversation:\n")

		for _, char := range characters {
			prompt.WriteString(fmt.Sprintf("Character: %s\n", char.Name))
			prompt.WriteString(fmt.Sprintf("Description: %s\n", char.Description))
			prompt.WriteString(fmt.Sprintf("Personality: %s\n", char.Personality))
			if char.SpeechStyle != "" {
				prompt.WriteString(fmt.Sprintf("Speech Style: %s\n", char.SpeechStyle))
			}
			prompt.WriteString("\n")
		}

		prompt.WriteString(fmt.Sprintf("Context of interaction: %s\n\n", contextDescription))
		prompt.WriteString(fmt.Sprintf("Topic of conversation: %s\n\n", topic))
		prompt.WriteString("Format the dialogue as a JSON array with the following structure for each dialogue:\n")
		prompt.WriteString("[\n")
		prompt.WriteString("  {\n")
		prompt.WriteString("    \"character_id\": \"ID of the speaking character\",\n")
		prompt.WriteString("    \"character_name\": \"Name of the speaking character\",\n")
		prompt.WriteString("    \"message\": \"What the character says\",\n")
		prompt.WriteString("    \"emotion\": \"Character's emotion (joy/anger/sadness/fear/surprise/etc.)\",\n")
		prompt.WriteString("    \"action\": \"Character's action or body language\"\n")
		prompt.WriteString("  },\n")
		prompt.WriteString("  { ... more dialogue entries ... }\n")
		prompt.WriteString("]\n\n")
		prompt.WriteString(`Create a sophisticated multi-turn conversation with these advanced requirements:

		**Dialogue Dynamics**:
		1. **Emotional Escalation/De-escalation**: Show how emotions build or subside throughout the conversation
		2. **Active Listening**: Characters should demonstrate they've heard and processed previous statements
		3. **Subtext and Implications**: Include what characters don't say directly but imply
		4. **Power Dynamics**: Reflect the social hierarchy and relationships between characters
		5. **Micro-Conflicts**: Small disagreements or misunderstandings that feel realistic
		6. **Character Growth Moments**: Brief instances where characters reveal new aspects of themselves
		
		**Technical Requirements**:
		- Each character must speak at least once with substantial content (minimum 2 sentences)
		- Conversation should have natural pauses and rhythm
		- Include transitional moments where focus shifts between characters
		- End with either resolution, escalation, or compelling setup for future interaction
		
		**Conversation Arc**: Beginning → Development → Climax/Turning Point → Resolution/Cliffhanger`)
	} else {
		// 中文提示词
		prompt.WriteString(`你正在导演一个复杂的角色互动场景。请创建一段引人入胜的多层次对话，展现每个角色的独特声音，同时推进故事发展。

关键要求：
1. **角色真实性**：每个角色都必须以其既定的声音和个性说话
2. **动态互动**：角色应真实地回应彼此的话语和情感
3. **情感深度**：包含潜台词、情感暗流和现实的角色动机
4. **自然流动**：确保对话节奏感觉有机且引人入胜
5. **冲突/张力**：根据角色关系引入适当的戏剧张力
6. **故事推进**：对话应该揭示角色深度或推进情节元素

`)
		prompt.WriteString("参与对话的角色：\n")

		for _, char := range characters {
			prompt.WriteString(fmt.Sprintf("角色：%s\n", char.Name))
			prompt.WriteString(fmt.Sprintf("描述：%s\n", char.Description))
			prompt.WriteString(fmt.Sprintf("性格：%s\n", char.Personality))
			if char.SpeechStyle != "" {
				prompt.WriteString(fmt.Sprintf("说话风格：%s\n", char.SpeechStyle))
			}
			prompt.WriteString("\n")
		}

		prompt.WriteString(fmt.Sprintf("互动背景：%s\n\n", contextDescription))
		prompt.WriteString(fmt.Sprintf("对话主题：%s\n\n", topic))
		prompt.WriteString("请以JSON数组格式输出对话，每段对话使用以下结构：\n")
		prompt.WriteString("[\n")
		prompt.WriteString("  {\n")
		prompt.WriteString("    \"character_id\": \"说话角色的ID\",\n")
		prompt.WriteString("    \"character_name\": \"说话角色的名字\",\n")
		prompt.WriteString("    \"message\": \"角色说的话\",\n")
		prompt.WriteString("    \"emotion\": \"角色的情绪(喜悦/愤怒/悲伤/恐惧/惊讶等)\",\n")
		prompt.WriteString("    \"action\": \"角色的动作或肢体语言\"\n")
		prompt.WriteString("  },\n")
		prompt.WriteString("  { ... 更多对话 ... }\n")
		prompt.WriteString("]\n\n")
		prompt.WriteString(`创建一个复杂的多轮对话，满足以下高级要求：

		**对话动态**：
		1. **情感升级/降级**：展示情感在整个对话过程中如何积累或消退
		2. **积极倾听**：角色应证明他们听到并处理了之前的陈述
		3. **潜台词和暗示**：包含角色不直接说出但暗示的内容
		4. **权力动态**：反映角色之间的社会等级和关系
		5. **微冲突**：感觉现实的小分歧或误解
		6. **角色成长时刻**：角色揭示自己新方面的简短瞬间
		
		**技术要求**：
		- 每个角色必须至少说一次有实质内容的话（最少2句话）
		- 对话应有自然的停顿和节奏
		- 包含焦点在角色间转移的过渡时刻
		- 以解决、升级或未来互动的引人入胜的设置结束
		
		**对话弧线**：开始 → 发展 → 高潮/转折点 → 解决/悬念`)
	}

	return prompt.String()
}

// GenerateCharacterInteraction 生成角色之间的互动对话
func (s *CharacterService) GenerateCharacterInteraction(
	sceneID string,
	characterIDs []string,
	topic string,
	contextDescription string,
) (*models.CharacterInteraction, error) {
	// 获取所有相关角色
	characters := make([]*models.Character, 0, len(characterIDs))
	for _, id := range characterIDs {
		character, err := s.GetCharacter(sceneID, id)
		if err != nil {
			return nil, err
		}
		characters = append(characters, character)
	}

	// 构建角色互动提示词
	prompt := s.buildCharacterInteractionPrompt(characters, topic, contextDescription)

	// 调用LLM生成对话
	if s.LLMService == nil {
		return nil, fmt.Errorf("LLM服务未配置")
	}

	// 使用结构化输出
	var dialogues []models.InteractionDialogue
	err := s.LLMService.CreateStructuredCompletion(
		context.Background(),
		topic,      // 用户消息
		prompt,     // 系统提示词
		&dialogues, // 输出结构
	)

	if err != nil {
		return nil, fmt.Errorf("生成角色互动失败: %w", err)
	}

	// 确保每个对话有角色ID
	characterMap := make(map[string]*models.Character)
	for _, char := range characters {
		characterMap[char.Name] = char
	}

	// 修正可能的缺失字段
	for i := range dialogues {
		// 如果对话缺少角色ID但有角色名，尝试补全
		if dialogues[i].CharacterID == "" && dialogues[i].CharacterName != "" {
			if char, ok := characterMap[dialogues[i].CharacterName]; ok {
				dialogues[i].CharacterID = char.ID
			} else {
				// 使用角色名的首字母+时间戳作为临时ID
				tempID := fmt.Sprintf("%s_%d", string(dialogues[i].CharacterName[0]), time.Now().UnixNano())
				dialogues[i].CharacterID = tempID
			}
		}

		// 设置时间戳
		dialogues[i].Timestamp = time.Now()
	}

	// 创建交互对象
	interaction := &models.CharacterInteraction{
		ID:           fmt.Sprintf("interaction_%d", time.Now().UnixNano()),
		SceneID:      sceneID,
		CharacterIDs: characterIDs,
		Topic:        topic,
		Dialogues:    dialogues,
		Timestamp:    time.Now(),
		Metadata: map[string]interface{}{
			"context_description": contextDescription,
		},
	}

	// 记录到上下文系统
	for _, dialogue := range dialogues {
		metadata := map[string]interface{}{
			"interaction_id": interaction.ID,
			"emotion":        dialogue.Emotion,
			"action":         dialogue.Action,
		}

		err := s.ContextService.AddConversation(
			sceneID,
			dialogue.CharacterID,
			dialogue.Message,
			metadata,
		)

		if err != nil {
			fmt.Printf("记录角色互动对话失败: %v\n", err)
		}
	}

	return interaction, nil
}

// SimulateCharactersConversation 模拟多轮角色对话
func (s *CharacterService) SimulateCharactersConversation(
	sceneID string,
	characterIDs []string,
	initialSituation string,
	numberOfTurns int,
) ([]models.InteractionDialogue, error) {
	// 获取所有相关角色
	characters := make([]*models.Character, 0, len(characterIDs))
	for _, id := range characterIDs {
		character, err := s.GetCharacter(sceneID, id)
		if err != nil {
			return nil, err
		}
		characters = append(characters, character)
	}

	// 限制轮数
	if numberOfTurns < 1 {
		numberOfTurns = 1
	} else if numberOfTurns > 10 {
		numberOfTurns = 10 // 限制最大轮数，防止请求过大
	}

	// 检测语言
	var isEnglish bool
	if len(characters) > 0 {
		isEnglish = isEnglishText(characters[0].Name + characters[0].Description)
	}

	// 构建模拟对话提示词
	var prompt strings.Builder

	if isEnglish {
		prompt.WriteString("You are simulating a conversation between multiple characters. Create a natural, flowing dialogue over multiple turns.\n\n")
		prompt.WriteString("Characters participating in this conversation:\n")
	} else {
		prompt.WriteString("你需要模拟多个角色之间的对话。请创建一段自然流畅的多轮对话。\n\n")
		prompt.WriteString("参与对话的角色：\n")
	}

	// 添加角色信息
	for _, char := range characters {
		if isEnglish {
			prompt.WriteString(fmt.Sprintf("Character: %s\n", char.Name))
			prompt.WriteString(fmt.Sprintf("Description: %s\n", char.Description))
			prompt.WriteString(fmt.Sprintf("Personality: %s\n", char.Personality))
			if char.SpeechStyle != "" {
				prompt.WriteString(fmt.Sprintf("Speech Style: %s\n", char.SpeechStyle))
			}
		} else {
			prompt.WriteString(fmt.Sprintf("角色：%s\n", char.Name))
			prompt.WriteString(fmt.Sprintf("描述：%s\n", char.Description))
			prompt.WriteString(fmt.Sprintf("性格：%s\n", char.Personality))
			if char.SpeechStyle != "" {
				prompt.WriteString(fmt.Sprintf("说话风格：%s\n", char.SpeechStyle))
			}
		}
		prompt.WriteString("\n")
	}

	// 添加初始场景和轮数
	if isEnglish {
		prompt.WriteString(fmt.Sprintf("Initial situation: %s\n\n", initialSituation))
		prompt.WriteString(fmt.Sprintf("Number of conversation turns: %d\n\n", numberOfTurns))
		prompt.WriteString("Format the dialogue as a JSON array with the following structure for each dialogue:\n")
	} else {
		prompt.WriteString(fmt.Sprintf("初始情境：%s\n\n", initialSituation))
		prompt.WriteString(fmt.Sprintf("对话轮数：%d\n\n", numberOfTurns))
		prompt.WriteString("请以JSON数组格式输出对话，每段对话使用以下结构：\n")
	}

	// 添加输出格式指导
	prompt.WriteString("[\n")
	prompt.WriteString("  {\n")
	if isEnglish {
		prompt.WriteString("    \"character_id\": \"ID of the speaking character\",\n")
		prompt.WriteString("    \"character_name\": \"Name of the speaking character\",\n")
		prompt.WriteString("    \"message\": \"What the character says\",\n")
		prompt.WriteString("    \"emotion\": \"Character's emotion\",\n")
		prompt.WriteString("    \"action\": \"Character's action or body language\"\n")
	} else {
		prompt.WriteString("    \"character_id\": \"说话角色的ID\",\n")
		prompt.WriteString("    \"character_name\": \"说话角色的名字\",\n")
		prompt.WriteString("    \"message\": \"角色说的话\",\n")
		prompt.WriteString("    \"emotion\": \"角色的情绪\",\n")
		prompt.WriteString("    \"action\": \"角色的动作或肢体语言\"\n")
	}
	prompt.WriteString("  },\n")
	prompt.WriteString("  { ... }\n")
	prompt.WriteString("]\n\n")

	// 添加指导说明
	if isEnglish {
		prompt.WriteString("Make the dialogue natural and responsive, with characters reacting to each other's statements and emotions. Ensure each character stays true to their personality and speech style. Each character should speak at least once, and the conversation should have a natural flow and conclusion.")
	} else {
		prompt.WriteString("请确保对话自然且有回应性，角色们应对彼此的发言和情绪做出反应。确保每个角色都忠于自己的性格和说话风格。每个角色至少应发言一次，对话应有自然的流动性和结论。")
	}

	// 调用LLM生成对话
	if s.LLMService == nil {
		return nil, fmt.Errorf("LLM服务未配置")
	}

	// 使用结构化输出
	var dialogues []models.InteractionDialogue
	err := s.LLMService.CreateStructuredCompletion(
		context.Background(),
		initialSituation, // 用户消息
		prompt.String(),  // 系统提示词
		&dialogues,       // 输出结构
	)

	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to simulate character conversation: %w", err)
		} else {
			return nil, fmt.Errorf("模拟角色对话失败: %w", err)
		}
	}

	// 处理同上面GenerateCharacterInteraction中相同的逻辑 - 确保对话有角色ID等
	characterMap := make(map[string]*models.Character)
	for _, char := range characters {
		characterMap[char.Name] = char
	}

	for i := range dialogues {
		if dialogues[i].CharacterID == "" && dialogues[i].CharacterName != "" {
			if char, ok := characterMap[dialogues[i].CharacterName]; ok {
				dialogues[i].CharacterID = char.ID
			} else {
				tempID := fmt.Sprintf("%s_%d", string(dialogues[i].CharacterName[0]), time.Now().UnixNano())
				dialogues[i].CharacterID = tempID
			}
		}

		dialogues[i].Timestamp = time.Now()
	}

	// 记录到上下文系统
	interactionID := fmt.Sprintf("simulation_%d", time.Now().UnixNano())
	for _, dialogue := range dialogues {
		metadata := map[string]interface{}{
			"simulation_id": interactionID,
			"emotion":       dialogue.Emotion,
			"action":        dialogue.Action,
		}

		err := s.ContextService.AddConversation(
			sceneID,
			dialogue.CharacterID,
			dialogue.Message,
			metadata,
		)

		if err != nil {
			if isEnglish {
				fmt.Printf("Failed to record character dialogue: %v\n", err)
			} else {
				fmt.Printf("记录角色对话失败: %v\n", err)
			}
		}
	}

	return dialogues, nil
}
