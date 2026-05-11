package service

import (
	"eino_ctf_agent/internal/skill"
)

// SkillService 技能服务，封装技能注册表的查询和重载操作。
type SkillService struct {
	registry *skill.Registry
}

func NewSkillService(registry *skill.Registry) *SkillService {
	return &SkillService{registry: registry}
}

func (s *SkillService) List() []skill.Skill {
	return s.registry.ListAll()
}

func (s *SkillService) Get(name string) (skill.Skill, bool) {
	return s.registry.GetByName(name)
}

func (s *SkillService) Reload() error {
	return s.registry.Reload()
}
