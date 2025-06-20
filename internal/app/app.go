// internal/app/app.go
package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/api"
	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/services"
	"github.com/Corphon/SceneIntruderMCP/internal/storage"
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
func initServices() error {
	// 获取当前配置
	cfg := config.GetCurrentConfig()

	// 使用现有的全局容器，而不是创建新的
	container := di.GetContainer() // ✅ 使用全局容器

	// 1. 首先初始化基础服务（无依赖）
	// 初始化LLM服务 (使用配置)
	var llmService *services.LLMService
	var err error

	if cfg.LLMProvider != "" && cfg.LLMConfig != nil {
		llmService, err = services.NewLLMService()
		if err != nil {
			log.Printf("警告: LLM服务初始化失败: %v", err)
			llmService = services.NewEmptyLLMService()
		}
	} else {
		log.Printf("未配置LLM提供商，使用空服务实例")
		llmService = services.NewEmptyLLMService()
	}
	container.Register("llm", llmService)

	// 初始化进度服务（基础服务）
	progressService := services.NewProgressService()
	container.Register("progress", progressService)

	// 初始化统计服务（基础服务）
	statsService := services.NewStatsService()
	container.Register("stats", statsService)

	// 初始化物品服务（基础服务）
	itemService := services.NewItemService()
	container.Register("item", itemService)

	// 初始化角色服务（基础服务）
	characterService := services.NewCharacterService()
	container.Register("character", characterService)

	// 初始化用户服务（基础服务）
	userService := services.NewUserService()
	container.Register("user", userService)

	// 2. 初始化有依赖的服务
	// 使用配置初始化场景服务
	scenesPath := cfg.DataDir + "/scenes"
	sceneService := services.NewSceneService(scenesPath)
	container.Register("scene", sceneService)

	// 初始化上下文服务（依赖场景服务）
	contextService := services.NewContextService(sceneService)
	container.Register("context", contextService)

	// 初始化剧情服务（依赖LLM服务）
	storyService := services.NewStoryService(llmService)
	container.Register("story", storyService)

	// ✅ 初始化导出服务（依赖多个基础服务）
	exportService := services.NewExportService(contextService, storyService, sceneService)
	container.Register("export", exportService)

	// 3. 最后初始化聚合服务（依赖多个服务）
	// 初始化场景聚合服务
	sceneAggregateService := services.NewSceneAggregateService(
		sceneService,
		characterService,
		contextService,
		storyService,
		progressService,
	)
	container.Register("scene_aggregate", sceneAggregateService)

	// 初始化交互聚合服务
	interactionAggregateService := &services.InteractionAggregateService{
		CharacterService: characterService,
		ContextService:   contextService,
		SceneService:     sceneService,
		StatsService:     statsService,
		StoryService:     storyService,
		ExportService:    exportService,
	}
	container.Register("interaction_aggregate", interactionAggregateService)

	log.Println("所有服务初始化完成")
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
	if err := initServices(); err != nil {
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

// 初始化日志系统
func initLogger(logDir string) error {
	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 设置日志输出
	logFile, err := os.OpenFile(
		fmt.Sprintf("%s/app_%s.log", logDir, time.Now().Format("2006-01-02")),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %w", err)
	}

	// 设置多输出
	log.SetOutput(logFile)

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
		log.Printf("服务器启动，监听端口: %s", app.config.Port)

		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 等待停止信号
	<-app.stopChan
	log.Println("接收到停止信号，正在关闭服务器...")

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

	log.Println("服务器已成功关闭")

	// 执行清理操作
	a.cleanup()

	return nil
}

// 清理资源
func (a *App) cleanup() {
	log.Println("开始清理资源...")

	// 获取依赖注入容器
	container := di.GetContainer()

	// 清理LLM服务缓存
	if llmService, ok := container.Get("llm").(*services.LLMService); ok && llmService != nil {
		// 如果需要，可以调用LLM服务的清理方法
		log.Println("清理LLM服务缓存")
	}

	// 清理文件缓存
	if fileCacheService, ok := container.Get("fileCache").(*storage.FileCacheService); ok && fileCacheService != nil {
		fileCacheService.ClearCache()
		log.Println("文件缓存已清理")
	}

	// 保存未完成的任务状态
	if progressService, ok := container.Get("progress").(*services.ProgressService); ok && progressService != nil {
		// 清理已完成的旧任务，保留最近10分钟的记录
		progressService.CleanupCompletedTasks(10 * time.Minute)
		log.Println("旧任务数据已清理")
	}

	// 关闭可能的数据库连接
	// db.Close() // 如果将来添加数据库

	log.Println("资源清理完成")
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
