// internal/api/handlers.go
package api

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/Corphon/SceneIntruderMCP/internal/utils"
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
	Logger           *utils.Logger              // 结构化日志记录器
	Metrics          *utils.APIMetrics          // 应用指标收集器
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

// SceneCommandRequest 大屏互动指令
type SceneCommandRequest struct {
	Input        string   `json:"input"`
	Mode         string   `json:"mode"`
	CharacterIDs []string `json:"character_ids"`
	ItemHints    []string `json:"item_hints"`
	SkillHints   []string `json:"skill_hints"`
	LocationIDs  []string `json:"location_ids"`
}

type storyCommandLLMResponse struct {
	Narration       string                      `json:"narration"`
	Content         string                      `json:"content"`
	Summary         string                      `json:"summary"`
	Choices         []storyCommandChoicePayload `json:"choices"`
	Recommendations []storyCommandChoicePayload `json:"recommendations"`
}

type storyCommandChoicePayload struct {
	ID          string      `json:"id"`
	Text        string      `json:"text"`
	Consequence string      `json:"consequence"`
	NextHint    string      `json:"next_hint"`
	Hint        string      `json:"hint"`
	Type        string      `json:"type"`
	Impact      interface{} `json:"impact"`
	Order       int         `json:"order"`
	Description string      `json:"description"`
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

	h.Response.Success(c, status, "WebSocket status retrieved successfully")
}

// 添加管理器控制API
func (h *Handler) CleanupWebSocketConnections(c *gin.Context) {
	wsManager.cleanupExpiredConnections()
	h.Response.Success(c, nil, "Connection cleanup executed")
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
		h.Response.InternalError(c, "Export service not initialized", "Failed to obtain export service instance")
		return
	}

	result, err := exportService.ExportSceneData(c.Request.Context(), sceneID, format, includeConversations)
	if err != nil {
		h.Response.InternalError(c, "Failed to export scene data", err.Error())
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
		h.Response.BadRequest(c, "Missing scene ID")
		return
	}

	// 验证导出格式
	supportedFormats := []string{"json", "markdown", "txt", "html", "csv"}
	if !contains(supportedFormats, strings.ToLower(format)) {
		h.Response.BadRequest(c, "Unsupported export format", fmt.Sprintf("Supported formats: %v", supportedFormats))
		return
	}
	// 获取导出服务
	exportService := h.getExportService()
	if exportService == nil {
		h.Response.InternalError(c, "Export service not initialized", "Failed to obtain export service instance")
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
				"Export operation timed out", "Please try again later or contact the administrator.")
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
		Logger:           utils.GetLogger(),
		Metrics:          utils.NewAPIMetrics(),
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

	if sceneID == "" {
		h.Response.BadRequest(c, "场景ID不能为空")
		return
	}

	startTime := time.Now()
	h.Logger.Info("Getting scene details", map[string]interface{}{
		"scene_id":  sceneID,
		"client_ip": c.ClientIP(),
	})

	sceneData, err := h.SceneService.LoadScene(sceneID)
	if err != nil {
		h.Logger.Error("Failed to load scene", map[string]interface{}{
			"scene_id":  sceneID,
			"error":     err.Error(),
			"client_ip": c.ClientIP(),
		})
		h.Metrics.RecordError("scene_load_failed", "scene_service")

		// Check if it's a "not found" error specifically
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "不存在") {
			h.Response.NotFound(c, "场景", "场景ID: "+sceneID)
		} else {
			h.Response.InternalError(c, "加载场景失败", err.Error())
		}
		return
	}

	if sceneData == nil {
		h.Response.NotFound(c, "场景", "场景ID: "+sceneID)
		return
	}

	duration := time.Since(startTime)
	h.Logger.Info("Scene details retrieved successfully", map[string]interface{}{
		"scene_id":        sceneID,
		"character_count": len(sceneData.Characters),
		"duration_ms":     duration.Milliseconds(),
		"client_ip":       c.ClientIP(),
	})

	h.Metrics.RecordAPIRequest("get_scene", "GET", http.StatusOK, duration)
	h.Metrics.RecordSceneInteraction(sceneID, "scene_viewed")

	h.Response.Success(c, sceneData, "场景数据获取成功")
}

// CreateScene 从文本创建新场景
func (h *Handler) CreateScene(c *gin.Context) {
	var req struct {
		Title string `json:"title" binding:"required"`
		Text  string `json:"text" binding:"required"`
	}

	startTime := time.Now()

	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Error("Invalid request parameters for create scene endpoint", map[string]interface{}{
			"error":     err.Error(),
			"client_ip": c.ClientIP(),
		})
		h.Metrics.RecordError("invalid_request", "create_scene_endpoint")
		h.Response.BadRequest(c, "请求参数错误", err.Error())
		return
	}

	// Additional validation improvements
	if len(req.Title) == 0 {
		h.Response.BadRequest(c, "标题不能为空")
		return
	}

	if len(req.Title) > 200 {
		h.Response.BadRequest(c, "标题过长", "标题不能超过200个字符")
		return
	}

	if len(req.Text) == 0 {
		h.Response.BadRequest(c, "文本内容不能为空")
		return
	}

	if len(req.Text) > 100000 { // 100KB limit
		h.Response.BadRequest(c, "文本内容过长", "文本内容不能超过100,000个字符")
		return
	}

	// Log the scene creation attempt
	h.Logger.Info("Creating new scene", map[string]interface{}{
		"title":       req.Title,
		"text_length": len(req.Text),
		"client_ip":   c.ClientIP(),
	})

	// Get user ID from context for ownership
	userID, isAuthenticated := GetUserFromContext(c)
	if !isAuthenticated {
		userID = "anonymous" // Handle unauthenticated users appropriately
	}

	// 创建场景
	scene, err := h.SceneService.CreateSceneFromText(userID, req.Text, req.Title)
	if err != nil {
		if errors.Is(err, services.ErrLLMNotReady) {
			if h.Logger != nil {
				h.Logger.Warn("LLM service not ready during scene creation", map[string]interface{}{
					"title":     req.Title,
					"client_ip": c.ClientIP(),
				})
			} else {
				log.Printf("LLM service not ready while creating scene %s", req.Title)
			}
			h.Metrics.RecordError("llm_not_ready", "llm_service")
			h.Response.Error(c, http.StatusServiceUnavailable, ErrorLLMNotReady,
				MessageLLMNotReady, "请在设置中配置LLM API密钥后重试")
			return
		}
		if h.Logger != nil {
			h.Logger.Error("Failed to create scene", map[string]interface{}{
				"title":     req.Title,
				"error":     err.Error(),
				"client_ip": c.ClientIP(),
			})
		}
		h.Metrics.RecordError("scene_creation_failed", "scene_service")
		h.Response.InternalError(c, "创建场景失败", err.Error())
		return
	}

	if scene == nil {
		h.Logger.Error("Scene creation returned nil", map[string]interface{}{
			"title":     req.Title,
			"client_ip": c.ClientIP(),
		})
		h.Response.InternalError(c, "创建场景失败", "返回了无效的场景数据")
		return
	}

	// 自动初始化故事数据
	storyService := h.getStoryService()
	if storyService != nil {
		// 使用默认偏好设置
		defaultPrefs := &models.UserPreferences{
			CreativityLevel: models.CreativityBalanced,
			AllowPlotTwists: true,
		}

		// 异步初始化故事，避免阻塞响应
		go func() {
			_, err := storyService.InitializeStoryForScene(scene.ID, defaultPrefs)
			if err != nil {
				h.Logger.Error("Failed to auto-initialize story for new scene", map[string]interface{}{
					"scene_id": scene.ID,
					"error":    err.Error(),
				})
			} else {
				h.Logger.Info("Successfully auto-initialized story for new scene", map[string]interface{}{
					"scene_id": scene.ID,
				})
			}
		}()
	}

	duration := time.Since(startTime)
	h.Logger.Info("Scene created successfully", map[string]interface{}{
		"scene_id":    scene.ID,
		"title":       scene.Title,
		"duration_ms": duration.Milliseconds(),
		"client_ip":   c.ClientIP(),
	})

	h.Metrics.RecordAPIRequest("create_scene", "POST", http.StatusCreated, duration)
	h.Metrics.RecordSceneInteraction(scene.ID, "scene_created")

	h.Response.Created(c, scene, "场景创建成功")
}

// DeleteScene 删除指定场景
func (h *Handler) DeleteScene(c *gin.Context) {
	sceneID := c.Param("id")
	if sceneID == "" {
		h.Response.BadRequest(c, "场景ID不能为空")
		return
	}

	h.Logger.Info("Deleting scene", map[string]interface{}{
		"scene_id":  sceneID,
		"client_ip": c.ClientIP(),
	})

	if err := h.SceneService.DeleteScene(sceneID); err != nil {
		h.Logger.Error("Failed to delete scene", map[string]interface{}{
			"scene_id":  sceneID,
			"error":     err.Error(),
			"client_ip": c.ClientIP(),
		})

		if strings.Contains(err.Error(), "不存在") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			h.Response.NotFound(c, "场景", "场景ID: "+sceneID)
			return
		}

		h.Response.InternalError(c, "删除场景失败", err.Error())
		return
	}

	storyService := h.getStoryService()
	if storyService != nil {
		if err := storyService.DeleteStoryData(sceneID); err != nil {
			h.Logger.Error("Failed to delete story data for scene", map[string]interface{}{
				"scene_id":  sceneID,
				"error":     err.Error(),
				"client_ip": c.ClientIP(),
			})
			h.Response.InternalError(c, "删除故事数据失败", err.Error())
			return
		}
	} else {
		h.Logger.Warn("Story service unavailable during scene deletion", map[string]interface{}{
			"scene_id":  sceneID,
			"client_ip": c.ClientIP(),
		})
	}

	h.Response.Success(c, gin.H{"scene_id": sceneID}, "场景删除成功")
}

