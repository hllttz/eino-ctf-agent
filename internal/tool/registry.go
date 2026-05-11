package tool

import (
	"fmt"
	"sync"

	einotool "github.com/cloudwego/eino/components/tool"
)

// Registry 工具注册表，管理所有可用的 Eino Tool 实例，并发安全。
// 命名上与 skill.Registry 区分——skill.Registry 管理 Skill 数据，Tool Registry 管理可执行的 Tool。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]einotool.BaseTool
}

// NewRegistry 创建空的工具注册表。
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]einotool.BaseTool)}
}

// Register 注册一个工具。若名称已存在则覆盖。
func (r *Registry) Register(name string, t einotool.BaseTool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = t
}

// All 返回所有已注册工具的切片，顺序不保证。
func (r *Registry) All() []einotool.BaseTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]einotool.BaseTool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// Get 按名称获取已注册工具。
func (r *Registry) Get(name string) (einotool.BaseTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return t, nil
}
