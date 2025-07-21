// internal/api/handlers.go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/llm"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/services"
	"github.com/gin-gonic/gin"
)

// Handler 处理API请求
type Handler struct {
	// 核心服务
	SceneService     *services.SceneService     // 场景服务
	CharacterService *services.CharacterService // 角色服务
	ContextService   *services.ContextService   // 上下文服务
	ProgressService  *services.ProgressService  // 进度跟踪服务
	AnalyzerService  *services.AnalyzerService  // 分析服务
	ConfigService    *services.ConfigService    // 配置服务
	StatsService     *services.StatsService     // 统计服务
	UserService      *services.UserService      // 用户服务
	WebSocketHandler *WebSocketHandler          // WebSocket 处理器
	Response         *ResponseHelper            // 响应助手
}

// TriggerCharacterInteractionRequest 触发角色互动的请求结构
type TriggerCharacterInteractionRequest struct {
	SceneID            string   `json:"scene_id"`            // 场景ID
	CharacterIDs       []string `json:"character_ids"`       // 参与互动的角色ID列表
	Topic              string   `json:"topic"`               // 互动主题
	ContextDescription string   `json:"context_description"` // 互动背景描述
}

// SimulateConversationRequest 模拟多轮对话的请求结构
type SimulateConversationRequest struct {
	SceneID          string   `json:"scene_id"`          // 场景ID
	CharacterIDs     []string `json:"character_ids"`     // 参与互动的角色ID列表
	InitialSituation string   `json:"initial_situation"` // 初始情境
	NumberOfTurns    int      `json:"number_of_turns"`   // 对话轮数
}

// APIResponse 标准API响应格式
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Message   string      `json:"message,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"` // 用于调试和追踪
}

// APIError 标准错误格式
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// PaginationMeta 分页元数据
type PaginationMeta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// PaginatedResponse 带分页的响应
type PaginatedResponse struct {
	*APIResponse
	Meta *PaginationMeta `json:"meta,omitempty"`
}

// ------------------------------------------------
// SceneWebSocket 处理场景 WebSocket 连接
func (h *Handler) SceneWebSocket(c *gin.Context) {
	h.WebSocketHandler.SceneWebSocket(c)
}

// UserStatusWebSocket 处理用户状态 WebSocket 连接
func (h *Handler) UserStatusWebSocket(c *gin.Context) {
	h.WebSocketHandler.UserStatusWebSocket(c)
}

// BroadcastToScene 提供外部调用的广播方法
func (h *Handler) BroadcastToScene(sceneID string, message map[string]interface{}) {
	wsManager.BroadcastToScene(sceneID, message)
}

// GetWebSocketStatus 获取 WebSocket 连接状态（调试用）
func (h *Handler) GetWebSocketStatus(c *gin.Context) {
	status := wsManager.GetStatus()
	status["ping_timeout_seconds"] = int(wsManager.pingTimeout.Seconds())
	status["timestamp"] = time.Now().Format(time.RFC3339)

	c.JSON(http.StatusOK, status)
}

