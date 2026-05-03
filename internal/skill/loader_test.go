package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoaderLoadAll(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: reverse-analysis
title: Reverse
description: Reverse engineering flow
triggers:
  - IDA
enabled: true
priority: 80
max_tokens: 1200
---

# Body
Use this skill.
`
	if err := os.WriteFile(filepath.Join(dir, "reverse-analysis.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	loaded, err := NewLoader(dir).LoadAll()
	if err != nil {
		t.Fatalf("LoadAll returned error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("len(loaded) = %d, want 1", len(loaded))
	}
	if loaded[0].Name != "reverse-analysis" || loaded[0].Priority != 80 {
		t.Fatalf("unexpected skill metadata: %+v", loaded[0])
	}
	if loaded[0].Body == "" {
		t.Fatal("skill body is empty")
	}
}
