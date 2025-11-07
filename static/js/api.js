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

        // 获取并添加认证token（如果存在）
        // Check and refresh token if needed
        let authToken = await this.refreshTokenIfNeeded();
        if (!authToken) {
            authToken = this.getAuthToken();
        }
        
        if (authToken) {
            config.headers['Authorization'] = `Bearer ${authToken}`;
        }

        // 处理FormData（文件上传）
        if (config.body instanceof FormData) {
            delete config.headers['Content-Type']; // 让浏览器自动设置
        } else if (config.body && typeof config.body === 'object') {
            config.body = JSON.stringify(config.body);
        }

        try {
            const response = await fetch(url, config);

            if (!response.ok) {
                // 处理认证错误
                if (response.status === 401) {
                    this.handleAuthError();
                    throw new Error('认证失败，请重新登录');
                }

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
     * 获取认证token
     */
    static getAuthToken() {
        // 从localStorage或sessionStorage获取token
        return localStorage.getItem('auth_token') || sessionStorage.getItem('auth_token');
    }

    /**
     * 设置认证token
     */
    static setAuthToken(token, remember = true) {
        if (remember) {
            localStorage.setItem('auth_token', token);
        } else {
            sessionStorage.setItem('auth_token', token);
        }
    }

    /**
     * 清除认证token
     */
    static clearAuthToken() {
        localStorage.removeItem('auth_token');
        sessionStorage.removeItem('auth_token');
    }

    /**
     * 处理认证错误
     */
    static handleAuthError() {
        console.warn('认证失败，跳转到登录页面');
        // 可以在这里重定向到登录页面或显示登录模态框
        this.clearAuthToken();
        if (window.location.pathname !== '/login' && window.location.pathname !== '/') {
            // 保存当前路径以便登录后返回
            sessionStorage.setItem('redirect_after_login', window.location.href);
            // 重定向到登录页面  
            window.location.href = '/login';
        }
    }

    /**
     * 检查并刷新认证token
     */
    static async refreshTokenIfNeeded() {
        const token = this.getAuthToken();
        if (!token) {
            return null;
        }

        // Check if token is expired or expiring soon
        try {
            const tokenPayload = this.parseJwt(token);
            const expiryTime = tokenPayload.exp * 1000; // Convert to milliseconds
            const currentTime = Date.now();
            const timeUntilExpiry = expiryTime - currentTime;

            // Refresh if token expires in less than 5 minutes
            if (timeUntilExpiry < 5 * 60 * 1000) {
                console.log('Token expiring soon, refreshing...');
                // In a real implementation, you would call a refresh endpoint
                // For now, we'll just return the current token
                return token;
            }
            
            return token;
        } catch (error) {
            console.error('Error parsing token:', error);
            return null;
        }
    }

    /**
     * 解析JWT token
     */
    static parseJwt(token) {
        try {
            const base64Url = token.split('.')[1];
            const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
            const jsonPayload = decodeURIComponent(
                atob(base64)
                    .split('')
                    .map(function(c) {
                        return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
                    })
                    .join('')
            );

            return JSON.parse(jsonPayload);
        } catch (error) {
            console.error('Error parsing JWT:', error);
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
        if (!sceneId) {
            throw new Error('场景ID不能为空');
        }

        // 构建查询参数
        const queryParams = new URLSearchParams();

        // 布尔参数处理
        if (options.includeConversations !== undefined) {
            queryParams.append('include_conversations', options.includeConversations.toString());
        }

        if (options.includeStory !== undefined) {
            queryParams.append('include_story', options.includeStory.toString());
        }

        if (options.includeUIState !== undefined) {
            queryParams.append('include_ui_state', options.includeUIState.toString());
        }

        if (options.includeProgress !== undefined) {
            queryParams.append('include_progress', options.includeProgress.toString());
        }

        if (options.includeCharacterStats !== undefined) {
            queryParams.append('include_character_stats', options.includeCharacterStats.toString());
        }

        // 数值参数处理
        if (options.conversationLimit && typeof options.conversationLimit === 'number') {
            queryParams.append('conversation_limit', options.conversationLimit.toString());
        }

        if (options.timeRange && typeof options.timeRange === 'string') {
            queryParams.append('time_range', options.timeRange);
        }

        // 用户偏好参数处理
        if (options.preferences && typeof options.preferences === 'object') {
            queryParams.append('preferences', JSON.stringify(options.preferences));
        }

        // 构建完整URL
        const url = `/scenes/${sceneId}/aggregate${queryParams.toString() ? '?' + queryParams.toString() : ''}`;

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
     * 获取场景统计数据（已废弃：使用API.getSceneAggregate替代）
     */
    async getSceneStats(sceneId) {
        console.warn('getSceneStats is deprecated. Use API.getSceneAggregate instead.');
        // Redirect to the proper aggregate method
        return this.getSceneAggregate(sceneId, {
            includeConversations: true,
            includeProgress: true
        });
    }

    /**
     * 获取场景对话列表
     */
    // Deprecated: Use API.getConversations instead (kept for backward compatibility)
    async getSceneConversations(sceneId, limit = 50) {
        console.warn('getSceneConversations is deprecated. Use API.getConversations instead.');
        return this.getConversations(sceneId, limit);
    }

    /**
     * 更新故事进度（已废弃：使用API.advanceStory替代）
     */
    async updateStoryProgress(sceneId, progressData) {
        console.warn('updateStoryProgress is deprecated. Use API.advanceStory instead.');
        return this.advanceStory(sceneId, progressData?.user_preferences || null);
    }

    /**
     * 创建场景对话
     */
    // Deprecated: Use appropriate story/interaction API methods instead (kept for backward compatibility)
    async createSceneConversation(sceneId, conversationData) {
        console.warn('createSceneConversation is deprecated. Use appropriate story/interaction API methods instead.');
        // Fixed the 'data' to 'body' property to be compatible with request method
        return this.request(`/scenes/${sceneId}/conversations`, {
            method: 'POST',
            body: conversationData
        });
    }

    // Redundant method - consolidated into other API methods
    // static getStoryProgress(sceneId) {
    //     return this.request(`/scenes/${sceneId}/story/progress`);
    // }
    // Use the existing getStoryData method instead

    // Redundant method - consolidated into other API methods  
    // static getSceneMetrics(sceneId) {
    //     return this.request(`/scenes/${sceneId}/metrics`);
    // }
    // Use the existing getSceneAggregate method instead

    // Redundant method - consolidated into other API methods
    // static getSceneAnalytics(sceneId, timeRange = '7d') {
    //     return this.request(`/scenes/${sceneId}/analytics?time_range=${timeRange}`);
    // }
    // Use the existing analytics endpoints appropriately through getSceneAnalytics method

    /**
    * 回溯故事到指定节点
    * @param {string} sceneId - 场景ID
    * @param {string|null} nodeId - 目标节点ID，null表示回溯到开始
    */
    static rewindStory(sceneId, nodeId = null) {
        if (!sceneId) {
            throw new Error('回溯故事需要 sceneId 参数');
        }

        // 构建请求体：
        // - 如果 nodeId 为 null 或 undefined，发送空对象（后端将理解为回溯到开始）
        // - 如果 nodeId 有值，发送包含 node_id 的对象
        const requestBody = nodeId ? { node_id: nodeId } : {};

        console.log(`回溯故事请求: sceneId=${sceneId}, nodeId=${nodeId}, requestBody=`, requestBody);

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

    /**
    * 重置故事到初始状态（基于回溯实现）
    * @param {string} sceneId - 场景ID
    * @param {object|null} preferences - 用户偏好设置（可选，暂不使用但保留接口兼容性）
    */
    static async resetStory(sceneId, preferences = null) {
        if (!sceneId) {
            throw new Error('重置故事需要 sceneId 参数');
        }

        try {
            // 使用回溯到开始来实现重置
            // 传递 null 作为 nodeId 表示回溯到最开始
            const result = await this.rewindStory(sceneId, null);

            console.log('故事重置成功:', result);
            return result;
        } catch (error) {
            console.error('重置故事失败:', error);
            throw new Error(`重置故事失败: ${error.message}`);
        }
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

    /**
     * 创建交互 - 统一的交互创建接口（对应后端的聚合交互API）
     */
    static createInteraction(interactionData) {
        // 验证必要参数
        if (!interactionData.scene_id) {
            throw new Error('场景ID不能为空');
        }

        if (!interactionData.character_id && !interactionData.character_ids) {
            throw new Error('角色ID不能为空');
        }

        if (!interactionData.message && !interactionData.topic) {
            throw new Error('消息内容或主题不能为空');
        }

        // 构建请求数据
        const requestData = {
            scene_id: interactionData.scene_id,
            message: interactionData.message || '',
            interaction_type: interactionData.interaction_type || 'chat',
            context: interactionData.context || {}
        };

        // 处理角色ID（支持单个或多个角色）
        if (interactionData.character_id) {
            // 单个角色交互
            requestData.character_ids = [interactionData.character_id];
        } else if (interactionData.character_ids && Array.isArray(interactionData.character_ids)) {
            // 多个角色交互
            requestData.character_ids = interactionData.character_ids;
        }

        // 根据交互类型选择不同的API端点
        switch (interactionData.interaction_type) {
            case 'chat':
            case 'skill_use':
            case 'user_interaction':
                // 使用聚合交互API处理用户交互
                return this.processInteractionAggregate(requestData);

            case 'character_interaction':
                // 角色间互动
                return this.triggerCharacterInteraction({
                    scene_id: requestData.scene_id,
                    character_ids: requestData.character_ids,
                    topic: interactionData.topic || requestData.message,
                    context_description: interactionData.context_description || ''
                });

            case 'character_simulation':
                // 角色对话模拟
                return this.simulateCharactersConversation({
                    scene_id: requestData.scene_id,
                    character_ids: requestData.character_ids,
                    initial_situation: interactionData.initial_situation || requestData.message,
                    number_of_turns: interactionData.number_of_turns || 3
                });

            default:
                // 默认使用聚合交互API
                return this.processInteractionAggregate(requestData);
        }
    }

    /**
     * 处理聚合交互 - 对应后端的 ProcessInteractionAggregate
     */
    static processInteractionAggregate(interactionData) {
        // 验证参数格式
        if (!interactionData || typeof interactionData !== 'object') {
            throw new Error('交互数据格式错误');
        }

        // 确保必要字段存在
        const requestData = {
            scene_id: interactionData.scene_id,
            character_ids: interactionData.character_ids || [],
            message: interactionData.message || '',
            interaction_type: interactionData.interaction_type || 'chat',
            context: {
                use_emotion: true,
                include_story_update: false,
                user_preferences: null,
                ...interactionData.context
            }
        };

        // 验证角色ID数组
        if (!Array.isArray(requestData.character_ids) || requestData.character_ids.length === 0) {
            throw new Error('至少需要指定一个角色ID');
        }

        return this.request('/interactions/aggregate', {
            method: 'POST',
            body: requestData
        });
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
        if (!userId) {
            throw new Error('获取道具需要 userId 参数');
        }

        return this.request(`/users/${userId}/items`);
    }

    /**
     * 获取单个用户道具
     */
    static getUserItem(userId, itemId) {
        if (!userId) {
            throw new Error('获取道具需要 userId 参数');
        }

        return this.request(`/users/${userId}/items/${itemId}`);
    }

    /**
     * 添加用户道具
     */
    static addUserItem(userId, itemData) {
        if (!userId) {
            throw new Error('添加道具需要 userId 参数');
        }

        return this.request(`/users/${userId}/items`, {
            method: 'POST',
            body: itemData
        });
    }

    /**
     * 更新用户道具
     */
    static updateUserItem(userId, itemId, itemData) {
        if (!userId || !itemId) {
            throw new Error('更新道具需要 userId 和 itemId 参数');
        }

        return this.request(`/users/${userId}/items/${itemId}`, {
            method: 'PUT',
            body: itemData
        });
    }

    /**
     * 删除用户道具
     */
    static deleteUserItem(userId, itemId) {
        if (!userId || !itemId) {
            throw new Error('删除道具需要 userId 和 itemId 参数');
        }

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
        // 确保taskId存在
        if (!taskId) {
            console.error('taskId不能为空');
            if (onError) onError(new Error('taskId不能为空'));
            return null;
        }

        const eventSource = new EventSource(`${this.BASE_URL}/progress/${taskId}`);

        // 存储事件处理器引用以便后续清理
        const handlers = {
            progress: (event) => {
                try {
                    const data = JSON.parse(event.data);
                    if (onProgress) onProgress(data);

                    // 检查是否完成
                    if (data.status === 'completed' || data.status === 'failed') {
                        // 延迟关闭连接以确保所有事件处理完毕
                        setTimeout(() => {
                            if (eventSource.readyState !== EventSource.CLOSED) {
                                eventSource.close();
                            }
                        }, 100);
                        
                        if (onComplete) onComplete(data);
                    }
                } catch (error) {
                    console.error('解析进度数据失败:', error);
                    if (onError) onError(error);
                }
            },
            
            connected: (event) => {
                console.log('进度订阅已连接');
            },
            
            heartbeat: (event) => {
                // 心跳事件，保持连接
            },
            
            error: (error) => {
                console.error('SSE连接错误:', error);
                // 检查是否是连接关闭错误，避免重复关闭
                if (eventSource.readyState !== EventSource.CLOSED) {
                    eventSource.close();
                }
                if (onError) onError(error);
            }
        };

        // 绑定事件处理器
        eventSource.addEventListener('progress', handlers.progress);
        eventSource.addEventListener('connected', handlers.connected);
        eventSource.addEventListener('heartbeat', handlers.heartbeat);
        eventSource.onerror = handlers.error;

        // 返回EventSource实例和清理函数，允许外部控制
        return {
            eventSource,
            close: () => {
                if (eventSource.readyState !== EventSource.CLOSED) {
                    eventSource.close();
                }
            },
            // 提供重新连接功能
            reconnect: () => {
                if (eventSource.readyState !== EventSource.CLOSED) {
                    eventSource.close();
                }
                return this.subscribeProgress(taskId, onProgress, onError, onComplete);
            }
        };
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

            // 更新后自动检查状态
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

        // 测试交互创建
        testCreateInteraction: async (sceneId, characterId, message) => {
            console.log('🔄 测试创建交互...');
            try {
                const result = await API.createInteraction({
                    scene_id: sceneId || 'test_scene',
                    character_id: characterId || 'test_character',
                    message: message || 'Hello, this is a test message',
                    interaction_type: 'chat',
                    context: {
                        use_emotion: true,
                        include_story_update: false
                    }
                });
                console.log('✅ 交互创建测试成功:', result);
                return result;
            } catch (error) {
                console.error('❌ 交互创建测试失败:', error);
                return { error: error.message };
            }
        },

        // 测试聚合交互
        testAggregateInteraction: async (data) => {
            console.log('🔄 测试聚合交互...');
            try {
                const result = await API.processInteractionAggregate(data || {
                    scene_id: 'test_scene',
                    character_ids: ['character1'],
                    message: 'Test message',
                    interaction_type: 'chat'
                });
                console.log('✅ 聚合交互测试成功:', result);
                return result;
            } catch (error) {
                console.error('❌ 聚合交互测试失败:', error);
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
        ],

        // 列出交互相关方法
        listInteractionMethods: () => [
            'createInteraction',
            'processInteractionAggregate',
            'triggerCharacterInteraction',
            'simulateCharactersConversation',
            'sendMessage',
            'sendMessageWithEmotion',
            'getCharacterInteractions',
            'getCharacterToCharacterInteractions'
        ]
    };

    console.log('🚀 API调试模式已启用');
    console.log('使用 window.API_DEBUG 查看调试工具');
}
