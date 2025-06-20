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
	"github.com/Corphon/SceneIntruderMCP/internal/models"
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

	// 初始化进度服务（新增）
	progressService := services.NewProgressService()
	container.Register("progress", progressService)

	// 初始化分析服务（新增）
	analyzerService := services.NewAnalyzerServiceWithProvider(llmService.GetProvider())
	container.Register("analyzer", analyzerService)

	// 获取配置服务
	configService, ok := container.Get("config").(*services.ConfigService)
	if !ok {
		configService = services.NewConfigService()
		container.Register("config", configService)
	}

	// 获取统计服务
	statsService, ok := container.Get("stats").(*services.StatsService)
	if !ok {
		statsService = services.NewStatsService()
		container.Register("stats", statsService)
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

	//使用HTTPS
	/*
		if !cfg.DebugMode { // 根据config.json中的debug_mode配置决定是否启用
			r.Use(func(c *gin.Context) {
				if c.Request.Header.Get("X-Forwarded-Proto") != "https" {
					c.Redirect(http.StatusPermanentRedirect,
						"https://"+c.Request.Host+c.Request.URL.Path)
					return
				}
				c.Next()
			})
		}
	*/
	// 静态文件服务
	r.Static("/static", cfg.StaticDir)

	// HTML模板
	r.LoadHTMLGlob(filepath.Join(cfg.TemplatesDir, "*.html"))

	// 页面路由
	r.GET("/", handler.IndexPage)
	r.GET("/scenes", handler.SceneSelectorPage)
	r.GET("/scenes/create", handler.CreateScenePage)
	r.GET("/scenes/:id", handler.ScenePage)
	r.GET("/settings", handler.SettingsPage)

	// API路由
	api := r.Group("/api")
	{
		// 聚合API端点
		api.GET("/scenes/:id/aggregate", handler.GetSceneAggregate)
		api.POST("/interactions/aggregate", handler.ProcessInteractionAggregate)

		// 设置相关路由
		settingsGroup := api.Group("/settings")
		{
			settingsGroup.GET("", handler.GetSettings)                     // GET /api/settings
			settingsGroup.POST("", handler.SaveSettings)                   // POST /api/settings
			settingsGroup.POST("/test-connection", handler.TestConnection) // POST /api/settings/test-connection
		}

		// 保留现有的细粒度API（向后兼容）
		api.GET("/scenes", handler.GetScenes)
		api.GET("/scenes/:id", handler.GetScene)
		api.GET("/scenes/:id/characters", handler.GetCharacters)
		api.POST("/chat", handler.Chat)

		//LLM相关
		api.GET("/config/llm/status", handler.GetLLMStatus)
		r.GET("/api/llm/models", handler.GetLLMModels)
		api.PUT("/config/llm", handler.UpdateLLMConfig)

		// 场景相关
		api.GET("/scenes", handler.GetScenes)
		api.GET("/scenes/:id", handler.GetScene)
		api.POST("/scenes", handler.CreateScene)

		// 角色相关
		api.GET("/scenes/:id/characters", handler.GetCharacters)

		// 聊天相关
		api.POST("/chat", handler.Chat)
		api.GET("/scenes/:id/conversations", handler.GetConversations)

		// 文件上传
		api.POST("/upload", handler.UploadFile)

		// 分析和进度相关
		api.POST("/analyze", handler.AnalyzeTextWithProgress)
		api.GET("/progress/:taskID", handler.SubscribeProgress)
		api.POST("/cancel/:taskID", handler.CancelAnalysisTask)

		// 故事相关路由
		api.GET("/scenes/:id/story/branches", handler.GetStoryBranches)
		api.POST("/scenes/:id/story/rewind", handler.RewindStoryToNode)

		// 角色情绪聊天
		api.POST("/chat/emotion", handler.ChatWithEmotion)

		// 角色互动相关路由
		api.POST("/interactions/trigger", handler.TriggerCharacterInteraction)
		api.POST("/interactions/simulate", handler.SimulateCharactersConversation)
		api.GET("/interactions/:scene_id", handler.GetCharacterInteractions)
		api.GET("/interactions/:scene_id/:character1_id/:character2_id", handler.GetCharacterToCharacterInteractions)

		// 交流导出路由
		api.GET("/scenes/:scene_id/export/interactions", handler.ExportInteractionSummary)
		// 故事导出路由
		api.GET("/scenes/:scene_id/export/story", handler.ExportStoryDocument)
		// 场景导出路由
		api.GET("/scenes/:scene_id/export/scene", handler.ExportSceneData)
	}

	// 用户道具和技能路由
	userItemsGroup := r.Group("/api/users/:user_id/items")
	{
		userItemsGroup.GET("", handler.GetUserItems)
		userItemsGroup.POST("", handler.AddUserItem)
		userItemsGroup.GET("/:item_id", handler.GetUserItem)
		userItemsGroup.PUT("/:item_id", handler.UpdateUserItem)
		userItemsGroup.DELETE("/:item_id", handler.DeleteUserItem)
	}

	userSkillsGroup := r.Group("/api/users/:user_id/skills")
	{
		userSkillsGroup.GET("", handler.GetUserSkills)
		userSkillsGroup.POST("", handler.AddUserSkill)
		userSkillsGroup.GET("/:skill_id", handler.GetUserSkill)
		userSkillsGroup.PUT("/:skill_id", handler.UpdateUserSkill)
		userSkillsGroup.DELETE("/:skill_id", handler.DeleteUserSkill)
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

// ExportStoryDocument 导出故事文档
func (h *Handler) ExportStoryDocument(c *gin.Context) {
	sceneID := c.Param("scene_id")
	format := c.DefaultQuery("format", "json")

	// 获取导出服务
	exportService := h.getExportService()
	if exportService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出服务未初始化"})
		return
	}

	// 导出故事文档
	result, err := exportService.ExportStoryAsDocument(c.Request.Context(), sceneID, format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 根据格式返回响应
	h.handleExportResponse(c, result, format)
}

// ExportSceneData 导出场景数据
func (h *Handler) ExportSceneData(c *gin.Context) {
	sceneID := c.Param("scene_id")
	format := c.DefaultQuery("format", "json")
	includeConversations := c.DefaultQuery("include_conversations", "false") == "true"

	// 获取导出服务
	exportService := h.getExportService()
	if exportService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出服务未初始化"})
		return
	}

	// 导出场景数据
	result, err := exportService.ExportSceneData(c.Request.Context(), sceneID, format, includeConversations)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 根据格式返回响应
	h.handleExportResponse(c, result, format)
}

/*
// ExportAllData 导出所有数据（可选）
func (h *Handler) ExportAllData(c *gin.Context) {
	sceneID := c.Param("scene_id")
	format := c.DefaultQuery("format", "json")

	// 获取导出服务
	exportService := h.getExportService()
	if exportService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出服务未初始化"})
		return
	}

	// 导出所有数据
	result, err := exportService.ExportAll(c.Request.Context(), sceneID, format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 根据格式返回响应
	h.handleExportResponse(c, result, format)
}
*/
// getExportService 获取导出服务实例
func (h *Handler) getExportService() *services.ExportService {
	container := di.GetContainer()

	// 尝试从容器获取
	if service, ok := container.Get("export").(*services.ExportService); ok {
		return service
	}

	// 如果不存在，创建新实例
	exportService := services.NewExportService(h.ContextService, h.getStoryService(), h.SceneService)

	// 注册到容器
	container.Register("export", exportService)

	return exportService
}

// handleExportResponse 统一处理导出响应
func (h *Handler) handleExportResponse(c *gin.Context, result *models.ExportResult, format string) {
	switch strings.ToLower(format) {
	case "json":
		c.JSON(http.StatusOK, result)
	case "markdown", "txt":
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(result.FilePath)))
		c.String(http.StatusOK, result.Content)
	case "html":
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(result.FilePath)))
		c.String(http.StatusOK, result.Content)
	default:
		c.JSON(http.StatusOK, result)
	}
}
