/**
 * 导出管理器
 * 基于后端 ExportService API 设计
 * 支持多种格式导出和进度显示
 */
class ExportManager {
    constructor() {
        this.exportOptions = {
            type: 'scene',           // scene, interactions, story
            format: 'json',          // json, markdown, html, txt
            includeConversations: true,
            includeAnalytics: true,
            optimizeSize: false
        };
        
        this.isExporting = false;
        this.currentProgress = 0;
        this.progressCallback = null;
        this.completeCallback = null;
        this.errorCallback = null;
        
        // 支持的导出类型配置
        this.exportTypes = {
            scene: {
                name: '场景数据',
                description: '完整的场景信息，包括角色、地点、道具等',
                icon: 'bi-geo-alt',
                color: 'primary',
                endpoint: '/api/scenes/{sceneId}/export',
                estimatedSize: 'large'
            },
            interactions: {
                name: '互动摘要', 
                description: '对话记录和互动分析报告',
                icon: 'bi-chat-dots',
                color: 'success',
                endpoint: '/api/scenes/{sceneId}/export/interactions',
                estimatedSize: 'medium'
            },
            story: {
                name: '故事文档',
                description: '故事节点、任务和剧情发展',
                icon: 'bi-book',
                color: 'warning',
                endpoint: '/api/scenes/{sceneId}/export/story',
                estimatedSize: 'medium'
            }
        };
        
        // 支持的格式配置
        this.formats = {
            json: {
                name: 'JSON',
                description: '适合程序处理',
                icon: 'bi-filetype-json',
                mimeType: 'application/json',
                extension: 'json',
                sizeMultiplier: 1.0
            },
            markdown: {
                name: 'Markdown',
                description: '适合文档编辑',
                icon: 'bi-markdown',
                mimeType: 'text/markdown',
                extension: 'md',
                sizeMultiplier: 1.2
            },
            html: {
                name: 'HTML',
                description: '适合查看和分享',
                icon: 'bi-filetype-html',
                mimeType: 'text/html',
                extension: 'html',
                sizeMultiplier: 2.0
            },
            txt: {
                name: '纯文本',
                description: '兼容性最佳',
                icon: 'bi-file-text',
                mimeType: 'text/plain',
                extension: 'txt',
                sizeMultiplier: 0.8
            }
        };
        
        this.init();
    }
    
    /**
     * 初始化导出管理器
     */
    init() {
        this.bindEvents();
        console.log('📤 ExportManager 已初始化');
    }
    
    /**
     * 绑定事件监听器
     */
    bindEvents() {
        // 监听导出按钮点击
        document.addEventListener('click', (e) => {
            if (e.target.matches('.export-btn, .export-btn *')) {
                e.preventDefault();
                const btn = e.target.closest('.export-btn');
                const type = btn?.dataset.type;
                const format = btn?.dataset.format;
                
                if (type) {
                    this.quickExport(type, format);
                } else {
                    this.showExportModal();
                }
            }
            
            // 导出模态框内的按钮
            if (e.target.matches('#start-export-btn')) {
                this.startExport();
            }
            
            if (e.target.matches('.export-type-option')) {
                this.selectExportType(e.target.dataset.type);
            }
            
            if (e.target.matches('.format-option')) {
                this.selectFormat(e.target.dataset.format);
            }
        });
        
        // 监听表单变化
        document.addEventListener('change', (e) => {
            if (e.target.matches('#export-modal input, #export-modal select')) {
                this.updateExportOptions();
                this.updatePreview();
            }
        });
    }
    
    /**
     * 显示导出模态框
     */
    showExportModal() {
        const modal = this.createExportModal();
        this.showModal(modal);
        this.updatePreview();
    }
    
    /**
     * 创建导出模态框
     */
    createExportModal() {
        // 移除已存在的模态框
        const existing = document.getElementById('export-modal');
        if (existing) {
            existing.remove();
        }
        
        const modal = document.createElement('div');
        modal.id = 'export-modal';
        modal.className = 'modal fade';
        modal.innerHTML = `
            <div class="modal-dialog modal-lg">
                <div class="modal-content">
                    <div class="modal-header">
                        <h5 class="modal-title">
                            <i class="bi bi-download"></i> 数据导出
                        </h5>
                        <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                    </div>
                    <div class="modal-body">
                        ${this.renderExportOptions()}
                        ${this.renderFormatSelection()}
                        ${this.renderAdvancedOptions()}
                        ${this.renderPreviewSection()}
                        ${this.renderProgressSection()}
                    </div>
                    <div class="modal-footer">
                        <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
                        <button type="button" class="btn btn-primary" id="start-export-btn">
                            <i class="bi bi-download"></i> 开始导出
                        </button>
                    </div>
                </div>
            </div>
        `;
        
        document.body.appendChild(modal);
        return modal;
    }
    
