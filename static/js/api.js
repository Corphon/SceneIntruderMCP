/**
 * API 通信模块
 * 基于后端实际API路由重新设计
 * 支持完整的RESTful API调用
 */
class API {
    static BASE_URL = '/api';

    /**
     * 基础请求方法
     */
    static async request(url, options = {}) {
        // 确保URL以/api开头（如果不是完整URL）
        if (!url.startsWith('http') && !url.startsWith('/api')) {
            url = `${this.BASE_URL}${url.startsWith('/') ? '' : '/'}${url}`;
        }

        const config = {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        };

        // 处理FormData（文件上传）
        if (config.body instanceof FormData) {
            delete config.headers['Content-Type']; // 让浏览器自动设置
        } else if (config.body && typeof config.body === 'object') {
            config.body = JSON.stringify(config.body);
        }

        try {
            const response = await fetch(url, config);

            if (!response.ok) {
                const errorText = await response.text();
                let errorMessage = `HTTP ${response.status}`;
                
                try {
                    const errorJson = JSON.parse(errorText);
                    errorMessage = errorJson.error || errorMessage;
                } catch {
                    errorMessage = errorText || errorMessage;
                }
                
                throw new Error(errorMessage);
            }

            const contentType = response.headers.get('content-type');
            if (contentType && contentType.includes('application/json')) {
                return await response.json();
            }

            return await response.text();
        } catch (error) {
            console.error('API请求失败:', error);
            
            // 安全地调用错误处理
            this._handleError('请求失败: ' + error.message, error);
            
            throw error;
        }
    }

    /**
     * 安全的错误处理
     */
    static _handleError(message, error = null) {
        console.error(message, error);
        
        if (typeof window.Utils !== 'undefined' && window.Utils.showError) {
            window.Utils.showError(message);
        } else if (typeof alert !== 'undefined') {
            alert(message);
        }
    }

    /**
     * 安全的成功处理
     */
    static _handleSuccess(message) {
        console.log('成功:', message);
        
        if (typeof window.Utils !== 'undefined' && window.Utils.showSuccess) {
            window.Utils.showSuccess(message);
        }
    }

    // ========================================
    // 场景管理 API
    // ========================================

    /**
     * 获取所有场景列表
     */
    static getScenes() {
        return this.request('/scenes');
    }

    /**
     * 获取单个场景详情
     */
    static getScene(sceneId) {
        return this.request(`/scenes/${sceneId}`);
    }

    /**
     * 创建新场景
     */
    static createScene(data) {
        return this.request('/scenes', {
            method: 'POST',
            body: data
        });
    }

    /**
     * 获取场景角色列表
     */
    static getCharacters(sceneId) {
        return this.request(`/scenes/${sceneId}/characters`);
    }

    /**
     * 获取场景对话历史
     */
    static getConversations(sceneId, limit = 50) {
        return this.request(`/scenes/${sceneId}/conversations?limit=${limit}`);
    }

    /**
     * 获取场景聚合数据
     */
    static getSceneAggregate(sceneId, options = {}) {
        const params = new URLSearchParams();
        
        if (options.conversationLimit) {
            params.append('conversation_limit', options.conversationLimit);
        }
        if (options.includeStory !== undefined) {
            params.append('include_story', options.includeStory);
        }
        if (options.includeUIState !== undefined) {
            params.append('include_ui_state', options.includeUIState);
        }
        if (options.includeConversations !== undefined) {
            params.append('include_conversations', options.includeConversations);
        }
        if (options.includeProgress !== undefined) {
            params.append('include_progress', options.includeProgress);
        }
        if (options.userPreferences) {
            params.append('preferences', JSON.stringify(options.userPreferences));
        }

        const queryString = params.toString();
        const url = `/scenes/${sceneId}/aggregate${queryString ? '?' + queryString : ''}`;
        return this.request(url);
    }

    // ========================================
    // 聊天相关 API
    // ========================================

    /**
     * 发送聊天消息
     */
    static sendMessage(sceneId, characterId, message) {
        return this.request('/chat', {
            method: 'POST',
            body: {
                scene_id: sceneId,
                character_id: characterId,
                message: message
            }
        });
    }

