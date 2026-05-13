package errors

import "net/http"

// 通用业务错误码。
const (
	CodeInvalidRequest   = "invalid_request"
	CodeUnauthorized     = "unauthorized"
	CodeNotFound         = "not_found"
	CodeInternalError    = "internal_error"
	CodeIndexFailed      = "index_failed"
	CodeEmbeddingFailed  = "embedding_failed"
	CodeLLMFailed        = "llm_failed"
	CodeToolCallFailed   = "tool_call_failed"
	CodeAgentMaxSteps    = "agent_max_steps"
	CodeFileUploadFailed = "file_upload_failed"
	CodeUnsupportedType  = "unsupported_type"
	CodeTimeout          = "timeout"
)

// AppError 应用层统一错误，handler 层通过 StatusCode 决定 HTTP 状态码。
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	status  int
}

func (e *AppError) Error() string   { return e.Message }
func (e *AppError) StatusCode() int { return e.status }

// New 创建 AppError，code 为业务错误码，status 为 HTTP 状态码。
func New(code string, message string, status int) *AppError {
	return &AppError{Code: code, Message: message, status: status}
}

// BadRequest 400 错误快捷构造函数。
func BadRequest(code, message string) *AppError {
	return New(code, message, http.StatusBadRequest)
}

// NotFound 404 错误快捷构造函数。
func NotFound(code, message string) *AppError {
	return New(code, message, http.StatusNotFound)
}

// Internal 500 错误快捷构造函数。
func Internal(code, message string) *AppError {
	return New(code, message, http.StatusInternalServerError)
}
