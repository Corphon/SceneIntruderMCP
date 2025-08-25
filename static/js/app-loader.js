/**
 * 应用加载器 - 智能依赖管理和初始化
 * 支持动态加载、错误处理、开发调试
 */
class AppLoader {
    constructor() {
        this.loadedScripts = new Set();
        this.loadedStyles = new Set();
        this.initPromise = null;
        this.isInitialized = false;

        // 依赖配置
        this.dependencies = {
            // 核心依赖（按顺序加载）
            core: [
                '/static/js/utils.js',
                '/static/js/api.js',
                '/static/js/app.js'
            ],

            // 扩展功能（可并行加载）
            extensions: [
                '/static/js/story.js',
                '/static/js/emotions.js',
                '/static/js/export.js',
                '/static/js/realtime.js'
            ],

            // 页面特定功能
            pages: {
                '/user_profile': ['/static/js/user-profile.js']
            }
        };

        // 初始化状态
        this.initStatus = {
            coreLoaded: false,
            extensionsLoaded: false,
            appInitialized: false
        };
    }

    /**
     * 主加载入口 - 支持重复调用
     */
    async loadApp() {
        if (this.initPromise) {
            return this.initPromise;
        }

        this.initPromise = this._doLoadApp();
        return this.initPromise;
    }

    /**
     * 实际的加载逻辑
     */
    async _doLoadApp() {
        try {
            console.log('🚀 开始加载应用依赖...');

            // 1. 加载核心依赖（按顺序）
            await this.loadCoreDependencies();

            // 2. 加载扩展功能（并行）
            await this.loadExtensions();

            // 3. 加载页面特定功能
            await this.loadPageSpecificDependencies();

            // 4. 初始化应用
            await this.initializeApp();

            console.log('✅ 应用加载完成');
            return true;

        } catch (error) {
            console.error('❌ 应用加载失败:', error);
            this.showLoadError(error);
            throw error;
        }
    }

    /**
     * 加载核心依赖
     */
    async loadCoreDependencies() {
        console.log('📦 加载核心依赖...');

        for (const script of this.dependencies.core) {
            await this.loadScript(script);
            const moduleName = this.getModuleName(script);
            console.log(`✅ ${moduleName} 加载完成`);
        }

        this.initStatus.coreLoaded = true;
        console.log('🎯 核心依赖加载完成');
    }

    /**
     * 加载扩展功能
     */
    async loadExtensions() {
        console.log('🔧 加载扩展功能...');

        const loadPromises = this.dependencies.extensions.map(script =>
            this.loadScript(script).catch(error => {
                console.warn(`⚠️ 扩展功能 ${this.getModuleName(script)} 加载失败:`, error);
                return null; // 允许失败，不阻断主流程
            })
        );

        await Promise.all(loadPromises);
        this.initStatus.extensionsLoaded = true;
        console.log('🎨 扩展功能加载完成');
    }

    /**
     * 加载页面特定依赖
     */
    async loadPageSpecificDependencies() {
        const path = window.location.pathname;

        for (const [pathPattern, scripts] of Object.entries(this.dependencies.pages)) {
            if (path.includes(pathPattern)) {
                console.log(`📄 加载页面特定功能: ${pathPattern}`);

                for (const script of scripts) {
                    try {
                        await this.loadScript(script);
                        console.log(`✅ ${this.getModuleName(script)} 加载完成`);
                    } catch (error) {
                        console.warn(`⚠️ 页面功能 ${this.getModuleName(script)} 加载失败:`, error);
                    }
                }
            }
        }
    }

    /**
     * 加载单个脚本文件
     */
    async loadScript(src) {
        if (this.loadedScripts.has(src)) {
            return; // 已加载，跳过
        }

        return new Promise((resolve, reject) => {
            const script = document.createElement('script');
            script.src = src;
            script.async = false; // 保持加载顺序

            script.onload = () => {
                this.loadedScripts.add(src);
                resolve();
            };

            script.onerror = () => {
                reject(new Error(`Failed to load: ${src}`));
            };

            document.head.appendChild(script);
        });
    }

    /**
     * 初始化应用
     */
    async initializeApp() {
        console.log('🎭 初始化应用...');

        // 检查核心依赖
        const missingClasses = this.checkCoreDependencies();
        if (missingClasses.length > 0) {
            throw new Error(`缺少核心依赖: ${missingClasses.join(', ')}`);
        }

        // 根据页面类型初始化
        await this.initializeByPageType();

        // 设置全局错误处理
        this.setupGlobalErrorHandling();

        // 设置开发模式
        this.setupDevelopmentMode();

        this.initStatus.appInitialized = true;
        this.isInitialized = true;

        console.log('🎉 应用初始化完成');
    }

