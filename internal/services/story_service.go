// internal/services/story_service.go
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/storage"
)

// 角色互动触发条件常量
const (
	TriggerTypeCharacterInteraction = "character_interaction"
)

// StoryService 管理故事进展和剧情分支
type StoryService struct {
	SceneService *SceneService
	LLMService   *LLMService
	FileStorage  *storage.FileStorage
	ItemService  *ItemService
	BasePath     string
}

// NewStoryService 创建故事服务
func NewStoryService(llmService *LLMService) *StoryService {
	// 创建基础路径
	basePath := "data/stories"
	if err := os.MkdirAll(basePath, 0755); err != nil {
		fmt.Printf("警告: 创建故事数据目录失败: %v\n", err)
	}

	// 创建文件存储
	fileStorage, err := storage.NewFileStorage(basePath)
	if err != nil {
		fmt.Printf("警告: 创建故事文件存储失败: %v\n", err)
		return nil
	}

	// 创建场景服务(如果需要)
	scenesPath := "data/scenes"
	sceneService := NewSceneService(scenesPath)

	// 创建物品服务(如果需要)
	itemService := NewItemService()

	return &StoryService{
		SceneService: sceneService,
		LLMService:   llmService,
		FileStorage:  fileStorage,
		ItemService:  itemService,
		BasePath:     basePath,
	}
}

// InitializeStoryForScene 初始化场景的故事线
func (s *StoryService) InitializeStoryForScene(sceneID string, preferences *models.UserPreferences) (*models.StoryData, error) {
	// 加载场景
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, fmt.Errorf("加载场景失败: %w", err)
	}

	// 从场景内容中提取故事节点
	storyData, err := s.extractInitialStoryFromText(sceneData, preferences)
	if err != nil {
		return nil, fmt.Errorf("提取故事信息失败: %w", err)
	}

	// 保存故事数据
	if err := s.saveStoryData(sceneID, storyData); err != nil {
		return nil, fmt.Errorf("保存故事数据失败: %w", err)
	}

	return storyData, nil
}

