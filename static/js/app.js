/**
 * SceneIntruderMCP 主应用程序
 * 基于后端完整API重新设计
 * 支持聚合数据、故事系统、用户管理等功能
 */
class SceneApp {
    constructor() {
        this.checkDependencies();

        this.currentScene = null;
        this.selectedCharacter = null;
        this.conversations = [];
        this.storyData = null;
        this.currentUser = null;
        this.isInitialized = false;
        this.aggregateData = null; // 场景聚合数据
        this.charts = new Map(); // 图表实例管理

        // 应用状态
        this.state = {
            isLoading: false,
            lastError: null,
            sceneLoaded: false,
            storyMode: false,
            interactionMode: false,
            dashboardVisible: false // 仪表板可见
        };

        // 事件监听器
        this.eventListeners = new Map();

        // 初始化调试工具
        this.initDebugMode();
    }

    // ========================================
    // 核心初始化方法
    // ========================================
    static isBootstrapAvailable() {
        return typeof bootstrap !== 'undefined' &&
            bootstrap.Toast &&
            bootstrap.Modal;
    }
    /**
     * 检查必要的依赖
     */
    checkDependencies() {
        const missing = [];

        if (typeof API === 'undefined') {
            missing.push('API');
        }

        if (typeof Utils === 'undefined') {
            missing.push('Utils');
        }

        if (missing.length > 0) {
            const message = `缺少必要的依赖: ${missing.join(', ')}`;
            console.error(message);

            // 显示错误（使用原生alert作为降级）
            if (typeof Utils !== 'undefined') {
                Utils.showError(message);
            } else {
                alert(`应用初始化失败: ${message}\n请确保正确加载了所有脚本文件`);
            }

            throw new Error(message);
        }

        // 检查Chart.js是否可用
        if (typeof Chart === 'undefined') {
            console.warn('⚠️ Chart.js未加载，数据可视化功能将受限');
        } else {
            console.log('✅ Chart.js已加载，数据可视化功能可用');
        }
    }
    /**
     * 初始化场景页面 - 使用聚合API
     */
    async initScene() {
        const sceneId = this.getSceneIdFromPage();
        if (!sceneId) {
            console.warn('未找到场景ID');
            return;
        }

        try {
            this.setState({ isLoading: true });

            // 使用聚合API获取完整场景数据
            const aggregateData = await API.getSceneAggregate(sceneId, {
                includeConversations: true,
                includeStory: true,
                includeUIState: true,
                includeProgress: true,
                conversationLimit: 50
            });

            // 设置应用数据
            this.currentScene = aggregateData.scene;
            this.conversations = aggregateData.conversations || [];
            this.storyData = aggregateData.story_data;

            // 渲染界面
            this.renderSceneInterface();
            this.renderConversations();
            this.renderStoryInterface();
            this.renderSceneDashboard();

            // 绑定事件
            this.bindSceneEvents();

            this.setState({
                isLoading: false,
                sceneLoaded: true
            });

            Utils.showSuccess('场景加载完成');
        } catch (error) {
            this.setState({
                isLoading: false,
                lastError: error.message
            });
            Utils.showError('场景加载失败: ' + error.message);
        }
    }