// GetCharacters 获取指定场景的所有角色
func (h *Handler) GetCharacters(c *gin.Context) {
	sceneID := c.Param("id")
	sceneData, err := h.SceneService.LoadScene(sceneID)
	if err != nil {
		h.Response.NotFound(c, "场景", "场景ID: "+sceneID)
		return
	}

	// 确保角色数据不为nil
	if sceneData.Characters == nil {
		sceneData.Characters = []*models.Character{}
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

	startTime := time.Now()

	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Error("Invalid request parameters for chat endpoint", map[string]interface{}{
			"error":     err.Error(),
			"client_ip": c.ClientIP(),
		})
		h.Metrics.RecordError("invalid_request", "chat_endpoint")
		h.Response.BadRequest(c, "请求参数错误", err.Error())
		return
	}

	// 生成角色回应
	response, err := h.CharacterService.GenerateResponse(req.SceneID, req.CharacterID, req.Message)
	if err != nil {
		h.Logger.Error("Failed to generate character response", map[string]interface{}{
			"scene_id":     req.SceneID,
			"character_id": req.CharacterID,
			"error":        err.Error(),
			"client_ip":    c.ClientIP(),
		})
		h.Metrics.RecordError("response_generation_failed", "character_service")
		h.Response.InternalError(c, "生成回应失败", err.Error())
		return
	}

	duration := time.Since(startTime)
	h.Logger.Info("Chat request completed successfully", map[string]interface{}{
		"scene_id":     req.SceneID,
		"character_id": req.CharacterID,
		"duration_ms":  duration.Milliseconds(),
		"client_ip":    c.ClientIP(),
	})

	h.Metrics.RecordAPIRequest("chat", "POST", http.StatusOK, duration)
	h.Metrics.RecordSceneInteraction(req.SceneID, "chat")

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
		h.Response.BadRequest(c, "获取上传文件失败", err.Error())
		return
	}

	// 检查文件大小 (limit to 10MB)
	const maxFileSize = 10 << 20 // 10 MB
	if file.Size > maxFileSize {
		h.Response.BadRequest(c, "文件过大", "文件大小不能超过10MB")
		return
	}

	// 检查文件类型
	ext := filepath.Ext(file.Filename)
	if ext != ".txt" && ext != ".md" {
		h.Response.BadRequest(c, "不支持的文件类型", "只支持.txt或.md文件")
		return
	}

	// Generate secure filename to prevent path traversal
	secureFilename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(file.Filename))
	secureFilename = filepath.Clean(secureFilename) // Clean to prevent path traversal

	// Validate that the cleaned filename still has the correct extension
	if filepath.Ext(secureFilename) != ext {
		h.Response.BadRequest(c, "无效的文件名", "文件名包含不允许的字符")
		return
	}

	// Store temporary file with secure name
	tempPath := filepath.Join("temp", secureFilename)
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		h.Response.InternalError(c, "保存文件失败", err.Error())
		return
	}

	// Read file content
	content, err := os.ReadFile(tempPath)
	if err != nil {
		h.Response.InternalError(c, "读取文件失败", err.Error())
		// Clean up the temp file in case of error
		os.Remove(tempPath)
		return
	}

	// Validate file content based on extension
	if ext == ".txt" || ext == ".md" {
		// Basic text file validation: check for null bytes and other binary indicators
		if strings.Contains(string(content), "\x00") {
			h.Response.BadRequest(c, "文件格式无效", "文件包含二进制内容")
			os.Remove(tempPath)
			return
		}

		// Limit content size to prevent memory exhaustion
		const maxContentLength = 1000000 // 1MB of content
		if len(content) > maxContentLength {
			h.Response.BadRequest(c, "文件内容过大", "文件内容不能超过1MB")
			os.Remove(tempPath)
			return
		}

		// Additional security checks for text files
		contentStr := string(content)

		// Check for potential script tags in markdown files
		if ext == ".md" {
			if strings.Contains(strings.ToLower(contentStr), "<script") ||
				strings.Contains(strings.ToLower(contentStr), "javascript:") ||
				strings.Contains(strings.ToLower(contentStr), "data:text/html") {
				h.Response.BadRequest(c, "文件内容不安全", "文件包含潜在危险内容")
				os.Remove(tempPath)
				return
			}
		}

		// Check for extremely long lines that could indicate binary content or be a DoS vector
		lines := strings.Split(contentStr, "\n")
		for _, line := range lines {
			if len(line) > 10000 { // 10KB per line is excessive for text
				h.Response.BadRequest(c, "文件内容格式异常", "文件包含过长的单行内容")
				os.Remove(tempPath)
				return
			}
		}
	}

	// Prepare response data
	responseData := map[string]interface{}{
		"filename": file.Filename,
		"content":  string(content),
		"size":     len(content),
	}

	// Delete temporary file after reading
	_ = os.Remove(tempPath)

	// Return success response
	h.Response.Success(c, responseData, "文件上传成功")
}

// AnalyzeTextWithProgress 处理文本分析请求，返回任务ID
func (h *Handler) AnalyzeTextWithProgress(c *gin.Context) {
	// 解析请求
	var req struct {
		Text  string `json:"text" binding:"required"`
		Title string `json:"title" binding:"required"`
	}

	startTime := time.Now()

	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Error("Invalid request parameters for analyze text endpoint", map[string]interface{}{
			"error":     err.Error(),
			"client_ip": c.ClientIP(),
		})
		h.Metrics.RecordError("invalid_request", "analyze_text_endpoint")
		h.Response.BadRequest(c, "请求参数错误", err.Error())
		return
	}

	if h.AnalyzerService == nil {
		h.Response.InternalError(c, "分析服务未初始化", "Analyzer service unavailable")
		return
	}

	llmSvc := h.AnalyzerService.LLMService
	if llmSvc == nil || !llmSvc.IsReady() {
		readyState := "LLM服务未配置或未就绪"
		if llmSvc != nil {
			readyState = llmSvc.GetReadyState()
		}
		h.Response.Error(c, http.StatusServiceUnavailable, "LLM_NOT_READY",
			"LLM服务未配置或未就绪", readyState)
		return
	}

	// Log the analysis attempt
	h.Logger.Info("Starting text analysis", map[string]interface{}{
		"title":       req.Title,
		"text_length": len(req.Text),
		"client_ip":   c.ClientIP(),
	})

	// 创建唯一任务ID
	taskID := fmt.Sprintf("analyze_%d", time.Now().UnixNano())

	// 创建进度跟踪器
	tracker := h.ProgressService.CreateTracker(taskID)

	// 启动后台分析
	go func() {
		// Create a context with timeout for the analysis
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// Log the start of the analysis
		h.Logger.Info("Starting background text analysis", map[string]interface{}{
			"task_id": taskID,
			"title":   req.Title,
		})

		// 执行分析
		result, err := h.AnalyzerService.AnalyzeTextWithProgress(ctx, req.Text, tracker)
		if err != nil {
			h.Logger.Error("Text analysis failed", map[string]interface{}{
				"task_id": taskID,
				"error":   err.Error(),
			})
			h.Metrics.RecordError("text_analysis_failed", "analyzer_service")
			tracker.Fail(err.Error())
			return
		}

		// Log successful analysis
		h.Logger.Info("Text analysis completed", map[string]interface{}{
			"task_id":         taskID,
			"character_count": len(result.Characters),
			"scene_count":     len(result.Scenes),
		})

		// Get user ID from context for ownership
		userID, _ := GetUserFromContext(c)

		// 分析完成后创建场景
		scene := &models.Scene{
			ID:          fmt.Sprintf("scene_%d", time.Now().UnixNano()),
			UserID:      userID, // Assign the user who created this scene
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
			h.Logger.Error("Failed to create scene with analyzed characters", map[string]interface{}{
				"task_id":  taskID,
				"scene_id": scene.ID,
				"error":    err.Error(),
			})
			tracker.Fail("场景创建失败: " + err.Error())
			return
		}

		// Log successful scene creation
		h.Logger.Info("Scene created from analysis", map[string]interface{}{
			"task_id":  taskID,
			"scene_id": scene.ID,
		})

		// Update task status with created scene ID
		tracker.Complete(fmt.Sprintf("分析完成，场景已创建: %s", scene.ID))
	}()

	duration := time.Since(startTime)
	h.Logger.Info("Text analysis request accepted", map[string]interface{}{
		"task_id":     taskID,
		"title":       req.Title,
		"duration_ms": duration.Milliseconds(),
		"client_ip":   c.ClientIP(),
	})

	h.Metrics.RecordAPIRequest("analyze_text", "POST", http.StatusAccepted, duration)
	h.Metrics.RecordUserAction(c.ClientIP(), "text_analysis_request")

	h.Response.Success(c, map[string]interface{}{
		"task_id": taskID,
		"message": "文本分析已开始，请订阅进度更新",
	}, "文本分析请求已接受")
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

	h.Response.Success(c, nil, "任务已取消")
}

// ChatWithEmotion 处理带情绪的聊天请求
func (h *Handler) ChatWithEmotion(c *gin.Context) {
	var req struct {
		SceneID     string `json:"scene_id" binding:"required"`
		CharacterID string `json:"character_id" binding:"required"`
		Message     string `json:"message" binding:"required"`
	}

	startTime := time.Now()

	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Error("Invalid request parameters for emotional chat endpoint", map[string]interface{}{
			"error":     err.Error(),
			"client_ip": c.ClientIP(),
		})
		h.Metrics.RecordError("invalid_request", "emotional_chat_endpoint")
		h.Response.BadRequest(c, "请求参数错误", err.Error())
		return
	}

	h.Logger.Info("Starting emotional chat request", map[string]interface{}{
		"scene_id":       req.SceneID,
		"character_id":   req.CharacterID,
		"message_length": len(req.Message),
		"client_ip":      c.ClientIP(),
	})

	// 使用新的方法生成带情绪的回应
	response, err := h.CharacterService.GenerateResponseWithEmotion(req.SceneID, req.CharacterID, req.Message)
	if err != nil {
		h.Logger.Error("Failed to generate emotional response", map[string]interface{}{
			"scene_id":     req.SceneID,
			"character_id": req.CharacterID,
			"error":        err.Error(),
			"client_ip":    c.ClientIP(),
		})
		h.Metrics.RecordError("emotional_response_generation_failed", "character_service")
		h.Response.InternalError(c, "生成回应失败", err.Error())
		return
	}

	duration := time.Since(startTime)
	h.Logger.Info("Emotional chat request completed", map[string]interface{}{
		"scene_id":        req.SceneID,
		"character_id":    req.CharacterID,
		"response_length": len(response.Response),
		"duration_ms":     duration.Milliseconds(),
		"tokens_used":     response.TokensUsed,
		"client_ip":       c.ClientIP(),
	})

	// 记录API使用情况
	h.StatsService.RecordAPIRequest(response.TokensUsed)
	h.Metrics.RecordAPIRequest("chat_emotion", "POST", http.StatusOK, duration)
	h.Metrics.RecordLLMRequest(response.CharacterName, "", response.TokensUsed, duration) // Assuming we can get provider info
	h.Metrics.RecordSceneInteraction(req.SceneID, "emotional_chat")

	h.Response.Success(c, response, "情绪化回应生成成功")
}