    /**
     * 根据页面类型初始化
     */
    async initializeByPageType() {
        const path = window.location.pathname;

        try {
            if (path.includes('/scenes/create')) {
                // 场景创建页面
                if (window.SceneApp?.initSceneCreate) {
                    window.SceneApp.initSceneCreate();
                }

            } else if (path.includes('/story')) {
                // 故事视图页面 - 新增
                const sceneId = this.extractSceneIdFromPath(path);
                await this.initStoryView(sceneId);

            } else if (path.includes('/scenes/') && path.match(/\/scenes\/[^\/]+$/)) {
                // 场景详情页面
                const sceneId = path.split('/').pop();
                await this.initSceneView(sceneId);

            } else if (path.includes('/user_profile') || path.includes('/profile')) {
                // 用户档案页面
                if (window.UserProfileManager?.init) {
                    window.UserProfileManager.init();
                }

            } else if (path.includes('/settings')) {
                // 设置页面
                if (window.SceneApp?.initSettings) {
                    window.SceneApp.initSettings();
                }

            } else if (path === '/' || path.includes('/dashboard')) {
                // 首页/仪表板
                if (window.SceneApp?.initDashboardState) {
                    window.SceneApp.initDashboardState();
                }
            }

        } catch (error) {
            console.error('页面初始化失败:', error);
            if (typeof Utils !== 'undefined') {
                Utils.showError('页面功能初始化失败');
            }
        }
    }

    /**
    * 初始化故事视图页面
    */
    async initStoryView(sceneId) {
        console.log(`📖 初始化故事视图: ${sceneId}`);

        try {
            // 使用新的依赖等待方法
            console.log('⏳ 等待故事管理器加载...');

            const storyDepsLoaded = await this.waitForDependencies(['StoryManager'], 8000, false);

            if (storyDepsLoaded && typeof StoryManager !== 'undefined') {
                // StoryManager 类可用，创建实例
                if (!window.storyManager) {
                    console.log('🏗️ 创建 StoryManager 实例...');
                    window.storyManager = new StoryManager();
                }

                // 等待实例初始化完成
                if (typeof window.StoryManager.loadStory === 'function') {
                    await window.StoryManager.loadStory(sceneId);
                    console.log('✅ 故事系统初始化完成');
                } else if (typeof window.StoryManager.init === 'function') {
                    await window.StoryManager.init(sceneId);
                    console.log('✅ 故事系统初始化完成（使用备用方法）');
                } else {
                    console.warn('⚠️ StoryManager 实例缺少必要方法');
                }
            } else {
                // 降级处理
                console.warn('⚠️ StoryManager 不可用，使用基础功能');
                this.showStoryLoadError('故事功能模块加载失败，请刷新页面重试');
            }

            // 尝试初始化其他扩展功能
            await this.initOptionalExtensions();

        } catch (error) {
            console.error('❌ 故事视图初始化失败:', error);
            this.showStoryLoadError(error.message);
        }
    }

    /**
    * 初始化可选的扩展功能
    */
    async initOptionalExtensions() {
        console.log('🔧 初始化可选扩展功能...');

        // 并行初始化扩展功能
        const extensionPromises = [
            this.initExportManager(),
            this.initEmotionDisplay(),
            this.initRealtimeFeatures()
        ];

        const results = await Promise.allSettled(extensionPromises);

        results.forEach((result, index) => {
            const extensionNames = ['ExportManager', 'EmotionDisplay', 'RealtimeFeatures'];
            if (result.status === 'fulfilled') {
                console.log(`✅ ${extensionNames[index]} 初始化成功`);
            } else {
                console.warn(`⚠️ ${extensionNames[index]} 初始化失败:`, result.reason);
            }
        });
    }

    /**
    * 初始化导出管理器
    */
    async initExportManager() {
        const loaded = await this.waitForDependencies(['ExportManager'], 3000, false);

        if (loaded && typeof ExportManager !== 'undefined') {
            if (!window.ExportManager) {
                window.ExportManager = new ExportManager();
            }
            return true;
        }

        throw new Error('ExportManager 加载失败');
    }

    /**
     * 初始化情绪显示
     */
    async initEmotionDisplay() {
        const loaded = await this.waitForDependencies(['EmotionDisplay'], 3000, false);

        if (loaded && typeof EmotionDisplay !== 'undefined') {
            // EmotionDisplay 通常自动监听聊天事件
            console.log('EmotionDisplay 已就绪');
            return true;
        }

        throw new Error('EmotionDisplay 加载失败');
    }