    /**
     * 初始化场景创建页面
     */
    initSceneCreate() {
        const form = document.getElementById('create-scene-form');
        if (!form) return;

        // 绑定表单提交
        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            await this.handleSceneCreation(form);
        });

        // 绑定文件上传
        const fileInput = document.getElementById('file-upload');
        if (fileInput) {
            fileInput.addEventListener('change', (e) => {
                this.handleFileUpload(e.target.files[0]);
            });
        }

        // 绑定实时预览
        const textArea = document.getElementById('scene-text');
        if (textArea) {
            textArea.addEventListener('input', () => {
                this.updateTextPreview(textArea.value);
            });
        }
    }

    /**
     * 初始化设置页面
     */
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
        if (testBtn) {
            testBtn.addEventListener('click', () => this.testConnection());
        }

        // 绑定模型选择
        const providerSelect = document.getElementById('llm-provider');
        if (providerSelect) {
            providerSelect.addEventListener('change', (e) => {
                this.loadModelsForProvider(e.target.value);
            });
        }
    }

    /**
     * 初始化用户档案页面
     */
    initUserProfile() {
        const userId = this.getCurrentUserId();
        if (!userId) {
            Utils.showError('未找到用户ID');
            return;
        }

        // 加载用户数据
        this.loadUserProfile(userId);

        // 绑定用户档案表单
        const profileForm = document.getElementById('profile-form');
        if (profileForm) {
            profileForm.addEventListener('submit', async (e) => {
                e.preventDefault();
                await this.updateUserProfile(userId);
            });
        }

        // 初始化道具管理
        this.initItemsManagement(userId);

        // 初始化技能管理
        this.initSkillsManagement(userId);
    }

    // ========================================
    // 场景交互核心功能
    // ========================================

    /**
     * 渲染场景界面
     */
    renderSceneInterface() {
        if (!this.currentScene) return;

        // 更新场景标题
        const titleEl = document.getElementById('scene-title');
        if (titleEl) {
            titleEl.textContent = this.currentScene.name || '未命名场景';
        }

        // 更新场景描述
        const descEl = document.getElementById('scene-description');
        if (descEl) {
            descEl.textContent = this.currentScene.description || '';
        }

        // 渲染角色列表
        this.renderCharacterList();

        // 更新界面模式
        this.updateInterfaceMode();
    }

    /**
     * 渲染角色列表
     */
    renderCharacterList() {
        const container = document.getElementById('characters-container');
        if (!container || !this.currentScene?.characters) return;

        container.innerHTML = '';

        this.currentScene.characters.forEach(character => {
            const characterEl = document.createElement('div');
            characterEl.className = 'character-item card mb-2 p-3';
            characterEl.dataset.characterId = character.id;

            characterEl.innerHTML = `
                <div class="d-flex align-items-center">
                    <div class="character-avatar me-3">
                        ${character.avatar ?
                    `<img src="${character.avatar}" alt="${character.name}" class="rounded-circle" width="40" height="40">` :
                    `<div class="avatar-placeholder rounded-circle bg-primary text-white d-flex align-items-center justify-content-center" style="width: 40px; height: 40px;">${character.name[0]}</div>`
                }
                    </div>
                    <div class="character-info flex-grow-1">
                        <h6 class="mb-1">${Utils.escapeHtml(character.name)}</h6>
                        <small class="text-muted">${Utils.escapeHtml(character.role || '角色')}</small>
                    </div>
                    <div class="character-actions">
                        <button class="btn btn-sm btn-outline-primary me-1" onclick="app.selectCharacter('${character.id}')">
                            <i class="bi bi-chat"></i> 对话
                        </button>
                        <button class="btn btn-sm btn-outline-secondary" onclick="app.viewCharacterDetails('${character.id}')">
                            <i class="bi bi-info-circle"></i>
                        </button>
                    </div>
                </div>
                ${character.description ? `<div class="character-description mt-2 small text-muted">${Utils.escapeHtml(character.description)}</div>` : ''}
            `;

            container.appendChild(characterEl);
        });
    }

    /**
     * 渲染对话历史
     */
    renderConversations() {
        const container = document.getElementById('chat-container');
        if (!container) return;

        container.innerHTML = '';

        if (!this.conversations || this.conversations.length === 0) {
            container.innerHTML = `
                <div class="text-center text-muted py-4">
                    <i class="bi bi-chat-dots fs-1"></i>
                    <p class="mt-2">还没有对话记录<br>选择一个角色开始对话吧！</p>
                </div>
            `;
            return;
        }

        this.conversations.forEach(conv => {
            this.addMessageToChat(conv, false);
        });

        // 滚动到底部
        container.scrollTop = container.scrollHeight;
    }

    /**
     * 添加消息到聊天界面
     */
    addMessageToChat(message, animate = true) {
        const container = document.getElementById('chat-container');
        if (!container) return;

        const messageEl = document.createElement('div');
        const isUser = message.speaker_id === 'user';

        messageEl.className = `message ${isUser ? 'user-message' : 'character-message'} mb-3`;
        if (animate) messageEl.classList.add('message-animate');

        const time = Utils.formatTime(message.timestamp);
        const speaker = isUser ? '你' : this.getCharacterName(message.speaker_id);
        const avatar = isUser ? '' : this.getCharacterAvatar(message.speaker_id);

        messageEl.innerHTML = `
            <div class="message-header d-flex justify-content-between align-items-center mb-2">
                <div class="d-flex align-items-center">
                    ${avatar ? `<img src="${avatar}" alt="${speaker}" class="rounded-circle me-2" width="24" height="24">` : ''}
                    <strong class="${isUser ? 'text-primary' : 'text-success'}">${speaker}</strong>
                </div>
                <small class="text-muted">${time}</small>
            </div>
            <div class="message-content">
                ${Utils.escapeHtml(message.content)}
                ${message.emotion ? `<div class="message-emotion mt-2"><span class="badge bg-light text-dark">${message.emotion}</span></div>` : ''}
            </div>
        `;

        container.appendChild(messageEl);

        // 平滑滚动到底部
        if (animate) {
            setTimeout(() => {
                container.scrollTo({
                    top: container.scrollHeight,
                    behavior: 'smooth'
                });
            }, 100);
        } else {
            container.scrollTop = container.scrollHeight;
        }
    }

    /**
     * 选择角色进行对话
     */
    selectCharacter(characterId) {
        // 更新选中状态
        document.querySelectorAll('.character-item').forEach(item => {
            item.classList.remove('selected', 'border-primary');
        });

        const selectedItem = document.querySelector(`[data-character-id="${characterId}"]`);
        if (selectedItem) {
            selectedItem.classList.add('selected', 'border-primary');
        }

        // 更新选中角色
        this.selectedCharacter = characterId;

        // 启用聊天界面
        this.enableChatInterface(characterId);

        // 显示角色信息
        this.displaySelectedCharacterInfo(characterId);
    }

    /**
     * 启用聊天界面
     */
    enableChatInterface(characterId) {
        const messageInput = document.getElementById('message-input');
        const sendBtn = document.getElementById('send-btn');
        const characterDisplay = document.getElementById('selected-character');

        if (messageInput) {
            messageInput.disabled = false;
            messageInput.placeholder = `与 ${this.getCharacterName(characterId)} 对话...`;
            messageInput.focus();
        }

        if (sendBtn) {
            sendBtn.disabled = false;
        }

        if (characterDisplay) {
            characterDisplay.innerHTML = `
                <div class="d-flex align-items-center">
                    ${this.getCharacterAvatar(characterId) ?
                    `<img src="${this.getCharacterAvatar(characterId)}" alt="avatar" class="rounded-circle me-2" width="24" height="24">` :
                    ''
                }
                    <span>正在与 <strong>${this.getCharacterName(characterId)}</strong> 对话</span>
                </div>
            `;
        }
    }

    /**
     * 发送消息 - 使用聚合交互API
     */
    async sendMessage() {
        const messageInput = document.getElementById('message-input');
        const message = messageInput?.value.trim();

        if (!message || !this.selectedCharacter) return;

        try {
            // 禁用输入
            this.setInputState(false);

            // 添加用户消息到界面
            this.addMessageToChat({
                speaker_id: 'user',
                content: message,
                timestamp: new Date()
            });

            // 清空输入框
            messageInput.value = '';

            // 使用聚合交互API
            const response = await API.processInteractionAggregate({
                scene_id: this.currentScene.id,
                character_ids: [this.selectedCharacter],
                message: message,
                interaction_type: 'chat',
                context: {
                    use_emotion: true,
                    include_story_update: this.state.storyMode
                }
            });

            // 处理响应
            if (response.character_response) {
                this.addMessageToChat({
                    speaker_id: this.selectedCharacter,
                    content: response.character_response.content,
                    emotion: response.character_response.emotion,
                    timestamp: new Date()
                });
            }

            // 更新故事状态（如果启用了故事模式）
            if (response.story_update && this.state.storyMode) {
                this.updateStoryDisplay(response.story_update);
            }

            // 记录统计
            if (response.stats) {
                this.updateStats(response.stats);
            }

        } catch (error) {
            Utils.showError('发送消息失败: ' + error.message);
        } finally {
            // 重新启用输入
            this.setInputState(true);
        }
    }

    // ========================================
    // 故事系统功能
    // ========================================

    /**
     * 渲染故事界面
     */
    renderStoryInterface() {
        if (!this.storyData) return;

        const storyContainer = document.getElementById('story-container');
        if (!storyContainer) return;

        storyContainer.innerHTML = `
            <div class="story-header mb-3">
                <h5 class="mb-2">${Utils.escapeHtml(this.storyData.intro || '故事开始')}</h5>
                <div class="story-progress">
                    <div class="progress mb-2">
                        <div class="progress-bar" role="progressbar" style="width: ${this.storyData.progress || 0}%"></div>
                    </div>
                    <small class="text-muted">进度: ${this.storyData.progress || 0}%</small>
                </div>
            </div>
            <div class="story-content">
                <div id="story-branches"></div>
                <div id="story-actions" class="mt-3"></div>
            </div>
        `;

        // 渲染故事分支
        this.renderStoryBranches();

        // 渲染故事操作按钮
        this.renderStoryActions();
    }

    /**
     * 渲染故事分支
     */
    renderStoryBranches() {
        const branchesContainer = document.getElementById('story-branches');
        if (!branchesContainer || !this.storyData?.root_nodes) return;

        branchesContainer.innerHTML = '';

        this.storyData.root_nodes.forEach(node => {
            const nodeEl = this.createStoryNodeElement(node);
            branchesContainer.appendChild(nodeEl);
        });
    }

    /**
     * 创建故事节点元素
     */
    createStoryNodeElement(node) {
        const nodeEl = document.createElement('div');
        nodeEl.className = `story-node card mb-2 ${node.is_active ? 'border-primary' : ''}`;

        nodeEl.innerHTML = `
            <div class="card-body">
                <div class="story-node-content mb-2">
                    ${Utils.escapeHtml(node.content)}
                </div>
                ${node.choices && node.choices.length > 0 ?
                `<div class="story-choices">
                        ${node.choices.map(choice => `
                            <button class="btn btn-sm btn-outline-primary me-2 mb-1" 
                                    onclick="app.makeStoryChoice('${node.id}', '${choice.id}')"
                                    ${choice.selected ? 'disabled' : ''}>
                                ${choice.selected ? '✓ ' : ''}${Utils.escapeHtml(choice.text)}
                            </button>
                        `).join('')}
                    </div>` : ''
            }
                ${node.children && node.children.length > 0 ?
                `<div class="story-children mt-2">
                        ${node.children.map(child => this.createStoryNodeElement(child).outerHTML).join('')}
                    </div>` : ''
            }
            </div>
        `;

        return nodeEl;
    }

    /**
     * 执行故事选择
     */
    async makeStoryChoice(nodeId, choiceId) {
        try {
            this.setState({ isLoading: true });

            const result = await API.makeStoryChoice(
                this.currentScene.id,
                nodeId,
                choiceId,
                this.currentUser?.preferences
            );

            if (result.success) {
                // 更新故事数据
                this.storyData = result.story_data;

                // 重新渲染故事界面
                this.renderStoryInterface();

                Utils.showSuccess('选择已执行');
            }

        } catch (error) {
            Utils.showError('执行选择失败: ' + error.message);
        } finally {
            this.setState({ isLoading: false });
        }
    }

    /**
     * 推进故事
     */
    async advanceStory() {
        try {
            this.setState({ isLoading: true });

            const result = await API.advanceStory(
                this.currentScene.id,
                this.currentUser?.preferences
            );

            if (result.success) {
                this.storyData = result.story_data;
                this.renderStoryInterface();

                Utils.showSuccess('故事已推进: ' + result.new_content);
            }

        } catch (error) {
            Utils.showError('推进故事失败: ' + error.message);
        } finally {
            this.setState({ isLoading: false });
        }
    }

    /**
     * 回溯故事
     */
    async rewindStory(nodeId) {
        try {
            this.setState({ isLoading: true });

            const result = await API.rewindStory(this.currentScene.id, nodeId);

            if (result.success) {
                this.storyData = result.story_data;
                this.renderStoryInterface();

                Utils.showSuccess('故事已回溯');
            }

        } catch (error) {
            Utils.showError('回溯故事失败: ' + error.message);
        } finally {
            this.setState({ isLoading: false });
        }
    }

    // ========================================
    // 用户管理功能
    // ========================================

    /**
     * 加载用户档案
     */
    async loadUserProfile(userId) {
        try {
            const profile = await API.getUserProfile(userId);
            this.currentUser = profile;

            // 渲染用户信息
            this.renderUserProfile();

            // 加载用户道具和技能
            await this.loadUserItems(userId);
            await this.loadUserSkills(userId);

        } catch (error) {
            Utils.showError('加载用户档案失败: ' + error.message);
        }
    }

    /**
     * 初始化道具管理
     */
    initItemsManagement(userId) {
        // 绑定添加道具按钮
        const addItemBtn = document.getElementById('add-item-btn');
        if (addItemBtn) {
            addItemBtn.addEventListener('click', () => {
                this.showAddItemModal(userId);
            });
        }

        // 加载现有道具
        this.loadUserItems(userId);
    }

    /**
     * 加载用户道具
     */
    async loadUserItems(userId) {
        try {
            const items = await API.getUserItems(userId);
            this.renderUserItems(items);
        } catch (error) {
            console.error('加载道具失败:', error);
        }
    }

    /**
     * 渲染用户道具
     */
    renderUserItems(items) {
        const container = document.getElementById('user-items-container');
        if (!container) return;

        container.innerHTML = '';

        if (!items || items.length === 0) {
            container.innerHTML = `
                <div class="text-center text-muted py-4">
                    <i class="bi bi-bag fs-1"></i>
                    <p class="mt-2">还没有道具<br>添加一些道具来增强你的角色吧！</p>
                </div>
            `;
            return;
        }

        items.forEach(item => {
            const itemEl = document.createElement('div');
            itemEl.className = 'col-md-6 col-lg-4 mb-3';

            itemEl.innerHTML = `
                <div class="card item-card">
                    <div class="card-body">
                        <h6 class="card-title">${Utils.escapeHtml(item.name)}</h6>
                        <p class="card-text small text-muted">${Utils.escapeHtml(item.description || '无描述')}</p>
                        <div class="item-meta">
                            <span class="badge bg-primary">${item.type || '道具'}</span>
                            ${item.rarity ? `<span class="badge bg-warning">${item.rarity}</span>` : ''}
                        </div>
                        <div class="item-actions mt-2">
                            <button class="btn btn-sm btn-outline-primary" onclick="app.editItem('${item.id}')">
                                <i class="bi bi-pencil"></i> 编辑
                            </button>
                            <button class="btn btn-sm btn-outline-danger" onclick="app.deleteItem('${item.id}')">
                                <i class="bi bi-trash"></i> 删除
                            </button>
                        </div>
                    </div>
                </div>
            `;

            container.appendChild(itemEl);
        });
    }

    // ========================================
    // 数据可视化功能
    // ========================================

    /**
     * 渲染场景统计仪表板
     */
    renderSceneDashboard() {
        if (!this.aggregateData) return;

        const dashboardContainer = this.getDashboardContainer();

        dashboardContainer.innerHTML = `
        <div class="dashboard-header d-flex justify-content-between align-items-center mb-4">
            <h4 class="mb-0">
                <i class="bi bi-graph-up"></i> 场景数据分析
            </h4>
            <div class="dashboard-actions">
                <button class="btn btn-sm btn-outline-secondary refresh-dashboard-btn" title="刷新数据">
                    <i class="bi bi-arrow-clockwise"></i>
                </button>
                <button class="btn btn-sm btn-outline-info toggle-dashboard-btn" title="切换显示">
                    <i class="bi bi-eye"></i>
                </button>
                <button class="btn btn-sm btn-outline-success export-dashboard-btn" title="导出报告">
                    <i class="bi bi-download"></i>
                </button>
            </div>
        </div>
        <div class="dashboard-content">
            <div class="row g-4">
                <!-- 基础统计卡片 -->
                <div class="col-12">
                    <div class="stats-cards-container">
                        ${this.renderStatsCards()}
                    </div>
                </div>
                
                <!-- 图表区域 -->
                <div class="col-md-6">
                    <div class="chart-card card">
                        <div class="card-header">
                            <h6 class="mb-0">
                                <i class="bi bi-pie-chart"></i> 角色互动分布
                            </h6>
                        </div>
                        <div class="card-body">
                            <canvas id="character-interaction-chart" width="400" height="300"></canvas>
                        </div>
                    </div>
                </div>
                
                <div class="col-md-6">
                    <div class="chart-card card">
                        <div class="card-header">
                            <h6 class="mb-0">
                                <i class="bi bi-graph-up"></i> 故事完成度
                            </h6>
                        </div>
                        <div class="card-body">
                            <canvas id="story-progress-chart" width="400" height="300"></canvas>
                        </div>
                    </div>
                </div>
                <!-- 关系图 -->
                <div class="col-12">
                    <div class="chart-card card">
                        <div class="card-header">
                            <h6 class="mb-0">
                                <i class="bi bi-diagram-3"></i> 角色关系网络
                            </h6>
                        </div>
                        <div class="card-body">
                            <div id="character-relationship-graph" style="height: 400px;"></div>
                        </div>
                    </div>
                </div>
                
                <!-- 时间线分析 -->
                <div class="col-12">
                    <div class="chart-card card">
                        <div class="card-header">
                            <h6 class="mb-0">
                                <i class="bi bi-clock-history"></i> 互动时间线
                            </h6>
                        </div>
                        <div class="card-body">
                            <canvas id="interaction-timeline-chart" width="800" height="200"></canvas>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `;

        // 渲染图表
        setTimeout(() => {
            this.renderCharacterInteractionChart();
            this.renderStoryProgressChart();
            this.renderCharacterRelationshipGraph();
            this.renderInteractionTimelineChart();
        }, 100);

        // 绑定仪表板事件
        this.bindDashboardEvents();
    }

    /**
 * 渲染统计卡片
 */
    renderStatsCards() {
        const stats = this.calculateSceneStats();

        return `
        <div class="row g-3">
            <div class="col-6 col-md-3">
                <div class="stat-card card text-center">
                    <div class="card-body">
                        <div class="stat-icon text-primary mb-2">
                            <i class="bi bi-people fs-1"></i>
                        </div>
                        <div class="stat-value h3">${stats.characterCount}</div>
                        <div class="stat-label text-muted">参与角色</div>
                    </div>
                </div>
            </div>
            
            <div class="col-6 col-md-3">
                <div class="stat-card card text-center">
                    <div class="card-body">
                        <div class="stat-icon text-success mb-2">
                            <i class="bi bi-chat-dots fs-1"></i>
                        </div>
                        <div class="stat-value h3">${stats.conversationCount}</div>
                        <div class="stat-label text-muted">对话次数</div>
                    </div>
                </div>
            </div>
            <div class="col-6 col-md-3">
                <div class="stat-card card text-center">
                    <div class="card-body">
                        <div class="stat-icon text-warning mb-2">
                            <i class="bi bi-book fs-1"></i>
                        </div>
                        <div class="stat-value h3">${stats.storyProgress}%</div>
                        <div class="stat-label text-muted">故事完成度</div>
                    </div>
                </div>
            </div>
            
            <div class="col-6 col-md-3">
                <div class="stat-card card text-center">
                    <div class="card-body">
                        <div class="stat-icon text-info mb-2">
                            <i class="bi bi-clock fs-1"></i>
                        </div>
                        <div class="stat-value h3">${stats.avgResponseTime}s</div>
                        <div class="stat-label text-muted">平均响应时间</div>
                    </div>
                </div>
            </div>
        </div>
    `;
    }

    /**
     * 计算场景统计数据
     */
    calculateSceneStats() {
        const stats = {
            characterCount: 0,
            conversationCount: 0,
            storyProgress: 0,
            avgResponseTime: 0
        };

        if (this.aggregateData) {
            stats.characterCount = this.aggregateData.characters?.length || 0;
            stats.conversationCount = this.aggregateData.recent_conversations?.length || 0;
            stats.storyProgress = Math.round(this.aggregateData.progress?.story_completion || 0);

            // 计算平均响应时间（模拟数据）
            stats.avgResponseTime = this.calculateAverageResponseTime();
        }

        return stats;
    }

    /**
 * 渲染角色互动分布图
 */
    renderCharacterInteractionChart() {
        if (typeof Chart === 'undefined') return;

        const canvas = document.getElementById('character-interaction-chart');
        if (!canvas) return;

        // 计算角色互动数据
        const interactionData = this.calculateCharacterInteractions();

        const chart = new Chart(canvas, {
            type: 'doughnut',
            data: {
                labels: interactionData.labels,
                datasets: [{
                    data: interactionData.values,
                    backgroundColor: [
                        '#FF6384', '#36A2EB', '#FFCE56', '#4BC0C0',
                        '#9966FF', '#FF9F40', '#FF6384', '#C9CBCF'
                    ],
                    borderWidth: 2,
                    borderColor: '#fff'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'bottom',
                        labels: {
                            padding: 20,
                            usePointStyle: true
                        }
                    },
                    tooltip: {
                        callbacks: {
                            label: (context) => {
                                const label = context.label || '';
                                const value = context.parsed;
                                const total = context.dataset.data.reduce((a, b) => a + b, 0);
                                const percentage = ((value / total) * 100).toFixed(1);
                                return `${label}: ${value} (${percentage}%)`;
                            }
                        }
                    }
                }
            }
        });

        this.charts.set('character-interaction', chart);
    }

    /**
 * 计算角色互动数据
 */
    calculateCharacterInteractions() {
        const interactions = new Map();

        if (this.conversations) {
            this.conversations.forEach(conv => {
                if (conv.speaker_id !== 'user') {
                    const characterName = this.getCharacterName(conv.speaker_id);
                    interactions.set(characterName, (interactions.get(characterName) || 0) + 1);
                }
            });
        }

        return {
            labels: Array.from(interactions.keys()),
            values: Array.from(interactions.values())
        };
    }

    /**
     * 绑定仪表板事件
     */
    bindDashboardEvents() {
        // 刷新仪表板
        document.addEventListener('click', (e) => {
            if (e.target.matches('.refresh-dashboard-btn')) {
                this.refreshDashboard();
            }

            if (e.target.matches('.toggle-dashboard-btn')) {
                this.toggleDashboard();
            }

            if (e.target.matches('.export-dashboard-btn')) {
                this.exportDashboardReport();
            }
        });
    }

    /**
 * 刷新仪表板
 */
    async refreshDashboard() {
        try {
            Utils.showSuccess('正在刷新数据...');

            // 重新获取聚合数据
            const sceneId = this.getSceneIdFromPage();
            if (sceneId) {
                this.aggregateData = await API.getSceneAggregate(sceneId, {
                    includeConversations: true,
                    includeStory: true,
                    includeUIState: true,
                    includeProgress: true,
                    conversationLimit: 50
                });

                // 重新渲染仪表板
                this.renderSceneDashboard();

                Utils.showSuccess('数据刷新完成');
            }
        } catch (error) {
            Utils.showError('刷新数据失败: ' + error.message);
        }
    }

    /**
 * 获取仪表板容器
 */
    getDashboardContainer() {
        let container = document.getElementById('scene-dashboard');
        if (!container) {
            container = document.createElement('div');
            container.id = 'scene-dashboard';
            container.className = 'scene-dashboard mt-4';

            // 插入到合适的位置
            const mainContent = document.querySelector('.container') ||
                document.querySelector('.scene-content') ||
                document.body;
            mainContent.appendChild(container);
        }
        return container;
    }

    /**
     * 销毁所有图表
     */
    destroyCharts() {
        this.charts.forEach(chart => {
            if (chart && typeof chart.destroy === 'function') {
                chart.destroy();
            }
        });
        this.charts.clear();
    }

    // ========================================
    // 场景创建功能
    // ========================================

    /**
     * 处理场景创建
     */
    async handleSceneCreation(form) {
        try {
            this.setState({ isLoading: true });

            const formData = new FormData(form);
            const data = {
                title: formData.get('scene_title') || '新场景',
                text: formData.get('scene_text')
            };

            // 验证数据
            if (!data.text || data.text.trim().length < 10) {
                throw new Error('场景文本至少需要10个字符');
            }

            Utils.showSuccess('正在创建场景...');

            // 使用分析API创建场景
            const result = await API.analyzeText({
                title: data.title,
                text: data.text
            });

            if (result.task_id) {
                // 订阅进度更新
                this.subscribeToProgress(result.task_id);
            }

        } catch (error) {
            Utils.showError('创建场景失败: ' + error.message);
            this.setState({ isLoading: false });
        }
    }

    /**
     * 订阅分析进度
     */
    subscribeToProgress(taskId) {
        const progressBar = document.getElementById('creation-progress');
        const statusText = document.getElementById('creation-status');

        if (progressBar) {
            progressBar.style.display = 'block';
        }

        const eventSource = API.subscribeProgress(
            taskId,
            // onProgress
            (data) => {
                if (progressBar) {
                    const progress = data.progress || 0;
                    progressBar.querySelector('.progress-bar').style.width = `${progress}%`;
                }

                if (statusText) {
                    statusText.textContent = data.message || '处理中...';
                }
            },
            // onError
            (error) => {
                Utils.showError('分析过程出错: ' + error.message);
                this.setState({ isLoading: false });
            },
            // onComplete
            (data) => {
                if (data.status === 'completed') {
                    Utils.showSuccess('场景创建成功！');

                    // 跳转到场景页面
                    if (data.scene_id) {
                        setTimeout(() => {
                            window.location.href = `/scenes/${data.scene_id}`;
                        }, 1000);
                    }
                } else {
                    Utils.showError('场景创建失败: ' + (data.message || '未知错误'));
                }

                this.setState({ isLoading: false });
            }
        );
    }

    /**
     * 处理文件上传
     */
    async handleFileUpload(file) {
        if (!file) return;

        try {
            Utils.showSuccess('正在上传文件...');

            const result = await API.uploadFile(file, (progress) => {
                // 更新上传进度
                const progressBar = document.getElementById('upload-progress');
                if (progressBar) {
                    progressBar.style.display = 'block';
                    progressBar.querySelector('.progress-bar').style.width = `${progress}%`;
                }
            });

            // 将文件内容填入文本框
            const textArea = document.getElementById('scene-text');
            if (textArea && result.content) {
                textArea.value = result.content;
                this.updateTextPreview(result.content);
            }

            Utils.showSuccess('文件上传成功');

        } catch (error) {
            Utils.showError('文件上传失败: ' + error.message);
        }
    }

    // ========================================
    // 设置管理功能
    // ========================================

    /**
     * 加载当前设置
     */
    async loadCurrentSettings() {
        try {
            const settings = await API.getSettings();

            // 填充表单
            if (settings.llm_provider) {
                const providerSelect = document.getElementById('llm-provider');
                if (providerSelect) {
                    providerSelect.value = settings.llm_provider;
                    // 加载对应的模型列表
                    await this.loadModelsForProvider(settings.llm_provider);
                }
            }

            if (settings.llm_config?.model) {
                const modelSelect = document.getElementById('model-name');
                if (modelSelect) {
                    modelSelect.value = settings.llm_config.model;
                }
            }

            if (settings.debug_mode !== undefined) {
                const debugCheck = document.getElementById('debug-mode');
                if (debugCheck) {
                    debugCheck.checked = settings.debug_mode;
                }
            }

            // 显示连接状态
            this.updateConnectionStatus();

        } catch (error) {
            console.error('加载设置失败:', error);
        }
    }

    /**
     * 加载指定提供商的模型列表
     */
    async loadModelsForProvider(provider) {
        if (!provider) return;

        try {
            const result = await API.getLLMModels(provider);
            const modelSelect = document.getElementById('model-name');

            if (modelSelect && result.models) {
                modelSelect.innerHTML = '<option value="">选择模型</option>';

                result.models.forEach(model => {
                    const option = document.createElement('option');
                    option.value = model;
                    option.textContent = model;
                    modelSelect.appendChild(option);
                });
            }
        } catch (error) {
            console.error('加载模型列表失败:', error);
        }
    }

    /**
     * 保存设置
     */
    async saveSettings() {
        try {
            const form = document.getElementById('settings-form');
            const formData = new FormData(form);

            const settings = {
                llm_provider: formData.get('llm_provider'),
                llm_config: {
                    api_key: formData.get('api_key'),
                    model: formData.get('model')
                },
                debug_mode: formData.get('debug_mode') === 'on'
            };

            await API.saveSettings(settings);
            Utils.showSuccess('设置保存成功');

            // 更新连接状态
            await this.updateConnectionStatus();

        } catch (error) {
            Utils.showError('保存设置失败: ' + error.message);
        }
    }

    /**
     * 测试连接
     */
    async testConnection() {
        try {
            Utils.showSuccess('正在测试连接...');

            const result = await API.testConnection();

            if (result.success) {
                Utils.showSuccess('连接测试成功');
                this.updateConnectionStatus('success');
            } else {
                Utils.showError('连接测试失败: ' + result.error);
                this.updateConnectionStatus('failed');
            }
        } catch (error) {
            Utils.showError('连接测试失败: ' + error.message);
            this.updateConnectionStatus('failed');
        }
    }

    /**
     * 更新连接状态显示
     */
    async updateConnectionStatus(status = null) {
        const statusEl = document.getElementById('connection-status');
        if (!statusEl) return;

        if (!status) {
            try {
                const llmStatus = await API.getLLMStatus();
                status = llmStatus.ready ? 'success' : 'failed';
            } catch {
                status = 'failed';
            }
        }

        const statusConfig = {
            success: { class: 'text-success', icon: 'check-circle', text: '连接正常' },
            failed: { class: 'text-danger', icon: 'x-circle', text: '连接失败' },
            testing: { class: 'text-warning', icon: 'clock', text: '测试中...' }
        };

        const config = statusConfig[status] || statusConfig.failed;

        statusEl.innerHTML = `
            <i class="bi bi-${config.icon} ${config.class}"></i>
            <span class="${config.class}">${config.text}</span>
        `;
    }

    // ========================================
    // 辅助工具方法
    // ========================================

    /**
     * 获取页面中的场景ID
     */
    getSceneIdFromPage() {
        // 尝试从多个地方获取场景ID
        const sceneIdEl = document.getElementById('scene-id');
        if (sceneIdEl) return sceneIdEl.value;

        const pathMatch = window.location.pathname.match(/\/scenes\/([^\/]+)/);
        if (pathMatch) return pathMatch[1];

        const metaEl = document.querySelector('meta[name="scene-id"]');
        if (metaEl) return metaEl.content;

        return null;
    }

    /**
     * 获取当前用户ID
     */
    getCurrentUserId() {
        // 这里应该从实际的用户认证系统获取
        return 'user_001'; // 临时硬编码
    }

    /**
     * 获取角色名称
     */
    getCharacterName(characterId) {
        const character = this.currentScene?.characters?.find(c => c.id === characterId);
        return character ? character.name : '未知角色';
    }

    /**
     * 获取角色头像
     */
    getCharacterAvatar(characterId) {
        const character = this.currentScene?.characters?.find(c => c.id === characterId);
        return character ? character.avatar : null;
    }

    /**
     * 设置应用状态
     */
    setState(newState) {
        this.state = { ...this.state, ...newState };
        this.updateLoadingState();
    }

    /**
     * 更新加载状态显示
     */
    updateLoadingState() {
        const loadingEl = document.getElementById('loading-indicator');
        if (loadingEl) {
            loadingEl.style.display = this.state.isLoading ? 'block' : 'none';
        }

        // 禁用/启用界面元素
        const interactiveElements = document.querySelectorAll('button, input, select, textarea');
        interactiveElements.forEach(el => {
            if (this.state.isLoading) {
                el.setAttribute('data-was-disabled', el.disabled);
                el.disabled = true;
            } else {
                const wasDisabled = el.getAttribute('data-was-disabled') === 'true';
                el.disabled = wasDisabled;
                el.removeAttribute('data-was-disabled');
            }
        });
    }

    /**
     * 设置输入状态
     */
    setInputState(enabled) {
        const messageInput = document.getElementById('message-input');
        const sendBtn = document.getElementById('send-btn');

        if (messageInput) {
            messageInput.disabled = !enabled;
        }

        if (sendBtn) {
            sendBtn.disabled = !enabled;
            sendBtn.innerHTML = enabled ?
                '<i class="bi bi-send"></i> 发送' :
                '<i class="bi bi-hourglass"></i> 发送中...';
        }
    }

    /**
     * 绑定场景页面事件
     */
    bindSceneEvents() {
        // 发送消息事件
        const sendBtn = document.getElementById('send-btn');
        const messageInput = document.getElementById('message-input');

        if (sendBtn) {
            sendBtn.addEventListener('click', () => this.sendMessage());
        }

        if (messageInput) {
            messageInput.addEventListener('keypress', (e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    this.sendMessage();
                }
            });
        }

        // 故事模式切换
        const storyModeToggle = document.getElementById('story-mode-toggle');
        if (storyModeToggle) {
            storyModeToggle.addEventListener('change', (e) => {
                this.setState({ storyMode: e.target.checked });
                this.updateInterfaceMode();
            });
        }

        // 互动模式切换
        const interactionModeToggle = document.getElementById('interaction-mode-toggle');
        if (interactionModeToggle) {
            interactionModeToggle.addEventListener('change', (e) => {
                this.setState({ interactionMode: e.target.checked });
                this.updateInterfaceMode();
            });
        }
    }

    /**
     * 更新界面模式
     */
    updateInterfaceMode() {
        const storyContainer = document.getElementById('story-container');
        const chatContainer = document.getElementById('chat-interface');

        if (storyContainer) {
            storyContainer.style.display = this.state.storyMode ? 'block' : 'none';
        }

        if (chatContainer) {
            chatContainer.classList.toggle('story-mode', this.state.storyMode);
            chatContainer.classList.toggle('interaction-mode', this.state.interactionMode);
        }
    }

    /**
     * 初始化调试模式
     */
    initDebugMode() {
        if (window.location.hostname === 'localhost' || window.location.search.includes('debug=1')) {
            window.app = this;
            window.appDebug = {
                getState: () => this.state,
                getCurrentScene: () => this.currentScene,
                getStoryData: () => this.storyData,
                getAggregateData: () => this.aggregateData,
                refreshDashboard: () => this.refreshDashboard(),
                exportReport: () => this.exportDashboardReport(),
                testAPI: () => API.healthCheck(),
                reloadScene: () => this.initScene(),
                clearCache: () => {
                    this.currentScene = null;
                    this.conversations = [];
                    this.storyData = null;
                }
            };
            console.log('🎭 SceneApp调试模式已启用');
            console.log('使用 window.appDebug 查看调试工具');
        }
    }

    // ========================================
    // 公共接口方法
    // ========================================

    /**
     * 查看角色详情
     */
    viewCharacterDetails(characterId) {
        const character = this.currentScene?.characters?.find(c => c.id === characterId);
        if (!character) return;

        // 显示角色详情模态框
        const modal = document.getElementById('character-details-modal');
        if (modal) {
            // 填充角色信息
            modal.querySelector('.character-name').textContent = character.name;
            modal.querySelector('.character-role').textContent = character.role || '角色';
            modal.querySelector('.character-description').textContent = character.description || '暂无描述';

            // 显示模态框
            const bsModal = new bootstrap.Modal(modal);
            bsModal.show();
        }
    }

    /**
     * 切换故事模式
     */
    toggleStoryMode() {
        this.setState({ storyMode: !this.state.storyMode });
        this.updateInterfaceMode();
    }

    /**
     * 切换互动模式
     */
    toggleInteractionMode() {
        this.setState({ interactionMode: !this.state.interactionMode });
        this.updateInterfaceMode();
    }

    /**
     * 导出场景数据
     */
    async exportSceneData(format = 'json') {
        try {
            const result = await API.exportSceneData(this.currentScene.id, format, true);

            // 创建下载链接
            const blob = new Blob([result.content], {
                type: format === 'json' ? 'application/json' : 'text/plain'
            });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `scene_${this.currentScene.id}.${format}`;
            a.click();
            URL.revokeObjectURL(url);

            Utils.showSuccess('场景数据导出成功');
        } catch (error) {
            Utils.showError('导出失败: ' + error.message);
        }
    }
}

