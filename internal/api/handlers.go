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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/llm"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocket 升级器配置
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 在生产环境中应该进行更严格的检查
		return true
	},
}

// WebSocket 连接管理器
type WebSocketManager struct {
	connections map[string]map[*websocket.Conn]bool // sceneID -> connections
	broadcast   chan []byte
	register    chan *WebSocketClient
	unregister  chan *WebSocketClient
	mutex       sync.RWMutex
}

type WebSocketClient struct {
	conn    *websocket.Conn
	sceneID string
	userID  string
	send    chan []byte
}

// 全局 WebSocket 管理器
var wsManager = &WebSocketManager{
	connections: make(map[string]map[*websocket.Conn]bool),
	broadcast:   make(chan []byte),
	register:    make(chan *WebSocketClient),
	unregister:  make(chan *WebSocketClient),
}

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

// -----------------------------------------------------------------------------
// 初始化 WebSocket 管理器
func init() {
	go wsManager.run()
}

// 运行 WebSocket 管理器
func (manager *WebSocketManager) run() {
	for {
		select {
		case client := <-manager.register:
			manager.mutex.Lock()
			if manager.connections[client.sceneID] == nil {
				manager.connections[client.sceneID] = make(map[*websocket.Conn]bool)
			}
			manager.connections[client.sceneID][client.conn] = true
			manager.mutex.Unlock()

			log.Printf("✅ WebSocket 客户端已连接到场景 %s", client.sceneID)

		case client := <-manager.unregister:
			manager.mutex.Lock()
			if connections, ok := manager.connections[client.sceneID]; ok {
				if _, ok := connections[client.conn]; ok {
					delete(connections, client.conn)
					close(client.send)

					// 如果场景没有连接了，清理映射
					if len(connections) == 0 {
						delete(manager.connections, client.sceneID)
					}
				}
			}
			manager.mutex.Unlock()

			log.Printf("❌ WebSocket 客户端已断开场景 %s", client.sceneID)

		case message := <-manager.broadcast:
			manager.mutex.RLock()
			for _, connections := range manager.connections {
				for conn := range connections {
					if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
						log.Printf("❌ 广播消息失败: %v", err)
						conn.Close()
						delete(connections, conn)
					}
				}
			}
			manager.mutex.RUnlock()
		}
	}
}

// SceneWebSocket 处理场景 WebSocket 连接
func (h *Handler) SceneWebSocket(c *gin.Context) {
	sceneID := c.Param("id")

	// 验证场景是否存在
	_, err := h.SceneService.LoadScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "场景不存在"})
		return
	}

	// 升级 HTTP 连接到 WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ WebSocket 升级失败: %v", err)
		return
	}
	defer conn.Close()

	// 获取用户ID（从查询参数或会话中）
	userID := c.DefaultQuery("user_id", "anonymous")

	// 创建客户端
	client := &WebSocketClient{
		conn:    conn,
		sceneID: sceneID,
		userID:  userID,
		send:    make(chan []byte, 256),
	}

	// 注册客户端
	wsManager.register <- client
	defer func() {
		wsManager.unregister <- client
	}()

	// 启动读写协程
	go h.handleWebSocketWrites(client)
	go h.handleWebSocketReads(client)

	// 发送连接确认消息
	welcomeMsg := map[string]interface{}{
		"type":      "connected",
		"scene_id":  sceneID,
		"user_id":   userID,
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "WebSocket 连接已建立",
	}

	if msgBytes, err := json.Marshal(welcomeMsg); err == nil {
		select {
		case client.send <- msgBytes:
		default:
			// 队列满，跳过初始消息
		}
	}

	// 保持连接活跃 - 使用更好的方式
	<-c.Request.Context().Done()
	log.Printf("📱 场景 %s 的 WebSocket 连接已关闭", sceneID)
}