    /**
    * 初始化实时功能
    */
    async initRealtimeFeatures() {
        const loaded = await this.waitForDependencies(['RealtimeManager'], 3000, false);

        if (loaded && typeof window.initSceneRealtime === 'function') {
            // 这里可以根据需要初始化实时功能
            console.log('实时功能已就绪');
            return true;
        }

        throw new Error('实时功能加载失败');
    }

    /**
 * 更新的场景视图初始化方法
 */
    async initSceneView(sceneId) {
        console.log(`🎭 初始化场景视图: ${sceneId}`);

        try {
            // 等待核心依赖
            const coreReady = await this.waitForCoreDependencies(10000);
            if (!coreReady) {
                throw new Error('核心依赖加载失败');
            }

            // 等待页面特定依赖
            await this.waitForPageSpecificDependencies('scene-view', 8000);

            // 并行初始化各个功能模块
            const initPromises = [
                this.initStoryManagerForScene(sceneId),
                this.initEmotionDisplay(),
                this.initExportManager(),
                this.initRealtimeForScene(sceneId)
            ];

            const results = await Promise.allSettled(initPromises);

            // 记录初始化结果
            const moduleNames = ['StoryManager', 'EmotionDisplay', 'ExportManager', 'Realtime'];
            results.forEach((result, index) => {
                if (result.status === 'fulfilled') {
                    console.log(`✅ ${moduleNames[index]} 初始化成功`);
                } else {
                    console.warn(`⚠️ ${moduleNames[index]} 初始化失败:`, result.reason);
                }
            });

            // 最后初始化场景应用（如果可用）
            if (window.SceneApp && typeof window.SceneApp.initScene === 'function') {
                await window.SceneApp.initScene();
            }

            console.log('🎉 场景视图初始化完成');

        } catch (error) {
            console.error('❌ 场景视图初始化失败:', error);
            if (typeof Utils !== 'undefined') {
                Utils.showError('场景加载失败: ' + error.message);
            }
        }
    }

    /**
 * 为场景初始化故事管理器
 */
    async initStoryManagerForScene(sceneId) {
        const loaded = await this.waitForDependencies(['StoryManager'], 5000, false);

        if (loaded && typeof StoryManager !== 'undefined') {
            // 使用静态方法加载场景故事
            if (typeof StoryManager.loadStory === 'function') {
                await StoryManager.loadStory(sceneId);
            } else {
                // 降级：创建实例并加载
                if (!window.storyManager) {
                    window.storyManager = new StoryManager();
                }
                if (typeof window.storyManager.loadStory === 'function') {
                    await window.storyManager.loadStory(sceneId);
                }
            }
            return true;
        }

        throw new Error('StoryManager 不可用');
    }

    /**
     * 为场景初始化实时功能
     */
    async initRealtimeForScene(sceneId) {
        if (typeof window.initSceneRealtime === 'function') {
            await window.initSceneRealtime(sceneId);
            return true;
        }

        throw new Error('实时功能不可用');
    }

    /**
     * 等待指定依赖加载完成
     * @param {string|Array} dependencies - 依赖名称或依赖数组
     * @param {number} timeout - 超时时间（毫秒）
     * @param {boolean} throwOnTimeout - 超时时是否抛出异常
     * @returns {Promise<boolean>} 是否成功加载所有依赖
     */
    async waitForDependencies(dependencies, timeout = 10000, throwOnTimeout = true) {
        // 标准化依赖列表
        const deps = Array.isArray(dependencies) ? dependencies : [dependencies];

        console.log(`⏳ 等待依赖加载: ${deps.join(', ')} (超时: ${timeout}ms)`);

        const startTime = Date.now();
        const checkInterval = 100; // 每100ms检查一次

        return new Promise((resolve, reject) => {
            const checkLoop = () => {
                // 检查所有依赖是否已加载
                const missingDeps = deps.filter(dep => !this.isDependencyLoaded(dep));

                if (missingDeps.length === 0) {
                    console.log(`✅ 所有依赖已加载: ${deps.join(', ')}`);
                    resolve(true);
                    return;
                }

                // 检查是否超时
                const elapsed = Date.now() - startTime;
                if (elapsed > timeout) {
                    const errorMessage = `依赖等待超时 (${elapsed}ms): ${missingDeps.join(', ')}`;
                    console.warn(`⚠️ ${errorMessage}`);

                    if (throwOnTimeout) {
                        reject(new Error(errorMessage));
                    } else {
                        resolve(false);
                    }
                    return;
                }

                // 继续等待
                setTimeout(checkLoop, checkInterval);
            };

            // 开始检查循环
            checkLoop();
        });
    }

