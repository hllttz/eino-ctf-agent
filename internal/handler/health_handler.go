package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler 处理健康检查请求。
type HealthHandler struct{}

// NewHealthHandler 创建 HealthHandler 实例。
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health 返回服务健康状态。
// GET /health → {"status": "ok"}
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