// ========================================
// 全局初始化
// ========================================

// 创建全局应用实例
try {
    // 创建全局应用实例
    window.SceneApp = new SceneApp();
    window.app = window.SceneApp; // 简化引用
} catch (error) {
    console.error('❌ SceneApp 创建失败:', error);

    // 显示用户友好的错误信息
    document.addEventListener('DOMContentLoaded', function () {
        const body = document.body;
        if (body) {
            body.innerHTML = `
                <div class="container mt-5">
                    <div class="alert alert-danger" role="alert">
                        <h4 class="alert-heading">应用加载失败</h4>
                        <p><strong>错误原因:</strong> ${error.message}</p>
                        <hr>
                        <p class="mb-0">
                            请检查网络连接或<a href="javascript:location.reload()" class="alert-link">刷新页面</a>重试。
                        </p>
                    </div>
                </div>
            `;
        }
    });
}

// 根据页面类型自动初始化
document.addEventListener('DOMContentLoaded', function () {
    const path = window.location.pathname;

    try {
        if (path.includes('/scenes/create')) {
            window.app.initSceneCreate();
        } else if (path.includes('/settings')) {
            window.app.initSettings();
        } else if (path.includes('/users/') && path.includes('/profile')) {
            window.app.initUserProfile();
        } else if (path.match(/\/scenes\/[^\/]+$/)) {
            // 场景页面 - 延迟初始化，等待页面完全加载
            setTimeout(() => {
                window.app.initScene();
            }, 100);
        }

        console.log('✅ SceneApp 初始化完成');
    } catch (error) {
        console.error('❌ SceneApp 初始化失败:', error);
        Utils.showError('应用初始化失败，请刷新页面重试');
    }
});

