package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"eino_ctf_agent/internal/model"
	"eino_ctf_agent/internal/service"
)

// ChatHandler 处理聊天相关的 HTTP 请求。
type ChatHandler struct {
	chatService *service.ChatService
}

// NewChatHandler 创建 ChatHandler 实例。
func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
	}
}

// Chat 处理非流式聊天请求。
// POST /api/chat
func (h *ChatHandler) Chat(c *gin.Context) {
	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	reply, err := h.chatService.Chat(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error:   "llm_error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, model.ChatResponse{
		Reply: reply,
	})
}
