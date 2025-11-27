// internal/api/auth_middleware.go
package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/auth"
	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/services"
	"github.com/gin-gonic/gin"
)

var tokenConfig *auth.TokenConfig

// InitializeAuth initializes the authentication system with config
func InitializeAuth() error {
	cfg := config.GetCurrentConfig()
	if cfg == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Generate a secure random key, falling back to a more secure pattern if environment not set
	var secret []byte
	var err error

	// Try to get secret from environment variable first
	envSecret := os.Getenv("AUTH_SECRET_KEY")
	if envSecret != "" {
		secret = []byte(envSecret)
	} else {
		// Check if in debug mode and use a consistent key for testing
		if os.Getenv("DEBUG_MODE") == "true" || cfg.DebugMode {
			// Use a consistent key during development to avoid session issues on restart
			secret = []byte("dev_auth_key_for_testing_purposes_only_")
			log.Printf("⚠️ 警告: 开发模式下使用固定认证密钥，生产环境请通过环境变量设置 AUTH_SECRET_KEY")
		} else {
			// Generate a truly random secret key if none is provided
			secret, err = auth.GenerateSecureKey(32) // 256-bit key
			if err != nil {
				// Fallback to a reasonably secure key based on multiple entropy sources
				entropy := fmt.Sprintf("%s_%d_%d", cfg.DataDir, time.Now().UnixNano(), os.Getpid())
				secret = []byte(entropy)
				log.Printf("Warning: When using derived keys, it is recommended to set them in environment variables AUTH_SECRET_KEY")
			}
		}
	}

	// Ensure the secret is at least 32 bytes
	if len(secret) < 32 {
		// Pad the secret if it's too short
		paddedSecret := make([]byte, 32)
		copy(paddedSecret, secret)
		secret = paddedSecret
	} else if len(secret) > 32 {
		// Truncate if too long
		secret = secret[:32]
	}

	tokenConfig = &auth.TokenConfig{
		Secret:     secret,
		Expiration: 24 * time.Hour, // Token expires in 24 hours
	}

	return nil
}

// AuthMiddleware provides authentication for API endpoints
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for certain endpoints (like login, health checks, etc.)
		if isPublicEndpoint(c) {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// Allow guest usage by treating missing credentials as console_user
			c.Set("user_id", "console_user")
			c.Set("user_authenticated", false)
			c.Next()
			return
		}

		// Extract token from "Bearer {token}" format
		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)

		if token == "" {
			c.Set("user_id", "console_user")
			c.Set("user_authenticated", false)
			c.Next()
			return
		}

		// Parse and validate token
		parsedToken, err := auth.ParseToken(token, tokenConfig)
		if err != nil {
			log.Printf("AuthMiddleware: invalid token detected (%v), downgrading to console_user", err)
			c.Set("user_id", "console_user")
			c.Set("user_authenticated", false)
			c.Set("auth_error", err.Error())
			c.Next()
			return
		}

		// Add user info to context for use in handlers
		c.Set("user_id", parsedToken.UserID)
		c.Set("user_authenticated", true)

		c.Next()
	}
}

