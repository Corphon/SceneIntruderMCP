/**
 * 用户档案管理器
 * 基于后端完整的用户道具和技能API重新设计
 * 支持用户道具、技能、成就等功能
 */
class UserProfile {
    constructor() {
        this.currentUserId = null;
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

        } catch (error) {
            console.error('❌ 编辑道具失败:', error);
            this.showError('编辑道具失败');
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

// 添加CSS样式
if (typeof document !== 'undefined') {
    const addUserProfileStyles = () => {
        if (document.getElementById('user-profile-styles')) return;

        const style = document.createElement('style');
        style.id = 'user-profile-styles';
        style.textContent = `
        /* 用户档案容器样式 */
        .user - profile - container {
        padding: 20px 0;
    }
            
            .empty - state {
        min - height: 300px;
        display: flex;
        flex - direction: column;
        justify - content: center;
        align - items: center;
    }
            
            .no - results {
        min - height: 200px;
        display: flex;
        flex - direction: column;
        justify - content: center;
        align - items: center;
    }

            /* 统计卡片样式 */
            .stat - card {
        background: white;
        border: 1px solid #dee2e6;
        border - radius: 8px;
        padding: 20px;
        text - align: center;
        transition: all 0.3s ease;
    }
            
            .stat - card:hover {
        transform: translateY(-2px);
        box - shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
    }
            
            .stat - icon {
        font - size: 2rem;
        margin - bottom: 10px;
    }
            
            .stat - value {
        font - size: 2rem;
        font - weight: 700;
        color: #495057;
        margin - bottom: 5px;
    }
            
            .stat - label {
        color: #6c757d;
        font - size: 0.9rem;
        font - weight: 500;
    }

            /* 道具卡片样式 */
            .item - card {
        transition: all 0.3s ease;
        border: 2px solid transparent;
    }
            
            .item - card:hover {
        transform: translateY(-2px);
        box - shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
    }
            
            .item - card.rarity - common {
        border - color: #6c757d;
    }
            
            .item - card.rarity - uncommon {
        border - color: #28a745;
    }
            
            .item - card.rarity - rare {
        border - color: #007bff;
    }
            
            .item - card.rarity - epic {
        border - color: #ffc107;
    }
            
            .item - card.rarity - legendary {
        border - color: #dc3545;
        box - shadow: 0 0 10px rgba(220, 53, 69, 0.3);
    }

            /* 技能卡片样式 */
            .skill - card {
        transition: all 0.3s ease;
        border: 1px solid #dee2e6;
    }
            
            .skill - card:hover {
        transform: translateY(-2px);
        box - shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
    }

            /* 效果列表样式 */
            .effects - list {
        padding - left: 1rem;
        margin - bottom: 0;
    }
            
            .effect - item {
        font - size: 0.9rem;
        color: #495057;
    }

            /* 稀有度点样式 */
            .rarity - dot {
        display: inline - block;
        width: 8px;
        height: 8px;
        border - radius: 50 %;
        margin - right: 8px;
    }

    /* 响应式设计 */
    @media(max - width: 768px) {
                .user - profile - container {
            padding: 10px 0;
        }
                
                .stat - card {
            padding: 15px;
        }
                
                .stat - value {
            font - size: 1.5rem;
        }
                
                .item - card, .skill - card {
            margin - bottom: 15px;
        }
    }
    `;

        document.head.appendChild(style);
        console.log('✅ UserProfile 样式已加载');
    };

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', addUserProfileStyles);
    } else {
        addUserProfileStyles();
    }
}