// UserStatusWebSocket 处理用户状态 WebSocket 连接
func (h *Handler) UserStatusWebSocket(c *gin.Context) {
	// 升级 HTTP 连接到 WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ 用户状态 WebSocket 升级失败: %v", err)
		return
	}
	defer conn.Close()

	// 获取用户ID
	userID := c.DefaultQuery("user_id", "anonymous")
	log.Printf("📱 用户状态 WebSocket 连接已建立: %s", userID)

	// 创建客户端
	client := &WebSocketClient{
		conn:    conn,
		sceneID: "user_status", // 特殊的场景ID用于用户状态
		userID:  userID,
		send:    make(chan []byte, 256),
	}

	// 注册客户端
	wsManager.register <- client
	defer func() {
		wsManager.unregister <- client
	}()

	// 启动读写协程
	go h.handleWebSocketWrites(client)
	go h.handleWebSocketReads(client)

	// 发送连接确认消息
	welcomeMsg := map[string]interface{}{
		"type":      "user_status_connected",
		"user_id":   userID,
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "用户状态连接已建立",
	}
	if msgBytes, err := json.Marshal(welcomeMsg); err == nil {
		select {
		case client.send <- msgBytes:
		default:
			// 队列满，跳过初始消息
		}
	}

	// 定期发送心跳和状态更新
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 发送心跳消息
			heartbeat := map[string]interface{}{
				"type":      "heartbeat",
				"timestamp": time.Now().Unix(),
			}
			if msgBytes, err := json.Marshal(heartbeat); err == nil {
				select {
				case client.send <- msgBytes:
				default:
					return // 客户端断开连接
				}
			}
		case <-c.Request.Context().Done():
			// HTTP 请求上下文取消，退出循环
			return
		}
	}
}

// handleWebSocketReads 处理 WebSocket 读取
func (h *Handler) handleWebSocketReads(client *WebSocketClient) {
	defer func() {
		wsManager.unregister <- client
		client.conn.Close()
	}()

	// 设置读取超时
	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var message map[string]interface{}
		err := client.conn.ReadJSON(&message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("❌ WebSocket 读取错误: %v", err)
			}
			break
		}

		// 处理收到的消息
		h.handleWebSocketMessage(client, message)
	}
}

// handleWebSocketWrites 处理 WebSocket 写入
func (h *Handler) handleWebSocketWrites(client *WebSocketClient) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleWebSocketMessage 处理 WebSocket 消息
func (h *Handler) handleWebSocketMessage(client *WebSocketClient, message map[string]interface{}) {
	msgType, ok := message["type"].(string)
	if !ok {
		log.Printf("⚠️ 收到无效的消息类型")
		return
	}

	switch msgType {
	case "character_interaction":
		h.handleCharacterInteractionMessage(client, message)

	case "story_choice":
		h.handleStoryChoiceMessage(client, message)

	case "user_status_update":
		h.handleUserStatusUpdateMessage(client, message)

	case "ping":
		// 响应 ping 消息
		pong := map[string]interface{}{
			"type":      "pong",
			"timestamp": time.Now().Unix(),
		}
		if msgBytes, err := json.Marshal(pong); err == nil {
			select {
			case client.send <- msgBytes:
			default:
				// 发送队列满，跳过
			}
		}

	default:
		log.Printf("⚠️ 未知的消息类型: %s", msgType)
	}
}

