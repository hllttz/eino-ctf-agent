package skill

import (
	"sort"
	"sync"
)

// Registry 技能注册表，管理已加载技能的增删查改，读写安全。
type Registry struct {
	loader *Loader
	mu     sync.RWMutex
	skills map[string]Skill
}

// NewRegistry 创建技能注册表并执行首次加载。
func NewRegistry(loader *Loader) (*Registry, error) {
	r := &Registry{loader: loader, skills: make(map[string]Skill)}
	if err := r.Reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// Reload 重新从文件系统加载所有技能文件，原子替换内存中的技能表。
func (r *Registry) Reload() error {
	loaded, err := r.loader.LoadAll()
	if err != nil {
		return err
	}
	next := make(map[string]Skill, len(loaded))
	for _, s := range loaded {
		next[s.Name] = s
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills = next
	return nil
}

// ListAll 返回所有已加载技能（不含 body），按优先级降序、同优先级按名称升序排列。
func (r *Registry) ListAll() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s.WithoutBody())
	}
	sortSkills(out)
	return out
}

// GetByName 按技能名称精确查询，返回完整 Skill（含 body）。
func (r *Registry) GetByName(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

func sortSkills(items []Skill) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			return items[i].Name < items[j].Name
		}
		return items[i].Priority > items[j].Priority
	})
}
