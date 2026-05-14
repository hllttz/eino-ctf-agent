package handler

import (
	"github.com/gin-gonic/gin"

	apperrors "eino_ctf_agent/internal/errors"
	"eino_ctf_agent/internal/pkg/response"
	"eino_ctf_agent/internal/pkg/security"
	"eino_ctf_agent/internal/service"
)

// SkillHandler 技能HTTP处理器，处理技能列表、详情和重载请求。
type SkillHandler struct {
	skillService *service.SkillService
}

func NewSkillHandler(skillService *service.SkillService) *SkillHandler {
	return &SkillHandler{skillService: skillService}
}

func (h *SkillHandler) List(c *gin.Context) {
	response.OK(c, h.skillService.List())
}

func (h *SkillHandler) Get(c *gin.Context) {
	name := c.Param("name")
	if !security.ValidSkillName(name) {
		response.Error(c, apperrors.BadRequest(
			"invalid_skill_name",
			"skill name may only contain letters, numbers, underscores, and hyphens",
		))
		return
	}

	s, ok := h.skillService.Get(name)
	if !ok {
		response.Error(c, apperrors.NotFound(
			"skill_not_found",
			"skill not found",
		))
		return
	}
	response.OK(c, s)
}

func (h *SkillHandler) Reload(c *gin.Context) {
	if err := h.skillService.Reload(); err != nil {
		response.Error(c, apperrors.Internal(
			"reload_skills_failed",
			err.Error(),
		))
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}