    /**
     * 检查单个依赖是否已加载
     * @param {string} dependency - 依赖名称
     * @returns {boolean} 是否已加载
     */
    isDependencyLoaded(dependency) {
        // 检查全局对象
        if (typeof window[dependency] !== 'undefined') {
            // 对于类，检查是否为函数
            if (typeof window[dependency] === 'function') {
                return true;
            }

            // 对于实例，检查是否存在且不为null
            if (window[dependency] !== null && typeof window[dependency] === 'object') {
                return true;
            }

            // 其他类型也认为已加载
            return true;
        }

        // 特殊检查规则
        switch (dependency.toLowerCase()) {
            case 'utils':
                return typeof Utils !== 'undefined' && typeof Utils.checkDependencies === 'function';

            case 'api':
                return typeof API !== 'undefined' && typeof API.request === 'function';

            case 'sceneapp':
                return typeof SceneApp !== 'undefined' && window.app instanceof SceneApp;

            case 'storymanager':
                // 检查类和实例
                return (typeof StoryManager !== 'undefined') ||
                    (typeof window.storyManager !== 'undefined' && window.storyManager !== null);

            case 'emotiondisplay':
                return typeof EmotionDisplay !== 'undefined';

            case 'exportmanager':
                return typeof ExportManager !== 'undefined';

            case 'realtimemanager':
                return typeof RealtimeManager !== 'undefined' ||
                    typeof window.realtimeManager !== 'undefined';

            case 'userprofilemanager':
                return typeof UserProfileManager !== 'undefined';

            default:
                return false;
        }
    }

    /**
     * 等待多个依赖组加载完成
     * @param {Object} dependencyGroups - 依赖组对象 {groupName: [dependencies]}
     * @param {number} timeout - 超时时间
     * @returns {Promise<Object>} 加载结果 {groupName: success}
     */
    async waitForDependencyGroups(dependencyGroups, timeout = 15000) {
        const results = {};
        const promises = [];

        for (const [groupName, deps] of Object.entries(dependencyGroups)) {
            const promise = this.waitForDependencies(deps, timeout, false)
                .then(success => {
                    results[groupName] = success;
                    return success;
                })
                .catch(error => {
                    console.warn(`依赖组 ${groupName} 加载失败:`, error);
                    results[groupName] = false;
                    return false;
                });

            promises.push(promise);
        }

        await Promise.all(promises);

        console.log('📊 依赖组加载结果:', results);
        return results;
    }

    /**
     * 检查核心依赖是否就绪
     * @returns {Promise<boolean>} 核心依赖是否就绪
     */
    async waitForCoreDependencies(timeout = 10000) {
        const coreDeps = ['Utils', 'API', 'SceneApp'];

        try {
            await this.waitForDependencies(coreDeps, timeout, true);
            console.log('✅ 核心依赖已就绪');
            return true;
        } catch (error) {
            console.error('❌ 核心依赖加载失败:', error);
            return false;
        }
    }

    /**
     * 检查扩展依赖是否就绪（可选）
     * @returns {Promise<Object>} 扩展依赖加载结果
     */
    async waitForExtensionDependencies(timeout = 8000) {
        const extensionGroups = {
            story: ['StoryManager'],
            emotion: ['EmotionDisplay'],
            export: ['ExportManager'],
            realtime: ['RealtimeManager'],
            profile: ['UserProfileManager']
        };

        console.log('🔧 检查扩展依赖...');
        const results = await this.waitForDependencyGroups(extensionGroups, timeout);

        const loadedCount = Object.values(results).filter(Boolean).length;
        const totalCount = Object.keys(results).length;

        console.log(`📈 扩展依赖加载完成: ${loadedCount}/${totalCount}`);

        return results;
    }

    /**
     * 智能依赖检查 - 根据页面类型检查相应依赖
     * @param {string} pageType - 页面类型
     * @returns {Promise<boolean>} 依赖是否满足要求
     */
    async waitForPageSpecificDependencies(pageType = null, timeout = 8000) {
        if (!pageType) {
            pageType = this.getPageType();
        }

        let requiredDeps = ['Utils', 'API']; // 基础依赖

        switch (pageType) {
            case 'scene-view':
                requiredDeps.push('SceneApp', 'RealtimeManager');
                break;

            case 'story-view':
                requiredDeps.push('SceneApp', 'StoryManager');
                break;

            case 'scene-create':
                requiredDeps.push('SceneApp');
                break;

            case 'user-profile':
                requiredDeps.push('UserProfileManager');
                break;

            case 'settings':
                requiredDeps.push('SceneApp');
                break;

            case 'dashboard':
                requiredDeps.push('SceneApp');
                break;

            default:
                requiredDeps.push('SceneApp');
        }

        console.log(`🎯 页面类型 "${pageType}" 需要依赖:`, requiredDeps);

        try {
            await this.waitForDependencies(requiredDeps, timeout, false);
            return true;
        } catch (error) {
            console.warn(`页面依赖检查失败:`, error);
            return false;
        }
    }

