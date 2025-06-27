/**
 * 情绪系统管理器
 * 基于后端情绪API重新设计，支持角色情绪显示和交互
 */
class EmotionManager {
    constructor() {
        this.currentEmotions = new Map();
        this.emotionHistory = [];
        this.isLoading = false;
        this.eventListeners = new Map();
        
        // 初始化状态
        this.state = {
            initialized: false,
            hasEmotionData: false,
            activeCharacters: new Set()
        };

        // 检查依赖并初始化
        this.initialize();
    }

    // ========================================
    // 核心初始化功能
    // ========================================

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
            const message = `EmotionManager缺少必要的依赖: ${missing.join(', ')}`;
            console.error(message);
            
            if (typeof Utils !== 'undefined') {
                Utils.showError(message);
            } else {
                alert(`情绪系统初始化失败: ${message}\n请确保正确加载了所有脚本文件`);
            }
            
            throw new Error(message);
        }
        
        console.log('✅ EmotionManager 依赖检查通过');
        return true;
    }

    /**
     * 初始化情绪管理器
     */
    async initialize() {
        try {
            // 检查依赖
            this.checkDependencies();
            
            // 等待依赖完全可用
            await this.waitForDependencies();
            
            // 初始化事件监听器
            this.initializeEventListeners();
            
            // 标记为已初始化
            this.state.initialized = true;
            
            console.log('✅ EmotionManager 初始化完成');
        } catch (error) {
            console.error('❌ EmotionManager 初始化失败:', error);
            this.showError('情绪系统初始化失败: ' + error.message);
        }
    }

    /**
     * 等待依赖加载完成
     */
    async waitForDependencies() {
        if (typeof Utils !== 'undefined' && typeof Utils.waitForDependencies === 'function') {
            return Utils.waitForDependencies(['API', 'Utils'], {
                timeout: 10000,
                context: 'EmotionManager'
            });
        }

        // 降级方法
        const timeout = 10000;
        const checkInterval = 100;
        const startTime = Date.now();

        return new Promise((resolve, reject) => {
            const checkLoop = () => {
                if (typeof API !== 'undefined' && typeof Utils !== 'undefined') {
                    console.log('✅ EmotionManager 依赖等待完成');
                    resolve();
                    return;
                }

                if (Date.now() - startTime > timeout) {
                    reject(new Error('EmotionManager 依赖等待超时'));
                    return;
                }

                setTimeout(checkLoop, checkInterval);
            };
            checkLoop();
        });
    }

    /**
     * 初始化事件监听器
     */
    initializeEventListeners() {
        // 监听聊天消息，提取情绪数据
        this.addEventDelegate('chatMessageReceived', (e) => {
            this.handleChatEmotion(e.detail);
        });

        // 监听角色互动事件
        this.addEventDelegate('characterInteraction', (e) => {
            this.handleCharacterInteraction(e.detail);
        });

        // 情绪卡片点击事件
        this.addEventDelegate('click', '.emotion-toggle-btn', (e, target) => {
            this.toggleEmotionDisplay();
        });

        // 情绪详情点击事件
        this.addEventDelegate('click', '.emotion-detail-btn', (e, target) => {
            const characterId = target.dataset.characterId;
            this.showEmotionDetail(characterId);
        });

        // 情绪历史点击事件
        this.addEventDelegate('click', '.emotion-history-btn', (e, target) => {
            const characterId = target.dataset.characterId;
            this.showEmotionHistory(characterId);
        });

        // 刷新情绪数据
        this.addEventDelegate('click', '.emotion-refresh-btn', (e, target) => {
            const sceneId = target.dataset.sceneId;
            this.refreshEmotionData(sceneId);
        });
    }

    /**
     * 添加事件委托
     */
    addEventDelegate(eventType, selector, handler) {
        let wrappedHandler;
        
        if (typeof selector === 'function') {
            // 如果selector是函数，则是自定义事件
            wrappedHandler = selector;
            document.addEventListener(eventType, wrappedHandler);
        } else {
            // DOM事件委托
            wrappedHandler = (e) => {
                const target = e.target.closest(selector);
                if (target) {
                    handler(e, target);
                }
            };
            document.addEventListener(eventType, wrappedHandler);
        }

        this.eventListeners.set(`${eventType}-${selector}`, wrappedHandler);
    }

    // ========================================
    // 情绪数据处理功能
    // ========================================

    /**
     * 处理聊天消息中的情绪数据
     */
    handleChatEmotion(messageData) {
        if (!messageData.emotion_data) return;

        const emotionEntry = {
            id: `emotion_${Date.now()}`,
            timestamp: new Date(),
            character_id: messageData.character_id,
            character_name: messageData.character_name,
            message: messageData.message,
            emotion_data: messageData.emotion_data
        };

        // 更新当前情绪状态
        this.currentEmotions.set(messageData.character_id, emotionEntry);
        
        // 添加到历史记录
        this.emotionHistory.push(emotionEntry);
        
        // 限制历史记录长度
        if (this.emotionHistory.length > 100) {
            this.emotionHistory = this.emotionHistory.slice(-50);
        }

        // 更新活跃角色列表
        this.state.activeCharacters.add(messageData.character_id);
        this.state.hasEmotionData = true;

        // 更新显示
        this.updateEmotionDisplay(messageData.character_id);
        
        console.log('📝 情绪数据已更新:', emotionEntry);
    }

    /**
     * 处理角色互动事件
     */
    handleCharacterInteraction(interactionData) {
        if (interactionData.emotional_response) {
            const emotionEntry = {
                id: `interaction_${Date.now()}`,
                timestamp: new Date(),
                character_id: interactionData.character_id,
                character_name: interactionData.character_name,
                interaction_type: 'character_interaction',
                emotion_data: interactionData.emotional_response
            };

            this.currentEmotions.set(interactionData.character_id, emotionEntry);
            this.emotionHistory.push(emotionEntry);
            
            this.updateEmotionDisplay(interactionData.character_id);
        }
    }

    /**
     * 从后端加载角色情绪数据
     */
    async loadCharacterEmotions(sceneId, characterId = null) {
        try {
            this.setLoading(true);
            
            console.log(`🔄 加载情绪数据: 场景=${sceneId}, 角色=${characterId || '全部'}`);
            
            // 调用后端API获取角色互动历史
            const interactions = await this.safeAPICall(() => 
                API.getCharacterInteractions(sceneId, { character_id: characterId, limit: 20 })
            );

            // 处理互动数据中的情绪信息
            for (const interaction of interactions) {
                if (interaction.emotional_response) {
                    const emotionEntry = {
                        id: interaction.id,
                        timestamp: new Date(interaction.timestamp),
                        character_id: interaction.character_id,
                        character_name: interaction.character_name,
                        message: interaction.content,
                        emotion_data: interaction.emotional_response
                    };

                    this.currentEmotions.set(interaction.character_id, emotionEntry);
                    
                    // 检查是否已存在，避免重复
                    if (!this.emotionHistory.find(e => e.id === emotionEntry.id)) {
                        this.emotionHistory.push(emotionEntry);
                    }
                    
                    this.state.activeCharacters.add(interaction.character_id);
                }
            }

            // 按时间排序
            this.emotionHistory.sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp));
            
            this.state.hasEmotionData = this.currentEmotions.size > 0;
            
            // 更新显示
            this.renderEmotionInterface();
            
            console.log('✅ 情绪数据加载完成');
            
        } catch (error) {
            console.error('❌ 加载情绪数据失败:', error);
            this.showError('加载情绪数据失败: ' + error.message);
        } finally {
            this.setLoading(false);
        }
    }

    /**
     * 刷新情绪数据
     */
    async refreshEmotionData(sceneId) {
        await this.loadCharacterEmotions(sceneId);
    }

    // ========================================
    // 界面渲染功能
    // ========================================

    /**
     * 渲染情绪界面
     */
    renderEmotionInterface() {
        if (!this.state.hasEmotionData) {
            this.renderEmptyState();
            return;
        }

        this.renderEmotionContainer();
        this.renderCharacterEmotions();
        this.renderEmotionSummary();
    }

    /**
     * 渲染空状态
     */
    renderEmptyState() {
        const container = this.getEmotionContainer();
        container.innerHTML = `
            <div class="emotion-empty-state text-center py-4">
                <i class="bi bi-emoji-neutral display-1 text-muted"></i>
                <h4 class="mt-3">暂无情绪数据</h4>
                <p class="text-muted">与角色互动后将显示情绪状态</p>
                <button class="btn btn-outline-primary emotion-refresh-btn" data-scene-id="${this.getCurrentSceneId()}">
                    <i class="bi bi-arrow-clockwise"></i> 刷新数据
                </button>
            </div>
        `;
    }

    /**
     * 渲染情绪容器
     */
    renderEmotionContainer() {
        const container = this.getEmotionContainer();
        container.innerHTML = `
            <div class="emotion-header d-flex justify-content-between align-items-center mb-3">
                <h4 class="mb-0">
                    <i class="bi bi-emoji-smile"></i>
                    角色情绪状态
                </h4>
                <div class="emotion-actions">
                    <button class="btn btn-sm btn-outline-secondary emotion-refresh-btn" 
                            data-scene-id="${this.getCurrentSceneId()}" title="刷新">
                        <i class="bi bi-arrow-clockwise"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-info emotion-toggle-btn" title="切换显示">
                        <i class="bi bi-layout-sidebar"></i>
                    </button>
                </div>
            </div>
            <div class="emotion-content">
                <div class="character-emotions-grid" id="character-emotions-grid"></div>
                <div class="emotion-summary mt-3" id="emotion-summary"></div>
            </div>
        `;
    }

    /**
     * 渲染角色情绪
     */
    renderCharacterEmotions() {
        const grid = document.getElementById('character-emotions-grid');
        if (!grid) return;

        const emotionCards = Array.from(this.currentEmotions.values()).map(emotion => 
            this.renderEmotionCard(emotion)
        ).join('');

        grid.innerHTML = emotionCards;
    }

    /**
     * 渲染单个情绪卡片
     */
    renderEmotionCard(emotionData) {
        const emotion = emotionData.emotion_data;
        const primaryEmotion = emotion.emotion || '平静';
        const intensity = emotion.intensity || 5;
        const moodChange = emotion.mood_change || '稳定';

        return `
            <div class="emotion-card card mb-3" data-character-id="${emotionData.character_id}">
                <div class="card-header">
                    <div class="d-flex justify-content-between align-items-center">
                        <h5 class="mb-0">${this.escapeHtml(emotionData.character_name)}</h5>
                        <small class="text-muted">${this.formatTime(emotionData.timestamp)}</small>
                    </div>
                </div>
                <div class="card-body">
                    <div class="emotion-main d-flex align-items-center mb-3">
                        <div class="emotion-icon me-3">
                            ${this.getEmotionIcon(primaryEmotion)}
                        </div>
                        <div class="emotion-details">
                            <div class="emotion-name h5 mb-1">${this.escapeHtml(primaryEmotion)}</div>
                            <div class="emotion-intensity">
                                <div class="progress" style="height: 6px;">
                                    <div class="progress-bar" 
                                         style="width: ${intensity * 10}%; background-color: ${this.getIntensityColor(intensity)}"
                                         role="progressbar"></div>
                                </div>
                                <small class="text-muted">强度: ${intensity}/10</small>
                            </div>
                        </div>
                    </div>
                    
                    ${emotion.expression ? `
                        <div class="emotion-expression mb-2">
                            <strong>表情:</strong> ${this.escapeHtml(emotion.expression)}
                        </div>
                    ` : ''}
                    
                    ${emotion.voice_tone ? `
                        <div class="emotion-voice mb-2">
                            <strong>语调:</strong> ${this.escapeHtml(emotion.voice_tone)}
                        </div>
                    ` : ''}
                    
                    <div class="emotion-mood mb-3">
                        <strong>心情变化:</strong> 
                        <span class="badge bg-${this.getMoodChangeColor(moodChange)}">${this.escapeHtml(moodChange)}</span>
                    </div>
                    
                    ${emotionData.message ? `
                        <div class="emotion-context">
                            <small class="text-muted">
                                "${this.escapeHtml(emotionData.message.substring(0, 100))}${emotionData.message.length > 100 ? '...' : ''}"
                            </small>
                        </div>
                    ` : ''}
                </div>
                <div class="card-footer">
                    <div class="btn-group btn-group-sm" role="group">
                        <button class="btn btn-outline-primary emotion-detail-btn" 
                                data-character-id="${emotionData.character_id}">
                            <i class="bi bi-info-circle"></i> 详情
                        </button>
                        <button class="btn btn-outline-secondary emotion-history-btn" 
                                data-character-id="${emotionData.character_id}">
                            <i class="bi bi-clock-history"></i> 历史
                        </button>
                    </div>
                </div>
            </div>
        `;
    }

    /**
     * 渲染情绪摘要
     */
    renderEmotionSummary() {
        const summary = document.getElementById('emotion-summary');
        if (!summary) return;

        const stats = this.calculateEmotionStats();
        
        summary.innerHTML = `
            <div class="emotion-summary-card card">
                <div class="card-header">
                    <h5 class="mb-0">情绪概览</h5>
                </div>
                <div class="card-body">
                    <div class="row g-3">
                        <div class="col-6 col-md-3">
                            <div class="stat-item text-center">
                                <div class="stat-value h4">${stats.activeCharacters}</div>
                                <div class="stat-label text-muted">活跃角色</div>
                            </div>
                        </div>
                        <div class="col-6 col-md-3">
                            <div class="stat-item text-center">
                                <div class="stat-value h4">${stats.avgIntensity.toFixed(1)}</div>
                                <div class="stat-label text-muted">平均强度</div>
                            </div>
                        </div>
                        <div class="col-6 col-md-3">
                            <div class="stat-item text-center">
                                <div class="stat-value h4">${stats.dominantEmotion}</div>
                                <div class="stat-label text-muted">主要情绪</div>
                            </div>
                        </div>
                        <div class="col-6 col-md-3">
                            <div class="stat-item text-center">
                                <div class="stat-value h4">${stats.totalInteractions}</div>
                                <div class="stat-label text-muted">互动次数</div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    // ========================================
    // 详情和历史显示
    // ========================================

    /**
     * 显示情绪详情
     */
    async showEmotionDetail(characterId) {
        const emotionData = this.currentEmotions.get(characterId);
        if (!emotionData) {
            this.showError('未找到该角色的情绪数据');
            return;
        }

        try {
            // 创建模态框
            const modal = this.createModal('emotion-detail-modal', '情绪详细分析');
            
            // 渲染详情内容
            const content = this.renderEmotionDetailContent(emotionData);
            modal.querySelector('.modal-body').innerHTML = content;
            
            // 显示模态框
            this.showModal(modal);
            
        } catch (error) {
            console.error('❌ 显示情绪详情失败:', error);
            this.showError('显示情绪详情失败');
        }
    }

    /**
     * 显示情绪历史
     */
    async showEmotionHistory(characterId) {
        try {
            // 筛选该角色的历史数据
            const characterHistory = this.emotionHistory.filter(e => e.character_id === characterId);
            
            if (characterHistory.length === 0) {
                this.showError('该角色暂无情绪历史记录');
                return;
            }

            // 创建模态框
            const modal = this.createModal('emotion-history-modal', '情绪变化历史');
            
            // 渲染历史内容
            const content = this.renderEmotionHistoryContent(characterHistory);
            modal.querySelector('.modal-body').innerHTML = content;
            
            // 显示模态框
            this.showModal(modal);
            
        } catch (error) {
            console.error('❌ 显示情绪历史失败:', error);
            this.showError('显示情绪历史失败');
        }
    }

    /**
     * 渲染情绪详情内容
     */
    renderEmotionDetailContent(emotionData) {
        const emotion = emotionData.emotion_data;
        
        return `
            <div class="emotion-detail-content">
                <div class="character-info mb-4">
                    <h4>${this.escapeHtml(emotionData.character_name)}</h4>
                    <div class="timestamp text-muted">
                        <i class="bi bi-clock"></i> ${this.formatTime(emotionData.timestamp, 'YYYY-MM-DD HH:mm:ss')}
                    </div>
                </div>
                
                <div class="emotion-analysis">
                    <h5>情绪分析</h5>
                    <div class="emotion-dimensions">
                        ${this.renderEmotionDimensions(emotion)}
                    </div>
                </div>
                
                ${emotionData.message ? `
                    <div class="context-info mt-3">
                        <h5>对话内容</h5>
                        <div class="context-message p-3 bg-light rounded">
                            "${this.escapeHtml(emotionData.message)}"
                        </div>
                    </div>
                ` : ''}
            </div>
        `;
    }

    /**
     * 渲染情绪历史内容
     */
    renderEmotionHistoryContent(historyData) {
        const recentHistory = historyData.slice(-20); // 显示最近20条
        
        return `
            <div class="emotion-history-content">
                <div class="history-stats mb-3">
                    <div class="row g-2">
                        <div class="col-4 text-center">
                            <div class="stat-value h5">${historyData.length}</div>
                            <div class="stat-label text-muted">总记录数</div>
                        </div>
                        <div class="col-4 text-center">
                            <div class="stat-value h5">${this.calculateAvgIntensity(historyData).toFixed(1)}</div>
                            <div class="stat-label text-muted">平均强度</div>
                        </div>
                        <div class="col-4 text-center">
                            <div class="stat-value h5">${this.getMostFrequentEmotion(historyData)}</div>
                            <div class="stat-label text-muted">常见情绪</div>
                        </div>
                    </div>
                </div>
                
                <div class="history-timeline">
                    ${recentHistory.map(entry => `
                        <div class="timeline-item mb-3">
                            <div class="d-flex">
                                <div class="timeline-marker me-3">
                                    ${this.getEmotionIcon(entry.emotion_data.emotion)}
                                </div>
                                <div class="timeline-content flex-grow-1">
                                    <div class="d-flex justify-content-between align-items-start">
                                        <div class="emotion-info">
                                            <strong>${this.escapeHtml(entry.emotion_data.emotion || '未知')}</strong>
                                            <span class="ms-2 badge bg-secondary">强度: ${entry.emotion_data.intensity || 5}</span>
                                        </div>
                                        <small class="text-muted">${this.formatTime(entry.timestamp, 'MM-DD HH:mm')}</small>
                                    </div>
                                    ${entry.message ? `
                                        <div class="message-preview text-muted small mt-1">
                                            "${this.escapeHtml(entry.message.substring(0, 80))}${entry.message.length > 80 ? '...' : ''}"
                                        </div>
                                    ` : ''}
                                </div>
                            </div>
                        </div>
                    `).join('')}
                </div>
                
                ${historyData.length > 20 ? `
                    <div class="text-center mt-3">
                        <small class="text-muted">显示最近20条记录，共${historyData.length}条</small>
                    </div>
                ` : ''}
            </div>
        `;
    }

    /**
     * 渲染情绪维度
     */
    renderEmotionDimensions(emotionData) {
        const dimensions = [
            { key: 'emotion', label: '主要情绪', icon: '😊' },
            { key: 'intensity', label: '强度', icon: '⚡' },
            { key: 'expression', label: '表情', icon: '😮' },
            { key: 'voice_tone', label: '语调', icon: '🎵' },
            { key: 'body_language', label: '肢体语言', icon: '🤲' },
            { key: 'mood_change', label: '心情变化', icon: '📈' }
        ];

        return dimensions.map(dim => {
            const value = emotionData[dim.key];
            if (!value && dim.key !== 'intensity') return '';
            
            return `
                <div class="dimension-item d-flex align-items-center mb-2">
                    <div class="dimension-icon me-2">${dim.icon}</div>
                    <div class="dimension-content">
                        <div class="dimension-label fw-bold">${dim.label}</div>
                        <div class="dimension-value">
                            ${dim.key === 'intensity' ? 
                                `${value || 5}/10 <div class="progress mt-1" style="height: 4px;"><div class="progress-bar" style="width: ${(value || 5) * 10}%"></div></div>` :
                                this.escapeHtml(value)
                            }
                        </div>
                    </div>
                </div>
            `;
        }).filter(Boolean).join('');
    }

    // ========================================
    // 辅助计算功能
    // ========================================

    /**
     * 计算情绪统计
     */
    calculateEmotionStats() {
        const emotions = Array.from(this.currentEmotions.values());
        
        let totalIntensity = 0;
        const emotionCounts = {};
        
        emotions.forEach(e => {
            const intensity = e.emotion_data.intensity || 5;
            totalIntensity += intensity;
            
            const emotion = e.emotion_data.emotion || '未知';
            emotionCounts[emotion] = (emotionCounts[emotion] || 0) + 1;
        });

        const avgIntensity = emotions.length > 0 ? totalIntensity / emotions.length : 0;
        const dominantEmotion = Object.keys(emotionCounts).reduce((a, b) => 
            emotionCounts[a] > emotionCounts[b] ? a : b, '未知');

        return {
            activeCharacters: this.state.activeCharacters.size,
            avgIntensity,
            dominantEmotion,
            totalInteractions: this.emotionHistory.length
        };
    }

    /**
     * 计算平均强度
     */
    calculateAvgIntensity(historyData) {
        if (historyData.length === 0) return 0;
        
        const totalIntensity = historyData.reduce((sum, entry) => {
            return sum + (entry.emotion_data.intensity || 5);
        }, 0);
        
        return totalIntensity / historyData.length;
    }

    /**
     * 获取最常见情绪
     */
    getMostFrequentEmotion(historyData) {
        const emotionCounts = {};
        
        historyData.forEach(entry => {
            const emotion = entry.emotion_data.emotion || '未知';
            emotionCounts[emotion] = (emotionCounts[emotion] || 0) + 1;
        });

        return Object.keys(emotionCounts).reduce((a, b) => 
            emotionCounts[a] > emotionCounts[b] ? a : b, '未知');
    }

    // ========================================
    // 工具方法
    // ========================================

    /**
     * 获取情绪图标
     */
    getEmotionIcon(emotion) {
        const iconMap = {
            '愤怒': '😠', '悲伤': '😢', '快乐': '😊', '恐惧': '😨',
            '惊讶': '😮', '厌恶': '🤢', '平静': '😌', '兴奋': '🤩',
            '好奇': '🤔', '满足': '😌', '焦虑': '😰', '自豪': '😎',
            '困惑': '😕', '失望': '😞', '欣慰': '😊', '紧张': '😬'
        };
        return iconMap[emotion] || '😐';
    }

    /**
     * 获取强度颜色
     */
    getIntensityColor(intensity) {
        if (intensity <= 3) return '#28a745'; // 绿色 - 低强度
        if (intensity <= 6) return '#ffc107'; // 黄色 - 中强度
        if (intensity <= 8) return '#fd7e14'; // 橙色 - 高强度
        return '#dc3545'; // 红色 - 极高强度
    }

    /**
     * 获取心情变化颜色
     */
    getMoodChangeColor(moodChange) {
        const colorMap = {
            '上升': 'success',
            '下降': 'danger', 
            '稳定': 'secondary',
            '波动': 'warning',
            '改善': 'info',
            '恶化': 'danger'
        };
        return colorMap[moodChange] || 'secondary';
    }

    /**
     * 获取情绪容器
     */
    getEmotionContainer() {
        let container = document.getElementById('emotion-container');
        if (!container) {
            container = document.createElement('div');
            container.id = 'emotion-container';
            container.className = 'emotion-container';
            
            // 找到合适的位置插入
            const mainContent = document.querySelector('.scene-content') || 
                               document.querySelector('.main-content') || 
                               document.body;
            mainContent.appendChild(container);
        }
        return container;
    }

    /**
     * 获取当前场景ID
     */
    getCurrentSceneId() {
        return window.currentSceneId || document.body.dataset.sceneId || '';
    }

    /**
     * 切换情绪显示
     */
    toggleEmotionDisplay() {
        const container = this.getEmotionContainer();
        if (container.style.display === 'none') {
            container.style.display = 'block';
        } else {
            container.style.display = 'none';
        }
    }

    /**
     * 更新单个角色的情绪显示
     */
    updateEmotionDisplay(characterId) {
        // 如果界面已渲染，只更新特定角色的卡片
        const existingCard = document.querySelector(`[data-character-id="${characterId}"]`);
        if (existingCard) {
            const emotionData = this.currentEmotions.get(characterId);
            if (emotionData) {
                const cardContainer = existingCard.closest('.emotion-card');
                if (cardContainer) {
                    cardContainer.outerHTML = this.renderEmotionCard(emotionData);
                }
            }
        } else {
            // 重新渲染整个界面
            this.renderEmotionInterface();
        }
    }

    /**
     * 设置加载状态
     */
    setLoading(isLoading) {
        this.isLoading = isLoading;
        
        const loadingIndicator = document.getElementById('emotion-loading');
        if (loadingIndicator) {
            loadingIndicator.style.display = isLoading ? 'block' : 'none';
        }
    }

    // ========================================
    // 安全调用方法
    // ========================================

    /**
     * 安全调用API方法
     */
    async safeAPICall(apiCall) {
        if (typeof API === 'undefined') {
            throw new Error('API不可用');
        }
        return await apiCall();
    }

    /**
     * 创建模态框
     */
    createModal(id, title) {
        // 移除已存在的模态框
        const existing = document.getElementById(id);
        if (existing) {
            existing.remove();
        }

        const modal = document.createElement('div');
        modal.id = id;
        modal.className = 'modal fade';
        modal.innerHTML = `
            <div class="modal-dialog modal-lg">
                <div class="modal-content">
                    <div class="modal-header">
                        <h5 class="modal-title">${this.escapeHtml(title)}</h5>
                        <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                    </div>
                    <div class="modal-body"></div>
                    <div class="modal-footer">
                        <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
                    </div>
                </div>
            </div>
        `;
        
        document.body.appendChild(modal);
        return modal;
    }

    /**
     * 显示模态框
     */
    showModal(modal) {
        if (typeof bootstrap !== 'undefined' && bootstrap.Modal) {
            const bsModal = new bootstrap.Modal(modal);
            bsModal.show();
        } else {
            // 降级处理
            modal.style.display = 'block';
            modal.classList.add('show');
        }
    }

    /**
     * HTML转义
     */
    escapeHtml(text) {
        if (typeof text !== 'string') return '';
        
        if (typeof Utils !== 'undefined' && typeof Utils.escapeHtml === 'function') {
            return Utils.escapeHtml(text);
        }
        
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /**
     * 格式化时间
     */
    formatTime(timestamp, format = 'HH:mm:ss') {
        if (typeof Utils !== 'undefined' && typeof Utils.formatTime === 'function') {
            return Utils.formatTime(timestamp, format);
        }
        
        const date = new Date(timestamp);
        
        if (format === 'YYYY-MM-DD HH:mm:ss') {
            return date.toLocaleString('zh-CN', {
                year: 'numeric',
                month: '2-digit',
                day: '2-digit',
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit',
                hour12: false
            });
        }
        
        if (format === 'MM-DD HH:mm') {
            return date.toLocaleString('zh-CN', {
                month: '2-digit',
                day: '2-digit',
                hour: '2-digit',
                minute: '2-digit',
                hour12: false
            });
        }

        return date.toLocaleTimeString('zh-CN', { hour12: false });
    }

    /**
     * 显示成功消息
     */
    showSuccess(message) {
        if (typeof Utils !== 'undefined' && typeof Utils.showSuccess === 'function') {
            Utils.showSuccess(message);
        } else {
            console.log('Success:', message);
        }
    }

    /**
     * 显示错误消息
     */
    showError(message) {
        if (typeof Utils !== 'undefined' && typeof Utils.showError === 'function') {
            Utils.showError(message);
        } else {
            console.error('Error:', message);
            alert('错误: ' + message);
        }
    }

    // ========================================
    // 公共接口方法
    // ========================================

    /**
     * 初始化场景情绪系统
     */
    async init(sceneId) {
        if (!sceneId) {
            console.warn('⚠️ 场景ID为空，跳过情绪数据加载');
            return;
        }

        try {
            console.log(`🎭 初始化场景 ${sceneId} 的情绪系统...`);
            
            await this.loadCharacterEmotions(sceneId);
            
            console.log('✅ 情绪系统初始化成功');
        } catch (error) {
            console.error('❌ 情绪系统初始化失败:', error);
            throw error;
        }
    }

    /**
     * 获取当前情绪状态
     */
    getCurrentEmotions() {
        return Object.fromEntries(this.currentEmotions);
    }

    /**
     * 获取情绪历史
     */
    getEmotionHistory() {
        return [...this.emotionHistory];
    }

    /**
     * 清空情绪数据
     */
    clearEmotionData() {
        this.currentEmotions.clear();
        this.emotionHistory = [];
        this.state.activeCharacters.clear();
        this.state.hasEmotionData = false;
        
        this.renderEmotionInterface();
    }

    /**
     * 检查是否已初始化
     */
    isInitialized() {
        return this.state.initialized;
    }

    /**
     * 销毁情绪管理器
     */
    destroy() {
        // 移除事件监听器
        this.eventListeners.forEach((handler, key) => {
            const [eventType] = key.split('-');
            document.removeEventListener(eventType, handler);
        });
        this.eventListeners.clear();

        // 清理数据
        this.clearEmotionData();
        
        console.log('🗑️ EmotionManager 已销毁');
    }
}

// ========================================
// 全局初始化
// ========================================

// 确保在DOM加载完成后创建全局实例
if (typeof window !== 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            window.EmotionManager = EmotionManager;
            window.emotionManager = new EmotionManager();
            console.log('🎭 EmotionManager 已准备就绪');
        });
    } else {
        window.EmotionManager = EmotionManager;
        window.emotionManager = new EmotionManager();
        console.log('🎭 EmotionManager 已准备就绪');
    }
}

// 添加CSS样式
if (typeof document !== 'undefined') {
    const addEmotionStyles = () => {
        if (document.getElementById('emotion-manager-styles')) return;

        const style = document.createElement('style');
        style.id = 'emotion-manager-styles';
        style.textContent = `
            /* 情绪容器样式 */
            .emotion-container {
                margin: 20px 0;
                padding: 20px;
                background: #f8f9fa;
                border-radius: 8px;
                border: 1px solid #dee2e6;
            }
            
            .emotion-empty-state {
                padding: 40px 20px;
            }
            
            /* 情绪卡片样式 */
            .emotion-card {
                transition: all 0.3s ease;
                border: 1px solid #dee2e6;
            }
            
            .emotion-card:hover {
                transform: translateY(-2px);
                box-shadow: 0 4px 12px rgba(0,0,0,0.1);
            }
            
            .emotion-main {
                align-items: center;
            }
            
            .emotion-icon {
                font-size: 2rem;
                line-height: 1;
            }
            
            .emotion-name {
                font-weight: 600;
                color: #495057;
            }
            
            .emotion-intensity .progress {
                width: 100px;
                margin-top: 4px;
            }
            
            /* 情绪历史时间线 */
            .timeline-item {
                position: relative;
                padding-left: 20px;
            }
            
            .timeline-marker {
                font-size: 1.2rem;
                line-height: 1;
            }
            
            .message-preview {
                font-style: italic;
                line-height: 1.3;
            }
            
            /* 情绪维度显示 */
            .dimension-item {
                background: rgba(0,0,0,0.02);
                padding: 8px;
                border-radius: 4px;
                margin-bottom: 8px;
            }
            
            .dimension-icon {
                font-size: 1.1rem;
                width: 24px;
                text-align: center;
            }
            
            .dimension-label {
                font-size: 0.9rem;
                font-weight: 600;
            }
            
            .dimension-value {
                font-size: 0.85rem;
                color: #6c757d;
            }
            
            /* 统计卡片 */
            .stat-item {
                padding: 12px;
                background: white;
                border-radius: 6px;
                border: 1px solid #e9ecef;
            }
            
            .stat-value {
                font-weight: 700;
                color: #495057;
                margin-bottom: 4px;
            }
            
            .stat-label {
                font-size: 0.8rem;
                font-weight: 500;
            }
            
            /* 响应式设计 */
            @media (max-width: 768px) {
                .emotion-container {
                    margin: 10px 0;
                    padding: 15px;
                }
                
                .emotion-main {
                    flex-direction: column;
                    text-align: center;
                }
                
                .emotion-icon {
                    margin-bottom: 10px;
                }
                
                .timeline-item {
                    padding-left: 0;
                    margin-bottom: 20px;
                }
            }
        `;
        
        document.head.appendChild(style);
        console.log('✅ EmotionManager 样式已加载');
    };
    
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', addEmotionStyles);
    } else {
        addEmotionStyles();
    }
}