    /**
     * 发送带情绪的聊天消息
     */
    static sendMessageWithEmotion(sceneId, characterId, message) {
        return this.request('/chat/emotion', {
            method: 'POST',
            body: {
                scene_id: sceneId,
                character_id: characterId,
                message: message
            }
        });
    }

    // ========================================
    // 故事系统 API
    // ========================================

    /**
     * 获取故事数据
     */
    static getStoryData(sceneId) {
        return this.request(`/scenes/${sceneId}/story`);
    }

    /**
     * 执行故事选择
     */
    static makeStoryChoice(sceneId, nodeId, choiceId, preferences = null) {
        // 参数验证
        if (!sceneId || !nodeId || !choiceId) {
            throw new Error('故事选择需要 sceneId, nodeId 和 choiceId 参数');
        }

        const requestBody = {
            node_id: nodeId,
            choice_id: choiceId
        };
        
        // 验证并添加偏好设置
        if (preferences) {
            if (typeof preferences === 'object' && preferences !== null) {
                requestBody.user_preferences = preferences;
            } else {
                console.warn('用户偏好必须是对象类型，已忽略');
            }
        }
        
        return this.request(`/scenes/${sceneId}/story/choice`, {
            method: 'POST',
            body: requestBody
        });
    }

    /**
     * 推进故事情节
     */
    static advanceStory(sceneId, preferences = null) {
        if (!sceneId) {
            throw new Error('故事推进需要 sceneId 参数');
        }

        const requestBody = {};
        
        if (preferences && typeof preferences === 'object') {
            requestBody.user_preferences = preferences;
        }
        
        return this.request(`/scenes/${sceneId}/story/advance`, {
            method: 'POST',
            body: requestBody
        });
    }

    /**
     * 获取故事分支
     */
    static getStoryBranches(sceneId, preferences = null) {
        let url = `/scenes/${sceneId}/story/branches`; 
        
        if (preferences) {
            url += `?preferences=${encodeURIComponent(JSON.stringify(preferences))}`;
        }
        
        return this.request(url);
    }

    /**
     * 回溯故事到指定节点
     */
    static rewindStory(sceneId, nodeId = null) {
        const requestBody = nodeId ? { node_id: nodeId } : {};
        
        return this.request(`/scenes/${sceneId}/story/rewind`, {
            method: 'POST',
            body: requestBody
        });
    }

    /**
     * 回溯故事到指定节点（兼容旧接口）
     */
    static rewindStoryToNode(sceneId, nodeId) {
        return this.request(`/scenes/${sceneId}/story/rewind`, { 
            method: 'POST',
            body: { node_id: nodeId }
        });
    }

    // ========================================
    // 角色互动 API
    // ========================================

    /**
     * 触发角色互动
     */
    static triggerCharacterInteraction(data) {
        return this.request('/interactions/trigger', {
            method: 'POST',
            body: data
        });
    }

    /**
     * 模拟角色对话
     */
    static simulateCharactersConversation(data) {
        return this.request('/interactions/simulate', {
            method: 'POST',
            body: data
        });
    }

    /**
     * 处理聚合交互
     */
    static processInteractionAggregate(data) {
        return this.request('/interactions/aggregate', {
            method: 'POST',
            body: data
        });
    }

    /**
     * 获取角色互动历史
     */
    static getCharacterInteractions(sceneId, options = {}) {
        const params = new URLSearchParams();
        
        if (options.limit) params.append('limit', options.limit);
        if (options.interactionId) params.append('interaction_id', options.interactionId);
        if (options.simulationId) params.append('simulation_id', options.simulationId);

        const queryString = params.toString();
        const url = `/interactions/${sceneId}${queryString ? '?' + queryString : ''}`;
        return this.request(url);
    }

    /**
     * 获取特定两个角色之间的互动
     */
    static getCharacterToCharacterInteractions(sceneId, character1Id, character2Id, limit = 20) {
        return this.request(`/interactions/${sceneId}/${character1Id}/${character2Id}?limit=${limit}`);
    }

    // ========================================
    // 用户管理 API
    // ========================================

    /**
     * 获取用户档案
     */
    static getUserProfile(userId) {
        if (!userId) {
            throw new Error('用户ID不能为空');
        }
        return this.request(`/users/${userId}`);
    }

