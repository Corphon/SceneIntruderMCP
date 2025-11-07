/**
 * 通用工具函数库
 * 基于后端API完整功能重新设计
 * 支持依赖管理、文件处理、实时通信等
 */
class Utils {
    // ========================================
    // 核心系统功能
    // ========================================

    /**
     * 检查JavaScript依赖是否已加载
     * @param {string|Array} dependencies - 依赖名称或依赖数组
     * @param {Object} options - 选项配置
     * @returns {Object} 检查结果
     */
    static checkDependencies(dependencies, options = {}) {
        const {
            throwOnMissing = false,
            showAlert = true,
            timeout = 5000,
            context = '应用'
        } = options;

        // 标准化依赖列表
        const deps = Array.isArray(dependencies) ? dependencies : [dependencies];
        const missing = [];
        const available = [];

        // 检查每个依赖
        deps.forEach(dep => {
            if (typeof window[dep] === 'undefined') {
                missing.push(dep);
            } else {
                available.push(dep);
            }
        });

        const result = {
            success: missing.length === 0,
            missing,
            available,
            message: missing.length > 0 ?
                `${context}缺少必要的依赖: ${missing.join(', ')}` :
                `${context}所有依赖已正确加载`
        };

        // 处理缺失依赖
        if (!result.success) {
            console.error('依赖检查失败:', result);

            if (showAlert) {
                if (typeof this.showError !== 'undefined') {
                    this.showError(result.message);
                } else {
                    alert(`${result.message}\n请确保正确加载了所有脚本文件`);
                }
            }

            if (throwOnMissing) {
                throw new Error(result.message);
            }
        } else {
            console.log('✅ 依赖检查通过:', available);
        }

        return result;
    }

    /**
     * 异步等待依赖加载完成
     * @param {string|Array} dependencies - 依赖名称或依赖数组
     * @param {Object} options - 选项配置
     * @returns {Promise<Object>} 检查结果
     */
    static async waitForDependencies(dependencies, options = {}) {
        const {
            timeout = 10000,
            checkInterval = 100,
            context = '应用'
        } = options;

        const deps = Array.isArray(dependencies) ? dependencies : [dependencies];
        const startTime = Date.now();

        return new Promise((resolve, reject) => {
            const checkLoop = () => {
                const result = this.checkDependencies(deps, {
                    throwOnMissing: false,
                    showAlert: false,
                    context
                });

                if (result.success) {
                    console.log(`✅ ${context}依赖等待完成:`, result.available);
                    resolve(result);
                    return;
                }

                // 检查超时
                if (Date.now() - startTime > timeout) {
                    const errorMsg = `${context}依赖等待超时: ${result.missing.join(', ')}`;
                    console.error(errorMsg);
                    reject(new Error(errorMsg));
                    return;
                }

                // 继续等待
                setTimeout(checkLoop, checkInterval);
            };

            checkLoop();
        });
    }

    /**
     * 安全地执行需要依赖的函数
     * @param {string|Array} dependencies - 依赖名称或依赖数组  
     * @param {Function} callback - 回调函数
     * @param {Function} fallback - 依赖缺失时的降级函数
     * @param {string} context - 上下文名称
     */
    static safeExecute(dependencies, callback, fallback = null, context = '函数') {
        const result = this.checkDependencies(dependencies, {
            throwOnMissing: false,
            showAlert: false,
            context
        });

        if (result.success) {
            try {
                return callback();
            } catch (error) {
                console.error(`${context}执行失败:`, error);
                if (fallback) return fallback(error);
                throw error;
            }
        } else {
            console.warn(`${context}跳过执行，缺少依赖:`, result.missing);
            if (fallback) return fallback(new Error(result.message));
            return null;
        }
    }

