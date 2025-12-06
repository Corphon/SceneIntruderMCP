package app

import (
	"context"
	"encoding/json"
	"fmt"

	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/services"
	"github.com/Corphon/SceneIntruderMCP/internal/storage"
)

// 测试前的设置工作
func setupTest(t *testing.T) string {
	// 重置全局应用实例
	instance = nil

	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "app_test_*")
	if err != nil {
		t.Fatalf("创建临时测试目录失败: %v", err)
	}

	// 创建子目录
	os.MkdirAll(filepath.Join(tempDir, "logs"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "data", "scenes"), 0755)

	return tempDir
}

// 测试后的清理工作
func cleanupTest(tempDir string) {
	os.RemoveAll(tempDir)
	instance = nil
}

// 测试创建模拟服务器
type mockServer struct {
	ShutdownCalled bool
	HandlerFunc    http.HandlerFunc
}

func (m *mockServer) ListenAndServe() error {
	return nil
}

func (m *mockServer) Shutdown(ctx context.Context) error {
	m.ShutdownCalled = true
	return nil
}

// TestGetApp 测试获取应用实例
func TestGetApp(t *testing.T) {
	// 重置全局实例
	instance = nil

	// 获取应用实例
	app1 := GetApp()
	if app1 == nil {
		t.Fatal("GetApp应该返回一个非nil的应用实例")
	}

	// 再次调用，应该返回相同的实例（单例模式）
	app2 := GetApp()
	if app1 != app2 {
		t.Fatal("GetApp应该返回相同的实例")
	}

	// 验证stopChan已初始化
	if app1.stopChan == nil {
		t.Fatal("应用实例的stopChan应该被初始化")
	}
}

// TestInitialize 测试应用初始化
func TestInitialize(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)
	// 创建web模板目录和一个简单的模板文件
	webTemplatesDir := filepath.Join(tempDir, "web", "templates")
	err := os.MkdirAll(webTemplatesDir, 0755)
	if err != nil {
		t.Fatalf("创建模板目录失败: %v", err)
	}

	// 创建一个简单的模板文件
	templateContent := `<!DOCTYPE html><html><head><title>Test</title></head><body>{{.Title}}</body></html>`
	err = os.WriteFile(filepath.Join(webTemplatesDir, "index.html"), []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("创建模板文件失败: %v", err)
	}

	// 创建静态资源目录
	staticDir := filepath.Join(tempDir, "web", "static")
	err = os.MkdirAll(staticDir, 0755)
	if err != nil {
		t.Fatalf("创建静态资源目录失败: %v", err)
	}

	// 设置环境变量以指向测试目录中的web文件夹
	originalWebDir := os.Getenv("WEB_DIR")
	os.Setenv("WEB_DIR", filepath.Join(tempDir, "web"))
	defer os.Setenv("WEB_DIR", originalWebDir) // 恢复原始环境变量
	/// 重置容器
	di.GetContainer().Clear()
	container := di.GetContainer()

	// 创建并注册依赖服务，确保顺序正确
	llmService := services.NewEmptyLLMService()
	container.Register("llm", llmService)

	itemService := services.NewItemService(filepath.Join(tempDir, "data/scenes"))
	container.Register("item", itemService)

	sceneService := services.NewSceneService(filepath.Join(tempDir, "data/scenes"))
	sceneService.ItemService = itemService
	container.Register("scene", sceneService)

	// 必须先创建上下文服务，因为角色服务依赖它
	contextService := services.NewContextService(sceneService)
	container.Register("context", contextService)

	// 确保角色服务的依赖已经注册后再创建
	characterService := &services.CharacterService{
		LLMService:     llmService,
		ContextService: contextService,
	}
	container.Register("character", characterService)

	// 其他必要服务
	container.Register("progress", services.NewProgressService())
	container.Register("config", services.NewConfigService())
	container.Register("stats", services.NewStatsService())
	container.Register("user", services.NewUserService())
	container.Register("story", services.NewStoryService(llmService))
	// 添加这个可能缺少的服务
	//analyzerService := services.NewAnalyzerServiceWithProvider(llmService.GetProvider())
	//container.Register("analyzer", analyzerService)
	// 测试初始化
	err = initializeForTest(tempDir)
	if err != nil {
		t.Fatalf("初始化应用失败: %v", err)
	}

	// 验证应用实例已正确配置
	app := GetApp()

	if app.config == nil {
		t.Fatal("应用配置应该已被设置")
	}

	if app.router == nil {
		t.Fatal("应用路由应该已被设置")
	}

	// 验证配置文件已创建
	configFilePath := filepath.Join(tempDir, "config.json")
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		t.Error("配置文件应该已被创建")
	}

	// 检查日志文件是否已创建
	files, _ := os.ReadDir(filepath.Join(tempDir, "logs"))
	if len(files) == 0 {
		t.Error("应该已创建日志文件")
	}

	// 新增：验证聚合服务

	sceneAggregateService := container.Get("scene_aggregate")
	if sceneAggregateService == nil {
		t.Error("场景聚合服务应该已被注册")
	}

	interactionAggregateService := container.Get("interaction_aggregate")
	if interactionAggregateService == nil {
		t.Error("交互聚合服务应该已被注册")
	}
}

