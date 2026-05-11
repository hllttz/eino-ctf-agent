package skill

import "strings"

// Router 技能路由器，根据用户查询匹配对应的技能。
type Router struct {
	registry *Registry
	max      int
}

func NewRouter(registry *Registry, max int) *Router {
	if max <= 0 {
		max = 3
	}
	return &Router{registry: registry, max: max}
}

func (r *Router) Match(query string) []Skill {
	query = strings.ToLower(query)
	matches := make([]Skill, 0)
	for _, s := range r.registry.ListAll() {
		if !s.Enabled {
			continue
		}
		for _, trigger := range s.Triggers {
			if trigger != "" && strings.Contains(query, strings.ToLower(trigger)) {
				full, ok := r.registry.GetByName(s.Name)
				if ok {
					matches = append(matches, full)
				}
				break
			}
		}
	}
	sortSkills(matches)
	if len(matches) > r.max {
		matches = matches[:r.max]
	}
	return matches
}