// handleCharacterInteractionMessage 处理角色交互消息
func (h *Handler) handleCharacterInteractionMessage(client *WebSocketClient, message map[string]interface{}) {
	characterID, ok := message["character_id"].(string)
	if !ok {
		h.sendErrorMessage(client, "缺少角色ID")
		return
	}

	userMessage, ok := message["message"].(string)
	if !ok {
		h.sendErrorMessage(client, "缺少消息内容")
		return
	}

	// 生成角色回应
	response, err := h.CharacterService.GenerateResponse(client.sceneID, characterID, userMessage)
	if err != nil {
		h.sendErrorMessage(client, "生成回应失败: "+err.Error())
		return
	}

	// 广播新对话给场景中的所有客户端
	conversationMsg := map[string]interface{}{
		"type":         "conversation:new",
		"scene_id":     client.sceneID,
		"character_id": characterID,
		"speaker_id":   characterID,
		"conversation": response,
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	h.broadcastToScene(client.sceneID, conversationMsg)
}

// handleStoryChoiceMessage 处理故事选择消息
func (h *Handler) handleStoryChoiceMessage(client *WebSocketClient, message map[string]interface{}) {
	nodeID, ok := message["node_id"].(string)
	if !ok {
		h.sendErrorMessage(client, "缺少节点ID")
		return
	}

	choiceID, ok := message["choice_id"].(string)
	if !ok {
		h.sendErrorMessage(client, "缺少选择ID")
		return
	}

	// 解析用户偏好（可选）
	var preferences *models.UserPreferences
	if prefData, exists := message["user_preferences"]; exists {
		if prefMap, ok := prefData.(map[string]interface{}); ok {
			preferences = &models.UserPreferences{}
			// 解析偏好设置
			if creativity, ok := prefMap["creativity_level"].(string); ok {
				preferences.CreativityLevel = models.CreativityLevel(creativity)
			}
			if plotTwists, ok := prefMap["allow_plot_twists"].(bool); ok {
				preferences.AllowPlotTwists = plotTwists
			}
		}
	}

	// 获取故事服务
	storyService := h.getStoryService()
	if storyService == nil {
		h.sendErrorMessage(client, "故事服务未初始化")
		return
	}

	// 执行故事选择
	nextNode, err := storyService.MakeChoice(client.sceneID, nodeID, choiceID, preferences)
	if err != nil {
		h.sendErrorMessage(client, "执行故事选择失败: "+err.Error())
		return
	}

	// 广播故事更新给场景中的所有客户端
	h.broadcastToScene(client.sceneID, map[string]interface{}{
		"type": "story:choice_made",
		"data": map[string]interface{}{
			"node_id":   nodeID,
			"choice_id": choiceID,
			"next_node": nextNode,
			"user_id":   client.userID,
		},
	})
}

// handleUserStatusUpdateMessage 处理用户状态更新消息
func (h *Handler) handleUserStatusUpdateMessage(client *WebSocketClient, message map[string]interface{}) {
	status, ok := message["status"].(string)
	if !ok {
		h.sendErrorMessage(client, "缺少状态信息")
		return
	}

	// 广播用户状态更新
	statusUpdateMsg := map[string]interface{}{
		"type":      "user:presence",
		"user_id":   client.userID,
		"scene_id":  client.sceneID,
		"status":    status,
		"action":    message["action"], // joined, left, etc.
		"timestamp": time.Now().Format(time.RFC3339),
	}

	h.broadcastToScene(client.sceneID, statusUpdateMsg)
}

// sendErrorMessage 发送错误消息给客户端
func (h *Handler) sendErrorMessage(client *WebSocketClient, errorMsg string) {
	errorResponse := map[string]interface{}{
		"type":      "error",
		"error":     errorMsg,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if msgBytes, err := json.Marshal(errorResponse); err == nil {
		select {
		case client.send <- msgBytes:
		default:
			// 客户端发送队列已满，跳过
			log.Printf("⚠️ 无法发送错误消息到客户端，队列已满")
		}
	}
}

// broadcastToScene 向场景中的所有客户端广播消息
func (h *Handler) broadcastToScene(sceneID string, message map[string]interface{}) {
	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("❌ 序列化广播消息失败: %v", err)
		return
	}

	wsManager.mutex.RLock()
	connections, exists := wsManager.connections[sceneID]
	wsManager.mutex.RUnlock()

	if !exists {
		return
	}

	for conn := range connections {
		if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
			log.Printf("❌ 广播消息失败: %v", err)
			conn.Close()

			// 清理断开的连接
			wsManager.mutex.Lock()
			delete(connections, conn)
			wsManager.mutex.Unlock()
		}
	}
}

// BroadcastToScene 提供外部调用的广播方法
func (h *Handler) BroadcastToScene(sceneID string, message map[string]interface{}) {
	h.broadcastToScene(sceneID, message)
}

