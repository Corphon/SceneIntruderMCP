// internal/api/websocket.go
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

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

// WebSocketClient 表示一个 WebSocket 客户端连接
type WebSocketClient struct {
	conn      WebSocketConnection
	sceneID   string
	userID    string
	send      chan []byte
	closed    int32     // 原子操作标志，0=开启，1=关闭
	lastPing  time.Time // 最后一次ping时间
	createdAt time.Time // 创建时间
}

// WebSocketManager 管理所有 WebSocket 连接
type WebSocketManager struct {
	connections   map[string]map[WebSocketConnection]*WebSocketClient // sceneID -> connections
	broadcast     chan []byte
	register      chan *WebSocketClient
	unregister    chan *WebSocketClient
	cleanup       chan bool
	mutex         sync.RWMutex
	pingTimeout   time.Duration
	cleanupTicker *time.Ticker
}

// 全局 WebSocket 管理器
var wsManager = &WebSocketManager{
	connections: make(map[string]map[WebSocketConnection]*WebSocketClient),
	broadcast:   make(chan []byte, 256),
	register:    make(chan *WebSocketClient, 256),
	unregister:  make(chan *WebSocketClient, 256),
	cleanup:     make(chan bool, 1),
	pingTimeout: 60 * time.Second,
}

// WebSocketConnection 定义 WebSocket 连接的接口
type WebSocketConnection interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	SetPongHandler(h func(appData string) error)
}

// WebSocketConnWrapper 包装真实的 websocket.Conn 以实现接口
type WebSocketConnWrapper struct {
	*websocket.Conn
}

// -----------------------------------------
// 初始化 WebSocket 管理器
func init() {
	go wsManager.run()
}

// ========================================
// WebSocketClient 方法
// ========================================

// Close 安全关闭客户端连接
func (client *WebSocketClient) Close() {
	if atomic.CompareAndSwapInt32(&client.closed, 0, 1) {
		// 只设置关闭标志，不关闭通道
		// 通道由 handleWebSocketWrites 的 defer 函数负责关闭
		if client.conn != nil {
			client.conn.Close()
		}
	}
}

// IsClosed 检查连接是否已关闭
func (client *WebSocketClient) IsClosed() bool {
	return atomic.LoadInt32(&client.closed) == 1
}

// UpdatePing 更新最后ping时间
func (client *WebSocketClient) UpdatePing() {
	client.lastPing = time.Now()
}

// IsExpired 检查连接是否超时
func (client *WebSocketClient) IsExpired(timeout time.Duration) bool {
	if timeout <= 0 {
		return true // 零超时时间立即过期
	}

	return time.Since(client.lastPing) > timeout
}

