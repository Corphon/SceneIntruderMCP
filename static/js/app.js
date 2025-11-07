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

            const aggregateData = await API.getSceneAggregate(sceneId, {
                includeConversations: true,      // 对应后端 include_conversations
                includeStory: true,              // 对应后端 include_story  
                includeUIState: true,            // 对应后端 include_ui_state
                includeProgress: true,           // 对应后端 include_progress
                includeCharacterStats: true,     // 新增参数
                conversationLimit: 50,           // 对应后端 conversation_limit
                timeRange: '7d',                 // 新增时间范围参数
                preferences: this.currentUser?.preferences // 对应后端 preferences
            });

            // 保存聚合数据供仪表板使用
            this.aggregateData = aggregateData;

            // 设置应用数据
            if (aggregateData.data) {
                // 如果有 data 包装
                this.currentScene = aggregateData.data.scene;
                this.conversations = aggregateData.data.conversations || [];
                this.storyData = aggregateData.data.story_data;
            } else {
                // 直接返回数据
                this.currentScene = aggregateData.scene;
                this.conversations = aggregateData.conversations || [];
                this.storyData = aggregateData.story_data;
            }

            // 验证必要数据
            if (!this.currentScene) {
                throw new Error('场景数据无效');
            }

            // 渲染界面
            this.renderSceneInterface();
            this.renderConversations();

            // 条件渲染故事界面和仪表板
            if (this.storyData) {
                this.renderStoryInterface();
            }

            if (aggregateData.stats || aggregateData.data?.stats) {
                // 初始化仪表板状态
                this.initDashboardState();

                // 如果仪表板应该显示，则渲染
                if (this.state.dashboardVisible) {
                    this.renderSceneDashboard();
                }
            }

            // 初始化核心功能（按依赖顺序）
            this.initSessionTimeManagement();   // 初始化会话时间管理          
            this.initUserActivityMonitoring();  // 初始化用户活动监控           
            this.initCharacterStatus();         // 初始化角色状态       
            this.initConversationUI();          // 初始化对话UI功能

            //🌐 初始化实时通信（在数据准备好后）
            if (this.currentScene) {
                await this.initRealtimeConnection(sceneId);
            }

            // 绑定事件
            this.bindSceneEvents();

            // 初始化故事通知系统
            this.initStoryNotificationSystem();

            // 初始化用户在线状态
            this.initOnlineUsersSystem();

            // 初始化滚动监听
            this.initScrollMonitoring();

            // 初始化统计更新
            this.updateConversationStats();

            this.setState({
                isLoading: false,
                sceneLoaded: true
            });

            // 记录初始活动
            this.updateLastActivity();

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
    * 初始化角色状态
    */
    initCharacterStatus() {
        // 初始化角色状态缓存
        this.characterStatusCache = new Map();

        // 获取所有角色元素
        const characterElements = document.querySelectorAll('[data-character-id]');

        // 为每个角色设置初始状态
        characterElements.forEach(element => {
            const characterId = element.dataset.characterId;

            // 设置默认状态
            this.updateCharacterStatusIndicator(element, 'offline', 'calm');
            this.updateCharacterCardStyle(element, 'offline');

            // 缓存初始状态
            this.characterStatusCache.set(characterId, {
                status: 'offline',
                mood: 'calm',
                timestamp: Date.now()
            });
        });

        console.log('✅ 角色状态已初始化');
    }

    /**
     * 初始化场景创建页面
     */
    initSceneCreate() {
        const form = document.getElementById('create-scene-form');
        if (!form) return;

        // 确保预览容器存在
        this.ensurePreviewContainer();

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

            // 初始化预览
            this.updateTextPreview(textArea.value);
        }

        // 绑定预览切换按钮
        const previewToggleBtn = document.getElementById('preview-toggle-btn');
        if (previewToggleBtn) {
            previewToggleBtn.addEventListener('click', () => {
                this.togglePreviewMode();
            });
        }
    }

    /**
    * 初始化故事通知系统
    */
    initStoryNotificationSystem() {
        // 初始化故事统计
        this.storyStats = {
            totalEvents: 0,
            eventTypes: {},
            lastEventTime: null
        };

        // 检查音效设置
        this.state.soundEnabled = localStorage.getItem('story_sounds_enabled') !== 'false';

        // 加载历史事件统计
        const events = this.getStoryEventHistory();
        if (events.length > 0) {
            this.storyStats.totalEvents = events.length;
            this.storyStats.lastEventTime = events[events.length - 1].timestamp;

            // 统计事件类型
            events.forEach(event => {
                this.storyStats.eventTypes[event.eventType] =
                    (this.storyStats.eventTypes[event.eventType] || 0) + 1;
            });
        }

        console.log('📖 故事通知系统已初始化');
    }

    /**
    * 初始化在线用户系统
    */
    initOnlineUsersSystem() {
        // 初始化在线用户列表
        this.onlineUsers = new Map();

        // 如果有实时管理器，监听用户状态
        if (this.realtimeManager) {
            this.realtimeManager.on('user:online', (data) => {
                this.onlineUsers.set(data.userId, {
                    username: data.username,
                    joinTime: Date.now(),
                    status: 'online'
                });
            });

            this.realtimeManager.on('user:offline', (data) => {
                this.onlineUsers.delete(data.userId);
            });
        }

        console.log('👥 在线用户系统已初始化');
    }

    /**
    * 确保预览容器存在
    */
    ensurePreviewContainer() {
        let previewContainer = document.getElementById('text-preview');

        if (!previewContainer) {
            // 如果预览容器不存在，创建一个
            const textArea = document.getElementById('scene-text');
            if (textArea && textArea.parentNode) {
                previewContainer = document.createElement('div');
                previewContainer.id = 'text-preview';
                previewContainer.className = 'text-preview-container mt-3 p-3 border rounded bg-light';

                // 插入到文本区域后面
                textArea.parentNode.insertBefore(previewContainer, textArea.nextSibling);

                // 初始化预览内容
                this.clearTextPreview();
            }
        }
    }

    /**
     * 更新文本预览
     */
    updateTextPreview(text) {
        const previewContainer = document.getElementById('text-preview');
        if (!previewContainer) return;

        // 如果文本为空，显示提示
        if (!text || text.trim().length === 0) {
            previewContainer.innerHTML = `
            <div class="text-muted text-center py-4">
                <i class="bi bi-file-text fs-1"></i>
                <p class="mt-2">开始输入文本以查看预览...</p>
            </div>
        `;
            return;
        }

        // 基本的文本分析和预览
        const analysis = this.analyzeText(text);

        previewContainer.innerHTML = `
        <div class="preview-header mb-3">
            <h6 class="mb-2">
                <i class="bi bi-eye"></i> 文本预览
                <span class="badge bg-secondary ms-2">${analysis.wordCount} 字</span>
            </h6>
        </div>
        
        <div class="preview-content">
            <!-- 文本摘要 -->
            <div class="preview-section mb-3">
                <h6 class="text-muted mb-2">内容摘要</h6>
                <div class="preview-summary">
                    ${this.generateTextSummary(text)}
                </div>
            </div>
            
            <!-- 检测到的实体 -->
            ${analysis.entities.length > 0 ? `
                <div class="preview-section mb-3">
                    <h6 class="text-muted mb-2">检测到的实体</h6>
                    <div class="entities-list">
                        ${analysis.entities.map(entity => `
                            <span class="badge bg-light text-dark me-1 mb-1" title="${entity.type}">
                                ${Utils.escapeHtml(entity.name)}
                            </span>
                        `).join('')}
                    </div>
                </div>
            ` : ''}
            
            <!-- 文本统计 -->
            <div class="preview-section mb-3">
                <h6 class="text-muted mb-2">文本统计</h6>
                <div class="row g-2">
                    <div class="col-6 col-md-3">
                        <div class="stat-item">
                            <div class="stat-value">${analysis.wordCount}</div>
                            <div class="stat-label">字数</div>
                        </div>
                    </div>
                    <div class="col-6 col-md-3">
                        <div class="stat-item">
                            <div class="stat-value">${analysis.sentenceCount}</div>
                            <div class="stat-label">句子数</div>
                        </div>
                    </div>
                    <div class="col-6 col-md-3">
                        <div class="stat-item">
                            <div class="stat-value">${analysis.paragraphCount}</div>
                            <div class="stat-label">段落数</div>
                        </div>
                    </div>
                    <div class="col-6 col-md-3">
                        <div class="stat-item">
                            <div class="stat-value">${analysis.readingTime}分钟</div>
                            <div class="stat-label">阅读时间</div>
                        </div>
                    </div>
                </div>
            </div>
            
            <!-- 格式化的文本内容 -->
            <div class="preview-section">
                <h6 class="text-muted mb-2">格式化预览</h6>
                <div class="formatted-text">
                    ${this.formatTextForPreview(text)}
                </div>
            </div>
        </div>
    `;

        // 更新创建按钮状态
        this.updateCreateButtonState(analysis);
    }

    /**
     * 分析文本内容
     */
    analyzeText(text) {
        if (!text) return { wordCount: 0, sentenceCount: 0, paragraphCount: 0, entities: [], readingTime: 0 };

        // 基本统计
        const wordCount = text.length; // 中文字符数
        const sentenceCount = (text.match(/[。！？.!?]/g) || []).length;
        const paragraphCount = text.split(/\n\s*\n/).filter(p => p.trim().length > 0).length;
        const readingTime = Math.ceil(wordCount / 300); // 假设每分钟阅读300字

        // 简单的实体检测（人名、地名等）
        const entities = this.extractEntities(text);

        return {
            wordCount,
            sentenceCount,
            paragraphCount,
            entities,
            readingTime
        };
    }

    /**
     * 提取实体（简单版本）
     */
    extractEntities(text) {
        const entities = [];

        // 简单的人名检测（以常见姓氏开头的2-4字词组）
        const namePattern = /[王李张刘陈杨黄赵吴周徐孙马朱胡郭何高林罗郑梁谢宋唐许韩冯邓曹彭曾萧田董袁潘于蒋蔡余杜叶程苏魏吕丁任沈姚卢姜崔钟谭陆汪范金石廖贾夏韦付方白邹孟熊秦邱江尹薛闫段雷侯龙史陶黎贺顾毛郝龚邵万钱严覃武戴莫孔向汤][a-zA-Z\u4e00-\u9fa5]{1,3}/g;
        const names = text.match(namePattern) || [];
        names.forEach(name => {
            if (!entities.find(e => e.name === name)) {
                entities.push({ name, type: '人名' });
            }
        });

        // 简单的地名检测（以常见地名词尾结尾）
        const placePattern = /[a-zA-Z\u4e00-\u9fa5]{2,}(?:市|县|区|镇|村|路|街|巷|山|河|湖|海|岛|省|州|国)/g;
        const places = text.match(placePattern) || [];
        places.forEach(place => {
            if (!entities.find(e => e.name === place)) {
                entities.push({ name: place, type: '地名' });
            }
        });

        // 组织机构检测
        const orgPattern = /[a-zA-Z\u4e00-\u9fa5]{2,}(?:公司|集团|企业|学校|大学|医院|银行|政府|部门|组织|协会|基金会)/g;
        const orgs = text.match(orgPattern) || [];
        orgs.forEach(org => {
            if (!entities.find(e => e.name === org)) {
                entities.push({ name: org, type: '机构' });
            }
        });

        return entities.slice(0, 20); // 限制显示数量
    }

    /**
     * 生成文本摘要
     */
    generateTextSummary(text) {
        if (!text || text.length < 50) {
            return '<span class="text-muted">文本过短，无法生成摘要</span>';
        }

        // 简单的摘要生成：取前100字并添加省略号
        const summary = text.substring(0, 100).trim();
        return `
        <div class="summary-text">
            ${Utils.escapeHtml(summary)}${text.length > 100 ? '...' : ''}
        </div>
    `;
    }

    /**
     * 格式化文本用于预览
     */
    formatTextForPreview(text) {
        if (!text) return '';

        // 将文本按段落分割并格式化
        const paragraphs = text.split(/\n\s*\n/).filter(p => p.trim().length > 0);

        return paragraphs.map(paragraph => {
            // 转义HTML并保留换行
            const escaped = Utils.escapeHtml(paragraph.trim());
            const withBreaks = escaped.replace(/\n/g, '<br>');

            return `<p class="mb-3">${withBreaks}</p>`;
        }).join('');
    }

    /**
     * 更新创建按钮状态
     */
    updateCreateButtonState(analysis) {
        const createBtn = document.getElementById('create-scene-btn');
        if (!createBtn) return;

        const isValid = analysis.wordCount >= 10; // 至少10个字符

        createBtn.disabled = !isValid;

        if (isValid) {
            createBtn.innerHTML = '<i class="bi bi-plus-circle"></i> 创建场景';
            createBtn.className = 'btn btn-primary';
        } else {
            createBtn.innerHTML = '<i class="bi bi-exclamation-triangle"></i> 文本过短';
            createBtn.className = 'btn btn-secondary';
        }
    }

    /**
     * 清空预览
     */
    clearTextPreview() {
        const previewContainer = document.getElementById('text-preview');
        if (previewContainer) {
            previewContainer.innerHTML = `
            <div class="text-muted text-center py-4">
                <i class="bi bi-file-text fs-1"></i>
                <p class="mt-2">开始输入文本以查看预览...</p>
            </div>
        `;
        }
    }

    /**
     * 高亮预览中的关键词
     */
    highlightKeywords(text, keywords) {
        return Utils.highlightKeywords(text, keywords, 'keyword-highlight');
    }

    /**
     * 预览模式切换
     */
    togglePreviewMode() {
        const previewContainer = document.getElementById('text-preview');
        const toggleBtn = document.getElementById('preview-toggle-btn');

        if (!previewContainer || !toggleBtn) return;

        const isHidden = previewContainer.style.display === 'none';

        previewContainer.style.display = isHidden ? 'block' : 'none';
        toggleBtn.innerHTML = isHidden ?
            '<i class="bi bi-eye-slash"></i> 隐藏预览' :
            '<i class="bi bi-eye"></i> 显示预览';
    }

    /**
     * 初始化设置页面
     */
    initSettings() {
        const form = document.getElementById('settings-form');
        if (!form) return;

        // 加载LLM提供商列表
        this.loadLLMProviders();

        // 加载当前设置
        this.loadCurrentSettings();

        // 初始化工具提示
        this.initTooltips();

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
                // Clear model input when provider changes
                const modelSelect = document.getElementById('model-select');
                const modelNameInput = document.getElementById('model-name');
                
                if (modelSelect) {
                    modelSelect.innerHTML = '<option value="">加载中...</option>';
                    modelSelect.disabled = true;
                }
                
                if (modelNameInput) {
                    modelNameInput.value = '';
                    modelNameInput.disabled = true;
                }
                
                // Load models for the selected provider
                this.loadModelsForProvider(e.target.value);
            });
        }
        
        // 绑定刷新模型按钮
        const refreshBtn = document.getElementById('refresh-models');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', async () => {
                const providerSelect = document.getElementById('llm-provider');
                if (providerSelect && providerSelect.value) {
                    await this.loadModelsForProvider(providerSelect.value);
                } else {
                    Utils.showInfo('请先选择一个提供商');
                }
            });
        }
        
        // 绑定模型选择和手动输入的联动
        const modelSelect = document.getElementById('model-select');
        const modelNameInput = document.getElementById('model-name');
        
        if (modelSelect) {
            modelSelect.addEventListener('change', () => {
                // 当选择下拉框时，清空手动输入框
                if (modelNameInput) {
                    modelNameInput.value = '';
                }
            });
        }
        
        if (modelNameInput) {
            modelNameInput.addEventListener('input', () => {
                // 当手动输入时，清空下拉框选择
                if (modelSelect) {
                    modelSelect.value = '';
                }
            });
        }
    }

    /**
     * 初始化工具提示
     */
    initTooltips() {
        // 检查Bootstrap是否可用
        if (typeof bootstrap !== 'undefined' && bootstrap.Tooltip) {
            // 初始化所有带data-bs-toggle="tooltip"属性的元素
            const tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
            tooltipTriggerList.map(function (tooltipTriggerEl) {
                return new bootstrap.Tooltip(tooltipTriggerEl);
            });
        } else {
            // 如果Bootstrap不可用，使用简单的原生提示
            const tooltipElements = document.querySelectorAll('[title]');
            tooltipElements.forEach((element) => {
                element.addEventListener('mouseenter', (e) => {
                    // 简单的原生提示实现
                    const title = element.getAttribute('title');
                    if (title) {
                        element.setAttribute('data-original-title', title);
                        element.removeAttribute('title');
                    }
                });
            });
        }
    }

    /**
    * 更新用户档案
    */
    async updateUserProfile(userId) {
        try {
            const form = document.getElementById('profile-form');
            const formData = new FormData(form);

            const profileData = {
                name: formData.get('name'),
                email: formData.get('email'),
                bio: formData.get('bio'),
                avatar: formData.get('avatar')
            };

            await API.updateUserProfile(userId, profileData);
            Utils.showSuccess('用户档案更新成功');

            // 重新加载用户数据
            await this.loadUserProfile(userId);

        } catch (error) {
            Utils.showError('更新用户档案失败: ' + error.message);
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
    // 技能管理
    // ========================================

    // 在 app.js 的用户管理功能部分添加这个方法

    /**
     * 初始化技能管理
     */
    initSkillsManagement(userId) {
        // 绑定添加技能按钮
        const addSkillBtn = document.getElementById('add-skill-btn');
        if (addSkillBtn) {
            addSkillBtn.addEventListener('click', () => {
                this.showAddSkillModal(userId);
            });
        }

        // 绑定技能筛选功能
        const skillCategoryFilter = document.getElementById('skill-category-filter');
        if (skillCategoryFilter) {
            skillCategoryFilter.addEventListener('change', () => {
                this.filterUserSkills();
            });
        }

        const skillSearchInput = document.getElementById('skill-search');
        if (skillSearchInput) {
            skillSearchInput.addEventListener('input', () => {
                this.filterUserSkills();
            });
        }

        // 加载现有技能
        this.loadUserSkills(userId);
    }

    /**
     * 加载用户技能
     */
    async loadUserSkills(userId) {
        try {
            const skills = await API.getUserSkills(userId);
            this.userSkills = skills || [];
            this.renderUserSkills(skills);
        } catch (error) {
            console.error('加载技能失败:', error);
            Utils.showError('加载技能失败: ' + error.message);
        }
    }

    /**
     * 渲染用户技能
     */
    renderUserSkills(skills) {
        const container = document.getElementById('user-skills-container');
        if (!container) return;

        container.innerHTML = '';

        if (!skills || skills.length === 0) {
            container.innerHTML = `
            <div class="text-center text-muted py-4">
                <i class="bi bi-lightning fs-1"></i>
                <p class="mt-2">还没有技能<br>添加一些技能来增强你的能力吧！</p>
                <button class="btn btn-primary mt-2" onclick="app.showAddSkillModal('${this.getCurrentUserId()}')">
                    <i class="bi bi-plus-circle"></i> 添加第一个技能
                </button>
            </div>
        `;
            return;
        }

        skills.forEach(skill => {
            const skillEl = document.createElement('div');
            skillEl.className = 'col-md-6 col-lg-4 mb-3';

            const categoryIcon = this.getSkillCategoryIcon(skill.category);
            const categoryLabel = this.getSkillCategoryLabel(skill.category);

            skillEl.innerHTML = `
            <div class="card skill-card h-100">
                <div class="card-body">
                    <div class="skill-header d-flex justify-content-between align-items-start mb-2">
                        <h6 class="card-title mb-0">${Utils.escapeHtml(skill.name)}</h6>
                        <span class="skill-category-icon" title="${categoryLabel}">${categoryIcon}</span>
                    </div>
                    
                    <p class="card-text small text-muted mb-3">
                        ${Utils.escapeHtml(skill.description || '无描述')}
                    </p>
                    
                    <div class="skill-meta mb-3">
                        <div class="row g-2">
                            <div class="col-6">
                                <small class="text-muted">类别:</small>
                                <div class="fw-medium">${categoryLabel}</div>
                            </div>
                            <div class="col-6">
                                <small class="text-muted">冷却:</small>
                                <div class="fw-medium">${this.formatCooldown(skill.cooldown)}</div>
                            </div>
                        </div>
                        
                        ${skill.mana_cost ? `
                            <div class="mt-2">
                                <small class="text-muted">法力消耗:</small>
                                <span class="badge bg-info ms-1">${skill.mana_cost}</span>
                            </div>
                        ` : ''}
                        
                        ${skill.requirements && skill.requirements.length > 0 ? `
                            <div class="mt-2">
                                <small class="text-muted">需求:</small>
                                <div class="skill-requirements">
                                    ${skill.requirements.map(req => `
                                        <span class="badge bg-secondary me-1">${Utils.escapeHtml(req)}</span>
                                    `).join('')}
                                </div>
                            </div>
                        ` : ''}
                    </div>
                    
                    ${skill.effects && skill.effects.length > 0 ? `
                        <div class="skill-effects mb-3">
                            <small class="text-muted">效果:</small>
                            <div class="effects-list">
                                ${skill.effects.map(effect => `
                                    <div class="effect-item small">
                                        <i class="bi bi-star"></i>
                                        ${this.formatSkillEffect(effect)}
                                    </div>
                                `).join('')}
                            </div>
                        </div>
                    ` : ''}
                    
                    <div class="skill-actions">
                        <button class="btn btn-sm btn-outline-primary me-1" onclick="app.editSkill('${skill.id}')">
                            <i class="bi bi-pencil"></i> 编辑
                        </button>
                        <button class="btn btn-sm btn-outline-success me-1" onclick="app.useSkill('${skill.id}')">
                            <i class="bi bi-lightning"></i> 使用
                        </button>
                        <button class="btn btn-sm btn-outline-danger" onclick="app.deleteSkill('${skill.id}')">
                            <i class="bi bi-trash"></i> 删除
                        </button>
                    </div>
                </div>
                
                <div class="card-footer bg-transparent">
                    <small class="text-muted">
                        创建于 ${Utils.formatTime(skill.created)}
                    </small>
                </div>
            </div>
        `;

            container.appendChild(skillEl);
        });
    }

    /**
     * 显示添加技能模态框
     */
    showAddSkillModal(userId) {
        // 创建模态框HTML
        const modalHtml = `
        <div class="modal fade" id="addSkillModal" tabindex="-1">
            <div class="modal-dialog modal-lg">
                <div class="modal-content">
                    <div class="modal-header">
                        <h5 class="modal-title">
                            <i class="bi bi-lightning"></i> 添加新技能
                        </h5>
                        <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                    </div>
                    <div class="modal-body">
                        <form id="addSkillForm">
                            <div class="row g-3">
                                <div class="col-md-8">
                                    <label for="skillName" class="form-label">技能名称 *</label>
                                    <input type="text" class="form-control" id="skillName" required>
                                </div>
                                <div class="col-md-4">
                                    <label for="skillCategory" class="form-label">类别 *</label>
                                    <select class="form-select" id="skillCategory" required>
                                        <option value="">选择类别</option>
                                        <option value="combat">战斗</option>
                                        <option value="magic">魔法</option>
                                        <option value="social">社交</option>
                                        <option value="mental">心理</option>
                                        <option value="physical">物理</option>
                                        <option value="crafting">制作</option>
                                        <option value="survival">生存</option>
                                        <option value="other">其他</option>
                                    </select>
                                </div>
                                
                                <div class="col-12">
                                    <label for="skillDescription" class="form-label">技能描述</label>
                                    <textarea class="form-control" id="skillDescription" rows="3" 
                                              placeholder="描述这个技能的作用和效果..."></textarea>
                                </div>
                                
                                <div class="col-md-6">
                                    <label for="skillCooldown" class="form-label">冷却时间 (秒)</label>
                                    <input type="number" class="form-control" id="skillCooldown" min="0" value="0">
                                </div>
                                <div class="col-md-6">
                                    <label for="skillManaCost" class="form-label">法力消耗</label>
                                    <input type="number" class="form-control" id="skillManaCost" min="0" value="0">
                                </div>
                                
                                <div class="col-12">
                                    <label class="form-label">技能效果</label>
                                    <div id="skillEffects">
                                        <div class="effect-item row g-2 mb-2">
                                            <div class="col-3">
                                                <select class="form-control form-control-sm effect-target">
                                                    <option value="self">自己</option>
                                                    <option value="other">他人</option>
                                                    <option value="area">范围</option>
                                                </select>
                                            </div>
                                            <div class="col-3">
                                                <select class="form-control form-control-sm effect-type">
                                                    <option value="heal">治疗</option>
                                                    <option value="damage">伤害</option>
                                                    <option value="buff">增益</option>
                                                    <option value="debuff">减益</option>
                                                    <option value="emotion_reveal">情感揭示</option>
                                                    <option value="mind_read">读心</option>
                                                    <option value="other">其他</option>
                                                </select>
                                            </div>
                                            <div class="col-2">
                                                <input type="number" class="form-control form-control-sm effect-value" 
                                                       placeholder="数值" required>
                                            </div>
                                            <div class="col-3">
                                                <input type="number" class="form-control form-control-sm effect-probability" 
                                                       placeholder="概率(0-1)" min="0" max="1" step="0.1" value="1" required>
                                            </div>
                                            <div class="col-1">
                                                <button type="button" class="btn btn-outline-danger btn-sm remove-effect-btn">
                                                    <i class="bi bi-x"></i>
                                                </button>
                                            </div>
                                        </div>
                                    </div>
                                    <button type="button" class="btn btn-sm btn-outline-success" onclick="app.addSkillEffectRow()">
                                        <i class="bi bi-plus"></i> 添加效果
                                    </button>
                                </div>
                                
                                <div class="col-12">
                                    <label class="form-label">使用需求</label>
                                    <div id="skillRequirements">
                                        <div class="requirement-item row g-2 mb-2">
                                            <div class="col-10">
                                                <input type="text" class="form-control form-control-sm requirement-text" 
                                                       placeholder="例如: mana >= 10, target_distance <= 5">
                                            </div>
                                            <div class="col-2">
                                                <button type="button" class="btn btn-outline-danger btn-sm remove-requirement-btn">
                                                    <i class="bi bi-x"></i>
                                                </button>
                                            </div>
                                        </div>
                                    </div>
                                    <button type="button" class="btn btn-sm btn-outline-success" onclick="app.addRequirementRow()">
                                        <i class="bi bi-plus"></i> 添加需求
                                    </button>
                                </div>
                            </div>
                        </form>
                    </div>
                    <div class="modal-footer">
                        <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
                        <button type="button" class="btn btn-primary save-skill-btn" onclick="app.saveNewSkill('${userId}')">
                            <i class="bi bi-lightning"></i> 保存技能
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `;

        // 移除已存在的模态框
        const existingModal = document.getElementById('addSkillModal');
        if (existingModal) {
            existingModal.remove();
        }

        // 添加新模态框
        document.body.insertAdjacentHTML('beforeend', modalHtml);

        // 绑定删除按钮事件
        this.bindSkillModalEvents();

        // 显示模态框
        if (typeof bootstrap !== 'undefined') {
            const modal = new bootstrap.Modal(document.getElementById('addSkillModal'));
            modal.show();
        }
    }

    /**
     * 绑定技能模态框事件
     */
    bindSkillModalEvents() {
        // 绑定删除效果按钮
        document.addEventListener('click', (e) => {
            if (e.target.matches('.remove-effect-btn') || e.target.closest('.remove-effect-btn')) {
                const row = e.target.closest('.effect-item');
                if (row && document.querySelectorAll('.effect-item').length > 1) {
                    row.remove();
                }
            }

            if (e.target.matches('.remove-requirement-btn') || e.target.closest('.remove-requirement-btn')) {
                const row = e.target.closest('.requirement-item');
                if (row) {
                    row.remove();
                }
            }
        });
    }

    /**
     * 添加技能效果行
     */
    addSkillEffectRow() {
        const container = document.getElementById('skillEffects');
        if (!container) return;

        const effectRow = document.createElement('div');
        effectRow.className = 'effect-item row g-2 mb-2';
        effectRow.innerHTML = `
        <div class="col-3">
            <select class="form-control form-control-sm effect-target">
                <option value="self">自己</option>
                <option value="other">他人</option>
                <option value="area">范围</option>
            </select>
        </div>
        <div class="col-3">
            <select class="form-control form-control-sm effect-type">
                <option value="heal">治疗</option>
                <option value="damage">伤害</option>
                <option value="buff">增益</option>
                <option value="debuff">减益</option>
                <option value="emotion_reveal">情感揭示</option>
                <option value="mind_read">读心</option>
                <option value="other">其他</option>
            </select>
        </div>
        <div class="col-2">
            <input type="number" class="form-control form-control-sm effect-value" 
                   placeholder="数值" required>
        </div>
        <div class="col-3">
            <input type="number" class="form-control form-control-sm effect-probability" 
                   placeholder="概率(0-1)" min="0" max="1" step="0.1" value="1" required>
        </div>
        <div class="col-1">
            <button type="button" class="btn btn-outline-danger btn-sm remove-effect-btn">
                <i class="bi bi-x"></i>
            </button>
        </div>
    `;

        container.appendChild(effectRow);
    }

    /**
     * 添加需求行
     */
    addRequirementRow() {
        const container = document.getElementById('skillRequirements');
        if (!container) return;

        const requirementRow = document.createElement('div');
        requirementRow.className = 'requirement-item row g-2 mb-2';
        requirementRow.innerHTML = `
        <div class="col-10">
            <input type="text" class="form-control form-control-sm requirement-text" 
                   placeholder="例如: mana >= 10, target_distance <= 5">
        </div>
        <div class="col-2">
            <button type="button" class="btn btn-outline-danger btn-sm remove-requirement-btn">
                <i class="bi bi-x"></i>
            </button>
        </div>
    `;

        container.appendChild(requirementRow);
    }

    /**
     * 保存新技能
     */
    async saveNewSkill(userId) {
        try {
            const form = document.getElementById('addSkillForm');
            if (!form) return;

            // 禁用保存按钮
            const saveBtn = document.querySelector('.save-skill-btn');
            if (saveBtn) {
                saveBtn.disabled = true;
                saveBtn.innerHTML = '<i class="bi bi-hourglass"></i> 保存中...';
            }

            // 收集表单数据
            const skillData = {
                name: document.getElementById('skillName').value.trim(),
                description: document.getElementById('skillDescription').value.trim(),
                category: document.getElementById('skillCategory').value,
                cooldown: parseInt(document.getElementById('skillCooldown').value) || 0,
                mana_cost: parseInt(document.getElementById('skillManaCost').value) || 0,
                effects: [],
                requirements: []
            };

            // 验证必填字段
            if (!skillData.name || !skillData.category) {
                throw new Error('请填写技能名称和类别');
            }

            // 收集效果数据
            const effectElements = document.querySelectorAll('.effect-item');
            effectElements.forEach(effectEl => {
                const target = effectEl.querySelector('.effect-target').value;
                const type = effectEl.querySelector('.effect-type').value;
                const value = parseFloat(effectEl.querySelector('.effect-value').value);
                const probability = parseFloat(effectEl.querySelector('.effect-probability').value);

                if (target && type && !isNaN(value) && !isNaN(probability)) {
                    skillData.effects.push({
                        target,
                        type,
                        value,
                        probability
                    });
                }
            });

            // 收集需求数据
            const requirementElements = document.querySelectorAll('.requirement-text');
            requirementElements.forEach(reqEl => {
                const requirement = reqEl.value.trim();
                if (requirement) {
                    skillData.requirements.push(requirement);
                }
            });

            // 调用API保存技能
            await API.addUserSkill(userId, skillData);

            // 关闭模态框
            const modal = bootstrap.Modal.getInstance(document.getElementById('addSkillModal'));
            if (modal) {
                modal.hide();
            }

            // 重新加载技能列表
            await this.loadUserSkills(userId);

            Utils.showSuccess('技能添加成功！');

        } catch (error) {
            console.error('保存技能失败:', error);
            Utils.showError('保存技能失败: ' + error.message);
        } finally {
            // 恢复保存按钮
            const saveBtn = document.querySelector('.save-skill-btn');
            if (saveBtn) {
                saveBtn.disabled = false;
                saveBtn.innerHTML = '<i class="bi bi-lightning"></i> 保存技能';
            }
        }
    }

    /**
     * 编辑技能
     */
    async editSkill(skillId) {
        try {
            const skill = this.userSkills?.find(s => s.id === skillId);
            if (!skill) {
                Utils.showError('未找到指定技能');
                return;
            }

            // 这里可以复用添加技能的模态框，但填充现有数据
            this.showAddSkillModal(this.getCurrentUserId());

            // 等待模态框渲染完成后填充数据
            setTimeout(() => {
                this.populateSkillForm(skill);
            }, 200);

        } catch (error) {
            console.error('编辑技能失败:', error);
            Utils.showError('编辑技能失败: ' + error.message);
        }
    }

    /**
     * 填充技能表单
     */
    populateSkillForm(skill) {
        document.getElementById('skillName').value = skill.name || '';
        document.getElementById('skillDescription').value = skill.description || '';
        document.getElementById('skillCategory').value = skill.category || '';
        document.getElementById('skillCooldown').value = skill.cooldown || 0;
        document.getElementById('skillManaCost').value = skill.mana_cost || 0;

        // 填充效果
        const effectsContainer = document.getElementById('skillEffects');
        if (effectsContainer && skill.effects && skill.effects.length > 0) {
            effectsContainer.innerHTML = '';
            skill.effects.forEach(effect => {
                this.addSkillEffectRow();
                const lastEffect = effectsContainer.lastElementChild;
                if (lastEffect) {
                    lastEffect.querySelector('.effect-target').value = effect.target || 'self';
                    lastEffect.querySelector('.effect-type').value = effect.type || 'other';
                    lastEffect.querySelector('.effect-value').value = effect.value || '';
                    lastEffect.querySelector('.effect-probability').value = effect.probability || 1;
                }
            });
        }

        // 填充需求
        const requirementsContainer = document.getElementById('skillRequirements');
        if (requirementsContainer && skill.requirements && skill.requirements.length > 0) {
            requirementsContainer.innerHTML = '';
            skill.requirements.forEach(requirement => {
                this.addRequirementRow();
                const lastRequirement = requirementsContainer.lastElementChild;
                if (lastRequirement) {
                    lastRequirement.querySelector('.requirement-text').value = requirement;
                }
            });
        }
    }

    /**
     * 使用技能
     */
    async useSkill(skillId) {
        try {
            const skill = this.userSkills?.find(s => s.id === skillId);
            if (!skill) {
                Utils.showError('未找到指定技能');
                return;
            }

            Utils.showInfo(`使用技能: ${skill.name}`);

            // 这里可以集成到场景交互中
            if (this.currentScene && this.selectedCharacter) {
                // 将技能使用作为特殊交互发送
                const response = await API.createInteraction({
                    scene_id: this.currentScene.id,
                    character_id: this.selectedCharacter,
                    message: `使用技能: ${skill.name}`,
                    interaction_type: 'skill_use',
                    context: {
                        skill_id: skillId,
                        skill_data: skill
                    }
                });

                if (response.success || response.data) {
                    Utils.showSuccess(`${skill.name} 使用成功！`);
                }
            } else {
                Utils.showInfo(`模拟使用技能: ${skill.name}`);
            }

        } catch (error) {
            console.error('使用技能失败:', error);
            Utils.showError('使用技能失败: ' + error.message);
        }
    }

    /**
     * 删除技能
     */
    async deleteSkill(skillId) {
        try {
            const skill = this.userSkills?.find(s => s.id === skillId);
            if (!skill) {
                Utils.showError('未找到指定技能');
                return;
            }

            const confirmed = confirm(`确定要删除技能 "${skill.name}" 吗？`);
            if (!confirmed) return;

            await API.deleteUserSkill(this.getCurrentUserId(), skillId);

            // 重新加载技能列表
            await this.loadUserSkills(this.getCurrentUserId());

            Utils.showSuccess('技能删除成功');

        } catch (error) {
            console.error('删除技能失败:', error);
            Utils.showError('删除技能失败: ' + error.message);
        }
    }

    /**
     * 筛选用户技能
     */
    filterUserSkills() {
        const categoryFilter = document.getElementById('skill-category-filter')?.value || '';
        const searchFilter = document.getElementById('skill-search')?.value.toLowerCase() || '';

        const filteredSkills = this.userSkills.filter(skill => {
            const categoryMatch = !categoryFilter || skill.category === categoryFilter;
            const searchMatch = !searchFilter ||
                skill.name.toLowerCase().includes(searchFilter) ||
                (skill.description && skill.description.toLowerCase().includes(searchFilter));

            return categoryMatch && searchMatch;
        });

        this.renderUserSkills(filteredSkills);
    }

    /**
     * 获取技能类别图标
     */
    getSkillCategoryIcon(category) {
        const icons = {
            'combat': '⚔️',
            'magic': '🔮',
            'social': '💬',
            'mental': '🧠',
            'physical': '💪',
            'crafting': '🔨',
            'survival': '🏕️',
            'other': '⚡'
        };
        return icons[category] || '⚡';
    }

    /**
     * 获取技能类别标签
     */
    getSkillCategoryLabel(category) {
        const labels = {
            'combat': '战斗',
            'magic': '魔法',
            'social': '社交',
            'mental': '心理',
            'physical': '物理',
            'crafting': '制作',
            'survival': '生存',
            'other': '其他'
        };
        return labels[category] || category;
    }

    /**
     * 格式化冷却时间
     */
    formatCooldown(cooldown) {
        if (!cooldown || cooldown === 0) return '无';

        if (cooldown < 60) {
            return `${cooldown}秒`;
        } else if (cooldown < 3600) {
            return `${Math.floor(cooldown / 60)}分钟`;
        } else {
            return `${Math.floor(cooldown / 3600)}小时`;
        }
    }

    /**
     * 格式化技能效果
     */
    formatSkillEffect(effect) {
        const targetLabels = {
            'self': '自己',
            'other': '他人',
            'area': '范围'
        };

        const typeLabels = {
            'heal': '治疗',
            'damage': '伤害',
            'buff': '增益',
            'debuff': '减益',
            'emotion_reveal': '情感揭示',
            'mind_read': '读心',
            'other': '其他'
        };

        const target = targetLabels[effect.target] || effect.target;
        const type = typeLabels[effect.type] || effect.type;
        const probability = effect.probability < 1 ? ` (${Math.round(effect.probability * 100)}%几率)` : '';

        return `对${target}造成${effect.value}点${type}${probability}`;
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

    // 在 app.js 的场景交互核心功能部分添加这个方法

    /**
     * 显示选中角色的详细信息
     */
    displaySelectedCharacterInfo(characterId) {
        if (!characterId) {
            this.clearCharacterInfo();
            return;
        }

        // 查找角色信息
        const character = this.findCharacterById(characterId);
        if (!character) {
            console.warn('未找到角色信息:', characterId);
            this.clearCharacterInfo();
            return;
        }

        // 获取角色信息显示容器
        const infoContainer = document.getElementById('character-info');
        if (!infoContainer) {
            console.warn('未找到角色信息容器 #character-info');
            return;
        }

        // 渲染角色详细信息
        infoContainer.innerHTML = `
        <div class="character-info-card">
            <div class="character-header mb-3">
                <div class="d-flex align-items-center">
                    <div class="character-avatar me-3">
                        ${character.avatar ?
                `<img src="${character.avatar}" alt="${character.name}" class="rounded-circle" width="64" height="64">` :
                `<div class="avatar-placeholder rounded-circle bg-primary text-white d-flex align-items-center justify-content-center" style="width: 64px; height: 64px; font-size: 1.5rem;">${character.name[0]}</div>`
            }
                    </div>
                    <div class="character-basic-info">
                        <h5 class="mb-1">${Utils.escapeHtml(character.name)}</h5>
                        <div class="character-role text-muted mb-1">${Utils.escapeHtml(character.role || '角色')}</div>
                        <div class="character-status">
                            <span class="badge bg-success">在线</span>
                            ${this.getCharacterMoodBadge(character)}
                        </div>
                    </div>
                </div>
            </div>

            <div class="character-details">
                <!-- 基本信息 -->
                <div class="info-section mb-3">
                    <h6 class="section-title">
                        <i class="bi bi-person"></i> 基本信息
                    </h6>
                    <div class="info-content">
                        ${character.description ? `
                            <div class="mb-2">
                                <strong>描述：</strong>
                                <div class="character-description">${Utils.escapeHtml(character.description)}</div>
                            </div>
                        ` : ''}
                        
                        ${character.age ? `
                            <div class="mb-2">
                                <strong>年龄：</strong> ${Utils.escapeHtml(character.age)}
                            </div>
                        ` : ''}
                        
                        ${character.background ? `
                            <div class="mb-2">
                                <strong>背景：</strong>
                                <div class="character-background">${Utils.escapeHtml(character.background)}</div>
                            </div>
                        ` : ''}
                    </div>
                </div>

                <!-- 性格特征 -->
                ${character.personality ? `
                    <div class="info-section mb-3">
                        <h6 class="section-title">
                            <i class="bi bi-heart"></i> 性格特征
                        </h6>
                        <div class="info-content">
                            <div class="personality-traits">
                                ${Utils.escapeHtml(character.personality)}
                            </div>
                        </div>
                    </div>
                ` : ''}

                <!-- 说话风格 -->
                ${character.speech_style ? `
                    <div class="info-section mb-3">
                        <h6 class="section-title">
                            <i class="bi bi-chat-quote"></i> 说话风格
                        </h6>
                        <div class="info-content">
                            <div class="speech-style">
                                ${Utils.escapeHtml(character.speech_style)}
                            </div>
                        </div>
                    </div>
                ` : ''}

                <!-- 知识背景 -->
                ${character.knowledge && character.knowledge.length > 0 ? `
                    <div class="info-section mb-3">
                        <h6 class="section-title">
                            <i class="bi bi-book"></i> 知识背景
                        </h6>
                        <div class="info-content">
                            <div class="knowledge-list">
                                ${character.knowledge.map(item => `
                                    <div class="knowledge-item">
                                        <i class="bi bi-check2"></i>
                                        ${Utils.escapeHtml(item)}
                                    </div>
                                `).join('')}
                            </div>
                        </div>
                    </div>
                ` : ''}

                <!-- 关系网络 -->
                ${character.relationships && Object.keys(character.relationships).length > 0 ? `
                    <div class="info-section mb-3">
                        <h6 class="section-title">
                            <i class="bi bi-people"></i> 关系网络
                        </h6>
                        <div class="info-content">
                            <div class="relationships-list">
                                ${Object.entries(character.relationships).map(([name, relation]) => `
                                    <div class="relationship-item d-flex justify-content-between align-items-center">
                                        <span class="relationship-name">${Utils.escapeHtml(name)}</span>
                                        <span class="relationship-type badge bg-light text-dark">${Utils.escapeHtml(relation)}</span>
                                    </div>
                                `).join('')}
                            </div>
                        </div>
                    </div>
                ` : ''}

                <!-- 互动统计 -->
                <div class="info-section mb-3">
                    <h6 class="section-title">
                        <i class="bi bi-graph-up"></i> 互动统计
                    </h6>
                    <div class="info-content">
                        <div class="interaction-stats">
                            <div class="row g-2">
                                <div class="col-6">
                                    <div class="stat-card text-center">
                                        <div class="stat-number">${this.getCharacterInteractionCount(characterId)}</div>
                                        <div class="stat-label">对话次数</div>
                                    </div>
                                </div>
                                <div class="col-6">
                                    <div class="stat-card text-center">
                                        <div class="stat-number">${this.getCharacterLastInteractionTime(characterId)}</div>
                                        <div class="stat-label">最后互动</div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- 操作按钮 -->
                <div class="character-actions mt-3">
                    <div class="btn-group w-100" role="group">
                        <button class="btn btn-primary" onclick="app.startChatWithCharacter('${characterId}')">
                            <i class="bi bi-chat"></i> 开始对话
                        </button>
                        <button class="btn btn-outline-secondary" onclick="app.viewCharacterHistory('${characterId}')">
                            <i class="bi bi-clock-history"></i> 历史记录
                        </button>
                        <button class="btn btn-outline-info" onclick="app.showCharacterDetails('${characterId}')">
                            <i class="bi bi-info-circle"></i> 详细信息
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `;

        // 确保容器可见
        infoContainer.style.display = 'block';

        // 添加显示动画
        infoContainer.classList.add('character-info-animate');
        setTimeout(() => {
            infoContainer.classList.remove('character-info-animate');
        }, 300);
    }

    /**
     * 清空角色信息显示
     */
    clearCharacterInfo() {
        const infoContainer = document.getElementById('character-info');
        if (infoContainer) {
            infoContainer.innerHTML = `
            <div class="character-info-placeholder text-center text-muted py-4">
                <i class="bi bi-person-circle fs-1"></i>
                <p class="mt-2">选择一个角色查看详细信息</p>
            </div>
        `;
        }
    }

    /**
     * 根据ID查找角色
     */
    findCharacterById(characterId) {
        if (!this.currentScene?.characters) {
            return null;
        }

        return this.currentScene.characters.find(char => char.id === characterId);
    }

    /**
     * 获取角色情绪徽章
     */
    getCharacterMoodBadge(character) {
        // 从最近的对话中获取情绪信息
        const recentEmotion = this.getCharacterRecentEmotion(character.id);

        if (recentEmotion) {
            const emotionColors = {
                '开心': 'bg-success',
                '悲伤': 'bg-primary',
                '愤怒': 'bg-danger',
                '惊讶': 'bg-warning',
                '恐惧': 'bg-dark',
                '厌恶': 'bg-secondary',
                '中性': 'bg-light text-dark'
            };

            const colorClass = emotionColors[recentEmotion] || 'bg-light text-dark';
            return `<span class="badge ${colorClass}">${recentEmotion}</span>`;
        }

        return '<span class="badge bg-light text-dark">平静</span>';
    }

    /**
     * 获取角色最近的情绪
     */
    getCharacterRecentEmotion(characterId) {
        if (!this.conversations || this.conversations.length === 0) {
            return null;
        }

        // 从最近的对话中查找该角色的情绪
        for (let i = this.conversations.length - 1; i >= 0; i--) {
            const conv = this.conversations[i];
            if (conv.speaker_id === characterId && conv.emotion) {
                return conv.emotion;
            }
        }

        return null;
    }

    /**
     * 获取角色互动次数
     */
    getCharacterInteractionCount(characterId) {
        if (!this.conversations) return 0;

        return this.conversations.filter(conv => conv.speaker_id === characterId).length;
    }

    /**
     * 获取角色最后互动时间
     */
    getCharacterLastInteractionTime(characterId) {
        if (!this.conversations) return '从未';

        const characterConvs = this.conversations.filter(conv => conv.speaker_id === characterId);
        if (characterConvs.length === 0) return '从未';

        const lastConv = characterConvs[characterConvs.length - 1];
        return Utils.formatTime(lastConv.timestamp, 'relative');
    }

    /**
     * 开始与指定角色对话
     */
    startChatWithCharacter(characterId) {
        // 选择角色并启用聊天界面
        this.selectCharacter(characterId);

        // 聚焦到消息输入框
        const messageInput = document.getElementById('message-input');
        if (messageInput) {
            messageInput.focus();
        }

        // 滚动到聊天区域
        const chatContainer = document.getElementById('chat-container');
        if (chatContainer) {
            chatContainer.scrollIntoView({ behavior: 'smooth' });
        }
    }

    /**
     * 查看角色对话历史
     */
    async viewCharacterHistory(characterId) {
        try {
            // 获取与该角色的对话历史
            const characterConversations = this.conversations.filter(conv =>
                conv.speaker_id === characterId ||
                (conv.speaker_id === 'user' && this.selectedCharacter === characterId)
            );

            if (characterConversations.length === 0) {
                Utils.showInfo('暂无与该角色的对话记录');
                return;
            }

            // 创建历史记录模态框
            this.showCharacterHistoryModal(characterId, characterConversations);

        } catch (error) {
            console.error('查看角色历史失败:', error);
            Utils.showError('查看历史记录失败: ' + error.message);
        }
    }

    /**
     * 显示角色历史记录模态框
     */
    showCharacterHistoryModal(characterId, conversations) {
        const character = this.findCharacterById(characterId);
        if (!character) return;

        // 移除已存在的模态框
        const existingModal = document.getElementById('character-history-modal');
        if (existingModal) {
            existingModal.remove();
        }

        const modal = document.createElement('div');
        modal.id = 'character-history-modal';
        modal.className = 'modal fade';
        modal.innerHTML = `
        <div class="modal-dialog modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-clock-history"></i> 
                        与 ${Utils.escapeHtml(character.name)} 的对话记录
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <div class="conversation-history" style="max-height: 400px; overflow-y: auto;">
                        ${conversations.map(conv => `
                            <div class="history-message mb-3 ${conv.speaker_id === 'user' ? 'user-message' : 'character-message'}">
                                <div class="message-header d-flex justify-content-between mb-1">
                                    <strong class="${conv.speaker_id === 'user' ? 'text-primary' : 'text-success'}">
                                        ${conv.speaker_id === 'user' ? '你' : character.name}
                                    </strong>
                                    <small class="text-muted">${Utils.formatTime(conv.timestamp)}</small>
                                </div>
                                <div class="message-content">
                                    ${Utils.escapeHtml(conv.content)}
                                    ${conv.emotion ? `<div class="message-emotion mt-1"><span class="badge bg-light text-dark">${conv.emotion}</span></div>` : ''}
                                </div>
                            </div>
                        `).join('')}
                    </div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
                    <button type="button" class="btn btn-primary" onclick="app.exportCharacterHistory('${characterId}')">
                        <i class="bi bi-download"></i> 导出记录
                    </button>
                </div>
            </div>
        </div>
    `;

        document.body.appendChild(modal);

        // 显示模态框
        if (typeof bootstrap !== 'undefined') {
            const bsModal = new bootstrap.Modal(modal);
            bsModal.show();
        }
    }

    /**
     * 显示角色详细信息模态框
     */
    showCharacterDetails(characterId) {
        const character = this.findCharacterById(characterId);
        if (!character) return;

        // 创建详细信息模态框
        const existingModal = document.getElementById('character-details-modal');
        if (existingModal) {
            existingModal.remove();
        }

        const modal = document.createElement('div');
        modal.id = 'character-details-modal';
        modal.className = 'modal fade';
        modal.innerHTML = `
        <div class="modal-dialog modal-xl">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-person-badge"></i> 
                        ${Utils.escapeHtml(character.name)} - 详细档案
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <div class="character-full-details">
                        <!-- 完整的角色信息展示 -->
                        <div class="row">
                            <div class="col-md-4">
                                <div class="character-avatar-large text-center mb-3">
                                    ${character.avatar ?
                `<img src="${character.avatar}" alt="${character.name}" class="rounded-circle" width="150" height="150">` :
                `<div class="avatar-placeholder-large rounded-circle bg-primary text-white d-flex align-items-center justify-content-center mx-auto" style="width: 150px; height: 150px; font-size: 3rem;">${character.name[0]}</div>`
            }
                                    <h4 class="mt-3">${Utils.escapeHtml(character.name)}</h4>
                                    <p class="text-muted">${Utils.escapeHtml(character.role || '角色')}</p>
                                </div>
                            </div>
                            <div class="col-md-8">
                                <!-- 详细信息内容 -->
                                ${this.renderCharacterFullDetails(character)}
                            </div>
                        </div>
                    </div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
                    <button type="button" class="btn btn-outline-primary" onclick="app.editCharacter('${characterId}')">
                        <i class="bi bi-pencil"></i> 编辑
                    </button>
                </div>
            </div>
        </div>
    `;

        document.body.appendChild(modal);

        // 显示模态框
        if (typeof bootstrap !== 'undefined') {
            const bsModal = new bootstrap.Modal(modal);
            bsModal.show();
        }
    }

    /**
     * 渲染角色完整详细信息
     */
    renderCharacterFullDetails(character) {
        return `
        <div class="character-details-tabs">
            <ul class="nav nav-tabs" role="tablist">
                <li class="nav-item" role="presentation">
                    <button class="nav-link active" data-bs-toggle="tab" data-bs-target="#basic-info" type="button" role="tab">基本信息</button>
                </li>
                <li class="nav-item" role="presentation">
                    <button class="nav-link" data-bs-toggle="tab" data-bs-target="#personality" type="button" role="tab">性格特征</button>
                </li>
                <li class="nav-item" role="presentation">
                    <button class="nav-link" data-bs-toggle="tab" data-bs-target="#relationships" type="button" role="tab">人际关系</button>
                </li>
                <li class="nav-item" role="presentation">
                    <button class="nav-link" data-bs-toggle="tab" data-bs-target="#statistics" type="button" role="tab">互动统计</button>
                </li>
            </ul>
            
            <div class="tab-content mt-3">
                <div class="tab-pane fade show active" id="basic-info" role="tabpanel">
                    ${this.renderBasicInfoTab(character)}
                </div>
                <div class="tab-pane fade" id="personality" role="tabpanel">
                    ${this.renderPersonalityTab(character)}
                </div>
                <div class="tab-pane fade" id="relationships" role="tabpanel">
                    ${this.renderRelationshipsTab(character)}
                </div>
                <div class="tab-pane fade" id="statistics" role="tabpanel">
                    ${this.renderStatisticsTab(character)}
                </div>
            </div>
        </div>
    `;
    }

    /**
     * 渲染基本信息标签页
     */
    renderBasicInfoTab(character) {
        return `
        <div class="basic-info-content">
            <div class="info-group mb-3">
                <label class="fw-bold">描述：</label>
                <p>${Utils.escapeHtml(character.description || '暂无描述')}</p>
            </div>
            
            <div class="info-group mb-3">
                <label class="fw-bold">背景：</label>
                <p>${Utils.escapeHtml(character.background || '暂无背景信息')}</p>
            </div>
            
            ${character.age ? `
                <div class="info-group mb-3">
                    <label class="fw-bold">年龄：</label>
                    <p>${Utils.escapeHtml(character.age)}</p>
                </div>
            ` : ''}
            
            <div class="info-group mb-3">
                <label class="fw-bold">创建时间：</label>
                <p>${Utils.formatTime(character.created_at || new Date())}</p>
            </div>
        </div>
    `;
    }

    /**
     * 渲染性格特征标签页
     */
    renderPersonalityTab(character) {
        return `
        <div class="personality-content">
            <div class="info-group mb-3">
                <label class="fw-bold">性格描述：</label>
                <p>${Utils.escapeHtml(character.personality || '暂无性格描述')}</p>
            </div>
            
            ${character.speech_style ? `
                <div class="info-group mb-3">
                    <label class="fw-bold">说话风格：</label>
                    <p>${Utils.escapeHtml(character.speech_style)}</p>
                </div>
            ` : ''}
            
            ${character.knowledge && character.knowledge.length > 0 ? `
                <div class="info-group mb-3">
                    <label class="fw-bold">知识领域：</label>
                    <ul class="list-unstyled">
                        ${character.knowledge.map(item => `
                            <li><i class="bi bi-check2 text-success"></i> ${Utils.escapeHtml(item)}</li>
                        `).join('')}
                    </ul>
                </div>
            ` : ''}
        </div>
    `;
    }

    /**
     * 渲染人际关系标签页
     */
    renderRelationshipsTab(character) {
        if (!character.relationships || Object.keys(character.relationships).length === 0) {
            return '<p class="text-muted">暂无人际关系信息</p>';
        }

        return `
        <div class="relationships-content">
            <div class="relationships-grid">
                ${Object.entries(character.relationships).map(([name, relation]) => `
                    <div class="relationship-card card mb-2">
                        <div class="card-body p-3">
                            <div class="d-flex justify-content-between align-items-center">
                                <div class="relationship-info">
                                    <h6 class="mb-1">${Utils.escapeHtml(name)}</h6>
                                    <span class="badge bg-primary">${Utils.escapeHtml(relation)}</span>
                                </div>
                                <i class="bi bi-person-lines-fill fs-4 text-muted"></i>
                            </div>
                        </div>
                    </div>
                `).join('')}
            </div>
        </div>
    `;
    }

    /**
     * 渲染统计信息标签页
     */
    renderStatisticsTab(character) {
        const interactionCount = this.getCharacterInteractionCount(character.id);
        const lastInteraction = this.getCharacterLastInteractionTime(character.id);
        const recentEmotion = this.getCharacterRecentEmotion(character.id);

        return `
        <div class="statistics-content">
            <div class="stats-grid row g-3">
                <div class="col-md-6">
                    <div class="stat-card card">
                        <div class="card-body text-center">
                            <i class="bi bi-chat-dots fs-1 text-primary"></i>
                            <h4 class="mt-2">${interactionCount}</h4>
                            <p class="text-muted">对话次数</p>
                        </div>
                    </div>
                </div>
                
                <div class="col-md-6">
                    <div class="stat-card card">
                        <div class="card-body text-center">
                            <i class="bi bi-clock fs-1 text-success"></i>
                            <h5 class="mt-2">${lastInteraction}</h5>
                            <p class="text-muted">最后互动</p>
                        </div>
                    </div>
                </div>
                
                ${recentEmotion ? `
                    <div class="col-md-6">
                        <div class="stat-card card">
                            <div class="card-body text-center">
                                <i class="bi bi-emoji-smile fs-1 text-warning"></i>
                                <h5 class="mt-2">${recentEmotion}</h5>
                                <p class="text-muted">当前情绪</p>
                            </div>
                        </div>
                    </div>
                ` : ''}
                
                <div class="col-md-6">
                    <div class="stat-card card">
                        <div class="card-body text-center">
                            <i class="bi bi-graph-up fs-1 text-info"></i>
                            <h5 class="mt-2">活跃</h5>
                            <p class="text-muted">互动状态</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `;
    }

    /**
     * 导出角色历史记录
     */
    async exportCharacterHistory(characterId) {
        try {
            const character = this.findCharacterById(characterId);
            if (!character) {
                Utils.showError('未找到角色信息');
                return;
            }

            const characterConversations = this.conversations.filter(conv =>
                conv.speaker_id === characterId ||
                (conv.speaker_id === 'user' && this.selectedCharacter === characterId)
            );

            if (characterConversations.length === 0) {
                Utils.showInfo('没有可导出的对话记录');
                return;
            }

            const exportData = {
                character: {
                    id: character.id,
                    name: character.name,
                    role: character.role
                },
                scene: {
                    id: this.currentScene.id,
                    name: this.currentScene.name
                },
                conversations: characterConversations,
                export_time: new Date().toISOString(),
                total_messages: characterConversations.length
            };

            const content = JSON.stringify(exportData, null, 2);
            const filename = `chat_history_${character.name}_${Date.now()}.json`;

            // 创建下载
            const blob = new Blob([content], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = filename;
            a.click();
            URL.revokeObjectURL(url);

            Utils.showSuccess('对话记录导出成功');

        } catch (error) {
            console.error('导出历史记录失败:', error);
            Utils.showError('导出失败: ' + error.message);
        }
    }

    /**
     * 编辑角色信息 (预留接口)
     */
    editCharacter(characterId) {
        Utils.showInfo('角色编辑功能正在开发中...');
        // 这里将来可以集成角色编辑功能
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
            const response = await API.createInteraction({
                scene_id: this.currentScene.id,
                character_id: this.selectedCharacter,
                message: message,
                interaction_type: 'chat',
                context: {
                    use_emotion: true,
                    include_story_update: this.state.storyMode,
                    user_preferences: this.currentUser?.preferences
                }
            });

            // 处理响应 - 适配后端返回格式
            if (response.success || response.data) {
                const responseData = response.data || response;

                if (responseData.character_response) {
                    this.addMessageToChat({
                        speaker_id: this.selectedCharacter,
                        content: responseData.character_response.content || responseData.character_response.message,
                        emotion: responseData.character_response.emotion,
                        timestamp: new Date()
                    });

                    // 更新对话列表
                    this.conversations.push({
                        speaker_id: 'user',
                        content: message,
                        timestamp: new Date()
                    }, {
                        speaker_id: this.selectedCharacter,
                        content: responseData.character_response.content || responseData.character_response.message,
                        emotion: responseData.character_response.emotion,
                        timestamp: new Date()
                    });
                }

                // 更新故事状态
                if (responseData.story_update && this.state.storyMode) {
                    this.updateStoryDisplay(responseData.story_update);
                }
            }

        } catch (error) {
            Utils.showError('发送消息失败: ' + error.message);
        } finally {
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
     * 渲染故事操作按钮
     */
    renderStoryActions() {
        const actionsContainer = document.getElementById('story-actions');
        if (!actionsContainer) return;

        actionsContainer.innerHTML = `
        <div class="story-actions-header mb-3">
            <h6 class="mb-0">
                <i class="bi bi-gear"></i> 故事操作
            </h6>
        </div>
        <div class="story-action-buttons">
            <div class="btn-group flex-wrap" role="group">
                <button class="btn btn-outline-primary" onclick="app.advanceStory()" ${!this.storyData ? 'disabled' : ''}>
                    <i class="bi bi-play-circle"></i> 推进故事
                </button>
                <button class="btn btn-outline-secondary" onclick="app.refreshStoryData()" title="刷新故事数据">
                    <i class="bi bi-arrow-clockwise"></i> 刷新
                </button>
                <button class="btn btn-outline-info" onclick="app.viewStoryBranches()" ${!this.storyData?.nodes ? 'disabled' : ''}>
                    <i class="bi bi-diagram-3"></i> 查看分支
                </button>
                <button class="btn btn-outline-warning" onclick="app.exportStoryData()" ${!this.storyData ? 'disabled' : ''}>
                    <i class="bi bi-download"></i> 导出故事
                </button>
                <button class="btn btn-outline-danger" onclick="app.resetStory()" ${!this.storyData ? 'disabled' : ''}>
                    <i class="bi bi-arrow-counterclockwise"></i> 重置故事
                </button>
            </div>
        </div>
        
        ${this.storyData ? `
            <div class="story-info mt-3">
                <div class="row g-2">
                    <div class="col-sm-6">
                        <div class="story-stat">
                            <small class="text-muted">当前状态:</small>
                            <div class="fw-medium">${this.storyData.current_state || '开始'}</div>
                        </div>
                    </div>
                    <div class="col-sm-6">
                        <div class="story-stat">
                            <small class="text-muted">节点数量:</small>
                            <div class="fw-medium">${this.storyData.nodes?.length || 0}</div>
                        </div>
                    </div>
                </div>
            </div>
        ` : ''}
    `;
    }

    /**
 * 刷新故事数据
 */
    async refreshStoryData() {
        try {
            this.setState({ isLoading: true });

            const storyData = await API.getStoryData(this.currentScene.id);
            this.storyData = storyData;

            // 重新渲染故事界面
            this.renderStoryInterface();

            Utils.showSuccess('故事数据已刷新');
        } catch (error) {
            Utils.showError('刷新故事数据失败: ' + error.message);
        } finally {
            this.setState({ isLoading: false });
        }
    }

    /**
     * 查看故事分支
     */
    async viewStoryBranches() {
        try {
            const branches = await API.getStoryBranches(this.currentScene.id);

            // 创建分支查看模态框
            this.showStoryBranchesModal(branches);

        } catch (error) {
            Utils.showError('获取故事分支失败: ' + error.message);
        }
    }

    /**
     * 显示故事分支模态框
     */
    showStoryBranchesModal(branches) {
        // 移除已存在的模态框
        const existingModal = document.getElementById('story-branches-modal');
        if (existingModal) {
            existingModal.remove();
        }

        const modal = document.createElement('div');
        modal.id = 'story-branches-modal';
        modal.className = 'modal fade';
        modal.innerHTML = `
        <div class="modal-dialog modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-diagram-3"></i> 故事分支图
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    ${this.renderBranchesTree(branches)}
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
                </div>
            </div>
        </div>
    `;

        document.body.appendChild(modal);

        // 显示模态框
        if (typeof bootstrap !== 'undefined') {
            const bsModal = new bootstrap.Modal(modal);
            bsModal.show();
        }
    }

    /**
     * 渲染分支树
     */
    renderBranchesTree(branches) {
        if (!branches || !branches.nodes) {
            return '<p class="text-muted">暂无故事分支数据</p>';
        }

        return `
        <div class="story-branches-tree">
            ${branches.nodes.map((node, index) => `
                <div class="branch-node ${node.is_revealed ? 'revealed' : 'hidden'}" data-node-id="${node.id}">
                    <div class="node-header">
                        <span class="node-number">${index + 1}</span>
                        <span class="node-title">${Utils.escapeHtml(node.content.substring(0, 50))}...</span>
                        ${node.is_revealed ?
                '<span class="badge bg-success">已解锁</span>' :
                '<span class="badge bg-secondary">未解锁</span>'
            }
                    </div>
                    ${node.choices && node.choices.length > 0 ? `
                        <div class="node-choices mt-2">
                            ${node.choices.map(choice => `
                                <div class="choice-item ${choice.selected ? 'selected' : ''}">
                                    <i class="bi bi-arrow-right"></i>
                                    ${Utils.escapeHtml(choice.text)}
                                    ${choice.selected ? '<i class="bi bi-check-circle text-success"></i>' : ''}
                                </div>
                            `).join('')}
                        </div>
                    ` : ''}
                </div>
            `).join('')}
        </div>
    `;
    }

    /**
     * 导出故事数据
     */
    async exportStoryData(format = 'json') {
        try {
            const result = await API.exportStoryDocument(this.currentScene.id, format);

            // 处理下载
            let content, mimeType, filename;

            if (typeof result === 'string') {
                content = result;
            } else if (result.content) {
                content = result.content;
            } else {
                content = JSON.stringify(result, null, 2);
            }

            switch (format) {
                case 'json':
                    mimeType = 'application/json';
                    break;
                case 'markdown':
                case 'md':
                    mimeType = 'text/markdown';
                    break;
                case 'txt':
                    mimeType = 'text/plain';
                    break;
                default:
                    mimeType = 'application/octet-stream';
            }

            filename = `story_${this.currentScene.id}_${Date.now()}.${format}`;

            // 创建下载
            const blob = new Blob([content], { type: mimeType });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = filename;
            a.click();
            URL.revokeObjectURL(url);

            Utils.showSuccess('故事数据导出成功');
        } catch (error) {
            Utils.showError('导出故事数据失败: ' + error.message);
        }
    }

    /**
     * 重置故事
     */
    async resetStory() {
        try {
            const confirmed = confirm('确定要重置整个故事吗？这将清除所有进度和选择记录。');
            if (!confirmed) return;

            this.setState({ isLoading: true });

            // 传递当前用户偏好设置（如果有的话）
            await API.resetStory(this.currentScene.id, this.currentUser?.preferences || null);

            // 重新加载故事数据
            await this.refreshStoryData();

            Utils.showSuccess('故事已重置');
        } catch (error) {
            Utils.showError('重置故事失败: ' + error.message);
        } finally {
            this.setState({ isLoading: false });
        }
    }

    /**
     * 更新故事显示 - 处理实时故事更新
     */
    updateStoryDisplay(storyUpdate) {
        if (!storyUpdate) return;

        // 更新故事数据
        if (storyUpdate.story_data) {
            this.storyData = storyUpdate.story_data;
        }

        // 显示更新通知
        if (storyUpdate.new_content) {
            Utils.showInfo('故事更新: ' + storyUpdate.new_content);
        }

        // 重新渲染故事界面
        this.renderStoryInterface();

        // 滚动到新内容
        setTimeout(() => {
            const storyContainer = document.getElementById('story-container');
            if (storyContainer) {
                storyContainer.scrollTop = storyContainer.scrollHeight;
            }
        }, 100);
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
                this.currentScene.id,   // sceneId
                nodeId,
                choiceId,
                this.currentUser?.preferences
            );

            if (result.success) {
                // 更新故事数据
                this.storyData = result.story_data || result.data;

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
                this.storyData = result.story_data || result.data;
                this.renderStoryInterface();

                Utils.showSuccess('故事已推进: ' + result.new_content || '继续');
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

            // 并行加载用户相关数据
            await Promise.all([
                this.loadUserItems(userId),
                this.loadUserSkills(userId)
            ]);

        } catch (error) {
            Utils.showError('加载用户档案失败: ' + error.message);
        }
    }

    /**
     * 渲染用户档案信息
     */
    renderUserProfile() {
        const profileContainer = this.getUserProfileContainer();
        if (!profileContainer) {
            console.warn('未找到用户档案容器');
            return;
        }

        // 如果没有当前用户数据，显示默认状态
        if (!this.currentUser) {
            profileContainer.innerHTML = `
            <div class="user-profile-placeholder text-center text-muted py-4">
                <i class="bi bi-person-circle fs-1"></i>
                <p class="mt-2">用户信息加载中...</p>
                <button class="btn btn-primary btn-sm" onclick="app.loadDefaultUserProfile()">
                    <i class="bi bi-person-plus"></i> 加载默认用户
                </button>
            </div>
        `;
            return;
        }

        // 渲染用户基本信息
        profileContainer.innerHTML = `
        <div class="user-profile-card">
            <!-- 用户头像和基本信息 -->
            <div class="user-header mb-3">
                <div class="d-flex align-items-center">
                    <div class="user-avatar me-3">
                        ${this.currentUser.avatar ?
                `<img src="${this.currentUser.avatar}" alt="${this.currentUser.display_name || this.currentUser.username}" class="rounded-circle" width="64" height="64">` :
                `<div class="avatar-placeholder rounded-circle bg-primary text-white d-flex align-items-center justify-content-center" style="width: 64px; height: 64px; font-size: 1.5rem;">
                                ${(this.currentUser.display_name || this.currentUser.username || 'U')[0].toUpperCase()}
                            </div>`
            }
                    </div>
                    <div class="user-basic-info">
                        <h5 class="mb-1">${Utils.escapeHtml(this.currentUser.display_name || this.currentUser.username || '未命名用户')}</h5>
                        <div class="user-id text-muted mb-1">ID: ${Utils.escapeHtml(this.currentUser.id || 'unknown')}</div>
                        <div class="user-status">
                            <span class="badge bg-success">在线</span>
                            ${this.currentUser.preferences ? '<span class="badge bg-info">已配置</span>' : '<span class="badge bg-warning">待配置</span>'}
                        </div>
                    </div>
                </div>
            </div>

            <!-- 用户详细信息 -->
            <div class="user-details">
                <!-- 个人简介 -->
                ${this.currentUser.bio ? `
                    <div class="info-section mb-3">
                        <h6 class="section-title">
                            <i class="bi bi-card-text"></i> 个人简介
                        </h6>
                        <div class="info-content">
                            <p class="user-bio">${Utils.escapeHtml(this.currentUser.bio)}</p>
                        </div>
                    </div>
                ` : ''}

                <!-- 统计信息 -->
                <div class="info-section mb-3">
                    <h6 class="section-title">
                        <i class="bi bi-graph-up"></i> 统计信息
                    </h6>
                    <div class="info-content">
                        <div class="user-stats">
                            <div class="row g-2">
                                <div class="col-6">
                                    <div class="stat-card text-center">
                                        <div class="stat-number">${this.currentUser.items_count || 0}</div>
                                        <div class="stat-label">道具数量</div>
                                    </div>
                                </div>
                                <div class="col-6">
                                    <div class="stat-card text-center">
                                        <div class="stat-number">${this.currentUser.skills_count || 0}</div>
                                        <div class="stat-label">技能数量</div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- 用户偏好设置 -->
                ${this.currentUser.preferences ? `
                    <div class="info-section mb-3">
                        <h6 class="section-title">
                            <i class="bi bi-gear"></i> 偏好设置
                        </h6>
                        <div class="info-content">
                            <div class="preferences-summary">
                                <div class="row g-2">
                                    <div class="col-sm-6">
                                        <small class="text-muted">创意等级:</small>
                                        <div class="fw-medium">${this.formatCreativityLevel(this.currentUser.preferences.creativity_level)}</div>
                                    </div>
                                    <div class="col-sm-6">
                                        <small class="text-muted">响应长度:</small>
                                        <div class="fw-medium">${this.formatResponseLength(this.currentUser.preferences.response_length)}</div>
                                    </div>
                                    <div class="col-sm-6">
                                        <small class="text-muted">语言风格:</small>
                                        <div class="fw-medium">${this.formatLanguageStyle(this.currentUser.preferences.language_style)}</div>
                                    </div>
                                    <div class="col-sm-6">
                                        <small class="text-muted">主题模式:</small>
                                        <div class="fw-medium">${this.currentUser.preferences.dark_mode ? '深色' : '浅色'}</div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                ` : ''}

                <!-- 保存的场景 -->
                ${this.currentUser.saved_scenes && this.currentUser.saved_scenes.length > 0 ? `
                    <div class="info-section mb-3">
                        <h6 class="section-title">
                            <i class="bi bi-bookmark"></i> 保存的场景
                        </h6>
                        <div class="info-content">
                            <div class="saved-scenes-list">
                                ${this.currentUser.saved_scenes.slice(0, 3).map(sceneId => `
                                    <div class="saved-scene-item">
                                        <i class="bi bi-bookmark-check"></i>
                                        <span class="scene-link" onclick="app.navigateToScene('${sceneId}')">${sceneId}</span>
                                    </div>
                                `).join('')}
                                ${this.currentUser.saved_scenes.length > 3 ? `
                                    <div class="more-scenes text-muted">
                                        还有 ${this.currentUser.saved_scenes.length - 3} 个场景...
                                    </div>
                                ` : ''}
                            </div>
                        </div>
                    </div>
                ` : ''}

                <!-- 操作按钮 -->
                <div class="user-actions mt-3">
                    <div class="btn-group w-100" role="group">
                        <button class="btn btn-primary" onclick="app.editUserProfile()">
                            <i class="bi bi-pencil"></i> 编辑档案
                        </button>
                        <button class="btn btn-outline-secondary" onclick="app.manageUserItems()">
                            <i class="bi bi-bag"></i> 管理道具
                        </button>
                        <button class="btn btn-outline-info" onclick="app.manageUserSkills()">
                            <i class="bi bi-lightning"></i> 管理技能
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `;

        // 确保容器可见
        profileContainer.style.display = 'block';

        // 添加显示动画
        profileContainer.classList.add('user-profile-animate');
        setTimeout(() => {
            profileContainer.classList.remove('user-profile-animate');
        }, 300);
    }

    /**
     * 获取用户档案容器
     */
    getUserProfileContainer() {
        // 尝试多个可能的容器ID
        const containerIds = ['user-profile-container', 'user-profile', 'profile-container'];

        for (const id of containerIds) {
            const container = document.getElementById(id);
            if (container) return container;
        }

        // 如果没有找到专门的容器，尝试在现有容器中创建
        const parentContainer = document.querySelector('.dashboard-content, .main-content, .content');
        if (parentContainer) {
            const newContainer = document.createElement('div');
            newContainer.id = 'user-profile-container';
            newContainer.className = 'user-profile-section';
            parentContainer.appendChild(newContainer);
            return newContainer;
        }

        return null;
    }

    /**
     * 加载默认用户档案
     */
    async loadDefaultUserProfile() {
        try {
            const defaultUserId = 'user_default';
            await this.loadUserProfile(defaultUserId);
        } catch (error) {
            console.error('加载默认用户档案失败:', error);
            Utils.showError('加载用户档案失败: ' + error.message);
        }
    }

    /**
     * 编辑用户档案
     */
    editUserProfile() {
        // 检查是否有独立的用户档案管理器
        if (typeof window.userProfile !== 'undefined' && window.userProfile.showEditProfileModal) {
            window.userProfile.showEditProfileModal(this.currentUser.id);
            return;
        }

        // 后备方案：显示简单的编辑模态框
        this.showSimpleEditProfileModal();
    }

    /**
     * 显示简单的编辑档案模态框
     */
    showSimpleEditProfileModal() {
        // 移除已存在的模态框
        const existingModal = document.getElementById('edit-profile-modal');
        if (existingModal) {
            existingModal.remove();
        }

        const modal = document.createElement('div');
        modal.id = 'edit-profile-modal';
        modal.className = 'modal fade';
        modal.innerHTML = `
        <div class="modal-dialog">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-pencil"></i> 编辑用户档案
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <form id="edit-profile-form">
                        <div class="mb-3">
                            <label for="edit-display-name" class="form-label">显示名称</label>
                            <input type="text" class="form-control" id="edit-display-name" 
                                   value="${Utils.escapeHtml(this.currentUser.display_name || '')}" required>
                        </div>
                        <div class="mb-3">
                            <label for="edit-bio" class="form-label">个人简介</label>
                            <textarea class="form-control" id="edit-bio" rows="3" 
                                      placeholder="介绍一下自己...">${Utils.escapeHtml(this.currentUser.bio || '')}</textarea>
                        </div>
                        <div class="mb-3">
                            <label for="edit-avatar" class="form-label">头像URL</label>
                            <input type="url" class="form-control" id="edit-avatar" 
                                   value="${Utils.escapeHtml(this.currentUser.avatar || '')}" 
                                   placeholder="https://example.com/avatar.jpg">
                        </div>
                    </form>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
                    <button type="button" class="btn btn-primary" onclick="app.saveUserProfile()">
                        <i class="bi bi-check"></i> 保存更改
                    </button>
                </div>
            </div>
        </div>
    `;

        document.body.appendChild(modal);

        // 显示模态框
        if (typeof bootstrap !== 'undefined') {
            const bsModal = new bootstrap.Modal(modal);
            bsModal.show();
        }
    }

    /**
     * 保存用户档案
     */
    async saveUserProfile() {
        try {
            const form = document.getElementById('edit-profile-form');
            if (!form) return;

            const profileData = {
                display_name: document.getElementById('edit-display-name').value.trim(),
                bio: document.getElementById('edit-bio').value.trim(),
                avatar: document.getElementById('edit-avatar').value.trim()
            };

            // 验证数据
            if (!profileData.display_name) {
                Utils.showError('显示名称不能为空');
                return;
            }

            // 保存到后端
            const updatedProfile = await API.updateUserProfile(this.currentUser.id, profileData);

            // 更新本地数据
            Object.assign(this.currentUser, updatedProfile);

            // 重新渲染
            this.renderUserProfile();

            // 关闭模态框
            const modal = bootstrap.Modal.getInstance(document.getElementById('edit-profile-modal'));
            if (modal) {
                modal.hide();
            }

            Utils.showSuccess('用户档案更新成功');

        } catch (error) {
            console.error('保存用户档案失败:', error);
            Utils.showError('保存失败: ' + error.message);
        }
    }

    /**
     * 管理用户道具
     */
    manageUserItems() {
        // 检查是否有独立的用户档案管理器
        if (typeof window.userProfile !== 'undefined') {
            // 导航到用户档案页面的道具部分
            window.location.href = `/user/profile?user_id=${this.currentUser.id}&tab=items`;
            return;
        }

        // 后备方案：简单提示
        Utils.showInfo('请前往用户档案页面管理道具');
    }

    /**
     * 管理用户技能
     */
    manageUserSkills() {
        // 检查是否有独立的用户档案管理器
        if (typeof window.userProfile !== 'undefined') {
            // 导航到用户档案页面的技能部分
            window.location.href = `/user/profile?user_id=${this.currentUser.id}&tab=skills`;
            return;
        }

        // 后备方案：调用app.js中的技能管理方法
        if (this.initSkillsManagement) {
            this.initSkillsManagement(this.currentUser.id);
        } else {
            Utils.showInfo('请前往用户档案页面管理技能');
        }
    }

    /**
     * 导航到指定场景
     */
    navigateToScene(sceneId) {
        window.location.href = `/scenes/${sceneId}`;
    }

    /**
     * 格式化创意等级
     */
    formatCreativityLevel(level) {
        const levels = {
            'STRICT': '严格',
            'BALANCED': '平衡',
            'EXPANSIVE': '扩展'
        };
        return levels[level] || level;
    }

    /**
     * 格式化响应长度
     */
    formatResponseLength(length) {
        const lengths = {
            'short': '简短',
            'medium': '中等',
            'long': '详细'
        };
        return lengths[length] || length;
    }

    /**
     * 格式化语言风格
     */
    formatLanguageStyle(style) {
        const styles = {
            'formal': '正式',
            'casual': '随意',
            'literary': '文学'
        };
        return styles[style] || style;
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
     * 显示添加道具模态框
     */
    showAddItemModal(userId) {
        if (!userId) {
            Utils.showError('请先指定用户ID');
            return;
        }

        // 检查是否有独立的用户档案管理器
        if (typeof window.userProfile !== 'undefined' && window.userProfile.showAddItemModal) {
            window.userProfile.showAddItemModal();
            return;
        }

        // 后备方案：创建简单的添加道具模态框
        this.createAddItemModal(userId);
    }

    /**
     * 创建添加道具模态框
     */
    createAddItemModal(userId) {
        // 移除已存在的模态框
        const existingModal = document.getElementById('add-item-modal');
        if (existingModal) {
            existingModal.remove();
        }

        const modal = document.createElement('div');
        modal.id = 'add-item-modal';
        modal.className = 'modal fade';
        modal.innerHTML = `
        <div class="modal-dialog modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-bag-plus"></i> 添加新道具
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <form id="add-item-form">
                        <div class="row g-3">
                            <!-- 基本信息 -->
                            <div class="col-12">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-info-circle"></i> 基本信息
                                </h6>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="item-name" class="form-label">道具名称 *</label>
                                <input type="text" class="form-control" id="item-name" 
                                       placeholder="输入道具名称" required>
                                <div class="form-text">道具的显示名称</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="item-type" class="form-label">道具类型</label>
                                <select class="form-select" id="item-type">
                                    <option value="">选择类型</option>
                                    <option value="weapon">武器</option>
                                    <option value="armor">护甲</option>
                                    <option value="consumable">消耗品</option>
                                    <option value="tool">工具</option>
                                    <option value="key_item">关键物品</option>
                                    <option value="accessory">饰品</option>
                                    <option value="material">材料</option>
                                    <option value="other">其他</option>
                                </select>
                                <div class="form-text">道具的分类</div>
                            </div>
                            
                            <div class="col-12">
                                <label for="item-description" class="form-label">道具描述</label>
                                <textarea class="form-control" id="item-description" rows="3" 
                                          placeholder="描述道具的外观、用途等..."></textarea>
                                <div class="form-text">详细描述道具的特征和用途</div>
                            </div>

                            <!-- 属性配置 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-sliders"></i> 属性配置
                                </h6>
                            </div>
                            
                            <div class="col-md-4">
                                <label for="item-rarity" class="form-label">稀有度</label>
                                <select class="form-select" id="item-rarity">
                                    <option value="">选择稀有度</option>
                                    <option value="common">普通</option>
                                    <option value="uncommon">不常见</option>
                                    <option value="rare">稀有</option>
                                    <option value="epic">史诗</option>
                                    <option value="legendary">传说</option>
                                </select>
                            </div>
                            
                            <div class="col-md-4">
                                <label for="item-value" class="form-label">道具价值</label>
                                <input type="number" class="form-control" id="item-value" 
                                       placeholder="0" min="0" step="1">
                                <div class="form-text">道具的经济价值</div>
                            </div>
                            
                            <div class="col-md-4">
                                <label for="item-quantity" class="form-label">数量</label>
                                <input type="number" class="form-control" id="item-quantity" 
                                       placeholder="1" min="1" step="1" value="1">
                                <div class="form-text">拥有的数量</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="item-durability" class="form-label">耐久度</label>
                                <input type="number" class="form-control" id="item-durability" 
                                       placeholder="100" min="0" max="100" step="1">
                                <div class="form-text">道具的耐用程度 (0-100)</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="item-weight" class="form-label">重量</label>
                                <input type="number" class="form-control" id="item-weight" 
                                       placeholder="0" min="0" step="0.1">
                                <div class="form-text">道具重量 (kg)</div>
                            </div>

                            <!-- 效果配置 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-magic"></i> 效果配置
                                </h6>
                            </div>
                            
                            <div class="col-12">
                                <label class="form-label">道具效果</label>
                                <div id="item-effects-container">
                                    <div class="effect-item row g-2 mb-2">
                                        <div class="col-md-4">
                                            <input type="text" class="form-control form-control-sm effect-type" 
                                                   placeholder="效果类型 (如: attack_bonus)">
                                        </div>
                                        <div class="col-md-3">
                                            <input type="number" class="form-control form-control-sm effect-value" 
                                                   placeholder="效果数值" step="0.1">
                                        </div>
                                        <div class="col-md-3">
                                            <input type="number" class="form-control form-control-sm effect-duration" 
                                                   placeholder="持续时间(秒)" min="0">
                                        </div>
                                        <div class="col-md-2">
                                            <button type="button" class="btn btn-outline-danger btn-sm remove-effect-btn">
                                                <i class="bi bi-x"></i>
                                            </button>
                                        </div>
                                    </div>
                                </div>
                                <button type="button" class="btn btn-sm btn-outline-success" onclick="app.addItemEffectRow()">
                                    <i class="bi bi-plus"></i> 添加效果
                                </button>
                                <div class="form-text mt-2">定义道具使用时产生的效果</div>
                            </div>

                            <!-- 使用限制 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-shield-check"></i> 使用限制
                                </h6>
                            </div>
                            
                            <div class="col-md-6">
                                <div class="form-check form-switch">
                                    <input class="form-check-input" type="checkbox" id="item-consumable">
                                    <label class="form-check-label" for="item-consumable">
                                        消耗性道具
                                    </label>
                                    <div class="form-text">使用后会减少数量</div>
                                </div>
                            </div>
                            
                            <div class="col-md-6">
                                <div class="form-check form-switch">
                                    <input class="form-check-input" type="checkbox" id="item-tradeable" checked>
                                    <label class="form-check-label" for="item-tradeable">
                                        可交易
                                    </label>
                                    <div class="form-text">允许与其他玩家交易</div>
                                </div>
                            </div>
                            
                            <div class="col-12">
                                <label for="item-requirements" class="form-label">使用需求</label>
                                <textarea class="form-control" id="item-requirements" rows="2" 
                                          placeholder="例如: level >= 10, strength >= 15"></textarea>
                                <div class="form-text">使用此道具所需的条件</div>
                            </div>
                        </div>
                    </form>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
                    <button type="button" class="btn btn-primary save-item-btn" onclick="app.saveNewItem('${userId}')">
                        <i class="bi bi-check"></i> 保存道具
                    </button>
                </div>
            </div>
        </div>
    `;

        document.body.appendChild(modal);

        // 绑定表单事件
        this.bindAddItemEvents();

        // 显示模态框
        if (typeof bootstrap !== 'undefined') {
            const bsModal = new bootstrap.Modal(modal);
            bsModal.show();
        }
    }

    /**
     * 绑定添加道具表单事件
     */
    bindAddItemEvents() {
        const form = document.getElementById('add-item-form');
        if (!form) return;

        // 实时验证
        form.addEventListener('input', (e) => {
            this.validateItemField(e.target);
        });

        // 移除效果行事件
        form.addEventListener('click', (e) => {
            if (e.target.matches('.remove-effect-btn') || e.target.closest('.remove-effect-btn')) {
                const effectItem = e.target.closest('.effect-item');
                if (effectItem && document.querySelectorAll('.effect-item').length > 1) {
                    effectItem.remove();
                }
            }
        });

        // 道具类型变化事件
        const typeSelect = document.getElementById('item-type');
        if (typeSelect) {
            typeSelect.addEventListener('change', (e) => {
                this.updateItemFormByType(e.target.value);
            });
        }
    }

    /**
     * 验证道具字段
     */
    validateItemField(field) {
        const value = field.value.trim();
        let isValid = true;
        let message = '';

        switch (field.id) {
            case 'item-name':
                isValid = value.length >= 2;
                message = isValid ? '' : '道具名称至少2个字符';
                break;
            case 'item-value':
                isValid = !value || (!isNaN(value) && parseFloat(value) >= 0);
                message = isValid ? '' : '价值必须是非负数';
                break;
            case 'item-quantity':
                isValid = !isNaN(value) && parseInt(value) >= 1;
                message = isValid ? '' : '数量必须是正整数';
                break;
            case 'item-durability':
                isValid = !value || (!isNaN(value) && parseFloat(value) >= 0 && parseFloat(value) <= 100);
                message = isValid ? '' : '耐久度必须在0-100之间';
                break;
            case 'item-weight':
                isValid = !value || (!isNaN(value) && parseFloat(value) >= 0);
                message = isValid ? '' : '重量必须是非负数';
                break;
        }

        // 更新字段状态
        field.classList.toggle('is-invalid', !isValid);
        field.classList.toggle('is-valid', isValid && value);

        // 更新错误消息
        let feedback = field.parentNode.querySelector('.invalid-feedback');
        if (!isValid && message) {
            if (!feedback) {
                feedback = document.createElement('div');
                feedback.className = 'invalid-feedback';
                field.parentNode.appendChild(feedback);
            }
            feedback.textContent = message;
        } else if (feedback) {
            feedback.remove();
        }

        return isValid;
    }

    /**
     * 根据道具类型更新表单
     */
    updateItemFormByType(itemType) {
        const durabilityField = document.getElementById('item-durability');
        const consumableCheck = document.getElementById('item-consumable');

        // 根据类型设置默认值
        switch (itemType) {
            case 'weapon':
            case 'armor':
                if (durabilityField) durabilityField.value = '100';
                if (consumableCheck) consumableCheck.checked = false;
                break;
            case 'consumable':
                if (durabilityField) durabilityField.value = '';
                if (consumableCheck) consumableCheck.checked = true;
                break;
            case 'tool':
                if (durabilityField) durabilityField.value = '100';
                if (consumableCheck) consumableCheck.checked = false;
                break;
            case 'key_item':
                if (durabilityField) durabilityField.value = '';
                if (consumableCheck) consumableCheck.checked = false;
                break;
        }
    }

    /**
     * 添加道具效果行
     */
    addItemEffectRow() {
        const container = document.getElementById('item-effects-container');
        if (!container) return;

        const effectRow = document.createElement('div');
        effectRow.className = 'effect-item row g-2 mb-2';
        effectRow.innerHTML = `
        <div class="col-md-4">
            <input type="text" class="form-control form-control-sm effect-type" 
                   placeholder="效果类型 (如: attack_bonus)">
        </div>
        <div class="col-md-3">
            <input type="number" class="form-control form-control-sm effect-value" 
                   placeholder="效果数值" step="0.1">
        </div>
        <div class="col-md-3">
            <input type="number" class="form-control form-control-sm effect-duration" 
                   placeholder="持续时间(秒)" min="0">
        </div>
        <div class="col-md-2">
            <button type="button" class="btn btn-outline-danger btn-sm remove-effect-btn">
                <i class="bi bi-x"></i>
            </button>
        </div>
    `;

        container.appendChild(effectRow);
    }

    /**
     * 保存新道具
     */
    async saveNewItem(userId) {
        try {
            const form = document.getElementById('add-item-form');
            if (!form) {
                throw new Error('找不到添加道具表单');
            }

            // 验证所有必填字段
            const nameField = document.getElementById('item-name');
            if (!this.validateItemField(nameField) || !nameField.value.trim()) {
                Utils.showError('请输入有效的道具名称');
                nameField.focus();
                return;
            }

            // 禁用保存按钮
            this.setButtonLoading('.save-item-btn', true, '保存中...');

            // 收集表单数据
            const itemData = this.collectItemFormData();

            console.log('💾 保存新道具:', itemData);

            // 调用API保存道具
            const result = await API.addUserItem(userId, itemData);

            if (result) {
                // 重新加载道具列表
                await this.loadUserItems(userId);

                // 隐藏模态框
                const modal = bootstrap.Modal.getInstance(document.getElementById('add-item-modal'));
                if (modal) {
                    modal.hide();
                }

                Utils.showSuccess('道具添加成功！');
            }

        } catch (error) {
            console.error('❌ 保存道具失败:', error);
            Utils.showError('保存失败: ' + error.message);
        } finally {
            // 恢复保存按钮
            this.setButtonLoading('.save-item-btn', false, '保存道具');
        }
    }

    /**
     * 收集道具表单数据
     */
    collectItemFormData() {
        const effects = [];
        const effectItems = document.querySelectorAll('.effect-item');

        effectItems.forEach(item => {
            const type = item.querySelector('.effect-type')?.value.trim();
            const value = item.querySelector('.effect-value')?.value.trim();
            const duration = item.querySelector('.effect-duration')?.value.trim();

            if (type && value) {
                effects.push({
                    type: type,
                    value: parseFloat(value) || 0,
                    duration: duration ? parseInt(duration) : null
                });
            }
        });

        return {
            name: document.getElementById('item-name').value.trim(),
            type: document.getElementById('item-type').value || 'other',
            description: document.getElementById('item-description').value.trim(),
            rarity: document.getElementById('item-rarity').value || 'common',
            value: parseFloat(document.getElementById('item-value').value) || 0,
            quantity: parseInt(document.getElementById('item-quantity').value) || 1,
            durability: document.getElementById('item-durability').value ?
                parseFloat(document.getElementById('item-durability').value) : null,
            weight: document.getElementById('item-weight').value ?
                parseFloat(document.getElementById('item-weight').value) : null,
            effects: effects,
            properties: {
                consumable: document.getElementById('item-consumable').checked,
                tradeable: document.getElementById('item-tradeable').checked,
                requirements: document.getElementById('item-requirements').value.trim()
            }
        };
    }

    /**
     * 编辑道具
     */
    editItem(itemId) {
        // 确保用户档案管理器可用
        if (typeof window.userProfile === 'undefined') {
            Utils.showError('用户档案管理器不可用');
            return;
        }

        // 确保用户ID已设置
        const userId = this.getCurrentUserId();
        if (!userId) {
            Utils.showError('无法获取用户ID');
            return;
        }

        try {
            // 确保用户档案管理器有当前用户ID
            if (!window.userProfile.currentUserId) {
                window.userProfile.setCurrentUser(userId);
            }

            // 安全调用用户档案管理器的方法
            window.userProfile.editItem(itemId);

        } catch (error) {
            console.error('调用编辑道具失败:', error);
            Utils.showError('编辑道具失败: ' + error.message);
        }
    }

    /**
     * 删除道具
     */
    async deleteItem(itemId) {
        try {
            const confirmed = await Utils.showConfirm('确定要删除这个道具吗？', {
                title: '确认删除',
                confirmText: '删除',
                cancelText: '取消',
                type: 'danger'
            });

            if (!confirmed) return;

            // 检查是否有独立的用户档案管理器
            if (typeof window.userProfile !== 'undefined' && window.userProfile.deleteItem) {
                await window.userProfile.deleteItem(itemId);
                return;
            }

            // 后备方案：调用API删除
            const userId = this.getCurrentUserId();
            if (!userId) {
                Utils.showError('无法获取用户ID');
                return;
            }

            await API.deleteUserItem(userId, itemId);

            // 重新加载道具列表
            await this.loadUserItems(userId);

            Utils.showSuccess('道具删除成功');

        } catch (error) {
            console.error('删除道具失败:', error);
            Utils.showError('删除失败: ' + error.message);
        }
    }

    /**
     * 设置按钮加载状态
     */
    setButtonLoading(selector, loading, text = null) {
        const button = document.querySelector(selector);
        if (!button) return;

        if (loading) {
            button.disabled = true;
            button.dataset.originalText = button.innerHTML;
            button.innerHTML = `
            <span class="spinner-border spinner-border-sm me-2" role="status"></span>
            ${text || '处理中...'}
        `;
        } else {
            button.disabled = false;
            button.innerHTML = button.dataset.originalText || text || '保存';
        }
    }

    /**
     * 加载用户道具
     */
    async loadUserItems(userId) {
        try {
            const items = await API.getUserItems(userId);
            this.renderUserItems(items);

            // 更新用户状态中的道具计数
            if (this.currentUser) {
                this.currentUser.items_count = items.length;
            }

        } catch (error) {
            console.error('加载道具失败:', error);
            Utils.showError('加载道具失败: ' + error.message);
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
                <button class="btn btn-sm btn-outline-secondary refresh-dashboard-btn dashboard-tooltip" 
                        title="刷新数据" data-tooltip="刷新仪表板数据">
                    <i class="bi bi-arrow-clockwise"></i>
                </button>
                <button class="btn btn-sm btn-outline-info toggle-dashboard-btn dashboard-tooltip" 
                        title="切换显示" data-tooltip="隐藏/显示仪表板">
                    <i class="bi bi-eye-slash"></i>
                </button>
                <button class="btn btn-sm btn-outline-success export-dashboard-btn dashboard-tooltip" 
                        title="导出报告" data-tooltip="导出仪表板报告">
                    <i class="bi bi-download"></i>
                </button>
                <button class="btn btn-sm btn-outline-warning force-refresh-dashboard-btn dashboard-tooltip" 
                        title="强制刷新" data-tooltip="强制重新渲染">
                    <i class="bi bi-arrow-repeat"></i>
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
     * 渲染故事完成度图表
     */
    renderStoryProgressChart() {
        if (typeof Chart === 'undefined') {
            console.warn('Chart.js 未加载，无法渲染故事完成度图表');
            return;
        }

        const canvas = document.getElementById('story-progress-chart');
        if (!canvas) {
            console.warn('未找到故事完成度图表容器');
            return;
        }

        // 计算故事进度数据
        const progressData = this.calculateStoryProgressData();

        const chart = new Chart(canvas, {
            type: 'line',
            data: {
                labels: progressData.labels,
                datasets: [{
                    label: '故事完成度',
                    data: progressData.progress,
                    borderColor: '#007bff',
                    backgroundColor: 'rgba(0, 123, 255, 0.1)',
                    borderWidth: 3,
                    fill: true,
                    tension: 0.4,
                    pointBackgroundColor: '#007bff',
                    pointBorderColor: '#fff',
                    pointBorderWidth: 2,
                    pointRadius: 5
                }, {
                    label: '节点揭示',
                    data: progressData.nodes,
                    borderColor: '#28a745',
                    backgroundColor: 'rgba(40, 167, 69, 0.1)',
                    borderWidth: 2,
                    fill: false,
                    tension: 0.3,
                    pointBackgroundColor: '#28a745',
                    pointBorderColor: '#fff',
                    pointBorderWidth: 2,
                    pointRadius: 4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    title: {
                        display: true,
                        text: '故事进展时间线',
                        font: {
                            size: 16,
                            weight: 'bold'
                        }
                    },
                    legend: {
                        position: 'top',
                        labels: {
                            usePointStyle: true,
                            padding: 20
                        }
                    },
                    tooltip: {
                        mode: 'index',
                        intersect: false,
                        callbacks: {
                            label: function (context) {
                                const label = context.dataset.label || '';
                                const value = context.parsed.y;
                                if (label === '故事完成度') {
                                    return `${label}: ${value}%`;
                                } else {
                                    return `${label}: ${value} 个节点`;
                                }
                            }
                        }
                    }
                },
                scales: {
                    x: {
                        display: true,
                        title: {
                            display: true,
                            text: '时间'
                        }
                    },
                    y: {
                        display: true,
                        title: {
                            display: true,
                            text: '进度 (%)'
                        },
                        min: 0,
                        max: 100
                    }
                },
                interaction: {
                    mode: 'nearest',
                    axis: 'x',
                    intersect: false
                },
                elements: {
                    point: {
                        hoverRadius: 8
                    }
                }
            }
        });

        this.charts.set('story-progress', chart);
    }

    /**
     * 计算故事进度数据
     */
    calculateStoryProgressData() {
        const data = {
            labels: [],
            progress: [],
            nodes: []
        };

        if (!this.aggregateData || !this.aggregateData.story_data) {
            // 如果没有数据，返回示例数据
            const now = new Date();
            for (let i = 0; i < 7; i++) {
                const date = new Date(now.getTime() - (6 - i) * 24 * 60 * 60 * 1000);
                data.labels.push(date.toLocaleDateString());
                data.progress.push(Math.min(i * 15 + Math.random() * 10, 100));
                data.nodes.push(i * 2 + Math.floor(Math.random() * 3));
            }
            return data;
        }

        const storyData = this.aggregateData.story_data;

        // 如果有真实数据，基于节点时间戳构建进度
        if (storyData.nodes && storyData.nodes.length > 0) {
            const nodes = storyData.nodes
                .filter(node => node.is_revealed)
                .sort((a, b) => new Date(a.created_at) - new Date(b.created_at));

            const totalNodes = storyData.nodes.length;
            let revealedCount = 0;

            nodes.forEach((node, index) => {
                if (node.is_revealed) {
                    revealedCount++;
                    const date = new Date(node.created_at);
                    data.labels.push(date.toLocaleDateString());
                    data.progress.push(Math.round((revealedCount / totalNodes) * 100));
                    data.nodes.push(revealedCount);
                }
            });
        }

        // 如果数据点不足，补充当前状态
        if (data.labels.length === 0) {
            data.labels.push('当前');
            data.progress.push(storyData.progress || 0);
            data.nodes.push(storyData.nodes ? storyData.nodes.filter(n => n.is_revealed).length : 0);
        }

        return data;
    }

    /**
     * 渲染角色关系网络图
     */
    renderCharacterRelationshipGraph() {
        const container = document.getElementById('character-relationship-graph');
        if (!container) {
            console.warn('未找到角色关系图容器');
            return;
        }

        // 检查是否有可用的图形库
        if (typeof d3 === 'undefined' && typeof vis === 'undefined') {
            // 使用简单的HTML/CSS实现
            this.renderSimpleRelationshipGraph(container);
            return;
        }

        // 计算角色关系数据
        const relationshipData = this.calculateCharacterRelationships();

        // 如果有 vis.js 可用
        if (typeof vis !== 'undefined') {
            this.renderVisNetworkGraph(container, relationshipData);
        } else if (typeof d3 !== 'undefined') {
            this.renderD3ForceGraph(container, relationshipData);
        } else {
            this.renderSimpleRelationshipGraph(container);
        }
    }

    /**
     * 使用简单HTML/CSS渲染关系图
     */
    renderSimpleRelationshipGraph(container) {
        const relationships = this.calculateCharacterRelationships();

        container.innerHTML = `
        <div class="simple-relationship-graph">
            <div class="characters-container">
                ${relationships.nodes.map(node => `
                    <div class="character-node" data-character="${node.id}">
                        <div class="character-avatar">
                            ${node.avatar ?
                `<img src="${node.avatar}" alt="${node.name}">` :
                `<div class="avatar-placeholder">${node.name[0]}</div>`
            }
                        </div>
                        <div class="character-name">${node.name}</div>
                        <div class="interaction-count">${node.interactions || 0} 次互动</div>
                    </div>
                `).join('')}
            </div>
            <div class="relationships-list">
                <h6>角色关系</h6>
                ${relationships.edges.map(edge => `
                    <div class="relationship-item">
                        <span class="from-character">${edge.from_name}</span>
                        <span class="relationship-type">${edge.type}</span>
                        <span class="to-character">${edge.to_name}</span>
                        <span class="strength">(强度: ${edge.strength})</span>
                    </div>
                `).join('')}
            </div>
        </div>
    `;
    }

    /**
     * 使用vis.js渲染网络图
     */
    renderVisNetworkGraph(container, relationshipData) {
        try {
            const nodes = new vis.DataSet(relationshipData.nodes.map(node => ({
                id: node.id,
                label: node.name,
                title: `${node.name}\n互动次数: ${node.interactions || 0}`,
                color: {
                    background: node.color || '#97c2fc',
                    border: '#2b7ce9'
                },
                font: { size: 14 }
            })));

            const edges = new vis.DataSet(relationshipData.edges.map(edge => ({
                from: edge.from,
                to: edge.to,
                label: edge.type,
                width: Math.max(1, edge.strength),
                color: {
                    color: edge.strength > 5 ? '#ff6b6b' : '#4ecdc4'
                }
            })));

            const data = { nodes, edges };
            const options = {
                physics: {
                    enabled: true,
                    stabilization: { iterations: 100 }
                },
                interaction: {
                    hover: true,
                    tooltipDelay: 200
                },
                layout: {
                    improvedLayout: true
                }
            };

            const network = new vis.Network(container, data, options);

            // 保存网络实例以便后续操作
            this.charts.set('character-relationship', network);
        } catch (error) {
            console.error('渲染vis.js网络图失败:', error);
            this.renderSimpleRelationshipGraph(container);
        }
    }

    /**
     * 使用D3.js渲染力导向图
     */
    renderD3ForceGraph(container, relationshipData) {
        try {
            const width = container.clientWidth || 400;
            const height = container.clientHeight || 400;

            // 清空容器
            d3.select(container).selectAll("*").remove();

            const svg = d3.select(container)
                .append("svg")
                .attr("width", width)
                .attr("height", height);

            const simulation = d3.forceSimulation(relationshipData.nodes)
                .force("link", d3.forceLink(relationshipData.edges).id(d => d.id))
                .force("charge", d3.forceManyBody().strength(-300))
                .force("center", d3.forceCenter(width / 2, height / 2));

            const link = svg.append("g")
                .selectAll("line")
                .data(relationshipData.edges)
                .enter().append("line")
                .attr("stroke", "#999")
                .attr("stroke-opacity", 0.6)
                .attr("stroke-width", d => Math.sqrt(d.strength));

            const node = svg.append("g")
                .selectAll("circle")
                .data(relationshipData.nodes)
                .enter().append("circle")
                .attr("r", 20)
                .attr("fill", d => d.color || "#69b3a2")
                .call(d3.drag()
                    .on("start", dragstarted)
                    .on("drag", dragged)
                    .on("end", dragended));

            const label = svg.append("g")
                .selectAll("text")
                .data(relationshipData.nodes)
                .enter().append("text")
                .text(d => d.name)
                .attr("font-size", "12px")
                .attr("text-anchor", "middle");

            simulation.on("tick", () => {
                link
                    .attr("x1", d => d.source.x)
                    .attr("y1", d => d.source.y)
                    .attr("x2", d => d.target.x)
                    .attr("y2", d => d.target.y);

                node
                    .attr("cx", d => d.x)
                    .attr("cy", d => d.y);

                label
                    .attr("x", d => d.x)
                    .attr("y", d => d.y + 5);
            });

            function dragstarted(event, d) {
                if (!event.active) simulation.alphaTarget(0.3).restart();
                d.fx = d.x;
                d.fy = d.y;
            }

            function dragged(event, d) {
                d.fx = event.x;
                d.fy = event.y;
            }

            function dragended(event, d) {
                if (!event.active) simulation.alphaTarget(0);
                d.fx = null;
                d.fy = null;
            }

            this.charts.set('character-relationship-d3', svg);
        } catch (error) {
            console.error('渲染D3.js图表失败:', error);
            this.renderSimpleRelationshipGraph(container);
        }
    }

    /**
     * 计算角色关系数据
     */
    calculateCharacterRelationships() {
        const nodes = [];
        const edges = [];
        const interactionMap = new Map();

        // 构建角色节点
        if (this.aggregateData && this.aggregateData.characters) {
            this.aggregateData.characters.forEach((character, index) => {
                nodes.push({
                    id: character.id,
                    name: character.name,
                    avatar: character.avatar,
                    interactions: 0,
                    color: this.getCharacterColor(index)
                });
            });
        }

        // 分析对话数据，构建关系边
        if (this.aggregateData && this.aggregateData.recent_conversations) {
            this.aggregateData.recent_conversations.forEach(conv => {
                const charId = conv.character_id;

                // 统计角色互动次数
                const nodeIndex = nodes.findIndex(n => n.id === charId);
                if (nodeIndex !== -1) {
                    nodes[nodeIndex].interactions++;
                }

                // 构建角色间关系
                // 这里简化处理，实际应该分析对话内容中提及的其他角色
                nodes.forEach(otherChar => {
                    if (otherChar.id !== charId && conv.content) {
                        const content = conv.content.toLowerCase();
                        const otherName = otherChar.name.toLowerCase();

                        if (content.includes(otherName)) {
                            const key = `${charId}-${otherChar.id}`;
                            if (!interactionMap.has(key)) {
                                interactionMap.set(key, {
                                    from: charId,
                                    to: otherChar.id,
                                    from_name: nodes.find(n => n.id === charId)?.name || charId,
                                    to_name: otherChar.name,
                                    strength: 1,
                                    type: '提及'
                                });
                            } else {
                                interactionMap.get(key).strength++;
                            }
                        }
                    }
                });
            });
        }

        // 转换关系映射为边数组
        interactionMap.forEach(relation => {
            edges.push(relation);
        });

        // 如果没有真实数据，创建示例数据
        if (nodes.length === 0) {
            const exampleChars = ['主角', '导师', '反派', '伙伴'];
            exampleChars.forEach((name, index) => {
                nodes.push({
                    id: `char_${index}`,
                    name: name,
                    interactions: Math.floor(Math.random() * 20) + 5,
                    color: this.getCharacterColor(index)
                });
            });

            // 添加示例关系
            edges.push(
                { from: 'char_0', to: 'char_1', from_name: '主角', to_name: '导师', strength: 8, type: '师徒' },
                { from: 'char_0', to: 'char_2', from_name: '主角', to_name: '反派', strength: 5, type: '对立' },
                { from: 'char_0', to: 'char_3', from_name: '主角', to_name: '伙伴', strength: 6, type: '友谊' }
            );
        }

        return { nodes, edges };
    }

    /**
     * 获取角色颜色
     */
    getCharacterColor(index) {
        const colors = [
            '#FF6384', '#36A2EB', '#FFCE56', '#4BC0C0',
            '#9966FF', '#FF9F40', '#FF6384', '#C9CBCF'
        ];
        return colors[index % colors.length];
    }

    /**
     * 渲染互动时间线图表
     */
    renderInteractionTimelineChart() {
        if (typeof Chart === 'undefined') {
            console.warn('Chart.js 未加载，无法渲染互动时间线图表');
            return;
        }

        const canvas = document.getElementById('interaction-timeline-chart');
        if (!canvas) {
            console.warn('未找到互动时间线图表容器');
            return;
        }

        // 计算时间线数据
        const timelineData = this.calculateInteractionTimelineData();

        const chart = new Chart(canvas, {
            type: 'bar',
            data: {
                labels: timelineData.labels,
                datasets: timelineData.datasets
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    title: {
                        display: true,
                        text: '角色互动时间线',
                        font: {
                            size: 16,
                            weight: 'bold'
                        }
                    },
                    legend: {
                        position: 'top',
                        labels: {
                            usePointStyle: true,
                            padding: 15
                        }
                    },
                    tooltip: {
                        mode: 'index',
                        intersect: false,
                        callbacks: {
                            label: function (context) {
                                const label = context.dataset.label || '';
                                const value = context.parsed.y;
                                return `${label}: ${value} 次互动`;
                            }
                        }
                    }
                },
                scales: {
                    x: {
                        display: true,
                        title: {
                            display: true,
                            text: '时间段'
                        },
                        stacked: false
                    },
                    y: {
                        display: true,
                        title: {
                            display: true,
                            text: '互动次数'
                        },
                        beginAtZero: true,
                        stacked: false
                    }
                },
                interaction: {
                    mode: 'nearest',
                    axis: 'x',
                    intersect: false
                }
            }
        });

        this.charts.set('interaction-timeline', chart);
    }

    /**
     * 计算互动时间线数据
     */
    calculateInteractionTimelineData() {
        const now = new Date();
        const labels = [];
        const characterData = new Map();

        // 生成过去7天的标签
        for (let i = 6; i >= 0; i--) {
            const date = new Date(now.getTime() - i * 24 * 60 * 60 * 1000);
            labels.push(date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' }));
        }

        // 如果有真实数据，分析对话记录
        if (this.aggregateData && this.aggregateData.recent_conversations) {
            // 按角色分组统计每天的互动
            this.aggregateData.recent_conversations.forEach(conv => {
                const convDate = new Date(conv.timestamp);
                const dayIndex = Math.floor((now - convDate) / (24 * 60 * 60 * 1000));

                if (dayIndex >= 0 && dayIndex < 7) {
                    const labelIndex = 6 - dayIndex;
                    const charId = conv.character_id;

                    if (!characterData.has(charId)) {
                        characterData.set(charId, {
                            name: this.getCharacterName(charId),
                            data: new Array(7).fill(0),
                            color: this.getCharacterColor(Array.from(characterData.keys()).length)
                        });
                    }

                    characterData.get(charId).data[labelIndex]++;
                }
            });
        }

        // 如果没有数据，生成示例数据
        if (characterData.size === 0) {
            const exampleChars = [
                { name: '主角', color: '#FF6384' },
                { name: '导师', color: '#36A2EB' },
                { name: '伙伴', color: '#FFCE56' },
                { name: '反派', color: '#4BC0C0' }
            ];

            exampleChars.forEach((char, index) => {
                const data = labels.map(() => Math.floor(Math.random() * 8) + 1);
                characterData.set(`char_${index}`, {
                    name: char.name,
                    data: data,
                    color: char.color
                });
            });
        }

        // 转换为Chart.js格式
        const datasets = Array.from(characterData.values()).map(char => ({
            label: char.name,
            data: char.data,
            backgroundColor: char.color,
            borderColor: char.color,
            borderWidth: 1,
            borderRadius: 2
        }));

        return { labels, datasets };
    }

    /**
     * 计算平均响应时间（模拟数据）
     */
    calculateAverageResponseTime() {
        // 这里应该基于真实的响应时间数据计算
        // 暂时返回模拟数据
        if (this.aggregateData && this.aggregateData.recent_conversations) {
            // 基于对话数量估算响应时间
            const convCount = this.aggregateData.recent_conversations.length;
            return Math.max(0.5, 3.0 - convCount * 0.1).toFixed(1);
        }

        return (2.5 + Math.random() * 1.5).toFixed(1);
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
        // 使用事件委托避免重复绑定
        document.removeEventListener('click', this.dashboardEventHandler);

        this.dashboardEventHandler = (e) => {
            // 刷新仪表板
            if (e.target.matches('.refresh-dashboard-btn') || e.target.closest('.refresh-dashboard-btn')) {
                e.preventDefault();
                this.refreshDashboard();
            }

            // 切换仪表板显示
            if (e.target.matches('.toggle-dashboard-btn') || e.target.closest('.toggle-dashboard-btn')) {
                e.preventDefault();
                this.toggleDashboard();
            }

            // 导出仪表板报告
            if (e.target.matches('.export-dashboard-btn') || e.target.closest('.export-dashboard-btn')) {
                e.preventDefault();
                this.exportDashboardReport();
            }

            // 智能切换按钮
            if (e.target.matches('.smart-toggle-dashboard-btn') || e.target.closest('.smart-toggle-dashboard-btn')) {
                e.preventDefault();
                this.smartToggleDashboard();
            }

            // 强制刷新按钮
            if (e.target.matches('.force-refresh-dashboard-btn') || e.target.closest('.force-refresh-dashboard-btn')) {
                e.preventDefault();
                this.forceRefreshDashboard();
            }
        };

        document.addEventListener('click', this.dashboardEventHandler);

        // 键盘快捷键
        document.addEventListener('keydown', (e) => {
            // Ctrl/Cmd + D: 切换仪表板
            if ((e.ctrlKey || e.metaKey) && e.key === 'd') {
                e.preventDefault();
                this.toggleDashboard();
            }

            // Ctrl/Cmd + R: 刷新仪表板
            if ((e.ctrlKey || e.metaKey) && e.key === 'r' && this.state.dashboardVisible) {
                e.preventDefault();
                this.refreshDashboard();
            }
        });
    }

    /**
     * 切换仪表板显示/隐藏
     */
    toggleDashboard() {
        const dashboardContainer = this.getDashboardContainer();
        const toggleBtn = document.querySelector('.toggle-dashboard-btn');

        if (!dashboardContainer) {
            console.warn('未找到仪表板容器');
            return;
        }

        // 切换显示状态
        const isVisible = this.state.dashboardVisible;
        this.setState({ dashboardVisible: !isVisible });

        if (this.state.dashboardVisible) {
            // 显示仪表板
            dashboardContainer.style.display = 'block';
            dashboardContainer.classList.add('dashboard-show');
            dashboardContainer.classList.remove('dashboard-hide');

            // 更新按钮图标和提示
            if (toggleBtn) {
                toggleBtn.innerHTML = '<i class="bi bi-eye-slash"></i>';
                toggleBtn.title = '隐藏仪表板';
                toggleBtn.classList.remove('btn-outline-info');
                toggleBtn.classList.add('btn-outline-warning');
            }

            // 延迟渲染图表，确保容器可见
            setTimeout(() => {
                this.renderCharacterInteractionChart();
                this.renderStoryProgressChart();
                this.renderCharacterRelationshipGraph();
                this.renderInteractionTimelineChart();
            }, 300);

            console.log('✅ 仪表板已显示');
        } else {
            // 隐藏仪表板
            dashboardContainer.classList.add('dashboard-hide');
            dashboardContainer.classList.remove('dashboard-show');

            // 更新按钮图标和提示
            if (toggleBtn) {
                toggleBtn.innerHTML = '<i class="bi bi-eye"></i>';
                toggleBtn.title = '显示仪表板';
                toggleBtn.classList.remove('btn-outline-warning');
                toggleBtn.classList.add('btn-outline-info');
            }

            // 延迟隐藏，等待动画完成
            setTimeout(() => {
                dashboardContainer.style.display = 'none';
                // 清理图表资源
                this.destroyCharts();
            }, 300);

            console.log('✅ 仪表板已隐藏');
        }

        // 保存状态到本地存储
        localStorage.setItem('dashboard-visible', this.state.dashboardVisible.toString());
    }

    /**
     * 初始化仪表板显示状态
     */
    initDashboardState() {
        // 从本地存储读取上次的显示状态
        const savedState = localStorage.getItem('dashboard-visible');
        const defaultVisible = true; // 默认显示

        this.setState({
            dashboardVisible: savedState !== null ? savedState === 'true' : defaultVisible
        });

        // 同步按钮状态
        const toggleBtn = document.querySelector('.toggle-dashboard-btn');
        if (toggleBtn) {
            if (this.state.dashboardVisible) {
                toggleBtn.innerHTML = '<i class="bi bi-eye-slash"></i>';
                toggleBtn.title = '隐藏仪表板';
                toggleBtn.classList.add('btn-outline-warning');
            } else {
                toggleBtn.innerHTML = '<i class="bi bi-eye"></i>';
                toggleBtn.title = '显示仪表板';
                toggleBtn.classList.add('btn-outline-info');
            }
        }

        // 应用初始状态
        if (!this.state.dashboardVisible) {
            const dashboardContainer = this.getDashboardContainer();
            if (dashboardContainer) {
                dashboardContainer.style.display = 'none';
            }
        }
    }

    /**
     * 智能切换仪表板（根据内容自动决定）
     */
    smartToggleDashboard() {
        // 如果没有聚合数据，提示用户
        if (!this.aggregateData) {
            Utils.showInfo('正在加载数据，请稍后再试...');
            return;
        }

        // 如果有数据但图表未渲染，先渲染再显示
        if (this.state.dashboardVisible && this.charts.size === 0) {
            Utils.showInfo('正在准备仪表板...');

            setTimeout(() => {
                this.renderSceneDashboard();
            }, 100);
        } else {
            // 正常切换
            this.toggleDashboard();
        }
    }

    /**
     * 强制刷新仪表板
     */
    forceRefreshDashboard() {
        // 先隐藏
        if (this.state.dashboardVisible) {
            this.setState({ dashboardVisible: false });
            const dashboardContainer = this.getDashboardContainer();
            if (dashboardContainer) {
                dashboardContainer.style.display = 'none';
            }
        }

        // 清理现有图表
        this.destroyCharts();

        // 重新显示并渲染
        setTimeout(() => {
            this.setState({ dashboardVisible: true });
            this.renderSceneDashboard();
        }, 200);
    }

    /**
     * 获取仪表板可见状态
     */
    isDashboardVisible() {
        return this.state.dashboardVisible;
    }

    /**
     * 设置仪表板可见状态
     */
    setDashboardVisible(visible) {
        if (this.state.dashboardVisible !== visible) {
            this.toggleDashboard();
        }
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
        this.charts.forEach((chart, key) => {
            try {
                // Chart.js 图表
                if (chart && typeof chart.destroy === 'function') {
                    chart.destroy();
                }
                // vis.js 网络图
                else if (chart && typeof chart.destroy === 'function') {
                    chart.destroy();
                }
                // D3.js 图表
                else if (chart && chart.remove) {
                    chart.remove();
                }
                // 其他清理
                else if (chart && chart.clear) {
                    chart.clear();
                }
            } catch (error) {
                console.warn(`清理图表 ${key} 时出错:`, error);
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
     * 加载可用的LLM提供商列表
     */
    async loadLLMProviders() {
        try {
            // Since there's no direct API to get all providers, we'll hardcode the known providers
            // but in a real implementation, there would be an API to fetch all available providers
            const providerSelect = document.getElementById('llm-provider');
            if (!providerSelect) return;

            // Clear existing options except the first one
            const firstOption = providerSelect.querySelector('option:first-child');
            providerSelect.innerHTML = '';
            
            if (firstOption) {
                providerSelect.appendChild(firstOption);
            }

            // Known providers that are implemented in the system
            const providers = [
                { value: 'openai', text: 'OpenAI' },
                { value: 'anthropic', text: 'Anthropic (Claude)' },
                { value: 'google', text: 'Google Gemini' },
                { value: 'qwen', text: 'Alibaba Qwen' },
                { value: 'mistral', text: 'Mistral AI' },
                { value: 'deepseek', text: 'DeepSeek' },
                { value: 'glm', text: 'Zhipu AI (GLM)' },
                { value: 'githubmodels', text: 'GitHub Models' },
                { value: 'grok', text: 'xAI (Grok)' },
                { value: 'openrouter', text: 'OpenRouter' },
                { value: 'local', text: '本地模型 (Ollama/Llama.cpp)' }
            ];

            providers.forEach(provider => {
                const option = document.createElement('option');
                option.value = provider.value;
                option.textContent = provider.text;
                providerSelect.appendChild(option);
            });

        } catch (error) {
            console.error('加载提供商列表失败:', error);
        }
    }

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
                const modelSelect = document.getElementById('model-select');
                const modelNameInput = document.getElementById('model-name');
                
                if (modelSelect) {
                    // 尝试在下拉列表中找到匹配的模型
                    if (modelSelect.querySelector(`option[value="${settings.llm_config.model}"]`)) {
                        modelSelect.value = settings.llm_config.model;
                    } else {
                        // 如果模型不在下拉列表中，清空下拉框并设置到手动输入框
                        modelSelect.value = '';
                        if (modelNameInput) {
                            modelNameInput.value = settings.llm_config.model;
                        }
                    }
                } else if (modelNameInput) {
                    modelNameInput.value = settings.llm_config.model;
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
            const modelSelect = document.getElementById('model-select');
            const modelNameInput = document.getElementById('model-name');
            const refreshBtn = document.getElementById('refresh-models');
            const modelLoading = document.getElementById('model-loading');

            if (modelSelect) {
                modelSelect.disabled = false;
                // Enable manual input as well
                if (modelNameInput) {
                    modelNameInput.disabled = false;
                }
                
                if (result && result.models) {
                    modelSelect.innerHTML = '<option value="">选择模型</option>';

                    result.models.forEach(model => {
                        const option = document.createElement('option');
                        option.value = model;
                        option.textContent = model;
                        modelSelect.appendChild(option);
                    });
                } else {
                    // If no models returned, show a message
                    modelSelect.innerHTML = '<option value="">无可用模型</option>';
                }
            }
            
            if (modelLoading) {
                modelLoading.classList.add('d-none');
            }
        } catch (error) {
            console.error('加载模型列表失败:', error);
            const modelSelect = document.getElementById('model-select');
            const modelNameInput = document.getElementById('model-name');
            const modelLoading = document.getElementById('model-loading');
            
            if (modelSelect) {
                modelSelect.innerHTML = '<option value="">加载失败</option>';
                modelSelect.disabled = false;
            }
            
            if (modelNameInput) {
                modelNameInput.disabled = false;
            }
            
            if (modelLoading) {
                modelLoading.classList.add('d-none');
            }
        }
    }

    /**
     * 保存设置
     */
    async saveSettings() {
        try {
            const form = document.getElementById('settings-form');
            const formData = new FormData(form);

            // Determine which model field to use
            let modelValue = '';
            
            // Check manual input first
            const manualModel = formData.get('model'); // This is the manual input field
            const selectModel = formData.get('model-select'); // This is the dropdown
            
            // Use manual input if it has a value, otherwise use the dropdown selection
            if (manualModel && manualModel.trim() !== '') {
                modelValue = manualModel.trim();
            } else if (selectModel && selectModel !== '') {
                modelValue = selectModel;
            }

            const settings = {
                llm_provider: formData.get('llm_provider'),
                llm_config: {
                    api_key: formData.get('api_key'),
                    model: modelValue
                },
                debug_mode: formData.get('debug_mode') === 'on',
                auto_save: formData.get('auto_save') === 'on',
                error_reporting: formData.get('error_reporting') === 'on',
                performance_monitoring: formData.get('performance_monitoring') === 'on'
            };

            await API.saveSettings(settings);
            
            // Update the status indicator
            const statusSpan = document.getElementById('settings-status');
            if (statusSpan) {
                statusSpan.textContent = '已保存';
                statusSpan.className = 'badge bg-success';
                
                // Reset after 3 seconds
                setTimeout(() => {
                    if (statusSpan) {
                        statusSpan.textContent = '已保存';
                        statusSpan.className = 'badge bg-secondary';
                    }
                }, 3000);
            }
            
            Utils.showSuccess('设置保存成功');
            console.log('Settings saved successfully');

            // 更新连接状态
            await this.updateConnectionStatus();

        } catch (error) {
            Utils.showError('保存设置失败: ' + error.message);
            console.error('保存设置失败:', error);
            
            // Update the status indicator to show error
            const statusSpan = document.getElementById('settings-status');
            if (statusSpan) {
                statusSpan.textContent = '保存失败';
                statusSpan.className = 'badge bg-danger';
                
                // Reset after 5 seconds
                setTimeout(() => {
                    if (statusSpan) {
                        statusSpan.textContent = '未保存';
                        statusSpan.className = 'badge bg-secondary';
                    }
                }, 5000);
            }
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
        if (!characterId) return '未知角色';

        // 安全地访问当前场景数据
        if (this.currentScene && Array.isArray(this.currentScene.characters)) {
            const character = this.currentScene.characters.find(c => c.id === characterId);
            if (character && character.name) {
                return character.name;
            }
        }

        // 如果在当前场景中找不到，尝试在其他可能的来源中查找
        if (this.aggregateData && this.aggregateData.characters) {
            const character = this.aggregateData.characters.find(c => c.id === characterId);
            if (character && character.name) {
                return character.name;
            }
        }

        // 如果都找不到，返回默认值
        return `角色${characterId}`;
    }

    /**
     * 获取角色头像
     */
    getCharacterAvatar(characterId) {
        if (!characterId) return null;

        // 安全地访问当前场景数据
        if (this.currentScene && Array.isArray(this.currentScene.characters)) {
            const character = this.currentScene.characters.find(c => c.id === characterId);
            if (character && character.avatar) {
                return character.avatar;
            }
        }

        // 如果在当前场景中找不到，尝试在其他可能的来源中查找
        if (this.aggregateData && this.aggregateData.characters) {
            const character = this.aggregateData.characters.find(c => c.id === characterId);
            if (character && character.avatar) {
                return character.avatar;
            }
        }

        // 默认头像
        return null;
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
            // 修正：使用正确的导出API方法
            const result = await API.exportSceneData(
                this.currentScene.id,
                format,
                true // includeConversations
            );

            // 处理不同的响应格式
            let content, mimeType, filename;

            if (typeof result === 'string') {
                content = result;
            } else if (result.content) {
                content = result.content;
            } else {
                content = JSON.stringify(result, null, 2);
            }

            // 设置MIME类型
            switch (format) {
                case 'json':
                    mimeType = 'application/json';
                    break;
                case 'txt':
                    mimeType = 'text/plain';
                    break;
                case 'md':
                    mimeType = 'text/markdown';
                    break;
                default:
                    mimeType = 'application/octet-stream';
            }

            filename = `scene_${this.currentScene.id}_${Date.now()}.${format}`;

            // 创建下载链接
            const blob = new Blob([content], { type: mimeType });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = filename;
            a.click();
            URL.revokeObjectURL(url);

            Utils.showSuccess('场景数据导出成功');
        } catch (error) {
            Utils.showError('导出失败: ' + error.message);
        }
    }

    /**
     * 导出仪表板报告
     */
    async exportDashboardReport() {
        try {
            Utils.showSuccess('正在生成报告...');

            // 获取仪表板数据
            const stats = this.calculateSceneStats();
            const interactionData = this.calculateCharacterInteractions();

            const report = {
                scene: {
                    id: this.currentScene.id,
                    name: this.currentScene.name,
                    created_at: this.currentScene.created_at
                },
                stats: stats,
                character_interactions: interactionData,
                conversations_count: this.conversations.length,
                export_time: new Date().toISOString()
            };

            const content = JSON.stringify(report, null, 2);
            const blob = new Blob([content], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `scene_${this.currentScene.id}_dashboard_report.json`;
            a.click();
            URL.revokeObjectURL(url);

            Utils.showSuccess('仪表板报告导出成功');
        } catch (error) {
            Utils.showError('导出报告失败: ' + error.message);
        }
    }

    /**
    * 增强错误处理 - 匹配后端错误响应格式
    */
    handleAPIError(error, operation = '操作') {
        let errorMessage = `${operation}失败`;

        if (error.response) {
            // HTTP错误响应
            if (error.response.data?.error) {
                errorMessage += `: ${error.response.data.error}`;
            } else if (error.response.data?.message) {
                errorMessage += `: ${error.response.data.message}`;
            } else {
                errorMessage += `: HTTP ${error.response.status}`;
            }
        } else if (error.message) {
            errorMessage += `: ${error.message}`;
        }

        console.error(`${operation}失败:`, error);
        Utils.showError(errorMessage);

        return errorMessage;
    }

    /**
     * 更新统计显示 - 新增方法
    */
    updateStats(stats) {
        if (!stats) return;

        // 更新统计显示
        const statsContainer = document.querySelector('.stats-cards-container');
        if (statsContainer && stats) {
            // 重新渲染统计卡片
            statsContainer.innerHTML = this.renderStatsCards();
        }

        // 更新图表数据
        if (this.charts.has('character-interaction')) {
            this.renderCharacterInteractionChart(stats.character_interactions);
        }
    }

    /**
     * 初始化实时通信 - 完整实现
     */
    async initRealtimeConnection(sceneId) {
        try {
            // 1. 检查 RealtimeManager 是否可用
            if (typeof window.RealtimeManager === 'undefined') {
                console.warn('⚠️ RealtimeManager 未加载，跳过实时通信初始化');
                return false;
            }

            // 2. 获取或创建 RealtimeManager 实例
            if (!window.realtimeManager) {
                window.realtimeManager = new window.RealtimeManager();
                console.log('🔗 RealtimeManager 实例已创建');
            }

            console.log(`🔄 正在初始化场景 ${sceneId} 的实时通信...`);

            // 3. 初始化场景实时功能
            const success = await window.realtimeManager.initSceneRealtime(sceneId);

            if (!success) {
                console.warn('⚠️ 场景实时功能初始化失败');
                return false;
            }

            // 4. 设置应用级别的事件监听器
            this.setupRealtimeEventListeners(sceneId);

            // 5. 绑定实时消息处理器
            this.bindRealtimeMessageHandlers();

            // 6. 初始化实时状态管理
            this.initRealtimeStateManagement();

            // 7. 设置连接状态监控
            this.setupConnectionMonitoring();

            // 8. 保存实时管理器引用
            this.realtimeManager = window.realtimeManager;

            console.log('✅ 实时通信初始化完成');

            // 9. 显示连接状态
            this.updateRealtimeStatus('connected', '实时通信已启用');

            return true;

        } catch (error) {
            console.error('❌ 实时通信初始化失败:', error);
            this.updateRealtimeStatus('error', '实时通信初始化失败: ' + error.message);
            return false;
        }
    }

    /**
     * 设置实时事件监听器
     */
    setupRealtimeEventListeners(sceneId) {
        if (!this.realtimeManager) return;

        // 监听新对话消息
        this.realtimeManager.on('conversation:new', (data) => {
            if (data.sceneId === sceneId) {
                this.handleNewConversation(data);
            }
        });

        // 监听角色状态更新
        this.realtimeManager.on('character:status_updated', (data) => {
            if (data.sceneId === sceneId) {
                this.handleCharacterStatusUpdate(data);
            }
        });

        // 监听故事事件
        this.realtimeManager.on('story:event', (data) => {
            if (data.sceneId === sceneId) {
                this.handleStoryEvent(data);
            }
        });

        // 监听用户在线状态
        this.realtimeManager.on('user:presence', (data) => {
            if (data.sceneId === sceneId) {
                this.handleUserPresence(data);
            }
        });

        // 监听场景状态更新
        this.realtimeManager.on('scene:state_updated', (data) => {
            if (data.sceneId === sceneId) {
                this.handleSceneStateUpdate(data);
            }
        });

        // 监听连接状态
        this.realtimeManager.on('scene:connected', (data) => {
            if (data.sceneId === sceneId) {
                this.updateRealtimeStatus('connected', '场景连接已建立');
            }
        });

        console.log('📡 实时事件监听器已设置');
    }

    /**
     * 绑定实时消息处理器
     */
    bindRealtimeMessageHandlers() {
        // 绑定发送消息功能
        const sendBtn = document.getElementById('send-btn');
        const messageInput = document.getElementById('message-input');

        if (sendBtn && messageInput) {
            // 移除旧的事件监听器
            const newSendBtn = sendBtn.cloneNode(true);
            sendBtn.parentNode.replaceChild(newSendBtn, sendBtn);

            // 绑定新的实时发送逻辑
            newSendBtn.addEventListener('click', () => {
                this.sendRealtimeMessage();
            });

            messageInput.addEventListener('keypress', (e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    this.sendRealtimeMessage();
                }
            });

            console.log('💬 实时消息处理器已绑定');
        }

        // 绑定角色选择功能
        document.addEventListener('click', (e) => {
            if (e.target.closest('.character-item')) {
                const characterItem = e.target.closest('.character-item');
                const characterId = characterItem.dataset.characterId;
                this.selectCharacterForRealtime(characterId);
            }
        });
    }

    /**
     * 初始化实时状态管理
     */
    initRealtimeStateManagement() {
        // 创建实时状态存储
        if (!this.realtimeState) {
            this.realtimeState = {
                connected: false,
                selectedCharacter: null,
                lastActivity: Date.now(),
                messageQueue: [],
                userStatus: 'active'
            };
        }

        // 定期更新用户活动状态
        this.activityTimer = setInterval(() => {
            this.updateUserActivity();
        }, 30000); // 每30秒更新一次

        console.log('📊 实时状态管理已初始化');
    }

    /**
     * 设置连接状态监控
     */
    setupConnectionMonitoring() {
        if (!this.realtimeManager) return;

        // 监听心跳事件
        this.realtimeManager.on('heartbeat', (data) => {
            this.updateConnectionMetrics(data);
        });

        // 定期检查连接状态
        this.connectionCheckTimer = setInterval(() => {
            this.checkRealtimeConnection();
        }, 60000); // 每分钟检查一次

        console.log('🔍 连接状态监控已启用');
    }

    /**
     * 发送实时消息
     */
    sendRealtimeMessage() {
        const messageInput = document.getElementById('message-input');
        const message = messageInput?.value?.trim();

        if (!message) return;

        const selectedCharacter = this.getSelectedCharacter();
        if (!selectedCharacter) {
            Utils.showWarning('请先选择一个角色');
            return;
        }

        const sceneId = this.getSceneIdFromPage();
        if (!sceneId) {
            Utils.showError('无法获取场景ID');
            return;
        }

        // 通过实时管理器发送消息
        const success = this.realtimeManager.sendCharacterInteraction(
            sceneId,
            selectedCharacter,
            message,
            {
                user_id: this.getCurrentUserId(),
                timestamp: Date.now(),
                message_type: 'chat'
            }
        );

        if (success) {
            // 清空输入框
            messageInput.value = '';

            // 更新UI状态
            this.setInputState(false);

            // 显示发送状态
            this.showMessageSendingStatus(message);

            // 消息发送计数
            this.incrementMessageSentCount();

            console.log('📤 实时消息已发送:', { sceneId, selectedCharacter, message });
        } else {
            Utils.showError('消息发送失败，请检查连接状态');
        }
    }

    /**
     * 选择角色进行实时互动
     */
    selectCharacterForRealtime(characterId) {
        // 更新本地状态
        this.realtimeState.selectedCharacter = characterId;

        // 更新UI显示
        document.querySelectorAll('.character-item').forEach(item => {
            item.classList.remove('selected', 'border-primary');
        });

        const characterElement = document.querySelector(`[data-character-id="${characterId}"]`);
        if (characterElement) {
            characterElement.classList.add('selected', 'border-primary');

            // 更新选择状态显示
            const characterName = characterElement.querySelector('.fw-bold')?.textContent;
            const selectedDisplay = document.getElementById('selected-character');
            if (selectedDisplay) {
                selectedDisplay.textContent = `已选择: ${characterName}`;
            }

            // 启用输入控件
            this.setInputState(true);
        }

        // 通知实时管理器角色选择变化
        const sceneId = this.getSceneIdFromPage();
        if (sceneId && this.realtimeManager) {
            this.realtimeManager.sendCharacterStatusUpdate(sceneId, characterId, 'selected');
        }

        console.log('👤 已选择角色进行实时互动:', characterId);
    }

    /**
     * 处理新对话消息 - 增强版
     */
    handleNewConversation(data) {
        const { conversation, speakerId, message, timestamp } = data;

        console.log('📨 收到新对话:', data);

        // 调用现有的添加对话方法
        if (this.addConversationToUI) {
            this.addConversationToUI(conversation);
        } else {
            // 降级处理
            this.addMessageToChat(conversation);
        }

        // 如果不是当前用户发送的消息，播放提示音
        if (speakerId !== this.getCurrentUserId()) {
            this.playNotificationSound();

            // 显示新消息通知
            this.showNewMessageNotification(conversation);
        }

        // 更新统计信息
        this.updateConversationStats();

        // 自动滚动到最新消息
        this.scrollToLatestMessage();
    }

    /**
     * 添加对话到UI - 核心方法
     */
    addConversationToUI(conversation) {
        if (!conversation) {
            console.warn('无效的对话数据');
            return;
        }

        try {
            console.log('📨 添加对话到UI:', conversation);

            // 更新本地对话数据
            this.addConversationToLocal(conversation);

            // 查找对话容器
            const conversationContainer = this.getConversationContainer();
            if (!conversationContainer) {
                console.error('找不到对话容器');
                this.createConversationContainer();
                return;
            }

            // 创建对话元素
            const conversationElement = this.createConversationElement(conversation);

            // 检查是否为重复消息
            if (this.isDuplicateMessage(conversation, conversationContainer)) {
                console.log('跳过重复消息:', conversation.id);
                return;
            }

            // 添加到容器
            conversationContainer.appendChild(conversationElement);

            // 应用动画效果
            this.animateNewConversation(conversationElement);

            // 更新对话计数
            this.updateConversationCount();

            // 自动滚动到最新消息
            this.scrollToLatestMessage();

            // 更新最后活动时间
            this.updateLastActivity();

            // 触发对话添加事件
            this.triggerConversationEvent('conversation_added', {
                conversation,
                element: conversationElement,
                timestamp: Date.now()
            });

            console.log('✅ 对话已添加到UI');

        } catch (error) {
            console.error('❌ 添加对话到UI失败:', error);
            // 降级处理
            this.addMessageToChat(conversation);
        }
    }

    /**
     * 更新最后活动时间
     */
    updateLastActivity() {
        const now = Date.now();

        // 更新本地状态
        if (this.realtimeState) {
            this.realtimeState.lastActivity = now;
        }

        // 更新全局最后活动时间
        this.lastActivityTime = now;

        // 更新本地存储（可选）
        try {
            localStorage.setItem('last_activity', now.toString());
        } catch (error) {
            console.warn('保存最后活动时间失败:', error);
        }

        // 更新UI显示
        this.updateLastActivityDisplay(now);

        // 发送用户活动状态到实时服务
        this.sendUserActivityStatus();

        // 重置空闲计时器
        this.resetIdleTimer();

        console.log('📱 最后活动时间已更新:', new Date(now).toLocaleTimeString());
    }

    /**
     * 更新最后活动时间显示
     */
    updateLastActivityDisplay(timestamp) {
        const displayElements = document.querySelectorAll('.last-activity-time, #last-activity-time');
        displayElements.forEach(element => {
            element.textContent = this.formatLastActivity(timestamp);
            element.title = `最后活动: ${new Date(timestamp).toLocaleString()}`;
        });

        // 更新页面标题中的活动状态（可选）
        this.updatePageActivityStatus(timestamp);
    }

    /**
     * 格式化最后活动时间
     */
    formatLastActivity(timestamp) {
        if (!timestamp) return '从未活动';

        const now = Date.now();
        const diffMs = now - timestamp;
        const diffSeconds = Math.floor(diffMs / 1000);
        const diffMinutes = Math.floor(diffSeconds / 60);
        const diffHours = Math.floor(diffMinutes / 60);

        if (diffSeconds < 10) {
            return '刚刚活动';
        } else if (diffSeconds < 60) {
            return `${diffSeconds}秒前`;
        } else if (diffMinutes < 60) {
            return `${diffMinutes}分钟前`;
        } else if (diffHours < 24) {
            return `${diffHours}小时前`;
        } else {
            const days = Math.floor(diffHours / 24);
            return `${days}天前`;
        }
    }

    /**
     * 发送用户活动状态
     */
    sendUserActivityStatus() {
        if (!this.realtimeManager || !this.realtimeState) {
            return;
        }

        // 发送用户状态更新
        this.realtimeManager.sendUserStatusUpdate('active', 'user_activity', {
            last_activity: this.realtimeState.lastActivity,
            scene_id: this.getSceneIdFromPage(),
            user_agent: navigator.userAgent,
            timestamp: Date.now()
        });
    }

    /**
     * 重置空闲计时器
     */
    resetIdleTimer() {
        // 清除现有的空闲计时器
        if (this.idleTimer) {
            clearTimeout(this.idleTimer);
        }

        // 设置新的空闲计时器（5分钟后标记为空闲）
        this.idleTimer = setTimeout(() => {
            this.markUserAsIdle();
        }, 5 * 60 * 1000); // 5分钟

        // 如果用户之前是空闲状态，现在变为活跃
        if (this.userIdleState === 'idle') {
            this.markUserAsActive();
        }
    }

    /**
     * 标记用户为空闲状态
     */
    markUserAsIdle() {
        this.userIdleState = 'idle';

        console.log('😴 用户进入空闲状态');

        // 发送空闲状态到实时服务
        if (this.realtimeManager) {
            this.realtimeManager.sendUserStatusUpdate('idle', 'user_idle', {
                idle_since: Date.now(),
                scene_id: this.getSceneIdFromPage()
            });
        }

        // 更新UI显示
        this.updateUserIdleUI(true);

        // 触发空闲事件
        this.triggerUserEvent('user_idle', {
            idle_since: Date.now(),
            last_activity: this.lastActivityTime
        });
    }

    /**
     * 标记用户为活跃状态
     */
    markUserAsActive() {
        const wasIdle = this.userIdleState === 'idle';
        this.userIdleState = 'active';

        if (wasIdle) {
            console.log('🔥 用户从空闲状态恢复活跃');

            // 发送活跃状态到实时服务
            if (this.realtimeManager) {
                this.realtimeManager.sendUserStatusUpdate('active', 'user_active', {
                    active_since: Date.now(),
                    scene_id: this.getSceneIdFromPage()
                });
            }

            // 更新UI显示
            this.updateUserIdleUI(false);

            // 触发活跃事件
            this.triggerUserEvent('user_active', {
                active_since: Date.now(),
                was_idle_duration: Date.now() - this.lastActivityTime
            });
        }
    }

    /**
     * 更新用户空闲状态UI
     */
    updateUserIdleUI(isIdle) {
        // 更新状态指示器
        const statusIndicators = document.querySelectorAll('.user-status-indicator, #user-status-indicator');
        statusIndicators.forEach(indicator => {
            indicator.className = `user-status-indicator ${isIdle ? 'idle' : 'active'}`;
            indicator.innerHTML = `
            <span class="status-dot ${isIdle ? 'idle' : 'active'}"></span>
            <span class="status-text">${isIdle ? '空闲中' : '活跃'}</span>
        `;
            indicator.title = isIdle ? '用户当前处于空闲状态' : '用户当前活跃';
        });

        // 更新页面可见性（可选）
        if (isIdle) {
            document.body.classList.add('user-idle');
        } else {
            document.body.classList.remove('user-idle');
        }
    }

    /**
     * 更新页面活动状态
     */
    updatePageActivityStatus(timestamp) {
        // 可以在页面标题中显示活动状态
        const originalTitle = document.title.replace(/ - \d+分钟前活动$/, '');

        const activityText = this.formatLastActivity(timestamp);
        if (activityText !== '刚刚活动') {
            document.title = `${originalTitle} - ${activityText}活动`;
        } else {
            document.title = originalTitle;
        }
    }

    /**
     * 初始化用户活动监控
     */
    initUserActivityMonitoring() {
        console.log('📊 初始化用户活动监控');

        // 初始化状态
        this.lastActivityTime = Date.now();
        this.userIdleState = 'active';
        this.idleTimer = null;

        // 从本地存储恢复最后活动时间
        try {
            const savedActivity = localStorage.getItem('last_activity');
            if (savedActivity) {
                this.lastActivityTime = parseInt(savedActivity, 10);
            }
        } catch (error) {
            console.warn('读取最后活动时间失败:', error);
        }

        // 监听用户活动事件
        const activityEvents = [
            'mousedown', 'mousemove', 'keypress', 'scroll',
            'touchstart', 'click', 'focus', 'blur'
        ];

        // 使用防抖来避免过于频繁的更新
        const debouncedUpdateActivity = this.debounce(() => {
            this.updateLastActivity();
        }, 1000); // 1秒内最多更新一次

        activityEvents.forEach(eventType => {
            document.addEventListener(eventType, debouncedUpdateActivity, {
                passive: true,
                capture: false
            });
        });

        // 监听页面可见性变化
        document.addEventListener('visibilitychange', () => {
            if (document.visibilityState === 'visible') {
                // 页面变为可见时更新活动时间
                this.updateLastActivity();
            } else {
                // 页面变为隐藏时可以考虑标记为空闲
                setTimeout(() => {
                    if (document.visibilityState === 'hidden') {
                        this.markUserAsIdle();
                    }
                }, 2000); // 2秒后如果页面仍然隐藏则标记为空闲
            }
        });

        // 监听窗口焦点变化
        window.addEventListener('focus', () => {
            this.markUserAsActive();
            this.updateLastActivity();
        });

        window.addEventListener('blur', () => {
            // 窗口失去焦点时不立即标记为空闲，等待空闲计时器
        });

        // 初始更新
        this.updateLastActivity();

        console.log('✅ 用户活动监控已启动');
    }

    /**
     * 停止用户活动监控
     */
    stopUserActivityMonitoring() {
        console.log('🛑 停止用户活动监控');

        // 清除空闲计时器
        if (this.idleTimer) {
            clearTimeout(this.idleTimer);
            this.idleTimer = null;
        }

        // 这里可以移除事件监听器，但由于使用了debounce，
        // 移除会比较复杂，通常在页面卸载时会自动清理
    }

    /**
     * 获取用户活动统计
     */
    getUserActivityStats() {
        const now = Date.now();
        const sessionStart = this.sessionStartTime || now;
        const sessionDuration = now - sessionStart;
        const lastActivity = this.lastActivityTime || now;
        const timeSinceLastActivity = now - lastActivity;

        return {
            session_duration: sessionDuration,
            session_duration_text: this.formatDuration(sessionDuration),
            last_activity: lastActivity,
            time_since_last_activity: timeSinceLastActivity,
            time_since_last_activity_text: this.formatLastActivity(lastActivity),
            current_state: this.userIdleState || 'active',
            scene_id: this.getSceneIdFromPage(),
            conversation_count: this.conversations ? this.conversations.length : 0,
            messages_sent: this.messagesSentCount || 0
        };
    }

    /**
     * 格式化持续时间
     */
    formatDuration(milliseconds) {
        const seconds = Math.floor(milliseconds / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);

        if (hours > 0) {
            return `${hours}小时${minutes % 60}分钟`;
        } else if (minutes > 0) {
            return `${minutes}分钟${seconds % 60}秒`;
        } else {
            return `${seconds}秒`;
        }
    }

    /**
     * 防抖函数
     */
    debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func.apply(this, args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    }

    /**
     * 触发用户事件
     */
    triggerUserEvent(eventType, eventData) {
        // 触发自定义事件
        const event = new CustomEvent(eventType, {
            detail: eventData
        });
        document.dispatchEvent(event);

        // 如果有实时管理器，也通过它触发事件
        if (this.realtimeManager && this.realtimeManager.emit) {
            this.realtimeManager.emit(eventType, eventData);
        }
    }

    /**
     * 更新消息发送计数
     */
    incrementMessageSentCount() {
        this.messagesSentCount = (this.messagesSentCount || 0) + 1;

        // 更新活动时间
        this.updateLastActivity();

        // 保存到本地存储
        try {
            localStorage.setItem('messages_sent_count', this.messagesSentCount.toString());
        } catch (error) {
            console.warn('保存消息计数失败:', error);
        }
    }

    /**
     * 获取会话开始时间
     */
    getSessionStartTime() {
        if (!this.sessionStartTime) {
            this.sessionStartTime = Date.now();

            // 保存到本地存储
            try {
                localStorage.setItem('session_start_time', this.sessionStartTime.toString());
            } catch (error) {
                console.warn('保存会话开始时间失败:', error);
            }
        }

        return this.sessionStartTime;
    }

    /**
     * 初始化会话时间管理
     */
    initSessionTimeManagement() {
        // 尝试从本地存储恢复会话开始时间
        try {
            const savedSessionStart = localStorage.getItem('session_start_time');
            if (savedSessionStart) {
                const sessionStart = parseInt(savedSessionStart, 10);
                const now = Date.now();

                // 如果会话时间超过6小时，重新开始新会话
                if (now - sessionStart > 6 * 60 * 60 * 1000) {
                    this.sessionStartTime = now;
                    localStorage.setItem('session_start_time', now.toString());
                } else {
                    this.sessionStartTime = sessionStart;
                }
            } else {
                this.sessionStartTime = Date.now();
                localStorage.setItem('session_start_time', this.sessionStartTime.toString());
            }
        } catch (error) {
            console.warn('初始化会话时间管理失败:', error);
            this.sessionStartTime = Date.now();
        }

        // 尝试恢复消息发送计数
        try {
            const savedMessageCount = localStorage.getItem('messages_sent_count');
            if (savedMessageCount) {
                this.messagesSentCount = parseInt(savedMessageCount, 10);
            } else {
                this.messagesSentCount = 0;
            }
        } catch (error) {
            console.warn('恢复消息计数失败:', error);
            this.messagesSentCount = 0;
        }

        console.log('📅 会话时间管理已初始化');
    }

    /**
     * 清理会话数据
     */
    clearSessionData() {
        this.sessionStartTime = Date.now();
        this.messagesSentCount = 0;
        this.lastActivityTime = Date.now();

        // 清理本地存储
        try {
            localStorage.removeItem('session_start_time');
            localStorage.removeItem('messages_sent_count');
            localStorage.removeItem('last_activity');
        } catch (error) {
            console.warn('清理会话数据失败:', error);
        }

        console.log('🧹 会话数据已清理');
    }

    /**
     * 监听页面卸载，保存最后活动时间
     */
    setupActivityPersistence() {
        // 监听页面卸载事件
        window.addEventListener('beforeunload', () => {
            // 更新最后活动时间
            this.updateLastActivity();

            // 发送最终状态
            if (this.realtimeManager) {
                this.realtimeManager.sendUserStatusUpdate('offline', 'user_leaving', {
                    session_duration: Date.now() - this.sessionStartTime,
                    messages_sent: this.messagesSentCount,
                    final_activity: Date.now()
                });
            }
        });

        // 监听页面隐藏事件（移动端友好）
        document.addEventListener('visibilitychange', () => {
            if (document.visibilityState === 'hidden') {
                this.updateLastActivity();
            }
        });
    }

    /**
     * 获取或创建对话容器
     */
    getConversationContainer() {
        // 按优先级查找对话容器
        const selectors = [
            '#conversation-history',
            '#chat-messages',
            '.conversation-container',
            '.chat-container',
            '.messages-container',
            '#messages'
        ];

        for (const selector of selectors) {
            const container = document.querySelector(selector);
            if (container) {
                return container;
            }
        }

        // 如果没找到，尝试创建一个
        return this.createConversationContainer();
    }

    /**
     * 创建对话容器
     */
    createConversationContainer() {
        console.log('📦 创建新的对话容器');

        const container = document.createElement('div');
        container.id = 'conversation-history';
        container.className = 'conversation-container';
        container.style.cssText = `
        max-height: 400px;
        overflow-y: auto;
        padding: 1rem;
        background: #f8f9fa;
        border: 1px solid #dee2e6;
        border-radius: 0.5rem;
        margin-bottom: 1rem;
    `;

        // 添加标题
        const title = document.createElement('h5');
        title.textContent = '对话历史';
        title.className = 'mb-3';

        container.appendChild(title);

        // 查找合适的位置插入
        const targetContainer = this.findInsertionPoint();
        if (targetContainer) {
            targetContainer.appendChild(container);
        } else {
            document.body.appendChild(container);
        }

        return container;
    }

    /**
     * 查找容器插入位置
     */
    findInsertionPoint() {
        // 查找场景内容区域
        const sceneContent = document.querySelector('.scene-content, .main-content, .container, main');
        if (sceneContent) {
            return sceneContent;
        }

        // 查找聊天界面区域
        const chatInterface = document.querySelector('.chat-interface, .chat-container');
        if (chatInterface) {
            return chatInterface.parentNode;
        }

        return null;
    }

    /**
     * 创建对话元素
     */
    createConversationElement(conversation) {
        const element = document.createElement('div');
        element.className = 'conversation-item mb-3';
        element.setAttribute('data-conversation-id', conversation.id || this.generateConversationId());
        element.setAttribute('data-speaker-id', conversation.speaker_id || conversation.speakerId || 'unknown');

        // 获取说话者信息
        const speakerInfo = this.getSpeakerInfo(conversation);

        // 格式化时间
        const timestamp = this.formatConversationTime(conversation.timestamp || Date.now());

        // 构建HTML内容
        element.innerHTML = `
        <div class="conversation-header d-flex align-items-center mb-2">
            <div class="speaker-avatar me-2">
                ${speakerInfo.avatar ?
                `<img src="${speakerInfo.avatar}" alt="${speakerInfo.name}" class="rounded-circle" width="32" height="32">` :
                `<div class="avatar-placeholder rounded-circle d-flex align-items-center justify-content-center" style="width: 32px; height: 32px; background: ${speakerInfo.color}; color: white; font-size: 14px; font-weight: 600;">
                        ${speakerInfo.initial}
                    </div>`
            }
            </div>
            <div class="speaker-info flex-grow-1">
                <div class="speaker-name fw-bold text-primary">${speakerInfo.name}</div>
                <div class="conversation-time text-muted small">${timestamp}</div>
            </div>
            <div class="conversation-actions">
                <button class="btn btn-sm btn-outline-secondary copy-conversation-btn" title="复制对话">
                    <i class="bi bi-clipboard"></i>
                </button>
            </div>
        </div>
        <div class="conversation-content">
            <div class="message-text">${this.formatConversationContent(conversation)}</div>
            ${this.renderConversationMetadata(conversation)}
        </div>
    `;

        // 绑定事件
        this.bindConversationElementEvents(element, conversation);

        return element;
    }

    /**
     * 获取说话者信息
     */
    getSpeakerInfo(conversation) {
        const speakerId = conversation.speaker_id || conversation.speakerId || conversation.character_id;

        // 默认信息
        let speakerInfo = {
            id: speakerId,
            name: '未知说话者',
            avatar: null,
            color: '#6c757d',
            initial: '?'
        };

        // 如果是用户消息
        if (conversation.message_type === 'user' || conversation.type === 'user') {
            speakerInfo = {
                id: 'user',
                name: '你',
                avatar: null,
                color: '#007bff',
                initial: '我'
            };
        }
        // 如果是角色消息
        else if (speakerId && this.currentScene?.characters) {
            const character = this.currentScene.characters.find(c => c.id === speakerId);
            if (character) {
                speakerInfo = {
                    id: character.id,
                    name: character.name,
                    avatar: character.avatar,
                    color: this.getCharacterColor(character.id),
                    initial: character.name.charAt(0).toUpperCase()
                };
            }
        }
        // 如果有直接的说话者名称
        else if (conversation.speaker_name || conversation.speakerName) {
            const name = conversation.speaker_name || conversation.speakerName;
            speakerInfo = {
                id: speakerId || 'unknown',
                name: name,
                avatar: null,
                color: this.getColorForName(name),
                initial: name.charAt(0).toUpperCase()
            };
        }

        return speakerInfo;
    }

    /**
     * 格式化对话内容
     */
    formatConversationContent(conversation) {
        let content = conversation.message || conversation.content || conversation.text || '';

        // HTML转义
        content = this.escapeHtml(content);

        // 处理换行
        content = content.replace(/\n/g, '<br>');

        // 处理简单的markdown（可选）
        if (this.shouldParseMarkdown(content)) {
            content = this.parseSimpleMarkdown(content);
        }

        return content;
    }

    /**
     * 渲染对话元数据
     */
    renderConversationMetadata(conversation) {
        const metadata = [];

        // 情绪信息
        if (conversation.emotion) {
            metadata.push(`<span class="badge bg-secondary me-1" title="情绪">${conversation.emotion}</span>`);
        }

        // 消息类型
        if (conversation.message_type && conversation.message_type !== 'chat') {
            metadata.push(`<span class="badge bg-info me-1" title="类型">${this.formatMessageType(conversation.message_type)}</span>`);
        }

        // 角色回应时间（如果有）
        if (conversation.response_time) {
            metadata.push(`<span class="badge bg-success me-1" title="响应时间">${conversation.response_time}ms</span>`);
        }

        // 使用的token数（如果有）
        if (conversation.tokens_used) {
            metadata.push(`<span class="badge bg-warning me-1" title="使用的token">${conversation.tokens_used}</span>`);
        }

        if (metadata.length > 0) {
            return `<div class="conversation-metadata mt-2">${metadata.join('')}</div>`;
        }

        return '';
    }

    /**
     * 绑定对话元素事件
     */
    bindConversationElementEvents(element, conversation) {
        // 复制按钮事件
        const copyBtn = element.querySelector('.copy-conversation-btn');
        if (copyBtn) {
            copyBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                this.copyConversationToClipboard(conversation);
            });
        }

        // 点击对话元素事件（可选功能）
        element.addEventListener('click', () => {
            this.handleConversationClick(conversation, element);
        });

        // 长按事件（移动端）
        let longPressTimer;
        element.addEventListener('touchstart', () => {
            longPressTimer = setTimeout(() => {
                this.showConversationContextMenu(conversation, element);
            }, 800);
        });

        element.addEventListener('touchend', () => {
            if (longPressTimer) {
                clearTimeout(longPressTimer);
            }
        });
    }

    /**
     * 复制对话到剪贴板
     */
    async copyConversationToClipboard(conversation) {
        const speakerInfo = this.getSpeakerInfo(conversation);
        const content = conversation.message || conversation.content || '';
        const timestamp = this.formatConversationTime(conversation.timestamp);

        const textToCopy = `${speakerInfo.name} (${timestamp}):\n${content}`;

        try {
            if (typeof Utils !== 'undefined' && Utils.copyToClipboard) {
                await Utils.copyToClipboard(textToCopy);
            } else {
                // 降级处理
                await navigator.clipboard.writeText(textToCopy);
                this.showNotification('对话已复制到剪贴板', 'success');
            }
        } catch (error) {
            console.error('复制失败:', error);
            this.showNotification('复制失败', 'error');
        }
    }

    /**
     * 处理对话点击事件
     */
    handleConversationClick(conversation, element) {
        // 高亮选中的对话
        document.querySelectorAll('.conversation-item').forEach(item => {
            item.classList.remove('selected');
        });
        element.classList.add('selected');

        // 显示对话详情（可选）
        this.showConversationDetails(conversation);
    }

    /**
     * 显示对话详情 - 完整实现
     */
    showConversationDetails(conversation) {
        if (!conversation) {
            console.warn('无效的对话数据');
            return;
        }

        try {
            console.log('显示对话详情:', conversation);

            // 创建或获取模态框
            const modal = this.createConversationDetailsModal();

            // 填充对话详情数据
            this.populateConversationDetails(modal, conversation);

            // 显示模态框
            this.showModal(modal);

            // 绑定模态框事件
            this.bindConversationDetailsEvents(modal, conversation);

        } catch (error) {
            console.error('显示对话详情失败:', error);
            this.showNotification('显示对话详情失败', 'error');
        }
    }

    /**
     * 创建对话详情模态框
     */
    createConversationDetailsModal() {
        // 检查是否已存在模态框
        let modal = document.getElementById('conversation-details-modal');

        if (!modal) {
            modal = document.createElement('div');
            modal.id = 'conversation-details-modal';
            modal.className = 'modal fade';
            modal.setAttribute('tabindex', '-1');
            modal.setAttribute('aria-labelledby', 'conversationDetailsModalLabel');
            modal.setAttribute('aria-hidden', 'true');

            modal.innerHTML = `
            <div class="modal-dialog modal-lg">
                <div class="modal-content">
                    <div class="modal-header">
                        <h5 class="modal-title" id="conversationDetailsModalLabel">
                            <i class="bi bi-chat-dots me-2"></i>
                            对话详情
                        </h5>
                        <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="关闭"></button>
                    </div>
                    <div class="modal-body">
                        <!-- 对话基本信息 -->
                        <div class="conversation-basic-info mb-4">
                            <div class="row">
                                <div class="col-md-6">
                                    <div class="info-card">
                                        <div class="info-header">
                                            <i class="bi bi-person-circle text-primary"></i>
                                            <span class="info-title">说话者信息</span>
                                        </div>
                                        <div class="info-content">
                                            <div class="speaker-info-display">
                                                <div class="speaker-avatar-large"></div>
                                                <div class="speaker-details">
                                                    <h6 class="speaker-name-display mb-1"></h6>
                                                    <small class="speaker-id-display text-muted"></small>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                                <div class="col-md-6">
                                    <div class="info-card">
                                        <div class="info-header">
                                            <i class="bi bi-clock text-success"></i>
                                            <span class="info-title">时间信息</span>
                                        </div>
                                        <div class="info-content">
                                            <div class="time-display"></div>
                                            <div class="relative-time-display text-muted"></div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- 对话内容 -->
                        <div class="conversation-content-section mb-4">
                            <div class="info-card">
                                <div class="info-header">
                                    <i class="bi bi-chat-text text-info"></i>
                                    <span class="info-title">对话内容</span>
                                    <div class="header-actions">
                                        <button class="btn btn-sm btn-outline-secondary copy-content-btn" title="复制内容">
                                            <i class="bi bi-clipboard"></i>
                                        </button>
                                        <button class="btn btn-sm btn-outline-secondary expand-content-btn" title="全屏查看">
                                            <i class="bi bi-arrows-fullscreen"></i>
                                        </button>
                                    </div>
                                </div>
                                <div class="info-content">
                                    <div class="conversation-content-display"></div>
                                    <div class="content-stats">
                                        <small class="text-muted">
                                            字符数: <span class="char-count">0</span> | 
                                            词数: <span class="word-count">0</span>
                                        </small>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- 元数据信息 -->
                        <div class="conversation-metadata-section mb-4">
                            <div class="info-card">
                                <div class="info-header">
                                    <i class="bi bi-info-circle text-warning"></i>
                                    <span class="info-title">元数据</span>
                                </div>
                                <div class="info-content">
                                    <div class="metadata-display"></div>
                                </div>
                            </div>
                        </div>

                        <!-- 技术信息 -->
                        <div class="conversation-technical-section mb-4">
                            <div class="info-card">
                                <div class="info-header">
                                    <i class="bi bi-gear text-secondary"></i>
                                    <span class="info-title">技术信息</span>
                                </div>
                                <div class="info-content">
                                    <div class="technical-info-display"></div>
                                </div>
                            </div>
                        </div>

                        <!-- 相关操作 -->
                        <div class="conversation-actions-section">
                            <div class="info-card">
                                <div class="info-header">
                                    <i class="bi bi-tools text-dark"></i>
                                    <span class="info-title">相关操作</span>
                                </div>
                                <div class="info-content">
                                    <div class="action-buttons">
                                        <button class="btn btn-outline-primary btn-sm reply-to-conversation-btn">
                                            <i class="bi bi-reply me-1"></i>回复此对话
                                        </button>
                                        <button class="btn btn-outline-success btn-sm quote-conversation-btn">
                                            <i class="bi bi-quote me-1"></i>引用内容
                                        </button>
                                        <button class="btn btn-outline-info btn-sm view-context-btn">
                                            <i class="bi bi-eye me-1"></i>查看上下文
                                        </button>
                                        <button class="btn btn-outline-secondary btn-sm export-conversation-btn">
                                            <i class="bi bi-download me-1"></i>导出对话
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- 调试信息（开发模式） -->
                        <div class="conversation-debug-section" style="display: none;">
                            <div class="info-card">
                                <div class="info-header">
                                    <i class="bi bi-bug text-danger"></i>
                                    <span class="info-title">调试信息</span>
                                </div>
                                <div class="info-content">
                                    <pre class="debug-info-display"></pre>
                                </div>
                            </div>
                        </div>
                    </div>
                    <div class="modal-footer">
                        <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
                        <button type="button" class="btn btn-primary copy-all-btn">
                            <i class="bi bi-clipboard me-1"></i>复制全部信息
                        </button>
                    </div>
                </div>
            </div>
        `;

            // 添加到页面
            document.body.appendChild(modal);
        }

        return modal;
    }

    /**
     * 填充对话详情数据
     */
    populateConversationDetails(modal, conversation) {
        try {
            // 获取说话者信息
            const speakerInfo = this.getSpeakerInfo(conversation);

            // 填充说话者信息
            this.populateSpeakerInfo(modal, speakerInfo);

            // 填充时间信息
            this.populateTimeInfo(modal, conversation);

            // 填充对话内容
            this.populateConversationContent(modal, conversation);

            // 填充元数据
            this.populateMetadata(modal, conversation);

            // 填充技术信息
            this.populateTechnicalInfo(modal, conversation);

            // 显示调试信息（如果是开发模式）
            this.populateDebugInfo(modal, conversation);

        } catch (error) {
            console.error('填充对话详情数据失败:', error);
        }
    }

    /**
     * 填充说话者信息
     */
    populateSpeakerInfo(modal, speakerInfo) {
        const avatarElement = modal.querySelector('.speaker-avatar-large');
        const nameElement = modal.querySelector('.speaker-name-display');
        const idElement = modal.querySelector('.speaker-id-display');

        // 设置头像
        if (speakerInfo.avatar) {
            avatarElement.innerHTML = `
            <img src="${speakerInfo.avatar}" alt="${speakerInfo.name}" 
                 class="rounded-circle" width="48" height="48">
        `;
        } else {
            avatarElement.innerHTML = `
            <div class="avatar-placeholder-large rounded-circle d-flex align-items-center justify-content-center" 
                 style="width: 48px; height: 48px; background: ${speakerInfo.color}; color: white; font-size: 18px; font-weight: 600;">
                ${speakerInfo.initial}
            </div>
        `;
        }

        // 设置名称和ID
        nameElement.textContent = speakerInfo.name;
        idElement.textContent = `ID: ${speakerInfo.id}`;
    }

    /**
     * 填充时间信息
     */
    populateTimeInfo(modal, conversation) {
        const timeDisplay = modal.querySelector('.time-display');
        const relativeTimeDisplay = modal.querySelector('.relative-time-display');

        const timestamp = conversation.timestamp || Date.now();
        const date = new Date(timestamp);

        // 完整时间
        timeDisplay.innerHTML = `
        <div class="mb-1">
            <strong>发送时间:</strong> ${date.toLocaleString()}
        </div>
        <div>
            <strong>UTC时间:</strong> ${date.toISOString()}
        </div>
    `;

        // 相对时间
        relativeTimeDisplay.textContent = this.formatConversationTime(timestamp);
    }

    /**
     * 填充对话内容
     */
    populateConversationContent(modal, conversation) {
        const contentDisplay = modal.querySelector('.conversation-content-display');
        const charCountElement = modal.querySelector('.char-count');
        const wordCountElement = modal.querySelector('.word-count');

        const content = conversation.message || conversation.content || conversation.text || '';

        // 显示格式化的内容
        contentDisplay.innerHTML = `
        <div class="content-preview">
            ${this.formatConversationContent(conversation)}
        </div>
        <div class="raw-content mt-3" style="display: none;">
            <h6>原始内容:</h6>
            <pre class="text-muted small">${this.escapeHtml(content)}</pre>
        </div>
    `;

        // 计算统计信息
        charCountElement.textContent = content.length;
        wordCountElement.textContent = this.countWords(content);

        // 添加切换原始内容的功能
        const toggleBtn = document.createElement('button');
        toggleBtn.className = 'btn btn-sm btn-outline-info mt-2';
        toggleBtn.innerHTML = '<i class="bi bi-code me-1"></i>查看原始内容';
        toggleBtn.onclick = () => this.toggleRawContent(modal);
        contentDisplay.appendChild(toggleBtn);
    }

    /**
     * 填充元数据
     */
    populateMetadata(modal, conversation) {
        const metadataDisplay = modal.querySelector('.metadata-display');

        const metadata = [];

        // 消息类型
        if (conversation.message_type || conversation.type) {
            const type = conversation.message_type || conversation.type;
            metadata.push({
                label: '消息类型',
                value: this.formatMessageType(type),
                icon: 'bi-tag'
            });
        }

        // 情绪信息
        if (conversation.emotion) {
            metadata.push({
                label: '情绪',
                value: conversation.emotion,
                icon: 'bi-emoji-smile'
            });
        }

        // 响应时间
        if (conversation.response_time) {
            metadata.push({
                label: '响应时间',
                value: `${conversation.response_time}ms`,
                icon: 'bi-stopwatch'
            });
        }

        // Token使用量
        if (conversation.tokens_used) {
            metadata.push({
                label: '使用的Token',
                value: conversation.tokens_used,
                icon: 'bi-cpu'
            });
        }

        // 置信度
        if (conversation.confidence) {
            metadata.push({
                label: '置信度',
                value: `${Math.round(conversation.confidence * 100)}%`,
                icon: 'bi-graph-up'
            });
        }

        // 目标角色
        if (conversation.target_character_id) {
            metadata.push({
                label: '目标角色',
                value: this.getCharacterName(conversation.target_character_id),
                icon: 'bi-person-check'
            });
        }

        // 渲染元数据
        if (metadata.length > 0) {
            metadataDisplay.innerHTML = metadata.map(item => `
            <div class="metadata-item d-flex align-items-center mb-2">
                <i class="${item.icon} text-primary me-2"></i>
                <span class="metadata-label fw-bold me-2">${item.label}:</span>
                <span class="metadata-value">${item.value}</span>
            </div>
        `).join('');
        } else {
            metadataDisplay.innerHTML = '<p class="text-muted">暂无元数据信息</p>';
        }
    }

    /**
     * 填充技术信息
     */
    populateTechnicalInfo(modal, conversation) {
        const technicalDisplay = modal.querySelector('.technical-info-display');

        const technicalInfo = [];

        // 对话ID
        if (conversation.id) {
            technicalInfo.push({
                label: '对话ID',
                value: conversation.id,
                copyable: true
            });
        }

        // 场景ID
        if (conversation.scene_id || conversation.sceneId) {
            const sceneId = conversation.scene_id || conversation.sceneId;
            technicalInfo.push({
                label: '场景ID',
                value: sceneId,
                copyable: true
            });
        }

        // 说话者ID
        if (conversation.speaker_id || conversation.speakerId) {
            const speakerId = conversation.speaker_id || conversation.speakerId;
            technicalInfo.push({
                label: '说话者ID',
                value: speakerId,
                copyable: true
            });
        }

        // 交互ID
        if (conversation.interaction_id) {
            technicalInfo.push({
                label: '交互ID',
                value: conversation.interaction_id,
                copyable: true
            });
        }

        // 模拟ID
        if (conversation.simulation_id) {
            technicalInfo.push({
                label: '模拟ID',
                value: conversation.simulation_id,
                copyable: true
            });
        }

        // 会话ID
        if (conversation.session_id) {
            technicalInfo.push({
                label: '会话ID',
                value: conversation.session_id,
                copyable: true
            });
        }

        // 渲染技术信息
        if (technicalInfo.length > 0) {
            technicalDisplay.innerHTML = technicalInfo.map(item => `
            <div class="technical-item d-flex align-items-center justify-content-between mb-2">
                <div>
                    <span class="technical-label fw-bold me-2">${item.label}:</span>
                    <span class="technical-value font-monospace">${item.value}</span>
                </div>
                ${item.copyable ? `
                    <button class="btn btn-sm btn-outline-secondary copy-tech-btn" 
                            data-value="${item.value}" title="复制">
                        <i class="bi bi-clipboard"></i>
                    </button>
                ` : ''}
            </div>
        `).join('');
        } else {
            technicalDisplay.innerHTML = '<p class="text-muted">暂无技术信息</p>';
        }
    }

    /**
     * 填充调试信息
     */
    populateDebugInfo(modal, conversation) {
        const debugSection = modal.querySelector('.conversation-debug-section');
        const debugDisplay = modal.querySelector('.debug-info-display');

        // 只在开发模式下显示
        if (window.location.hostname === 'localhost' || window.location.search.includes('debug=1')) {
            debugSection.style.display = 'block';
            debugDisplay.textContent = JSON.stringify(conversation, null, 2);
        }
    }

    /**
     * 绑定对话详情模态框事件
     */
    bindConversationDetailsEvents(modal, conversation) {
        // 复制内容按钮
        const copyContentBtn = modal.querySelector('.copy-content-btn');
        if (copyContentBtn) {
            copyContentBtn.onclick = () => this.copyConversationContent(conversation);
        }

        // 全屏查看按钮
        const expandBtn = modal.querySelector('.expand-content-btn');
        if (expandBtn) {
            expandBtn.onclick = () => this.expandConversationContent(conversation);
        }

        // 回复对话按钮
        const replyBtn = modal.querySelector('.reply-to-conversation-btn');
        if (replyBtn) {
            replyBtn.onclick = () => {
                this.replyToConversation(conversation);
                this.closeModal(modal);
            };
        }

        // 引用内容按钮
        const quoteBtn = modal.querySelector('.quote-conversation-btn');
        if (quoteBtn) {
            quoteBtn.onclick = () => this.quoteConversationContent(conversation);
        }

        // 查看上下文按钮
        const contextBtn = modal.querySelector('.view-context-btn');
        if (contextBtn) {
            contextBtn.onclick = () => this.viewConversationContext(conversation);
        }

        // 导出对话按钮
        const exportBtn = modal.querySelector('.export-conversation-btn');
        if (exportBtn) {
            exportBtn.onclick = () => this.exportSingleConversation(conversation);
        }

        // 复制全部信息按钮
        const copyAllBtn = modal.querySelector('.copy-all-btn');
        if (copyAllBtn) {
            copyAllBtn.onclick = () => this.copyAllConversationInfo(conversation);
        }

        // 技术信息复制按钮
        const techCopyBtns = modal.querySelectorAll('.copy-tech-btn');
        techCopyBtns.forEach(btn => {
            btn.onclick = (e) => {
                e.stopPropagation();
                const value = btn.dataset.value;
                this.copyToClipboard(value);
            };
        });
    }

    /**
     * 显示模态框
     */
    showModal(modal) {
        // 如果使用Bootstrap 5
        if (typeof bootstrap !== 'undefined' && bootstrap.Modal) {
            const modalInstance = new bootstrap.Modal(modal);
            modalInstance.show();
        } else {
            // 降级处理：简单显示
            modal.style.display = 'block';
            modal.classList.add('show');
            document.body.classList.add('modal-open');

            // 添加背景遮罩
            const backdrop = document.createElement('div');
            backdrop.className = 'modal-backdrop fade show';
            backdrop.id = 'conversation-modal-backdrop';
            document.body.appendChild(backdrop);
        }
    }

    /**
     * 关闭模态框
     */
    closeModal(modal) {
        if (typeof bootstrap !== 'undefined' && bootstrap.Modal) {
            const modalInstance = bootstrap.Modal.getInstance(modal);
            if (modalInstance) {
                modalInstance.hide();
            }
        } else {
            // 降级处理
            modal.style.display = 'none';
            modal.classList.remove('show');
            document.body.classList.remove('modal-open');

            // 移除背景遮罩
            const backdrop = document.getElementById('conversation-modal-backdrop');
            if (backdrop) {
                backdrop.remove();
            }
        }
    }

    /**
     * 复制对话内容
     */
    copyConversationContent(conversation) {
        const content = conversation.message || conversation.content || conversation.text || '';
        this.copyToClipboard(content);
        this.showNotification('对话内容已复制', 'success');
    }

    /**
     * 全屏查看对话内容
     */
    expandConversationContent(conversation) {
        const content = conversation.message || conversation.content || conversation.text || '';
        const speakerInfo = this.getSpeakerInfo(conversation);

        // 创建全屏显示
        const fullscreenDiv = document.createElement('div');
        fullscreenDiv.className = 'conversation-fullscreen';
        fullscreenDiv.style.cssText = `
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background: rgba(0,0,0,0.9);
        z-index: 9999;
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        color: white;
        padding: 2rem;
    `;

        fullscreenDiv.innerHTML = `
        <div class="fullscreen-header mb-4 text-center">
            <h3>${speakerInfo.name}</h3>
            <p class="text-muted">${this.formatConversationTime(conversation.timestamp)}</p>
        </div>
        <div class="fullscreen-content" style="
            max-width: 80%;
            max-height: 70%;
            overflow-y: auto;
            font-size: 1.2rem;
            line-height: 1.6;
            text-align: center;
        ">
            ${this.formatConversationContent(conversation)}
        </div>
        <div class="fullscreen-footer mt-4">
            <button class="btn btn-secondary" onclick="this.parentNode.parentNode.remove()">
                <i class="bi bi-x-lg me-1"></i>关闭
            </button>
        </div>
    `;

        document.body.appendChild(fullscreenDiv);

        // ESC键关闭
        const closeOnEsc = (e) => {
            if (e.key === 'Escape') {
                fullscreenDiv.remove();
                document.removeEventListener('keydown', closeOnEsc);
            }
        };
        document.addEventListener('keydown', closeOnEsc);
    }

    /**
     * 引用对话内容
     */
    quoteConversationContent(conversation) {
        const messageInput = document.getElementById('message-input');
        if (!messageInput) {
            this.showNotification('未找到消息输入框', 'warning');
            return;
        }

        const content = conversation.message || conversation.content || conversation.text || '';
        const speakerInfo = this.getSpeakerInfo(conversation);
        const quoteText = `> ${speakerInfo.name}: ${content}\n\n`;

        // 在当前光标位置插入引用
        this.insertTextAtCursor(messageInput, quoteText);
        messageInput.focus();

        this.showNotification('内容已引用到输入框', 'success');
    }

    /**
     * 在光标位置插入文本
     */
    insertTextAtCursor(inputElement, text) {
        if (!inputElement || !text) {
            console.warn('insertTextAtCursor: 无效的输入元素或文本');
            return false;
        }

        try {
            const elementType = this.detectInputElementType(inputElement);

            switch (elementType) {
                case 'input-text':
                case 'textarea':
                    return this.insertTextIntoInput(inputElement, text);

                case 'contenteditable':
                    return this.insertTextIntoContentEditable(inputElement, text);

                default:
                    // 降级：追加到末尾
                    if (inputElement.value !== undefined) {
                        inputElement.value += text;
                        this.moveCursorToEnd(inputElement);
                        return true;
                    }
                    return false;
            }

        } catch (error) {
            console.error('插入文本失败:', error);
            return false;
        }
    }

    /**
     * 检测输入元素类型
     */
    detectInputElementType(element) {
        if (!element) return 'unknown';

        const tagName = element.tagName.toLowerCase();

        if (tagName === 'input') {
            const type = element.type.toLowerCase();
            if (['text', 'search', 'url', 'tel', 'password'].includes(type)) {
                return 'input-text';
            }
            return 'input-other';
        } else if (tagName === 'textarea') {
            return 'textarea';
        } else if (element.contentEditable === 'true') {
            return 'contenteditable';
        }

        return 'unknown';
    }

    /**
     * 在文本输入框中插入文本
     */
    insertTextIntoInput(inputElement, text) {
        try {
            const cursorPos = this.getCurrentCursorPosition(inputElement);

            if (cursorPos >= 0) {
                const value = inputElement.value;
                const newValue = value.substring(0, cursorPos) + text + value.substring(cursorPos);

                inputElement.value = newValue;
                this.setCursorPosition(inputElement, cursorPos + text.length);

                // 触发输入事件
                this.triggerInputEvents(inputElement);
                return true;
            }

            // 降级：追加到末尾
            inputElement.value += text;
            this.moveCursorToEnd(inputElement);
            this.triggerInputEvents(inputElement);
            return true;

        } catch (error) {
            console.error('在输入框中插入文本失败:', error);
            return false;
        }
    }

    /**
     * 在contenteditable元素中插入文本
     */
    insertTextIntoContentEditable(element, text) {
        try {
            if (window.getSelection && document.createRange) {
                const selection = window.getSelection();

                if (selection.rangeCount > 0) {
                    const range = selection.getRangeAt(0);
                    range.deleteContents();
                    range.insertNode(document.createTextNode(text));
                    range.collapse(false);

                    selection.removeAllRanges();
                    selection.addRange(range);

                    // 触发输入事件
                    this.triggerInputEvents(element);
                    return true;
                }
            }

            // 降级：追加到末尾
            element.textContent += text;
            this.moveCursorToEnd(element);
            this.triggerInputEvents(element);
            return true;

        } catch (error) {
            console.error('在contenteditable中插入文本失败:', error);
            return false;
        }
    }

    /**
     * 获取当前光标位置
     */
    getCurrentCursorPosition(inputElement) {
        if (!inputElement) return -1;

        try {
            const elementType = this.detectInputElementType(inputElement);

            switch (elementType) {
                case 'input-text':
                case 'textarea':
                    if ('selectionStart' in inputElement) {
                        return inputElement.selectionStart;
                    }
                    break;

                case 'contenteditable':
                    if (window.getSelection) {
                        const selection = window.getSelection();
                        if (selection.rangeCount > 0) {
                            const range = selection.getRangeAt(0);
                            return this.getTextPositionFromRange(inputElement, range);
                        }
                    }
                    break;
            }

            return -1;

        } catch (error) {
            console.error('获取光标位置失败:', error);
            return -1;
        }
    }

    /**
     * 从Range对象获取文本位置
     */
    getTextPositionFromRange(element, range) {
        let position = 0;

        function traverse(node) {
            if (node === range.startContainer) {
                return position + range.startOffset;
            }

            if (node.nodeType === Node.TEXT_NODE) {
                position += node.textContent.length;
            } else {
                for (let child of node.childNodes) {
                    const result = traverse(child);
                    if (result !== undefined) return result;
                }
            }

            return undefined;
        }

        const result = traverse(element);
        return result !== undefined ? result : position;
    }

    /**
     * 设置光标位置
     */
    setCursorPosition(inputElement, position) {
        if (!inputElement) {
            console.warn('输入元素无效');
            return false;
        }

        try {
            // 方法1: 使用 setSelectionRange (现代浏览器)
            if (typeof inputElement.setSelectionRange === 'function') {
                inputElement.setSelectionRange(position, position);
                return true;
            }

            // 方法2: 使用 createTextRange (IE兼容)
            if (inputElement.createTextRange) {
                const range = inputElement.createTextRange();
                range.collapse(true);
                range.moveEnd('character', position);
                range.moveStart('character', position);
                range.select();
                return true;
            }

            // 方法3: 使用 Selection API (现代浏览器替代方案)
            if (window.getSelection && document.createRange) {
                const selection = window.getSelection();
                const range = document.createRange();

                // 对于input/textarea，需要特殊处理
                if (inputElement.tagName === 'INPUT' || inputElement.tagName === 'TEXTAREA') {
                    // 尝试通过selectionStart/selectionEnd设置
                    if ('selectionStart' in inputElement) {
                        inputElement.selectionStart = position;
                        inputElement.selectionEnd = position;
                        return true;
                    }
                } else {
                    // 对于contenteditable元素
                    if (inputElement.firstChild) {
                        range.setStart(inputElement.firstChild, Math.min(position, inputElement.textContent.length));
                        range.setEnd(inputElement.firstChild, Math.min(position, inputElement.textContent.length));
                        selection.removeAllRanges();
                        selection.addRange(range);
                        return true;
                    }
                }
            }

            // 方法4: 最后的降级方案 - 移动到末尾
            if ('selectionStart' in inputElement) {
                inputElement.selectionStart = inputElement.value.length;
                inputElement.selectionEnd = inputElement.value.length;
                return true;
            }

            console.warn('无法设置光标位置，使用默认位置');
            return false;

        } catch (error) {
            console.error('设置光标位置失败:', error);
            return false;
        }
    }

    /**
     * 移动光标到文本末尾
     */
    moveCursorToEnd(inputElement) {
        if (!inputElement) return false;

        try {
            const textLength = inputElement.value ? inputElement.value.length :
                inputElement.textContent ? inputElement.textContent.length : 0;

            return this.setCursorPosition(inputElement, textLength);

        } catch (error) {
            console.error('移动光标到末尾失败:', error);
            return false;
        }
    }

    /**
     * 移动光标到文本开头
     */
    moveCursorToStart(inputElement) {
        if (!inputElement) return false;

        try {
            return this.setCursorPosition(inputElement, 0);

        } catch (error) {
            console.error('移动光标到开头失败:', error);
            return false;
        }
    }

    /**
     * 选择指定范围的文本
     */
    selectTextRange(inputElement, start, end) {
        if (!inputElement) return false;

        try {
            const elementType = this.detectInputElementType(inputElement);

            switch (elementType) {
                case 'input-text':
                case 'textarea':
                    return this.selectTextInputRange(inputElement, start, end);

                case 'contenteditable':
                    return this.selectContentEditableRange(inputElement, start, end);

                default:
                    console.warn('不支持的元素类型:', elementType);
                    return false;
            }

        } catch (error) {
            console.error('选择文本范围失败:', error);
            return false;
        }
    }

    /**
     * 选择文本输入框的指定范围
     */
    selectTextInputRange(inputElement, start, end) {
        try {
            const maxLength = inputElement.value.length;
            const safeStart = Math.max(0, Math.min(start, maxLength));
            const safeEnd = Math.max(safeStart, Math.min(end, maxLength));

            if (typeof inputElement.setSelectionRange === 'function') {
                inputElement.setSelectionRange(safeStart, safeEnd);
                return true;
            }

            if ('selectionStart' in inputElement && 'selectionEnd' in inputElement) {
                inputElement.selectionStart = safeStart;
                inputElement.selectionEnd = safeEnd;
                return true;
            }

            if (inputElement.createTextRange) {
                const range = inputElement.createTextRange();
                range.collapse(true);
                range.moveStart('character', safeStart);
                range.moveEnd('character', safeEnd - safeStart);
                range.select();
                return true;
            }

            return false;

        } catch (error) {
            console.error('选择文本输入范围失败:', error);
            return false;
        }
    }

    /**
     * 选择contenteditable元素的指定范围
     */
    selectContentEditableRange(element, start, end) {
        try {
            if (!window.getSelection || !document.createRange) {
                return false;
            }

            const selection = window.getSelection();
            const range = document.createRange();

            const startNode = this.findTextNodeAtPosition(element, start);
            const endNode = this.findTextNodeAtPosition(element, end);

            if (startNode.node && endNode.node) {
                range.setStart(startNode.node, startNode.offset);
                range.setEnd(endNode.node, endNode.offset);

                selection.removeAllRanges();
                selection.addRange(range);
                return true;
            }

            return false;

        } catch (error) {
            console.error('选择contenteditable范围失败:', error);
            return false;
        }
    }

    /**
     * 在指定位置查找文本节点
     */
    findTextNodeAtPosition(element, position) {
        let currentPosition = 0;

        function traverse(node) {
            if (node.nodeType === Node.TEXT_NODE) {
                const nodeLength = node.textContent.length;
                if (currentPosition + nodeLength >= position) {
                    return {
                        node: node,
                        offset: position - currentPosition
                    };
                }
                currentPosition += nodeLength;
            } else {
                for (let child of node.childNodes) {
                    const result = traverse(child);
                    if (result) return result;
                }
            }
            return null;
        }

        const result = traverse(element);
        return result || { node: element, offset: 0 };
    }

    /**
     * 智能文本插入（自动检测并处理不同情况）
     */
    smartInsertText(inputElement, text, options = {}) {
        const defaultOptions = {
            replaceSelection: true,      // 是否替换选中的文本
            moveCursorToEnd: true,       // 是否将光标移到插入文本的末尾
            triggerEvents: true,         // 是否触发input事件
            addSpace: false,             // 是否在文本后添加空格
            addNewline: false            // 是否在文本后添加换行
        };

        const config = { ...defaultOptions, ...options };

        try {
            // 处理文本
            let insertText = text;
            if (config.addSpace && !insertText.endsWith(' ')) {
                insertText += ' ';
            }
            if (config.addNewline && !insertText.endsWith('\n')) {
                insertText += '\n';
            }

            // 执行插入
            const success = this.insertTextAtCursor(inputElement, insertText);

            // 触发事件
            if (success && config.triggerEvents) {
                this.triggerInputEvents(inputElement);
            }

            return success;

        } catch (error) {
            console.error('智能文本插入失败:', error);
            return false;
        }
    }

    /**
     * 触发输入相关事件
     */
    triggerInputEvents(inputElement) {
        try {
            // 触发input事件
            const inputEvent = new Event('input', {
                bubbles: true,
                cancelable: true
            });
            inputElement.dispatchEvent(inputEvent);

            // 触发change事件
            const changeEvent = new Event('change', {
                bubbles: true,
                cancelable: true
            });
            inputElement.dispatchEvent(changeEvent);

            // 对于某些框架，可能需要触发自定义事件
            if (inputElement.oninput) {
                inputElement.oninput(inputEvent);
            }

        } catch (error) {
            console.warn('触发输入事件失败:', error);
        }
    }

    /**
     * 复制文本到剪贴板
     */
    async copyToClipboard(text) {
        try {
            if (navigator.clipboard && window.isSecureContext) {
                await navigator.clipboard.writeText(text);
            } else {
                // 降级处理
                const textArea = document.createElement('textarea');
                textArea.value = text;
                textArea.style.position = 'fixed';
                textArea.style.left = '-999999px';
                textArea.style.top = '-999999px';
                document.body.appendChild(textArea);
                textArea.focus();
                textArea.select();
                document.execCommand('copy');
                textArea.remove();
            }
            return true;
        } catch (error) {
            console.error('复制失败:', error);
            return false;
        }
    }

    /**
     * 从剪贴板粘贴文本
     */
    async pasteFromClipboard(inputElement) {
        try {
            let text = '';

            if (navigator.clipboard && window.isSecureContext) {
                text = await navigator.clipboard.readText();
            } else {
                // 降级处理：模拟Ctrl+V
                console.warn('无法直接从剪贴板读取，请使用Ctrl+V手动粘贴');
                return false;
            }

            if (text) {
                return this.insertTextAtCursor(inputElement, text);
            }

            return false;

        } catch (error) {
            console.error('粘贴失败:', error);
            return false;
        }
    }

    /**
     * 清空输入框内容
     */
    clearInput(inputElement) {
        if (!inputElement) return false;

        try {
            if (inputElement.value !== undefined) {
                inputElement.value = '';
            } else if (inputElement.textContent !== undefined) {
                inputElement.textContent = '';
            } else if (inputElement.innerHTML !== undefined) {
                inputElement.innerHTML = '';
            }

            // 触发事件
            this.triggerInputEvents(inputElement);

            return true;

        } catch (error) {
            console.error('清空输入框失败:', error);
            return false;
        }
    }

    /**
     * 在当前位置插入HTML内容（仅用于contenteditable）
     */
    insertHTMLAtCursor(element, html) {
        if (!element || element.contentEditable !== 'true') {
            console.warn('insertHTMLAtCursor只能用于contenteditable元素');
            return false;
        }

        try {
            if (window.getSelection && document.createRange) {
                const selection = window.getSelection();

                if (selection.rangeCount > 0) {
                    const range = selection.getRangeAt(0);
                    range.deleteContents();

                    const fragment = document.createDocumentFragment();
                    const div = document.createElement('div');
                    div.innerHTML = html;

                    while (div.firstChild) {
                        fragment.appendChild(div.firstChild);
                    }

                    range.insertNode(fragment);
                    range.collapse(false);

                    selection.removeAllRanges();
                    selection.addRange(range);

                    this.triggerInputEvents(element);
                    return true;
                }
            }

            // 降级：追加到末尾
            element.innerHTML += html;
            this.moveCursorToEnd(element);
            this.triggerInputEvents(element);
            return true;

        } catch (error) {
            console.error('插入HTML失败:', error);
            return false;
        }
    }

    /**
     * 获取选中的文本
     */
    getSelectedText(inputElement) {
        try {
            const elementType = this.detectInputElementType(inputElement);

            switch (elementType) {
                case 'input-text':
                case 'textarea':
                    if ('selectionStart' in inputElement && 'selectionEnd' in inputElement) {
                        const start = inputElement.selectionStart;
                        const end = inputElement.selectionEnd;
                        return inputElement.value.substring(start, end);
                    }
                    break;

                case 'contenteditable':
                    if (window.getSelection) {
                        const selection = window.getSelection();
                        return selection.toString();
                    }
                    break;
            }

            return '';

        } catch (error) {
            console.error('获取选中文本失败:', error);
            return '';
        }
    }

    /**
     * 替换选中的文本
     */
    replaceSelectedText(inputElement, newText) {
        try {
            const elementType = this.detectInputElementType(inputElement);

            switch (elementType) {
                case 'input-text':
                case 'textarea':
                    if ('selectionStart' in inputElement && 'selectionEnd' in inputElement) {
                        const start = inputElement.selectionStart;
                        const end = inputElement.selectionEnd;
                        const value = inputElement.value;

                        inputElement.value = value.substring(0, start) + newText + value.substring(end);
                        inputElement.selectionStart = start;
                        inputElement.selectionEnd = start + newText.length;

                        this.triggerInputEvents(inputElement);
                        return true;
                    }
                    break;

                case 'contenteditable':
                    if (window.getSelection) {
                        const selection = window.getSelection();
                        if (selection.rangeCount > 0) {
                            const range = selection.getRangeAt(0);
                            range.deleteContents();
                            range.insertNode(document.createTextNode(newText));
                            range.collapse(false);

                            this.triggerInputEvents(inputElement);
                            return true;
                        }
                    }
                    break;
            }

            return false;

        } catch (error) {
            console.error('替换选中文本失败:', error);
            return false;
        }
    }

    /**
     * 测试文本插入功能
     */
    testTextInsertion() {
        console.log('🔧 测试文本插入功能...');

        const messageInput = document.getElementById('message-input');
        if (!messageInput) {
            console.warn('未找到消息输入框，无法测试');
            return;
        }

        try {
            // 测试基本插入
            this.clearInput(messageInput);
            const success1 = this.insertTextAtCursor(messageInput, 'Hello ');
            console.log('基本文本插入:', success1 ? '✅' : '❌');

            // 测试智能插入
            const success2 = this.smartInsertText(messageInput, 'World', {
                addSpace: true,
                triggerEvents: true
            });
            console.log('智能文本插入:', success2 ? '✅' : '❌');

            // 测试光标位置
            this.setCursorPosition(messageInput, 6);
            const success3 = this.insertTextAtCursor(messageInput, 'Beautiful ');
            console.log('光标位置插入:', success3 ? '✅' : '❌');

            // 清理
            setTimeout(() => {
                this.clearInput(messageInput);
            }, 3000);

            console.log('✅ 文本插入功能测试完成');

        } catch (error) {
            console.error('❌ 文本插入测试失败:', error);
        }
    }

    /**
     * 初始化键盘快捷键
     */
    initKeyboardShortcuts() {
        console.log('⌨️ 初始化键盘快捷键...');

        document.addEventListener('keydown', (e) => {
            // Ctrl/Cmd + Enter: 发送消息
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
                e.preventDefault();
                this.sendRealtimeMessage();
                return;
            }

            // Ctrl/Cmd + K: 清空输入框
            if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
                e.preventDefault();
                const messageInput = document.getElementById('message-input');
                if (messageInput) {
                    this.clearInput(messageInput);
                    messageInput.focus();
                }
                return;
            }

            // Ctrl/Cmd + V: 增强粘贴
            if ((e.ctrlKey || e.metaKey) && e.key === 'v') {
                const activeElement = document.activeElement;
                if (activeElement && activeElement.id === 'message-input') {
                    // 让默认粘贴行为执行，然后处理
                    setTimeout(() => {
                        this.triggerInputEvents(activeElement);
                    }, 10);
                }
                return;
            }

            // Esc: 取消当前操作
            if (e.key === 'Escape') {
                e.preventDefault();
                // 关闭模态框
                const modals = document.querySelectorAll('.modal.show');
                modals.forEach(modal => {
                    this.closeModal(modal);
                });
                return;
            }

            // F1: 显示帮助
            if (e.key === 'F1') {
                e.preventDefault();
                this.showKeyboardShortcutsHelp();
                return;
            }
        });

        console.log('✅ 键盘快捷键已初始化');
    }

    /**
     * 显示键盘快捷键帮助
     */
    showKeyboardShortcutsHelp() {
        const shortcuts = [
            { key: 'Ctrl/Cmd + Enter', desc: '发送消息' },
            { key: 'Ctrl/Cmd + K', desc: '清空输入框' },
            { key: 'Ctrl/Cmd + V', desc: '粘贴文本' },
            { key: 'Esc', desc: '取消当前操作/关闭模态框' },
            { key: 'F1', desc: '显示快捷键帮助' }
        ];

        const helpContent = shortcuts.map(s =>
            `<div class="shortcut-item"><kbd>${s.key}</kbd> - ${s.desc}</div>`
        ).join('');

        // 创建帮助模态框
        const helpModal = document.createElement('div');
        helpModal.className = 'modal fade';
        helpModal.innerHTML = `
        <div class="modal-dialog">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-keyboard me-2"></i>键盘快捷键
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <div class="shortcuts-list">
                        ${helpContent}
                    </div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
                </div>
            </div>
        </div>
    `;

        // 添加样式
        const style = document.createElement('style');
        style.textContent = `
        .shortcut-item {
            display: flex;
            align-items: center;
            padding: 0.5rem 0;
            border-bottom: 1px solid #f0f0f0;
        }
        .shortcut-item:last-child {
            border-bottom: none;
        }
        .shortcut-item kbd {
            background: #f8f9fa;
            border: 1px solid #dee2e6;
            border-radius: 0.25rem;
            padding: 0.25rem 0.5rem;
            font-size: 0.875rem;
            margin-right: 1rem;
            min-width: 120px;
        }
    `;

        if (!document.getElementById('shortcuts-help-styles')) {
            style.id = 'shortcuts-help-styles';
            document.head.appendChild(style);
        }

        document.body.appendChild(helpModal);
        this.showModal(helpModal);

        // 自动移除模态框
        helpModal.addEventListener('hidden.bs.modal', () => {
            helpModal.remove();
        });
    }

    /**
     * 查看对话上下文
     */
    viewConversationContext(conversation) {
        // 获取对话的上下文（前后几条消息）
        const contextSize = 3; // 前后各3条消息
        const allConversations = this.conversations || [];

        // 找到当前对话的索引
        const currentIndex = allConversations.findIndex(conv =>
            conv.id === conversation.id ||
            (conv.timestamp === conversation.timestamp &&
                conv.speaker_id === conversation.speaker_id)
        );

        if (currentIndex === -1) {
            this.showNotification('无法找到对话上下文', 'warning');
            return;
        }

        // 获取上下文对话
        const startIndex = Math.max(0, currentIndex - contextSize);
        const endIndex = Math.min(allConversations.length, currentIndex + contextSize + 1);
        const contextConversations = allConversations.slice(startIndex, endIndex);

        // 显示上下文对话
        this.showConversationContext(contextConversations, currentIndex - startIndex);
    }

    /**
     * 导出单个对话
     */
    exportSingleConversation(conversation) {
        const speakerInfo = this.getSpeakerInfo(conversation);
        const content = conversation.message || conversation.content || conversation.text || '';
        const timestamp = new Date(conversation.timestamp || Date.now());

        const exportData = {
            conversation_id: conversation.id,
            speaker: {
                id: speakerInfo.id,
                name: speakerInfo.name
            },
            content: content,
            timestamp: timestamp.toISOString(),
            formatted_time: timestamp.toLocaleString(),
            metadata: {
                message_type: conversation.message_type || conversation.type,
                emotion: conversation.emotion,
                response_time: conversation.response_time,
                tokens_used: conversation.tokens_used
            }
        };

        // 下载为JSON文件
        const blob = new Blob([JSON.stringify(exportData, null, 2)], {
            type: 'application/json'
        });

        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `conversation_${conversation.id || Date.now()}.json`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);

        this.showNotification('对话已导出', 'success');
    }

    /**
     * 复制所有对话信息
     */
    copyAllConversationInfo(conversation) {
        const speakerInfo = this.getSpeakerInfo(conversation);
        const content = conversation.message || conversation.content || conversation.text || '';
        const timestamp = new Date(conversation.timestamp || Date.now());

        const allInfo = `
对话详情
========
说话者: ${speakerInfo.name} (${speakerInfo.id})
时间: ${timestamp.toLocaleString()}
内容: ${content}

元数据:
- 消息类型: ${conversation.message_type || conversation.type || '未知'}
- 情绪: ${conversation.emotion || '无'}
- 响应时间: ${conversation.response_time || '无'}ms
- 使用Token: ${conversation.tokens_used || '无'}

技术信息:
- 对话ID: ${conversation.id || '无'}
- 场景ID: ${conversation.scene_id || conversation.sceneId || '无'}
- 说话者ID: ${conversation.speaker_id || conversation.speakerId || '无'}

导出时间: ${new Date().toLocaleString()}
    `.trim();

        this.copyToClipboard(allInfo);
        this.showNotification('所有信息已复制到剪贴板', 'success');
    }

    /**
     * 切换原始内容显示
     */
    toggleRawContent(modal) {
        const rawContent = modal.querySelector('.raw-content');
        const toggleBtn = modal.querySelector('.btn-outline-info');

        if (rawContent.style.display === 'none') {
            rawContent.style.display = 'block';
            toggleBtn.innerHTML = '<i class="bi bi-eye-slash me-1"></i>隐藏原始内容';
        } else {
            rawContent.style.display = 'none';
            toggleBtn.innerHTML = '<i class="bi bi-code me-1"></i>查看原始内容';
        }
    }

    /**
     * 计算单词数量
     */
    countWords(text) {
        if (!text) return 0;

        // 简单的单词计数，适用于中英文混合
        const words = text.trim().split(/\s+/);
        const chineseChars = text.match(/[\u4e00-\u9fff]/g) || [];

        return words.length + chineseChars.length;
    }

    /**
     * 复制到剪贴板的通用方法
     */
    async copyToClipboard(text) {
        try {
            if (navigator.clipboard && window.isSecureContext) {
                await navigator.clipboard.writeText(text);
            } else {
                // 降级处理
                const textArea = document.createElement('textarea');
                textArea.value = text;
                textArea.style.position = 'fixed';
                textArea.style.left = '-999999px';
                textArea.style.top = '-999999px';
                document.body.appendChild(textArea);
                textArea.focus();
                textArea.select();
                document.execCommand('copy');
                textArea.remove();
            }
            return true;
        } catch (error) {
            console.error('复制失败:', error);
            return false;
        }
    }

    /**
     * 显示对话上下文
     */
    showConversationContext(contextConversations, currentIndex) {
        // 创建上下文模态框
        const contextModal = document.createElement('div');
        contextModal.className = 'modal fade';
        contextModal.id = 'conversation-context-modal';

        contextModal.innerHTML = `
        <div class="modal-dialog modal-xl">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-chat-left-text me-2"></i>
                        对话上下文
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <div class="context-conversations">
                        ${contextConversations.map((conv, index) => {
            const speakerInfo = this.getSpeakerInfo(conv);
            const isCurrentConversation = index === currentIndex;
            return `
                                <div class="context-conversation-item ${isCurrentConversation ? 'current-conversation' : ''} mb-3 p-3 border rounded">
                                    <div class="d-flex align-items-center mb-2">
                                        <div class="speaker-avatar me-2">
                                            ${speakerInfo.avatar ?
                    `<img src="${speakerInfo.avatar}" class="rounded-circle" width="24" height="24">` :
                    `<div class="avatar-placeholder rounded-circle" style="width: 24px; height: 24px; background: ${speakerInfo.color}; color: white; font-size: 10px; display: flex; align-items: center; justify-content: center;">${speakerInfo.initial}</div>`
                }
                                        </div>
                                        <strong class="speaker-name">${speakerInfo.name}</strong>
                                        <small class="text-muted ms-auto">${this.formatConversationTime(conv.timestamp)}</small>
                                        ${isCurrentConversation ? '<span class="badge bg-primary ms-2">当前对话</span>' : ''}
                                    </div>
                                    <div class="conversation-content">
                                        ${this.formatConversationContent(conv)}
                                    </div>
                                </div>
                            `;
        }).join('')}
                    </div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
                </div>
            </div>
        </div>
    `;

        document.body.appendChild(contextModal);

        // 显示模态框
        this.showModal(contextModal);

        // 自动移除模态框
        contextModal.addEventListener('hidden.bs.modal', () => {
            contextModal.remove();
        });
    }

    /**
     * 检查是否为重复消息
     */
    isDuplicateMessage(conversation, container) {
        const conversationId = conversation.id || this.generateConversationId(conversation);
        const existingElement = container.querySelector(`[data-conversation-id="${conversationId}"]`);
        return !!existingElement;
    }

    /**
     * 生成对话ID
     */
    generateConversationId(conversation = null) {
        if (conversation) {
            // 基于内容和时间戳生成ID
            const content = conversation.message || conversation.content || '';
            const timestamp = conversation.timestamp || Date.now();
            const speaker = conversation.speaker_id || conversation.speakerId || 'unknown';
            return `conv_${speaker}_${timestamp}_${content.length}`;
        }

        return `conv_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    }

    /**
     * 添加新对话动画
     */
    animateNewConversation(element) {
        // 添加进入动画
        element.style.opacity = '0';
        element.style.transform = 'translateY(20px)';
        element.style.transition = 'all 0.3s ease-out';

        // 使用requestAnimationFrame确保动画执行
        requestAnimationFrame(() => {
            element.style.opacity = '1';
            element.style.transform = 'translateY(0)';
        });

        // 动画完成后清理样式
        setTimeout(() => {
            element.style.transition = '';
        }, 300);
    }

    /**
     * 更新对话计数
     */
    updateConversationCount() {
        const count = this.conversations ? this.conversations.length :
            document.querySelectorAll('.conversation-item').length;

        // 更新计数显示
        const countElements = document.querySelectorAll('.conversation-count, #conversation-count');
        countElements.forEach(element => {
            element.textContent = count;
        });

        // 更新统计卡片
        const statsCards = document.querySelectorAll('.stat-card');
        statsCards.forEach(card => {
            const statLabel = card.querySelector('.stat-label');
            if (statLabel && statLabel.textContent.includes('对话')) {
                const statValue = card.querySelector('.stat-value');
                if (statValue) {
                    this.animateNumber(statValue, count);
                }
            }
        });
    }

    /**
     * 添加对话到本地数据
     */
    addConversationToLocal(conversation) {
        // 初始化对话数组
        if (!this.conversations) {
            this.conversations = [];
        }

        // 检查是否已存在
        const existingIndex = this.conversations.findIndex(c =>
            c.id === conversation.id ||
            (c.timestamp === conversation.timestamp &&
                c.speaker_id === conversation.speaker_id &&
                c.message === conversation.message)
        );

        if (existingIndex === -1) {
            // 添加新对话
            this.conversations.push(conversation);

            // 保持对话数量限制（可选）
            const maxConversations = 100;
            if (this.conversations.length > maxConversations) {
                this.conversations = this.conversations.slice(-maxConversations);
            }

            // 按时间戳排序
            this.conversations.sort((a, b) =>
                (a.timestamp || 0) - (b.timestamp || 0)
            );
        } else {
            // 更新现有对话
            this.conversations[existingIndex] = { ...this.conversations[existingIndex], ...conversation };
        }

        // 保存到本地存储（可选）
        this.saveConversationsToLocal();
    }

    /**
     * 保存对话到本地存储
     */
    saveConversationsToLocal() {
        try {
            const sceneId = this.getSceneIdFromPage();
            if (sceneId && this.conversations) {
                const key = `conversations_${sceneId}`;
                const data = {
                    conversations: this.conversations.slice(-50), // 只保存最近50条
                    timestamp: Date.now()
                };
                localStorage.setItem(key, JSON.stringify(data));
            }
        } catch (error) {
            console.warn('保存对话到本地存储失败:', error);
        }
    }

    /**
     * 从本地存储加载对话
     */
    loadConversationsFromLocal() {
        try {
            const sceneId = this.getSceneIdFromPage();
            if (sceneId) {
                const key = `conversations_${sceneId}`;
                const data = localStorage.getItem(key);
                if (data) {
                    const parsed = JSON.parse(data);
                    if (parsed.conversations && Array.isArray(parsed.conversations)) {
                        this.conversations = parsed.conversations;
                        console.log(`从本地存储加载了 ${this.conversations.length} 条对话`);
                    }
                }
            }
        } catch (error) {
            console.warn('从本地存储加载对话失败:', error);
        }
    }

    /**
     * 降级处理：添加消息到聊天区域
     */
    addMessageToChat(conversation) {
        console.log('🔄 使用降级方案添加消息到聊天区域');

        try {
            // 查找聊天消息容器
            const chatContainer = document.querySelector('#chat-messages, .chat-messages, .messages');
            if (!chatContainer) {
                console.warn('未找到聊天容器，创建简单的消息显示');
                this.createSimpleMessageDisplay(conversation);
                return;
            }

            // 创建简单的消息元素
            const messageElement = document.createElement('div');
            messageElement.className = 'chat-message mb-2 p-2 border rounded';

            const speakerInfo = this.getSpeakerInfo(conversation);
            const content = conversation.message || conversation.content || '';
            const timestamp = this.formatConversationTime(conversation.timestamp);

            messageElement.innerHTML = `
            <div class="message-header d-flex justify-content-between">
                <strong class="speaker-name">${speakerInfo.name}</strong>
                <small class="text-muted">${timestamp}</small>
            </div>
            <div class="message-content mt-1">${this.escapeHtml(content)}</div>
        `;

            chatContainer.appendChild(messageElement);

            // 滚动到底部
            chatContainer.scrollTop = chatContainer.scrollHeight;

        } catch (error) {
            console.error('降级方案也失败了:', error);
            // 最后的降级：控制台输出
            console.log('💬 新消息:', conversation);
        }
    }

    /**
     * 创建简单的消息显示
     */
    createSimpleMessageDisplay(conversation) {
        const container = document.createElement('div');
        container.className = 'simple-message-display';
        container.style.cssText = `
        position: fixed;
        bottom: 20px;
        right: 20px;
        background: white;
        border: 1px solid #ccc;
        border-radius: 8px;
        padding: 15px;
        max-width: 300px;
        box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        z-index: 1000;
        animation: slideInUp 0.3s ease;
    `;

        const speakerInfo = this.getSpeakerInfo(conversation);
        const content = conversation.message || conversation.content || '';

        container.innerHTML = `
        <div class="message-notification">
            <div class="d-flex align-items-center mb-2">
                <strong>${speakerInfo.name}</strong>
                <button class="btn btn-sm ms-auto" onclick="this.parentNode.parentNode.parentNode.remove()">×</button>
            </div>
            <div class="message-content">${this.escapeHtml(content)}</div>
        </div>
    `;

        document.body.appendChild(container);

        // 5秒后自动消失
        setTimeout(() => {
            if (container.parentNode) {
                container.remove();
            }
        }, 5000);
    }

    /**
     * 格式化对话时间
     */
    formatConversationTime(timestamp) {
        if (!timestamp) return '刚刚';

        const date = new Date(timestamp);
        const now = new Date();
        const diffMs = now - date;
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMs / 3600000);
        const diffDays = Math.floor(diffMs / 86400000);

        if (diffMins < 1) {
            return '刚刚';
        } else if (diffMins < 60) {
            return `${diffMins}分钟前`;
        } else if (diffHours < 24) {
            return `${diffHours}小时前`;
        } else if (diffDays < 7) {
            return `${diffDays}天前`;
        } else {
            return date.toLocaleDateString();
        }
    }

    /**
     * 格式化消息类型
     */
    formatMessageType(type) {
        const typeMap = {
            'user': '用户',
            'character': '角色',
            'system': '系统',
            'narrator': '旁白',
            'action': '动作',
            'thought': '思考'
        };
        return typeMap[type] || type;
    }

    /**
     * 获取角色颜色
     */
    getCharacterColor(characterId) {
        if (!characterId) return '#6c757d';

        // 为角色ID生成一致的颜色
        const colors = [
            '#007bff', '#28a745', '#dc3545', '#ffc107',
            '#17a2b8', '#6f42c1', '#e83e8c', '#fd7e14'
        ];

        let hash = 0;
        for (let i = 0; i < characterId.length; i++) {
            hash = characterId.charCodeAt(i) + ((hash << 5) - hash);
        }

        return colors[Math.abs(hash) % colors.length];
    }

    /**
     * 获取名称对应的颜色
     */
    getColorForName(name) {
        if (!name) return '#6c757d';

        const colors = [
            '#007bff', '#28a745', '#dc3545', '#ffc107',
            '#17a2b8', '#6f42c1', '#e83e8c', '#fd7e14'
        ];

        let hash = 0;
        for (let i = 0; i < name.length; i++) {
            hash = name.charCodeAt(i) + ((hash << 5) - hash);
        }

        return colors[Math.abs(hash) % colors.length];
    }

    /**
     * 判断是否应该解析Markdown
     */
    shouldParseMarkdown(content) {
        // 简单判断：如果包含markdown标记则解析
        return /[*_`]/.test(content);
    }

    /**
     * 解析简单的Markdown
     */
    parseSimpleMarkdown(content) {
        return content
            .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')  // 加粗
            .replace(/\*(.*?)\*/g, '<em>$1</em>')              // 斜体
            .replace(/`(.*?)`/g, '<code>$1</code>');           // 行内代码
    }

    /**
     * HTML转义
     */
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /**
     * 显示通知
     */
    showNotification(message, type = 'info') {
        if (typeof Utils !== 'undefined') {
            switch (type) {
                case 'success':
                    Utils.showSuccess(message);
                    break;
                case 'error':
                    Utils.showError(message);
                    break;
                case 'warning':
                    Utils.showWarning(message);
                    break;
                default:
                    Utils.showInfo(message);
            }
        } else {
            console.log(`[${type.toUpperCase()}] ${message}`);
        }
    }

    /**
     * 触发对话事件
     */
    triggerConversationEvent(eventType, eventData) {
        // 触发自定义事件
        const event = new CustomEvent(eventType, {
            detail: eventData
        });
        document.dispatchEvent(event);

        // 如果有实时管理器，也通过它触发事件
        if (this.realtimeManager && this.realtimeManager.emit) {
            this.realtimeManager.emit(eventType, eventData);
        }
    }

    /**
     * 显示对话上下文菜单
     */
    showConversationContextMenu(conversation, element) {
        // 创建上下文菜单
        const menu = document.createElement('div');
        menu.className = 'conversation-context-menu';
        menu.style.cssText = `
        position: fixed;
        background: white;
        border: 1px solid #ddd;
        border-radius: 4px;
        box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        z-index: 1000;
        min-width: 150px;
    `;

        menu.innerHTML = `
        <div class="menu-item p-2 hover:bg-gray-100 cursor-pointer" data-action="copy">
            <i class="bi bi-clipboard me-2"></i>复制
        </div>
        <div class="menu-item p-2 hover:bg-gray-100 cursor-pointer" data-action="reply">
            <i class="bi bi-reply me-2"></i>回复
        </div>
        <div class="menu-item p-2 hover:bg-gray-100 cursor-pointer" data-action="details">
            <i class="bi bi-info-circle me-2"></i>详情
        </div>
    `;

        // 绑定菜单事件
        menu.addEventListener('click', (e) => {
            const action = e.target.closest('.menu-item')?.dataset.action;
            if (action) {
                this.handleContextMenuAction(action, conversation, element);
                menu.remove();
            }
        });

        // 定位菜单
        const rect = element.getBoundingClientRect();
        menu.style.top = `${rect.top + window.scrollY}px`;
        menu.style.left = `${rect.right + 10}px`;

        document.body.appendChild(menu);

        // 点击其他地方关闭菜单
        const closeMenu = (e) => {
            if (!menu.contains(e.target)) {
                menu.remove();
                document.removeEventListener('click', closeMenu);
            }
        };

        setTimeout(() => {
            document.addEventListener('click', closeMenu);
        }, 100);
    }

    /**
     * 处理上下文菜单动作
     */
    handleContextMenuAction(action, conversation, element) {
        switch (action) {
            case 'copy':
                this.copyConversationToClipboard(conversation);
                break;
            case 'reply':
                this.replyToConversation(conversation);
                break;
            case 'details':
                this.showConversationDetails(conversation);
                break;
        }
    }

    /**
     * 回复对话
     */
    replyToConversation(conversation) {
        const messageInput = document.getElementById('message-input');
        if (messageInput) {
            const speakerInfo = this.getSpeakerInfo(conversation);
            const replyText = `@${speakerInfo.name} `;
            messageInput.value = replyText;
            messageInput.focus();
            messageInput.setSelectionRange(replyText.length, replyText.length);
        }
    }

    /**
     * 清理所有对话
     */
    clearAllConversations() {
        // 清理UI
        const container = this.getConversationContainer();
        if (container) {
            const conversations = container.querySelectorAll('.conversation-item');
            conversations.forEach(item => item.remove());
        }

        // 清理本地数据
        this.conversations = [];

        // 清理本地存储
        const sceneId = this.getSceneIdFromPage();
        if (sceneId) {
            localStorage.removeItem(`conversations_${sceneId}`);
        }

        // 更新计数
        this.updateConversationCount();

        console.log('🧹 所有对话已清理');
    }

    /**
     * 重新渲染所有对话
     */
    rerenderAllConversations() {
        console.log('🔄 重新渲染所有对话');

        const container = this.getConversationContainer();
        if (!container || !this.conversations) {
            return;
        }

        // 清空容器
        const conversations = container.querySelectorAll('.conversation-item');
        conversations.forEach(item => item.remove());

        // 重新添加所有对话
        this.conversations.forEach(conversation => {
            const element = this.createConversationElement(conversation);
            container.appendChild(element);
        });

        // 滚动到最新消息
        this.scrollToLatestMessage();
    }

    /**
     * 初始化对话功能
     */
    initConversationUI() {
        console.log('🎬 初始化对话UI功能');

        // 从本地存储加载对话
        this.loadConversationsFromLocal();

        // 如果有现有对话，重新渲染
        if (this.conversations && this.conversations.length > 0) {
            this.rerenderAllConversations();
        }

        // 绑定清理按钮（如果存在）
        const clearBtn = document.getElementById('clear-conversations-btn');
        if (clearBtn) {
            clearBtn.addEventListener('click', () => {
                if (confirm('确定要清理所有对话吗？')) {
                    this.clearAllConversations();
                }
            });
        }

        console.log('✅ 对话UI功能初始化完成');
    }

    /**
     * 更新对话统计信息
     */
    updateConversationStats() {
        try {
            // 计算基础统计数据
            const stats = {
                totalConversations: this.conversations ? this.conversations.length : 0,
                charactersCount: this.currentScene ? (this.currentScene.characters ? this.currentScene.characters.length : 0) : 0,
                lastActivity: new Date().toISOString(),
                activeCharacters: new Set()
            };

            // 统计活跃角色
            if (this.conversations) {
                this.conversations.forEach(conv => {
                    if (conv.character_id || conv.speaker_id) {
                        stats.activeCharacters.add(conv.character_id || conv.speaker_id);
                    }
                });
            }

            // 转换为数组
            stats.activeCharactersCount = stats.activeCharacters.size;
            delete stats.activeCharacters; // 移除Set对象，因为不能序列化

            // 更新UI中的统计显示
            this.updateStatsDisplay(stats);

            // 更新仪表板（如果可见）
            if (this.state.dashboardVisible && this.charts.size > 0) {
                // 延迟更新仪表板，避免频繁刷新
                if (this.statsUpdateTimer) {
                    clearTimeout(this.statsUpdateTimer);
                }

                this.statsUpdateTimer = setTimeout(() => {
                    this.refreshDashboardStats();
                }, 1000);
            }

            // 保存统计到本地存储（可选）
            this.saveStatsToLocalStorage(stats);

            console.log('📊 对话统计已更新:', stats);

        } catch (error) {
            console.error('❌ 更新对话统计失败:', error);
        }
    }

    /**
     * 更新统计显示
     */
    updateStatsDisplay(stats) {
        // 更新页面中的统计卡片
        const statsElements = {
            'total-conversations': stats.totalConversations,
            'characters-count': stats.charactersCount,
            'active-characters': stats.activeCharactersCount,
            'last-activity': this.formatLastActivity(stats.lastActivity)
        };

        Object.entries(statsElements).forEach(([id, value]) => {
            const element = document.getElementById(id);
            if (element) {
                if (typeof value === 'number') {
                    this.animateNumber(element, value);
                } else {
                    element.textContent = value;
                }
            }
        });

        // 更新统计卡片（如果存在）
        this.updateStatCards(stats);

        // 更新页面标题中的未读消息数（如果有新消息）
        this.updatePageTitle(stats);
    }

    /**
     * 更新统计卡片
     */
    updateStatCards(stats) {
        const cardMappings = [
            { selector: '.stat-card .stat-value:contains("参与角色")', value: stats.charactersCount },
            { selector: '.stat-card .stat-value:contains("对话次数")', value: stats.totalConversations },
            { selector: '.stat-card .stat-value:contains("活跃角色")', value: stats.activeCharactersCount }
        ];

        cardMappings.forEach(mapping => {
            // 查找包含特定文本的统计卡片
            const cards = document.querySelectorAll('.stat-card');
            cards.forEach(card => {
                const label = card.querySelector('.stat-label');
                if (label && label.textContent.includes(mapping.selector.split('"')[1])) {
                    const valueElement = card.querySelector('.stat-value');
                    if (valueElement) {
                        this.animateNumber(valueElement, mapping.value);
                    }
                }
            });
        });
    }

    /**
     * 数字动画效果
     */
    animateNumber(element, targetValue) {
        const currentValue = parseInt(element.textContent) || 0;
        const difference = targetValue - currentValue;

        if (difference === 0) return;

        const duration = 500; // 动画持续时间
        const steps = 20; // 动画步数
        const stepValue = difference / steps;
        const stepDuration = duration / steps;

        let currentStep = 0;

        const animate = () => {
            if (currentStep <= steps) {
                const newValue = Math.round(currentValue + (stepValue * currentStep));
                element.textContent = newValue;
                currentStep++;
                setTimeout(animate, stepDuration);
            } else {
                element.textContent = targetValue; // 确保最终值正确
            }
        };

        animate();
    }

    /**
     * 格式化最后活动时间
     */
    formatLastActivity(timestamp) {
        const now = new Date();
        const lastActivity = new Date(timestamp);
        const diffMs = now - lastActivity;
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMs / 3600000);

        if (diffMins < 1) {
            return '刚刚';
        } else if (diffMins < 60) {
            return `${diffMins}分钟前`;
        } else if (diffHours < 24) {
            return `${diffHours}小时前`;
        } else {
            return lastActivity.toLocaleDateString();
        }
    }

    /**
     * 更新页面标题
     */
    updatePageTitle(stats) {
        const baseTitle = document.title.split(' - ')[0] || 'SceneIntruderMCP';

        // 如果有新的对话，在标题中显示
        if (this.lastConversationCount && stats.totalConversations > this.lastConversationCount) {
            const newMessages = stats.totalConversations - this.lastConversationCount;
            document.title = `(${newMessages}) ${baseTitle}`;

            // 3秒后恢复原标题
            setTimeout(() => {
                document.title = baseTitle;
            }, 3000);
        }

        this.lastConversationCount = stats.totalConversations;
    }

    /**
     * 保存统计到本地存储
     */
    saveStatsToLocalStorage(stats) {
        try {
            const sceneId = this.getSceneIdFromPage();
            if (sceneId) {
                const key = `scene_stats_${sceneId}`;
                const statsData = {
                    ...stats,
                    timestamp: Date.now(),
                    sceneId: sceneId
                };
                localStorage.setItem(key, JSON.stringify(statsData));
            }
        } catch (error) {
            console.warn('保存统计到本地存储失败:', error);
        }
    }

    /**
     * 自动滚动到最新消息
     */
    scrollToLatestMessage() {
        try {
            // 查找对话容器
            const conversationContainer = this.findConversationContainer();

            if (!conversationContainer) {
                console.warn('未找到对话容器，无法滚动');
                return;
            }

            // 平滑滚动到底部
            this.smoothScrollToBottom(conversationContainer);

            // 如果页面不在焦点，闪烁标签页标题
            if (document.hidden) {
                this.flashPageTitle();
            }

            // 标记最新消息（添加视觉效果）
            this.highlightLatestMessage();

            console.log('📜 已滚动到最新消息');

        } catch (error) {
            console.error('❌ 滚动到最新消息失败:', error);
        }
    }

    /**
     * 查找对话容器
     */
    findConversationContainer() {
        // 按优先级查找对话容器
        const selectors = [
            '#conversation-history',
            '#chat-messages',
            '.conversation-container',
            '.chat-container',
            '.messages-container',
            '#messages',
            '.conversation-list'
        ];

        for (const selector of selectors) {
            const container = document.querySelector(selector);
            if (container) {
                return container;
            }
        }

        // 如果没找到专门的容器，查找包含消息的通用容器
        const messageElements = document.querySelectorAll('.message, .conversation-item, .chat-message');
        if (messageElements.length > 0) {
            return messageElements[0].closest('.container, .content, .main, main, body');
        }

        return null;
    }

    /**
     * 平滑滚动到底部
     */
    smoothScrollToBottom(container) {
        if (!container) return;

        // 检查是否需要滚动
        const isNearBottom = this.isNearBottom(container);

        // 如果用户已经滚动到其他位置，询问是否要滚动到最新消息
        if (!isNearBottom && this.shouldAskBeforeScroll()) {
            this.showScrollToBottomPrompt(container);
            return;
        }

        // 执行平滑滚动
        const scrollOptions = {
            top: container.scrollHeight,
            behavior: 'smooth'
        };

        // 优先使用 scrollTo，如果不支持则使用 scrollTop
        if (container.scrollTo) {
            container.scrollTo(scrollOptions);
        } else {
            // 降级到即时滚动
            container.scrollTop = container.scrollHeight;
        }

        // 添加滚动动画效果
        this.addScrollAnimation(container);
    }

    /**
     * 检查是否接近底部
     */
    isNearBottom(container, threshold = 100) {
        const scrollTop = container.scrollTop;
        const scrollHeight = container.scrollHeight;
        const clientHeight = container.clientHeight;

        return (scrollHeight - scrollTop - clientHeight) < threshold;
    }

    /**
     * 是否应该询问用户是否滚动
     */
    shouldAskBeforeScroll() {
        // 如果用户最近有交互行为，则不自动滚动
        const lastUserInteraction = this.realtimeState?.lastActivity || 0;
        const timeSinceInteraction = Date.now() - lastUserInteraction;

        return timeSinceInteraction > 10000; // 10秒内有交互则不自动滚动
    }

    /**
     * 显示滚动到底部提示
     */
    showScrollToBottomPrompt(container) {
        // 创建提示元素
        const prompt = document.createElement('div');
        prompt.className = 'scroll-to-bottom-prompt';
        prompt.innerHTML = `
        <div class="prompt-content">
            <span>有新消息</span>
            <button class="btn btn-sm btn-primary scroll-btn">查看</button>
            <button class="btn btn-sm btn-outline-secondary dismiss-btn">×</button>
        </div>
    `;

        // 添加样式
        prompt.style.cssText = `
        position: fixed;
        bottom: 20px;
        right: 20px;
        background: rgba(0, 123, 255, 0.9);
        color: white;
        padding: 10px 15px;
        border-radius: 25px;
        box-shadow: 0 4px 12px rgba(0,0,0,0.2);
        z-index: 1000;
        animation: slideInUp 0.3s ease;
        font-size: 14px;
    `;

        // 绑定事件
        prompt.querySelector('.scroll-btn').addEventListener('click', () => {
            this.smoothScrollToBottom(container);
            prompt.remove();
        });

        prompt.querySelector('.dismiss-btn').addEventListener('click', () => {
            prompt.remove();
        });

        // 添加到页面
        document.body.appendChild(prompt);

        // 5秒后自动消失
        setTimeout(() => {
            if (prompt.parentNode) {
                prompt.remove();
            }
        }, 5000);
    }

    /**
     * 添加滚动动画效果
     */
    addScrollAnimation(container) {
        container.style.transition = 'scroll-behavior 0.3s ease';

        setTimeout(() => {
            container.style.transition = '';
        }, 300);
    }

    /**
     * 高亮最新消息
     */
    highlightLatestMessage() {
        // 查找最新的消息元素
        const messageSelectors = [
            '.message:last-child',
            '.conversation-item:last-child',
            '.chat-message:last-child'
        ];

        let latestMessage = null;
        for (const selector of messageSelectors) {
            latestMessage = document.querySelector(selector);
            if (latestMessage) break;
        }

        if (!latestMessage) return;

        // 添加高亮效果
        latestMessage.classList.add('new-message-highlight');

        // 2秒后移除高亮
        setTimeout(() => {
            latestMessage.classList.remove('new-message-highlight');
        }, 2000);
    }

    /**
     * 闪烁页面标题
     */
    flashPageTitle() {
        const originalTitle = document.title;
        let flashCount = 0;
        const maxFlashes = 6;

        const flashInterval = setInterval(() => {
            document.title = flashCount % 2 === 0 ? '💬 新消息!' : originalTitle;
            flashCount++;

            if (flashCount >= maxFlashes) {
                clearInterval(flashInterval);
                document.title = originalTitle;
            }
        }, 500);

        // 当用户回到页面时停止闪烁
        const stopFlashing = () => {
            clearInterval(flashInterval);
            document.title = originalTitle;
            document.removeEventListener('visibilitychange', stopFlashing);
        };

        document.addEventListener('visibilitychange', () => {
            if (!document.hidden) {
                stopFlashing();
            }
        });
    }

    /**
     * 刷新仪表板统计
     */
    refreshDashboardStats() {
        if (!this.state.dashboardVisible) return;

        try {
            // 重新计算统计数据
            const stats = this.calculateSceneStats();

            // 更新统计卡片
            const statsContainer = document.querySelector('.stats-cards-container');
            if (statsContainer) {
                statsContainer.innerHTML = this.renderStatsCards();
            }

            // 更新角色互动图表
            if (this.charts.has('character-interaction')) {
                this.renderCharacterInteractionChart();
            }

            // 更新时间线图表
            if (this.charts.has('interaction-timeline')) {
                this.renderInteractionTimelineChart();
            }

            console.log('📊 仪表板统计已刷新');

        } catch (error) {
            console.error('❌ 刷新仪表板统计失败:', error);
        }
    }

    /**
     * 获取消息发送状态显示
     */
    showMessageSendingStatus(message) {
        // 创建发送状态提示
        const statusElement = document.createElement('div');
        statusElement.className = 'message-sending-status';
        statusElement.innerHTML = `
        <div class="status-content">
            <i class="bi bi-clock-history"></i>
            <span>发送中...</span>
        </div>
    `;

        // 添加样式
        statusElement.style.cssText = `
        position: fixed;
        top: 20px;
        left: 50%;
        transform: translateX(-50%);
        background: rgba(0, 123, 255, 0.1);
        border: 1px solid #007bff;
        color: #007bff;
        padding: 8px 16px;
        border-radius: 20px;
        font-size: 14px;
        z-index: 1000;
        animation: fadeInDown 0.3s ease;
    `;

        // 添加到页面
        document.body.appendChild(statusElement);

        // 2秒后自动移除
        setTimeout(() => {
            if (statusElement.parentNode) {
                statusElement.style.animation = 'fadeOutUp 0.3s ease';
                setTimeout(() => {
                    statusElement.remove();
                }, 300);
            }
        }, 2000);

        // 保存引用以便外部更新状态
        this.currentSendingStatus = statusElement;
    }

    /**
     * 更新消息发送状态
     */
    updateMessageSendingStatus(status, message) {
        if (!this.currentSendingStatus) return;

        const statusContent = this.currentSendingStatus.querySelector('.status-content');
        if (!statusContent) return;

        switch (status) {
            case 'success':
                statusContent.innerHTML = `
                <i class="bi bi-check-circle text-success"></i>
                <span class="text-success">发送成功</span>
            `;
                break;
            case 'error':
                statusContent.innerHTML = `
                <i class="bi bi-exclamation-circle text-danger"></i>
                <span class="text-danger">发送失败</span>
            `;
                break;
            case 'retry':
                statusContent.innerHTML = `
                <i class="bi bi-arrow-clockwise text-warning"></i>
                <span class="text-warning">重试中...</span>
            `;
                break;
        }

        // 1.5秒后移除状态提示
        setTimeout(() => {
            if (this.currentSendingStatus && this.currentSendingStatus.parentNode) {
                this.currentSendingStatus.remove();
                this.currentSendingStatus = null;
            }
        }, 1500);
    }

    /**
     * 初始化消息滚动监听
     */
    initScrollMonitoring() {
        const container = this.findConversationContainer();
        if (!container) return;

        // 监听滚动事件
        let scrollTimeout;
        container.addEventListener('scroll', () => {
            // 清除之前的定时器
            if (scrollTimeout) {
                clearTimeout(scrollTimeout);
            }

            // 延迟检查滚动状态
            scrollTimeout = setTimeout(() => {
                const isNearBottom = this.isNearBottom(container);

                // 更新滚动状态
                this.state.isScrolledToBottom = isNearBottom;

                // 如果用户滚动离开底部，显示"滚动到底部"按钮
                this.toggleScrollToBottomButton(!isNearBottom);
            }, 100);
        });

        console.log('📜 消息滚动监听已初始化');
    }

    /**
     * 切换滚动到底部按钮
     */
    toggleScrollToBottomButton(show) {
        let button = document.getElementById('scroll-to-bottom-btn');

        if (show && !button) {
            // 创建滚动按钮
            button = document.createElement('button');
            button.id = 'scroll-to-bottom-btn';
            button.className = 'btn btn-primary btn-sm';
            button.innerHTML = '<i class="bi bi-arrow-down"></i>';
            button.title = '滚动到最新消息';

            button.style.cssText = `
            position: fixed;
            bottom: 100px;
            right: 20px;
            border-radius: 50%;
            width: 40px;
            height: 40px;
            z-index: 1000;
            box-shadow: 0 2px 8px rgba(0,0,0,0.2);
            animation: fadeInUp 0.3s ease;
        `;

            button.addEventListener('click', () => {
                const container = this.findConversationContainer();
                if (container) {
                    this.smoothScrollToBottom(container);
                }
            });

            document.body.appendChild(button);
        } else if (!show && button) {
            // 隐藏滚动按钮
            button.style.animation = 'fadeOutDown 0.3s ease';
            setTimeout(() => {
                if (button.parentNode) {
                    button.remove();
                }
            }, 300);
        }
    }

    /**
     * 处理角色状态更新
     */
    handleCharacterStatusUpdate(data) {
        const { characterId, status, mood, activity } = data;

        console.log('👤 角色状态更新:', data);

        // 更新角色UI显示
        const characterElement = document.querySelector(`[data-character-id="${characterId}"]`);
        if (characterElement) {
            // 使用新的方法更新状态指示器和卡片样式
            this.updateCharacterStatusIndicator(characterElement, status, mood);
            this.updateCharacterCardStyle(characterElement, status);

            // 触发状态监听器
            this.triggerCharacterStatusListeners(characterId, status, mood);
        }

        // 显示状态变化通知
        if (status === 'busy') {
            const characterName = this.getCharacterName(characterId);
            Utils.showInfo(`${characterName} 正在忙碌中...`, 3000);
        } else if (status === 'online') {
            const characterName = this.getCharacterName(characterId);
            Utils.showSuccess(`${characterName} 现在可以互动了`, 2000);
        }

        // 更新全局角色状态缓存
        if (!this.characterStatusCache) {
            this.characterStatusCache = new Map();
        }

        this.characterStatusCache.set(characterId, { status, mood, timestamp: Date.now() });

        // 如果是当前选中的角色，更新交互界面
        if (this.realtimeState?.selectedCharacter === characterId) {
            this.updateSelectedCharacterInterface(status, mood);
        }
    }

    /**
 * 更新选中角色的交互界面
 */
    updateSelectedCharacterInterface(status, mood) {
        const messageInput = document.getElementById('message-input');
        const sendBtn = document.getElementById('send-btn');

        // 根据状态更新交互控件
        const isInteractive = ['online', 'typing'].includes(status);

        if (messageInput) {
            messageInput.disabled = !isInteractive;
            messageInput.placeholder = isInteractive ?
                '输入消息...' :
                `角色当前${this.getStatusConfig(status).text}，请稍后再试`;
        }

        if (sendBtn) {
            sendBtn.disabled = !isInteractive;
        }

        // 显示状态提示
        const statusDisplay = document.getElementById('selected-character-status');
        if (statusDisplay) {
            const statusConfig = this.getStatusConfig(status);
            const moodConfig = this.getMoodConfig(mood);

            statusDisplay.innerHTML = `
            <span class="status-info">
                <i class="bi bi-${statusConfig.icon}" style="color: ${statusConfig.color}"></i>
                ${statusConfig.text} ${moodConfig.emoji}
            </span>
        `;
        }
    }

    /**
     * 更新角色状态指示器
     */
    updateCharacterStatusIndicator(characterElement, status, mood) {
        if (!characterElement) return;

        try {
            // 查找或创建状态指示器
            let statusIndicator = characterElement.querySelector('.character-status-indicator');
            if (!statusIndicator) {
                statusIndicator = this.createCharacterStatusIndicator();
                characterElement.appendChild(statusIndicator);
            }

            // 更新状态显示
            this.updateStatusIndicatorContent(statusIndicator, status, mood);

            // 添加状态变化动画
            this.animateStatusChange(statusIndicator, status);

            console.log(`✅ 角色状态指示器已更新: status=${status}, mood=${mood}`);

        } catch (error) {
            console.error('❌ 更新角色状态指示器失败:', error);
        }
    }

    /**
     * 创建角色状态指示器元素
     */
    createCharacterStatusIndicator() {
        const indicator = document.createElement('div');
        indicator.className = 'character-status-indicator';
        indicator.innerHTML = `
        <div class="status-badge">
            <i class="status-icon"></i>
            <span class="status-text"></span>
        </div>
        <div class="mood-indicator">
            <span class="mood-emoji"></span>
            <span class="mood-text"></span>
        </div>
    `;

        // 添加基础样式
        indicator.style.cssText = `
        position: absolute;
        top: 5px;
        right: 5px;
        z-index: 10;
        display: flex;
        flex-direction: column;
        gap: 2px;
        font-size: 11px;
    `;

        return indicator;
    }

    /**
     * 更新状态指示器内容
     */
    updateStatusIndicatorContent(statusIndicator, status, mood) {
        const statusBadge = statusIndicator.querySelector('.status-badge');
        const statusIcon = statusIndicator.querySelector('.status-icon');
        const statusText = statusIndicator.querySelector('.status-text');
        const moodEmoji = statusIndicator.querySelector('.mood-emoji');
        const moodText = statusIndicator.querySelector('.mood-text');

        // 状态配置映射
        const statusConfig = this.getStatusConfig(status);
        const moodConfig = this.getMoodConfig(mood);

        // 更新状态徽章
        if (statusBadge && statusConfig) {
            statusBadge.className = `status-badge ${statusConfig.class}`;
            statusBadge.title = statusConfig.description;
        }

        // 更新状态图标
        if (statusIcon && statusConfig) {
            statusIcon.className = `status-icon bi bi-${statusConfig.icon}`;
        }

        // 更新状态文本
        if (statusText && statusConfig) {
            statusText.textContent = statusConfig.text;
        }

        // 更新心情指示器
        if (moodEmoji && moodConfig) {
            moodEmoji.textContent = moodConfig.emoji;
            moodEmoji.title = moodConfig.description;
        }

        if (moodText && moodConfig) {
            moodText.textContent = moodConfig.text;
            moodText.className = `mood-text ${moodConfig.class}`;
        }
    }

    /**
     * 获取状态配置
     */
    getStatusConfig(status) {
        const configs = {
            'online': {
                class: 'status-online',
                icon: 'circle-fill',
                text: '在线',
                description: '角色当前在线并可以互动',
                color: '#28a745'
            },
            'busy': {
                class: 'status-busy',
                icon: 'hourglass-split',
                text: '忙碌',
                description: '角色正在处理其他事务',
                color: '#ffc107'
            },
            'away': {
                class: 'status-away',
                icon: 'moon',
                text: '离开',
                description: '角色暂时离开',
                color: '#6c757d'
            },
            'offline': {
                class: 'status-offline',
                icon: 'circle',
                text: '离线',
                description: '角色当前离线',
                color: '#dc3545'
            },
            'typing': {
                class: 'status-typing',
                icon: 'three-dots',
                text: '输入中',
                description: '角色正在输入回复',
                color: '#007bff'
            },
            'thinking': {
                class: 'status-thinking',
                icon: 'lightbulb',
                text: '思考中',
                description: '角色正在思考回应',
                color: '#17a2b8'
            }
        };

        return configs[status] || configs['offline'];
    }

    /**
     * 获取心情配置
     */
    getMoodConfig(mood) {
        const configs = {
            'happy': {
                emoji: '😊',
                text: '开心',
                class: 'mood-positive',
                description: '角色心情愉快'
            },
            'excited': {
                emoji: '🤩',
                text: '兴奋',
                class: 'mood-positive',
                description: '角色情绪高涨'
            },
            'sad': {
                emoji: '😢',
                text: '伤心',
                class: 'mood-negative',
                description: '角色感到悲伤'
            },
            'angry': {
                emoji: '😠',
                text: '愤怒',
                class: 'mood-negative',
                description: '角色感到愤怒'
            },
            'confused': {
                emoji: '😕',
                text: '困惑',
                class: 'mood-neutral',
                description: '角色感到困惑'
            },
            'calm': {
                emoji: '😌',
                text: '平静',
                class: 'mood-neutral',
                description: '角色心情平静'
            },
            'surprised': {
                emoji: '😲',
                text: '惊讶',
                class: 'mood-neutral',
                description: '角色感到惊讶'
            },
            'tired': {
                emoji: '😴',
                text: '疲惫',
                class: 'mood-negative',
                description: '角色感到疲惫'
            },
            'curious': {
                emoji: '🤔',
                text: '好奇',
                class: 'mood-positive',
                description: '角色充满好奇'
            },
            'worried': {
                emoji: '😟',
                text: '担心',
                class: 'mood-negative',
                description: '角色感到担心'
            }
        };

        return configs[mood] || configs['calm'];
    }

    /**
     * 添加状态变化动画
     */
    animateStatusChange(statusIndicator, status) {
        // 移除之前的动画类
        statusIndicator.classList.remove('status-changing', 'status-pulse', 'status-glow');

        // 根据状态添加不同的动画效果
        switch (status) {
            case 'typing':
            case 'thinking':
                statusIndicator.classList.add('status-pulse');
                break;
            case 'online':
                statusIndicator.classList.add('status-glow');
                break;
            default:
                statusIndicator.classList.add('status-changing');
        }

        // 动画结束后移除类
        setTimeout(() => {
            statusIndicator.classList.remove('status-changing', 'status-pulse', 'status-glow');
        }, 1000);
    }

    /**
     * 更新角色卡片样式
     */
    updateCharacterCardStyle(characterElement, status) {
        if (!characterElement) return;

        try {
            // 移除所有状态相关的CSS类
            const statusClasses = [
                'character-online', 'character-busy', 'character-away',
                'character-offline', 'character-typing', 'character-thinking'
            ];

            statusClasses.forEach(cls => {
                characterElement.classList.remove(cls);
            });

            // 添加新的状态类
            const statusClass = `character-${status}`;
            characterElement.classList.add(statusClass);

            // 更新边框和背景色
            this.updateCharacterCardVisuals(characterElement, status);

            // 更新交互状态
            this.updateCharacterInteractivity(characterElement, status);

            // 添加状态变化的视觉反馈
            this.addCharacterCardAnimation(characterElement, status);

            console.log(`✅ 角色卡片样式已更新: ${status}`);

        } catch (error) {
            console.error('❌ 更新角色卡片样式失败:', error);
        }
    }

    /**
     * 更新角色卡片视觉效果
     */
    updateCharacterCardVisuals(characterElement, status) {
        const statusConfig = this.getStatusConfig(status);

        // 更新边框颜色
        characterElement.style.borderLeftColor = statusConfig.color;
        characterElement.style.borderLeftWidth = '4px';
        characterElement.style.borderLeftStyle = 'solid';

        // 更新背景色（轻微的状态提示）
        const alpha = status === 'offline' ? 0.05 : 0.1;
        const rgb = this.hexToRgb(statusConfig.color);
        if (rgb) {
            characterElement.style.backgroundColor = `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${alpha})`;
        }

        // 更新阴影效果
        if (status === 'online' || status === 'typing') {
            characterElement.style.boxShadow = `0 2px 8px rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, 0.3)`;
        } else {
            characterElement.style.boxShadow = '0 2px 4px rgba(0,0,0,0.1)';
        }
    }

    /**
     * 更新角色交互性
     */
    updateCharacterInteractivity(characterElement, status) {
        // 根据状态启用/禁用交互
        const isInteractive = ['online', 'typing', 'thinking'].includes(status);

        // 更新指针样式
        characterElement.style.cursor = isInteractive ? 'pointer' : 'default';

        // 更新不透明度
        characterElement.style.opacity = status === 'offline' ? '0.6' : '1';

        // 添加/移除交互提示
        if (isInteractive) {
            characterElement.title = '点击与此角色互动';
            characterElement.classList.add('interactive');
        } else {
            characterElement.title = `角色当前${this.getStatusConfig(status).text}，暂时无法互动`;
            characterElement.classList.remove('interactive');
        }

        // 更新内部按钮状态
        const buttons = characterElement.querySelectorAll('button, .btn');
        buttons.forEach(btn => {
            btn.disabled = !isInteractive;
            if (!isInteractive) {
                btn.classList.add('disabled');
            } else {
                btn.classList.remove('disabled');
            }
        });
    }

    /**
     * 添加角色卡片动画效果
     */
    addCharacterCardAnimation(characterElement, status) {
        // 移除之前的动画类
        const animationClasses = [
            'card-pulse', 'card-glow', 'card-shake', 'card-bounce', 'card-fade'
        ];
        animationClasses.forEach(cls => {
            characterElement.classList.remove(cls);
        });

        // 根据状态添加相应动画
        switch (status) {
            case 'online':
                characterElement.classList.add('card-glow');
                break;
            case 'typing':
            case 'thinking':
                characterElement.classList.add('card-pulse');
                break;
            case 'busy':
                characterElement.classList.add('card-shake');
                setTimeout(() => {
                    characterElement.classList.remove('card-shake');
                }, 1000);
                break;
            case 'offline':
                characterElement.classList.add('card-fade');
                break;
            default:
                characterElement.classList.add('card-bounce');
                setTimeout(() => {
                    characterElement.classList.remove('card-bounce');
                }, 600);
        }
    }

    /**
     * 辅助函数：十六进制颜色转RGB
     */
    hexToRgb(hex) {
        const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
        return result ? {
            r: parseInt(result[1], 16),
            g: parseInt(result[2], 16),
            b: parseInt(result[3], 16)
        } : null;
    }

    /**
     * 批量更新角色状态
     */
    updateMultipleCharacterStatus(updates) {
        if (!Array.isArray(updates)) return;

        updates.forEach(update => {
            const { characterId, status, mood } = update;
            const characterElement = document.querySelector(`[data-character-id="${characterId}"]`);

            if (characterElement) {
                this.updateCharacterStatusIndicator(characterElement, status, mood);
                this.updateCharacterCardStyle(characterElement, status);
            }
        });

        console.log(`✅ 批量更新了 ${updates.length} 个角色的状态`);
    }

    /**
     * 获取角色当前状态
     */
    getCharacterCurrentStatus(characterId) {
        const characterElement = document.querySelector(`[data-character-id="${characterId}"]`);
        if (!characterElement) return null;

        // 从CSS类中提取状态
        const statusClasses = [
            'character-online', 'character-busy', 'character-away',
            'character-offline', 'character-typing', 'character-thinking'
        ];

        for (const cls of statusClasses) {
            if (characterElement.classList.contains(cls)) {
                return cls.replace('character-', '');
            }
        }

        return 'offline'; // 默认状态
    }

    /**
     * 重置所有角色状态为默认
     */
    resetAllCharacterStatus() {
        const characterElements = document.querySelectorAll('[data-character-id]');

        characterElements.forEach(element => {
            this.updateCharacterStatusIndicator(element, 'offline', 'calm');
            this.updateCharacterCardStyle(element, 'offline');
        });

        console.log('✅ 所有角色状态已重置为默认');
    }

    /**
     * 添加角色状态监听器
     */
    addCharacterStatusListener(characterId, callback) {
        if (!this.characterStatusListeners) {
            this.characterStatusListeners = new Map();
        }

        if (!this.characterStatusListeners.has(characterId)) {
            this.characterStatusListeners.set(characterId, []);
        }

        this.characterStatusListeners.get(characterId).push(callback);
    }

    /**
     * 移除角色状态监听器
     */
    removeCharacterStatusListener(characterId, callback) {
        if (!this.characterStatusListeners || !this.characterStatusListeners.has(characterId)) {
            return;
        }

        const listeners = this.characterStatusListeners.get(characterId);
        const index = listeners.indexOf(callback);
        if (index > -1) {
            listeners.splice(index, 1);
        }
    }

    /**
     * 触发角色状态监听器
     */
    triggerCharacterStatusListeners(characterId, status, mood) {
        if (!this.characterStatusListeners || !this.characterStatusListeners.has(characterId)) {
            return;
        }

        const listeners = this.characterStatusListeners.get(characterId);
        listeners.forEach(callback => {
            try {
                callback(characterId, status, mood);
            } catch (error) {
                console.error('角色状态监听器执行失败:', error);
            }
        });
    }

    /**
     * 处理故事事件
     */
    handleStoryEvent(data) {
        const { eventType, eventData, description } = data;

        console.log('📖 故事事件:', data);

        // 显示故事事件通知
        if (description) {
            this.showStoryEventNotification(description, eventType);
        }

        // 更新故事界面
        if (this.storyData && eventData) {
            this.updateStoryDisplay(eventData);
        }

        // 刷新仪表板数据
        if (this.state.dashboardVisible) {
            setTimeout(() => {
                this.refreshDashboard();
            }, 1000);
        }
    }

    /**
     * 显示故事事件通知
     */
    showStoryEventNotification(description, eventType) {
        if (!description) return;

        try {
            // 根据事件类型确定通知样式
            const notificationConfig = this.getStoryEventNotificationConfig(eventType);

            // 创建通知元素
            const notification = this.createStoryEventNotification(description, notificationConfig);

            // 显示通知
            this.displayStoryNotification(notification);

            // 播放事件音效
            this.playStoryEventSound(eventType);

            // 记录故事事件
            this.logStoryEvent(description, eventType);

            console.log(`📖 故事事件通知已显示: ${eventType} - ${description}`);

        } catch (error) {
            console.error('❌ 显示故事事件通知失败:', error);
            // 降级处理
            this.showFallbackStoryNotification(description);
        }
    }

    /**
     * 获取故事事件通知配置
     */
    getStoryEventNotificationConfig(eventType) {
        const configs = {
            'story_progress': {
                icon: '📖',
                class: 'story-progress',
                color: '#007bff',
                duration: 4000,
                sound: 'story_progress'
            },
            'character_development': {
                icon: '👤',
                class: 'character-development',
                color: '#28a745',
                duration: 5000,
                sound: 'character_event'
            },
            'plot_twist': {
                icon: '🌪️',
                class: 'plot-twist',
                color: '#dc3545',
                duration: 6000,
                sound: 'plot_twist'
            },
            'location_discovered': {
                icon: '🗺️',
                class: 'location-discovered',
                color: '#17a2b8',
                duration: 4000,
                sound: 'discovery'
            },
            'item_acquired': {
                icon: '📦',
                class: 'item-acquired',
                color: '#ffc107',
                duration: 3000,
                sound: 'item_acquired'
            },
            'objective_completed': {
                icon: '✅',
                class: 'objective-completed',
                color: '#28a745',
                duration: 5000,
                sound: 'success'
            },
            'relationship_change': {
                icon: '💫',
                class: 'relationship-change',
                color: '#e83e8c',
                duration: 4000,
                sound: 'relationship'
            },
            'time_passage': {
                icon: '⏰',
                class: 'time-passage',
                color: '#6f42c1',
                duration: 3000,
                sound: 'time_event'
            },
            'environment_change': {
                icon: '🌍',
                class: 'environment-change',
                color: '#20c997',
                duration: 4000,
                sound: 'environment'
            },
            'conflict_escalation': {
                icon: '⚔️',
                class: 'conflict-escalation',
                color: '#fd7e14',
                duration: 5000,
                sound: 'conflict'
            },
            'mystery_revealed': {
                icon: '🔍',
                class: 'mystery-revealed',
                color: '#6610f2',
                duration: 6000,
                sound: 'revelation'
            }
        };

        return configs[eventType] || configs['story_progress'];
    }

    /**
     * 创建故事事件通知元素
     */
    createStoryEventNotification(description, config) {
        const notification = document.createElement('div');
        notification.className = `story-event-notification ${config.class}`;

        // 设置基础样式
        notification.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        max-width: 400px;
        background: linear-gradient(135deg, ${config.color}15, ${config.color}25);
        border: 1px solid ${config.color}40;
        border-left: 4px solid ${config.color};
        border-radius: 8px;
        padding: 16px 20px;
        margin-bottom: 10px;
        z-index: 1000;
        box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        backdrop-filter: blur(10px);
        animation: storyNotificationSlideIn 0.4s ease-out;
        font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
    `;

        // 创建内容结构
        notification.innerHTML = `
        <div class="story-notification-header">
            <span class="story-event-icon">${config.icon}</span>
            <span class="story-event-type">${this.getEventTypeLabel(notification.classList[1])}</span>
            <button class="story-notification-close" aria-label="关闭通知">×</button>
        </div>
        <div class="story-notification-body">
            <p class="story-event-description">${this.escapeHtml(description)}</p>
        </div>
        <div class="story-notification-footer">
            <div class="story-progress-bar">
                <div class="story-progress-fill" style="background-color: ${config.color}"></div>
            </div>
            <span class="story-notification-time">${new Date().toLocaleTimeString()}</span>
        </div>
    `;

        // 存储配置信息
        notification._config = config;
        notification._timestamp = Date.now();

        return notification;
    }

    /**
     * 显示故事通知
     */
    displayStoryNotification(notification) {
        // 添加到页面
        document.body.appendChild(notification);

        // 获取配置
        const config = notification._config;

        // 启动进度条动画
        const progressFill = notification.querySelector('.story-progress-fill');
        if (progressFill) {
            setTimeout(() => {
                progressFill.style.width = '100%';
                progressFill.style.transition = `width ${config.duration}ms linear`;
            }, 100);
        }

        // 绑定关闭按钮事件
        const closeBtn = notification.querySelector('.story-notification-close');
        if (closeBtn) {
            closeBtn.addEventListener('click', () => {
                this.dismissStoryNotification(notification);
            });
        }

        // 添加点击展开功能
        notification.addEventListener('click', (e) => {
            if (!e.target.classList.contains('story-notification-close')) {
                this.expandStoryNotification(notification);
            }
        });

        // 自动消失
        setTimeout(() => {
            if (notification.parentNode) {
                this.dismissStoryNotification(notification);
            }
        }, config.duration);

        // 管理通知堆叠
        this.manageNotificationStack();
    }

    /**
     * 关闭故事通知
     */
    dismissStoryNotification(notification) {
        if (!notification || !notification.parentNode) return;

        notification.style.animation = 'storyNotificationSlideOut 0.3s ease-in forwards';

        setTimeout(() => {
            if (notification.parentNode) {
                notification.remove();
            }
            this.manageNotificationStack();
        }, 300);
    }

    /**
     * 展开故事通知
     */
    expandStoryNotification(notification) {
        const body = notification.querySelector('.story-notification-body');
        const description = body.querySelector('.story-event-description');

        if (body && description) {
            // 切换展开状态
            const isExpanded = notification.classList.contains('expanded');

            if (isExpanded) {
                notification.classList.remove('expanded');
                description.style.maxHeight = '60px';
                description.style.overflow = 'hidden';
            } else {
                notification.classList.add('expanded');
                description.style.maxHeight = 'none';
                description.style.overflow = 'visible';

                // 添加展开指示器
                if (!body.querySelector('.expansion-indicator')) {
                    const indicator = document.createElement('div');
                    indicator.className = 'expansion-indicator';
                    indicator.innerHTML = '<small>点击收起</small>';
                    body.appendChild(indicator);
                }
            }
        }
    }

    /**
     * 管理通知堆叠
     */
    manageNotificationStack() {
        const notifications = document.querySelectorAll('.story-event-notification');

        // 重新排列通知位置
        notifications.forEach((notification, index) => {
            const topOffset = 20 + (index * 10); // 每个通知间隔10px
            notification.style.top = `${topOffset}px`;
            notification.style.zIndex = 1000 - index;
        });

        // 如果通知太多，移除最老的
        if (notifications.length > 5) {
            for (let i = 5; i < notifications.length; i++) {
                this.dismissStoryNotification(notifications[i]);
            }
        }
    }

    /**
     * 播放故事事件音效
     */
    playStoryEventSound(eventType) {
        // 检查是否启用音效
        if (!this.state.soundEnabled) return;

        try {
            // 尝试使用实时管理器播放音效
            if (this.realtimeManager && this.realtimeManager.playStorySound) {
                this.realtimeManager.playStorySound(eventType);
                return;
            }

            // 降级到简单音效播放
            this.playSimpleStorySound(eventType);

        } catch (error) {
            console.warn('播放故事音效失败:', error);
        }
    }

    /**
     * 播放简单故事音效
     */
    playSimpleStorySound(eventType) {
        // 音频频率映射
        const soundMap = {
            'story_progress': 440,     // A4
            'character_development': 523, // C5
            'plot_twist': 659,         // E5
            'location_discovered': 392, // G4
            'item_acquired': 523,      // C5
            'objective_completed': 659, // E5
            'relationship_change': 587, // D5
            'time_passage': 349,       // F4
            'environment_change': 440,  // A4
            'conflict_escalation': 294, // D4
            'mystery_revealed': 740     // F#5
        };

        const frequency = soundMap[eventType] || 440;

        // 使用Web Audio API创建简单音效
        if (typeof AudioContext !== 'undefined' || typeof webkitAudioContext !== 'undefined') {
            try {
                const audioContext = new (AudioContext || webkitAudioContext)();
                const oscillator = audioContext.createOscillator();
                const gainNode = audioContext.createGain();

                oscillator.connect(gainNode);
                gainNode.connect(audioContext.destination);

                oscillator.frequency.value = frequency;
                oscillator.type = 'sine';

                gainNode.gain.setValueAtTime(0.1, audioContext.currentTime);
                gainNode.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.5);

                oscillator.start(audioContext.currentTime);
                oscillator.stop(audioContext.currentTime + 0.5);
            } catch (error) {
                console.warn('Web Audio API 音效播放失败:', error);
            }
        }
    }

    /**
     * 记录故事事件
     */
    logStoryEvent(description, eventType) {
        // 记录到本地存储
        try {
            const sceneId = this.getSceneIdFromPage();
            if (sceneId) {
                const key = `story_events_${sceneId}`;
                let events = JSON.parse(localStorage.getItem(key) || '[]');

                events.push({
                    description,
                    eventType,
                    timestamp: new Date().toISOString(),
                    sceneId
                });

                // 只保留最近50个事件
                if (events.length > 50) {
                    events = events.slice(-50);
                }

                localStorage.setItem(key, JSON.stringify(events));
            }
        } catch (error) {
            console.warn('记录故事事件失败:', error);
        }

        // 更新故事统计
        if (this.storyStats) {
            this.storyStats.totalEvents = (this.storyStats.totalEvents || 0) + 1;
            this.storyStats.eventTypes = this.storyStats.eventTypes || {};
            this.storyStats.eventTypes[eventType] = (this.storyStats.eventTypes[eventType] || 0) + 1;
        }
    }

    /**
     * 显示场景变化通知
     */
    showSceneChangeNotification(description) {
        if (!description) return;

        try {
            // 创建场景变化通知
            const notification = document.createElement('div');
            notification.className = 'scene-change-notification';

            notification.innerHTML = `
            <div class="scene-notification-content">
                <div class="scene-notification-icon">🎭</div>
                <div class="scene-notification-text">
                    <h6>场景变化</h6>
                    <p>${this.escapeHtml(description)}</p>
                </div>
                <button class="scene-notification-close">×</button>
            </div>
        `;

            // 设置样式
            notification.style.cssText = `
            position: fixed;
            bottom: 20px;
            left: 20px;
            max-width: 350px;
            background: linear-gradient(135deg, #6f42c1, #7952b3);
            color: white;
            border-radius: 10px;
            padding: 16px;
            box-shadow: 0 6px 20px rgba(111, 66, 193, 0.3);
            z-index: 1000;
            animation: sceneNotificationSlideUp 0.4s ease-out;
        `;

            // 绑定关闭事件
            const closeBtn = notification.querySelector('.scene-notification-close');
            closeBtn.addEventListener('click', () => {
                notification.style.animation = 'sceneNotificationSlideDown 0.3s ease-in forwards';
                setTimeout(() => notification.remove(), 300);
            });

            // 添加到页面
            document.body.appendChild(notification);

            // 自动关闭
            setTimeout(() => {
                if (notification.parentNode) {
                    notification.style.animation = 'sceneNotificationSlideDown 0.3s ease-in forwards';
                    setTimeout(() => notification.remove(), 300);
                }
            }, 5000);

            console.log('🎭 场景变化通知已显示:', description);

        } catch (error) {
            console.error('❌ 显示场景变化通知失败:', error);
            // 降级处理
            Utils.showInfo(`场景变化: ${description}`, 5000);
        }
    }

    /**
     * 更新在线用户列表
     */
    updateOnlineUsersList(data) {
        try {
            const { userId, username, action, sceneId } = data;

            // 查找或创建在线用户容器
            let usersContainer = document.getElementById('online-users-list');
            if (!usersContainer) {
                usersContainer = this.createOnlineUsersContainer();
            }

            if (action === 'joined') {
                this.addUserToOnlineList(usersContainer, userId, username);
            } else if (action === 'left') {
                this.removeUserFromOnlineList(usersContainer, userId);
            }

            // 更新用户计数
            this.updateOnlineUsersCount();

            console.log(`👥 在线用户列表已更新: ${username} ${action}`);

        } catch (error) {
            console.error('❌ 更新在线用户列表失败:', error);
        }
    }

    /**
     * 创建在线用户容器
     */
    createOnlineUsersContainer() {
        const container = document.createElement('div');
        container.id = 'online-users-list';
        container.className = 'online-users-container';

        container.innerHTML = `
        <div class="online-users-header">
            <h6>
                <i class="bi bi-people"></i>
                在线用户 (<span id="online-users-count">0</span>)
            </h6>
            <button class="btn btn-sm btn-outline-secondary toggle-users-list" title="折叠/展开">
                <i class="bi bi-chevron-up"></i>
            </button>
        </div>
        <div class="online-users-list" id="users-list-content"></div>
    `;

        // 设置样式
        container.style.cssText = `
        position: fixed;
        top: 80px;
        left: 20px;
        width: 200px;
        background: rgba(255, 255, 255, 0.95);
        border: 1px solid #e9ecef;
        border-radius: 8px;
        padding: 12px;
        z-index: 999;
        box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        backdrop-filter: blur(5px);
    `;

        // 绑定折叠/展开事件
        const toggleBtn = container.querySelector('.toggle-users-list');
        const listContent = container.querySelector('#users-list-content');

        toggleBtn.addEventListener('click', () => {
            const isCollapsed = listContent.style.display === 'none';
            listContent.style.display = isCollapsed ? 'block' : 'none';
            toggleBtn.querySelector('i').className = isCollapsed ?
                'bi bi-chevron-up' : 'bi bi-chevron-down';
        });

        // 添加到页面
        document.body.appendChild(container);

        return container;
    }

    /**
     * 添加用户到在线列表
     */
    addUserToOnlineList(container, userId, username) {
        const listContent = container.querySelector('#users-list-content');

        // 检查用户是否已存在
        if (listContent.querySelector(`[data-user-id="${userId}"]`)) {
            return;
        }

        // 创建用户元素
        const userElement = document.createElement('div');
        userElement.className = 'online-user-item';
        userElement.dataset.userId = userId;

        userElement.innerHTML = `
        <div class="user-avatar">
            <i class="bi bi-person-circle"></i>
        </div>
        <div class="user-info">
            <span class="username">${this.escapeHtml(username)}</span>
            <div class="user-status online">
                <span class="status-dot"></span>
                <span class="status-text">在线</span>
            </div>
        </div>
    `;

        // 添加样式
        userElement.style.cssText = `
        display: flex;
        align-items: center;
        padding: 6px 0;
        border-bottom: 1px solid #f1f3f4;
        font-size: 12px;
    `;

        listContent.appendChild(userElement);
    }

    /**
     * 从在线列表移除用户
     */
    removeUserFromOnlineList(container, userId) {
        const userElement = container.querySelector(`[data-user-id="${userId}"]`);
        if (userElement) {
            userElement.style.animation = 'fadeOutLeft 0.3s ease-in forwards';
            setTimeout(() => {
                if (userElement.parentNode) {
                    userElement.remove();
                    this.updateOnlineUsersCount();
                }
            }, 300);
        }
    }

    /**
     * 更新在线用户数量
     */
    updateOnlineUsersCount() {
        const countElement = document.getElementById('online-users-count');
        const usersList = document.getElementById('users-list-content');

        if (countElement && usersList) {
            const count = usersList.querySelectorAll('.online-user-item').length;
            countElement.textContent = count;
        }
    }

    /**
     * 更新故事显示
     */
    updateStoryDisplay(eventData) {
        try {
            // 更新故事数据
            if (eventData && this.storyData) {
                // 合并新的故事数据
                Object.assign(this.storyData, eventData);
            }

            // 重新渲染故事界面
            if (this.renderStoryInterface) {
                this.renderStoryInterface();
            }

            // 更新故事进度显示
            this.updateStoryProgressDisplay(eventData);

            // 如果仪表板可见，更新相关图表
            if (this.state.dashboardVisible) {
                setTimeout(() => {
                    if (this.charts.has('story-progress')) {
                        this.renderStoryProgressChart();
                    }
                }, 500);
            }

            console.log('📖 故事显示已更新');

        } catch (error) {
            console.error('❌ 更新故事显示失败:', error);
        }
    }

    /**
     * 更新故事进度显示
     */
    updateStoryProgressDisplay(eventData) {
        // 查找故事进度元素
        const progressElements = [
            '#story-progress',
            '.story-progress',
            '#story-completion',
            '.story-completion-bar'
        ];

        for (const selector of progressElements) {
            const element = document.querySelector(selector);
            if (element) {
                this.updateProgressElement(element, eventData);
            }
        }
    }

    /**
     * 更新进度元素
     */
    updateProgressElement(element, eventData) {
        if (!eventData || !eventData.progress) return;

        const progress = eventData.progress;

        // 如果是进度条
        if (element.classList.contains('progress-bar') || element.querySelector('.progress-bar')) {
            const progressBar = element.classList.contains('progress-bar') ?
                element : element.querySelector('.progress-bar');

            if (progressBar) {
                progressBar.style.width = `${progress}%`;
                progressBar.setAttribute('aria-valuenow', progress);

                // 更新文本
                const progressText = element.querySelector('.progress-text');
                if (progressText) {
                    progressText.textContent = `${Math.round(progress)}%`;
                }
            }
        }
        // 如果是文本显示
        else if (element.textContent !== undefined) {
            element.textContent = `故事进度: ${Math.round(progress)}%`;
        }
    }

    /**
     * 降级故事通知显示
     */
    showFallbackStoryNotification(description) {
        // 使用Utils显示简单通知
        if (typeof Utils !== 'undefined' && Utils.showInfo) {
            Utils.showInfo(`📖 ${description}`, 4000);
        } else {
            // 最后的降级方案
            console.log(`📖 故事事件: ${description}`);

            // 尝试浏览器通知API
            if ('Notification' in window && Notification.permission === 'granted') {
                new Notification('故事事件', {
                    body: description,
                    icon: '/static/favicon.ico'
                });
            }
        }
    }

    /**
     * 获取事件类型标签
     */
    getEventTypeLabel(eventClass) {
        const labels = {
            'story-progress': '故事进展',
            'character-development': '角色发展',
            'plot-twist': '剧情转折',
            'location-discovered': '地点发现',
            'item-acquired': '物品获得',
            'objective-completed': '目标完成',
            'relationship-change': '关系变化',
            'time-passage': '时间流逝',
            'environment-change': '环境变化',
            'conflict-escalation': '冲突升级',
            'mystery-revealed': '谜团揭示'
        };

        return labels[eventClass] || '故事事件';
    }

    /**
     * HTML转义
     */
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /**
     * 获取故事事件历史
     */
    getStoryEventHistory(sceneId = null) {
        try {
            const targetSceneId = sceneId || this.getSceneIdFromPage();
            if (!targetSceneId) return [];

            const key = `story_events_${targetSceneId}`;
            return JSON.parse(localStorage.getItem(key) || '[]');
        } catch (error) {
            console.warn('获取故事事件历史失败:', error);
            return [];
        }
    }

    /**
     * 清除故事事件历史
     */
    clearStoryEventHistory(sceneId = null) {
        try {
            const targetSceneId = sceneId || this.getSceneIdFromPage();
            if (!targetSceneId) return;

            const key = `story_events_${targetSceneId}`;
            localStorage.removeItem(key);

            console.log('📖 故事事件历史已清除');
        } catch (error) {
            console.warn('清除故事事件历史失败:', error);
        }
    }

    /**
     * 导出故事事件
     */
    exportStoryEvents(sceneId = null, format = 'json') {
        try {
            const events = this.getStoryEventHistory(sceneId);
            const targetSceneId = sceneId || this.getSceneIdFromPage();

            if (events.length === 0) {
                Utils.showWarning('没有故事事件可导出');
                return;
            }

            let content, filename, mimeType;

            switch (format.toLowerCase()) {
                case 'json':
                    content = JSON.stringify(events, null, 2);
                    filename = `story_events_${targetSceneId}.json`;
                    mimeType = 'application/json';
                    break;

                case 'txt':
                    content = events.map(event =>
                        `[${event.timestamp}] ${event.eventType}: ${event.description}`
                    ).join('\n');
                    filename = `story_events_${targetSceneId}.txt`;
                    mimeType = 'text/plain';
                    break;

                case 'markdown':
                    content = `# 故事事件记录 - ${targetSceneId}\n\n` +
                        events.map(event =>
                            `## ${this.getEventTypeLabel(event.eventType)}\n` +
                            `**时间**: ${new Date(event.timestamp).toLocaleString()}\n` +
                            `**描述**: ${event.description}\n`
                        ).join('\n');
                    filename = `story_events_${targetSceneId}.md`;
                    mimeType = 'text/markdown';
                    break;

                default:
                    throw new Error('不支持的导出格式');
            }

            // 创建并下载文件
            const blob = new Blob([content], { type: mimeType });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = filename;
            a.click();
            URL.revokeObjectURL(url);

            Utils.showSuccess(`故事事件已导出为 ${filename}`);

        } catch (error) {
            console.error('导出故事事件失败:', error);
            Utils.showError('导出故事事件失败: ' + error.message);
        }
    }

    /**
     * 处理用户在线状态
     */
    handleUserPresence(data) {
        const { userId, username, action } = data;

        console.log('👥 用户在线状态:', data);

        // 更新在线用户列表
        this.updateOnlineUsersList(data);

        // 显示用户进出通知
        if (action === 'joined') {
            Utils.showInfo(`${username} 加入了场景`, 2000);
        } else if (action === 'left') {
            Utils.showInfo(`${username} 离开了场景`, 2000);
        }
    }

    /**
     * 处理场景状态更新
     */
    handleSceneStateUpdate(data) {
        const { state, changes } = data;

        console.log('🎭 场景状态更新:', data);

        // 更新场景状态
        if (this.currentScene) {
            Object.assign(this.currentScene, state);
        }

        // 如果有重要变化，通知用户
        if (changes && changes.length > 0) {
            const importantChanges = changes.filter(change =>
                change.type === 'environment_change' ||
                change.type === 'time_change'
            );

            if (importantChanges.length > 0) {
                const descriptions = importantChanges.map(change => change.description);
                this.showSceneChangeNotification(descriptions.join(', '));
            }
        }
    }

    /**
     * 更新场景状态
     */
    updateSceneState(newState) {
        if (!newState || typeof newState !== 'object') {
            console.warn('无效的场景状态数据:', newState);
            return;
        }

        try {
            console.log('🎭 更新场景状态:', newState);

            // 备份当前状态
            const previousState = this.currentScene ? { ...this.currentScene } : null;

            // 更新当前场景状态
            if (this.currentScene) {
                // 深度合并状态更新
                this.currentScene = {
                    ...this.currentScene,
                    ...newState,
                    last_updated: new Date().toISOString()
                };
            } else {
                // 如果当前场景不存在，创建新的场景状态
                this.currentScene = {
                    ...newState,
                    last_updated: new Date().toISOString()
                };
            }

            // 更新聚合数据中的场景状态
            if (this.aggregateData) {
                if (this.aggregateData.data) {
                    this.aggregateData.data.scene = this.currentScene;
                } else {
                    this.aggregateData.scene = this.currentScene;
                }
            }

            // 检测具体的状态变化
            const changes = this.detectSceneStateChanges(previousState, this.currentScene);

            // 处理状态变化
            this.handleSceneStateChanges(changes);

            // 更新UI显示
            this.updateSceneStateUI(changes);

            // 触发场景状态更新事件
            this.triggerSceneStateEvent('scene_state_updated', {
                previous_state: previousState,
                current_state: this.currentScene,
                changes: changes,
                timestamp: new Date().toISOString()
            });

            console.log('✅ 场景状态更新完成');

        } catch (error) {
            console.error('❌ 更新场景状态失败:', error);
            this.showError('场景状态更新失败: ' + error.message);
        }
    }

    /**
     * 检测场景状态变化
     */
    detectSceneStateChanges(previousState, currentState) {
        const changes = [];

        if (!previousState) {
            changes.push({
                type: 'scene_initialized',
                description: '场景已初始化',
                data: currentState
            });
            return changes;
        }

        // 检测场景基本信息变化
        if (previousState.title !== currentState.title) {
            changes.push({
                type: 'title_changed',
                description: `场景标题从 "${previousState.title}" 更改为 "${currentState.title}"`,
                previous: previousState.title,
                current: currentState.title
            });
        }

        if (previousState.description !== currentState.description) {
            changes.push({
                type: 'description_changed',
                description: '场景描述已更新',
                previous: previousState.description,
                current: currentState.description
            });
        }

        // 检测状态字段变化
        if (previousState.status !== currentState.status) {
            changes.push({
                type: 'status_changed',
                description: `场景状态从 "${previousState.status}" 更改为 "${currentState.status}"`,
                previous: previousState.status,
                current: currentState.status
            });
        }

        // 检测角色数量变化
        const prevCharacterCount = previousState.characters ? previousState.characters.length : 0;
        const currCharacterCount = currentState.characters ? currentState.characters.length : 0;

        if (prevCharacterCount !== currCharacterCount) {
            changes.push({
                type: 'character_count_changed',
                description: `角色数量从 ${prevCharacterCount} 个变为 ${currCharacterCount} 个`,
                previous: prevCharacterCount,
                current: currCharacterCount
            });
        }

        // 检测设置变化
        if (JSON.stringify(previousState.settings) !== JSON.stringify(currentState.settings)) {
            changes.push({
                type: 'settings_changed',
                description: '场景设置已更新',
                previous: previousState.settings,
                current: currentState.settings
            });
        }

        // 检测上下文变化
        if (JSON.stringify(previousState.context) !== JSON.stringify(currentState.context)) {
            changes.push({
                type: 'context_changed',
                description: '场景上下文已更新',
                previous: previousState.context,
                current: currentState.context
            });
        }

        return changes;
    }

    /**
     * 处理场景状态变化
     */
    handleSceneStateChanges(changes) {
        if (!changes || changes.length === 0) {
            return;
        }

        changes.forEach(change => {
            switch (change.type) {
                case 'scene_initialized':
                    this.handleSceneInitialized(change.data);
                    break;

                case 'title_changed':
                    this.handleSceneTitleChanged(change.previous, change.current);
                    break;

                case 'description_changed':
                    this.handleSceneDescriptionChanged(change.previous, change.current);
                    break;

                case 'status_changed':
                    this.handleSceneStatusChanged(change.previous, change.current);
                    break;

                case 'character_count_changed':
                    this.handleCharacterCountChanged(change.previous, change.current);
                    break;

                case 'settings_changed':
                    this.handleSceneSettingsChanged(change.previous, change.current);
                    break;

                case 'context_changed':
                    this.handleSceneContextChanged(change.previous, change.current);
                    break;

                default:
                    console.log('未知的场景状态变化类型:', change.type);
            }
        });
    }

    /**
     * 处理场景初始化
     */
    handleSceneInitialized(sceneData) {
        console.log('🎭 场景已初始化:', sceneData);

        // 更新页面标题
        if (sceneData.title) {
            document.title = `${sceneData.title} - SceneIntruderMCP`;
        }

        // 显示初始化通知
        this.showSuccess('场景已成功加载');
    }

    /**
     * 处理场景标题变化
     */
    handleSceneTitleChanged(previousTitle, currentTitle) {
        console.log('📝 场景标题已更新:', previousTitle, '->', currentTitle);

        // 更新页面标题
        document.title = `${currentTitle} - SceneIntruderMCP`;

        // 更新标题显示元素
        const titleElements = document.querySelectorAll('.scene-title, .current-scene-title, #scene-title');
        titleElements.forEach(element => {
            element.textContent = currentTitle;
        });

        // 显示更新通知
        this.showInfo(`场景标题已更新为: ${currentTitle}`);
    }

    /**
     * 处理场景描述变化
     */
    handleSceneDescriptionChanged(previousDescription, currentDescription) {
        console.log('📄 场景描述已更新');

        // 更新描述显示元素
        const descriptionElements = document.querySelectorAll('.scene-description, .current-scene-description, #scene-description');
        descriptionElements.forEach(element => {
            element.textContent = currentDescription;
        });

        // 显示更新通知
        this.showInfo('场景描述已更新');
    }

    /**
     * 处理场景状态变化
     */
    handleSceneStatusChanged(previousStatus, currentStatus) {
        console.log('📊 场景状态已更新:', previousStatus, '->', currentStatus);

        // 更新状态显示元素
        const statusElements = document.querySelectorAll('.scene-status, .current-scene-status, #scene-status');
        statusElements.forEach(element => {
            element.textContent = this.formatSceneStatus(currentStatus);
            element.className = `scene-status status-${currentStatus}`;
        });

        // 根据状态显示不同的通知
        switch (currentStatus) {
            case 'active':
                this.showSuccess('场景已激活，可以开始互动');
                break;
            case 'paused':
                this.showWarning('场景已暂停');
                break;
            case 'completed':
                this.showSuccess('场景已完成');
                break;
            case 'error':
                this.showError('场景出现错误');
                break;
            default:
                this.showInfo(`场景状态: ${this.formatSceneStatus(currentStatus)}`);
        }
    }

    /**
     * 处理角色数量变化
     */
    handleCharacterCountChanged(previousCount, currentCount) {
        console.log('👥 角色数量已更新:', previousCount, '->', currentCount);

        // 重新渲染角色列表
        if (this.renderCharacterList) {
            this.renderCharacterList();
        }

        // 更新角色计数显示
        const countElements = document.querySelectorAll('.character-count, .characters-count, #character-count');
        countElements.forEach(element => {
            element.textContent = currentCount;
        });

        // 显示通知
        if (currentCount > previousCount) {
            const addedCount = currentCount - previousCount;
            this.showInfo(`新增了 ${addedCount} 个角色`);
        } else if (currentCount < previousCount) {
            const removedCount = previousCount - currentCount;
            this.showInfo(`移除了 ${removedCount} 个角色`);
        }
    }

    /**
     * 处理场景设置变化
     */
    handleSceneSettingsChanged(previousSettings, currentSettings) {
        console.log('⚙️ 场景设置已更新');

        // 检查具体的设置变化
        if (previousSettings && currentSettings) {
            // 检查创意等级变化
            if (previousSettings.creativity_level !== currentSettings.creativity_level) {
                this.showInfo(`创意等级已更新为: ${this.formatCreativityLevel(currentSettings.creativity_level)}`);
            }

            // 检查响应长度变化
            if (previousSettings.response_length !== currentSettings.response_length) {
                this.showInfo(`响应长度已更新为: ${this.formatResponseLength(currentSettings.response_length)}`);
            }

            // 检查语言风格变化
            if (previousSettings.language_style !== currentSettings.language_style) {
                this.showInfo(`语言风格已更新为: ${this.formatLanguageStyle(currentSettings.language_style)}`);
            }
        }

        // 应用新设置
        this.applySceneSettings(currentSettings);
    }

    /**
     * 处理场景上下文变化
     */
    handleSceneContextChanged(previousContext, currentContext) {
        console.log('📝 场景上下文已更新');

        // 检查上下文的具体变化
        if (previousContext && currentContext) {
            // 检查时间设定变化
            if (previousContext.time_setting !== currentContext.time_setting) {
                this.showInfo('时间设定已更新');
            }

            // 检查地点变化
            if (previousContext.location !== currentContext.location) {
                this.showInfo(`当前地点: ${currentContext.location}`);
            }

            // 检查氛围变化
            if (previousContext.mood !== currentContext.mood) {
                this.showInfo(`场景氛围: ${currentContext.mood}`);
            }
        }

        // 更新上下文显示
        this.updateSceneContextDisplay(currentContext);
    }

    /**
     * 更新场景状态UI
     */
    updateSceneStateUI(changes) {
        if (!changes || changes.length === 0) {
            return;
        }

        // 更新场景信息显示
        this.updateSceneInfoDisplay();

        // 更新状态指示器
        this.updateSceneStatusIndicator();

        // 更新场景头部信息
        this.updateSceneHeaderInfo();

        // 如果有重要变化，刷新仪表板
        const importantChanges = ['character_count_changed', 'status_changed', 'settings_changed'];
        const hasImportantChanges = changes.some(change => importantChanges.includes(change.type));

        if (hasImportantChanges && this.state.dashboardVisible) {
            setTimeout(() => {
                this.refreshDashboard();
            }, 1000);
        }
    }

    /**
     * 更新场景信息显示
     */
    updateSceneInfoDisplay() {
        if (!this.currentScene) return;

        // 更新场景标题
        const titleElements = document.querySelectorAll('.scene-title, .current-scene-title, #scene-title');
        titleElements.forEach(element => {
            element.textContent = this.currentScene.title || this.currentScene.name || '未命名场景';
        });

        // 更新场景描述
        const descriptionElements = document.querySelectorAll('.scene-description, .current-scene-description, #scene-description');
        descriptionElements.forEach(element => {
            element.textContent = this.currentScene.description || '无描述';
        });

        // 更新最后更新时间
        const updateTimeElements = document.querySelectorAll('.scene-last-updated, #scene-last-updated');
        updateTimeElements.forEach(element => {
            const updateTime = this.currentScene.last_updated || this.currentScene.lastUpdated;
            if (updateTime) {
                element.textContent = this.formatTime(updateTime);
                element.title = `最后更新: ${new Date(updateTime).toLocaleString()}`;
            }
        });
    }

    /**
     * 更新场景状态指示器
     */
    updateSceneStatusIndicator() {
        const indicators = document.querySelectorAll('.scene-status-indicator, .status-indicator');
        indicators.forEach(indicator => {
            if (this.currentScene && this.currentScene.status) {
                const status = this.currentScene.status;
                const statusConfig = this.getSceneStatusConfig(status);

                indicator.className = `scene-status-indicator status-${status}`;
                indicator.innerHTML = `
                <span class="status-icon">${statusConfig.icon}</span>
                <span class="status-text">${statusConfig.label}</span>
            `;
                indicator.title = statusConfig.description;
            }
        });
    }

    /**
     * 更新场景头部信息
     */
    updateSceneHeaderInfo() {
        const headerInfo = document.querySelector('.scene-header-info, #scene-header-info');
        if (!headerInfo || !this.currentScene) return;

        const characterCount = this.currentScene.characters ? this.currentScene.characters.length : 0;
        const conversationCount = this.conversations ? this.conversations.length : 0;

        headerInfo.innerHTML = `
        <div class="scene-stats">
            <span class="stat-item">
                <i class="bi bi-people"></i>
                <span class="stat-value">${characterCount}</span>
                <span class="stat-label">角色</span>
            </span>
            <span class="stat-item">
                <i class="bi bi-chat-dots"></i>
                <span class="stat-value">${conversationCount}</span>
                <span class="stat-label">对话</span>
            </span>
            <span class="stat-item">
                <i class="bi bi-clock"></i>
                <span class="stat-value">${this.formatTime(this.currentScene.last_updated)}</span>
                <span class="stat-label">更新</span>
            </span>
        </div>
    `;
    }

    /**
     * 更新场景上下文显示
     */
    updateSceneContextDisplay(context) {
        if (!context) return;

        const contextDisplay = document.querySelector('.scene-context-display, #scene-context-display');
        if (!contextDisplay) return;

        contextDisplay.innerHTML = `
        <div class="context-info">
            ${context.time_setting ? `
                <div class="context-item">
                    <i class="bi bi-clock"></i>
                    <span>${context.time_setting}</span>
                </div>
            ` : ''}
            ${context.location ? `
                <div class="context-item">
                    <i class="bi bi-geo-alt"></i>
                    <span>${context.location}</span>
                </div>
            ` : ''}
            ${context.mood ? `
                <div class="context-item">
                    <i class="bi bi-emoji-neutral"></i>
                    <span>${context.mood}</span>
                </div>
            ` : ''}
        </div>
    `;
    }

    /**
     * 应用场景设置
     */
    applySceneSettings(settings) {
        if (!settings) return;

        // 应用界面设置
        if (settings.dark_mode !== undefined) {
            document.body.classList.toggle('dark-mode', settings.dark_mode);
        }

        // 保存设置到本地存储
        try {
            localStorage.setItem('scene_settings', JSON.stringify(settings));
        } catch (error) {
            console.warn('保存场景设置到本地存储失败:', error);
        }
    }

    /**
     * 触发场景状态事件
     */
    triggerSceneStateEvent(eventType, eventData) {
        // 触发自定义事件
        const event = new CustomEvent(eventType, {
            detail: eventData
        });
        document.dispatchEvent(event);

        // 如果有实时管理器，也通过它触发事件
        if (this.realtimeManager && this.realtimeManager.emit) {
            this.realtimeManager.emit(eventType, eventData);
        }
    }

    /**
     * 获取场景状态配置
     */
    getSceneStatusConfig(status) {
        const configs = {
            'active': {
                icon: '🟢',
                label: '活跃',
                description: '场景正在运行中'
            },
            'paused': {
                icon: '🟡',
                label: '暂停',
                description: '场景已暂停'
            },
            'completed': {
                icon: '✅',
                label: '完成',
                description: '场景已完成'
            },
            'error': {
                icon: '🔴',
                label: '错误',
                description: '场景出现错误'
            },
            'initializing': {
                icon: '🔄',
                label: '初始化',
                description: '场景正在初始化'
            }
        };

        return configs[status] || {
            icon: '⚪',
            label: status || '未知',
            description: '未知状态'
        };
    }

    /**
     * 格式化场景状态
     */
    formatSceneStatus(status) {
        const statusMap = {
            'active': '活跃',
            'paused': '暂停',
            'completed': '完成',
            'error': '错误',
            'initializing': '初始化中'
        };

        return statusMap[status] || status || '未知';
    }

    /**
     * 重新同步场景状态
     */
    async resyncSceneState() {
        const sceneId = this.getSceneIdFromPage();
        if (!sceneId) {
            console.warn('无法获取场景ID，跳过状态同步');
            return;
        }

        try {
            console.log('🔄 重新同步场景状态...');

            // 重新获取场景聚合数据
            const aggregateData = await API.getSceneAggregate(sceneId, {
                includeConversations: true,
                includeStory: true,
                includeUIState: true,
                includeProgress: true
            });

            // 更新本地数据
            this.aggregateData = aggregateData;

            if (aggregateData.data) {
                this.currentScene = aggregateData.data.scene;
            } else {
                this.currentScene = aggregateData.scene;
            }

            // 触发状态更新
            this.updateSceneState(this.currentScene);

            console.log('✅ 场景状态同步完成');
            this.showSuccess('场景状态已同步');

        } catch (error) {
            console.error('❌ 场景状态同步失败:', error);
            this.showError('场景状态同步失败: ' + error.message);
        }
    }

    /**
     * 获取场景状态摘要
     */
    getSceneStateSummary() {
        if (!this.currentScene) {
            return {
                status: 'no_scene',
                message: '没有加载场景'
            };
        }

        const characterCount = this.currentScene.characters ? this.currentScene.characters.length : 0;
        const conversationCount = this.conversations ? this.conversations.length : 0;
        const lastUpdated = this.currentScene.last_updated || this.currentScene.lastUpdated;

        return {
            status: this.currentScene.status || 'unknown',
            scene_id: this.currentScene.id,
            title: this.currentScene.title || this.currentScene.name,
            character_count: characterCount,
            conversation_count: conversationCount,
            last_updated: lastUpdated,
            has_story_data: !!this.storyData,
            dashboard_visible: this.state.dashboardVisible
        };
    }

    /**
     * 监听场景状态变化事件（用于外部监听）
     */
    onSceneStateChange(callback) {
        if (typeof callback !== 'function') {
            console.error('场景状态变化回调必须是函数');
            return;
        }

        // 添加事件监听器
        const eventHandler = (event) => {
            callback(event.detail);
        };

        document.addEventListener('scene_state_updated', eventHandler);

        // 返回取消监听的函数
        return () => {
            document.removeEventListener('scene_state_updated', eventHandler);
        };
    }

    /**
     * 更新实时状态显示
     */
    updateRealtimeStatus(status, message) {
        const statusElement = document.getElementById('realtime-status');
        if (!statusElement) {
            // 创建状态显示元素
            this.createRealtimeStatusElement();
            return this.updateRealtimeStatus(status, message);
        }

        const statusConfig = {
            'connected': { class: 'text-success', icon: 'wifi', text: '已连接' },
            'connecting': { class: 'text-warning', icon: 'hourglass-split', text: '连接中...' },
            'disconnected': { class: 'text-danger', icon: 'wifi-off', text: '已断开' },
            'error': { class: 'text-danger', icon: 'exclamation-triangle', text: '连接错误' }
        };

        const config = statusConfig[status] || statusConfig.disconnected;

        statusElement.innerHTML = `
        <i class="bi bi-${config.icon} ${config.class}"></i>
        <span class="${config.class}">${message || config.text}</span>
    `;

        // 更新全局状态
        this.realtimeState.connected = status === 'connected';
    }

    /**
     * 创建实时状态显示元素
     */
    createRealtimeStatusElement() {
        const statusElement = document.createElement('div');
        statusElement.id = 'realtime-status';
        statusElement.className = 'realtime-status position-fixed';
        statusElement.style.cssText = `
        top: 20px;
        right: 20px;
        background: rgba(255, 255, 255, 0.95);
        padding: 8px 12px;
        border-radius: 20px;
        font-size: 12px;
        box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        z-index: 1000;
        transition: all 0.3s ease;
    `;

        document.body.appendChild(statusElement);
    }

    /**
     * 检查实时连接状态
     */
    checkRealtimeConnection() {
        if (!this.realtimeManager) return;

        const status = this.realtimeManager.getConnectionStatus();
        const sceneId = this.getSceneIdFromPage();
        const connectionId = `scene_${sceneId}`;

        if (status[connectionId]) {
            const connStatus = status[connectionId];
            if (connStatus.readyState !== 1) { // WebSocket.OPEN
                this.updateRealtimeStatus('disconnected', '连接已断开');

                // 尝试重新连接
                setTimeout(() => {
                    this.attemptReconnect(sceneId);
                }, 2000);
            }
        }
    }

    /**
     * 尝试重新连接
     */
    async attemptReconnect(sceneId) {
        if (!this.realtimeManager) return;

        console.log('🔄 尝试重新连接实时通信...');
        this.updateRealtimeStatus('connecting', '正在重新连接...');

        try {
            const success = await this.realtimeManager.initSceneRealtime(sceneId);
            if (success) {
                this.updateRealtimeStatus('connected', '重新连接成功');
                Utils.showSuccess('实时通信已恢复');
            } else {
                this.updateRealtimeStatus('error', '重新连接失败');
            }
        } catch (error) {
            console.error('重新连接失败:', error);
            this.updateRealtimeStatus('error', '重新连接失败');
        }
    }

    /**
     * 更新用户活动状态
     */
    updateUserActivity() {
        if (!this.realtimeManager || !this.realtimeState.connected) return;

        const now = Date.now();
        const timeSinceLastActivity = now - this.realtimeState.lastActivity;

        // 如果超过5分钟没有活动，标记为离开
        let status = 'active';
        if (timeSinceLastActivity > 5 * 60 * 1000) {
            status = 'away';
        } else if (timeSinceLastActivity > 15 * 60 * 1000) {
            status = 'idle';
        }

        // 只在状态变化时发送更新
        if (status !== this.realtimeState.userStatus) {
            this.realtimeState.userStatus = status;
            this.realtimeManager.sendUserStatusUpdate(status, 'activity_update', {
                last_activity: this.realtimeState.lastActivity,
                scene_id: this.getSceneIdFromPage()
            });
        }
    }

    /**
     * 显示新消息通知
     */
    showNewMessageNotification(conversation) {
        const speakerName = conversation.speaker_name || conversation.character_name || '未知角色';
        const content = conversation.content || conversation.message || '';
        const preview = content.length > 50 ? content.substring(0, 50) + '...' : content;

        // 创建通知元素
        const notification = document.createElement('div');
        notification.className = 'new-message-notification toast align-items-center';
        notification.innerHTML = `
        <div class="d-flex">
            <div class="toast-body">
                <strong>${speakerName}</strong><br>
                <small>${preview}</small>
            </div>
            <button type="button" class="btn-close me-2 m-auto" data-bs-dismiss="toast"></button>
        </div>
    `;

        // 添加到页面并显示
        document.body.appendChild(notification);

        if (typeof bootstrap !== 'undefined') {
            const toast = new bootstrap.Toast(notification);
            toast.show();

            // 自动清理
            setTimeout(() => {
                if (notification.parentNode) {
                    notification.remove();
                }
            }, 5000);
        }
    }

    /**
     * 播放通知音效
     */
    playNotificationSound() {
        // 使用实时管理器的音效播放功能
        if (this.realtimeManager && this.realtimeManager.playNotificationSound) {
            this.realtimeManager.playNotificationSound();
        }
    }

    /**
     * 获取选中的角色
     */
    getSelectedCharacter() {
        return this.realtimeState?.selectedCharacter ||
            document.querySelector('.character-item.selected')?.dataset.characterId;
    }

    /**
     * 清理实时通信资源
     */
    cleanupRealtimeConnection() {
        // 清理定时器
        if (this.activityTimer) {
            clearInterval(this.activityTimer);
            this.activityTimer = null;
        }

        if (this.connectionCheckTimer) {
            clearInterval(this.connectionCheckTimer);
            this.connectionCheckTimer = null;
        }

        // 清理实时管理器
        if (this.realtimeManager) {
            this.realtimeManager.cleanup();
            this.realtimeManager = null;
        }

        // 清理状态
        this.realtimeState = null;

        // 移除状态显示
        const statusElement = document.getElementById('realtime-status');
        if (statusElement) {
            statusElement.remove();
        }

        console.log('🧹 实时通信资源已清理');
    }

    /**
     * 处理实时消息 - 兼容原有接口
     */
    handleRealtimeMessage(message) {
        // 这是原有的方法，现在通过事件系统处理
        console.log('📨 收到实时消息 (兼容模式):', message);

        switch (message.type) {
            case 'new_conversation':
                this.handleNewConversation({ conversation: message.data });
                break;
            case 'story_update':
                this.handleStoryEvent({ eventData: message.data });
                break;
            case 'user_joined':
                this.handleUserPresence({
                    action: 'joined',
                    username: message.data.user_name,
                    userId: message.data.user_id
                });
                break;
            case 'user_left':
                this.handleUserPresence({
                    action: 'left',
                    username: message.data.user_name,
                    userId: message.data.user_id
                });
                break;
            default:
                console.log('未处理的消息类型:', message.type);
        }
    }

    /**
     * 处理实时消息 - 预备方法
     */
    handleRealtimeMessage(message) {
        switch (message.type) {
            case 'new_conversation':
                this.addMessageToChat(message.data, true);
                break;
            case 'story_update':
                this.updateStoryDisplay(message.data);
                break;
            case 'user_joined':
                Utils.showInfo(`${message.data.user_name} 加入了场景`);
                break;
        }
    }

    /**
    * 应用清理
    */
    cleanup() {
        // 清理实时通信
        this.cleanupRealtimeConnection();

        // 清理图表
        this.destroyCharts();

        // 清理定时器
        if (this.refreshTimer) {
            clearInterval(this.refreshTimer);
        }

        // 清理统计更新定时器
        if (this.statsUpdateTimer) {
            clearTimeout(this.statsUpdateTimer);
            this.statsUpdateTimer = null;
        }

        // 清理事件监听器
        if (this.dashboardEventHandler) {
            document.removeEventListener('click', this.dashboardEventHandler);
        }

        // 清理发送状态提示
        if (this.currentSendingStatus && this.currentSendingStatus.parentNode) {
            this.currentSendingStatus.remove();
            this.currentSendingStatus = null;
        }

        // 清理滚动按钮
        const scrollBtn = document.getElementById('scroll-to-bottom-btn');
        if (scrollBtn) {
            scrollBtn.remove();
        }

        // 清理提示元素
        document.querySelectorAll('.scroll-to-bottom-prompt, .message-sending-status').forEach(el => {
            el.remove();
        });

        console.log('🧹 应用资源已清理');
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

// 场景状态调试工具
if (typeof window !== 'undefined' &&
    (window.location?.hostname === 'localhost' || window.location?.search.includes('debug=1'))) {

    window.SCENE_STATE_DEBUG = {
        // 获取当前场景状态
        getCurrentState: () => {
            return window.app ? window.app.getSceneStateSummary() : null;
        },

        // 模拟场景状态更新
        simulateStateUpdate: (newState) => {
            if (window.app && window.app.updateSceneState) {
                window.app.updateSceneState(newState);
                return true;
            }
            return false;
        },

        // 重新同步场景状态
        resyncState: async () => {
            if (window.app && window.app.resyncSceneState) {
                await window.app.resyncSceneState();
                return true;
            }
            return false;
        },

        // 测试状态变化
        testStateChanges: () => {
            const testStates = [
                { status: 'active', title: '测试场景 - 活跃' },
                { status: 'paused', title: '测试场景 - 暂停' },
                { status: 'completed', title: '测试场景 - 完成' }
            ];

            testStates.forEach((state, index) => {
                setTimeout(() => {
                    window.SCENE_STATE_DEBUG.simulateStateUpdate(state);
                }, index * 2000);
            });
        },

        // 监听状态变化
        watchStateChanges: () => {
            if (window.app && window.app.onSceneStateChange) {
                return window.app.onSceneStateChange((data) => {
                    console.log('🎭 场景状态变化:', data);
                });
            }
            return null;
        }
    };

    console.log('🎭 场景状态调试工具已加载');
    console.log('使用 window.SCENE_STATE_DEBUG 进行调试');
}

// 在开发环境中添加用户活动调试工具
if (typeof window !== 'undefined' &&
    (window.location?.hostname === 'localhost' || window.location?.search.includes('debug=1'))) {

    window.USER_ACTIVITY_DEBUG = {
        // 获取活动统计
        getActivityStats: () => {
            return window.app ? window.app.getUserActivityStats() : null;
        },

        // 强制更新活动时间
        updateActivity: () => {
            if (window.app && window.app.updateLastActivity) {
                window.app.updateLastActivity();
                return true;
            }
            return false;
        },

        // 模拟用户空闲
        simulateIdle: () => {
            if (window.app && window.app.markUserAsIdle) {
                window.app.markUserAsIdle();
                return true;
            }
            return false;
        },

        // 模拟用户活跃
        simulateActive: () => {
            if (window.app && window.app.markUserAsActive) {
                window.app.markUserAsActive();
                return true;
            }
            return false;
        },

        // 显示活动统计
        showStats: () => {
            const stats = window.USER_ACTIVITY_DEBUG.getActivityStats();
            if (stats) {
                console.table(stats);
                return stats;
            }
            return null;
        },

        // 清理会话数据
        clearSession: () => {
            if (window.app && window.app.clearSessionData) {
                window.app.clearSessionData();
                return true;
            }
            return false;
        },

        // 获取格式化的会话时间
        getSessionDuration: () => {
            const stats = window.USER_ACTIVITY_DEBUG.getActivityStats();
            return stats ? stats.session_duration_text : null;
        },

        // 触发活动事件测试
        testActivityEvents: () => {
            const events = ['mousedown', 'keypress', 'scroll', 'click'];
            events.forEach(eventType => {
                document.dispatchEvent(new Event(eventType, { bubbles: true }));
            });
            console.log('✅ 活动事件测试完成');
        },

        // 监控活动状态变化
        watchActivityChanges: () => {
            const events = ['user_idle', 'user_active'];
            events.forEach(eventType => {
                document.addEventListener(eventType, (e) => {
                    console.log(`🔄 用户状态变化: ${eventType}`, e.detail);
                });
            });
            console.log('👀 开始监控用户活动状态变化');
        }
    };

    console.log('📊 用户活动调试工具已加载');
    console.log('使用 window.USER_ACTIVITY_DEBUG 进行调试');
}

// 在开发环境中添加文本插入调试工具
if (typeof window !== 'undefined' &&
    (window.location?.hostname === 'localhost' || window.location?.search.includes('debug=1'))) {

    window.TEXT_INSERTION_DEBUG = {
        // 测试基本插入
        testBasicInsertion: () => {
            const input = document.getElementById('message-input');
            if (!input) return false;

            return window.app.insertTextAtCursor(input, 'Test text');
        },

        // 测试智能插入
        testSmartInsertion: () => {
            const input = document.getElementById('message-input');
            if (!input) return false;

            return window.app.smartInsertText(input, 'Smart text', {
                addSpace: true,
                triggerEvents: true
            });
        },

        // 测试光标控制
        testCursorControl: () => {
            const input = document.getElementById('message-input');
            if (!input) return false;

            input.value = 'Hello World';
            window.app.setCursorPosition(input, 5);
            return window.app.insertTextAtCursor(input, ' Beautiful');
        },

        // 测试引用功能
        testQuoteFunction: () => {
            const mockConversation = {
                speaker_name: '测试角色',
                message: '这是一条测试消息'
            };

            return window.app.quoteConversationContent(mockConversation);
        },

        // 获取光标信息
        getCursorInfo: () => {
            const input = document.getElementById('message-input');
            if (!input) return null;

            return {
                position: window.app.getCurrentCursorPosition(input),
                elementType: window.app.detectInputElementType(input),
                selectedText: window.app.getSelectedText(input),
                value: input.value
            };
        },

        // 运行所有测试
        runAllTests: () => {
            console.log('🔧 运行所有文本插入测试...');

            const tests = [
                { name: '基本插入', fn: window.TEXT_INSERTION_DEBUG.testBasicInsertion },
                { name: '智能插入', fn: window.TEXT_INSERTION_DEBUG.testSmartInsertion },
                { name: '光标控制', fn: window.TEXT_INSERTION_DEBUG.testCursorControl },
                { name: '引用功能', fn: window.TEXT_INSERTION_DEBUG.testQuoteFunction }
            ];

            const results = tests.map(test => ({
                name: test.name,
                success: test.fn()
            }));

            console.table(results);
            return results;
        },

        // 清理测试
        cleanup: () => {
            const input = document.getElementById('message-input');
            if (input) {
                window.app.clearInput(input);
            }
        }
    };

    console.log('🔧 文本插入调试工具已加载');
    console.log('使用 window.TEXT_INSERTION_DEBUG 进行调试');
}

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
window.addEventListener('beforeunload', () => {
    if (window.app && window.app.cleanup) {
        window.app.cleanup();
    }
});