// GetWebSocketStatus 获取 WebSocket 连接状态（调试用）
func (h *Handler) GetWebSocketStatus(c *gin.Context) {
	wsManager.mutex.RLock()
	status := make(map[string]int)
	totalConnections := 0

	for sceneID, connections := range wsManager.connections {
		count := len(connections)
		status[sceneID] = count
		totalConnections += count
	}
	wsManager.mutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"total_connections": totalConnections,
		"scene_connections": status,
		"timestamp":         time.Now().Format(time.RFC3339),
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

	// 获取导出服务
	exportService := h.getExportService()
	if exportService == nil {
		h.respondWithError(c, http.StatusInternalServerError, "导出服务未初始化")
		return
	}

	// 导出场景数据
	result, err := exportService.ExportSceneData(c.Request.Context(), sceneID, format, includeConversations)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "导出场景数据失败: "+err.Error())
		return
	}

	// 统一处理导出响应
	h.handleExportResponse(c, result, format)
}

// ExportInteractions 导出互动摘要
func (h *Handler) ExportInteractions(c *gin.Context) {
	sceneID := c.Param("id")
	format := c.DefaultQuery("format", "json")

	// 获取导出服务
	exportService := h.getExportService()
	if exportService == nil {
		h.respondWithError(c, http.StatusInternalServerError, "导出服务未初始化")
		return
	}

	// 导出互动摘要
	result, err := exportService.ExportInteractionSummary(c.Request.Context(), sceneID, format)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "导出互动摘要失败: "+err.Error())
		return
	}

	// 统一处理导出响应
	h.handleExportResponse(c, result, format)
}

// ExportStory 导出故事文档
func (h *Handler) ExportStory(c *gin.Context) {
	sceneID := c.Param("id")
	format := c.DefaultQuery("format", "json")

	// 获取导出服务
	exportService := h.getExportService()
	if exportService == nil {
		h.respondWithError(c, http.StatusInternalServerError, "导出服务未初始化")
		return
	}

	// 导出故事文档
	result, err := exportService.ExportStoryAsDocument(c.Request.Context(), sceneID, format)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "导出故事文档失败: "+err.Error())
		return
	}

	// 统一处理导出响应
	h.handleExportResponse(c, result, format)
}

// getExportService 获取导出服务实例
func (h *Handler) getExportService() *services.ExportService {
	container := di.GetContainer()

	// 尝试从容器获取
	if service, ok := container.Get("export").(*services.ExportService); ok {
		return service
	}

	// 如果不存在，创建新实例
	exportService := services.NewExportService(h.ContextService, h.getStoryService(), h.SceneService)

	// 注册到容器
	container.Register("export", exportService)

	return exportService
}

// handleExportResponse 统一处理导出响应
func (h *Handler) handleExportResponse(c *gin.Context, result *models.ExportResult, format string) {
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
	}
}

// GetScenes 获取所有场景列表
func (h *Handler) GetScenes(c *gin.Context) {
	scenes, err := h.SceneService.GetAllScenes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, scenes)
}

// GetScene 获取指定场景详情
func (h *Handler) GetScene(c *gin.Context) {
	sceneID := c.Param("id")
	sceneData, err := h.SceneService.LoadScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "场景不存在"})
		return
	}

	c.JSON(http.StatusOK, sceneData)
}

