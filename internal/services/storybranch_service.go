// internal/services/storybranch_service.go
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

// StoryBranchService 管理故事分支和节点
type StoryBranchService struct {
	BasePath     string
	SceneService *SceneService
	LLMService   *LLMService
}

// NewStoryBranchService 创建故事分支服务
func NewStoryBranchService(basePath string) *StoryBranchService {
	if basePath == "" {
		basePath = "data/story_branches"
	}

	// 创建基础目录
	if err := os.MkdirAll(basePath, 0755); err != nil {
		fmt.Printf("警告: 创建故事分支目录失败: %v\n", err)
	}

	return &StoryBranchService{
		BasePath: basePath,
	}
}

// CreateBranch 创建一个新的故事分支
func (s *StoryBranchService) CreateBranch(sceneID, name string, rootNode *models.StoryNode) (*models.StoryBranch, error) {
	// 创建分支目录
	branchID := fmt.Sprintf("branch_%d", time.Now().UnixNano())
	branchDir := filepath.Join(s.BasePath, sceneID, branchID)
	if err := os.MkdirAll(branchDir, 0755); err != nil {
		return nil, fmt.Errorf("创建分支目录失败: %w", err)
	}

	// 设置根节点ID
	if rootNode.ID == "" {
		rootNode.ID = fmt.Sprintf("node_%d", time.Now().UnixNano())
	}
	rootNode.SceneID = sceneID
	rootNode.CreatedAt = time.Now()

	// 保存根节点
	if err := s.SaveNode(rootNode); err != nil {
		return nil, fmt.Errorf("保存根节点失败: %w", err)
	}

	// 创建分支对象
	branch := &models.StoryBranch{
		ID:         branchID,
		SceneID:    sceneID,
		Name:       name,
		RootNodeID: rootNode.ID,
		IsActive:   true,
		CreatedAt:  time.Now(),
	}

	// 保存分支数据
	branchDataJSON, err := json.MarshalIndent(branch, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化分支数据失败: %w", err)
	}

	branchFilePath := filepath.Join(branchDir, "branch.json")
	if err := os.WriteFile(branchFilePath, branchDataJSON, 0644); err != nil {
		return nil, fmt.Errorf("保存分支数据失败: %w", err)
	}

	return branch, nil
}

// SaveNode 保存故事节点
func (s *StoryBranchService) SaveNode(node *models.StoryNode) error {
	// 确保节点目录存在
	nodeDir := filepath.Join(s.BasePath, node.SceneID, "nodes")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		return fmt.Errorf("创建节点目录失败: %w", err)
	}

	// 序列化节点数据
	nodeDataJSON, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化节点数据失败: %w", err)
	}

	// 保存节点数据
	nodeFilePath := filepath.Join(nodeDir, node.ID+".json")
	if err := os.WriteFile(nodeFilePath, nodeDataJSON, 0644); err != nil {
		return fmt.Errorf("保存节点数据失败: %w", err)
	}

	return nil
}

// GetNode 获取故事节点
func (s *StoryBranchService) GetNode(sceneID, nodeID string) (*models.StoryNode, error) {
	nodeFilePath := filepath.Join(s.BasePath, sceneID, "nodes", nodeID+".json")

	// 读取节点数据
	nodeData, err := os.ReadFile(nodeFilePath)
	if err != nil {
		return nil, fmt.Errorf("读取节点数据失败: %w", err)
	}

	var node models.StoryNode
	if err := json.Unmarshal(nodeData, &node); err != nil {
		return nil, fmt.Errorf("解析节点数据失败: %w", err)
	}

	return &node, nil
}

// GetBranch 获取故事分支
func (s *StoryBranchService) GetBranch(sceneID, branchID string) (*models.StoryBranch, error) {
	branchFilePath := filepath.Join(s.BasePath, sceneID, branchID, "branch.json")

	// 读取分支数据
	branchData, err := os.ReadFile(branchFilePath)
	if err != nil {
		return nil, fmt.Errorf("读取分支数据失败: %w", err)
	}

	var branch models.StoryBranch
	if err := json.Unmarshal(branchData, &branch); err != nil {
		return nil, fmt.Errorf("解析分支数据失败: %w", err)
	}

	return &branch, nil
}

