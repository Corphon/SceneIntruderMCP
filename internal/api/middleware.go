// internal/api/middleware.go
package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/gin-gonic/gin"
)

// Logger 中间件记录请求日志
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 请求开始时间
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 请求结束时间
		endTime := time.Now()
		// 执行时间
		latencyTime := endTime.Sub(startTime)
		// 请求方式
		reqMethod := c.Request.Method
		// 请求路由
		reqUri := c.Request.RequestURI
		// 状态码
		statusCode := c.Writer.Status()
		// 请求IP
		clientIP := c.ClientIP()

		// 日志格式
		fmt.Fprintf(gin.DefaultWriter, "| %d | %v | %s | %s | %s\n",
			statusCode, latencyTime, clientIP, reqMethod, reqUri)
	}
}

// ErrorHandler 中间件处理错误
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) > 0 {
			// 获取最后一个错误
			err := c.Errors.Last()

			// 根据错误类型返回不同状态码
			switch err.Type {
			case gin.ErrorTypeBind:
				// 参数绑定错误
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			case gin.ErrorTypeRender:
				// 响应渲染错误
				c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
			case gin.ErrorTypePrivate:
				// 自定义错误，状态码可能已经设置
				if !c.Writer.Written() {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				}
			default:
				// 其他错误
				c.JSON(http.StatusInternalServerError, gin.H{"error": "未知错误"})
			}
		}
	}
}

// Auth 中间件验证请求身份
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取API密钥
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key") // Also check query parameter as fallback
		}

		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API密钥缺失"})
			return
		}

		// 从配置中获取有效的API密钥进行验证
		cfg := config.GetCurrentConfig()
		if cfg == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "配置系统未初始化"})
			return
		}

		// 获取解密后的API密钥进行比较
		// The LLMConfig field in the config returned by GetCurrentConfig already contains decrypted values
		validAPIKey := ""
		if cfg.LLMConfig != nil {
			validAPIKey = cfg.LLMConfig["api_key"]
		}
		if validAPIKey == "" || apiKey != validAPIKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "无效的API密钥"})
			return
		}

		c.Next()
	}
}

// RequestSizeLimiter 限制请求体大小
func RequestSizeLimiter(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}

// Timeout 请求超时中间件
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 创建超时上下文
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		// 替换请求上下文
		c.Request = c.Request.WithContext(ctx)

		// 创建完成通道
		done := make(chan bool)

		// 处理请求的goroutine
		go func() {
			c.Next()
			done <- true
		}()

		// 等待请求完成或超时
		select {
		case <-done:
			// 请求正常完成
			return
		case <-ctx.Done():
			// 请求超时
			if ctx.Err() == context.DeadlineExceeded {
				c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{
					"error": "请求处理超时",
				})
			}
		}
	}
}

// SecureHeaders 添加安全相关的HTTP头
func SecureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}