// 从文本中提取初始故事节点和任务
func (s *StoryService) extractInitialStoryFromText(sceneData *SceneData, preferences *models.UserPreferences) (*models.StoryData, error) {
	// 检测场景语言
	isEnglish := isEnglishText(sceneData.Scene.Name + " " + sceneData.Scene.Description)

	// 如果场景名称和描述不能确定，尝试检查角色名称
	if !isEnglish && len(sceneData.Characters) > 0 {
		characterNames := ""
		for _, char := range sceneData.Characters {
			characterNames += char.Name + " "
		}
		isEnglish = isEnglishText(characterNames)
	}

	// 准备提示词
	var prompt, systemPrompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`Analyze the following scene and character information to create an initial setting for an interactive story:

Scene Title: %s
Scene Description: %s
Era/Period: %s
Main Locations: %s
Main Themes: %s

Key Characters:
%s

Based on this information, please create an engaging interactive story setup including:
1. Story background introduction
2. Main quests and objectives
3. Exploration locations
4. Initial story nodes
5. Possible story branches and decision points

Return in JSON format:
{
  "intro": "Overall story introduction",
  "main_objective": "Main story goal",
  "locations": [
    {
      "name": "Location name",
      "description": "Location description",
      "accessible": true/false,
      "requires_item": "Optional, ID of required item"
    }
  ],
  "initial_nodes": [
    {
      "content": "Story node content",
      "type": "main/side/hidden",
      "choices": [
        {
          "text": "Choice text",
          "consequence": "Choice consequence description",
          "next_node_hint": "Next node content hint"
        }
      ]
    }
  ],
  "tasks": [
    {
      "title": "Task title",
      "description": "Task description",
      "objectives": ["Objective 1", "Objective 2"],
      "reward": "Task reward description"
    }
  ]
}`,
			sceneData.Scene.Name,
			sceneData.Scene.Description,
			sceneData.Scene.Era,
			formatLocations(sceneData.Scene.Locations),
			formatThemes(sceneData.Scene.Themes),
			formatCharacters(sceneData.Characters),
		)

		systemPrompt = "You are a creative story designer responsible for creating engaging interactive stories."
	} else {
		// 中文提示词（原有逻辑）
		prompt = fmt.Sprintf(`分析以下场景和角色信息，创建一个交互式故事的初始设置：

场景标题: %s
场景描述: %s
时代背景: %s
主要地点: %s
主要主题: %s

主要角色:
%s

请根据这些信息创建一个有趣的交互式故事初始设置，包括：
1. 故事背景介绍
2. 主要任务和目标
3. 探索地点
4. 初始故事节点
5. 可能的故事分支和决策点

返回JSON格式:
{
  "intro": "故事总体介绍",
  "main_objective": "主要故事目标",
  "locations": [
    {
      "name": "地点名称",
      "description": "地点描述",
      "accessible": true/false,
      "requires_item": "可选，需要的物品ID"
    }
  ],
  "initial_nodes": [
    {
      "content": "故事节点内容",
      "type": "main/side/hidden",
      "choices": [
        {
          "text": "选择文本",
          "consequence": "选择后果描述",
          "next_node_hint": "下一个节点内容提示"
        }
      ]
    }
  ],
  "tasks": [
    {
      "title": "任务标题",
      "description": "任务描述",
      "objectives": ["目标1", "目标2"],
      "reward": "任务奖励描述"
    }
  ]
}`,
			sceneData.Scene.Name,
			sceneData.Scene.Description,
			sceneData.Scene.Era,
			formatLocations(sceneData.Scene.Locations),
			formatThemes(sceneData.Scene.Themes),
			formatCharacters(sceneData.Characters),
		)

		systemPrompt = "你是一个创意故事设计师，负责创建引人入胜的交互式故事。"
	}

	resp, err := s.LLMService.CreateChatCompletion(
		context.Background(),
		ChatCompletionRequest{
			Model: s.getLLMModel(preferences),
			Messages: []ChatCompletionMessage{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: prompt,
				},
			},
			// 请求JSON格式输出
			ExtraParams: map[string]interface{}{
				"response_format": map[string]string{
					"type": "json_object",
				},
			},
		},
	)

	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to generate story data: %w", err)
		} else {
			return nil, fmt.Errorf("生成故事数据失败: %w", err)
		}
	}

	jsonStr := resp.Choices[0].Message.Content

	// 解析返回的JSON
	var storySetup struct {
		Intro         string `json:"intro"`
		MainObjective string `json:"main_objective"`
		Locations     []struct {
			Name         string `json:"name"`
			Description  string `json:"description"`
			Accessible   bool   `json:"accessible"`
			RequiresItem string `json:"requires_item,omitempty"`
		} `json:"locations"`
		InitialNodes []struct {
			Content string `json:"content"`
			Type    string `json:"type"`
			Choices []struct {
				Text         string `json:"text"`
				Consequence  string `json:"consequence"`
				NextNodeHint string `json:"next_node_hint"`
			} `json:"choices"`
		} `json:"initial_nodes"`
		Tasks []struct {
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Objectives  []string `json:"objectives"`
			Reward      string   `json:"reward"`
		} `json:"tasks"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &storySetup); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to parse story data: %w", err)
		} else {
			return nil, fmt.Errorf("解析故事数据失败: %w", err)
		}
	}

	// 转换为故事数据模型
	storyData := &models.StoryData{
		SceneID:       sceneData.Scene.ID,
		Intro:         storySetup.Intro,
		MainObjective: storySetup.MainObjective,
		CurrentState: func() string {
			if isEnglish {
				return "Initial"
			}
			return "初始"
		}(),
		Progress:    0,
		Nodes:       []models.StoryNode{},
		Tasks:       []models.Task{},
		Locations:   []models.StoryLocation{},
		LastUpdated: time.Now(),
	}

	// 添加地点
	for _, loc := range storySetup.Locations {
		storyData.Locations = append(storyData.Locations, models.StoryLocation{
			ID:           fmt.Sprintf("loc_%s_%d", sceneData.Scene.ID, len(storyData.Locations)+1),
			SceneID:      sceneData.Scene.ID,
			Name:         loc.Name,
			Description:  loc.Description,
			Accessible:   loc.Accessible,
			RequiresItem: loc.RequiresItem,
			Source:       models.SourceExplicit,
		})
	}

	// 添加初始节点
	for i, node := range storySetup.InitialNodes {
		var choices []models.StoryChoice
		for j, choice := range node.Choices {
			choices = append(choices, models.StoryChoice{
				ID:           fmt.Sprintf("choice_%s_%d_%d", sceneData.Scene.ID, i+1, j+1),
				Text:         choice.Text,
				Consequence:  choice.Consequence,
				NextNodeHint: choice.NextNodeHint,
				Selected:     false,
			})
		}

		storyData.Nodes = append(storyData.Nodes, models.StoryNode{
			ID:         fmt.Sprintf("node_%s_%d", sceneData.Scene.ID, i+1),
			SceneID:    sceneData.Scene.ID,
			Content:    node.Content,
			Type:       node.Type,
			Choices:    choices,
			IsRevealed: i == 0, // 只有第一个节点默认显示
			CreatedAt:  time.Now(),
			Source:     models.SourceExplicit,
		})
	}

	// 添加任务
	for i, task := range storySetup.Tasks {
		objectives := make([]models.Objective, 0, len(task.Objectives))
		for j, obj := range task.Objectives {
			objectives = append(objectives, models.Objective{
				ID:          fmt.Sprintf("obj_%s_%d_%d", sceneData.Scene.ID, i+1, j+1),
				Description: obj,
				Completed:   false,
			})
		}

		storyData.Tasks = append(storyData.Tasks, models.Task{
			ID:          fmt.Sprintf("task_%s_%d", sceneData.Scene.ID, i+1),
			SceneID:     sceneData.Scene.ID,
			Title:       task.Title,
			Description: task.Description,
			Objectives:  objectives,
			Reward:      task.Reward,
			Completed:   false,
			IsRevealed:  i == 0, // 只有第一个任务默认显示
			Source:      models.SourceExplicit,
		})
	}

	return storyData, nil
}

// getLLMModel 根据用户偏好和可用配置获取合适的LLM模型名称
func (s *StoryService) getLLMModel(preferences *models.UserPreferences) string {
	// 如果提供了用户偏好设置，并且用户有指定模型
	if preferences != nil && preferences.PreferredModel != "" {
		return preferences.PreferredModel
	}

	// 使用LLMService的GetDefaultModel方法获取默认模型
	if s.LLMService != nil {
		defaultModel := s.LLMService.GetDefaultModel()
		if defaultModel != "" {
			return defaultModel
		}
	}

	// 极少数情况下如果仍然无法获取模型，使用通用默认值
	return "gpt-4o"
}

// 保存故事数据到文件
func (s *StoryService) saveStoryData(sceneID string, storyData *models.StoryData) error {
	storyDataJSON, err := json.MarshalIndent(storyData, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化故事数据失败: %w", err)
	}

	storyPath := filepath.Join(s.BasePath, sceneID, "story.json")
	if err := os.WriteFile(storyPath, storyDataJSON, 0644); err != nil {
		return fmt.Errorf("保存故事数据失败: %w", err)
	}

	return nil
}

// GetStoryData 获取场景的故事数据
func (s *StoryService) GetStoryData(sceneID string, preferences *models.UserPreferences) (*models.StoryData, error) {
	storyPath := filepath.Join(s.BasePath, sceneID, "story.json")

	// 检查故事数据文件是否存在
	if _, err := os.Stat(storyPath); os.IsNotExist(err) {
		// 如果不存在，创建初始故事数据
		return s.InitializeStoryForScene(sceneID, preferences)
	}

	// 读取故事数据
	storyDataBytes, err := os.ReadFile(storyPath)
	if err != nil {
		return nil, fmt.Errorf("读取故事数据失败: %w", err)
	}

	var storyData models.StoryData
	if err := json.Unmarshal(storyDataBytes, &storyData); err != nil {
		return nil, fmt.Errorf("解析故事数据失败: %w", err)
	}

	return &storyData, nil
}

// MakeChoice 处理玩家做出的故事选择
func (s *StoryService) MakeChoice(sceneID, nodeID, choiceID string, preferences *models.UserPreferences) (*models.StoryNode, error) {
	// 获取故事数据
	storyData, err := s.GetStoryData(sceneID, preferences)
	if err != nil {
		return nil, err
	}

	// 查找节点和选择
	var currentNode *models.StoryNode
	var selectedChoice *models.StoryChoice

	for i, node := range storyData.Nodes {
		if node.ID == nodeID {
			currentNode = &storyData.Nodes[i]
			for j, choice := range node.Choices {
				if choice.ID == choiceID {
					selectedChoice = &currentNode.Choices[j]
					currentNode.Choices[j].Selected = true
					break
				}
			}
			break
		}
	}

	if currentNode == nil || selectedChoice == nil {
		return nil, fmt.Errorf("无效的节点或选择")
	}

	// 生成下一个故事节点
	nextNode, err := s.generateNextStoryNode(sceneID, currentNode, selectedChoice, preferences)
	if err != nil {
		return nil, err
	}

	// 添加新节点到故事数据
	storyData.Nodes = append(storyData.Nodes, *nextNode)

	// 更新故事进度
	storyData.Progress += 5
	if storyData.Progress > 100 {
		storyData.Progress = 100
	}

	// 更新当前状态
	if storyData.Progress >= 100 {
		storyData.CurrentState = "结局"
	} else if storyData.Progress >= 75 {
		storyData.CurrentState = "高潮"
	} else if storyData.Progress >= 50 {
		storyData.CurrentState = "发展"
	} else if storyData.Progress >= 25 {
		storyData.CurrentState = "冲突"
	}

	// 保存更新后的故事数据
	if err := s.saveStoryData(sceneID, storyData); err != nil {
		return nil, err
	}

	// 处理可能的角色互动触发
	_, _ = s.ProcessCharacterInteractionTriggers(sceneID, nextNode.ID, preferences)

	return nextNode, nil
}

// 根据当前节点和选择生成下一个故事节点
func (s *StoryService) generateNextStoryNode(sceneID string, currentNode *models.StoryNode, selectedChoice *models.StoryChoice, preferences *models.UserPreferences) (*models.StoryNode, error) {
	// 加载场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, err
	}

	// 获取故事数据 - 添加这一部分
	storyData, err := s.GetStoryData(sceneID, preferences)
	if err != nil {
		return nil, err
	}

	// 检测语言
	isEnglish := isEnglishText(sceneData.Scene.Name + " " + currentNode.Content + " " + selectedChoice.Text)

	// 获取创造性级别文本表示
	var creativityStr string
	var allowPlotTwists bool
	if preferences != nil {
		switch preferences.CreativityLevel {
		case models.CreativityStrict:
			if isEnglish {
				creativityStr = "Low"
			} else {
				creativityStr = "低"
			}
			allowPlotTwists = false
		case models.CreativityBalanced:
			if isEnglish {
				creativityStr = "Medium"
			} else {
				creativityStr = "中"
			}
			allowPlotTwists = true
		case models.CreativityExpansive:
			if isEnglish {
				creativityStr = "High"
			} else {
				creativityStr = "高"
			}
			allowPlotTwists = true
		default:
			if isEnglish {
				creativityStr = "Medium"
			} else {
				creativityStr = "中"
			}
			allowPlotTwists = true
		}
	} else {
		if isEnglish {
			creativityStr = "Medium"
		} else {
			creativityStr = "中"
		}
		allowPlotTwists = true
	}

	var prompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`In the world of "%s", the player encounters the following situation:

%s

Player's choice: %s
Choice consequence: %s
Hint for next node: %s

Current story progress: %d%%
Current state: %s

Based on this choice, create a new story node that advances the plot.
Creativity level: %s
Allow plot twists: %v

Respond with a JSON object in the following format:
{
  "content": "Detailed description of the story node",
  "type": "event/choice/interaction",
  "choices": [
    {
      "text": "Choice text",
      "consequence": "Brief description of possible consequences",
      "next_node_hint": "Hint for the next node content"
    }
  ],
  "new_task": {
    "title": "Optional, if there's a new task",
    "description": "Task description",
    "objectives": ["Objective 1", "Objective 2"],
    "reward": "Completion reward"
  },
  "new_location": {
    "name": "Optional, if there's a new location",
    "description": "Location description",
    "accessible": true
  },
  "new_item": {
    "name": "Optional, if there's a new item",
    "description": "Item description",
    "type": "Item type"
  },
  "character_interactions": [
    {
      "trigger_condition": "Condition when this interaction should happen",
      "character_ids": ["character1_id", "character2_id"],
      "topic": "Topic of interaction",
      "context_description": "Brief context for the interaction"
    }
  ]
}`,
			sceneData.Scene.Name,
			currentNode.Content,
			selectedChoice.Text,
			selectedChoice.Consequence,
			selectedChoice.NextNodeHint,
			storyData.Progress,
			storyData.CurrentState,
			creativityStr,
			allowPlotTwists)
	} else {
		// 中文提示词（原有逻辑）
		prompt = fmt.Sprintf(`在《%s》的世界中，玩家遇到了以下情况:

%s

玩家选择: %s
选择后果: %s
下一节点提示: %s

当前故事进度: %d%%
当前状态: %s

根据这个选择，创建一个新的故事节点来推进剧情。
创造性级别: %s
允许剧情转折: %v

返回JSON格式:
{
  "content": "详细的故事节点描述",
  "type": "event/choice/interaction",
  "choices": [
    {
      "text": "选项文本",
      "consequence": "可能的后果简述",
      "next_node_hint": "下一节点的内容提示"
    }
  ],
  "new_task": {
    "title": "可选，如果有新任务",
    "description": "任务描述",
    "objectives": ["目标1", "目标2"],
    "reward": "完成奖励"
  },
  "new_location": {
    "name": "可选，如果有新地点",
    "description": "地点描述",
    "accessible": true
  },
  "new_item": {
    "name": "可选，如果有新物品",
    "description": "物品描述",
    "type": "物品类型"
  },
  "character_interactions": [
    {
      "trigger_condition": "何时触发此互动的条件",
      "character_ids": ["角色1_id", "角色2_id"],
      "topic": "互动主题",
      "context_description": "互动的简要背景"
    }
  ]
}`,
			sceneData.Scene.Name,
			currentNode.Content,
			selectedChoice.Text,
			selectedChoice.Consequence,
			selectedChoice.NextNodeHint,
			storyData.Progress,
			storyData.CurrentState,
			creativityStr,
			allowPlotTwists)
	}

	// 根据语言选择系统提示词
	var systemPrompt string
	if isEnglish {
		systemPrompt = "You are a creative story designer responsible for creating engaging interactive stories with character interactions."
	} else {
		systemPrompt = "你是一个创意故事设计师，负责创建引人入胜的交互式故事，包括角色之间的互动。"
	}

	resp, err := s.LLMService.CreateChatCompletion(
		context.Background(),
		ChatCompletionRequest{
			Model: s.getLLMModel(preferences),
			Messages: []ChatCompletionMessage{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: prompt,
				},
			},
			// 请求JSON格式输出
			ExtraParams: map[string]interface{}{
				"response_format": map[string]string{
					"type": "json_object",
				},
			},
		},
	)

	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to generate next node: %w", err)
		} else {
			return nil, fmt.Errorf("生成下一个节点失败: %w", err)
		}
	}

	jsonStr := resp.Choices[0].Message.Content

	// 解析返回的JSON
	var nodeData struct {
		Content string `json:"content"`
		Type    string `json:"type"`
		Choices []struct {
			Text         string `json:"text"`
			Consequence  string `json:"consequence"`
			NextNodeHint string `json:"next_node_hint"`
		} `json:"choices"`
		NewTask *struct {
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Objectives  []string `json:"objectives"`
			Reward      string   `json:"reward"`
		} `json:"new_task,omitempty"`
		NewLocation *struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Accessible  bool   `json:"accessible"`
		} `json:"new_location,omitempty"`
		NewItem *struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Type        string `json:"type"`
		} `json:"new_item,omitempty"`
		CharacterInteractions []struct {
			TriggerCondition   string   `json:"trigger_condition"`
			CharacterIDs       []string `json:"character_ids"`
			Topic              string   `json:"topic"`
			ContextDescription string   `json:"context_description"`
		} `json:"character_interactions,omitempty"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &nodeData); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to parse node data: %w", err)
		} else {
			return nil, fmt.Errorf("解析节点数据失败: %w", err)
		}
	}

	// 创建新节点
	nodeID := fmt.Sprintf("node_%s_%d", sceneID, time.Now().UnixNano())
	newNode := &models.StoryNode{
		ID:         nodeID,
		SceneID:    sceneID,
		ParentID:   currentNode.ID,
		Content:    nodeData.Content,
		Type:       nodeData.Type,
		Choices:    []models.StoryChoice{},
		IsRevealed: true,
		CreatedAt:  time.Now(),
		Source:     models.SourceGenerated,
		Metadata:   map[string]interface{}{},
	}

	// 添加选择
	for i, choice := range nodeData.Choices {
		newNode.Choices = append(newNode.Choices, models.StoryChoice{
			ID:           fmt.Sprintf("choice_%s_%d", nodeID, i+1),
			Text:         choice.Text,
			Consequence:  choice.Consequence,
			NextNodeHint: choice.NextNodeHint,
			Selected:     false,
			CreatedAt:    time.Now(),
		})
	}

	// 处理角色互动触发器
	if len(nodeData.CharacterInteractions) > 0 {
		interactionTriggers := make([]models.InteractionTrigger, 0, len(nodeData.CharacterInteractions))
		for i, interaction := range nodeData.CharacterInteractions {
			trigger := models.InteractionTrigger{
				ID:                 fmt.Sprintf("trigger_%s_%d", nodeID, i+1),
				Condition:          interaction.TriggerCondition,
				CharacterIDs:       interaction.CharacterIDs,
				Topic:              interaction.Topic,
				ContextDescription: interaction.ContextDescription,
				Triggered:          false,
				CreatedAt:          time.Now(),
			}
			interactionTriggers = append(interactionTriggers, trigger)
		}
		newNode.Metadata["interaction_triggers"] = interactionTriggers
	}

	// 添加新任务
	if nodeData.NewTask != nil {
		taskID := fmt.Sprintf("task_%s_%d", sceneID, time.Now().UnixNano())
		objectives := make([]models.Objective, 0, len(nodeData.NewTask.Objectives))
		for i, obj := range nodeData.NewTask.Objectives {
			objectives = append(objectives, models.Objective{
				ID:          fmt.Sprintf("obj_%s_%d", taskID, i+1),
				Description: obj,
				Completed:   false,
			})
		}

		task := models.Task{
			ID:          taskID,
			SceneID:     sceneID,
			Title:       nodeData.NewTask.Title,
			Description: nodeData.NewTask.Description,
			Objectives:  objectives,
			Reward:      nodeData.NewTask.Reward,
			Completed:   false,
			IsRevealed:  true,
			Source:      models.SourceGenerated,
		}

		storyData.Tasks = append(storyData.Tasks, task)
	}

	// 添加新地点
	if nodeData.NewLocation != nil {
		locationID := fmt.Sprintf("loc_%s_%d", sceneID, time.Now().UnixNano())
		location := models.StoryLocation{
			ID:          locationID,
			SceneID:     sceneID,
			Name:        nodeData.NewLocation.Name,
			Description: nodeData.NewLocation.Description,
			Accessible:  nodeData.NewLocation.Accessible,
			Source:      models.SourceGenerated,
		}

		storyData.Locations = append(storyData.Locations, location)
	}

	// 添加新物品（如果适用）
	if nodeData.NewItem != nil && s.ItemService != nil {
		item := &models.Item{
			ID:          fmt.Sprintf("item_%s_%d", sceneID, time.Now().UnixNano()),
			SceneID:     sceneID,
			Name:        nodeData.NewItem.Name,
			Description: nodeData.NewItem.Description,
			Type:        nodeData.NewItem.Type,
			IsOwned:     true,
			FoundAt:     time.Now(),
			Source:      models.SourceStory,
		}

		// 调用ItemService保存物品
		if err := s.ItemService.AddItem(sceneID, item); err != nil {
			if isEnglish {
				fmt.Printf("Warning: Failed to save new item: %v\n", err)
			} else {
				fmt.Printf("警告: 保存新物品失败: %v\n", err)
			}
		}
	}

	// 保存更新后的故事数据
	if nodeData.NewTask != nil || nodeData.NewLocation != nil {
		if err := s.saveStoryData(sceneID, storyData); err != nil {
			if isEnglish {
				fmt.Printf("Warning: Failed to save updated story data: %v\n", err)
			} else {
				fmt.Printf("警告: 保存更新的故事数据失败: %v\n", err)
			}
		}
	}

	return newNode, nil
}