// CreateScene 从文本创建新场景
func (h *Handler) CreateScene(c *gin.Context) {
	var req struct {
		Title string `json:"title" binding:"required"`
		Text  string `json:"text" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 创建场景
	scene, err := h.SceneService.CreateSceneFromText(req.Text, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建场景失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, scene)
}

// GetCharacters 获取指定场景的所有角色
func (h *Handler) GetCharacters(c *gin.Context) {
	sceneID := c.Param("id")
	sceneData, err := h.SceneService.LoadScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "场景不存在"})
		return
	}

	c.JSON(http.StatusOK, sceneData.Characters)
}

// Chat 处理聊天请求
func (h *Handler) Chat(c *gin.Context) {
	var req struct {
		SceneID     string `json:"scene_id" binding:"required"`
		CharacterID string `json:"character_id" binding:"required"`
		Message     string `json:"message" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成角色回应
	response, err := h.CharacterService.GenerateResponse(req.SceneID, req.CharacterID, req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成回应失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetConversations 获取对话历史
func (h *Handler) GetConversations(c *gin.Context) {
	sceneID := c.Param("id")
	limitStr := c.DefaultQuery("limit", "20")

	var limit int
	if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
		limit = 20
	}

	conversations, err := h.ContextService.GetRecentConversations(sceneID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取对话失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, conversations)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "故事服务未初始化"})
		return
	}

	// 使用偏好设置进行故事选择
	nextNode, err := storyService.MakeChoice(sceneID, req.NodeID, req.ChoiceID, req.UserPreferences)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "执行故事选择失败: " + err.Error()})
		return
	}

	// 获取更新后的故事数据
	storyData, err := storyService.GetStoryData(sceneID, req.UserPreferences)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取故事数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "选择执行成功",
		"next_node":  nextNode,
		"story_data": storyData,
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

	// 安全地获取 LLM 配置信息
	llmConfig := make(map[string]interface{})
	if cfg.LLMConfig != nil {
		llmConfig["model"] = cfg.LLMConfig["model"]
		llmConfig["has_api_key"] = cfg.LLMConfig["api_key"] != ""
		// 不返回实际的 API key
	}

	c.JSON(http.StatusOK, gin.H{
		"llm_provider": cfg.LLMProvider,
		"debug_mode":   cfg.DebugMode,
		"port":         cfg.Port,
		"llm_config":   llmConfig,
	})
}