    /**
     * 依赖加载重试机制
     * @param {string|Array} dependencies - 依赖列表
     * @param {Object} options - 重试选项
     * @returns {Promise<boolean>} 是否成功
     */
    async retryDependencyLoading(dependencies, options = {}) {
        const {
            maxRetries = 3,
            retryDelay = 1000,
            timeout = 5000,
            onRetry = null
        } = options;

        for (let attempt = 1; attempt <= maxRetries; attempt++) {
            try {
                console.log(`🔄 依赖加载尝试 ${attempt}/${maxRetries}:`, dependencies);

                const success = await this.waitForDependencies(dependencies, timeout, false);

                if (success) {
                    if (attempt > 1) {
                        console.log(`✅ 依赖在第 ${attempt} 次尝试时加载成功`);
                    }
                    return true;
                }

                if (attempt < maxRetries) {
                    console.log(`⏳ 第 ${attempt} 次尝试失败，${retryDelay}ms 后重试...`);

                    if (onRetry) {
                        onRetry(attempt, maxRetries);
                    }

                    await new Promise(resolve => setTimeout(resolve, retryDelay));
                }

            } catch (error) {
                console.warn(`第 ${attempt} 次依赖加载尝试出错:`, error);

                if (attempt === maxRetries) {
                    throw error;
                }

                await new Promise(resolve => setTimeout(resolve, retryDelay));
            }
        }

        console.error(`❌ 依赖加载在 ${maxRetries} 次尝试后仍然失败:`, dependencies);
        return false;
    }

    /**
     * 获取依赖加载状态报告
     * @returns {Object} 依赖状态报告
     */
    getDependencyReport() {
        const allDependencies = [
            'Utils', 'API', 'SceneApp',
            'StoryManager', 'EmotionDisplay', 'ExportManager',
            'RealtimeManager', 'UserProfileManager'
        ];

        const report = {
            timestamp: new Date().toISOString(),
            total: allDependencies.length,
            loaded: 0,
            missing: [],
            details: {}
        };

        allDependencies.forEach(dep => {
            const isLoaded = this.isDependencyLoaded(dep);
            report.details[dep] = {
                loaded: isLoaded,
                type: typeof window[dep],
                available: window[dep] !== undefined
            };

            if (isLoaded) {
                report.loaded++;
            } else {
                report.missing.push(dep);
            }
        });

        report.loadedPercentage = Math.round((report.loaded / report.total) * 100);

        return report;
    }

    /**
     * 监视依赖加载状态
     * @param {Array} dependencies - 要监视的依赖
     * @param {Function} callback - 状态变化回调
     * @param {number} interval - 检查间隔（毫秒）
     * @returns {Function} 停止监视的函数
     */
    watchDependencies(dependencies, callback, interval = 1000) {
        const deps = Array.isArray(dependencies) ? dependencies : [dependencies];
        let lastState = {};

        const checkState = () => {
            const currentState = {};
            let hasChanges = false;

            deps.forEach(dep => {
                const isLoaded = this.isDependencyLoaded(dep);
                currentState[dep] = isLoaded;

                if (lastState[dep] !== isLoaded) {
                    hasChanges = true;
                }
            });

            if (hasChanges) {
                callback(currentState, lastState);
                lastState = { ...currentState };
            }
        };

        // 立即检查一次
        deps.forEach(dep => {
            lastState[dep] = this.isDependencyLoaded(dep);
        });
        callback(lastState, {});

        // 定期检查
        const intervalId = setInterval(checkState, interval);

        // 返回停止函数
        return () => {
            clearInterval(intervalId);
            console.log('🛑 停止监视依赖:', deps);
        };
    }

    /**
     * 预加载依赖（如果需要）
     * @param {Array} dependencies - 需要预加载的依赖
     * @returns {Promise<void>}
     */
    async preloadDependencies(dependencies) {
        const deps = Array.isArray(dependencies) ? dependencies : [dependencies];

        console.log('🚀 开始预加载依赖:', deps);

        const loadPromises = deps.map(async (dep) => {
            if (this.isDependencyLoaded(dep)) {
                console.log(`✅ 依赖 ${dep} 已存在，跳过预加载`);
                return true;
            }

            // 根据依赖名称确定脚本路径
            const scriptPath = this.getScriptPathForDependency(dep);
            if (!scriptPath) {
                console.warn(`⚠️ 未找到依赖 ${dep} 的脚本路径`);
                return false;
            }

            try {
                await this.loadScript(scriptPath);
                console.log(`✅ 预加载依赖 ${dep} 成功`);
                return true;
            } catch (error) {
                console.error(`❌ 预加载依赖 ${dep} 失败:`, error);
                return false;
            }
        });

        const results = await Promise.all(loadPromises);
        const successCount = results.filter(Boolean).length;

        console.log(`📊 预加载完成: ${successCount}/${deps.length} 个依赖成功加载`);
    }

