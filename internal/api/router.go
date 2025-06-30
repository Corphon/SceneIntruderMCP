// internal/api/router.go
package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/services"
	"github.com/gin-gonic/gin"
)

// SetupRouter 配置HTTP路由
func SetupRouter() (*gin.Engine, error) {
	// 获取配置
	cfg := config.GetCurrentConfig()

	// 确保临时目录存在
	os.MkdirAll("temp", 0755)

	// 获取依赖注入容器
	container := di.GetContainer()

	// 从容器获取服务
	sceneService, ok := container.Get("scene").(*services.SceneService)
	if !ok {
		return nil, fmt.Errorf("场景服务未正确初始化")
	}

	contextService, ok := container.Get("context").(*services.ContextService)
	if !ok {
		return nil, fmt.Errorf("上下文服务未正确初始化")
	}

	characterService, ok := container.Get("character").(*services.CharacterService)
	if !ok {
		return nil, fmt.Errorf("角色服务未正确初始化")
	}

	// 获取LLM服务（用于初始化分析服务）
	llmService, ok := container.Get("llm").(*services.LLMService)
	if !ok {
		return nil, fmt.Errorf("LLM服务未正确初始化")
	}

	// 获取进度服务（确保在使用前已初始化）
	progressService, ok := container.Get("progress").(*services.ProgressService)
	if !ok {
		// 如果不存在，创建新实例
		progressService = services.NewProgressService()
		container.Register("progress", progressService)
	}

	// 获取统计服务（确保在使用前已初始化）
	statsService, ok := container.Get("stats").(*services.StatsService)
	if !ok {
		statsService = services.NewStatsService()
		container.Register("stats", statsService)
	}

	// 注册故事服务
	storyService := services.NewStoryService(llmService)
	container.Register("story", storyService)

	// 注册导出服务
	exportService := services.NewExportService(contextService, storyService, sceneService)
	container.Register("export", exportService)

	// 注册场景聚合服务
	sceneAggregateService := services.NewSceneAggregateService(
		sceneService, characterService, contextService, storyService, progressService)
	container.Register("scene_aggregate", sceneAggregateService)

	// 注册交互聚合服务 - 修复参数顺序
	interactionAggregateService := &services.InteractionAggregateService{
		CharacterService: characterService,
		ContextService:   contextService,
		SceneService:     sceneService,
		StatsService:     statsService,
		StoryService:     storyService,
		ExportService:    exportService,
	}
	container.Register("interaction_aggregate", interactionAggregateService)

	// 初始化分析服务
	analyzerService := services.NewAnalyzerServiceWithProvider(llmService.GetProvider())
	container.Register("analyzer", analyzerService)

	// 获取配置服务
	configService, ok := container.Get("config").(*services.ConfigService)
	if !ok {
		configService = services.NewConfigService()
		container.Register("config", configService)
	}

	// 获取用户服务
	userService, ok := container.Get("user").(*services.UserService)
	if !ok {
		userService = services.NewUserService()
		container.Register("user", userService)
	}

	// 创建API处理器
	handler := NewHandler(
		sceneService,
		characterService,
		contextService,
		progressService,
		analyzerService,
		configService,
		statsService,
		userService,
	)

	// 创建路由
	r := gin.Default()

	// 启用CORS
	r.Use(corsMiddleware())

	// HTTPS重定向（生产环境）
	if !cfg.DebugMode {
		r.Use(func(c *gin.Context) {
			if c.Request.Header.Get("X-Forwarded-Proto") != "https" {
				c.Redirect(http.StatusPermanentRedirect,
					"https://"+c.Request.Host+c.Request.URL.Path)
				return
			}
			c.Next()
		})
	}

	// 静态文件服务
	r.Static("/static", cfg.StaticDir)

	// HTML模板
	r.LoadHTMLGlob(filepath.Join(cfg.TemplatesDir, "*.html"))

	// ===============================
	// 页面路由
	// ===============================
	r.GET("/", handler.IndexPage)
	r.GET("/scenes", handler.SceneSelectorPage)
	r.GET("/scenes/create", handler.CreateScenePage)
	r.GET("/scenes/:id", handler.ScenePage)
	r.GET("/settings", handler.SettingsPage)
	r.GET("/scenes/:id/story", handler.StoryViewPage)

	// WebSocket 支持
	r.GET("/ws/scene/:id", handler.SceneWebSocket)
	r.GET("/ws/user/status", handler.UserStatusWebSocket)

	// ===============================
	// API路由组
	// ===============================
	api := r.Group("/api")
	{
		// 聚合API端点
		api.GET("/scenes/:id/aggregate", handler.GetSceneAggregate)
		api.POST("/interactions/aggregate", handler.ProcessInteractionAggregate)

		// ===============================
		// 设置相关路由
		// ===============================
		settingsGroup := api.Group("/settings")
		{
			settingsGroup.GET("", handler.GetSettings)
			settingsGroup.POST("", handler.SaveSettings)
			settingsGroup.POST("/test-connection", handler.TestConnection)
		}

		// ===============================
		// LLM配置相关路由
		// ===============================
		llmGroup := api.Group("/llm")
		{
			llmGroup.GET("/status", handler.GetLLMStatus)
			llmGroup.GET("/models", handler.GetLLMModels)
			llmGroup.PUT("/config", handler.UpdateLLMConfig)
		}

		// ===============================
		// 场景相关路由
		// ===============================
		scenesGroup := api.Group("/scenes")
		{
			scenesGroup.GET("", handler.GetScenes)
			scenesGroup.POST("", handler.CreateScene)
			scenesGroup.GET("/:id", handler.GetScene)
			scenesGroup.GET("/:id/characters", handler.GetCharacters)
			scenesGroup.GET("/:id/conversations", handler.GetConversations) // 只保留一次

			// ✅ 故事相关路由
			storyGroup := scenesGroup.Group("/:id/story")
			{
				storyGroup.GET("", handler.GetStoryData)              // GET /api/scenes/:id/story
				storyGroup.POST("/choice", handler.MakeStoryChoice)   // POST /api/scenes/:id/story/choice
				storyGroup.POST("/advance", handler.AdvanceStory)     // POST /api/scenes/:id/story/advance
				storyGroup.POST("/rewind", handler.RewindStory)       // POST /api/scenes/:id/story/rewind
				storyGroup.GET("/branches", handler.GetStoryBranches) // GET /api/scenes/:id/story/branches
			}

			// ✅ 导出相关路由
			exportGroup := scenesGroup.Group("/:id/export")
			{
				exportGroup.GET("/scene", handler.ExportScene)
				exportGroup.GET("/interactions", handler.ExportInteractions)
				exportGroup.GET("/story", handler.ExportStory)
			}
		}

		// ===============================
		// 聊天相关路由
		// ===============================
		chatGroup := api.Group("/chat")
		{
			chatGroup.POST("", handler.Chat)                    // 普通聊天
			chatGroup.POST("/emotion", handler.ChatWithEmotion) // 带情绪聊天
		}

		// ===============================
		// 角色互动相关路由
		// ===============================
		interactionsGroup := api.Group("/interactions")
		{
			interactionsGroup.POST("/trigger", handler.TriggerCharacterInteraction)
			interactionsGroup.POST("/simulate", handler.SimulateCharactersConversation)
			interactionsGroup.GET("/:scene_id", handler.GetCharacterInteractions)
			interactionsGroup.GET("/:scene_id/:character1_id/:character2_id", handler.GetCharacterToCharacterInteractions)
		}

		// ===============================
		// 文件上传
		// ===============================
		api.POST("/upload", handler.UploadFile)

		// ===============================
		// 分析和进度相关
		// ===============================
		api.POST("/analyze", handler.AnalyzeTextWithProgress)
		api.GET("/progress/:taskID", handler.SubscribeProgress)
		api.POST("/cancel/:taskID", handler.CancelAnalysisTask)

		// ===============================
		// 用户管理路由 - 去除重复，统一组织
		// ===============================
		usersGroup := api.Group("/users/:user_id")
		{
			// 用户档案
			usersGroup.GET("", handler.GetUserProfile)
			usersGroup.PUT("", handler.UpdateUserProfile)
			usersGroup.GET("/preferences", handler.GetUserPreferences)
			usersGroup.PUT("/preferences", handler.UpdateUserPreferences)

			// 道具管理
			itemsGroup := usersGroup.Group("/items")
			{
				itemsGroup.GET("", handler.GetUserItems)
				itemsGroup.POST("", handler.AddUserItem)
				itemsGroup.GET("/:item_id", handler.GetUserItem)
				itemsGroup.PUT("/:item_id", handler.UpdateUserItem)
				itemsGroup.DELETE("/:item_id", handler.DeleteUserItem)
			}

			// 技能管理
			skillsGroup := usersGroup.Group("/skills")
			{
				skillsGroup.GET("", handler.GetUserSkills)
				skillsGroup.POST("", handler.AddUserSkill)
				skillsGroup.GET("/:skill_id", handler.GetUserSkill)
				skillsGroup.PUT("/:skill_id", handler.UpdateUserSkill)
				skillsGroup.DELETE("/:skill_id", handler.DeleteUserSkill)
			}
		}

		//调试路由
		api.GET("/ws/status", handler.GetWebSocketStatus)
	}

	return r, nil
}

// corsMiddleware 实现跨域资源共享
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
