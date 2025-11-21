// internal/errors/errors.go
package errors

import (
	"errors"
	"fmt"
)

// ErrorType 定义错误类型
type ErrorType string

const (
	// 通用错误类型
	ErrorTypeValidation   ErrorType = "validation_error"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeError        ErrorType = "processing_error"
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	ErrorTypeForbidden    ErrorType = "forbidden"
	ErrorTypeConflict     ErrorType = "conflict"
	ErrorTypeTimeout      ErrorType = "timeout"
)

// AppError 应用程序错误结构
type AppError struct {
	Type    ErrorType
	Message string
	Err     error
	Code    string // 用户友好的错误代码
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap 实现错误链接
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError 创建新的 AppError
func NewAppError(errType ErrorType, message string, originalError error) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Err:     originalError,
		Code:    generateErrorCode(errType),
	}
}

// NewValidationError 创建验证错误
func NewValidationError(message string, originalError error) *AppError {
	return NewAppError(ErrorTypeValidation, message, originalError)
}

// NewNotFoundError 创建未找到错误
func NewNotFoundError(message string, originalError error) *AppError {
	return NewAppError(ErrorTypeNotFound, message, originalError)
}

// NewProcessingError 创建处理错误
func NewProcessingError(message string, originalError error) *AppError {
	return NewAppError(ErrorTypeError, message, originalError)
}

// NewUnauthorizedError 创建未授权错误
func NewUnauthorizedError(message string, originalError error) *AppError {
	return NewAppError(ErrorTypeUnauthorized, message, originalError)
}

// NewForbiddenError 创建禁止错误
func NewForbiddenError(message string, originalError error) *AppError {
	return NewAppError(ErrorTypeForbidden, message, originalError)
}

// NewConflictError 创建冲突错误
func NewConflictError(message string, originalError error) *AppError {
	return NewAppError(ErrorTypeConflict, message, originalError)
}

// IsValidationError 检查是否为验证错误
func IsValidationError(err error) bool {
	var appError *AppError
	if errors.As(err, &appError) {
		return appError.Type == ErrorTypeValidation
	}
	return false
}

// IsNotFoundError 检查是否为未找到错误
func IsNotFoundError(err error) bool {
	var appError *AppError
	if errors.As(err, &appError) {
		return appError.Type == ErrorTypeNotFound
	}
	return false
}

// IsUnauthorizedError 检查是否为未授权错误
func IsUnauthorizedError(err error) bool {
	var appError *AppError
	if errors.As(err, &appError) {
		return appError.Type == ErrorTypeUnauthorized
	}
	return false
}

// IsForbiddenError 检查是否为禁止错误
func IsForbiddenError(err error) bool {
	var appError *AppError
	if errors.As(err, &appError) {
		return appError.Type == ErrorTypeForbidden
	}
	return false
}

// IsConflictError 检查是否为冲突错误
func IsConflictError(err error) bool {
	var appError *AppError
	if errors.As(err, &appError) {
		return appError.Type == ErrorTypeConflict
	}
	return false
}

// generateErrorCode 根据错误类型生成错误代码
func generateErrorCode(errType ErrorType) string {
	switch errType {
	case ErrorTypeValidation:
		return "VALIDATION_ERROR"
	case ErrorTypeNotFound:
		return "NOT_FOUND"
	case ErrorTypeError:
		return "PROCESSING_ERROR"
	case ErrorTypeUnauthorized:
		return "UNAUTHORIZED"
	case ErrorTypeForbidden:
		return "FORBIDDEN"
	case ErrorTypeConflict:
		return "CONFLICT"
	case ErrorTypeTimeout:
		return "TIMEOUT"
	default:
		return "UNKNOWN_ERROR"
	}
}

// WrapError 包装现有错误
func WrapError(err error, message string, errType ErrorType) error {
	if err == nil {
		return nil
	}
	
	var appError *AppError
	if errors.As(err, &appError) {
		// 如果已经是 AppError，只更新消息
		return &AppError{
			Type:    appError.Type,
			Message: fmt.Sprintf("%s: %s", message, appError.Message),
			Err:     appError,
			Code:    appError.Code,
		}
	}
	
	// 否则创建新的 AppError
	return NewAppError(errType, message, err)
}