// 添加管理器控制API
func (h *Handler) CleanupWebSocketConnections(c *gin.Context) {
	wsManager.cleanupExpiredConnections()
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "连接清理已执行",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// ========================================
// 导出功能处理器
// ========================================

// ExportScene 导出场景数据
func (h *Handler) ExportScene(c *gin.Context) {
	sceneID := c.Param("id")
	format := c.DefaultQuery("format", "json")
	includeConversations := c.DefaultQuery("include_conversations", "false") == "true"

	exportService := h.getExportService()
	if exportService == nil {
		h.Response.InternalError(c, "导出服务未初始化", "无法获取导出服务实例")
		return
	}

	result, err := exportService.ExportSceneData(c.Request.Context(), sceneID, format, includeConversations)
	if err != nil {
		h.Response.InternalError(c, "导出场景数据失败", err.Error())
		return
	}

	// 使用统一的导出响应方法
	h.Response.ExportResponse(c, result, format)
}

// ExportInteractions 导出互动摘要
func (h *Handler) ExportInteractions(c *gin.Context) {
	sceneID := c.Param("id")
	format := c.DefaultQuery("format", "json")

	// 验证场景ID
	if sceneID == "" {
		h.Response.BadRequest(c, "缺少场景ID")
		return
	}

	// 验证导出格式
	supportedFormats := []string{"json", "markdown", "txt", "html", "csv"}
	if !contains(supportedFormats, strings.ToLower(format)) {
		h.Response.BadRequest(c, "不支持的导出格式", fmt.Sprintf("支持的格式: %v", supportedFormats))
		return
	}
	// 获取导出服务
	exportService := h.getExportService()
	if exportService == nil {
		h.Response.InternalError(c, "导出服务未初始化", "无法获取导出服务实例")
		return
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 导出互动摘要
	result, err := exportService.ExportInteractionSummary(ctx, sceneID, format)
	if err != nil {
		// 根据错误类型返回不同的错误码
		if ctx.Err() == context.DeadlineExceeded {
			h.Response.Error(c, http.StatusRequestTimeout, ErrorExportTimeout,
				"导出操作超时", "请稍后重试或联系管理员")
			return
		}

		if strings.Contains(err.Error(), "no data") {
			h.Response.Error(c, http.StatusNotFound, ErrorExportDataEmpty,
				"没有可导出的数据", "场景中没有互动记录")
			return
		}

		h.Response.Error(c, http.StatusInternalServerError, ErrorExportFailed,
			"导出互动摘要失败", err.Error())
		return
	}

	// 检查导出结果
	if result == nil || result.Content == "" {
		h.Response.Error(c, http.StatusNotFound, ErrorExportDataEmpty,
			"导出结果为空", "没有找到可导出的数据")
		return
	}

	// 使用专用的导出响应方法
	h.Response.ExportResponse(c, result, format)
}

// ExportStory 导出故事文档
func (h *Handler) ExportStory(c *gin.Context) {
	sceneID := c.Param("id")
	format := c.DefaultQuery("format", "json")

	// 验证场景ID
	if sceneID == "" {
		h.Response.BadRequest(c, "缺少场景ID")
		return
	}

	// 验证导出格式
	supportedFormats := []string{"json", "markdown", "txt", "html", "pdf"}
	if !contains(supportedFormats, strings.ToLower(format)) {
		h.Response.BadRequest(c, "不支持的导出格式", fmt.Sprintf("支持的格式: %v", supportedFormats))
		return
	}

	// 获取导出服务
	exportService := h.getExportService()
	if exportService == nil {
		h.Response.Error(c, http.StatusServiceUnavailable, ErrorExportServiceUnavailable,
			"导出服务未初始化", "无法获取导出服务实例")
		return
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()

	// 导出故事文档
	result, err := exportService.ExportStoryAsDocument(ctx, sceneID, format)
	if err != nil {
		// 根据错误类型返回不同的错误码
		if ctx.Err() == context.DeadlineExceeded {
			h.Response.Error(c, http.StatusRequestTimeout, ErrorExportTimeout,
				"导出操作超时", "故事文档较大，请稍后重试")
			return
		}

		if strings.Contains(err.Error(), "no story data") {
			h.Response.Error(c, http.StatusNotFound, ErrorExportDataEmpty,
				"没有可导出的故事数据", "场景中没有故事记录")
			return
		}

		h.Response.Error(c, http.StatusInternalServerError, ErrorExportFailed,
			"导出故事文档失败", err.Error())
		return
	}

	// 检查导出结果
	if result == nil || result.Content == "" {
		h.Response.Error(c, http.StatusNotFound, ErrorExportDataEmpty,
			"导出结果为空", "没有找到可导出的故事数据")
		return
	}

	// 使用专用的导出响应方法
	h.Response.ExportResponse(c, result, format)
}

// 辅助函数：检查字符串是否在切片中
func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

// getExportService 获取导出服务实例
func (h *Handler) getExportService() *services.ExportService {
	container := di.GetContainer()
	exportService, ok := container.Get("export").(*services.ExportService)
	if !ok {
		log.Printf("警告: 无法从容器获取导出服务")
		return nil
	}
	return exportService
}

// ---------------------------------------------------------
// NewHandler 创建API处理器
func NewHandler(
	sceneService *services.SceneService,
	characterService *services.CharacterService,
	contextService *services.ContextService,
	progressService *services.ProgressService,
	analyzerService *services.AnalyzerService,
	configService *services.ConfigService,
	statsService *services.StatsService,
	userService *services.UserService) *Handler {

	return &Handler{
		SceneService:     sceneService,
		CharacterService: characterService,
		ContextService:   contextService,
		ProgressService:  progressService,
		AnalyzerService:  analyzerService,
		ConfigService:    configService,
		StatsService:     statsService,
		UserService:      userService,
		WebSocketHandler: NewWebSocketHandler(),
		Response:         NewResponseHelper(),
	}
}

// GetScenes 获取所有场景列表
func (h *Handler) GetScenes(c *gin.Context) {
	scenes, err := h.SceneService.GetAllScenes()
	if err != nil {
		h.Response.InternalError(c, "获取场景列表失败", err.Error())
		return
	}

	h.Response.Success(c, scenes, "场景列表获取成功")
}

// GetScene 获取指定场景详情
func (h *Handler) GetScene(c *gin.Context) {
	sceneID := c.Param("id")
	sceneData, err := h.SceneService.LoadScene(sceneID)
	if err != nil {
		h.Response.NotFound(c, "场景", "场景ID: "+sceneID)
		return
	}

	h.Response.Success(c, sceneData, "场景数据获取成功")
}

// CreateScene 从文本创建新场景
func (h *Handler) CreateScene(c *gin.Context) {
	var req struct {
		Title string `json:"title" binding:"required"`
		Text  string `json:"text" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.Response.BadRequest(c, "请求参数错误", err.Error())
		return
	}

	// 创建场景
	scene, err := h.SceneService.CreateSceneFromText(req.Text, req.Title)
	if err != nil {
		h.Response.InternalError(c, "创建场景失败", err.Error())
		return
	}

	h.Response.Created(c, scene, "场景创建成功")
}

// GetCharacters 获取指定场景的所有角色
func (h *Handler) GetCharacters(c *gin.Context) {
	sceneID := c.Param("id")
	sceneData, err := h.SceneService.LoadScene(sceneID)
	if err != nil {
		h.Response.NotFound(c, "场景", "场景ID: "+sceneID)
		return
	}

	h.Response.Success(c, sceneData.Characters, "角色列表获取成功")
}

// Chat 处理聊天请求
func (h *Handler) Chat(c *gin.Context) {
	var req struct {
		SceneID     string `json:"scene_id" binding:"required"`
		CharacterID string `json:"character_id" binding:"required"`
		Message     string `json:"message" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.Response.BadRequest(c, "请求参数错误", err.Error())
		return
	}

	// 生成角色回应
	response, err := h.CharacterService.GenerateResponse(req.SceneID, req.CharacterID, req.Message)
	if err != nil {
		h.Response.InternalError(c, "生成回应失败", err.Error())
		return
	}

	h.Response.Success(c, response, "回应生成成功")
}

// GetConversations 获取对话历史
func (h *Handler) GetConversations(c *gin.Context) {
	sceneID := c.Param("id")
	limitStr := c.DefaultQuery("limit", "20")
	page := c.DefaultQuery("page", "1")

	var limit int
	if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
		limit = 20
	}

	var pageNum int
	if _, err := fmt.Sscanf(page, "%d", &pageNum); err != nil {
		pageNum = 1
	}

	conversations, err := h.ContextService.GetRecentConversations(sceneID, limit)
	if err != nil {
		h.Response.InternalError(c, "获取对话失败", err.Error())
		return
	}

	// 如果需要分页，计算分页信息
	if c.Query("paginated") == "true" {
		// 这里需要从服务层获取总数
		total := len(conversations) // 简化处理，实际应该从数据库获取
		meta := &PaginationMeta{
			Page:       pageNum,
			PerPage:    limit,
			Total:      total,
			TotalPages: (total + limit - 1) / limit,
		}
		h.Response.PaginatedSuccess(c, conversations, meta, "对话历史获取成功")
	} else {
		h.Response.Success(c, conversations, "对话历史获取成功")
	}
}

// UploadFile 处理文件上传
func (h *Handler) UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取上传文件失败"})
		return
	}

	// 检查文件类型
	ext := filepath.Ext(file.Filename)
	if ext != ".txt" && ext != ".md" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持.txt或.md文件"})
		return
	}

	// 存储临时文件
	tempPath := filepath.Join("temp", time.Now().Format("20060102150405")+ext)
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	// 读取文件内容
	content, err := os.ReadFile(tempPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	// 返回文件内容和文件名
	c.JSON(http.StatusOK, gin.H{
		"filename": file.Filename,
		"content":  string(content),
	})

	// 删除临时文件
	_ = os.Remove(tempPath)
}

