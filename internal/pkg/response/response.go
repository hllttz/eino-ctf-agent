package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "eino_ctf_agent/internal/errors"
)

// APIResponse API 统一成功响应体。
type APIResponse struct {
	Data any `json:"data,omitempty"`
}

// APIErrorResponse API 统一错误响应体。
type APIErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// OK 返回 200 成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, APIResponse{Data: data})
}

// Created 返回 201 创建成功。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, APIResponse{Data: data})
}

// NoContent 返回 204。
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error 根据 AppError 返回统一错误响应。
// 若 appErr 为 nil 则回退到 generic 500。
func Error(c *gin.Context, appErr *apperrors.AppError) {
	if appErr == nil {
		c.JSON(http.StatusInternalServerError, APIErrorResponse{
			Error:   apperrors.CodeInternalError,
			Message: "internal error",
		})
		return
	}
	c.JSON(appErr.StatusCode(), APIErrorResponse{
		Error:   appErr.Code,
		Message: appErr.Message,
	})
}

// ErrorRaw 返回自定义 code/message/status 的错误响应。
func ErrorRaw(c *gin.Context, code, message string, status int) {
	c.JSON(status, APIErrorResponse{
		Error:   code,
		Message: message,
	})
}
