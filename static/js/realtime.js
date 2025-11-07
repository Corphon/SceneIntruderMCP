/**
 * 实时通信管理器
 * 基于现有 Utils.js WebSocket 功能增强
 * 支持角色状态、故事事件、用户互动的实时更新
 */
class RealtimeManager {
    constructor() {
        this.connections = new Map(); // 多个连接管理
        this.eventHandlers = new Map(); // 事件处理器
        this.reconnectAttempts = new Map(); // 重连尝试计数
        this.heartbeatTimers = new Map(); // 心跳定时器
        this.subscriptions = new Map(); // 订阅管理
        this.connectionErrors = new Map(); // 连接错误记录
        this.onlineUsers = new Map(); // 在线用户列表

        // 配置选项
        this.options = {
            reconnectInterval: 3000,  // 重连间隔
            maxReconnectAttempts: 5,  // 最大重连次数
            heartbeatInterval: 30000, // 心跳间隔
            messageQueueSize: 100,    // 消息队列大小
            debug: window.location.hostname === 'localhost'
        };

        // 消息队列
        this.messageQueue = [];
        this.isOnline = navigator.onLine;

        this.init();
    }

    /**
     * 初始化实时通信管理器
     */
    init() {
        this.bindGlobalEvents();
        this.initNetworkMonitoring();
        console.log('🔗 RealtimeManager 已初始化');
    }

    /**
     * 绑定全局事件
     */
    bindGlobalEvents() {
        // 页面可见性变化
        document.addEventListener('visibilitychange', () => {
            if (document.hidden) {
                this.pauseHeartbeats();
            } else {
                this.resumeHeartbeats();
                this.checkAllConnections();
            }
        });

        // 页面卸载前清理
        window.addEventListener('beforeunload', () => {
            this.cleanup();
        });
    }

    /**
     * 初始化网络监控
     */
    initNetworkMonitoring() {
        window.addEventListener('online', () => {
            this.isOnline = true;
            this.handleNetworkRestore();
        });

        window.addEventListener('offline', () => {
            this.isOnline = false;
            this.handleNetworkLoss();
        });
    }

    // ========================================
    // 连接管理
    // ========================================

    /**
     * 创建场景实时连接
     */
    async connectToScene(sceneId) {
        const connectionId = `scene_${sceneId}`;

        if (this.connections.has(connectionId)) {
            console.log(`🔗 场景 ${sceneId} 已连接`);
            return this.connections.get(connectionId);
        }

        try {
            const wsUrl = this.buildWebSocketUrl('/ws/scene', { sceneId });
            const ws = await this.createConnection(connectionId, wsUrl, {
                onopen: () => this.handleSceneConnect(sceneId),
                onmessage: (event) => this.handleSceneMessage(sceneId, event),
                onclose: () => this.handleSceneDisconnect(sceneId),
                onerror: (error) => this.handleSceneError(sceneId, error)
            });

            // 订阅场景事件
            this.subscribeToSceneEvents(sceneId);

            return ws;
        } catch (error) {
            console.error(`❌ 场景连接失败: ${error.message}`);
            Utils.showError(`场景连接失败: ${error.message}`);
            throw error;
        }
    }

    // 在 realtime.js 的场景实时功能部分添加这些方法

    /**
     * 处理场景断开连接
     */
    handleSceneDisconnect(sceneId) {
        console.log(`🚫 场景 ${sceneId} 连接已断开`);

        // 更新连接状态
        this.updateConnectionStatus(sceneId, 'disconnected');

        // 触发断开事件
        this.emit('scene:disconnected', { sceneId, timestamp: Date.now() });

        // 显示断开通知
        if (typeof Utils !== 'undefined' && Utils.showWarning) {
            Utils.showWarning('场景连接已断开，正在尝试重连...');
        }

        // 更新UI状态
        this.updateSceneConnectionUI(sceneId, false);

        // 尝试重连
        const connectionId = `scene_${sceneId}`;
        if (this.isOnline && this.connections.has(connectionId)) {
            this.attemptReconnect(connectionId);
        }
    }

    /**
     * 处理场景连接错误
     */
    handleSceneError(sceneId, error) {
        console.error(`❌ 场景 ${sceneId} 连接错误:`, error);

        // 记录错误信息
        this.recordConnectionError(sceneId, error);

        // 触发错误事件
        this.emit('scene:error', {
            sceneId,
            error: error.message || error,
            timestamp: Date.now()
        });

        // 根据错误类型显示不同的提示
        const errorMessage = this.getErrorMessage(error);
        if (typeof Utils !== 'undefined' && Utils.showError) {
            Utils.showError(`场景连接错误: ${errorMessage}`);
        }

        // 更新UI错误状态
        this.updateSceneErrorUI(sceneId, error);

        // 如果不是网络错误，延迟重连
        if (!this.isNetworkError(error)) {
            const connectionId = `scene_${sceneId}`;
            setTimeout(() => {
                this.attemptReconnect(connectionId);
            }, 5000);
        }
    }

    /**
     * 订阅场景事件
     */
    subscribeToSceneEvents(sceneId) {
        console.log(`📺 订阅场景 ${sceneId} 的事件`);

        // 发送订阅消息到服务器
        const connectionId = `scene_${sceneId}`;
        this.sendMessage(connectionId, {
            type: 'subscribe_events',
            events: [
                'character_status_update',
                'new_conversation',
                'story_event',
                'scene_state_update',
                'user_presence'
            ],
            sceneId: sceneId
        });

        // 记录订阅状态
        if (!this.subscriptions.has(sceneId)) {
            this.subscriptions.set(sceneId, new Set());
        }

        const sceneSubscriptions = this.subscriptions.get(sceneId);
        sceneSubscriptions.add('character_status_update');
        sceneSubscriptions.add('new_conversation');
        sceneSubscriptions.add('story_event');
        sceneSubscriptions.add('scene_state_update');
        sceneSubscriptions.add('user_presence');

        console.log(`✅ 场景 ${sceneId} 事件订阅完成`);
    }

    /**
     * 取消订阅场景事件
     */
    unsubscribeFromSceneEvents(sceneId) {
        console.log(`📺 取消订阅场景 ${sceneId} 的事件`);

        // 发送取消订阅消息
        const connectionId = `scene_${sceneId}`;
        this.sendMessage(connectionId, {
            type: 'unsubscribe_events',
            sceneId: sceneId
        });

        // 清理订阅状态
        this.subscriptions.delete(sceneId);

        console.log(`✅ 场景 ${sceneId} 事件订阅已取消`);
    }

    /**
     * 更新连接状态
     */
    updateConnectionStatus(sceneId, status) {
        const statusMap = {
            'connecting': '连接中',
            'connected': '已连接',
            'disconnected': '已断开',
            'error': '连接错误'
        };

        const statusText = statusMap[status] || status;
        console.log(`🔗 场景 ${sceneId} 状态: ${statusText}`);

        // 触发状态变化事件
        this.emit('connection:status_changed', {
            sceneId,
            status,
            statusText,
            timestamp: Date.now()
        });
    }

    /**
     * 更新场景连接UI状态
     */
    updateSceneConnectionUI(sceneId, isConnected) {
        // 查找连接状态指示器
        let statusIndicator = document.querySelector('.scene-connection-status');

        if (!statusIndicator) {
            // 创建连接状态指示器
            statusIndicator = this.createConnectionStatusIndicator();
        }

        // 更新状态显示
        const statusClass = isConnected ? 'connected' : 'disconnected';
        const statusText = isConnected ? '实时连接已建立' : '连接已断开';
        const statusIcon = isConnected ? '🟢' : '🔴';

        statusIndicator.className = `scene-connection-status ${statusClass}`;
        statusIndicator.innerHTML = `
        <span class="status-icon">${statusIcon}</span>
        <span class="status-text">${statusText}</span>
        ${!isConnected ? '<span class="reconnect-text">正在重连...</span>' : ''}
    `;

        // 如果连接断开，添加脉动效果
        if (!isConnected) {
            statusIndicator.classList.add('pulse');
        } else {
            statusIndicator.classList.remove('pulse');
        }
    }

