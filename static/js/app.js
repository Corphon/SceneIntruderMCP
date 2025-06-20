// 主应用逻辑
class SceneApp {
    constructor() {
        this.currentScene = null;
        this.selectedCharacter = null;
        this.conversations = [];
    }

    // 初始化场景页面
    async initScene() {
        const sceneId = document.getElementById('scene-id')?.value;
        if (!sceneId) return;

        try {
            // 加载场景数据
            this.currentScene = await API.getScene(sceneId);

            // 加载对话历史
            await this.loadConversations();

            // 绑定事件
            this.bindEvents();

            Utils.showSuccess('场景加载完成');
        } catch (error) {
            Utils.showError('场景加载失败');
        }
    }

    // 加载对话历史
    async loadConversations() {
        try {
            this.conversations = await API.getConversations(this.currentScene.ID);
            this.renderConversations();
        } catch (error) {
            console.error('加载对话历史失败:', error);
        }
    }

    // 渲染对话
    renderConversations() {
        const container = document.getElementById('chat-container');
        container.innerHTML = '';

        this.conversations.forEach(conv => {
            this.addMessageToChat(conv);
        });

        // 滚动到底部
        container.scrollTop = container.scrollHeight;
    }

    // 添加消息到聊天
    addMessageToChat(message) {
        const container = document.getElementById('chat-container');
        const messageEl = document.createElement('div');
        messageEl.className = `message ${message.speaker_id === 'user' ? 'user-message' : 'character-message'} mb-3`;

        const time = Utils.formatTime(message.timestamp);
        const speaker = message.speaker_id === 'user' ? '你' :
            this.getCharacterName(message.speaker_id);

        messageEl.innerHTML = `
            <div class="message-header d-flex justify-content-between">
                <strong>${speaker}</strong>
                <small class="text-muted">${time}</small>
            </div>
            <div class="message-content">${Utils.escapeHtml(message.content)}</div>
        `;

        container.appendChild(messageEl);
        container.scrollTop = container.scrollHeight;
    }

    // 获取角色名称
    getCharacterName(characterId) {
        const character = this.currentScene?.Characters?.find(c => c.ID === characterId);
        return character ? character.Name : '未知角色';
    }

    // 绑定事件
    bindEvents() {
        // 角色选择
        document.querySelectorAll('.character-item').forEach(item => {
            item.addEventListener('click', (e) => {
                this.selectCharacter(e.currentTarget.dataset.characterId);
            });
        });

        // 发送消息
        const sendBtn = document.getElementById('send-btn');
        const messageInput = document.getElementById('message-input');

        sendBtn.addEventListener('click', () => this.sendMessage());

        messageInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.sendMessage();
            }
        });
    }

    // 选择角色
    selectCharacter(characterId) {
        // 更新选中状态
        document.querySelectorAll('.character-item').forEach(item => {
            item.classList.remove('selected');
        });

        const selectedItem = document.querySelector(`[data-character-id="${characterId}"]`);
        selectedItem.classList.add('selected');

        // 更新选中角色
        this.selectedCharacter = characterId;

        // 启用输入
        const messageInput = document.getElementById('message-input');
        const sendBtn = document.getElementById('send-btn');
        const selectedDisplay = document.getElementById('selected-character');

        messageInput.disabled = false;
        sendBtn.disabled = false;
        selectedDisplay.textContent = `与 ${this.getCharacterName(characterId)} 对话`;

        messageInput.focus();
    }

    // 发送消息
    async sendMessage() {
        const messageInput = document.getElementById('message-input');
        const message = messageInput.value.trim();

        if (!message || !this.selectedCharacter) return;

        try {
            // 添加用户消息到界面
            this.addMessageToChat({
                speaker_id: 'user',
                content: message,
                timestamp: new Date()
            });

            // 清空输入框
            messageInput.value = '';

            // 发送到后端
            const response = await API.sendMessage(
                this.currentScene.ID,
                this.selectedCharacter,
                message
            );

            // 添加AI回复到界面
            if (response && response.content) {
                this.addMessageToChat({
                    speaker_id: this.selectedCharacter,
                    content: response.content,
                    timestamp: new Date()
                });
            }

        } catch (error) {
            Utils.showError('发送消息失败');
        }
    }

    // 初始化场景创建页面
    initSceneCreate() {
        const form = document.getElementById('create-scene-form');
        if (!form) return;

        form.addEventListener('submit', async (e) => {
            e.preventDefault();

            const formData = new FormData(form);
            const data = {
                text: formData.get('scene_text'),
                title: formData.get('scene_title') || '新场景'
            };

            try {
                Utils.showSuccess('正在创建场景...');
                const result = await API.createScene(data);

                if (result && result.scene_id) {
                    Utils.showSuccess('场景创建成功');
                    setTimeout(() => {
                        window.location.href = `/scenes/${result.scene_id}`;
                    }, 1000);
                }
            } catch (error) {
                Utils.showError('创建场景失败');
            }
        });
    }

    // 初始化设置页面
    initSettings() {
        const form = document.getElementById('settings-form');
        if (!form) return;

        // 加载当前设置
        this.loadCurrentSettings();

        // 绑定表单提交
        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            await this.saveSettings();
        });

        // 绑定测试连接
        const testBtn = document.getElementById('test-connection');
        testBtn.addEventListener('click', () => this.testConnection());
    }

    // 加载当前设置
    async loadCurrentSettings() {
        try {
            const settings = await API.getSettings();

            if (settings.llm_provider) {
                document.getElementById('llm-provider').value = settings.llm_provider;
            }
            if (settings.model) {
                document.getElementById('model-name').value = settings.model;
            }
            if (settings.debug_mode) {
                document.getElementById('debug-mode').checked = settings.debug_mode;
            }
        } catch (error) {
            console.error('加载设置失败:', error);
        }
    }

    // 保存设置
    async saveSettings() {
        const formData = new FormData(document.getElementById('settings-form'));
        const settings = {
            llm_provider: formData.get('llm_provider'),
            api_key: formData.get('api_key'),
            model: formData.get('model'),
            debug_mode: formData.get('debug_mode') === 'on'
        };

        try {
            await API.saveSettings(settings);
            Utils.showSuccess('设置保存成功');
        } catch (error) {
            Utils.showError('保存设置失败');
        }
    }

    // 测试连接
    async testConnection() {
        try {
            Utils.showSuccess('正在测试连接...');
            const result = await API.testConnection();
            if (result.success) {
                Utils.showSuccess('连接测试成功');
            } else {
                Utils.showError('连接测试失败: ' + result.error);
            }
        } catch (error) {
            Utils.showError('连接测试失败');
        }
    }
}

// 创建全局实例
window.SceneApp = new SceneApp();

// 根据页面类型自动初始化
document.addEventListener('DOMContentLoaded', function () {
    const path = window.location.pathname;

    if (path.includes('/scenes/create')) {
        window.SceneApp.initSceneCreate();
    }
    // 场景页面的初始化在模板的scripts块中调用
});