// CompleteObjective 完成任务目标
func (s *StoryService) CompleteObjective(sceneID, taskID, objectiveID string) error {
	// 获取故事数据
	storyData, err := s.GetStoryData(sceneID, nil)
	if err != nil {
		return err
	}

	// 查找任务和目标
	taskFound := false
	objectiveFound := false
	allObjectivesCompleted := true

	for i, task := range storyData.Tasks {
		if task.ID == taskID {
			taskFound = true
			for j, objective := range task.Objectives {
				if objective.ID == objectiveID {
					objectiveFound = true
					storyData.Tasks[i].Objectives[j].Completed = true
				}
				if !storyData.Tasks[i].Objectives[j].Completed {
					allObjectivesCompleted = false
				}
			}

			// 如果所有目标都完成，标记任务为完成
			if allObjectivesCompleted {
				storyData.Tasks[i].Completed = true

				// 任务完成时增加故事进度
				storyData.Progress += 10
				if storyData.Progress > 100 {
					storyData.Progress = 100
				}

				// 更新当前状态
				if storyData.Progress >= 100 {
					storyData.CurrentState = "结局"
				} else if storyData.Progress >= 75 {
					storyData.CurrentState = "高潮"
				} else if storyData.Progress >= 50 {
					storyData.CurrentState = "发展"
				} else if storyData.Progress >= 25 {
					storyData.CurrentState = "冲突"
				}
			}
			break
		}
	}

	if !taskFound || !objectiveFound {
		return fmt.Errorf("无效的任务或目标")
	}

	// 保存更新后的故事数据
	if err := s.saveStoryData(sceneID, storyData); err != nil {
		return err
	}

	return nil
}