// IndexPage 返回主页
func (h *Handler) IndexPage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

// SceneSelectorPage 返回场景选择页面
func (h *Handler) SceneSelectorPage(c *gin.Context) {
	scenes, err := h.SceneService.GetAllScenes()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "获取场景列表失败: " + err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "scene_selector.html", gin.H{
		"scenes": scenes,
	})
}

// CreateScenePage 返回创建场景页面
func (h *Handler) CreateScenePage(c *gin.Context) {
	c.HTML(http.StatusOK, "create_scene.html", nil)
}

// ScenePage 返回场景交互页面
func (h *Handler) ScenePage(c *gin.Context) {
	sceneID := c.Param("id")
	sceneData, err := h.SceneService.LoadScene(sceneID)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"error":      "Scene not found",
			"timestamp":  time.Now().Format(time.RFC3339),
			"request_id": c.GetString("request_id"), // 需要中间件设置
			"error_code": "404",
		})
		return
	}

	c.HTML(http.StatusOK, "scene.html", gin.H{
		"scene":      sceneData.Scene,
		"characters": sceneData.Characters,
	})
}

// AnalyzeTextWithProgress 处理文本分析请求，返回任务ID
func (h *Handler) AnalyzeTextWithProgress(c *gin.Context) {
	// 解析请求
	var req struct {
		Text  string `json:"text" binding:"required"`
		Title string `json:"title" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 创建唯一任务ID
	taskID := fmt.Sprintf("analyze_%d", time.Now().UnixNano())

	// 创建进度跟踪器
	tracker := h.ProgressService.CreateTracker(taskID)

	// 启动后台分析
	go func() {
		// 创建任务级别context，支持超时和取消
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// 执行分析
		result, err := h.AnalyzerService.AnalyzeTextWithProgress(ctx, req.Text, tracker)
		if err != nil {
			tracker.Fail(err.Error())
			return
		}

		// 分析完成后创建场景
		scene := &models.Scene{
			ID:          fmt.Sprintf("scene_%d", time.Now().UnixNano()),
			Name:        req.Title,
			CreatedAt:   time.Now(),
			LastUpdated: time.Now(),
		}

		// 如果分析结果包含场景信息，则使用第一个场景的数据
		if len(result.Scenes) > 0 {
			firstScene := result.Scenes[0]
			scene.Description = firstScene.Description
			scene.Era = firstScene.Era
			scene.Themes = firstScene.Themes
			scene.Locations = firstScene.Locations
			// 如果名称为空，才使用请求中的标题
			if firstScene.Name != "" {
				scene.Name = firstScene.Name
			}
		} else {
			// 没有分析到场景，使用默认值
			scene.Description = "基于文本分析创建的场景"
		}

		// 保存场景和角色
		if err := h.SceneService.CreateSceneWithCharacters(scene, result.Characters); err != nil {
			tracker.Fail("场景创建失败: " + err.Error())
			return
		}

		// 更新任务状态，包含创建的场景ID
		tracker.Complete(fmt.Sprintf("分析完成，场景已创建: %s", scene.ID))
	}()

	// 返回任务ID
	c.JSON(http.StatusAccepted, gin.H{
		"task_id": taskID,
		"message": "文本分析已开始，请订阅进度更新",
	})
}

// SubscribeProgress 订阅任务进度的SSE端点
func (h *Handler) SubscribeProgress(c *gin.Context) {
	taskID := c.Param("taskID")

	// 获取进度跟踪器
	tracker, exists := h.ProgressService.GetTracker(taskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	// 设置SSE响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	// 获取客户端连接
	clientGone := c.Request.Context().Done()

	// 订阅进度更新
	updateChan := tracker.Subscribe()
	defer tracker.Unsubscribe(updateChan)

	// 发送心跳和更新
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// 发送初始事件保持连接打开
	fmt.Fprintf(c.Writer, "event: connected\ndata: {\"message\":\"连接已建立\"}\n\n")
	c.Writer.Flush()

	for {
		select {
		case <-clientGone:
			// 客户端断开连接
			return
		case update, ok := <-updateChan:
			if !ok {
				// 通道已关闭
				return
			}
			// 发送进度更新
			data, _ := json.Marshal(update)
			fmt.Fprintf(c.Writer, "event: progress\ndata: %s\n\n", string(data))
			c.Writer.Flush()

			// 如果任务已完成或失败，结束连接
			if update.Status == "completed" || update.Status == "failed" {
				return
			}
		case <-ticker.C:
			// 发送心跳以保持连接
			fmt.Fprintf(c.Writer, "event: heartbeat\ndata: {\"time\":%d}\n\n", time.Now().Unix())
			c.Writer.Flush()
		}
	}
}

// CancelAnalysisTask 取消正在进行的分析任务
func (h *Handler) CancelAnalysisTask(c *gin.Context) {
	taskID := c.Param("taskID")

	// 获取进度跟踪器
	tracker, exists := h.ProgressService.GetTracker(taskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	// 标记任务为失败
	tracker.Fail("用户取消了任务")

	c.JSON(http.StatusOK, gin.H{"message": "任务已取消"})
}

// ChatWithEmotion 处理带情绪的聊天请求
func (h *Handler) ChatWithEmotion(c *gin.Context) {
	var req struct {
		SceneID     string `json:"scene_id" binding:"required"`
		CharacterID string `json:"character_id" binding:"required"`
		Message     string `json:"message" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 使用新的方法生成带情绪的回应
	response, err := h.CharacterService.GenerateResponseWithEmotion(req.SceneID, req.CharacterID, req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("生成回应失败: %v", err)})
		return
	}
	// 记录API使用情况（假设tokenCount是从LLM响应中获取的）
	h.StatsService.RecordAPIRequest(response.TokensUsed)
	c.JSON(http.StatusOK, response)
}

