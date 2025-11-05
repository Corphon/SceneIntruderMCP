// internal/api/rate_limit_middleware.go
package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a simple rate limiter using a token bucket algorithm
type RateLimiter struct {
	visitors map[string]*Visitor
	mu       sync.RWMutex
}

// Visitor represents a client with rate limiting data
type Visitor struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*Visitor),
	}

	// Start cleanup goroutine to remove old entries
	go rl.cleanup()

	return rl
}

// cleanup removes visitors that haven't made requests in over an hour
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, visitor := range rl.visitors {
			if now.After(visitor.Reset) {
				delete(rl.visitors, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow checks if a visitor is allowed to make a request
func (rl *RateLimiter) Allow(key string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	visitor, exists := rl.visitors[key]

	if !exists || now.After(visitor.Reset) {
		// New visitor or previous window has expired
		rl.visitors[key] = &Visitor{
			Limit:     limit,
			Remaining: limit - 1,
			Reset:     now.Add(window),
		}
		return true
	}

	if visitor.Remaining <= 0 {
		// No remaining requests in this window
		return false
	}

	// Decrement remaining requests
	visitor.Remaining--
	return true
}

// GetRateLimitHeaders returns the rate limit headers
func (rl *RateLimiter) GetRateLimitHeaders(key string, limit int, window time.Duration) (int, int, int64) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	visitor, exists := rl.visitors[key]
	if !exists {
		return limit, limit, time.Now().Add(window).Unix()
	}

	remaining := visitor.Remaining
	if remaining < 0 {
		remaining = 0
	}

	resetTime := visitor.Reset.Unix()
	return limit, remaining, resetTime
}

// Global rate limiter instance
var rateLimiter = NewRateLimiter()

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(limit int, window time.Duration, keyFunc func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := keyFunc(c)

		if !rateLimiter.Allow(key, limit, window) {
			// Get current rate limit values for headers
			limit, remaining, reset := rateLimiter.GetRateLimitHeaders(key, limit, window)

			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", reset))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"success":   false,
				"error":     "Rate limit exceeded",
				"code":      "RATE_LIMIT_EXCEEDED",
				"timestamp": time.Now().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		// Add rate limit headers to response
		limit, remaining, reset := rateLimiter.GetRateLimitHeaders(key, limit, window)
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", reset))

		c.Next()
	}
}

// RateLimitByIP applies rate limiting based on client IP address
func RateLimitByIP(limit int, window time.Duration) gin.HandlerFunc {
	return RateLimitMiddleware(limit, window, func(c *gin.Context) string {
		// Get real IP considering proxies
		clientIP := c.ClientIP()
		return clientIP
	})
}

// RateLimitByUser applies rate limiting based on user ID from context or headers
func RateLimitByUser(limit int, window time.Duration) gin.HandlerFunc {
	return RateLimitMiddleware(limit, window, func(c *gin.Context) string {
		// Try to get user ID from context or header
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			// Fallback to IP if no user ID is provided
			userID = c.ClientIP()
		}
		return userID
	})
}

// ChatRateLimit applies specific rate limiting for chat endpoints
func ChatRateLimit() gin.HandlerFunc {
	// 30 requests per minute for chat endpoints
	return RateLimitByUser(30, time.Minute)
}

// AnalysisRateLimit applies specific rate limiting for analysis endpoints
func AnalysisRateLimit() gin.HandlerFunc {
	// 10 requests per hour for analysis endpoints (they're more resource-intensive)
	return RateLimitByUser(10, time.Hour)
}

// DefaultRateLimit applies general rate limiting for most API endpoints
func DefaultRateLimit() gin.HandlerFunc {
	// 100 requests per minute by IP
	return RateLimitByIP(100, time.Minute)
}
