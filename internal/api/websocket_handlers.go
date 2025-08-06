// internal/api/websocket_handlers.go
package api

import (
	"encoding/json"
	"log"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketHandler 处理 WebSocket 相关的 HTTP 请求
type WebSocketHandler struct {
	sceneService     *services.SceneService
	characterService *services.CharacterService
	storyService     *services.StoryService
	contextService   *services.ContextService
}

// NewWebSocketHandler 创建 WebSocket 处理器
func NewWebSocketHandler() *WebSocketHandler {
	container := di.GetContainer()

	return &WebSocketHandler{
		sceneService:     container.Get("scene").(*services.SceneService),
		characterService: container.Get("character").(*services.CharacterService),
		storyService:     container.Get("story").(*services.StoryService),
		contextService:   container.Get("context").(*services.ContextService),
	}
}

// SceneWebSocket 处理场景 WebSocket 连接
func (wh *WebSocketHandler) SceneWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ 场景 WebSocket 升级失败: %v", err)
		return
	}

	// 获取参数
	sceneID := c.Param("id")
	userID := c.DefaultQuery("user_id", "anonymous")

	// 创建客户端
	client := &WebSocketClient{
		conn:      &WebSocketConnWrapper{conn},
		sceneID:   sceneID,
		userID:    userID,
		send:      make(chan []byte, 256),
		closed:    0,
		lastPing:  time.Now(),
		createdAt: time.Now(),
	}

	// 注册客户端
	wsManager.register <- client
	defer func() {
		wsManager.unregister <- client
	}()

	// 启动读写协程
	go wh.handleWebSocketWrites(client)
	go wh.handleWebSocketReads(client)

	// 发送连接确认消息
	wh.sendWelcomeMessage(client, sceneID, userID)

	// 等待连接关闭
	<-c.Request.Context().Done()
	log.Printf("📱 场景 %s 的 WebSocket 连接已关闭 (用户: %s)", sceneID, userID)
}

// UserStatusWebSocket 处理用户状态 WebSocket 连接
func (wh *WebSocketHandler) UserStatusWebSocket(c *gin.Context) {
	// 升级 HTTP 连接到 WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ 用户状态 WebSocket 升级失败: %v", err)
		return
	}

	// 获取用户ID
	userID := c.DefaultQuery("user_id", "anonymous")

	// 创建客户端 - 修复：需要包装连接
	client := &WebSocketClient{
		conn:      &WebSocketConnWrapper{conn}, // 修复：包装连接
		sceneID:   "user_status",               // 特殊的场景ID
		userID:    userID,
		send:      make(chan []byte, 256),
		closed:    0,
		lastPing:  time.Now(),
		createdAt: time.Now(),
	}

	// 注册客户端
	wsManager.register <- client
	defer func() {
		wsManager.unregister <- client
	}()

	// 启动读写协程
	go wh.handleWebSocketWrites(client)
	go wh.handleWebSocketReads(client)

	// 发送连接确认消息
	wh.sendUserStatusWelcome(client, userID)

	// 定期发送心跳
	wh.startHeartbeat(c, client)
}

// handleWebSocketReads 处理 WebSocket 读取
func (wh *WebSocketHandler) handleWebSocketReads(client *WebSocketClient) {
	defer func() {
		if !client.IsClosed() {
			wsManager.unregister <- client
		}
	}()

	// 设置读取超时和ping处理
	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.UpdatePing()
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		if client.IsClosed() {
			break
		}

		// 修复：安全的类型断言
		_, messageBytes, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("❌ WebSocket 读取错误: %v", err)
			}
			break
		}

		// 解析JSON消息
		var message map[string]interface{}
		if err := json.Unmarshal(messageBytes, &message); err != nil {
			log.Printf("⚠️ JSON解析失败: %v", err)
			continue
		}

		// 更新活跃时间
		client.UpdatePing()

		// 处理收到的消息
		wh.handleMessage(client, message)
	}
}

// handleWebSocketWrites 处理 WebSocket 写入
func (wh *WebSocketHandler) handleWebSocketWrites(client *WebSocketClient) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		close(client.send)
		if !client.IsClosed() {
			client.Close()
		}
	}()

	for {
		select {
		case message, ok := <-client.send:
			if client.IsClosed() {
				return
			}

			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("❌ WebSocket 写入失败: %v", err)
				return
			}

		case <-ticker.C:
			if client.IsClosed() {
				return
			}

			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("❌ WebSocket ping 失败: %v", err)
				return
			}
			client.UpdatePing()
		}
	}
}

// handleMessage 处理收到的 WebSocket 消息
func (wh *WebSocketHandler) handleMessage(client *WebSocketClient, message map[string]interface{}) {
	msgType, ok := message["type"].(string)
	if !ok {
		log.Printf("⚠️ 收到无效的消息类型")
		return
	}

	switch msgType {
	case "character_interaction":
		wh.handleCharacterInteraction(client, message)
	case "story_choice":
		wh.handleStoryChoice(client, message)
	case "user_status_update":
		wh.handleUserStatusUpdate(client, message)
	case "ping":
		wh.handlePing(client)
	default:
		log.Printf("⚠️ 未知的消息类型: %s", msgType)
	}
}

