// internal/api/error_codes.go
package api

// API错误代码常量
const (
	// 通用错误
	ErrorBadRequest    = "BAD_REQUEST"
	ErrorNotFound      = "NOT_FOUND"
	ErrorInternalError = "INTERNAL_ERROR"
	ErrorConflict      = "CONFLICT"
	ErrorForbidden     = "FORBIDDEN"
	ErrorUnauthorized  = "UNAUTHORIZED"

	// 场景相关错误
	ErrorSceneNotFound     = "SCENE_NOT_FOUND"
	ErrorSceneCreateFailed = "SCENE_CREATE_FAILED"
	ErrorSceneInvalid      = "SCENE_INVALID"

	// 角色相关错误
	ErrorCharacterNotFound = "CHARACTER_NOT_FOUND"
	ErrorCharacterInvalid  = "CHARACTER_INVALID"

	// 角色互动相关错误
	ErrorCharacterInteractionFailed   = "CHARACTER_INTERACTION_FAILED"
	ErrorInsufficientCharacters       = "INSUFFICIENT_CHARACTERS"
	ErrorConversationSimulationFailed = "CONVERSATION_SIMULATION_FAILED"
	ErrorInteractionHistoryFailed     = "INTERACTION_HISTORY_FAILED"
	ErrorInvalidInteractionParams     = "INVALID_INTERACTION_PARAMS"

	// 故事相关错误
	ErrorStoryNotFound     = "STORY_NOT_FOUND"
	ErrorChoiceInvalid     = "CHOICE_INVALID"
	ErrorChoiceAlreadyMade = "CHOICE_ALREADY_MADE"
	ErrorNodeNotFound      = "NODE_NOT_FOUND"

	// LLM服务相关错误
	ErrorLLMServiceUnavailable = "LLM_SERVICE_UNAVAILABLE"
	ErrorLLMConfigInvalid      = "LLM_CONFIG_INVALID"
	ErrorConnectionFailed      = "CONNECTION_FAILED"

	// 文件相关错误
	ErrorFileUploadFailed = "FILE_UPLOAD_FAILED"
	ErrorFileInvalid      = "FILE_INVALID"
	ErrorFileNotFound     = "FILE_NOT_FOUND"

	// 导出相关错误
	ErrorExportFailed             = "EXPORT_FAILED"
	ErrorExportServiceUnavailable = "EXPORT_SERVICE_UNAVAILABLE"
	ErrorExportFormatInvalid      = "EXPORT_FORMAT_INVALID"
	ErrorExportDataEmpty          = "EXPORT_DATA_EMPTY"
	ErrorExportTooLarge           = "EXPORT_TOO_LARGE"
	ErrorExportTimeout            = "EXPORT_TIMEOUT"

	// 配置健康相关
	ErrorConfigUnhealthy    = "CONFIG_UNHEALTHY"
	ErrorConfigNotLoaded    = "CONFIG_NOT_LOADED"
	ErrorLLMProviderMissing = "LLM_PROVIDER_MISSING"
	ErrorAPIKeyMissing      = "API_KEY_MISSING"
)