    /**
     * 渲染导出选项
     */
    renderExportOptions() {
        return `
            <div class="export-section mb-4">
                <h6 class="mb-3">
                    <i class="bi bi-collection"></i> 选择导出类型
                </h6>
                <div class="row g-3">
                    ${Object.entries(this.exportTypes).map(([key, type]) => `
                        <div class="col-md-4">
                            <div class="export-type-option card h-100 ${key === this.exportOptions.type ? 'border-primary bg-light' : ''}" 
                                 data-type="${key}" style="cursor: pointer;">
                                <div class="card-body text-center">
                                    <i class="${type.icon} fs-1 text-${type.color} mb-2"></i>
                                    <h6 class="card-title">${type.name}</h6>
                                    <p class="card-text small text-muted">${type.description}</p>
                                    <span class="badge bg-${type.color} bg-opacity-10 text-${type.color}">
                                        ${this.getSizeLabel(type.estimatedSize)}
                                    </span>
                                </div>
                            </div>
                        </div>
                    `).join('')}
                </div>
            </div>
        `;
    }
    
    /**
     * 渲染格式选择
     */
    renderFormatSelection() {
        return `
            <div class="export-section mb-4">
                <h6 class="mb-3">
                    <i class="bi bi-file-earmark"></i> 选择导出格式
                </h6>
                <div class="row g-3">
                    ${Object.entries(this.formats).map(([key, format]) => `
                        <div class="col-md-3">
                            <div class="format-option card h-100 ${key === this.exportOptions.format ? 'border-success bg-light' : ''}" 
                                 data-format="${key}" style="cursor: pointer;">
                                <div class="card-body text-center">
                                    <i class="${format.icon} fs-2 mb-2"></i>
                                    <h6 class="card-title">${format.name}</h6>
                                    <p class="card-text small text-muted">${format.description}</p>
                                </div>
                            </div>
                        </div>
                    `).join('')}
                </div>
            </div>
        `;
    }
    