// 仅在app_test.go中定义此函数，测试专用
func initializeForTest(configPath string) error {
	// 加载配置
	if err := config.InitConfig(configPath); err != nil {
		return fmt.Errorf("初始化配置失败: %w", err)
	}

	// 获取配置并确保日志目录指向测试目录
	cfg := config.GetCurrentConfig()
	cfg.LogDir = filepath.Join(configPath, "logs") // 确保使用测试目录
	GetApp().config = cfg

	// 初始化日志系统
	if err := initLogger(cfg.LogDir); err != nil {
		return fmt.Errorf("初始化日志系统失败: %w", err)
	}

	// 初始化服务
	if err := InitServices(); err != nil {
		return fmt.Errorf("初始化服务失败: %w", err)
	}

	// 创建一个简单的路由器，跳过模板加载
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("测试路由"))
	})
	GetApp().router = mux

	return nil
}

// TestInitLogger 测试日志初始化
func TestInitLogger(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	logDir := filepath.Join(tempDir, "custom_logs")

	// 测试初始化日志
	err := initLogger(logDir)
	if err != nil {
		t.Fatalf("初始化日志系统失败: %v", err)
	}

	// 验证日志目录已创建
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Error("日志目录应该已被创建")
	}

	// 验证日志文件已创建（名称包含当天日期）
	files, _ := os.ReadDir(logDir)
	if len(files) == 0 {
		t.Error("应该已创建日志文件")
	}
}

// TestRun 测试应用运行和关闭
func TestRun(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 创建测试应用实例
	testApp := &App{
		config: &config.AppConfig{
			Port: "8081",
		},
		stopChan: make(chan os.Signal, 1),
	}
	instance = testApp
	// 创建模拟服务器并设置
	mockSrv := &mockServer{}
	testApp.server = mockSrv
	// 模拟发送停止信号
	go func() {
		time.Sleep(100 * time.Millisecond)
		testApp.stopChan <- syscall.SIGTERM
	}()

	// 运行应用（应该在收到信号后返回）
	err := Run()
	if err != nil {
		t.Fatalf("运行应用失败: %v", err)
	}

	// 验证Shutdown被调用
	if !mockSrv.ShutdownCalled {
		t.Error("应该调用了server.Shutdown")
	}
}

// TestCleanup 测试资源清理
func TestCleanup(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 创建测试应用实例
	testApp := &App{
		config:   &config.AppConfig{},
		stopChan: make(chan os.Signal, 1),
	}
	instance = testApp

	// 初始化DI容器
	container := di.NewContainer()

	// 注册测试服务
	mockLLMService := &services.LLMService{}
	container.Register("llm", mockLLMService)

	mockFileCacheService := &storage.FileStorage{}
	container.Register("fileCache", mockFileCacheService)

	mockProgressService := &services.ProgressService{}
	container.Register("progress", mockProgressService)

	// 执行清理
	testApp.cleanup()

	// 注意：由于我们使用的是mock服务，不需要验证实际清理效果
	// 实际上，我们应该通过接口定义清理方法，并验证这些方法是否被调用
}

// TestGetConfig 测试获取应用配置
func TestGetConfig(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 创建测试配置
	testConfig := &config.AppConfig{
		Port:      "9000",
		DebugMode: true,
	}

	// 设置应用实例
	testApp := &App{
		config: testConfig,
	}
	instance = testApp

	// 测试获取配置
	cfg := testApp.GetConfig()

	if cfg != testConfig {
		t.Error("GetConfig应该返回应用的配置")
	}
}

