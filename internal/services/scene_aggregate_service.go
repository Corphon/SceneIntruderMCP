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

	return &SceneAggregateService{
		SceneService:     sceneService,
		CharacterService: characterService,
		ContextService:   contextService,
		StoryService:     storyService,
		ProgressService:  progressService,
	}
}

// GetSceneAggregate 获取完整的场景聚合数据
func (s *SceneAggregateService) GetSceneAggregate(ctx context.Context, sceneID string, options *AggregateOptions) (*SceneAggregateData, error) {
	// 输入验证
	if sceneID == "" || strings.TrimSpace(sceneID) == "" {
		return nil, fmt.Errorf("场景ID不能为空")
	}

	// 服务可用性检查
	if s.SceneService == nil {
		return nil, fmt.Errorf("SceneService 未初始化")
	}
	if s.StoryService == nil {
		return nil, fmt.Errorf("StoryService 未初始化")
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

	// 使用goroutine并行获取数据
	var (
		scene         *models.Scene
		characters    []*models.Character
		storyData     *models.StoryData
		conversations []models.Conversation
		//progress      *SceneProgress

		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	// 并行获取基础数据
	wg.Add(2)

	// 获取场景和角色信息
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("获取场景数据时发生panic: %v", r))
				mu.Unlock()
			}
		}()

		if s.SceneService == nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("SceneService 为 nil"))
			mu.Unlock()
			return
		}

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

	// 获取故事数据
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("获取故事数据时发生panic: %v", r))
				mu.Unlock()
			}
		}()

		if !options.IncludeStoryData {
			return
		}

		if s.StoryService == nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("StoryService 为 nil"))
			mu.Unlock()
			return
		}

		story, err := s.StoryService.GetStoryData(sceneID, nil)
		if err != nil {
			return
		}
		storyData = story
	}()

	wg.Wait()

	// 检查是否有致命错误
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

	// 获取对话历史（串行执行，依赖上下文服务）
	if options.IncludeConversations && s.ContextService != nil {
		convs, err := s.ContextService.GetRecentConversations(sceneID, options.ConversationLimit)
		if err != nil {
			// 对话获取失败不是致命错误，记录日志但继续执行
			// 可以选择设置为空数组或者添加到非致命错误列表
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

	// ✅ 利用 scene 信息增强进度计算

	// 1. 基于场景地点计算探索进度
	if scene != nil && len(scene.Locations) > 0 {
		// 检查对话中提到的地点
		exploredLocations := make(map[string]bool)
		for _, conv := range conversations {
			messageContent := strings.ToLower(conv.Content)
			for _, location := range scene.Locations {
				locationName := strings.ToLower(location.Name)
				if strings.Contains(messageContent, locationName) {
					exploredLocations[location.Name] = true
				}
			}
		}

		// 添加探索地点到解锁内容
		for locationName := range exploredLocations {
			progress.UnlockedContent = append(progress.UnlockedContent, fmt.Sprintf("探索了%s", locationName))
		}

		// 基于地点探索度添加成就
		explorationRate := float64(len(exploredLocations)) / float64(len(scene.Locations))
		if explorationRate >= 0.5 {
			progress.Achievements = append(progress.Achievements, "location_explorer")
		}
		if explorationRate >= 1.0 {
			progress.Achievements = append(progress.Achievements, "master_explorer")
		}
	}

	// 2. 基于场景道具计算收集进度
	if scene != nil && len(scene.Items) > 0 {
		foundItems := make(map[string]bool)
		for _, conv := range conversations {
			messageContent := strings.ToLower(conv.Content)
			for _, item := range scene.Items {
				itemName := strings.ToLower(item.Name)
				// 检查是否提到了获得、找到、发现等关键词 + 道具名
				if (strings.Contains(messageContent, "获得") ||
					strings.Contains(messageContent, "找到") ||
					strings.Contains(messageContent, "发现") ||
					strings.Contains(messageContent, "得到")) &&
					strings.Contains(messageContent, itemName) {
					foundItems[item.Name] = true
				}
			}
		}

		// 添加发现道具到解锁内容
		for itemName := range foundItems {
			progress.UnlockedContent = append(progress.UnlockedContent, fmt.Sprintf("获得了%s", itemName))
		}

		// 基于道具收集度添加成就
		collectionRate := float64(len(foundItems)) / float64(len(scene.Items))
		if collectionRate >= 0.5 {
			progress.Achievements = append(progress.Achievements, "item_collector")
		}
		if collectionRate >= 1.0 {
			progress.Achievements = append(progress.Achievements, "treasure_hunter")
		}
	}

	// 3. 基于场景主题计算主题探索进度
	if scene != nil && len(scene.Themes) > 0 {
		exploredThemes := make(map[string]bool)
		for _, conv := range conversations {
			messageContent := strings.ToLower(conv.Content)
			for _, theme := range scene.Themes {
				themeName := strings.ToLower(theme)
				// 简单的主题匹配
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

	// 4. 基于场景氛围调整体验描述
	if scene != nil && scene.Atmosphere != "" {
		atmosphereBonus := fmt.Sprintf("体验了%s的氛围", scene.Atmosphere)
		progress.UnlockedContent = append(progress.UnlockedContent, atmosphereBonus)
	}

	// 5. 基于场景时代背景添加历史感知成就
	if scene != nil && scene.Era != "" {
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

	// 6. 基于场景创建时间计算新手/资深玩家状态
	if scene != nil {
		sceneAge := time.Since(scene.CreatedAt)
		if sceneAge > 7*24*time.Hour { // 场景存在超过一周
			progress.Achievements = append(progress.Achievements, "veteran_player")
		}

		// 基于最后访问时间判断活跃度
		timeSinceAccess := time.Since(scene.LastAccessed)
		if timeSinceAccess < 24*time.Hour {
			progress.Achievements = append(progress.Achievements, "daily_player")
		}
	}

	// 计算故事完成度（保留原有逻辑）
	if story != nil && len(story.Nodes) > 0 {
		revealedCount := 0
		for _, node := range story.Nodes {
			if node.IsRevealed {
				revealedCount++
			}
			// 收集解锁内容
			if node.IsRevealed && node.Type == "unlock" {
				progress.UnlockedContent = append(progress.UnlockedContent, node.Content)
			}
		}
		progress.StoryCompletion = float64(revealedCount) / float64(len(story.Nodes))
	}

	// 检查原有成就（保留原有逻辑）
	if progress.StoryCompletion >= 0.5 {
		progress.Achievements = append(progress.Achievements, "story_explorer")
	}
	if progress.CharacterInteractions >= 10 {
		progress.Achievements = append(progress.Achievements, "social_butterfly")
	}

	// ✅ 基于综合信息计算总体完成度
	if scene != nil {
		var completionFactors []float64

		// 故事完成度权重40%
		completionFactors = append(completionFactors, progress.StoryCompletion*0.4)

		// 地点探索度权重20%
		if len(scene.Locations) > 0 {
			exploredCount := 0
			for _, content := range progress.UnlockedContent {
				if strings.Contains(content, "探索了") {
					exploredCount++
				}
			}
			locationCompletion := float64(exploredCount) / float64(len(scene.Locations))
			completionFactors = append(completionFactors, locationCompletion*0.2)
		}

		// 道具收集度权重20%
		if len(scene.Items) > 0 {
			foundCount := 0
			for _, content := range progress.UnlockedContent {
				if strings.Contains(content, "获得了") {
					foundCount++
				}
			}
			itemCompletion := float64(foundCount) / float64(len(scene.Items))
			completionFactors = append(completionFactors, itemCompletion*0.2)
		}

		// 交互活跃度权重20%
		interactionCompletion := math.Min(float64(progress.CharacterInteractions)/20.0, 1.0)
		completionFactors = append(completionFactors, interactionCompletion*0.2)

		// 计算综合完成度
		totalCompletion := 0.0
		for _, factor := range completionFactors {
			totalCompletion += factor
		}

		// 更新故事完成度为综合完成度
		progress.StoryCompletion = math.Min(totalCompletion, 1.0)
	}

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