// GetStoryData 获取指定场景的故事数据
func (h *Handler) GetStoryData(c *gin.Context) {
	sceneID := c.Param("id")
	storyService := h.getStoryService()

	storyData, err := storyService.GetStoryData(sceneID, nil)
	if err != nil {
		h.Response.NotFound(c, "故事数据", "故事数据不存在")
		return
	}

	sanitized := sanitizeStoryDataForClient(storyData)
	h.Response.Success(c, sanitized, "故事数据获取成功")
}

const maxOriginalPreviewRunes = 600

func sanitizeStoryDataForClient(data *models.StoryData) *models.StoryData {
	if data == nil {
		return nil
	}
	cloned := *data
	if len(data.Nodes) > 0 {
		cloned.Nodes = make([]models.StoryNode, len(data.Nodes))
		for i, node := range data.Nodes {
			sanitizedNode := node
			sanitizedNode.OriginalContent = truncateOriginalPreview(node.OriginalContent)
			cloned.Nodes[i] = sanitizedNode
		}
	}
	return &cloned
}

func truncateOriginalPreview(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= maxOriginalPreviewRunes {
		return trimmed
	}
	return string(runes[:maxOriginalPreviewRunes]) + "..."
}

// GetStoryNodeContent 返回指定故事节点的原文和内容
func (h *Handler) GetStoryNodeContent(c *gin.Context) {
	sceneID := c.Param("id")
	nodeID := c.Param("node_id")
	if sceneID == "" || nodeID == "" {
		h.Response.BadRequest(c, "缺少必要参数", "需要 scene_id 和 node_id")
		return
	}

	storyService := h.getStoryService()
	if storyService == nil {
		h.Response.InternalError(c, "故事服务未初始化", "无法获取故事服务实例")
		return
	}

	node, err := storyService.GetStoryNode(sceneID, nodeID)
	if err != nil {
		h.Response.NotFound(c, "故事节点", err.Error())
		return
	}

	response := map[string]interface{}{
		"node":                 node,
		"content":              node.Content,
		"original_content":     node.OriginalContent,
		"related_item_ids":     node.RelatedItemIDs,
		"metadata":             node.Metadata,
		"character_links":      node.CharacterInteractions,
		"interaction_triggers": node.InteractionTriggers,
	}

	h.Response.Success(c, response, "故事节点内容获取成功")
}

// InsertStoryNode 将指定节点内容直接发布到互动屏幕
func (h *Handler) InsertStoryNode(c *gin.Context) {
	sceneID := c.Param("id")
	nodeID := c.Param("node_id")
	if sceneID == "" || nodeID == "" {
		h.Response.BadRequest(c, "缺少必要参数", "需要 scene_id 和 node_id")
		return
	}

	var req struct {
		Content string `json:"content,omitempty"`
	}
	if c.Request != nil && c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			h.Response.BadRequest(c, "请求体格式错误", err.Error())
			return
		}
	}

	storyService := h.getStoryService()
	if storyService == nil {
		h.Response.InternalError(c, "故事服务未初始化", "无法获取故事服务实例")
		return
	}

	node, err := storyService.GetStoryNode(sceneID, nodeID)
	if err != nil {
		h.Response.NotFound(c, "故事节点", err.Error())
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		content = strings.TrimSpace(node.Content)
	}
	if content == "" {
		content = strings.TrimSpace(node.OriginalContent)
	}
	if content == "" {
		h.Response.BadRequest(c, "节点内容为空", "目标节点未包含可用文本")
		return
	}

	if h.ContextService != nil {
		meta := map[string]interface{}{
			"conversation_type": "story_console",
			"mode":              "story",
			"source":            "sidebar_insert",
		}
		if err := h.ContextService.AddConversation(sceneID, "story", content, meta, node.ID); err != nil {
			if h.Logger != nil {
				h.Logger.Warn("记录节点插入失败", map[string]interface{}{
					"error":    err.Error(),
					"scene_id": sceneID,
					"node_id":  nodeID,
				})
			} else {
				log.Printf("failed to record sidebar insert for %s/%s: %v", sceneID, nodeID, err)
			}
		}
	}

	response := map[string]interface{}{
		"node":    node,
		"content": content,
	}

	h.Response.Success(c, response, "节点内容已插入互动屏幕")
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
		h.Response.BadRequest(c, "参数格式错误", err.Error())
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
		// 如果错误是"故事数据不存在"，尝试初始化
		if strings.Contains(err.Error(), "故事数据不存在") {
			_, initErr := storyService.InitializeStoryForScene(sceneID, preferences)
			if initErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("初始化故事失败: %v", initErr)})
				return
			}
			// 初始化后再次尝试推进
			storyUpdate, err = storyService.AdvanceStory(sceneID, preferences)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("推进故事失败: %v", err)})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("推进故事失败: %v", err)})
			return
		}
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

