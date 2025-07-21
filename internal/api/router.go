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
// internal/api/router.go
func SetupRouter() (*gin.Engine, error) {
	// 获取配置
	cfg := config.GetCurrentConfig()

	// 确保临时目录存在
	os.MkdirAll("temp", 0755)

	// 获取依赖注入容器
	container := di.GetContainer()

	// ✅ 只从容器获取服务，不再创建新实例
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

	progressService, ok := container.Get("progress").(*services.ProgressService)
	if !ok {
		return nil, fmt.Errorf("进度服务未正确初始化")
	}

	analyzerService, ok := container.Get("analyzer").(*services.AnalyzerService)
	if !ok {
		return nil, fmt.Errorf("分析服务未正确初始化")
	}

	configService, ok := container.Get("config").(*services.ConfigService)
	if !ok {
		return nil, fmt.Errorf("配置服务未正确初始化")
	}

	statsService, ok := container.Get("stats").(*services.StatsService)
	if !ok {
		return nil, fmt.Errorf("统计服务未正确初始化")
	}

	userService, ok := container.Get("user").(*services.UserService)
	if !ok {
		return nil, fmt.Errorf("用户服务未正确初始化")
	}

	// ✅ 创建API处理器 - 只传递从容器获取的服务
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
	r.GET("/user/profile", handler.UserProfilePage) // 添加用户档案页面
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
			scenesGroup.GET("/:id/conversations", handler.GetConversations)

			// 故事相关路由
			storyGroup := scenesGroup.Group("/:id/story")
			{
				storyGroup.GET("", handler.GetStoryData)
				storyGroup.POST("/choice", handler.MakeStoryChoice)
				storyGroup.POST("/advance", handler.AdvanceStory)
				storyGroup.POST("/rewind", handler.RewindStory)
				storyGroup.GET("/branches", handler.GetStoryBranches)
				storyGroup.POST("/batch", handler.BatchStoryOperations)
			}

			// 导出相关路由
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
			chatGroup.POST("", handler.Chat)
			chatGroup.POST("/emotion", handler.ChatWithEmotion)
		}

		// ===============================
		// 角色互动相关路由
		// ===============================
		interactions := api.Group("/interactions")
		{
			interactions.POST("/trigger", handler.TriggerCharacterInteraction)
			interactions.POST("/simulate", handler.SimulateCharactersConversation)
			interactions.GET("/:scene_id", handler.GetCharacterInteractions)
			interactions.GET("/:scene_id/:character1_id/:character2_id", handler.GetCharacterToCharacterInteractions)
		}

		// ===============================
		// 配置相关路由
		// ===============================
		configGroup := api.Group("/config")
		{
			configGroup.GET("/health", handler.GetConfigHealth)
			configGroup.GET("/metrics", handler.GetConfigMetrics)
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
		// 用户管理路由
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

		// 调试路由
		api.GET("/ws/status", handler.GetWebSocketStatus)

		// WebSocket 管理路由
		wsGroup := api.Group("/ws")
		{
			wsGroup.GET("/status", handler.GetWebSocketStatus)
			wsGroup.POST("/cleanup", handler.CleanupWebSocketConnections)
		}
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
