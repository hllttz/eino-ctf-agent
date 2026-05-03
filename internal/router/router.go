package router

import (
	"github.com/gin-gonic/gin"

	"eino_ctf_agent/internal/handler"
)

func Setup(
	engine *gin.Engine,
	chatHandler *handler.ChatHandler,
	knowledgeHandler *handler.KnowledgeHandler,
	skillHandler *handler.SkillHandler,
) {
	healthHandler := handler.NewHealthHandler()

	engine.GET("/health", healthHandler.Health)

	api := engine.Group("/api")
	{
		api.POST("/chat", chatHandler.Chat)
		api.POST("/chat/stream", chatHandler.Stream)

		knowledge := api.Group("/knowledge")
		{
			knowledge.POST("/upload", knowledgeHandler.Upload)
			knowledge.GET("/documents", knowledgeHandler.ListDocuments)
			knowledge.DELETE("/documents/:id", knowledgeHandler.DeleteDocument)
		}

		api.GET("/skills", skillHandler.List)
		api.GET("/skills/:name", skillHandler.Get)
		api.POST("/skills/reload", skillHandler.Reload)
	}
}
