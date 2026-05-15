package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

var allowedCommands = map[string]bool{
	// 文件分析
	"file": true, "strings": true, "xxd": true, "hexdump": true, "od": true,
	// 二进制分析
	"readelf": true, "objdump": true, "nm": true, "size": true, "ldd": true,
	// 压缩包操作（仅允许列表模式，见 validateArchiveArgs）
	"unzip": true, "tar": true, "zipinfo": true,
	// 文件浏览（cat 已移除，读取文件内容应使用 file_reader）
	"head": true, "tail": true, "wc": true, "grep": true,
	"find": true, "ls": true, "stat": true,
	// 文本处理
	"awk": true, "sed": true, "sort": true, "uniq": true, "cut": true,
	"tr": true, "diff": true, "echo": true,
}

// ArgList 兼容 []string 和 string 两种 JSON 类型。
// 模型可能在单参数时传入 "args": "1.exe" 而非 "args": ["1.exe"]。
type ArgList []string

// UnmarshalJSON 支持 args 为 JSON 数组或单个字符串。
func (a *ArgList) UnmarshalJSON(data []byte) error {
	// 先尝试 []string
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*a = arr
		return nil
	}
	// 兼容单个 string，自动转为单元素切片
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*a = []string{s}
		return nil
	}
	return fmt.Errorf("args must be a JSON string or JSON array of strings, got: %s", string(data))
}

// CommandExecutorInput 命令执行工具的输入参数。
type CommandExecutorInput struct {
	Command string  `json:"command" jsonschema:"description=command name, must be in the allowlist"`
	Args    ArgList `json:"args" jsonschema:"description=command arguments as JSON array of strings (e.g. [\"file1.exe\"]), NOT a single string"`
	Timeout int     `json:"timeout,omitempty" jsonschema:"description=timeout in seconds, defaults to 5, max 20"`
}

// CommandExecutorOutput 命令执行工具的输出结果。
type CommandExecutorOutput struct {
	Stdout    string `json:"stdout" jsonschema:"description=standard output, truncated if exceeds limit"`
	Stderr    string `json:"stderr" jsonschema:"description=standard error output, truncated if exceeds limit"`
	ExitCode  int    `json:"exit_code" jsonschema:"description=command exit code, -1 if timeout or execution error"`
	Truncated bool   `json:"truncated" jsonschema:"description=whether stdout or stderr was truncated"`
	Error     string `json:"error,omitempty" jsonschema:"description=execution error message if any"`
}

