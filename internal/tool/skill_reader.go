package tool

import (
	"fmt"

	"eino_ctf_agent/internal/pkg/security"
	"eino_ctf_agent/internal/skill"
)

type SkillReader struct {
	registry *skill.Registry
}

func NewSkillReader(registry *skill.Registry) *SkillReader {
	return &SkillReader{registry: registry}
}

func (r *SkillReader) Read(skillName string) (string, error) {
	if !security.ValidSkillName(skillName) {
		return "", fmt.Errorf("invalid skill name")
	}
	s, ok := r.registry.GetByName(skillName)
	if !ok {
		return "", fmt.Errorf("skill not found: %s", skillName)
	}
	if !s.Enabled {
		return "", fmt.Errorf("skill is disabled: %s", skillName)
	}
	return s.Body, nil
}
