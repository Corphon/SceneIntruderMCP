// API 通信模块
class API {
    static async request(url, options = {}) {
        const config = {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        };

        if (config.body && typeof config.body === 'object') {
            config.body = JSON.stringify(config.body);
        }

        try {
            const response = await fetch(url, config);

            if (!response.ok) {
                const errorText = await response.text();
                throw new Error(`HTTP ${response.status}: ${errorText}`);
            }

            const contentType = response.headers.get('content-type');
            if (contentType && contentType.includes('application/json')) {
                return await response.json();
            }

            return await response.text();
        } catch (error) {
            console.error('API请求失败:', error);
            Utils.showError('请求失败: ' + error.message);
            throw error;
        }
    }

    // 场景相关 - 匹配后端路由
    static getScenes() {
        return this.request('/api/scenes');
    }

    static getScene(id) {
        return this.request(`/api/scenes/${id}`);
    }

    static createScene(data) {
        return this.request('/api/scenes', {
            method: 'POST',
            body: data
        });
    }

    // 对话相关 - 修正API路径
    static sendMessage(sceneId, characterId, message) {
        return this.request('/api/chat', {
            method: 'POST',
            body: {
                scene_id: sceneId,
                character_id: characterId,
                message: message
            }
        });
    }

    // 修正对话历史获取路径
    static getConversations(sceneId, limit = 50) {
        return this.request(`/api/conversations/${sceneId}?limit=${limit}`);
    }

    // 角色相关
    static getCharacters(sceneId) {
        return this.request(`/api/scenes/${sceneId}/characters`);
    }

    // 新增：场景聚合数据
    static getSceneAggregate(sceneId, options = {}) {
        const params = new URLSearchParams();
        if (options.conversationLimit) params.append('conversation_limit', options.conversationLimit);
        if (options.includeStory !== undefined) params.append('include_story', options.includeStory);
        if (options.includeUIState !== undefined) params.append('include_ui_state', options.includeUIState);

        const queryString = params.toString();
        const url = `/api/scenes/${sceneId}/aggregate${queryString ? '?' + queryString : ''}`;
        return this.request(url);
    }

    // 新增：触发角色互动
    static triggerCharacterInteraction(data) {
        return this.request('/api/interactions/trigger', {
            method: 'POST',
            body: data
        });
    }

    // 新增：模拟角色对话
    static simulateCharactersConversation(data) {
        return this.request('/api/interactions/simulate', {
            method: 'POST',
            body: data
        });
    }

    // 设置相关
    static getSettings() {
        return this.request('/api/settings');
    }

    static saveSettings(settings) {
        return this.request('/api/settings', {
            method: 'POST',
            body: settings
        });
    }

    static testConnection() {
        return this.request('/api/settings/test-connection', {
            method: 'POST'
        });
    }
}

window.API = API;