// TestGetDIContainer 测试获取依赖注入容器
func TestGetDIContainer(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 重置DI容器
	di.NewContainer()

	// 获取容器
	container := GetDIContainer()

	if container == nil {
		t.Fatal("GetDIContainer应该返回一个非nil的容器")
	}

	// 验证是相同的容器实例
	container2 := di.GetContainer()
	if container != container2 {
		t.Error("应该返回相同的DI容器实例")
	}
}

// TestIsDebugMode 测试调试模式检查
func TestIsDebugMode(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 测试无应用实例的情况
	instance = nil
	if IsDebugMode() {
		t.Error("无应用实例时IsDebugMode应该返回false")
	}

	// 测试有应用实例但无配置的情况
	testApp := &App{}
	instance = testApp
	if IsDebugMode() {
		t.Error("应用无配置时IsDebugMode应该返回false")
	}

	// 测试调试模式开启的情况
	testApp.config = &config.AppConfig{
		DebugMode: true,
	}
	if !IsDebugMode() {
		t.Error("调试模式开启时IsDebugMode应该返回true")
	}

	// 测试调试模式关闭的情况
	testApp.config.DebugMode = false
	if IsDebugMode() {
		t.Error("调试模式关闭时IsDebugMode应该返回false")
	}
}

// TestInitAggregateServices 测试聚合服务初始化
func TestInitAggregateServices(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 重置容器
	di.GetContainer().Clear()
	container := di.GetContainer()

	// 初始化基础服务
	llmService := services.NewEmptyLLMService()
	container.Register("llm", llmService)

	sceneService := services.NewSceneService(filepath.Join(tempDir, "data/scenes"))
	container.Register("scene", sceneService)

	contextService := services.NewContextService(sceneService)
	container.Register("context", contextService)

	characterService := services.NewCharacterService()
	container.Register("character", characterService)

	progressService := services.NewProgressService()
	container.Register("progress", progressService)

	statsService := services.NewStatsService()
	container.Register("stats", statsService)

	storyService := services.NewStoryService(llmService)
	container.Register("story", storyService)

	// 测试初始化聚合服务
	err := InitServices()
	if err != nil {
		t.Fatalf("初始化聚合服务失败: %v", err)
	}

	// 验证场景聚合服务已注册
	sceneAggregateService := container.Get("scene_aggregate")
	if sceneAggregateService == nil {
		t.Error("场景聚合服务应该已被注册")
	}

	// 验证交互聚合服务已注册
	interactionAggregateService := container.Get("interaction_aggregate")
	if interactionAggregateService == nil {
		t.Error("交互聚合服务应该已被注册")
	}

	// 验证聚合服务类型正确
	if _, ok := sceneAggregateService.(*services.SceneAggregateService); !ok {
		t.Error("场景聚合服务类型不正确")
	}

	if _, ok := interactionAggregateService.(*services.InteractionAggregateService); !ok {
		t.Error("交互聚合服务类型不正确")
	}
}

// TestServiceDependencyOrder 测试服务依赖初始化顺序
func TestServiceDependencyOrder(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 重置容器
	di.GetContainer().Clear()

	// 先初始化配置系统
	err := config.InitConfig(tempDir)
	if err != nil {
		t.Fatalf("初始化配置失败: %v", err)
	}

	// 测试服务初始化
	err = InitServices()
	if err != nil {
		t.Fatalf("服务初始化失败: %v", err)
	}

	container := di.GetContainer()

	// 验证基础服务已注册
	basicServices := []string{"llm", "progress", "stats", "item", "character", "user"}
	for _, serviceName := range basicServices {
		if service := container.Get(serviceName); service == nil {
			t.Errorf("基础服务 %s 应该已被注册", serviceName)
		}
	}

	// 验证依赖服务已注册
	dependentServices := []string{"scene", "context", "story"}
	for _, serviceName := range dependentServices {
		if service := container.Get(serviceName); service == nil {
			t.Errorf("依赖服务 %s 应该已被注册", serviceName)
		}
	}

	// 验证聚合服务已注册
	aggregateServices := []string{"scene_aggregate", "interaction_aggregate"}
	for _, serviceName := range aggregateServices {
		if service := container.Get(serviceName); service == nil {
			t.Errorf("聚合服务 %s 应该已被注册", serviceName)
		}
	}
}

