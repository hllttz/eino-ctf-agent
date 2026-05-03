package security

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var skillNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func ValidSkillName(name string) bool {
	return skillNamePattern.MatchString(name)
}

func SafeJoin(baseDir, path string) (string, error) {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(filepath.Join(baseAbs, path))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes base directory")
	}
	return targetAbs, nil
}

func ForbiddenSensitiveFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	if name == ".env" || strings.Contains(name, "secret") || strings.Contains(name, "private") {
		return true
	}
	switch ext {
	case ".db", ".sqlite", ".sqlite3", ".key", ".pem", ".crt", ".p12":
		return true
	default:
		return false
	}
}