    /**
     * 更新场景错误UI状态
     */
    updateSceneErrorUI(sceneId, error) {
        // 查找或创建错误显示区域
        let errorContainer = document.querySelector('.scene-error-container');

        if (!errorContainer) {
            errorContainer = document.createElement('div');
            errorContainer.className = 'scene-error-container';
            document.body.appendChild(errorContainer);
        }

        // 创建错误通知
        const errorNotification = document.createElement('div');
        errorNotification.className = 'scene-error-notification';
        errorNotification.innerHTML = `
        <div class="error-header">
            <span class="error-icon">⚠️</span>
            <span class="error-title">场景连接错误</span>
            <button class="error-close" onclick="this.parentNode.parentNode.remove()">×</button>
        </div>
        <div class="error-body">
            <p class="error-message">${this.getErrorMessage(error)}</p>
            <small class="error-time">${new Date().toLocaleTimeString()}</small>
        </div>
        <div class="error-actions">
            <button class="btn-retry" onclick="window.realtimeManager?.reconnectToScene('${sceneId}')">
                重新连接
            </button>
        </div>
    `;

        errorContainer.appendChild(errorNotification);

        // 自动移除旧的错误通知（保留最近3个）
        const notifications = errorContainer.querySelectorAll('.scene-error-notification');
        if (notifications.length > 3) {
            notifications[0].remove();
        }

        // 5秒后自动隐藏
        setTimeout(() => {
            if (errorNotification.parentNode) {
                errorNotification.style.opacity = '0.5';
            }
        }, 5000);
    }

    /**
     * 创建连接状态指示器
     */
    createConnectionStatusIndicator() {
        const indicator = document.createElement('div');
        indicator.className = 'scene-connection-status';
        indicator.style.cssText = `
        position: fixed;
        top: 10px;
        right: 10px;
        background: rgba(0, 0, 0, 0.8);
        color: white;
        padding: 8px 12px;
        border-radius: 20px;
        font-size: 12px;
        z-index: 1000;
        display: flex;
        align-items: center;
        gap: 6px;
        transition: all 0.3s ease;
    `;

        // 添加到页面
        document.body.appendChild(indicator);

        // 添加点击事件显示详细信息
        indicator.addEventListener('click', () => {
            this.showConnectionDetails();
        });

        return indicator;
    }

    /**
     * 显示连接详细信息
     */
    showConnectionDetails() {
        const status = this.getConnectionStatus();
        const details = Object.entries(status).map(([id, info]) => {
            return `${id}: ${info.readyStateText} (重连次数: ${info.reconnectAttempts})`;
        }).join('\n');

        if (typeof Utils !== 'undefined' && Utils.showInfo) {
            Utils.showInfo(`连接状态详情:\n${details}`);
        } else {
            alert(`连接状态详情:\n${details}`);
        }
    }

    /**
     * 记录连接错误
     */
    recordConnectionError(sceneId, error) {
        // 初始化错误记录
        if (!this.connectionErrors) {
            this.connectionErrors = new Map();
        }

        if (!this.connectionErrors.has(sceneId)) {
            this.connectionErrors.set(sceneId, []);
        }

        // 记录错误
        const errorRecord = {
            timestamp: Date.now(),
            error: error.message || error.toString(),
            type: this.getErrorType(error)
        };

        const sceneErrors = this.connectionErrors.get(sceneId);
        sceneErrors.push(errorRecord);

        // 只保留最近10个错误
        if (sceneErrors.length > 10) {
            sceneErrors.shift();
        }

        this.debugLog(`记录场景 ${sceneId} 错误:`, errorRecord);
    }

    /**
     * 获取用户友好的错误消息
     */
    getErrorMessage(error) {
        if (!error) return '未知错误';

        const errorString = error.message || error.toString();

        // 网络相关错误
        if (errorString.includes('NetworkError') || errorString.includes('network')) {
            return '网络连接问题，请检查网络状态';
        }

        // WebSocket相关错误
        if (errorString.includes('WebSocket')) {
            return 'WebSocket连接失败，请刷新页面重试';
        }

        // 服务器错误
        if (errorString.includes('500') || errorString.includes('Internal Server Error')) {
            return '服务器内部错误，请稍后重试';
        }

        // 权限错误
        if (errorString.includes('403') || errorString.includes('Forbidden')) {
            return '访问权限不足';
        }

        // 超时错误
        if (errorString.includes('timeout')) {
            return '连接超时，请检查网络状态';
        }

        // 默认返回原始错误信息（截取前100个字符）
        return errorString.length > 100 ?
            errorString.substring(0, 100) + '...' :
            errorString;
    }

    /**
     * 获取错误类型
     */
    getErrorType(error) {
        const errorString = (error.message || error.toString()).toLowerCase();

        if (errorString.includes('network')) return 'network';
        if (errorString.includes('websocket')) return 'websocket';
        if (errorString.includes('timeout')) return 'timeout';
        if (errorString.includes('403') || errorString.includes('forbidden')) return 'permission';
        if (errorString.includes('500')) return 'server';

        return 'unknown';
    }

    /**
     * 判断是否为网络错误
     */
    isNetworkError(error) {
        const networkErrors = ['NetworkError', 'network', 'offline', 'timeout'];
        const errorString = (error.message || error.toString()).toLowerCase();

        return networkErrors.some(keyword => errorString.includes(keyword));
    }

    /**
     * 重新连接到场景（公共方法）
     */
    async reconnectToScene(sceneId) {
        try {
            console.log(`🔄 手动重连场景 ${sceneId}`);

            // 先断开现有连接
            const connectionId = `scene_${sceneId}`;
            if (this.connections.has(connectionId)) {
                const connection = this.connections.get(connectionId);
                connection.close();
                this.connections.delete(connectionId);
            }

            // 重置重连计数
            this.reconnectAttempts.set(connectionId, 0);

            // 重新连接
            await this.connectToScene(sceneId);

            if (typeof Utils !== 'undefined' && Utils.showSuccess) {
                Utils.showSuccess('场景重连成功');
            }

        } catch (error) {
            console.error(`场景重连失败:`, error);
            if (typeof Utils !== 'undefined' && Utils.showError) {
                Utils.showError('场景重连失败: ' + error.message);
            }
        }
    }

    /**
     * 处理用户状态连接
     */
    handleUserStatusConnect() {
        console.log('👤 用户状态连接已建立');

        // 触发用户状态连接事件
        this.emit('user_status:connected', { timestamp: Date.now() });

        // 发送初始状态请求
        this.sendMessage('user_status', {
            type: 'request_status',
            userId: this.getCurrentUserId()
        });
    }

    /**
     * 处理用户状态消息
     */
    handleUserStatusMessage(event) {
        try {
            const data = JSON.parse(event.data);
            this.debugLog('收到用户状态消息:', data);

            switch (data.type) {
                case 'status_update':
                    this.handleUserStatusUpdate(data);
                    break;

                case 'user_list_update':
                    this.handleUserListUpdate(data);
                    break;

                case 'heartbeat_response':
                    this.handleHeartbeatResponse('user_status', data);
                    break;

                default:
                    console.warn(`未知用户状态消息类型: ${data.type}`);
            }

        } catch (error) {
            console.error('解析用户状态消息失败:', error);
        }
    }

    /**
     * 处理用户状态断开
     */
    handleUserStatusDisconnect() {
        console.log('👤 用户状态连接已断开');

        // 触发用户状态断开事件
        this.emit('user_status:disconnected', { timestamp: Date.now() });

        // 尝试重连
        if (this.isOnline) {
            this.attemptReconnect('user_status');
        }
    }