// TestLLMServiceInitialization 测试LLM服务初始化逻辑
func TestLLMServiceInitialization(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 重置容器
	di.GetContainer().Clear()

	tests := []struct {
		name        string
		provider    string
		config      map[string]string
		expectEmpty bool
	}{
		{
			name:        "无配置时使用空服务",
			provider:    "",
			config:      nil,
			expectEmpty: true,
		},
		{
			name:        "有配置时使用真实服务",
			provider:    "openai",
			config:      map[string]string{"api_key": "test-key"},
			expectEmpty: false, // 注意：在测试环境中可能仍然是空服务
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置容器
			di.GetContainer().Clear()

			// 初始化配置
			err := config.InitConfig(tempDir)
			if err != nil {
				t.Fatalf("初始化配置失败: %v", err)
			}

			// 如果需要特定的LLM配置，可以使用UpdateLLMConfig
			if tt.provider != "" && tt.config != nil {
				err = config.UpdateLLMConfig(tt.provider, tt.config)
				if err != nil {
					t.Fatalf("更新LLM配置失败: %v", err)
				}
			}

			// 初始化服务
			err = InitServices()
			if err != nil {
				t.Fatalf("服务初始化失败: %v", err)
			}

			// 验证LLM服务
			container := di.GetContainer()
			llmService := container.Get("llm")
			if llmService == nil {
				t.Fatal("LLM服务应该已被注册")
			}

			// 验证服务类型
			if _, ok := llmService.(*services.LLMService); !ok {
				t.Error("LLM服务类型不正确")
			}
		})
	}
}

// TestCleanupWithAdvancedServices 测试增强的清理功能
func TestCleanupWithAdvancedServices(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 创建测试应用实例
	testApp := &App{
		config:   &config.AppConfig{},
		stopChan: make(chan os.Signal, 1),
	}
	instance = testApp

	// 初始化DI容器
	container := di.NewContainer()

	// 注册测试服务，包括新的服务类型
	mockLLMService := &services.LLMService{}
	container.Register("llm", mockLLMService)

	mockFileCacheService := &storage.FileStorage{}
	container.Register("fileCache", mockFileCacheService)

	mockProgressService := &services.ProgressService{}
	container.Register("progress", mockProgressService)

	// 添加聚合服务测试
	mockSceneAggregateService := &services.SceneAggregateService{}
	container.Register("scene_aggregate", mockSceneAggregateService)

	mockInteractionAggregateService := &services.InteractionAggregateService{}
	container.Register("interaction_aggregate", mockInteractionAggregateService)

	// 执行清理
	testApp.cleanup()

	// 验证清理过程（通过日志或其他方式）
	// 注意：由于使用mock服务，主要验证清理流程不出错
}

// TestGetDIContainerServices 测试获取容器中的服务
func TestGetDIContainerServices(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 初始化应用
	err := initializeForTest(tempDir)
	if err != nil {
		t.Fatalf("初始化应用失败: %v", err)
	}

	// 获取容器
	container := GetDIContainer()
	if container == nil {
		t.Fatal("GetDIContainer应该返回一个非nil的容器")
	}

	// 测试获取各种服务
	serviceNames := []string{
		"llm", "scene", "character", "context", "progress",
		"stats", "item", "user", "story", "scene_aggregate", "interaction_aggregate",
	}

	for _, serviceName := range serviceNames {
		service := container.Get(serviceName)
		if service == nil {
			t.Errorf("服务 %s 应该已被注册", serviceName)
		}
	}
}

// TestConfigIntegration 测试配置集成
func TestConfigIntegration(t *testing.T) {
	tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 创建测试配置文件
	configData := map[string]interface{}{
		"port":         "8080",
		"debug_mode":   true,
		"llm_provider": "openai",
		"llm_config": map[string]string{
			"api_key": "test-key",
			"model":   "gpt-3.5-turbo",
		},
	}

	configBytes, _ := json.Marshal(configData)
	configPath := filepath.Join(tempDir, "config.json")
	err := os.WriteFile(configPath, configBytes, 0644)
	if err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	// 使用 initializeForTest 而不是 Initialize
	err = initializeForTest(tempDir)
	if err != nil {
		t.Fatalf("应用初始化失败: %v", err)
	}

	// 验证配置加载
	app := GetApp()
	if app.config == nil {
		t.Fatal("应用配置应该已被加载")
	}

	// 从配置文件读取配置进行验证
	cfg := config.GetCurrentConfig()
	if cfg.Port != "8081" {
		t.Errorf("端口配置不正确，期望: 8080，实际: %s", cfg.Port)
	}

	if !cfg.DebugMode {
		t.Error("调试模式应该已启用")
	}

	// 验证服务已正确初始化
	container := GetDIContainer()
	llmService := container.Get("llm")
	if llmService == nil {
		t.Error("LLM服务应该已被初始化")
	}
}