// GetActiveBranch 获取场景的活跃分支
func (s *StoryBranchService) GetActiveBranch(sceneID string) (*models.StoryBranch, error) {
	branchesDir := filepath.Join(s.BasePath, sceneID)

	// 读取分支目录
	entries, err := os.ReadDir(branchesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 没有分支
		}
		return nil, fmt.Errorf("读取分支目录失败: %w", err)
	}

	// 查找活跃分支
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		branchID := entry.Name()
		branch, err := s.GetBranch(sceneID, branchID)
		if err != nil {
			continue // 跳过错误的分支
		}

		if branch.IsActive {
			return branch, nil
		}
	}

	return nil, nil // 没有找到活跃分支
}

// GenerateChoices 使用LLM生成故事选择
func (s *StoryBranchService) GenerateChoices(ctx context.Context, node *models.StoryNode, sceneData *SceneData) ([]models.StoryChoice, error) {
	// 检测语言
	isEnglish := isEnglishText(sceneData.Scene.Name + " " + sceneData.Scene.Description + " " + node.Content)

	// 构建提示词
	var prompt, systemPrompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`Generate 4 meaningful choices for the following story node:

Scene: %s
%s

Current node content:
%s

Generate exactly 4 distinct player choices following these guidelines:

**Choice Variety Requirements**:
1. **Action-based choice**: Direct action or intervention
2. **Dialogue-based choice**: Communication or negotiation approach  
3. **Investigation choice**: Exploration or information gathering
4. **Strategic choice**: Planning or indirect approach

**Quality Standards**:
- Each choice should have 15-30 words for clarity
- Consequences should be meaningful and distinct
- Avoid choices that lead to immediate dead ends
- Balance risk/reward in different ways
- Consider character personality compatibility

**Format each choice as**:
- Text: Clear, specific action description
- Consequence: Brief preview of likely outcome direction`,
			sceneData.Scene.Name,
			sceneData.Scene.Description,
			node.Content)

		systemPrompt = `You are an interactive story expert, specializing in creating meaningful and diverse story branch choices. 
Each choice should bring noticeably different directions to the story. Return 3-4 clear, engaging options.`
	} else {
		// 中文提示词
		prompt = fmt.Sprintf(`为以下故事节点生成4个有意义的选择:

场景: %s
%s

当前节点内容:
%s

准确生成4个不同的玩家选择，遵循以下准则：

**选择多样性要求**：
1. **行动类选择**：直接行动或干预
2. **对话类选择**：沟通或谈判方式
3. **调查类选择**：探索或信息收集
4. **策略类选择**：计划或间接方式

**质量标准**：
- 每个选择应有15-30个字以保证清晰度
- 后果应有意义且不同
- 避免导致即时死胡同的选择
- 以不同方式平衡风险/回报
- 考虑角色个性兼容性

**每个选择的格式**：
- 文本：清晰、具体的行动描述
- 后果：可能结果方向的简要预览`,
			sceneData.Scene.Name,
			sceneData.Scene.Description,
			node.Content)

		systemPrompt = `你是一个互动故事专家，专长于创建有意义且多样化的故事分支选择。
每个选择都应该为故事带来明显不同的发展方向。返回3-4个清晰、引人入胜的选项。`
	}

	type ChoicesResponse struct {
		Choices []struct {
			Text        string `json:"text"`
			Consequence string `json:"consequence"`
		} `json:"choices"`
	}

	var response ChoicesResponse
	err := s.LLMService.CreateStructuredCompletion(ctx, prompt, systemPrompt, &response)
	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to generate choices: %w", err)
		} else {
			return nil, fmt.Errorf("生成选择失败: %w", err)
		}
	}

	// 转换为模型格式
	choices := make([]models.StoryChoice, len(response.Choices))
	for i, c := range response.Choices {
		choices[i] = models.StoryChoice{
			ID:          fmt.Sprintf("choice_%d", time.Now().UnixNano()+int64(i)),
			Text:        c.Text,
			Consequence: c.Consequence,
			CreatedAt:   time.Now(),
		}
	}

	return choices, nil
}