    /**
     * 处理用户状态错误
     */
    handleUserStatusError(error) {
        console.error('👤 用户状态连接错误:', error);

        // 触发用户状态错误事件
        this.emit('user_status:error', {
            error: error.message || error,
            timestamp: Date.now()
        });
    }

    /**
     * 处理用户状态更新
     */
    handleUserStatusUpdate(data) {
        const { userId, status, lastSeen } = data;

        // 更新在线用户状态
        if (this.onlineUsers.has(userId)) {
            const userInfo = this.onlineUsers.get(userId);
            userInfo.status = status;
            userInfo.lastSeen = lastSeen;
        }

        // 触发用户状态更新事件
        this.emit('user:status_updated', {
            userId,
            status,
            lastSeen,
            timestamp: data.timestamp
        });
    }

    /**
     * 处理用户列表更新
     */
    handleUserListUpdate(data) {
        const { users } = data;

        // 更新在线用户列表
        this.onlineUsers.clear();
        users.forEach(user => {
            this.onlineUsers.set(user.userId, {
                username: user.username,
                status: user.status,
                joinTime: user.joinTime,
                lastSeen: user.lastSeen
            });
        });

        // 触发用户列表更新事件
        this.emit('user:list_updated', {
            users,
            timestamp: data.timestamp
        });
    }

    /**
     * 获取连接错误历史
     */
    getConnectionErrors(sceneId = null) {
        if (!this.connectionErrors) return [];

        if (sceneId) {
            return this.connectionErrors.get(sceneId) || [];
        }

        // 返回所有错误
        const allErrors = [];
        this.connectionErrors.forEach((errors, sceneId) => {
            errors.forEach(error => {
                allErrors.push({ ...error, sceneId });
            });
        });

        return allErrors.sort((a, b) => b.timestamp - a.timestamp);
    }

    /**
     * 清除连接错误历史
     */
    clearConnectionErrors(sceneId = null) {
        if (!this.connectionErrors) return;

        if (sceneId) {
            this.connectionErrors.delete(sceneId);
        } else {
            this.connectionErrors.clear();
        }

        console.log(`🧹 连接错误历史已清除 ${sceneId ? `(场景: ${sceneId})` : '(全部)'}`);
    }

    /**
     * 添加连接状态监控
     */
    startConnectionMonitoring() {
        // 每30秒检查一次连接状态
        this.connectionMonitor = setInterval(() => {
            this.checkAllConnections();
        }, 30000);

        console.log('📊 连接状态监控已启动');
    }

    /**
     * 停止连接状态监控
     */
    stopConnectionMonitoring() {
        if (this.connectionMonitor) {
            clearInterval(this.connectionMonitor);
            this.connectionMonitor = null;
            console.log('📊 连接状态监控已停止');
        }
    }

    /**
     * 创建用户状态连接
     */
    async connectToUserStatus() {
        const connectionId = 'user_status';

        if (this.connections.has(connectionId)) {
            return this.connections.get(connectionId);
        }

        try {
            const wsUrl = this.buildWebSocketUrl('/ws/user/status');
            const ws = await this.createConnection(connectionId, wsUrl, {
                onopen: () => this.handleUserStatusConnect(),
                onmessage: (event) => this.handleUserStatusMessage(event),
                onclose: () => this.handleUserStatusDisconnect(),
                onerror: (error) => this.handleUserStatusError(error)
            });

            return ws;
        } catch (error) {
            console.error(`❌ 用户状态连接失败: ${error.message}`);
            throw error;
        }
    }

    /**
     * 创建通用连接
     */
    async createConnection(connectionId, url, handlers = {}) {
        try {
            // 使用 Utils.js 的 WebSocket 创建功能
            const ws = Utils.createWebSocketConnection(url, handlers);

            this.connections.set(connectionId, ws);
            this.reconnectAttempts.set(connectionId, 0);

            // 启动心跳
            this.startHeartbeat(connectionId);

            console.log(`✅ 连接 ${connectionId} 已建立`);
            return ws;

        } catch (error) {
            console.error(`❌ 创建连接失败 ${connectionId}:`, error);
            throw error;
        }
    }

    /**
     * 构建 WebSocket URL
     */
    buildWebSocketUrl(path, params = {}) {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.host;
        let url = `${protocol}//${host}${path}`;

        if (Object.keys(params).length > 0) {
            const queryString = new URLSearchParams(params).toString();
            url += `?${queryString}`;
        }

        return url;
    }

    // ========================================
    // 场景实时功能
    // ========================================

    /**
     * 处理场景连接
     */
    handleSceneConnect(sceneId) {
        console.log(`🎭 场景 ${sceneId} 连接成功`);

        // 触发连接事件
        this.emit('scene:connected', { sceneId });

        // 请求初始状态
        this.sendMessage(`scene_${sceneId}`, {
            type: 'request_initial_state',
            timestamp: Date.now()
        });
    }

    /**
     * 处理场景消息
     */
    handleSceneMessage(sceneId, event) {
        try {
            const data = JSON.parse(event.data);
            this.debugLog('收到场景消息:', data);

            switch (data.type) {
                case 'character_status_update':
                    this.handleCharacterStatusUpdate(sceneId, data);
                    break;

                case 'new_conversation':
                    this.handleNewConversation(sceneId, data);
                    break;

                case 'story_event':
                    this.handleStoryEvent(sceneId, data);
                    break;

                case 'scene_state_update':
                    this.handleSceneStateUpdate(sceneId, data);
                    break;

                case 'user_joined':
                case 'user_left':
                    this.handleUserPresence(sceneId, data);
                    break;

                case 'heartbeat_response':
                    this.handleHeartbeatResponse(sceneId, data);
                    break;

                default:
                    console.warn(`未知消息类型: ${data.type}`);
            }

        } catch (error) {
            console.error('解析场景消息失败:', error);
        }
    }

    /**
     * 处理角色状态更新
     */
    handleCharacterStatusUpdate(sceneId, data) {
        const { characterId, status, mood, activity } = data;

        // 更新UI中的角色状态
        this.updateCharacterUI(characterId, { status, mood, activity });

        // 触发事件
        this.emit('character:status_updated', {
            sceneId,
            characterId,
            status,
            mood,
            activity,
            timestamp: data.timestamp
        });

        // 显示状态变化通知
        if (status === 'busy') {
            Utils.showInfo(`${this.getCharacterName(characterId)} 正在忙碌中...`);
        } else if (status === 'available') {
            Utils.showSuccess(`${this.getCharacterName(characterId)} 现在可以对话了`);
        }
    }

    /**
     * 处理新对话
     */
    handleNewConversation(sceneId, data) {
        const { conversation, speakerId, message } = data;

        // 更新对话列表
        if (window.SceneApp && window.SceneApp.addConversationToUI) {
            window.SceneApp.addConversationToUI(conversation);
        }

        // 触发事件
        this.emit('conversation:new', {
            sceneId,
            conversation,
            speakerId,
            message,
            timestamp: data.timestamp
        });

        // 播放提示音（如果不是当前用户发送的）
        if (speakerId !== this.getCurrentUserId()) {
            this.playNotificationSound();
        }
    }

    /**
     * 处理故事事件
     */
    handleStoryEvent(sceneId, data) {
        const { eventType, eventData, description } = data;

        this.debugLog('故事事件:', eventType, eventData);

        // 触发事件
        this.emit('story:event', {
            sceneId,
            eventType,
            eventData,
            description,
            timestamp: data.timestamp
        });

        // 显示故事事件通知
        if (description) {
            Utils.showInfo(description, 4000);
        }

        // 更新故事进度UI
        if (window.StoryManager && window.StoryManager.updateProgress) {
            window.StoryManager.updateProgress(eventData);
        }
    }