// GetStoryData 获取指定场景的故事数据
func (h *Handler) GetStoryData(c *gin.Context) {
	sceneID := c.Param("id")
	storyService := h.getStoryService()

	storyData, err := storyService.GetStoryData(sceneID, nil)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "故事数据不存在"})
		return
	}

	c.JSON(http.StatusOK, storyData)
}

// MakeStoryChoice 处理故事选择逻辑
func (h *Handler) MakeStoryChoice(c *gin.Context) {
	sceneID := c.Param("id")

	var req struct {
		NodeID          string                  `json:"node_id" binding:"required"`
		ChoiceID        string                  `json:"choice_id" binding:"required"`
		UserPreferences *models.UserPreferences `json:"user_preferences,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误: " + err.Error()})
		return
	}

	// 获取故事服务
	storyService := h.getStoryService()
	if storyService == nil {
		h.Response.InternalError(c, "故事服务未初始化", "无法获取故事服务实例")
		return
	}

	// 执行故事选择（并发安全）
	nextNode, err := storyService.MakeChoice(sceneID, req.NodeID, req.ChoiceID, req.UserPreferences)
	if err != nil {
		if strings.Contains(err.Error(), "选择已被选中") {
			h.Response.Conflict(c, err.Error())
			return
		}
		if strings.Contains(err.Error(), "无效的节点或选择") {
			h.Response.BadRequest(c, err.Error())
			return
		}
		h.Response.InternalError(c, "执行故事选择失败", err.Error())
		return
	}

	// 获取更新后的故事数据
	storyData, err := storyService.GetStoryData(sceneID, req.UserPreferences)
	if err != nil {
		h.Response.InternalError(c, "获取故事数据失败", err.Error())
		return
	}

	result := map[string]interface{}{
		"next_node":  nextNode,
		"story_data": storyData,
	}

	h.Response.Success(c, result, "选择执行成功")
}

// BatchStoryOperations 批量故事操作
func (h *Handler) BatchStoryOperations(c *gin.Context) {
	sceneID := c.Param("id")

	var req struct {
		Operations []struct {
			Type string      `json:"type"`
			Data interface{} `json:"data"`
		} `json:"operations"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	storyService := h.getStoryService()
	if storyService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "故事服务不可用"})
		return
	}

	// 执行批量操作
	err := storyService.ExecuteBatchOperation(sceneID, func(storyData *models.StoryData) error {
		for _, op := range req.Operations {
			switch op.Type {
			case "complete_objective":
				// 处理完成目标操作
			case "unlock_location":
				// 处理解锁地点操作
			case "add_item":
				// 处理添加物品操作
			default:
				return fmt.Errorf("未知操作类型: %s", op.Type)
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "批量操作失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "批量操作执行成功",
	})
}

// AdvanceStory 推进故事情节
func (h *Handler) AdvanceStory(c *gin.Context) {
	sceneID := c.Param("id")

	if sceneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少场景ID"})
		return
	}

	// 解析请求体中的偏好设置
	var req struct {
		UserPreferences *models.UserPreferences `json:"user_preferences,omitempty"`
	}

	// 尝试解析请求体，如果失败则使用默认偏好
	if err := c.ShouldBindJSON(&req); err != nil {
		req.UserPreferences = nil // 使用默认偏好
	}

	// 获取StoryService实例
	storyService := h.getStoryService()
	if storyService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "故事服务未初始化"})
		return
	}

	// 如果没有偏好设置，使用默认值
	preferences := req.UserPreferences
	if preferences == nil {
		preferences = &models.UserPreferences{
			CreativityLevel:   models.CreativityBalanced,
			AllowPlotTwists:   true,
			ResponseLength:    "medium",
			LanguageStyle:     "casual",
			NotificationLevel: "important",
			DarkMode:          false,
		}
	}

	// 推进故事
	storyUpdate, err := storyService.AdvanceStory(sceneID, preferences)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("推进故事失败: %v", err)})
		return
	}

	// 获取更新后的故事数据
	storyData, err := storyService.GetStoryData(sceneID, preferences)
	if err != nil {
		// 即使获取完整数据失败，也返回更新信息
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"message":      "故事已推进",
			"story_update": storyUpdate,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "故事已推进",
		"story_update": storyUpdate,
		"story_data":   storyData,
	})
}

