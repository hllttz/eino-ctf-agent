package tool

import (
	"fmt"
	"os"
	"path/filepath"

	"eino_ctf_agent/internal/pkg/security"
)

const (
	defaultOutputLimit   = 100 * 1024 // 100KB
	defaultCmdTimeout    = 5          // seconds
	maxCmdTimeout        = 20         // seconds
	defaultPythonTimeout = 5          // seconds
	maxPythonTimeout     = 20         // seconds
	defaultFileMaxBytes  = 1 << 20    // 1MB
)

var workDir string

// SetWorkDir 设置 CTF 工具的工作目录，所有文件路径操作限制在此目录内。
func SetWorkDir(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve work dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	workDir = abs
	return nil
}

// resolvePath 将相对路径安全解析到工作目录内，防止 ../ 路径穿越和绝对路径访问。
func resolvePath(path string) (string, error) {
	if workDir == "" {
		return "", fmt.Errorf("work dir not configured")
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute path not allowed: %s", path)
	}
	return security.SafeJoin(workDir, path)
}

// truncateOutput 按字节截断输出，返回截断后的字符串和是否截断。
func truncateOutput(s string, maxBytes int) (string, bool) {
	if len(s) <= maxBytes {
		return s, false
	}
	return s[:maxBytes], true
}

// agentTmpDir 返回 .agent_tmp 子目录路径，不存在则创建。
func agentTmpDir() (string, error) {
	if workDir == "" {
		return "", fmt.Errorf("work dir not configured")
	}
	tmpDir := filepath.Join(workDir, ".agent_tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("create agent tmp dir: %w", err)
	}
	return tmpDir, nil
}