// CreateNextNode 创建下一个故事节点
func (s *StoryBranchService) CreateNextNode(ctx context.Context, parentNode *models.StoryNode, choice *models.StoryChoice, sceneData *SceneData) (*models.StoryNode, error) {
	// 检测语言
	isEnglish := isEnglishText(sceneData.Scene.Name + " " + sceneData.Scene.Description + " " + parentNode.Content)

	// 构建提示词
	var prompt, systemPrompt string

	if isEnglish {
		// 英文提示词
		prompt = fmt.Sprintf(`Create the next story node based on the following information:

Scene: %s
%s

Previous node content:
%s

User choice:
%s

Please create a new story node that describes what happens after the user's choice. The content should be vivid and interesting, creating a new direction for the story.`,
			sceneData.Scene.Name,
			sceneData.Scene.Description,
			parentNode.Content,
			choice.Text)

		systemPrompt = `You are a master interactive storyteller specializing in dynamic narrative progression.

**Content Creation Standards**:
- **Sensory Details**: Include vivid sensory descriptions (sight, sound, atmosphere)
- **Character Development**: Show how the choice affects character growth or relationships
- **World Building**: Reveal new aspects of the story world through consequences
- **Emotional Resonance**: Create emotional impact that matches the choice's significance
- **Forward Momentum**: End with a natural setup for the next decision point

**Narrative Techniques**:
- Use active voice and present tense for immediacy
- Include internal character thoughts or reactions when appropriate
- Show consequences through actions and dialogue, not just exposition
- Maintain consistency with established character personalities and world rules
- Create content that feels substantial (100-200 words) but not overwhelming

**Story Continuity**: Ensure the new content flows naturally from the previous node while opening new narrative possibilities.`
	} else {
		// 中文提示词
		prompt = fmt.Sprintf(`基于以下信息创建下一个故事节点:

场景: %s
%s

上一个节点内容:
%s

用户选择:
%s

请创建一个新的故事节点，描述用户选择后发生的事件。内容应该生动有趣，并为故事创造新的发展方向。`,
			sceneData.Scene.Name,
			sceneData.Scene.Description,
			parentNode.Content,
			choice.Text)

		systemPrompt = `你是一位精通互动叙事的大师级故事创作者，专长于动态叙事推进。

**内容创作标准**：
- **感官细节**：包含生动的感官描述（视觉、听觉、氛围）
- **角色发展**：展示选择如何影响角色成长或关系
- **世界构建**：通过后果揭示故事世界的新方面
- **情感共鸣**：创造与选择重要性相匹配的情感影响
- **前进动力**：以自然的下一个决策点设置结束

**叙事技巧**：
- 使用主动语态和现在时增强即时感
- 适当时包含角色内心想法或反应
- 通过行动和对话而非仅仅说明来展示后果
- 保持与既定角色个性和世界规则的一致性
- 创造感觉充实（100-200字）但不压倒性的内容

**故事连续性**：确保新内容从前一节点自然流动，同时开启新的叙事可能性。`
	}

	type NodeResponse struct {
		Content string `json:"content"`
		Type    string `json:"type"`
	}

	var response NodeResponse
	err := s.LLMService.CreateStructuredCompletion(ctx, prompt, systemPrompt, &response)
	if err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to generate node content: %w", err)
		} else {
			return nil, fmt.Errorf("生成节点内容失败: %w", err)
		}
	}

	// 创建新节点
	nextNode := &models.StoryNode{
		ID:        fmt.Sprintf("node_%d", time.Now().UnixNano()),
		SceneID:   parentNode.SceneID,
		ParentID:  parentNode.ID,
		Type:      response.Type,
		Content:   response.Content,
		CreatedAt: time.Now(),
	}

	// 保存节点
	if err := s.SaveNode(nextNode); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to save new node: %w", err)
		} else {
			return nil, fmt.Errorf("保存新节点失败: %w", err)
		}
	}

	// 更新选择指向新节点
	choice.NextNodeID = nextNode.ID
	choice.Selected = true

	// 更新父节点的选择
	for i, c := range parentNode.Choices {
		if c.ID == choice.ID {
			parentNode.Choices[i] = *choice
			break
		}
	}

	// 保存更新后的父节点
	if err := s.SaveNode(parentNode); err != nil {
		if isEnglish {
			return nil, fmt.Errorf("failed to update parent node: %w", err)
		} else {
			return nil, fmt.Errorf("更新父节点失败: %w", err)
		}
	}

	return nextNode, nil
}