// UnlockLocation 解锁场景地点
func (s *StoryService) UnlockLocation(sceneID, locationID string) error {
	// 获取故事数据
	storyData, err := s.GetStoryData(sceneID, nil)
	if err != nil {
		return err
	}

	// 查找地点
	for i, location := range storyData.Locations {
		if location.ID == locationID {
			storyData.Locations[i].Accessible = true
			break
		}
	}

	// 保存更新后的故事数据
	if err := s.saveStoryData(sceneID, storyData); err != nil {
		return err
	}

	return nil
}

// ExploreLocation 探索地点，可能触发新的故事节点或发现物品
func (s *StoryService) ExploreLocation(sceneID, locationID string, preferences *models.UserPreferences) (*models.ExplorationResult, error) {
	// 获取故事数据
	storyData, err := s.GetStoryData(sceneID, nil)
	if err != nil {
		return nil, err
	}

	// 查找地点
	var location *models.StoryLocation
	for i, loc := range storyData.Locations {
		if loc.ID == locationID {
			location = &storyData.Locations[i]
			break
		}
	}

	if location == nil {
		return nil, fmt.Errorf("地点不存在")
	}

	if !location.Accessible {
		return nil, fmt.Errorf("此地点尚未解锁")
	}

	// 加载场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, err
	}

	// 检测语言
	isEnglish := isEnglishText(sceneData.Scene.Name + " " + location.Name + " " + location.Description)

	// 准备提示词和系统提示词
	var prompt, systemPrompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`In the world of "%s", the player is exploring the location: %s
Location description: %s

Scene background: %s

Creativity level: %s
Allow plot twists: %v

Please describe what the player discovers while exploring this location, which may include:
1. Detailed environment description
2. Possible items found
3. Possible story events triggered
4. Hidden clues

Return in JSON format:
{
  "description": "Detailed exploration description",
  "found_item": {
    "name": "Item name",
    "description": "Item description",
    "type": "Item type"
  },
  "story_event": {
    "content": "Story event description",
    "type": "discovery/encounter/revelation",
    "choices": [
      {
        "text": "Choice text",
        "consequence": "Choice consequence"
      }
    ]
  },
  "new_clue": "Discovered clue"
}`,
			sceneData.Scene.Name,
			location.Name,
			location.Description,
			sceneData.Scene.Description,
			string(preferences.CreativityLevel),
			preferences.AllowPlotTwists)

		systemPrompt = "You are a creative story designer responsible for creating engaging interactive stories."
	} else {
		// 中文提示词
		prompt = fmt.Sprintf(`在《%s》的世界中，玩家正在探索地点: %s
地点描述: %s

场景背景: %s

创造性级别: %s
允许剧情转折: %v

请描述玩家探索这个地点的发现，可能包括:
1. 详细的环境描述
2. 可能发现的物品
3. 可能触发的故事事件
4. 隐藏的线索

返回JSON格式:
{
  "description": "详细的探索描述",
  "found_item": {
    "name": "物品名称",
    "description": "物品描述",
    "type": "物品类型"
  },
  "story_event": {
    "content": "故事事件描述",
    "type": "discovery/encounter/revelation",
    "choices": [
      {
        "text": "选择文本",
        "consequence": "选择后果"
      }
    ]
  },
  "new_clue": "发现的线索"
}`,
			sceneData.Scene.Name,
			location.Name,
			location.Description,
			sceneData.Scene.Description,
			string(preferences.CreativityLevel),
			preferences.AllowPlotTwists)

		systemPrompt = "你是一个创意故事设计师，负责创建引人入胜的交互式故事。"
	}

	resp, err := s.LLMService.CreateChatCompletion(
		context.Background(),
		ChatCompletionRequest{
			Model: s.getLLMModel(preferences),
			Messages: []ChatCompletionMessage{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: prompt,
				},
			},
			// 请求JSON格式输出
			ExtraParams: map[string]interface{}{
				"response_format": map[string]string{
					"type": "json_object",
				},
			},
		},
	)

	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to generate exploration result: %w", err)
		} else {
			return nil, fmt.Errorf("生成探索结果失败: %w", err)
		}
	}

	jsonStr := resp.Choices[0].Message.Content

	// 解析返回的JSON
	var explorationData struct {
		Description string `json:"description"`
		FoundItem   *struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Type        string `json:"type"`
		} `json:"found_item,omitempty"`
		StoryEvent *struct {
			Content string `json:"content"`
			Type    string `json:"type"`
			Choices []struct {
				Text        string `json:"text"`
				Consequence string `json:"consequence"`
			} `json:"choices"`
		} `json:"story_event,omitempty"`
		NewClue string `json:"new_clue,omitempty"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &explorationData); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to parse exploration data: %w", err)
		} else {
			return nil, fmt.Errorf("解析探索数据失败: %w", err)
		}
	}

	// 构建探索结果
	result := &models.ExplorationResult{
		LocationID:   locationID,
		Description:  explorationData.Description,
		NewClue:      explorationData.NewClue,
		ExploredTime: time.Now(),
	}

	// 处理发现的物品
	if explorationData.FoundItem != nil {
		item := models.Item{
			ID:          fmt.Sprintf("item_%s_%d", sceneID, time.Now().UnixNano()),
			SceneID:     sceneID,
			Name:        explorationData.FoundItem.Name,
			Description: explorationData.FoundItem.Description,
			Type:        explorationData.FoundItem.Type,
			IsOwned:     true,
			FoundAt:     time.Now(),
			Source:      models.SourceExploration,
		}

		result.FoundItem = &item

		// 保存发现的物品
		if s.ItemService != nil {
			if err := s.ItemService.AddItem(sceneID, &item); err != nil {
				if isEnglish {
					return nil, fmt.Errorf("failed to save discovered item: %w", err)
				} else {
					return nil, fmt.Errorf("保存发现的物品失败: %w", err)
				}
			}
		} else {
			// 记录日志：ItemService未初始化，物品仅返回但未持久化
			if isEnglish {
				fmt.Printf("Warning: ItemService not initialized, item '%s' not saved to persistent storage\n", item.Name)
			} else {
				fmt.Printf("警告: ItemService未初始化，物品'%s'未保存到持久化存储\n", item.Name)
			}
		}
	}

	// 处理故事事件
	if explorationData.StoryEvent != nil {
		// 创建新的故事节点
		nodeID := fmt.Sprintf("node_%s_%d", sceneID, time.Now().UnixNano())
		storyNode := models.StoryNode{
			ID:         nodeID,
			SceneID:    sceneID,
			Content:    explorationData.StoryEvent.Content,
			Type:       explorationData.StoryEvent.Type,
			CreatedAt:  time.Now(),
			IsRevealed: true,
			Source:     models.SourceExploration,
			Choices:    []models.StoryChoice{},
		}

		// 添加选择
		for i, choice := range explorationData.StoryEvent.Choices {
			storyNode.Choices = append(storyNode.Choices, models.StoryChoice{
				ID:          fmt.Sprintf("choice_%s_%d", nodeID, i+1),
				Text:        choice.Text,
				Consequence: choice.Consequence,
				Selected:    false,
			})
		}

		// 将节点添加到故事数据
		storyData.Nodes = append(storyData.Nodes, storyNode)
		result.StoryNode = &storyNode

		// 保存更新后的故事数据
		if err := s.saveStoryData(sceneID, storyData); err != nil {
			if isEnglish {
				return nil, fmt.Errorf("failed to save exploration-triggered story node: %w", err)
			} else {
				return nil, fmt.Errorf("保存探索触发的故事节点失败: %w", err)
			}
		}
	}

	return result, nil
}

// GetAvailableChoices 获取当前可用的剧情选择
func (s *StoryService) GetAvailableChoices(sceneID string) ([]models.StoryChoice, error) {
	// 获取故事数据
	storyData, err := s.GetStoryData(sceneID, nil)
	if err != nil {
		return nil, err
	}

	var availableChoices []models.StoryChoice

	// 查找最新的、已显示的、未选择的故事节点
	var latestRevealedNode *models.StoryNode
	latestTime := time.Time{}

	for i := range storyData.Nodes {
		node := &storyData.Nodes[i]
		if node.IsRevealed && node.CreatedAt.After(latestTime) {
			// 检查是否有未选择的选项
			hasUnselectedChoices := false
			for _, choice := range node.Choices {
				if !choice.Selected {
					hasUnselectedChoices = true
					break
				}
			}

			if hasUnselectedChoices {
				latestRevealedNode = node
				latestTime = node.CreatedAt
			}
		}
	}

	// 如果找到了最新的节点，收集其未选择的选项
	if latestRevealedNode != nil {
		for _, choice := range latestRevealedNode.Choices {
			if !choice.Selected {
				availableChoices = append(availableChoices, choice)
			}
		}
	}

	return availableChoices, nil
}

// AdvanceStory 推进故事情节
func (s *StoryService) AdvanceStory(sceneID string, preferences *models.UserPreferences) (*models.StoryUpdate, error) {
	// 获取故事数据
	storyData, err := s.GetStoryData(sceneID, nil)
	if err != nil {
		return nil, err
	}

	// 加载场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, err
	}

	// 检测语言
	isEnglish := isEnglishText(sceneData.Scene.Name + " " + storyData.Intro + " " + storyData.MainObjective)

	// 检查故事进度
	if storyData.Progress >= 100 {
		if isEnglish {
			return nil, fmt.Errorf("the story has already ended")
		} else {
			return nil, fmt.Errorf("故事已经结束")
		}
	}

	// 创建故事更新提示
	creativityStr := string(preferences.CreativityLevel)
	allowPlotTwists := preferences.AllowPlotTwists

	var prompt string
	var systemPrompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`In the story "%s", the current progress is %d%%, and the state is "%s".