    /**
     * 根据依赖名称获取脚本路径
     * @param {string} dependency - 依赖名称
     * @returns {string|null} 脚本路径
     */
    getScriptPathForDependency(dependency) {
        const dependencyMap = {
            'Utils': '/static/js/utils.js',
            'API': '/static/js/api.js',
            'SceneApp': '/static/js/app.js',
            'StoryManager': '/static/js/story.js',
            'EmotionDisplay': '/static/js/emotions.js',
            'ExportManager': '/static/js/export.js',
            'RealtimeManager': '/static/js/realtime.js',
            'UserProfileManager': '/static/js/user-profile.js'
        };

        return dependencyMap[dependency] || null;
    }

    /**
     * 清理失败的依赖加载
     * @param {Array} dependencies - 要清理的依赖
     */
    cleanupFailedDependencies(dependencies) {
        const deps = Array.isArray(dependencies) ? dependencies : [dependencies];

        deps.forEach(dep => {
            // 移除全局引用
            if (window[dep]) {
                try {
                    delete window[dep];
                    console.log(`🧹 已清理失败的依赖: ${dep}`);
                } catch (error) {
                    console.warn(`清理依赖 ${dep} 时出错:`, error);
                }
            }

            // 移除对应的script标签（如果存在且标记为失败）
            const scriptPath = this.getScriptPathForDependency(dep);
            if (scriptPath) {
                const scripts = document.querySelectorAll(`script[src="${scriptPath}"]`);
                scripts.forEach(script => {
                    if (script.dataset.loadFailed === 'true') {
                        script.remove();
                        console.log(`🧹 已移除失败的脚本标签: ${scriptPath}`);
                    }
                });
            }
        });
    }

    /**
     * 从路径提取场景ID
     */
    extractSceneIdFromPath(path) {
        // 匹配 /scenes/{sceneId}/story 格式
        const match = path.match(/\/scenes\/([^\/]+)\/story/);
        return match ? match[1] : null;
    }

    /**
     * 显示故事加载错误
     */
    showStoryLoadError(errorMessage) {
        const container = document.getElementById('story-container');
        if (container) {
            container.innerHTML = `
            <div class="alert alert-danger">
                <h6><i class="bi bi-exclamation-triangle"></i> 加载失败</h6>
                <p>无法加载故事数据: ${errorMessage}</p>
                <div class="mt-3">
                    <button type="button" class="btn btn-sm btn-outline-danger me-2" onclick="location.reload()">
                        <i class="bi bi-arrow-clockwise"></i> 重新加载
                    </button>
                    <button type="button" class="btn btn-sm btn-outline-secondary" onclick="history.back()">
                        <i class="bi bi-arrow-left"></i> 返回
                    </button>
                </div>
            </div>
        `;
        }
    }

    /**
     * 更新页面类型检测
     */
    getPageType() {
        const path = window.location.pathname;

        if (path.includes('/scenes/create')) return 'scene-create';
        if (path.includes('/story')) return 'story-view';  // 新增
        if (path.includes('/scenes/') && path.match(/\/scenes\/[^\/]+$/)) return 'scene-view';
        if (path.includes('/user_profile')) return 'user-profile';
        if (path.includes('/settings')) return 'settings';
        if (path === '/' || path.includes('/dashboard')) return 'dashboard';

        return 'unknown';
    }

    /**
     * 初始化场景视图（增强版）
     */
    async initSceneView(sceneId) {
        console.log(`🎭 初始化场景视图: ${sceneId}`);

        try {
            // 初始化故事管理器
            if (typeof StoryManager !== 'undefined') {
                await StoryManager.loadStory(sceneId);
            }

            // 初始化情绪显示
            if (typeof EmotionDisplay !== 'undefined') {
                // EmotionDisplay 会自动监听聊天事件
            }

            // 初始化导出管理器
            if (typeof ExportManager !== 'undefined') {
                if (!window.ExportManager) {
                    window.ExportManager = new ExportManager();
                }
            }

            // 初始化实时通信
            if (typeof window.initSceneRealtime === 'function') {
                await window.initSceneRealtime(sceneId);
            }

            // 初始化场景应用
            if (window.SceneApp?.initScene) {
                window.SceneApp.initScene();
            }

        } catch (error) {
            console.error('场景视图初始化失败:', error);
            if (typeof Utils !== 'undefined') {
                Utils.showError('场景加载失败');
            }
        }
    }

