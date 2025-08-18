/**
 * 用户档案管理器
 * 基于后端完整的用户道具和技能API重新设计
 * 支持用户道具、技能、成就等功能
 */
class UserProfile {
    constructor() {
        this.currentUserId = null;
        this.userData = null;
        this.userItems = [];
        this.userSkills = [];
        this.userAchievements = [];
        this.filteredItems = [];
        this.filteredSkills = [];
        this.isLoading = false;
        this.eventListeners = new Map();

        // 初始化状态
        this.state = {
            initialized: false,
            hasUserData: false,
            totalItems: 0,
            totalSkills: 0,
            totalAchievements: 0
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
            const message = `UserProfile缺少必要的依赖: ${missing.join(', ')} `;
            console.error(message);

            // 显示错误（使用原生alert作为降级）
            if (typeof Utils !== 'undefined') {
                Utils.showError(message);
            } else {
                alert(`用户档案系统初始化失败: ${message} \n请确保正确加载了所有脚本文件`);
            }

            throw new Error(message);
        }

        console.log('✅ UserProfile 依赖检查通过');
        return true;
    }

    /**
     * 检查Bootstrap是否可用
     */
    static isBootstrapAvailable() {
        return typeof bootstrap !== 'undefined' &&
            bootstrap.Toast &&
            bootstrap.Modal;
    }