    /**
     * 更新用户档案
     */
    static updateUserProfile(userId, profileData) {
        if (!userId) {
            throw new Error('用户ID不能为空');
        }
        
        if (!profileData || typeof profileData !== 'object') {
            throw new Error('档案数据格式错误');
        }
        
        // 验证允许的字段
        const allowedFields = ['username', 'display_name', 'bio', 'avatar', 'preferences'];
        const validatedData = {};
        
        for (const [key, value] of Object.entries(profileData)) {
            if (allowedFields.includes(key) && value !== undefined) {
                validatedData[key] = value;
            }
        }
        
        return this.request(`/users/${userId}`, {
            method: 'PUT',
            body: validatedData
        });
    }

    /**
     * 获取用户偏好设置
     */
    static getUserPreferences(userId) {
        if (!userId) {
            throw new Error('用户ID不能为空');
        }

        return this.request(`/users/${userId}/preferences`);
    }

    /**
     * 更新用户偏好设置
     */
    static updateUserPreferences(userId, preferences) {
        if (!userId) {
            this._handleError('用户ID不能为空');
            return Promise.reject(new Error('用户ID不能为空'));
        }
        
        if (!preferences || typeof preferences !== 'object') {
            this._handleError('偏好设置数据无效');
            return Promise.reject(new Error('偏好设置数据无效'));
        }
        
        // 验证创意等级枚举值
        const validCreativityLevels = ['STRICT', 'BALANCED', 'EXPANSIVE'];
        if (preferences.creativity_level && !validCreativityLevels.includes(preferences.creativity_level)) {
            this._handleError('无效的创意等级设置');
            return Promise.reject(new Error('无效的创意等级设置'));
        }
        
        // 验证响应长度
        const validResponseLengths = ['short', 'medium', 'long'];
        if (preferences.response_length && !validResponseLengths.includes(preferences.response_length)) {
            this._handleError('无效的响应长度设置');
            return Promise.reject(new Error('无效的响应长度设置'));
        }
        
        return this.request(`/users/${userId}/preferences`, {
            method: 'PUT',
            body: preferences
        });
    }

    // ========================================
    // 用户道具管理 API
    // ========================================

    /**
     * 获取用户道具列表
     */
    static getUserItems(userId) {
        return this.request(`/users/${userId}/items`);
    }

    /**
     * 获取单个用户道具
     */
    static getUserItem(userId, itemId) {
        return this.request(`/users/${userId}/items/${itemId}`);
    }

    /**
     * 添加用户道具
     */
    static addUserItem(userId, itemData) {
        return this.request(`/users/${userId}/items`, {
            method: 'POST',
            body: itemData
        });
    }

    /**
     * 更新用户道具
     */
    static updateUserItem(userId, itemId, itemData) {
        return this.request(`/users/${userId}/items/${itemId}`, {
            method: 'PUT',
            body: itemData
        });
    }

    /**
     * 删除用户道具
     */
    static deleteUserItem(userId, itemId) {
        return this.request(`/users/${userId}/items/${itemId}`, {
            method: 'DELETE'
        });
    }

    // ========================================
    // 用户技能管理 API
    // ========================================

    /**
     * 获取用户技能列表
     */
    static getUserSkills(userId) {
        return this.request(`/users/${userId}/skills`);
    }

    /**
     * 获取单个用户技能
     */
    static getUserSkill(userId, skillId) {
        return this.request(`/users/${userId}/skills/${skillId}`);
    }

    /**
     * 添加用户技能
     */
    static addUserSkill(userId, skillData) {
        return this.request(`/users/${userId}/skills`, {
            method: 'POST',
            body: skillData
        });
    }

    /**
     * 更新用户技能
     */
    static updateUserSkill(userId, skillId, skillData) {
        return this.request(`/users/${userId}/skills/${skillId}`, {
            method: 'PUT',
            body: skillData
        });
    }

    /**
     * 删除用户技能
     */
    static deleteUserSkill(userId, skillId) {
        return this.request(`/users/${userId}/skills/${skillId}`, {
            method: 'DELETE'
        });
    }

    // ========================================
    // 导出功能 API
    // ========================================

    /**
     * 导出交互摘要
     */
    static exportInteractionSummary(sceneId, format = 'json') {
        return this.request(`/scenes/${sceneId}/export/interactions?format=${format}`);
    }