    /**
     * 检查特定功能是否可用
     * @param {string} feature - 功能名称
     * @returns {boolean} 是否可用
     */
    static isFeatureAvailable(feature) {
        const featureMap = {
            // API功能
            'api': () => typeof window.API !== 'undefined',
            'chat': () => typeof window.API !== 'undefined' && typeof window.API.sendMessage === 'function',
            'story': () => typeof window.StoryManager !== 'undefined',
            'export': () => typeof window.API !== 'undefined' && typeof window.API.exportSceneData === 'function',

            // UI功能  
            'toast': () => typeof bootstrap !== 'undefined' || typeof this.showToast === 'function',
            'modal': () => typeof bootstrap !== 'undefined',
            'utils': () => typeof window.Utils !== 'undefined',

            // 浏览器功能
            'fetch': () => typeof fetch !== 'undefined',
            'websocket': () => typeof WebSocket !== 'undefined',
            'eventSource': () => typeof EventSource !== 'undefined',
            'localStorage': () => {
                try {
                    const test = '__test__';
                    localStorage.setItem(test, test);
                    localStorage.removeItem(test);
                    return true;
                } catch {
                    return false;
                }
            },
            'clipboard': () => navigator.clipboard && window.isSecureContext,
            'fileReader': () => typeof FileReader !== 'undefined'
        };

        const checker = featureMap[feature];
        return checker ? checker() : false;
    }

    // ========================================
    // UI 消息系统
    // ========================================

    /**
     * 显示成功消息
     */
    static showSuccess(message, duration = 3000) {
        this.showToast(message, 'success', duration);
    }

    /**
     * 显示错误消息
     */
    static showError(message, duration = 5000) {
        this.showToast(message, 'error', duration);
    }

    /**
     * 显示警告消息
     */
    static showWarning(message, duration = 4000) {
        this.showToast(message, 'warning', duration);
    }

    /**
     * 显示信息消息
     */
    static showInfo(message, duration = 3000) {
        this.showToast(message, 'info', duration);
    }

    /**
     * 显示Toast消息
     */
    static showToast(message, type = 'info', duration = 3000) {
        // 尝试使用Bootstrap Toast
        if (this.isBootstrapAvailable()) {
            this.showBootstrapToast(message, type, duration);
            return;
        }

        // 降级到自定义Toast
        this.showCustomToast(message, type, duration);
    }

    /**
     * 显示Bootstrap Toast
     */
    static showBootstrapToast(message, type, duration) {
        const toastContainer = this.getOrCreateToastContainer();

        const toastId = 'toast-' + Date.now();
        const toastHtml = `
            <div id="${toastId}" class="toast align-items-center text-white bg-${this.getBootstrapColor(type)} border-0" role="alert" aria-live="assertive" aria-atomic="true">
                <div class="d-flex">
                    <div class="toast-body">
                        <i class="bi bi-${this.getBootstrapIcon(type)} me-2"></i>
                        ${this.escapeHtml(message)}
                    </div>
                    <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast" aria-label="Close"></button>
                </div>
            </div>
        `;

        toastContainer.insertAdjacentHTML('beforeend', toastHtml);

        const toastElement = document.getElementById(toastId);
        const toast = new bootstrap.Toast(toastElement, {
            delay: duration
        });

        toast.show();

        // 清理已隐藏的toast
        toastElement.addEventListener('hidden.bs.toast', () => {
            toastElement.remove();
        });
    }

    /**
     * 显示自定义Toast
     */
    static showCustomToast(message, type, duration) {
        const toastContainer = this.getOrCreateToastContainer();

        const toastId = 'toast-' + Date.now();
        const toast = document.createElement('div');
        toast.id = toastId;
        toast.className = `custom-toast toast-${type}`;
        toast.innerHTML = `
            <div class="toast-content">
                <span class="toast-icon">${this.getCustomIcon(type)}</span>
                <span class="toast-message">${this.escapeHtml(message)}</span>
                <button class="toast-close" onclick="this.parentElement.parentElement.remove()">&times;</button>
            </div>
        `;

        // 添加样式（如果还没有）
        this.addCustomToastStyles();

        toastContainer.appendChild(toast);

        // 显示动画
        setTimeout(() => {
            toast.classList.add('show');
        }, 10);

        // 自动隐藏
        setTimeout(() => {
            toast.classList.remove('show');
            setTimeout(() => {
                if (toast.parentNode) {
                    toast.remove();
                }
            }, 300);
        }, duration);
    }