    /**
     * 检查核心依赖
     */
    checkCoreDependencies() {
        const requiredClasses = ['Utils', 'API', 'SceneApp'];
        return requiredClasses.filter(cls => typeof window[cls] === 'undefined');
    }

    /**
     * 检查所有依赖
     */
    checkAllDependencies() {
        const dependencies = {
            // 核心依赖
            'Utils': typeof Utils !== 'undefined',
            'API': typeof API !== 'undefined',
            'SceneApp': typeof SceneApp !== 'undefined',

            // 扩展功能
            'StoryManager': typeof StoryManager !== 'undefined',
            'EmotionDisplay': typeof EmotionDisplay !== 'undefined',
            'ExportManager': typeof ExportManager !== 'undefined',
            'RealtimeManager': typeof RealtimeManager !== 'undefined',

            // 页面特定
            'UserProfileManager': typeof UserProfileManager !== 'undefined'
        };

        console.log('📊 依赖检查结果:', dependencies);
        return dependencies;
    }

    /**
     * 设置开发模式
     */
    setupDevelopmentMode() {
        const isDev = window.location.hostname === 'localhost' ||
            window.location.search.includes('debug=1');

        if (isDev) {
            // 开发工具
            window.appLoader = this;
            window.appLoaderDebug = {
                getLoadedScripts: () => Array.from(this.loadedScripts),
                getInitStatus: () => this.initStatus,
                checkDependencies: () => this.checkAllDependencies(),
                reloadApp: () => this.reloadApp(),
                getPageType: () => this.getPageType()
            };

            console.log('🔧 开发模式已启用');
            console.log('使用 window.appLoaderDebug 查看调试工具');
        }
    }

    /**
     * 设置全局错误处理
     */
    setupGlobalErrorHandling() {
        // 捕获未处理的Promise错误
        window.addEventListener('unhandledrejection', (event) => {
            console.error('未处理的Promise错误:', event.reason);
            if (typeof Utils !== 'undefined') {
                Utils.showError('发生了未预期的错误');
            }
            event.preventDefault();
        });

        // 捕获JavaScript运行时错误
        window.addEventListener('error', (event) => {
            console.error('JavaScript错误:', event.error);
            if (typeof Utils !== 'undefined' && this.isInitialized) {
                Utils.showError('页面运行出错');
            }
        });
    }

    /**
     * 显示加载错误
     */
    showLoadError(error) {
        const errorDiv = document.createElement('div');
        errorDiv.className = 'alert alert-danger position-fixed top-0 start-50 translate-middle-x';
        errorDiv.style.zIndex = '10000';
        errorDiv.style.maxWidth = '500px';
        errorDiv.innerHTML = `
            <h4>应用加载失败</h4>
            <p>无法正常加载应用组件，请检查网络连接或刷新页面重试。</p>
            <small class="text-muted">错误详情: ${error.message}</small>
            <div class="mt-2">
                <button type="button" class="btn btn-light btn-sm me-2" onclick="location.reload()">
                    <i class="bi bi-arrow-clockwise"></i> 刷新页面
                </button>
                <button type="button" class="btn btn-outline-light btn-sm" onclick="this.parentNode.parentNode.remove()">
                    <i class="bi bi-x"></i> 关闭
                </button>
            </div>
        `;
        document.body.appendChild(errorDiv);

        // 5秒后自动移除
        setTimeout(() => {
            if (errorDiv.parentNode) {
                errorDiv.remove();
            }
        }, 10000);
    }

    // ========================================
    // 工具方法
    // ========================================

    /**
     * 获取模块名称
     */
    getModuleName(scriptPath) {
        const filename = scriptPath.split('/').pop();
        return filename.replace('.js', '').replace('-', ' ').replace(/\b\w/g, l => l.toUpperCase());
    }

    /**
     * 获取页面类型
     */
    getPageType() {
        const path = window.location.pathname;

        if (path.includes('/scenes/create')) return 'scene-create';
        if (path.includes('/scenes/') && path.match(/\/scenes\/[^\/]+$/)) return 'scene-view';
        if (path.includes('/user_profile')) return 'user-profile';
        if (path.includes('/settings')) return 'settings';
        if (path === '/' || path.includes('/dashboard')) return 'dashboard';

        return 'unknown';
    }

    /**
     * 重新加载应用
     */
    async reloadApp() {
        console.log('🔄 重新加载应用...');

        // 重置状态
        this.initPromise = null;
        this.isInitialized = false;
        this.initStatus = {
            coreLoaded: false,
            extensionsLoaded: false,
            appInitialized: false
        };

        // 重新加载
        return this.loadApp();
    }