// handleCharacterInteraction 处理角色交互消息
func (wh *WebSocketHandler) handleCharacterInteraction(client *WebSocketClient, message map[string]interface{}) {
	characterID, ok := message["character_id"].(string)
	if !ok {
		wh.sendError(client, "缺少角色ID")
		return
	}

	userMessage, ok := message["message"].(string)
	if !ok {
		wh.sendError(client, "缺少消息内容")
		return
	}

	// nil检查
	if wh.characterService == nil {
		wh.sendError(client, "角色服务不可用")
		return
	}

	// 生成角色回应
	response, err := wh.characterService.GenerateResponse(client.sceneID, characterID, userMessage)
	if err != nil {
		wh.sendError(client, "生成回应失败: "+err.Error())
		return
	}

	// 广播新对话
	conversationMsg := map[string]interface{}{
		"type":         "conversation:new",
		"scene_id":     client.sceneID,
		"character_id": characterID,
		"speaker_id":   characterID,
		"conversation": response,
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	wsManager.BroadcastToScene(client.sceneID, conversationMsg)
}

// handleStoryChoice 处理故事选择消息
func (wh *WebSocketHandler) handleStoryChoice(client *WebSocketClient, message map[string]interface{}) {
	nodeID, ok := message["node_id"].(string)
	if !ok {
		wh.sendError(client, "缺少节点ID")
		return
	}

	choiceID, ok := message["choice_id"].(string)
	if !ok {
		wh.sendError(client, "缺少选择ID")
		return
	}

	// nil检查
	if wh.storyService == nil {
		wh.sendError(client, "故事服务不可用")
		return
	}

	// 解析用户偏好
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

	// 添加用户信息到上下文
	if preferences == nil {
		preferences = &models.UserPreferences{}
	}

	// 执行故事选择
	nextNode, err := wh.storyService.MakeChoice(client.sceneID, nodeID, choiceID, preferences)
	if err != nil {
		wh.sendError(client, "执行故事选择失败: "+err.Error())
		return
	}

	// 发送确认消息给发起客户端
	confirmMsg := map[string]interface{}{
		"type": "story:choice_confirmed",
		"data": map[string]interface{}{
			"node_id":   nodeID,
			"choice_id": choiceID,
			"next_node": nextNode,
			"user_id":   client.userID,
		},
	}
	client.SendMessage(confirmMsg)
}

// handleUserStatusUpdate 处理用户状态更新消息
func (wh *WebSocketHandler) handleUserStatusUpdate(client *WebSocketClient, message map[string]interface{}) {
	status, ok := message["status"].(string)
	if !ok {
		wh.sendError(client, "缺少状态信息")
		return
	}

	// 广播用户状态更新
	statusUpdateMsg := map[string]interface{}{
		"type":      "user:presence",
		"user_id":   client.userID,
		"scene_id":  client.sceneID,
		"status":    status,
		"action":    message["action"],
		"timestamp": time.Now().Format(time.RFC3339),
	}

	wsManager.BroadcastToScene(client.sceneID, statusUpdateMsg)
}

// handlePing 处理ping消息
func (wh *WebSocketHandler) handlePing(client *WebSocketClient) {
	pong := map[string]interface{}{
		"type":      "pong",
		"timestamp": time.Now().Unix(),
	}

	client.SendMessage(pong)
}

// sendWelcomeMessage 发送欢迎消息
func (wh *WebSocketHandler) sendWelcomeMessage(client *WebSocketClient, sceneID, userID string) {
	welcomeMsg := map[string]interface{}{
		"type":      "connected",
		"scene_id":  sceneID,
		"user_id":   userID,
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "WebSocket 连接已建立",
	}

	client.SendMessage(welcomeMsg)
}

// sendUserStatusWelcome 发送用户状态欢迎消息
func (wh *WebSocketHandler) sendUserStatusWelcome(client *WebSocketClient, userID string) {
	welcomeMsg := map[string]interface{}{
		"type":      "user_status_connected",
		"user_id":   userID,
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "用户状态连接已建立",
	}

	client.SendMessage(welcomeMsg)
}

// startHeartbeat 启动心跳
func (wh *WebSocketHandler) startHeartbeat(c *gin.Context, client *WebSocketClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if client.IsClosed() {
				return
			}

			heartbeat := map[string]interface{}{
				"type":      "heartbeat",
				"timestamp": time.Now().Unix(),
			}

			client.SendMessage(heartbeat)

		case <-c.Request.Context().Done():
			return
		}
	}
}

// sendError 发送错误消息
func (wh *WebSocketHandler) sendError(client *WebSocketClient, errorMsg string) {
	errorResponse := map[string]interface{}{
		"type":      "error",
		"error":     errorMsg,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if msgBytes, err := json.Marshal(errorResponse); err == nil {
		select {
		case client.send <- msgBytes:
		default:
			log.Printf("⚠️ 无法发送错误消息到客户端，队列已满")
		}
	}
}