    /**
     * 处理场景状态更新
     */
    handleSceneStateUpdate(sceneId, data) {
        const { state, changes } = data;

        // 更新场景状态
        if (window.SceneApp && window.SceneApp.updateSceneState) {
            window.SceneApp.updateSceneState(state);
        }

        // 触发事件
        this.emit('scene:state_updated', {
            sceneId,
            state,
            changes,
            timestamp: data.timestamp
        });
    }

    /**
     * 处理用户在线状态
     */
    handleUserPresence(sceneId, data) {
        const { userId, username, action } = data;

        if (action === 'joined') {
            Utils.showInfo(`${username} 加入了场景`);
        } else if (action === 'left') {
            Utils.showInfo(`${username} 离开了场景`);
        }

        // 更新在线用户列表
        this.updateOnlineUsersList(sceneId, data);

        // 触发事件
        this.emit('user:presence', {
            sceneId,
            userId,
            username,
            action,
            timestamp: data.timestamp
        });
    }

    /**
     * 更新在线用户列表
     */
    updateOnlineUsersList(sceneId, data) {
        const { userId, username, action } = data;

        console.log(`👥 更新在线用户列表: ${username} ${action}`);

        // 更新内部在线用户数据
        if (action === 'joined') {
            this.onlineUsers.set(userId, {
                username: username,
                joinTime: Date.now(),
                status: 'online',
                sceneId: sceneId
            });
        } else if (action === 'left') {
            this.onlineUsers.delete(userId);
        }

        // 更新UI中的在线用户列表
        this.updateOnlineUsersUI(sceneId);

        // 显示用户状态通知
        if (action === 'joined') {
            this.showRealtimeNotification(`${username} 加入了场景`, 'info');
        } else if (action === 'left') {
            this.showRealtimeNotification(`${username} 离开了场景`, 'info');
        }

        // 触发在线用户列表更新事件
        this.emit('online_users:updated', {
            sceneId,
            userId,
            username,
            action,
            totalUsers: this.onlineUsers.size,
            timestamp: Date.now()
        });
    }

    /**
     * 更新在线用户列表UI
     */
    updateOnlineUsersUI(sceneId) {
        // 查找或创建在线用户列表容器
        let onlineUsersList = document.getElementById('online-users-list');

        if (!onlineUsersList) {
            onlineUsersList = this.createOnlineUsersListContainer();
        }

        // 更新在线用户数量
        const userCount = this.onlineUsers.size;
        const countElement = onlineUsersList.querySelector('.user-count');
        if (countElement) {
            countElement.textContent = userCount;
        }

        // 更新用户列表
        const userListContainer = onlineUsersList.querySelector('.users-container');
        if (userListContainer) {
            userListContainer.innerHTML = '';

            // 渲染所有在线用户
            this.onlineUsers.forEach((userInfo, userId) => {
                const userElement = this.createUserElement(userId, userInfo);
                userListContainer.appendChild(userElement);
            });
        }

        // 更新页面头部的用户计数（如果存在）
        this.updatePageUserCount(userCount);
    }

    /**
     * 创建在线用户列表容器
     */
    createOnlineUsersListContainer() {
        const onlineUsersList = document.createElement('div');
        onlineUsersList.id = 'online-users-list';
        onlineUsersList.className = 'online-users-list';
        onlineUsersList.innerHTML = `
        <div class="online-users-header">
            <h6 class="mb-2">
                <i class="bi bi-people"></i>
                在线用户 
                <span class="badge bg-primary user-count">0</span>
            </h6>
        </div>
        <div class="users-container">
            <!-- 用户列表将在这里动态生成 -->
        </div>
    `;

        // 找到合适的位置插入用户列表
        const targetContainer = this.findUserListInsertionPoint();
        if (targetContainer) {
            targetContainer.appendChild(onlineUsersList);
        } else {
            // 如果找不到合适位置，添加到页面右上角
            onlineUsersList.style.cssText = `
            position: fixed;
            top: 60px;
            right: 20px;
            width: 250px;
            background: white;
            border: 1px solid #dee2e6;
            border-radius: 8px;
            padding: 15px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
            z-index: 1000;
        `;
            document.body.appendChild(onlineUsersList);
        }

        return onlineUsersList;
    }

    /**
     * 查找用户列表的合适插入位置
     */
    findUserListInsertionPoint() {
        // 优先查找角色列表卡片
        const characterCard = document.querySelector('.characters-section, .character-list, .col-md-3 .card');
        if (characterCard) {
            return characterCard.querySelector('.card-body') || characterCard;
        }

        // 查找侧边栏
        const sidebar = document.querySelector('.sidebar, .side-panel, .col-md-3');
        if (sidebar) {
            return sidebar;
        }

        // 查找主容器
        const mainContainer = document.querySelector('.container, .main-content, main');
        return mainContainer;
    }

    /**
     * 创建单个用户元素
     */
    createUserElement(userId, userInfo) {
        const userElement = document.createElement('div');
        userElement.className = 'user-item d-flex align-items-center mb-2';
        userElement.id = `online-user-${userId}`;

        // 计算在线时长
        const onlineTime = userInfo.joinTime ? Date.now() - userInfo.joinTime : 0;
        const onlineTimeText = this.formatOnlineTime(onlineTime);

        userElement.innerHTML = `
        <div class="user-avatar me-2">
            <div class="avatar-circle ${this.getUserStatusClass(userInfo.status)}">
                ${this.getUserInitials(userInfo.username)}
            </div>
        </div>
        <div class="user-info flex-grow-1">
            <div class="user-name fw-bold">${this.escapeHtml(userInfo.username)}</div>
            <div class="user-status small text-muted">${onlineTimeText}</div>
        </div>
        <div class="user-status-indicator">
            <span class="status-dot ${userInfo.status}"></span>
        </div>
    `;

        // 添加点击事件（可选）
        userElement.addEventListener('click', () => {
            this.handleUserClick(userId, userInfo);
        });

        return userElement;
    }

    /**
     * 获取用户状态样式类
     */
    getUserStatusClass(status) {
        const statusClasses = {
            'online': 'status-online',
            'busy': 'status-busy',
            'away': 'status-away',
            'offline': 'status-offline'
        };
        return statusClasses[status] || 'status-offline';
    }

    /**
     * 获取用户名首字母
     */
    getUserInitials(username) {
        if (!username) return '?';

        const words = username.trim().split(/\s+/);
        if (words.length >= 2) {
            return (words[0][0] + words[1][0]).toUpperCase();
        } else {
            return username.substring(0, 2).toUpperCase();
        }
    }

    /**
     * 格式化在线时长
     */
    formatOnlineTime(milliseconds) {
        const seconds = Math.floor(milliseconds / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);

        if (hours > 0) {
            return `在线 ${hours} 小时`;
        } else if (minutes > 0) {
            return `在线 ${minutes} 分钟`;
        } else {
            return '刚刚上线';
        }
    }

    /**
     * 更新页面用户计数
     */
    updatePageUserCount(count) {
        // 更新导航栏或头部的用户计数
        const countElements = document.querySelectorAll('.online-user-count, #online-user-count');
        countElements.forEach(element => {
            element.textContent = count;
        });

        // 更新页面标题（可选）
        if (count > 0) {
            const originalTitle = document.title.replace(/ \(\d+\)$/, '');
            document.title = `${originalTitle} (${count})`;
        }
    }

    /**
     * 处理用户点击事件
     */
    handleUserClick(userId, userInfo) {
        console.log('用户点击:', userId, userInfo);

        // 可以添加用户交互功能，如：
        // - 发送私信
        // - 查看用户资料
        // - 邀请协作等

        // 简单的示例：显示用户信息
        this.showUserInfo(userId, userInfo);
    }