// 页面可见性变化时的处理
document.addEventListener('visibilitychange', function () {
    if (document.visibilityState === 'visible' && window.app.state.sceneLoaded) {
        // 页面重新可见时，刷新数据
        setTimeout(() => {
            if (window.app.currentScene && window.app.state.dashboardVisible) {
                window.app.refreshDashboard();
            }
        }, 1000);
    }
});

// 错误处理
window.addEventListener('error', function (event) {
    console.error('全局错误:', event.error);
    if (window.app) {
        window.app.setState({ lastError: event.error.message });
    }
});

// 未处理的Promise拒绝
window.addEventListener('unhandledrejection', function (event) {
    console.error('未处理的Promise拒绝:', event.reason);
    Utils.showError('系统错误: ' + event.reason.message);
    event.preventDefault();
});

// 页面卸载时清理资源
window.addEventListener('beforeunload', function() {
    if (window.app) {
        window.app.destroyCharts();
    }
});

// ========================================
// CSS样式增强
// ========================================

// 添加仪表板样式
if (typeof document !== 'undefined') {
    const addDashboardStyles = () => {
        if (document.getElementById('dashboard-styles')) return;

        const style = document.createElement('style');
        style.id = 'dashboard-styles';
        style.textContent = `
            /* 仪表板容器样式 */
            .scene-dashboard {
                background: #f8f9fa;
                border-radius: 12px;
                padding: 24px;
                margin: 20px 0;
                border: 1px solid #e9ecef;
            }
            
            .dashboard-header {
                border-bottom: 2px solid #e9ecef;
                padding-bottom: 16px;
                margin-bottom: 24px;
            }
            
            /* 统计卡片样式 */
            .stat-card {
                transition: all 0.3s ease;
                border: 1px solid #e9ecef;
                border-radius: 12px;
            }
            
            .stat-card:hover {
                transform: translateY(-4px);
                box-shadow: 0 8px 25px rgba(0,0,0,0.1);
            }
            
            .stat-icon {
                opacity: 0.8;
            }
            
            .stat-value {
                font-weight: 700;
                color: #2c3e50;
                margin: 8px 0;
            }
            
            .stat-label {
                font-size: 0.9rem;
                font-weight: 500;
            }
            
            /* 图表卡片样式 */
            .chart-card {
                border: 1px solid #e9ecef;
                border-radius: 12px;
                transition: all 0.3s ease;
            }
            
            .chart-card:hover {
                box-shadow: 0 4px 12px rgba(0,0,0,0.1);
            }
            
            .chart-card .card-header {
                background: #ffffff;
                border-bottom: 1px solid #e9ecef;
                border-radius: 12px 12px 0 0;
            }
            
            /* 关系图样式 */
            #character-relationship-graph {
                background: #fafafa;
                border-radius: 8px;
                border: 1px solid #e9ecef;
            }
            
            /* 响应式设计 */
            @media (max-width: 768px) {
                .scene-dashboard {
                    padding: 16px;
                    margin: 10px 0;
                }
                
                .dashboard-header {
                    flex-direction: column;
                    gap: 12px;
                }
                
                .chart-card canvas {
                    height: 250px !important;
                }
                
                #character-relationship-graph {
                    height: 300px !important;
                }
            }
        `;
        
        document.head.appendChild(style);
        console.log('✅ Dashboard 样式已加载');
    };
    
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', addDashboardStyles);
    } else {
        addDashboardStyles();
    }
}
