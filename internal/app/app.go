// internal/app/app.go
package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/api"
	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/services"
	"github.com/Corphon/SceneIntruderMCP/internal/storage"
	"github.com/Corphon/SceneIntruderMCP/internal/utils"

	// Import LLM providers for their init() functions to register the providers
	_ "github.com/Corphon/SceneIntruderMCP/internal/llm/providers/anthropic"
	_ "github.com/Corphon/SceneIntruderMCP/internal/llm/providers/deepseek"
	_ "github.com/Corphon/SceneIntruderMCP/internal/llm/providers/githubmodels"
	_ "github.com/Corphon/SceneIntruderMCP/internal/llm/providers/glm"
	_ "github.com/Corphon/SceneIntruderMCP/internal/llm/providers/google"
	_ "github.com/Corphon/SceneIntruderMCP/internal/llm/providers/grok"
	_ "github.com/Corphon/SceneIntruderMCP/internal/llm/providers/mistral"
	_ "github.com/Corphon/SceneIntruderMCP/internal/llm/providers/nvidia"
	_ "github.com/Corphon/SceneIntruderMCP/internal/llm/providers/openai"
	_ "github.com/Corphon/SceneIntruderMCP/internal/llm/providers/openrouter"
	_ "github.com/Corphon/SceneIntruderMCP/internal/llm/providers/qwen"
)

// App 代表应用程序实例
type App struct {
	server   HTTPServer
	router   http.Handler
	config   *config.AppConfig
	stopChan chan os.Signal
}

type HTTPServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

// 全局应用实例
var instance *App

// 获取应用实例（单例模式）
func GetApp() *App {
	if instance == nil {
		instance = &App{
			stopChan: make(chan os.Signal, 1),
		}
	}
	return instance
}

// 初始化服务
func InitServices() error {
	container := di.GetContainer()

	// 1. 基础服务（无依赖）
	llmService, err := services.NewLLMService()
	if err != nil {
		// 如果LLM服务初始化失败，创建空服务作为fallback
		llmService = services.NewEmptyLLMService()
	}
	container.Register("llm", llmService)

	progressService := services.NewProgressService()
	container.Register("progress", progressService)
	progressService.StartAutoCleanup()

	// Phase1: 最小 JobQueue（异步任务执行与取消的统一底座）
	jobQueue := services.NewJobQueue(runtime.NumCPU(), 256)
	container.Register("job_queue", jobQueue)

	statsService := services.NewStatsService()
	container.Register("stats", statsService)

	configService := services.NewConfigService()
	container.Register("config", configService)

	userService := services.NewUserService()
	container.Register("user", userService)

	cfg := config.GetCurrentConfig()

	itemService := services.NewItemService(cfg.DataDir + "/scenes")
	container.Register("item", itemService)

	// 2. 依赖基础服务的服务
	sceneService := services.NewSceneService(cfg.DataDir + "/scenes")
	sceneService.ItemService = itemService
	container.Register("scene", sceneService)

	contextService := services.NewContextService(sceneService)
	container.Register("context", contextService)

	// 3. 依赖多个服务的服务
	characterService := services.NewCharacterService()
	container.Register("character", characterService)

	// 4. 高级服务（依赖前面的服务）
	storyService := services.NewStoryService(llmService)
	// 复用统一的场景/上下文/角色/物品服务，避免 storyService 内部自建导致缓存与数据源分叉
	storyService.SceneService = sceneService
	storyService.ContextService = contextService
	storyService.ItemService = itemService
	storyService.CharacterService = characterService
	storyService.BasePath = cfg.DataDir + "/stories"
	if fs, err := storage.NewFileStorage(storyService.BasePath); err == nil {
		storyService.FileStorage = fs
	}
	container.Register("story", storyService)

	analyzerService := services.NewAnalyzerServiceWithLLMService(llmService)
	container.Register("analyzer", analyzerService)

	// 5. 聚合服务（依赖多个其他服务）
	exportService := services.NewExportService(contextService, storyService, sceneService)
	container.Register("export", exportService)

	// 6. v1.4.0 scripts（新增，不影响现有 scene/story 主路径）
	scriptService, err := services.NewScriptService(cfg.DataDir+"/scripts", llmService)
	if err != nil {
		return fmt.Errorf("初始化 scripts 服务失败: %w", err)
	}
	container.Register("script", scriptService)

	// 7. v2 comics repository（最小落盘结构）
	comicRepo, err := services.NewComicRepository(cfg.DataDir + "/comics")
	if err != nil {
		return fmt.Errorf("初始化 comics 存储失败: %w", err)
	}
	container.Register("comic_repo", comicRepo)

	videoRepo, err := services.NewVideoRepository(cfg.DataDir + "/comics")
	if err != nil {
		return fmt.Errorf("初始化 video 存储失败: %w", err)
	}
	container.Register("video_repo", videoRepo)

	// 7.5 v2 vision service（Phase1：至少 1 个可用 provider）
	visionService := services.NewVisionService(comicRepo)
	visionService.Stats = statsService
	// Phase5: apply vision settings from config (best-effort, fallback to placeholder).
	if err := services.ApplyVisionConfig(visionService, cfg); err != nil {
		utils.GetLogger().Warn("failed to apply vision config; using placeholder", map[string]interface{}{"err": err.Error()})
	}
	container.Register("vision", visionService)

	// 8. v2 comics service（骨架，后续逐步落地 analyze/prompts/generate）
	comicService := services.NewComicService(
		comicRepo,
		jobQueue,
		progressService,
		llmService,
		visionService,
		sceneService,
		storyService,
	)
	container.Register("comic", comicService)

	videoService := services.NewVideoService(videoRepo, comicRepo, jobQueue, progressService, nil)
	if err := services.ApplyVideoConfig(videoService, cfg); err != nil {
		utils.GetLogger().Warn("failed to apply video config; using mock video provider", map[string]interface{}{"err": err.Error()})
	}
	container.Register("video", videoService)

	sceneAggregateService := services.NewSceneAggregateService(
		sceneService, characterService, contextService, storyService, progressService)
	container.Register("scene_aggregate", sceneAggregateService)

	interactionAggregateService := services.NewInteractionAggregateService(
		characterService,
		contextService,
		sceneService,
		statsService,
		storyService,
		exportService,
	)
	container.Register("interaction_aggregate", interactionAggregateService)

	return nil
}

