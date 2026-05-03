package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"eino_ctf_agent/internal/model"
	"eino_ctf_agent/internal/pkg/security"
	"eino_ctf_agent/internal/service"
)

type SkillHandler struct {
	skillService *service.SkillService
}

func NewSkillHandler(skillService *service.SkillService) *SkillHandler {
	return &SkillHandler{skillService: skillService}
}

func (h *SkillHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, h.skillService.List())
}

func (h *SkillHandler) Get(c *gin.Context) {
	name := c.Param("name")
	if !security.ValidSkillName(name) {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "invalid_skill_name",
			Message: "skill name may only contain letters, numbers, underscores, and hyphens",
		})
		return
	}

	s, ok := h.skillService.Get(name)
	if !ok {
		c.JSON(http.StatusNotFound, model.ErrorResponse{
			Error:   "skill_not_found",
			Message: "skill not found",
		})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *SkillHandler) Reload(c *gin.Context) {
	if err := h.skillService.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error:   "reload_skills_failed",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