    /**
     * 导出故事文档
     */
    static exportStoryDocument(sceneId, format = 'json') {
        return this.request(`/scenes/${sceneId}/export/story?format=${format}`);
    }

    /**
     * 导出场景数据
     */
    static exportSceneData(sceneId, format = 'json', includeConversations = false) {
        return this.request(`/scenes/${sceneId}/export/scene?format=${format}&include_conversations=${includeConversations}`);
    }

    // ========================================
    // 分析和进度 API
    // ========================================

    /**
     * 分析文本内容
     */
    static analyzeText(data) {
        return this.request('/analyze', {
            method: 'POST',
            body: data
        });
    }

    /**
     * 获取分析进度
     */
    static getAnalysisProgress(taskId) {
        return this.request(`/progress/${taskId}`);
    }

    /**
     * 取消分析任务
     */
    static cancelAnalysisTask(taskId) {
        return this.request(`/cancel/${taskId}`, {
            method: 'POST'
        });
    }

    /**
     * 订阅分析进度（SSE）
     */
    static subscribeProgress(taskId, onProgress, onError, onComplete) {
        const eventSource = new EventSource(`${this.BASE_URL}/progress/${taskId}`);
        
        eventSource.addEventListener('progress', (event) => {
            try {
                const data = JSON.parse(event.data);
                if (onProgress) onProgress(data);
                
                // 检查是否完成
                if (data.status === 'completed' || data.status === 'failed') {
                    eventSource.close();
                    if (onComplete) onComplete(data);
                }
            } catch (error) {
                console.error('解析进度数据失败:', error);
                if (onError) onError(error);
            }
        });

        eventSource.addEventListener('connected', (event) => {
            console.log('进度订阅已连接');
        });

        eventSource.addEventListener('heartbeat', (event) => {
            // 心跳事件，保持连接
        });

        eventSource.onerror = (error) => {
            console.error('SSE连接错误:', error);
            eventSource.close();
            if (onError) onError(error);
        };

        // 返回EventSource实例，允许外部关闭
        return eventSource;
    }

    // ========================================
    // 系统设置 API
    // ========================================

    /**
     * 获取系统设置
     */
    static getSettings() {
        return this.request('/settings');
    }

    /**
     * 保存系统设置
     */
    static saveSettings(settings) {
        return this.request('/settings', {
            method: 'POST',
            body: settings
        });
    }

    /**
     * 测试连接
     */
    static testConnection(data = {}) {
        return this.request('/settings/test-connection', {
            method: 'POST',
            body: data
        });
    }

    // ========================================
    // LLM 相关 API
    // ========================================

    /**
     * 获取LLM状态
     */
    static getLLMStatus() {
        return this.request('/llm/status');
    }

    /**
     * 获取LLM模型列表
     */
    static getLLMModels(provider = '') {
        const url = provider ? `/llm/models?provider=${provider}` : '/llm/models';
        return this.request(url);
    }

    /**
     * 更新LLM配置
     */
    static updateLLMConfig(provider, config) {
        return this.request('/llm/config', {
            method: 'PUT',
            body: {
                provider: provider,
                config: config
            }
        });
    }

    // ========================================
    // 文件上传 API
    // ========================================

    /**
     * 上传文件
     */
    static uploadFile(file, onProgress = null) {
        const formData = new FormData();
        formData.append('file', file);

        // 如果需要进度回调，使用XMLHttpRequest
        if (onProgress) {
            return new Promise((resolve, reject) => {
                const xhr = new XMLHttpRequest();
                
                xhr.upload.addEventListener('progress', (event) => {
                    if (event.lengthComputable) {
                        const percentComplete = (event.loaded / event.total) * 100;
                        onProgress(percentComplete);
                    }
                });

                xhr.addEventListener('load', () => {
                    if (xhr.status >= 200 && xhr.status < 300) {
                        try {
                            const response = JSON.parse(xhr.responseText);
                            resolve(response);
                        } catch (error) {
                            resolve(xhr.responseText);
                        }
                    } else {
                        reject(new Error(`HTTP ${xhr.status}: ${xhr.statusText}`));
                    }
                });

                xhr.addEventListener('error', () => {
                    reject(new Error('上传失败'));
                });

                xhr.open('POST', `${this.BASE_URL}/upload`);
                xhr.send(formData);
            });
        }

        // 普通上传
        return this.request('/upload', {
            method: 'POST',
            body: formData
        });
    }