// Initialize 初始化应用
func Initialize(configPath string) error {
	// 加载配置
	if err := config.InitConfig(configPath); err != nil {
		return fmt.Errorf("初始化配置失败: %w", err)
	}

	// 获取配置
	cfg := config.GetCurrentConfig()
	GetApp().config = cfg

	// 初始化日志系统
	if err := initLogger(cfg.LogDir); err != nil {
		return fmt.Errorf("初始化日志系统失败: %w", err)
	}

	// 初始化服务
	if err := InitServices(); err != nil {
		return fmt.Errorf("初始化服务失败: %w", err)
	}

	// 初始化API路由
	router, err := api.SetupRouter()
	if err != nil {
		return fmt.Errorf("初始化API路由失败: %w", err)
	}
	GetApp().router = router

	return nil
}

// ReinitializeLLMService 重新初始化LLM服务（在配置更新后调用）
func ReinitializeLLMService() error {
	container := di.GetContainer()

	// 重新创建LLM服务
	llmService, err := services.NewLLMService()
	if err != nil {
		// 如果LLM服务初始化失败，创建空服务作为fallback
		llmService = services.NewEmptyLLMService()
	}

	// 更新容器中的LLM服务
	container.Register("llm", llmService)

	// 重新创建依赖LLM服务的Analyzer服务
	analyzerService := services.NewAnalyzerServiceWithLLMService(llmService)
	container.Register("analyzer", analyzerService)

	// 重新创建依赖LLM服务的Story服务并复用已有依赖，保持缓存与锁一致
	storyService := services.NewStoryService(llmService)
	if sceneSvc, ok := container.Get("scene").(*services.SceneService); ok {
		storyService.SceneService = sceneSvc
	}
	if ctxSvc, ok := container.Get("context").(*services.ContextService); ok {
		storyService.ContextService = ctxSvc
	}
	if itemSvc, ok := container.Get("item").(*services.ItemService); ok {
		storyService.ItemService = itemSvc
	}
	if charSvc, ok := container.Get("character").(*services.CharacterService); ok {
		storyService.CharacterService = charSvc
	}
	cfg := config.GetCurrentConfig()
	if cfg != nil {
		storyService.BasePath = cfg.DataDir + "/stories"
	}
	if storyService.BasePath != "" {
		if fs, err := storage.NewFileStorage(storyService.BasePath); err == nil {
			storyService.FileStorage = fs
		}
	}
	container.Register("story", storyService)

	return nil
}