    /**
     * 显示确认对话框
     */
    static async showConfirm(message, options = {}) {
        const {
            title = '确认',
            confirmText = '确认',
            cancelText = '取消',
            type = 'warning'
        } = options;

        return new Promise((resolve) => {
            if (this.isBootstrapAvailable()) {
                this.showBootstrapConfirm(message, title, confirmText, cancelText, type, resolve);
            } else {
                resolve(confirm(message));
            }
        });
    }

    /**
     * 显示Bootstrap确认对话框
     */
    static showBootstrapConfirm(message, title, confirmText, cancelText, type, resolve) {
        const modalId = 'confirm-modal-' + Date.now();
        const modalHtml = `
            <div class="modal fade" id="${modalId}" tabindex="-1" aria-hidden="true">
                <div class="modal-dialog">
                    <div class="modal-content">
                        <div class="modal-header">
                            <h5 class="modal-title">
                                <i class="bi bi-${this.getBootstrapIcon(type)} me-2"></i>
                                ${this.escapeHtml(title)}
                            </h5>
                            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                        </div>
                        <div class="modal-body">
                            ${this.escapeHtml(message)}
                        </div>
                        <div class="modal-footer">
                            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">${this.escapeHtml(cancelText)}</button>
                            <button type="button" class="btn btn-${this.getBootstrapColor(type)}" id="${modalId}-confirm">${this.escapeHtml(confirmText)}</button>
                        </div>
                    </div>
                </div>
            </div>
        `;

        document.body.insertAdjacentHTML('beforeend', modalHtml);

        const modalElement = document.getElementById(modalId);
        const modal = new bootstrap.Modal(modalElement);

        // 绑定事件
        document.getElementById(`${modalId}-confirm`).addEventListener('click', () => {
            modal.hide();
            resolve(true);
        });

        modalElement.addEventListener('hidden.bs.modal', () => {
            modalElement.remove();
            resolve(false);
        });

        modal.show();
    }

