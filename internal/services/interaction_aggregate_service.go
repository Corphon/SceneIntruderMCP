// internal/services/interaction_aggregate_service.go
package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

type InteractionAggregateService struct {
	CharacterService *CharacterService
	ContextService   *ContextService
	SceneService     *SceneService
	StatsService     *StatsService
	StoryService     *StoryService
	ExportService    *ExportService
}

// InteractionRequest 交互请求
type InteractionRequest struct {
	SceneID      string                 `json:"scene_id"`
	CharacterIDs []string               `json:"character_ids"`
	Message      string                 `json:"message"`
	EmotionData  *EmotionData           `json:"emotion_data,omitempty"`
	Context      map[string]interface{} `json:"context,omitempty"`
	Options      *InteractionOptions    `json:"options,omitempty"`
}

// InteractionOptions 交互选项
type InteractionOptions struct {
	GenerateFollowUps   bool `json:"generate_follow_ups"`
	UpdateStoryProgress bool `json:"update_story_progress"`
	SaveToHistory       bool `json:"save_to_history"`
	TriggerEvents       bool `json:"trigger_events"`
}

// InteractionResult 交互结果
type InteractionResult struct {
	// 角色响应
	Messages []CharacterMessage `json:"messages"`

	// 故事更新
	StoryUpdates *StoryUpdate `json:"story_updates,omitempty"`

	// 角色状态变化
	CharacterStates map[string]CharacterState `json:"character_states"`

	// 新的选择项
	NewChoices []*models.StoryChoice `json:"new_choices,omitempty"`

	// UI更新指令
	UIUpdates *UIUpdateCommands `json:"ui_updates"`

	// 通知消息
	Notifications []Notification `json:"notifications"`

	// 事件触发
	Events []GameEvent `json:"events,omitempty"`

	// 统计信息
	Stats *InteractionStats `json:"stats"`
}

