package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"eino_ctf_agent/internal/pkg/security"
)

type Loader struct {
	dir string
}

func NewLoader(dir string) *Loader {
	return &Loader{dir: dir}
}

func (l *Loader) LoadAll() ([]Skill, error) {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return nil, fmt.Errorf("create skills dir: %w", err)
	}

	skills := make([]Skill, 0)
	err := filepath.WalkDir(l.dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if strings.ToLower(filepath.Ext(name)) != ".md" && name != "SKILL.md" {
			return nil
		}
		if security.ForbiddenSensitiveFile(path) {
			return nil
		}
		loaded, err := l.Load(path)
		if err != nil {
			return fmt.Errorf("load skill %s: %w", path, err)
		}
		skills = append(skills, loaded)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return skills, nil
}

func (l *Loader) Load(path string) (Skill, error) {
	baseAbs, err := filepath.Abs(l.dir)
	if err != nil {
		return Skill{}, err
	}
	targetAbs, err := filepath.Abs(path)
	if err != nil {
		return Skill{}, err
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return Skill{}, err
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return Skill{}, fmt.Errorf("skill path escapes skills dir")
	}

	content, err := os.ReadFile(targetAbs)
	if err != nil {
		return Skill{}, err
	}
	s, err := parseSkillMarkdown(string(content))
	if err != nil {
		return Skill{}, err
	}
	if s.Name == "" {
		s.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if !security.ValidSkillName(s.Name) {
		return Skill{}, fmt.Errorf("invalid skill name %q", s.Name)
	}
	if s.Title == "" {
		s.Title = s.Name
	}
	if s.MaxTokens <= 0 {
		s.MaxTokens = 2000
	}
	s.Path = targetAbs
	return s, nil
}

func parseSkillMarkdown(content string) (Skill, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	s := Skill{Enabled: true, MaxTokens: 2000}
	if !strings.HasPrefix(content, "---\n") {
		s.Body = strings.TrimSpace(content)
		return s, nil
	}

	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return Skill{}, fmt.Errorf("missing closing front matter marker")
	}
	meta := rest[:end]
	body := rest[end+len("\n---\n"):]
	if err := yaml.Unmarshal([]byte(meta), &s); err != nil {
		return Skill{}, fmt.Errorf("parse front matter: %w", err)
	}
	s.Body = strings.TrimSpace(body)
	return s, nil
}