// RewindStory 回溯故事到指定节点
func (h *Handler) RewindStory(c *gin.Context) {
	sceneID := c.Param("scene_id")

	var req struct {
		NodeID string `json:"node_id" binding:"required"` // 目标节点ID
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	// 验证参数
	if sceneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少场景ID"})
		return
	}

	// 获取StoryService实例
	storyService := h.getStoryService()
	if storyService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "故事服务未初始化"})
		return
	}

	// 创建超时上下文
	_, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	// 执行回溯操作
	storyData, err := storyService.RewindToNode(sceneID, req.NodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("回溯故事失败: %v", err)})
		return
	}

	// 构建分支视图数据
	branchView := buildStoryBranchView(storyData)

	// 获取回溯到的节点信息
	var targetNode *models.StoryNode
	for i := range storyData.Nodes {
		if storyData.Nodes[i].ID == req.NodeID {
			targetNode = &storyData.Nodes[i]
			break
		}
	}

	// 记录API使用情况
	if h.StatsService != nil {
		h.StatsService.RecordAPIRequest(2) // 回溯操作相对简单
	}

	response := gin.H{
		"success":        true,
		"message":        "故事已成功回溯",
		"story_data":     branchView,
		"progress":       storyData.Progress,
		"current_state":  storyData.CurrentState,
		"target_node_id": req.NodeID,
	}

	// 添加目标节点信息（如果找到）
	if targetNode != nil {
		response["target_node"] = map[string]interface{}{
			"id":         targetNode.ID,
			"content":    targetNode.Content,
			"type":       targetNode.Type,
			"created_at": targetNode.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetStoryBranches 获取场景的所有故事分支
func (h *Handler) GetStoryBranches(c *gin.Context) {
	sceneID := c.Param("id")

	// 获取StoryService实例
	storyService := h.getStoryService()
	if storyService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "故事服务未初始化"})
		return
	}

	// 解析用户偏好设置（支持查询参数）
	var preferences *models.UserPreferences
	if prefJSON := c.Query("preferences"); prefJSON != "" {
		preferences = &models.UserPreferences{}
		if err := json.Unmarshal([]byte(prefJSON), preferences); err != nil {
			// 解析失败，记录日志但继续使用默认值
			log.Printf("解析用户偏好失败: %v", err)
			preferences = nil
		}
	}

	// 获取故事数据
	storyData, err := storyService.GetStoryData(sceneID, preferences)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取故事数据失败: %v", err)})
		return
	}

	// 构建分支视图数据
	branchView := buildStoryBranchView(storyData)

	c.JSON(http.StatusOK, branchView)
}

// 获取StoryService实例（从DI容器或其他地方）
func (h *Handler) getStoryService() *services.StoryService {
	// 从依赖注入容器获取
	container := di.GetContainer()
	storyService, ok := container.Get("story").(*services.StoryService)
	if !ok {
		log.Printf("警告: 无法从容器获取故事服务")
		return nil
	}
	return storyService
}

// 构建故事分支视图结构
func buildStoryBranchView(storyData *models.StoryData) map[string]interface{} {
	// 构建节点映射，方便查找
	nodeMap := make(map[string]*models.StoryNode, len(storyData.Nodes))
	for i := range storyData.Nodes {
		nodeMap[storyData.Nodes[i].ID] = &storyData.Nodes[i]
	}

	// 构建节点树
	rootNodes := make([]*models.StoryNode, 0)
	childrenMap := make(map[string][]*models.StoryNode)

	// 找出根节点和子节点
	for i := range storyData.Nodes {
		node := &storyData.Nodes[i]
		if node.ParentID == "" {
			rootNodes = append(rootNodes, node)
		} else if node.IsRevealed {
			// 只添加已揭示的节点
			children := childrenMap[node.ParentID]
			childrenMap[node.ParentID] = append(children, node)
		}
	}

	// 标记当前活跃路径
	currentPath := findCurrentPath(storyData.Nodes)

	return map[string]interface{}{
		"scene_id":       storyData.SceneID,
		"intro":          storyData.Intro,
		"main_objective": storyData.MainObjective,
		"current_state":  storyData.CurrentState,
		"progress":       storyData.Progress,
		"root_nodes":     serializeNodeTree(rootNodes, childrenMap, currentPath),
		"current_path":   currentPath,
	}
}

// 序列化节点树为前端友好的格式
func serializeNodeTree(nodes []*models.StoryNode, childrenMap map[string][]*models.StoryNode, currentPath map[string]bool) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(nodes))

	for _, node := range nodes {
		if !node.IsRevealed {
			continue // 跳过未揭示的节点
		}

		// 序列化选择
		choices := make([]map[string]interface{}, 0, len(node.Choices))
		for _, choice := range node.Choices {
			serializedChoice := map[string]interface{}{
				"id":           choice.ID,
				"text":         choice.Text,
				"consequence":  choice.Consequence,
				"selected":     choice.Selected,
				"next_node_id": choice.NextNodeID,
			}
			choices = append(choices, serializedChoice)
		}

		// 序列化节点
		nodeData := map[string]interface{}{
			"id":         node.ID,
			"content":    node.Content,
			"type":       node.Type,
			"choices":    choices,
			"created_at": node.CreatedAt,
			"is_active":  currentPath[node.ID],
		}

		// 递归处理子节点
		children := childrenMap[node.ID]
		if len(children) > 0 {
			nodeData["children"] = serializeNodeTree(children, childrenMap, currentPath)
		}

		result = append(result, nodeData)
	}

	return result
}

// 找出当前活跃路径上的所有节点
func findCurrentPath(nodes []models.StoryNode) map[string]bool {
	path := make(map[string]bool)

	// 找出最新的已选择的节点
	var latestNode *models.StoryNode
	latestTime := time.Time{}

	for i := range nodes {
		node := &nodes[i]
		if node.IsRevealed && node.CreatedAt.After(latestTime) {
			// 检查是否有已选择的选项
			hasSelectedChoice := false
			for _, choice := range node.Choices {
				if choice.Selected {
					hasSelectedChoice = true
					break
				}
			}

			// 优先选择有已选择选项的节点
			if hasSelectedChoice || latestNode == nil {
				latestNode = node
				latestTime = node.CreatedAt
			}
		}
	}

	// 回溯构建活跃路径
	if latestNode != nil {
		// 添加当前节点到路径
		currentNode := latestNode
		for currentNode != nil {
			path[currentNode.ID] = true

			// 如果没有父节点ID，则结束
			if currentNode.ParentID == "" {
				break
			}

			// 查找父节点
			parentID := currentNode.ParentID
			currentNode = nil
			for i := range nodes {
				if nodes[i].ID == parentID {
					currentNode = &nodes[i]
					break
				}
			}
		}
	}

	return path
}