// CharacterMessage 角色消息
type CharacterMessage struct {
	CharacterID   string                 `json:"character_id"`
	CharacterName string                 `json:"character_name"`
	Content       string                 `json:"content"`
	EmotionData   *EmotionData           `json:"emotion_data"`
	Timestamp     time.Time              `json:"timestamp"`
	MessageType   string                 `json:"message_type"` // response, action, thought
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// StoryUpdate 故事更新
type StoryUpdate struct {
	NewNodes        []*models.StoryNode `json:"new_nodes,omitempty"`
	UpdatedNodes    []*models.StoryNode `json:"updated_nodes,omitempty"`
	ProgressChange  float64             `json:"progress_change"`
	UnlockedContent []string            `json:"unlocked_content,omitempty"`
	UpdatedTasks    []*models.Task      `json:"updated_tasks,omitempty"`   // 添加任务更新
	CompletedTasks  []*models.Task      `json:"completed_tasks,omitempty"` // 添加已完成任务
	TaskChanges     []TaskChange        `json:"task_changes,omitempty"`
}

// TaskChange 任务变化记录
type TaskChange struct {
	TaskID    string    `json:"task_id"`
	Type      string    `json:"type"`       // "completed", "updated", "new"
	OldStatus bool      `json:"old_status"` // 原状态
	NewStatus bool      `json:"new_status"` // 新状态
	ChangedAt time.Time `json:"changed_at"`
	Reason    string    `json:"reason"` // 变化原因
}

// CharacterState 角色状态
type CharacterState struct {
	CharacterID     string                 `json:"character_id"`
	Mood            string                 `json:"mood"`
	Energy          float64                `json:"energy"`
	Relationship    map[string]float64     `json:"relationship"` // 与其他角色的关系值
	CurrentActivity string                 `json:"current_activity"`
	StatusEffects   []string               `json:"status_effects"`
	LastUpdated     time.Time              `json:"last_updated"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// UIUpdateCommands UI更新指令
type UIUpdateCommands struct {
	ScrollToBottom      bool           `json:"scroll_to_bottom"`
	HighlightCharacters []string       `json:"highlight_characters"`
	UpdateChatBadges    map[string]int `json:"update_chat_badges"`
	TriggerAnimations   []UIAnimation  `json:"trigger_animations"`
	UpdateTabs          []TabUpdate    `json:"update_tabs"`
}

// UIAnimation UI动画
type UIAnimation struct {
	Target   string                 `json:"target"`   // CSS选择器或元素ID
	Type     string                 `json:"type"`     // fade, slide, bounce, etc.
	Duration int                    `json:"duration"` // 毫秒
	Params   map[string]interface{} `json:"params"`
}

// TabUpdate 标签更新
type TabUpdate struct {
	TabID      string `json:"tab_id"`
	BadgeCount int    `json:"badge_count"`
	IsActive   bool   `json:"is_active"`
	Title      string `json:"title,omitempty"`
}

// Notification 通知
type Notification struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"` // info, success, warning, error, achievement
	Title    string                 `json:"title"`
	Message  string                 `json:"message"`
	Duration int                    `json:"duration"` // 显示时间（毫秒）
	Actions  []NotificationAction   `json:"actions,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NotificationAction 通知操作
type NotificationAction struct {
	Label  string                 `json:"label"`
	Action string                 `json:"action"`
	Style  string                 `json:"style"` // primary, secondary, danger
	Params map[string]interface{} `json:"params,omitempty"`
}

// GameEvent 游戏事件
type GameEvent struct {
	EventType string                 `json:"event_type"`
	EventData map[string]interface{} `json:"event_data"`
	Triggers  []string               `json:"triggers"` // 触发条件
	Effects   []string               `json:"effects"`  // 效果
	Timestamp time.Time              `json:"timestamp"`
}

// InteractionStats 交互统计
type InteractionStats struct {
	TokensUsed          int       `json:"tokens_used"`
	ProcessingTime      int64     `json:"processing_time_ms"`
	CharactersInvolved  int       `json:"characters_involved"`
	MessagesGenerated   int       `json:"messages_generated"`
	EventsTriggered     int       `json:"events_triggered"`
	TotalInteractions   int       `json:"total_interactions"`
	TotalMessages       int       `json:"total_messages"`
	TotalTokensUsed     int       `json:"total_tokens_used"`
	TotalProcessingTime int64     `json:"total_processing_time"`
	TotalTasksCompleted int       `json:"total_tasks_completed"`
	TotalProgressChange float64   `json:"total_progress_change"`
	LastUpdated         time.Time `json:"last_updated"`
}

// KeywordAnalysis 关键词分析结果
type KeywordAnalysis struct {
	ProgressMultiplier float64
	Events             []string
	UnlockTriggers     []string
}

// StoryImpact 故事影响分析结果
type StoryImpact struct {
	ProgressChange          float64
	ShouldCreateNode        bool
	ShouldUpdateCurrentNode bool
	SignificanceLevel       int                // 1-10，交互的重要程度
	EmotionalImpact         float64            // 情绪影响强度
	RelationshipChanges     map[string]float64 // 角色关系变化
	KeyEvents               []string           // 关键事件
	UnlockTriggers          []string           // 解锁触发器
}

// TaskCompletionInfo 任务完成信息
type TaskCompletionInfo struct {
	Task            *models.Task
	MatchedKeywords []string
	CompletionHints int
}

// ------------------------------------
// ProcessInteraction 处理完整的交互流程
func (s *InteractionAggregateService) ProcessInteraction(
	ctx context.Context,
	request *InteractionRequest) (*InteractionResult, error) {

	startTime := time.Now()

	// 设置默认选项
	if request.Options == nil {
		request.Options = &InteractionOptions{
			GenerateFollowUps:   true,
			UpdateStoryProgress: true,
			SaveToHistory:       true,
			TriggerEvents:       true,
		}
	}

	result := &InteractionResult{
		Messages:        []CharacterMessage{},
		CharacterStates: make(map[string]CharacterState),
		UIUpdates:       &UIUpdateCommands{},
		Notifications:   []Notification{},
		Events:          []GameEvent{},
	}

	// 1. 验证场景和角色
	sceneData, err := s.SceneService.LoadScene(request.SceneID)
	if err != nil {
		return nil, fmt.Errorf("加载场景失败: %w", err)
	}

	// 验证角色ID
	validCharacters := make(map[string]*models.Character)
	for _, char := range sceneData.Characters {
		validCharacters[char.ID] = char
	}

	for _, charID := range request.CharacterIDs {
		if _, exists := validCharacters[charID]; !exists {
			return nil, fmt.Errorf("角色ID %s 无效", charID)
		}
	}

	// 2. 生成角色响应
	totalTokens := 0
	successfulResponses := 0
	for _, characterID := range request.CharacterIDs {
		character := validCharacters[characterID]

		// 生成带情绪的响应
		response, err := s.CharacterService.GenerateResponseWithEmotion(
			request.SceneID,
			characterID,
			request.Message)

		if err != nil {
			// 记录错误但继续处理其他角色
			result.Notifications = append(result.Notifications, Notification{
				ID:       fmt.Sprintf("error_%s_%d", characterID, time.Now().UnixNano()),
				Type:     "warning",
				Title:    "角色响应失败",
				Message:  fmt.Sprintf("角色 %s 响应生成失败: %s", character.Name, err.Error()),
				Duration: 5000,
			})
			continue
		}

		// 转换为标准格式
		message := CharacterMessage{
			CharacterID:   characterID,
			CharacterName: character.Name,
			Content:       response.Response,
			EmotionData: &EmotionData{
				Emotion:           response.Emotion,
				Intensity:         response.Intensity,
				BodyLanguage:      response.BodyLanguage,
				FacialExpression:  response.FacialExpression,
				VoiceTone:         response.VoiceTone,
				SecondaryEmotions: response.SecondaryEmotions,
			},
			Timestamp:   time.Now(),
			MessageType: "response",
			Metadata: map[string]interface{}{
				"body_language":      response.BodyLanguage,
				"facial_expression":  response.FacialExpression,
				"voice_tone":         response.VoiceTone,
				"secondary_emotions": response.SecondaryEmotions,
				"tokens_used":        response.TokensUsed,
			},
		}

		result.Messages = append(result.Messages, message)
		totalTokens += response.TokensUsed
		successfulResponses++

		// 更新角色状态
		characterState := CharacterState{
			CharacterID:     characterID,
			Mood:            inferMoodFromEmotion(response.Emotion, response.Intensity),
			Energy:          calculateEnergyLevel(response),
			Relationship:    make(map[string]float64),
			CurrentActivity: extractActivityFromBodyLanguage(response.BodyLanguage),
			StatusEffects:   extractStatusEffects(response.SecondaryEmotions),
		}

		// 计算与其他角色的关系变化
		for _, otherID := range request.CharacterIDs {
			if otherID != characterID {
				relationshipChange := calculateRelationshipChangeFromResponse(response, request.Message)
				characterState.Relationship[otherID] = relationshipChange
			}
		}

		result.CharacterStates[characterID] = characterState
	}
	// 检查是否有成功的响应
	if successfulResponses == 0 {
		return nil, fmt.Errorf("所有角色响应生成都失败了")
	}

	// 3. 更新故事进度（如果启用）
	if request.Options.UpdateStoryProgress {
		storyUpdate, err := s.updateStoryProgress(request, result.Messages)
		if err == nil && storyUpdate != nil {
			result.StoryUpdates = storyUpdate

			// 检查是否有任务完成 - 增强通知信息
			if len(storyUpdate.CompletedTasks) > 0 {
				// 为每个完成的任务添加通知
				for i, completedTask := range storyUpdate.CompletedTasks {
					// 获取对应的任务变化信息
					var matchedKeywords []string
					if i < len(storyUpdate.TaskChanges) {
						// 从Reason字段中提取关键词信息
						reason := storyUpdate.TaskChanges[i].Reason
						if strings.Contains(reason, "关键词:") {
							parts := strings.Split(reason, "关键词:")
							if len(parts) > 1 {
								matchedKeywords = strings.Split(strings.TrimSpace(parts[1]), ", ")
							}
						}
					}
					// 构建更详细的通知消息
					var message string
					if len(matchedKeywords) > 0 {
						message = fmt.Sprintf("恭喜！您已完成任务：%s\n匹配到的关键要素：%s",
							completedTask.Title, strings.Join(matchedKeywords, ", "))
					} else {
						message = fmt.Sprintf("恭喜！您已完成任务：%s", completedTask.Title)
					}

					// 任务完成事件
					if request.Options.TriggerEvents && len(storyUpdate.CompletedTasks) > 0 {
						s.addTaskCompletionEvents(result, storyUpdate.CompletedTasks)
					}

					result.Notifications = append(result.Notifications, Notification{
						ID:       fmt.Sprintf("task_completed_%s_%d", completedTask.ID, time.Now().UnixNano()),
						Type:     "success",
						Title:    "任务完成",
						Message:  message,
						Duration: 5000,
						Actions: []NotificationAction{
							{
								Label:  "查看详情",
								Action: "view_task_details",
								Style:  "primary",
								Params: map[string]interface{}{
									"task_id":          completedTask.ID,
									"matched_keywords": matchedKeywords,
								},
							},
							{
								Label:  "查看奖励",
								Action: "view_task_reward",
								Style:  "secondary",
								Params: map[string]interface{}{
									"task_id": completedTask.ID,
									"reward":  completedTask.Reward,
								},
							},
						},
						Metadata: map[string]interface{}{
							"task_id":          completedTask.ID,
							"task_title":       completedTask.Title,
							"task_description": completedTask.Description,
							"completion_time":  time.Now(),
							"trigger_type":     "interaction_analysis",
							"matched_keywords": matchedKeywords,
							"completion_hints": len(matchedKeywords),
						},
					})
				}

				// 检查是否解锁了成就
				s.checkTaskCompletionAchievements(result, storyUpdate.CompletedTasks)
			}
			// 如果有任务状态变化，添加UI更新指令
			if len(storyUpdate.TaskChanges) > 0 {
				// 更新任务UI
				result.UIUpdates.UpdateTabs = append(result.UIUpdates.UpdateTabs, TabUpdate{
					TabID:      "tasks",
					BadgeCount: len(storyUpdate.CompletedTasks),
					Title:      "任务",
				})

				// 添加任务完成动画
				for _, completedTask := range storyUpdate.CompletedTasks {
					result.UIUpdates.TriggerAnimations = append(result.UIUpdates.TriggerAnimations, UIAnimation{
						Target:   fmt.Sprintf("#task-%s", completedTask.ID),
						Type:     "task_complete",
						Duration: 2000,
						Params: map[string]interface{}{
							"effect": "checkmark_bounce",
							"color":  "#4CAF50",
						},
					})
				}
			}

			// 如果有新内容解锁，添加通知
			if len(storyUpdate.UnlockedContent) > 0 {
				result.Notifications = append(result.Notifications, Notification{
					ID:       fmt.Sprintf("unlock_%d", time.Now().UnixNano()),
					Type:     "success",
					Title:    "新内容解锁",
					Message:  fmt.Sprintf("解锁了 %d 项新内容", len(storyUpdate.UnlockedContent)),
					Duration: 3000,
					Actions: []NotificationAction{{
						Label:  "查看",
						Action: "show_unlocked_content",
						Style:  "primary",
					}},
				})
			}
			// 检查故事进度里程碑
			s.checkStoryProgressMilestones(result, storyUpdate.ProgressChange)
		}
	}

	// 4. 生成新的选择项（如果启用）
	if request.Options.GenerateFollowUps {
		choices, err := s.generateFollowUpChoices(request, result.Messages)
		if err == nil {
			result.NewChoices = choices
		}
	}

	// 5. 保存到历史记录（如果启用）
	if request.Options.SaveToHistory {
		if err := s.saveInteractionToHistory(request, result); err != nil {
			// 记录错误但不影响主流程
			result.Notifications = append(result.Notifications, Notification{
				ID:       fmt.Sprintf("save_error_%d", time.Now().UnixNano()),
				Type:     "warning",
				Title:    "保存失败",
				Message:  "交互历史保存失败，但不影响当前对话",
				Duration: 3000,
			})
		}
	}

	// 6. 触发游戏事件（如果启用）
	if request.Options.TriggerEvents {
		events := s.checkAndTriggerEvents(request, result)
		result.Events = events
	}

	// 7. 构建UI更新指令
	result.UIUpdates = s.buildUIUpdateCommands(request, result)

	// 8. 记录统计信息
	processingTime := time.Since(startTime).Milliseconds()
	result.Stats = &InteractionStats{
		TokensUsed:         totalTokens,
		ProcessingTime:     processingTime,
		CharactersInvolved: len(request.CharacterIDs),
		MessagesGenerated:  len(result.Messages),
		EventsTriggered:    len(result.Events),
	}

	// 9. 更新全局统计
	s.StatsService.RecordAPIRequest(totalTokens)

	return result, nil
}

// 辅助函数实现
// checkTaskCompletionAchievements 检查任务完成成就
func (s *InteractionAggregateService) checkTaskCompletionAchievements(
	result *InteractionResult,
	completedTasks []*models.Task) {

	// 检查是否达成特定成就
	taskCount := len(completedTasks)

	if taskCount >= 3 {
		result.Notifications = append(result.Notifications, Notification{
			ID:       fmt.Sprintf("achievement_task_master_%d", time.Now().UnixNano()),
			Type:     "achievement",
			Title:    "任务大师",
			Message:  "在单次交互中完成了3个或更多任务！",
			Duration: 8000,
			Actions: []NotificationAction{{
				Label:  "查看成就",
				Action: "view_achievement",
				Style:  "primary",
				Params: map[string]interface{}{
					"achievement_id": "task_master",
				},
			}},
			Metadata: map[string]interface{}{
				"achievement_type": "task_completion",
				"tasks_completed":  taskCount,
			},
		})
	}

	// 检查特定类型的任务完成
	for _, task := range completedTasks {
		if strings.Contains(strings.ToLower(task.Title), "主要") ||
			strings.Contains(strings.ToLower(task.Title), "main") {
			result.Notifications = append(result.Notifications, Notification{
				ID:       fmt.Sprintf("main_task_completed_%d", time.Now().UnixNano()),
				Type:     "achievement",
				Title:    "故事推进者",
				Message:  "完成了重要的主线任务！",
				Duration: 7000,
			})
		}
	}
}

// checkStoryProgressMilestones 检查故事进度里程碑
func (s *InteractionAggregateService) checkStoryProgressMilestones(
	result *InteractionResult,
	progressChange float64) {

	// 如果进度变化较大，添加特殊通知
	if progressChange >= 10.0 {
		result.Notifications = append(result.Notifications, Notification{
			ID:       fmt.Sprintf("major_progress_%d", time.Now().UnixNano()),
			Type:     "success",
			Title:    "重大进展",
			Message:  fmt.Sprintf("故事进度大幅推进了 %.1f%%！", progressChange),
			Duration: 4000,
			Actions: []NotificationAction{{
				Label:  "查看故事",
				Action: "view_story_progress",
				Style:  "primary",
			}},
		})
	}
}

// 添加任务完成事件到游戏事件中
func (s *InteractionAggregateService) addTaskCompletionEvents(
	result *InteractionResult,
	completedTasks []*models.Task) {

	for _, task := range completedTasks {
		event := GameEvent{
			EventType: "task_completion",
			EventData: map[string]interface{}{
				"task_id":          task.ID,
				"task_title":       task.Title,
				"task_description": task.Description,
				"completion_time":  time.Now(),
				"reward":           task.Reward,
			},
			Triggers:  []string{"interaction_analysis", "keyword_match"},
			Effects:   []string{"progress_increase", "content_unlock"},
			Timestamp: time.Now(),
		}
		result.Events = append(result.Events, event)
	}
}

// 计算能量级别函数
func calculateEnergyLevel(response *models.EmotionalResponse) float64 {
	// 根据情绪计算能量级别
	baseEnergy := 0.5

	emotion := strings.ToLower(response.Emotion)
	switch emotion {
	case "joy", "喜悦", "高兴", "excited", "enthusiastic":
		baseEnergy = 0.8
	case "sadness", "悲伤", "难过", "tired", "depressed":
		baseEnergy = 0.2
	case "anger", "愤怒", "生气", "frustrated":
		baseEnergy = 0.7
	case "fear", "恐惧", "害怕":
		baseEnergy = 0.3
	case "surprise", "惊讶":
		baseEnergy = 0.6
	case "neutral", "中性", "calm", "peaceful":
		baseEnergy = 0.5
	}

	// 根据强度调整（1-10的范围）
	intensityFactor := float64(response.Intensity) / 10.0
	energy := baseEnergy + (intensityFactor-0.5)*0.3

	// 确保在合理范围内
	if energy < 0.1 {
		energy = 0.1
	} else if energy > 1.0 {
		energy = 1.0
	}

	return energy
}

// 根据情绪和强度推断心情
func inferMoodFromEmotion(emotion string, intensity int) string {
	moodMap := map[string]map[string]string{
		"joy": {
			"low":    "content",
			"medium": "happy",
			"high":   "euphoric",
		},
		"anger": {
			"low":    "annoyed",
			"medium": "angry",
			"high":   "furious",
		},
		"sadness": {
			"low":    "melancholy",
			"medium": "sad",
			"high":   "depressed",
		},
		"fear": {
			"low":    "nervous",
			"medium": "afraid",
			"high":   "terrified",
		},
		"surprise": {
			"low":    "curious",
			"medium": "surprised",
			"high":   "shocked",
		},
		"neutral": {
			"low":    "calm",
			"medium": "neutral",
			"high":   "composed",
		},
	}

	// 根据强度分级
	var intensityLevel string
	if intensity <= 3 {
		intensityLevel = "low"
	} else if intensity <= 7 {
		intensityLevel = "medium"
	} else {
		intensityLevel = "high"
	}

	// 标准化情绪名称
	normalizedEmotion := strings.ToLower(emotion)
	switch normalizedEmotion {
	case "喜悦", "高兴", "快乐", "happy":
		normalizedEmotion = "joy"
	case "愤怒", "生气", "angry":
		normalizedEmotion = "anger"
	case "悲伤", "难过", "sad":
		normalizedEmotion = "sadness"
	case "恐惧", "害怕", "fear":
		normalizedEmotion = "fear"
	case "惊讶", "surprise":
		normalizedEmotion = "surprise"
	default:
		normalizedEmotion = "neutral"
	}

	if moods, exists := moodMap[normalizedEmotion]; exists {
		if mood, exists := moods[intensityLevel]; exists {
			return mood
		}
	}

	return "neutral"
}

// 从身体语言提取活动
func extractActivityFromBodyLanguage(bodyLanguage string) string {
	if bodyLanguage == "" {
		return "talking"
	}

	bodyLanguage = strings.ToLower(bodyLanguage)

	// 中英文关键词匹配
	activities := map[string]string{
		"走":     "walking",
		"跑":     "running",
		"坐":     "sitting",
		"站":     "standing",
		"躺":     "lying",
		"思考":    "thinking",
		"笑":     "laughing",
		"哭":     "crying",
		"点头":    "nodding",
		"摇头":    "shaking_head",
		"挥手":    "waving",
		"拥抱":    "hugging",
		"握手":    "handshaking",
		"walk":  "walking",
		"sit":   "sitting",
		"stand": "standing",
		"think": "thinking",
		"laugh": "laughing",
		"cry":   "crying",
		"nod":   "nodding",
		"wave":  "waving",
		"hug":   "hugging",
	}

	for keyword, activity := range activities {
		if strings.Contains(bodyLanguage, keyword) {
			return activity
		}
	}

	return "talking"
}

// 从次要情绪提取状态效果
func extractStatusEffects(secondaryEmotions []string) []string {
	if len(secondaryEmotions) == 0 {
		return []string{}
	}

	statusEffects := []string{}

	for _, emotion := range secondaryEmotions {
		emotion = strings.ToLower(emotion)
		switch emotion {
		case "confusion", "困惑":
			statusEffects = append(statusEffects, "confused")
		case "excitement", "兴奋":
			statusEffects = append(statusEffects, "energized")
		case "exhaustion", "疲惫":
			statusEffects = append(statusEffects, "tired")
		case "curiosity", "好奇":
			statusEffects = append(statusEffects, "curious")
		case "confidence", "自信":
			statusEffects = append(statusEffects, "confident")
		case "nervousness", "紧张":
			statusEffects = append(statusEffects, "nervous")
		}
	}

	return statusEffects
}

// 计算关系变化函数
func calculateRelationshipChangeFromResponse(response *models.EmotionalResponse, message string) float64 {
	// 根据情绪和消息内容计算关系变化
	baseChange := 0.0

	emotion := strings.ToLower(response.Emotion)
	switch emotion {
	case "joy", "喜悦", "高兴", "grateful", "friendly":
		baseChange = 0.1
	case "anger", "愤怒", "生气", "frustrated", "annoyed":
		baseChange = -0.1
	case "sadness", "悲伤", "难过", "disappointed":
		baseChange = -0.05
	case "excitement", "兴奋", "enthusiastic":
		baseChange = 0.15
	case "fear", "恐惧", "害怕":
		baseChange = -0.02
	}

	// 根据强度调整
	intensityFactor := float64(response.Intensity) / 10.0
	baseChange *= intensityFactor

	// 根据消息内容调整
	messageLower := strings.ToLower(message)
	if strings.Contains(messageLower, "谢谢") || strings.Contains(messageLower, "thank") {
		baseChange += 0.05
	}
	if strings.Contains(messageLower, "抱歉") || strings.Contains(messageLower, "sorry") {
		baseChange += 0.03
	}
	if strings.Contains(messageLower, "喜欢") || strings.Contains(messageLower, "like") {
		baseChange += 0.08
	}
	// 检查身体语言的积极性
	bodyLanguage := strings.ToLower(response.BodyLanguage)
	if strings.Contains(bodyLanguage, "微笑") || strings.Contains(bodyLanguage, "smile") {
		baseChange += 0.03
	}
	if strings.Contains(bodyLanguage, "拥抱") || strings.Contains(bodyLanguage, "hug") {
		baseChange += 0.05
	}

	return baseChange
}

// updateStoryProgress 更新故事进度
func (s *InteractionAggregateService) updateStoryProgress(
	request *InteractionRequest,
	messages []CharacterMessage) (*StoryUpdate, error) {

	// 1. 获取故事服务实例
	storyService := s.getStoryService()
	if storyService == nil {
		return nil, fmt.Errorf("故事服务未初始化")
	}

	// 2. 获取当前故事数据
	currentStory, err := storyService.GetStoryData(request.SceneID, nil)
	if err != nil {
		return nil, fmt.Errorf("获取故事数据失败: %w", err)
	}

	// 3. 分析角色交互内容，确定故事影响
	storyImpact := s.analyzeInteractionStoryImpact(request, messages, currentStory)

	// 4. 构建故事更新结果
	storyUpdate := &StoryUpdate{
		NewNodes:        []*models.StoryNode{},
		UpdatedNodes:    []*models.StoryNode{},
		ProgressChange:  storyImpact.ProgressChange,
		UnlockedContent: []string{},
		UpdatedTasks:    []*models.Task{}, // 初始化任务更新
		CompletedTasks:  []*models.Task{}, // 初始化已完成任务
		TaskChanges:     []TaskChange{},   // 初始化任务变化
	}

	// 5. 基于交互内容创建新的故事节点（如果有重要事件）
	if storyImpact.ShouldCreateNode {
		newNode, err := s.createStoryNodeFromInteraction(request, messages, currentStory)
		if err == nil && newNode != nil {
			storyUpdate.NewNodes = append(storyUpdate.NewNodes, newNode)

			// 将新节点添加到当前故事数据中
			currentStory.Nodes = append(currentStory.Nodes, *newNode)
		}
	}

	// 6. 更新现有故事节点（如果交互影响了当前节点）
	if storyImpact.ShouldUpdateCurrentNode {
		updatedNode := s.updateCurrentStoryNode(currentStory, storyImpact)
		if updatedNode != nil {
			storyUpdate.UpdatedNodes = append(storyUpdate.UpdatedNodes, updatedNode)
		}
	}

	// 7. 检查任务完成情况
	taskUpdates := s.checkTaskCompletionFromInteractionEnhanced(request, messages, currentStory)
	if len(taskUpdates) > 0 {
		// 更新任务状态到故事数据并记录变化
		for _, taskInfo := range taskUpdates {
			taskUpdate := taskInfo.Task
			matchedKeywords := taskInfo.MatchedKeywords

			for i := range currentStory.Tasks {
				if currentStory.Tasks[i].ID == taskUpdate.ID {
					oldStatus := currentStory.Tasks[i].Completed
					currentStory.Tasks[i] = *taskUpdate

					// 记录任务变化，包含匹配的关键词
					taskChange := TaskChange{
						TaskID:    taskUpdate.ID,
						Type:      "completed",
						OldStatus: oldStatus,
						NewStatus: taskUpdate.Completed,
						ChangedAt: time.Now(),
						Reason:    fmt.Sprintf("自动检测到任务完成关键词: %s", strings.Join(matchedKeywords, ", ")),
					}
					storyUpdate.TaskChanges = append(storyUpdate.TaskChanges, taskChange)

					// 如果任务刚刚完成，添加到已完成任务列表
					if !oldStatus && taskUpdate.Completed {
						storyUpdate.CompletedTasks = append(storyUpdate.CompletedTasks, taskUpdate)
					}

					// 添加到更新任务列表
					storyUpdate.UpdatedTasks = append(storyUpdate.UpdatedTasks, taskUpdate)
					break
				}
			}
		}
	}

	// 8. 检查解锁内容
	unlockedContent := s.checkUnlockedContent(storyImpact, currentStory)
	storyUpdate.UnlockedContent = unlockedContent

	// 9. 更新故事进度百分比
	newProgress := currentStory.Progress + int(storyImpact.ProgressChange)
	if newProgress > 100 {
		newProgress = 100
	}
	currentStory.Progress = newProgress

	// 10. 保存更新后的故事数据
	if err := s.saveUpdatedStoryData(request.SceneID, currentStory); err != nil {
		return nil, fmt.Errorf("保存故事数据失败: %w", err)
	}

	return storyUpdate, nil
}

// checkTaskCompletionFromInteractionEnhanced 检查交互是否完成了任务（增强版本）
func (s *InteractionAggregateService) checkTaskCompletionFromInteractionEnhanced(
	request *InteractionRequest,
	messages []CharacterMessage,
	currentStory *models.StoryData) []*TaskCompletionInfo {

	completionInfos := []*TaskCompletionInfo{}

	// 分析消息内容，检查是否包含任务完成的线索
	allText := strings.ToLower(request.Message)
	for _, message := range messages {
		allText += " " + strings.ToLower(message.Content)
	}

	// 检查每个未完成的任务
	for _, task := range currentStory.Tasks {
		if task.Completed {
			continue
		}

		// 检查任务相关关键词
		taskKeywords := s.extractTaskKeywords(task)
		completionHints := 0
		matchedKeywords := []string{}

		for _, keyword := range taskKeywords {
			if strings.Contains(allText, strings.ToLower(keyword)) {
				completionHints++
				matchedKeywords = append(matchedKeywords, keyword)
			}
		}

		// 如果找到足够的完成线索，标记任务为完成
		if completionHints >= 2 {
			updatedTask := task
			updatedTask.Completed = true

			// 更新目标完成状态
			for i := range updatedTask.Objectives {
				updatedTask.Objectives[i].Completed = true
			}

			// 使用匹配的关键词丰富任务描述
			if updatedTask.Description != "" {
				keywordsList := strings.Join(matchedKeywords, ", ")
				updatedTask.Description += fmt.Sprintf(" [已完成 - %s，匹配关键词: %s]",
					time.Now().Format("2006-01-02 15:04"), keywordsList)
			}

			// 创建任务完成信息
			completionInfo := &TaskCompletionInfo{
				Task:            &updatedTask,
				MatchedKeywords: matchedKeywords,
				CompletionHints: completionHints,
			}

			completionInfos = append(completionInfos, completionInfo)
		}
	}

	return completionInfos
}

// analyzeInteractionStoryImpact 分析交互对故事的影响
func (s *InteractionAggregateService) analyzeInteractionStoryImpact(
	request *InteractionRequest,
	messages []CharacterMessage,
	currentStory *models.StoryData) *StoryImpact {

	impact := &StoryImpact{
		ProgressChange:          0.0,
		ShouldCreateNode:        false,
		ShouldUpdateCurrentNode: false,
		SignificanceLevel:       1,
		EmotionalImpact:         0.0,
		RelationshipChanges:     make(map[string]float64),
		KeyEvents:               []string{},
		UnlockTriggers:          []string{},
	}

	// 1. 分析交互的重要性
	significance := s.calculateInteractionSignificance(request, messages)
	impact.SignificanceLevel = significance

	// 2. 计算基础进度变化
	baseProgressChange := 0.02 // 基础2%进度

	// 根据当前进度调整增长率（早期进度增长更快）
	progressFactor := 1.0
	if currentStory.Progress < 25 {
		progressFactor = 1.5 // 早期阶段进度增长更快
	} else if currentStory.Progress > 75 {
		progressFactor = 0.7 // 后期阶段进度增长更慢
	}

	// 根据重要性调整进度
	switch {
	case significance >= 8:
		impact.ProgressChange = baseProgressChange * 3.0 * progressFactor
		impact.ShouldCreateNode = true
	case significance >= 6:
		impact.ProgressChange = baseProgressChange * 2.0 * progressFactor
		impact.ShouldUpdateCurrentNode = true
	case significance >= 4:
		impact.ProgressChange = baseProgressChange * 1.5 * progressFactor
	default:
		impact.ProgressChange = baseProgressChange * progressFactor
	}

	if len(currentStory.Tasks) > 0 {
		allText := strings.ToLower(request.Message)
		for _, message := range messages {
			allText += " " + strings.ToLower(message.Content)
		}

		for _, task := range currentStory.Tasks {
			if !task.Completed {
				taskKeywords := s.extractTaskKeywords(task)
				for _, keyword := range taskKeywords {
					if strings.Contains(allText, strings.ToLower(keyword)) {
						impact.ProgressChange += 0.02 // 与任务相关增加2%进度
						impact.KeyEvents = append(impact.KeyEvents,
							fmt.Sprintf("task_related_%s", task.ID))
						break
					}
				}
			}
		}
	}

	if len(currentStory.Nodes) < 5 || impact.SignificanceLevel >= 7 {
		impact.ShouldCreateNode = true
	} else if len(currentStory.Nodes) > 10 {
		// 节点过多时，优先更新现有节点而不是创建新节点
		impact.ShouldUpdateCurrentNode = true
		impact.ShouldCreateNode = false
	}

	if len(currentStory.Locations) > 0 {
		allText := strings.ToLower(request.Message)
		for _, message := range messages {
			allText += " " + strings.ToLower(message.Content)
		}

		locationKeywords := []string{"去", "到", "探索", "寻找", "go", "visit", "explore", "find"}
		for _, keyword := range locationKeywords {
			if strings.Contains(allText, keyword) {
				impact.UnlockTriggers = append(impact.UnlockTriggers, "location_exploration")
				break
			}
		}
	}

	// 3. 分析情绪影响
	totalEmotionalIntensity := 0.0
	for _, message := range messages {
		if message.EmotionData != nil {
			intensity := float64(message.EmotionData.Intensity) / 10.0
			totalEmotionalIntensity += intensity

			// 检查特殊情绪触发事件
			if intensity > 0.8 {
				impact.KeyEvents = append(impact.KeyEvents,
					fmt.Sprintf("strong_%s_emotion", message.EmotionData.Emotion))
			}
		}
	}
	impact.EmotionalImpact = totalEmotionalIntensity / float64(len(messages))

	// 4. 分析消息内容中的关键词
	keywordAnalysis := s.analyzeMessageKeywords(request.Message, messages)
	impact.ProgressChange *= keywordAnalysis.ProgressMultiplier
	impact.KeyEvents = append(impact.KeyEvents, keywordAnalysis.Events...)
	impact.UnlockTriggers = append(impact.UnlockTriggers, keywordAnalysis.UnlockTriggers...)

	// 5. 检查角色关系变化
	for _, message := range messages {
		if len(request.CharacterIDs) > 1 {
			// 多角色交互可能影响关系
			for _, otherCharID := range request.CharacterIDs {
				if otherCharID != message.CharacterID {
					relationshipChange := s.calculateRelationshipChangeFromMessage(message, request.Message)
					impact.RelationshipChanges[fmt.Sprintf("%s-%s", message.CharacterID, otherCharID)] = relationshipChange
				}
			}
		}
	}

	// 6. 检查是否触发故事分支
	if impact.EmotionalImpact > 0.7 || len(impact.KeyEvents) > 2 {
		impact.ShouldCreateNode = true
		impact.ProgressChange += 0.03 // 额外3%进度
	}

	return impact
}

// calculateInteractionSignificance 计算交互重要性 (1-10)
func (s *InteractionAggregateService) calculateInteractionSignificance(
	request *InteractionRequest,
	messages []CharacterMessage) int {

	significance := 1

	// 1. 基于参与角色数量
	switch len(request.CharacterIDs) {
	case 1:
		significance += 1
	case 2:
		significance += 2
	case 3:
		significance += 3
	default:
		significance += 4 // 多角色交互很重要
	}

	// 2. 基于情绪强度
	maxIntensity := 0
	for _, message := range messages {
		if message.EmotionData != nil && message.EmotionData.Intensity > maxIntensity {
			maxIntensity = message.EmotionData.Intensity
		}
	}
	significance += maxIntensity / 2 // 最大强度的一半

	// 3. 基于消息长度和复杂性
	totalMessageLength := len(request.Message)
	for _, message := range messages {
		totalMessageLength += len(message.Content)
	}

	if totalMessageLength > 500 {
		significance += 2
	} else if totalMessageLength > 200 {
		significance += 1
	}

	// 4. 基于特殊关键词
	importantKeywords := []string{
		"重要", "关键", "秘密", "发现", "真相", "危险", "死亡", "爱情", "背叛",
		"important", "key", "secret", "discover", "truth", "danger", "death", "love", "betray",
	}

	allText := strings.ToLower(request.Message)
	for _, message := range messages {
		allText += " " + strings.ToLower(message.Content)
	}

	for _, keyword := range importantKeywords {
		if strings.Contains(allText, keyword) {
			significance += 1
		}
	}

	// 确保在合理范围内
	if significance > 10 {
		significance = 10
	} else if significance < 1 {
		significance = 1
	}

	return significance
}

// analyzeMessageKeywords 分析消息中的关键词
func (s *InteractionAggregateService) analyzeMessageKeywords(
	userMessage string,
	messages []CharacterMessage) *KeywordAnalysis {

	analysis := &KeywordAnalysis{
		ProgressMultiplier: 1.0,
		Events:             []string{},
		UnlockTriggers:     []string{},
	}

	// 合并所有文本内容
	allText := strings.ToLower(userMessage)
	for _, message := range messages {
		allText += " " + strings.ToLower(message.Content)
	}

	// 进度影响关键词
	progressKeywords := map[string]float64{
		"突破":           1.5,
		"发现":           1.3,
		"解决":           1.4,
		"完成":           1.2,
		"成功":           1.2,
		"失败":           0.8,
		"困难":           0.9,
		"阻碍":           0.8,
		"breakthrough": 1.5,
		"discover":     1.3,
		"solve":        1.4,
		"complete":     1.2,
		"success":      1.2,
		"failure":      0.8,
		"difficult":    0.9,
		"obstacle":     0.8,
	}

	// 事件触发关键词
	eventKeywords := map[string]string{
		"战斗":       "combat_initiated",
		"逃跑":       "escape_attempt",
		"探索":       "exploration_started",
		"调查":       "investigation_started",
		"谈判":       "negotiation_started",
		"romance":  "romance_development",
		"conflict": "conflict_escalation",
		"alliance": "alliance_formed",
		"betrayal": "betrayal_detected",
		"mystery":  "mystery_deepened",
	}

	// 解锁触发关键词
	unlockKeywords := map[string]string{
		"钥匙":       "key_obtained",
		"密码":       "password_learned",
		"地图":       "map_revealed",
		"线索":       "clue_discovered",
		"信息":       "information_gained",
		"key":      "key_obtained",
		"password": "password_learned",
		"map":      "map_revealed",
		"clue":     "clue_discovered",
		"info":     "information_gained",
	}

	// 分析进度影响
	for keyword, multiplier := range progressKeywords {
		if strings.Contains(allText, keyword) {
			analysis.ProgressMultiplier *= multiplier
		}
	}

	// 分析事件触发
	for keyword, event := range eventKeywords {
		if strings.Contains(allText, keyword) {
			analysis.Events = append(analysis.Events, event)
		}
	}

	// 分析解锁触发
	for keyword, trigger := range unlockKeywords {
		if strings.Contains(allText, keyword) {
			analysis.UnlockTriggers = append(analysis.UnlockTriggers, trigger)
		}
	}

	return analysis
}

// createStoryNodeFromInteraction 基于交互创建故事节点
func (s *InteractionAggregateService) createStoryNodeFromInteraction(
	request *InteractionRequest,
	messages []CharacterMessage,
	currentStory *models.StoryData) (*models.StoryNode, error) {

	// 获取故事服务
	storyService := s.getStoryService()
	if storyService == nil {
		return nil, fmt.Errorf("故事服务未初始化")
	}

	// ✅ 利用 currentStory 信息构建更智能的节点内容
	content := s.buildNodeContentFromInteraction(request, messages)

	// ✅ 根据当前故事状态增强内容
	content = s.enhanceContentWithStoryContext(content, currentStory, messages)

	// ✅ 根据故事进度确定节点类型
	nodeType := s.determineNodeType(currentStory, messages)

	// ✅ 根据已有节点数量生成合适的ID
	nodeIndex := len(currentStory.Nodes) + 1
	nodeID := fmt.Sprintf("interaction_node_%s_%d", currentStory.SceneID, nodeIndex)

	newNode := &models.StoryNode{
		ID:         nodeID,
		SceneID:    request.SceneID,
		Content:    content,
		Type:       nodeType,
		IsRevealed: true,
		CreatedAt:  time.Now(),
		Source:     models.SourceGenerated,
		Choices:    []models.StoryChoice{},
		Metadata:   make(map[string]interface{}),
	}

	// ✅ 利用故事状态设置节点元数据
	newNode.Metadata["title"] = s.generateNodeTitle(currentStory, messages)
	newNode.Metadata["story_progress"] = currentStory.Progress
	newNode.Metadata["story_state"] = currentStory.CurrentState
	newNode.Metadata["interaction_type"] = "character_dialogue"
	newNode.Metadata["character_ids"] = request.CharacterIDs
	newNode.Metadata["user_message"] = request.Message
	newNode.Metadata["node_index"] = nodeIndex
	newNode.Metadata["is_current_active"] = true

	// ✅ 检查是否与现有任务相关
	relatedTasks := s.findRelatedTasks(currentStory.Tasks, request.Message, messages)
	if len(relatedTasks) > 0 {
		newNode.Metadata["related_tasks"] = relatedTasks
	}

	// ✅ 检查是否与地点相关
	relatedLocations := s.findRelatedLocations(currentStory.Locations, request.Message, messages)
	if len(relatedLocations) > 0 {
		newNode.Metadata["related_locations"] = relatedLocations
	}

	// ✅ 基于故事状态生成选择项
	choices := s.generateChoicesWithStoryContext(messages, currentStory)
	newNode.Choices = choices

	return newNode, nil
}

// 增强内容的辅助方法
func (s *InteractionAggregateService) enhanceContentWithStoryContext(
	baseContent string,
	currentStory *models.StoryData,
	messages []CharacterMessage) string {

	var enhancedContent strings.Builder
	enhancedContent.WriteString(baseContent)

	// 添加故事背景信息
	enhancedContent.WriteString("\n\n---\n\n")
	enhancedContent.WriteString("## 故事背景\n\n")
	enhancedContent.WriteString(fmt.Sprintf("**当前状态**: %s\n", currentStory.CurrentState))
	enhancedContent.WriteString(fmt.Sprintf("**故事进度**: %d%%\n", currentStory.Progress))

	// ✅ 利用 messages 参数分析角色情绪和状态
	if len(messages) > 0 {
		enhancedContent.WriteString("\n## 角色状态分析\n\n")

		// 分析每个角色的情绪状态
		for _, message := range messages {
			if message.EmotionData != nil {
				enhancedContent.WriteString(fmt.Sprintf("**%s**: %s (强度: %d",
					message.CharacterName,
					message.EmotionData.Emotion,
					message.EmotionData.Intensity))

				if message.EmotionData.BodyLanguage != "" {
					enhancedContent.WriteString(fmt.Sprintf(", 行为: %s", message.EmotionData.BodyLanguage))
				}
				if message.EmotionData.VoiceTone != "" {
					enhancedContent.WriteString(fmt.Sprintf(", 语调: %s", message.EmotionData.VoiceTone))
				}
				enhancedContent.WriteString(")\n")
			}
		}
	}

	// ✅ 利用 messages 内容分析与任务的关联
	allText := strings.ToLower(baseContent)
	for _, message := range messages {
		allText += " " + strings.ToLower(message.Content)
	}

	relatedTaskCount := 0
	for _, task := range currentStory.Tasks {
		if !task.Completed {
			taskKeywords := s.extractTaskKeywords(task)
			for _, keyword := range taskKeywords {
				if strings.Contains(allText, strings.ToLower(keyword)) {
					if relatedTaskCount == 0 {
						enhancedContent.WriteString("\n## 相关任务\n\n")
					}
					enhancedContent.WriteString(fmt.Sprintf("- **%s**: %s\n", task.Title, task.Description))
					relatedTaskCount++
					break
				}
			}
		}
	}

	// ✅ 基于 messages 的情绪数据添加场景氛围描述
	if len(messages) > 0 {
		atmosphereDescription := s.generateAtmosphereFromMessages(messages)
		if atmosphereDescription != "" {
			enhancedContent.WriteString(fmt.Sprintf("\n## 场景氛围\n\n%s\n", atmosphereDescription))
		}
	}

	// ✅ 基于 messages 的时间信息添加时序说明
	if len(messages) > 1 {
		enhancedContent.WriteString("\n## 对话时序\n\n")
		enhancedContent.WriteString(fmt.Sprintf("本次互动包含 %d 条角色响应，", len(messages)))

		// 计算对话的时间跨度
		if len(messages) >= 2 {
			firstTime := messages[0].Timestamp
			lastTime := messages[len(messages)-1].Timestamp
			duration := lastTime.Sub(firstTime)

			if duration > 0 {
				enhancedContent.WriteString(fmt.Sprintf("对话历时 %.1f 秒。", duration.Seconds()))
			} else {
				enhancedContent.WriteString("响应几乎同时产生。")
			}
		}
		enhancedContent.WriteString("\n")
	}

	// ✅ 分析角色互动模式
	if len(messages) > 1 {
		interactionPattern := s.analyzeInteractionPattern(messages)
		if interactionPattern != "" {
			enhancedContent.WriteString(fmt.Sprintf("\n## 互动模式\n\n%s\n", interactionPattern))
		}
	}

	return enhancedContent.String()
}

// 新增辅助方法：根据消息生成场景氛围描述
func (s *InteractionAggregateService) generateAtmosphereFromMessages(messages []CharacterMessage) string {
	if len(messages) == 0 {
		return ""
	}

	// 收集所有情绪数据
	emotionCounts := make(map[string]int)
	totalIntensity := 0
	bodyLanguageElements := []string{}
	voiceToneElements := []string{}

	for _, message := range messages {
		if message.EmotionData != nil {
			emotion := strings.ToLower(message.EmotionData.Emotion)
			emotionCounts[emotion]++
			totalIntensity += message.EmotionData.Intensity

			if message.EmotionData.BodyLanguage != "" {
				bodyLanguageElements = append(bodyLanguageElements, message.EmotionData.BodyLanguage)
			}
			if message.EmotionData.VoiceTone != "" {
				voiceToneElements = append(voiceToneElements, message.EmotionData.VoiceTone)
			}
		}
	}

	if len(emotionCounts) == 0 {
		return ""
	}

	var atmosphere strings.Builder

	// 分析主导情绪
	dominantEmotion := ""
	maxCount := 0
	for emotion, count := range emotionCounts {
		if count > maxCount {
			maxCount = count
			dominantEmotion = emotion
		}
	}

	// 计算平均强度
	avgIntensity := float64(totalIntensity) / float64(len(messages))

	// 生成氛围描述
	switch dominantEmotion {
	case "joy", "喜悦", "高兴":
		if avgIntensity > 7 {
			atmosphere.WriteString("场景充满了欢声笑语，")
		} else {
			atmosphere.WriteString("现场氛围轻松愉快，")
		}
	case "anger", "愤怒", "生气":
		if avgIntensity > 7 {
			atmosphere.WriteString("空气中弥漫着紧张的火药味，")
		} else {
			atmosphere.WriteString("气氛略显紧张，")
		}
	case "sadness", "悲伤", "难过":
		atmosphere.WriteString("现场笼罩着一层淡淡的忧郁，")
	case "fear", "恐惧", "害怕":
		atmosphere.WriteString("不安的情绪在空气中蔓延，")
	case "surprise", "惊讶":
		atmosphere.WriteString("意外的发现让现场充满了惊喜，")
	default:
		atmosphere.WriteString("场景保持着平静的基调，")
	}

	// 添加身体语言描述
	if len(bodyLanguageElements) > 0 {
		uniqueBodyLanguage := removeDuplicates(bodyLanguageElements)
		atmosphere.WriteString(fmt.Sprintf("角色们的行为表现为：%s。", strings.Join(uniqueBodyLanguage, "、")))
	}

	// 添加语调描述
	if len(voiceToneElements) > 0 {
		uniqueVoiceTones := removeDuplicates(voiceToneElements)
		atmosphere.WriteString(fmt.Sprintf("对话中的语调变化包括：%s。", strings.Join(uniqueVoiceTones, "、")))
	}
	return atmosphere.String()
}

// 新增辅助方法：分析角色互动模式
func (s *InteractionAggregateService) analyzeInteractionPattern(messages []CharacterMessage) string {
	if len(messages) < 2 {
		return ""
	}

	// 分析情绪变化趋势
	emotionChanges := []string{}
	for i := 1; i < len(messages); i++ {
		prev := messages[i-1]
		curr := messages[i]

		if prev.EmotionData != nil && curr.EmotionData != nil {
			prevIntensity := prev.EmotionData.Intensity
			currIntensity := curr.EmotionData.Intensity

			if currIntensity > prevIntensity+2 {
				emotionChanges = append(emotionChanges, "情绪升级")
			} else if currIntensity < prevIntensity-2 {
				emotionChanges = append(emotionChanges, "情绪缓和")
			}
		}
	}

	// 检查角色数量和互动类型
	characterCount := len(messages)
	var pattern strings.Builder

	if characterCount == 2 {
		pattern.WriteString("这是一次双向对话，")
	} else {
		pattern.WriteString(fmt.Sprintf("这是一次涉及 %d 位角色的群体互动，", characterCount))
	}

	if len(emotionChanges) > 0 {
		pattern.WriteString(fmt.Sprintf("对话过程中出现了%s。", strings.Join(emotionChanges, "和")))
	} else {
		pattern.WriteString("各角色情绪保持相对稳定。")
	}

	return pattern.String()
}

// 辅助函数：去除重复元素
func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

// 根据故事状态确定节点类型
func (s *InteractionAggregateService) determineNodeType(
	currentStory *models.StoryData,
	messages []CharacterMessage) string {

	// 基础类型基于故事进度
	baseType := ""
	switch {
	case currentStory.Progress < 25:
		baseType = "early_interaction"
	case currentStory.Progress < 50:
		baseType = "development_interaction"
	case currentStory.Progress < 75:
		baseType = "climax_interaction"
	case currentStory.Progress >= 75:
		baseType = "resolution_interaction"
	default:
		baseType = "interaction"
	}

	// ✅ 基于 messages 的内容和情绪进一步细化类型
	if len(messages) == 0 {
		return baseType
	}

	// 分析消息特征
	highIntensityCount := 0
	conflictKeywords := 0
	romanceKeywords := 0
	mysteryKeywords := 0
	actionKeywords := 0
	multiCharacterInteraction := len(messages) > 1

	allText := ""
	for _, message := range messages {
		allText += strings.ToLower(message.Content) + " "

		// 分析情绪强度
		if message.EmotionData != nil && message.EmotionData.Intensity > 7 {
			highIntensityCount++
		}
	}

	// 检查特殊关键词类型
	conflictWords := []string{"战斗", "冲突", "愤怒", "争吵", "敌人", "攻击", "fight", "conflict", "angry", "enemy", "attack"}
	romanceWords := []string{"爱", "喜欢", "心动", "浪漫", "亲吻", "love", "like", "romantic", "kiss", "heart"}
	mysteryWords := []string{"秘密", "谜团", "线索", "调查", "真相", "隐藏", "secret", "mystery", "clue", "investigate", "truth", "hidden"}
	actionWords := []string{"跑", "追", "逃", "行动", "快速", "紧急", "run", "chase", "escape", "action", "quick", "urgent"}

	for _, word := range conflictWords {
		if strings.Contains(allText, word) {
			conflictKeywords++
		}
	}
	for _, word := range romanceWords {
		if strings.Contains(allText, word) {
			romanceKeywords++
		}
	}
	for _, word := range mysteryWords {
		if strings.Contains(allText, word) {
			mysteryKeywords++
		}
	}
	for _, word := range actionWords {
		if strings.Contains(allText, word) {
			actionKeywords++
		}
	}

	// ✅ 基于消息内容和情绪数据细化节点类型

	// 高强度情绪交互
	if highIntensityCount >= len(messages)/2 {
		return baseType + "_intense"
	}

	// 冲突类型交互
	if conflictKeywords >= 2 {
		return baseType + "_conflict"
	}

	// 浪漫类型交互
	if romanceKeywords >= 2 {
		return baseType + "_romance"
	}

	// 悬疑类型交互
	if mysteryKeywords >= 2 {
		return baseType + "_mystery"
	}

	// 动作类型交互
	if actionKeywords >= 2 {
		return baseType + "_action"
	}

	// 多角色群体交互
	if multiCharacterInteraction {
		return baseType + "_group"
	}

	// ✅ 基于消息类型进一步细化
	thoughtCount := 0
	actionCount := 0
	responseCount := 0

	for _, message := range messages {
		switch message.MessageType {
		case "thought":
			thoughtCount++
		case "action":
			actionCount++
		case "response":
			responseCount++
		}
	}

	// 如果主要是内心独白
	if thoughtCount > responseCount && thoughtCount > actionCount {
		return baseType + "_introspective"
	}

	// 如果主要是行动描述
	if actionCount > responseCount && actionCount > thoughtCount {
		return baseType + "_active"
	}

	// ✅ 基于情绪类型组合判断
	emotionTypes := make(map[string]int)
	for _, message := range messages {
		if message.EmotionData != nil {
			emotion := strings.ToLower(message.EmotionData.Emotion)
			emotionTypes[emotion]++
		}
	}

	// 找到主导情绪
	dominantEmotion := ""
	maxCount := 0
	for emotion, count := range emotionTypes {
		if count > maxCount {
			maxCount = count
			dominantEmotion = emotion
		}
	}

	// 基于主导情绪调整类型
	switch dominantEmotion {
	case "anger", "愤怒":
		return baseType + "_confrontational"
	case "joy", "喜悦", "happiness":
		return baseType + "_joyful"
	case "sadness", "悲伤":
		return baseType + "_melancholic"
	case "fear", "恐惧":
		return baseType + "_tense"
	case "surprise", "惊讶":
		return baseType + "_revealing"
	}

	// ✅ 基于对话长度和复杂性
	totalLength := 0
	for _, message := range messages {
		totalLength += len(message.Content)
	}

	if totalLength > 1000 {
		return baseType + "_detailed"
	} else if totalLength < 100 {
		return baseType + "_brief"
	}

	return baseType
}

// 生成节点标题
func (s *InteractionAggregateService) generateNodeTitle(
	currentStory *models.StoryData,
	messages []CharacterMessage) string {

	// 基础标题根据角色数量
	baseTitle := ""
	if len(messages) == 1 {
		baseTitle = fmt.Sprintf("与%s的对话", messages[0].CharacterName)
	} else if len(messages) > 1 {
		baseTitle = fmt.Sprintf("%d位角色的群体对话", len(messages))
	} else {
		baseTitle = "角色互动记录"
	}

	// ✅ 利用 currentStory 信息增强标题

	// 1. 基于故事进度添加阶段信息
	var stagePrefix string
	switch {
	case currentStory.Progress < 25:
		stagePrefix = "[序章]"
	case currentStory.Progress < 50:
		stagePrefix = "[发展]"
	case currentStory.Progress < 75:
		stagePrefix = "[高潮]"
	case currentStory.Progress >= 75:
		stagePrefix = "[结局]"
	default:
		stagePrefix = "[进行中]"
	}

	// 2. 基于当前状态添加情境信息
	var contextSuffix string
	if currentStory.CurrentState != "" {
		// 根据当前状态添加上下文
		state := strings.ToLower(currentStory.CurrentState)
		switch {
		case strings.Contains(state, "紧张") || strings.Contains(state, "危险"):
			contextSuffix = " - 紧张时刻"
		case strings.Contains(state, "平静") || strings.Contains(state, "安全"):
			contextSuffix = " - 平静交流"
		case strings.Contains(state, "调查") || strings.Contains(state, "探索"):
			contextSuffix = " - 信息收集"
		case strings.Contains(state, "冲突") || strings.Contains(state, "争议"):
			contextSuffix = " - 冲突解决"
		case strings.Contains(state, "结盟") || strings.Contains(state, "合作"):
			contextSuffix = " - 合作商议"
		default:
			if currentStory.CurrentState != "初始" && currentStory.CurrentState != "Initial" {
				contextSuffix = fmt.Sprintf(" - %s", currentStory.CurrentState)
			}
		}
	}

	// 3. 基于相关任务添加任务相关信息
	var taskHint string
	if len(messages) > 0 {
		allText := ""
		for _, message := range messages {
			allText += strings.ToLower(message.Content) + " "
		}

		// 检查是否与重要任务相关
		for _, task := range currentStory.Tasks {
			if !task.Completed {
				taskKeywords := s.extractTaskKeywords(task)
				keywordMatches := 0
				for _, keyword := range taskKeywords {
					if strings.Contains(allText, strings.ToLower(keyword)) {
						keywordMatches++
					}
				}

				// 如果匹配度较高，在标题中提示任务相关性
				if keywordMatches >= 2 {
					taskHint = fmt.Sprintf(" - 关于「%s」", task.Title)
					break // 只使用第一个匹配的任务
				}
			}
		}
	}

	// 4. 基于地点信息添加位置上下文
	var locationHint string
	if len(currentStory.Locations) > 0 && len(messages) > 0 {
		allText := ""
		for _, message := range messages {
			allText += strings.ToLower(message.Content) + " "
		}

		// 检查是否提到了特定地点
		for _, location := range currentStory.Locations {
			locationName := strings.ToLower(location.Name)
			if strings.Contains(allText, locationName) {
				locationHint = fmt.Sprintf(" @ %s", location.Name)
				break // 只使用第一个匹配的地点
			}
		}
	}

	// 5. 基于情绪强度调整标题风格
	var emotionModifier string
	if len(messages) > 0 {
		maxIntensity := 0
		dominantEmotion := ""

		for _, message := range messages {
			if message.EmotionData != nil {
				if message.EmotionData.Intensity > maxIntensity {
					maxIntensity = message.EmotionData.Intensity
					dominantEmotion = strings.ToLower(message.EmotionData.Emotion)
				}
			}
		}

		// 高强度情绪的标题修饰
		if maxIntensity >= 8 {
			switch dominantEmotion {
			case "anger", "愤怒":
				emotionModifier = "【激烈】"
			case "joy", "喜悦", "happiness":
				emotionModifier = "【欢快】"
			case "sadness", "悲伤":
				emotionModifier = "【沉重】"
			case "fear", "恐惧":
				emotionModifier = "【紧张】"
			case "surprise", "惊讶":
				emotionModifier = "【震惊】"
			default:
				emotionModifier = "【激动】"
			}
		}
	}

	// 6. 基于节点数量添加序号
	nodeIndex := len(currentStory.Nodes) + 1
	indexSuffix := fmt.Sprintf(" (#%d)", nodeIndex)

	// 7. 组合最终标题
	finalTitle := stagePrefix + emotionModifier + baseTitle + taskHint + locationHint + contextSuffix + indexSuffix

	// 8. 确保标题长度合理
	if len(finalTitle) > 80 {
		// 简化标题，优先保留核心信息
		finalTitle = stagePrefix + baseTitle + taskHint + indexSuffix
	}

	return finalTitle
}

// 查找相关任务
func (s *InteractionAggregateService) findRelatedTasks(
	tasks []models.Task,
	userMessage string,
	messages []CharacterMessage) []string {

	relatedTasks := []string{}
	allText := strings.ToLower(userMessage)
	for _, msg := range messages {
		allText += " " + strings.ToLower(msg.Content)
	}

	for _, task := range tasks {
		if !task.Completed {
			taskKeywords := s.extractTaskKeywords(task)
			for _, keyword := range taskKeywords {
				if strings.Contains(allText, strings.ToLower(keyword)) {
					relatedTasks = append(relatedTasks, task.ID)
					break
				}
			}
		}
	}

	return relatedTasks
}

// 查找相关地点
func (s *InteractionAggregateService) findRelatedLocations(
	locations []models.StoryLocation,
	userMessage string,
	messages []CharacterMessage) []string {

	relatedLocations := []string{}
	allText := strings.ToLower(userMessage)
	for _, msg := range messages {
		allText += " " + strings.ToLower(msg.Content)
	}

	for _, location := range locations {
		locationText := strings.ToLower(location.Name + " " + location.Description)
		words := strings.Fields(locationText)

		for _, word := range words {
			if len(word) > 2 && strings.Contains(allText, word) {
				relatedLocations = append(relatedLocations, location.ID)
				break
			}
		}
	}

	return relatedLocations
}

// 基于故事状态生成选择项
func (s *InteractionAggregateService) generateChoicesWithStoryContext(
	messages []CharacterMessage,
	currentStory *models.StoryData) []models.StoryChoice {

	choices := []models.StoryChoice{}

	// 基于角色响应生成选择
	for i, message := range messages {
		if i >= 3 { // 限制选择数量
			break
		}

		var choiceText string
		var consequence string

		// 根据故事进度调整选择文本
		if currentStory.Progress < 50 {
			choiceText = fmt.Sprintf("深入了解%s的想法", message.CharacterName)
			consequence = fmt.Sprintf("与%s建立更深层的联系", message.CharacterName)
		} else {
			choiceText = fmt.Sprintf("询问%s关于关键信息", message.CharacterName)
			consequence = fmt.Sprintf("可能从%s处获得重要线索", message.CharacterName)
		}

		choice := models.StoryChoice{
			ID:          fmt.Sprintf("choice_%s_%d", message.CharacterID, time.Now().UnixNano()),
			Text:        choiceText,
			Consequence: consequence,
			NextNodeID:  "", // 需要后续填充
		}

		choices = append(choices, choice)
	}

	// 如果有未完成的任务，添加任务相关选择
	if len(currentStory.Tasks) > 0 {
		for _, task := range currentStory.Tasks {
			if !task.Completed && len(choices) < 4 {
				choice := models.StoryChoice{
					ID:          fmt.Sprintf("task_choice_%s_%d", task.ID, time.Now().UnixNano()),
					Text:        fmt.Sprintf("讨论任务：%s", task.Title),
					Consequence: "推进任务进展",
					NextNodeID:  "",
				}
				choices = append(choices, choice)
				break // 只添加一个任务选择
			}
		}
	}

	return choices
}

// buildNodeContentFromInteraction 从交互构建节点内容
func (s *InteractionAggregateService) buildNodeContentFromInteraction(
	request *InteractionRequest,
	messages []CharacterMessage) string {

	var contentBuilder strings.Builder

	// 将标题信息直接放在Content中
	contentBuilder.WriteString("## 角色互动记录\n\n")
	contentBuilder.WriteString(fmt.Sprintf("**时间**: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// 记录交互概要
	if len(request.CharacterIDs) > 1 {
		contentBuilder.WriteString(fmt.Sprintf("**参与角色**: %d位角色参与了这次对话\n\n", len(request.CharacterIDs)))
	}

	// 记录用户消息
	contentBuilder.WriteString(fmt.Sprintf("**用户**: %s\n\n", request.Message))

	// 记录角色响应
	for _, message := range messages {
		contentBuilder.WriteString(fmt.Sprintf("**%s**: %s\n", message.CharacterName, message.Content))

		if message.EmotionData != nil {
			contentBuilder.WriteString(fmt.Sprintf("*（情绪: %s，强度: %d",
				message.EmotionData.Emotion, message.EmotionData.Intensity))

			// 添加更多情绪细节
			if message.EmotionData.BodyLanguage != "" {
				contentBuilder.WriteString(fmt.Sprintf("，身体语言: %s", message.EmotionData.BodyLanguage))
			}
			if message.EmotionData.VoiceTone != "" {
				contentBuilder.WriteString(fmt.Sprintf("，语调: %s", message.EmotionData.VoiceTone))
			}

			contentBuilder.WriteString("）*\n")
		}
		contentBuilder.WriteString("\n")
	}

	// 添加互动总结
	if len(messages) > 1 {
		contentBuilder.WriteString("---\n\n")
		contentBuilder.WriteString("**互动总结**: 这是一次多角色对话，展现了角色间的动态交流。\n")
	}

	return contentBuilder.String()
}

// updateCurrentStoryNode 更新当前故事节点
func (s *InteractionAggregateService) updateCurrentStoryNode(
	currentStory *models.StoryData,
	impact *StoryImpact) *models.StoryNode {

	// 找到当前活跃节点 - 使用现有的逻辑
	var currentNode *models.StoryNode
	latestTime := time.Time{}

	for i := range currentStory.Nodes {
		node := &currentStory.Nodes[i]
		if node.IsRevealed && node.CreatedAt.After(latestTime) {
			// 检查是否有已选择的选项
			hasSelectedChoice := false
			for _, choice := range node.Choices {
				if choice.Selected {
					hasSelectedChoice = true
					break
				}
			}

			// 优先选择有已选择选项的节点，或者选择最新的已显示节点
			if hasSelectedChoice || currentNode == nil {
				currentNode = node
				latestTime = node.CreatedAt
			}
		}
	}

	if currentNode == nil {
		return nil
	}

	// 更新节点内容，添加交互影响
	if len(impact.KeyEvents) > 0 {
		currentNode.Content += fmt.Sprintf("\n\n**最新发展**: %s", strings.Join(impact.KeyEvents, ", "))
	}

	// 在 Metadata 中记录更新时间
	if currentNode.Metadata == nil {
		currentNode.Metadata = make(map[string]interface{})
	}
	currentNode.Metadata["last_updated"] = time.Now()
	currentNode.Metadata["interaction_updates"] = impact.KeyEvents

	return currentNode
}

// extractTaskKeywords 提取任务相关关键词
func (s *InteractionAggregateService) extractTaskKeywords(task models.Task) []string {
	keywords := []string{}

	// 从任务标题和描述中提取关键词
	taskText := strings.ToLower(task.Title + " " + task.Description)

	// 简单的关键词提取（实际项目中可以使用更复杂的NLP技术）
	commonWords := []string{"的", "了", "在", "和", "与", "为", "是", "有", "到", "将", "被", "从", "对", "把", "给"}

	words := strings.Fields(taskText)
	for _, word := range words {
		if len(word) > 1 && !contains(commonWords, word) {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// contains 辅助函数：检查切片是否包含元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// checkUnlockedContent 检查解锁内容
func (s *InteractionAggregateService) checkUnlockedContent(
	impact *StoryImpact,
	currentStory *models.StoryData) []string {

	unlockedContent := []string{}

	// 基于解锁触发器检查
	for _, trigger := range impact.UnlockTriggers {
		switch trigger {
		case "key_obtained":
			unlockedContent = append(unlockedContent, "新区域: 神秘房间")
		case "password_learned":
			unlockedContent = append(unlockedContent, "新功能: 电脑终端访问")
		case "map_revealed":
			unlockedContent = append(unlockedContent, "新地图: 秘密通道")
		case "clue_discovered":
			unlockedContent = append(unlockedContent, "新线索: 重要信息")
		case "information_gained":
			unlockedContent = append(unlockedContent, "新知识: 背景故事")
		}
	}

	// 基于进度检查解锁
	if currentStory.Progress >= 25 && currentStory.Progress < 50 {
		unlockedContent = append(unlockedContent, "故事分支: 第二章节")
	} else if currentStory.Progress >= 50 && currentStory.Progress < 75 {
		unlockedContent = append(unlockedContent, "新角色: 神秘访客")
	} else if currentStory.Progress >= 75 {
		unlockedContent = append(unlockedContent, "最终章节: 真相大白")
	}

	return unlockedContent
}

// saveUpdatedStoryData 保存更新后的故事数据
func (s *InteractionAggregateService) saveUpdatedStoryData(
	sceneID string,
	storyData *models.StoryData) error {

	storyService := s.getStoryService()
	if storyService == nil {
		return fmt.Errorf("故事服务未初始化")
	}

	// 更新时间戳
	storyData.LastUpdated = time.Now()

	// 调用故事服务保存数据
	return storyService.SaveStoryData(sceneID, storyData)
}

// getStoryService 获取故事服务实例
func (s *InteractionAggregateService) getStoryService() *StoryService {
	if s.StoryService == nil {
		// 尝试从DI容器获取
		container := di.GetContainer()
		if storyService, ok := container.Get("story").(*StoryService); ok {
			s.StoryService = storyService
			return storyService
		}
		return nil
	}
	return s.StoryService
}

// generateFollowUpChoices 生成后续选择
func (s *InteractionAggregateService) generateFollowUpChoices(
	request *InteractionRequest,
	messages []CharacterMessage) ([]*models.StoryChoice, error) {

	// 获取故事服务和当前故事状态
	storyService := s.getStoryService()
	if storyService == nil {
		return nil, fmt.Errorf("故事服务未初始化")
	}

	currentStory, err := storyService.GetStoryData(request.SceneID, nil)
	if err != nil {
		// 如果获取故事数据失败，返回基础选择
		return s.generateBasicFollowUpChoices(messages), nil
	}

	choices := []*models.StoryChoice{}

	// ✅ 基于 request.Message 和 request.CharacterIDs 生成上下文相关选择

	// 1. 基于用户原始消息生成探索性选择
	userMessage := strings.ToLower(request.Message)
	if strings.Contains(userMessage, "为什么") || strings.Contains(userMessage, "why") {
		choices = append(choices, &models.StoryChoice{
			ID:          fmt.Sprintf("explore_why_%d", time.Now().UnixNano()),
			Text:        "深入探讨这个问题的原因",
			Consequence: "可能获得更深层的见解",
			NextNodeID:  "",
			Metadata: map[string]interface{}{
				"choice_type": "exploration",
				"trigger":     "why_question",
			},
		})
	}

	if strings.Contains(userMessage, "如何") || strings.Contains(userMessage, "怎么") || strings.Contains(userMessage, "how") {
		choices = append(choices, &models.StoryChoice{
			ID:          fmt.Sprintf("explore_how_%d", time.Now().UnixNano()),
			Text:        "询问具体的方法或步骤",
			Consequence: "获得实用的解决方案",
			NextNodeID:  "",
			Metadata: map[string]interface{}{
				"choice_type": "solution_seeking",
				"trigger":     "how_question",
			},
		})
	}

	// 2. 基于参与角色数量生成群体或个体选择
	if len(request.CharacterIDs) > 1 {
		// 多角色场景：生成分别与各角色互动的选择
		for _, charID := range request.CharacterIDs {
			// 找到对应的角色响应
			var targetMessage *CharacterMessage
			for _, msg := range messages {
				if msg.CharacterID == charID {
					targetMessage = &msg
					break
				}
			}

			if targetMessage != nil {
				choice := &models.StoryChoice{
					ID:          fmt.Sprintf("focus_%s_%d", charID, time.Now().UnixNano()),
					Text:        fmt.Sprintf("专门与%s进一步交流", targetMessage.CharacterName),
					Consequence: fmt.Sprintf("加深与%s的关系，了解其独特观点", targetMessage.CharacterName),
					NextNodeID:  "",
					Metadata: map[string]interface{}{
						"choice_type":      "individual_focus",
						"target_character": charID,
						"character_name":   targetMessage.CharacterName,
					},
				}
				choices = append(choices, choice)
			}
		}

		// 添加群体讨论选择
		choices = append(choices, &models.StoryChoice{
			ID:          fmt.Sprintf("group_discussion_%d", time.Now().UnixNano()),
			Text:        "让所有角色一起讨论这个话题",
			Consequence: "促进角色间的互动，可能产生新的观点碰撞",
			NextNodeID:  "",
			Metadata: map[string]interface{}{
				"choice_type":       "group_interaction",
				"participant_count": len(request.CharacterIDs),
				"character_ids":     request.CharacterIDs,
			},
		})
	} else if len(request.CharacterIDs) == 1 {
		// 单角色场景：生成深度互动选择
		charID := request.CharacterIDs[0]
		var targetMessage *CharacterMessage
		for _, msg := range messages {
			if msg.CharacterID == charID {
				targetMessage = &msg
				break
			}
		}

		if targetMessage != nil {
			// 基于角色情绪生成对应选择
			if targetMessage.EmotionData != nil {
				emotion := strings.ToLower(targetMessage.EmotionData.Emotion)
				intensity := targetMessage.EmotionData.Intensity

				switch emotion {
				case "joy", "喜悦", "happiness":
					choices = append(choices, &models.StoryChoice{
						ID:          fmt.Sprintf("share_joy_%d", time.Now().UnixNano()),
						Text:        "分享这份快乐的心情",
						Consequence: "增进友谊，营造愉快氛围",
						NextNodeID:  "",
					})
				case "sadness", "悲伤":
					choices = append(choices, &models.StoryChoice{
						ID:          fmt.Sprintf("comfort_%d", time.Now().UnixNano()),
						Text:        "安慰并支持对方",
						Consequence: "提供情感支持，可能获得信任",
						NextNodeID:  "",
					})
				case "anger", "愤怒":
					if intensity > 7 {
						choices = append(choices, &models.StoryChoice{
							ID:          fmt.Sprintf("calm_down_%d", time.Now().UnixNano()),
							Text:        "尝试缓解紧张情绪",
							Consequence: "可能平息冲突，但也可能激化矛盾",
							NextNodeID:  "",
						})
					} else {
						choices = append(choices, &models.StoryChoice{
							ID:          fmt.Sprintf("understand_anger_%d", time.Now().UnixNano()),
							Text:        "了解愤怒的原因",
							Consequence: "深入理解问题核心",
							NextNodeID:  "",
						})
					}
				case "fear", "恐惧":
					choices = append(choices, &models.StoryChoice{
						ID:          fmt.Sprintf("reassure_%d", time.Now().UnixNano()),
						Text:        "给予安全感和保证",
						Consequence: "建立信任，可能获得重要信息",
						NextNodeID:  "",
					})
				}
			}
		}
	}

	// 3. 基于当前故事状态生成情境相关选择
	if currentStory != nil {
		// 基于故事进度生成选择
		switch {
		case currentStory.Progress < 25:
			choices = append(choices, &models.StoryChoice{
				ID:          fmt.Sprintf("explore_background_%d", time.Now().UnixNano()),
				Text:        "了解更多背景信息",
				Consequence: "获得故事世界的深层知识",
				NextNodeID:  "",
				Metadata: map[string]interface{}{
					"choice_type": "world_building",
					"stage":       "early",
				},
			})
		case currentStory.Progress >= 75:
			choices = append(choices, &models.StoryChoice{
				ID:          fmt.Sprintf("seek_resolution_%d", time.Now().UnixNano()),
				Text:        "寻求问题的最终解决方案",
				Consequence: "可能推进故事向结局发展",
				NextNodeID:  "",
				Metadata: map[string]interface{}{
					"choice_type": "resolution_seeking",
					"stage":       "finale",
				},
			})
		default:
			choices = append(choices, &models.StoryChoice{
				ID:          fmt.Sprintf("advance_plot_%d", time.Now().UnixNano()),
				Text:        "推进当前的情节发展",
				Consequence: "加快故事节奏，可能触发新事件",
				NextNodeID:  "",
				Metadata: map[string]interface{}{
					"choice_type": "plot_advancement",
					"stage":       "development",
				},
			})
		}

		// 4. 基于未完成任务生成任务相关选择
		for _, task := range currentStory.Tasks {
			if !task.Completed && len(choices) < 5 {
				choice := &models.StoryChoice{
					ID:          fmt.Sprintf("task_followup_%s_%d", task.ID, time.Now().UnixNano()),
					Text:        fmt.Sprintf("讨论「%s」的进展", task.Title),
					Consequence: "推进任务完成，可能获得新线索",
					NextNodeID:  "",
					Metadata: map[string]interface{}{
						"choice_type": "task_related",
						"task_id":     task.ID,
						"task_title":  task.Title,
					},
				}
				choices = append(choices, choice)
				break // 只添加一个任务选择
			}
		}
	}

	// 5. 基于用户的情绪数据生成选择（如果有）
	if request.EmotionData != nil {
		userEmotion := strings.ToLower(request.EmotionData.Emotion)
		switch userEmotion {
		case "curiosity", "好奇":
			choices = append(choices, &models.StoryChoice{
				ID:          fmt.Sprintf("satisfy_curiosity_%d", time.Now().UnixNano()),
				Text:        "满足好奇心，深入了解",
				Consequence: "获得详细信息，可能解开谜团",
				NextNodeID:  "",
				Metadata: map[string]interface{}{
					"choice_type":  "curiosity_driven",
					"user_emotion": userEmotion,
				},
			})
		case "concern", "担心":
			choices = append(choices, &models.StoryChoice{
				ID:          fmt.Sprintf("address_concern_%d", time.Now().UnixNano()),
				Text:        "表达担忧并寻求安慰",
				Consequence: "获得情感支持，可能得到保护承诺",
				NextNodeID:  "",
				Metadata: map[string]interface{}{
					"choice_type":  "concern_addressing",
					"user_emotion": userEmotion,
				},
			})
		}
	}

	// 6. 基于上下文信息生成选择（如果有）
	if request.Context != nil {
		if location, exists := request.Context["current_location"]; exists {
			choices = append(choices, &models.StoryChoice{
				ID:          fmt.Sprintf("explore_location_%d", time.Now().UnixNano()),
				Text:        fmt.Sprintf("探索当前位置：%v", location),
				Consequence: "发现环境中的新信息或隐藏要素",
				NextNodeID:  "",
				Metadata: map[string]interface{}{
					"choice_type": "location_exploration",
					"location":    location,
				},
			})
		}

		if timeOfDay, exists := request.Context["time_of_day"]; exists {
			choices = append(choices, &models.StoryChoice{
				ID:          fmt.Sprintf("time_sensitive_%d", time.Now().UnixNano()),
				Text:        fmt.Sprintf("考虑当前时间（%v）的影响", timeOfDay),
				Consequence: "基于时间因素做出决策",
				NextNodeID:  "",
				Metadata: map[string]interface{}{
					"choice_type": "time_sensitive",
					"time":        timeOfDay,
				},
			})
		}
	}

	// 7. 确保至少有基础选择
	if len(choices) == 0 {
		return s.generateBasicFollowUpChoices(messages), nil
	}

	// 8. 限制选择数量并添加通用选择
	if len(choices) > 4 {
		choices = choices[:4]
	}

	// 添加一个通用的继续对话选择
	choices = append(choices, &models.StoryChoice{
		ID:          fmt.Sprintf("continue_conversation_%d", time.Now().UnixNano()),
		Text:        "继续当前对话",
		Consequence: "保持对话流畅性",
		NextNodeID:  "",
		Metadata: map[string]interface{}{
			"choice_type": "continuation",
		},
	})

	return choices, nil
}

// 生成基础后续选择（备用方法）
func (s *InteractionAggregateService) generateBasicFollowUpChoices(
	messages []CharacterMessage) []*models.StoryChoice {

	choices := []*models.StoryChoice{}

	// 简单实现：为每个角色生成一个后续选择
	for i, message := range messages {
		choice := &models.StoryChoice{
			ID:          fmt.Sprintf("follow_up_%s_%d", message.CharacterID, time.Now().UnixNano()),
			Text:        fmt.Sprintf("继续与%s对话", message.CharacterName),
			Consequence: "继续当前话题",
			NextNodeID:  "",
			Metadata: map[string]interface{}{
				"choice_type":    "basic_followup",
				"character_id":   message.CharacterID,
				"character_name": message.CharacterName,
			},
		}
		choices = append(choices, choice)

		// 限制选择数量
		if i >= 2 {
			break
		}
	}

	return choices
}

// saveInteractionToHistory 保存交互历史（兼容前端格式）
func (s *InteractionAggregateService) saveInteractionToHistory(
	request *InteractionRequest,
	result *InteractionResult) error {

	// 生成交互会话ID，用于关联所有相关对话
	interactionID := fmt.Sprintf("interaction_%d", time.Now().UnixNano())

	// 保存用户输入
	if request.Message != "" {
		userMetadata := map[string]interface{}{
			"conversation_type": "character_interaction", // 与前端期望的类型匹配
			"interaction_id":    interactionID,
			"character_ids":     request.CharacterIDs,
			"message_type":      "user_input",
			"speaker_type":      "user",
		}

		if err := s.ContextService.AddConversation(
			request.SceneID,
			"user",
			request.Message,
			userMetadata,
		); err != nil {
			return fmt.Errorf("保存用户消息失败: %w", err)
		}
	}

	// 为每个角色响应创建对话记录
	for _, message := range result.Messages {
		metadata := map[string]interface{}{
			"conversation_type": "character_interaction", // 前端期望的类型
			"interaction_id":    interactionID,           // 关联ID
			"character_name":    message.CharacterName,   // 角色名称
			"character_ids":     request.CharacterIDs,    // 所有参与角色
			"user_message":      request.Message,         // 原始用户消息
			"message_type":      message.MessageType,     // 消息类型
			"speaker_type":      "character",             // 发言者类型
			"emotion_data":      message.EmotionData,     // 情绪数据
		}

		// 合并消息的元数据
		if message.Metadata != nil {
			for k, v := range message.Metadata {
				// 避免覆盖重要的元数据字段
				if k != "conversation_type" && k != "interaction_id" {
					metadata[k] = v
				}
			}
		}

		if err := s.ContextService.AddConversation(
			request.SceneID,
			message.CharacterID,
			message.Content,
			metadata,
		); err != nil {
			return fmt.Errorf("保存角色 %s 的对话失败: %w", message.CharacterName, err)
		}
	}

	return nil
}

// checkAndTriggerEvents 检查并触发事件
func (s *InteractionAggregateService) checkAndTriggerEvents(
	request *InteractionRequest,
	result *InteractionResult) []GameEvent {

	events := []GameEvent{}

	// ✅ 基于 request.CharacterIDs 检查多角色互动事件
	if len(request.CharacterIDs) >= 2 {
		events = append(events, GameEvent{
			EventType: "multi_character_interaction",
			EventData: map[string]interface{}{
				"scene_id":         request.SceneID,
				"character_count":  len(request.CharacterIDs),
				"character_ids":    request.CharacterIDs,
				"interaction_type": "group_conversation",
				"user_message":     request.Message,
			},
			Triggers:  []string{"multi_character_chat", "group_dynamics"},
			Effects:   []string{"relationship_development", "social_experience"},
			Timestamp: time.Now(),
		})

		// 检查是否达成社交成就
		if len(request.CharacterIDs) >= 3 {
			events = append(events, GameEvent{
				EventType: "achievement",
				EventData: map[string]interface{}{
					"achievement_id":  "social_butterfly",
					"title":           "社交蝴蝶",
					"description":     "同时与3个或更多角色对话",
					"character_count": len(request.CharacterIDs),
					"scene_id":        request.SceneID,
				},
				Triggers:  []string{"multi_character_interaction"},
				Effects:   []string{"unlock_achievement", "social_bonus"},
				Timestamp: time.Now(),
			})
		}
	}

	// ✅ 基于 request.Message 内容检查特殊事件
	userMessage := strings.ToLower(request.Message)

	// 检查探索相关事件
	explorationKeywords := []string{"探索", "寻找", "调查", "搜索", "explore", "find", "investigate", "search"}
	for _, keyword := range explorationKeywords {
		if strings.Contains(userMessage, keyword) {
			events = append(events, GameEvent{
				EventType: "exploration_initiated",
				EventData: map[string]interface{}{
					"trigger_keyword": keyword,
					"user_message":    request.Message,
					"scene_id":        request.SceneID,
					"explorer_count":  len(request.CharacterIDs),
				},
				Triggers:  []string{"exploration_intent", "curiosity_driven"},
				Effects:   []string{"discovery_chance", "exploration_bonus"},
				Timestamp: time.Now(),
			})
			break
		}
	}

	// 检查解谜相关事件
	puzzleKeywords := []string{"谜题", "线索", "秘密", "密码", "puzzle", "clue", "secret", "password", "mystery"}
	for _, keyword := range puzzleKeywords {
		if strings.Contains(userMessage, keyword) {
			events = append(events, GameEvent{
				EventType: "puzzle_engagement",
				EventData: map[string]interface{}{
					"puzzle_keyword": keyword,
					"user_message":   request.Message,
					"scene_id":       request.SceneID,
					"participants":   request.CharacterIDs,
				},
				Triggers:  []string{"puzzle_solving", "mystery_interest"},
				Effects:   []string{"intelligence_boost", "puzzle_progress"},
				Timestamp: time.Now(),
			})
			break
		}
	}

	// 检查冲突相关事件
	conflictKeywords := []string{"战斗", "冲突", "争吵", "对抗", "fight", "conflict", "argue", "confront"}
	for _, keyword := range conflictKeywords {
		if strings.Contains(userMessage, keyword) {
			events = append(events, GameEvent{
				EventType: "conflict_escalation",
				EventData: map[string]interface{}{
					"conflict_type":  keyword,
					"user_message":   request.Message,
					"scene_id":       request.SceneID,
					"involved_chars": request.CharacterIDs,
					"tension_level":  "moderate",
				},
				Triggers:  []string{"conflict_intent", "confrontation"},
				Effects:   []string{"tension_increase", "relationship_strain"},
				Timestamp: time.Now(),
			})
			break
		}
	}

	// ✅ 基于 request.EmotionData 检查情绪驱动事件
	if request.EmotionData != nil {
		userEmotion := strings.ToLower(request.EmotionData.Emotion)
		intensity := request.EmotionData.Intensity

		// 高强度情绪事件
		if intensity >= 8 {
			events = append(events, GameEvent{
				EventType: "intense_emotion_display",
				EventData: map[string]interface{}{
					"emotion":       userEmotion,
					"intensity":     intensity,
					"user_message":  request.Message,
					"scene_id":      request.SceneID,
					"witnesses":     request.CharacterIDs,
					"body_language": request.EmotionData.BodyLanguage,
					"voice_tone":    request.EmotionData.VoiceTone,
				},
				Triggers:  []string{"emotional_peak", "intense_feeling"},
				Effects:   []string{"emotional_contagion", "memorable_moment"},
				Timestamp: time.Now(),
			})
		}

		// 特定情绪事件
		switch userEmotion {
		case "anger", "愤怒":
			events = append(events, GameEvent{
				EventType: "anger_expression",
				EventData: map[string]interface{}{
					"intensity":    intensity,
					"scene_id":     request.SceneID,
					"target_chars": request.CharacterIDs,
					"context":      request.Message,
				},
				Triggers:  []string{"anger_display", "emotional_tension"},
				Effects:   []string{"intimidation_effect", "conflict_risk"},
				Timestamp: time.Now(),
			})
		case "joy", "喜悦", "happiness":
			events = append(events, GameEvent{
				EventType: "joy_sharing",
				EventData: map[string]interface{}{
					"intensity":      intensity,
					"scene_id":       request.SceneID,
					"shared_with":    request.CharacterIDs,
					"joyful_message": request.Message,
				},
				Triggers:  []string{"positive_emotion", "happiness_spread"},
				Effects:   []string{"mood_boost", "relationship_improvement"},
				Timestamp: time.Now(),
			})
		case "fear", "恐惧":
			events = append(events, GameEvent{
				EventType: "fear_expression",
				EventData: map[string]interface{}{
					"intensity":     intensity,
					"scene_id":      request.SceneID,
					"support_chars": request.CharacterIDs,
					"fear_context":  request.Message,
				},
				Triggers:  []string{"vulnerability_display", "fear_admission"},
				Effects:   []string{"protection_instinct", "bonding_opportunity"},
				Timestamp: time.Now(),
			})
		}
	}

	// ✅ 基于 request.Context 检查环境事件
	if request.Context != nil {
		// 检查时间相关事件
		if timeOfDay, exists := request.Context["time_of_day"]; exists {
			timeStr := fmt.Sprintf("%v", timeOfDay)
			if strings.Contains(strings.ToLower(timeStr), "night") ||
				strings.Contains(strings.ToLower(timeStr), "深夜") {
				events = append(events, GameEvent{
					EventType: "late_night_interaction",
					EventData: map[string]interface{}{
						"time_of_day":    timeOfDay,
						"scene_id":       request.SceneID,
						"night_owls":     request.CharacterIDs,
						"midnight_topic": request.Message,
					},
					Triggers:  []string{"nocturnal_activity", "intimate_timing"},
					Effects:   []string{"deeper_connection", "secret_sharing"},
					Timestamp: time.Now(),
				})
			}
		}

		// 检查位置相关事件
		if location, exists := request.Context["current_location"]; exists {
			locationStr := strings.ToLower(fmt.Sprintf("%v", location))

			// 特殊位置事件
			specialLocations := map[string]string{
				"图书馆":    "library_interaction",
				"garden": "garden_conversation",
				"roof":   "rooftop_meeting",
				"秘密":     "secret_location_discovery",
			}

			for keyword, eventType := range specialLocations {
				if strings.Contains(locationStr, keyword) {
					events = append(events, GameEvent{
						EventType: eventType,
						EventData: map[string]interface{}{
							"location":     location,
							"scene_id":     request.SceneID,
							"participants": request.CharacterIDs,
							"activity":     request.Message,
						},
						Triggers:  []string{"location_specific", "environmental_influence"},
						Effects:   []string{"location_bonus", "atmospheric_enhancement"},
						Timestamp: time.Now(),
					})
					break
				}
			}
		}

		// 检查天气相关事件
		if weather, exists := request.Context["weather"]; exists {
			weatherStr := strings.ToLower(fmt.Sprintf("%v", weather))
			if strings.Contains(weatherStr, "rain") || strings.Contains(weatherStr, "雨") {
				events = append(events, GameEvent{
					EventType: "rainy_day_bonding",
					EventData: map[string]interface{}{
						"weather":       weather,
						"scene_id":      request.SceneID,
						"shelter_mates": request.CharacterIDs,
						"conversation":  request.Message,
					},
					Triggers:  []string{"weather_influence", "cozy_atmosphere"},
					Effects:   []string{"intimacy_boost", "comfort_sharing"},
					Timestamp: time.Now(),
				})
			}
		}
	}

	// ✅ 基于 request.SceneID 检查场景特定事件
	sceneSpecificEvents := map[string]string{
		"library":   "knowledge_seeking",
		"cafeteria": "social_dining",
		"classroom": "academic_discussion",
		"dormitory": "private_conversation",
		"garden":    "peaceful_dialogue",
	}

	sceneIDLower := strings.ToLower(request.SceneID)
	for sceneKeyword, eventType := range sceneSpecificEvents {
		if strings.Contains(sceneIDLower, sceneKeyword) {
			events = append(events, GameEvent{
				EventType: eventType,
				EventData: map[string]interface{}{
					"scene_id":          request.SceneID,
					"scene_type":        sceneKeyword,
					"participants":      request.CharacterIDs,
					"interaction_topic": request.Message,
					"scene_atmosphere":  "conducive",
				},
				Triggers:  []string{"scene_appropriate", "environment_match"},
				Effects:   []string{"scene_bonus", "thematic_enhancement"},
				Timestamp: time.Now(),
			})
			break
		}
	}

	// 检查成就触发（保留原有逻辑，但增强数据）
	if len(result.Messages) >= 2 {
		events = append(events, GameEvent{
			EventType: "achievement",
			EventData: map[string]interface{}{
				"achievement_id":  "multi_character_chat",
				"title":           "社交达人",
				"description":     "同时与多个角色对话",
				"scene_id":        request.SceneID,
				"character_count": len(request.CharacterIDs),
				"response_count":  len(result.Messages),
				"user_message":    request.Message,
			},
			Triggers:  []string{"multi_character_interaction", "social_success"},
			Effects:   []string{"unlock_achievement", "social_experience"},
			Timestamp: time.Now(),
		})
	}

	// 检查关系变化事件（增强原有逻辑）
	for charID, state := range result.CharacterStates {
		for otherID, relationshipChange := range state.Relationship {
			if relationshipChange > 0.1 {
				events = append(events, GameEvent{
					EventType: "relationship_improvement",
					EventData: map[string]interface{}{
						"character1":       charID,
						"character2":       otherID,
						"change":           relationshipChange,
						"scene_id":         request.SceneID,
						"trigger_message":  request.Message,
						"improvement_type": "positive_interaction",
					},
					Triggers:  []string{"positive_interaction", "bond_strengthening"},
					Effects:   []string{"relationship_bonus", "trust_building"},
					Timestamp: time.Now(),
				})
			} else if relationshipChange < -0.1 {
				// 负面关系变化事件
				events = append(events, GameEvent{
					EventType: "relationship_strain",
					EventData: map[string]interface{}{
						"character1":      charID,
						"character2":      otherID,
						"change":          relationshipChange,
						"scene_id":        request.SceneID,
						"trigger_message": request.Message,
						"strain_type":     "negative_interaction",
					},
					Triggers:  []string{"negative_interaction", "conflict_emergence"},
					Effects:   []string{"relationship_penalty", "tension_increase"},
					Timestamp: time.Now(),
				})
			}
		}
	}

	// ✅ 基于消息长度和复杂性检查深度对话事件
	messageLength := len(request.Message)
	totalResponseLength := 0
	for _, msg := range result.Messages {
		totalResponseLength += len(msg.Content)
	}

	if messageLength > 200 || totalResponseLength > 500 {
		events = append(events, GameEvent{
			EventType: "deep_conversation",
			EventData: map[string]interface{}{
				"message_length":     messageLength,
				"response_length":    totalResponseLength,
				"scene_id":           request.SceneID,
				"participants":       request.CharacterIDs,
				"conversation_depth": "substantial",
			},
			Triggers:  []string{"lengthy_discussion", "detailed_interaction"},
			Effects:   []string{"understanding_boost", "meaningful_connection"},
			Timestamp: time.Now(),
		})
	}

	return events
}

// buildUIUpdateCommands 构建UI更新指令
func (s *InteractionAggregateService) buildUIUpdateCommands(
	request *InteractionRequest,
	result *InteractionResult) *UIUpdateCommands {

	commands := &UIUpdateCommands{
		ScrollToBottom:      true,
		HighlightCharacters: request.CharacterIDs,
		UpdateChatBadges:    make(map[string]int),
		TriggerAnimations:   []UIAnimation{},
		UpdateTabs:          []TabUpdate{},
	}

	// 为每个参与的角色更新聊天徽章
	for _, charID := range request.CharacterIDs {
		commands.UpdateChatBadges[charID] = 1
	}

	// 如果有多个角色响应，添加动画效果
	if len(result.Messages) > 1 {
		commands.TriggerAnimations = append(commands.TriggerAnimations, UIAnimation{
			Target:   ".character-list",
			Type:     "highlight",
			Duration: 1000,
			Params: map[string]interface{}{
				"color": "#4CAF50",
			},
		})
	}

	// 更新相关标签页
	commands.UpdateTabs = append(commands.UpdateTabs, TabUpdate{
		TabID:      "chat",
		BadgeCount: len(result.Messages),
		IsActive:   true,
	})

	// 如果有故事更新，更新故事标签
	if result.StoryUpdates != nil {
		badgeCount := len(result.StoryUpdates.NewNodes)

		// 如果有任务完成，增加徽章计数
		if len(result.StoryUpdates.CompletedTasks) > 0 {
			badgeCount += len(result.StoryUpdates.CompletedTasks)
		}

		if badgeCount > 0 {
			commands.UpdateTabs = append(commands.UpdateTabs, TabUpdate{
				TabID:      "story",
				BadgeCount: badgeCount,
				Title:      "故事",
			})
		}

		// 专门为任务完成更新任务标签
		if len(result.StoryUpdates.CompletedTasks) > 0 {
			commands.UpdateTabs = append(commands.UpdateTabs, TabUpdate{
				TabID:      "tasks",
				BadgeCount: len(result.StoryUpdates.CompletedTasks),
				Title:      fmt.Sprintf("任务 (+%d)", len(result.StoryUpdates.CompletedTasks)),
			})
		}
	}

	return commands
}

// calculateRelationshipChangeFromMessage 计算基于消息的关系变化
func (s *InteractionAggregateService) calculateRelationshipChangeFromMessage(
	message CharacterMessage, originalMessage string) float64 {

	if message.EmotionData == nil {
		return 0.0
	}

	// 转换为 EmotionalResponse 格式
	emotionalResponse := &models.EmotionalResponse{
		Emotion:           message.EmotionData.Emotion,
		Intensity:         message.EmotionData.Intensity,
		BodyLanguage:      message.EmotionData.BodyLanguage,
		FacialExpression:  message.EmotionData.FacialExpression,
		VoiceTone:         message.EmotionData.VoiceTone,
		SecondaryEmotions: message.EmotionData.SecondaryEmotions,
	}

	return calculateRelationshipChangeFromResponse(emotionalResponse, originalMessage)
}

// 委托导出功能
func (s *InteractionAggregateService) ExportInteraction(ctx context.Context, sceneID string, format string) (*models.ExportResult, error) {
	return s.ExportService.ExportInteractionSummary(ctx, sceneID, format)
}