    // ========================================
    // 便利方法
    // ========================================

    /**
     * 批量调用API（并发）
     */
    static async batchRequest(requests) {
        try {
            const promises = requests.map(req => {
                if (typeof req === 'function') {
                    return req();
                } else if (req.url) {
                    return this.request(req.url, req.options);
                }
                throw new Error('Invalid request format');
            });

            return await Promise.all(promises);
        } catch (error) {
            console.error('批量请求失败:', error);
            throw error;
        }
    }

    /**
     * 带重试的请求
     */
    static async requestWithRetry(url, options = {}, maxRetries = 3) {
        let lastError;
        
        for (let i = 0; i <= maxRetries; i++) {
            try {
                return await this.request(url, options);
            } catch (error) {
                lastError = error;
                
                if (i < maxRetries) {
                    // 指数退避
                    const delay = Math.pow(2, i) * 1000;
                    await new Promise(resolve => setTimeout(resolve, delay));
                    console.log(`重试第 ${i + 1} 次...`);
                }
            }
        }
        
        throw lastError;
    }

    /**
     * 检查API健康状态
     */
    static async healthCheck() {
        try {
            await this.request('/settings');
            return { status: 'healthy', timestamp: new Date().toISOString() };
        } catch (error) {
            return { 
                status: 'unhealthy', 
                error: error.message, 
                timestamp: new Date().toISOString() 
            };
        }
    }

    // ========================================
    // WebSocket 调试和管理 API
    // ========================================

    /**
     * 获取 WebSocket 连接状态（调试用）
     */
    static getWebSocketStatus() {
        return this.request('/ws/status');
    }

    /**
     * 清理 WebSocket 连接
     */
    static cleanupWebSocketConnections() {
        return this.request('/ws/cleanup', {
            method: 'POST'
        });
    }

    // ========================================
    // 配置健康检查 API
    // ========================================

    /**
     * 获取配置健康状态
     */
    static getConfigHealth() {
        return this.request('/config/health');
    }

    /**
     * 获取配置服务指标
     */
    static getConfigMetrics() {
        return this.request('/config/metrics');
    }

    // ========================================
    // 增强的 LLM 管理 API
    // ========================================

    /**
     * 更新LLM配置（增强版）
     */
    static async updateLLMConfig(provider, config) {
        try {
            // 更新配置
            const result = await this.request('/llm/config', {
                method: 'PUT',
                body: {
                    provider: provider,
                    config: config
                }
            });

            // 🔧 更新后自动检查状态
            try {
                const status = await this.getLLMStatus();
                console.log('LLM配置更新后状态:', status);
            } catch (statusError) {
                console.warn('获取LLM状态失败:', statusError.message);
            }

            return result;
        } catch (error) {
            console.error('LLM配置更新失败:', error);
            throw error;
        }
    }

    /**
     * 测试LLM连接（与后端的TestConnection对应）
     */
    static testLLMConnection() {
        return this.request('/settings/test-connection', {
            method: 'POST'
        });
    }

    // ========================================
    // 故事系统增强 API
    // ========================================

    /**
     * 批处理故事操作
     */
    static batchStoryOperations(sceneId, operations) {
        if (!sceneId || !Array.isArray(operations)) {
            throw new Error('批处理故事操作需要 sceneId 和操作数组');
        }

        return this.request(`/scenes/${sceneId}/story/batch`, {
            method: 'POST',
            body: {
                operations: operations
            }
        });
    }

    // ========================================
    // 系统集成增强
    // ========================================

    /**
     * 综合健康检查（包含所有子系统）
     */
    static async comprehensiveHealthCheck() {
        try {
            const results = await this.batchRequest([
                () => this.healthCheck(),           // 基础API健康检查
                () => this.getLLMStatus(),          // LLM服务状态
                () => this.getConfigHealth(),       // 配置健康状态
                () => this.getWebSocketStatus()     // WebSocket状态
            ]);

            return {
                status: 'healthy',
                timestamp: new Date().toISOString(),
                details: {
                    api: results[0],
                    llm: results[1],
                    config: results[2],
                    websocket: results[3]
                }
            };
        } catch (error) {
            return {
                status: 'unhealthy',
                error: error.message,
                timestamp: new Date().toISOString()
            };
        }
    }