// 添加通用的设置保存方法
func (h *Handler) SaveSettings(c *gin.Context) {
	var request struct {
		LLMProvider string            `json:"llm_provider"`
		LLMConfig   map[string]string `json:"llm_config"`
		DebugMode   bool              `json:"debug_mode"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求数据: " + err.Error(),
		})
		return
	}

	// 保存LLM配置
	if request.LLMProvider != "" && request.LLMConfig != nil {
		err := h.ConfigService.UpdateLLMConfig(request.LLMProvider, request.LLMConfig, "web_ui")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "保存LLM配置失败: " + err.Error(),
			})
			return
		}
	}

	// 这里可以添加其他设置的保存逻辑
	// 比如保存 debug_mode 等

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "设置保存成功",
	})
}

// 添加连接测试方法
func (h *Handler) TestConnection(c *gin.Context) {
	// 获取LLM服务实例
	container := di.GetContainer()
	llmService, ok := container.Get("llm").(*services.LLMService)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "无法获取LLM服务实例",
		})
		return
	}

	// 测试连接
	if llmService.IsReady() {
		// 可以尝试发送一个简单的测试请求
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"message":  "连接测试成功",
			"provider": llmService.GetProviderName(),
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "LLM服务未就绪: " + llmService.GetReadyState(),
		})
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

	var modelValue string
	c.JSON(http.StatusOK, gin.H{
		"ready":    llmService.IsReady(),
		"status":   llmService.GetReadyState(),
		"provider": llmService.GetProviderName(),
		"config": map[string]interface{}{
			"provider": cfg.LLMProvider,
			// 返回API密钥的存在状态，但不返回实际密钥
			"has_api_key": cfg.LLMConfig != nil && cfg.LLMConfig["api_key"] != "",
			"model":       modelValue,
		},
	})
}

// UpdateLLMConfig 更新LLM配置
func (h *Handler) UpdateLLMConfig(c *gin.Context) {
	// 获取请求体
	var request struct {
		Provider string            `json:"provider"`
		Config   map[string]string `json:"config"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求数据: " + err.Error(),
		})
		return
	}

	// 验证请求数据
	if request.Provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "必须提供LLM服务提供商名称",
		})
		return
	}

	if request.Config == nil {
		request.Config = make(map[string]string)
	}

	// 调用配置服务更新LLM配置
	err := h.ConfigService.UpdateLLMConfig(request.Provider, request.Config, "web_ui")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新LLM配置失败: " + err.Error(),
		})
		return
	}

	// 获取LLM服务并重新初始化
	container := di.GetContainer()
	llmService, ok := container.Get("llm").(*services.LLMService)

	if !ok || llmService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新LLM服务失败: 无法获取LLM服务实例",
		})
		return
	}

	// 使用新配置更新提供商
	if err := llmService.UpdateProvider(request.Provider, request.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "更新LLM提供商失败: " + err.Error(),
			"details": "配置已保存但服务未更新，请重启应用",
		})
		return
	}

	// 更新分析服务（重新初始化分析服务以使用新的LLM提供商）
	// 创建新的分析服务
	newAnalyzerService := h.AnalyzerService // 先保留当前服务实例作为后备

	// 尝试创建更新的分析服务
	if llmService.IsReady() {
		// 如果LLM服务已就绪，尝试获取新的分析服务
		llmProvider := llmService.GetProvider()
		if llmProvider != nil {
			// 使用Provider创建专门的分析服务
			tmpService := services.NewAnalyzerServiceWithProvider(llmProvider)
			if tmpService != nil {
				newAnalyzerService = tmpService
				log.Printf("已使用新的LLM提供商(%s)更新分析服务", llmService.GetProviderName())
			}
		}
	} else {
		// LLM服务未就绪，使用默认分析服务
		tmpService, err := services.NewAnalyzerService()
		if err == nil && tmpService != nil {
			newAnalyzerService = tmpService
			log.Printf("已使用默认配置更新分析服务")
		}
	}

	// 更新handler中的分析服务实例
	h.AnalyzerService = newAnalyzerService

	// 返回更新后的状态
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "LLM配置已更新",
		"provider": request.Provider,
		"status":   llmService.GetReadyState(),
		"ready":    llmService.IsReady(),
	})
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
// @Summary 触发角色之间的互动对话
// @Description 根据指定主题生成多个角色之间的互动对话
// @Tags 角色互动
// @Accept json
// @Produce json
// @Param request body TriggerCharacterInteractionRequest true "互动请求参数"
// @Success 200 {object} models.CharacterInteraction "角色互动结果"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/interactions/trigger [post]
func TriggerCharacterInteraction(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req TriggerCharacterInteractionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
		return
	}

	// 验证参数
	if req.SceneID == "" {
		RespondWithError(w, http.StatusBadRequest, "缺少场景ID")
		return
	}
	if len(req.CharacterIDs) < 2 {
		RespondWithError(w, http.StatusBadRequest, "至少需要两个角色才能进行互动")
		return
	}
	if req.Topic == "" {
		RespondWithError(w, http.StatusBadRequest, "缺少互动主题")
		return
	}

	// 获取角色服务
	container := di.GetContainer()
	charServiceObj := container.Get("character")
	if charServiceObj == nil {
		RespondWithError(w, http.StatusInternalServerError, "角色服务不可用")
		return
	}
	characterService := charServiceObj.(*services.CharacterService)

	// 触发角色互动
	interaction, err := characterService.GenerateCharacterInteraction(
		req.SceneID,
		req.CharacterIDs,
		req.Topic,
		req.ContextDescription,
	)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("生成角色互动失败: %v", err))
		return
	}

	// 返回生成的互动内容
	RespondWithJSON(w, http.StatusOK, interaction)
}

// SimulateCharactersConversation 处理函数 - 模拟角色多轮对话
// @Summary 模拟多个角色之间的多轮对话
// @Description 基于给定初始情境，生成多个角色之间的多轮对话
// @Tags 角色互动
// @Accept json
// @Produce json
// @Param request body SimulateConversationRequest true "对话模拟请求参数"
// @Success 200 {array} models.InteractionDialogue "模拟对话结果"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/interactions/simulate [post]
func SimulateCharactersConversation(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req SimulateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
		return
	}

	// 验证参数
	if req.SceneID == "" {
		RespondWithError(w, http.StatusBadRequest, "缺少场景ID")
		return
	}
	if len(req.CharacterIDs) < 2 {
		RespondWithError(w, http.StatusBadRequest, "至少需要两个角色才能进行对话")
		return
	}
	if req.InitialSituation == "" {
		RespondWithError(w, http.StatusBadRequest, "缺少初始情境描述")
		return
	}
	if req.NumberOfTurns <= 0 {
		req.NumberOfTurns = 3 // 默认轮数
	}

	// 获取角色服务
	container := di.GetContainer()
	charServiceObj := container.Get("character")
	if charServiceObj == nil {
		RespondWithError(w, http.StatusInternalServerError, "角色服务不可用")
		return
	}
	characterService := charServiceObj.(*services.CharacterService)

	// 模拟角色对话
	dialogues, err := characterService.SimulateCharactersConversation(
		req.SceneID,
		req.CharacterIDs,
		req.InitialSituation,
		req.NumberOfTurns,
	)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("模拟角色对话失败: %v", err))
		return
	}

	// 返回生成的对话内容
	RespondWithJSON(w, http.StatusOK, dialogues)
}

