// internal/api/router.go
package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	// API路由组 - 添加默认速率限制
	// ===============================
	api := r.Group("/api")
	api.Use(DefaultRateLimit()) // Apply default rate limiting to all API routes
	{
		// 聚合API端点
		api.GET("/scenes/:id/aggregate", RequireAuthForScene(), handler.GetSceneAggregate)
		api.POST("/interactions/aggregate", AuthMiddleware(), handler.ProcessInteractionAggregate)

		// ===============================
		// 设置相关路由
		// ===============================
		settingsGroup := api.Group("/settings")
		{
			settingsGroup.GET("", handler.GetSettings)
			settingsGroup.POST("", AuthMiddleware(), handler.SaveSettings)
			settingsGroup.POST("/test-connection", AuthMiddleware(), handler.TestConnection)
		}

		// ===============================
		// LLM配置相关路由
		// ===============================
		llmGroup := api.Group("/llm")
		{
			llmGroup.GET("/status", handler.GetLLMStatus)
			llmGroup.GET("/models", handler.GetLLMModels)
			llmGroup.PUT("/config", AuthMiddleware(), handler.UpdateLLMConfig)
		}

		// ===============================
		// 场景相关路由
		// ===============================
		scenesGroup := api.Group("/scenes")
		{
			scenesGroup.GET("", handler.GetScenes)
			scenesGroup.POST("", AuthMiddleware(), handler.CreateScene)
			scenesGroup.GET("/:id", RequireAuthForScene(), handler.GetScene)
			scenesGroup.GET("/:id/characters", RequireAuthForScene(), handler.GetCharacters)
			scenesGroup.GET("/:id/conversations", RequireAuthForScene(), handler.GetConversations)

			// 故事相关路由
			storyGroup := scenesGroup.Group("/:id/story")
			{
				storyGroup.GET("", RequireAuthForScene(), handler.GetStoryData)
				storyGroup.POST("/choice", RequireAuthForScene(), handler.MakeStoryChoice)
				storyGroup.POST("/advance", RequireAuthForScene(), handler.AdvanceStory)
				storyGroup.POST("/rewind", RequireAuthForScene(), handler.RewindStory)
				storyGroup.GET("/branches", RequireAuthForScene(), handler.GetStoryBranches)
				storyGroup.POST("/batch", RequireAuthForScene(), handler.BatchStoryOperations)
			}

			// 导出相关路由 - 保持默认 rate limit
			exportGroup := scenesGroup.Group("/:id/export")
			{
				exportGroup.GET("/scene", RequireAuthForScene(), handler.ExportScene)
				exportGroup.GET("/interactions", RequireAuthForScene(), handler.ExportInteractions)
				exportGroup.GET("/story", RequireAuthForScene(), handler.ExportStory)
			}
		}

		// ===============================
		// 聊天相关路由 - 更严格的限流
		// ===============================
		chatGroup := api.Group("/chat")
		chatGroup.Use(ChatRateLimit()) // Apply stricter rate limiting for chat endpoints
		{
			chatGroup.POST("", AuthMiddleware(), handler.Chat)
			chatGroup.POST("/emotion", AuthMiddleware(), handler.ChatWithEmotion)
		}

		// ===============================
		// 角色互动相关路由 - 使用聊天限流
		// ===============================
		interactions := api.Group("/interactions")
		interactions.Use(ChatRateLimit()) // Apply chat rate limiting
		{
			interactions.POST("/trigger", AuthMiddleware(), handler.TriggerCharacterInteraction)
			interactions.POST("/simulate", AuthMiddleware(), handler.SimulateCharactersConversation)
			interactions.GET("/:scene_id", RequireAuthForScene(), handler.GetCharacterInteractions)
			interactions.GET("/:scene_id/:character1_id/:character2_id", RequireAuthForScene(), handler.GetCharacterToCharacterInteractions)
		}

		// ===============================
		// 配置相关路由
		// ===============================
		configGroup := api.Group("/config")
		{
			configGroup.GET("/health", AuthMiddleware(), handler.GetConfigHealth)
			configGroup.GET("/metrics", AuthMiddleware(), handler.GetConfigMetrics)
		}

		// ===============================
		// 文件上传 - use analysis rate limit as it's resource-intensive
		// ===============================
		api.POST("/upload", AuthMiddleware(), AnalysisRateLimit(), handler.UploadFile)

		// ===============================
		// 分析和进度相关 - stricter rate limiting as these are resource-intensive
		// ===============================
		api.POST("/analyze", AuthMiddleware(), AnalysisRateLimit(), handler.AnalyzeTextWithProgress)
		api.GET("/progress/:taskID", handler.SubscribeProgress) // No rate limiting for progress since it's SSE
		api.POST("/cancel/:taskID", AuthMiddleware(), handler.CancelAnalysisTask)

		// ===============================
		// 用户管理路由
		// ===============================
		usersGroup := api.Group("/users/:user_id")
		usersGroup.Use(RequireAuthForUser()) // Require user authentication for user-specific routes
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
		api.GET("/ws/status", AuthMiddleware(), handler.GetWebSocketStatus)

		// WebSocket 管理路由
		wsGroup := api.Group("/ws")
		{
			wsGroup.GET("/status", AuthMiddleware(), handler.GetWebSocketStatus)
			wsGroup.POST("/cleanup", AuthMiddleware(), handler.CleanupWebSocketConnections)
		}
	}

	return r, nil
}

// corsMiddleware 实现跨域资源共享
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// In production, limit to specific origins rather than wildcard
		allowedOrigin := getEnv("ALLOWED_ORIGIN", "")
		if allowedOrigin == "" {
			// For development environments, allow current origin or localhost
			origin := c.GetHeader("Origin")
			
			// Improved origin validation for security
			if isValidOrigin(origin) {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				// Default to allowing only from the same origin in production
				c.Writer.Header().Set("Access-Control-Allow-Origin", "null")
			}
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-API-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isValidOrigin validates the origin for security
func isValidOrigin(origin string) bool {
	// List of valid origins (add yours here for production)
	validOrigins := []string{
		"http://localhost",
		"https://localhost", 
		"http://127.0.0.1",
		"http://0.0.0.0",
		"https://127.0.0.1",
	}

	for _, validOrigin := range validOrigins {
		if strings.HasPrefix(origin, validOrigin) {
			return true
		}
	}
	return false
}

// Helper function to get environment variables (this needs to be added to the file)
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