    /**
     * 显示用户信息
     */
    showUserInfo(userId, userInfo) {
        const message = `
        用户: ${userInfo.username}
        状态: ${userInfo.status}
        加入时间: ${new Date(userInfo.joinTime).toLocaleString()}
        在线时长: ${this.formatOnlineTime(Date.now() - userInfo.joinTime)}
    `;

        if (typeof Utils !== 'undefined' && Utils.showInfo) {
            Utils.showInfo(message);
        } else {
            alert(message);
        }
    }

    /**
     * 清空在线用户列表
     */
    clearOnlineUsersList() {
        this.onlineUsers.clear();
        this.updateOnlineUsersUI();

        console.log('🧹 在线用户列表已清空');
    }

    /**
     * 获取在线用户数量
     */
    getOnlineUserCount() {
        return this.onlineUsers.size;
    }

    /**
     * 获取在线用户列表
     */
    getOnlineUsers() {
        const users = [];
        this.onlineUsers.forEach((userInfo, userId) => {
            users.push({
                userId,
                ...userInfo
            });
        });
        return users;
    }

    /**
     * 检查用户是否在线
     */
    isUserOnline(userId) {
        return this.onlineUsers.has(userId);
    }

    /**
     * 移除用户从在线列表
     */
    removeUserFromOnlineList(userId) {
        if (this.onlineUsers.has(userId)) {
            const userInfo = this.onlineUsers.get(userId);
            this.onlineUsers.delete(userId);

            // 更新UI
            this.updateOnlineUsersUI();

            // 触发事件
            this.emit('online_users:user_removed', {
                userId,
                username: userInfo.username,
                timestamp: Date.now()
            });

            console.log(`👤 用户 ${userInfo.username} 已从在线列表移除`);
        }
    }

    /**
     * 添加用户到在线列表
     */
    addUserToOnlineList(userId, userInfo) {
        this.onlineUsers.set(userId, {
            ...userInfo,
            joinTime: userInfo.joinTime || Date.now(),
            status: userInfo.status || 'online'
        });

        // 更新UI
        this.updateOnlineUsersUI();

        // 触发事件
        this.emit('online_users:user_added', {
            userId,
            userInfo,
            timestamp: Date.now()
        });

        console.log(`👤 用户 ${userInfo.username} 已添加到在线列表`);
    }

    /**
     * 设置用户状态
     */
    setUserStatus(userId, status) {
        if (this.onlineUsers.has(userId)) {
            const userInfo = this.onlineUsers.get(userId);
            userInfo.status = status;

            // 更新UI
            this.updateOnlineUsersUI();

            // 触发事件
            this.emit('online_users:status_changed', {
                userId,
                status,
                username: userInfo.username,
                timestamp: Date.now()
            });

            console.log(`👤 用户 ${userInfo.username} 状态更新为: ${status}`);
        }
    }

    /**
     * HTML转义（安全处理）
     */
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    // ========================================
    // 消息发送
    // ========================================

    /**
     * 发送消息
     */
    sendMessage(connectionId, message) {
        const connection = this.connections.get(connectionId);

        if (!connection || connection.readyState !== WebSocket.OPEN) {
            this.debugLog(`连接 ${connectionId} 不可用，消息加入队列`);
            this.queueMessage(connectionId, message);
            return false;
        }

        try {
            const messageStr = JSON.stringify({
                ...message,
                timestamp: Date.now(),
                clientId: this.getClientId()
            });

            connection.send(messageStr);
            this.debugLog('发送消息:', message);
            return true;

        } catch (error) {
            console.error('发送消息失败:', error);
            this.queueMessage(connectionId, message);
            return false;
        }
    }

    /**
     * 发送角色互动消息
     */
    sendCharacterInteraction(sceneId, characterId, message) {
        return this.sendMessage(`scene_${sceneId}`, {
            type: 'character_interaction',
            characterId,
            message,
            userId: this.getCurrentUserId()
        });
    }

    /**
     * 发送角色状态更新
     */
    sendCharacterStatusUpdate(sceneId, characterId, status) {
        return this.sendMessage(`scene_${sceneId}`, {
            type: 'update_character_status',
            characterId,
            status
        });
    }

    /**
     * 发送故事进度更新
     */
    sendStoryProgress(sceneId, progressData) {
        return this.sendMessage(`scene_${sceneId}`, {
            type: 'story_progress_update',
            progressData
        });
    }

    /**
     * 发送故事选择消息 - 修正格式
     */
    sendStoryChoice(sceneId, nodeId, choiceId, preferences = null) {
        const connectionId = `scene_${sceneId}`;

        const message = {
            type: 'story_choice',
            node_id: nodeId,
            choice_id: choiceId
        };

        // 添加用户偏好（如果提供）
        if (preferences && typeof preferences === 'object') {
            message.user_preferences = preferences;
        }

        return this.sendMessage(connectionId, message);
    }

    /**
     * 发送用户状态更新 - 标准化格式
     */
    sendUserStatusUpdate(status, action = null, additionalData = {}) {
        const message = {
            type: 'user_status_update',
            status: status
        };

        if (action) {
            message.action = action;
        }

        // 合并额外数据
        Object.assign(message, additionalData);

        return this.sendMessage('user_status', message);
    }

    // ========================================
    // 心跳和重连机制
    // ========================================

    /**
     * 启动心跳
     */
    startHeartbeat(connectionId) {
        this.stopHeartbeat(connectionId); // 清除之前的心跳

        const timer = setInterval(() => {
            this.sendHeartbeat(connectionId);
        }, this.options.heartbeatInterval);

        this.heartbeatTimers.set(connectionId, timer);
    }

    /**
     * 发送心跳
     */
    sendHeartbeat(connectionId) {
        this.sendMessage(connectionId, {
            type: 'heartbeat',
            timestamp: Date.now()
        });
    }

    /**
     * 处理心跳响应
     */
    handleHeartbeatResponse(connectionId, data) {
        const latency = Date.now() - data.timestamp;
        this.debugLog(`心跳延迟: ${latency}ms`);

        // 触发心跳事件
        this.emit('heartbeat', { connectionId, latency });
    }

    /**
     * 停止心跳
     */
    stopHeartbeat(connectionId) {
        const timer = this.heartbeatTimers.get(connectionId);
        if (timer) {
            clearInterval(timer);
            this.heartbeatTimers.delete(connectionId);
        }
    }

    /**
     * 暂停所有心跳
     */
    pauseHeartbeats() {
        this.heartbeatTimers.forEach(timer => clearInterval(timer));
        this.heartbeatTimers.clear();
    }

    /**
     * 恢复心跳
     */
    resumeHeartbeats() {
        this.connections.forEach((connection, connectionId) => {
            if (connection.readyState === WebSocket.OPEN) {
                this.startHeartbeat(connectionId);
            }
        });
    }

    /**
     * 尝试重连
     */
    async attemptReconnect(connectionId) {
        const attempts = this.reconnectAttempts.get(connectionId) || 0;

        if (attempts >= this.options.maxReconnectAttempts) {
            console.error(`❌ 连接 ${connectionId} 重连次数超限`);
            Utils.showError('连接已断开，请刷新页面重试');
            return;
        }

        this.reconnectAttempts.set(connectionId, attempts + 1);

        console.log(`🔄 尝试重连 ${connectionId} (${attempts + 1}/${this.options.maxReconnectAttempts})`);

        setTimeout(async () => {
            try {
                // 根据连接类型重新连接
                if (connectionId.startsWith('scene_')) {
                    const sceneId = connectionId.replace('scene_', '');
                    await this.connectToScene(sceneId);
                } else if (connectionId === 'user_status') {
                    await this.connectToUserStatus();
                }

                // 重连成功后发送队列中的消息
                this.processMessageQueue(connectionId);

            } catch (error) {
                console.error(`重连失败: ${error.message}`);
                this.attemptReconnect(connectionId);
            }
        }, this.options.reconnectInterval);
    }