// SendMessage 安全发送消息到客户端
func (client *WebSocketClient) SendMessage(message map[string]interface{}) error {
	if client.IsClosed() {
		return nil // 客户端已关闭，直接返回
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// 双重检查，避免竞态条件
	if client.IsClosed() {
		return nil
	}

	select {
	case client.send <- msgBytes:
		return nil
	default:
		// 队列满，记录警告但不阻塞
		log.Printf("⚠️ 客户端 %s 消息队列已满，消息被丢弃", client.userID)
		return nil
	}
}

// SendError 发送错误消息到客户端
func (client *WebSocketClient) SendError(errorMsg string) {
	errorResponse := map[string]interface{}{
		"type":      "error",
		"error":     errorMsg,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	client.SendMessage(errorResponse)
}

// ========================================
// WebSocketManager 方法
// ========================================

// run 运行 WebSocket 管理器主循环
func (manager *WebSocketManager) run() {
	// 启动定期清理
	manager.cleanupTicker = time.NewTicker(30 * time.Second)
	defer manager.cleanupTicker.Stop()

	for {
		select {
		case client := <-manager.register:
			manager.registerClient(client)

		case client := <-manager.unregister:
			manager.unregisterClient(client)

		case <-manager.cleanupTicker.C:
			manager.cleanupExpiredConnections()

		case message := <-manager.broadcast:
			manager.broadcastMessage(message)

		case <-manager.cleanup:
			manager.shutdown()
			return
		}
	}
}

// registerClient 注册新客户端
func (manager *WebSocketManager) registerClient(client *WebSocketClient) {
	if client == nil {
		log.Printf("⚠️ 尝试注册 nil 客户端，忽略")
		return
	}

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	if manager.connections[client.sceneID] == nil {
		manager.connections[client.sceneID] = make(map[WebSocketConnection]*WebSocketClient)
	}

	manager.connections[client.sceneID][client.conn] = client
	client.UpdatePing() // 初始化ping时间

	log.Printf("✅ WebSocket 客户端已连接到场景 %s (用户: %s)", client.sceneID, client.userID)
}

// unregisterClient 安全注销客户端
func (manager *WebSocketManager) unregisterClient(client *WebSocketClient) {
	if client == nil {
		log.Printf("⚠️ 尝试注销 nil 客户端，忽略")
		return
	}

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	// 查找并移除客户端
	if connections, exists := manager.connections[client.sceneID]; exists {
		delete(connections, client.conn)

		// 如果场景没有连接了，清理场景
		if len(connections) == 0 {
			delete(manager.connections, client.sceneID)
		}
	}

	// 关闭客户端连接
	if !client.IsClosed() {
		client.Close()
	}

	log.Printf("🔌 WebSocket 客户端已断开连接 (场景: %s, 用户: %s)", client.sceneID, client.userID)
}

// cleanupExpiredConnections 清理过期和死连接
func (manager *WebSocketManager) cleanupExpiredConnections() {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	for sceneID, connections := range manager.connections {
		for conn, client := range connections {
			if client.IsClosed() || client.IsExpired(manager.pingTimeout) {
				delete(connections, conn)
				if !client.IsClosed() {
					client.Close()
				}
			}
		}
		if len(connections) == 0 {
			delete(manager.connections, sceneID)
		}
	}
}

// broadcastMessage 广播消息
func (manager *WebSocketManager) broadcastMessage(message []byte) {
	manager.mutex.RLock()
	allClients := make([]*WebSocketClient, 0)
	for _, connections := range manager.connections {
		for _, client := range connections {
			if !client.IsClosed() {
				allClients = append(allClients, client)
			}
		}
	}
	manager.mutex.RUnlock()

	if len(allClients) > 0 {
		manager.processBatch(allClients, message)
	}
}

// processBatch 处理批量消息发送
func (manager *WebSocketManager) processBatch(clients []*WebSocketClient, message []byte) {
	failedCount := 0
	for _, client := range clients {
		if client.IsClosed() {
			continue
		}

		select {
		case client.send <- message:
			// 消息发送成功
		default:
			// 队列满，限制失败处理数量
			failedCount++
			if failedCount <= 5 { // 每批次最多处理5个失败连接
				go func(c *WebSocketClient) {
					c.Close()
					select {
					case manager.unregister <- c:
					case <-time.After(50 * time.Millisecond):
						// 超时放弃
					}
				}(client)
			} else {
				// 直接关闭，不进入unregister队列
				client.Close()
			}
		}
	}
}

// shutdown 优雅关闭管理器
func (manager *WebSocketManager) shutdown() {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	log.Println("🛑 正在关闭 WebSocket 管理器...")

	// 关闭所有连接
	for _, connections := range manager.connections {
		for _, client := range connections {
			client.Close()
		}
	}

	// 清空连接映射
	manager.connections = make(map[string]map[WebSocketConnection]*WebSocketClient)

	log.Println("✅ WebSocket 管理器已关闭")
}

// GetStatus 获取管理器状态
func (manager *WebSocketManager) GetStatus() map[string]interface{} {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	// 初始化scenes为非nil的map
	scenes := make(map[string]interface{})
	totalConnections := 0

	for sceneID, connections := range manager.connections {
		activeConnections := 0
		users := make([]interface{}, 0)

		for _, client := range connections {
			if client != nil && !client.IsClosed() {
				activeConnections++
				userInfo := map[string]interface{}{
					"user_id":      client.userID,
					"scene_id":     client.sceneID,
					"connected_at": client.createdAt.Format(time.RFC3339),
					"last_ping":    client.lastPing.Format(time.RFC3339),
				}
				users = append(users, userInfo)
			}
		}

		scenes[sceneID] = map[string]interface{}{
			"client_count": activeConnections,
			"users":        users,
		}
		totalConnections += activeConnections
	}

	return map[string]interface{}{
		"total_scenes":      len(manager.connections),
		"total_connections": totalConnections,
		"scenes":            scenes,
	}
}

// BroadcastToScene 向指定场景广播消息
func (manager *WebSocketManager) BroadcastToScene(sceneID string, message map[string]interface{}) {
	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("❌ 序列化广播消息失败: %v", err)
		return
	}

	manager.mutex.RLock()
	connections, exists := manager.connections[sceneID]
	if !exists {
		manager.mutex.RUnlock()
		return
	}

	clientConnections := make([]*WebSocketClient, 0, len(connections))
	for _, client := range connections {
		if !client.IsClosed() {
			clientConnections = append(clientConnections, client)
		}
	}
	manager.mutex.RUnlock()

	if len(clientConnections) > 0 {
		manager.processBatch(clientConnections, msgBytes)
	}
}

// ReadJSON 读取JSON消息 - 为了兼容测试和handlers
func (w *WebSocketConnWrapper) ReadJSON(v interface{}) error {
	return w.Conn.ReadJSON(v)
}

// WriteJSON 写入JSON消息 - 为了兼容测试和handlers
func (w *WebSocketConnWrapper) WriteJSON(v interface{}) error {
	return w.Conn.WriteJSON(v)
}