// RewindStoryToNode 回溯故事到指定节点
func (h *Handler) RewindStoryToNode(c *gin.Context) {
	sceneID := c.Param("id")

	var req struct {
		NodeID string `json:"node_id" binding:"required"` // 目标节点ID
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 验证参数
	if sceneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少场景ID"})
		return
	}

	// 获取StoryService实例
	storyService := h.getStoryService()
	if storyService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "故事服务未初始化"})
		return
	}

	// 执行回溯操作
	storyData, err := storyService.RewindToNode(sceneID, req.NodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("回溯故事失败: %v", err)})
		return
	}

	// 构建分支视图数据
	branchView := buildStoryBranchView(storyData)

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "故事已成功回溯",
		"story_data":  branchView,
		"target_node": req.NodeID,
	})
}

// SettingsPage 返回设置页面
func (h *Handler) SettingsPage(c *gin.Context) {
	// 从配置服务获取当前设置
	config := h.ConfigService.GetCurrentConfig()
	stats := h.StatsService.GetUsageStats()

	c.HTML(http.StatusOK, "setting.html", gin.H{
		"current_provider": config.LLMProvider,
		"current_model":    config.LLMConfig["model"],
		"debug_mode":       config.DebugMode,
		"today_requests":   stats.TodayRequests,
		"monthly_tokens":   stats.MonthlyTokens,
	})
}

// 添加这个方法，作为前端 API.getSettings() 的对应接口
func (h *Handler) GetSettings(c *gin.Context) {
	cfg := config.GetCurrentConfig()

	llmConfig := make(map[string]interface{})
	if cfg.LLMConfig != nil {
		llmConfig["model"] = cfg.LLMConfig["model"]
		llmConfig["has_api_key"] = cfg.LLMConfig["api_key"] != ""
	}

	data := map[string]interface{}{
		"llm_provider": cfg.LLMProvider,
		"debug_mode":   cfg.DebugMode,
		"port":         cfg.Port,
		"llm_config":   llmConfig,
	}

	h.Response.Success(c, data, "设置获取成功")
}

// 添加通用的设置保存方法
func (h *Handler) SaveSettings(c *gin.Context) {
	var request struct {
		LLMProvider string            `json:"llm_provider"`
		LLMConfig   map[string]string `json:"llm_config"`
		DebugMode   bool              `json:"debug_mode"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.Response.BadRequest(c, "无效的请求数据", err.Error())
		return
	}

	// 保存LLM配置
	if request.LLMProvider != "" && request.LLMConfig != nil {
		err := h.ConfigService.UpdateLLMConfig(request.LLMProvider, request.LLMConfig, "web_ui")
		if err != nil {
			h.Response.InternalError(c, "保存LLM配置失败", err.Error())
			return
		}
	}

	h.Response.Success(c, nil, "设置保存成功")
}

// 添加连接测试方法
func (h *Handler) TestConnection(c *gin.Context) {
	container := di.GetContainer()
	llmService, ok := container.Get("llm").(*services.LLMService)
	if !ok {
		h.Response.InternalError(c, "无法获取LLM服务实例")
		return
	}

	if llmService.IsReady() {
		// 尝试一个简单的测试调用
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		// 简单的连接测试
		request := services.ChatCompletionRequest{
			Messages: []services.ChatCompletionMessage{
				{
					Role:    services.RoleSystem,
					Content: "You are a helpful assistant.",
				},
				{
					Role:    services.RoleUser,
					Content: "Hello",
				},
			},
			Model:       "", // 使用默认模型
			Temperature: 0.1,
			MaxTokens:   5,
		}

		_, err := llmService.CreateChatCompletion(ctx, request)

		if err != nil {
			h.Response.Error(c, http.StatusServiceUnavailable, "CONNECTION_TEST_FAILED",
				"连接测试失败", err.Error())
			return
		}

		data := map[string]interface{}{
			"provider": llmService.GetProviderName(),
			"status":   "connected",
			"test":     "passed",
		}
		h.Response.Success(c, data, "连接测试成功")
	} else {
		h.Response.Error(c, http.StatusServiceUnavailable, "CONNECTION_FAILED",
			"LLM服务未就绪", llmService.GetReadyState())
	}
}

// GetLLMStatus 获取LLM服务状态
func (h *Handler) GetLLMStatus(c *gin.Context) {
	// 获取LLM服务实例
	container := di.GetContainer()
	llmService, ok := container.Get("llm").(*services.LLMService)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "无法获取LLM服务实例",
		})
		return
	}

	// 获取当前配置
	cfg := config.GetCurrentConfig()

	// 获取更详细的状态信息
	status := map[string]interface{}{
		"ready":    llmService.IsReady(),
		"status":   llmService.GetReadyState(),
		"provider": llmService.GetProviderName(),
		"config": map[string]interface{}{
			"provider":    cfg.LLMProvider,
			"has_api_key": cfg.LLMConfig != nil && cfg.LLMConfig["api_key"] != "",
		},
	}

	// 添加模型信息
	if cfg.LLMConfig != nil {
		if model, ok := cfg.LLMConfig["default_model"]; ok {
			status["config"].(map[string]interface{})["model"] = model
		}
	}

	c.JSON(http.StatusOK, status)
}

// UpdateLLMConfig 更新LLM配置
func (h *Handler) UpdateLLMConfig(c *gin.Context) {
	var req struct {
		Provider string            `json:"provider" binding:"required"`
		Config   map[string]string `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.Response.BadRequest(c, "无效的请求格式", err.Error())
		return
	}

	// 开始事务
	tx := h.ConfigService.BeginTransaction()

	// 在事务中更新配置
	if err := tx.UpdateLLMConfigInTransaction(req.Provider, req.Config, "web_api"); err != nil {
		h.Response.BadRequest(c, "配置验证失败", err.Error())
		return
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		h.Response.InternalError(c, "配置更新失败", err.Error())
		return
	}

	// 更新 LLMService
	container := di.GetContainer()
	if llmService, ok := container.Get("llm").(*services.LLMService); ok {
		if err := llmService.UpdateProvider(req.Provider, req.Config); err != nil {
			// 配置已保存，但 LLM 服务更新失败
			h.Response.Error(c, http.StatusPartialContent, "CONFIG_UPDATED_LLM_FAILED",
				"配置已保存，但LLM服务更新失败", err.Error())
			return
		}
	} else {
		h.Response.Error(c, http.StatusPartialContent, "CONFIG_UPDATED_LLM_UNAVAILABLE",
			"配置已保存，但无法获取LLM服务", "请重启应用以使配置生效")
		return
	}

	h.Response.Success(c, nil, "LLM配置更新成功")
}