    // ========================================
    // 事件系统
    // ========================================

    /**
     * 监听事件
     */
    on(event, handler) {
        if (!this.eventHandlers.has(event)) {
            this.eventHandlers.set(event, []);
        }
        this.eventHandlers.get(event).push(handler);
    }

    /**
     * 移除事件监听器
     */
    off(event, handler) {
        if (!this.eventHandlers.has(event)) return;

        const handlers = this.eventHandlers.get(event);
        const index = handlers.indexOf(handler);
        if (index > -1) {
            handlers.splice(index, 1);
        }
    }

    /**
     * 触发事件
     */
    emit(event, data) {
        if (!this.eventHandlers.has(event)) return;

        this.eventHandlers.get(event).forEach(handler => {
            try {
                handler(data);
            } catch (error) {
                console.error(`事件处理器错误 ${event}:`, error);
            }
        });
    }

    // ========================================
    // 工具方法
    // ========================================

    /**
     * 更新角色UI
     */
    updateCharacterUI(characterId, statusData) {
        const characterElement = document.querySelector(`[data-character-id="${characterId}"]`);
        if (!characterElement) return;

        // 更新状态指示器
        let statusIndicator = characterElement.querySelector('.status-indicator');
        if (!statusIndicator) {
            statusIndicator = document.createElement('div');
            statusIndicator.className = 'status-indicator';
            characterElement.appendChild(statusIndicator);
        }

        // 根据状态设置样式
        statusIndicator.className = `status-indicator status-${statusData.status}`;
        statusIndicator.title = `状态: ${statusData.status}`;

        // 添加状态样式
        if (!document.getElementById('realtime-status-styles')) {
            const style = document.createElement('style');
            style.id = 'realtime-status-styles';
            style.textContent = `
                .status-indicator {
                    position: absolute;
                    top: 5px;
                    right: 5px;
                    width: 12px;
                    height: 12px;
                    border-radius: 50%;
                    border: 2px solid white;
                }
                .status-available { background-color: #28a745; }
                .status-busy { background-color: #dc3545; }
                .status-away { background-color: #ffc107; }
                .status-offline { background-color: #6c757d; }
            `;
            document.head.appendChild(style);
        }
    }

    /**
     * 获取角色名称
     */
    getCharacterName(characterId) {
        if (!characterId) return `角色${characterId}`;

        // 首先尝试从DOM中获取
        const characterElement = document.querySelector(`[data-character-id="${characterId}"] .fw-bold`);
        if (characterElement && characterElement.textContent) {
            return characterElement.textContent;
        }

        // 如果DOM中找不到，尝试通过app实例获取
        if (window.SceneApp && typeof window.SceneApp.getCharacterName === 'function') {
            try {
                return window.SceneApp.getCharacterName(characterId);
            } catch (error) {
                console.debug('无法通过SceneApp获取角色名称:', error);
            }
        }

        // 返回默认名称
        return `角色${characterId}`;
    }

    /**
     * 获取当前用户ID
     */
    getCurrentUserId() {
        // 尝试从多个来源获取用户ID
        return window.currentUserId ||
            localStorage.getItem('userId') ||
            'anonymous';
    }

    /**
     * 获取客户端ID
     */
    getClientId() {
        if (!this.clientId) {
            this.clientId = 'client_' + Math.random().toString(36).substr(2, 9);
        }
        return this.clientId;
    }