// GetCharacterInteractions 处理函数 - 获取角色互动历史
// @Summary 获取场景中的角色互动历史
// @Description 获取指定场景中符合条件的角色互动历史记录
// @Tags 角色互动
// @Accept json
// @Produce json
// @Param scene_id path string true "场景ID"
// @Param limit query int false "返回结果数量限制" default(20)
// @Param interaction_id query string false "特定互动ID"
// @Param simulation_id query string false "特定模拟ID"
// @Success 200 {array} models.Conversation "互动记录列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/interactions/{scene_id} [get]
func GetCharacterInteractions(w http.ResponseWriter, r *http.Request) {
	// 获取URL参数
	params := r.URL.Query()
	sceneID := params.Get("scene_id")
	if sceneID == "" {
		RespondWithError(w, http.StatusBadRequest, "缺少场景ID")
		return
	}

	// 获取过滤参数
	filter := make(map[string]interface{})

	// 处理特定互动ID过滤
	if interactionID := params.Get("interaction_id"); interactionID != "" {
		filter["interaction_id"] = interactionID
	}

	// 处理特定模拟ID过滤
	if simulationID := params.Get("simulation_id"); simulationID != "" {
		filter["simulation_id"] = simulationID
	}

	// 处理其他可能的过滤条件...

	// 获取限制数量
	limit := 20 // 默认限制
	if limitStr := params.Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// 获取上下文服务
	container := di.GetContainer()
	ctxServiceObj := container.Get("context")
	if ctxServiceObj == nil {
		RespondWithError(w, http.StatusInternalServerError, "上下文服务不可用")
		return
	}
	contextService := ctxServiceObj.(*services.ContextService)

	// 获取角色互动历史
	interactions, err := contextService.GetCharacterInteractions(sceneID, filter, limit)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("获取角色互动历史失败: %v", err))
		return
	}

	// 返回互动历史
	RespondWithJSON(w, http.StatusOK, interactions)
}

// GetCharacterToCharacterInteractions 处理函数 - 获取特定两个角色之间的互动
// @Summary 获取特定两个角色之间的互动历史
// @Description 获取指定场景中两个特定角色之间的互动历史记录
// @Tags 角色互动
// @Accept json
// @Produce json
// @Param scene_id path string true "场景ID"
// @Param character1_id path string true "角色1 ID"
// @Param character2_id path string true "角色2 ID"
// @Param limit query int false "返回结果数量限制" default(20)
// @Success 200 {array} models.Conversation "互动记录列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/interactions/{scene_id}/{character1_id}/{character2_id} [get]
func GetCharacterToCharacterInteractions(w http.ResponseWriter, r *http.Request) {
	// 获取URL参数
	params := r.URL.Query()
	sceneID := params.Get("scene_id")
	character1ID := params.Get("character1_id")
	character2ID := params.Get("character2_id")

	// 验证必要的参数
	if sceneID == "" || character1ID == "" || character2ID == "" {
		RespondWithError(w, http.StatusBadRequest, "缺少必要参数: scene_id, character1_id, character2_id")
		return
	}

	// 获取限制数量
	limit := 20 // 默认限制
	if limitStr := params.Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// 获取上下文服务
	container := di.GetContainer()
	ctxServiceObj := container.Get("context")
	if ctxServiceObj == nil {
		RespondWithError(w, http.StatusInternalServerError, "上下文服务不可用")
		return
	}
	contextService := ctxServiceObj.(*services.ContextService)

	// 获取两个角色之间的互动
	interactions, err := contextService.GetCharacterToCharacterInteractions(sceneID, character1ID, character2ID, limit)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("获取角色互动历史失败: %v", err))
		return
	}

	// 返回互动历史
	RespondWithJSON(w, http.StatusOK, interactions)
}

