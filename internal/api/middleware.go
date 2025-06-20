// internal/api/middleware.go
package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

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
		// 这里简化处理，实际应用中应该有更复杂的认证逻辑
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API密钥缺失"})
			return
		}

		// 这里可以添加API密钥验证逻辑
		// 示例中简单使用固定值，实际应用应使用安全存储
		if apiKey != "dev_api_key" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "无效的API密钥"})
			return
		}

		c.Next()
	}
}

// RateLimiter 简单的请求限流中间件
func RateLimiter() gin.HandlerFunc {
	// 使用简单的计数器实现，实际应用应使用更复杂的算法如令牌桶
	type client struct {
		lastSeen time.Time
		count    int
	}

	// 存储客户端访问记录
	clients := make(map[string]*client)

	// 清理旧记录的goroutine
	ctx, cancel := context.WithCancel(context.Background())
	go func(ctx context.Context) {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
				// 清理5分钟前的记录
				threshold := time.Now().Add(-5 * time.Minute)
				for ip, c := range clients {
					if c.lastSeen.Before(threshold) {
						delete(clients, ip)
					}
				}
			}
		}
	}(ctx)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		// 获取或创建客户端记录
		cl, exists := clients[ip]
		if !exists {
			clients[ip] = &client{
				lastSeen: time.Now(),
				count:    1,
			}
			c.Next()
			return
		}

		// 检查是否在1分钟内超过100次请求
		if cl.count > 100 && time.Since(cl.lastSeen) < time.Minute {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "请求过于频繁，请稍后再试",
			})
			return
		}

		// 更新记录
		if time.Since(cl.lastSeen) >= time.Minute {
			cl.count = 0
		}
		cl.lastSeen = time.Now()
		cl.count++

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
