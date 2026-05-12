package tool

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"eino_ctf_agent/internal/pkg/security"
)

// pythonRunnerEnv 是 python_runner 执行脚本时的最小环境变量。
// 不继承当前进程环境，不包含 API key、代理变量、用户环境变量。
// 这不是安全沙箱；脚本运行在进程级别，不具备 OS 级网络隔离。
var pythonRunnerEnv = []string{
	"PATH=/usr/bin:/bin",
	"PYTHONNOUSERSITE=1",
}

// PythonRunnerInput Python 脚本执行工具的输入参数。
type PythonRunnerInput struct {
	Script  string `json:"script" jsonschema:"description=Python script content to execute"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"description=timeout in seconds, defaults to 5, max 20"`
}

// PythonRunnerOutput Python 脚本执行工具的输出结果。
type PythonRunnerOutput struct {
	Stdout     string `json:"stdout" jsonschema:"description=standard output, truncated if exceeds limit"`
	Stderr     string `json:"stderr" jsonschema:"description=standard error output, truncated if exceeds limit"`
	ExitCode   int    `json:"exit_code" jsonschema:"description=script exit code, -1 if timeout or execution error"`
	Truncated  bool   `json:"truncated" jsonschema:"description=whether stdout or stderr was truncated"`
	ScriptPath string `json:"script_path" jsonschema:"description=relative path to the saved script file for reproducibility"`
	Error      string `json:"error,omitempty" jsonschema:"description=execution error message if any"`
}

// NewPythonRunnerTool 创建 Python 脚本执行工具。
// 脚本保存在 .agent_tmp/ 方便复现。使用最小环境变量，不主动提供网络能力。
// 通过路径限制、timeout、输出截断降低风险。这不是安全沙箱。
func NewPythonRunnerTool() (einotool.InvokableTool, error) {
	return utils.InferTool[PythonRunnerInput, PythonRunnerOutput](
		"python_runner",
		"Execute a Python script with minimal environment. "+
			"The script is saved to .agent_tmp/ for reproducibility and executed with python3. "+
			"Uses restricted environment variables (PATH only, no user site-packages). "+
			"Has timeout (default 5s, max 20s) and output length limits. "+
			"This is not a security sandbox; no OS-level network isolation is enforced. "+
			"Use this for CTF calculations, data decoding, format conversion, or any analysis logic "+
			"that cannot be expressed through command_executor alone.",
		func(ctx context.Context, input PythonRunnerInput) (PythonRunnerOutput, error) {
			if strings.TrimSpace(input.Script) == "" {
				return PythonRunnerOutput{
					ExitCode: -1,
					Error:    "script is empty",
				}, nil
			}

			timeout := input.Timeout
			if timeout <= 0 {
				timeout = defaultPythonTimeout
			}
			if timeout > maxPythonTimeout {
				return PythonRunnerOutput{
					ExitCode: -1,
					Error:    fmt.Sprintf("timeout exceeds maximum allowed: %d > %d", timeout, maxPythonTimeout),
				}, nil
			}

			if _, err := agentTmpDir(); err != nil {
				return PythonRunnerOutput{
					ExitCode: -1,
					Error:    err.Error(),
				}, nil
			}

			scriptName := fmt.Sprintf("script_%d.py", time.Now().UnixNano())

			safePath, err := security.SafeJoin(workDir, filepath.Join(".agent_tmp", scriptName))
			if err != nil {
				return PythonRunnerOutput{
					ExitCode: -1,
					Error:    fmt.Sprintf("script path rejected: %s", err.Error()),
				}, nil
			}

			if err := os.WriteFile(safePath, []byte(input.Script), 0644); err != nil {
				return PythonRunnerOutput{
					ExitCode: -1,
					Error:    fmt.Sprintf("failed to write script: %s", err.Error()),
				}, nil
			}

			execCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
			defer cancel()

			c := exec.CommandContext(execCtx, "python3", safePath)
			c.Dir = workDir
			c.Env = pythonRunnerEnv

			var stdoutBuf, stderrBuf bytes.Buffer
			c.Stdout = &stdoutBuf
			c.Stderr = &stderrBuf
			runErr := c.Run()

			exitCode := 0
			truncated := false

			if runErr != nil {
				if execCtx.Err() == context.DeadlineExceeded {
					return PythonRunnerOutput{
						ExitCode:   -1,
						Error:      fmt.Sprintf("script timed out after %ds", timeout),
						ScriptPath: filepath.Join(".agent_tmp", scriptName),
					}, nil
				}
				if exitErr, ok := runErr.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					return PythonRunnerOutput{
						ExitCode:   -1,
						Error:      fmt.Sprintf("script execution failed: %s", runErr.Error()),
						ScriptPath: filepath.Join(".agent_tmp", scriptName),
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

			return PythonRunnerOutput{
				Stdout:     strings.TrimRight(limitedStdout, "\n"),
				Stderr:     strings.TrimRight(limitedStderr, "\n"),
				ExitCode:   exitCode,
				Truncated:  truncated,
				ScriptPath: filepath.Join(".agent_tmp", scriptName),
			}, nil
		},
	)
}
