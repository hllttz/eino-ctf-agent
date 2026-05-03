package prompt

import (
	"strings"
	"testing"

	"eino_ctf_agent/internal/skill"
)

func TestBuildRAGSystemPromptIncludesSkills(t *testing.T) {
	p := BuildRAGSystemPrompt(nil, []skill.Skill{
		{
			Name:        "reverse-analysis",
			Title:       "Reverse",
			Description: "Reverse engineering workflow",
			Enabled:     true,
			Priority:    80,
			MaxTokens:   2000,
			Body:        "Find main and verify functions.",
		},
	}, 5)

	for _, want := range []string{
		"[Active Skills]",
		`name="reverse-analysis"`,
		"Find main and verify functions.",
		"当前知识库中没有找到直接依据",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt does not contain %q:\n%s", want, p)
		}
	}
}
