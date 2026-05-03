package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"eino_ctf_agent/internal/model"
	"eino_ctf_agent/internal/service"
)

type KnowledgeHandler struct {
	knowledgeService *service.KnowledgeService
}

func NewKnowledgeHandler(knowledgeService *service.KnowledgeService) *KnowledgeHandler {
	return &KnowledgeHandler{knowledgeService: knowledgeService}
}

func (h *KnowledgeHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "invalid_request",
			Message: "missing multipart file field: file",
		})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "invalid_file",
			Message: err.Error(),
		})
		return
	}
	defer src.Close()

	doc, err := h.knowledgeService.UploadMarkdown(c.Request.Context(), file.Filename, src)
	if err != nil {
		status := http.StatusInternalServerError
		if doc != nil {
			c.JSON(status, doc)
			return
		}
		c.JSON(status, model.ErrorResponse{
			Error:   "index_failed",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusAccepted, doc)
}

func (h *KnowledgeHandler) ListDocuments(c *gin.Context) {
	docs, err := h.knowledgeService.ListDocuments(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error:   "list_documents_failed",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, docs)
}

func (h *KnowledgeHandler) DeleteDocument(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "invalid_request",
			Message: "document id is required",
		})
		return
	}
	if err := h.knowledgeService.DeleteDocument(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error:   "delete_document_failed",
			Message: err.Error(),
		})
		return
	}
	c.Status(http.StatusNoContent)
}