    /**
     * 播放通知音
     */
    playNotificationSound() {
        if ('audioContext' in window || 'webkitAudioContext' in window) {
            // 使用 Web Audio API 播放简单的提示音
            const audioContext = new (window.AudioContext || window.webkitAudioContext)();
            const oscillator = audioContext.createOscillator();
            const gainNode = audioContext.createGain();

            oscillator.connect(gainNode);
            gainNode.connect(audioContext.destination);

            oscillator.frequency.value = 800;
            oscillator.type = 'sine';

            gainNode.gain.setValueAtTime(0.3, audioContext.currentTime);
            gainNode.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.3);

            oscillator.start(audioContext.currentTime);
            oscillator.stop(audioContext.currentTime + 0.3);
        }
    }

    /**
     * 消息队列管理
     */
    queueMessage(connectionId, message) {
        this.messageQueue.push({ connectionId, message, timestamp: Date.now() });

        // 保持队列大小
        if (this.messageQueue.length > this.options.messageQueueSize) {
            this.messageQueue.shift();
        }
    }

    /**
     * 处理消息队列
     */
    processMessageQueue(connectionId) {
        const messages = this.messageQueue.filter(item => item.connectionId === connectionId);

        messages.forEach(item => {
            this.sendMessage(connectionId, item.message);
        });

        // 移除已发送的消息
        this.messageQueue = this.messageQueue.filter(item => item.connectionId !== connectionId);
    }

    /**
     * 检查所有连接状态
     */
    checkAllConnections() {
        this.connections.forEach((connection, connectionId) => {
            if (connection.readyState !== WebSocket.OPEN) {
                this.attemptReconnect(connectionId);
            }
        });
    }

    /**
     * 处理网络恢复
     */
    handleNetworkRestore() {
        console.log('🌐 网络已恢复');
        Utils.showSuccess('网络连接已恢复');
        this.checkAllConnections();
    }

    /**
     * 处理网络断开
     */
    handleNetworkLoss() {
        console.log('🚫 网络已断开');
        Utils.showWarning('网络连接已断开，正在尝试重连...');
    }

    /**
     * 调试日志
     */
    debugLog(...args) {
        if (this.options.debug) {
            console.log('[RealtimeManager]', ...args);
        }
    }

    /**
     * 获取连接状态
     */
    getConnectionStatus() {
        const status = {};
        this.connections.forEach((connection, connectionId) => {
            status[connectionId] = {
                readyState: connection.readyState,
                readyStateText: this.getReadyStateText(connection.readyState),
                reconnectAttempts: this.reconnectAttempts.get(connectionId) || 0
            };
        });
        return status;
    }

    /**
     * 获取连接状态文本
     */
    getReadyStateText(readyState) {
        const states = {
            0: 'CONNECTING',
            1: 'OPEN',
            2: 'CLOSING',
            3: 'CLOSED'
        };
        return states[readyState] || 'UNKNOWN';
    }

    /**
     * 清理资源
     */
    cleanup() {
        // 停止所有心跳
        this.pauseHeartbeats();

        // 关闭所有连接
        this.connections.forEach(connection => {
            if (connection.readyState === WebSocket.OPEN) {
                connection.close();
            }
        });

        // 清理数据
        this.connections.clear();
        this.eventHandlers.clear();
        this.reconnectAttempts.clear();
        this.subscriptions.clear();
        this.messageQueue = [];

        console.log('🧹 RealtimeManager 已清理');
    }

    /**
     * 销毁管理器
     */
    destroy() {
        this.cleanup();
    }

    // ========================================
    // 场景专用功能
    // ========================================

    /**
     * 初始化场景实时功能（供外部调用）
     */
    async initSceneRealtime(sceneId) {
        try {
            console.log(`🔗 初始化场景 ${sceneId} 的实时功能`);

            // 连接到场景
            await this.connectToScene(sceneId);

            // 连接到用户状态
            await this.connectToUserStatus();

            // 设置事件监听器
            this.setupSceneEventListeners(sceneId);

            if (typeof Utils !== 'undefined') {
                Utils.showSuccess('实时功能已启用');
            }

            return true;

        } catch (error) {
            console.error('实时功能初始化失败:', error);
            if (typeof Utils !== 'undefined') {
                Utils.showWarning('实时功能暂时不可用');
            }
            return false;
        }
    }

    /**
     * 设置场景事件监听器
     */
    setupSceneEventListeners(sceneId) {
        // 角色状态更新
        this.on('character:status_updated', (data) => {
            this.updateCharacterStatusUI(data.characterId, data.status, data.mood);
        });

        // 新对话消息
        this.on('conversation:new', (data) => {
            if (data.sceneId === sceneId) {
                this.addNewConversationToUI(data.conversation);

                // 如果不是当前用户发送的，显示通知
                if (data.speakerId !== this.getCurrentUserId()) {
                    this.showRealtimeNotification(`新消息来自 ${this.getCharacterName(data.speakerId)}`);
                }
            }
        });

        // 故事事件
        this.on('story:event', (data) => {
            if (data.sceneId === sceneId) {
                this.handleStoryEventUI(data);
            }
        });

        // 用户在线状态
        this.on('user:presence', (data) => {
            if (data.sceneId === sceneId) {
                this.updateUserPresenceUI(data);
            }
        });

        // 场景状态更新
        this.on('scene:state_updated', (data) => {
            if (data.sceneId === sceneId) {
                this.updateSceneStateUI(data.state, data.changes);
            }
        });

        // 连接状态事件
        this.on('scene:connected', (data) => {
            if (data.sceneId === sceneId) {
                console.log('✅ 场景实时连接已建立');
            }
        });
    }

    // ========================================
    // UI 更新方法
    // ========================================

    /**
     * 更新角色状态UI
     */
    updateCharacterStatusUI(characterId, status, mood) {
        const characterElement = document.querySelector(`[data-character-id="${characterId}"]`);
        if (!characterElement) return;

        // 更新或创建状态指示器
        let statusIndicator = characterElement.querySelector('.status-indicator');
        if (!statusIndicator) {
            statusIndicator = document.createElement('div');
            statusIndicator.className = 'status-indicator';
            characterElement.appendChild(statusIndicator);
        }

        // 设置状态样式
        statusIndicator.className = `status-indicator status-${status}`;
        statusIndicator.title = `状态: ${status}${mood ? `, 心情: ${mood}` : ''}`;

        // 添加状态变化动画
        characterElement.style.transition = 'all 0.3s ease';
        characterElement.style.transform = 'scale(1.05)';
        setTimeout(() => {
            characterElement.style.transform = 'scale(1)';
        }, 300);

        // 确保状态样式已加载
        this.ensureStatusStyles();
    }

    /**
     * 添加新对话到UI
     */
    addNewConversationToUI(conversation) {
        if (window.SceneApp && window.SceneApp.addConversationToUI) {
            // 使用已有的方法添加对话
            window.SceneApp.addConversationToUI(conversation);

            // 添加新消息高亮效果
            setTimeout(() => {
                const lastMessage = document.querySelector('#chat-container .conversation-item:last-child');
                if (lastMessage) {
                    lastMessage.classList.add('new-message');
                }
            }, 100);
        } else {
            // 降级处理：直接操作DOM
            this.addConversationToChatContainer(conversation);
        }
    }

    /**
     * 处理故事事件UI
     */
    handleStoryEventUI(data) {
        const { eventType, eventData, description } = data;

        console.log('📖 故事事件:', eventType, eventData);

        // 显示故事事件通知
        if (description) {
            this.showRealtimeNotification(description, 'story');
        }

        // 更新故事相关UI
        if (eventType === 'progress_update' && window.StoryManager) {
            window.StoryManager.updateProgress(eventData);
        }
    }

    /**
     * 更新用户在线状态UI
     */
    updateUserPresenceUI(data) {
        const { userId, username, action } = data;

        // 更新在线用户列表（如果存在）
        let onlineUsersList = document.getElementById('online-users-list');
        if (!onlineUsersList) {
            // 创建在线用户列表
            onlineUsersList = this.createOnlineUsersList();
        }

        if (action === 'joined') {
            const userElement = document.createElement('span');
            userElement.className = 'badge bg-success me-1 mb-1';
            userElement.textContent = username;
            userElement.id = `user-${userId}`;
            onlineUsersList.appendChild(userElement);
        } else if (action === 'left') {
            const userElement = document.getElementById(`user-${userId}`);
            if (userElement) {
                userElement.remove();
            }
        }
    }

    /**
     * 更新场景状态UI
     */
    updateSceneStateUI(state, changes) {
        console.log('🎭 场景状态更新:', changes);

        // 如果有重要的场景变化，通知用户
        if (changes && changes.length > 0) {
            const importantChanges = changes.filter(change =>
                change.type === 'environment_change' ||
                change.type === 'time_change'
            );

            if (importantChanges.length > 0) {
                const descriptions = importantChanges.map(change => change.description);
                this.showRealtimeNotification(descriptions.join(', '), 'scene');
            }
        }
    }

    /**
     * 显示实时通知
     */
    showRealtimeNotification(message, type = 'info') {
        // 创建通知元素
        const notification = document.createElement('div');
        const alertClass = type === 'story' ? 'warning' : type === 'scene' ? 'info' : 'primary';
        const icon = type === 'story' ? '📖' : type === 'scene' ? '🎭' : '💬';

        notification.className = `alert alert-${alertClass} alert-dismissible fade show realtime-notification`;
        notification.innerHTML = `
            <strong>${icon}</strong> ${message}
            <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
        `;

        document.body.appendChild(notification);

        // 自动隐藏
        setTimeout(() => {
            if (notification.parentNode) {
                notification.remove();
            }
        }, 5000);
    }

    // ========================================
    // 辅助方法
    // ========================================

    /**
     * 创建在线用户列表
     */
    createOnlineUsersList() {
        const onlineUsersList = document.createElement('div');
        onlineUsersList.id = 'online-users-list';
        onlineUsersList.className = 'online-users-list mt-2 p-2';
        onlineUsersList.innerHTML = '<small class="text-muted d-block mb-1">在线用户:</small>';

        const charactersCard = document.querySelector('.col-md-3 .card .card-body');
        if (charactersCard) {
            charactersCard.appendChild(onlineUsersList);
        }

        return onlineUsersList;
    }

    /**
     * 确保状态样式已加载
     */
    ensureStatusStyles() {
        if (document.getElementById('realtime-status-styles')) return;

        const style = document.createElement('style');
        style.id = 'realtime-status-styles';
        style.textContent = `
            .status-indicator {
                position: absolute;
                top: 5px;
                right: 5px;
                width: 12px;
                height: 12px;
                border-radius: 50%;
                border: 2px solid white;
                z-index: 10;
            }
            .status-available { background-color: #28a745; }
            .status-busy { background-color: #dc3545; }
            .status-away { background-color: #ffc107; }
            .status-offline { background-color: #6c757d; }
            
            .online-users-list {
                border-top: 1px solid #e9ecef;
                background-color: #f8f9fa;
                border-radius: 6px;
            }
            
            .realtime-notification {
                position: fixed;
                top: 20px;
                right: 20px;
                z-index: 1050;
                max-width: 300px;
                animation: slideInRight 0.3s ease;
            }

            @keyframes slideInRight {
                from {
                    transform: translateX(100%);
                    opacity: 0;
                }
                to {
                    transform: translateX(0);
                    opacity: 1;
                }
            }
        `;

        document.head.appendChild(style);
    }

    /**
     * 直接添加对话到聊天容器（降级处理）
     */
    addConversationToChatContainer(conversation) {
        const chatContainer = document.getElementById('chat-container');
        if (!chatContainer) return;

        const messageElement = document.createElement('div');
        messageElement.className = 'conversation-item mb-2 p-2 rounded new-message';
        messageElement.innerHTML = `
            <div class="d-flex align-items-start">
                <div class="me-2">
                    <small class="text-muted">${conversation.speaker_name || '未知'}</small>
                </div>
                <div class="flex-grow-1">
                    <div class="message-content">${conversation.content || conversation.message}</div>
                    <small class="text-muted">${new Date(conversation.created_at).toLocaleTimeString()}</small>
                </div>
            </div>
        `;

        chatContainer.appendChild(messageElement);
        chatContainer.scrollTop = chatContainer.scrollHeight;
    }

    // ========================================
    // 场景交互增强
    // ========================================

    /**
     * 增强角色选择功能
     */
    enhanceCharacterSelection() {
        document.addEventListener('click', (e) => {
            if (e.target.closest('.character-item')) {
                const characterItem = e.target.closest('.character-item');
                const characterId = characterItem.dataset.characterId;
                const sceneId = this.getCurrentSceneId();

                // 现有的角色选择逻辑
                this.selectCharacter(characterId);

                // 发送角色状态更新（如果实时管理器可用）
                if (sceneId) {
                    this.sendCharacterStatusUpdate(sceneId, characterId, 'selected');
                }
            }
        });
    }

    /**
     * 增强消息发送功能
     */
    enhanceMessageSending() {
        const sendBtn = document.getElementById('send-btn');
        const messageInput = document.getElementById('message-input');

        if (!sendBtn || !messageInput) return;

        const handleSendMessage = () => {
            const message = messageInput.value.trim();
            const selectedCharacter = this.getSelectedCharacter();
            const sceneId = this.getCurrentSceneId();

            if (message && selectedCharacter && sceneId) {
                // 通过实时连接发送消息
                const success = this.sendCharacterInteraction(sceneId, selectedCharacter, message);

                if (success) {
                    // 清空输入框
                    messageInput.value = '';

                    // 临时禁用按钮防止重复发送
                    sendBtn.disabled = true;
                    setTimeout(() => {
                        sendBtn.disabled = false;
                    }, 500);
                }
            }
        };
        // 绑定发送按钮
        sendBtn.addEventListener('click', handleSendMessage);

        // 绑定回车键
        messageInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                handleSendMessage();
            }
        });
    }

    /**
     * 选择角色
     */
    selectCharacter(characterId) {
        // 清除之前的选择
        document.querySelectorAll('.character-item').forEach(item => {
            item.classList.remove('selected', 'border-primary');
        });

        // 选中新角色
        const characterElement = document.querySelector(`[data-character-id="${characterId}"]`);
        if (characterElement) {
            characterElement.classList.add('selected', 'border-primary');

            // 更新UI状态
            const characterName = characterElement.querySelector('.fw-bold').textContent;
            const selectedCharacterDiv = document.getElementById('selected-character');
            if (selectedCharacterDiv) {
                selectedCharacterDiv.textContent = `已选择: ${characterName}`;
            }

            // 启用输入
            const messageInput = document.getElementById('message-input');
            const sendBtn = document.getElementById('send-btn');
            if (messageInput) messageInput.disabled = false;
            if (sendBtn) sendBtn.disabled = false;
        }
    }

    /**
     * 获取选中的角色
     */
    getSelectedCharacter() {
        const selectedElement = document.querySelector('.character-item.selected');
        return selectedElement ? selectedElement.dataset.characterId : null;
    }

    /**
     * 获取当前场景ID
     */
    getCurrentSceneId() {
        // 优先从页面元素获取
        const sceneIdInput = document.getElementById('scene-id');
        if (sceneIdInput) {
            return sceneIdInput.value;
        }

        // 从URL路径获取
        const pathMatch = window.location.pathname.match(/\/scenes\/([^\/]+)/);
        if (pathMatch) {
            return pathMatch[1];
        }

        // 从URL参数获取
        const urlParams = new URLSearchParams(window.location.search);
        return urlParams.get('scene') || urlParams.get('sceneId') || urlParams.get('scene_id');
    }

    /**
     * 初始化场景页面增强功能
     */
    initScenePageEnhancements() {
        this.enhanceCharacterSelection();
        this.enhanceMessageSending();
        this.ensureStatusStyles();
    }
}

