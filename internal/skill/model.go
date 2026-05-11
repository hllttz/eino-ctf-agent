package skill

// Skill 技能定义，对应data/skills/目录下的Markdown技能文件。
type Skill struct {
	Name        string   `json:"name" yaml:"name"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	Triggers    []string `json:"triggers" yaml:"triggers"`
	Enabled     bool     `json:"enabled" yaml:"enabled"`
	Priority    int      `json:"priority" yaml:"priority"`
	MaxTokens   int      `json:"max_tokens" yaml:"max_tokens"`
	Body        string   `json:"body,omitempty" yaml:"-"`
	Path        string   `json:"-" yaml:"-"`
}

func (s Skill) WithoutBody() Skill {
	s.Body = ""
	s.Path = ""
	return s
}