// 初始化日志系统
func initLogger(logDir string) error {
	logFile := fmt.Sprintf("%s/app_%s.log", logDir, time.Now().Format("2006-01-02"))
	if err := utils.InitLogger(logFile); err != nil {
		return fmt.Errorf("初始化结构化日志失败: %w", err)
	}

	utils.GetLogger().Info("日志系统已初始化", map[string]interface{}{
		"log_file": logFile,
	})
	return nil
}

// Run 启动应用
func Run() error {
	app := GetApp()

	// 获取配置
	if app.config == nil {
		return fmt.Errorf("应用未初始化，请先调用Initialize")
	}

	// 仅在服务器未设置时创建新服务器
	if app.server == nil {
		app.server = &http.Server{
			Addr:    ":" + app.config.Port,
			Handler: app.router,
		}
	}

	// 设置信号处理
	signal.Notify(app.stopChan, syscall.SIGINT, syscall.SIGTERM)

	// 在独立的goroutine中启动服务器
	go func() {
		utils.GetLogger().Info("服务器启动，监听端口", map[string]interface{}{
			"port": app.config.Port,
		})

		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.GetLogger().Fatal("服务器启动失败", map[string]interface{}{
				"err": err,
			})
		}
	}()

	// 等待停止信号
	<-app.stopChan
	utils.GetLogger().Info("接收到停止信号，正在关闭服务器", nil)

	// 优雅关闭
	return app.Shutdown()
}

// Shutdown 优雅关闭服务器
func (a *App) Shutdown() error {
	// 设置超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 关闭HTTP服务器
	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("服务器关闭失败: %w", err)
	}

	utils.GetLogger().Info("服务器已成功关闭", nil)

	// 执行清理操作
	a.cleanup()

	return nil
}

// 清理资源
func (a *App) cleanup() {
	utils.GetLogger().Info("开始清理资源", nil)

	// 获取依赖注入容器
	container := di.GetContainer()

	// 进度服务的优雅关闭
	if progressService, ok := container.Get("progress").(*services.ProgressService); ok && progressService != nil {
		progressService.Stop()
		utils.GetLogger().Info("进度服务已停止", nil)
	}

	// JobQueue 的优雅关闭
	if jobQueue, ok := container.Get("job_queue").(*services.JobQueue); ok && jobQueue != nil {
		jobQueue.Stop()
		utils.GetLogger().Info("JobQueue 已停止", nil)
	}

	// 清理LLM服务缓存
	if llmService, ok := container.Get("llm").(*services.LLMService); ok && llmService != nil {
		// 如果需要，可以调用LLM服务的清理方法
		utils.GetLogger().Info("清理LLM服务缓存", nil)
		_ = llmService
	}

	// 清理文件缓存
	if fileCacheService, ok := container.Get("fileCache").(*storage.FileStorage); ok && fileCacheService != nil {
		fileCacheService.StartCacheCleanup()
		utils.GetLogger().Info("文件缓存已清理", nil)
	}

	// 保存未完成的任务状态
	if progressService, ok := container.Get("progress").(*services.ProgressService); ok && progressService != nil {
		// 清理已完成的旧任务，保留最近10分钟的记录
		progressService.CleanupCompletedTasks(10 * time.Minute)
		utils.GetLogger().Info("旧任务数据已清理", nil)
	}

	// 关闭可能的数据库连接
	// db.Close() // 如果将来添加数据库

	utils.GetLogger().Info("资源清理完成", nil)
}

// GetConfig 返回应用配置
func (a *App) GetConfig() *config.AppConfig {
	return a.config
}

// GetDIContainer 返回依赖注入容器
func GetDIContainer() *di.Container {
	return di.GetContainer()
}

// IsDebugMode 检查是否处于调试模式
func IsDebugMode() bool {
	if app := GetApp(); app != nil && app.config != nil {
		return app.config.DebugMode
	}
	return false
}