// ========================================
// 全局实例和便捷函数
// ========================================

/**
 * 初始化场景实时功能（供 HTML 调用）
 */
window.initSceneRealtime = async function (sceneId) {
    if (window.realtimeManager) {
        const success = await window.realtimeManager.initSceneRealtime(sceneId);
        if (success) {
            // 初始化页面增强功能
            window.realtimeManager.initScenePageEnhancements();
        }
        return success;
    }
    console.warn('RealtimeManager not available');
    return false;
};

/**
 * 刷新场景实时连接
 */
window.refreshSceneRealtime = function () {
    const sceneId = window.realtimeManager?.getCurrentSceneId();
    if (sceneId && window.realtimeManager) {
        return window.initSceneRealtime(sceneId);
    }
    return false;
};

// 确保在DOM加载完成后创建全局实例
if (typeof window !== 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            window.RealtimeManager = RealtimeManager;
            window.realtimeManager = new RealtimeManager();
            console.log('🔗 RealtimeManager 已准备就绪');
        });
    } else {
        window.RealtimeManager = RealtimeManager;
        window.realtimeManager = new RealtimeManager();
        console.log('🔗 RealtimeManager 已准备就绪');
    }
}

// 模块导出（如果支持）
if (typeof module !== 'undefined' && module.exports) {
    module.exports = RealtimeManager;
}

//在线用户管理调试工具
if (typeof window !== 'undefined' && 
    (window.location?.hostname === 'localhost' || window.location?.search.includes('debug=1'))) {

    window.ONLINE_USERS_DEBUG = {
        // 获取在线用户列表
        getOnlineUsers: () => {
            return window.realtimeManager ? window.realtimeManager.getOnlineUsers() : [];
        },

        // 获取在线用户数量
        getUserCount: () => {
            return window.realtimeManager ? window.realtimeManager.getOnlineUserCount() : 0;
        },

        // 模拟用户加入
        simulateUserJoin: (username = '测试用户') => {
            if (window.realtimeManager) {
                const userId = 'test_user_' + Date.now();
                window.realtimeManager.updateOnlineUsersList('test_scene', {
                    userId,
                    username,
                    action: 'joined'
                });
                return userId;
            }
            return null;
        },

        // 模拟用户离开
        simulateUserLeave: (userId = null) => {
            if (window.realtimeManager) {
                const users = window.realtimeManager.getOnlineUsers();
                const targetUserId = userId || (users.length > 0 ? users[0].userId : null);
                
                if (targetUserId) {
                    const user = users.find(u => u.userId === targetUserId);
                    window.realtimeManager.updateOnlineUsersList('test_scene', {
                        userId: targetUserId,
                        username: user ? user.username : '未知用户',
                        action: 'left'
                    });
                    return targetUserId;
                }
            }
            return null;
        },

        // 清空在线用户列表
        clearUsers: () => {
            if (window.realtimeManager) {
                window.realtimeManager.clearOnlineUsersList();
                return true;
            }
            return false;
        },

        // 添加多个测试用户
        addTestUsers: (count = 3) => {
            const userIds = [];
            for (let i = 1; i <= count; i++) {
                const userId = window.ONLINE_USERS_DEBUG.simulateUserJoin(`用户${i}`);
                if (userId) userIds.push(userId);
            }
            return userIds;
        },

        // 检查用户是否在线
        isUserOnline: (userId) => {
            return window.realtimeManager ? window.realtimeManager.isUserOnline(userId) : false;
        },

        // 设置用户状态
        setUserStatus: (userId, status) => {
            if (window.realtimeManager) {
                window.realtimeManager.setUserStatus(userId, status);
                return true;
            }
            return false;
        }
    };

    console.log('👥 在线用户调试工具已加载');
    console.log('使用 window.ONLINE_USERS_DEBUG 进行调试');
}
