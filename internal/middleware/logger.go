package middleware

import (
	"log"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"

	"eino_ctf_agent/internal/service"
)

// apiKeyPattern 匹配常见 API key/secret/token 模式。
var apiKeyPattern = regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password|authorization)\s*[:=]\s*[^\s,;"]+`)

// sanitizeLog 对日志消息中的 API key 进行脱敏。
func sanitizeLog(msg string) string {
	return apiKeyPattern.ReplaceAllString(msg, "$1=***REDACTED***")
}

// traceIDFromContext 从 gin context 的底层 request context 中提取 traceID。
func traceIDFromContext(c *gin.Context) string {
	if id, ok := service.TraceIDFromContext(c.Request.Context()); ok {
		return id
	}
	return ""
}

// Logger 请求日志中间件，记录 request_id、耗时、状态码。
// LLM API Key 和敏感信息自动脱敏。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		traceID := traceIDFromContext(c)

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		clientIP := c.ClientIP()

		logMsg := sanitizeLog(
			formatLog(traceID, method, path, status, duration, clientIP),
		)
		log.Println(logMsg)
	}
}

func formatLog(traceID, method, path string, status int, duration time.Duration, clientIP string) string {
	if traceID != "" {
		return "[REQ] trace=" + traceID +
			" method=" + method +
			" path=" + path +
			" status=" + itoa(status) +
			" duration=" + duration.String() +
			" ip=" + clientIP
	}
	return "[REQ] method=" + method +
		" path=" + path +
		" status=" + itoa(status) +
		" duration=" + duration.String() +
		" ip=" + clientIP
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
