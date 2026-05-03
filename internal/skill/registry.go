package skill

import (
	"fmt"
	"sort"
	"sync"
)

type Registry struct {
	loader *Loader
	mu     sync.RWMutex
	skills map[string]Skill
}

func NewRegistry(loader *Loader) (*Registry, error) {
	r := &Registry{loader: loader, skills: make(map[string]Skill)}
	if err := r.Reload(); err != nil {
		return nil, err
	}
	return r, nil
}

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

func (r *Registry) GetByName(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

func (r *Registry) MustGetByName(name string) (Skill, error) {
	s, ok := r.GetByName(name)
	if !ok {
		return Skill{}, fmt.Errorf("skill not found: %s", name)
	}
	return s, nil
}

func sortSkills(items []Skill) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			return items[i].Name < items[j].Name
		}
		return items[i].Priority > items[j].Priority
	})
}
