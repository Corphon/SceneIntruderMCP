/**
 * 故事系统管理器
 * 基于后端StoryService API重新设计
 * 支持故事节点、分支选择、时间线管理
 */
class StoryManager {
    constructor() {
        this.currentStoryData = null;
        this.sceneId = null;
        this.isLoading = false;
        this.eventListeners = new Map();
        
        // 初始化状态
        this.state = {
            initialized: false,
            hasStoryData: false,
            currentNodeIndex: 0,
            selectedChoices: [],
            storyProgress: 0
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
            const message = `StoryManager缺少必要的依赖: ${missing.join(', ')}`;
            console.error(message);
            
            if (typeof Utils !== 'undefined') {
                Utils.showError(message);
            } else {
                alert(`故事系统初始化失败: ${message}\n请确保正确加载了所有脚本文件`);
            }
            
            throw new Error(message);
        }
        
        console.log('✅ StoryManager 依赖检查通过');
        return true;
    }

    /**
     * 初始化故事管理器
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
            
            console.log('✅ StoryManager 初始化完成');
        } catch (error) {
            console.error('❌ StoryManager 初始化失败:', error);
            this.showError('故事系统初始化失败: ' + error.message);
        }
    }

    /**
     * 等待依赖加载完成
     */
    async waitForDependencies() {
        if (typeof Utils !== 'undefined' && typeof Utils.waitForDependencies === 'function') {
            return Utils.waitForDependencies(['API', 'Utils'], {
                timeout: 10000,
                context: 'StoryManager'
            });
        }

        // 降级方法
        const timeout = 10000;
        const checkInterval = 100;
        const startTime = Date.now();

        return new Promise((resolve, reject) => {
            const checkLoop = () => {
                if (typeof API !== 'undefined' && typeof Utils !== 'undefined') {
                    console.log('✅ StoryManager 依赖等待完成');
                    resolve();
                    return;
                }

                if (Date.now() - startTime > timeout) {
                    reject(new Error('StoryManager 依赖等待超时'));
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
        // 故事选择点击事件
        this.addEventDelegate('click', '.story-choice-btn', (e, target) => {
            const choiceId = target.dataset.choiceId;
            const nodeId = target.dataset.nodeId;
            this.makeChoice(nodeId, choiceId);
        });

        // 时间线节点点击事件
        this.addEventDelegate('click', '.timeline-node', (e, target) => {
            const nodeIndex = parseInt(target.dataset.nodeIndex);
            if (e.ctrlKey) {
                // Ctrl+点击回溯到该节点
                this.revertToNode(nodeIndex);
            } else {
                // 普通点击跳转到该节点
                this.jumpToNode(nodeIndex);
            }
        });

        // 故事导出事件
        this.addEventDelegate('click', '.story-export-btn', (e, target) => {
            const format = target.dataset.format || 'html';
            this.exportStory(format);
        });

        // 故事刷新事件
        this.addEventDelegate('click', '.story-refresh-btn', (e, target) => {
            this.refreshStory();
        });

        // 故事重置事件
        this.addEventDelegate('click', '.story-reset-btn', (e, target) => {
            this.resetStory();
        });
    }

    /**
     * 添加事件委托
     */
    addEventDelegate(eventType, selector, handler) {
        const wrappedHandler = (e) => {
            const target = e.target.closest(selector);
            if (target) {
                handler(e, target);
            }
        };
        
        document.addEventListener(eventType, wrappedHandler);
        this.eventListeners.set(`${eventType}-${selector}`, wrappedHandler);
    }

    // ========================================
    // 故事数据加载
    // ========================================

    /**
     * 加载故事数据
     */
    async loadStory(sceneId) {
        try {
            this.setLoading(true);
            this.sceneId = sceneId;
            
            console.log(`📖 加载场景故事: ${sceneId}`);
            
            // 调用后端API获取故事数据
            const storyData = await this.safeAPICall(() => 
                API.getStoryData(sceneId)
            );

            this.currentStoryData = storyData;
            this.state.hasStoryData = true;
            
            // 计算当前进度
            this.updateStoryProgress();
            
            // 渲染故事界面
            this.renderStoryInterface();
            
            console.log('✅ 故事数据加载完成');
            
        } catch (error) {
            console.error('❌ 加载故事数据失败:', error);
            this.showError('加载故事数据失败: ' + error.message);
            this.renderErrorState();
        } finally {
            this.setLoading(false);
        }
    }

    /**
     * 刷新故事数据
     */
    async refreshStory() {
        if (this.sceneId) {
            await this.loadStory(this.sceneId);
        }
    }

    // ========================================
    // 故事选择处理
    // ========================================

    /**
     * 处理用户选择
     */
    async makeChoice(nodeId, choiceId) {
        try {
            this.setLoading(true);
            
            console.log(`📝 处理选择: 节点=${nodeId}, 选择=${choiceId}`);
            
            // 调用后端API处理选择
            const result = await this.safeAPICall(() => 
                API.makeStoryChoice(this.sceneId, nodeId, choiceId)
            );

            // 更新故事数据
            this.currentStoryData = result.storyData;
            
            // 记录选择
            this.state.selectedChoices.push({
                nodeId,
                choiceId,
                timestamp: new Date()
            });
            
            // 更新进度
            this.updateStoryProgress();
            
            // 重新渲染故事
            this.renderStoryInterface();
            
            // 滚动到新内容
            this.scrollToLatestNode();
            
            console.log('✅ 选择处理完成');
            
        } catch (error) {
            console.error('❌ 处理选择失败:', error);
            this.showError('处理选择失败: ' + error.message);
        } finally {
            this.setLoading(false);
        }
    }

    /**
     * 回溯到指定节点
     */
    async revertToNode(nodeIndex) {
        try {
            const confirmed = await this.safeConfirm(
                `确定要回溯到第 ${nodeIndex + 1} 个节点吗？这将清除之后的所有选择。`,
                { type: 'warning' }
            );
            
            if (!confirmed) return;
            
            this.setLoading(true);
            
            console.log(`⏪ 回溯到节点: ${nodeIndex}`);
            
            // 调用后端API回溯故事
            const result = await this.safeAPICall(() => 
                API.revertStoryToNode(this.sceneId, nodeIndex)
            );

            // 更新故事数据
            this.currentStoryData = result.storyData;
            this.state.currentNodeIndex = nodeIndex;
            
            // 清除之后的选择记录
            this.state.selectedChoices = this.state.selectedChoices.slice(0, nodeIndex);
            
            // 更新进度
            this.updateStoryProgress();
            
            // 重新渲染故事
            this.renderStoryInterface();
            
            this.showSuccess('已成功回溯到指定节点');
            
        } catch (error) {
            console.error('❌ 故事回溯失败:', error);
            this.showError('故事回溯失败: ' + error.message);
        } finally {
            this.setLoading(false);
        }
    }

    /**
     * 跳转到指定节点
     */
    jumpToNode(nodeIndex) {
        if (!this.currentStoryData || !this.currentStoryData.nodes) return;
        
        const node = this.currentStoryData.nodes[nodeIndex];
        if (!node || !node.is_revealed) {
            this.showError('该节点尚未解锁');
            return;
        }
        
        this.state.currentNodeIndex = nodeIndex;
        
        // 滚动到指定节点
        const nodeElement = document.querySelector(`[data-node-index="${nodeIndex}"]`);
        if (nodeElement) {
            nodeElement.scrollIntoView({ 
                behavior: 'smooth', 
                block: 'center' 
            });
        }
    }

    // ========================================
    // 故事界面渲染
    // ========================================

    /**
     * 渲染故事界面
     */
    renderStoryInterface() {
        if (!this.state.hasStoryData) {
            this.renderEmptyState();
            return;
        }

        this.renderStoryContainer();
        this.renderStoryHeader();
        this.renderStoryNodes();
        this.renderStoryTimeline();
        this.renderStoryStats();
    }

    /**
     * 渲染空状态
     */
    renderEmptyState() {
        const container = this.getStoryContainer();
        container.innerHTML = `
            <div class="story-empty-state text-center py-5">
                <i class="bi bi-book display-1 text-muted"></i>
                <h4 class="mt-3">暂无故事数据</h4>
                <p class="text-muted">开始与角色互动来生成故事内容</p>
                <button class="btn btn-outline-primary story-refresh-btn">
                    <i class="bi bi-arrow-clockwise"></i> 刷新数据
                </button>
            </div>
        `;
    }

    /**
     * 渲染错误状态
     */
    renderErrorState() {
        const container = this.getStoryContainer();
        container.innerHTML = `
            <div class="story-error-state text-center py-5">
                <i class="bi bi-exclamation-triangle display-1 text-danger"></i>
                <h4 class="mt-3">加载失败</h4>
                <p class="text-muted">无法加载故事数据，请检查网络连接或稍后重试</p>
                <button class="btn btn-outline-primary story-refresh-btn">
                    <i class="bi bi-arrow-clockwise"></i> 重新加载
                </button>
            </div>
        `;
    }

    /**
     * 渲染故事容器
     */
    renderStoryContainer() {
        const container = this.getStoryContainer();
        container.innerHTML = `
            <div class="story-content">
                <div class="story-header" id="story-header"></div>
                <div class="story-timeline mb-4" id="story-timeline"></div>
                <div class="story-nodes" id="story-nodes"></div>
                <div class="story-stats mt-4" id="story-stats"></div>
            </div>
        `;
    }

    /**
     * 渲染故事头部
     */
    renderStoryHeader() {
        const header = document.getElementById('story-header');
        if (!header || !this.currentStoryData) return;

        const story = this.currentStoryData;
        
        header.innerHTML = `
            <div class="d-flex justify-content-between align-items-start mb-4">
                <div class="story-info">
                    <h3 class="story-title">${this.escapeHtml(story.title || '未命名故事')}</h3>
                    <p class="story-intro text-muted">${this.escapeHtml(story.intro || '')}</p>
                    ${story.main_objective ? `
                        <div class="story-objective">
                            <strong>主要目标:</strong> ${this.escapeHtml(story.main_objective)}
                        </div>
                    ` : ''}
                </div>
                <div class="story-actions">
                    <div class="btn-group">
                        <button class="btn btn-outline-secondary story-refresh-btn" title="刷新">
                            <i class="bi bi-arrow-clockwise"></i>
                        </button>
                        <button class="btn btn-outline-info story-export-btn" data-format="html" title="导出HTML">
                            <i class="bi bi-file-earmark-text"></i>
                        </button>
                        <button class="btn btn-outline-success story-export-btn" data-format="json" title="导出JSON">
                            <i class="bi bi-file-earmark-code"></i>
                        </button>
                        <button class="btn btn-outline-warning story-reset-btn" title="重置故事">
                            <i class="bi bi-arrow-counterclockwise"></i>
                        </button>
                    </div>
                </div>
            </div>
            <div class="story-progress mb-3">
                <div class="d-flex justify-content-between align-items-center mb-2">
                    <span>故事进度</span>
                    <span class="text-muted">${this.state.storyProgress.toFixed(1)}%</span>
                </div>
                <div class="progress">
                    <div class="progress-bar" 
                         style="width: ${this.state.storyProgress}%" 
                         role="progressbar"></div>
                </div>
            </div>
        `;
    }

    /**
     * 渲染故事节点
     */
    renderStoryNodes() {
        const container = document.getElementById('story-nodes');
        if (!container || !this.currentStoryData?.nodes) return;

        const nodesHtml = this.currentStoryData.nodes
            .filter(node => node.is_revealed)
            .map((node, index) => this.renderStoryNode(node, index))
            .join('');

        container.innerHTML = nodesHtml;
    }

    /**
     * 渲染单个故事节点
     */
    renderStoryNode(node, index) {
        const isCurrentNode = index === this.state.currentNodeIndex;
        const hasChoices = node.choices && node.choices.length > 0;
        const hasUnselectedChoices = hasChoices && node.choices.some(choice => !choice.selected);

        return `
            <div class="story-node ${isCurrentNode ? 'current' : ''}" 
                 data-node-index="${index}" 
                 data-node-id="${node.id}">
                <div class="node-header">
                    <div class="node-indicator">
                        <span class="node-number">${index + 1}</span>
                        <div class="node-type-badge badge bg-${this.getNodeTypeColor(node.type)}">
                            ${this.getNodeTypeLabel(node.type)}
                        </div>
                    </div>
                    <div class="node-timestamp text-muted">
                        ${this.formatTime(node.timestamp)}
                    </div>
                </div>
                
                <div class="node-content">
                    <div class="node-text">
                        ${this.escapeHtml(node.content).replace(/\n/g, '<br>')}
                    </div>
                    
                    ${node.character_action ? `
                        <div class="character-action mt-3">
                            <div class="action-header">
                                <i class="bi bi-person-check"></i>
                                <strong>${this.escapeHtml(node.character_name || '角色')}</strong> 的行动
                            </div>
                            <div class="action-content">
                                ${this.escapeHtml(node.character_action)}
                            </div>
                        </div>
                    ` : ''}
                    
                    ${hasChoices ? `
                        <div class="story-choices mt-4">
                            <h6 class="choices-title">
                                <i class="bi bi-list-check"></i>
                                选择你的行动
                            </h6>
                            <div class="choices-grid">
                                ${node.choices.map(choice => this.renderChoice(node.id, choice)).join('')}
                            </div>
                        </div>
                    ` : ''}
                </div>
            </div>
        `;
    }

    /**
     * 渲染选择按钮
     */
    renderChoice(nodeId, choice) {
        const isSelected = choice.selected;
        const isDisabled = isSelected || this.isLoading;
        
        return `
            <div class="choice-item ${isSelected ? 'selected' : ''} ${isDisabled ? 'disabled' : ''}">
                <button class="btn story-choice-btn ${isSelected ? 'btn-success' : 'btn-outline-primary'}" 
                        data-node-id="${nodeId}" 
                        data-choice-id="${choice.id}"
                        ${isDisabled ? 'disabled' : ''}>
                    <div class="choice-type-badge">
                        ${this.getChoiceTypeIcon(choice.type)}
                        ${this.getChoiceTypeLabel(choice.type)}
                    </div>
                    <div class="choice-text">
                        ${this.escapeHtml(choice.text)}
                    </div>
                    ${isSelected ? `
                        <div class="choice-selected-indicator">
                            <i class="bi bi-check-circle"></i> 已选择
                        </div>
                    ` : ''}
                </button>
            </div>
        `;
    }

    /**
     * 渲染故事时间线
     */
    renderStoryTimeline() {
        const container = document.getElementById('story-timeline');
        if (!container || !this.currentStoryData?.nodes) return;

        const revealedNodes = this.currentStoryData.nodes.filter(node => node.is_revealed);
        
        container.innerHTML = `
            <div class="timeline-header mb-3">
                <h6 class="mb-0">
                    <i class="bi bi-clock-history"></i> 
                    故事时间线
                    <small class="text-muted">(Ctrl+点击可回溯)</small>
                </h6>
            </div>
            <div class="timeline-track">
                ${revealedNodes.map((node, index) => `
                    <div class="timeline-node ${index === this.state.currentNodeIndex ? 'current' : ''}" 
                         data-node-index="${index}"
                         title="${this.escapeHtml(node.content.substring(0, 50))}...">
                        <div class="timeline-dot"></div>
                        <div class="timeline-label">${index + 1}</div>
                    </div>
                `).join('')}
            </div>
        `;
    }

    /**
     * 渲染故事统计
     */
    renderStoryStats() {
        const container = document.getElementById('story-stats');
        if (!container || !this.currentStoryData) return;

        const stats = this.calculateStoryStats();
        
        container.innerHTML = `
            <div class="story-stats-card card">
                <div class="card-header">
                    <h6 class="mb-0">
                        <i class="bi bi-graph-up"></i> 故事统计
                    </h6>
                </div>
                <div class="card-body">
                    <div class="row g-3">
                        <div class="col-6 col-md-3">
                            <div class="stat-item text-center">
                                <div class="stat-value h5">${stats.totalNodes}</div>
                                <div class="stat-label text-muted">故事节点</div>
                            </div>
                        </div>
                        <div class="col-6 col-md-3">
                            <div class="stat-item text-center">
                                <div class="stat-value h5">${stats.totalChoices}</div>
                                <div class="stat-label text-muted">选择数量</div>
                            </div>
                        </div>
                        <div class="col-6 col-md-3">
                            <div class="stat-item text-center">
                                <div class="stat-value h5">${stats.revealedNodes}</div>
                                <div class="stat-label text-muted">已解锁</div>
                            </div>
                        </div>
                        <div class="col-6 col-md-3">
                            <div class="stat-item text-center">
                                <div class="stat-value h5">${stats.completionRate}%</div>
                                <div class="stat-label text-muted">完成度</div>
                            </div>
                        </div>
                    </div>
                    
                    ${stats.characterActions > 0 ? `
                        <div class="mt-3 pt-3 border-top">
                            <div class="row">
                                <div class="col-6">
                                    <div class="stat-item text-center">
                                        <div class="stat-value h6">${stats.characterActions}</div>
                                        <div class="stat-label text-muted">角色行动</div>
                                    </div>
                                </div>
                                <div class="col-6">
                                    <div class="stat-item text-center">
                                        <div class="stat-value h6">${stats.estimatedReadTime}分钟</div>
                                        <div class="stat-label text-muted">阅读时长</div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    ` : ''}
                </div>
            </div>
        `;
    }

    // ========================================
    // 故事导出功能
    // ========================================

    /**
     * 导出故事
     */
    async exportStory(format = 'html') {
        try {
            this.setLoading(true);
            
            console.log(`📤 导出故事: 格式=${format}`);
            
            // 调用后端API导出故事
            const result = await this.safeAPICall(() => 
                API.exportStory(this.sceneId, format, {
                    include_choices: true,
                    include_stats: true,
                    include_timeline: true
                })
            );

            // 触发文件下载
            this.downloadFile(result.content, result.filename, result.mime_type);
            
            this.showSuccess(`故事已成功导出为 ${format.toUpperCase()} 格式`);
            
        } catch (error) {
            console.error('❌ 导出故事失败:', error);
            this.showError('导出故事失败: ' + error.message);
        } finally {
            this.setLoading(false);
        }
    }

    /**
     * 重置故事
     */
    async resetStory() {
        try {
            const confirmed = await this.safeConfirm(
                '确定要重置整个故事吗？这将清除所有进度和选择记录。',
                { type: 'danger' }
            );
            
            if (!confirmed) return;
            
            this.setLoading(true);
            
            // 调用后端API重置故事
            await this.safeAPICall(() => 
                API.resetStory(this.sceneId)
            );

            // 重新加载故事
            await this.loadStory(this.sceneId);
            
            this.showSuccess('故事已重置');
            
        } catch (error) {
            console.error('❌ 重置故事失败:', error);
            this.showError('重置故事失败: ' + error.message);
        } finally {
            this.setLoading(false);
        }
    }

    // ========================================
    // 辅助计算功能
    // ========================================

    /**
     * 更新故事进度
     */
    updateStoryProgress() {
        if (!this.currentStoryData?.nodes) {
            this.state.storyProgress = 0;
            return;
        }

        const totalNodes = this.currentStoryData.nodes.length;
        const revealedNodes = this.currentStoryData.nodes.filter(node => node.is_revealed).length;
        
        this.state.storyProgress = totalNodes > 0 ? (revealedNodes / totalNodes) * 100 : 0;
    }

    /**
     * 计算故事统计
     */
    calculateStoryStats() {
        if (!this.currentStoryData?.nodes) {
            return {
                totalNodes: 0,
                revealedNodes: 0,
                totalChoices: 0,
                characterActions: 0,
                completionRate: 0,
                estimatedReadTime: 0
            };
        }

        const nodes = this.currentStoryData.nodes;
        const revealedNodes = nodes.filter(node => node.is_revealed);
        const totalChoices = revealedNodes.reduce((sum, node) => sum + (node.choices?.length || 0), 0);
        const characterActions = revealedNodes.filter(node => node.character_action).length;
        
        // 估算阅读时间（按每分钟200字计算）
        const totalContent = revealedNodes.reduce((sum, node) => sum + (node.content?.length || 0), 0);
        const estimatedReadTime = Math.ceil(totalContent / 200);

        return {
            totalNodes: nodes.length,
            revealedNodes: revealedNodes.length,
            totalChoices,
            characterActions,
            completionRate: Math.round(this.state.storyProgress),
            estimatedReadTime
        };
    }

    // ========================================
    // 工具方法
    // ========================================

    /**
     * 获取节点类型颜色
     */
    getNodeTypeColor(type) {
        const colorMap = {
            'narrative': 'primary',
            'dialogue': 'info',
            'action': 'success',
            'decision': 'warning',
            'conclusion': 'secondary'
        };
        return colorMap[type] || 'secondary';
    }

    /**
     * 获取节点类型标签
     */
    getNodeTypeLabel(type) {
        const labelMap = {
            'narrative': '叙述',
            'dialogue': '对话',
            'action': '行动',
            'decision': '决策',
            'conclusion': '结局'
        };
        return labelMap[type] || type;
    }

    /**
     * 获取选择类型图标
     */
    getChoiceTypeIcon(type) {
        const iconMap = {
            'action': '⚡',
            'dialogue': '💬',
            'exploration': '🔍',
            'strategy': '🎯'
        };
        return iconMap[type] || '📌';
    }

    /**
     * 获取选择类型标签
     */
    getChoiceTypeLabel(type) {
        const labelMap = {
            'action': '行动',
            'dialogue': '对话',
            'exploration': '探索',
            'strategy': '策略'
        };
        return labelMap[type] || type;
    }

    /**
     * 滚动到最新节点
     */
    scrollToLatestNode() {
        const latestNode = document.querySelector('.story-node:last-child');
        if (latestNode) {
            setTimeout(() => {
                latestNode.scrollIntoView({ 
                    behavior: 'smooth', 
                    block: 'start' 
                });
            }, 100);
        }
    }

    /**
     * 下载文件
     */
    downloadFile(content, filename, mimeType) {
        const blob = new Blob([content], { type: mimeType });
        const url = URL.createObjectURL(blob);
        
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        
        URL.revokeObjectURL(url);
    }

    /**
     * 获取故事容器
     */
    getStoryContainer() {
        let container = document.getElementById('story-container');
        if (!container) {
            container = document.createElement('div');
            container.id = 'story-container';
            container.className = 'story-container';
            
            const mainContent = document.querySelector('.container') || document.body;
            mainContent.appendChild(container);
        }
        return container;
    }

    /**
     * 设置加载状态
     */
    setLoading(isLoading) {
        this.isLoading = isLoading;
        
        // 更新按钮状态
        document.querySelectorAll('.story-choice-btn').forEach(btn => {
            btn.disabled = isLoading;
        });
        
        document.querySelectorAll('.story-refresh-btn, .story-export-btn, .story-reset-btn').forEach(btn => {
            btn.disabled = isLoading;
        });
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
     * 安全调用确认对话框
     */
    async safeConfirm(message, options = {}) {
        if (typeof Utils !== 'undefined' && typeof Utils.showConfirm === 'function') {
            return await Utils.showConfirm(message, options);
        }
        
        return confirm(message);
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
    formatTime(timestamp) {
        if (typeof Utils !== 'undefined' && typeof Utils.formatTime === 'function') {
            return Utils.formatTime(timestamp);
        }
        
        const date = new Date(timestamp);
        return date.toLocaleString('zh-CN');
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
     * 初始化场景故事系统
     */
    async init(sceneId) {
        if (!sceneId) {
            console.warn('⚠️ 场景ID为空，跳过故事数据加载');
            return;
        }

        try {
            console.log(`📚 初始化场景 ${sceneId} 的故事系统...`);
            
            await this.loadStory(sceneId);
            
            console.log('✅ 故事系统初始化成功');
        } catch (error) {
            console.error('❌ 故事系统初始化失败:', error);
            throw error;
        }
    }

    /**
     * 获取当前故事状态
     */
    getStoryState() {
        return {
            ...this.state,
            storyData: this.currentStoryData,
            sceneId: this.sceneId,
            isLoading: this.isLoading
        };
    }

    /**
     * 检查是否已初始化
     */
    isInitialized() {
        return this.state.initialized;
    }

    /**
     * 销毁故事管理器
     */
    destroy() {
        // 移除事件监听器
        this.eventListeners.forEach((handler, key) => {
            const [eventType] = key.split('-');
            document.removeEventListener(eventType, handler);
        });
        this.eventListeners.clear();

        // 清理数据
        this.currentStoryData = null;
        this.sceneId = null;
        this.state.selectedChoices = [];
        
        console.log('🗑️ StoryManager 已销毁');
    }
}

// ========================================
// 全局函数（保持向后兼容）
// ========================================

/**
 * 刷新故事
 */
function refreshStory() {
    if (window.storyManager) {
        window.storyManager.refreshStory();
    }
}

/**
 * 导出故事
 */
function exportStory(format = 'html') {
    if (window.storyManager) {
        window.storyManager.exportStory(format);
    }
}

// ========================================
// 全局初始化
// ========================================

// 确保在DOM加载完成后创建全局实例
if (typeof window !== 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            window.StoryManager = StoryManager;
            window.storyManager = new StoryManager();
            console.log('📚 StoryManager 已准备就绪');
        });
    } else {
        window.StoryManager = StoryManager;
        window.storyManager = new StoryManager();
        console.log('📚 StoryManager 已准备就绪');
    }
}

// 添加CSS样式
if (typeof document !== 'undefined') {
    const addStoryStyles = () => {
        if (document.getElementById('story-manager-styles')) return;

        const style = document.createElement('style');
        style.id = 'story-manager-styles';
        style.textContent = `
            /* 故事容器样式 */
            .story-container {
                max-width: 1200px;
                margin: 0 auto;
                padding: 20px;
            }
            
            .story-empty-state, .story-error-state {
                padding: 60px 20px;
            }
            
            /* 故事节点样式 */
            .story-node {
                background: white;
                border: 1px solid #dee2e6;
                border-radius: 12px;
                margin-bottom: 30px;
                padding: 24px;
                position: relative;
                transition: all 0.3s ease;
            }
            
            .story-node.current {
                border-color: #007bff;
                box-shadow: 0 0 0 3px rgba(0, 123, 255, 0.1);
            }
            
            .node-header {
                display: flex;
                justify-content: between;
                align-items: center;
                margin-bottom: 16px;
            }
            
            .node-indicator {
                display: flex;
                align-items: center;
                gap: 12px;
            }
            
            .node-number {
                background: #007bff;
                color: white;
                border-radius: 50%;
                width: 32px;
                height: 32px;
                display: flex;
                align-items: center;
                justify-content: center;
                font-weight: 600;
                font-size: 14px;
            }
            
            .node-content {
                line-height: 1.7;
            }
            
            .node-text {
                font-size: 16px;
                color: #2c3e50;
                margin-bottom: 16px;
            }
            
            /* 角色行动样式 */
            .character-action {
                background: #f8f9fa;
                border-left: 4px solid #28a745;
                padding: 16px;
                border-radius: 8px;
                margin: 16px 0;
            }
            
            .action-header {
                display: flex;
                align-items: center;
                gap: 8px;
                margin-bottom: 8px;
                color: #28a745;
                font-weight: 600;
            }
            
            /* 选择按钮样式 */
            .story-choices {
                margin-top: 24px;
            }
            
            .choices-title {
                margin-bottom: 16px;
                color: #495057;
                display: flex;
                align-items: center;
                gap: 8px;
            }
            
            .choices-grid {
                display: grid;
                gap: 12px;
                grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            }
            
            .choice-item {
                transition: all 0.3s ease;
            }
            
            .story-choice-btn {
                width: 100%;
                text-align: left;
                padding: 16px;
                border-radius: 8px;
                border: 2px solid #dee2e6;
                background: white;
                transition: all 0.3s ease;
                position: relative;
            }
            
            .story-choice-btn:hover:not(:disabled) {
                transform: translateY(-2px);
                box-shadow: 0 4px 12px rgba(0,0,0,0.1);
            }
            
            .story-choice-btn.btn-success {
                background: #d4edda;
                border-color: #28a745;
            }
            
            .choice-type-badge {
                display: flex;
                align-items: center;
                gap: 6px;
                font-size: 12px;
                font-weight: 600;
                margin-bottom: 8px;
                color: #6c757d;
            }
            
            .choice-text {
                font-size: 14px;
                line-height: 1.5;
                color: #495057;
            }
            
            .choice-selected-indicator {
                margin-top: 8px;
                color: #28a745;
                font-size: 12px;
                font-weight: 600;
                display: flex;
                align-items: center;
                gap: 4px;
            }
            
            /* 时间线样式 */
            .timeline-track {
                display: flex;
                align-items: center;
                gap: 8px;
                overflow-x: auto;
                padding: 16px 0;
                background: #f8f9fa;
                border-radius: 8px;
                padding: 16px;
            }
            
            .timeline-node {
                display: flex;
                flex-direction: column;
                align-items: center;
                cursor: pointer;
                transition: all 0.3s ease;
                min-width: 60px;
            }
            
            .timeline-node:hover {
                transform: scale(1.1);
            }
            
            .timeline-node.current .timeline-dot {
                background: #007bff;
                box-shadow: 0 0 0 4px rgba(0, 123, 255, 0.2);
            }
            
            .timeline-dot {
                width: 16px;
                height: 16px;
                border-radius: 50%;
                background: #dee2e6;
                margin-bottom: 8px;
                transition: all 0.3s ease;
            }
            
            .timeline-label {
                font-size: 12px;
                color: #6c757d;
                font-weight: 600;
            }
            
            /* 统计卡片样式 */
            .story-stats-card {
                border: 1px solid #dee2e6;
            }
            
            .stat-item {
                padding: 16px;
                background: #f8f9fa;
                border-radius: 8px;
                text-align: center;
            }
            
            .stat-value {
                font-weight: 700;
                color: #495057;
                margin-bottom: 4px;
            }
            
            .stat-label {
                font-size: 0.85rem;
                font-weight: 500;
            }
            
            /* 响应式设计 */
            @media (max-width: 768px) {
                .story-container {
                    padding: 16px;
                }
                
                .story-node {
                    padding: 16px;
                    margin-bottom: 20px;
                }
                
                .choices-grid {
                    grid-template-columns: 1fr;
                }
                
                .timeline-track {
                    padding: 12px;
                }
                
                .node-header {
                    flex-direction: column;
                    align-items: flex-start;
                    gap: 8px;
                }
            }
        `;
        
        document.head.appendChild(style);
        console.log('✅ StoryManager 样式已加载');
    };
    
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', addStoryStyles);
    } else {
        addStoryStyles();
    }
}
