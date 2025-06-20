// internal/services/export_service.go
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

type ExportService struct {
	ContextService *ContextService
	StoryService   *StoryService
	SceneService   *SceneService
}

func NewExportService(contextService *ContextService, storyService *StoryService, sceneService *SceneService) *ExportService {
	return &ExportService{
		ContextService: contextService,
		StoryService:   storyService,
		SceneService:   sceneService,
	}
}

// Export相关方法--------------------------
// ExportInteractionSummary 导出交互摘要功能
func (s *ExportService) ExportInteractionSummary(ctx context.Context, sceneID string, format string) (*models.ExportResult, error) {
	// 1. 验证输入参数
	if sceneID == "" {
		return nil, fmt.Errorf("场景ID不能为空")
	}

	supportedFormats := []string{"json", "markdown", "txt", "html"}
	if !contains(supportedFormats, strings.ToLower(format)) {
		return nil, fmt.Errorf("不支持的导出格式: %s，支持的格式: %v", format, supportedFormats)
	}

	// 2. 获取场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, fmt.Errorf("加载场景失败: %w", err)
	}

	// 3. 获取交互历史
	conversations, err := s.getInteractionHistory(sceneID)
	if err != nil {
		return nil, fmt.Errorf("获取交互历史失败: %w", err)
	}

	// 4. 获取故事数据（如果存在）
	var storyData *models.StoryData
	if s.StoryService != nil {
		storyData, _ = s.StoryService.GetStoryData(sceneID, nil)
	}

	// 5. 分析和统计数据
	stats := s.analyzeInteractionStatistics(conversations, sceneData.Characters)

	// 6. 生成摘要内容
	summary := s.generateInteractionSummary(sceneData, conversations, storyData, stats)

	// 7. 根据格式生成内容
	content, err := s.formatExportContent(sceneData, conversations, summary, stats, format)
	if err != nil {
		return nil, fmt.Errorf("格式化导出内容失败: %w", err)
	}

	// 8. 创建导出结果
	result := &models.ExportResult{
		SceneID:          sceneID,
		Title:            fmt.Sprintf("%s - 交互摘要", sceneData.Scene.Title),
		Format:           format,
		Content:          content,
		GeneratedAt:      time.Now(),
		Characters:       sceneData.Characters,
		Conversations:    conversations,
		Summary:          summary,
		InteractionStats: stats,
	}

	// 9. 保存到 data 目录
	filePath, fileSize, err := s.saveExportToDataDir(result)
	if err != nil {
		return nil, fmt.Errorf("保存导出文件失败: %w", err)
	}

	result.FilePath = filePath
	result.FileSize = fileSize

	return result, nil
}

// getInteractionHistory 获取交互历史
func (s *ExportService) getInteractionHistory(sceneID string) ([]models.Conversation, error) {
	if s.ContextService == nil {
		return []models.Conversation{}, nil
	}

	// 获取角色互动类型的对话
	filter := map[string]interface{}{
		"conversation_type": "character_interaction",
	}

	conversations, err := s.ContextService.GetCharacterInteractions(sceneID, filter, 1000)
	if err != nil {
		// 如果获取失败，尝试获取所有对话
		allConversations, fallbackErr := s.ContextService.GetRecentConversations(sceneID, 1000)
		if fallbackErr != nil {
			return nil, fmt.Errorf("获取对话历史失败: %w", err)
		}
		return allConversations, nil
	}

	return conversations, nil
}

// analyzeInteractionStatistics 分析交互统计
func (s *ExportService) analyzeInteractionStatistics(
	conversations []models.Conversation,
	characters []*models.Character) *models.InteractionExportStats {

	stats := &models.InteractionExportStats{
		EmotionDistribution: make(map[string]int),
		TopKeywords:         []string{},
	}

	if len(conversations) == 0 {
		return stats
	}

	// 基础统计
	stats.TotalMessages = len(conversations)
	stats.CharacterCount = len(characters)

	// 日期范围
	if len(conversations) > 0 {
		stats.DateRange.StartDate = conversations[0].Timestamp
		stats.DateRange.EndDate = conversations[0].Timestamp

		for _, conv := range conversations {
			if conv.Timestamp.Before(stats.DateRange.StartDate) {
				stats.DateRange.StartDate = conv.Timestamp
			}
			if conv.Timestamp.After(stats.DateRange.EndDate) {
				stats.DateRange.EndDate = conv.Timestamp
			}
		}
	}

	// 统计交互次数（按 interaction_id 分组）
	interactionIDs := make(map[string]bool)
	wordCount := make(map[string]int)

	for _, conv := range conversations {
		// 统计独立交互
		if conv.Metadata != nil {
			if interactionID, exists := conv.Metadata["interaction_id"]; exists {
				interactionIDs[fmt.Sprintf("%v", interactionID)] = true
			}
		}

		// 情绪分布统计
		if len(conv.Emotions) > 0 {
			for _, emotion := range conv.Emotions {
				stats.EmotionDistribution[emotion]++
			}
		}

		// 关键词统计
		words := strings.Fields(strings.ToLower(conv.Content))
		for _, word := range words {
			if len(word) > 3 && !isCommonWord(word) {
				wordCount[word]++
			}
		}
	}

	stats.TotalInteractions = len(interactionIDs)

	// 提取热门关键词
	stats.TopKeywords = extractTopKeywords(wordCount, 10)

	return stats
}

// generateInteractionSummary 生成交互摘要
func (s *ExportService) generateInteractionSummary(
	sceneData *SceneData,
	conversations []models.Conversation,
	storyData *models.StoryData,
	stats *models.InteractionExportStats) string {

	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("## %s - 交互摘要报告\n\n", sceneData.Scene.Title))

	// 基础信息
	summary.WriteString("### 基础信息\n\n")
	summary.WriteString(fmt.Sprintf("- **场景**: %s\n", sceneData.Scene.Title))
	summary.WriteString(fmt.Sprintf("- **描述**: %s\n", sceneData.Scene.Description))
	summary.WriteString(fmt.Sprintf("- **角色数量**: %d\n", len(sceneData.Characters)))
	summary.WriteString(fmt.Sprintf("- **总交互次数**: %d\n", stats.TotalInteractions))
	summary.WriteString(fmt.Sprintf("- **总消息数**: %d\n", stats.TotalMessages))

	if !stats.DateRange.StartDate.IsZero() {
		summary.WriteString(fmt.Sprintf("- **时间范围**: %s 至 %s\n",
			stats.DateRange.StartDate.Format("2006-01-02 15:04"),
			stats.DateRange.EndDate.Format("2006-01-02 15:04")))
	}
	summary.WriteString("\n")

	// ✅ 利用 conversations 参数分析对话特征
	if len(conversations) > 0 {
		summary.WriteString("### 对话概览\n\n")

		// 分析发言者分布
		speakerCount := make(map[string]int)
		userMessages := 0
		characterMessages := 0

		for _, conv := range conversations {
			speakerCount[conv.Speaker]++

			// 通过metadata判断是用户还是角色消息
			if conv.Metadata != nil {
				if speakerType, exists := conv.Metadata["speaker_type"]; exists {
					switch speakerType {
					case "user":
						userMessages++
					case "character":
						characterMessages++
					case "system":
						// 如果将来需要处理系统消息
						//systemMessages++
					default:
						// 对于未知类型，可以记录警告并按默认规则处理
						if conv.Speaker == "user" || conv.Speaker == "用户" {
							userMessages++
						} else {
							characterMessages++
						}
					}
				}
			}

			// 如果没有metadata，通过Speaker字段判断
			if conv.Speaker == "user" || conv.Speaker == "用户" {
				userMessages++
			} else {
				characterMessages++
			}
		}

		summary.WriteString(fmt.Sprintf("- **用户发言**: %d 条\n", userMessages))
		summary.WriteString(fmt.Sprintf("- **角色响应**: %d 条\n", characterMessages))

		// 最活跃的发言者
		var mostActiveSpeaker string
		maxCount := 0
		for speaker, count := range speakerCount {
			if count > maxCount && speaker != "user" && speaker != "用户" {
				maxCount = count
				mostActiveSpeaker = speaker
			}
		}
		if mostActiveSpeaker != "" {
			summary.WriteString(fmt.Sprintf("- **最活跃角色**: %s (%d 条发言)\n", mostActiveSpeaker, maxCount))
		}

		// 分析对话时间分布
		if len(conversations) >= 2 {
			firstTime := conversations[0].Timestamp
			lastTime := conversations[len(conversations)-1].Timestamp
			duration := lastTime.Sub(firstTime)

			if duration > 0 {
				summary.WriteString(fmt.Sprintf("- **对话时长**: %.1f 分钟\n", duration.Minutes()))
				avgInterval := duration.Seconds() / float64(len(conversations)-1)
				summary.WriteString(fmt.Sprintf("- **平均发言间隔**: %.1f 秒\n", avgInterval))
			}
		}

		summary.WriteString("\n")
	}

	// ✅ 基于 conversations 的内容分析热门话题
	if len(conversations) > 0 {
		summary.WriteString("### 对话内容分析\n\n")

		// 分析消息长度
		totalLength := 0
		longestMessage := ""
		shortestMessage := ""
		minLength := 10000
		maxLength := 0

		for _, conv := range conversations {
			length := len(conv.Content)
			totalLength += length

			if length > maxLength {
				maxLength = length
				longestMessage = conv.Content
				if len(longestMessage) > 100 {
					longestMessage = longestMessage[:100] + "..."
				}
			}

			if length < minLength && length > 0 {
				minLength = length
				shortestMessage = conv.Content
				if len(shortestMessage) > 50 {
					shortestMessage = shortestMessage[:50] + "..."
				}
			}
		}

		if len(conversations) > 0 {
			avgLength := float64(totalLength) / float64(len(conversations))
			summary.WriteString(fmt.Sprintf("- **平均消息长度**: %.1f 字符\n", avgLength))
			summary.WriteString(fmt.Sprintf("- **最长消息**: %d 字符 - \"%s\"\n", maxLength, longestMessage))
			if minLength < 10000 {
				summary.WriteString(fmt.Sprintf("- **最短消息**: %d 字符 - \"%s\"\n", minLength, shortestMessage))
			}
		}

		// ✅ 分析对话中的关键主题
		topicKeywords := make(map[string]int)
		questionCount := 0
		exclamationCount := 0

		for _, conv := range conversations {
			content := strings.ToLower(conv.Content)

			// 统计问句和感叹句
			if strings.Contains(content, "?") || strings.Contains(content, "？") ||
				strings.Contains(content, "什么") || strings.Contains(content, "为什么") ||
				strings.Contains(content, "怎么") || strings.Contains(content, "如何") {
				questionCount++
			}

			if strings.Contains(content, "!") || strings.Contains(content, "！") {
				exclamationCount++
			}

			// 分析话题关键词
			words := strings.Fields(content)
			for _, word := range words {
				if len(word) > 2 && !isCommonWord(word) {
					topicKeywords[word]++
				}
			}
		}

		summary.WriteString(fmt.Sprintf("- **问句数量**: %d 条 (%.1f%%)\n",
			questionCount, float64(questionCount)/float64(len(conversations))*100))
		summary.WriteString(fmt.Sprintf("- **感叹句数量**: %d 条 (%.1f%%)\n",
			exclamationCount, float64(exclamationCount)/float64(len(conversations))*100))

		// 显示热门话题词汇
		if len(topicKeywords) > 0 {
			type wordFreq struct {
				word  string
				count int
			}

			var frequencies []wordFreq
			for word, count := range topicKeywords {
				if count >= 2 { // 只显示出现2次以上的词汇
					frequencies = append(frequencies, wordFreq{word, count})
				}
			}

			sort.Slice(frequencies, func(i, j int) bool {
				return frequencies[i].count > frequencies[j].count
			})

			if len(frequencies) > 0 {
				summary.WriteString("- **热门话题词汇**: ")
				for i, freq := range frequencies {
					if i >= 5 {
						break
					} // 只显示前5个
					if i > 0 {
						summary.WriteString(", ")
					}
					summary.WriteString(fmt.Sprintf("%s(%d次)", freq.word, freq.count))
				}
				summary.WriteString("\n")
			}
		}

		summary.WriteString("\n")
	}

	// ✅ 基于 conversations 的情绪变化分析
	if len(conversations) > 0 {
		emotionFlow := []string{}
		emotionChanges := 0
		prevEmotion := ""

		for _, conv := range conversations {
			if len(conv.Emotions) > 0 {
				currentEmotion := conv.Emotions[0] // 取主要情绪
				emotionFlow = append(emotionFlow, currentEmotion)

				if prevEmotion != "" && prevEmotion != currentEmotion {
					emotionChanges++
				}
				prevEmotion = currentEmotion
			}
		}

		if len(emotionFlow) > 0 {
			summary.WriteString("### 情绪变化轨迹\n\n")
			summary.WriteString(fmt.Sprintf("- **情绪变化次数**: %d 次\n", emotionChanges))

			if len(emotionFlow) <= 10 {
				summary.WriteString(fmt.Sprintf("- **情绪流程**: %s\n", strings.Join(emotionFlow, " → ")))
			} else {
				// 显示前5个和后5个情绪
				start := strings.Join(emotionFlow[:5], " → ")
				end := strings.Join(emotionFlow[len(emotionFlow)-5:], " → ")
				summary.WriteString(fmt.Sprintf("- **情绪流程**: %s → ... → %s\n", start, end))
			}

			// 分析情绪稳定性
			stabilityRatio := 1.0 - float64(emotionChanges)/float64(len(emotionFlow))
			var stabilityLevel string
			switch {
			case stabilityRatio >= 0.8:
				stabilityLevel = "非常稳定"
			case stabilityRatio >= 0.6:
				stabilityLevel = "相对稳定"
			case stabilityRatio >= 0.4:
				stabilityLevel = "变化适中"
			case stabilityRatio >= 0.2:
				stabilityLevel = "变化较大"
			default:
				stabilityLevel = "情绪波动剧烈"
			}
			summary.WriteString(fmt.Sprintf("- **情绪稳定性**: %s (%.1f%%)\n", stabilityLevel, stabilityRatio*100))
			summary.WriteString("\n")
		}
	}

	// ✅ 基于 conversations 分析交互模式
	if len(conversations) > 0 {
		summary.WriteString("### 交互模式分析\n\n")

		// 分析互动ID分布（如果有）
		interactionGroups := make(map[string]int)
		multiCharacterInteractions := 0

		for _, conv := range conversations {
			if conv.Metadata != nil {
				if interactionID, exists := conv.Metadata["interaction_id"]; exists {
					interactionGroups[fmt.Sprintf("%v", interactionID)]++
				}

				if characterIDs, exists := conv.Metadata["character_ids"]; exists {
					if charIDSlice, ok := characterIDs.([]interface{}); ok {
						if len(charIDSlice) > 1 {
							multiCharacterInteractions++
						}
					}
				}
			}
		}

		if len(interactionGroups) > 0 {
			summary.WriteString(fmt.Sprintf("- **独立交互会话**: %d 个\n", len(interactionGroups)))

			// 分析交互规模
			totalMessages := 0
			maxMessagesInInteraction := 0
			for _, count := range interactionGroups {
				totalMessages += count
				if count > maxMessagesInInteraction {
					maxMessagesInInteraction = count
				}
			}

			avgMessagesPerInteraction := float64(totalMessages) / float64(len(interactionGroups))
			summary.WriteString(fmt.Sprintf("- **平均每次交互消息数**: %.1f 条\n", avgMessagesPerInteraction))
			summary.WriteString(fmt.Sprintf("- **最长交互会话**: %d 条消息\n", maxMessagesInInteraction))
		}

		if multiCharacterInteractions > 0 {
			summary.WriteString(fmt.Sprintf("- **多角色互动消息**: %d 条\n", multiCharacterInteractions))
			multiCharRatio := float64(multiCharacterInteractions) / float64(len(conversations)) * 100
			summary.WriteString(fmt.Sprintf("- **群体互动比例**: %.1f%%\n", multiCharRatio))
		}

		summary.WriteString("\n")
	}

	// 角色信息
	summary.WriteString("### 参与角色\n\n")
	for _, char := range sceneData.Characters {
		summary.WriteString(fmt.Sprintf("- **%s**: %s\n", char.Name, char.Description))
	}
	summary.WriteString("\n")

	// 情绪分布（保持原有逻辑）
	if len(stats.EmotionDistribution) > 0 {
		summary.WriteString("### 情绪分布统计\n\n")
		for emotion, count := range stats.EmotionDistribution {
			percentage := float64(count) / float64(stats.TotalMessages) * 100
			summary.WriteString(fmt.Sprintf("- **%s**: %d次 (%.1f%%)\n", emotion, count, percentage))
		}
		summary.WriteString("\n")
	}

	// 热门关键词（保持原有逻辑）
	if len(stats.TopKeywords) > 0 {
		summary.WriteString("### 热门话题关键词\n\n")
		for i, keyword := range stats.TopKeywords {
			if i >= 10 {
				break
			}
			summary.WriteString(fmt.Sprintf("%d. %s\n", i+1, keyword))
		}
		summary.WriteString("\n")
	}

	// 故事进度（如果有）
	if storyData != nil {
		summary.WriteString("### 故事进度\n\n")
		summary.WriteString(fmt.Sprintf("- **当前进度**: %d%%\n", storyData.Progress))
		summary.WriteString(fmt.Sprintf("- **当前状态**: %s\n", storyData.CurrentState))

		completedTasks := 0
		for _, task := range storyData.Tasks {
			if task.Completed {
				completedTasks++
			}
		}
		summary.WriteString(fmt.Sprintf("- **已完成任务**: %d / %d\n", completedTasks, len(storyData.Tasks)))
		summary.WriteString("\n")
	}

	// 活跃度分析（保持原有逻辑）
	summary.WriteString("### 交互活跃度分析\n\n")
	if stats.TotalInteractions > 0 {
		avgMessagesPerInteraction := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
		summary.WriteString(fmt.Sprintf("- **平均每次交互消息数**: %.1f\n", avgMessagesPerInteraction))

		// 简单的活跃度评级
		var activityLevel string
		switch {
		case avgMessagesPerInteraction >= 5:
			activityLevel = "非常活跃"
		case avgMessagesPerInteraction >= 3:
			activityLevel = "活跃"
		case avgMessagesPerInteraction >= 2:
			activityLevel = "一般"
		default:
			activityLevel = "较少互动"
		}
		summary.WriteString(fmt.Sprintf("- **活跃度评级**: %s\n", activityLevel))
	}

	// ✅ 基于 conversations 添加互动质量评估
	if len(conversations) > 0 {
		summary.WriteString("\n### 互动质量评估\n\n")

		// 计算互动深度指标
		totalContentLength := 0
		substantialMessages := 0 // 超过50字符的消息

		for _, conv := range conversations {
			contentLength := len(conv.Content)
			totalContentLength += contentLength

			if contentLength > 50 {
				substantialMessages++
			}
		}

		avgContentLength := float64(totalContentLength) / float64(len(conversations))
		substantialRatio := float64(substantialMessages) / float64(len(conversations)) * 100

		summary.WriteString(fmt.Sprintf("- **平均内容深度**: %.1f 字符/消息\n", avgContentLength))
		summary.WriteString(fmt.Sprintf("- **深度消息比例**: %.1f%% (%d/%d)\n",
			substantialRatio, substantialMessages, len(conversations)))

		// 互动质量评级
		var qualityLevel string
		qualityScore := (avgContentLength/100)*0.6 + (substantialRatio/100)*0.4

		switch {
		case qualityScore >= 0.8:
			qualityLevel = "优秀 - 内容丰富，互动深入"
		case qualityScore >= 0.6:
			qualityLevel = "良好 - 内容充实，互动较好"
		case qualityScore >= 0.4:
			qualityLevel = "一般 - 基础互动，有改进空间"
		case qualityScore >= 0.2:
			qualityLevel = "待提升 - 互动较浅，建议增加内容深度"
		default:
			qualityLevel = "需要改进 - 互动内容过于简单"
		}

		summary.WriteString(fmt.Sprintf("- **互动质量评级**: %s\n", qualityLevel))
	}

	return summary.String()
}

// formatExportContent 根据格式生成内容
func (s *ExportService) formatExportContent(
	sceneData *SceneData,
	conversations []models.Conversation,
	summary string,
	stats *models.InteractionExportStats,
	format string) (string, error) {

	switch strings.ToLower(format) {
	case "json":
		return s.formatAsJSON(sceneData, conversations, summary, stats)
	case "markdown":
		return s.formatAsMarkdown(sceneData, conversations, summary, stats)
	case "txt":
		return s.formatAsText(sceneData, conversations, summary, stats)
	case "html":
		return s.formatAsHTML(sceneData, conversations, summary, stats)
	default:
		return "", fmt.Errorf("不支持的格式: %s", format)
	}
}

// formatAsJSON JSON格式导出
func (s *ExportService) formatAsJSON(
	sceneData *SceneData,
	conversations []models.Conversation,
	summary string,
	stats *models.InteractionExportStats) (string, error) {

	// ✅ 添加空值检查
	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	if stats == nil {
		return "", fmt.Errorf("统计数据不能为空")
	}

	exportData := map[string]interface{}{
		"scene_info": map[string]interface{}{
			"id":          sceneData.Scene.ID,
			"title":       sceneData.Scene.Title,
			"description": sceneData.Scene.Description,
			"created_at":  sceneData.Scene.CreatedAt.Format("2006-01-02 15:04:05"),
		},
		"summary":       summary,
		"statistics":    stats,
		"characters":    sceneData.Characters,
		"conversations": conversations,
		"export_info": map[string]interface{}{
			"generated_at": time.Now().Format("2006-01-02 15:04:05"),
			"format":       "json",
			"version":      "1.0",
		},
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON序列化失败: %w", err)
	}

	return string(jsonData), nil
}

