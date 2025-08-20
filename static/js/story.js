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

        // 手动进度调整事件
        this.addEventDelegate('click', '.progress-increment-btn', (e, target) => {
            const amount = parseInt(target.dataset.amount || '5');
            this.incrementProgress(amount);
        });

        // 进度重置事件
        this.addEventDelegate('click', '.progress-reset-btn', (e, target) => {
            if (confirm('确定要重置故事进度吗？')) {
                this.resetProgress();
            }
        });

        // 节点快速跳转事件
        this.addEventDelegate('click', '.node-quick-jump', (e, target) => {
            const nodeId = target.dataset.nodeId;
            this.scrollToNode(nodeId);
        });

        // 监听自定义故事进度事件
        document.addEventListener('storyProgressUpdated', (e) => {
            console.log('📈 故事进度更新事件:', e.detail);

            // 可以在这里添加额外的处理逻辑
            this.onProgressUpdated(e.detail);
        });

        // 监听实时通信的故事事件
        document.addEventListener('story:event', (e) => {
            this.handleRealtimeStoryEvent(e.detail);
        });

        // 监听故事数据变化事件
        document.addEventListener('story:data_changed', (e) => {
            this.handleStoryDataChange(e.detail);
        });
    }

    /**
    * 处理进度更新后的回调
    */
    onProgressUpdated(detail) {
        try {
            // 更新页面标题中的进度
            this.updatePageTitleProgress(detail.progress);

            // 保存进度到本地存储
            this.saveProgressToLocalStorage(detail);

            // 触发成就检查
            this.checkProgressAchievements(detail);

        } catch (error) {
            console.error('❌ 进度更新回调失败:', error);
        }
    }

    /**
    * 处理实时故事事件
    */
    handleRealtimeStoryEvent(eventDetail) {
        if (!eventDetail) return;

        try {
            const { eventType, eventData } = eventDetail;

            switch (eventType) {
                case 'progress_update':
                    this.updateProgress(eventData);
                    break;

                case 'node_unlocked':
                    this.handleUnlockedNodes([eventData]);
                    break;

                case 'objective_completed':
                    this.handleCompletedObjectives([eventData]);
                    break;

                case 'story_advanced':
                    this.batchUpdateStoryState(eventData);
                    break;

                default:
                    console.log('📖 未处理的故事事件:', eventType, eventData);
            }

        } catch (error) {
            console.error('❌ 处理实时故事事件失败:', error);
        }
    }

    /**
    * 处理故事数据变化
    */
    handleStoryDataChange(changeDetail) {
        if (!changeDetail) return;

        try {
            console.log('📊 故事数据变化:', changeDetail);

            // 如果是外部数据更新，重新加载故事
            if (changeDetail.source === 'external') {
                this.refreshStory();
            }

            // 如果是节点数据更新，重新渲染
            if (changeDetail.type === 'nodes_updated') {
                this.renderStoryNodes();
            }

        } catch (error) {
            console.error('❌ 处理故事数据变化失败:', error);
        }
    }

    /**
    * 更新页面标题中的进度
    */
    updatePageTitleProgress(progress) {
        const originalTitle = document.title.replace(/ \(\d+%\)$/, '');
        document.title = `${originalTitle} (${Math.round(progress)}%)`;
    }

    /**
     * 保存进度到本地存储
     */
    saveProgressToLocalStorage(progressDetail) {
        try {
            const progressData = {
                sceneId: this.sceneId,
                progress: progressDetail.progress,
                currentNodeIndex: this.state.currentNodeIndex,
                selectedChoices: this.state.selectedChoices,
                timestamp: Date.now()
            };

            localStorage.setItem(
                `story_progress_${this.sceneId}`,
                JSON.stringify(progressData)
            );

        } catch (error) {
            console.warn('保存进度到本地存储失败:', error);
        }
    }

    /**
    * 从本地存储加载进度
    */
    loadProgressFromLocalStorage() {
        try {
            const savedData = localStorage.getItem(`story_progress_${this.sceneId}`);
            if (savedData) {
                const progressData = JSON.parse(savedData);

                // 检查数据有效性（不超过24小时）
                const dataAge = Date.now() - progressData.timestamp;
                if (dataAge < 24 * 60 * 60 * 1000) {
                    return progressData;
                }
            }
        } catch (error) {
            console.warn('从本地存储加载进度失败:', error);
        }
        return null;
    }

    /**
     * 检查进度成就
     */
    checkProgressAchievements(progressDetail) {
        const progress = progressDetail.progress;

        // 进度里程碑成就
        const milestones = [25, 50, 75, 100];
        milestones.forEach(milestone => {
            if (progress >= milestone && !this.hasAchievement(`progress_${milestone}`)) {
                this.unlockAchievement(`progress_${milestone}`, `故事进度达到${milestone}%`);
            }
        });

        // 选择数量成就
        const choiceCount = this.state.selectedChoices.length;
        if (choiceCount >= 10 && !this.hasAchievement('choices_10')) {
            this.unlockAchievement('choices_10', '已做出10个故事选择');
        }
    }

    /**
    * 检查是否已有成就
    */
    hasAchievement(achievementId) {
        const achievements = JSON.parse(localStorage.getItem('story_achievements') || '[]');
        return achievements.includes(achievementId);
    }

    /**
     * 解锁成就
     */
    unlockAchievement(achievementId, description) {
        try {
            const achievements = JSON.parse(localStorage.getItem('story_achievements') || '[]');
            if (!achievements.includes(achievementId)) {
                achievements.push(achievementId);
                localStorage.setItem('story_achievements', JSON.stringify(achievements));

                // 显示成就通知
                this.showAchievementNotification(description);
            }
        } catch (error) {
            console.error('解锁成就失败:', error);
        }
    }

    /**
     * 显示成就通知
     */
    showAchievementNotification(description) {
        const message = `🏆 成就解锁: ${description}`;

        if (typeof Utils !== 'undefined' && Utils.showSuccess) {
            Utils.showSuccess(message, 6000);
        } else {
            console.log(message);
        }
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
                API.rewindStoryToNode(this.sceneId, nodeIndex)
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

    /**
     * 更新故事进度 - 供外部调用
     */
    updateProgress(progressData) {
        if (!progressData || typeof progressData !== 'object') {
            console.warn('⚠️ updateProgress: 无效的进度数据');
            return;
        }

        try {
            console.log('📈 更新故事进度:', progressData);

            // 更新内部故事数据
            if (this.currentStoryData) {
                // 更新进度百分比
                if (typeof progressData.progress === 'number') {
                    this.currentStoryData.progress = Math.max(0, Math.min(100, progressData.progress));
                    this.state.storyProgress = this.currentStoryData.progress;
                }

                // 更新当前状态
                if (progressData.current_state) {
                    this.currentStoryData.current_state = progressData.current_state;
                }

                // 更新故事节点数据
                if (progressData.story_data) {
                    this.mergeStoryData(progressData.story_data);
                }

                // 处理新解锁的节点
                if (progressData.unlocked_nodes && Array.isArray(progressData.unlocked_nodes)) {
                    this.handleUnlockedNodes(progressData.unlocked_nodes);
                }

                // 处理完成的目标
                if (progressData.completed_objectives && Array.isArray(progressData.completed_objectives)) {
                    this.handleCompletedObjectives(progressData.completed_objectives);
                }
            }

            // 更新UI
            this.updateProgressUI(progressData);

            // 触发进度更新事件
            this.triggerProgressEvent(progressData);

            console.log('✅ 故事进度更新完成');

        } catch (error) {
            console.error('❌ 更新故事进度失败:', error);
            this.showError('更新故事进度失败: ' + error.message);
        }
    }

    /**
     * 合并故事数据
     */
    mergeStoryData(newStoryData) {
        if (!newStoryData || !this.currentStoryData) return;

        try {
            // 合并节点数据
            if (newStoryData.nodes && Array.isArray(newStoryData.nodes)) {
                this.mergeStoryNodes(newStoryData.nodes);
            }

            // 更新主要目标
            if (newStoryData.main_objective) {
                this.currentStoryData.main_objective = newStoryData.main_objective;
            }

            // 更新当前状态
            if (newStoryData.current_state) {
                this.currentStoryData.current_state = newStoryData.current_state;
            }

            // 更新介绍文本
            if (newStoryData.intro) {
                this.currentStoryData.intro = newStoryData.intro;
            }

            console.log('📊 故事数据合并完成');

        } catch (error) {
            console.error('❌ 合并故事数据失败:', error);
        }
    }

    /**
     * 合并故事节点
     */
    mergeStoryNodes(newNodes) {
        if (!this.currentStoryData.nodes) {
            this.currentStoryData.nodes = [];
        }

        newNodes.forEach(newNode => {
            // 查找现有节点
            const existingIndex = this.currentStoryData.nodes.findIndex(
                node => node.id === newNode.id
            );

            if (existingIndex >= 0) {
                // 更新现有节点
                this.currentStoryData.nodes[existingIndex] = {
                    ...this.currentStoryData.nodes[existingIndex],
                    ...newNode
                };
            } else {
                // 添加新节点
                this.currentStoryData.nodes.push(newNode);
            }
        });

        // 按创建时间排序
        this.currentStoryData.nodes.sort((a, b) =>
            new Date(a.created_at || a.timestamp) - new Date(b.created_at || b.timestamp)
        );
    }

    /**
     * 处理新解锁的节点
     */
    handleUnlockedNodes(unlockedNodes) {
        if (!Array.isArray(unlockedNodes) || unlockedNodes.length === 0) return;

        try {
            console.log('🔓 处理新解锁的节点:', unlockedNodes);

            unlockedNodes.forEach(nodeData => {
                // 添加解锁动画效果
                this.animateNodeUnlock(nodeData.id);

                // 显示解锁通知
                this.showUnlockNotification(nodeData);
            });

            // 重新渲染故事节点以显示新内容
            this.renderStoryNodes();

            // 滚动到最新解锁的节点
            if (unlockedNodes.length > 0) {
                setTimeout(() => {
                    this.scrollToNode(unlockedNodes[unlockedNodes.length - 1].id);
                }, 500);
            }

        } catch (error) {
            console.error('❌ 处理解锁节点失败:', error);
        }
    }

    /**
     * 处理完成的目标
     */
    handleCompletedObjectives(completedObjectives) {
        if (!Array.isArray(completedObjectives) || completedObjectives.length === 0) return;

        try {
            console.log('🎯 处理完成的目标:', completedObjectives);

            completedObjectives.forEach(objective => {
                // 显示目标完成通知
                this.showObjectiveCompletedNotification(objective);

                // 触发庆祝动画
                this.triggerCelebrationAnimation();
            });

        } catch (error) {
            console.error('❌ 处理完成目标失败:', error);
        }
    }

    /**
     * 更新进度UI
     */
    updateProgressUI(progressData) {
        try {
            // 更新进度条
            this.updateProgressBar(progressData.progress || this.state.storyProgress);

            // 更新进度文本
            this.updateProgressText(progressData);

            // 更新故事统计
            this.renderStoryStats();

            // 更新故事头部信息
            this.updateStoryHeader(progressData);

        } catch (error) {
            console.error('❌ 更新进度UI失败:', error);
        }
    }

    /**
     * 更新进度条
     */
    updateProgressBar(progress) {
        const progressBars = document.querySelectorAll('.progress-bar');
        const progressTexts = document.querySelectorAll('.progress-text');

        const safeProgress = Math.max(0, Math.min(100, progress || 0));

        progressBars.forEach(bar => {
            bar.style.width = `${safeProgress}%`;
            bar.setAttribute('aria-valuenow', safeProgress);

            // 添加进度变化动画
            bar.style.transition = 'width 0.6s ease-in-out';
        });

        progressTexts.forEach(text => {
            text.textContent = `${safeProgress.toFixed(1)}%`;
        });
    }

    /**
     * 更新进度文本
     */
    updateProgressText(progressData) {
        const progressElements = document.querySelectorAll('.story-progress-text');

        progressElements.forEach(element => {
            let progressText = '';

            if (progressData.current_state) {
                progressText += `当前状态: ${progressData.current_state}`;
            }

            if (progressData.completed_objectives) {
                progressText += ` | 已完成目标: ${progressData.completed_objectives.length}`;
            }

            if (progressData.unlocked_nodes) {
                progressText += ` | 新解锁节点: ${progressData.unlocked_nodes.length}`;
            }

            element.textContent = progressText;
        });
    }

    /**
     * 更新故事头部信息
     */
    updateStoryHeader(progressData) {
        const headerElement = document.getElementById('story-header');
        if (!headerElement) return;

        // 更新主要目标（如果有变化）
        if (progressData.main_objective) {
            const objectiveElement = headerElement.querySelector('.story-objective');
            if (objectiveElement) {
                objectiveElement.innerHTML = `<strong>主要目标:</strong> ${this.escapeHtml(progressData.main_objective)}`;
            }
        }

        // 更新当前状态显示
        if (progressData.current_state) {
            let stateElement = headerElement.querySelector('.story-current-state');
            if (!stateElement) {
                stateElement = document.createElement('div');
                stateElement.className = 'story-current-state mt-2';
                headerElement.querySelector('.story-info').appendChild(stateElement);
            }
            stateElement.innerHTML = `<strong>当前状态:</strong> ${this.escapeHtml(progressData.current_state)}`;
        }
    }

    /**
     * 触发进度更新事件
     */
    triggerProgressEvent(progressData) {
        try {
            // 创建自定义事件
            const event = new CustomEvent('storyProgressUpdated', {
                detail: {
                    progress: this.state.storyProgress,
                    progressData: progressData,
                    storyData: this.currentStoryData,
                    timestamp: Date.now()
                }
            });

            // 触发事件
            document.dispatchEvent(event);

            // 如果有实时管理器，也通过它触发事件
            if (window.RealtimeManager && typeof window.RealtimeManager.emit === 'function') {
                window.RealtimeManager.emit('story:progress_updated', {
                    sceneId: this.sceneId,
                    progress: this.state.storyProgress,
                    progressData: progressData
                });
            }

        } catch (error) {
            console.error('❌ 触发进度事件失败:', error);
        }
    }

    /**
     * 动画显示节点解锁
     */
    animateNodeUnlock(nodeId) {
        const nodeElement = document.querySelector(`[data-node-id="${nodeId}"]`);
        if (!nodeElement) return;

        try {
            // 添加解锁动画类
            nodeElement.classList.add('node-unlocking');

            // 创建解锁效果
            const unlockEffect = document.createElement('div');
            unlockEffect.className = 'unlock-effect';
            unlockEffect.innerHTML = '🔓';
            nodeElement.appendChild(unlockEffect);

            // 移除动画类和效果
            setTimeout(() => {
                nodeElement.classList.remove('node-unlocking');
                if (unlockEffect.parentNode) {
                    unlockEffect.remove();
                }
            }, 2000);

        } catch (error) {
            console.error('❌ 节点解锁动画失败:', error);
        }
    }

    /**
     * 显示解锁通知
     */
    showUnlockNotification(nodeData) {
        const message = `🔓 新故事节点已解锁: ${nodeData.title || '未命名节点'}`;

        if (typeof Utils !== 'undefined' && Utils.showSuccess) {
            Utils.showSuccess(message, 4000);
        } else {
            console.log(message);
        }
    }

    /**
     * 显示目标完成通知
     */
    showObjectiveCompletedNotification(objective) {
        const message = `🎯 目标完成: ${objective.title || objective.description || '未知目标'}`;

        if (typeof Utils !== 'undefined' && Utils.showSuccess) {
            Utils.showSuccess(message, 5000);
        } else {
            console.log(message);
        }
    }

    /**
     * 触发庆祝动画
     */
    triggerCelebrationAnimation() {
        try {
            // 创建庆祝效果
            const celebration = document.createElement('div');
            celebration.className = 'celebration-animation';
            celebration.innerHTML = '🎉🎊✨';
            celebration.style.cssText = `
            position: fixed;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            font-size: 3rem;
            z-index: 9999;
            pointer-events: none;
            animation: celebrationPulse 2s ease-out;
        `;

            document.body.appendChild(celebration);

            // 移除庆祝效果
            setTimeout(() => {
                if (celebration.parentNode) {
                    celebration.remove();
                }
            }, 2000);

            // 添加庆祝动画样式
            this.addCelebrationStyles();

        } catch (error) {
            console.error('❌ 庆祝动画失败:', error);
        }
    }

    /**
     * 添加庆祝动画样式
     */
    addCelebrationStyles() {
        if (document.getElementById('celebration-styles')) return;

        const style = document.createElement('style');
        style.id = 'celebration-styles';
        style.textContent = `
        @keyframes celebrationPulse {
            0% {
                transform: translate(-50%, -50%) scale(0);
                opacity: 0;
            }
            50% {
                transform: translate(-50%, -50%) scale(1.2);
                opacity: 1;
            }
            100% {
                transform: translate(-50%, -50%) scale(1);
                opacity: 0;
            }
        }

        .node-unlocking {
            animation: nodeUnlockPulse 1s ease-in-out;
            border: 2px solid #28a745;
            box-shadow: 0 0 15px rgba(40, 167, 69, 0.5);
        }

        @keyframes nodeUnlockPulse {
            0%, 100% {
                transform: scale(1);
            }
            50% {
                transform: scale(1.05);
            }
        }

        .unlock-effect {
            position: absolute;
            top: -10px;
            right: -10px;
            font-size: 1.5rem;
            animation: unlockBounce 1s ease-out;
        }

        @keyframes unlockBounce {
            0% {
                transform: scale(0) rotate(0deg);
                opacity: 0;
            }
            50% {
                transform: scale(1.3) rotate(180deg);
                opacity: 1;
            }
            100% {
                transform: scale(1) rotate(360deg);
                opacity: 0;
            }
        }
    `;

        document.head.appendChild(style);
    }

    /**
     * 滚动到指定节点
     */
    scrollToNode(nodeId) {
        const nodeElement = document.querySelector(`[data-node-id="${nodeId}"]`);
        if (nodeElement) {
            nodeElement.scrollIntoView({
                behavior: 'smooth',
                block: 'center'
            });

            // 添加高亮效果
            nodeElement.classList.add('highlighted');
            setTimeout(() => {
                nodeElement.classList.remove('highlighted');
            }, 3000);
        }
    }

    /**
     * 获取当前故事进度
     */
    getCurrentProgress() {
        return {
            progress: this.state.storyProgress,
            currentNodeIndex: this.state.currentNodeIndex,
            totalNodes: this.currentStoryData?.nodes?.length || 0,
            revealedNodes: this.currentStoryData?.nodes?.filter(node => node.is_revealed).length || 0,
            selectedChoices: this.state.selectedChoices.length,
            sceneId: this.sceneId
        };
    }

    /**
     * 设置故事进度（手动设置）
     */
    setStoryProgress(progress) {
        if (typeof progress !== 'number' || progress < 0 || progress > 100) {
            console.warn('⚠️ 无效的进度值:', progress);
            return false;
        }

        this.state.storyProgress = progress;
        this.updateProgressBar(progress);

        console.log(`📊 故事进度已设置为: ${progress}%`);
        return true;
    }

    /**
     * 增加故事进度
     */
    incrementProgress(amount = 5) {
        const newProgress = Math.min(100, this.state.storyProgress + amount);
        return this.setStoryProgress(newProgress);
    }

    /**
     * 重置故事进度
     */
    resetProgress() {
        this.state.storyProgress = 0;
        this.state.currentNodeIndex = 0;
        this.state.selectedChoices = [];

        this.updateProgressBar(0);

        console.log('🔄 故事进度已重置');
    }

    /**
     * 批量更新故事状态
     */
    batchUpdateStoryState(updates) {
        if (!updates || typeof updates !== 'object') {
            console.warn('⚠️ 无效的批量更新数据');
            return;
        }

        try {
            console.log('📦 批量更新故事状态:', updates);

            let hasChanges = false;

            // 更新进度
            if (typeof updates.progress === 'number') {
                this.state.storyProgress = Math.max(0, Math.min(100, updates.progress));
                hasChanges = true;
            }

            // 更新当前节点索引
            if (typeof updates.currentNodeIndex === 'number') {
                this.state.currentNodeIndex = updates.currentNodeIndex;
                hasChanges = true;
            }

            // 更新故事数据
            if (updates.storyData) {
                this.mergeStoryData(updates.storyData);
                hasChanges = true;
            }

            // 处理新解锁内容
            if (updates.unlockedNodes) {
                this.handleUnlockedNodes(updates.unlockedNodes);
                hasChanges = true;
            }

            // 处理完成的目标
            if (updates.completedObjectives) {
                this.handleCompletedObjectives(updates.completedObjectives);
                hasChanges = true;
            }

            // 如果有变化，更新UI
            if (hasChanges) {
                this.updateProgressUI(updates);
                this.renderStoryInterface();
            }

            console.log('✅ 批量更新完成');

        } catch (error) {
            console.error('❌ 批量更新故事状态失败:', error);
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
                API.exportStoryDocument(this.sceneId, format, {
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

// 在开发环境中添加故事进度调试工具
if (typeof window !== 'undefined' && 
    (window.location?.hostname === 'localhost' || window.location?.search.includes('debug=1'))) {

    window.STORY_PROGRESS_DEBUG = {
        // 手动更新进度
        updateProgress: (progressData) => {
            if (window.storyManager && window.storyManager.updateProgress) {
                window.storyManager.updateProgress(progressData);
                return true;
            }
            return false;
        },

        // 设置进度百分比
        setProgress: (percentage) => {
            if (window.storyManager && window.storyManager.setStoryProgress) {
                return window.storyManager.setStoryProgress(percentage);
            }
            return false;
        },

        // 增加进度
        incrementProgress: (amount = 10) => {
            if (window.storyManager && window.storyManager.incrementProgress) {
                return window.storyManager.incrementProgress(amount);
            }
            return false;
        },

        // 重置进度
        resetProgress: () => {
            if (window.storyManager && window.storyManager.resetProgress) {
                window.storyManager.resetProgress();
                return true;
            }
            return false;
        },

        // 获取当前进度
        getCurrentProgress: () => {
            if (window.storyManager && window.storyManager.getCurrentProgress) {
                return window.storyManager.getCurrentProgress();
            }
            return null;
        },

        // 模拟节点解锁
        simulateNodeUnlock: (nodeData = { id: 'test_node', title: '测试节点' }) => {
            if (window.storyManager && window.storyManager.handleUnlockedNodes) {
                window.storyManager.handleUnlockedNodes([nodeData]);
                return true;
            }
            return false;
        },

        // 模拟目标完成
        simulateObjectiveComplete: (objective = { title: '测试目标', description: '这是一个测试目标' }) => {
            if (window.storyManager && window.storyManager.handleCompletedObjectives) {
                window.storyManager.handleCompletedObjectives([objective]);
                return true;
            }
            return false;
        },

        // 批量更新测试
        testBatchUpdate: () => {
            const updates = {
                progress: Math.random() * 100,
                currentNodeIndex: Math.floor(Math.random() * 5),
                unlockedNodes: [{ id: 'batch_test_node', title: '批量测试节点' }],
                completedObjectives: [{ title: '批量测试目标' }]
            };

            return window.STORY_PROGRESS_DEBUG.updateProgress(updates);
        },

        // 触发庆祝动画
        triggerCelebration: () => {
            if (window.storyManager && window.storyManager.triggerCelebrationAnimation) {
                window.storyManager.triggerCelebrationAnimation();
                return true;
            }
            return false;
        },

        // 获取故事状态
        getStoryState: () => {
            if (window.storyManager && window.storyManager.getStoryState) {
                return window.storyManager.getStoryState();
            }
            return null;
        },

        // 运行所有测试
        runAllTests: () => {
            console.log('🔧 运行所有故事进度测试...');
            
            const tests = [
                { name: '设置进度', fn: () => window.STORY_PROGRESS_DEBUG.setProgress(50) },
                { name: '增加进度', fn: () => window.STORY_PROGRESS_DEBUG.incrementProgress(10) },
                { name: '模拟解锁', fn: () => window.STORY_PROGRESS_DEBUG.simulateNodeUnlock() },
                { name: '模拟完成', fn: () => window.STORY_PROGRESS_DEBUG.simulateObjectiveComplete() },
                { name: '批量更新', fn: () => window.STORY_PROGRESS_DEBUG.testBatchUpdate() }
            ];
            
            const results = tests.map(test => ({
                name: test.name,
                success: test.fn()
            }));
            
            console.table(results);
            return results;
        }
    };

    console.log('📈 故事进度调试工具已加载');
    console.log('使用 window.STORY_PROGRESS_DEBUG 进行调试');
}