// HandleSceneCommand 统一处理网页端的自由指令
func (h *Handler) HandleSceneCommand(c *gin.Context) {
	sceneID := c.Param("id")
	if sceneID == "" {
		h.Response.BadRequest(c, "缺少场景ID")
		return
	}

	var req SceneCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Response.BadRequest(c, "参数格式错误", err.Error())
		return
	}

	input := strings.TrimSpace(req.Input)
	if input == "" {
		h.Response.BadRequest(c, "指令内容不能为空")
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "story"
	}
	expectStoryNarration := mode == "story"

	storyService := h.getStoryService()
	if storyService == nil {
		h.Response.InternalError(c, "故事服务未初始化", "无法获取故事服务实例")
		return
	}

	sceneData, err := h.SceneService.LoadScene(sceneID)
	if err != nil {
		h.Response.InternalError(c, "加载场景失败", err.Error())
		return
	}

	var storyData *models.StoryData
	storyData, _ = storyService.GetStoryData(sceneID, nil)
	activeNode := latestRevealedStoryNode(storyData)

	container := di.GetContainer()
	llmService, ok := container.Get("llm").(*services.LLMService)
	if !ok || llmService == nil {
		h.Response.InternalError(c, "LLM服务未初始化", "无法获取LLM服务实例")
		return
	}
	if !llmService.IsReady() {
		h.Response.Error(c, http.StatusServiceUnavailable, "LLM_NOT_READY",
			"LLM服务未就绪", llmService.GetReadyState())
		return
	}

	var narrationContext string
	responseContext := ""
	var promptMessages []services.ChatCompletionMessage
	systemPrompt := ""
	var extraParams map[string]interface{}
	if strings.EqualFold(mode, "story") {
		originalText := ""
		if activeNode != nil {
			originalText = strings.TrimSpace(activeNode.Content)
			if originalText == "" {
				originalText = strings.TrimSpace(activeNode.OriginalContent)
			}
		}
		processingText := ""
		if h.ContextService != nil && activeNode != nil {
			if entries, err := h.ContextService.GetConsoleStoryEntriesByNode(sceneID, activeNode.ID, 3); err == nil {
				processingText = formatProcessingEntries(entries)
			} else if h.Logger != nil {
				h.Logger.Warn("获取节点旁白失败", map[string]interface{}{"error": err.Error(), "scene_id": sceneID, "node_id": activeNode.ID})
			} else {
				log.Printf("failed to load node narration for %s/%s: %v", sceneID, activeNode.ID, err)
			}
		}
		systemPrompt = buildStoryModeSystemPrompt(sceneData)
		storyUserPrompt := buildStoryModeUserPrompt(originalText, processingText, input)
		storyUserPrompt += "\n\nRespond strictly in JSON with fields: narration (string) and choices (array of {text, consequence, next_hint, type, impact})."
		responseContext = processingText
		promptMessages = []services.ChatCompletionMessage{
			{Role: services.RoleSystem, Content: systemPrompt},
			{Role: services.RoleUser, Content: storyUserPrompt},
		}
		extraParams = map[string]interface{}{
			"response_format": map[string]string{
				"type": "json_object",
			},
		}
	} else {
		if h.ContextService != nil {
			if entries, err := h.ContextService.GetRecentConsoleStoryEntries(sceneID, 3); err == nil {
				narrationContext = formatConsoleStoryHistory(entries)
			} else if h.Logger != nil {
				h.Logger.Warn("获取剧情旁白失败", map[string]interface{}{"error": err.Error(), "scene_id": sceneID})
			} else {
				log.Printf("failed to load narration context for %s: %v", sceneID, err)
			}
		}
		contextPrompt, legacySystemPrompt := buildSceneCommandContext(sceneData, storyData, mode, &req, narrationContext)
		responseContext = contextPrompt
		systemPrompt = legacySystemPrompt
		promptMessages = []services.ChatCompletionMessage{
			{Role: services.RoleSystem, Content: systemPrompt},
			{Role: services.RoleUser, Content: contextPrompt + "\n\n玩家指令: " + input},
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()

	request := services.ChatCompletionRequest{
		Messages:    promptMessages,
		Model:       "",
		MaxTokens:   700,
		Temperature: 0.7,
		ExtraParams: extraParams,
	}

	resp, err := llmService.CreateChatCompletion(ctx, request)
	fallbackUsed := false
	var answer string
	var rawLLMContent string
	if err != nil {
		fallbackUsed = true
		answer = fmt.Sprintf("[系统提示] LLM 暂不可用: %v\n\n玩家指令：%s", err.Error(), input)
	} else if len(resp.Choices) == 0 {
		fallbackUsed = true
		answer = fmt.Sprintf("[系统提示] LLM 未返回内容，已保留玩家指令：%s", input)
	} else {
		rawLLMContent = resp.Choices[0].Message.Content
		answer = strings.TrimSpace(rawLLMContent)
		if answer == "" {
			fallbackUsed = true
			answer = fmt.Sprintf("[系统提示] LLM 返回空响应，已保留玩家指令：%s", input)
		}
	}

	var choiceSummaries []map[string]interface{}
	if !fallbackUsed && expectStoryNarration && rawLLMContent != "" {
		if payload, err := parseStoryModeLLMResponse(rawLLMContent); err == nil {
			if narration := payload.resolveNarration(); narration != "" {
				answer = narration
			}
			if activeNode != nil {
				translatedChoices := convertStoryModeChoices(activeNode.ID, payload.candidateChoices(), input)
				if len(translatedChoices) > 0 {
					if err := storyService.UpdateNodeChoices(sceneID, activeNode.ID, translatedChoices); err != nil {
						if h.Logger != nil {
							h.Logger.Warn("更新故事节点推荐选项失败", map[string]interface{}{
								"error":    err.Error(),
								"scene_id": sceneID,
								"node_id":  activeNode.ID,
							})
						} else {
							log.Printf("failed to update node choices for %s/%s: %v", sceneID, activeNode.ID, err)
						}
					} else {
						choiceSummaries = summarizeStoryChoiceModels(translatedChoices)
					}
				}
			}
		} else if h.Logger != nil {
			h.Logger.Debug("解析指令结构化响应失败", map[string]interface{}{
				"error":    err.Error(),
				"scene_id": sceneID,
			})
		}
	}

	userID, ok := GetUserFromContext(c)
	if !ok || userID == "" {
		userID = "web_user"
	}

	userMeta := map[string]interface{}{
		"conversation_type": "story_console",
		"mode":              mode,
		"channel":           "user",
		"skill_hints":       req.SkillHints,
		"item_hints":        req.ItemHints,
	}
	activeNodeID := ""
	if activeNode != nil {
		activeNodeID = activeNode.ID
	}
	if activeNodeID != "" {
		userMeta["node_id"] = activeNodeID
	}
	if err := h.ContextService.AddConversation(sceneID, userID, input, userMeta, activeNodeID); err != nil {
		h.Logger.Warn("记录用户指令失败", map[string]interface{}{"error": err.Error(), "scene_id": sceneID})
	}

	speakerID := fmt.Sprintf("console_%s", mode)
	if mode == "character" && len(req.CharacterIDs) == 1 {
		speakerID = req.CharacterIDs[0]
	}
	aiMeta := map[string]interface{}{
		"conversation_type": "story_console",
		"mode":              mode,
		"channel":           "ai",
	}
	if activeNodeID != "" {
		aiMeta["node_id"] = activeNodeID
	}
	if err := h.ContextService.AddConversation(sceneID, speakerID, answer, aiMeta, activeNodeID); err != nil {
		h.Logger.Warn("记录AI回应失败", map[string]interface{}{"error": err.Error(), "scene_id": sceneID})
	}

	result := map[string]interface{}{
		"reply":         answer,
		"mode":          mode,
		"context":       responseContext,
		"characters":    resolveCharacterSummaries(sceneData.Characters, req.CharacterIDs),
		"skill_hints":   req.SkillHints,
		"item_hints":    req.ItemHints,
		"timestamp":     time.Now().Format(time.RFC3339),
		"fallback_used": fallbackUsed,
	}
	if len(choiceSummaries) > 0 {
		result["choice_suggestions"] = choiceSummaries
		result["choice_node_id"] = activeNodeID
	}

	h.Response.Success(c, result, "互动指令执行成功")
}

func buildSceneCommandContext(sceneData *services.SceneData, storyData *models.StoryData, mode string, req *SceneCommandRequest, narrationContext string) (string, string) {
	var focusCharacter *models.Character
	latestNode := latestRevealedStoryNode(storyData)
	characterLookup := make(map[string]*models.Character)
	for _, ch := range sceneData.Characters {
		if ch == nil {
			continue
		}
		characterLookup[ch.ID] = ch
	}

	selectedNames := make([]string, 0, len(req.CharacterIDs))
	for _, id := range req.CharacterIDs {
		if ch, ok := characterLookup[id]; ok {
			selectedNames = append(selectedNames, fmt.Sprintf("%s（%s）", ch.Name, ch.Role))
			if focusCharacter == nil {
				focusCharacter = ch
			}
		}
	}

	var taskHighlights []string
	var locationHighlights []string
	if storyData != nil {
		taskHighlights = summarizeActiveTasks(storyData.Tasks)
		locationHighlights = summarizeTargetLocations(storyData.Locations, req.LocationIDs)
	}

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("场景：%s\n", sceneData.Scene.Title))
	if sceneData.Scene.Description != "" {
		builder.WriteString(fmt.Sprintf("背景：%s\n", sceneData.Scene.Description))
	}
	if latestNode != nil {
		builder.WriteString(fmt.Sprintf("最新剧情：%s\n", strings.TrimSpace(latestNode.Content)))
	}

	if strings.TrimSpace(narrationContext) != "" {
		builder.WriteString("上下文变化：\n")
		builder.WriteString(strings.TrimSpace(narrationContext))
		builder.WriteString("\n")
	}

	if latestNode != nil {
		originalSnippet := strings.TrimSpace(latestNode.OriginalContent)
		if originalSnippet == "" {
			originalSnippet = strings.TrimSpace(latestNode.Content)
		}
		if originalSnippet != "" {
			builder.WriteString("原文基线：\n")
			builder.WriteString(originalSnippet)
			builder.WriteString("\n")
		}
	}
	if storyData != nil {
		builder.WriteString(fmt.Sprintf("当前状态：%s · 进度%d%%\n", storyData.CurrentState, storyData.Progress))
	}
	if len(taskHighlights) > 0 {
		builder.WriteString("关键任务：\n")
		for _, highlight := range taskHighlights {
			builder.WriteString(" - " + highlight + "\n")
		}
	}
	if len(locationHighlights) > 0 {
		builder.WriteString("地点线索：\n")
		for _, highlight := range locationHighlights {
			builder.WriteString(" - " + highlight + "\n")
		}
	}
	if len(selectedNames) > 0 {
		builder.WriteString("关注角色：" + strings.Join(selectedNames, "、") + "\n")
	}
	if len(req.SkillHints) > 0 {
		builder.WriteString("技能提示：" + strings.Join(req.SkillHints, ", ") + "\n")
	}
	if len(req.ItemHints) > 0 {
		builder.WriteString("物品提示：" + strings.Join(req.ItemHints, ", ") + "\n")
	}
	builder.WriteString("请结合上述信息，用沉浸式叙事或对话推动剧情。")

	systemPrompt := buildSceneCommandSystemPrompt(mode, focusCharacter, selectedNames)
	return builder.String(), systemPrompt
}

func latestRevealedStoryNode(storyData *models.StoryData) *models.StoryNode {
	if storyData == nil || len(storyData.Nodes) == 0 {
		return nil
	}
	for i := len(storyData.Nodes) - 1; i >= 0; i-- {
		if storyData.Nodes[i].IsRevealed {
			return &storyData.Nodes[i]
		}
	}
	return &storyData.Nodes[len(storyData.Nodes)-1]
}

func formatProcessingEntries(entries []models.Conversation) string {
	if len(entries) == 0 {
		return "（暂无旁白加工）"
	}
	var builder strings.Builder
	order := 1
	for _, entry := range entries {
		content := extractConversationContent(entry)
		if content == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("[%d] %s\n", order, content))
		order++
	}
	formatted := strings.TrimSpace(builder.String())
	if formatted == "" {
		return "（暂无旁白加工）"
	}
	return formatted
}

func buildStoryModeSystemPrompt(sceneData *services.SceneData) string {
	sceneTitle := ""
	if sceneData != nil {
		sceneTitle = strings.TrimSpace(sceneData.Scene.Title)
	}
	if sceneTitle == "" {
		sceneTitle = "互动故事"
	}
	return fmt.Sprintf("你是《%s》的旁白主持人，需要根据 original、processing、user_new_message 三段信息顺序理解上下文并续写下一段剧情。输出应保持沉浸式第三人称或第二人称叙述，长度控制在约220个汉字（或150个英文词）以内，推动剧情但不要一次性结束冲突，也不要剧透终局，只输出正文。", sceneTitle)
}

func buildStoryModeUserPrompt(originalText, processingText, userMessage string) string {
	original := strings.TrimSpace(originalText)
	if original == "" {
		original = "（暂无原文内容）"
	}
	processing := strings.TrimSpace(processingText)
	if processing == "" {
		processing = "（暂无旁白加工）"
	}
	userInput := strings.TrimSpace(userMessage)
	if userInput == "" {
		userInput = "（玩家暂未提供新指令，仅按原剧情推进）"
	}
	return fmt.Sprintf("original:\n%s\n\nprocessing:\n%s\n\nuser_new_message:\n%s", original, processing, userInput)
}

