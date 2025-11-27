// cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/api"
	"github.com/Corphon/SceneIntruderMCP/internal/app"
	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/utils"
	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化结构化日志
	logFile := filepath.Join("logs", fmt.Sprintf("app_%s.log", time.Now().Format("2006-01-02")))
	if err := utils.InitLogger(logFile); err != nil {
		log.Printf("WARNING: 无法初始化结构化日志: %v", err)
		log.Println("🚀 启动 SceneIntruderMCP 服务器...")
	} else {
		logger := utils.GetLogger()
		logger.Info("SceneIntruderMCP server starting", nil)
	}

	// 1. 首先加载基础配置
	baseConfig, err := config.Load()
	if err != nil {
		utils.GetLogger().Fatal("Failed to load configuration", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	utils.GetLogger().Info("Configuration loaded successfully", map[string]interface{}{
		"port": baseConfig.Port,
	})

	// 2. 创建必要的目录
	createDirectories(baseConfig)
	utils.GetLogger().Info("Directory structure created", nil)

	// 3. 初始化配置系统
	if err := config.InitConfig(baseConfig.DataDir); err != nil {
		utils.GetLogger().Fatal("Failed to initialize configuration system", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	utils.GetLogger().Info("Configuration system initialized", nil)

	// 4. 初始化依赖注入容器
	container := di.GetContainer()
	utils.GetLogger().Info("Dependency injection container initialized", map[string]interface{}{
		"service_count": len(container.GetNames()),
	})

	// 5. 初始化所有服务（按依赖顺序）
	if err := app.InitServices(); err != nil {
		utils.GetLogger().Fatal("Failed to initialize services", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	utils.GetLogger().Info("All services initialized", nil)

	// 6. 设置路由（只获取服务，不创建）
	if err := performHealthCheck(); err != nil {
		utils.GetLogger().Warn("Service health check warning", map[string]interface{}{
			"error": err.Error(),
		})
	}

	router, err := api.SetupRouter()
	if err != nil {
		utils.GetLogger().Fatal("Failed to setup router", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	utils.GetLogger().Info("Router setup completed", nil)

	// Start metrics collection in background
	metrics := utils.NewAPIMetrics()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	metrics.StartMetricsCollection(ctx)

	// Initialize authentication system
	if err := api.InitializeAuth(); err != nil {
		utils.GetLogger().Error("Failed to initialize authentication system", map[string]interface{}{
			"error": err.Error(),
		})
		// Continue without auth for now (in production, this might be fatal)
	} else {
		utils.GetLogger().Info("Authentication system initialized", nil)
	}

	// 7. 启动服务器
	utils.GetLogger().Info("Server starting", map[string]interface{}{
		"port":         baseConfig.Port,
		"url":          fmt.Sprintf("http://localhost:%s", baseConfig.Port),
		"settings_url": fmt.Sprintf("http://localhost:%s/settings", baseConfig.Port),
	})

	setupGracefulShutdown(router, baseConfig.Port)
}

// 健康检查函数
func performHealthCheck() error {
	container := di.GetContainer()

	// 检查关键服务是否已注册
	criticalServices := []string{"llm", "scene", "config", "character"}

	for _, serviceName := range criticalServices {
		if service := container.Get(serviceName); service == nil {
			return fmt.Errorf("关键服务未注册: %s", serviceName)
		}
	}

	log.Println("✅ 服务健康检查通过")
	return nil
}

// 优雅关闭函数
func setupGracefulShutdown(router *gin.Engine, port string) {
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// 在新的 goroutine 中启动服务器
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ 启动服务器失败: %v", err)
		}
	}()

	// 等待中断信号以进行优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 正在关闭服务器...")

	// 给定超时时间关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("❌ 服务器强制关闭: %v", err)
	}

	log.Println("✅ 服务器优雅关闭完成")
}

// createDirectories 创建应用所需的目录结构
func createDirectories(cfg *config.Config) {
	dirs := []string{
		cfg.DataDir,
		filepath.Join(cfg.DataDir, "scenes"),
		filepath.Join(cfg.DataDir, "users"),
		filepath.Join(cfg.DataDir, "exports"),
		"temp",
		cfg.LogDir,
		cfg.StaticDir,
		cfg.TemplatesDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("创建目录失败 %s: %v", dir, err)
		}
	}

	verifyFrontendBuild(cfg)
}

// ensureStaticFiles 确保静态文件目录和基本文件存在

// verifyFrontendBuild 确保前端构建产物可用于 Go 服务
func verifyFrontendBuild(cfg *config.Config) {
	distDir := filepath.Join("frontend", "dist")
	info, err := os.Stat(distDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("⚠️ 未找到前端构建目录 %s，请先进入 frontend 执行 `npm install && npm run build`。", distDir)
		} else {
			log.Printf("⚠️ 检查前端构建目录失败: %v", err)
		}
		return
	}

	if !info.IsDir() {
		log.Printf("⚠️ %s 不是有效的目录，无法加载前端资源", distDir)
		return
	}

	if err := syncFrontendAssets(cfg, distDir); err != nil {
		log.Printf("⚠️ 同步前端资源失败: %v", err)
	}
}

// copyDirectory 递归复制目录内容
func copyDirectory(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		return copyFile(path, targetPath)
	})
}

func syncFrontendAssets(cfg *config.Config, distDir string) error {
	assetsSrc := filepath.Join(distDir, "assets")
	if err := ensureAssetsDirectory(assetsSrc, cfg.StaticDir); err != nil {
		return err
	}

	if err := ensureTemplatesFromSPA(distDir, cfg.TemplatesDir); err != nil {
		return err
	}

	return nil
}

func ensureAssetsDirectory(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("⚠️ 前端构建缺少 assets 目录: %s", src)
			return nil
		}
		return fmt.Errorf("检查 assets 目录失败: %w", err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("%s 不是有效的目录", src)
	}

	absSrc, _ := filepath.Abs(src)
	absDst, _ := filepath.Abs(dst)

	if absSrc == absDst {
		return nil // 直接指向同一目录，无需复制
	}

	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("清理静态目录失败: %w", err)
	}

	return copyDirectory(absSrc, absDst)
}

func ensureTemplatesFromSPA(distDir, templatesDir string) error {
	indexPath := filepath.Join(distDir, "index.html")
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("读取 index.html 失败: %w", err)
	}

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return fmt.Errorf("创建模板目录失败: %w", err)
	}

	templates := []string{
		"index.html",
	}

	for _, name := range templates {
		dst := filepath.Join(templatesDir, name)
		if err := os.WriteFile(dst, indexContent, 0644); err != nil {
			return fmt.Errorf("写入模板 %s 失败: %w", name, err)
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}
