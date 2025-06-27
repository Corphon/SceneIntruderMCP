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
        const characterElement = document.querySelector(`[data-character-id="${characterId}"] .fw-bold`);
        return characterElement ? characterElement.textContent : `角色${characterId}`;
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
window.initSceneRealtime = async function(sceneId) {
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
window.refreshSceneRealtime = function() {
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