// NewCommandExecutorTool 创建命令执行工具。
// 通过 allowlist、路径参数校验、tar/unzip 列表模式限制、timeout 和输出截断降低风险。
// 这不是安全沙箱；命令在进程级别运行，不具备 OS 级隔离。
func NewCommandExecutorTool() (einotool.InvokableTool, error) {
	return utils.InferTool[CommandExecutorInput, CommandExecutorOutput](
		"command_executor",
		"Execute safe, read-only analysis commands. "+
			"Allowed commands: file, strings, xxd, hexdump, readelf, objdump, nm, size, ldd, "+
			"unzip (list only), tar (list only), zipinfo, head, tail, wc, grep, find, ls, stat, "+
			"awk, sed, sort, uniq, cut, tr, diff, od, echo. "+
			"All file path arguments must be within the working directory; absolute paths and "+
			"../ traversal are rejected. tar is restricted to -t/--list mode, unzip to -l mode. "+
			"IMPORTANT: args must be a JSON ARRAY of strings, e.g. [\"1.exe\"], [\"-a\", \"file.o\"]. "+
			"Never pass args as a single string like \"1.exe\". "+
			"Has timeout (default 5s, max 20s) and output length limits. "+
			"Use this to analyze binary files, search for patterns, or explore file structure. "+
			"Example: {\"command\":\"strings\",\"args\":[\"binary.exe\"],\"timeout\":10}",
		func(ctx context.Context, input CommandExecutorInput) (CommandExecutorOutput, error) {
			cmd := input.Command
			if !allowedCommands[cmd] {
				return CommandExecutorOutput{
					ExitCode: -1,
					Error:    fmt.Sprintf("command not allowed: %s", cmd),
				}, nil
			}

			timeout := input.Timeout
			if timeout <= 0 {
				timeout = defaultCmdTimeout
			}
			if timeout > maxCmdTimeout {
				return CommandExecutorOutput{
					ExitCode: -1,
					Error:    fmt.Sprintf("timeout exceeds maximum allowed: %d > %d", timeout, maxCmdTimeout),
				}, nil
			}

			// 校验所有参数中的文件路径，禁止绝对路径和 ../ 穿越
			validatedArgs, err := validatePathArgs([]string(input.Args))
			if err != nil {
				return CommandExecutorOutput{
					ExitCode: -1,
					Error:    fmt.Sprintf("path argument rejected: %s", err.Error()),
				}, nil
			}

			// tar/unzip 仅允许列表模式
			if err := validateArchiveArgs(cmd, validatedArgs); err != nil {
				return CommandExecutorOutput{
					ExitCode: -1,
					Error:    err.Error(),
				}, nil
			}

			execCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
			defer cancel()

			c := exec.CommandContext(execCtx, cmd, validatedArgs...)
			if workDir != "" {
				c.Dir = workDir
			}

			var stdoutBuf, stderrBuf bytes.Buffer
			c.Stdout = &stdoutBuf
			c.Stderr = &stderrBuf
			runErr := c.Run()

			exitCode := 0
			truncated := false

			if runErr != nil {
				if execCtx.Err() == context.DeadlineExceeded {
					return CommandExecutorOutput{
						ExitCode: -1,
						Error:    fmt.Sprintf("command timed out after %ds", timeout),
					}, nil
				}
				if exitErr, ok := runErr.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					return CommandExecutorOutput{
						ExitCode: -1,
						Error:    fmt.Sprintf("command execution failed: %s", runErr.Error()),
					}, nil
				}
			}

			limitedStdout, outTrunc := truncateOutput(stdoutBuf.String(), defaultOutputLimit)
			limitedStderr, errTrunc := truncateOutput(stderrBuf.String(), defaultOutputLimit/2)
			truncated = outTrunc || errTrunc

			if truncated {
				if outTrunc {
					limitedStdout += "\n...[stdout truncated]"
				}
				if errTrunc {
					limitedStderr += "\n...[stderr truncated]"
				}
			}

			return CommandExecutorOutput{
				Stdout:    strings.TrimRight(limitedStdout, "\n"),
				Stderr:    strings.TrimRight(limitedStderr, "\n"),
				ExitCode:  exitCode,
				Truncated: truncated,
			}, nil
		},
	)
}

// validatePathArgs 校验命令参数中的文件路径。
// 禁止绝对路径和 ../ 穿越；包含 / 的路径参数会被解析到工作目录内。
func validatePathArgs(args []string) ([]string, error) {
	resolved := make([]string, len(args))
	for i, arg := range args {
		// 跳过以 - 开头的 flag 参数
		if strings.HasPrefix(arg, "-") {
			resolved[i] = arg
			continue
		}
		// 禁止绝对路径
		if filepath.IsAbs(arg) {
			return nil, fmt.Errorf("absolute path not allowed: %s", arg)
		}
		// 禁止 ../ 路径穿越
		if strings.Contains(arg, "..") {
			return nil, fmt.Errorf("path traversal not allowed: %s", arg)
		}
		// 包含 / 的相对路径：解析到工作目录内
		if strings.Contains(arg, "/") {
			rp, err := resolvePath(arg)
			if err != nil {
				return nil, fmt.Errorf("invalid path %q: %w", arg, err)
			}
			resolved[i] = rp
		} else {
			resolved[i] = arg
		}
	}
	return resolved, nil
}

// validateArchiveArgs 确保 tar 仅允许列表模式（-t/--list），unzip 仅允许列表模式（-l）。
func validateArchiveArgs(cmd string, args []string) error {
	switch cmd {
	case "tar":
		if !hasListFlag(args) {
			return fmt.Errorf("tar is restricted to list mode: use -t or --list (e.g. tar -tf archive.tar)")
		}
	case "unzip":
		if !hasUnzipListFlag(args) {
			return fmt.Errorf("unzip is restricted to list mode: use -l (e.g. unzip -l archive.zip)")
		}
	}
	return nil
}

func hasListFlag(args []string) bool {
	for _, a := range args {
		if a == "--list" {
			return true
		}
		// -t 可与其他 flag 组合：-t, -tf, -tvf
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "t") {
			return true
		}
	}
	return false
}

func hasUnzipListFlag(args []string) bool {
	for _, a := range args {
		if a == "-l" {
			return true
		}
	}
	return false
}
