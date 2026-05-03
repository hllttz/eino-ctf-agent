package router

import (
	"eino_ctf_agent/internal/handler"

	"github.com/gin-gonic/gin"
)

// Setup 注册所有路由。
// 接收各 handler 实例作为参数，避免 router 直接依赖 service / config。
func Setup(engine *gin.Engine, chatHandler *handler.ChatHandler) {
	healthHandler := handler.NewHealthHandler()

	// 健康检查（不在 /api 组下，方便运维直接访问）
	engine.GET("/health", healthHandler.Health)

	// API 路由组
	api := engine.Group("/api")
	{
		// Phase 1: 非流式聊天
		api.POST("/chat", chatHandler.Chat)

		// Phase 2: 流式聊天（待实现）
		// api.POST("/chat/stream", chatHandler.Stream)
	}
}
