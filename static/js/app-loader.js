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
                if (window.SceneApp?.initDashboard) {
                    window.SceneApp.initDashboard();
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
 * 初始化故事视图页面 - 新增方法
 */
    async initStoryView(sceneId) {
        console.log(`📖 初始化故事视图: ${sceneId}`);

        try {
            // 等待 StoryManager 加载
            await this.waitForDependencies(['StoryManager'], 5000, false);

            if (typeof StoryManager !== 'undefined') {
                // 初始化故事管理器
                if (!window.storyManager) {
                    window.storyManager = new StoryManager();
                }

                // 加载场景故事
                await window.storyManager.init(sceneId);
                console.log('✅ 故事系统初始化完成');
            } else {
                // 降级处理
                console.warn('⚠️ StoryManager 不可用，使用基础功能');
                this.showStoryLoadError('故事功能模块加载失败');
            }

            // 初始化导出管理器
            if (typeof ExportManager !== 'undefined') {
                if (!window.exportManager) {
                    window.exportManager = new ExportManager();
                }
            }

        } catch (error) {
            console.error('故事视图初始化失败:', error);
            this.showStoryLoadError(error.message);
        }
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
                await StoryManager.loadSceneStory(sceneId);
            }

            // 初始化情绪显示
            if (typeof EmotionDisplay !== 'undefined') {
                // EmotionDisplay 会自动监听聊天事件
            }

            // 初始化导出管理器
            if (typeof ExportManager !== 'undefined') {
                if (!window.exportManager) {
                    window.exportManager = new ExportManager();
                }
            }

            // 初始化实时通信
            if (typeof window.initSceneRealtime === 'function') {
                await window.initSceneRealtime(sceneId);
            }

            // 初始化场景应用
            if (window.SceneApp?.initSceneView) {
                window.SceneApp.initSceneView(sceneId);
            } else if (window.SceneApp?.initScene) {
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