    /**
     * 渲染高级选项
     */
    renderAdvancedOptions() {
        return `
            <div class="export-section mb-4">
                <h6 class="mb-3">
                    <i class="bi bi-gear"></i> 高级选项
                </h6>
                <div class="row g-3">
                    <div class="col-md-6">
                        <div class="form-check">
                            <input class="form-check-input" type="checkbox" id="include-conversations" 
                                   ${this.exportOptions.includeConversations ? 'checked' : ''}>
                            <label class="form-check-label" for="include-conversations">
                                <i class="bi bi-chat-dots"></i> 包含对话记录
                                <small class="d-block text-muted">导出完整的对话历史</small>
                            </label>
                        </div>
                    </div>
                    <div class="col-md-6">
                        <div class="form-check">
                            <input class="form-check-input" type="checkbox" id="include-analytics" 
                                   ${this.exportOptions.includeAnalytics ? 'checked' : ''}>
                            <label class="form-check-label" for="include-analytics">
                                <i class="bi bi-graph-up"></i> 包含数据分析
                                <small class="d-block text-muted">添加统计图表和洞察</small>
                            </label>
                        </div>
                    </div>
                    <div class="col-md-6">
                        <div class="form-check">
                            <input class="form-check-input" type="checkbox" id="optimize-size" 
                                   ${this.exportOptions.optimizeSize ? 'checked' : ''}>
                            <label class="form-check-label" for="optimize-size">
                                <i class="bi bi-archive"></i> 优化文件大小
                                <small class="d-block text-muted">压缩内容以减小文件体积</small>
                            </label>
                        </div>
                    </div>
                    <div class="col-md-6">
                        <div class="form-check">
                            <input class="form-check-input" type="checkbox" id="include-metadata">
                            <label class="form-check-label" for="include-metadata">
                                <i class="bi bi-info-circle"></i> 包含元数据
                                <small class="d-block text-muted">添加导出时间、版本等信息</small>
                            </label>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }
    
    /**
     * 渲染预览部分
     */
    renderPreviewSection() {
        return `
            <div class="export-section mb-4" id="export-preview-section" style="display: none;">
                <h6 class="mb-3">
                    <i class="bi bi-eye"></i> 导出预览
                </h6>
                <div class="alert alert-info" id="export-preview">
                    <div id="preview-content"></div>
                </div>
            </div>
        `;
    }
    
    /**
     * 渲染进度部分
     */
    renderProgressSection() {
        return `
            <div class="export-section" id="export-progress-section" style="display: none;">
                <h6 class="mb-3">
                    <i class="bi bi-hourglass-split"></i> 导出进度
                </h6>
                <div class="progress mb-3">
                    <div class="progress-bar progress-bar-striped progress-bar-animated" 
                         role="progressbar" style="width: 0%" id="export-progress-bar">
                        <span id="progress-percentage">0%</span>
                    </div>
                </div>
                <div class="d-flex justify-content-between align-items-center">
                    <small class="text-muted" id="progress-status">准备中...</small>
                    <small class="text-muted" id="progress-eta"></small>
                </div>
            </div>
        `;
    }
    
    /**
     * 选择导出类型
     */
    selectExportType(type) {
        this.exportOptions.type = type;
        
        // 更新UI
        document.querySelectorAll('.export-type-option').forEach(option => {
            option.classList.remove('border-primary', 'bg-light');
        });
        
        const selectedOption = document.querySelector(`[data-type="${type}"]`);
        if (selectedOption) {
            selectedOption.classList.add('border-primary', 'bg-light');
        }
        
        this.updatePreview();
    }
    
    /**
     * 选择格式
     */
    selectFormat(format) {
        this.exportOptions.format = format;
        
        // 更新UI
        document.querySelectorAll('.format-option').forEach(option => {
            option.classList.remove('border-success', 'bg-light');
        });
        
        const selectedOption = document.querySelector(`[data-format="${format}"]`);
        if (selectedOption) {
            selectedOption.classList.add('border-success', 'bg-light');
        }
        
        this.updatePreview();
    }
    
    /**
     * 更新导出选项
     */
    updateExportOptions() {
        const modal = document.getElementById('export-modal');
        if (!modal) return;
        
        this.exportOptions.includeConversations = modal.querySelector('#include-conversations')?.checked || false;
        this.exportOptions.includeAnalytics = modal.querySelector('#include-analytics')?.checked || false;
        this.exportOptions.optimizeSize = modal.querySelector('#optimize-size')?.checked || false;
    }
    
    /**
     * 更新预览
     */
    updatePreview() {
        const previewSection = document.getElementById('export-preview-section');
        const previewContent = document.getElementById('preview-content');
        
        if (!previewSection || !previewContent) return;
        
        const typeConfig = this.exportTypes[this.exportOptions.type];
        const formatConfig = this.formats[this.exportOptions.format];
        
        if (!typeConfig || !formatConfig) {
            previewSection.style.display = 'none';
            return;
        }
        
        const estimatedSize = this.estimateFileSize();
        const sceneId = this.getCurrentSceneId();
        
        previewContent.innerHTML = `
            <div class="row g-3">
                <div class="col-md-6">
                    <h6 class="mb-2">导出内容</h6>
                    <p class="mb-1"><strong>类型:</strong> ${typeConfig.name}</p>
                    <p class="mb-1"><strong>格式:</strong> ${formatConfig.name}</p>
                    <p class="mb-1"><strong>场景:</strong> ${sceneId || '当前场景'}</p>
                    <p class="mb-0"><strong>描述:</strong> ${typeConfig.description}</p>
                </div>
                <div class="col-md-6">
                    <h6 class="mb-2">文件信息</h6>
                    <p class="mb-1"><strong>预估大小:</strong> ${estimatedSize}</p>
                    <p class="mb-1"><strong>文件类型:</strong> ${formatConfig.extension.toUpperCase()}</p>
                    <p class="mb-1"><strong>MIME类型:</strong> ${formatConfig.mimeType}</p>
                    <p class="mb-0"><strong>包含:</strong> ${this.getIncludedFeatures().join(', ')}</p>
                </div>
            </div>
        `;
        
        previewSection.style.display = 'block';
    }
    
    /**
     * 估算文件大小
     */
    estimateFileSize() {
        const typeConfig = this.exportTypes[this.exportOptions.type];
        const formatConfig = this.formats[this.exportOptions.format];
        
        if (!typeConfig || !formatConfig) return '未知';
        
        // 基础大小估算（KB）
        let baseSize = 0;
        switch (this.exportOptions.type) {
            case 'scene':
                baseSize = 100; // 场景数据基础大小
                break;
            case 'interactions':
                baseSize = 50; // 互动摘要基础大小
                break;
            case 'story':
                baseSize = 80; // 故事文档基础大小
                break;
        }
        
        // 格式系数
        baseSize *= formatConfig.sizeMultiplier;
        
        // 选项增加
        if (this.exportOptions.includeConversations) {
            baseSize *= 1.5;
        }
        if (this.exportOptions.includeAnalytics) {
            baseSize *= 1.3;
        }
        if (this.exportOptions.optimizeSize) {
            baseSize *= 0.7;
        }
        
        return this.formatFileSize(baseSize * 1024);
    }
    
    /**
     * 获取包含的功能列表
     */
    getIncludedFeatures() {
        const features = ['基础数据'];
        
        if (this.exportOptions.includeConversations) {
            features.push('对话记录');
        }
        if (this.exportOptions.includeAnalytics) {
            features.push('数据分析');
        }
        if (this.exportOptions.optimizeSize) {
            features.push('大小优化');
        }
        
        return features;
    }
    
    /**
     * 开始导出
     */
    async startExport() {
        if (this.isExporting) return;
        
        try {
            this.isExporting = true;
            this.showProgress();
            this.updateProgress(0, '准备导出...');
            
            const sceneId = this.getCurrentSceneId();
            if (!sceneId) {
                throw new Error('未找到场景ID');
            }
            
            // 禁用开始按钮
            const startBtn = document.getElementById('start-export-btn');
            if (startBtn) {
                startBtn.disabled = true;
                startBtn.innerHTML = '<i class="bi bi-hourglass-split"></i> 导出中...';
            }
            
            // 执行导出
            const result = await this.executeExport(sceneId);
            
            // 完成导出
            this.updateProgress(100, '导出完成');
            this.downloadFile(result);
            
            // 显示成功消息
            this.showSuccess('导出成功完成');
            
            // 关闭模态框
            setTimeout(() => {
                this.hideModal();
            }, 1500);
            
        } catch (error) {
            this.updateProgress(0, '导出失败: ' + error.message);
            this.showError('导出失败: ' + error.message);
            
            // 重新启用按钮
            const startBtn = document.getElementById('start-export-btn');
            if (startBtn) {
                startBtn.disabled = false;
                startBtn.innerHTML = '<i class="bi bi-download"></i> 开始导出';
            }
        } finally {
            this.isExporting = false;
        }
    }
    
    /**
     * 执行导出
     */
    async executeExport(sceneId) {
        this.updateProgress(10, '连接服务器...');
        
        // 构建API URL
        const typeConfig = this.exportTypes[this.exportOptions.type];
        let url = typeConfig.endpoint.replace('{sceneId}', sceneId);
        
        // 添加参数
        const params = new URLSearchParams({
            format: this.exportOptions.format,
            include_conversations: this.exportOptions.includeConversations,
            include_analytics: this.exportOptions.includeAnalytics,
            optimize_size: this.exportOptions.optimizeSize
        });
        
        url += '?' + params.toString();
        
        this.updateProgress(30, '发送请求...');
        
        // 发送请求
        const response = await fetch(url, {
            method: 'GET',
            headers: {
                'Accept': 'application/json',
                'Content-Type': 'application/json'
            }
        });
        
        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`);
        }
        
        this.updateProgress(60, '处理数据...');
        
        const result = await response.json();
        
        this.updateProgress(90, '准备下载...');
        
        return result;
    }
    
    /**
     * 快速导出
     */
    async quickExport(type, format = 'html') {
        try {
            this.exportOptions.type = type;
            this.exportOptions.format = format;
            
            const sceneId = this.getCurrentSceneId();
            if (!sceneId) {
                throw new Error('未找到场景ID');
            }
            
            this.showSuccess('正在导出数据...');
            
            const result = await this.executeExport(sceneId);
            this.downloadFile(result);
            
            this.showSuccess('导出成功完成');
            
        } catch (error) {
            this.showError('导出失败: ' + error.message);
        }
    }
    
    /**
     * 下载文件
     */
    downloadFile(result) {
        if (!result || !result.content) {
            throw new Error('导出结果无效');
        }
        
        const formatConfig = this.formats[this.exportOptions.format];
        const typeConfig = this.exportTypes[this.exportOptions.type];
        
        // 创建文件名
        const timestamp = new Date().toISOString().split('T')[0];
        const sceneId = this.getCurrentSceneId() || 'scene';
        const fileName = `${sceneId}_${this.exportOptions.type}_${timestamp}.${formatConfig.extension}`;
        
        // 创建下载
        const blob = new Blob([result.content], { type: formatConfig.mimeType });
        const url = URL.createObjectURL(blob);
        
        const a = document.createElement('a');
        a.href = url;
        a.download = fileName;
        a.style.display = 'none';
        
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        
        URL.revokeObjectURL(url);
        
        // 触发回调
        if (this.completeCallback) {
            this.completeCallback(result, fileName);
        }
    }
    
    /**
     * 显示进度
     */
    showProgress() {
        const progressSection = document.getElementById('export-progress-section');
        if (progressSection) {
            progressSection.style.display = 'block';
        }
    }
    
    /**
     * 更新进度
     */
    updateProgress(percentage, status, eta = null) {
        this.currentProgress = percentage;
        
        const progressBar = document.getElementById('export-progress-bar');
        const progressPercentage = document.getElementById('progress-percentage');
        const progressStatus = document.getElementById('progress-status');
        const progressEta = document.getElementById('progress-eta');
        
        if (progressBar) {
            progressBar.style.width = `${percentage}%`;
            progressBar.setAttribute('aria-valuenow', percentage);
        }
        
        if (progressPercentage) {
            progressPercentage.textContent = `${percentage}%`;
        }
        
        if (progressStatus) {
            progressStatus.textContent = status;
        }
        
        if (progressEta && eta) {
            progressEta.textContent = `预计剩余: ${eta}`;
        }
        
        // 触发回调
        if (this.progressCallback) {
            this.progressCallback(percentage, status, eta);
        }
    }
    
    /**
     * 获取当前场景ID
     */
    getCurrentSceneId() {
        // 尝试多种方式获取场景ID
        
        // 从URL路径获取
        const pathMatch = window.location.pathname.match(/\/scenes\/([^\/]+)/);
        if (pathMatch) {
            return pathMatch[1];
        }
        
        // 从URL参数获取
        const urlParams = new URLSearchParams(window.location.search);
        const sceneParam = urlParams.get('scene') || urlParams.get('sceneId') || urlParams.get('scene_id');
        if (sceneParam) {
            return sceneParam;
        }
        
        // 从全局变量获取
        if (typeof window.sceneId !== 'undefined') {
            return window.sceneId;
        }
        
        if (typeof window.app !== 'undefined' && window.app.currentScene) {
            return window.app.currentScene.id;
        }
        
        // 从页面元素获取
        const sceneElement = document.querySelector('[data-scene-id]');
        if (sceneElement) {
            return sceneElement.dataset.sceneId;
        }
        
        return null;
    }
    
    /**
     * 获取大小标签
     */
    getSizeLabel(size) {
        const labels = {
            small: '小文件',
            medium: '中等文件',
            large: '大文件'
        };
        return labels[size] || '未知大小';
    }
    
    /**
     * 格式化文件大小
     */
    formatFileSize(bytes) {
        if (bytes === 0) return '0 Bytes';
        
        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }
    
    /**
     * 显示模态框
     */
    showModal(modal) {
        if (typeof bootstrap !== 'undefined') {
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
    hideModal() {
        const modal = document.getElementById('export-modal');
        if (!modal) return;
        
        if (typeof bootstrap !== 'undefined') {
            const bsModal = bootstrap.Modal.getInstance(modal);
            if (bsModal) {
                bsModal.hide();
            }
        } else {
            modal.style.display = 'none';
            modal.classList.remove('show');
        }
    }
    
    /**
     * 显示成功消息
     */
    showSuccess(message) {
        if (typeof Utils !== 'undefined' && Utils.showSuccess) {
            Utils.showSuccess(message);
        } else {
            console.log('✅ Export Success:', message);
        }
    }
    
    /**
     * 显示错误消息
     */
    showError(message) {
        if (typeof Utils !== 'undefined' && Utils.showError) {
            Utils.showError(message);
        } else {
            console.error('❌ Export Error:', message);
            alert('导出错误: ' + message);
        }
    }
    
    /**
     * 设置进度回调
     */
    onProgress(callback) {
        this.progressCallback = callback;
        return this;
    }
    
    /**
     * 设置完成回调
     */
    onComplete(callback) {
        this.completeCallback = callback;
        return this;
    }
    
    /**
     * 设置错误回调
     */
    onError(callback) {
        this.errorCallback = callback;
        return this;
    }
    
    /**
     * 重置导出状态
     */
    reset() {
        this.isExporting = false;
        this.currentProgress = 0;
        
        // 重置UI
        const progressSection = document.getElementById('export-progress-section');
        if (progressSection) {
            progressSection.style.display = 'none';
        }
        
        const startBtn = document.getElementById('start-export-btn');
        if (startBtn) {
            startBtn.disabled = false;
            startBtn.innerHTML = '<i class="bi bi-download"></i> 开始导出';
        }
    }
    
    /**
     * 获取导出状态
     */
    getStatus() {
        return {
            isExporting: this.isExporting,
            progress: this.currentProgress,
            options: { ...this.exportOptions }
        };
    }
    
    /**
     * 销毁导出管理器
     */
    destroy() {
        // 清理事件监听器
        const modal = document.getElementById('export-modal');
        if (modal) {
            modal.remove();
        }
        
        // 重置状态
        this.reset();
        
        console.log('📤 ExportManager 已销毁');
    }
}

// ========================================
// 全局初始化和便捷函数
// ========================================

/**
 * 全局导出函数（向后兼容）
 */
window.exportScene = function(format = 'html') {
    if (window.exportManager) {
        return window.exportManager.quickExport('scene', format);
    }
    console.warn('ExportManager not initialized');
};

window.exportInteractions = function(format = 'html') {
    if (window.exportManager) {
        return window.exportManager.quickExport('interactions', format);
    }
    console.warn('ExportManager not initialized');
};

window.exportStory = function(format = 'html') {
    if (window.exportManager) {
        return window.exportManager.quickExport('story', format);
    }
    console.warn('ExportManager not initialized');
};

window.showExportModal = function() {
    if (window.exportManager) {
        return window.exportManager.showExportModal();
    }
    console.warn('ExportManager not initialized');
};

// 确保在DOM加载完成后创建全局实例
if (typeof window !== 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            window.ExportManager = ExportManager;
            window.exportManager = new ExportManager();
            console.log('📤 ExportManager 已准备就绪');
        });
    } else {
        window.ExportManager = ExportManager;
        window.exportManager = new ExportManager();
        console.log('📤 ExportManager 已准备就绪');
    }
}