    /**
     * 重新初始化LLM服务
     */
    static async reinitializeLLM(provider, config) {
        try {
            // 1. 更新配置
            await this.updateLLMConfig(provider, config);
            
            // 2. 测试连接
            await this.testLLMConnection();
            
            // 3. 获取最新状态
            const status = await this.getLLMStatus();
            
            if (status.ready) {
                this._handleSuccess('LLM服务重新初始化成功');
                return status;
            } else {
                throw new Error('LLM服务初始化后仍未就绪');
            }
        } catch (error) {
            this._handleError('LLM服务重新初始化失败: ' + error.message);
            throw error;
        }
    }

    // ========================================
    // 调试和开发增强
    // ========================================

    /**
     * 获取系统完整状态
     */
    static async getSystemStatus() {
        try {
            const [health, llmStatus, configHealth, wsStatus] = await Promise.allSettled([
                this.healthCheck(),
                this.getLLMStatus(),
                this.getConfigHealth(),
                this.getWebSocketStatus()
            ]);

            return {
                api: health.status === 'fulfilled' ? health.value : { error: health.reason?.message },
                llm: llmStatus.status === 'fulfilled' ? llmStatus.value : { error: llmStatus.reason?.message },
                config: configHealth.status === 'fulfilled' ? configHealth.value : { error: configHealth.reason?.message },
                websocket: wsStatus.status === 'fulfilled' ? wsStatus.value : { error: wsStatus.reason?.message },
                timestamp: new Date().toISOString()
            };
        } catch (error) {
            console.error('获取系统状态失败:', error);
            throw error;
        }
    }
}

// 确保全局可用
window.API = API;

// 添加调试辅助
if (typeof window !== 'undefined' && window.location?.hostname === 'localhost') {
    window.API_DEBUG = {
        // 列出所有可用的API方法
        listMethods: () => {
            const methods = [];
            for (const key of Object.getOwnPropertyNames(API)) {
                if (typeof API[key] === 'function' && key !== 'constructor') {
                    methods.push(key);
                }
            }
            return methods.sort();
        },
        
        // 测试基础连接
        testConnection: () => API.healthCheck(),

        // 测试LLM连接
        testAllConnections: async () => {
            console.log('🔍 测试所有连接...');
            try {
                const result = await API.comprehensiveHealthCheck();
                console.log('✅ 综合健康检查结果:', result);
                return result;
            } catch (error) {
                console.error('❌ 综合健康检查失败:', error);
                return { error: error.message };
            }
        },

        // 测试LLM设置
        testLLMSetup: async (provider, config) => {
            console.log(`🤖 测试LLM设置 (${provider})...`);
            try {
                const result = await API.reinitializeLLM(provider, config);
                console.log('✅ LLM设置测试成功:', result);
                return result;
            } catch (error) {
                console.error('❌ LLM设置测试失败:', error);
                return { error: error.message };
            }
        },
        
        // 获取API基础信息
        getInfo: () => ({
            baseUrl: API.BASE_URL,
            methods: window.API_DEBUG.listMethods().length,
            userAgent: navigator.userAgent
        }),
        
        // 获取系统健康状态
        getSystemDashboard: async () => {
            console.log('📊 获取系统仪表板...');
            try {
                const status = await API.getSystemStatus();
                console.table(status);
                return status;
            } catch (error) {
                console.error('❌ 获取系统状态失败:', error);
                return { error: error.message };
            }
        },

        // 列出新增的方法
        listNewMethods: () => [
            'getWebSocketStatus',
            'cleanupWebSocketConnections', 
            'getConfigHealth',
            'getConfigMetrics',
            'testLLMConnection',
            'batchStoryOperations',
            'comprehensiveHealthCheck',
            'reinitializeLLM',
            'getSystemStatus'
        ]
    };
    
    console.log('🚀 API调试模式已启用');
    console.log('使用 window.API_DEBUG 查看调试工具');
}