// RespondWithError 发送错误响应
func RespondWithError(w http.ResponseWriter, code int, message string) {
	RespondWithJSON(w, code, map[string]string{"error": message})
}

// RespondWithJSON 发送JSON响应
func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// TriggerCharacterInteraction 处理函数 - 触发角色互动
func (h *Handler) TriggerCharacterInteraction(c *gin.Context) {
	// 解析请求体
	var req TriggerCharacterInteractionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式: " + err.Error()})
		return
	}

	// 验证参数
	if req.SceneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少场景ID"})
		return
	}
	if len(req.CharacterIDs) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少需要两个角色才能进行互动"})
		return
	}
	if req.Topic == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少互动主题"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("生成角色互动失败: %v", err)})
		return
	}

	// 返回生成的互动内容
	c.JSON(http.StatusOK, interaction)
}

// SimulateCharactersConversation 处理函数 - 模拟角色多轮对话
func (h *Handler) SimulateCharactersConversation(c *gin.Context) {
	// 解析请求体
	var req SimulateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式: " + err.Error()})
		return
	}

	// 验证参数
	if req.SceneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少场景ID"})
		return
	}
	if len(req.CharacterIDs) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少需要两个角色才能进行对话"})
		return
	}
	if req.InitialSituation == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少初始情境描述"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("模拟角色对话失败: %v", err)})
		return
	}

	// 返回生成的对话内容
	c.JSON(http.StatusOK, dialogues)
}

// GetCharacterInteractions 处理函数 - 获取角色互动历史
func (h *Handler) GetCharacterInteractions(c *gin.Context) {
	// 获取URL参数
	sceneID := c.Param("scene_id")
	if sceneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少场景ID"})
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

	// 获取角色互动历史
	interactions, err := h.ContextService.GetCharacterInteractions(sceneID, filter, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取角色互动历史失败: %v", err)})
		return
	}

	// 返回互动历史
	c.JSON(http.StatusOK, interactions)
}

// GetCharacterToCharacterInteractions 处理函数 - 获取特定两个角色之间的互动
func (h *Handler) GetCharacterToCharacterInteractions(c *gin.Context) {
	// 获取URL参数
	sceneID := c.Param("scene_id")
	character1ID := c.Param("character1_id")
	character2ID := c.Param("character2_id")

	// 验证必要的参数
	if sceneID == "" || character1ID == "" || character2ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要参数: scene_id, character1_id, character2_id"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取角色互动历史失败: %v", err)})
		return
	}

	// 返回互动历史
	c.JSON(http.StatusOK, interactions)
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

	// 尝试从容器获取
	if service, ok := container.Get("interaction_aggregate").(*services.InteractionAggregateService); ok {
		// 确保服务的所有字段都正确设置
		if service.StoryService == nil {
			service.StoryService = h.getStoryService()
		}
		return service
	}

	// 获取故事服务实例
	storyService := h.getStoryService()
	if storyService == nil {
		log.Printf("Warning: StoryService is nil, some features may not work properly")
	}

	// 创建新实例
	service := &services.InteractionAggregateService{
		CharacterService: h.CharacterService,
		ContextService:   h.ContextService,
		SceneService:     h.SceneService,
		StatsService:     h.StatsService,
		StoryService:     storyService,
	}

	// 注册到容器
	container.Register("interaction_aggregate", service)

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

// 标准化错误响应格式
func (h *Handler) respondWithError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"error":     message,
		"timestamp": time.Now().Format(time.RFC3339),
		"path":      c.Request.URL.Path,
	})
}