Story Introduction: %s
Main Objective: %s

Please generate a natural story progression event that happens automatically without requiring player choices.
This event should push the story forward and add depth or complexity to the narrative.

Creativity level: %s
Allow plot twists: %v

Return in JSON format:
{
  "title": "Event title",
  "content": "Detailed event description",
  "type": "revelation/encounter/discovery/complication",
  "progress_impact": 5,
  "new_task": {
    "title": "Optional, if there's a new task",
    "description": "Task description",
    "objectives": ["Objective 1", "Objective 2"],
    "reward": "Completion reward"
  },
  "new_clue": "Optional, newly discovered clue"
}`,
			sceneData.Scene.Name,
			storyData.Progress,
			storyData.CurrentState,
			storyData.Intro,
			storyData.MainObjective,
			creativityStr,
			allowPlotTwists)

		systemPrompt = "You are a creative story designer responsible for creating engaging interactive stories."
	} else {
		// 中文提示词
		prompt = fmt.Sprintf(`在《%s》的故事中，当前进展为 %d%%，状态为"%s"。
故事简介: %s
主要目标: %s

请根据当前进展生成一个自然的故事推进事件，不需要玩家选择就能自动发生。
这个事件应该能够向前推动故事情节，增加故事的深度或复杂性。

创造性级别: %s
允许剧情转折: %v