// isPublicEndpoint checks if the current endpoint should skip authentication
func isPublicEndpoint(c *gin.Context) bool {
	publicPaths := []string{
		"/api/llm/status",               // LLM status for setup
		"/api/llm/models",               // LLM models for setup
		"/api/ws/status",                // WebSocket status
		"/",                             // Main page
		"/scenes",                       // Scene listing (could be public)
		"/scenes/create",                // Scene creation (could be public)
		"/settings",                     // Settings page - needs to be accessible for initial setup
		"/login",                        // Login page
		"/register",                     // Register page
		"/api/auth/login",               // Login API endpoint
		"/api/auth/logout",              // Logout API endpoint
		"/api/settings/test-connection", // Test connection should be accessible without auth for initial setup
	}

	currentPath := c.Request.URL.Path

	for _, path := range publicPaths {
		if currentPath == path || strings.HasPrefix(currentPath, path+"/") {
			return true
		}
	}

	// Check for specific public routes
	if c.Request.Method == "GET" &&
		(strings.HasPrefix(currentPath, "/static/") ||
			strings.HasPrefix(currentPath, "/scenes/") ||
			strings.HasPrefix(currentPath, "/uploads/") ||
			strings.HasSuffix(currentPath, ".css") ||
			strings.HasSuffix(currentPath, ".js") ||
			strings.HasSuffix(currentPath, ".png") ||
			strings.HasSuffix(currentPath, ".jpg") ||
			strings.HasSuffix(currentPath, ".jpeg") ||
			strings.HasSuffix(currentPath, ".gif") ||
			strings.HasSuffix(currentPath, ".ico") ||
			strings.HasSuffix(currentPath, ".svg") ||
			strings.HasSuffix(currentPath, ".woff") ||
			strings.HasSuffix(currentPath, ".woff2")) {
		return true
	}

	return false
}

// GenerateUserToken creates an authentication token for a user
func GenerateUserToken(userID string) (string, error) {
	if tokenConfig == nil {
		return "", fmt.Errorf("auth not initialized")
	}

	return auth.GenerateToken(userID, tokenConfig)
}

// GetUserFromContext retrieves the authenticated user from the context
func GetUserFromContext(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}

	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		return "", false
	}

	if authenticatedVal, exists := c.Get("user_authenticated"); exists {
		if authenticated, ok := authenticatedVal.(bool); ok {
			return userIDStr, authenticated
		}
	}

	return userIDStr, false
}

// RequireAuthForScene ensures the user has access to a specific scene
func RequireAuthForScene() gin.HandlerFunc {
	return func(c *gin.Context) {
		sceneID := c.Param("id")
		userID, userAuthenticated := GetUserFromContext(c)

		// If user is not authenticated, allow access to public scenes only
		// For now, we'll implement a simple check - in a real system, you'd check scene permissions
		if !userAuthenticated {
			// For unauthenticated users, we could check if the scene is public
			// In our file-based system, we'll allow access for now but could implement
			// a check for scene ownership or permissions later
			c.Next()
			return
		}

		// If user is authenticated, verify they have access to this scene
		if sceneID != "" && userID != "" {
			// In a real implementation, we'd check if the scene belongs to the user
			// For example, we might check a scene metadata file for ownership info
			// Since we're using file-based storage, we could check if the scene exists
			// and potentially verify ownership by checking scene creator information

			// For now, we'll get the scene service from the DI container to check access
			container := di.GetContainer()
			sceneService, ok := container.Get("scene").(*services.SceneService)
			if !ok {
				// If we can't access the scene service, continue but log a warning
				c.Next()
				return
			}

			// Try to load the scene to verify it exists
			_, err := sceneService.LoadScene(sceneID)
			if err != nil {
				// Scene doesn't exist or can't be loaded
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error":   "Scene not found",
					"code":    "SCENE_NOT_FOUND",
				})
				c.Abort()
				return
			}

			// In a real system, we'd check if sceneData.Scene.UserID == userID
			// For now, we'll just ensure the scene exists and continue
			// A more complete implementation would check actual ownership/permissions
		}

		c.Next()
	}
}

// RequireAuthForUser ensures the user can only access their own data
func RequireAuthForUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestedUserID := c.Param("user_id")
		authUserID, userAuthenticated := GetUserFromContext(c)

		if !userAuthenticated {
			if requestedUserID == "console_user" {
				// Allow console user to access personal resources during guest sessions
				c.Set("user_id", requestedUserID)
				c.Set("user_authenticated", false)
				c.Next()
				return
			}
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "Authentication required",
				"code":    "FORBIDDEN",
			})
			c.Abort()
			return
		}

		// Allow access if the requested user ID matches the authenticated user ID
		if requestedUserID != authUserID {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "Access denied: Cannot access other users' data",
				"code":    "FORBIDDEN",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