    /**
     * 初始化用户档案管理器
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

            console.log('✅ UserProfile 初始化完成');
        } catch (error) {
            console.error('❌ UserProfile 初始化失败:', error);
            this.showError('用户档案系统初始化失败: ' + error.message);
        }
    }

    /**
     * 等待依赖加载完成
     */
    async waitForDependencies() {
        // 如果Utils可用，使用其方法
        if (typeof Utils !== 'undefined' && typeof Utils.waitForDependencies === 'function') {
            return Utils.waitForDependencies(['API', 'Utils'], {
                timeout: 10000,
                context: 'UserProfile'
            });
        }

        // 降级方法：简单轮询检查
        const timeout = 10000;
        const checkInterval = 100;
        const startTime = Date.now();

        return new Promise((resolve, reject) => {
            const checkLoop = () => {
                if (typeof API !== 'undefined' && typeof Utils !== 'undefined') {
                    console.log('✅ UserProfile 依赖等待完成');
                    resolve();
                    return;
                }

                if (Date.now() - startTime > timeout) {
                    const errorMsg = 'UserProfile 依赖等待超时';
                    console.error(errorMsg);
                    reject(new Error(errorMsg));
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
        // 道具操作事件
        this.addEventDelegate('click', '.edit-item-btn', (e, target) => {
            this.editItem(target.dataset.itemId);
        });

        this.addEventDelegate('click', '.delete-item-btn', (e, target) => {
            this.deleteItem(target.dataset.itemId);
        });

        // 技能操作事件
        this.addEventDelegate('click', '.edit-skill-btn', (e, target) => {
            this.editSkill(target.dataset.skillId);
        });

        this.addEventDelegate('click', '.delete-skill-btn', (e, target) => {
            this.deleteSkill(target.dataset.skillId);
        });

        // 保存操作事件
        this.addEventDelegate('click', '.save-item-btn', (e, target) => {
            this.saveNewItem();
        });

        this.addEventDelegate('click', '.save-skill-btn', (e, target) => {
            this.saveNewSkill();
        });

        // 筛选事件
        this.addEventDelegate('change', '#item-category-filter', (e, target) => {
            this.filterItems();
        });

        this.addEventDelegate('change', '#item-rarity-filter', (e, target) => {
            this.filterItems();
        });

        this.addEventDelegate('change', '#skill-category-filter', (e, target) => {
            this.filterSkills();
        });

        this.addEventDelegate('input', '#skill-search', (e, target) => {
            this.filterSkills();
        });

        // 导出事件
        this.addEventDelegate('click', '.export-data-btn', (e, target) => {
            this.exportUserData();
        });

        // 刷新事件
        this.addEventDelegate('click', '.refresh-profile-btn', (e, target) => {
            if (this.currentUserId) {
                this.loadUserProfile(this.currentUserId);
            }
        });

        // 添加道具效果行
        this.addEventDelegate('click', '.add-effect-btn', (e, target) => {
            this.addEffectRow();
        });

        // 删除道具效果行
        this.addEventDelegate('click', '.remove-effect-btn', (e, target) => {
            this.removeEffectRow(target);
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
        this.eventListeners.set(`${eventType} -${selector} `, wrappedHandler);
    }

    // ========================================
    // 用户数据加载功能
    // ========================================

    /**
    * 设置当前用户ID (如果还没有的话)
    */
    setCurrentUser(userId) {
        this.currentUserId = userId;
        console.log('👤 设置当前用户:', userId);
    }

    /**
    * 获取当前用户ID
    */
    getCurrentUserId() {
        return this.currentUserId;
    }

    /**
     * 加载用户档案数据
     */
    async loadUserProfile(userId) {
        if (!userId) {
            this.showError('用户ID不能为空');
            return;
        }

        try {
            this.setLoading(true);
            this.currentUserId = userId;

            console.log(`👤 开始加载用户 ${userId} 的档案数据...`);

            // 更新用户ID显示
            this.updateUserIdDisplay(userId);

            // 并行加载用户数据
            const [itemsResult, skillsResult] = await Promise.all([
                this.safeAPICall(() => API.getUserItems(userId)),
                this.safeAPICall(() => API.getUserSkills(userId))
            ]);

            this.userItems = itemsResult || [];
            this.userSkills = skillsResult || [];
            this.userAchievements = []; // TODO: 后续可以从后端获取成就数据

            // 更新状态
            this.updateUserState();

            // 渲染数据
            this.renderUserProfile();

            this.showSuccess('用户档案加载完成');
            console.log('✅ 用户档案加载成功');

        } catch (error) {
            console.error('❌ 加载用户档案失败:', error);
            this.showError('无法加载用户档案数据: ' + error.message);
        } finally {
            this.setLoading(false);
        }
    }

    /**
     * 更新用户ID显示
     */
    updateUserIdDisplay(userId) {
        const userIdElement = document.getElementById('user-id');
        if (userIdElement) {
            userIdElement.textContent = `ID: ${userId} `;
        }
    }

    /**
     * 更新用户状态
     */
    updateUserState() {
        this.state.hasUserData = this.userItems.length > 0 || this.userSkills.length > 0;
        this.state.totalItems = this.userItems.length;
        this.state.totalSkills = this.userSkills.length;
        this.state.totalAchievements = this.userAchievements.length;
    }

    /**
     * 渲染用户档案
     */
    renderUserProfile() {
        this.renderUserStats();
        this.renderItems();
        this.renderSkills();
        this.renderAchievements();
    }

    /**
     * 创建标准化的用户偏好对象
     */
    createStandardPreferences(formData) {
        return {
            creativity_level: formData.creativity_level || "BALANCED",
            allow_plot_twists: formData.allow_plot_twists !== false,
            response_length: formData.response_length || "medium",
            language_style: formData.language_style || "casual",
            notification_level: formData.notification_enabled ? "all" : "none",
            preferred_model: formData.preferred_model || "",
            dark_mode: formData.theme === "dark",
            auto_save: formData.auto_save !== false
        };
    }

    /**
     * 将后端偏好转换为前端格式
     */
    convertPreferencesFromBackend(preferences) {
        return {
            creativity_level: preferences.creativity_level,
            allow_plot_twists: preferences.allow_plot_twists,
            response_length: preferences.response_length,
            language_style: preferences.language_style,
            notification_enabled: preferences.notification_level !== "none", // 转换格式
            theme: preferences.dark_mode ? "dark" : "light", // 转换格式
            auto_save: preferences.auto_save,
            preferred_model: preferences.preferred_model
        };
    }

    /**
     * 更新用户偏好 - 使用标准化格式
     */
    async updateUserPreferences(userId, preferences) {
        const standardPreferences = this.createStandardPreferences(preferences);

        try {
            const result = await API.updateUserPreferences(userId, standardPreferences);
            this.showSuccess('偏好设置已更新');
            return result;
        } catch (error) {
            this.showError('更新偏好设置失败: ' + error.message);
            throw error;
        }
    }

    // ========================================
    // 统计信息渲染
    // ========================================

    /**
     * 渲染用户统计信息
     */
    renderUserStats() {
        const elements = {
            'total-items': this.state.totalItems,
            'total-skills': this.state.totalSkills,
            'total-achievements': this.state.totalAchievements
        };

        Object.entries(elements).forEach(([id, value]) => {
            const element = document.getElementById(id);
            if (element) {
                element.textContent = value;
            }
        });

        // 渲染统计卡片
        this.renderStatsCards();
    }

    /**
     * 渲染统计卡片
     */
    renderStatsCards() {
        const statsContainer = document.getElementById('user-stats-container');
        if (!statsContainer) return;

        const itemsByCategory = this.groupItemsByCategory();
        const skillsByCategory = this.groupSkillsByCategory();
        const itemsByRarity = this.groupItemsByRarity();

        statsContainer.innerHTML = `
            < div class="row g-3" >
                <div class="col-6 col-md-3">
                    <div class="stat-card text-center">
                        <div class="stat-icon">📦</div>
                        <div class="stat-value">${this.state.totalItems}</div>
                        <div class="stat-label">总道具</div>
                    </div>
                </div>
                <div class="col-6 col-md-3">
                    <div class="stat-card text-center">
                        <div class="stat-icon">⚡</div>
                        <div class="stat-value">${this.state.totalSkills}</div>
                        <div class="stat-label">总技能</div>
                    </div>
                </div>
                <div class="col-6 col-md-3">
                    <div class="stat-card text-center">
                        <div class="stat-icon">🏆</div>
                        <div class="stat-value">${this.state.totalAchievements}</div>
                        <div class="stat-label">成就数</div>
                    </div>
                </div>
                <div class="col-6 col-md-3">
                    <div class="stat-card text-center">
                        <div class="stat-icon">🔥</div>
                        <div class="stat-value">${this.calculateProfileScore()}</div>
                        <div class="stat-label">档案评分</div>
                    </div>
                </div>
            </div >

        <div class="row g-3 mt-3">
            <div class="col-md-6">
                <div class="category-breakdown-card card">
                    <div class="card-header">
                        <h6 class="mb-0">道具分类统计</h6>
                    </div>
                    <div class="card-body">
                        ${this.renderCategoryBreakdown(itemsByCategory)}
                    </div>
                </div>
            </div>
            <div class="col-md-6">
                <div class="rarity-breakdown-card card">
                    <div class="card-header">
                        <h6 class="mb-0">稀有度分布</h6>
                    </div>
                    <div class="card-body">
                        ${this.renderRarityBreakdown(itemsByRarity)}
                    </div>
                </div>
            </div>
        </div>
    `;
    }

    // ========================================
    // 道具管理功能
    // ========================================

    /**
     * 渲染道具列表
     */
    renderItems() {
        const container = document.getElementById('items-container');
        if (!container) return;

        if (!this.userItems.length) {
            container.innerHTML = this.renderEmptyItemsState();
            return;
        }

        const itemsHTML = this.userItems.map(item => this.renderItemCard(item)).join('');
        container.innerHTML = `< div class="row" > ${itemsHTML}</div > `;
    }

    /**
     * 渲染空道具状态
     */
    renderEmptyItemsState() {
        return `
        < div class="empty-state text-center p-4" >
                <i class="bi bi-bag display-4 text-muted"></i>
                <h4 class="mt-3">还没有任何道具</h4>
                <p class="text-muted">开始添加您的第一个道具吧！</p>
                <button class="btn btn-primary" onclick="showAddItemModal()">
                    <i class="bi bi-plus-circle"></i> 添加第一个道具
                </button>
            </div >
        `;
    }

    /**
     * 渲染单个道具卡片
     */
    renderItemCard(item) {
        const rarityClass = this.getRarityClass(item.rarity);
        const categoryIcon = this.getCategoryIcon(item.category);

        return `
        < div class="col-md-6 col-lg-4 mb-3" >
            <div class="card item-card ${rarityClass}">
                <div class="card-header d-flex justify-content-between align-items-center">
                    <div class="item-header">
                        <span class="item-icon">${categoryIcon}</span>
                        <span class="item-name">${this.escapeHtml(item.name)}</span>
                    </div>
                    <div class="item-rarity-badge">
                        ${this.getRarityLabel(item.rarity)}
                    </div>
                </div>
                <div class="card-body">
                    <p class="item-description">${this.escapeHtml(item.description || '无描述')}</p>

                    ${item.effects && item.effects.length > 0 ? `
                            <div class="item-effects">
                                <small class="text-muted">效果:</small>
                                <ul class="effects-list">
                                    ${item.effects.map(effect => `
                                        <li class="effect-item">
                                            ${this.formatEffect(effect)}
                                        </li>
                                    `).join('')}
                                </ul>
                            </div>
                        ` : ''}

                    ${item.value !== undefined ? `
                            <div class="item-value mb-2">
                                <small class="text-muted">价值:</small>
                                <span class="fw-bold">${item.value}</span>
                            </div>
                        ` : ''}

                    <div class="item-metadata">
                        <small class="text-muted">
                            创建时间: ${this.formatDateTime(item.created_at)}
                        </small>
                    </div>
                </div>
                <div class="card-footer">
                    <div class="btn-group w-100">
                        <button class="btn btn-outline-primary btn-sm edit-item-btn"
                            data-item-id="${item.id}">
                            <i class="bi bi-pencil"></i> 编辑
                        </button>
                        <button class="btn btn-outline-danger btn-sm delete-item-btn"
                            data-item-id="${item.id}">
                            <i class="bi bi-trash"></i> 删除
                        </button>
                    </div>
                </div>
            </div>
            </div >
        `;
    }

    /**
     * 保存新道具
     */
    async saveNewItem() {
        try {
            const formData = this.getItemFormData();

            if (!this.validateItemData(formData)) {
                return;
            }

            this.setButtonLoading('.save-item-btn', true, '保存中...');

            console.log('💾 保存新道具:', formData);

            // 调用后端API添加道具
            const result = await this.safeAPICall(() =>
                API.addUserItem(this.currentUserId, formData)
            );

            if (result) {
                this.userItems.push(result);
                this.updateUserState();
                this.renderUserProfile();

                // 隐藏模态框
                this.hideModal('addItemModal');

                // 重置表单
                this.resetForm('addItemForm');

                this.showSuccess('道具添加成功！');
                console.log('✅ 道具保存成功');
            }

        } catch (error) {
            console.error('❌ 保存道具失败:', error);
            this.showError('保存道具失败: ' + error.message);
        } finally {
            this.setButtonLoading('.save-item-btn', false, '保存道具');
        }
    }

    /**
     * 编辑道具
     */
    async editItem(itemId) {
        const item = this.userItems.find(i => i.id === itemId);
        if (!item) {
            this.showError('未找到指定道具');
            return;
        }

        try {
            // 填充编辑表单
            this.populateItemForm(item);

            // 显示编辑模态框
            this.showModal('editItemModal');

            // 绑定保存事件 - 使用API方法
            this.bindEditItemSaveEvent(itemId);

        } catch (error) {
            console.error('❌ 编辑道具失败:', error);
            this.showError('编辑道具失败');
        }
    }

    /**
    * 绑定编辑道具保存事件
    */
    bindEditItemSaveEvent(itemId) {
        const saveButton = document.querySelector('.save-edit-item-btn');
        if (!saveButton) return;

        // 移除之前的事件监听器
        const newSaveButton = saveButton.cloneNode(true);
        saveButton.parentNode.replaceChild(newSaveButton, saveButton);

        // 绑定新的保存事件
        newSaveButton.addEventListener('click', async () => {
            await this.saveEditedItem(itemId);
        });
    }

    /**
    * 保存编辑的道具 - 使用API方法
    */
    async saveEditedItem(itemId) {
        try {
            const formData = this.getItemFormData();

            if (!this.validateItemData(formData)) {
                return;
            }

            this.setButtonLoading('.save-edit-item-btn', true, '保存中...');

            console.log('💾 更新道具:', itemId, formData);

            // 调用API更新道具
            const result = await this.safeAPICall(() =>
                API.updateUserItem(this.currentUserId, itemId, formData)
            );

            if (result) {
                // 更新本地数据
                const index = this.userItems.findIndex(item => item.id === itemId);
                if (index !== -1) {
                    this.userItems[index] = { ...this.userItems[index], ...result };
                }

                this.updateUserState();
                this.renderUserProfile();

                // 隐藏模态框
                this.hideModal('editItemModal');

                this.showSuccess('道具更新成功！');
                console.log('✅ 道具更新成功');
            }

        } catch (error) {
            console.error('❌ 更新道具失败:', error);
            this.showError('更新道具失败: ' + error.message);
        } finally {
            this.setButtonLoading('.save-edit-item-btn', false, '保存更改');
        }
    }

    /**
     * 删除道具
     */
    async deleteItem(itemId) {
        try {
            const confirmed = await this.safeConfirm('确定要删除这个道具吗？');
            if (!confirmed) return;

            console.log(`🗑️ 删除道具: ${itemId} `);

            // 调用后端API删除道具
            await this.safeAPICall(() => API.deleteUserItem(this.currentUserId, itemId));

            // 从本地数据中移除
            this.userItems = this.userItems.filter(item => item.id !== itemId);
            this.updateUserState();
            this.renderUserProfile();

            this.showSuccess('道具删除成功');
            console.log('✅ 道具删除成功');

        } catch (error) {
            console.error('❌ 删除道具失败:', error);
            this.showError('删除道具失败: ' + error.message);
        }
    }

    // ========================================
    // 技能管理功能
    // ========================================

    /**
     * 渲染技能列表
     */
    renderSkills() {
        const container = document.getElementById('skills-container');
        if (!container) return;

        if (!this.userSkills.length) {
            container.innerHTML = this.renderEmptySkillsState();
            return;
        }

        const skillsHTML = this.userSkills.map(skill => this.renderSkillCard(skill)).join('');
        container.innerHTML = `< div class="row" > ${skillsHTML}</div > `;
    }

    /**
     * 渲染空技能状态
     */
    renderEmptySkillsState() {
        return `
        < div class="empty-state text-center p-4" >
                <i class="bi bi-lightning display-4 text-muted"></i>
                <h4 class="mt-3">还没有任何技能</h4>
                <p class="text-muted">开始添加您的第一个技能吧！</p>
                <button class="btn btn-primary" onclick="showAddSkillModal()">
                    <i class="bi bi-plus-circle"></i> 添加第一个技能
                </button>
            </div >
        `;
    }

    /**
     * 渲染单个技能卡片
     */
    renderSkillCard(skill) {
        const categoryIcon = this.getSkillCategoryIcon(skill.category);

        return `
        < div class="col-md-6 col-lg-4 mb-3" >
            <div class="card skill-card">
                <div class="card-header d-flex justify-content-between align-items-center">
                    <div class="skill-header">
                        <span class="skill-icon">${categoryIcon}</span>
                        <span class="skill-name">${this.escapeHtml(skill.name)}</span>
                    </div>
                    <div class="skill-category-badge">
                        ${this.getSkillCategoryLabel(skill.category)}
                    </div>
                </div>
                <div class="card-body">
                    <p class="skill-description">${this.escapeHtml(skill.description || '无描述')}</p>

                    ${skill.effects && skill.effects.length > 0 ? `
                            <div class="skill-effects">
                                <small class="text-muted">效果:</small>
                                <ul class="effects-list">
                                    ${skill.effects.map(effect => `
                                        <li class="effect-item">
                                            ${this.formatEffect(effect)}
                                        </li>
                                    `).join('')}
                                </ul>
                            </div>
                        ` : ''}

                    <div class="skill-stats">
                        <div class="row text-center">
                            ${skill.cooldown ? `
                                    <div class="col-6">
                                        <div class="stat-value">${skill.cooldown}s</div>
                                        <div class="stat-label">冷却时间</div>
                                    </div>
                                ` : ''}
                            ${skill.mana_cost ? `
                                    <div class="col-6">
                                        <div class="stat-value">${skill.mana_cost}</div>
                                        <div class="stat-label">法力消耗</div>
                                    </div>
                                ` : ''}
                        </div>
                    </div>

                    <div class="skill-metadata">
                        <small class="text-muted">
                            创建时间: ${this.formatDateTime(skill.created_at)}
                        </small>
                    </div>
                </div>
                <div class="card-footer">
                    <div class="btn-group w-100">
                        <button class="btn btn-outline-primary btn-sm edit-skill-btn"
                            data-skill-id="${skill.id}">
                            <i class="bi bi-pencil"></i> 编辑
                        </button>
                        <button class="btn btn-outline-danger btn-sm delete-skill-btn"
                            data-skill-id="${skill.id}">
                            <i class="bi bi-trash"></i> 删除
                        </button>
                    </div>
                </div>
            </div>
            </div >
        `;
    }

    /**
     * 保存新技能
     */
    async saveNewSkill() {
        try {
            const formData = this.getSkillFormData();

            if (!this.validateSkillData(formData)) {
                return;
            }

            this.setButtonLoading('.save-skill-btn', true, '保存中...');

            console.log('💾 保存新技能:', formData);

            // 调用后端API添加技能
            const result = await this.safeAPICall(() =>
                API.addUserSkill(this.currentUserId, formData)
            );

            if (result) {
                this.userSkills.push(result);
                this.updateUserState();
                this.renderUserProfile();

                // 隐藏模态框
                this.hideModal('addSkillModal');

                // 重置表单
                this.resetForm('addSkillForm');

                this.showSuccess('技能添加成功！');
                console.log('✅ 技能保存成功');
            }

        } catch (error) {
            console.error('❌ 保存技能失败:', error);
            this.showError('保存技能失败: ' + error.message);
        } finally {
            this.setButtonLoading('.save-skill-btn', false, '保存技能');
        }
    }

    /**
     * 编辑技能
     */
    async editSkill(skillId) {
        const skill = this.userSkills.find(s => s.id === skillId);
        if (!skill) {
            this.showError('未找到指定技能');
            return;
        }

        try {
            // 填充编辑表单
            this.populateSkillForm(skill);

            // 显示编辑模态框
            this.showModal('editSkillModal');

        } catch (error) {
            console.error('❌ 编辑技能失败:', error);
            this.showError('编辑技能失败');
        }
    }

    /**
     * 删除技能
     */
    async deleteSkill(skillId) {
        try {
            const confirmed = await this.safeConfirm('确定要删除这个技能吗？');
            if (!confirmed) return;

            console.log(`🗑️ 删除技能: ${skillId} `);

            // 调用后端API删除技能
            await this.safeAPICall(() => API.deleteUserSkill(this.currentUserId, skillId));

            // 从本地数据中移除
            this.userSkills = this.userSkills.filter(skill => skill.id !== skillId);
            this.updateUserState();
            this.renderUserProfile();

            this.showSuccess('技能删除成功');
            console.log('✅ 技能删除成功');

        } catch (error) {
            console.error('❌ 删除技能失败:', error);
            this.showError('删除技能失败: ' + error.message);
        }
    }

    // ========================================
    // 成就系统
    // ========================================

    /**
     * 渲染成就系统
     */
    renderAchievements() {
        const container = document.getElementById('achievements-container');
        if (!container) return;

        // 暂时显示静态内容，后续可以从后端获取成就数据
        container.innerHTML = `
        < div class="achievements-placeholder text-center p-4" >
                <i class="bi bi-trophy display-4 text-muted"></i>
                <h4 class="mt-3">成就系统</h4>
                <p class="text-muted">成就功能正在开发中，敬请期待！</p>
            </div >
        `;
    }

    // ========================================
    // 表单处理功能
    // ========================================

    /**
     * 获取道具表单数据
     */
    getItemFormData() {
        const effects = [];
        document.querySelectorAll('#itemEffects .effect-item').forEach(row => {
            const target = row.querySelector('.effect-target')?.value;
            const type = row.querySelector('.effect-type')?.value;
            const value = parseFloat(row.querySelector('.effect-value')?.value);
            const probability = parseFloat(row.querySelector('.effect-probability')?.value);

            if (target && type && !isNaN(value) && !isNaN(probability)) {
                effects.push({ target, type, value, probability });
            }
        });

        return {
            name: document.getElementById('itemName')?.value || '',
            description: document.getElementById('itemDescription')?.value || '',
            category: document.getElementById('itemCategory')?.value || '',
            rarity: document.getElementById('itemRarity')?.value || '',
            value: parseFloat(document.getElementById('itemValue')?.value) || 0,
            effects: effects
        };
    }

    /**
     * 获取技能表单数据
     */
    getSkillFormData() {
        const effects = [];
        document.querySelectorAll('#skillEffects .effect-item').forEach(row => {
            const target = row.querySelector('.effect-target')?.value;
            const type = row.querySelector('.effect-type')?.value;
            const value = parseFloat(row.querySelector('.effect-value')?.value);
            const probability = parseFloat(row.querySelector('.effect-probability')?.value);

            if (target && type && !isNaN(value) && !isNaN(probability)) {
                effects.push({ target, type, value, probability });
            }
        });

        return {
            name: document.getElementById('skillName')?.value || '',
            description: document.getElementById('skillDescription')?.value || '',
            category: document.getElementById('skillCategory')?.value || '',
            cooldown: parseInt(document.getElementById('skillCooldown')?.value) || 0,
            mana_cost: parseInt(document.getElementById('skillManaCost')?.value) || 0,
            effects: effects
        };
    }

    /**
     * 验证道具数据
     */
    validateItemData(data) {
        if (!data.name.trim()) {
            this.showError('请输入道具名称');
            return false;
        }

        if (!data.category) {
            this.showError('请选择道具类别');
            return false;
        }

        if (!data.rarity) {
            this.showError('请选择稀有度');
            return false;
        }

        return true;
    }

    /**
     * 验证技能数据
     */
    validateSkillData(data) {
        if (!data.name.trim()) {
            this.showError('请输入技能名称');
            return false;
        }

        if (!data.category) {
            this.showError('请选择技能类别');
            return false;
        }

        return true;
    }

    /**
     * 填充道具表单
     */
    populateItemForm(item) {
        const elements = {
            'itemName': item.name,
            'itemDescription': item.description,
            'itemCategory': item.category,
            'itemRarity': item.rarity,
            'itemValue': item.value
        };

        Object.entries(elements).forEach(([id, value]) => {
            const element = document.getElementById(id);
            if (element && value !== undefined) {
                element.value = value;
            }
        });

        // 填充效果列表
        this.populateEffects('itemEffects', item.effects || []);
    }

    /**
     * 填充技能表单
     */
    populateSkillForm(skill) {
        const elements = {
            'skillName': skill.name,
            'skillDescription': skill.description,
            'skillCategory': skill.category,
            'skillCooldown': skill.cooldown,
            'skillManaCost': skill.mana_cost
        };

        Object.entries(elements).forEach(([id, value]) => {
            const element = document.getElementById(id);
            if (element && value !== undefined) {
                element.value = value;
            }
        });

        // 填充效果列表
        this.populateEffects('skillEffects', skill.effects || []);
    }

    /**
     * 填充效果列表
     */
    populateEffects(containerId, effects) {
        const container = document.getElementById(containerId);
        if (!container) return;

        container.innerHTML = '';

        effects.forEach(effect => {
            this.addEffectRow(containerId, effect);
        });

        // 如果没有效果，添加一个空行
        if (effects.length === 0) {
            this.addEffectRow(containerId);
        }
    }

    /**
     * 添加效果行
     */
    addEffectRow(containerId = 'itemEffects', effect = null) {
        const container = document.getElementById(containerId);
        if (!container) return;

        const row = document.createElement('div');
        row.className = 'effect-item row g-2 mb-2';
        row.innerHTML = `
        < div class="col-3" >
            <select class="form-select form-select-sm effect-target" required>
                <option value="">选择目标</option>
                <option value="self" ${effect?.target === 'self' ? 'selected' : ''}>自己</option>
                <option value="other" ${effect?.target === 'other' ? 'selected' : ''}>其他角色</option>
                <option value="scene" ${effect?.target === 'scene' ? 'selected' : ''}>场景</option>
            </select>
            </div >
            <div class="col-3">
                <select class="form-select form-select-sm effect-type" required>
                    <option value="">选择类型</option>
                    <option value="health" ${effect?.type === 'health' ? 'selected' : ''}>生命值</option>
                    <option value="mana" ${effect?.type === 'mana' ? 'selected' : ''}>法力值</option>
                    <option value="attack" ${effect?.type === 'attack' ? 'selected' : ''}>攻击力</option>
                    <option value="defense" ${effect?.type === 'defense' ? 'selected' : ''}>防御力</option>
                    <option value="speed" ${effect?.type === 'speed' ? 'selected' : ''}>速度</option>
                    <option value="luck" ${effect?.type === 'luck' ? 'selected' : ''}>幸运值</option>
                </select>
            </div>
            <div class="col-2">
                <input type="number" class="form-control form-control-sm effect-value" 
                       placeholder="数值" value="${effect?.value || ''}" required>
            </div>
            <div class="col-3">
                <input type="number" class="form-control form-control-sm effect-probability" 
                       placeholder="概率(0-1)" min="0" max="1" step="0.1" 
                       value="${effect?.probability || 1}" required>
            </div>
            <div class="col-1">
                <button type="button" class="btn btn-outline-danger btn-sm remove-effect-btn">
                    <i class="bi bi-x"></i>
                </button>
            </div>
    `;

        container.appendChild(row);
    }

    /**
     * 移除效果行
     */
    removeEffectRow(button) {
        const row = button.closest('.effect-item');
        if (row) {
            row.remove();
        }
    }

    // ========================================
    // 筛选功能
    // ========================================

    /**
     * 筛选道具
     */
    filterItems() {
        const categoryFilter = document.getElementById('item-category-filter')?.value || '';
        const rarityFilter = document.getElementById('item-rarity-filter')?.value || '';
        const searchFilter = document.getElementById('item-search')?.value.toLowerCase() || '';

        this.filteredItems = this.userItems.filter(item => {
            const categoryMatch = !categoryFilter || item.category === categoryFilter;
            const rarityMatch = !rarityFilter || item.rarity === rarityFilter;
            const searchMatch = !searchFilter ||
                item.name.toLowerCase().includes(searchFilter) ||
                (item.description && item.description.toLowerCase().includes(searchFilter));

            return categoryMatch && rarityMatch && searchMatch;
        });

        this.renderFilteredItems();
    }

    /**
     * 筛选技能
     */
    filterSkills() {
        const categoryFilter = document.getElementById('skill-category-filter')?.value || '';
        const searchFilter = document.getElementById('skill-search')?.value.toLowerCase() || '';

        this.filteredSkills = this.userSkills.filter(skill => {
            const categoryMatch = !categoryFilter || skill.category === categoryFilter;
            const searchMatch = !searchFilter ||
                skill.name.toLowerCase().includes(searchFilter) ||
                (skill.description && skill.description.toLowerCase().includes(searchFilter));

            return categoryMatch && searchMatch;
        });

        this.renderFilteredSkills();
    }

    /**
     * 渲染筛选后的道具
     */
    renderFilteredItems() {
        const container = document.getElementById('items-container');
        if (!container) return;

        const itemsToRender = this.filteredItems.length > 0 ? this.filteredItems : this.userItems;

        if (!itemsToRender.length) {
            container.innerHTML = `
        < div class="no-results text-center p-4" >
                    <i class="bi bi-search display-4 text-muted"></i>
                    <h4 class="mt-3">没有找到匹配的道具</h4>
                    <p class="text-muted">尝试调整筛选条件</p>
                </div >
        `;
            return;
        }

        const itemsHTML = itemsToRender.map(item => this.renderItemCard(item)).join('');
        container.innerHTML = `< div class="row" > ${itemsHTML}</div > `;
    }

    /**
     * 渲染筛选后的技能
     */
    renderFilteredSkills() {
        const container = document.getElementById('skills-container');
        if (!container) return;

        const skillsToRender = this.filteredSkills.length > 0 ? this.filteredSkills : this.userSkills;

        if (!skillsToRender.length) {
            container.innerHTML = `
        < div class="no-results text-center p-4" >
                    <i class="bi bi-search display-4 text-muted"></i>
                    <h4 class="mt-3">没有找到匹配的技能</h4>
                    <p class="text-muted">尝试调整筛选条件</p>
                </div >
        `;
            return;
        }

        const skillsHTML = skillsToRender.map(skill => this.renderSkillCard(skill)).join('');
        container.innerHTML = `< div class="row" > ${skillsHTML}</div > `;
    }

    // ========================================
    // 数据导出功能
    // ========================================

    /**
     * 导出用户数据
     */
    async exportUserData() {
        try {
            const userData = {
                user_id: this.currentUserId,
                items: this.userItems,
                skills: this.userSkills,
                achievements: this.userAchievements,
                exported_at: new Date().toISOString(),
                export_version: '1.0'
            };

            const blob = new Blob([JSON.stringify(userData, null, 2)], {
                type: 'application/json'
            });

            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = `user_profile_${this.currentUserId}_${Date.now()}.json`;
            link.style.display = 'none';

            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);

            setTimeout(() => URL.revokeObjectURL(url), 1000);

            this.showSuccess('用户数据导出成功');
            console.log('✅ 用户数据导出成功');

        } catch (error) {
            console.error('❌ 导出用户数据失败:', error);
            this.showError('导出失败: ' + error.message);
        }
    }

    // ========================================
    // 辅助计算功能
    // ========================================

    /**
     * 计算档案评分
     */
    calculateProfileScore() {
        let score = 0;

        // 道具评分
        score += this.userItems.length * 10;
        score += this.userItems.filter(i => i.rarity === 'rare').length * 20;
        score += this.userItems.filter(i => i.rarity === 'epic').length * 50;
        score += this.userItems.filter(i => i.rarity === 'legendary').length * 100;

        // 技能评分
        score += this.userSkills.length * 15;

        // 成就评分
        score += this.userAchievements.length * 25;

        return Math.min(score, 9999); // 最大9999分
    }

    /**
     * 按类别分组道具
     */
    groupItemsByCategory() {
        const groups = {};
        this.userItems.forEach(item => {
            const category = item.category || 'other';
            groups[category] = (groups[category] || 0) + 1;
        });
        return groups;
    }

    /**
     * 按类别分组技能
     */
    groupSkillsByCategory() {
        const groups = {};
        this.userSkills.forEach(skill => {
            const category = skill.category || 'other';
            groups[category] = (groups[category] || 0) + 1;
        });
        return groups;
    }

    /**
     * 按稀有度分组道具
     */
    groupItemsByRarity() {
        const groups = {};
        this.userItems.forEach(item => {
            const rarity = item.rarity || 'common';
            groups[rarity] = (groups[rarity] || 0) + 1;
        });
        return groups;
    }

    /**
     * 渲染类别分布
     */
    renderCategoryBreakdown(categoryData) {
        const total = Object.values(categoryData).reduce((sum, count) => sum + count, 0);

        if (total === 0) {
            return '<p class="text-muted text-center">暂无数据</p>';
        }

        return Object.entries(categoryData).map(([category, count]) => {
            const percentage = Math.round((count / total) * 100);
            const label = this.getCategoryLabel(category);

            return `
        < div class="category-item d-flex justify-content-between align-items-center mb-2" >
                    <div class="category-info">
                        <span class="category-icon">${this.getCategoryIcon(category)}</span>
                        <span class="category-name">${label}</span>
                    </div>
                    <div class="category-stats">
                        <span class="badge bg-primary">${count}</span>
                        <small class="text-muted ms-1">${percentage}%</small>
                    </div>
                </div >
        `;
        }).join('');
    }

    /**
     * 渲染稀有度分布
     */
    renderRarityBreakdown(rarityData) {
        const total = Object.values(rarityData).reduce((sum, count) => sum + count, 0);

        if (total === 0) {
            return '<p class="text-muted text-center">暂无数据</p>';
        }

        const rarityOrder = ['common', 'uncommon', 'rare', 'epic', 'legendary'];

        return rarityOrder.map(rarity => {
            const count = rarityData[rarity] || 0;
            if (count === 0) return '';

            const percentage = Math.round((count / total) * 100);
            const label = this.getRarityLabel(rarity);
            const colorClass = this.getRarityColorClass(rarity);

            return `
        < div class="rarity-item d-flex justify-content-between align-items-center mb-2" >
                    <div class="rarity-info">
                        <span class="rarity-dot ${colorClass}"></span>
                        <span class="rarity-name">${label}</span>
                    </div>
                    <div class="rarity-stats">
                        <span class="badge bg-secondary">${count}</span>
                        <small class="text-muted ms-1">${percentage}%</small>
                    </div>
                </div >
        `;
        }).filter(Boolean).join('');
    }

    // ========================================
    // 界面辅助功能
    // ========================================

    /**
     * 设置加载状态
     */
    setLoading(isLoading) {
        this.isLoading = isLoading;

        // 更新加载指示器
        const loadingIndicator = document.getElementById('profile-loading');
        if (loadingIndicator) {
            loadingIndicator.style.display = isLoading ? 'block' : 'none';
        }

        // 禁用/启用操作按钮
        const buttons = document.querySelectorAll('.save-item-btn, .save-skill-btn, .export-data-btn');
        buttons.forEach(button => {
            button.disabled = isLoading;
        });
    }

    /**
     * 设置按钮加载状态
     */
    setButtonLoading(selector, isLoading, loadingText = '处理中...') {
        const button = document.querySelector(selector);
        if (!button) return;

        if (isLoading) {
            button.dataset.originalText = button.innerHTML;
            button.innerHTML = `
        < div class="spinner-border spinner-border-sm me-2" role = "status" aria - hidden="true" ></div >
            ${loadingText}
    `;
            button.disabled = true;
        } else {
            if (button.dataset.originalText) {
                button.innerHTML = button.dataset.originalText;
                delete button.dataset.originalText;
            }
            button.disabled = false;
        }
    }

    /**
     * 显示模态框
     */
    showModal(modalId) {
        const modal = document.getElementById(modalId);
        if (!modal) return;

        if (UserProfile.isBootstrapAvailable()) {
            const bsModal = new bootstrap.Modal(modal);
            bsModal.show();
        } else {
            // 降级处理
            modal.style.display = 'block';
            modal.classList.add('show');
        }
    }

    /**
     * 隐藏模态框
     */
    hideModal(modalId) {
        const modal = document.getElementById(modalId);
        if (!modal) return;

        if (UserProfile.isBootstrapAvailable()) {
            const bsModal = bootstrap.Modal.getInstance(modal);
            if (bsModal) {
                bsModal.hide();
            }
        } else {
            // 降级处理
            modal.style.display = 'none';
            modal.classList.remove('show');
        }
    }

    /**
     * 重置表单
     */
    resetForm(formId) {
        const form = document.getElementById(formId);
        if (form) {
            form.reset();

            // 清空效果列表
            const effectsContainer = form.querySelector('.effects-container');
            if (effectsContainer) {
                effectsContainer.innerHTML = '';
            }
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
     * 安全调用确认对话框
     */
    async safeConfirm(message, options = {}) {
        if (typeof Utils !== 'undefined' && typeof Utils.showConfirm === 'function') {
            return await Utils.showConfirm(message, {
                title: '确认操作',
                confirmText: '确认',
                cancelText: '取消',
                type: 'warning',
                ...options
            });
        }

        // 降级到原生confirm
        return confirm(message);
    }

    // ========================================
    // 工具方法
    // ========================================

    /**
     * HTML转义
     */
    escapeHtml(text) {
        if (typeof text !== 'string') return '';

        // 如果Utils可用，使用其方法
        if (typeof Utils !== 'undefined' && typeof Utils.escapeHtml === 'function') {
            return Utils.escapeHtml(text);
        }

        // 降级方法
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /**
     * 格式化日期时间
     */
    formatDateTime(timestamp) {
        // 如果Utils可用，使用其方法
        if (typeof Utils !== 'undefined' && typeof Utils.formatDateTime === 'function') {
            return Utils.formatDateTime(timestamp);
        }

        // 降级方法
        const date = new Date(timestamp);
        return date.toLocaleString('zh-CN');
    }

    /**
     * 获取稀有度样式类
     */
    getRarityClass(rarity) {
        const classes = {
            'common': 'rarity-common',
            'uncommon': 'rarity-uncommon',
            'rare': 'rarity-rare',
            'epic': 'rarity-epic',
            'legendary': 'rarity-legendary'
        };
        return classes[rarity] || 'rarity-common';
    }

    /**
     * 获取稀有度颜色类
     */
    getRarityColorClass(rarity) {
        const classes = {
            'common': 'bg-secondary',
            'uncommon': 'bg-success',
            'rare': 'bg-primary',
            'epic': 'bg-warning',
            'legendary': 'bg-danger'
        };
        return classes[rarity] || 'bg-secondary';
    }

    /**
     * 获取稀有度标签
     */
    getRarityLabel(rarity) {
        const labels = {
            'common': '普通',
            'uncommon': '不常见',
            'rare': '稀有',
            'epic': '史诗',
            'legendary': '传说'
        };
        return labels[rarity] || rarity;
    }

    /**
     * 获取类别图标
     */
    getCategoryIcon(category) {
        const icons = {
            'weapon': '⚔️',
            'armor': '🛡️',
            'accessory': '💍',
            'consumable': '🧪',
            'tool': '🔧',
            'material': '🔩',
            'quest': '📜',
            'other': '📦'
        };
        return icons[category] || '📦';
    }

    /**
     * 获取类别标签
     */
    getCategoryLabel(category) {
        const labels = {
            'weapon': '武器',
            'armor': '护甲',
            'accessory': '饰品',
            'consumable': '消耗品',
            'tool': '工具',
            'material': '材料',
            'quest': '任务',
            'other': '其他'
        };
        return labels[category] || category;
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
            'mental': '精神',
            'physical': '体能',
            'crafting': '制作',
            'survival': '生存',
            'other': '其他'
        };
        return labels[category] || category;
    }

    /**
     * 格式化效果描述
     */
    formatEffect(effect) {
        const targetLabel = effect.target === 'self' ? '自己' :
            effect.target === 'other' ? '其他角色' :
                effect.target === 'scene' ? '场景' : '目标';
        const sign = effect.value > 0 ? '+' : '';
        const probability = effect.probability < 1 ? ` (${Math.round(effect.probability * 100)} % 几率)` : '';

        return `${targetLabel} ${effect.type} ${sign}${effect.value}${probability} `;
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
     * 获取当前用户状态
     */
    getUserState() {
        return {
            ...this.state,
            currentUserId: this.currentUserId,
            userItems: this.userItems,
            userSkills: this.userSkills,
            userAchievements: this.userAchievements,
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
     * 销毁用户档案管理器
     */
    destroy() {
        // 移除事件监听器
        this.eventListeners.forEach((handler, key) => {
            const [eventType] = key.split('-');
            document.removeEventListener(eventType, handler);
        });
        this.eventListeners.clear();

        // 清理数据
        this.currentUserId = null;
        this.userItems = [];
        this.userSkills = [];
        this.userAchievements = [];
        this.filteredItems = [];
        this.filteredSkills = [];

        console.log('🗑️ UserProfile 已销毁');
    }

    // ========================================
    // 用户档案模态框
    // ========================================

    /**
     * 显示编辑用户档案模态框
     */
    showEditProfileModal(userId) {
        // 验证用户ID
        if (!userId) {
            this.showError('用户ID不能为空');
            return;
        }

        // 设置当前用户ID
        this.currentUserId = userId;

        // 首先加载用户数据
        this.loadUserData(userId).then(() => {
            this.createEditProfileModal();
        }).catch(error => {
            console.error('加载用户数据失败:', error);
            this.showError('加载用户数据失败: ' + error.message);
        });
    }

    /**
     * 创建编辑用户档案模态框
     */
    createEditProfileModal() {
        // 移除已存在的模态框
        const existingModal = document.getElementById('editProfileModal');
        if (existingModal) {
            existingModal.remove();
        }

        const modal = document.createElement('div');
        modal.id = 'editProfileModal';
        modal.className = 'modal fade';
        modal.innerHTML = `
        <div class="modal-dialog modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-person-gear"></i> 编辑用户档案
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <form id="editProfileForm">
                        <div class="row g-3">
                            <!-- 基本信息 -->
                            <div class="col-12">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-person-badge"></i> 基本信息
                                </h6>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editUsername" class="form-label">用户名 *</label>
                                <input type="text" class="form-control" id="editUsername" 
                                       value="${this.escapeHtml(this.userData?.username || '')}" required>
                                <div class="form-text">用于登录的唯一标识</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editDisplayName" class="form-label">显示名称 *</label>
                                <input type="text" class="form-control" id="editDisplayName" 
                                       value="${this.escapeHtml(this.userData?.display_name || '')}" required>
                                <div class="form-text">在界面中显示的名称</div>
                            </div>
                            
                            <div class="col-12">
                                <label for="editBio" class="form-label">个人简介</label>
                                <textarea class="form-control" id="editBio" rows="3" 
                                          placeholder="介绍一下自己...">${this.escapeHtml(this.userData?.bio || '')}</textarea>
                                <div class="form-text">简单介绍您的背景或兴趣</div>
                            </div>
                            
                            <div class="col-12">
                                <label for="editAvatar" class="form-label">头像URL</label>
                                <input type="url" class="form-control" id="editAvatar" 
                                       value="${this.escapeHtml(this.userData?.avatar || '')}" 
                                       placeholder="https://example.com/avatar.jpg">
                                <div class="form-text">支持 JPG、PNG 格式的图片链接</div>
                            </div>

                            <!-- 偏好设置 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-gear"></i> 偏好设置
                                </h6>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editCreativityLevel" class="form-label">创意等级</label>
                                <select class="form-select" id="editCreativityLevel">
                                    <option value="STRICT" ${this.userData?.preferences?.creativity_level === 'STRICT' ? 'selected' : ''}>严格模式</option>
                                    <option value="BALANCED" ${this.userData?.preferences?.creativity_level === 'BALANCED' ? 'selected' : ''}>平衡模式</option>
                                    <option value="EXPANSIVE" ${this.userData?.preferences?.creativity_level === 'EXPANSIVE' ? 'selected' : ''}>扩展模式</option>
                                </select>
                                <div class="form-text">控制AI回应的创意程度</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editResponseLength" class="form-label">响应长度</label>
                                <select class="form-select" id="editResponseLength">
                                    <option value="short" ${this.userData?.preferences?.response_length === 'short' ? 'selected' : ''}>简短</option>
                                    <option value="medium" ${this.userData?.preferences?.response_length === 'medium' ? 'selected' : ''}>中等</option>
                                    <option value="long" ${this.userData?.preferences?.response_length === 'long' ? 'selected' : ''}>详细</option>
                                </select>
                                <div class="form-text">AI回应的详细程度</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editLanguageStyle" class="form-label">语言风格</label>
                                <select class="form-select" id="editLanguageStyle">
                                    <option value="formal" ${this.userData?.preferences?.language_style === 'formal' ? 'selected' : ''}>正式</option>
                                    <option value="casual" ${this.userData?.preferences?.language_style === 'casual' ? 'selected' : ''}>随意</option>
                                    <option value="literary" ${this.userData?.preferences?.language_style === 'literary' ? 'selected' : ''}>文学</option>
                                </select>
                                <div class="form-text">AI使用的语言风格</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editNotificationLevel" class="form-label">通知级别</label>
                                <select class="form-select" id="editNotificationLevel">
                                    <option value="all" ${this.userData?.preferences?.notification_level === 'all' ? 'selected' : ''}>全部通知</option>
                                    <option value="important" ${this.userData?.preferences?.notification_level === 'important' ? 'selected' : ''}>重要通知</option>
                                    <option value="none" ${this.userData?.preferences?.notification_level === 'none' ? 'selected' : ''}>不通知</option>
                                </select>
                                <div class="form-text">接收通知的级别</div>
                            </div>
                            
                            <div class="col-12">
                                <div class="form-check form-switch">
                                    <input class="form-check-input" type="checkbox" id="editDarkMode" 
                                           ${this.userData?.preferences?.dark_mode ? 'checked' : ''}>
                                    <label class="form-check-label" for="editDarkMode">
                                        深色模式
                                    </label>
                                    <div class="form-text">启用深色主题界面</div>
                                </div>
                            </div>
                            
                            <div class="col-12">
                                <div class="form-check form-switch">
                                    <input class="form-check-input" type="checkbox" id="editAllowPlotTwists" 
                                           ${this.userData?.preferences?.allow_plot_twists ? 'checked' : ''}>
                                    <label class="form-check-label" for="editAllowPlotTwists">
                                        允许剧情转折
                                    </label>
                                    <div class="form-text">允许AI在故事中加入意外转折</div>
                                </div>
                            </div>
                        </div>
                    </form>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">
                        <i class="bi bi-x-circle"></i> 取消
                    </button>
                    <button type="button" class="btn btn-primary save-profile-btn" onclick="userProfile.saveEditedProfile()">
                        <i class="bi bi-check-circle"></i> 保存更改
                    </button>
                </div>
            </div>
        </div>
    `;

        document.body.appendChild(modal);

        // 绑定表单验证事件
        this.bindEditProfileEvents();

        // 显示模态框
        this.showModal(modal);
    }

    /**
     * 绑定编辑档案事件
     */
    bindEditProfileEvents() {
        const form = document.getElementById('editProfileForm');
        if (!form) return;

        // 实时验证
        form.addEventListener('input', (e) => {
            this.validateProfileField(e.target);
        });

        // 头像URL预览
        const avatarInput = document.getElementById('editAvatar');
        if (avatarInput) {
            avatarInput.addEventListener('blur', () => {
                this.previewAvatar(avatarInput.value);
            });
        }
    }

    /**
     * 验证档案字段
     */
    validateProfileField(field) {
        const value = field.value.trim();
        let isValid = true;
        let message = '';

        switch (field.id) {
            case 'editUsername':
                isValid = value.length >= 3 && /^[a-zA-Z0-9_]+$/.test(value);
                message = isValid ? '' : '用户名至少3个字符，只能包含字母、数字和下划线';
                break;
            case 'editDisplayName':
                isValid = value.length >= 2;
                message = isValid ? '' : '显示名称至少2个字符';
                break;
            case 'editAvatar':
                if (value) {
                    try {
                        new URL(value);
                        isValid = /\.(jpg|jpeg|png|gif)$/i.test(value);
                        message = isValid ? '' : '请提供有效的图片URL';
                    } catch {
                        isValid = false;
                        message = '请提供有效的URL格式';
                    }
                }
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
     * 预览头像
     */
    previewAvatar(url) {
        if (!url) return;

        // 创建预览图片
        const img = new Image();
        img.onload = () => {
            // 显示预览成功提示
            this.showSuccess('头像预览加载成功');
        };
        img.onerror = () => {
            this.showError('无法加载头像图片，请检查URL是否正确');
        };
        img.src = url;
    }

    /**
     * 保存编辑的档案
     */
    async saveEditedProfile() {
        try {
            const form = document.getElementById('editProfileForm');
            if (!form) {
                throw new Error('找不到编辑表单');
            }

            // 验证所有必填字段
            const requiredFields = ['editUsername', 'editDisplayName'];
            let isValid = true;

            for (const fieldId of requiredFields) {
                const field = document.getElementById(fieldId);
                if (!this.validateProfileField(field)) {
                    isValid = false;
                }
            }

            if (!isValid) {
                this.showError('请修正表单中的错误');
                return;
            }

            // 禁用保存按钮
            this.setButtonLoading('.save-profile-btn', true, '保存中...');

            // 收集表单数据
            const profileData = {
                username: document.getElementById('editUsername').value.trim(),
                display_name: document.getElementById('editDisplayName').value.trim(),
                bio: document.getElementById('editBio').value.trim(),
                avatar: document.getElementById('editAvatar').value.trim(),
                preferences: {
                    creativity_level: document.getElementById('editCreativityLevel').value,
                    response_length: document.getElementById('editResponseLength').value,
                    language_style: document.getElementById('editLanguageStyle').value,
                    notification_level: document.getElementById('editNotificationLevel').value,
                    dark_mode: document.getElementById('editDarkMode').checked,
                    allow_plot_twists: document.getElementById('editAllowPlotTwists').checked
                }
            };

            console.log('💾 保存用户档案:', profileData);

            // 调用API保存档案
            const result = await this.safeAPICall(() =>
                API.updateUserProfile(this.currentUserId, profileData)
            );

            if (result) {
                // 更新本地数据
                this.userData = { ...this.userData, ...result };

                // 重新渲染用户界面
                this.renderUserProfile();

                // 隐藏模态框
                this.hideModal('editProfileModal');

                // 显示成功消息
                this.showSuccess('用户档案更新成功！');

                // 如果应用了深色模式设置，立即应用
                if (profileData.preferences.dark_mode !== this.userData?.preferences?.dark_mode) {
                    this.applyThemeMode(profileData.preferences.dark_mode);
                }
            }

        } catch (error) {
            console.error('❌ 保存用户档案失败:', error);
            this.showError('保存失败: ' + error.message);
        } finally {
            // 恢复保存按钮
            this.setButtonLoading('.save-profile-btn', false, '保存更改');
        }
    }

    /**
     * 应用主题模式
     */
    applyThemeMode(darkMode) {
        if (darkMode) {
            document.body.classList.add('dark-theme');
            document.documentElement.setAttribute('data-theme', 'dark');
        } else {
            document.body.classList.remove('dark-theme');
            document.documentElement.setAttribute('data-theme', 'light');
        }

        // 保存到本地存储
        localStorage.setItem('theme-mode', darkMode ? 'dark' : 'light');
    }

    /**
     * 显示添加道具模态框 (增强版)
     */
    showAddItemModal() {
        if (!this.currentUserId) {
            this.showError('请先选择用户');
            return;
        }

        this.createAddItemModal();
    }

    /**
     * 显示添加技能模态框 (增强版)
     */
    showAddSkillModal() {
        if (!this.currentUserId) {
            this.showError('请先选择用户');
            return;
        }

        this.createAddSkillModal();
    }

    /**
     * 创建添加道具模态框
     */
    createAddItemModal() {
        // 移除已存在的模态框
        const existingModal = document.getElementById('addItemModal');
        if (existingModal) {
            existingModal.remove();
        }

        const modal = document.createElement('div');
        modal.id = 'addItemModal';
        modal.className = 'modal fade';
        modal.innerHTML = `
        <div class="modal-dialog modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-plus-circle"></i> 添加新道具
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <form id="addItemForm">
                        <div class="row g-3">
                            <!-- 基本信息 -->
                            <div class="col-12">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-info-circle"></i> 基本信息
                                </h6>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="itemName" class="form-label">道具名称 *</label>
                                <input type="text" class="form-control" id="itemName" 
                                       placeholder="输入道具名称" required>
                                <div class="form-text">道具的唯一标识名称</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="itemCategory" class="form-label">道具类别 *</label>
                                <select class="form-select" id="itemCategory" required>
                                    <option value="">选择类别</option>
                                    <option value="weapon">武器</option>
                                    <option value="armor">护甲</option>
                                    <option value="accessory">饰品</option>
                                    <option value="consumable">消耗品</option>
                                    <option value="tool">工具</option>
                                    <option value="material">材料</option>
                                    <option value="quest">任务物品</option>
                                    <option value="other">其他</option>
                                </select>
                                <div class="form-text">道具的分类类型</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="itemRarity" class="form-label">稀有度 *</label>
                                <select class="form-select" id="itemRarity" required>
                                    <option value="">选择稀有度</option>
                                    <option value="common">普通</option>
                                    <option value="uncommon">不常见</option>
                                    <option value="rare">稀有</option>
                                    <option value="epic">史诗</option>
                                    <option value="legendary">传说</option>
                                </select>
                                <div class="form-text">道具的稀有程度</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="itemValue" class="form-label">道具价值</label>
                                <input type="number" class="form-control" id="itemValue" 
                                       placeholder="0" min="0" step="0.01">
                                <div class="form-text">道具的经济价值（可选）</div>
                            </div>
                            
                            <div class="col-12">
                                <label for="itemDescription" class="form-label">道具描述</label>
                                <textarea class="form-control" id="itemDescription" rows="3" 
                                          placeholder="描述道具的外观、用途或背景故事..."></textarea>
                                <div class="form-text">详细描述道具的特征和用途</div>
                            </div>

                            <!-- 道具效果 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-magic"></i> 道具效果
                                </h6>
                                <div class="mb-3">
                                    <button type="button" class="btn btn-outline-primary btn-sm" 
                                            onclick="userProfile.addEffectRow('itemEffects')">
                                        <i class="bi bi-plus"></i> 添加效果
                                    </button>
                                    <small class="text-muted ms-2">定义道具使用时的效果</small>
                                </div>
                                <div id="itemEffects" class="effects-container">
                                    <!-- 效果行将在这里动态添加 -->
                                </div>
                                <div class="form-text">
                                    <small>效果说明：目标为影响对象，类型为属性名称，数值为变化量，概率为触发几率(0-1)</small>
                                </div>
                            </div>

                            <!-- 预览区域 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-eye"></i> 道具预览
                                </h6>
                                <div id="itemPreview" class="card">
                                    <div class="card-body">
                                        <div class="text-muted text-center py-3">
                                            <i class="bi bi-box display-4"></i>
                                            <p class="mb-0 mt-2">填写道具信息后将显示预览</p>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </form>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">
                        <i class="bi bi-x-circle"></i> 取消
                    </button>
                    <button type="button" class="btn btn-primary save-item-btn" 
                            onclick="userProfile.saveNewItem()">
                        <i class="bi bi-check-circle"></i> 保存道具
                    </button>
                </div>
            </div>
        </div>
    `;

        document.body.appendChild(modal);

        // 绑定表单事件
        this.bindItemModalEvents();

        // 显示模态框
        this.showModal('addItemModal');

        // 添加一个默认的效果行
        setTimeout(() => {
            this.addEffectRow('itemEffects');
        }, 100);
    }

    /**
     * 创建添加技能模态框
     */
    createAddSkillModal() {
        // 移除已存在的模态框
        const existingModal = document.getElementById('addSkillModal');
        if (existingModal) {
            existingModal.remove();
        }

        const modal = document.createElement('div');
        modal.id = 'addSkillModal';
        modal.className = 'modal fade';
        modal.innerHTML = `
        <div class="modal-dialog modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-lightning-charge"></i> 添加新技能
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <form id="addSkillForm">
                        <div class="row g-3">
                            <!-- 基本信息 -->
                            <div class="col-12">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-info-circle"></i> 基本信息
                                </h6>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="skillName" class="form-label">技能名称 *</label>
                                <input type="text" class="form-control" id="skillName" 
                                       placeholder="输入技能名称" required>
                                <div class="form-text">技能的唯一标识名称</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="skillCategory" class="form-label">技能类别 *</label>
                                <select class="form-select" id="skillCategory" required>
                                    <option value="">选择类别</option>
                                    <option value="combat">战斗</option>
                                    <option value="magic">魔法</option>
                                    <option value="social">社交</option>
                                    <option value="mental">精神</option>
                                    <option value="physical">体能</option>
                                    <option value="crafting">制作</option>
                                    <option value="survival">生存</option>
                                    <option value="other">其他</option>
                                </select>
                                <div class="form-text">技能的分类类型</div>
                            </div>
                            
                            <div class="col-12">
                                <label for="skillDescription" class="form-label">技能描述</label>
                                <textarea class="form-control" id="skillDescription" rows="3" 
                                          placeholder="描述技能的效果、使用方法或学习背景..."></textarea>
                                <div class="form-text">详细描述技能的功能和使用条件</div>
                            </div>

                            <!-- 技能属性 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-gear"></i> 技能属性
                                </h6>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="skillCooldown" class="form-label">冷却时间（秒）</label>
                                <input type="number" class="form-control" id="skillCooldown" 
                                       placeholder="0" min="0" step="1">
                                <div class="form-text">技能使用后的冷却时间</div>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="skillManaCost" class="form-label">法力消耗</label>
                                <input type="number" class="form-control" id="skillManaCost" 
                                       placeholder="0" min="0" step="1">
                                <div class="form-text">使用技能消耗的法力值</div>
                            </div>

                            <!-- 技能效果 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-magic"></i> 技能效果
                                </h6>
                                <div class="mb-3">
                                    <button type="button" class="btn btn-outline-primary btn-sm" 
                                            onclick="userProfile.addEffectRow('skillEffects')">
                                        <i class="bi bi-plus"></i> 添加效果
                                    </button>
                                    <small class="text-muted ms-2">定义技能使用时的效果</small>
                                </div>
                                <div id="skillEffects" class="effects-container">
                                    <!-- 效果行将在这里动态添加 -->
                                </div>
                                <div class="form-text">
                                    <small>效果说明：目标为影响对象，类型为属性名称，数值为变化量，概率为触发几率(0-1)</small>
                                </div>
                            </div>

                            <!-- 预览区域 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-eye"></i> 技能预览
                                </h6>
                                <div id="skillPreview" class="card">
                                    <div class="card-body">
                                        <div class="text-muted text-center py-3">
                                            <i class="bi bi-lightning display-4"></i>
                                            <p class="mb-0 mt-2">填写技能信息后将显示预览</p>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </form>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">
                        <i class="bi bi-x-circle"></i> 取消
                    </button>
                    <button type="button" class="btn btn-primary save-skill-btn" 
                            onclick="userProfile.saveNewSkill()">
                        <i class="bi bi-check-circle"></i> 保存技能
                    </button>
                </div>
            </div>
        </div>
    `;

        document.body.appendChild(modal);

        // 绑定表单事件
        this.bindSkillModalEvents();

        // 显示模态框
        this.showModal('addSkillModal');

        // 添加一个默认的效果行
        setTimeout(() => {
            this.addEffectRow('skillEffects');
        }, 100);
    }

    /**
     * 绑定道具模态框事件
     */
    bindItemModalEvents() {
        const form = document.getElementById('addItemForm');
        if (!form) return;

        // 实时预览更新
        form.addEventListener('input', () => {
            this.updateItemPreview();
        });

        form.addEventListener('change', () => {
            this.updateItemPreview();
        });

        // 效果行移除事件委托
        const effectsContainer = document.getElementById('itemEffects');
        if (effectsContainer) {
            effectsContainer.addEventListener('click', (e) => {
                if (e.target.closest('.remove-effect-btn')) {
                    this.removeEffectRow(e.target.closest('.remove-effect-btn'));
                    this.updateItemPreview(); // 更新预览
                }
            });

            // 效果输入变化时更新预览
            effectsContainer.addEventListener('input', () => {
                this.updateItemPreview();
            });
        }

        // 表单验证
        form.addEventListener('submit', (e) => {
            e.preventDefault();
            this.saveNewItem();
        });
    }

    /**
     * 绑定技能模态框事件
     */
    bindSkillModalEvents() {
        const form = document.getElementById('addSkillForm');
        if (!form) return;

        // 实时预览更新
        form.addEventListener('input', () => {
            this.updateSkillPreview();
        });

        form.addEventListener('change', () => {
            this.updateSkillPreview();
        });

        // 效果行移除事件委托
        const effectsContainer = document.getElementById('skillEffects');
        if (effectsContainer) {
            effectsContainer.addEventListener('click', (e) => {
                if (e.target.closest('.remove-effect-btn')) {
                    this.removeEffectRow(e.target.closest('.remove-effect-btn'));
                    this.updateSkillPreview(); // 更新预览
                }
            });

            // 效果输入变化时更新预览
            effectsContainer.addEventListener('input', () => {
                this.updateSkillPreview();
            });
        }

        // 表单验证
        form.addEventListener('submit', (e) => {
            e.preventDefault();
            this.saveNewSkill();
        });
    }

    /**
     * 更新道具预览
     */
    updateItemPreview() {
        const previewContainer = document.getElementById('itemPreview');
        if (!previewContainer) return;

        try {
            // 获取表单数据
            const formData = this.getItemFormData();

            // 如果名称为空，显示默认预览
            if (!formData.name.trim()) {
                previewContainer.innerHTML = `
                <div class="card-body">
                    <div class="text-muted text-center py-3">
                        <i class="bi bi-box display-4"></i>
                        <p class="mb-0 mt-2">填写道具信息后将显示预览</p>
                    </div>
                </div>
            `;
                return;
            }

            // 生成预览HTML
            const categoryIcon = this.getCategoryIcon(formData.category);
            const rarityClass = this.getRarityClass(formData.rarity);
            const rarityLabel = this.getRarityLabel(formData.rarity);

            previewContainer.innerHTML = `
            <div class="card-header d-flex justify-content-between align-items-center ${rarityClass}">
                <div class="item-header">
                    <span class="item-icon fs-4">${categoryIcon}</span>
                    <span class="item-name fw-bold ms-2">${this.escapeHtml(formData.name)}</span>
                </div>
                <div class="item-rarity-badge">
                    <span class="badge bg-secondary">${rarityLabel}</span>
                </div>
            </div>
            <div class="card-body">
                ${formData.description ? `
                    <p class="item-description mb-3">${this.escapeHtml(formData.description)}</p>
                ` : ''}
                
                ${formData.effects && formData.effects.length > 0 ? `
                    <div class="item-effects mb-3">
                        <h6 class="text-muted mb-2">
                            <i class="bi bi-magic"></i> 效果
                        </h6>
                        <ul class="list-unstyled mb-0">
                            ${formData.effects.map(effect => `
                                <li class="small">
                                    <i class="bi bi-arrow-right text-primary"></i>
                                    ${this.formatEffect(effect)}
                                </li>
                            `).join('')}
                        </ul>
                    </div>
                ` : ''}
                
                <div class="row text-center">
                    <div class="col-6">
                        <div class="stat-value fw-bold">${this.getCategoryLabel(formData.category)}</div>
                        <div class="stat-label small text-muted">类别</div>
                    </div>
                    ${formData.value > 0 ? `
                        <div class="col-6">
                            <div class="stat-value fw-bold">${formData.value}</div>
                            <div class="stat-label small text-muted">价值</div>
                        </div>
                    ` : ''}
                </div>
            </div>
        `;

        } catch (error) {
            console.error('更新道具预览失败:', error);
            previewContainer.innerHTML = `
            <div class="card-body">
                <div class="text-danger text-center py-3">
                    <i class="bi bi-exclamation-triangle display-4"></i>
                    <p class="mb-0 mt-2">预览生成失败</p>
                </div>
            </div>
        `;
        }
    }

    /**
     * 更新技能预览
     */
    updateSkillPreview() {
        const previewContainer = document.getElementById('skillPreview');
        if (!previewContainer) return;

        try {
            // 获取表单数据
            const formData = this.getSkillFormData();

            // 如果名称为空，显示默认预览
            if (!formData.name.trim()) {
                previewContainer.innerHTML = `
                <div class="card-body">
                    <div class="text-muted text-center py-3">
                        <i class="bi bi-lightning display-4"></i>
                        <p class="mb-0 mt-2">填写技能信息后将显示预览</p>
                    </div>
                </div>
            `;
                return;
            }

            // 生成预览HTML
            const categoryIcon = this.getSkillCategoryIcon(formData.category);
            const categoryLabel = this.getSkillCategoryLabel(formData.category);

            previewContainer.innerHTML = `
            <div class="card-header d-flex justify-content-between align-items-center">
                <div class="skill-header">
                    <span class="skill-icon fs-4">${categoryIcon}</span>
                    <span class="skill-name fw-bold ms-2">${this.escapeHtml(formData.name)}</span>
                </div>
                <div class="skill-category-badge">
                    <span class="badge bg-primary">${categoryLabel}</span>
                </div>
            </div>
            <div class="card-body">
                ${formData.description ? `
                    <p class="skill-description mb-3">${this.escapeHtml(formData.description)}</p>
                ` : ''}
                
                ${formData.effects && formData.effects.length > 0 ? `
                    <div class="skill-effects mb-3">
                        <h6 class="text-muted mb-2">
                            <i class="bi bi-magic"></i> 效果
                        </h6>
                        <ul class="list-unstyled mb-0">
                            ${formData.effects.map(effect => `
                                <li class="small">
                                    <i class="bi bi-arrow-right text-primary"></i>
                                    ${this.formatEffect(effect)}
                                </li>
                            `).join('')}
                        </ul>
                    </div>
                ` : ''}
                
                <div class="skill-stats">
                    <div class="row text-center">
                        ${formData.cooldown > 0 ? `
                            <div class="col-6">
                                <div class="stat-value fw-bold">${formData.cooldown}s</div>
                                <div class="stat-label small text-muted">冷却时间</div>
                            </div>
                        ` : ''}
                        ${formData.mana_cost > 0 ? `
                            <div class="col-6">
                                <div class="stat-value fw-bold">${formData.mana_cost}</div>
                                <div class="stat-label small text-muted">法力消耗</div>
                            </div>
                        ` : ''}
                        ${formData.cooldown === 0 && formData.mana_cost === 0 ? `
                            <div class="col-12">
                                <div class="stat-value fw-bold">被动技能</div>
                                <div class="stat-label small text-muted">无消耗</div>
                            </div>
                        ` : ''}
                    </div>
                </div>
            </div>
        `;

        } catch (error) {
            console.error('更新技能预览失败:', error);
            previewContainer.innerHTML = `
            <div class="card-body">
                <div class="text-danger text-center py-3">
                    <i class="bi bi-exclamation-triangle display-4"></i>
                    <p class="mb-0 mt-2">预览生成失败</p>
                </div>
            </div>
        `;
        }
    }

    /**
     * 创建编辑道具模态框
     */
    createEditItemModal(item) {
        // 移除已存在的模态框
        const existingModal = document.getElementById('editItemModal');
        if (existingModal) {
            existingModal.remove();
        }

        const modal = document.createElement('div');
        modal.id = 'editItemModal';
        modal.className = 'modal fade';
        modal.innerHTML = `
        <div class="modal-dialog modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-pencil-square"></i> 编辑道具: ${this.escapeHtml(item.name)}
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <form id="editItemForm">
                        <input type="hidden" id="editItemId" value="${item.id}">
                        <div class="row g-3">
                            <!-- 与添加道具表单类似的结构，但使用edit前缀的ID -->
                            <div class="col-12">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-info-circle"></i> 基本信息
                                </h6>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editItemName" class="form-label">道具名称 *</label>
                                <input type="text" class="form-control" id="editItemName" 
                                       value="${this.escapeHtml(item.name)}" required>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editItemCategory" class="form-label">道具类别 *</label>
                                <select class="form-select" id="editItemCategory" required>
                                    <option value="">选择类别</option>
                                    <option value="weapon" ${item.category === 'weapon' ? 'selected' : ''}>武器</option>
                                    <option value="armor" ${item.category === 'armor' ? 'selected' : ''}>护甲</option>
                                    <option value="accessory" ${item.category === 'accessory' ? 'selected' : ''}>饰品</option>
                                    <option value="consumable" ${item.category === 'consumable' ? 'selected' : ''}>消耗品</option>
                                    <option value="tool" ${item.category === 'tool' ? 'selected' : ''}>工具</option>
                                    <option value="material" ${item.category === 'material' ? 'selected' : ''}>材料</option>
                                    <option value="quest" ${item.category === 'quest' ? 'selected' : ''}>任务物品</option>
                                    <option value="other" ${item.category === 'other' ? 'selected' : ''}>其他</option>
                                </select>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editItemRarity" class="form-label">稀有度 *</label>
                                <select class="form-select" id="editItemRarity" required>
                                    <option value="">选择稀有度</option>
                                    <option value="common" ${item.rarity === 'common' ? 'selected' : ''}>普通</option>
                                    <option value="uncommon" ${item.rarity === 'uncommon' ? 'selected' : ''}>不常见</option>
                                    <option value="rare" ${item.rarity === 'rare' ? 'selected' : ''}>稀有</option>
                                    <option value="epic" ${item.rarity === 'epic' ? 'selected' : ''}>史诗</option>
                                    <option value="legendary" ${item.rarity === 'legendary' ? 'selected' : ''}>传说</option>
                                </select>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editItemValue" class="form-label">道具价值</label>
                                <input type="number" class="form-control" id="editItemValue" 
                                       value="${item.value || 0}" min="0" step="0.01">
                            </div>
                            
                            <div class="col-12">
                                <label for="editItemDescription" class="form-label">道具描述</label>
                                <textarea class="form-control" id="editItemDescription" rows="3">${this.escapeHtml(item.description || '')}</textarea>
                            </div>

                            <!-- 道具效果 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-magic"></i> 道具效果
                                </h6>
                                <div class="mb-3">
                                    <button type="button" class="btn btn-outline-primary btn-sm" 
                                            onclick="userProfile.addEffectRow('editItemEffects')">
                                        <i class="bi bi-plus"></i> 添加效果
                                    </button>
                                </div>
                                <div id="editItemEffects" class="effects-container">
                                    <!-- 效果行将在这里填充 -->
                                </div>
                            </div>
                        </div>
                    </form>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">
                        <i class="bi bi-x-circle"></i> 取消
                    </button>
                    <button type="button" class="btn btn-primary save-edit-item-btn">
                        <i class="bi bi-check-circle"></i> 保存更改
                    </button>
                </div>
            </div>
        </div>
    `;

        document.body.appendChild(modal);

        // 填充效果
        if (item.effects && item.effects.length > 0) {
            item.effects.forEach(effect => {
                this.addEffectRow('editItemEffects', effect);
            });
        } else {
            this.addEffectRow('editItemEffects');
        }

        // 绑定编辑事件
        this.bindEditItemSaveEvent(item.id);

        // 显示模态框
        this.showModal('editItemModal');
    }

    /**
     * 创建编辑技能模态框
     */
    createEditSkillModal(skill) {
        // 移除已存在的模态框
        const existingModal = document.getElementById('editSkillModal');
        if (existingModal) {
            existingModal.remove();
        }

        const modal = document.createElement('div');
        modal.id = 'editSkillModal';
        modal.className = 'modal fade';
        modal.innerHTML = `
        <div class="modal-dialog modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="bi bi-pencil-square"></i> 编辑技能: ${this.escapeHtml(skill.name)}
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <form id="editSkillForm">
                        <input type="hidden" id="editSkillId" value="${skill.id}">
                        <div class="row g-3">
                            <!-- 与添加技能表单类似的结构，但使用edit前缀的ID -->
                            <div class="col-12">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-info-circle"></i> 基本信息
                                </h6>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editSkillName" class="form-label">技能名称 *</label>
                                <input type="text" class="form-control" id="editSkillName" 
                                       value="${this.escapeHtml(skill.name)}" required>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editSkillCategory" class="form-label">技能类别 *</label>
                                <select class="form-select" id="editSkillCategory" required>
                                    <option value="">选择类别</option>
                                    <option value="combat" ${skill.category === 'combat' ? 'selected' : ''}>战斗</option>
                                    <option value="magic" ${skill.category === 'magic' ? 'selected' : ''}>魔法</option>
                                    <option value="social" ${skill.category === 'social' ? 'selected' : ''}>社交</option>
                                    <option value="mental" ${skill.category === 'mental' ? 'selected' : ''}>精神</option>
                                    <option value="physical" ${skill.category === 'physical' ? 'selected' : ''}>体能</option>
                                    <option value="crafting" ${skill.category === 'crafting' ? 'selected' : ''}>制作</option>
                                    <option value="survival" ${skill.category === 'survival' ? 'selected' : ''}>生存</option>
                                    <option value="other" ${skill.category === 'other' ? 'selected' : ''}>其他</option>
                                </select>
                            </div>
                            
                            <div class="col-12">
                                <label for="editSkillDescription" class="form-label">技能描述</label>
                                <textarea class="form-control" id="editSkillDescription" rows="3">${this.escapeHtml(skill.description || '')}</textarea>
                            </div>

                            <!-- 技能属性 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-gear"></i> 技能属性
                                </h6>
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editSkillCooldown" class="form-label">冷却时间（秒）</label>
                                <input type="number" class="form-control" id="editSkillCooldown" 
                                       value="${skill.cooldown || 0}" min="0" step="1">
                            </div>
                            
                            <div class="col-md-6">
                                <label for="editSkillManaCost" class="form-label">法力消耗</label>
                                <input type="number" class="form-control" id="editSkillManaCost" 
                                       value="${skill.mana_cost || 0}" min="0" step="1">
                            </div>

                            <!-- 技能效果 -->
                            <div class="col-12 mt-4">
                                <h6 class="border-bottom pb-2 mb-3">
                                    <i class="bi bi-magic"></i> 技能效果
                                </h6>
                                <div class="mb-3">
                                    <button type="button" class="btn btn-outline-primary btn-sm" 
                                            onclick="userProfile.addEffectRow('editSkillEffects')">
                                        <i class="bi bi-plus"></i> 添加效果
                                    </button>
                                </div>
                                <div id="editSkillEffects" class="effects-container">
                                    <!-- 效果行将在这里填充 -->
                                </div>
                            </div>
                        </div>
                    </form>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">
                        <i class="bi bi-x-circle"></i> 取消
                    </button>
                    <button type="button" class="btn btn-primary save-edit-skill-btn">
                        <i class="bi bi-check-circle"></i> 保存更改
                    </button>
                </div>
            </div>
        </div>
    `;

        document.body.appendChild(modal);

        // 填充效果
        if (skill.effects && skill.effects.length > 0) {
            skill.effects.forEach(effect => {
                this.addEffectRow('editSkillEffects', effect);
            });
        } else {
            this.addEffectRow('editSkillEffects');
        }

        // 绑定编辑事件
        this.bindEditSkillSaveEvent(skill.id);

        // 显示模态框
        this.showModal('editSkillModal');
    }

    /**
     * 绑定编辑技能保存事件
     */
    bindEditSkillSaveEvent(skillId) {
        const saveButton = document.querySelector('.save-edit-skill-btn');
        if (!saveButton) return;

        // 移除之前的事件监听器
        const newSaveButton = saveButton.cloneNode(true);
        saveButton.parentNode.replaceChild(newSaveButton, saveButton);

        // 绑定新的保存事件
        newSaveButton.addEventListener('click', async () => {
            await this.saveEditedSkill(skillId);
        });
    }

    /**
     * 保存编辑的技能
     */
    async saveEditedSkill(skillId) {
        try {
            // 获取编辑表单数据
            const formData = this.getEditSkillFormData();

            if (!this.validateSkillData(formData)) {
                return;
            }

            this.setButtonLoading('.save-edit-skill-btn', true, '保存中...');

            console.log('💾 保存编辑的技能:', formData);

            // 调用后端API更新技能
            const result = await this.safeAPICall(() =>
                API.updateUserSkill(this.currentUserId, skillId, formData)
            );

            if (result) {
                // 更新本地数据
                const skillIndex = this.userSkills.findIndex(s => s.id === skillId);
                if (skillIndex !== -1) {
                    this.userSkills[skillIndex] = { ...this.userSkills[skillIndex], ...result };
                }

                // 重新渲染界面
                this.updateUserState();
                this.renderUserProfile();

                // 隐藏模态框
                this.hideModal('editSkillModal');

                this.showSuccess('技能更新成功');
                console.log('✅ 技能更新成功');
            }

        } catch (error) {
            console.error('❌ 保存编辑技能失败:', error);
            this.showError('保存技能失败: ' + error.message);
        } finally {
            this.setButtonLoading('.save-edit-skill-btn', false, '保存更改');
        }
    }

    /**
     * 获取编辑技能表单数据
     */
    getEditSkillFormData() {
        const effects = [];
        document.querySelectorAll('#editSkillEffects .effect-item').forEach(row => {
            const target = row.querySelector('.effect-target')?.value;
            const type = row.querySelector('.effect-type')?.value;
            const value = parseFloat(row.querySelector('.effect-value')?.value);
            const probability = parseFloat(row.querySelector('.effect-probability')?.value);

            if (target && type && !isNaN(value) && !isNaN(probability)) {
                effects.push({ target, type, value, probability });
            }
        });

        return {
            name: document.getElementById('editSkillName')?.value || '',
            description: document.getElementById('editSkillDescription')?.value || '',
            category: document.getElementById('editSkillCategory')?.value || '',
            cooldown: parseInt(document.getElementById('editSkillCooldown')?.value) || 0,
            mana_cost: parseInt(document.getElementById('editSkillManaCost')?.value) || 0,
            effects: effects
        };
    }

    /**
     * 获取编辑道具表单数据
     */
    getEditItemFormData() {
        const effects = [];
        document.querySelectorAll('#editItemEffects .effect-item').forEach(row => {
            const target = row.querySelector('.effect-target')?.value;
            const type = row.querySelector('.effect-type')?.value;
            const value = parseFloat(row.querySelector('.effect-value')?.value);
            const probability = parseFloat(row.querySelector('.effect-probability')?.value);

            if (target && type && !isNaN(value) && !isNaN(probability)) {
                effects.push({ target, type, value, probability });
            }
        });

        return {
            name: document.getElementById('editItemName')?.value || '',
            description: document.getElementById('editItemDescription')?.value || '',
            category: document.getElementById('editItemCategory')?.value || '',
            rarity: document.getElementById('editItemRarity')?.value || '',
            value: parseFloat(document.getElementById('editItemValue')?.value) || 0,
            effects: effects
        };
    }

    /**
     * 检查Bootstrap是否可用
     */
    static isBootstrapAvailable() {
        return typeof bootstrap !== 'undefined' && bootstrap.Modal;
    }

    /**
     * 显示模态框（增强版）
     */
    showModal(modalId) {
        const modal = document.getElementById(modalId);
        if (!modal) {
            console.warn(`模态框 ${modalId} 不存在`);
            return;
        }

        try {
            if (UserProfile.isBootstrapAvailable()) {
                const bsModal = new bootstrap.Modal(modal, {
                    backdrop: 'static',
                    keyboard: true,
                    focus: true
                });
                bsModal.show();

                // 存储实例以便后续操作
                modal._bsModal = bsModal;
            } else {
                // Bootstrap 不可用时的降级处理
                modal.style.display = 'block';
                modal.classList.add('show');
                document.body.classList.add('modal-open');

                // 创建背景遮罩
                const backdrop = document.createElement('div');
                backdrop.className = 'modal-backdrop fade show';
                backdrop.id = `${modalId}-backdrop`;
                document.body.appendChild(backdrop);
            }

            console.log(`✅ 模态框 ${modalId} 已显示`);
        } catch (error) {
            console.error(`显示模态框 ${modalId} 失败:`, error);
            this.showError('无法显示对话框，请刷新页面重试');
        }
    }

    /**
     * 隐藏模态框（增强版）
     */
    hideModal(modalId) {
        const modal = document.getElementById(modalId);
        if (!modal) return;

        try {
            if (UserProfile.isBootstrapAvailable() && modal._bsModal) {
                modal._bsModal.hide();
                delete modal._bsModal;
            } else {
                // 降级处理
                modal.style.display = 'none';
                modal.classList.remove('show');
                document.body.classList.remove('modal-open');

                // 移除背景遮罩
                const backdrop = document.getElementById(`${modalId}-backdrop`);
                if (backdrop) {
                    backdrop.remove();
                }
            }

            console.log(`✅ 模态框 ${modalId} 已隐藏`);
        } catch (error) {
            console.error(`隐藏模态框 ${modalId} 失败:`, error);
        }
    }



    /**
     * 加载用户数据
     */
    async loadUserData(userId) {
        try {
            console.log('📥 加载用户数据:', userId);

            // 并行加载用户档案和偏好设置
            const [profile, preferences] = await Promise.all([
                this.safeAPICall(() => API.getUserProfile(userId)),
                this.safeAPICall(() => API.getUserPreferences(userId))
            ]);

            // 合并数据
            this.userData = {
                ...profile,
                preferences: preferences || {
                    creativity_level: 'BALANCED',
                    response_length: 'medium',
                    language_style: 'casual',
                    notification_level: 'important',
                    dark_mode: false,
                    allow_plot_twists: true
                }
            };

            console.log('✅ 用户数据加载成功:', this.userData);

        } catch (error) {
            console.error('❌ 加载用户数据失败:', error);

            // 使用默认数据
            this.userData = {
                id: userId,
                username: 'unknown_user',
                display_name: '未知用户',
                bio: '',
                avatar: '',
                preferences: {
                    creativity_level: 'BALANCED',
                    response_length: 'medium',
                    language_style: 'casual',
                    notification_level: 'important',
                    dark_mode: false,
                    allow_plot_twists: true
                }
            };

            throw error;
        }
    }

}

// ========================================
// 全局函数（保持向后兼容）
// ========================================

/**
 * 显示添加道具模态框
 */
function showAddItemModal() {
    if (window.userProfile) {
        window.userProfile.showModal('addItemModal');
    }
}

/**
 * 显示添加技能模态框
 */
function showAddSkillModal() {
    if (window.userProfile) {
        window.userProfile.showModal('addSkillModal');
    }
}

/**
 * 全局函数：显示添加道具模态框
 */
function showAddItemModal() {
    if (window.userProfile && window.userProfile.showAddItemModal) {
        window.userProfile.showAddItemModal();
    } else {
        console.warn('用户档案管理器未初始化');
        alert('请先初始化用户档案系统');
    }
}

/**
 * 全局函数：显示添加技能模态框
 */
function showAddSkillModal() {
    if (window.userProfile && window.userProfile.showAddSkillModal) {
        window.userProfile.showAddSkillModal();
    } else {
        console.warn('用户档案管理器未初始化');
        alert('请先初始化用户档案系统');
    }
}

/**
 * 全局函数：显示编辑用户档案模态框
 */
function showEditProfileModal() {
    if (window.userProfile && window.userProfile.showEditProfileModal) {
        // 获取当前用户ID
        const userId = window.userProfile.currentUserId || 'user_default';
        window.userProfile.showEditProfileModal(userId);
    } else {
        console.warn('用户档案管理器未初始化');
        alert('请先初始化用户档案系统');
    }
}

/**
 * 全局函数：导出用户数据
 */
function exportUserData() {
    if (window.userProfile && window.userProfile.exportUserData) {
        window.userProfile.exportUserData();
    } else {
        console.warn('用户档案管理器未初始化');
        alert('请先初始化用户档案系统');
    }
}

// ========================================
// 全局初始化
// ========================================

// 确保在DOM加载完成后创建全局实例
if (typeof window !== 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            window.UserProfile = UserProfile;
            window.userProfile = new UserProfile();
            console.log('👤 UserProfile 已准备就绪');
        });
    } else {
        window.UserProfile = UserProfile;
        window.userProfile = new UserProfile();
        console.log('👤 UserProfile 已准备就绪');
    }
}