// GetConfigHealth 获取配置健康状态
func (h *Handler) GetConfigHealth(c *gin.Context) {
	// 使用 services 包中的 NewConfigHealthCheck 函数
	healthCheck := services.NewConfigHealthCheck(h.ConfigService)
	health := healthCheck.CheckHealth()

	// 根据健康状态返回不同的HTTP状态码
	if health["status"] == "healthy" {
		h.Response.Success(c, health, "配置健康状态正常")
	} else {
		h.Response.Error(c, http.StatusServiceUnavailable, ErrorConfigUnhealthy,
			"配置健康状态异常", "请检查配置详情")
	}
}

// GetConfigMetrics 获取配置服务指标
func (h *Handler) GetConfigMetrics(c *gin.Context) {
	metrics := h.ConfigService.GetMetrics()
	h.Response.Success(c, metrics, "配置指标获取成功")
}

// GetLLMModels 获取指定LLM提供商支持的模型列表
func (h *Handler) GetLLMModels(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少提供商参数"})
		return
	}

	// 直接使用现有的函数获取模型列表
	models := llm.GetSupportedModelsForProvider(provider)

	// 检查提供商是否存在
	if len(models) == 0 {
		// 验证提供商是否在注册列表中
		availableProviders := llm.ListProviders()
		providerExists := false
		for _, p := range availableProviders {
			if p == provider {
				providerExists = true
				break
			}
		}

		if !providerExists {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "不支持的LLM提供商: " + provider,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"provider": provider,
		"models":   models,
		"count":    len(models),
	})
}

// TriggerCharacterInteraction 处理函数 - 触发角色互动
func (h *Handler) TriggerCharacterInteraction(c *gin.Context) {
	// 解析请求体
	var req TriggerCharacterInteractionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Response.BadRequest(c, "无效的请求格式", err.Error())
		return
	}

	// 验证参数
	if req.SceneID == "" {
		h.Response.BadRequest(c, "缺少场景ID")
		return
	}
	if len(req.CharacterIDs) < 2 {
		h.Response.BadRequest(c, "至少需要两个角色才能进行互动")
		return
	}
	if req.Topic == "" {
		h.Response.BadRequest(c, "缺少互动主题")
		return
	}

	// 触发角色互动
	interaction, err := h.CharacterService.GenerateCharacterInteraction(
		req.SceneID,
		req.CharacterIDs,
		req.Topic,
		req.ContextDescription,
	)

	if err != nil {
		h.Response.InternalError(c, "生成角色互动失败", err.Error())
		return
	}

	// 广播互动事件到 WebSocket
	go func() {
		h.BroadcastToScene(req.SceneID, map[string]interface{}{
			"type": "character_interaction",
			"data": interaction,
		})
	}()

	// 返回生成的互动内容
	h.Response.Success(c, interaction, "角色互动生成成功")
}

// SimulateCharactersConversation 处理函数 - 模拟角色多轮对话
func (h *Handler) SimulateCharactersConversation(c *gin.Context) {
	// 解析请求体
	var req SimulateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Response.BadRequest(c, "无效的请求格式", err.Error())
		return
	}

	// 验证参数
	if req.SceneID == "" {
		h.Response.BadRequest(c, "缺少场景ID")
		return
	}
	if len(req.CharacterIDs) < 2 {
		h.Response.BadRequest(c, "至少需要两个角色才能进行对话")
		return
	}
	if req.InitialSituation == "" {
		h.Response.BadRequest(c, "缺少初始情境描述")
		return
	}
	if req.NumberOfTurns <= 0 {
		req.NumberOfTurns = 3 // 默认轮数
	}

	// 模拟角色对话
	dialogues, err := h.CharacterService.SimulateCharactersConversation(
		req.SceneID,
		req.CharacterIDs,
		req.InitialSituation,
		req.NumberOfTurns,
	)

	if err != nil {
		h.Response.InternalError(c, "模拟角色对话失败", err.Error())
		return
	}

	// 广播对话模拟事件到 WebSocket
	go func() {
		h.BroadcastToScene(req.SceneID, map[string]interface{}{
			"type": "conversation_simulation",
			"data": dialogues,
		})
	}()

	// 返回生成的对话内容
	h.Response.Success(c, dialogues, "角色对话模拟成功")
}

// GetCharacterInteractions 处理函数 - 获取角色互动历史
func (h *Handler) GetCharacterInteractions(c *gin.Context) {
	// 获取URL参数
	sceneID := c.Param("scene_id")
	if sceneID == "" {
		h.Response.BadRequest(c, "缺少场景ID")
		return
	}

	// 获取过滤参数
	filter := make(map[string]interface{})

	// 处理特定互动ID过滤
	if interactionID := c.Query("interaction_id"); interactionID != "" {
		filter["interaction_id"] = interactionID
	}

	// 处理特定模拟ID过滤
	if simulationID := c.Query("simulation_id"); simulationID != "" {
		filter["simulation_id"] = simulationID
	}

	// 获取限制数量
	limit := 20 // 默认限制
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// 获取分页参数
	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	// 获取角色互动历史
	interactions, err := h.ContextService.GetCharacterInteractions(sceneID, filter, limit)
	if err != nil {
		h.Response.InternalError(c, "获取角色互动历史失败", err.Error())
		return
	}

	// 如果需要分页
	if c.Query("paginated") == "true" {
		total := len(interactions) // 简化处理，实际应该从数据库获取
		meta := &PaginationMeta{
			Page:       page,
			PerPage:    limit,
			Total:      total,
			TotalPages: (total + limit - 1) / limit,
		}
		h.Response.PaginatedSuccess(c, interactions, meta, "角色互动历史获取成功")
	} else {
		h.Response.Success(c, interactions, "角色互动历史获取成功")
	}
}