    /**
     * 获取或创建Toast容器
     */
    static getOrCreateToastContainer() {
        let container = document.getElementById('toast-container');
        if (!container) {
            container = document.createElement('div');
            container.id = 'toast-container';
            container.className = 'toast-container position-fixed top-0 end-0 p-3';
            container.style.zIndex = '9999';
            document.body.appendChild(container);
        }
        return container;
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
     * 获取Bootstrap颜色类
     */
    static getBootstrapColor(type) {
        const colors = {
            success: 'success',
            error: 'danger',
            warning: 'warning',
            info: 'info'
        };
        return colors[type] || 'secondary';
    }

    /**
     * 获取Bootstrap图标
     */
    static getBootstrapIcon(type) {
        const icons = {
            success: 'check-circle',
            error: 'exclamation-triangle',
            warning: 'exclamation-triangle',
            info: 'info-circle'
        };
        return icons[type] || 'info-circle';
    }

    /**
     * 获取自定义图标
     */
    static getCustomIcon(type) {
        const icons = {
            success: '✓',
            error: '✗',
            warning: '⚠',
            info: 'ℹ'
        };
        return icons[type] || 'ℹ';
    }

    /**
     * 添加自定义Toast样式
     */
    static addCustomToastStyles() {
        if (document.getElementById('custom-toast-styles')) return;

        const style = document.createElement('style');
        style.id = 'custom-toast-styles';
        style.textContent = `
            .custom-toast {
                background: white;
                border-radius: 8px;
                box-shadow: 0 4px 12px rgba(0,0,0,0.15);
                margin-bottom: 10px;
                transform: translateX(100%);
                transition: transform 0.3s ease;
                opacity: 0;
                min-width: 300px;
            }
            .custom-toast.show {
                transform: translateX(0);
                opacity: 1;
            }
            .custom-toast.toast-success { border-left: 4px solid #28a745; }
            .custom-toast.toast-error { border-left: 4px solid #dc3545; }
            .custom-toast.toast-warning { border-left: 4px solid #ffc107; }
            .custom-toast.toast-info { border-left: 4px solid #17a2b8; }
            .toast-content {
                display: flex;
                align-items: center;
                padding: 12px 16px;
            }
            .toast-icon {
                margin-right: 8px;
                font-weight: bold;
                font-size: 16px;
            }
            .toast-success .toast-icon { color: #28a745; }
            .toast-error .toast-icon { color: #dc3545; }
            .toast-warning .toast-icon { color: #ffc107; }
            .toast-info .toast-icon { color: #17a2b8; }
            .toast-message {
                flex: 1;
                color: #333;
                line-height: 1.4;
            }
            .toast-close {
                background: none;
                border: none;
                font-size: 18px;
                cursor: pointer;
                color: #999;
                margin-left: 8px;
                width: 20px;
                height: 20px;
                display: flex;
                align-items: center;
                justify-content: center;
            }
            .toast-close:hover {
                color: #666;
            }
        `;
        document.head.appendChild(style);
    }

    // ========================================
    // 文件处理功能
    // ========================================

    /**
     * 读取文件内容
     * @param {File} file - 文件对象
     * @param {string} readAs - 读取方式 ('text', 'dataURL', 'arrayBuffer')
     * @returns {Promise<string|ArrayBuffer>} 文件内容
     */
    static readFile(file, readAs = 'text') {
        return new Promise((resolve, reject) => {
            if (!this.isFeatureAvailable('fileReader')) {
                reject(new Error('浏览器不支持文件读取'));
                return;
            }

            const reader = new FileReader();

            reader.onload = (e) => resolve(e.target.result);
            reader.onerror = (e) => reject(new Error('文件读取失败'));

            switch (readAs) {
                case 'text':
                    reader.readAsText(file);
                    break;
                case 'dataURL':
                    reader.readAsDataURL(file);
                    break;
                case 'arrayBuffer':
                    reader.readAsArrayBuffer(file);
                    break;
                default:
                    reject(new Error('不支持的读取方式: ' + readAs));
            }
        });
    }

    /**
     * 验证文件类型和大小
     * @param {File} file - 文件对象
     * @param {Object} rules - 验证规则
     * @returns {Object} 验证结果
     */
    static validateFile(file, rules = {}) {
        const {
            maxSize = 10 * 1024 * 1024, // 10MB
            allowedTypes = [],
            allowedExtensions = []
        } = rules;

        const result = {
            valid: true,
            errors: []
        };

        // 检查文件大小
        if (file.size > maxSize) {
            result.valid = false;
            result.errors.push(`文件大小超过限制 (${this.formatFileSize(maxSize)})`);
        }

        // 检查文件类型
        if (allowedTypes.length > 0 && !allowedTypes.includes(file.type)) {
            result.valid = false;
            result.errors.push(`不支持的文件类型: ${file.type}`);
        }

        // 检查文件扩展名
        if (allowedExtensions.length > 0) {
            const extension = file.name.split('.').pop().toLowerCase();
            if (!allowedExtensions.includes(extension)) {
                result.valid = false;
                result.errors.push(`不支持的文件扩展名: .${extension}`);
            }
        }

        return result;
    }

    /**
     * 格式化文件大小
     */
    static formatFileSize(bytes) {
        if (bytes === 0) return '0 Bytes';

        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));

        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    /**
     * 下载文件
     */
    static downloadFile(data, filename, type = 'application/octet-stream') {
        const blob = new Blob([data], { type });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = filename;
        link.style.display = 'none';
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);
    }

    // ========================================
    // 实时通信功能
    // ========================================

    /**
     * 创建SSE连接
     * @param {string} url - SSE端点URL
     * @param {Object} handlers - 事件处理器
     * @returns {EventSource} EventSource实例
     */
    static createSSEConnection(url, handlers = {}) {
        if (!this.isFeatureAvailable('eventSource')) {
            throw new Error('浏览器不支持Server-Sent Events');
        }

        const eventSource = new EventSource(url);

        // 绑定事件处理器
        Object.entries(handlers).forEach(([event, handler]) => {
            if (typeof handler === 'function') {
                eventSource.addEventListener(event, handler);
            }
        });

        // 默认错误处理
        if (!handlers.error) {
            eventSource.onerror = (error) => {
                console.error('SSE连接错误:', error);
                this.showError('实时连接中断');
            };
        }

        return eventSource;
    }

    /**
     * 创建WebSocket连接
     * @param {string} url - WebSocket URL
     * @param {Object} handlers - 事件处理器
     * @returns {WebSocket} WebSocket实例
     */
    static createWebSocketConnection(url, handlers = {}) {
        if (!this.isFeatureAvailable('websocket')) {
            throw new Error('浏览器不支持WebSocket');
        }

        const ws = new WebSocket(url);

        // 绑定事件处理器
        if (handlers.onOpen) ws.onopen = handlers.onOpen;
        if (handlers.onMessage) ws.onmessage = handlers.onMessage;
        if (handlers.onClose) ws.onclose = handlers.onClose;
        if (handlers.onError) ws.onerror = handlers.onError;

        // 默认错误处理
        if (!handlers.onError) {
            ws.onerror = (error) => {
                console.error('WebSocket错误:', error);
                this.showError('WebSocket连接错误');
            };
        }

        return ws;
    }

    // ========================================
    // 数据处理功能
    // ========================================

    /**
     * HTML转义
     */
    static escapeHtml(text) {
        if (typeof text !== 'string') return '';

        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /**
     * 反转义HTML
     */
    static unescapeHtml(html) {
        const div = document.createElement('div');
        div.innerHTML = html;
        return div.textContent || div.innerText;
    }

    /**
     * 深拷贝对象
     */
    static deepClone(obj) {
        if (obj === null || typeof obj !== 'object') return obj;
        if (obj instanceof Date) return new Date(obj.getTime());
        if (obj instanceof Array) return obj.map(item => this.deepClone(item));
        if (typeof obj === 'object') {
            const clonedObj = {};
            for (const key in obj) {
                if (obj.hasOwnProperty(key)) {
                    clonedObj[key] = this.deepClone(obj[key]);
                }
            }
            return clonedObj;
        }
    }

    /**
     * 合并对象（深度合并）
     */
    static deepMerge(target, ...sources) {
        if (!sources.length) return target;
        const source = sources.shift();

        if (this.isObject(target) && this.isObject(source)) {
            for (const key in source) {
                if (this.isObject(source[key])) {
                    if (!target[key]) Object.assign(target, { [key]: {} });
                    this.deepMerge(target[key], source[key]);
                } else {
                    Object.assign(target, { [key]: source[key] });
                }
            }
        }

        return this.deepMerge(target, ...sources);
    }

    /**
     * 检查是否为对象
     */
    static isObject(item) {
        return item && typeof item === 'object' && !Array.isArray(item);
    }

    /**
     * 验证数据结构
     * @param {any} data - 要验证的数据
     * @param {Object} schema - 验证模式
     * @returns {Object} 验证结果
     */
    static validateData(data, schema) {
        const errors = [];

        const validate = (value, rules, path = '') => {
            if (rules.required && (value === undefined || value === null)) {
                errors.push(`${path}是必需的`);
                return;
            }

            if (value === undefined || value === null) return;

            if (rules.type && typeof value !== rules.type) {
                errors.push(`${path}类型错误，期望${rules.type}，实际${typeof value}`);
                return;
            }

            if (rules.minLength && value.length < rules.minLength) {
                errors.push(`${path}长度不能少于${rules.minLength}`);
            }

            if (rules.maxLength && value.length > rules.maxLength) {
                errors.push(`${path}长度不能超过${rules.maxLength}`);
            }

            if (rules.pattern && !rules.pattern.test(value)) {
                errors.push(`${path}格式不正确`);
            }

            if (rules.enum && !rules.enum.includes(value)) {
                errors.push(`${path}值不在允许范围内`);
            }

            if (rules.properties && typeof value === 'object') {
                Object.entries(rules.properties).forEach(([key, subRules]) => {
                    validate(value[key], subRules, path ? `${path}.${key}` : key);
                });
            }
        };

        validate(data, schema);

        return {
            valid: errors.length === 0,
            errors
        };
    }

    // ========================================
    // 时间和格式化功能
    // ========================================

    /**
     * 格式化时间
     */
    static formatTime(timestamp, format = 'HH:mm:ss') {
        const date = new Date(timestamp);

        if (format === 'HH:mm:ss') {
            return date.toLocaleTimeString('zh-CN', {
                hour12: false,
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit'
            });
        }

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

        return date.toLocaleString('zh-CN');
    }

    /**
     * 格式化相对时间
     */
    static formatRelativeTime(timestamp) {
        const now = new Date();
        const date = new Date(timestamp);
        const diff = now - date;

        const seconds = Math.floor(diff / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);

        if (seconds < 60) return '刚刚';
        if (minutes < 60) return `${minutes}分钟前`;
        if (hours < 24) return `${hours}小时前`;
        if (days < 7) return `${days}天前`;

        return this.formatTime(timestamp, 'YYYY-MM-DD');
    }

    /**
     * 格式化数字
     */
    static formatNumber(num, decimals = 2) {
        return Number(num).toLocaleString('zh-CN', {
            minimumFractionDigits: decimals,
            maximumFractionDigits: decimals
        });
    }

    /**
     * 转义正则表达式特殊字符
     * @param {string} string - 要转义的字符串  
     * @returns {string} 转义后的字符串，可以安全地用于正则表达式
     */
    static escapeRegex(string) {
        if (string === null || string === undefined) {
            return '';
        }

        // 确保输入是字符串
        const str = String(string);

        // 转义所有正则表达式特殊字符
        // . * + ? ^ $ { } ( ) | [ ] \
        return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }

    /**
     * 转义HTML特殊字符并处理正则表达式 - 组合方法
     * @param {string} text - 要处理的文本
     * @returns {string} 转义后的文本
     */
    static escapeHtmlAndRegex(text) {
        return this.escapeRegex(this.escapeHtml(text));
    }

    /**
     * 安全创建正则表达式
     * @param {string} pattern - 正则表达式模式
     * @param {string} flags - 正则表达式标志（如 'gi'）
     * @returns {RegExp|null} 正则表达式对象，创建失败返回null
     */
    static createSafeRegex(pattern, flags = '') {
        try {
            const escapedPattern = this.escapeRegex(pattern);
            return new RegExp(escapedPattern, flags);
        } catch (error) {
            console.warn('创建正则表达式失败:', error);
            return null;
        }
    }

    /**
     * 高亮文本中的关键词 - 通用版本
     * @param {string} text - 要处理的文本
     * @param {string|string[]} keywords - 关键词（字符串或数组）
     * @param {string} highlightClass - 高亮样式类名（默认使用mark标签）
     * @returns {string} 处理后的HTML文本
     */
    static highlightKeywords(text, keywords, highlightClass = null) {
        if (!text || !keywords) return text;

        // 确保关键词是数组
        const keywordArray = Array.isArray(keywords) ? keywords : [keywords];

        let highlightedText = text;

        keywordArray.forEach(keyword => {
            if (!keyword || typeof keyword !== 'string') return;

            const regex = new RegExp(`(${this.escapeRegex(keyword)})`, 'gi');

            if (highlightClass) {
                highlightedText = highlightedText.replace(
                    regex,
                    `<span class="${highlightClass}">$1</span>`
                );
            } else {
                highlightedText = highlightedText.replace(regex, '<mark>$1</mark>');
            }
        });

        return highlightedText;
    }

    /**
     * 搜索文本并返回匹配信息
     * @param {string} text - 要搜索的文本
     * @param {string} searchTerm - 搜索词
     * @param {boolean} caseSensitive - 是否区分大小写
     * @returns {object} 搜索结果信息
     */
    static searchText(text, searchTerm, caseSensitive = false) {
        // 验证输入参数
        if (typeof text !== 'string' || typeof searchTerm !== 'string') {
            console.warn('搜索文本或搜索词类型错误', { text: typeof text, searchTerm: typeof searchTerm });
            return { matches: [], count: 0, positions: [] };
        }

        if (!text || !searchTerm) {
            return { matches: [], count: 0, positions: [] };
        }

        // 防止正则表达式拒绝服务攻击，限制搜索词长度
        if (searchTerm.length > 100) {
            console.warn('搜索词过长，已截断', searchTerm);
            searchTerm = searchTerm.substring(0, 100);
        }

        const flags = caseSensitive ? 'g' : 'gi';
        let regex;
        try {
            regex = new RegExp(this.escapeRegex(searchTerm), flags);
        } catch (error) {
            console.error('正则表达式创建失败:', error);
            return { matches: [], count: 0, positions: [] };
        }

        const matches = [];
        const positions = [];
        let match;

        // 限制匹配数量防止长时间运行
        const maxMatches = 1000;
        while ((match = regex.exec(text)) !== null) {
            matches.push(match[0]);
            positions.push({
                start: match.index,
                end: match.index + match[0].length,
                text: match[0]
            });

            // 防止无限循环
            if (match.index === regex.lastIndex) {
                regex.lastIndex++;
            }

            // 限制匹配数量
            if (matches.length >= maxMatches) {
                console.warn(`匹配数量达到上限 ${maxMatches}，停止搜索`);
                break;
            }
        }

        return {
            matches: matches,
            count: matches.length,
            positions: positions,
            hasMatches: matches.length > 0
        };
    }

    // ========================================
    // 异步工具功能
    // ========================================

    /**
     * 防抖函数
     */
    static debounce(func, wait, immediate = false) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                timeout = null;
                if (!immediate) func(...args);
            };
            const callNow = immediate && !timeout;
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
            if (callNow) func(...args);
        };
    }

    /**
     * 节流函数
     */
    static throttle(func, limit) {
        let inThrottle;
        return function (...args) {
            if (!inThrottle) {
                func.apply(this, args);
                inThrottle = true;
                setTimeout(() => inThrottle = false, limit);
            }
        };
    }

    /**
     * 等待指定时间
     */
    static sleep(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }

    /**
     * 重试函数
     */
    static async retry(fn, options = {}) {
        const {
            retries = 3,
            delay = 1000,
            backoff = 2,
            onRetry = null
        } = options;

        let lastError;

        for (let i = 0; i <= retries; i++) {
            try {
                return await fn();
            } catch (error) {
                lastError = error;

                if (i === retries) break;

                if (onRetry) {
                    onRetry(error, i + 1);
                }

                await this.sleep(delay * Math.pow(backoff, i));
            }
        }

        throw lastError;
    }

    // ========================================
    // 浏览器和设备功能
    // ========================================

    /**
     * 复制文本到剪贴板
     */
    static async copyToClipboard(text) {
        if (this.isFeatureAvailable('clipboard')) {
            try {
                await navigator.clipboard.writeText(text);
                this.showSuccess('已复制到剪贴板');
                return true;
            } catch (error) {
                console.error('复制失败:', error);
            }
        }

        // 降级方案
        try {
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
            this.showSuccess('已复制到剪贴板');
            return true;
        } catch (error) {
            this.showError('复制失败');
            return false;
        }
    }

    /**
     * 生成UUID
     */
    static generateUUID() {
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
            const r = Math.random() * 16 | 0;
            const v = c === 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        });
    }

    /**
     * 生成随机ID
     */
    static generateId(prefix = 'id') {
        return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 11)}`;
    }

    /**
     * 检查是否为移动设备
     */
    static isMobile() {
        return /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent);
    }

    /**
     * 获取URL参数
     */
    static getUrlParams() {
        const params = {};
        const urlParams = new URLSearchParams(window.location.search);
        for (const [key, value] of urlParams) {
            params[key] = value;
        }
        return params;
    }

    /**
     * 设置URL参数
     */
    static setUrlParam(key, value) {
        const url = new URL(window.location);
        url.searchParams.set(key, value);
        window.history.pushState({}, '', url);
    }

    /**
     * 滚动到指定元素
     */
    static scrollToElement(element, behavior = 'smooth') {
        if (typeof element === 'string') {
            element = document.querySelector(element);
        }

        if (element) {
            element.scrollIntoView({ behavior, block: 'start' });
        }
    }

    /**
     * 检查元素是否在视窗内
     */
    static isElementInViewport(element) {
        if (typeof element === 'string') {
            element = document.querySelector(element);
        }

        if (!element) return false;

        const rect = element.getBoundingClientRect();
        return (
            rect.top >= 0 &&
            rect.left >= 0 &&
            rect.bottom <= (window.innerHeight || document.documentElement.clientHeight) &&
            rect.right <= (window.innerWidth || document.documentElement.clientWidth)
        );
    }

    // ========================================
    // 加载和资源管理
    // ========================================

    /**
     * 加载外部脚本
     */
    static loadScript(src) {
        return new Promise((resolve, reject) => {
            const existing = document.querySelector(`script[src="${src}"]`);
            if (existing) {
                resolve();
                return;
            }

            const script = document.createElement('script');
            script.src = src;
            script.onload = resolve;
            script.onerror = () => reject(new Error(`Failed to load script: ${src}`));
            document.head.appendChild(script);
        });
    }

    /**
     * 加载外部CSS
     */
    static loadCSS(href) {
        return new Promise((resolve, reject) => {
            const existing = document.querySelector(`link[href="${href}"]`);
            if (existing) {
                resolve();
                return;
            }

            const link = document.createElement('link');
            link.rel = 'stylesheet';
            link.href = href;
            link.onload = resolve;
            link.onerror = () => reject(new Error(`Failed to load CSS: ${href}`));
            document.head.appendChild(link);
        });
    }

    // ========================================
    // 调试和开发工具
    // ========================================

    /**
     * 获取依赖状态报告
     */
    static getDependencyReport() {
        const coreFeatures = ['api', 'utils', 'fetch', 'localStorage'];
        const optionalFeatures = ['toast', 'modal', 'story', 'export', 'websocket', 'clipboard'];

        const report = {
            timestamp: new Date().toISOString(),
            core: {},
            optional: {},
            summary: {
                coreReady: true,
                optionalCount: 0,
                issues: []
            }
        };

        // 检查核心功能
        coreFeatures.forEach(feature => {
            const available = this.isFeatureAvailable(feature);
            report.core[feature] = available;

            if (!available) {
                report.summary.coreReady = false;
                report.summary.issues.push(`核心功能缺失: ${feature}`);
            }
        });

        // 检查可选功能
        optionalFeatures.forEach(feature => {
            const available = this.isFeatureAvailable(feature);
            report.optional[feature] = available;

            if (available) {
                report.summary.optionalCount++;
            }
        });

        return report;
    }

    /**
     * 打印系统信息
     */
    static printSystemInfo() {
        const info = {
            userAgent: navigator.userAgent,
            screen: `${screen.width}x${screen.height}`,
            viewport: `${window.innerWidth}x${window.innerHeight}`,
            device: this.isMobile() ? 'mobile' : 'desktop',
            features: this.getDependencyReport(),
            location: window.location.href,
            timestamp: new Date().toISOString()
        };

        console.group('🔧 系统信息');
        console.table(info);
        console.groupEnd();

        return info;
    }
}

// 确保全局可用
window.Utils = Utils;

// 导出模块（如果支持）
if (typeof module !== 'undefined' && module.exports) {
    module.exports = Utils;
}

// 开发环境调试工具
if (typeof window !== 'undefined' &&
    (window.location?.hostname === 'localhost' || window.location?.search.includes('debug=1'))) {

    window.UTILS_DEBUG = {
        // 列出所有可用方法
        listMethods: () => {
            const methods = [];
            for (const key of Object.getOwnPropertyNames(Utils)) {
                if (typeof Utils[key] === 'function' && key !== 'constructor') {
                    methods.push(key);
                }
            }
            return methods.sort();
        },

        // 功能检查
        checkFeatures: () => Utils.getDependencyReport(),

        // 系统信息
        systemInfo: () => Utils.printSystemInfo(),

        // 测试工具
        test: {
            toast: () => {
                Utils.showSuccess('成功消息测试');
                Utils.showError('错误消息测试');
                Utils.showWarning('警告消息测试');
                Utils.showInfo('信息消息测试');
            },

            confirm: async () => {
                const result = await Utils.showConfirm('这是一个确认对话框测试');
                Utils.showInfo(`确认结果: ${result}`);
            },

            fileSize: () => {
                console.log('文件大小格式化测试:');
                console.log('1024 bytes =', Utils.formatFileSize(1024));
                console.log('1048576 bytes =', Utils.formatFileSize(1048576));
                console.log('1073741824 bytes =', Utils.formatFileSize(1073741824));
            }
        }
    };

    console.log('🔧 Utils调试模式已启用');
    console.log('使用 window.UTILS_DEBUG 查看调试工具');
    console.log('可用方法数量:', window.UTILS_DEBUG.listMethods().length);
}