返回JSON格式:
{
  "title": "事件标题",
  "content": "事件详细描述",
  "type": "revelation/encounter/discovery/complication",
  "progress_impact": 5,
  "new_task": {
    "title": "可选，如果有新任务",
    "description": "任务描述",
    "objectives": ["目标1", "目标2"],
    "reward": "完成奖励"
  },
  "new_clue": "可选，新发现的线索"
}`,
			sceneData.Scene.Name,
			storyData.Progress,
			storyData.CurrentState,
			storyData.Intro,
			storyData.MainObjective,
			creativityStr,
			allowPlotTwists)

		systemPrompt = "你是一个创意故事设计师，负责创建引人入胜的交互式故事。"
	}

	resp, err := s.LLMService.CreateChatCompletion(
		context.Background(),
		ChatCompletionRequest{
			Model: s.getLLMModel(preferences),
			Messages: []ChatCompletionMessage{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: prompt,
				},
			},
			// 请求JSON格式输出
			ExtraParams: map[string]interface{}{
				"response_format": map[string]string{
					"type": "json_object",
				},
			},
		},
	)

	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to generate story advancement event: %w", err)
		} else {
			return nil, fmt.Errorf("生成故事推进事件失败: %w", err)
		}
	}

	jsonStr := resp.Choices[0].Message.Content

	// 解析返回的JSON
	var eventData struct {
		Title          string `json:"title"`
		Content        string `json:"content"`
		Type           string `json:"type"`
		ProgressImpact int    `json:"progress_impact"`
		NewTask        *struct {
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Objectives  []string `json:"objectives"`
			Reward      string   `json:"reward"`
		} `json:"new_task,omitempty"`
		NewClue string `json:"new_clue,omitempty"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &eventData); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to parse story event data: %w", err)
		} else {
			return nil, fmt.Errorf("解析故事事件数据失败: %w", err)
		}
	}

	// 创建故事更新
	storyUpdate := &models.StoryUpdate{
		ID:        fmt.Sprintf("update_%s_%d", sceneID, time.Now().UnixNano()),
		SceneID:   sceneID,
		Title:     eventData.Title,
		Content:   eventData.Content,
		Type:      eventData.Type,
		CreatedAt: time.Now(),
		Source:    models.SourceSystem,
	}

	// 更新故事进度
	storyData.Progress += eventData.ProgressImpact
	if storyData.Progress > 100 {
		storyData.Progress = 100
	}

	// 更新当前状态
	if storyData.Progress >= 100 {
		if isEnglish {
			storyData.CurrentState = "Ending"
		} else {
			storyData.CurrentState = "结局"
		}
	} else if storyData.Progress >= 75 {
		if isEnglish {
			storyData.CurrentState = "Climax"
		} else {
			storyData.CurrentState = "高潮"
		}
	} else if storyData.Progress >= 50 {
		if isEnglish {
			storyData.CurrentState = "Development"
		} else {
			storyData.CurrentState = "发展"
		}
	} else if storyData.Progress >= 25 {
		if isEnglish {
			storyData.CurrentState = "Conflict"
		} else {
			storyData.CurrentState = "冲突"
		}
	} else {
		if isEnglish {
			storyData.CurrentState = "Initial"
		} else {
			storyData.CurrentState = "初始"
		}
	}

	// 处理新任务
	if eventData.NewTask != nil {
		taskID := fmt.Sprintf("task_%s_%d", sceneID, time.Now().UnixNano())
		objectives := make([]models.Objective, 0, len(eventData.NewTask.Objectives))
		for i, obj := range eventData.NewTask.Objectives {
			objectives = append(objectives, models.Objective{
				ID:          fmt.Sprintf("obj_%s_%d", taskID, i+1),
				Description: obj,
				Completed:   false,
			})
		}

		task := models.Task{
			ID:          taskID,
			SceneID:     sceneID,
			Title:       eventData.NewTask.Title,
			Description: eventData.NewTask.Description,
			Objectives:  objectives,
			Reward:      eventData.NewTask.Reward,
			Completed:   false,
			IsRevealed:  true,
			Source:      models.SourceSystem,
		}

		storyData.Tasks = append(storyData.Tasks, task)
		storyUpdate.NewTask = &task
	}

	// 处理新线索
	if eventData.NewClue != "" {
		storyUpdate.NewClue = eventData.NewClue
	}

	// 保存更新后的故事数据
	if err := s.saveStoryData(sceneID, storyData); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to save updated story data: %w", err)
		} else {
			return nil, fmt.Errorf("保存更新的故事数据失败: %w", err)
		}
	}

	return storyUpdate, nil
}