// GetCharacterToCharacterInteractions 处理函数 - 获取特定两个角色之间的互动
func (h *Handler) GetCharacterToCharacterInteractions(c *gin.Context) {
	// 获取URL参数
	sceneID := c.Param("scene_id")
	character1ID := c.Param("character1_id")
	character2ID := c.Param("character2_id")

	// 验证必要的参数
	if sceneID == "" || character1ID == "" || character2ID == "" {
		h.Response.BadRequest(c, "缺少必要参数: scene_id, character1_id, character2_id")
		return
	}

	// 获取限制数量
	limit := 20 // 默认限制
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// 获取两个角色之间的互动
	interactions, err := h.ContextService.GetCharacterToCharacterInteractions(sceneID, character1ID, character2ID, limit)
	if err != nil {
		h.Response.InternalError(c, "获取角色互动历史失败", err.Error())
		return
	}

	// 如果没有找到互动记录
	if len(interactions) == 0 {
		h.Response.Success(c, []interface{}{}, "暂无互动记录")
		return
	}

	h.Response.Success(c, interactions, "角色互动历史获取成功")
}

// GetSceneAggregate 获取场景聚合数据
func (h *Handler) GetSceneAggregate(c *gin.Context) {
	sceneID := c.Param("id")

	// 解析查询参数
	options := &services.AggregateOptions{
		IncludeConversations: c.DefaultQuery("include_conversations", "true") == "true",
		IncludeStoryData:     c.DefaultQuery("include_story", "true") == "true",
		IncludeUIState:       c.DefaultQuery("include_ui_state", "true") == "true",
		IncludeProgress:      c.DefaultQuery("include_progress", "true") == "true",
	}

	// 解析对话限制
	if limitStr := c.Query("conversation_limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			options.ConversationLimit = limit
		} else {
			options.ConversationLimit = 20
		}
	} else {
		options.ConversationLimit = 20
	}

	// 解析用户偏好（如果提供）
	if prefJSON := c.Query("preferences"); prefJSON != "" {
		var preferences models.UserPreferences
		if err := json.Unmarshal([]byte(prefJSON), &preferences); err == nil {
			options.UserPreferences = &preferences
		}
	}

	// 获取场景聚合服务
	aggregateService := h.getSceneAggregateService()
	if aggregateService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "场景聚合服务未初始化"})
		return
	}

	// 创建上下文
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 获取聚合数据
	aggregateData, err := aggregateService.GetSceneAggregate(ctx, sceneID, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取场景数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, aggregateData)
}

// getSceneAggregateService 获取场景聚合服务实例
func (h *Handler) getSceneAggregateService() *services.SceneAggregateService {
	container := di.GetContainer()

	// 尝试从容器获取
	if service, ok := container.Get("scene_aggregate").(*services.SceneAggregateService); ok {
		return service
	}

	// 如果不存在，创建新实例
	storyService := h.getStoryService()
	if storyService == nil {
		return nil
	}

	service := services.NewSceneAggregateService(
		h.SceneService,
		h.CharacterService,
		h.ContextService,
		storyService,
		h.ProgressService,
	)

	// 注册到容器
	container.Register("scene_aggregate", service)

	return service
}

// ProcessInteractionAggregate 处理聚合交互请求
func (h *Handler) ProcessInteractionAggregate(c *gin.Context) {
	var request services.InteractionRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 验证必要参数
	if request.SceneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少场景ID"})
		return
	}

	if len(request.CharacterIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少需要选择一个角色"})
		return
	}

	if request.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "消息内容不能为空"})
		return
	}

	// 获取交互聚合服务
	interactionService := h.getInteractionAggregateService()
	if interactionService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "交互聚合服务未初始化"})
		return
	}

	// 创建上下文
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 处理交互
	result, err := interactionService.ProcessInteraction(ctx, &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "处理交互失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// getInteractionAggregateService 获取交互聚合服务实例
func (h *Handler) getInteractionAggregateService() *services.InteractionAggregateService {
	container := di.GetContainer()
	service, ok := container.Get("interaction_aggregate").(*services.InteractionAggregateService)
	if !ok {
		log.Printf("警告: 无法从容器获取交互聚合服务")
		return nil
	}
	return service
}

// ExportInteractionSummary 导出交互摘要
func (h *Handler) ExportInteractionSummary(c *gin.Context) {
	sceneID := c.Param("scene_id")
	format := c.DefaultQuery("format", "json")

	// 获取交互聚合服务
	interactionService := h.getInteractionAggregateService()
	if interactionService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "交互聚合服务未初始化"})
		return
	}

	// 导出交互摘要
	result, err := interactionService.ExportInteraction(c.Request.Context(), sceneID, format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 根据格式返回不同的响应
	switch strings.ToLower(format) {
	case "json":
		c.JSON(http.StatusOK, result)
	case "markdown", "txt":
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(result.FilePath)))
		c.String(http.StatusOK, result.Content)
	case "html":
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(result.FilePath)))
		c.String(http.StatusOK, result.Content)
	default:
		c.JSON(http.StatusOK, result)
	}
}

// 故事视图页面处理器
func (h *Handler) StoryViewPage(c *gin.Context) {
	sceneID := c.Param("id")
	sceneData, err := h.SceneService.LoadScene(sceneID)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"error": "Scene not found",
		})
		return
	}

	c.HTML(http.StatusOK, "story_view.html", gin.H{
		"scene":      sceneData.Scene,
		"characters": sceneData.Characters,
	})
}

// UserProfilePage 返回用户档案页面
func (h *Handler) UserProfilePage(c *gin.Context) {
	// 获取用户ID（从query参数或默认值）
	userID := c.Query("user_id")
	if userID == "" {
		userID = "user_default" // 默认用户ID
	}

	// 获取配置
	cfg := config.GetCurrentConfig()

	// 渲染用户档案页面
	c.HTML(http.StatusOK, "user_profile.html", gin.H{
		"title":      "用户档案 - SceneIntruderMCP",
		"user_id":    userID,
		"debug":      cfg.DebugMode,
		"static_url": "/static",
	})
}