// CSS样式（如果需要独立样式）
if (typeof document !== 'undefined') {
    const addExportStyles = () => {
        if (document.getElementById('export-styles')) return;

        const style = document.createElement('style');
        style.id = 'export-styles';
        style.textContent = `
            /* 导出模态框样式 */
            .export-section {
                padding: 20px 0;
                border-bottom: 1px solid #e9ecef;
            }
            
            .export-section:last-child {
                border-bottom: none;
            }
            
            .export-type-option,
            .format-option {
                transition: all 0.3s ease;
                cursor: pointer;
            }
            
            .export-type-option:hover,
            .format-option:hover {
                transform: translateY(-2px);
                box-shadow: 0 4px 8px rgba(0,0,0,0.1);
            }
            
            .export-type-option.border-primary,
            .format-option.border-success {
                transform: translateY(-2px);
                box-shadow: 0 4px 12px rgba(0,0,0,0.15);
            }
            
            #export-progress-bar {
                min-width: 60px;
                text-align: center;
            }
            
            .form-check-label {
                cursor: pointer;
            }
            
            .form-check-label:hover {
                color: #495057;
            }
            
            /* 快速导出按钮样式 */
            .export-btn {
                position: relative;
                overflow: hidden;
            }
            
            .export-btn::after {
                content: '';
                position: absolute;
                top: 50%;
                left: 50%;
                width: 0;
                height: 0;
                background: rgba(255,255,255,0.3);
                border-radius: 50%;
                transform: translate(-50%, -50%);
                transition: all 0.3s ease;
            }
            
            .export-btn:hover::after {
                width: 100%;
                height: 100%;
            }
            
            /* 响应式设计 */
            @media (max-width: 768px) {
                .export-type-option,
                .format-option {
                    margin-bottom: 10px;
                }
                
                .modal-dialog {
                    margin: 10px;
                }
                
                .export-section {
                    padding: 15px 0;
                }
            }
        `;
        
        document.head.appendChild(style);
        console.log('✅ Export 样式已加载');
    };
    
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', addExportStyles);
    } else {
        addExportStyles();
    }
}

// 模块导出（如果支持）
if (typeof module !== 'undefined' && module.exports) {
    module.exports = ExportManager;
}