// formatAsMarkdown Markdown格式导出
func (s *ExportService) formatAsMarkdown(
	sceneData *SceneData,
	conversations []models.Conversation,
	summary string,
	stats *models.InteractionExportStats) (string, error) {

	// ✅ 添加空值检查
	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	if stats == nil {
		return "", fmt.Errorf("统计数据不能为空")
	}

	var content strings.Builder

	// 标题
	content.WriteString(fmt.Sprintf("# %s - 交互摘要报告\n\n", sceneData.Scene.Title))
	content.WriteString(fmt.Sprintf("**场景ID**: %s\n\n", sceneData.Scene.ID))

	// 基础信息
	content.WriteString("## 基础信息\n\n")
	content.WriteString(fmt.Sprintf("- **场景ID**: %s\n", sceneData.Scene.ID))
	content.WriteString(fmt.Sprintf("- **场景名称**: %s\n", sceneData.Scene.Title))
	content.WriteString(fmt.Sprintf("- **场景描述**: %s\n", sceneData.Scene.Description))
	content.WriteString(fmt.Sprintf("- **生成时间**: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// 统计信息
	content.WriteString("## 统计数据\n\n")
	content.WriteString(fmt.Sprintf("- **总消息数**: %d\n", stats.TotalMessages))
	content.WriteString(fmt.Sprintf("- **参与角色数**: %d\n", stats.CharacterCount))
	content.WriteString(fmt.Sprintf("- **交互次数**: %d\n\n", stats.TotalInteractions))

	// 摘要内容
	content.WriteString("## 详细分析\n\n")
	content.WriteString(summary)
	content.WriteString("\n\n")

	// ✅ 利用 sceneData 添加详细的场景信息头部
	content.WriteString(fmt.Sprintf("# %s - 完整交互记录\n\n", sceneData.Scene.Title))

	// 场景基本信息
	content.WriteString("## 📋 场景信息\n\n")
	content.WriteString(fmt.Sprintf("- **场景ID**: %s\n", sceneData.Scene.ID))
	content.WriteString(fmt.Sprintf("- **场景名称**: %s\n", sceneData.Scene.Title))
	content.WriteString(fmt.Sprintf("- **场景描述**: %s\n", sceneData.Scene.Description))

	if sceneData.Scene.Source != "" {
		content.WriteString(fmt.Sprintf("- **数据来源**: %s\n", sceneData.Scene.Source))
	}

	content.WriteString(fmt.Sprintf("- **创建时间**: %s\n", sceneData.Scene.CreatedAt.Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("- **最后访问**: %s\n", sceneData.Scene.LastAccessed.Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("- **最后更新**: %s\n", sceneData.Scene.LastUpdated.Format("2006-01-02 15:04:05")))

	// ✅ 利用 sceneData 添加场景环境信息
	if len(sceneData.Scene.Locations) > 0 {
		content.WriteString(fmt.Sprintf("- **可用地点**: %d 个\n", len(sceneData.Scene.Locations)))
		for _, location := range sceneData.Scene.Locations {
			content.WriteString(fmt.Sprintf("  - %s: %s\n", location.Name, location.Description))
		}
	}

	if len(sceneData.Scene.Themes) > 0 {
		content.WriteString(fmt.Sprintf("- **主要主题**: %s\n", strings.Join(sceneData.Scene.Themes, ", ")))
	}

	if sceneData.Scene.Era != "" {
		content.WriteString(fmt.Sprintf("- **时代背景**: %s\n", sceneData.Scene.Era))
	}

	if sceneData.Scene.Atmosphere != "" {
		content.WriteString(fmt.Sprintf("- **氛围设定**: %s\n", sceneData.Scene.Atmosphere))
	}

	// ✅ 利用 sceneData.Items 添加道具信息
	if len(sceneData.Scene.Items) > 0 {
		content.WriteString(fmt.Sprintf("- **可用道具**: %d 个\n", len(sceneData.Scene.Items)))
		for _, item := range sceneData.Scene.Items {
			content.WriteString(fmt.Sprintf("  - **%s**: %s", item.Name, item.Description))
			if item.Type != "" {
				content.WriteString(fmt.Sprintf(" (类型: %s)", item.Type))
			}
			content.WriteString("\n")
		}
	}

	// ✅ 利用 sceneData.Characters 添加详细角色信息
	content.WriteString(fmt.Sprintf("\n## 👥 参与角色 (%d 位)\n\n", len(sceneData.Characters)))
	for i, char := range sceneData.Characters {
		content.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, char.Name))
		content.WriteString(fmt.Sprintf("- **角色ID**: %s\n", char.ID))
		content.WriteString(fmt.Sprintf("- **描述**: %s\n", char.Description))

		if char.Role != "" {
			content.WriteString(fmt.Sprintf("- **角色**: %s\n", char.Role))
		}

		if char.Personality != "" {
			content.WriteString(fmt.Sprintf("- **性格**: %s\n", char.Personality))
		}

		if char.Background != "" {
			content.WriteString(fmt.Sprintf("- **背景**: %s\n", char.Background))
		}

		if len(char.Knowledge) > 0 {
			content.WriteString(fmt.Sprintf("- **知识领域**: %s\n", strings.Join(char.Knowledge, ", ")))
		}

		if char.Role != "" {
			content.WriteString(fmt.Sprintf("- **角色**: %s\n", char.Role))
		}

		if char.SpeechStyle != "" {
			content.WriteString(fmt.Sprintf("- **说话风格**: %s\n", char.SpeechStyle))
		}

		if char.Novel != "" {
			content.WriteString(fmt.Sprintf("- **出处**: %s\n", char.Novel))
		}

		if len(char.Relationships) > 0 {
			content.WriteString("- **人际关系**:\n")
			for otherID, relationship := range char.Relationships {
				// 找到对应角色名称
				var otherName string
				for _, otherChar := range sceneData.Characters {
					if otherChar.ID == otherID {
						otherName = otherChar.Name
						break
					}
				}
				if otherName != "" {
					content.WriteString(fmt.Sprintf("  - 与%s: %s\n", otherName, relationship))
				} else {
					content.WriteString(fmt.Sprintf("  - 与%s: %s\n", otherID, relationship))
				}
			}
		}

		content.WriteString(fmt.Sprintf("- **创建时间**: %s\n", char.CreatedAt.Format("2006-01-02 15:04")))
		if !char.LastUpdated.IsZero() && !char.LastUpdated.Equal(char.CreatedAt) {
			content.WriteString(fmt.Sprintf("- **最后更新**: %s\n", char.LastUpdated.Format("2006-01-02 15:04")))
		}

		content.WriteString("\n")
	}

	// ✅ 利用 stats 添加详细统计信息
	content.WriteString("## 📊 交互统计摘要\n\n")

	// 基础统计
	content.WriteString("### 基础数据\n\n")
	content.WriteString(fmt.Sprintf("- **总交互次数**: %d\n", stats.TotalInteractions))
	content.WriteString(fmt.Sprintf("- **总消息数量**: %d\n", stats.TotalMessages))
	content.WriteString(fmt.Sprintf("- **参与角色数**: %d\n", stats.CharacterCount))

	// 时间范围信息
	if !stats.DateRange.StartDate.IsZero() && !stats.DateRange.EndDate.IsZero() {
		duration := stats.DateRange.EndDate.Sub(stats.DateRange.StartDate)
		content.WriteString(fmt.Sprintf("- **交互时间范围**: %s 至 %s\n",
			stats.DateRange.StartDate.Format("2006-01-02 15:04:05"),
			stats.DateRange.EndDate.Format("2006-01-02 15:04:05")))
		content.WriteString(fmt.Sprintf("- **总交互时长**: %.1f 分钟\n", duration.Minutes()))

		if stats.TotalInteractions > 0 {
			avgDuration := duration.Minutes() / float64(stats.TotalInteractions)
			content.WriteString(fmt.Sprintf("- **平均交互时长**: %.1f 分钟\n", avgDuration))
		}

		if stats.TotalMessages > 1 {
			avgInterval := duration.Seconds() / float64(stats.TotalMessages-1)
			content.WriteString(fmt.Sprintf("- **平均消息间隔**: %.1f 秒\n", avgInterval))
		}
	}

	// 计算平均值
	if stats.TotalInteractions > 0 {
		avgMessagesPerInteraction := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
		content.WriteString(fmt.Sprintf("- **平均每次交互消息数**: %.1f\n", avgMessagesPerInteraction))
	}

	content.WriteString("\n")

	// ✅ 利用 stats.EmotionDistribution 添加情绪分析
	if len(stats.EmotionDistribution) > 0 {
		content.WriteString("### 情绪分布分析\n\n")

		// 按频次排序情绪
		type emotionStat struct {
			emotion string
			count   int
		}

		var emotionStats []emotionStat
		totalEmotionCount := 0
		for emotion, count := range stats.EmotionDistribution {
			emotionStats = append(emotionStats, emotionStat{emotion, count})
			totalEmotionCount += count
		}

		sort.Slice(emotionStats, func(i, j int) bool {
			return emotionStats[i].count > emotionStats[j].count
		})

		content.WriteString("| 情绪类型 | 出现次数 | 占比 | 可视化 |\n")
		content.WriteString("|---------|----------|------|--------|\n")

		for _, stat := range emotionStats {
			percentage := float64(stat.count) / float64(totalEmotionCount) * 100

			// 创建简单的条形图可视化
			barLength := int(percentage / 5) // 每5%一个字符
			if barLength > 20 {
				barLength = 20
			} // 最大20个字符
			bar := strings.Repeat("█", barLength) + strings.Repeat("░", 20-barLength)

			content.WriteString(fmt.Sprintf("| %s | %d | %.1f%% | %s |\n",
				stat.emotion, stat.count, percentage, bar))
		}

		content.WriteString("\n")

		// 情绪分析洞察
		if len(emotionStats) > 0 {
			dominantEmotion := emotionStats[0]
			content.WriteString("#### 情绪分析洞察\n\n")
			content.WriteString(fmt.Sprintf("- **主导情绪**: %s (%.1f%%，%d次)\n",
				dominantEmotion.emotion,
				float64(dominantEmotion.count)/float64(totalEmotionCount)*100,
				dominantEmotion.count))

			// 情绪多样性分析
			emotionDiversity := len(emotionStats)
			content.WriteString(fmt.Sprintf("- **情绪多样性**: %d 种不同情绪\n", emotionDiversity))

			var diversityLevel string
			switch {
			case emotionDiversity >= 8:
				diversityLevel = "极高 - 情绪表达非常丰富"
			case emotionDiversity >= 6:
				diversityLevel = "高 - 情绪变化多样"
			case emotionDiversity >= 4:
				diversityLevel = "中等 - 有一定情绪变化"
			case emotionDiversity >= 2:
				diversityLevel = "较低 - 情绪相对单一"
			default:
				diversityLevel = "低 - 情绪表达单调"
			}
			content.WriteString(fmt.Sprintf("- **多样性评级**: %s\n", diversityLevel))

			content.WriteString("\n")
		}
	}

	// ✅ 利用 stats.TopKeywords 添加关键词分析
	if len(stats.TopKeywords) > 0 {
		content.WriteString("### 热门话题关键词\n\n")
		content.WriteString("以下是对话中出现频率最高的关键词：\n\n")

		// 创建编号列表
		for i, keyword := range stats.TopKeywords {
			if i >= 15 {
				break
			} // 最多显示15个关键词

			// 使用不同的标记符号来增加视觉层次
			var marker string
			switch {
			case i < 3:
				marker = "🥇🥈🥉"[i*3 : i*3+3] // 前三名使用奖牌
			case i < 5:
				marker = "⭐"
			case i < 10:
				marker = "▶️"
			default:
				marker = "•"
			}

			content.WriteString(fmt.Sprintf("%s **%s**\n", marker, keyword))
		}

		content.WriteString("\n")

		// 关键词洞察
		content.WriteString("#### 话题分析洞察\n\n")
		content.WriteString(fmt.Sprintf("- **关键词总数**: %d 个\n", len(stats.TopKeywords)))

		// 简单的主题分类（基于关键词）
		topicCategories := map[string][]string{
			"情感相关": {},
			"行动相关": {},
			"对象相关": {},
			"描述相关": {},
		}

		emotionWords := []string{"喜欢", "讨厌", "开心", "难过", "愤怒", "恐惧", "love", "hate", "happy", "sad", "angry", "fear"}
		actionWords := []string{"做", "去", "来", "看", "听", "说", "走", "跑", "do", "go", "come", "see", "hear", "say", "walk", "run"}
		objectWords := []string{"东西", "物品", "书", "食物", "水", "房子", "thing", "item", "book", "food", "water", "house"}

		for _, keyword := range stats.TopKeywords {
			keywordLower := strings.ToLower(keyword)
			categorized := false

			for _, emotion := range emotionWords {
				if strings.Contains(keywordLower, emotion) {
					topicCategories["情感相关"] = append(topicCategories["情感相关"], keyword)
					categorized = true
					break
				}
			}

			if !categorized {
				for _, action := range actionWords {
					if strings.Contains(keywordLower, action) {
						topicCategories["行动相关"] = append(topicCategories["行动相关"], keyword)
						categorized = true
						break
					}
				}
			}

			if !categorized {
				for _, object := range objectWords {
					if strings.Contains(keywordLower, object) {
						topicCategories["对象相关"] = append(topicCategories["对象相关"], keyword)
						categorized = true
						break
					}
				}
			}

			if !categorized {
				topicCategories["描述相关"] = append(topicCategories["描述相关"], keyword)
			}
		}

		// 显示话题分类
		for category, words := range topicCategories {
			if len(words) > 0 {
				content.WriteString(fmt.Sprintf("- **%s**: %s\n", category, strings.Join(words, ", ")))
			}
		}

		content.WriteString("\n")
	}

	// 添加摘要内容（保持原有的summary）
	content.WriteString("## 📝 详细分析报告\n\n")
	content.WriteString(summary)

	// ✅ 利用 conversations 和 sceneData 添加详细对话记录
	content.WriteString("\n## 💬 完整对话记录\n\n")

	// 按交互分组
	interactionGroups := s.groupConversationsByInteraction(conversations)

	for i, group := range interactionGroups {
		content.WriteString(fmt.Sprintf("### 📖 交互会话 #%d\n\n", i+1))

		if len(group) > 0 && !group[0].Timestamp.IsZero() {
			content.WriteString(fmt.Sprintf("**⏰ 开始时间**: %s\n", group[0].Timestamp.Format("2006-01-02 15:04:05")))

			if len(group) > 1 {
				lastTime := group[len(group)-1].Timestamp
				duration := lastTime.Sub(group[0].Timestamp)
				content.WriteString(fmt.Sprintf("**⏱️ 持续时间**: %.1f 分钟\n", duration.Minutes()))
			}

			content.WriteString(fmt.Sprintf("**💭 消息数量**: %d 条\n\n", len(group)))
		}

		// 显示对话内容
		for j, conv := range group {
			// 尝试从角色列表中找到角色名称
			speakerName := conv.Speaker
			if conv.SpeakerID != "" && conv.SpeakerID != "user" {
				for _, char := range sceneData.Characters {
					if char.ID == conv.SpeakerID {
						speakerName = char.Name
						break
					}
				}
			}

			// 添加消息序号和时间戳
			if !conv.Timestamp.IsZero() {
				content.WriteString(fmt.Sprintf("**[%d]** `%s` **%s**:\n",
					j+1, conv.Timestamp.Format("15:04:05"), speakerName))
			} else {
				content.WriteString(fmt.Sprintf("**[%d]** **%s**:\n", j+1, speakerName))
			}

			// 消息内容
			content.WriteString(fmt.Sprintf("> %s\n", conv.Content))

			// 添加情绪信息
			if len(conv.Emotions) > 0 {
				content.WriteString(fmt.Sprintf("*🎭 情绪: %s*\n", strings.Join(conv.Emotions, ", ")))
			}

			// 添加元数据信息（如果有重要信息）
			if conv.Metadata != nil {
				if messageType, exists := conv.Metadata["message_type"]; exists {
					content.WriteString(fmt.Sprintf("*📝 类型: %v*\n", messageType))
				}
			}

			content.WriteString("\n")
		}

		content.WriteString("---\n\n")
	}

	// ✅ 利用 sceneData 和 stats 添加总结和洞察
	content.WriteString("## 🎯 总结与洞察\n\n")

	// 交互效率分析
	content.WriteString("### 交互效率\n\n")
	if stats.TotalInteractions > 0 && stats.TotalMessages > 0 {
		efficiency := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
		content.WriteString(fmt.Sprintf("- **交互效率**: %.1f 消息/交互\n", efficiency))

		var efficiencyLevel string
		switch {
		case efficiency >= 8:
			efficiencyLevel = "极高 - 深度对话，内容丰富"
		case efficiency >= 5:
			efficiencyLevel = "高 - 有效互动，内容充实"
		case efficiency >= 3:
			efficiencyLevel = "中等 - 正常交流水平"
		case efficiency >= 1.5:
			efficiencyLevel = "较低 - 互动较为简短"
		default:
			efficiencyLevel = "低 - 需要提升互动深度"
		}
		content.WriteString(fmt.Sprintf("- **效率评级**: %s\n", efficiencyLevel))
	}

	// 角色参与度分析
	content.WriteString("\n### 角色参与度\n\n")
	characterParticipation := make(map[string]int)
	for _, conv := range conversations {
		if conv.SpeakerID != "" && conv.SpeakerID != "user" {
			characterParticipation[conv.SpeakerID]++
		}
	}

	if len(characterParticipation) > 0 {
		content.WriteString("| 角色 | 发言次数 | 参与度 |\n")
		content.WriteString("|------|----------|--------|\n")

		type charStat struct {
			id    string
			name  string
			count int
		}

		var charStats []charStat
		totalParticipation := 0

		for charID, count := range characterParticipation {
			// 找到角色名称
			var charName string
			for _, char := range sceneData.Characters {
				if char.ID == charID {
					charName = char.Name
					break
				}
			}
			if charName == "" {
				charName = charID
			}

			charStats = append(charStats, charStat{charID, charName, count})
			totalParticipation += count
		}

		// 按参与度排序
		sort.Slice(charStats, func(i, j int) bool {
			return charStats[i].count > charStats[j].count
		})

		for _, stat := range charStats {
			percentage := float64(stat.count) / float64(totalParticipation) * 100
			content.WriteString(fmt.Sprintf("| %s | %d | %.1f%% |\n",
				stat.name, stat.count, percentage))
		}
	}

	// 场景利用度分析
	content.WriteString("\n### 场景要素利用度\n\n")

	// 分析地点提及情况
	if len(sceneData.Scene.Locations) > 0 {
		locationMentions := make(map[string]int)
		allText := ""
		for _, conv := range conversations {
			allText += strings.ToLower(conv.Content) + " "
		}

		for _, location := range sceneData.Scene.Locations {
			locationName := strings.ToLower(location.Name)
			count := strings.Count(allText, locationName)
			if count > 0 {
				locationMentions[location.Name] = count
			}
		}

		if len(locationMentions) > 0 {
			content.WriteString("#### 地点利用情况\n\n")
			for locationName, count := range locationMentions {
				content.WriteString(fmt.Sprintf("- **%s**: 提及 %d 次\n", locationName, count))
			}

			utilizationRate := float64(len(locationMentions)) / float64(len(sceneData.Scene.Locations)) * 100
			content.WriteString(fmt.Sprintf("- **地点利用率**: %.1f%% (%d/%d)\n\n",
				utilizationRate, len(locationMentions), len(sceneData.Scene.Locations)))
		}
	}

	// 分析道具提及情况
	if len(sceneData.Scene.Items) > 0 {
		itemMentions := make(map[string]int)
		allText := ""
		for _, conv := range conversations {
			allText += strings.ToLower(conv.Content) + " "
		}

		for _, item := range sceneData.Scene.Items {
			itemName := strings.ToLower(item.Name)
			count := strings.Count(allText, itemName)
			if count > 0 {
				itemMentions[item.Name] = count
			}
		}

		if len(itemMentions) > 0 {
			content.WriteString("#### 道具利用情况\n\n")
			for itemName, count := range itemMentions {
				content.WriteString(fmt.Sprintf("- **%s**: 提及 %d 次\n", itemName, count))
			}

			utilizationRate := float64(len(itemMentions)) / float64(len(sceneData.Scene.Items)) * 100
			content.WriteString(fmt.Sprintf("- **道具利用率**: %.1f%% (%d/%d)\n\n",
				utilizationRate, len(itemMentions), len(sceneData.Scene.Items)))
		}
	}

	// 添加导出信息
	content.WriteString("## 📄 导出信息\n\n")
	content.WriteString(fmt.Sprintf("- **导出时间**: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString("- **导出格式**: Markdown\n")
	content.WriteString("- **数据来源**: SceneIntruderMCP 交互聚合服务\n")
	content.WriteString("- **版本**: v1.0\n")

	// ✅ 利用 sceneData 添加场景元数据
	content.WriteString(fmt.Sprintf("- **场景数据版本**: %s\n", sceneData.Scene.LastUpdated.Format("2006-01-02")))
	content.WriteString(fmt.Sprintf("- **统计数据包含**: %d 条对话，%d 次交互\n",
		stats.TotalMessages, stats.TotalInteractions))

	return content.String(), nil
}

// formatAsText 纯文本格式导出
func (s *ExportService) formatAsText(
	sceneData *SceneData,
	conversations []models.Conversation,
	summary string,
	stats *models.InteractionExportStats) (string, error) {

	// ✅ 添加空值检查
	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	if stats == nil {
		return "", fmt.Errorf("统计数据不能为空")
	}

	var content strings.Builder

	// 标题和分隔符
	title := fmt.Sprintf("%s - 交互摘要报告", sceneData.Scene.Title)
	content.WriteString(title + "\n")
	content.WriteString(strings.Repeat("=", len(title)) + "\n\n")

	// 基础信息
	content.WriteString("基础信息:\n")
	content.WriteString(fmt.Sprintf("场景ID: %s\n", sceneData.Scene.ID))
	content.WriteString(fmt.Sprintf("场景名称: %s\n", sceneData.Scene.Title))
	content.WriteString(fmt.Sprintf("场景描述: %s\n", sceneData.Scene.Description))
	content.WriteString(fmt.Sprintf("生成时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// 统计信息

	content.WriteString("统计数据:\n")
	content.WriteString(fmt.Sprintf("总消息数: %d\n", stats.TotalMessages))
	content.WriteString(fmt.Sprintf("参与角色数: %d\n", stats.CharacterCount))
	content.WriteString(fmt.Sprintf("交互次数: %d\n\n", stats.TotalInteractions))

	// 摘要内容
	content.WriteString("详细分析:\n")
	content.WriteString(strings.Repeat("-", 20) + "\n")
	content.WriteString(summary)
	content.WriteString("\n\n")

	// ✅ 利用 sceneData 添加详细的场景信息头部
	content.WriteString(strings.Repeat("=", 60) + "\n")
	content.WriteString(fmt.Sprintf("    %s - 完整交互记录\n", sceneData.Scene.Title))
	content.WriteString(strings.Repeat("=", 60) + "\n\n")

	// ✅ 利用 sceneData 添加场景基本信息
	content.WriteString("场景信息\n")
	content.WriteString(strings.Repeat("-", 30) + "\n")
	content.WriteString(fmt.Sprintf("场景ID: %s\n", sceneData.Scene.ID))
	content.WriteString(fmt.Sprintf("场景名称: %s\n", sceneData.Scene.Title))
	content.WriteString(fmt.Sprintf("场景描述: %s\n", sceneData.Scene.Description))

	if sceneData.Scene.Source != "" {
		content.WriteString(fmt.Sprintf("数据来源: %s\n", sceneData.Scene.Source))
	}

	content.WriteString(fmt.Sprintf("创建时间: %s\n", sceneData.Scene.CreatedAt.Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("最后访问: %s\n", sceneData.Scene.LastAccessed.Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("最后更新: %s\n", sceneData.Scene.LastUpdated.Format("2006-01-02 15:04:05")))

	// ✅ 利用 sceneData 添加环境信息
	if len(sceneData.Scene.Locations) > 0 {
		content.WriteString(fmt.Sprintf("可用地点: %d 个\n", len(sceneData.Scene.Locations)))
		for i, location := range sceneData.Scene.Locations {
			if i >= 5 { // 限制显示数量以免太长
				content.WriteString(fmt.Sprintf("  ...以及其他 %d 个地点\n", len(sceneData.Scene.Locations)-5))
				break
			}
			content.WriteString(fmt.Sprintf("  - %s: %s\n", location.Name, location.Description))
		}
	}

	if len(sceneData.Scene.Themes) > 0 {
		content.WriteString(fmt.Sprintf("主要主题: %s\n", strings.Join(sceneData.Scene.Themes, ", ")))
	}

	if sceneData.Scene.Era != "" {
		content.WriteString(fmt.Sprintf("时代背景: %s\n", sceneData.Scene.Era))
	}

	if sceneData.Scene.Atmosphere != "" {
		content.WriteString(fmt.Sprintf("氛围设定: %s\n", sceneData.Scene.Atmosphere))
	}

	// ✅ 利用 sceneData 添加道具信息
	if len(sceneData.Scene.Items) > 0 {
		content.WriteString(fmt.Sprintf("可用道具: %d 个\n", len(sceneData.Scene.Items)))
		for i, item := range sceneData.Scene.Items {
			if i >= 5 { // 限制显示数量
				content.WriteString(fmt.Sprintf("  ...以及其他 %d 个道具\n", len(sceneData.Scene.Items)-5))
				break
			}
			itemDesc := fmt.Sprintf("  - %s: %s", item.Name, item.Description)
			if item.Type != "" {
				itemDesc += fmt.Sprintf(" (类型: %s)", item.Type)
			}
			content.WriteString(itemDesc + "\n")
		}
	}

	content.WriteString("\n")

	// ✅ 利用 sceneData.Characters 添加角色信息
	content.WriteString(fmt.Sprintf("参与角色 (%d 位)\n", len(sceneData.Characters)))
	content.WriteString(strings.Repeat("-", 30) + "\n")

	for i, char := range sceneData.Characters {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, char.Name))
		content.WriteString(fmt.Sprintf("   角色ID: %s\n", char.ID))
		content.WriteString(fmt.Sprintf("   描述: %s\n", char.Description))

		if char.Role != "" {
			content.WriteString(fmt.Sprintf("   角色: %s\n", char.Role))
		}

		if char.Personality != "" {
			content.WriteString(fmt.Sprintf("   性格: %s\n", char.Personality))
		}

		if char.Background != "" {
			content.WriteString(fmt.Sprintf("   背景: %s\n", char.Background))
		}

		if char.SpeechStyle != "" {
			content.WriteString(fmt.Sprintf("   说话风格: %s\n", char.SpeechStyle))
		}

		if char.Novel != "" {
			content.WriteString(fmt.Sprintf("   出处: %s\n", char.Novel))
		}

		if len(char.Knowledge) > 0 {
			content.WriteString(fmt.Sprintf("   知识领域: %s\n", strings.Join(char.Knowledge, ", ")))
		}

		if len(char.Relationships) > 0 {
			content.WriteString("   人际关系:\n")
			for otherID, relationship := range char.Relationships {
				// 找到对应角色名称
				var otherName string
				for _, otherChar := range sceneData.Characters {
					if otherChar.ID == otherID {
						otherName = otherChar.Name
						break
					}
				}
				if otherName != "" {
					content.WriteString(fmt.Sprintf("     - 与%s: %s\n", otherName, relationship))
				} else {
					content.WriteString(fmt.Sprintf("     - 与%s: %s\n", otherID, relationship))
				}
			}
		}

		content.WriteString(fmt.Sprintf("   创建时间: %s\n", char.CreatedAt.Format("2006-01-02 15:04")))
		if !char.LastUpdated.IsZero() && !char.LastUpdated.Equal(char.CreatedAt) {
			content.WriteString(fmt.Sprintf("   最后更新: %s\n", char.LastUpdated.Format("2006-01-02 15:04")))
		}

		content.WriteString("\n")
	}

	// ✅ 利用 stats 添加详细统计信息
	content.WriteString("交互统计摘要\n")
	content.WriteString(strings.Repeat("-", 30) + "\n")

	// 基础统计
	content.WriteString("基础数据:\n")
	content.WriteString(fmt.Sprintf("  总交互次数: %d\n", stats.TotalInteractions))
	content.WriteString(fmt.Sprintf("  总消息数量: %d\n", stats.TotalMessages))
	content.WriteString(fmt.Sprintf("  参与角色数: %d\n", stats.CharacterCount))

	// 时间范围统计
	if !stats.DateRange.StartDate.IsZero() && !stats.DateRange.EndDate.IsZero() {
		duration := stats.DateRange.EndDate.Sub(stats.DateRange.StartDate)
		content.WriteString(fmt.Sprintf("  交互时间范围: %s 至 %s\n",
			stats.DateRange.StartDate.Format("2006-01-02 15:04:05"),
			stats.DateRange.EndDate.Format("2006-01-02 15:04:05")))
		content.WriteString(fmt.Sprintf("  总交互时长: %.1f 分钟\n", duration.Minutes()))

		if stats.TotalInteractions > 0 {
			avgDuration := duration.Minutes() / float64(stats.TotalInteractions)
			content.WriteString(fmt.Sprintf("  平均交互时长: %.1f 分钟\n", avgDuration))
		}

		if stats.TotalMessages > 1 {
			avgInterval := duration.Seconds() / float64(stats.TotalMessages-1)
			content.WriteString(fmt.Sprintf("  平均消息间隔: %.1f 秒\n", avgInterval))
		}
	}

	// 效率分析
	if stats.TotalInteractions > 0 {
		avgMessagesPerInteraction := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
		content.WriteString(fmt.Sprintf("  平均每次交互消息数: %.1f\n", avgMessagesPerInteraction))

		// 效率评级
		var efficiencyLevel string
		switch {
		case avgMessagesPerInteraction >= 8:
			efficiencyLevel = "极高"
		case avgMessagesPerInteraction >= 5:
			efficiencyLevel = "高"
		case avgMessagesPerInteraction >= 3:
			efficiencyLevel = "中等"
		case avgMessagesPerInteraction >= 1.5:
			efficiencyLevel = "较低"
		default:
			efficiencyLevel = "低"
		}
		content.WriteString(fmt.Sprintf("  交互效率评级: %s\n", efficiencyLevel))
	}

	content.WriteString("\n")

	// ✅ 利用 stats.EmotionDistribution 添加情绪分析
	if len(stats.EmotionDistribution) > 0 {
		content.WriteString("情绪分布分析:\n")

		// 按频次排序情绪
		type emotionStat struct {
			emotion string
			count   int
		}

		var emotionStats []emotionStat
		totalEmotionCount := 0
		for emotion, count := range stats.EmotionDistribution {
			emotionStats = append(emotionStats, emotionStat{emotion, count})
			totalEmotionCount += count
		}

		sort.Slice(emotionStats, func(i, j int) bool {
			return emotionStats[i].count > emotionStats[j].count
		})

		// 显示情绪分布
		for i, stat := range emotionStats {
			if i >= 10 { // 限制显示前10个情绪
				break
			}
			percentage := float64(stat.count) / float64(totalEmotionCount) * 100

			// 创建简单的文本条形图
			barLength := int(percentage / 5) // 每5%一个字符
			if barLength > 20 {
				barLength = 20
			} // 最大20个字符
			bar := strings.Repeat("█", barLength) + strings.Repeat("░", 20-barLength)

			content.WriteString(fmt.Sprintf("  %s: %d次 (%.1f%%) %s\n",
				stat.emotion, stat.count, percentage, bar))
		}

		// 情绪分析洞察
		if len(emotionStats) > 0 {
			dominantEmotion := emotionStats[0]
			content.WriteString("\n情绪分析洞察:\n")
			content.WriteString(fmt.Sprintf("  主导情绪: %s (%.1f%%, %d次)\n",
				dominantEmotion.emotion,
				float64(dominantEmotion.count)/float64(totalEmotionCount)*100,
				dominantEmotion.count))

			emotionDiversity := len(emotionStats)
			var diversityLevel string
			switch {
			case emotionDiversity >= 8:
				diversityLevel = "极高"
			case emotionDiversity >= 6:
				diversityLevel = "高"
			case emotionDiversity >= 4:
				diversityLevel = "中等"
			case emotionDiversity >= 2:
				diversityLevel = "较低"
			default:
				diversityLevel = "低"
			}
			content.WriteString(fmt.Sprintf("  情绪多样性: %d种不同情绪 (%s)\n", emotionDiversity, diversityLevel))
		}

		content.WriteString("\n")
	}

	// ✅ 利用 stats.TopKeywords 添加关键词分析
	if len(stats.TopKeywords) > 0 {
		content.WriteString("热门话题关键词:\n")
		content.WriteString("以下是对话中出现频率最高的关键词:\n")

		for i, keyword := range stats.TopKeywords {
			if i >= 15 { // 最多显示15个关键词
				break
			}

			// 使用简单的排名标记
			var marker string
			switch {
			case i < 3:
				marker = fmt.Sprintf("[第%d名]", i+1)
			case i < 10:
				marker = fmt.Sprintf("  %d. ", i+1)
			default:
				marker = fmt.Sprintf(" %d. ", i+1)
			}

			content.WriteString(fmt.Sprintf("%s %s\n", marker, keyword))
		}

		// 关键词分析洞察
		content.WriteString("\n关键词分析洞察:\n")
		content.WriteString(fmt.Sprintf("  关键词总数: %d个\n", len(stats.TopKeywords)))

		// 简单的主题分类
		emotionWords := []string{"喜欢", "讨厌", "开心", "难过", "愤怒", "恐惧", "love", "hate", "happy", "sad", "angry", "fear"}
		actionWords := []string{"做", "去", "来", "看", "听", "说", "走", "跑", "do", "go", "come", "see", "hear", "say", "walk", "run"}
		objectWords := []string{"东西", "物品", "书", "食物", "水", "房子", "thing", "item", "book", "food", "water", "house"}

		emotionCount := 0
		actionCount := 0
		objectCount := 0
		descriptiveCount := 0

		for _, keyword := range stats.TopKeywords {
			keywordLower := strings.ToLower(keyword)
			categorized := false

			for _, emotion := range emotionWords {
				if strings.Contains(keywordLower, emotion) {
					emotionCount++
					categorized = true
					break
				}
			}

			if !categorized {
				for _, action := range actionWords {
					if strings.Contains(keywordLower, action) {
						actionCount++
						categorized = true
						break
					}
				}
			}

			if !categorized {
				for _, object := range objectWords {
					if strings.Contains(keywordLower, object) {
						objectCount++
						categorized = true
						break
					}
				}
			}

			if !categorized {
				descriptiveCount++
			}
		}

		if emotionCount > 0 {
			content.WriteString(fmt.Sprintf("  情感相关词汇: %d个\n", emotionCount))
		}
		if actionCount > 0 {
			content.WriteString(fmt.Sprintf("  行动相关词汇: %d个\n", actionCount))
		}
		if objectCount > 0 {
			content.WriteString(fmt.Sprintf("  对象相关词汇: %d个\n", objectCount))
		}
		if descriptiveCount > 0 {
			content.WriteString(fmt.Sprintf("  描述相关词汇: %d个\n", descriptiveCount))
		}

		content.WriteString("\n")
	}

	// 移除Markdown格式符号的摘要内容
	content.WriteString("详细分析报告\n")
	content.WriteString(strings.Repeat("-", 30) + "\n")

	textSummary := strings.ReplaceAll(summary, "#", "")
	textSummary = strings.ReplaceAll(textSummary, "**", "")
	textSummary = strings.ReplaceAll(textSummary, "*", "")
	textSummary = strings.ReplaceAll(textSummary, "> ", "  ")

	content.WriteString(textSummary)
	content.WriteString("\n")

	// ✅ 利用 conversations 和 sceneData 添加详细对话记录
	content.WriteString(strings.Repeat("=", 50) + "\n")
	content.WriteString("完整对话记录\n")
	content.WriteString(strings.Repeat("=", 50) + "\n\n")

	// 按交互分组
	interactionGroups := s.groupConversationsByInteraction(conversations)

	for i, group := range interactionGroups {
		content.WriteString(fmt.Sprintf("交互会话 #%d\n", i+1))
		content.WriteString(strings.Repeat("-", 20) + "\n")

		if len(group) > 0 && !group[0].Timestamp.IsZero() {
			content.WriteString(fmt.Sprintf("开始时间: %s\n", group[0].Timestamp.Format("2006-01-02 15:04:05")))

			if len(group) > 1 {
				lastTime := group[len(group)-1].Timestamp
				duration := lastTime.Sub(group[0].Timestamp)
				content.WriteString(fmt.Sprintf("持续时间: %.1f 分钟\n", duration.Minutes()))
			}

			content.WriteString(fmt.Sprintf("消息数量: %d 条\n", len(group)))
			content.WriteString("\n")
		}

		// 显示对话内容
		for j, conv := range group {
			// 尝试从角色列表中找到角色名称
			speakerName := conv.Speaker
			if conv.SpeakerID != "" && conv.SpeakerID != "user" {
				for _, char := range sceneData.Characters {
					if char.ID == conv.SpeakerID {
						speakerName = char.Name
						break
					}
				}
			}

			// 添加消息序号和时间戳
			if !conv.Timestamp.IsZero() {
				content.WriteString(fmt.Sprintf("[%d] %s (%s):\n",
					j+1, speakerName, conv.Timestamp.Format("15:04:05")))
			} else {
				content.WriteString(fmt.Sprintf("[%d] %s:\n", j+1, speakerName))
			}

			// 消息内容（缩进显示）
			lines := strings.Split(conv.Content, "\n")
			for _, line := range lines {
				content.WriteString(fmt.Sprintf("    %s\n", line))
			}

			// 添加情绪信息
			if len(conv.Emotions) > 0 {
				content.WriteString(fmt.Sprintf("    [情绪: %s]\n", strings.Join(conv.Emotions, ", ")))
			}

			// 添加元数据信息（如果有重要信息）
			if conv.Metadata != nil {
				if messageType, exists := conv.Metadata["message_type"]; exists {
					content.WriteString(fmt.Sprintf("    [类型: %v]\n", messageType))
				}
			}

			content.WriteString("\n")
		}

		content.WriteString(strings.Repeat("-", 20) + "\n\n")
	}

	// ✅ 利用 sceneData 和 stats 添加总结洞察
	content.WriteString("总结与洞察\n")
	content.WriteString(strings.Repeat("-", 30) + "\n")

	// 交互效率分析
	if stats.TotalInteractions > 0 && stats.TotalMessages > 0 {
		efficiency := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
		content.WriteString(fmt.Sprintf("交互效率: %.1f 消息/交互\n", efficiency))

		var efficiencyDesc string
		switch {
		case efficiency >= 8:
			efficiencyDesc = "极高 - 深度对话，内容丰富"
		case efficiency >= 5:
			efficiencyDesc = "高 - 有效互动，内容充实"
		case efficiency >= 3:
			efficiencyDesc = "中等 - 正常交流水平"
		case efficiency >= 1.5:
			efficiencyDesc = "较低 - 互动较为简短"
		default:
			efficiencyDesc = "低 - 需要提升互动深度"
		}
		content.WriteString(fmt.Sprintf("效率评级: %s\n", efficiencyDesc))
	}

	// 角色参与度分析
	characterParticipation := make(map[string]int)
	for _, conv := range conversations {
		if conv.SpeakerID != "" && conv.SpeakerID != "user" {
			characterParticipation[conv.SpeakerID]++
		}
	}

	if len(characterParticipation) > 0 {
		content.WriteString("\n角色参与度:\n")

		type charStat struct {
			id    string
			name  string
			count int
		}

		var charStats []charStat
		totalParticipation := 0

		for charID, count := range characterParticipation {
			// 找到角色名称
			var charName string
			for _, char := range sceneData.Characters {
				if char.ID == charID {
					charName = char.Name
					break
				}
			}
			if charName == "" {
				charName = charID
			}

			charStats = append(charStats, charStat{charID, charName, count})
			totalParticipation += count
		}

		// 按参与度排序
		sort.Slice(charStats, func(i, j int) bool {
			return charStats[i].count > charStats[j].count
		})

		for _, stat := range charStats {
			percentage := float64(stat.count) / float64(totalParticipation) * 100
			content.WriteString(fmt.Sprintf("  %s: %d次发言 (%.1f%%)\n",
				stat.name, stat.count, percentage))
		}
	}

	// 场景要素利用度分析
	content.WriteString("\n场景要素利用度:\n")

	// 分析地点提及情况
	if len(sceneData.Scene.Locations) > 0 {
		locationMentions := make(map[string]int)
		allText := ""
		for _, conv := range conversations {
			allText += strings.ToLower(conv.Content) + " "
		}

		mentionedCount := 0
		for _, location := range sceneData.Scene.Locations {
			locationName := strings.ToLower(location.Name)
			count := strings.Count(allText, locationName)
			if count > 0 {
				locationMentions[location.Name] = count
				mentionedCount++
			}
		}

		if len(locationMentions) > 0 {
			content.WriteString("  地点利用情况:\n")
			for locationName, count := range locationMentions {
				content.WriteString(fmt.Sprintf("    %s: 提及 %d 次\n", locationName, count))
			}

			utilizationRate := float64(mentionedCount) / float64(len(sceneData.Scene.Locations)) * 100
			content.WriteString(fmt.Sprintf("  地点利用率: %.1f%% (%d/%d)\n",
				utilizationRate, mentionedCount, len(sceneData.Scene.Locations)))
		} else {
			content.WriteString("  地点利用情况: 无明确地点提及\n")
		}
	}

	// 分析道具提及情况
	if len(sceneData.Scene.Items) > 0 {
		itemMentions := make(map[string]int)
		allText := ""
		for _, conv := range conversations {
			allText += strings.ToLower(conv.Content) + " "
		}

		mentionedCount := 0
		for _, item := range sceneData.Scene.Items {
			itemName := strings.ToLower(item.Name)
			count := strings.Count(allText, itemName)
			if count > 0 {
				itemMentions[item.Name] = count
				mentionedCount++
			}
		}

		if len(itemMentions) > 0 {
			content.WriteString("  道具利用情况:\n")
			for itemName, count := range itemMentions {
				content.WriteString(fmt.Sprintf("    %s: 提及 %d 次\n", itemName, count))
			}

			utilizationRate := float64(mentionedCount) / float64(len(sceneData.Scene.Items)) * 100
			content.WriteString(fmt.Sprintf("  道具利用率: %.1f%% (%d/%d)\n",
				utilizationRate, mentionedCount, len(sceneData.Scene.Items)))
		} else {
			content.WriteString("  道具利用情况: 无明确道具提及\n")
		}
	}

	// 添加导出信息
	content.WriteString("\n" + strings.Repeat("=", 50) + "\n")
	content.WriteString("导出信息\n")
	content.WriteString(strings.Repeat("=", 50) + "\n")
	content.WriteString(fmt.Sprintf("导出时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString("导出格式: 纯文本\n")
	content.WriteString("数据来源: SceneIntruderMCP 交互聚合服务\n")
	content.WriteString("版本: v1.0\n")

	// ✅ 利用 sceneData 添加场景元数据
	content.WriteString(fmt.Sprintf("场景数据版本: %s\n", sceneData.Scene.LastUpdated.Format("2006-01-02")))
	content.WriteString(fmt.Sprintf("统计数据包含: %d 条对话，%d 次交互\n",
		stats.TotalMessages, stats.TotalInteractions))
	content.WriteString(fmt.Sprintf("文档长度: %d 字符\n", len(content.String())))

	return content.String(), nil
}

// formatAsHTML HTML格式导出
func (s *ExportService) formatAsHTML(
	sceneData *SceneData,
	conversations []models.Conversation,
	summary string,
	stats *models.InteractionExportStats) (string, error) {

	// ✅ 添加空值检查
	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	if stats == nil {
		return "", fmt.Errorf("统计数据不能为空")
	}

	var content strings.Builder

	// HTML头部
	content.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>`)
	content.WriteString(sceneData.Scene.Title + " - 交互摘要")
	content.WriteString(`</title>
    <style>
        body { 
            font-family: 'Microsoft YaHei', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Arial, sans-serif; 
            margin: 0; 
            padding: 20px; 
            line-height: 1.6; 
            color: #333;
            background-color: #f8f9fa;
        }
        .container { max-width: 1200px; margin: 0 auto; background: white; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { 
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); 
            color: white; 
            padding: 30px; 
            border-radius: 8px 8px 0 0; 
            text-align: center;
        }
        .header h1 { margin: 0; font-size: 2.5em; font-weight: 300; }
        .header .subtitle { margin: 10px 0 0 0; opacity: 0.9; font-size: 1.1em; }
        
        .content { padding: 30px; }
        .section { margin-bottom: 40px; }
        .section h2 { 
            color: #2c3e50; 
            border-bottom: 3px solid #3498db; 
            padding-bottom: 10px; 
            margin-bottom: 20px;
            font-size: 1.8em;
        }
        .section h3 { 
            color: #34495e; 
            margin-top: 30px; 
            margin-bottom: 15px;
            font-size: 1.4em;
        }
        
        .stats-grid { 
            display: grid; 
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); 
            gap: 20px; 
            margin: 20px 0; 
        }
        .stat-card { 
            background: #f8f9fa; 
            padding: 20px; 
            border-radius: 8px; 
            text-align: center; 
            border: 1px solid #e9ecef;
            transition: transform 0.2s ease;
        }
        .stat-card:hover { transform: translateY(-2px); box-shadow: 0 4px 15px rgba(0,0,0,0.1); }
        .stat-number { font-size: 2.5em; font-weight: bold; color: #3498db; margin: 0; }
        .stat-label { color: #6c757d; margin: 5px 0 0 0; font-size: 0.9em; }
        
        .conversation { 
            background: #fff; 
            padding: 20px; 
            margin: 15px 0; 
            border-radius: 8px; 
            border-left: 4px solid #3498db;
            box-shadow: 0 2px 5px rgba(0,0,0,0.05);
        }
        .conversation.user { border-left-color: #e74c3c; }
        .conversation.character { border-left-color: #27ae60; }
        
        .speaker { 
            font-weight: bold; 
            color: #2c3e50; 
            font-size: 1.1em;
            margin-bottom: 5px;
        }
        .speaker.user { color: #e74c3c; }
        .speaker.character { color: #27ae60; }
        
        .timestamp { 
            color: #7f8c8d; 
            font-size: 0.85em; 
            margin-bottom: 10px;
            font-family: 'Courier New', monospace;
        }
        .message-content { 
            margin: 10px 0; 
            font-size: 1.05em;
            line-height: 1.6;
        }
        .emotion { 
            color: #e67e22; 
            font-style: italic; 
            background: #fef9e7;
            padding: 5px 10px;
            border-radius: 15px;
            font-size: 0.9em;
            display: inline-block;
            margin-top: 10px;
        }
        
        .emotion-chart { 
            display: grid; 
            gap: 10px; 
            margin: 20px 0; 
        }
        .emotion-bar { 
            display: flex; 
            align-items: center; 
            background: #f8f9fa; 
            border-radius: 5px; 
            overflow: hidden;
        }
        .emotion-label { 
            min-width: 100px; 
            padding: 10px; 
            font-weight: bold; 
            background: #e9ecef;
        }
        .emotion-fill { 
            height: 30px; 
            background: linear-gradient(90deg, #3498db, #2980b9); 
            display: flex; 
            align-items: center; 
            padding: 0 10px; 
            color: white; 
            font-size: 0.9em;
        }
        
        .keyword-cloud { 
            display: flex; 
            flex-wrap: wrap; 
            gap: 10px; 
            margin: 20px 0; 
        }
        .keyword { 
            background: #3498db; 
            color: white; 
            padding: 8px 15px; 
            border-radius: 20px; 
            font-size: 0.9em;
            transition: background 0.2s ease;
        }
        .keyword:hover { background: #2980b9; }
        .keyword.top-3 { background: #e74c3c; }
        .keyword.top-5 { background: #f39c12; }
        
        .scene-info { 
            background: #f8f9fa; 
            padding: 20px; 
            border-radius: 8px; 
            margin: 20px 0; 
        }
        .character-grid { 
            display: grid; 
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); 
            gap: 20px; 
            margin: 20px 0; 
        }
        .character-card { 
            background: #fff; 
            padding: 20px; 
            border-radius: 8px; 
            border: 1px solid #e9ecef;
            box-shadow: 0 2px 5px rgba(0,0,0,0.05);
        }
        .character-name { 
            font-size: 1.3em; 
            font-weight: bold; 
            color: #2c3e50; 
            margin-bottom: 10px;
        }
        
        .progress-bar { 
            background: #e9ecef; 
            height: 20px; 
            border-radius: 10px; 
            overflow: hidden; 
            margin: 10px 0;
        }
        .progress-fill { 
            height: 100%; 
            background: linear-gradient(90deg, #27ae60, #2ecc71); 
            transition: width 0.3s ease;
        }
        
        .export-info { 
            background: #e8f4fd; 
            padding: 20px; 
            border-radius: 8px; 
            margin: 30px 0; 
            border: 1px solid #bee5eb;
        }
        
        @media (max-width: 768px) {
            .stats-grid { grid-template-columns: repeat(2, 1fr); }
            .character-grid { grid-template-columns: 1fr; }
            .container { margin: 10px; }
            .content { padding: 20px; }
        }
        
        @media print {
            body { background: white; }
            .container { box-shadow: none; }
            .conversation { break-inside: avoid; }
        }
    </style>
</head>
<body>
    <div class="container">`)

	// ✅ 利用 sceneData 和 stats 的详细头部信息
	content.WriteString(`<div class="header">
        <h1>`)
	content.WriteString(sceneData.Scene.Title)
	content.WriteString(`</h1>
        <div class="subtitle">完整交互分析报告</div>
        <div class="subtitle">`)

	// 基础信息
	content.WriteString(`
    <div class="section">
        <h2>📋 基础信息</h2>
        <p><strong>场景ID:</strong> `)
	content.WriteString(sceneData.Scene.ID)
	content.WriteString(`</p>
        <p><strong>场景名称:</strong> `)
	content.WriteString(sceneData.Scene.Title)
	content.WriteString(`</p>
        <p><strong>场景描述:</strong> `)
	content.WriteString(sceneData.Scene.Description)
	content.WriteString(`</p>
    </div>`)

	// 统计信息

	content.WriteString(`
    <div class="section">
        <h2>📊 统计数据</h2>
        <div class="stats">
            <div class="stat-card">
                <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", stats.TotalMessages))
	content.WriteString(`</div>
                <div class="stat-label">总消息数</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", stats.CharacterCount))
	content.WriteString(`</div>
                <div class="stat-label">参与角色数</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", stats.TotalInteractions))
	content.WriteString(`</div>
                <div class="stat-label">交互次数</div>
            </div>
        </div>
    </div>`)

	// 摘要内容
	content.WriteString(`
    <div class="section">
        <h2>📝 详细分析报告</h2>`)

	// 简单的Markdown到HTML转换
	htmlSummary := strings.ReplaceAll(summary, "### ", "<h3>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "## ", "<h2>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "**", "<strong>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "*", "</strong>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "\n- ", "<li>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "\n\n", "</p><p>")
	htmlSummary = "<p>" + htmlSummary + "</p>"
	content.WriteString(htmlSummary)

	// ✅ 利用 stats 显示基础统计
	if !stats.DateRange.StartDate.IsZero() {
		content.WriteString(fmt.Sprintf("时间范围: %s 至 %s",
			stats.DateRange.StartDate.Format("2006-01-02"),
			stats.DateRange.EndDate.Format("2006-01-02")))
	}
	content.WriteString(`</div>
    </div>

    <div class="content">`)

	// ✅ 利用 stats 添加概览统计卡片
	content.WriteString(`<div class="section">
        <h2>📊 数据概览</h2>
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", stats.TotalInteractions))
	content.WriteString(`</div>
                <div class="stat-label">总交互次数</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", stats.TotalMessages))
	content.WriteString(`</div>
                <div class="stat-label">总消息数量</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", stats.CharacterCount))
	content.WriteString(`</div>
                <div class="stat-label">参与角色数</div>
            </div>`)

	// ✅ 计算并显示平均效率
	if stats.TotalInteractions > 0 {
		avgMessages := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
		content.WriteString(`<div class="stat-card">
                <div class="stat-number">`)
		content.WriteString(fmt.Sprintf("%.1f", avgMessages))
		content.WriteString(`</div>
                <div class="stat-label">平均消息/交互</div>
            </div>`)
	}

	// ✅ 显示时间跨度
	if !stats.DateRange.StartDate.IsZero() && !stats.DateRange.EndDate.IsZero() {
		duration := stats.DateRange.EndDate.Sub(stats.DateRange.StartDate)
		content.WriteString(`<div class="stat-card">
                <div class="stat-number">`)
		content.WriteString(fmt.Sprintf("%.0f", duration.Hours()))
		content.WriteString(`</div>
                <div class="stat-label">总时长 (小时)</div>
            </div>`)
	}

	content.WriteString(`</div>
    </div>`)

	// ✅ 利用 sceneData 添加场景信息
	content.WriteString(`<div class="section">
        <h2>🎬 场景信息</h2>
        <div class="scene-info">
            <p><strong>场景ID:</strong> `)
	content.WriteString(sceneData.Scene.ID)
	content.WriteString(`</p>
            <p><strong>描述:</strong> `)
	content.WriteString(sceneData.Scene.Description)
	content.WriteString(`</p>`)

	if sceneData.Scene.Source != "" {
		content.WriteString(`<p><strong>数据来源:</strong> `)
		content.WriteString(sceneData.Scene.Source)
		content.WriteString(`</p>`)
	}

	if len(sceneData.Scene.Themes) > 0 {
		content.WriteString(`<p><strong>主题:</strong> `)
		content.WriteString(strings.Join(sceneData.Scene.Themes, ", "))
		content.WriteString(`</p>`)
	}

	if sceneData.Scene.Era != "" {
		content.WriteString(`<p><strong>时代背景:</strong> `)
		content.WriteString(sceneData.Scene.Era)
		content.WriteString(`</p>`)
	}

	content.WriteString(`</div>
    </div>`)

	// ✅ 利用 sceneData.Characters 添加角色信息卡片
	if len(sceneData.Characters) > 0 {
		content.WriteString(`<div class="section">
            <h2>👥 参与角色</h2>
            <div class="character-grid">`)

		for _, char := range sceneData.Characters {
			content.WriteString(`<div class="character-card">
                <div class="character-name">`)
			content.WriteString(char.Name)
			content.WriteString(`</div>
                <p><strong>描述:</strong> `)
			content.WriteString(char.Description)
			content.WriteString(`</p>`)

			if char.Role != "" {
				content.WriteString(`<p><strong>角色:</strong> `)
				content.WriteString(char.Role)
				content.WriteString(`</p>`)
			}

			if char.Personality != "" {
				content.WriteString(`<p><strong>性格:</strong> `)
				content.WriteString(char.Personality)
				content.WriteString(`</p>`)
			}

			if len(char.Knowledge) > 0 {
				content.WriteString(`<p><strong>知识领域:</strong> `)
				content.WriteString(strings.Join(char.Knowledge, ", "))
				content.WriteString(`</p>`)
			}

			content.WriteString(`</div>`)
		}

		content.WriteString(`</div>
        </div>`)
	}

	// ✅ 利用 stats.EmotionDistribution 添加交互式情绪分析
	if len(stats.EmotionDistribution) > 0 {
		content.WriteString(`<div class="section">
            <h2>🎭 情绪分布分析</h2>`)

		// 计算总情绪数和排序
		type emotionData struct {
			emotion string
			count   int
			percent float64
		}

		var emotions []emotionData
		totalEmotions := 0
		for _, count := range stats.EmotionDistribution {
			totalEmotions += count
		}

		for emotion, count := range stats.EmotionDistribution {
			percent := float64(count) / float64(totalEmotions) * 100
			emotions = append(emotions, emotionData{emotion, count, percent})
		}

		// 按数量排序
		sort.Slice(emotions, func(i, j int) bool {
			return emotions[i].count > emotions[j].count
		})

		content.WriteString(`<div class="emotion-chart">`)
		for _, emotion := range emotions {
			content.WriteString(`<div class="emotion-bar">
                <div class="emotion-label">`)
			content.WriteString(emotion.emotion)
			content.WriteString(`</div>
                <div class="emotion-fill" style="width: `)
			content.WriteString(fmt.Sprintf("%.1f%%", emotion.percent))
			content.WriteString(`;">`)
			content.WriteString(fmt.Sprintf("%d次 (%.1f%%)", emotion.count, emotion.percent))
			content.WriteString(`</div>
            </div>`)
		}
		content.WriteString(`</div>`)

		// 情绪分析洞察
		if len(emotions) > 0 {
			dominant := emotions[0]
			content.WriteString(`<div class="scene-info">
                <h3>情绪分析洞察</h3>
                <p><strong>主导情绪:</strong> `)
			content.WriteString(fmt.Sprintf("%s (%.1f%%, %d次)", dominant.emotion, dominant.percent, dominant.count))
			content.WriteString(`</p>
                <p><strong>情绪多样性:</strong> `)

			diversityLevel := ""
			switch {
			case len(emotions) >= 8:
				diversityLevel = "极高 - 情绪表达非常丰富"
			case len(emotions) >= 6:
				diversityLevel = "高 - 情绪变化多样"
			case len(emotions) >= 4:
				diversityLevel = "中等 - 有一定情绪变化"
			case len(emotions) >= 2:
				diversityLevel = "较低 - 情绪相对单一"
			default:
				diversityLevel = "低 - 情绪表达单调"
			}

			content.WriteString(fmt.Sprintf("%d种不同情绪 (%s)", len(emotions), diversityLevel))
			content.WriteString(`</p>
            </div>`)
		}

		content.WriteString(`</div>`)
	}

	// ✅ 利用 stats.TopKeywords 添加关键词云
	if len(stats.TopKeywords) > 0 {
		content.WriteString(`<div class="section">
            <h2>🔤 热门话题关键词</h2>
            <div class="keyword-cloud">`)

		for i, keyword := range stats.TopKeywords {
			if i >= 20 {
				break
			} // 限制显示数量

			var cssClass string
			if i < 3 {
				cssClass = "keyword top-3" // 前三名红色
			} else if i < 5 {
				cssClass = "keyword top-5" // 前五名橙色
			} else {
				cssClass = "keyword" // 其他蓝色
			}

			content.WriteString(fmt.Sprintf(`<span class="%s" title="排名第%d">%s</span>`,
				cssClass, i+1, keyword))
		}

		content.WriteString(`</div>
            <div class="scene-info">
                <p><strong>关键词总数:</strong> `)
		content.WriteString(fmt.Sprintf("%d个", len(stats.TopKeywords)))
		content.WriteString(`</p>
                <p><strong>说明:</strong> 基于对话内容自动提取的高频词汇，反映了交互的主要话题方向。</p>
            </div>
        </div>`)
	}

	// 将Markdown摘要转换为HTML
	content.WriteString(`<div class="section">
        <h2>📝 详细分析报告</h2>`)

	// 简单的Markdown到HTML转换
	htmlSummary = strings.ReplaceAll(summary, "### ", "<h3>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "## ", "<h2>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "**", "<strong>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "*", "</strong>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "\n- ", "<li>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "\n\n", "</p><p>")
	htmlSummary = "<p>" + htmlSummary + "</p>"

	content.WriteString(htmlSummary)
	content.WriteString(`</div>`)

	// ✅ 利用 conversations 和 sceneData 添加格式化的对话记录
	if len(conversations) > 0 {
		content.WriteString(`<div class="section">
            <h2>💬 完整对话记录</h2>`)

		// 按交互分组
		interactionGroups := s.groupConversationsByInteraction(conversations)

		for groupIndex, group := range interactionGroups {
			content.WriteString(fmt.Sprintf(`<h3>📖 交互会话 #%d</h3>`, groupIndex+1))

			if len(group) > 0 && !group[0].Timestamp.IsZero() {
				content.WriteString(`<div class="scene-info">`)
				content.WriteString(fmt.Sprintf(`<p><strong>开始时间:</strong> %s</p>`,
					group[0].Timestamp.Format("2006-01-02 15:04:05")))

				if len(group) > 1 {
					lastTime := group[len(group)-1].Timestamp
					duration := lastTime.Sub(group[0].Timestamp)
					content.WriteString(fmt.Sprintf(`<p><strong>持续时间:</strong> %.1f 分钟</p>`, duration.Minutes()))
				}

				content.WriteString(fmt.Sprintf(`<p><strong>消息数量:</strong> %d 条</p>`, len(group)))
				content.WriteString(`</div>`)
			}

			// 显示对话内容
			for _, conv := range group {
				// 判断发言者类型
				isUser := (conv.Speaker == "user" || conv.Speaker == "用户")
				if conv.Metadata != nil {
					if speakerType, exists := conv.Metadata["speaker_type"]; exists {
						isUser = (speakerType == "user")
					}
				}

				var conversationClass, speakerClass string
				if isUser {
					conversationClass = "conversation user"
					speakerClass = "speaker user"
				} else {
					conversationClass = "conversation character"
					speakerClass = "speaker character"
				}

				content.WriteString(fmt.Sprintf(`<div class="%s">`, conversationClass))

				// 尝试从角色列表中找到角色名称
				speakerName := conv.Speaker
				if conv.SpeakerID != "" && conv.SpeakerID != "user" {
					for _, char := range sceneData.Characters {
						if char.ID == conv.SpeakerID {
							speakerName = char.Name
							break
						}
					}
				}

				content.WriteString(fmt.Sprintf(`<div class="%s">%s</div>`, speakerClass, speakerName))

				if !conv.Timestamp.IsZero() {
					content.WriteString(fmt.Sprintf(`<div class="timestamp">%s</div>`,
						conv.Timestamp.Format("2006-01-02 15:04:05")))
				}

				content.WriteString(fmt.Sprintf(`<div class="message-content">%s</div>`, conv.Content))

				// 添加情绪信息
				if len(conv.Emotions) > 0 {
					content.WriteString(fmt.Sprintf(`<div class="emotion">🎭 %s</div>`,
						strings.Join(conv.Emotions, ", ")))
				}

				content.WriteString(`</div>`)
			}
		}

		content.WriteString(`</div>`)
	}

	// ✅ 利用 stats 添加导出信息
	content.WriteString(`<div class="export-info">
        <h3>📄 导出信息</h3>
        <p><strong>导出时间:</strong> `)
	content.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	content.WriteString(`</p>
        <p><strong>导出格式:</strong> HTML</p>
        <p><strong>数据来源:</strong> SceneIntruderMCP 交互聚合服务</p>
        <p><strong>版本:</strong> v1.0</p>
        <p><strong>场景数据版本:</strong> `)
	content.WriteString(sceneData.Scene.LastUpdated.Format("2006-01-02"))
	content.WriteString(`</p>
        <p><strong>统计数据包含:</strong> `)
	content.WriteString(fmt.Sprintf("%d 条对话，%d 次交互", stats.TotalMessages, stats.TotalInteractions))
	content.WriteString(`</p>
    </div>`)

	// HTML尾部
	content.WriteString(`
    </div>
</div>

<script>
// 添加一些交互功能
document.addEventListener('DOMContentLoaded', function() {
    // 为统计卡片添加点击动画
    const statCards = document.querySelectorAll('.stat-card');
    statCards.forEach(card => {
        card.addEventListener('click', function() {
            this.style.transform = 'scale(1.05)';
            setTimeout(() => {
                this.style.transform = 'translateY(-2px)';
            }, 200);
        });
    });
    
    // 为关键词添加点击效果
    const keywords = document.querySelectorAll('.keyword');
    keywords.forEach(keyword => {
        keyword.addEventListener('click', function() {
            const text = this.textContent;
            // 可以在这里添加搜索功能或高亮功能
            console.log('点击了关键词:', text);
        });
    });
    
    // 添加对话折叠功能
    const conversationSections = document.querySelectorAll('.section h2');
    conversationSections.forEach(header => {
        if (header.textContent.includes('对话记录')) {
            header.style.cursor = 'pointer';
            header.addEventListener('click', function() {
                const section = this.parentElement;
                const content = section.querySelector('h3, .conversation');
                if (content) {
                    const isHidden = content.style.display === 'none';
                    const allContent = section.querySelectorAll('h3, .conversation, .scene-info');
                    allContent.forEach(el => {
                        el.style.display = isHidden ? 'block' : 'none';
                    });
                    this.textContent = this.textContent.replace(
                        isHidden ? '▶' : '▼', 
                        isHidden ? '▼' : '▶'
                    );
                    if (!this.textContent.includes('▶') && !this.textContent.includes('▼')) {
                        this.textContent = (isHidden ? '▼' : '▶') + ' ' + this.textContent;
                    }
                }
            });
        }
    });
    
    // 添加打印功能
    if (window.location.search.includes('print=true')) {
        window.print();
    }
});
</script>

</body>
</html>`)

	return content.String(), nil
}

// saveExportToDataDir 保存导出文件到data目录
func (s *ExportService) saveExportToDataDir(result *models.ExportResult) (string, int64, error) {
	// 创建导出目录
	exportDir := filepath.Join("data", "exports", "interactions")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return "", 0, fmt.Errorf("创建导出目录失败: %w", err)
	}

	// 生成文件名
	timestamp := result.GeneratedAt.Format("20060102_150405")
	fileName := fmt.Sprintf("%s_interaction_summary_%s.%s",
		result.SceneID, timestamp, result.Format)

	filePath := filepath.Join(exportDir, fileName)

	// 写入文件
	if err := os.WriteFile(filePath, []byte(result.Content), 0644); err != nil {
		return "", 0, fmt.Errorf("写入导出文件失败: %w", err)
	}

	// 获取文件大小
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("获取文件信息失败: %w", err)
	}

	return filePath, fileInfo.Size(), nil
}

// 辅助函数
func (s *ExportService) groupConversationsByInteraction(conversations []models.Conversation) [][]models.Conversation {
	groups := make(map[string][]models.Conversation)

	for _, conv := range conversations {
		interactionID := "default"
		if conv.Metadata != nil {
			if id, exists := conv.Metadata["interaction_id"]; exists {
				interactionID = fmt.Sprintf("%v", id)
			}
		}
		groups[interactionID] = append(groups[interactionID], conv)
	}

	// 转换为切片并排序
	var result [][]models.Conversation
	for _, group := range groups {
		// 按时间排序组内对话
		sort.Slice(group, func(i, j int) bool {
			return group[i].Timestamp.Before(group[j].Timestamp)
		})
		result = append(result, group)
	}

	// 按第一条消息时间排序组
	sort.Slice(result, func(i, j int) bool {
		if len(result[i]) == 0 || len(result[j]) == 0 {
			return false
		}
		return result[i][0].Timestamp.Before(result[j][0].Timestamp)
	})

	return result
}

func isCommonWord(word string) bool {
	commonWords := []string{"的", "了", "在", "和", "与", "为", "是", "有", "到", "将", "被", "从", "对", "把", "给", "一个", "这个", "那个", "什么", "怎么", "为什么"}
	for _, common := range commonWords {
		if word == common {
			return true
		}
	}
	return false
}

func extractTopKeywords(wordCount map[string]int, limit int) []string {
	type wordFreq struct {
		word  string
		count int
	}

	var frequencies []wordFreq
	for word, count := range wordCount {
		frequencies = append(frequencies, wordFreq{word, count})
	}

	sort.Slice(frequencies, func(i, j int) bool {
		return frequencies[i].count > frequencies[j].count
	})

	var keywords []string
	for i, freq := range frequencies {
		if i >= limit {
			break
		}
		keywords = append(keywords, freq.word)
	}

	return keywords
}

// ---------------------------------------------
// ✅ 故事导出方法
// ExportStoryAsDocument 导出故事文档
func (s *ExportService) ExportStoryAsDocument(ctx context.Context, sceneID string, format string) (*models.ExportResult, error) {
	// 1. 验证输入参数
	if sceneID == "" {
		return nil, fmt.Errorf("场景ID不能为空")
	}

	supportedFormats := []string{"json", "markdown", "txt", "html"}
	if !contains(supportedFormats, strings.ToLower(format)) {
		return nil, fmt.Errorf("不支持的导出格式: %s，支持的格式: %v", format, supportedFormats)
	}

	// 2. 获取场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, fmt.Errorf("加载场景失败: %w", err)
	}

	// 3. 获取故事数据
	var storyData *models.StoryData
	if s.StoryService != nil {
		storyData, err = s.StoryService.GetStoryData(sceneID, nil)
		if err != nil {
			return nil, fmt.Errorf("获取故事数据失败: %w", err)
		}
	} else {
		return nil, fmt.Errorf("故事服务不可用")
	}

	// 4. 分析故事统计数据
	storyStats := s.analyzeStoryStatistics(storyData)

	// 5. 生成故事摘要内容
	storySummary := s.generateStorySummary(sceneData, storyData, storyStats)

	// 6. 根据格式生成内容
	content, err := s.formatStoryExportContent(sceneData, storyData, storySummary, storyStats, format)
	if err != nil {
		return nil, fmt.Errorf("格式化故事导出内容失败: %w", err)
	}

	// 7. 创建导出结果
	result := &models.ExportResult{
		SceneID:       sceneID,
		Title:         fmt.Sprintf("%s - 故事文档", sceneData.Scene.Title),
		Format:        format,
		Content:       content,
		ExportType:    "story",
		GeneratedAt:   time.Now(),
		Characters:    sceneData.Characters,
		StoryData:     storyData,
		SceneMetadata: s.buildSceneMetadata(sceneData),
	}

	// 8. 保存到 data 目录
	filePath, fileSize, err := s.saveStoryExportToDataDir(result)
	if err != nil {
		return nil, fmt.Errorf("保存故事导出文件失败: %w", err)
	}

	result.FilePath = filePath
	result.FileSize = fileSize

	return result, nil
}

// analyzeStoryStatistics 分析故事统计数据
func (s *ExportService) analyzeStoryStatistics(storyData *models.StoryData) *models.StoryExportStats {
	stats := &models.StoryExportStats{
		NodesByType:          make(map[string]int),
		TasksByStatus:        make(map[string]int),
		CharacterInvolvement: make(map[string]int),
	}

	if storyData == nil {
		return stats
	}

	// 基础统计
	stats.TotalNodes = len(storyData.Nodes)
	stats.TotalTasks = len(storyData.Tasks)
	stats.TotalChoices = 0
	stats.Progress = storyData.Progress
	stats.CurrentState = storyData.CurrentState

	// 分析节点类型分布
	for _, node := range storyData.Nodes {
		stats.NodesByType[node.Type]++
		stats.TotalChoices += len(node.Choices)

		// 统计已揭示的节点
		if node.IsRevealed {
			stats.RevealedNodes++
		}
	}

	// 分析任务状态
	for _, task := range storyData.Tasks {
		if task.Completed {
			stats.TasksByStatus["completed"]++
			stats.CompletedTasks++
		} else if task.IsRevealed {
			stats.TasksByStatus["active"]++
		} else {
			stats.TasksByStatus["hidden"]++
		}

		// 分析角色参与度
		if task.TriggerCharacterID != "" {
			stats.CharacterInvolvement[task.TriggerCharacterID]++
		}
	}

	// 分析故事分支
	rootNodes := 0
	maxDepth := 0
	for _, node := range storyData.Nodes {
		if node.ParentID == "" {
			rootNodes++
		}
		// 计算节点深度（简化版本）
		depth := s.calculateNodeDepth(node.ID, storyData.Nodes)
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	stats.BranchCount = rootNodes
	stats.MaxDepth = maxDepth

	// 计算故事完整性
	if stats.TotalTasks > 0 {
		stats.CompletionRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
	}

	// 分析选择分布
	selectedChoices := 0
	for _, node := range storyData.Nodes {
		for _, choice := range node.Choices {
			if choice.Selected {
				selectedChoices++
			}
		}
	}
	stats.SelectedChoices = selectedChoices

	return stats
}

// calculateNodeDepth 计算节点深度
func (s *ExportService) calculateNodeDepth(nodeID string, nodes []models.StoryNode) int {
	for _, node := range nodes {
		if node.ID == nodeID {
			if node.ParentID == "" {
				return 0
			}
			return 1 + s.calculateNodeDepth(node.ParentID, nodes)
		}
	}
	return 0
}

// generateStorySummary 生成故事摘要
func (s *ExportService) generateStorySummary(
	sceneData *SceneData,
	storyData *models.StoryData,
	stats *models.StoryExportStats) string {

	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("## %s - 故事文档报告\n\n", sceneData.Scene.Title))

	// 故事基本信息
	summary.WriteString("### 故事概览\n\n")
	summary.WriteString(fmt.Sprintf("- **故事简介**: %s\n", storyData.Intro))
	summary.WriteString(fmt.Sprintf("- **主要目标**: %s\n", storyData.MainObjective))
	summary.WriteString(fmt.Sprintf("- **当前状态**: %s\n", storyData.CurrentState))
	summary.WriteString(fmt.Sprintf("- **故事进度**: %d%%\n", storyData.Progress))
	summary.WriteString(fmt.Sprintf("- **完成度**: %.1f%% (%d/%d 任务完成)\n",
		stats.CompletionRate, stats.CompletedTasks, stats.TotalTasks))
	summary.WriteString("\n")

	// 故事结构分析
	summary.WriteString("### 故事结构分析\n\n")
	summary.WriteString(fmt.Sprintf("- **故事节点总数**: %d 个\n", stats.TotalNodes))
	summary.WriteString(fmt.Sprintf("- **已揭示节点**: %d 个 (%.1f%%)\n",
		stats.RevealedNodes, float64(stats.RevealedNodes)/float64(stats.TotalNodes)*100))
	summary.WriteString(fmt.Sprintf("- **故事分支数**: %d 个\n", stats.BranchCount))
	summary.WriteString(fmt.Sprintf("- **最大深度**: %d 层\n", stats.MaxDepth))
	summary.WriteString(fmt.Sprintf("- **选择总数**: %d 个\n", stats.TotalChoices))
	summary.WriteString(fmt.Sprintf("- **已做选择**: %d 个\n", stats.SelectedChoices))
	summary.WriteString("\n")

	// 节点类型分布
	if len(stats.NodesByType) > 0 {
		summary.WriteString("### 节点类型分布\n\n")
		for nodeType, count := range stats.NodesByType {
			percentage := float64(count) / float64(stats.TotalNodes) * 100
			summary.WriteString(fmt.Sprintf("- **%s**: %d 个 (%.1f%%)\n", nodeType, count, percentage))
		}
		summary.WriteString("\n")
	}

	// 任务完成情况
	summary.WriteString("### 任务完成情况\n\n")
	summary.WriteString(fmt.Sprintf("- **总任务数**: %d 个\n", stats.TotalTasks))
	summary.WriteString(fmt.Sprintf("- **已完成任务**: %d 个\n", stats.CompletedTasks))
	summary.WriteString(fmt.Sprintf("- **进行中任务**: %d 个\n", stats.TasksByStatus["active"]))
	summary.WriteString(fmt.Sprintf("- **隐藏任务**: %d 个\n", stats.TasksByStatus["hidden"]))
	summary.WriteString("\n")

	// 角色参与度
	if len(stats.CharacterInvolvement) > 0 {
		summary.WriteString("### 角色参与度\n\n")
		for charID, involvement := range stats.CharacterInvolvement {
			// 查找角色名称
			var charName string
			for _, char := range sceneData.Characters {
				if char.ID == charID {
					charName = char.Name
					break
				}
			}
			if charName == "" {
				charName = charID
			}
			summary.WriteString(fmt.Sprintf("- **%s**: 参与 %d 个任务\n", charName, involvement))
		}
		summary.WriteString("\n")
	}

	// 故事发展评估
	summary.WriteString("### 故事发展评估\n\n")

	// 进度评估
	var progressLevel string
	switch {
	case storyData.Progress >= 90:
		progressLevel = "接近尾声 - 故事即将完结"
	case storyData.Progress >= 70:
		progressLevel = "后期发展 - 主要冲突趋于解决"
	case storyData.Progress >= 50:
		progressLevel = "中期发展 - 故事冲突加剧"
	case storyData.Progress >= 30:
		progressLevel = "初期发展 - 故事情节逐步展开"
	case storyData.Progress >= 10:
		progressLevel = "开始阶段 - 世界观和角色介绍"
	default:
		progressLevel = "序幕阶段 - 故事刚刚开始"
	}
	summary.WriteString(fmt.Sprintf("- **发展阶段**: %s\n", progressLevel))

	// 复杂度评估
	var complexityLevel string
	avgChoicesPerNode := float64(stats.TotalChoices) / float64(stats.TotalNodes)
	switch {
	case avgChoicesPerNode >= 4:
		complexityLevel = "高 - 选择丰富，分支复杂"
	case avgChoicesPerNode >= 2.5:
		complexityLevel = "中等 - 适度的选择和分支"
	case avgChoicesPerNode >= 1.5:
		complexityLevel = "较低 - 相对线性的发展"
	default:
		complexityLevel = "低 - 线性故事发展"
	}
	summary.WriteString(fmt.Sprintf("- **故事复杂度**: %s (平均 %.1f 选择/节点)\n", complexityLevel, avgChoicesPerNode))

	// 互动性评估
	interactionRate := float64(stats.SelectedChoices) / float64(stats.TotalChoices) * 100
	var interactionLevel string
	switch {
	case interactionRate >= 70:
		interactionLevel = "高 - 玩家积极参与决策"
	case interactionRate >= 50:
		interactionLevel = "中等 - 适度的玩家参与"
	case interactionRate >= 30:
		interactionLevel = "较低 - 玩家参与有限"
	default:
		interactionLevel = "低 - 缺乏玩家互动"
	}
	summary.WriteString(fmt.Sprintf("- **互动性水平**: %s (%.1f%% 选择已做出)\n", interactionLevel, interactionRate))

	return summary.String()
}

// formatStoryExportContent 根据格式生成故事导出内容
func (s *ExportService) formatStoryExportContent(
	sceneData *SceneData,
	storyData *models.StoryData,
	summary string,
	stats *models.StoryExportStats,
	format string) (string, error) {

	switch strings.ToLower(format) {
	case "json":
		return s.formatStoryAsJSON(sceneData, storyData, summary, stats)
	case "markdown":
		return s.formatStoryAsMarkdown(sceneData, storyData, summary, stats)
	case "txt":
		return s.formatStoryAsText(sceneData, storyData, summary, stats)
	case "html":
		return s.formatStoryAsHTML(sceneData, storyData, summary, stats)
	default:
		return "", fmt.Errorf("不支持的格式: %s", format)
	}
}

// formatStoryAsJSON JSON格式导出故事
func (s *ExportService) formatStoryAsJSON(
	sceneData *SceneData,
	storyData *models.StoryData,
	summary string,
	stats *models.StoryExportStats) (string, error) {

	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	if storyData == nil {
		return "", fmt.Errorf("故事数据不能为空")
	}

	exportData := map[string]interface{}{
		"scene_info": map[string]interface{}{
			"id":          sceneData.Scene.ID,
			"title":       sceneData.Scene.Title,
			"description": sceneData.Scene.Description,
			"themes":      sceneData.Scene.Themes,
			"era":         sceneData.Scene.Era,
			"atmosphere":  sceneData.Scene.Atmosphere,
			"created_at":  sceneData.Scene.CreatedAt,
		},
		"story_data": map[string]interface{}{
			"intro":          storyData.Intro,
			"main_objective": storyData.MainObjective,
			"current_state":  storyData.CurrentState,
			"progress":       storyData.Progress,
			"nodes":          storyData.Nodes,
			"tasks":          storyData.Tasks,
			"locations":      storyData.Locations,
		},
		"summary":    summary,
		"statistics": stats,
		"characters": sceneData.Characters,
		"export_info": map[string]interface{}{
			"generated_at": time.Now(),
			"format":       "json",
			"export_type":  "story",
			"version":      "1.0",
		},
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON序列化失败: %w", err)
	}

	return string(jsonData), nil
}

// formatStoryAsMarkdown Markdown格式导出故事
func (s *ExportService) formatStoryAsMarkdown(
	sceneData *SceneData,
	storyData *models.StoryData,
	summary string,
	stats *models.StoryExportStats) (string, error) {

	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	if storyData == nil {
		return "", fmt.Errorf("故事数据不能为空")
	}

	var content strings.Builder

	// 标题和基本信息
	content.WriteString(fmt.Sprintf("# %s - 完整故事文档\n\n", sceneData.Scene.Title))

	// ✅ 使用 stats 添加统计概览
	content.WriteString("## 📊 故事统计概览\n\n")
	content.WriteString(fmt.Sprintf("- **故事进度**: %d%%\n", stats.Progress))
	content.WriteString(fmt.Sprintf("- **当前状态**: %s\n", stats.CurrentState))
	content.WriteString(fmt.Sprintf("- **总节点数**: %d 个\n", stats.TotalNodes))
	content.WriteString(fmt.Sprintf("- **已揭示节点**: %d 个\n", stats.RevealedNodes))
	content.WriteString(fmt.Sprintf("- **总任务数**: %d 个\n", stats.TotalTasks))
	content.WriteString(fmt.Sprintf("- **已完成任务**: %d 个\n", stats.CompletedTasks))
	content.WriteString(fmt.Sprintf("- **完成率**: %.1f%%\n", stats.CompletionRate))
	content.WriteString(fmt.Sprintf("- **故事分支数**: %d 个\n", stats.BranchCount))
	content.WriteString(fmt.Sprintf("- **最大深度**: %d 层\n", stats.MaxDepth))
	content.WriteString(fmt.Sprintf("- **总选择数**: %d 个\n", stats.TotalChoices))
	content.WriteString(fmt.Sprintf("- **已做选择**: %d 个\n", stats.SelectedChoices))
	content.WriteString("\n")

	// ✅ 使用 stats 添加节点类型分布
	if len(stats.NodesByType) > 0 {
		content.WriteString("### 节点类型分布\n\n")
		content.WriteString("| 节点类型 | 数量 | 占比 |\n")
		content.WriteString("|---------|------|------|\n")

		for nodeType, count := range stats.NodesByType {
			percentage := float64(count) / float64(stats.TotalNodes) * 100
			content.WriteString(fmt.Sprintf("| %s | %d | %.1f%% |\n", nodeType, count, percentage))
		}
		content.WriteString("\n")
	}
	// ✅ 使用 stats 添加任务状态分布
	if len(stats.TasksByStatus) > 0 {
		content.WriteString("### 任务状态分布\n\n")
		content.WriteString("| 任务状态 | 数量 |\n")
		content.WriteString("|---------|------|\n")

		for status, count := range stats.TasksByStatus {
			statusName := status
			switch status {
			case "completed":
				statusName = "已完成"
			case "active":
				statusName = "进行中"
			case "hidden":
				statusName = "隐藏"
			}
			content.WriteString(fmt.Sprintf("| %s | %d |\n", statusName, count))
		}
		content.WriteString("\n")
	}
	// ✅ 使用 stats 添加角色参与度
	if len(stats.CharacterInvolvement) > 0 {
		content.WriteString("### 角色参与度\n\n")
		content.WriteString("| 角色 | 参与任务数 |\n")
		content.WriteString("|------|----------|\n")

		for charID, involvement := range stats.CharacterInvolvement {
			// 查找角色名称
			var charName string
			for _, char := range sceneData.Characters {
				if char.ID == charID {
					charName = char.Name
					break
				}
			}
			if charName == "" {
				charName = charID
			}
			content.WriteString(fmt.Sprintf("| %s | %d |\n", charName, involvement))
		}
		content.WriteString("\n")
	}

	// 故事摘要
	content.WriteString(summary)
	content.WriteString("\n")

	// 故事节点详情
	content.WriteString("## 📖 故事节点详情\n\n")

	// 按类型组织节点
	nodesByType := make(map[string][]models.StoryNode)
	for _, node := range storyData.Nodes {
		nodesByType[node.Type] = append(nodesByType[node.Type], node)
	}

	for nodeType, nodes := range nodesByType {
		content.WriteString(fmt.Sprintf("### %s 类型节点\n\n", nodeType))

		for i, node := range nodes {
			if !node.IsRevealed {
				continue // 跳过未揭示的节点
			}

			content.WriteString(fmt.Sprintf("#### %d. 节点: %s\n\n", i+1, node.ID))
			content.WriteString(fmt.Sprintf("**内容**: %s\n\n", node.Content))
			content.WriteString(fmt.Sprintf("- **创建时间**: %s\n", node.CreatedAt.Format("2006-01-02 15:04")))

			if node.ParentID != "" {
				content.WriteString(fmt.Sprintf("- **父节点**: %s\n", node.ParentID))
			}

			// 显示选择
			if len(node.Choices) > 0 {
				content.WriteString("- **可用选择**:\n")
				for j, choice := range node.Choices {
					status := ""
					if choice.Selected {
						status = " ✅"
					}
					content.WriteString(fmt.Sprintf("  %d. %s%s\n", j+1, choice.Text, status))
					if choice.Consequence != "" {
						content.WriteString(fmt.Sprintf("     - *后果*: %s\n", choice.Consequence))
					}
				}
			}
			content.WriteString("\n")
		}
	}

	// 任务详情
	content.WriteString("## 📋 任务详情\n\n")

	// 按状态组织任务
	completedTasks := []models.Task{}
	activeTasks := []models.Task{}
	hiddenTasks := []models.Task{}

	for _, task := range storyData.Tasks {
		if task.Completed {
			completedTasks = append(completedTasks, task)
		} else if task.IsRevealed {
			activeTasks = append(activeTasks, task)
		} else {
			hiddenTasks = append(hiddenTasks, task)
		}
	}

	// 已完成任务
	if len(completedTasks) > 0 {
		content.WriteString("### ✅ 已完成任务\n\n")
		for i, task := range completedTasks {
			content.WriteString(fmt.Sprintf("#### %d. %s\n\n", i+1, task.Title))
			content.WriteString(fmt.Sprintf("**描述**: %s\n\n", task.Description))
			if len(task.Objectives) > 0 {
				content.WriteString("**目标**:\n")
				for j, obj := range task.Objectives {
					content.WriteString(fmt.Sprintf("%d. %s\n", j+1, obj.Description))
				}
				content.WriteString("\n")
			}
			if task.Reward != "" {
				content.WriteString(fmt.Sprintf("**奖励**: %s\n", task.Reward))
			}
			content.WriteString("\n")
		}
	}

	// 进行中任务
	if len(activeTasks) > 0 {
		content.WriteString("### 🔄 进行中任务\n\n")
		for i, task := range activeTasks {
			content.WriteString(fmt.Sprintf("#### %d. %s\n\n", i+1, task.Title))
			content.WriteString(fmt.Sprintf("**描述**: %s\n\n", task.Description))
			if len(task.Objectives) > 0 {
				content.WriteString("**目标**:\n")
				for j, obj := range task.Objectives {
					content.WriteString(fmt.Sprintf("%d. %s\n", j+1, obj.Description))
				}
				content.WriteString("\n")
			}
			content.WriteString("\n")
		}
	}

	// 隐藏任务（仅显示数量）
	if len(hiddenTasks) > 0 {
		content.WriteString(fmt.Sprintf("### 🔒 隐藏任务 (%d 个)\n\n", len(hiddenTasks)))
		content.WriteString("*这些任务尚未揭示，将在满足特定条件后显示。*\n\n")
	}

	// 故事地点
	if len(storyData.Locations) > 0 {
		content.WriteString("## 🗺️ 故事地点\n\n")
		for i, location := range storyData.Locations {
			content.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, location.Name))
			content.WriteString(fmt.Sprintf("**描述**: %s\n\n", location.Description))
		}
	}

	// 导出信息
	content.WriteString("## 📄 导出信息\n\n")
	content.WriteString(fmt.Sprintf("- **导出时间**: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString("- **导出格式**: Markdown\n")
	content.WriteString("- **导出类型**: 故事文档\n")
	content.WriteString("- **数据来源**: SceneIntruderMCP 故事服务\n")
	content.WriteString("- **版本**: v1.0\n")

	return content.String(), nil
}

// formatStoryAsText 纯文本格式导出故事
func (s *ExportService) formatStoryAsText(
	sceneData *SceneData,
	storyData *models.StoryData,
	summary string,
	stats *models.StoryExportStats) (string, error) {

	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	if storyData == nil {
		return "", fmt.Errorf("故事数据不能为空")
	}

	var content strings.Builder

	// 标题
	content.WriteString(strings.Repeat("=", 60) + "\n")
	content.WriteString(fmt.Sprintf("    %s - 完整故事文档\n", sceneData.Scene.Title))
	content.WriteString(strings.Repeat("=", 60) + "\n\n")

	// ✅ 使用 stats 添加统计概览
	content.WriteString("故事统计概览\n")
	content.WriteString(strings.Repeat("-", 30) + "\n")
	content.WriteString(fmt.Sprintf("故事进度: %d%%\n", stats.Progress))
	content.WriteString(fmt.Sprintf("当前状态: %s\n", stats.CurrentState))
	content.WriteString(fmt.Sprintf("总节点数: %d 个\n", stats.TotalNodes))
	content.WriteString(fmt.Sprintf("已揭示节点: %d 个\n", stats.RevealedNodes))
	content.WriteString(fmt.Sprintf("总任务数: %d 个\n", stats.TotalTasks))
	content.WriteString(fmt.Sprintf("已完成任务: %d 个\n", stats.CompletedTasks))
	content.WriteString(fmt.Sprintf("完成率: %.1f%%\n", stats.CompletionRate))
	content.WriteString(fmt.Sprintf("故事分支数: %d 个\n", stats.BranchCount))
	content.WriteString(fmt.Sprintf("最大深度: %d 层\n", stats.MaxDepth))
	content.WriteString(fmt.Sprintf("总选择数: %d 个\n", stats.TotalChoices))
	content.WriteString(fmt.Sprintf("已做选择: %d 个\n", stats.SelectedChoices))
	content.WriteString("\n")

	// ✅ 使用 stats 显示节点类型分布
	if len(stats.NodesByType) > 0 {
		content.WriteString("节点类型分布:\n")
		for nodeType, count := range stats.NodesByType {
			percentage := float64(count) / float64(stats.TotalNodes) * 100
			content.WriteString(fmt.Sprintf("  %s: %d 个 (%.1f%%)\n", nodeType, count, percentage))
		}
		content.WriteString("\n")
	}

	// ✅ 使用 stats 显示任务状态分布
	if len(stats.TasksByStatus) > 0 {
		content.WriteString("任务状态分布:\n")
		for status, count := range stats.TasksByStatus {
			statusName := status
			switch status {
			case "completed":
				statusName = "已完成"
			case "active":
				statusName = "进行中"
			case "hidden":
				statusName = "隐藏"
			}
			content.WriteString(fmt.Sprintf("  %s: %d 个\n", statusName, count))
		}
		content.WriteString("\n")
	}

	// ✅ 使用 stats 显示角色参与度
	if len(stats.CharacterInvolvement) > 0 {
		content.WriteString("角色参与度:\n")
		for charID, involvement := range stats.CharacterInvolvement {
			// 查找角色名称
			var charName string
			for _, char := range sceneData.Characters {
				if char.ID == charID {
					charName = char.Name
					break
				}
			}
			if charName == "" {
				charName = charID
			}
			content.WriteString(fmt.Sprintf("  %s: 参与 %d 个任务\n", charName, involvement))
		}
		content.WriteString("\n")
	}

	// 移除Markdown格式的摘要
	textSummary := strings.ReplaceAll(summary, "#", "")
	textSummary = strings.ReplaceAll(textSummary, "**", "")
	textSummary = strings.ReplaceAll(textSummary, "*", "")
	content.WriteString(textSummary)
	content.WriteString("\n")

	// 故事节点详情
	content.WriteString(strings.Repeat("-", 40) + "\n")
	content.WriteString("故事节点详情\n")
	content.WriteString(strings.Repeat("-", 40) + "\n\n")

	nodeIndex := 1
	for _, node := range storyData.Nodes {
		if !node.IsRevealed {
			continue
		}

		content.WriteString(fmt.Sprintf("%d. 节点: %s (%s)\n", nodeIndex, node.ID, node.Type))
		content.WriteString(fmt.Sprintf("   内容: %s\n", node.Content))
		content.WriteString(fmt.Sprintf("   创建时间: %s\n", node.CreatedAt.Format("2006-01-02 15:04")))

		if len(node.Choices) > 0 {
			content.WriteString("   可用选择:\n")
			for j, choice := range node.Choices {
				status := ""
				if choice.Selected {
					status = " [已选择]"
				}
				content.WriteString(fmt.Sprintf("     %d. %s%s\n", j+1, choice.Text, status))
			}
		}
		content.WriteString("\n")
		nodeIndex++
	}

	// 任务详情
	content.WriteString(strings.Repeat("-", 40) + "\n")
	content.WriteString("任务详情\n")
	content.WriteString(strings.Repeat("-", 40) + "\n\n")

	taskIndex := 1
	for _, task := range storyData.Tasks {
		if !task.IsRevealed && !task.Completed {
			continue
		}

		status := "进行中"
		if task.Completed {
			status = "已完成"
		}

		content.WriteString(fmt.Sprintf("%d. %s [%s]\n", taskIndex, task.Title, status))
		content.WriteString(fmt.Sprintf("   描述: %s\n", task.Description))

		if len(task.Objectives) > 0 {
			content.WriteString("   目标:\n")
			for j, obj := range task.Objectives {
				content.WriteString(fmt.Sprintf("     %d. %s\n", j+1, obj.Description))
			}
		}

		if task.Reward != "" {
			content.WriteString(fmt.Sprintf("   奖励: %s\n", task.Reward))
		}
		content.WriteString("\n")
		taskIndex++
	}

	// ✅ 使用 stats 添加故事分析洞察
	content.WriteString(strings.Repeat("-", 40) + "\n")
	content.WriteString("故事分析洞察\n")
	content.WriteString(strings.Repeat("-", 40) + "\n\n")

	// 进度分析
	var progressLevel string
	switch {
	case stats.Progress >= 90:
		progressLevel = "接近尾声 - 故事即将完结"
	case stats.Progress >= 70:
		progressLevel = "后期发展 - 主要冲突趋于解决"
	case stats.Progress >= 50:
		progressLevel = "中期发展 - 故事冲突加剧"
	case stats.Progress >= 30:
		progressLevel = "初期发展 - 故事情节逐步展开"
	case stats.Progress >= 10:
		progressLevel = "开始阶段 - 世界观和角色介绍"
	default:
		progressLevel = "序幕阶段 - 故事刚刚开始"
	}
	content.WriteString(fmt.Sprintf("发展阶段: %s\n", progressLevel))

	// 复杂度分析
	var complexityLevel string
	if stats.TotalNodes > 0 {
		avgChoicesPerNode := float64(stats.TotalChoices) / float64(stats.TotalNodes)
		switch {
		case avgChoicesPerNode >= 4:
			complexityLevel = "高 - 选择丰富，分支复杂"
		case avgChoicesPerNode >= 2.5:
			complexityLevel = "中等 - 适度的选择和分支"
		case avgChoicesPerNode >= 1.5:
			complexityLevel = "较低 - 相对线性的发展"
		default:
			complexityLevel = "低 - 线性故事发展"
		}
		content.WriteString(fmt.Sprintf("故事复杂度: %s (平均 %.1f 选择/节点)\n", complexityLevel, avgChoicesPerNode))
	}

	// 互动性分析
	if stats.TotalChoices > 0 {
		interactionRate := float64(stats.SelectedChoices) / float64(stats.TotalChoices) * 100
		var interactionLevel string
		switch {
		case interactionRate >= 70:
			interactionLevel = "高 - 玩家积极参与决策"
		case interactionRate >= 50:
			interactionLevel = "中等 - 适度的玩家参与"
		case interactionRate >= 30:
			interactionLevel = "较低 - 玩家参与有限"
		default:
			interactionLevel = "低 - 缺乏玩家互动"
		}
		content.WriteString(fmt.Sprintf("互动性水平: %s (%.1f%% 选择已做出)\n", interactionLevel, interactionRate))
	}

	content.WriteString("\n")

	// 导出信息
	content.WriteString(strings.Repeat("=", 60) + "\n")
	content.WriteString("导出信息\n")
	content.WriteString(strings.Repeat("=", 60) + "\n")
	content.WriteString(fmt.Sprintf("导出时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString("导出格式: 纯文本\n")
	content.WriteString("导出类型: 故事文档\n")
	content.WriteString("数据来源: SceneIntruderMCP 故事服务\n")
	content.WriteString("版本: v1.0\n")

	// ✅ 使用 stats 添加数据完整性信息
	content.WriteString(fmt.Sprintf("统计数据: %d 节点, %d 任务, %d 选择\n",
		stats.TotalNodes, stats.TotalTasks, stats.TotalChoices))
	content.WriteString(fmt.Sprintf("导出完整性: %.1f%% 节点已揭示\n",
		float64(stats.RevealedNodes)/float64(stats.TotalNodes)*100))

	return content.String(), nil
}

// formatStoryAsHTML HTML格式导出故事
func (s *ExportService) formatStoryAsHTML(
	sceneData *SceneData,
	storyData *models.StoryData,
	summary string,
	stats *models.StoryExportStats) (string, error) {

	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	if storyData == nil {
		return "", fmt.Errorf("故事数据不能为空")
	}

	var content strings.Builder

	// HTML 头部
	content.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>`)
	content.WriteString(sceneData.Scene.Title + " - 故事文档")
	content.WriteString(`</title>
    <style>
        body { 
            font-family: 'Microsoft YaHei', Arial, sans-serif; 
            margin: 20px; 
            line-height: 1.6; 
            color: #333;
            background-color: #f5f5f5;
        }
        .container { 
            max-width: 1200px; 
            margin: 0 auto; 
            background: white; 
            padding: 30px; 
            border-radius: 10px; 
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .header { 
            text-align: center; 
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); 
            color: white; 
            padding: 30px; 
            margin: -30px -30px 30px -30px; 
            border-radius: 10px 10px 0 0;
        }
        .header h1 { margin: 0; font-size: 2.5em; }
        .section { margin-bottom: 30px; }
        .section h2 { 
            color: #2c3e50; 
            border-bottom: 2px solid #3498db; 
            padding-bottom: 10px; 
        }
        .node-card, .task-card { 
            background: #f8f9fa; 
            padding: 20px; 
            margin: 15px 0; 
            border-radius: 8px; 
            border-left: 4px solid #3498db;
        }
        .task-card.completed { border-left-color: #27ae60; }
        .task-card.active { border-left-color: #f39c12; }
        .task-card.hidden { border-left-color: #95a5a6; }
        .choice { 
            background: #e8f4fd; 
            padding: 10px; 
            margin: 5px 0; 
            border-radius: 5px; 
        }
        .choice.selected { 
            background: #d4edda; 
            border-left: 3px solid #28a745; 
        }
        .stats-grid { 
            display: grid; 
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); 
            gap: 15px; 
            margin: 20px 0; 
        }
        .stat-card { 
            background: #f8f9fa; 
            padding: 15px; 
            text-align: center; 
            border-radius: 8px; 
            border: 1px solid #dee2e6;
        }
        .stat-number { font-size: 2em; font-weight: bold; color: #3498db; }
        .stat-label { color: #6c757d; font-size: 0.9em; }
        .progress-bar { 
            background: #e9ecef; 
            height: 20px; 
            border-radius: 10px; 
            overflow: hidden; 
            margin: 10px 0;
        }
        .progress-fill { 
            height: 100%; 
            background: linear-gradient(90deg, #28a745, #20c997); 
            transition: width 0.3s ease;
        }
        .summary-section {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            border-left: 4px solid #6f42c1;
            margin: 20px 0;
        }
        .summary-content {
            font-size: 1.05em;
            line-height: 1.7;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>`)
	content.WriteString(sceneData.Scene.Title)
	content.WriteString(`</h1>
            <p>完整故事文档</p>
            <p>故事进度: `)
	content.WriteString(fmt.Sprintf("%d%%", storyData.Progress))
	content.WriteString(`</p>
        </div>

        <div class="section">
            <h2>📊 故事统计</h2>
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", stats.TotalNodes))
	content.WriteString(`</div>
                    <div class="stat-label">故事节点</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", stats.TotalTasks))
	content.WriteString(`</div>
                    <div class="stat-label">总任务数</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", stats.CompletedTasks))
	content.WriteString(`</div>
                    <div class="stat-label">已完成任务</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%.1f%%", stats.CompletionRate))
	content.WriteString(`</div>
                    <div class="stat-label">完成度</div>
                </div>
            </div>
            <div class="progress-bar">
                <div class="progress-fill" style="width: `)
	content.WriteString(fmt.Sprintf("%.1f%%", stats.CompletionRate))
	content.WriteString(`"></div>
            </div>
        </div>`)

	// ✅ 使用 summary 参数添加故事摘要内容
	content.WriteString(`<div class="section">
            <h2>📝 故事分析报告</h2>
            <div class="summary-section">
                <div class="summary-content">`)

	// 将 Markdown 格式的摘要转换为 HTML
	htmlSummary := strings.ReplaceAll(summary, "### ", "<h3>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "## ", "<h2>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "**", "<strong>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "*", "</strong>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "\n- ", "<li>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "\n\n", "</p><p>")

	// 处理列表格式
	htmlSummary = strings.ReplaceAll(htmlSummary, "<li>", "</p><ul><li>")

	// 包装在段落中
	if !strings.HasPrefix(htmlSummary, "<h2>") {
		htmlSummary = "<p>" + htmlSummary
	}
	if !strings.HasSuffix(htmlSummary, "</p>") && !strings.HasSuffix(htmlSummary, "</ul>") {
		htmlSummary = htmlSummary + "</p>"
	}

	content.WriteString(htmlSummary)
	content.WriteString(`</div>
            </div>
        </div>`)

	// 故事概览
	content.WriteString(`<div class="section">
            <h2>📖 故事概览</h2>
            <p><strong>故事简介：</strong>`)
	content.WriteString(storyData.Intro)
	content.WriteString(`</p>
            <p><strong>主要目标：</strong>`)
	content.WriteString(storyData.MainObjective)
	content.WriteString(`</p>
            <p><strong>当前状态：</strong>`)
	content.WriteString(storyData.CurrentState)
	content.WriteString(`</p>
        </div>`)

	// 故事节点
	content.WriteString(`<div class="section">
            <h2>📚 故事节点</h2>`)

	for i, node := range storyData.Nodes {
		if !node.IsRevealed {
			continue
		}

		content.WriteString(`<div class="node-card">
                <h3>节点 `)
		content.WriteString(fmt.Sprintf("%d: %s", i+1, node.ID))
		content.WriteString(`</h3>
                <p><strong>类型：</strong>`)
		content.WriteString(node.Type)
		content.WriteString(`</p>
                <p><strong>内容：</strong>`)
		content.WriteString(node.Content)
		content.WriteString(`</p>`)

		if len(node.Choices) > 0 {
			content.WriteString(`<h4>可用选择：</h4>`)
			for j, choice := range node.Choices {
				cssClass := "choice"
				if choice.Selected {
					cssClass += " selected"
				}
				content.WriteString(fmt.Sprintf(`<div class="%s">`, cssClass))
				content.WriteString(fmt.Sprintf(`<strong>%d.</strong> %s`, j+1, choice.Text))
				if choice.Selected {
					content.WriteString(` ✅`)
				}
				if choice.Consequence != "" {
					content.WriteString(`<br><em>后果：`)
					content.WriteString(choice.Consequence)
					content.WriteString(`</em>`)
				}
				content.WriteString(`</div>`)
			}
		}

		content.WriteString(`</div>`)
	}

	content.WriteString(`</div>`)

	// 任务详情
	content.WriteString(`<div class="section">
            <h2>📋 任务详情</h2>`)

	for i, task := range storyData.Tasks {
		if !task.IsRevealed && !task.Completed {
			continue
		}

		cssClass := "task-card"
		status := "进行中"
		if task.Completed {
			cssClass += " completed"
			status = "已完成"
		} else if task.IsRevealed {
			cssClass += " active"
		} else {
			cssClass += " hidden"
			status = "隐藏"
		}

		content.WriteString(fmt.Sprintf(`<div class="%s">`, cssClass))
		content.WriteString(`<h3>`)
		content.WriteString(fmt.Sprintf("%d. %s [%s]", i+1, task.Title, status))
		content.WriteString(`</h3>
                <p><strong>描述：</strong>`)
		content.WriteString(task.Description)
		content.WriteString(`</p>`)

		if len(task.Objectives) > 0 {
			content.WriteString(`<p><strong>目标：</strong></p><ul>`)
			for _, obj := range task.Objectives {
				content.WriteString(`<li>`)
				content.WriteString(obj.Description)
				content.WriteString(`</li>`)
			}
			content.WriteString(`</ul>`)
		}

		if task.Reward != "" {
			content.WriteString(`<p><strong>奖励：</strong>`)
			content.WriteString(task.Reward)
			content.WriteString(`</p>`)
		}

		content.WriteString(`</div>`)
	}

	content.WriteString(`</div>`)

	// 导出信息
	content.WriteString(`<div class="section">
            <h2>📄 导出信息</h2>
            <p><strong>导出时间：</strong>`)
	content.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	content.WriteString(`</p>
            <p><strong>导出格式：</strong>HTML</p>
            <p><strong>导出类型：</strong>故事文档</p>
            <p><strong>数据来源：</strong>SceneIntruderMCP 故事服务</p>
            <p><strong>版本：</strong>v1.0</p>
        </div>

    </div>
</body>
</html>`)

	return content.String(), nil
}

// saveStoryExportToDataDir 保存故事导出文件到data目录
func (s *ExportService) saveStoryExportToDataDir(result *models.ExportResult) (string, int64, error) {
	// 创建导出目录
	exportDir := filepath.Join("data", "exports", "stories")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return "", 0, fmt.Errorf("创建导出目录失败: %w", err)
	}

	// 生成文件名
	timestamp := result.GeneratedAt.Format("20060102_150405")
	fileName := fmt.Sprintf("%s_story_document_%s.%s",
		result.SceneID, timestamp, result.Format)

	filePath := filepath.Join(exportDir, fileName)

	// 写入文件
	if err := os.WriteFile(filePath, []byte(result.Content), 0644); err != nil {
		return "", 0, fmt.Errorf("写入导出文件失败: %w", err)
	}

	// 获取文件大小
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("获取文件信息失败: %w", err)
	}

	return filePath, fileInfo.Size(), nil
}

// buildSceneMetadata 构建场景元数据
func (s *ExportService) buildSceneMetadata(sceneData *SceneData) *models.SceneMetadata {
	return &models.SceneMetadata{
		ID:             sceneData.Scene.ID,
		Name:           sceneData.Scene.Title,
		Source:         sceneData.Scene.Source,
		CreatedAt:      sceneData.Scene.CreatedAt,
		LastAccessed:   sceneData.Scene.LastAccessed,
		CharacterCount: len(sceneData.Characters),
	}
}

// ---------------------------------------------
// ✅ 场景导出方法
func (s *ExportService) ExportSceneData(ctx context.Context, sceneID string, format string, includeConversations bool) (*models.ExportResult, error) {
	// 1. 验证输入参数
	if sceneID == "" {
		return nil, fmt.Errorf("场景ID不能为空")
	}

	supportedFormats := []string{"json", "markdown", "txt", "html"}
	if !contains(supportedFormats, strings.ToLower(format)) {
		return nil, fmt.Errorf("不支持的导出格式: %s，支持的格式: %v", format, supportedFormats)
	}

	// 2. 获取场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, fmt.Errorf("加载场景失败: %w", err)
	}

	// 3. 获取对话记录（如果需要）
	var conversations []models.Conversation
	if includeConversations {
		conversations, err = s.getInteractionHistory(sceneID)
		if err != nil {
			// 对话获取失败不阻止导出，仅记录日志
			conversations = []models.Conversation{}
		}
	}

	// 4. 分析场景统计数据
	sceneStats := s.analyzeSceneStatistics(sceneData, conversations)

	// 5. 生成场景摘要内容
	sceneSummary := s.generateSceneSummary(sceneData, conversations, sceneStats, includeConversations)

	// 6. 根据格式生成内容
	content, err := s.formatSceneExportContent(sceneData, conversations, sceneSummary, sceneStats, format, includeConversations)
	if err != nil {
		return nil, fmt.Errorf("格式化场景导出内容失败: %w", err)
	}

	// 7. 创建导出结果
	result := &models.ExportResult{
		SceneID:       sceneID,
		Title:         fmt.Sprintf("%s - 场景数据", sceneData.Scene.Title),
		Format:        format,
		Content:       content,
		ExportType:    "scene",
		GeneratedAt:   time.Now(),
		Characters:    sceneData.Characters,
		SceneMetadata: s.buildSceneMetadata(sceneData),
	}

	// 如果包含对话，添加对话数据
	if includeConversations {
		result.Conversations = conversations
	}

	// 8. 保存到 data 目录
	filePath, fileSize, err := s.saveSceneExportToDataDir(result)
	if err != nil {
		return nil, fmt.Errorf("保存场景导出文件失败: %w", err)
	}

	result.FilePath = filePath
	result.FileSize = fileSize

	return result, nil
}

// formatSceneExportContent 根据格式生成场景导出内容
func (s *ExportService) formatSceneExportContent(
	sceneData *SceneData,
	conversations []models.Conversation,
	summary string,
	stats *models.InteractionExportStats, // 改为使用现有结构体
	format string,
	includeConversations bool) (string, error) {

	switch strings.ToLower(format) {
	case "json":
		return s.formatSceneAsJSON(sceneData, conversations, summary, stats, includeConversations)
	case "markdown":
		return s.formatSceneAsMarkdown(sceneData, conversations, summary, stats, includeConversations)
	case "txt":
		return s.formatSceneAsText(sceneData, conversations, summary, stats, includeConversations)
	case "html":
		return s.formatSceneAsHTML(sceneData, conversations, summary, stats, includeConversations)
	default:
		return "", fmt.Errorf("不支持的格式: %s", format)
	}
}

// analyzeSceneStatistics 分析场景统计数据
func (s *ExportService) analyzeSceneStatistics(sceneData *SceneData, conversations []models.Conversation) *models.InteractionExportStats {
	stats := &models.InteractionExportStats{
		EmotionDistribution: make(map[string]int),
		TopKeywords:         []string{},
	}

	if sceneData == nil {
		return stats
	}

	// 基础统计
	stats.TotalMessages = len(conversations)
	stats.CharacterCount = len(sceneData.Characters)

	// 设置日期范围
	if len(conversations) > 0 {
		stats.DateRange.StartDate = conversations[0].Timestamp
		stats.DateRange.EndDate = conversations[0].Timestamp

		for _, conv := range conversations {
			if conv.Timestamp.Before(stats.DateRange.StartDate) {
				stats.DateRange.StartDate = conv.Timestamp
			}
			if conv.Timestamp.After(stats.DateRange.EndDate) {
				stats.DateRange.EndDate = conv.Timestamp
			}

			// 情绪分布统计
			if len(conv.Emotions) > 0 {
				for _, emotion := range conv.Emotions {
					stats.EmotionDistribution[emotion]++
				}
			}
		}

		// 统计交互次数
		interactionIDs := make(map[string]bool)
		wordCount := make(map[string]int)

		for _, conv := range conversations {
			// 统计独立交互
			if conv.Metadata != nil {
				if interactionID, exists := conv.Metadata["interaction_id"]; exists {
					interactionIDs[fmt.Sprintf("%v", interactionID)] = true
				}
			}

			// 关键词统计
			words := strings.Fields(strings.ToLower(conv.Content))
			for _, word := range words {
				if len(word) > 3 && !isCommonWord(word) {
					wordCount[word]++
				}
			}
		}

		stats.TotalInteractions = len(interactionIDs)
		stats.TopKeywords = extractTopKeywords(wordCount, 10)
	}

	return stats
}

// generateSceneSummary 生成场景摘要
func (s *ExportService) generateSceneSummary(
	sceneData *SceneData,
	conversations []models.Conversation,
	stats *models.InteractionExportStats,
	includeConversations bool) string {

	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("## %s - 场景数据报告\n\n", sceneData.Scene.Title))

	// 场景基本信息
	summary.WriteString("### 场景概览\n\n")
	summary.WriteString(fmt.Sprintf("- **场景ID**: %s\n", sceneData.Scene.ID))
	summary.WriteString(fmt.Sprintf("- **场景名称**: %s\n", sceneData.Scene.Title))
	summary.WriteString(fmt.Sprintf("- **场景描述**: %s\n", sceneData.Scene.Description))

	if sceneData.Scene.Source != "" {
		summary.WriteString(fmt.Sprintf("- **数据来源**: %s\n", sceneData.Scene.Source))
	}

	summary.WriteString(fmt.Sprintf("- **创建时间**: %s\n", sceneData.Scene.CreatedAt.Format("2006-01-02 15:04:05")))
	summary.WriteString(fmt.Sprintf("- **最后更新**: %s\n", sceneData.Scene.LastUpdated.Format("2006-01-02 15:04:05")))
	summary.WriteString(fmt.Sprintf("- **最后访问**: %s\n", sceneData.Scene.LastAccessed.Format("2006-01-02 15:04:05")))
	summary.WriteString("\n")

	// 环境设定
	summary.WriteString("### 环境设定\n\n")
	if len(sceneData.Scene.Themes) > 0 {
		summary.WriteString(fmt.Sprintf("- **主要主题**: %s\n", strings.Join(sceneData.Scene.Themes, ", ")))
	}
	if sceneData.Scene.Era != "" {
		summary.WriteString(fmt.Sprintf("- **时代背景**: %s\n", sceneData.Scene.Era))
	}
	if sceneData.Scene.Atmosphere != "" {
		summary.WriteString(fmt.Sprintf("- **氛围设定**: %s\n", sceneData.Scene.Atmosphere))
	}
	summary.WriteString("\n")

	// 统计数据概览
	summary.WriteString("### 数据统计\n\n")
	summary.WriteString(fmt.Sprintf("- **角色数量**: %d 位\n", stats.CharacterCount))
	summary.WriteString(fmt.Sprintf("- **地点数量**: %d 个\n", len(sceneData.Scene.Locations)))
	summary.WriteString(fmt.Sprintf("- **道具数量**: %d 个\n", len(sceneData.Scene.Items)))
	summary.WriteString(fmt.Sprintf("- **消息数量**: %d 条\n", stats.TotalMessages))
	summary.WriteString(fmt.Sprintf("- **交互次数**: %d 次\n", stats.TotalInteractions))

	if includeConversations && stats.TotalMessages > 0 {
		summary.WriteString(fmt.Sprintf("- **对话记录**: %d 条\n", stats.TotalMessages))
	}
	summary.WriteString("\n")

	// ✅ 使用 conversations 参数进行对话内容分析
	if len(conversations) > 0 {
		summary.WriteString("### 对话内容分析\n\n")

		// 分析发言者分布
		speakerCount := make(map[string]int)
		userMessages := 0
		characterMessages := 0
		totalContentLength := 0

		for _, conv := range conversations {
			speakerCount[conv.Speaker]++
			totalContentLength += len(conv.Content)

			// 通过metadata判断是用户还是角色消息
			if conv.Metadata != nil {
				if speakerType, exists := conv.Metadata["speaker_type"]; exists {
					switch speakerType {
					case "user":
						userMessages++
					case "character":
						characterMessages++
					default:
						// 对于未知类型，通过Speaker字段判断
						if conv.Speaker == "user" || conv.Speaker == "用户" {
							userMessages++
						} else {
							characterMessages++
						}
					}
				}
			} else {
				// 如果没有metadata，通过Speaker字段判断
				if conv.Speaker == "user" || conv.Speaker == "用户" {
					userMessages++
				} else {
					characterMessages++
				}
			}
		}

		summary.WriteString(fmt.Sprintf("- **用户发言**: %d 条 (%.1f%%)\n",
			userMessages, float64(userMessages)/float64(len(conversations))*100))
		summary.WriteString(fmt.Sprintf("- **角色响应**: %d 条 (%.1f%%)\n",
			characterMessages, float64(characterMessages)/float64(len(conversations))*100))

		// 最活跃的发言者
		var mostActiveSpeaker string
		maxCount := 0
		for speaker, count := range speakerCount {
			if count > maxCount && speaker != "user" && speaker != "用户" {
				maxCount = count
				mostActiveSpeaker = speaker
			}
		}
		if mostActiveSpeaker != "" {
			summary.WriteString(fmt.Sprintf("- **最活跃角色**: %s (%d 条发言)\n", mostActiveSpeaker, maxCount))
		}

		// 对话质量分析
		if len(conversations) > 0 {
			avgContentLength := float64(totalContentLength) / float64(len(conversations))
			summary.WriteString(fmt.Sprintf("- **平均消息长度**: %.1f 字符\n", avgContentLength))

			// 互动质量评级
			var qualityLevel string
			switch {
			case avgContentLength >= 100:
				qualityLevel = "高 - 内容详细丰富"
			case avgContentLength >= 50:
				qualityLevel = "中等 - 内容适中"
			case avgContentLength >= 20:
				qualityLevel = "较低 - 内容相对简短"
			default:
				qualityLevel = "低 - 内容过于简单"
			}
			summary.WriteString(fmt.Sprintf("- **内容质量**: %s\n", qualityLevel))
		}

		// 时间分布分析
		if len(conversations) >= 2 {
			firstTime := conversations[0].Timestamp
			lastTime := conversations[len(conversations)-1].Timestamp

			if !firstTime.IsZero() && !lastTime.IsZero() {
				duration := lastTime.Sub(firstTime)
				summary.WriteString(fmt.Sprintf("- **对话时间跨度**: %.1f 分钟\n", duration.Minutes()))

				if duration.Minutes() > 0 {
					avgInterval := duration.Seconds() / float64(len(conversations)-1)
					summary.WriteString(fmt.Sprintf("- **平均发言间隔**: %.1f 秒\n", avgInterval))
				}
			}
		}

		summary.WriteString("\n")
	}

	// ✅ 使用 conversations 分析角色互动模式
	if len(conversations) > 0 && len(sceneData.Characters) > 1 {
		summary.WriteString("### 角色互动模式\n\n")

		// 分析角色参与度
		characterParticipation := make(map[string]int)
		for _, conv := range conversations {
			if conv.SpeakerID != "" && conv.SpeakerID != "user" {
				characterParticipation[conv.SpeakerID]++
			}
		}

		if len(characterParticipation) > 0 {
			// 计算参与度分布
			totalCharacterMessages := 0
			for _, count := range characterParticipation {
				totalCharacterMessages += count
			}

			summary.WriteString(fmt.Sprintf("- **参与角色数**: %d / %d\n",
				len(characterParticipation), len(sceneData.Characters)))

			participationRate := float64(len(characterParticipation)) / float64(len(sceneData.Characters)) * 100
			summary.WriteString(fmt.Sprintf("- **角色参与率**: %.1f%%\n", participationRate))

			// 分析互动均衡性
			if len(characterParticipation) > 1 {
				maxParticipation := 0
				minParticipation := totalCharacterMessages
				for _, count := range characterParticipation {
					if count > maxParticipation {
						maxParticipation = count
					}
					if count < minParticipation {
						minParticipation = count
					}
				}

				balanceRatio := float64(minParticipation) / float64(maxParticipation) * 100
				var balanceLevel string
				switch {
				case balanceRatio >= 80:
					balanceLevel = "高 - 角色发言相对均衡"
				case balanceRatio >= 60:
					balanceLevel = "中等 - 角色参与度适中"
				case balanceRatio >= 40:
					balanceLevel = "较低 - 部分角色发言较少"
				default:
					balanceLevel = "低 - 角色参与度差异较大"
				}
				summary.WriteString(fmt.Sprintf("- **互动均衡性**: %s (%.1f%%)\n", balanceLevel, balanceRatio))
			}
		}

		summary.WriteString("\n")
	}

	// ✅ 使用 conversations 分析场景要素利用情况
	if len(conversations) > 0 {
		summary.WriteString("### 场景要素利用情况\n\n")

		allText := ""
		for _, conv := range conversations {
			allText += strings.ToLower(conv.Content) + " "
		}

		// 分析地点提及情况
		if len(sceneData.Scene.Locations) > 0 {
			mentionedLocations := 0
			for _, location := range sceneData.Scene.Locations {
				locationName := strings.ToLower(location.Name)
				if strings.Contains(allText, locationName) {
					mentionedLocations++
				}
			}

			locationUtilization := float64(mentionedLocations) / float64(len(sceneData.Scene.Locations)) * 100
			summary.WriteString(fmt.Sprintf("- **地点利用率**: %.1f%% (%d/%d 地点被提及)\n",
				locationUtilization, mentionedLocations, len(sceneData.Scene.Locations)))
		}

		// 分析道具提及情况
		if len(sceneData.Scene.Items) > 0 {
			mentionedItems := 0
			for _, item := range sceneData.Scene.Items {
				itemName := strings.ToLower(item.Name)
				if strings.Contains(allText, itemName) {
					mentionedItems++
				}
			}

			itemUtilization := float64(mentionedItems) / float64(len(sceneData.Scene.Items)) * 100
			summary.WriteString(fmt.Sprintf("- **道具利用率**: %.1f%% (%d/%d 道具被提及)\n",
				itemUtilization, mentionedItems, len(sceneData.Scene.Items)))
		}

		summary.WriteString("\n")
	}

	// 基于现有字段的分析（保持原有逻辑）
	if len(stats.EmotionDistribution) > 0 {
		summary.WriteString("### 情绪分布\n\n")
		for emotion, count := range stats.EmotionDistribution {
			percentage := float64(count) / float64(stats.TotalMessages) * 100
			summary.WriteString(fmt.Sprintf("- **%s**: %d次 (%.1f%%)\n", emotion, count, percentage))
		}
		summary.WriteString("\n")
	}

	if len(stats.TopKeywords) > 0 {
		summary.WriteString("### 热门关键词\n\n")
		for i, keyword := range stats.TopKeywords {
			if i >= 10 {
				break
			}
			summary.WriteString(fmt.Sprintf("%d. %s\n", i+1, keyword))
		}
		summary.WriteString("\n")
	}

	// ✅ 基于 conversations 的活动状态评估
	if len(conversations) > 0 {
		summary.WriteString("### 场景活跃度评估\n\n")

		// 计算活跃度指标
		if stats.TotalInteractions > 0 {
			avgMessagesPerInteraction := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
			summary.WriteString(fmt.Sprintf("- **平均交互深度**: %.1f 消息/交互\n", avgMessagesPerInteraction))

			var activityLevel string
			switch {
			case avgMessagesPerInteraction >= 8:
				activityLevel = "极高 - 深度互动，内容丰富"
			case avgMessagesPerInteraction >= 5:
				activityLevel = "高 - 活跃互动，参与度好"
			case avgMessagesPerInteraction >= 3:
				activityLevel = "中等 - 正常交流水平"
			case avgMessagesPerInteraction >= 1.5:
				activityLevel = "较低 - 互动相对简短"
			default:
				activityLevel = "低 - 需要提升互动质量"
			}
			summary.WriteString(fmt.Sprintf("- **活跃度评级**: %s\n", activityLevel))
		}

		// 场景完整性评估
		utilizationScore := 0.0
		factors := 0

		if len(sceneData.Characters) > 0 {
			characterParticipation := make(map[string]bool)
			for _, conv := range conversations {
				if conv.SpeakerID != "" && conv.SpeakerID != "user" {
					characterParticipation[conv.SpeakerID] = true
				}
			}
			characterUtilization := float64(len(characterParticipation)) / float64(len(sceneData.Characters))
			utilizationScore += characterUtilization
			factors++
		}

		if factors > 0 {
			overallUtilization := utilizationScore / float64(factors) * 100
			var utilizationLevel string
			switch {
			case overallUtilization >= 80:
				utilizationLevel = "优秀 - 场景要素充分利用"
			case overallUtilization >= 60:
				utilizationLevel = "良好 - 大部分要素得到使用"
			case overallUtilization >= 40:
				utilizationLevel = "一般 - 部分要素有待发掘"
			default:
				utilizationLevel = "待提升 - 许多场景要素未被利用"
			}
			summary.WriteString(fmt.Sprintf("- **场景完整性**: %s (%.1f%%)\n", utilizationLevel, overallUtilization))
		}

		summary.WriteString("\n")
	}

	return summary.String()
}

// formatSceneAsJSON JSON格式导出场景
func (s *ExportService) formatSceneAsJSON(
	sceneData *SceneData,
	conversations []models.Conversation,
	summary string,
	stats *models.InteractionExportStats,
	includeConversations bool) (string, error) {

	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	exportData := map[string]interface{}{
		"scene_info": map[string]interface{}{
			"id":            sceneData.Scene.ID,
			"title":         sceneData.Scene.Title,
			"description":   sceneData.Scene.Description,
			"source":        sceneData.Scene.Source,
			"themes":        sceneData.Scene.Themes,
			"era":           sceneData.Scene.Era,
			"atmosphere":    sceneData.Scene.Atmosphere,
			"created_at":    sceneData.Scene.CreatedAt,
			"last_updated":  sceneData.Scene.LastUpdated,
			"last_accessed": sceneData.Scene.LastAccessed,
		},
		"characters": sceneData.Characters,
		"locations":  sceneData.Scene.Locations,
		"items":      sceneData.Scene.Items,
		"context":    sceneData.Context,
		"settings":   sceneData.Settings,
		"summary":    summary,
		"statistics": stats,
		"export_info": map[string]interface{}{
			"generated_at":          time.Now(),
			"format":                "json",
			"export_type":           "scene",
			"include_conversations": includeConversations,
			"version":               "1.0",
		},
	}

	if includeConversations {
		exportData["conversations"] = conversations
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON序列化失败: %w", err)
	}

	return string(jsonData), nil
}

// formatSceneAsMarkdown Markdown格式导出场景
func (s *ExportService) formatSceneAsMarkdown(
	sceneData *SceneData,
	conversations []models.Conversation,
	summary string,
	stats *models.InteractionExportStats,
	includeConversations bool) (string, error) {

	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	var content strings.Builder

	// 标题和基本信息
	content.WriteString(fmt.Sprintf("# %s - 完整场景数据\n\n", sceneData.Scene.Title))

	// ✅ 使用 stats 添加统计概览
	content.WriteString("## 📊 场景统计概览\n\n")
	content.WriteString(fmt.Sprintf("- **角色数量**: %d 位\n", stats.CharacterCount))
	content.WriteString(fmt.Sprintf("- **地点数量**: %d 个\n", len(sceneData.Scene.Locations)))
	content.WriteString(fmt.Sprintf("- **道具数量**: %d 个\n", len(sceneData.Scene.Items)))
	content.WriteString(fmt.Sprintf("- **总消息数**: %d 条\n", stats.TotalMessages))
	content.WriteString(fmt.Sprintf("- **交互次数**: %d 次\n", stats.TotalInteractions))

	// 时间范围统计
	if !stats.DateRange.StartDate.IsZero() && !stats.DateRange.EndDate.IsZero() {
		duration := stats.DateRange.EndDate.Sub(stats.DateRange.StartDate)
		content.WriteString(fmt.Sprintf("- **活动时间跨度**: %s 至 %s\n",
			stats.DateRange.StartDate.Format("2006-01-02 15:04"),
			stats.DateRange.EndDate.Format("2006-01-02 15:04")))
		content.WriteString(fmt.Sprintf("- **总活动时长**: %.1f 小时\n", duration.Hours()))
	}

	// 互动效率
	if stats.TotalInteractions > 0 {
		avgMessages := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
		content.WriteString(fmt.Sprintf("- **平均互动深度**: %.1f 消息/交互\n", avgMessages))
	}

	content.WriteString("\n")

	// ✅ 使用 stats 添加情绪分布分析
	if len(stats.EmotionDistribution) > 0 {
		content.WriteString("### 情绪分布分析\n\n")
		content.WriteString("| 情绪类型 | 出现次数 | 占比 |\n")
		content.WriteString("|---------|----------|------|\n")

		// 按出现次数排序
		type emotionStat struct {
			emotion string
			count   int
		}

		var emotionStats []emotionStat
		totalEmotions := 0
		for emotion, count := range stats.EmotionDistribution {
			emotionStats = append(emotionStats, emotionStat{emotion, count})
			totalEmotions += count
		}

		sort.Slice(emotionStats, func(i, j int) bool {
			return emotionStats[i].count > emotionStats[j].count
		})

		for _, stat := range emotionStats {
			percentage := float64(stat.count) / float64(totalEmotions) * 100
			content.WriteString(fmt.Sprintf("| %s | %d | %.1f%% |\n",
				stat.emotion, stat.count, percentage))
		}

		content.WriteString("\n")
	}

	// ✅ 使用 stats 添加关键词分析
	if len(stats.TopKeywords) > 0 {
		content.WriteString("### 热门关键词\n\n")
		content.WriteString("对话中出现频率最高的关键词：\n\n")

		for i, keyword := range stats.TopKeywords {
			if i >= 15 { // 限制显示数量
				break
			}

			var marker string
			switch {
			case i < 3:
				marker = "🥇🥈🥉"[i*3 : i*3+3] // 前三名使用奖牌
			case i < 5:
				marker = "⭐"
			default:
				marker = "▶️"
			}

			content.WriteString(fmt.Sprintf("%s **%s**\n", marker, keyword))
		}

		content.WriteString("\n")
	}

	// 场景摘要
	content.WriteString(summary)
	content.WriteString("\n")

	// 角色详细信息
	content.WriteString("## 👥 角色详细信息\n\n")
	for i, char := range sceneData.Characters {
		content.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, char.Name))
		content.WriteString(fmt.Sprintf("- **角色ID**: %s\n", char.ID))
		content.WriteString(fmt.Sprintf("- **描述**: %s\n", char.Description))

		if char.Role != "" {
			content.WriteString(fmt.Sprintf("- **角色定位**: %s\n", char.Role))
		}
		if char.Personality != "" {
			content.WriteString(fmt.Sprintf("- **性格特征**: %s\n", char.Personality))
		}
		if char.Background != "" {
			content.WriteString(fmt.Sprintf("- **背景故事**: %s\n", char.Background))
		}
		if char.SpeechStyle != "" {
			content.WriteString(fmt.Sprintf("- **说话风格**: %s\n", char.SpeechStyle))
		}
		if char.Novel != "" {
			content.WriteString(fmt.Sprintf("- **出处作品**: %s\n", char.Novel))
		}

		if len(char.Knowledge) > 0 {
			content.WriteString(fmt.Sprintf("- **知识领域**: %s\n", strings.Join(char.Knowledge, ", ")))
		}

		if len(char.Relationships) > 0 {
			content.WriteString("- **人际关系**:\n")
			for otherID, relationship := range char.Relationships {
				// 查找对应角色名称
				var otherName string
				for _, otherChar := range sceneData.Characters {
					if otherChar.ID == otherID {
						otherName = otherChar.Name
						break
					}
				}
				if otherName != "" {
					content.WriteString(fmt.Sprintf("  - 与 **%s**: %s\n", otherName, relationship))
				} else {
					content.WriteString(fmt.Sprintf("  - 与 %s: %s\n", otherID, relationship))
				}
			}
		}

		content.WriteString(fmt.Sprintf("- **创建时间**: %s\n", char.CreatedAt.Format("2006-01-02 15:04")))
		if !char.LastUpdated.IsZero() {
			content.WriteString(fmt.Sprintf("- **最后更新**: %s\n", char.LastUpdated.Format("2006-01-02 15:04")))
		}
		content.WriteString("\n")
	}

	// 地点信息
	if len(sceneData.Scene.Locations) > 0 {
		content.WriteString("## 🗺️ 地点信息\n\n")
		for i, location := range sceneData.Scene.Locations {
			content.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, location.Name))
			content.WriteString(fmt.Sprintf("**描述**: %s\n\n", location.Description))
		}
	}

	// 道具信息
	if len(sceneData.Scene.Items) > 0 {
		content.WriteString("## 🎒 道具信息\n\n")
		for i, item := range sceneData.Scene.Items {
			content.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, item.Name))
			content.WriteString(fmt.Sprintf("**描述**: %s\n\n", item.Description))

			if item.Type != "" {
				content.WriteString(fmt.Sprintf("**类型**: %s\n", item.Type))
			}

			if item.Location != "" {
				content.WriteString(fmt.Sprintf("**位置**: %s\n", item.Location))
			}

			// 使用 IsOwned 替代 Owner 字段
			if item.IsOwned {
				content.WriteString("**状态**: ✅ 已拥有\n")
			} else {
				content.WriteString("**状态**: ⭕ 未拥有\n")
			}

			// 显示图片信息（如果有）
			if item.ImageURL != "" {
				content.WriteString(fmt.Sprintf("**图片**: ![%s](%s)\n", item.Name, item.ImageURL))
			}

			// 显示可用对象（如果有）
			if len(item.UsableWith) > 0 {
				content.WriteString(fmt.Sprintf("**可配合使用**: %s\n", strings.Join(item.UsableWith, ", ")))
			}

			// 显示发现时间（如果有）
			if !item.FoundAt.IsZero() {
				content.WriteString(fmt.Sprintf("**发现时间**: %s\n", item.FoundAt.Format("2006-01-02 15:04")))
			}

			// 显示额外属性（如果有）
			if len(item.Properties) > 0 {
				content.WriteString("**额外属性**:\n")
				for key, value := range item.Properties {
					content.WriteString(fmt.Sprintf("  - %s: %v\n", key, value))
				}
			}

			content.WriteString("\n")
		}
	}

	// 场景设置
	content.WriteString("## ⚙️ 场景设置\n\n")

	// 基础设置
	content.WriteString("### 基础设置\n\n")
	content.WriteString(fmt.Sprintf("- **允许自由聊天**: %t\n", sceneData.Settings.AllowFreeChat))
	content.WriteString(fmt.Sprintf("- **允许剧情转折**: %t\n", sceneData.Settings.AllowPlotTwists))

	// 互动配置
	content.WriteString("\n### 互动配置\n\n")
	content.WriteString(fmt.Sprintf("- **互动风格**: %s\n", sceneData.Settings.InteractionStyle))
	content.WriteString(fmt.Sprintf("- **创意程度**: %.1f/1.0", sceneData.Settings.CreativityLevel))

	// 添加创意等级描述
	var creativityLevel string
	switch {
	case sceneData.Settings.CreativityLevel >= 0.8:
		creativityLevel = " (极高创意)"
	case sceneData.Settings.CreativityLevel >= 0.6:
		creativityLevel = " (高创意)"
	case sceneData.Settings.CreativityLevel >= 0.4:
		creativityLevel = " (中等创意)"
	case sceneData.Settings.CreativityLevel >= 0.2:
		creativityLevel = " (低创意)"
	default:
		creativityLevel = " (保守模式)"
	}
	content.WriteString(creativityLevel + "\n")

	// 回复配置
	content.WriteString("\n### 回复配置\n\n")
	content.WriteString(fmt.Sprintf("- **回复长度**: %s\n", sceneData.Settings.ResponseLength))
	content.WriteString(fmt.Sprintf("- **语言复杂度**: %s\n", sceneData.Settings.LanguageComplexity))

	// 系统信息
	content.WriteString("\n### 系统信息\n\n")
	content.WriteString(fmt.Sprintf("- **场景ID**: %s\n", sceneData.Settings.SceneID))
	content.WriteString(fmt.Sprintf("- **最后更新**: %s\n", sceneData.Settings.LastUpdated.Format("2006-01-02 15:04:05")))

	// 推断的内容策略
	content.WriteString("\n### 内容策略\n\n")
	if sceneData.Settings.AllowFreeChat && sceneData.Settings.AllowPlotTwists {
		content.WriteString("- **内容策略**: 开放自由 - 允许自由对话和剧情发展\n")
	} else if sceneData.Settings.AllowFreeChat {
		content.WriteString("- **内容策略**: 半开放 - 允许自由对话但剧情受限\n")
	} else if sceneData.Settings.AllowPlotTwists {
		content.WriteString("- **内容策略**: 剧情导向 - 限制自由对话但允许剧情转折\n")
	} else {
		content.WriteString("- **内容策略**: 严格控制 - 限制自由对话和剧情转折\n")
	}

	content.WriteString("\n")

	// 场景上下文
	if len(sceneData.Context.Conversations) > 0 {
		content.WriteString("## 📋 场景上下文\n\n")

		content.WriteString(fmt.Sprintf("- **场景ID**: %s\n", sceneData.Context.SceneID))
		content.WriteString(fmt.Sprintf("- **对话记录**: %d 条\n", len(sceneData.Context.Conversations)))
		content.WriteString(fmt.Sprintf("- **最后更新**: %s\n", sceneData.Context.LastUpdated.Format("2006-01-02 15:04:05")))

		// 分析对话参与者
		speakers := make(map[string]int)
		for _, conv := range sceneData.Context.Conversations {
			if conv.Speaker != "" {
				speakers[conv.Speaker]++
			}
		}

		content.WriteString(fmt.Sprintf("- **活跃参与者**: %d 位\n", len(speakers)))

		// 显示最活跃的参与者
		type speakerStat struct {
			name  string
			count int
		}

		var speakerList []speakerStat
		for speaker, count := range speakers {
			speakerList = append(speakerList, speakerStat{speaker, count})
		}

		sort.Slice(speakerList, func(i, j int) bool {
			return speakerList[i].count > speakerList[j].count
		})

		if len(speakerList) > 0 {
			topSpeakers := make([]string, 0, 3)
			for i, speaker := range speakerList {
				if i >= 3 {
					break
				}
				topSpeakers = append(topSpeakers, fmt.Sprintf("%s (%d)", speaker.name, speaker.count))
			}
			content.WriteString(fmt.Sprintf("- **主要发言者**: %s\n", strings.Join(topSpeakers, ", ")))
		}

		// 时间范围
		if len(sceneData.Context.Conversations) > 1 {
			firstTime := sceneData.Context.Conversations[0].Timestamp
			lastTime := sceneData.Context.Conversations[len(sceneData.Context.Conversations)-1].Timestamp

			if !firstTime.IsZero() && !lastTime.IsZero() {
				duration := lastTime.Sub(firstTime)
				content.WriteString(fmt.Sprintf("- **活动时间跨度**: %s 至 %s (%.1f 小时)\n",
					firstTime.Format("2006-01-02 15:04"),
					lastTime.Format("2006-01-02 15:04"),
					duration.Hours()))
			}
		}

		content.WriteString("\n")
	}

	// ✅ 使用 stats 添加场景活跃度分析
	if stats.TotalMessages > 0 {
		content.WriteString("## 📈 场景活跃度分析\n\n")

		// 活跃度评级
		var activityLevel string
		if stats.TotalInteractions > 0 {
			avgMessages := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
			switch {
			case avgMessages >= 8:
				activityLevel = "极高 - 深度互动，内容丰富"
			case avgMessages >= 5:
				activityLevel = "高 - 活跃互动，参与度好"
			case avgMessages >= 3:
				activityLevel = "中等 - 正常交流水平"
			case avgMessages >= 1.5:
				activityLevel = "较低 - 互动相对简短"
			default:
				activityLevel = "低 - 需要提升互动质量"
			}
			content.WriteString(fmt.Sprintf("- **活跃度评级**: %s\n", activityLevel))
			content.WriteString(fmt.Sprintf("- **平均互动深度**: %.1f 消息/交互\n", avgMessages))
		}

		// 场景利用度评估
		utilizationFactors := 0
		utilizationScore := 0.0

		// 角色利用度
		if len(sceneData.Characters) > 0 {
			activeCharacters := 0
			for _, conv := range conversations {
				if conv.SpeakerID != "" && conv.SpeakerID != "user" {
					// 检查是否是场景中的角色
					for _, char := range sceneData.Characters {
						if char.ID == conv.SpeakerID {
							activeCharacters++
							break
						}
					}
				}
			}

			if activeCharacters > 0 {
				charUtilization := float64(activeCharacters) / float64(len(sceneData.Characters))
				utilizationScore += charUtilization
				utilizationFactors++
				content.WriteString(fmt.Sprintf("- **角色利用率**: %.1f%% (%d/%d 角色参与)\n",
					charUtilization*100, activeCharacters, len(sceneData.Characters)))
			}
		}

		// 整体利用度评估
		if utilizationFactors > 0 {
			overallUtilization := utilizationScore / float64(utilizationFactors) * 100
			var utilizationLevel string
			switch {
			case overallUtilization >= 80:
				utilizationLevel = "优秀 - 场景要素充分利用"
			case overallUtilization >= 60:
				utilizationLevel = "良好 - 大部分要素得到使用"
			case overallUtilization >= 40:
				utilizationLevel = "一般 - 部分要素有待发掘"
			default:
				utilizationLevel = "待提升 - 许多场景要素未被利用"
			}
			content.WriteString(fmt.Sprintf("- **整体利用度**: %s (%.1f%%)\n", utilizationLevel, overallUtilization))
		}

		content.WriteString("\n")
	}

	// 对话记录（如果包含）
	if includeConversations && len(conversations) > 0 {
		content.WriteString("## 💬 对话记录\n\n")

		// 按交互分组
		interactionGroups := s.groupConversationsByInteraction(conversations)

		for groupIndex, group := range interactionGroups {
			content.WriteString(fmt.Sprintf("### 📖 对话组 #%d\n\n", groupIndex+1))

			if len(group) > 0 && !group[0].Timestamp.IsZero() {
				content.WriteString(fmt.Sprintf("**时间**: %s\n", group[0].Timestamp.Format("2006-01-02 15:04:05")))
				content.WriteString(fmt.Sprintf("**消息数**: %d 条\n\n", len(group)))
			}

			for j, conv := range group {
				// 查找角色名称
				speakerName := conv.Speaker
				if conv.SpeakerID != "" && conv.SpeakerID != "user" {
					for _, char := range sceneData.Characters {
						if char.ID == conv.SpeakerID {
							speakerName = char.Name
							break
						}
					}
				}

				content.WriteString(fmt.Sprintf("**[%d]** **%s**: %s\n\n", j+1, speakerName, conv.Content))

				if len(conv.Emotions) > 0 {
					content.WriteString(fmt.Sprintf("*情绪: %s*\n\n", strings.Join(conv.Emotions, ", ")))
				}
			}

			content.WriteString("---\n\n")
		}
	}

	// 导出信息
	content.WriteString("## 📄 导出信息\n\n")
	content.WriteString(fmt.Sprintf("- **导出时间**: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString("- **导出格式**: Markdown\n")
	content.WriteString("- **导出类型**: 场景数据\n")
	content.WriteString(fmt.Sprintf("- **包含对话**: %t\n", includeConversations))
	content.WriteString("- **数据来源**: SceneIntruderMCP 场景服务\n")
	content.WriteString("- **版本**: v1.0\n")

	// ✅ 使用 stats 添加统计摘要
	content.WriteString(fmt.Sprintf("- **统计数据**: %d 条消息，%d 次交互\n",
		stats.TotalMessages, stats.TotalInteractions))
	if len(stats.EmotionDistribution) > 0 {
		content.WriteString(fmt.Sprintf("- **情绪类型**: %d 种不同情绪\n", len(stats.EmotionDistribution)))
	}
	if len(stats.TopKeywords) > 0 {
		content.WriteString(fmt.Sprintf("- **关键词数量**: %d 个热门词汇\n", len(stats.TopKeywords)))
	}

	return content.String(), nil
}

// truncateString 截断字符串到指定长度
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	// 避免在单词中间截断
	if maxLen > 3 {
		truncated := s[:maxLen-3]
		// 寻找最后一个空格
		if lastSpace := strings.LastIndex(truncated, " "); lastSpace > maxLen/2 {
			return s[:lastSpace] + "..."
		}
		return truncated + "..."
	}

	return s[:maxLen]
}

// formatSceneAsText 纯文本格式导出场景
func (s *ExportService) formatSceneAsText(
	sceneData *SceneData,
	conversations []models.Conversation,
	summary string,
	stats *models.InteractionExportStats,
	includeConversations bool) (string, error) {

	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	var content strings.Builder

	// 标题
	content.WriteString(strings.Repeat("=", 60) + "\n")
	content.WriteString(fmt.Sprintf("    %s - 完整场景数据\n", sceneData.Scene.Title))
	content.WriteString(strings.Repeat("=", 60) + "\n\n")

	// ✅ 使用 stats 添加统计概览
	content.WriteString("场景统计概览\n")
	content.WriteString(strings.Repeat("-", 30) + "\n")
	content.WriteString(fmt.Sprintf("角色数量: %d 位\n", stats.CharacterCount))
	content.WriteString(fmt.Sprintf("地点数量: %d 个\n", len(sceneData.Scene.Locations)))
	content.WriteString(fmt.Sprintf("道具数量: %d 个\n", len(sceneData.Scene.Items)))
	content.WriteString(fmt.Sprintf("总消息数: %d 条\n", stats.TotalMessages))
	content.WriteString(fmt.Sprintf("交互次数: %d 次\n", stats.TotalInteractions))

	// ✅ 使用 stats 显示时间范围
	if !stats.DateRange.StartDate.IsZero() && !stats.DateRange.EndDate.IsZero() {
		duration := stats.DateRange.EndDate.Sub(stats.DateRange.StartDate)
		content.WriteString(fmt.Sprintf("活动时间跨度: %s 至 %s\n",
			stats.DateRange.StartDate.Format("2006-01-02 15:04"),
			stats.DateRange.EndDate.Format("2006-01-02 15:04")))
		content.WriteString(fmt.Sprintf("总活动时长: %.1f 小时\n", duration.Hours()))
	}

	// ✅ 使用 stats 显示互动效率
	if stats.TotalInteractions > 0 {
		avgMessages := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
		content.WriteString(fmt.Sprintf("平均互动深度: %.1f 消息/交互\n", avgMessages))

		// 效率评级
		var efficiencyLevel string
		switch {
		case avgMessages >= 8:
			efficiencyLevel = "极高"
		case avgMessages >= 5:
			efficiencyLevel = "高"
		case avgMessages >= 3:
			efficiencyLevel = "中等"
		case avgMessages >= 1.5:
			efficiencyLevel = "较低"
		default:
			efficiencyLevel = "低"
		}
		content.WriteString(fmt.Sprintf("效率评级: %s\n", efficiencyLevel))
	}

	content.WriteString("\n")

	// ✅ 使用 stats 添加情绪分布分析
	if len(stats.EmotionDistribution) > 0 {
		content.WriteString("情绪分布分析\n")
		content.WriteString(strings.Repeat("-", 20) + "\n")

		// 按频次排序情绪
		type emotionStat struct {
			emotion string
			count   int
		}

		var emotionStats []emotionStat
		totalEmotions := 0
		for emotion, count := range stats.EmotionDistribution {
			emotionStats = append(emotionStats, emotionStat{emotion, count})
			totalEmotions += count
		}

		sort.Slice(emotionStats, func(i, j int) bool {
			return emotionStats[i].count > emotionStats[j].count
		})

		for _, stat := range emotionStats {
			percentage := float64(stat.count) / float64(totalEmotions) * 100
			// 创建简单的文本条形图
			barLength := int(percentage / 5) // 每5%一个字符
			if barLength > 20 {
				barLength = 20
			}
			bar := strings.Repeat("█", barLength) + strings.Repeat("░", 20-barLength)
			content.WriteString(fmt.Sprintf("  %s: %d次 (%.1f%%) %s\n",
				stat.emotion, stat.count, percentage, bar))
		}

		content.WriteString("\n")
	}

	// ✅ 使用 stats 添加关键词分析
	if len(stats.TopKeywords) > 0 {
		content.WriteString("热门关键词\n")
		content.WriteString(strings.Repeat("-", 20) + "\n")
		content.WriteString("对话中出现频率最高的关键词:\n")

		for i, keyword := range stats.TopKeywords {
			if i >= 15 { // 限制显示数量
				break
			}

			// 使用简单的排名标记
			var marker string
			switch {
			case i < 3:
				marker = fmt.Sprintf("[第%d名]", i+1)
			case i < 10:
				marker = fmt.Sprintf("  %d. ", i+1)
			default:
				marker = fmt.Sprintf(" %d. ", i+1)
			}

			content.WriteString(fmt.Sprintf("%s %s\n", marker, keyword))
		}

		content.WriteString("\n")
	}

	// 移除Markdown格式的摘要
	textSummary := strings.ReplaceAll(summary, "#", "")
	textSummary = strings.ReplaceAll(textSummary, "**", "")
	textSummary = strings.ReplaceAll(textSummary, "*", "")
	content.WriteString("场景分析报告\n")
	content.WriteString(strings.Repeat("-", 30) + "\n")
	content.WriteString(textSummary)
	content.WriteString("\n")

	// 角色详细信息
	content.WriteString(strings.Repeat("-", 40) + "\n")
	content.WriteString("角色详细信息\n")
	content.WriteString(strings.Repeat("-", 40) + "\n\n")

	for i, char := range sceneData.Characters {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, char.Name))
		content.WriteString(fmt.Sprintf("   角色ID: %s\n", char.ID))
		content.WriteString(fmt.Sprintf("   描述: %s\n", char.Description))

		if char.Role != "" {
			content.WriteString(fmt.Sprintf("   角色定位: %s\n", char.Role))
		}
		if char.Personality != "" {
			content.WriteString(fmt.Sprintf("   性格特征: %s\n", char.Personality))
		}
		if char.Background != "" {
			content.WriteString(fmt.Sprintf("   背景故事: %s\n", char.Background))
		}
		if char.SpeechStyle != "" {
			content.WriteString(fmt.Sprintf("   说话风格: %s\n", char.SpeechStyle))
		}

		if len(char.Knowledge) > 0 {
			content.WriteString(fmt.Sprintf("   知识领域: %s\n", strings.Join(char.Knowledge, ", ")))
		}

		if len(char.Relationships) > 0 {
			content.WriteString("   人际关系:\n")
			for otherID, relationship := range char.Relationships {
				var otherName string
				for _, otherChar := range sceneData.Characters {
					if otherChar.ID == otherID {
						otherName = otherChar.Name
						break
					}
				}
				if otherName != "" {
					content.WriteString(fmt.Sprintf("     - 与%s: %s\n", otherName, relationship))
				}
			}
		}

		content.WriteString(fmt.Sprintf("   创建时间: %s\n", char.CreatedAt.Format("2006-01-02 15:04")))
		content.WriteString("\n")
	}

	// 地点信息
	if len(sceneData.Scene.Locations) > 0 {
		content.WriteString(strings.Repeat("-", 40) + "\n")
		content.WriteString("地点信息\n")
		content.WriteString(strings.Repeat("-", 40) + "\n\n")

		for i, location := range sceneData.Scene.Locations {
			content.WriteString(fmt.Sprintf("%d. %s\n", i+1, location.Name))
			content.WriteString(fmt.Sprintf("   描述: %s\n", location.Description))
			content.WriteString("\n")
		}
	}

	// 道具信息
	if len(sceneData.Scene.Items) > 0 {
		content.WriteString(strings.Repeat("-", 40) + "\n")
		content.WriteString("道具信息\n")
		content.WriteString(strings.Repeat("-", 40) + "\n\n")

		for i, item := range sceneData.Scene.Items {
			content.WriteString(fmt.Sprintf("%d. %s\n", i+1, item.Name))
			content.WriteString(fmt.Sprintf("   描述: %s\n", item.Description))

			if item.Type != "" {
				content.WriteString(fmt.Sprintf("   类型: %s\n", item.Type))
			}
			if item.Location != "" {
				content.WriteString(fmt.Sprintf("   位置: %s\n", item.Location))
			}

			if item.IsOwned {
				content.WriteString("   状态: 已拥有\n")
			} else {
				content.WriteString("   状态: 未拥有\n")
			}

			content.WriteString("\n")
		}
	}

	// 场景设置
	content.WriteString(strings.Repeat("-", 40) + "\n")
	content.WriteString("场景设置\n")
	content.WriteString(strings.Repeat("-", 40) + "\n\n")

	content.WriteString(fmt.Sprintf("允许自由聊天: %t\n", sceneData.Settings.AllowFreeChat))
	content.WriteString(fmt.Sprintf("允许剧情转折: %t\n", sceneData.Settings.AllowPlotTwists))
	content.WriteString(fmt.Sprintf("创意程度: %.1f/1.0\n", sceneData.Settings.CreativityLevel))
	content.WriteString(fmt.Sprintf("回复长度: %s\n", sceneData.Settings.ResponseLength))
	content.WriteString(fmt.Sprintf("互动风格: %s\n", sceneData.Settings.InteractionStyle))
	content.WriteString(fmt.Sprintf("语言复杂度: %s\n", sceneData.Settings.LanguageComplexity))
	content.WriteString(fmt.Sprintf("最后更新: %s\n", sceneData.Settings.LastUpdated.Format("2006-01-02 15:04:05")))
	content.WriteString("\n")

	// 场景上下文
	if len(sceneData.Context.Conversations) > 0 {
		content.WriteString(strings.Repeat("-", 40) + "\n")
		content.WriteString("场景上下文\n")
		content.WriteString(strings.Repeat("-", 40) + "\n\n")

		content.WriteString(fmt.Sprintf("场景ID: %s\n", sceneData.Context.SceneID))
		content.WriteString(fmt.Sprintf("对话记录数量: %d 条\n", len(sceneData.Context.Conversations)))
		content.WriteString(fmt.Sprintf("最后更新: %s\n", sceneData.Context.LastUpdated.Format("2006-01-02 15:04:05")))

		// 显示最近活动
		if len(sceneData.Context.Conversations) > 0 {
			content.WriteString("\n最近活动:\n")
			recentCount := 3
			if len(sceneData.Context.Conversations) < recentCount {
				recentCount = len(sceneData.Context.Conversations)
			}

			recentConversations := sceneData.Context.Conversations[len(sceneData.Context.Conversations)-recentCount:]
			for i, conv := range recentConversations {
				content.WriteString(fmt.Sprintf("  %d. %s: %s\n",
					i+1,
					conv.Speaker,
					truncateString(conv.Content, 40)))
			}
		}

		content.WriteString("\n")
	}

	// ✅ 使用 stats 添加场景活跃度分析
	if stats.TotalMessages > 0 {
		content.WriteString(strings.Repeat("-", 40) + "\n")
		content.WriteString("场景活跃度分析\n")
		content.WriteString(strings.Repeat("-", 40) + "\n\n")

		// 活跃度评级
		if stats.TotalInteractions > 0 {
			avgMessages := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
			var activityLevel string
			switch {
			case avgMessages >= 8:
				activityLevel = "极高 - 深度互动，内容丰富"
			case avgMessages >= 5:
				activityLevel = "高 - 活跃互动，参与度好"
			case avgMessages >= 3:
				activityLevel = "中等 - 正常交流水平"
			case avgMessages >= 1.5:
				activityLevel = "较低 - 互动相对简短"
			default:
				activityLevel = "低 - 需要提升互动质量"
			}
			content.WriteString(fmt.Sprintf("活跃度评级: %s\n", activityLevel))
			content.WriteString(fmt.Sprintf("平均互动深度: %.1f 消息/交互\n", avgMessages))
		}

		// 角色参与度
		if len(conversations) > 0 {
			characterParticipation := make(map[string]int)
			for _, conv := range conversations {
				if conv.SpeakerID != "" && conv.SpeakerID != "user" {
					characterParticipation[conv.SpeakerID]++
				}
			}

			if len(characterParticipation) > 0 && len(sceneData.Characters) > 0 {
				activeCharacters := len(characterParticipation)
				charUtilization := float64(activeCharacters) / float64(len(sceneData.Characters)) * 100
				content.WriteString(fmt.Sprintf("角色利用率: %.1f%% (%d/%d 角色参与)\n",
					charUtilization, activeCharacters, len(sceneData.Characters)))

				var utilizationLevel string
				switch {
				case charUtilization >= 80:
					utilizationLevel = "优秀 - 场景要素充分利用"
				case charUtilization >= 60:
					utilizationLevel = "良好 - 大部分要素得到使用"
				case charUtilization >= 40:
					utilizationLevel = "一般 - 部分要素有待发掘"
				default:
					utilizationLevel = "待提升 - 许多场景要素未被利用"
				}
				content.WriteString(fmt.Sprintf("整体利用度: %s\n", utilizationLevel))
			}
		}

		content.WriteString("\n")
	}

	// 对话记录（如果包含）
	if includeConversations && len(conversations) > 0 {
		content.WriteString(strings.Repeat("-", 40) + "\n")
		content.WriteString("对话记录\n")
		content.WriteString(strings.Repeat("-", 40) + "\n\n")

		interactionGroups := s.groupConversationsByInteraction(conversations)

		for groupIndex, group := range interactionGroups {
			content.WriteString(fmt.Sprintf("对话组 #%d:\n", groupIndex+1))

			if len(group) > 0 && !group[0].Timestamp.IsZero() {
				content.WriteString(fmt.Sprintf("  时间: %s\n", group[0].Timestamp.Format("2006-01-02 15:04:05")))
				content.WriteString(fmt.Sprintf("  消息数: %d 条\n", len(group)))
			}

			for j, conv := range group {
				speakerName := conv.Speaker
				if conv.SpeakerID != "" && conv.SpeakerID != "user" {
					for _, char := range sceneData.Characters {
						if char.ID == conv.SpeakerID {
							speakerName = char.Name
							break
						}
					}
				}

				content.WriteString(fmt.Sprintf("  [%d] %s: %s\n", j+1, speakerName, conv.Content))

				if len(conv.Emotions) > 0 {
					content.WriteString(fmt.Sprintf("      [情绪: %s]\n", strings.Join(conv.Emotions, ", ")))
				}
			}
			content.WriteString("\n")
		}
	}

	// 导出信息
	content.WriteString(strings.Repeat("=", 60) + "\n")
	content.WriteString("导出信息\n")
	content.WriteString(strings.Repeat("=", 60) + "\n")
	content.WriteString(fmt.Sprintf("导出时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString("导出格式: 纯文本\n")
	content.WriteString("导出类型: 场景数据\n")
	content.WriteString(fmt.Sprintf("包含对话: %t\n", includeConversations))
	content.WriteString("数据来源: SceneIntruderMCP 场景服务\n")
	content.WriteString("版本: v1.0\n")

	// ✅ 使用 stats 添加统计摘要
	content.WriteString(fmt.Sprintf("统计数据: %d 条消息，%d 次交互\n",
		stats.TotalMessages, stats.TotalInteractions))
	if len(stats.EmotionDistribution) > 0 {
		content.WriteString(fmt.Sprintf("情绪类型: %d 种不同情绪\n", len(stats.EmotionDistribution)))
	}
	if len(stats.TopKeywords) > 0 {
		content.WriteString(fmt.Sprintf("关键词数量: %d 个热门词汇\n", len(stats.TopKeywords)))
	}

	return content.String(), nil
}

// formatSceneAsHTML HTML格式导出场景
func (s *ExportService) formatSceneAsHTML(
	sceneData *SceneData,
	conversations []models.Conversation,
	summary string,
	stats *models.InteractionExportStats,
	includeConversations bool) (string, error) {

	if sceneData == nil {
		return "", fmt.Errorf("场景数据不能为空")
	}

	var content strings.Builder

	// HTML 头部
	content.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>`)
	content.WriteString(sceneData.Scene.Title + " - 场景数据")
	content.WriteString(`</title>
    <style>
        body { 
            font-family: 'Microsoft YaHei', Arial, sans-serif; 
            margin: 20px; 
            line-height: 1.6; 
            color: #333;
            background-color: #f5f5f5;
        }
        .container { 
            max-width: 1200px; 
            margin: 0 auto; 
            background: white; 
            padding: 30px; 
            border-radius: 10px; 
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .header { 
            text-align: center; 
            background: linear-gradient(135deg, #28a745 0%, #20c997 100%); 
            color: white; 
            padding: 30px; 
            margin: -30px -30px 30px -30px; 
            border-radius: 10px 10px 0 0;
        }
        .header h1 { margin: 0; font-size: 2.5em; }
        .section { margin-bottom: 30px; }
        .section h2 { 
            color: #2c3e50; 
            border-bottom: 2px solid #28a745; 
            padding-bottom: 10px; 
        }
        .summary-section {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            border-left: 4px solid #6f42c1;
            margin: 20px 0;
        }
        .summary-content {
            font-size: 1.05em;
            line-height: 1.7;
        }
        .character-card, .location-card, .item-card { 
            background: #f8f9fa; 
            padding: 20px; 
            margin: 15px 0; 
            border-radius: 8px; 
            border-left: 4px solid #28a745;
        }
        .location-card { border-left-color: #17a2b8; }
        .item-card { border-left-color: #ffc107; }
        .stats-grid { 
            display: grid; 
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); 
            gap: 15px; 
            margin: 20px 0; 
        }
        .stat-card { 
            background: #f8f9fa; 
            padding: 15px; 
            text-align: center; 
            border-radius: 8px; 
            border: 1px solid #dee2e6;
        }
        .stat-number { font-size: 2em; font-weight: bold; color: #28a745; }
        .stat-label { color: #6c757d; font-size: 0.9em; }
        .progress-bar { 
            background: #e9ecef; 
            height: 20px; 
            border-radius: 10px; 
            overflow: hidden; 
            margin: 10px 0;
        }
        .progress-fill { 
            height: 100%; 
            background: linear-gradient(90deg, #28a745, #20c997); 
            transition: width 0.3s ease;
        }
        .conversation-group { 
            background: #fff; 
            padding: 20px; 
            margin: 15px 0; 
            border-radius: 8px; 
            border-left: 4px solid #6f42c1;
        }
        .message { 
            background: #f8f9fa; 
            padding: 10px 15px; 
            margin: 5px 0; 
            border-radius: 5px; 
        }
        .message.user { background: #e3f2fd; }
        .message.character { background: #e8f5e8; }
        .speaker { font-weight: bold; margin-bottom: 5px; }
        .speaker.user { color: #1976d2; }
        .speaker.character { color: #388e3c; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>`)
	content.WriteString(sceneData.Scene.Title)
	content.WriteString(`</h1>
            <p>完整场景数据</p>
        </div>`)

	// 统计概览
	content.WriteString(`<div class="section">
            <h2>📊 数据概览</h2>
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", stats.CharacterCount))
	content.WriteString(`</div>
                    <div class="stat-label">角色数量</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", len(sceneData.Scene.Locations)))
	content.WriteString(`</div>
                    <div class="stat-label">地点数量</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">`)
	content.WriteString(fmt.Sprintf("%d", len(sceneData.Scene.Items)))
	content.WriteString(`</div>
                    <div class="stat-label">道具数量</div>
                </div>`)

	if includeConversations {
		content.WriteString(`<div class="stat-card">
                    <div class="stat-number">`)
		content.WriteString(fmt.Sprintf("%d", stats.TotalMessages))
		content.WriteString(`</div>
                    <div class="stat-label">对话数量</div>
                </div>`)
	}

	// 添加交互次数统计
	if stats.TotalInteractions > 0 {
		content.WriteString(`<div class="stat-card">
                    <div class="stat-number">`)
		content.WriteString(fmt.Sprintf("%d", stats.TotalInteractions))
		content.WriteString(`</div>
                    <div class="stat-label">交互次数</div>
                </div>`)
	}

	// 添加平均互动深度
	if stats.TotalInteractions > 0 && stats.TotalMessages > 0 {
		avgMessages := float64(stats.TotalMessages) / float64(stats.TotalInteractions)
		content.WriteString(`<div class="stat-card">
                    <div class="stat-number">`)
		content.WriteString(fmt.Sprintf("%.1f", avgMessages))
		content.WriteString(`</div>
                    <div class="stat-label">平均消息/交互</div>
                </div>`)
	}

	content.WriteString(`</div>
        </div>`)

	// ✅ 使用 summary 参数添加场景分析报告
	content.WriteString(`<div class="section">
            <h2>📝 场景分析报告</h2>
            <div class="summary-section">
                <div class="summary-content">`)

	// 将 Markdown 格式的摘要转换为 HTML
	htmlSummary := strings.ReplaceAll(summary, "### ", "<h3>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "## ", "<h2>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "**", "<strong>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "*", "</strong>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "\n- ", "<li>")
	htmlSummary = strings.ReplaceAll(htmlSummary, "\n\n", "</p><p>")

	// 处理列表格式
	htmlSummary = strings.ReplaceAll(htmlSummary, "<li>", "</p><ul><li>")

	// 包装在段落中
	if !strings.HasPrefix(htmlSummary, "<h2>") {
		htmlSummary = "<p>" + htmlSummary
	}
	if !strings.HasSuffix(htmlSummary, "</p>") && !strings.HasSuffix(htmlSummary, "</ul>") {
		htmlSummary = htmlSummary + "</p>"
	}

	content.WriteString(htmlSummary)
	content.WriteString(`</div>
            </div>
        </div>`)

	// 场景设置
	content.WriteString(`<div class="section">
            <h2>⚙️ 场景设置</h2>
            <div class="character-card">
                <p><strong>允许自由聊天:</strong> `)
	if sceneData.Settings.AllowFreeChat {
		content.WriteString(`<span style="color: #28a745;">✅ 是</span>`)
	} else {
		content.WriteString(`<span style="color: #dc3545;">❌ 否</span>`)
	}
	content.WriteString(`</p>
                <p><strong>允许剧情转折:</strong> `)
	if sceneData.Settings.AllowPlotTwists {
		content.WriteString(`<span style="color: #28a745;">✅ 是</span>`)
	} else {
		content.WriteString(`<span style="color: #dc3545;">❌ 否</span>`)
	}
	content.WriteString(`</p>
                <p><strong>创意程度:</strong> `)
	content.WriteString(fmt.Sprintf("%.1f/1.0", sceneData.Settings.CreativityLevel))

	// 添加创意程度的进度条
	creativityPercent := sceneData.Settings.CreativityLevel * 100
	content.WriteString(`</p>
                <div class="progress-bar" style="margin: 10px 0;">
                    <div class="progress-fill" style="width: `)
	content.WriteString(fmt.Sprintf("%.1f%%", creativityPercent))
	content.WriteString(`; background: linear-gradient(90deg, #ffc107, #fd7e14);"></div>
                </div>
                <p><strong>回复长度:</strong> `)

	// 回复长度的中文翻译
	var lengthText string
	switch sceneData.Settings.ResponseLength {
	case "short":
		lengthText = "简短"
	case "medium":
		lengthText = "中等"
	case "long":
		lengthText = "详细"
	default:
		lengthText = sceneData.Settings.ResponseLength
	}
	content.WriteString(lengthText)

	content.WriteString(`</p>
                <p><strong>互动风格:</strong> `)

	// 互动风格的中文翻译
	var styleText string
	switch sceneData.Settings.InteractionStyle {
	case "casual":
		styleText = "轻松休闲"
	case "formal":
		styleText = "正式严肃"
	case "dramatic":
		styleText = "戏剧化"
	default:
		styleText = sceneData.Settings.InteractionStyle
	}
	content.WriteString(styleText)

	content.WriteString(`</p>
                <p><strong>语言复杂度:</strong> `)

	// 语言复杂度的中文翻译
	var complexityText string
	switch sceneData.Settings.LanguageComplexity {
	case "simple":
		complexityText = "简单"
	case "normal":
		complexityText = "正常"
	case "complex":
		complexityText = "复杂"
	default:
		complexityText = sceneData.Settings.LanguageComplexity
	}
	content.WriteString(complexityText)

	content.WriteString(`</p>
                <p><strong>最后更新:</strong> `)
	content.WriteString(sceneData.Settings.LastUpdated.Format("2006-01-02 15:04:05"))
	content.WriteString(`</p>
            </div>
        </div>`)

	// 角色信息
	content.WriteString(`<div class="section">
            <h2>👥 角色信息</h2>`)

	for i, char := range sceneData.Characters {
		content.WriteString(`<div class="character-card">
                <h3>`)
		content.WriteString(fmt.Sprintf("%d. %s", i+1, char.Name))
		content.WriteString(`</h3>
                <p><strong>描述:</strong> `)
		content.WriteString(char.Description)
		content.WriteString(`</p>`)

		if char.Role != "" {
			content.WriteString(`<p><strong>角色:</strong> `)
			content.WriteString(char.Role)
			content.WriteString(`</p>`)
		}

		if char.Personality != "" {
			content.WriteString(`<p><strong>性格:</strong> `)
			content.WriteString(char.Personality)
			content.WriteString(`</p>`)
		}

		if char.Background != "" {
			content.WriteString(`<p><strong>背景:</strong> `)
			content.WriteString(char.Background)
			content.WriteString(`</p>`)
		}

		if len(char.Knowledge) > 0 {
			content.WriteString(`<p><strong>知识领域:</strong> `)
			content.WriteString(strings.Join(char.Knowledge, ", "))
			content.WriteString(`</p>`)
		}

		if len(char.Relationships) > 0 {
			content.WriteString(`<p><strong>人际关系:</strong></p><ul>`)
			for otherID, relationship := range char.Relationships {
				// 查找对应角色名称
				var otherName string
				for _, otherChar := range sceneData.Characters {
					if otherChar.ID == otherID {
						otherName = otherChar.Name
						break
					}
				}
				if otherName != "" {
					content.WriteString(fmt.Sprintf(`<li>与 <strong>%s</strong>: %s</li>`, otherName, relationship))
				} else {
					content.WriteString(fmt.Sprintf(`<li>与 %s: %s</li>`, otherID, relationship))
				}
			}
			content.WriteString(`</ul>`)
		}

		content.WriteString(`<p><strong>创建时间:</strong> `)
		content.WriteString(char.CreatedAt.Format("2006-01-02 15:04"))
		content.WriteString(`</p>`)

		content.WriteString(`</div>`)
	}

	content.WriteString(`</div>`)

	// 地点信息
	if len(sceneData.Scene.Locations) > 0 {
		content.WriteString(`<div class="section">
                <h2>🗺️ 地点信息</h2>`)

		for i, location := range sceneData.Scene.Locations {
			content.WriteString(`<div class="location-card">
                    <h3>`)
			content.WriteString(fmt.Sprintf("%d. %s", i+1, location.Name))
			content.WriteString(`</h3>
                    <p><strong>描述:</strong> `)
			content.WriteString(location.Description)
			content.WriteString(`</p>`)

			content.WriteString(`</div>`)
		}

		content.WriteString(`</div>`)
	}

	// 道具信息
	if len(sceneData.Scene.Items) > 0 {
		content.WriteString(`<div class="section">
            <h2>🎒 道具信息</h2>`)

		for i, item := range sceneData.Scene.Items {
			content.WriteString(`<div class="item-card">
                <h3>`)
			content.WriteString(fmt.Sprintf("%d. %s", i+1, item.Name))
			content.WriteString(`</h3>
                <p><strong>描述:</strong> `)
			content.WriteString(item.Description)
			content.WriteString(`</p>`)

			if item.Type != "" {
				content.WriteString(`<p><strong>类型:</strong> `)
				content.WriteString(item.Type)
				content.WriteString(`</p>`)
			}

			if item.Location != "" {
				content.WriteString(`<p><strong>位置:</strong> `)
				content.WriteString(item.Location)
				content.WriteString(`</p>`)
			}

			content.WriteString(`<p><strong>状态:</strong> `)
			if item.IsOwned {
				content.WriteString(`<span style="color: #28a745;">✅ 已拥有</span>`)
			} else {
				content.WriteString(`<span style="color: #ffc107;">⭕ 未拥有</span>`)
			}
			content.WriteString(`</p>`)

			// 显示额外属性（如果有）
			if len(item.Properties) > 0 {
				content.WriteString(`<p><strong>额外属性:</strong></p><ul>`)
				for key, value := range item.Properties {
					content.WriteString(fmt.Sprintf(`<li>%s: %v</li>`, key, value))
				}
				content.WriteString(`</ul>`)
			}

			content.WriteString(`</div>`)
		}

		content.WriteString(`</div>`)
	}

	// 场景上下文
	if len(sceneData.Context.Conversations) > 0 {
		content.WriteString(`<div class="section">
        <h2>📋 场景上下文</h2>
        <div class="character-card">
            <p><strong>场景ID:</strong> `)
		content.WriteString(sceneData.Context.SceneID)
		content.WriteString(`</p>
            <p><strong>对话记录数量:</strong> `)
		content.WriteString(fmt.Sprintf("%d 条", len(sceneData.Context.Conversations)))
		content.WriteString(`</p>
            <p><strong>最后更新:</strong> `)
		content.WriteString(sceneData.Context.LastUpdated.Format("2006-01-02 15:04:05"))
		content.WriteString(`</p>`)

		// 显示最近活动
		if len(sceneData.Context.Conversations) > 0 {
			content.WriteString(`<h4>最近活动</h4>
            <div style="background: #f8f9fa; padding: 15px; border-radius: 5px; margin: 10px 0;">`)

			recentCount := 5
			if len(sceneData.Context.Conversations) < recentCount {
				recentCount = len(sceneData.Context.Conversations)
			}

			recentConversations := sceneData.Context.Conversations[len(sceneData.Context.Conversations)-recentCount:]
			for _, conv := range recentConversations {
				timeStr := ""
				if !conv.Timestamp.IsZero() {
					timeStr = fmt.Sprintf(`<span style="color: #6c757d; font-size: 0.9em;">[%s]</span> `,
						conv.Timestamp.Format("15:04"))
				}

				content.WriteString(fmt.Sprintf(`<p style="margin: 5px 0;">%s<strong>%s:</strong> %s</p>`,
					timeStr,
					conv.Speaker,
					truncateString(conv.Content, 80)))
			}

			content.WriteString(`</div>`)
		}

		content.WriteString(`</div>
    </div>`)
	}

	// 对话记录（如果包含）
	if includeConversations && len(conversations) > 0 {
		content.WriteString(`<div class="section">
                <h2>💬 对话记录</h2>`)

		interactionGroups := s.groupConversationsByInteraction(conversations)

		for groupIndex, group := range interactionGroups {
			content.WriteString(`<div class="conversation-group">
                    <h3>`)
			content.WriteString(fmt.Sprintf("对话组 #%d", groupIndex+1))
			content.WriteString(`</h3>`)

			if len(group) > 0 && !group[0].Timestamp.IsZero() {
				content.WriteString(`<p><strong>开始时间:</strong> `)
				content.WriteString(group[0].Timestamp.Format("2006-01-02 15:04:05"))
				content.WriteString(`</p>
                    <p><strong>消息数量:</strong> `)
				content.WriteString(fmt.Sprintf("%d 条", len(group)))
				content.WriteString(`</p>`)
			}

			for _, conv := range group {
				isUser := (conv.Speaker == "user" || conv.Speaker == "用户")
				if conv.Metadata != nil {
					if speakerType, exists := conv.Metadata["speaker_type"]; exists {
						isUser = (speakerType == "user")
					}
				}

				var messageClass, speakerClass string
				if isUser {
					messageClass = "message user"
					speakerClass = "speaker user"
				} else {
					messageClass = "message character"
					speakerClass = "speaker character"
				}

				speakerName := conv.Speaker
				if conv.SpeakerID != "" && conv.SpeakerID != "user" {
					for _, char := range sceneData.Characters {
						if char.ID == conv.SpeakerID {
							speakerName = char.Name
							break
						}
					}
				}

				content.WriteString(fmt.Sprintf(`<div class="%s">`, messageClass))
				content.WriteString(fmt.Sprintf(`<div class="%s">%s</div>`, speakerClass, speakerName))

				// 添加时间戳
				if !conv.Timestamp.IsZero() {
					content.WriteString(fmt.Sprintf(`<div style="color: #6c757d; font-size: 0.9em; margin-bottom: 5px;">%s</div>`,
						conv.Timestamp.Format("2006-01-02 15:04:05")))
				}

				content.WriteString(fmt.Sprintf(`<div>%s</div>`, conv.Content))

				// 添加情绪信息
				if len(conv.Emotions) > 0 {
					content.WriteString(`<div style="color: #e67e22; font-style: italic; margin-top: 5px;">🎭 `)
					content.WriteString(strings.Join(conv.Emotions, ", "))
					content.WriteString(`</div>`)
				}

				content.WriteString(`</div>`)
			}

			content.WriteString(`</div>`)
		}

		content.WriteString(`</div>`)
	}

	// 导出信息
	content.WriteString(`<div class="section">
            <h2>📄 导出信息</h2>
            <div class="character-card">
                <p><strong>导出时间:</strong> `)
	content.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	content.WriteString(`</p>
                <p><strong>导出格式:</strong> HTML</p>
                <p><strong>导出类型:</strong> 场景数据</p>
                <p><strong>包含对话:</strong> `)
	content.WriteString(fmt.Sprintf("%t", includeConversations))
	content.WriteString(`</p>
                <p><strong>数据来源:</strong> SceneIntruderMCP 场景服务</p>
                <p><strong>版本:</strong> v1.0</p>
                <p><strong>统计数据:</strong> `)
	content.WriteString(fmt.Sprintf("%d 条消息，%d 次交互", stats.TotalMessages, stats.TotalInteractions))
	content.WriteString(`</p>
            </div>
        </div>

    </div>
</body>
</html>`)

	return content.String(), nil
}

// saveSceneExportToDataDir 保存场景导出文件到data目录
func (s *ExportService) saveSceneExportToDataDir(result *models.ExportResult) (string, int64, error) {
	// 创建导出目录
	exportDir := filepath.Join("data", "exports", "scenes")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return "", 0, fmt.Errorf("创建导出目录失败: %w", err)
	}

	// 生成文件名
	timestamp := result.GeneratedAt.Format("20060102_150405")
	fileName := fmt.Sprintf("%s_scene_data_%s.%s",
		result.SceneID, timestamp, result.Format)

	filePath := filepath.Join(exportDir, fileName)

	// 写入文件
	if err := os.WriteFile(filePath, []byte(result.Content), 0644); err != nil {
		return "", 0, fmt.Errorf("写入导出文件失败: %w", err)
	}

	// 获取文件大小
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("获取文件信息失败: %w", err)
	}

	return filePath, fileInfo.Size(), nil
}
