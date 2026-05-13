package middleware

import (
	"log"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	apperrors "eino_ctf_agent/internal/errors"
	"eino_ctf_agent/internal/pkg/response"
)

// Recovery panic 恢复中间件。
// 捕获 goroutine panic，记录堆栈，返回统一错误格式，防止服务崩溃。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] %v\n%s", r, debug.Stack())
				response.Error(c, apperrors.Internal(
					apperrors.CodeInternalError,
					"internal server error",
				))
				c.Abort()
			}
		}()
		c.Next()
	}
}