// CreateStoryBranch 创建故事分支
func (s *StoryService) CreateStoryBranch(sceneID string, triggerType string, triggerID string, preferences *models.UserPreferences) (*models.StoryNode, error) {
	// 获取故事数据
	storyData, err := s.GetStoryData(sceneID, nil)
	if err != nil {
		return nil, err
	}

	// 加载场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, err
	}

	// 检测语言
	isEnglish := isEnglishText(sceneData.Scene.Name + " " + storyData.Intro)

	// 准备分支创建的提示
	creativityStr := string(preferences.CreativityLevel)
	allowPlotTwists := preferences.AllowPlotTwists

	var triggerDescription string

	// 根据触发类型获取相应的描述
	switch triggerType {
	case "item":
		// 查找物品描述 - 这里应该调用ItemService
		if isEnglish {
			triggerDescription = "(Item triggered)"
		} else {
			triggerDescription = "（物品触发）"
		}
	case "location":
		// 查找地点描述
		for _, loc := range storyData.Locations {
			if loc.ID == triggerID {
				if isEnglish {
					triggerDescription = fmt.Sprintf("Location: %s - %s", loc.Name, loc.Description)
				} else {
					triggerDescription = fmt.Sprintf("地点：%s - %s", loc.Name, loc.Description)
				}
				break
			}
		}
	case "task":
		// 查找任务描述
		for _, task := range storyData.Tasks {
			if task.ID == triggerID {
				if isEnglish {
					triggerDescription = fmt.Sprintf("Task: %s - %s", task.Title, task.Description)
				} else {
					triggerDescription = fmt.Sprintf("任务：%s - %s", task.Title, task.Description)
				}
				break
			}
		}
	default:
		if isEnglish {
			triggerDescription = "Unknown trigger"
		} else {
			triggerDescription = "未知触发器"
		}
	}

	var prompt string
	var systemPrompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`In the world of "%s", the player has encountered the following situation:

%s

Current story state: %s
Story progress: %d%%

Based on this trigger, create a new branch story node that provides multiple choices for the player.
This branch should coordinate with the main storyline, but add depth to the story or provide additional content.

Creativity level: %s
Allow plot twists: %v

Return in JSON format:
{
  "content": "Detailed story node description",
  "type": "branch/side/optional",
  "choices": [
    {
      "text": "Choice text",
      "consequence": "Description of choice consequence",
      "next_node_hint": "Subsequent development hint"
    }
  ]
}`,
			sceneData.Scene.Name,
			triggerDescription,
			storyData.CurrentState,
			storyData.Progress,
			creativityStr,
			allowPlotTwists)

		systemPrompt = "You are a creative story designer responsible for creating engaging interactive stories."
	} else {
		// 中文提示词
		prompt = fmt.Sprintf(`在《%s》的世界中，玩家遇到了以下情况:

%s

当前故事状态: %s
故事进展: %d%%

根据这个触发因素，创建一个新的分支故事节点，提供玩家多个选择。
这个分支应该与主线故事协调，但能增加故事的深度或提供额外的内容。

创造性级别: %s
允许剧情转折: %v

返回JSON格式:
{
  "content": "详细的故事节点描述",
  "type": "branch/side/optional",
  "choices": [
    {
      "text": "选择文本",
      "consequence": "选择后果描述",
      "next_node_hint": "后续发展提示"
    }
  ]
}`,
			sceneData.Scene.Name,
			triggerDescription,
			storyData.CurrentState,
			storyData.Progress,
			creativityStr,
			allowPlotTwists)

		systemPrompt = "你是一个创意故事设计师，负责创建引人入胜的交互式故事。"
	}

	resp, err := s.LLMService.CreateChatCompletion(
		context.Background(),
		ChatCompletionRequest{
			Model: s.getLLMModel(preferences),
			Messages: []ChatCompletionMessage{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: prompt,
				},
			},
			// 请求JSON格式输出
			ExtraParams: map[string]interface{}{
				"response_format": map[string]string{
					"type": "json_object",
				},
			},
		},
	)

	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to generate story branch: %w", err)
		} else {
			return nil, fmt.Errorf("生成故事分支失败: %w", err)
		}
	}

	jsonStr := resp.Choices[0].Message.Content

	// 解析返回的JSON
	var branchData struct {
		Content string `json:"content"`
		Type    string `json:"type"`
		Choices []struct {
			Text         string `json:"text"`
			Consequence  string `json:"consequence"`
			NextNodeHint string `json:"next_node_hint"`
		} `json:"choices"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &branchData); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to parse story branch data: %w", err)
		} else {
			return nil, fmt.Errorf("解析故事分支数据失败: %w", err)
		}
	}

	// 创建新的故事节点
	nodeID := fmt.Sprintf("branch_%s_%d", sceneID, time.Now().UnixNano())
	storyNode := models.StoryNode{
		ID:         nodeID,
		SceneID:    sceneID,
		Content:    branchData.Content,
		Type:       branchData.Type,
		IsRevealed: true,
		CreatedAt:  time.Now(),
		Source:     models.SourceBranch,
		Choices:    []models.StoryChoice{},
	}

	// 添加选择
	for i, choice := range branchData.Choices {
		storyNode.Choices = append(storyNode.Choices, models.StoryChoice{
			ID:           fmt.Sprintf("choice_%s_%d", nodeID, i+1),
			Text:         choice.Text,
			Consequence:  choice.Consequence,
			NextNodeHint: choice.NextNodeHint,
			Selected:     false,
		})
	}

	// 将节点添加到故事数据
	storyData.Nodes = append(storyData.Nodes, storyNode)

	// 保存更新后的故事数据
	if err := s.saveStoryData(sceneID, storyData); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to save updated story data: %w", err)
		} else {
			return nil, fmt.Errorf("保存更新后的故事数据失败: %w", err)
		}
	}

	return &storyNode, nil
}

// EvaluateStoryProgress 评估故事进展状态
func (s *StoryService) EvaluateStoryProgress(sceneID string) (*models.StoryProgressStatus, error) {
	// 获取故事数据
	storyData, err := s.GetStoryData(sceneID, nil)
	if err != nil {
		return nil, err
	}

	// 计算任务完成情况
	totalTasks := len(storyData.Tasks)
	completedTasks := 0
	for _, task := range storyData.Tasks {
		if task.Completed {
			completedTasks++
		}
	}

	// 计算地点探索情况
	totalLocations := len(storyData.Locations)
	accessibleLocations := 0
	for _, loc := range storyData.Locations {
		if loc.Accessible {
			accessibleLocations++
		}
	}

	// 计算故事节点情况
	totalNodes := len(storyData.Nodes)

	// 创建进展状态
	status := &models.StoryProgressStatus{
		SceneID:             sceneID,
		Progress:            storyData.Progress,
		CurrentState:        storyData.CurrentState,
		CompletedTasks:      completedTasks,
		TotalTasks:          totalTasks,
		TaskCompletionRate:  float64(completedTasks) / float64(totalTasks) * 100,
		AccessibleLocations: accessibleLocations,
		TotalLocations:      totalLocations,
		LocationAccessRate:  float64(accessibleLocations) / float64(totalLocations) * 100,
		TotalStoryNodes:     totalNodes,
		EstimatedCompletion: time.Duration(0),
		IsMainObjectiveMet:  storyData.Progress >= 100,
	}

	// 估算完成时间 - 采用动态估算而非固定值
	// 基于故事复杂度（任务数量、地点数量、节点数量）和当前进度计算
	if status.Progress > 0 {
		// 基础估计时间基于故事复杂度
		baseTimePerTask := 15 * time.Minute     // 每个任务平均15分钟
		baseTimePerLocation := 10 * time.Minute // 每个地点平均10分钟
		baseTimePerNode := 5 * time.Minute      // 每个节点平均5分钟

		// 计算总体复杂度时间
		storyComplexityTime := time.Duration(totalTasks)*baseTimePerTask +
			time.Duration(totalLocations)*baseTimePerLocation +
			time.Duration(totalNodes)*baseTimePerNode

		// 考虑已消耗时间（如果有的话）
		var elapsedTime time.Duration
		if len(storyData.Nodes) > 1 {
			firstNodeTime := storyData.Nodes[0].CreatedAt
			latestNodeTime := time.Now()
			for _, node := range storyData.Nodes {
				if node.CreatedAt.After(latestNodeTime) {
					latestNodeTime = node.CreatedAt
				}
			}
			elapsedTime = latestNodeTime.Sub(firstNodeTime)
		}

		// 根据当前进度和已消耗时间估算剩余时间
		remainingProgress := 100 - status.Progress
		if elapsedTime > 0 && status.Progress > 0 {
			// 使用实际速度估算
			estimatedTotalTime := time.Duration(float64(elapsedTime) * 100 / float64(status.Progress))
			status.EstimatedCompletion = time.Duration(float64(estimatedTotalTime) * float64(remainingProgress) / 100.0)
		} else {
			// 使用复杂度估算
			status.EstimatedCompletion = time.Duration(float64(storyComplexityTime) * float64(remainingProgress) / 100.0)
		}

		// 设置合理的上下限
		minTime := 15 * time.Minute
		maxTime := 6 * time.Hour
		if status.EstimatedCompletion < minTime {
			status.EstimatedCompletion = minTime
		} else if status.EstimatedCompletion > maxTime {
			status.EstimatedCompletion = maxTime
		}
	}

	return status, nil
}

// 辅助函数：格式化位置信息
func formatLocations(locations []models.Location) string {
	var locationNames []string
	for _, loc := range locations {
		locationNames = append(locationNames, loc.Name)
	}
	return strings.Join(locationNames, ", ")
}

// 辅助函数：格式化主题信息
func formatThemes(themes []string) string {
	return strings.Join(themes, ", ")
}

// 辅助函数：格式化角色信息
func formatCharacters(characters []*models.Character) string {
	var result strings.Builder
	for _, char := range characters {
		result.WriteString(fmt.Sprintf("- %s: %s\n", char.Name, char.Personality))
	}
	return result.String()
}

// RewindToNode 回溯故事到指定节点
func (s *StoryService) RewindToNode(sceneID, nodeID string) (*models.StoryData, error) {
	// 获取故事数据
	storyData, err := s.GetStoryData(sceneID, nil)
	if err != nil {
		return nil, err
	}

	// 查找目标节点
	var targetNode *models.StoryNode
	for i := range storyData.Nodes {
		if storyData.Nodes[i].ID == nodeID {
			targetNode = &storyData.Nodes[i]
			break
		}
	}

	if targetNode == nil {
		return nil, fmt.Errorf("节点不存在或不可回溯")
	}

	// 标记目标节点之后的所有节点为未揭示
	for i := range storyData.Nodes {
		node := &storyData.Nodes[i]
		if node.CreatedAt.After(targetNode.CreatedAt) && node.ID != nodeID {
			node.IsRevealed = false
			// 重置该节点的所有选择
			for j := range node.Choices {
				node.Choices[j].Selected = false
			}
		}
	}

	// 重置目标节点的选择状态
	for i := range targetNode.Choices {
		targetNode.Choices[i].Selected = false
	}

	// 重新计算故事进度
	newProgress := calculateProgress(storyData, targetNode)
	if newProgress >= 0 {
		storyData.Progress = newProgress
	}

	// 更新当前状态
	if storyData.Progress >= 100 {
		storyData.CurrentState = "结局"
	} else if storyData.Progress >= 75 {
		storyData.CurrentState = "高潮"
	} else if storyData.Progress >= 50 {
		storyData.CurrentState = "发展"
	} else if storyData.Progress >= 25 {
		storyData.CurrentState = "冲突"
	} else {
		storyData.CurrentState = "初始"
	}

	// 保存更新后的故事数据
	if err := s.saveStoryData(sceneID, storyData); err != nil {
		return nil, err
	}

	return storyData, nil
}

// 计算基于指定节点的故事进度
func calculateProgress(storyData *models.StoryData, referenceNode *models.StoryNode) int {
	// 计算节点总数和已揭示节点数
	totalNodes := 0
	revealedNodes := 0

	for _, node := range storyData.Nodes {
		if node.IsRevealed && !node.CreatedAt.After(referenceNode.CreatedAt) {
			revealedNodes++
		}
		totalNodes++
	}

	// 如果没有节点，返回0进度
	if totalNodes == 0 {
		return 0
	}

	// 基于已揭示节点百分比计算进度
	progress := (revealedNodes * 100) / totalNodes

	// 考虑完成的任务
	completedTasks := 0
	totalTasks := len(storyData.Tasks)
	for _, task := range storyData.Tasks {
		if task.IsRevealed && task.Completed {
			completedTasks++
		}
	}

	// 任务进度
	taskProgress := 0
	if totalTasks > 0 {
		taskProgress = (completedTasks * 100) / totalTasks
	}

	// 综合节点进度和任务进度
	return (progress*3 + taskProgress) / 4
}

// ProcessCharacterInteractionTriggers 方法，处理故事节点中的角色互动触发器
func (s *StoryService) ProcessCharacterInteractionTriggers(sceneID string, nodeID string, preferences *models.UserPreferences) ([]*models.CharacterInteraction, error) {
	// 获取场景数据
	sceneData, err := s.SceneService.LoadScene(sceneID)
	if err != nil {
		return nil, err
	}

	// 获取故事数据
	storyData, err := s.GetStoryData(sceneID, preferences)
	if err != nil {
		return nil, err
	}

	// 查找节点
	var node *models.StoryNode
	for i := range storyData.Nodes {
		if storyData.Nodes[i].ID == nodeID {
			node = &storyData.Nodes[i]
			break
		}
	}

	if node == nil {
		return nil, fmt.Errorf("节点不存在: %s", nodeID)
	}

	// 检测语言
	isEnglish := isEnglishText(sceneData.Scene.Name)

	// 获取节点的交互触发器
	if node.Metadata == nil || node.Metadata["interaction_triggers"] == nil {
		return nil, nil // 没有触发器，直接返回
	}

	// 获取交互触发器列表
	var triggers []models.InteractionTrigger
	triggersData, err := json.Marshal(node.Metadata["interaction_triggers"])
	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to process interaction triggers: %w", err)
		} else {
			return nil, fmt.Errorf("处理互动触发器失败: %w", err)
		}
	}

	if err := json.Unmarshal(triggersData, &triggers); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to parse interaction triggers: %w", err)
		} else {
			return nil, fmt.Errorf("解析互动触发器失败: %w", err)
		}
	}

	// 获取角色服务
	var characterService *CharacterService
	// 使用全局依赖注入容器获取角色服务
	container := di.GetContainer()
	if charServiceObj := container.Get("character"); charServiceObj != nil {
		characterService = charServiceObj.(*CharacterService)
	}

	if characterService == nil {
		if isEnglish {
			return nil, fmt.Errorf("character service not available")
		} else {
			return nil, fmt.Errorf("角色服务不可用")
		}
	}

	// 处理每个触发器
	var generatedInteractions []*models.CharacterInteraction
	for i := range triggers {
		// 跳过已触发的
		if triggers[i].Triggered {
			continue
		}

		// 实际项目中，这里应根据条件判断是否触发
		// 简化版本：全部触发

		// 生成角色互动
		interaction, err := characterService.GenerateCharacterInteraction(
			sceneID,
			triggers[i].CharacterIDs,
			triggers[i].Topic,
			triggers[i].ContextDescription,
		)

		if err != nil {
			fmt.Printf("触发角色互动失败: %v\n", err)
			continue
		}

		// 标记为已触发
		triggers[i].Triggered = true
		generatedInteractions = append(generatedInteractions, interaction)
	}

	// 更新触发器状态
	node.Metadata["interaction_triggers"] = triggers

	// 如果有互动被触发，保存故事数据
	if len(generatedInteractions) > 0 {
		if err := s.saveStoryData(sceneID, storyData); err != nil {
			if isEnglish {
				return nil, fmt.Errorf("failed to save story data after triggering interactions: %w", err)
			} else {
				return nil, fmt.Errorf("触发互动后保存故事数据失败: %w", err)
			}
		}
	}

	return generatedInteractions, nil
}

// SaveStoryData 保存故事数据到文件（公开方法）
func (s *StoryService) SaveStoryData(sceneID string, storyData *models.StoryData) error {
	if s == nil {
		return fmt.Errorf("故事服务未初始化")
	}

	// 调用内部的保存方法
	return s.saveStoryData(sceneID, storyData)
}
