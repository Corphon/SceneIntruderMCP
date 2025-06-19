package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"math"
	"os"
	"path/filepath"

	"github.com/Corphon/SceneIntruderMCP/internal/app"
	"github.com/Corphon/SceneIntruderMCP/internal/config"
)

func main() {
	// 设置日志
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// 创建必要的目录
	createDirectories()

	// 加载基础配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化应用
	if err := app.Initialize(cfg.DataDir); err != nil {
		log.Fatalf("初始化应用失败: %v", err)
	}

	// 运行应用
	if err := app.Run(); err != nil {
		log.Fatalf("运行应用失败: %v", err)
	}
}

// createDirectories 创建应用所需的目录结构
func createDirectories() {
	dirs := []string{
		"data",
		"data/scenes",
		"temp",
		"logs",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("创建目录失败 %s: %v", dir, err)
		}
	}

	// 确保静态文件目录存在
	ensureStaticFiles()
}

// ensureStaticFiles 确保静态文件目录和基本文件存在
func ensureStaticFiles() {
	// 确保目录存在
	dirs := []string{
		"static",
		"static/css",
		"static/js",
		"static/images",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("创建静态文件目录失败 %s: %v", dir, err)
		}
	}

	// 如果没有示例图像，创建一个默认的 emoji 风格图像
	defaultImagePath := filepath.Join("static", "images", "default-character.jpg")
	if _, err := os.Stat(defaultImagePath); os.IsNotExist(err) {
		log.Printf("默认角色图像不存在，生成替代图像: %s", defaultImagePath)

		if err := generateEmojiImage(defaultImagePath); err != nil {
			log.Printf("警告: 无法生成替代图像: %v", err)
			log.Println("请确保提供一个默认的角色图像文件")
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
