package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"eino_ctf_agent/internal/model"
	"eino_ctf_agent/internal/pkg/sse"
	"eino_ctf_agent/internal/service"
)

// ChatHandler 聊天HTTP处理器，处理同步和流式聊天请求。
type ChatHandler struct {
	chatService *service.ChatService
}

func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

func (h *ChatHandler) Chat(c *gin.Context) {
	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	ctx := c.Request.Context()
	if traceID := c.Request.Header.Get("X-Request-ID"); traceID != "" {
		ctx = service.ContextWithTraceID(ctx, traceID)
	}

	resp, err := h.chatService.Chat(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error:   "chat_error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *ChatHandler) Stream(c *gin.Context) {
	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	ctx := c.Request.Context()
	if traceID := c.Request.Header.Get("X-Request-ID"); traceID != "" {
		ctx = service.ContextWithTraceID(ctx, traceID)
	}

	sse.SetHeaders(c.Writer)
	c.Status(http.StatusOK)
	writer := sse.NewWriter(c.Writer)

	stream, err := h.chatService.Stream(ctx, &req)
	if err != nil {
		_ = writer.Event("error", model.ErrorResponse{Error: "chat_error", Message: err.Error()})
		_ = writer.Event("done", gin.H{})
		return
	}
	defer stream.Reader.Close()

	if len(stream.Skills) > 0 {
		if err := writer.Event("skill_used", gin.H{"skills": stream.Skills}); err != nil {
			return
		}
	}

	for _, citation := range stream.Citations {
		if err := writer.Event("citation", citation); err != nil {
			return
		}
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		default:
		}

		chunk, err := stream.Reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = writer.Event("error", model.ErrorResponse{Error: "stream_error", Message: err.Error()})
			break
		}
		if chunk.Content == "" {
			continue
		}
		if err := writer.Event("message_delta", gin.H{"content": chunk.Content}); err != nil {
			return
		}
	}

	_ = writer.Event("done", gin.H{})
}
