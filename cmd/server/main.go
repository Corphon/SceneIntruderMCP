// cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"log"
	"math"
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
	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("🚀 启动 SceneIntruderMCP 服务器...")

	// 1. 首先加载基础配置
	baseConfig, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Printf("✅ 基础配置加载完成，端口: %s", baseConfig.Port)

	// 2. 创建必要的目录
	createDirectories(baseConfig)
	log.Println("✅ 目录结构创建完成")

	// 3. 初始化配置系统
	if err := config.InitConfig(baseConfig.DataDir); err != nil {
		log.Fatalf("初始化配置系统失败: %v", err)
	}
	log.Println("✅ 配置系统初始化完成")

	// 4. 初始化依赖注入容器
	container := di.GetContainer()
	log.Printf("✅ 依赖注入容器初始化完成，服务数量: %d", len(container.GetNames()))

	// 5. 初始化所有服务（按依赖顺序）
	if err := app.InitServices(); err != nil {
		log.Fatalf("初始化服务失败: %v", err)
	}
	log.Println("✅ 所有服务初始化完成")

	// 6. 设置路由（只获取服务，不创建）
	if err := performHealthCheck(); err != nil {
		log.Printf("⚠️ 服务健康检查警告: %v", err)
	}

	router, err := api.SetupRouter()
	if err != nil {
		log.Fatalf("❌ 设置路由失败: %v", err)
	}
	log.Println("✅ 路由设置完成")

	// 7. 启动服务器
	log.Printf("🌐 服务器启动在端口 %s", baseConfig.Port)
	log.Printf("🔗 访问地址: http://localhost:%s", baseConfig.Port)
	log.Printf("🔗 设置页面: http://localhost:%s/settings", baseConfig.Port)

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
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("创建目录失败 %s: %v", dir, err)
		}
	}

	// 确保静态文件目录存在
	ensureStaticFiles(cfg)
}

// ensureStaticFiles 确保静态文件目录和基本文件存在
func ensureStaticFiles(cfg *config.Config) {
	// 确保目录存在
	dirs := []string{
		cfg.StaticDir,
		filepath.Join(cfg.StaticDir, "css"),
		filepath.Join(cfg.StaticDir, "js"),
		filepath.Join(cfg.StaticDir, "images"),
		filepath.Join(cfg.StaticDir, "uploads"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("创建静态文件目录失败 %s: %v", dir, err)
		}
	}

	// 复制静态文件
	log.Println("🔧 复制静态文件...")
	copyStaticFiles(cfg)

	// 如果没有示例图像，创建一个默认的
	defaultImagePath := filepath.Join(cfg.StaticDir, "images", "default-character.jpg")
	if _, err := os.Stat(defaultImagePath); os.IsNotExist(err) {
		log.Printf("默认角色图像不存在，生成替代图像: %s", defaultImagePath)

		if err := generateEmojiImage(defaultImagePath); err != nil {
			log.Printf("警告: 无法生成替代图像: %v", err)
		} else {
			log.Printf("成功生成默认角色替代图像")
		}
	}
}

// generateEmojiImage 生成一个简单的彩色图像作为角色头像
func generateEmojiImage(outputPath string) error {
	// 图像尺寸
	width, height := 512, 512

	// 创建一个新的 RGBA 图像
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 设置背景颜色 - 使用柔和的蓝色
	bgColor := color.RGBA{66, 133, 244, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// 生成一个简单的图案 - 中心渐变圆
	center := image.Point{width / 2, height / 2}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 计算当前点到中心的距离
			dx := float64(x - center.X)
			dy := float64(y - center.Y)
			distance := math.Sqrt(dx*dx + dy*dy)

			// 创建基于距离的渐变效果
			if distance < float64(width/2) {
				// 渐变效果 - 距离越远越暗
				factor := 1.0 - (distance / float64(width/2) * 0.7)

				// 生成不同色调
				r := uint8(86 + 100*factor)
				g := uint8(153 + 70*factor)
				b := uint8(244)

				img.Set(x, y, color.RGBA{r, g, b, 255})
			}
		}
	}

	// 生成一个简单的边框
	borderWidth := 10
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x < borderWidth || x >= width-borderWidth ||
				y < borderWidth || y >= height-borderWidth {
				img.Set(x, y, color.RGBA{41, 98, 198, 255}) // 深蓝色边框
			}
		}
	}

	// 保存为JPEG图像
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建图像文件失败: %w", err)
	}
	defer outputFile.Close()

	return jpeg.Encode(outputFile, img, &jpeg.Options{Quality: 90})
}

// 静态文件复制函数
func copyStaticFiles(cfg *config.Config) {
	// 定义需要复制的静态文件
	staticFiles := map[string]string{
		// 源文件路径 -> 目标文件路径
		"static/js/app.js":          filepath.Join(cfg.StaticDir, "js", "app.js"),
		"static/js/app-loader.js":   filepath.Join(cfg.StaticDir, "js", "app-loader.js"),
		"static/js/api.js":          filepath.Join(cfg.StaticDir, "js", "api.js"),
		"static/js/emotions.js":     filepath.Join(cfg.StaticDir, "js", "emotions.js"),
		"static/js/export.js":       filepath.Join(cfg.StaticDir, "js", "export.js"),
		"static/js/story.js":        filepath.Join(cfg.StaticDir, "js", "story.js"),
		"static/js/realtime.js":     filepath.Join(cfg.StaticDir, "js", "realtime.js"),
		"static/js/user-profile.js": filepath.Join(cfg.StaticDir, "js", "user-profile.js"),
		"static/js/utils.js":        filepath.Join(cfg.StaticDir, "js", "utils.js"),
		"static/css/style.css":      filepath.Join(cfg.StaticDir, "css", "style.css"),
	}

	for src, dst := range staticFiles {
		// 检查源文件是否存在
		if _, err := os.Stat(src); os.IsNotExist(err) {
			log.Printf("警告: 静态文件不存在，跳过复制: %s", src)
			continue
		}

		// 检查目标文件是否已存在
		if _, err := os.Stat(dst); err == nil {
			log.Printf("静态文件已存在，跳过复制: %s", dst)
			continue
		}

		// 复制文件
		if err := copyFile(src, dst); err != nil {
			log.Printf("警告: 复制静态文件失败 %s -> %s: %v", src, dst, err)
		} else {
			log.Printf("成功复制静态文件: %s -> %s", src, dst)
		}
	}
}

// 文件复制辅助函数
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
