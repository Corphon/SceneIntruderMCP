// internal/api/response_helpers.go
package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/gin-gonic/gin"
)

// ResponseHelper 响应助手类
type ResponseHelper struct{}

// NewResponseHelper 创建响应助手
func NewResponseHelper() *ResponseHelper {
	return &ResponseHelper{}
}

// Success 成功响应
func (rh *ResponseHelper) Success(c *gin.Context, data interface{}, message ...string) {
	response := &APIResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
		RequestID: rh.getRequestID(c),
	}

	if len(message) > 0 {
		response.Message = message[0]
	}

	c.JSON(http.StatusOK, response)
}

// Created 创建成功响应
func (rh *ResponseHelper) Created(c *gin.Context, data interface{}, message ...string) {
	response := &APIResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
		RequestID: rh.getRequestID(c),
	}

	if len(message) > 0 {
		response.Message = message[0]
	} else {
		response.Message = "资源创建成功"
	}

	c.JSON(http.StatusCreated, response)
}

// sanitizeErrorMessage removes sensitive information from error messages
func sanitizeErrorMessage(message string) string {
	// Remove potential sensitive information like API keys, file paths, etc.
	// This is a basic implementation - you might want to expand this list
	sensitivePatterns := []string{
		"api_key", "API_KEY", "apikey", "ApiKey", 
		"password", "secret", "token", 
		"config", "CONFIG", "config_",
		"env", "ENV", "environment",
		"/", "\\", "C:", "D:", // File path indicators
		".json", ".yaml", ".yml", ".env", // File extensions that might contain sensitive info
	}
	
	sanitized := message
	for _, pattern := range sensitivePatterns {
		// Replace occurrences with generic text to prevent information disclosure
		if strings.Contains(strings.ToLower(sanitized), strings.ToLower(pattern)) {
			// For security reasons, we'll replace the entire message if it contains sensitive patterns
			// This is conservative approach - in production you might want more sophisticated sanitization
			if strings.Contains(strings.ToLower(sanitized), "api_key") || 
			   strings.Contains(strings.ToLower(sanitized), "secret") ||
			   strings.Contains(strings.ToLower(sanitized), "token") {
				return "An internal error occurred"
			}
		}
	}
	return sanitized
}

// Error 错误响应
func (rh *ResponseHelper) Error(c *gin.Context, statusCode int, errorCode, message string, details ...string) {
	// Sanitize the error message to prevent information disclosure
	sanitizedMessage := sanitizeErrorMessage(message)
	
	apiError := &APIError{
		Code:    errorCode,
		Message: sanitizedMessage,
	}

	if len(details) > 0 {
		// Also sanitize details
		apiError.Details = sanitizeErrorMessage(details[0])
	}

	response := &APIResponse{
		Success:   false,
		Error:     apiError,
		Timestamp: time.Now(),
		RequestID: rh.getRequestID(c),
	}

	c.JSON(statusCode, response)
}

// BadRequest 400错误响应
func (rh *ResponseHelper) BadRequest(c *gin.Context, message string, details ...string) {
	rh.Error(c, http.StatusBadRequest, "BAD_REQUEST", message, details...)
}

// NotFound 404错误响应
func (rh *ResponseHelper) NotFound(c *gin.Context, resource string, details ...string) {
	message := resource + "不存在"
	code := "NOT_FOUND"
	if resource != "" {
		code = rh.getResourceNotFoundCode(resource)
	}
	rh.Error(c, http.StatusNotFound, code, message, details...)
}

// InternalError 500错误响应
func (rh *ResponseHelper) InternalError(c *gin.Context, message string, details ...string) {
	rh.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", message, details...)
}

// Conflict 409错误响应
func (rh *ResponseHelper) Conflict(c *gin.Context, message string, details ...string) {
	rh.Error(c, http.StatusConflict, "CONFLICT", message, details...)
}

// Forbidden 403错误响应
func (rh *ResponseHelper) Forbidden(c *gin.Context, message string, details ...string) {
	rh.Error(c, http.StatusForbidden, "FORBIDDEN", message, details...)
}

// PaginatedSuccess 分页成功响应
func (rh *ResponseHelper) PaginatedSuccess(c *gin.Context, data interface{}, meta *PaginationMeta, message ...string) {
	response := &PaginatedResponse{
		APIResponse: &APIResponse{
			Success:   true,
			Data:      data,
			Timestamp: time.Now(),
			RequestID: rh.getRequestID(c),
		},
		Meta: meta,
	}

	if len(message) > 0 {
		response.APIResponse.Message = message[0]
	}

	c.JSON(http.StatusOK, response)
}

// FileResponse 文件下载响应
func (rh *ResponseHelper) FileResponse(c *gin.Context, content string, filename string, contentType string) {
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.String(http.StatusOK, content)
}

// getRequestID 获取请求ID
func (rh *ResponseHelper) getRequestID(c *gin.Context) string {
	if requestID := c.GetString("request_id"); requestID != "" {
		return requestID
	}
	return ""
}

// getResourceNotFoundCode 根据资源类型生成错误代码
func (rh *ResponseHelper) getResourceNotFoundCode(resource string) string {
	switch resource {
	case "场景", "scene":
		return "SCENE_NOT_FOUND"
	case "角色", "character":
		return "CHARACTER_NOT_FOUND"
	case "故事", "story":
		return "STORY_NOT_FOUND"
	case "用户", "user":
		return "USER_NOT_FOUND"
	default:
		return "RESOURCE_NOT_FOUND"
	}
}

// ExportResponse 导出响应（专用于导出功能）
func (rh *ResponseHelper) ExportResponse(c *gin.Context, result *models.ExportResult, format string) {
	switch strings.ToLower(format) {
	case "json":
		rh.Success(c, result, "导出成功")
	case "markdown", "txt":
		rh.FileResponse(c, result.Content, filepath.Base(result.FilePath), "text/plain; charset=utf-8")
	case "html":
		rh.FileResponse(c, result.Content, filepath.Base(result.FilePath), "text/html; charset=utf-8")
	case "csv":
		rh.FileResponse(c, result.Content, filepath.Base(result.FilePath), "text/csv; charset=utf-8")
	case "pdf":
		rh.FileResponse(c, result.Content, filepath.Base(result.FilePath), "application/pdf")
	default:
		rh.Success(c, result, "导出成功")
	}
}

// StreamResponse 流式响应（用于大文件或实时数据）
func (rh *ResponseHelper) StreamResponse(c *gin.Context, contentType string, callback func(writer gin.ResponseWriter) error) {
	c.Header("Content-Type", contentType)
	c.Header("Transfer-Encoding", "chunked")
	c.Header("Cache-Control", "no-cache")

	c.Stream(func(w io.Writer) bool {
		if err := callback(c.Writer); err != nil {
			// 记录错误但不中断流
			log.Printf("流式响应错误: %v", err)
			return false
		}
		return true
	})
}

// DownloadResponse 下载响应（强制下载）
func (rh *ResponseHelper) DownloadResponse(c *gin.Context, content string, filename string, contentType string) {
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Header("Content-Length", fmt.Sprintf("%d", len(content)))
	c.String(http.StatusOK, content)
}