    /**
     * 动态加载模块
     */
    async loadModule(modulePath) {
        try {
            await this.loadScript(modulePath);
            console.log(`✅ 动态模块 ${this.getModuleName(modulePath)} 加载完成`);
            return true;
        } catch (error) {
            console.error(`❌ 动态模块加载失败: ${modulePath}`, error);
            return false;
        }
    }

    /**
     * 获取加载状态
     */
    getStatus() {
        return {
            isInitialized: this.isInitialized,
            initStatus: { ...this.initStatus },
            loadedScripts: Array.from(this.loadedScripts),
            pageType: this.getPageType()
        };
    }


}

// ========================================
// 全局初始化
// ========================================

// 创建全局加载器实例
window.AppLoader = new AppLoader();

// 提供便捷的全局方法
window.reloadApp = () => window.AppLoader.reloadApp();
window.loadModule = (path) => window.AppLoader.loadModule(path);

// DOM加载完成后自动启动
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        window.AppLoader.loadApp();
    });
} else {
    // DOM已经加载完成，延迟一点启动以确保其他脚本加载
    setTimeout(() => {
        window.AppLoader.loadApp();
    }, 100);
}

// 页面可见性变化时的处理
document.addEventListener('visibilitychange', () => {
    if (!document.hidden && window.AppLoader.isInitialized) {
        // 页面重新可见时，检查应用状态
        console.log('📱 页面重新可见，检查应用状态...');

        if (typeof window.realtimeManager !== 'undefined') {
            window.realtimeManager.checkAllConnections?.();
        }
    }
});

// 在开发环境中添加依赖管理调试工具
if (typeof window !== 'undefined' && 
    (window.location?.hostname === 'localhost' || window.location?.search.includes('debug=1'))) {

    window.DEPENDENCY_DEBUG = {
        // 检查所有依赖状态
        checkAll: () => {
            if (window.AppLoader && window.AppLoader.getDependencyReport) {
                const report = window.AppLoader.getDependencyReport();
                console.table(report.details);
                return report;
            }
            return null;
        },

        // 等待特定依赖
        wait: async (deps, timeout = 5000) => {
            if (window.AppLoader && window.AppLoader.waitForDependencies) {
                try {
                    const result = await window.AppLoader.waitForDependencies(deps, timeout, false);
                    console.log(`依赖等待结果: ${result}`);
                    return result;
                } catch (error) {
                    console.error('依赖等待失败:', error);
                    return false;
                }
            }
            return false;
        },

        // 重试加载依赖
        retry: async (deps, options = {}) => {
            if (window.AppLoader && window.AppLoader.retryDependencyLoading) {
                return await window.AppLoader.retryDependencyLoading(deps, options);
            }
            return false;
        },

        // 监视依赖变化
        watch: (deps, interval = 1000) => {
            if (window.AppLoader && window.AppLoader.watchDependencies) {
                return window.AppLoader.watchDependencies(deps, (current, previous) => {
                    console.log('依赖状态变化:', { current, previous });
                }, interval);
            }
            return null;
        },

        // 清理失败的依赖
        cleanup: (deps) => {
            if (window.AppLoader && window.AppLoader.cleanupFailedDependencies) {
                window.AppLoader.cleanupFailedDependencies(deps);
            }
        },

        // 获取依赖详情
        getInfo: (dep) => {
            if (window.AppLoader && window.AppLoader.isDependencyLoaded) {
                return {
                    name: dep,
                    loaded: window.AppLoader.isDependencyLoaded(dep),
                    type: typeof window[dep],
                    available: window[dep] !== undefined,
                    value: window[dep]
                };
            }
            return null;
        },

        // 运行完整依赖测试
        runTests: async () => {
            console.log('🔧 运行依赖管理测试...');
            
            const tests = [
                {
                    name: '核心依赖检查',
                    fn: () => window.DEPENDENCY_DEBUG.checkAll()
                },
                {
                    name: '等待测试',
                    fn: () => window.DEPENDENCY_DEBUG.wait(['Utils'], 1000)
                },
                {
                    name: '依赖信息',
                    fn: () => window.DEPENDENCY_DEBUG.getInfo('Utils')
                }
            ];
            
            const results = [];
            for (const test of tests) {
                try {
                    const result = await test.fn();
                    results.push({ name: test.name, success: !!result, result });
                } catch (error) {
                    results.push({ name: test.name, success: false, error: error.message });
                }
            }
            
            console.table(results);
            return results;
        }
    };

    console.log('🔧 依赖管理调试工具已加载');
    console.log('使用 window.DEPENDENCY_DEBUG 进行调试');
}