func parseStoryModeLLMResponse(raw string) (*storyCommandLLMResponse, error) {
	cleaned := services.CleanLLMJSONResponse(raw)
	if strings.TrimSpace(cleaned) == "" {
		cleaned = services.SanitizeLLMJSONResponse(raw)
	}
	clamped := clampJSONEnvelope(cleaned)
	if strings.TrimSpace(clamped) == "" {
		return nil, fmt.Errorf("LLM响应中缺少JSON内容")
	}
	var payload storyCommandLLMResponse
	if err := json.Unmarshal([]byte(clamped), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func clampJSONEnvelope(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	start := strings.IndexAny(trimmed, "{[")
	if start > 0 {
		trimmed = trimmed[start:]
	}
	if trimmed == "" {
		return ""
	}
	switch trimmed[0] {
	case '{':
		if idx := strings.LastIndexByte(trimmed, '}'); idx >= 0 {
			return strings.TrimSpace(trimmed[:idx+1])
		}
	case '[':
		if idx := strings.LastIndexByte(trimmed, ']'); idx >= 0 {
			return strings.TrimSpace(trimmed[:idx+1])
		}
	}
	return trimmed
}

func (p *storyCommandLLMResponse) resolveNarration() string {
	if p == nil {
		return ""
	}
	candidates := []string{p.Narration, p.Content, p.Summary}
	for _, value := range candidates {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (p *storyCommandLLMResponse) candidateChoices() []storyCommandChoicePayload {
	if p == nil {
		return nil
	}
	if len(p.Choices) > 0 {
		return p.Choices
	}
	if len(p.Recommendations) > 0 {
		return p.Recommendations
	}
	return nil
}

func convertStoryModeChoices(nodeID string, payload []storyCommandChoicePayload, userInput string) []models.StoryChoice {
	if len(payload) == 0 {
		return nil
	}
	now := time.Now()
	trimmedInput := strings.TrimSpace(userInput)
	choices := make([]models.StoryChoice, 0, len(payload))
	for idx, candidate := range payload {
		text := strings.TrimSpace(candidate.Text)
		if text == "" {
			continue
		}
		choiceID := strings.TrimSpace(candidate.ID)
		if choiceID == "" {
			choiceID = fmt.Sprintf("choice_%s_cmd_%d_%d", nodeID, now.Unix(), idx+1)
		}
		nextHint := firstNonEmpty(candidate.NextHint, candidate.Hint, candidate.Consequence)
		fallbackImpact := float64(len(payload) - idx)
		impact, impactLabel := normalizeChoiceImpact(candidate.Impact, fallbackImpact)
		metadata := map[string]interface{}{
			"source":   "story_command",
			"llm_rank": idx,
			"hint_raw": strings.TrimSpace(candidate.Hint),
		}
		if trimmedInput != "" {
			metadata["user_command"] = trimmedInput
		}
		if strings.TrimSpace(candidate.Description) != "" {
			metadata["description"] = strings.TrimSpace(candidate.Description)
		}
		if impactLabel != "" {
			metadata["impact_text"] = impactLabel
		}
		choices = append(choices, models.StoryChoice{
			ID:           choiceID,
			Text:         text,
			Consequence:  strings.TrimSpace(candidate.Consequence),
			NextNodeHint: strings.TrimSpace(nextHint),
			Selected:     false,
			CreatedAt:    now,
			Description:  strings.TrimSpace(candidate.Description),
			Type:         firstNonEmpty(strings.TrimSpace(candidate.Type), "branch"),
			Impact:       impact,
			Order:        idx,
			Metadata:     metadata,
		})
		if len(choices) >= 4 {
			break
		}
	}
	return choices
}

func normalizeChoiceImpact(raw interface{}, fallback float64) (float64, string) {
	if raw == nil {
		return fallback, ""
	}
	switch v := raw.(type) {
	case float64:
		if v == 0 {
			return fallback, ""
		}
		return v, ""
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return fallback, ""
		}
		if parsed, err := strconv.ParseFloat(strings.TrimSuffix(trimmed, "%"), 64); err == nil {
			return parsed, ""
		}
		return fallback, trimmed
	default:
		return fallback, ""
	}
}

func summarizeStoryChoiceModels(choices []models.StoryChoice) []map[string]interface{} {
	if len(choices) == 0 {
		return nil
	}
	summaries := make([]map[string]interface{}, 0, len(choices))
	for _, choice := range choices {
		summaries = append(summaries, map[string]interface{}{
			"id":          choice.ID,
			"text":        choice.Text,
			"consequence": choice.Consequence,
			"next_hint":   choice.NextNodeHint,
			"impact":      choice.Impact,
			"type":        choice.Type,
			"order":       choice.Order,
		})
		if choice.Metadata != nil {
			if label, ok := choice.Metadata["impact_text"].(string); ok && strings.TrimSpace(label) != "" {
				summaries[len(summaries)-1]["impact_text"] = strings.TrimSpace(label)
			}
		}
	}
	return summaries
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func formatConsoleStoryHistory(entries []models.Conversation) string {
	if len(entries) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("最近剧情旁白：\n")
	for _, entry := range entries {
		content := extractConversationContent(entry)
		if content == "" {
			continue
		}
		timeLabel := ""
		if !entry.Timestamp.IsZero() {
			timeLabel = entry.Timestamp.Format("15:04")
		}
		if timeLabel != "" {
			builder.WriteString(fmt.Sprintf("[%s] %s\n", timeLabel, content))
		} else {
			builder.WriteString(content + "\n")
		}
	}
	formatted := strings.TrimSpace(builder.String())
	if formatted == "最近剧情旁白：" {
		return ""
	}
	return formatted
}

func extractConversationContent(entry models.Conversation) string {
	if trimmed := strings.TrimSpace(entry.Content); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(entry.Message); trimmed != "" {
		return trimmed
	}
	if entry.Metadata != nil {
		if raw, ok := entry.Metadata["content"].(string); ok {
			if trimmed := strings.TrimSpace(raw); trimmed != "" {
				return trimmed
			}
		}
		if raw, ok := entry.Metadata["message"].(string); ok {
			if trimmed := strings.TrimSpace(raw); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func buildSceneCommandSystemPrompt(mode string, focus *models.Character, selectedNames []string) string {
	switch mode {
	case "character":
		if focus != nil {
			return fmt.Sprintf("你正在扮演角色“%s”。请以第一人称，保持其性格（%s）与背景（%s），针对玩家输入给出生动回应。",
				focus.Name, focus.Personality, focus.Description)
		}
		return "你是当前被选中的角色，请以第一人称视角回应玩家。"
	case "group":
		if len(selectedNames) > 0 {
			return fmt.Sprintf("你是叙事者，负责描绘这些角色之间的互动：%s。请以剧本化的方式描写他们的举动与台词，并给出下一步建议。",
				strings.Join(selectedNames, "、"))
		}
		return "你是叙事者，编排多个角色的互动场景。"
	case "skill":
		return "你是剧情主持人，重点描述技能发动带来的影响，并提示可以探索的方向。"
	default:
		return "你是沉浸式故事主持人，请扮演旁白或世界，引导玩家并适时提供下一步行动建议；务必根据当前进度循序推动情节，除非玩家明确要求，否则不要直接给出终极结局或一次性解决所有冲突，应保留悬念并给出可跟进的行动。"
	}
}

func summarizeActiveTasks(tasks []models.Task) []string {
	highlights := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.Completed {
			continue
		}
		nextObjective := ""
		for _, obj := range task.Objectives {
			if !obj.Completed {
				nextObjective = obj.Description
				break
			}
		}
		highlight := task.Title
		if nextObjective != "" {
			highlight = fmt.Sprintf("%s → %s", task.Title, nextObjective)
		} else if task.Description != "" {
			highlight = fmt.Sprintf("%s → %s", task.Title, task.Description)
		}
		highlights = append(highlights, highlight)
		if len(highlights) >= 3 {
			break
		}
	}
	return highlights
}

func summarizeTargetLocations(locations []models.StoryLocation, targetIDs []string) []string {
	if len(targetIDs) == 0 {
		return nil
	}
	lookup := make(map[string]models.StoryLocation)
	for _, loc := range locations {
		lookup[loc.ID] = loc
	}
	highlights := make([]string, 0, len(targetIDs))
	for _, id := range targetIDs {
		if loc, ok := lookup[id]; ok {
			status := "未解锁"
			if loc.Accessible {
				status = "可探索"
			}
			highlights = append(highlights, fmt.Sprintf("%s（%s）", loc.Name, status))
		}
	}
	return highlights
}

func resolveCharacterSummaries(characters []*models.Character, ids []string) []map[string]string {
	if len(ids) == 0 {
		return nil
	}
	lookup := make(map[string]*models.Character)
	for _, ch := range characters {
		if ch != nil {
			lookup[ch.ID] = ch
		}
	}
	result := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		if ch, ok := lookup[id]; ok {
			result = append(result, map[string]string{
				"id":   ch.ID,
				"name": ch.Name,
				"role": ch.Role,
				"mood": ch.Personality,
			})
		}
	}
	return result
}

// RewindStory 回溯故事到指定节点
func (h *Handler) RewindStory(c *gin.Context) {
	sceneID := c.Param("id")

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

	var targetNode *models.StoryNode
	removedNodeIDs := make([]string, 0)
	var cutoff time.Time
	for i := range storyData.Nodes {
		node := &storyData.Nodes[i]
		if node.ID == req.NodeID {
			targetNode = node
			cutoff = node.CreatedAt
			continue
		}
		if !cutoff.IsZero() && node.CreatedAt.After(cutoff) {
			removedNodeIDs = append(removedNodeIDs, node.ID)
		}
	}

	if h.ContextService != nil && len(removedNodeIDs) > 0 {
		if err := h.ContextService.RemoveConversationsAfterNode(sceneID, removedNodeIDs, cutoff); err != nil {
			h.Logger.Warn("回溯后清理上下文失败", map[string]interface{}{
				"error":    err.Error(),
				"scene_id": sceneID,
				"node_id":  req.NodeID,
			})
		}
	}

	// 构建分支视图数据
	branchView := buildStoryBranchView(storyData)

	// 获取回溯到的节点信息
	if targetNode == nil {
		for i := range storyData.Nodes {
			if storyData.Nodes[i].ID == req.NodeID {
				targetNode = &storyData.Nodes[i]
				break
			}
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
		LLMProvider string                 `json:"llm_provider"`
		LLMConfig   map[string]interface{} `json:"llm_config"` // Use interface{} to handle different types
		DebugMode   bool                   `json:"debug_mode"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.Response.BadRequest(c, "无效的请求数据", err.Error())
		return
	}

	// Convert interface{} map to string map, handling different types
	stringConfig := make(map[string]string)
	for key, value := range request.LLMConfig {
		switch v := value.(type) {
		case string:
			stringConfig[key] = v
		case float64: // JSON numbers are unmarshaled as float64
			// Check if it's actually an integer (whole number)
			if v == float64(int64(v)) {
				// It's a whole number, convert to int then string
				stringConfig[key] = strconv.FormatInt(int64(v), 10)
			} else {
				// It's a decimal number, convert directly to string
				stringConfig[key] = strconv.FormatFloat(v, 'f', -1, 64)
			}
		case float32:
			// Check if it's actually an integer
			if v == float32(int64(v)) {
				stringConfig[key] = strconv.FormatInt(int64(v), 10)
			} else {
				stringConfig[key] = strconv.FormatFloat(float64(v), 'f', -1, 64)
			}
		case int:
			stringConfig[key] = strconv.Itoa(v)
		case int32:
			stringConfig[key] = strconv.FormatInt(int64(v), 10)
		case int64:
			stringConfig[key] = strconv.FormatInt(v, 10)
		case bool:
			stringConfig[key] = strconv.FormatBool(v)
		case nil:
			stringConfig[key] = ""
		default:
			// Handle unexpected types gracefully
			stringConfig[key] = fmt.Sprintf("%v", v) // fallback to string representation
		}
	}

	// 保存LLM配置
	if request.LLMProvider != "" && len(stringConfig) > 0 {
		err := h.ConfigService.UpdateLLMConfig(request.LLMProvider, stringConfig, "web_ui")
		if err != nil {
			h.Response.InternalError(c, "保存LLM配置失败", err.Error())
			return
		}
	}

	h.Response.Success(c, nil, "设置保存成功")
}

// 添加连接测试方法
func (h *Handler) TestConnection(c *gin.Context) {
	// 解析请求体,支持临时配置测试
	var testRequest struct {
		Provider  string            `json:"provider"`
		LLMConfig map[string]string `json:"llm_config"`
	}

	// 尝试解析请求体(如果有的话)
	hasTemporaryConfig := false
	if err := c.ShouldBindJSON(&testRequest); err == nil {
		if testRequest.Provider != "" && len(testRequest.LLMConfig) > 0 {
			hasTemporaryConfig = true
		}
	}

	container := di.GetContainer()
	llmService, ok := container.Get("llm").(*services.LLMService)
	if !ok {
		h.Response.InternalError(c, "无法获取LLM服务实例")
		return
	}

	// 如果提供了临时配置,先尝试使用临时配置测试
	if hasTemporaryConfig {
		h.Logger.Info("Testing connection with temporary config", map[string]interface{}{
			"provider": testRequest.Provider,
		})

		// 创建临时LLM客户端进行测试
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		// 尝试更新并测试
		tempService := &services.LLMService{}
		if err := tempService.UpdateProvider(testRequest.Provider, testRequest.LLMConfig); err != nil {
			h.Response.Error(c, http.StatusBadRequest, "INVALID_CONFIG",
				"配置验证失败", err.Error())
			return
		}

		// 执行测试请求
		modelName := strings.TrimSpace(testRequest.LLMConfig["default_model"])
		if modelName == "" {
			modelName = strings.TrimSpace(testRequest.LLMConfig["model"])
		}

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
			Model:       modelName,
			Temperature: 0.1,
			MaxTokens:   5,
		}

		_, err := tempService.CreateChatCompletion(ctx, request)
		if err != nil {
			h.Response.Error(c, http.StatusServiceUnavailable, "CONNECTION_TEST_FAILED",
				"连接测试失败", err.Error())
			return
		}

		data := map[string]interface{}{
			"provider": testRequest.Provider,
			"status":   "connected",
			"test":     "passed",
			"mode":     "temporary_config",
		}
		h.Response.Success(c, data, "连接测试成功")
		return
	}

	// 使用当前保存的配置进行测试
	// Check if LLM service is ready
	if !llmService.IsReady() {
		readyState := llmService.GetReadyState()

		// Check if the unready state is specifically due to missing API key
		switch readyState {
		case "API key not configured":
			h.Response.Error(c, http.StatusServiceUnavailable, "LLM_NOT_CONFIGURED",
				"LLM service not configured", "API key not configured")
			return
		case "LLM provider not configured":
			h.Response.Error(c, http.StatusServiceUnavailable, "LLM_NOT_CONFIGURED",
				"LLM service not configured", "LLM provider not configured")
			return
		}

		h.Response.Error(c, http.StatusServiceUnavailable, "CONNECTION_FAILED",
			"LLM service not ready", readyState)
		return
	}

	// LLM service is ready, try a simple test call
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Simple connection test
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
		Model:       "", // Use default model
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
		"mode":     "saved_config",
	}
	h.Response.Success(c, data, "连接测试成功")
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

	// Determine if the service is configured but not ready for other reasons
	cfgReady := cfg.LLMProvider != "" && cfg.LLMConfig != nil && cfg.LLMConfig["api_key"] != ""
	if !llmService.IsReady() {
		status["configured"] = cfgReady
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

// UpdateStoryProgress 更新故事进度
func (h *Handler) UpdateStoryProgress(c *gin.Context) {
	sceneIDStr := c.Param("id")
	_, err := strconv.ParseUint(sceneIDStr, 10, 32)
	if err != nil {
		h.Response.Error(c, http.StatusBadRequest, "INVALID_SCENE_ID", "Invalid scene ID", err.Error())
		return
	}

	var progressUpdate struct {
		Progress            float64                `json:"progress"`
		CurrentState        string                 `json:"current_state"`
		UnlockedNodes       []interface{}          `json:"unlocked_nodes"`
		CompletedObjectives []interface{}          `json:"completed_objectives"`
		StoryData           map[string]interface{} `json:"story_data"`
	}

	if err := c.ShouldBindJSON(&progressUpdate); err != nil {
		h.Response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err.Error())
		return
	}

	// 这里可以添加实际的更新逻辑
	// 目前返回更新后的数据
	h.Response.Success(c, progressUpdate, "Story progress updated successfully")
}

// ========================================
// 场景物品管理 API
// ========================================

// GetSceneItems 获取场景所有物品
func (h *Handler) GetSceneItems(c *gin.Context) {
	sceneID := c.Param("id")
	if sceneID == "" {
		h.Response.BadRequest(c, "场景ID不能为空")
		return
	}

	// 获取物品服务
	container := di.GetContainer()
	itemService, ok := container.Get("item").(*services.ItemService)
	if !ok || itemService == nil {
		h.Response.InternalError(c, "物品服务未初始化", "无法获取物品服务实例")
		return
	}

	items, err := itemService.GetAllItems(sceneID)
	if err != nil {
		h.Response.InternalError(c, "获取物品列表失败", err.Error())
		return
	}

	sceneItems := make([]*models.Item, 0, len(items))
	ownedItems := make([]*models.Item, 0)
	for _, item := range items {
		if item.IsInventoryOnly() {
			ownedItems = append(ownedItems, item)
		} else {
			sceneItems = append(sceneItems, item)
		}
	}

	response := struct {
		SceneItems []*models.Item `json:"scene_items"`
		OwnedItems []*models.Item `json:"owned_items"`
		Total      int            `json:"total"`
	}{
		SceneItems: sceneItems,
		OwnedItems: ownedItems,
		Total:      len(items),
	}

	h.Response.Success(c, response, "物品列表获取成功")
}

// GetSceneItem 获取场景指定物品
func (h *Handler) GetSceneItem(c *gin.Context) {
	sceneID := c.Param("id")
	itemID := c.Param("item_id")

	if sceneID == "" || itemID == "" {
		h.Response.BadRequest(c, "场景ID和物品ID不能为空")
		return
	}

	// 获取物品服务
	container := di.GetContainer()
	itemService, ok := container.Get("item").(*services.ItemService)
	if !ok || itemService == nil {
		h.Response.InternalError(c, "物品服务未初始化", "无法获取物品服务实例")
		return
	}

	item, err := itemService.GetItem(sceneID, itemID)
	if err != nil {
		h.Response.NotFound(c, "物品", "物品ID: "+itemID)
		return
	}

	h.Response.Success(c, item, "物品信息获取成功")
}

// AddSceneItem 添加物品到场景
func (h *Handler) AddSceneItem(c *gin.Context) {
	sceneID := c.Param("id")
	if sceneID == "" {
		h.Response.BadRequest(c, "场景ID不能为空")
		return
	}

	var item models.Item
	if err := c.ShouldBindJSON(&item); err != nil {
		h.Response.BadRequest(c, "无效的请求数据", err.Error())
		return
	}

	// 设置场景ID
	item.SceneID = sceneID
	item.ID = fmt.Sprintf("item_%d", time.Now().UnixNano())
	item.CreatedAt = time.Now()
	item.LastUpdated = time.Now()

	// 获取物品服务
	container := di.GetContainer()
	itemService, ok := container.Get("item").(*services.ItemService)
	if !ok || itemService == nil {
		h.Response.InternalError(c, "物品服务未初始化", "无法获取物品服务实例")
		return
	}

	if err := itemService.AddItem(sceneID, &item); err != nil {
		h.Response.InternalError(c, "添加物品失败", err.Error())
		return
	}

	h.Response.Created(c, item, "物品添加成功")
}

// UpdateSceneItem 更新场景物品
func (h *Handler) UpdateSceneItem(c *gin.Context) {
	sceneID := c.Param("id")
	itemID := c.Param("item_id")

	if sceneID == "" || itemID == "" {
		h.Response.BadRequest(c, "场景ID和物品ID不能为空")
		return
	}

	var item models.Item
	if err := c.ShouldBindJSON(&item); err != nil {
		h.Response.BadRequest(c, "无效的请求数据", err.Error())
		return
	}

	// 确保ID一致
	item.ID = itemID
	item.SceneID = sceneID
	item.LastUpdated = time.Now()

	// 获取物品服务
	container := di.GetContainer()
	itemService, ok := container.Get("item").(*services.ItemService)
	if !ok || itemService == nil {
		h.Response.InternalError(c, "物品服务未初始化", "无法获取物品服务实例")
		return
	}

	if err := itemService.UpdateItem(sceneID, &item); err != nil {
		h.Response.InternalError(c, "更新物品失败", err.Error())
		return
	}

	h.Response.Success(c, item, "物品更新成功")
}

// DeleteSceneItem 删除场景物品
func (h *Handler) DeleteSceneItem(c *gin.Context) {
	sceneID := c.Param("id")
	itemID := c.Param("item_id")

	if sceneID == "" || itemID == "" {
		h.Response.BadRequest(c, "场景ID和物品ID不能为空")
		return
	}

	// 获取物品服务
	container := di.GetContainer()
	itemService, ok := container.Get("item").(*services.ItemService)
	if !ok || itemService == nil {
		h.Response.InternalError(c, "物品服务未初始化", "无法获取物品服务实例")
		return
	}

	if err := itemService.DeleteItem(sceneID, itemID); err != nil {
		h.Response.InternalError(c, "删除物品失败", err.Error())
		return
	}

	h.Response.Success(c, nil, "物品删除成功")
}

// ========================================
// 故事高级功能 API
// ========================================

// CompleteTaskObjective 完成任务目标
func (h *Handler) CompleteTaskObjective(c *gin.Context) {
	sceneID := c.Param("id")
	taskID := c.Param("task_id")
	objectiveID := c.Param("objective_id")

	if sceneID == "" || taskID == "" || objectiveID == "" {
		h.Response.BadRequest(c, "场景ID、任务ID和目标ID不能为空")
		return
	}

	storyService := h.getStoryService()
	if storyService == nil {
		h.Response.InternalError(c, "故事服务未初始化", "无法获取故事服务实例")
		return
	}

	if err := storyService.CompleteObjective(sceneID, taskID, objectiveID); err != nil {
		h.Response.InternalError(c, "完成目标失败", err.Error())
		return
	}

	// 获取更新后的故事数据
	storyData, err := storyService.GetStoryForScene(sceneID)
	if err != nil {
		h.Response.InternalError(c, "获取故事数据失败", err.Error())
		return
	}

	h.Response.Success(c, storyData, "目标完成成功")
}

// UnlockStoryLocation 解锁故事地点
func (h *Handler) UnlockStoryLocation(c *gin.Context) {
	sceneID := c.Param("id")
	locationID := c.Param("location_id")

	if sceneID == "" || locationID == "" {
		h.Response.BadRequest(c, "场景ID和地点ID不能为空")
		return
	}

	storyService := h.getStoryService()
	if storyService == nil {
		h.Response.InternalError(c, "故事服务未初始化", "无法获取故事服务实例")
		return
	}

	if err := storyService.UnlockLocation(sceneID, locationID); err != nil {
		h.Response.InternalError(c, "解锁地点失败", err.Error())
		return
	}

	// 获取更新后的故事数据
	storyData, err := storyService.GetStoryForScene(sceneID)
	if err != nil {
		h.Response.InternalError(c, "获取故事数据失败", err.Error())
		return
	}

	h.Response.Success(c, storyData, "地点解锁成功")
}

// ExploreStoryLocation 探索故事地点
func (h *Handler) ExploreStoryLocation(c *gin.Context) {
	sceneID := c.Param("id")
	locationID := c.Param("location_id")

	if sceneID == "" || locationID == "" {
		h.Response.BadRequest(c, "场景ID和地点ID不能为空")
		return
	}

	// 解析用户偏好（如果提供）
	var preferences *models.UserPreferences
	if prefJSON := c.Query("preferences"); prefJSON != "" {
		preferences = &models.UserPreferences{}
		if err := json.Unmarshal([]byte(prefJSON), preferences); err != nil {
			preferences = nil
		}
	}

	if preferences == nil {
		preferences = &models.UserPreferences{
			CreativityLevel: models.CreativityBalanced,
			AllowPlotTwists: true,
		}
	}

	storyService := h.getStoryService()
	if storyService == nil {
		h.Response.InternalError(c, "故事服务未初始化", "无法获取故事服务实例")
		return
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := storyService.ExploreLocation(sceneID, locationID, preferences)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			h.Response.Error(c, http.StatusRequestTimeout, "REQUEST_TIMEOUT",
				"探索操作超时", "请稍后重试")
			return
		}
		h.Response.InternalError(c, "探索地点失败", err.Error())
		return
	}

	h.Response.Success(c, result, "地点探索成功")
}

// GetAvailableStoryChoices 获取当前可用的故事选择
func (h *Handler) GetAvailableStoryChoices(c *gin.Context) {
	sceneID := c.Param("id")
	if sceneID == "" {
		h.Response.BadRequest(c, "场景ID不能为空")
		return
	}

	storyService := h.getStoryService()
	if storyService == nil {
		h.Response.InternalError(c, "故事服务未初始化", "无法获取故事服务实例")
		return
	}

	choices, err := storyService.GetAvailableChoices(sceneID)
	if err != nil {
		h.Response.InternalError(c, "获取故事选择失败", err.Error())
		return
	}

	h.Response.Success(c, choices, "故事选择获取成功")
}

// GetSceneStats 获取场景统计
func (h *Handler) GetSceneStats(c *gin.Context) {
	sceneIDStr := c.Param("id")
	_, err := strconv.ParseUint(sceneIDStr, 10, 32)
	if err != nil {
		h.Response.Error(c, http.StatusBadRequest, "INVALID_SCENE_ID", "Invalid scene ID", err.Error())
		return
	}

	stats := map[string]interface{}{
		"scene_id":                sceneIDStr,
		"character_count":         0,
		"conversation_count":      0,
		"interaction_count":       0,
		"story_progress":          0.0,
		"last_activity":           time.Now(),
		"created_at":              time.Now(),
		"character_interactions":  []interface{}{},
		"character_relationships": []interface{}{},
		"interaction_timeline":    []interface{}{},
	}

	h.Response.Success(c, stats, "Scene statistics retrieved successfully")
}

// GetSceneConversations 获取场景对话
func (h *Handler) GetSceneConversations(c *gin.Context) {
	sceneID := c.Param("id")
	if sceneID == "" {
		h.Response.BadRequest(c, "场景ID不能为空")
		return
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			if parsedLimit > 200 {
				parsedLimit = 200
			}
			limit = parsedLimit
		}
	}

	conversations := make([]models.Conversation, 0)
	if h.ContextService != nil {
		if convs, err := h.ContextService.GetRecentConversations(sceneID, limit); err == nil {
			conversations = convs
		} else {
			log.Printf("警告: 获取场景(%s)对话失败: %v", sceneID, err)
		}
	} else {
		log.Printf("警告: ContextService 未初始化，无法获取场景(%s)对话", sceneID)
	}

	h.Response.Success(c, map[string]interface{}{
		"scene_id":      sceneID,
		"conversations": conversations,
		"total":         len(conversations),
		"limit":         limit,
	}, "Scene conversations retrieved successfully")
}

// CreateSceneConversation 创建场景对话
func (h *Handler) CreateSceneConversation(c *gin.Context) {
	sceneIDStr := c.Param("id")
	sceneID, err := strconv.ParseUint(sceneIDStr, 10, 32)
	if err != nil {
		h.Response.Error(c, http.StatusBadRequest, "INVALID_SCENE_ID", "Invalid scene ID", err.Error())
		return
	}

	var conversationData struct {
		SpeakerID   string                 `json:"speaker_id"`
		Message     string                 `json:"message"`
		MessageType string                 `json:"message_type"`
		Context     map[string]interface{} `json:"context"`
		Emotions    []string               `json:"emotions"`
	}

	if err := c.ShouldBindJSON(&conversationData); err != nil {
		h.Response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", err.Error())
		return
	}

	// 创建对话记录
	conversation := map[string]interface{}{
		"id":           fmt.Sprintf("conv_%d_%d", sceneID, time.Now().Unix()),
		"scene_id":     sceneIDStr,
		"speaker_id":   conversationData.SpeakerID,
		"message":      conversationData.Message,
		"message_type": conversationData.MessageType,
		"emotions":     conversationData.Emotions,
		"context":      conversationData.Context,
		"timestamp":    time.Now(),
	}

	h.Response.Success(c, conversation, "Conversation created successfully")
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

// Login 处理用户登录请求
func (h *Handler) Login(c *gin.Context) {
	// 从请求体获取登录凭据
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.Response.BadRequest(c, "登录凭据格式错误", err.Error())
		return
	}

	// 在开发环境中，简化登录流程，后续可扩展为真正的用户认证
	// 建议在生产环境中实现安全的用户认证系统
	cfg := config.GetCurrentConfig()

	// 检查是否为默认开发账户
	isValid := false
	if cfg.DebugMode {
		// 开发模式下，接受默认凭据
		isValid = (req.Username == "admin" && req.Password == "admin") ||
			(req.Username == "user" && req.Password == "user")
	} else {
		// 生产模式下，可以使用配置的服务来验证用户
		// 简单起见，这里直接获取用户，如果存在则认为有效
		_, err := h.UserService.GetUser(req.Username)
		isValid = (err == nil)
	}

	if !isValid {
		h.Response.Error(c, http.StatusUnauthorized, "INVALID_CREDENTIALS",
			"用户名或密码错误", "请检查您的登录凭据")
		return
	}

	// 生成认证令牌
	token, err := GenerateUserToken(req.Username)
	if err != nil {
		h.Response.InternalError(c, "生成认证令牌失败", err.Error())
		return
	}

	// 更新用户最后登录时间
	user, err := h.UserService.GetUser(req.Username)
	if err == nil {
		// 更新用户的最后登录时间
		user.LastLogin = time.Now()
		h.UserService.SaveUser(user)
		h.Logger.Info("User login successful", map[string]interface{}{
			"username":  req.Username,
			"client_ip": c.ClientIP(),
		})
	} else {
		// 如果用户不存在但在开发模式下有效，创建临时用户
		if cfg.DebugMode {
			h.Logger.Info("Development login successful", map[string]interface{}{
				"username":  req.Username,
				"client_ip": c.ClientIP(),
			})
		}
	}

	// 返回成功响应
	h.Response.Success(c, gin.H{
		"token":   token,
		"user_id": req.Username,
	}, "登录成功")
}

// Logout 处理用户登出请求
func (h *Handler) Logout(c *gin.Context) {
	// 在实际实现中，可能需要将令牌加入黑名单
	// 但现在我们简单地删除令牌并返回成功

	userID, isAuthenticated := GetUserFromContext(c)
	if isAuthenticated {
		h.Logger.Info("User logged out", map[string]interface{}{
			"user_id":   userID,
			"client_ip": c.ClientIP(),
		})
	}

	h.Response.Success(c, nil, "登出成功")
}

// ========================================
// 用户管理 API Handlers
// ========================================

// GetUserProfile 获取用户档案
func (h *Handler) GetUserProfile(c *gin.Context) {
	userID := c.Param("user_id")

	// 权限检查 - 确保用户只能访问自己的档案
	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权访问其他用户的档案", "")
		return
	}

	user, err := h.UserService.GetUser(userID)
	if err != nil {
		h.Response.NotFound(c, "用户", "用户ID: "+userID)
		return
	}

	h.Response.Success(c, user, "用户档案获取成功")
}

// UpdateUserProfile 更新用户档案
func (h *Handler) UpdateUserProfile(c *gin.Context) {
	userID := c.Param("user_id")

	// 权限检查
	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权修改其他用户的档案", "")
		return
	}

	var req struct {
		Username    string                  `json:"username,omitempty"`
		DisplayName string                  `json:"display_name,omitempty"`
		Bio         string                  `json:"bio,omitempty"`
		Avatar      string                  `json:"avatar,omitempty"`
		Preferences *models.UserPreferences `json:"preferences,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.Response.BadRequest(c, "无效的请求数据", err.Error())
		return
	}

	user, err := h.UserService.GetUser(userID)
	if err != nil {
		h.Response.NotFound(c, "用户", "用户ID: "+userID)
		return
	}

	if req.Username != "" {
		user.Username = req.Username
	}
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Bio != "" {
		user.Bio = req.Bio
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}
	if req.Preferences != nil {
		user.Preferences = *req.Preferences
	}

	if err := h.UserService.SaveUser(user); err != nil {
		h.Response.InternalError(c, "更新用户档案失败", err.Error())
		return
	}

	h.Response.Success(c, user, "用户档案更新成功")
}

// GetUserPreferences 获取用户偏好
func (h *Handler) GetUserPreferences(c *gin.Context) {
	userID := c.Param("user_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权访问其他用户的偏好设置", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	preferences, err := h.UserService.GetUserPreferences(userID)
	if err != nil {
		h.Response.InternalError(c, "获取用户偏好失败", err.Error())
		return
	}

	h.Response.Success(c, preferences, "用户偏好获取成功")
}

// UpdateUserPreferences 更新用户偏好
func (h *Handler) UpdateUserPreferences(c *gin.Context) {
	userID := c.Param("user_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权修改其他用户的偏好设置", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	var preferences models.UserPreferences
	if err := c.ShouldBindJSON(&preferences); err != nil {
		h.Response.BadRequest(c, "无效的请求数据", err.Error())
		return
	}

	if err := h.UserService.UpdateUserPreferences(userID, preferences); err != nil {
		h.Response.InternalError(c, "更新用户偏好失败", err.Error())
		return
	}

	h.Response.Success(c, preferences, "用户偏好更新成功")
}

// GetUserItems 获取用户物品
func (h *Handler) GetUserItems(c *gin.Context) {
	userID := c.Param("user_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权访问其他用户的物品", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	items, err := h.UserService.GetUserItems(userID)
	if err != nil {
		h.Response.InternalError(c, "获取用户物品失败", err.Error())
		return
	}

	h.Response.Success(c, items, "用户物品列表获取成功")
}

// AddUserItem 添加用户物品
func (h *Handler) AddUserItem(c *gin.Context) {
	userID := c.Param("user_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权为其他用户添加物品", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	var item models.UserItem
	if err := c.ShouldBindJSON(&item); err != nil {
		h.Response.BadRequest(c, "无效的请求数据", err.Error())
		return
	}

	if item.ID == "" {
		item.ID = fmt.Sprintf("item_%d", time.Now().UnixNano())
	}

	if err := h.UserService.AddUserItem(userID, item); err != nil {
		h.Response.InternalError(c, "添加物品失败", err.Error())
		return
	}

	created, err := h.UserService.GetUserItem(userID, item.ID)
	if err != nil {
		h.Response.Created(c, item, "物品添加成功")
		return
	}

	h.Response.Created(c, created, "物品添加成功")
}

// GetUserItem 获取单个用户物品
func (h *Handler) GetUserItem(c *gin.Context) {
	userID := c.Param("user_id")
	itemID := c.Param("item_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权访问其他用户的物品", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	item, err := h.UserService.GetUserItem(userID, itemID)
	if err != nil {
		h.Response.NotFound(c, "物品", "物品ID: "+itemID)
		return
	}

	h.Response.Success(c, item, "物品信息获取成功")
}

// UpdateUserItem 更新用户物品
func (h *Handler) UpdateUserItem(c *gin.Context) {
	userID := c.Param("user_id")
	itemID := c.Param("item_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权修改其他用户的物品", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	var item models.UserItem
	if err := c.ShouldBindJSON(&item); err != nil {
		h.Response.BadRequest(c, "无效的请求数据", err.Error())
		return
	}

	item.ID = itemID
	if err := h.UserService.UpdateUserItem(userID, itemID, item); err != nil {
		h.Response.InternalError(c, "更新物品失败", err.Error())
		return
	}

	updated, err := h.UserService.GetUserItem(userID, itemID)
	if err != nil {
		h.Response.Success(c, item, "物品更新成功")
		return
	}

	h.Response.Success(c, updated, "物品更新成功")
}

// DeleteUserItem 删除用户物品
func (h *Handler) DeleteUserItem(c *gin.Context) {
	userID := c.Param("user_id")
	itemID := c.Param("item_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权删除其他用户的物品", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	if err := h.UserService.DeleteUserItem(userID, itemID); err != nil {
		h.Response.InternalError(c, "删除物品失败", err.Error())
		return
	}

	h.Response.Success(c, nil, "物品删除成功")
}

type userSkillRequest struct {
	models.UserSkill
	LegacyEffect string `json:"effect,omitempty"`
}

func (req *userSkillRequest) toModel() models.UserSkill {
	skill := req.UserSkill
	if len(skill.Effects) == 0 {
		legacy := strings.TrimSpace(req.LegacyEffect)
		if legacy != "" {
			skill.Effects = []models.SkillEffect{
				{
					Description: legacy,
					Target:      "other",
					Type:        "special",
					Value:       0,
					Probability: 1,
				},
			}
		}
	}

	return skill
}

func extractSkillValidationMessage(err error) string {
	message := err.Error()
	prefix := services.ErrSkillValidation.Error() + ": "
	if strings.HasPrefix(message, prefix) {
		return strings.TrimPrefix(message, prefix)
	}
	if message == services.ErrSkillValidation.Error() {
		return "技能数据验证失败"
	}
	return message
}

// GetUserSkills 获取用户技能
func (h *Handler) GetUserSkills(c *gin.Context) {
	userID := c.Param("user_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权访问其他用户的技能", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	skills, err := h.UserService.GetUserSkills(userID)
	if err != nil {
		h.Response.InternalError(c, "获取用户技能失败", err.Error())
		return
	}

	h.Response.Success(c, skills, "用户技能列表获取成功")
}

// AddUserSkill 添加用户技能
func (h *Handler) AddUserSkill(c *gin.Context) {
	userID := c.Param("user_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权为其他用户添加技能", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	var req userSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Response.BadRequest(c, "无效的请求数据", err.Error())
		return
	}

	skill := req.toModel()
	if skill.ID == "" {
		skill.ID = fmt.Sprintf("skill_%d", time.Now().UnixNano())
	}

	if err := h.UserService.AddUserSkill(userID, skill); err != nil {
		if errors.Is(err, services.ErrSkillValidation) {
			h.Response.BadRequest(c, extractSkillValidationMessage(err), "")
			return
		}
		h.Response.InternalError(c, "添加技能失败", err.Error())
		return
	}

	saved, err := h.UserService.GetUserSkill(userID, skill.ID)
	if err != nil {
		h.Response.Created(c, skill, "技能添加成功")
		return
	}

	h.Response.Created(c, saved, "技能添加成功")
}

// GetUserSkill 获取单个用户技能
func (h *Handler) GetUserSkill(c *gin.Context) {
	userID := c.Param("user_id")
	skillID := c.Param("skill_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权访问其他用户的技能", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	skill, err := h.UserService.GetUserSkill(userID, skillID)
	if err != nil {
		h.Response.NotFound(c, "技能", "技能ID: "+skillID)
		return
	}

	h.Response.Success(c, skill, "技能信息获取成功")
}

// UpdateUserSkill 更新用户技能
func (h *Handler) UpdateUserSkill(c *gin.Context) {
	userID := c.Param("user_id")
	skillID := c.Param("skill_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权修改其他用户的技能", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	var req userSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Response.BadRequest(c, "无效的请求数据", err.Error())
		return
	}

	skill := req.toModel()
	skill.ID = skillID
	if err := h.UserService.UpdateUserSkill(userID, skillID, skill); err != nil {
		if errors.Is(err, services.ErrSkillValidation) {
			h.Response.BadRequest(c, extractSkillValidationMessage(err), "")
			return
		}
		h.Response.InternalError(c, "更新技能失败", err.Error())
		return
	}

	updated, err := h.UserService.GetUserSkill(userID, skillID)
	if err != nil {
		h.Response.Success(c, skill, "技能更新成功")
		return
	}

	h.Response.Success(c, updated, "技能更新成功")
}

// DeleteUserSkill 删除用户技能
func (h *Handler) DeleteUserSkill(c *gin.Context) {
	userID := c.Param("user_id")
	skillID := c.Param("skill_id")

	currentUserID, _ := GetUserFromContext(c)
	if currentUserID != userID {
		h.Response.Error(c, http.StatusForbidden, "FORBIDDEN",
			"无权删除其他用户的技能", "")
		return
	}

	if err := h.UserService.EnsureUserExists(userID); err != nil {
		h.Response.InternalError(c, "初始化用户数据失败", err.Error())
		return
	}

	if err := h.UserService.DeleteUserSkill(userID, skillID); err != nil {
		h.Response.InternalError(c, "删除技能失败", err.Error())
		return
	}

	h.Response.Success(c, nil, "技能删除成功")
}
