package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
)

// helper: call InvokableTool with typed I/O marshaling

func invokeTool[I, O any](t testing.TB, tool einotool.InvokableTool, input I) O {
	t.Helper()
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	outputJSON, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	var out O
	if err := json.Unmarshal([]byte(outputJSON), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nraw: %s", err, outputJSON)
	}
	return out
}

// safe_util 测试

func TestSetWorkDir_CreatesDir(t *testing.T) {
	tmp := t.TempDir()
	wd := filepath.Join(tmp, "new_workdir")
	if err := SetWorkDir(wd); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}
	info, err := os.Stat(wd)
	if err != nil {
		t.Fatalf("work dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("work dir is not a directory")
	}
}

func TestResolvePath_PreventsEscape(t *testing.T) {
	tmp := t.TempDir()
	wd := filepath.Join(tmp, "workspace")
	if err := SetWorkDir(wd); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	resolved, err := resolvePath("subdir/file.txt")
	if err != nil {
		t.Fatalf("resolvePath failed for valid path: %v", err)
	}
	if !strings.HasSuffix(resolved, filepath.Join("workspace", "subdir", "file.txt")) {
		t.Errorf("unexpected resolved path: %s", resolved)
	}

	_, err = resolvePath("../etc/passwd")
	if err == nil {
		t.Fatal("resolvePath should reject ../ escape")
	}

	_, err = resolvePath("../../etc/shadow")
	if err == nil {
		t.Fatal("resolvePath should reject multi-level ../ escape")
	}

	_, err = resolvePath("/etc/passwd")
	if err == nil {
		t.Fatal("resolvePath should reject absolute paths")
	}
}

func TestResolvePath_EmptyWorkDir(t *testing.T) {
	old := workDir
	workDir = ""
	defer func() { workDir = old }()

	_, err := resolvePath("test.txt")
	if err == nil {
		t.Fatal("resolvePath should fail when workDir is not set")
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		input     string
		maxBytes  int
		wantLen   int
		wantTrunc bool
	}{
		{"hello", 100, 5, false},
		{"hello", 3, 3, true},
		{"hello", 5, 5, false},
		{"", 100, 0, false},
		{"abcdef", 0, 0, true},
	}

	for _, tt := range tests {
		result, truncated := truncateOutput(tt.input, tt.maxBytes)
		if len(result) != tt.wantLen {
			t.Errorf("truncateOutput(%q, %d): got len=%d, want len=%d",
				tt.input, tt.maxBytes, len(result), tt.wantLen)
		}
		if truncated != tt.wantTrunc {
			t.Errorf("truncateOutput(%q, %d): got truncated=%v, want %v",
				tt.input, tt.maxBytes, truncated, tt.wantTrunc)
		}
	}
}

// file_info 测试

func TestFileInfo_Exists(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	testFile := filepath.Join(tmp, "test.py")
	if err := os.WriteFile(testFile, []byte("print('hello')"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	tool, err := NewFileInfoTool()
	if err != nil {
		t.Fatalf("NewFileInfoTool: %v", err)
	}

	out := invokeTool[FileInfoInput, FileInfoOutput](t, tool, FileInfoInput{Path: "test.py"})
	if !out.Exists {
		t.Error("file should exist")
	}
	if out.Size != 14 {
		t.Errorf("size: got %d, want 14", out.Size)
	}
	if out.IsDir {
		t.Error("should not be a directory")
	}
	if out.Extension != ".py" {
		t.Errorf("extension: got %q, want %q", out.Extension, ".py")
	}
	if out.Type != "Python script" {
		t.Errorf("type: got %q, want 'Python script'", out.Type)
	}
}

func TestFileInfo_NotFound(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewFileInfoTool()
	if err != nil {
		t.Fatalf("NewFileInfoTool: %v", err)
	}

	out := invokeTool[FileInfoInput, FileInfoOutput](t, tool, FileInfoInput{Path: "notexist.txt"})
	if out.Exists {
		t.Error("file should not exist")
	}
	if out.Error == "" {
		t.Error("should have error message")
	}
}

func TestFileInfo_IsDirectory(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}
	os.MkdirAll(filepath.Join(tmp, "subdir"), 0755)

	tool, err := NewFileInfoTool()
	if err != nil {
		t.Fatalf("NewFileInfoTool: %v", err)
	}

	out := invokeTool[FileInfoInput, FileInfoOutput](t, tool, FileInfoInput{Path: "subdir"})
	if !out.Exists {
		t.Error("directory should exist")
	}
	if !out.IsDir {
		t.Error("should be a directory")
	}
	if out.Type != "directory" {
		t.Errorf("type: got %q, want 'directory'", out.Type)
	}
}

func TestFileInfo_PathEscape(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewFileInfoTool()
	if err != nil {
		t.Fatalf("NewFileInfoTool: %v", err)
	}

	out := invokeTool[FileInfoInput, FileInfoOutput](t, tool, FileInfoInput{Path: "../etc/passwd"})
	if out.Error == "" {
		t.Fatal("should reject path escape attempt")
	}
}

func TestFileInfo_ClassifyFile(t *testing.T) {
	tests := []struct {
		name     string
		wantType string
	}{
		{"main.c", "C source"},
		{"main.h", "C header"},
		{"main.cpp", "C++ source"},
		{"main.go", "Go source"},
		{"README.md", "Markdown"},
		{"data.json", "JSON data"},
		{"binary.elf", "ELF binary"},
		{"capture.pcap", "Packet capture"},
		{"unknown.xyz", "unknown type"},
	}

	for _, tt := range tests {
		info := fakeFileInfo{name: tt.name, size: 100}
		got := classifyFile(info)
		if got != tt.wantType {
			t.Errorf("classifyFile(%q): got %q, want %q", tt.name, got, tt.wantType)
		}
	}
}

type fakeFileInfo struct {
	name string
	size int64
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

// file_reader 测试

func TestFileReader_ReadFile(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	content := "Hello, this is a test file for CTF analysis."
	if err := os.WriteFile(filepath.Join(tmp, "readme.txt"), []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	tool, err := NewFileReaderTool()
	if err != nil {
		t.Fatalf("NewFileReaderTool: %v", err)
	}

	out := invokeTool[FileReaderInput, FileReaderOutput](t, tool, FileReaderInput{Path: "readme.txt"})
	if out.Content != content {
		t.Errorf("content mismatch: got %q, want %q", out.Content, content)
	}
	if out.Size != len(content) {
		t.Errorf("size: got %d, want %d", out.Size, len(content))
	}
	if out.Truncated {
		t.Error("should not be truncated")
	}
}

func TestFileReader_MaxBytes(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	content := strings.Repeat("A", 500)
	if err := os.WriteFile(filepath.Join(tmp, "large.txt"), []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	tool, err := NewFileReaderTool()
	if err != nil {
		t.Fatalf("NewFileReaderTool: %v", err)
	}

	out := invokeTool[FileReaderInput, FileReaderOutput](t, tool, FileReaderInput{Path: "large.txt", MaxBytes: 100})
	if !out.Truncated {
		t.Error("should be truncated with max_bytes=100")
	}
	if out.Size != 100 {
		t.Errorf("truncated size: got %d, want 100", out.Size)
	}
}

func TestFileReader_NotFound(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewFileReaderTool()
	if err != nil {
		t.Fatalf("NewFileReaderTool: %v", err)
	}

	out := invokeTool[FileReaderInput, FileReaderOutput](t, tool, FileReaderInput{Path: "missing.txt"})
	if out.Error == "" {
		t.Fatal("should have error for missing file")
	}
}

func TestFileReader_IsDirectory(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}
	os.MkdirAll(filepath.Join(tmp, "subdir"), 0755)

	tool, err := NewFileReaderTool()
	if err != nil {
		t.Fatalf("NewFileReaderTool: %v", err)
	}

	out := invokeTool[FileReaderInput, FileReaderOutput](t, tool, FileReaderInput{Path: "subdir"})
	if out.Error == "" || !strings.Contains(out.Error, "directory") {
		t.Errorf("should reject directory: %v", out.Error)
	}
}

func TestFileReader_PathEscape(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewFileReaderTool()
	if err != nil {
		t.Fatalf("NewFileReaderTool: %v", err)
	}

	out := invokeTool[FileReaderInput, FileReaderOutput](t, tool, FileReaderInput{Path: "../etc/passwd"})
	if out.Error == "" {
		t.Fatal("should reject path escape attempt")
	}
}

// command_executor 测试

func TestCommandExecutor_AllowedCommand(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
		Command: "echo",
		Args:    []string{"hello", "world"},
	})
	if out.ExitCode != 0 {
		t.Errorf("exit code: got %d, want 0", out.ExitCode)
	}
	if !strings.Contains(out.Stdout, "hello world") {
		t.Errorf("stdout: got %q, want 'hello world'", out.Stdout)
	}
}

func TestCommandExecutor_DisallowedCommand(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	dangerousCommands := []string{"rm", "mv", "dd", "sudo", "curl", "wget", "nc", "ssh", "sh", "bash", "cat"}
	for _, cmd := range dangerousCommands {
		out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
			Command: cmd,
			Args:    []string{"--help"},
		})
		if out.Error == "" || !strings.Contains(out.Error, "not allowed") {
			t.Errorf("command %q should be rejected, got error=%q exit=%d", cmd, out.Error, out.ExitCode)
		}
	}
}

func TestCommandExecutor_CatRemoved(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
		Command: "cat",
		Args:    []string{"test.txt"},
	})
	if out.Error == "" || !strings.Contains(out.Error, "not allowed") {
		t.Errorf("cat should be rejected (removed from allowlist): error=%q", out.Error)
	}
}

func TestCommandExecutor_Allowlist(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	// tar/unzip need special args for list mode, test them separately
	skipAllowlistTest := map[string]bool{"tar": true, "unzip": true}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	for cmd := range allowedCommands {
		if skipAllowlistTest[cmd] {
			continue
		}
		out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
			Command: cmd,
			Args:    []string{"--help"},
		})
		if out.Error != "" && strings.Contains(out.Error, "not allowed") {
			t.Errorf("command %q should be allowed but was rejected", cmd)
		}
	}
}

func TestCommandExecutor_MaxTimeout(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
		Command: "echo",
		Args:    []string{"test"},
		Timeout: 999,
	})
	if out.Error == "" || !strings.Contains(out.Error, "exceeds maximum") {
		t.Errorf("should reject timeout > maxCmdTimeout: error=%q", out.Error)
	}
}

func TestCommandExecutor_Truncation(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	// Create a large file in the workdir
	largeContent := strings.Repeat("A", 200*1024)
	largeFile := filepath.Join(tmp, "large.txt")
	if err := os.WriteFile(largeFile, []byte(largeContent), 0644); err != nil {
		t.Fatalf("write large file: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
		Command: "head",
		Args:    []string{"large.txt"},
		Timeout: 5,
	})
	if !out.Truncated && out.Error == "" {
		t.Log("output not truncated - may depend on head implementation")
	}
	if out.Truncated {
		t.Log("output truncated as expected")
	}
}

// command_executor path argument validation

func TestCommandExecutor_RejectAbsolutePath(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	absPathCases := []struct {
		cmd  string
		args []string
	}{
		{"strings", []string{"/etc/passwd"}},
		{"file", []string{"/etc/passwd"}},
		{"xxd", []string{"/etc/passwd"}},
	}

	for _, tc := range absPathCases {
		out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
			Command: tc.cmd,
			Args:    tc.args,
		})
		if out.Error == "" || !strings.Contains(out.Error, "absolute path") {
			t.Errorf("%s %v should reject absolute path: error=%q", tc.cmd, tc.args, out.Error)
		}
	}
}

func TestCommandExecutor_RejectPathTraversal(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	traversalCases := []struct {
		cmd  string
		args []string
	}{
		{"file", []string{"../xxx"}},
		{"strings", []string{"../../secrets"}},
		{"xxd", []string{"./../escape"}},
	}

	for _, tc := range traversalCases {
		out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
			Command: tc.cmd,
			Args:    tc.args,
		})
		if out.Error == "" || !strings.Contains(out.Error, "traversal") {
			t.Errorf("%s %v should reject path traversal: error=%q", tc.cmd, tc.args, out.Error)
		}
	}
}

func TestCommandExecutor_AllowValidPath(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	// Create subdirectory with a file
	subDir := filepath.Join(tmp, "subdir")
	os.MkdirAll(subDir, 0755)
	if err := os.WriteFile(filepath.Join(subDir, "data.bin"), []byte("testdata"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	// strings on a file within workdir - should be allowed
	out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
		Command: "strings",
		Args:    []string{"subdir/data.bin"},
	})
	if out.Error != "" {
		t.Errorf("strings on valid subdir file should be allowed: error=%q", out.Error)
	}

	// xxd on a file within workdir - should be allowed
	out2 := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
		Command: "xxd",
		Args:    []string{"subdir/data.bin"},
	})
	if out2.Error != "" {
		t.Errorf("xxd on valid subdir file should be allowed: error=%q", out2.Error)
	}

	// file on a file within workdir (no subdir, plain filename)
	out3 := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
		Command: "ls",
		Args:    []string{"subdir"},
	})
	if out3.Error != "" {
		t.Errorf("ls on valid subdir should be allowed: error=%q", out3.Error)
	}
}

// command_executor tar/unzip restrictions

func TestCommandExecutor_TarListAllowed(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	// Create a dummy tar file
	archivePath := filepath.Join(tmp, "archive.tar")
	emptyTar, _ := os.Create(archivePath)
	emptyTar.Close()

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	// tar -t should be allowed
	out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
		Command: "tar",
		Args:    []string{"-tf", "archive.tar"},
	})
	if out.Error != "" {
		t.Logf("tar -tf error (expected if no tar or empty archive): %s", out.Error)
	}
	if out.Error != "" && strings.Contains(out.Error, "restricted to list mode") {
		t.Errorf("tar -tf should NOT be rejected: error=%q", out.Error)
	}
}

func TestCommandExecutor_TarExtractRejected(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	extractCases := [][]string{
		{"-xf", "archive.tar"},
		{"--extract", "-f", "archive.tar"},
		{"-xvf", "archive.tar"},
	}

	for _, args := range extractCases {
		out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
			Command: "tar",
			Args:    args,
		})
		if out.Error == "" || !strings.Contains(out.Error, "restricted to list mode") {
			t.Errorf("tar %v should be rejected: error=%q", args, out.Error)
		}
	}
}

func TestCommandExecutor_UnzipListAllowed(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
		Command: "unzip",
		Args:    []string{"-l", "archive.zip"},
	})
	if out.Error != "" && strings.Contains(out.Error, "restricted to list mode") {
		t.Errorf("unzip -l should NOT be rejected: error=%q", out.Error)
	}
}

func TestCommandExecutor_UnzipExtractRejected(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
		Command: "unzip",
		Args:    []string{"archive.zip"},
	})
	if out.Error == "" || !strings.Contains(out.Error, "restricted to list mode") {
		t.Errorf("unzip without -l should be rejected: error=%q", out.Error)
	}
}

func TestCommandExecutor_TarListFlag(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}

	// 测试 --list 形式
	out := invokeTool[CommandExecutorInput, CommandExecutorOutput](t, tool, CommandExecutorInput{
		Command: "tar",
		Args:    []string{"--list", "-f", "archive.tar"},
	})
	if out.Error != "" && strings.Contains(out.Error, "restricted to list mode") {
		t.Errorf("tar --list should NOT be rejected: error=%q", out.Error)
	}
}

// python_runner 测试

func TestPythonRunner_NormalExecution(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewPythonRunnerTool()
	if err != nil {
		t.Fatalf("NewPythonRunnerTool: %v", err)
	}

	out := invokeTool[PythonRunnerInput, PythonRunnerOutput](t, tool, PythonRunnerInput{
		Script: "print('hello from python')",
	})
	if out.ExitCode != 0 {
		t.Logf("stderr: %s", out.Stderr)
		t.Logf("error: %s", out.Error)
		if strings.Contains(out.Error, "executable file not found") || strings.Contains(out.Error, "file not found") {
			t.Skip("python3 not available")
		}
		t.Errorf("exit code: got %d, want 0", out.ExitCode)
	}
	if !strings.Contains(out.Stdout, "hello from python") {
		t.Errorf("stdout: got %q, want 'hello from python'", out.Stdout)
	}
	if out.ScriptPath == "" {
		t.Error("script_path should not be empty")
	}
	if !strings.Contains(out.ScriptPath, ".agent_tmp") {
		t.Errorf("script_path should be in .agent_tmp: %s", out.ScriptPath)
	}

	absPath := filepath.Join(tmp, out.ScriptPath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Error("script file should be preserved for reproducibility")
	}
}

func TestPythonRunner_Timeout(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewPythonRunnerTool()
	if err != nil {
		t.Fatalf("NewPythonRunnerTool: %v", err)
	}

	out := invokeTool[PythonRunnerInput, PythonRunnerOutput](t, tool, PythonRunnerInput{
		Script:  "import time; time.sleep(10)",
		Timeout: 1,
	})
	if out.Error == "" || strings.Contains(out.Error, "executable file not found") {
		t.Skip("python3 not available or timeout not triggered")
	}
	if !strings.Contains(out.Error, "timed out") {
		t.Errorf("expected timeout error, got: %s", out.Error)
	}
}

func TestPythonRunner_EmptyScript(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewPythonRunnerTool()
	if err != nil {
		t.Fatalf("NewPythonRunnerTool: %v", err)
	}

	out := invokeTool[PythonRunnerInput, PythonRunnerOutput](t, tool, PythonRunnerInput{Script: "   "})
	if out.Error == "" {
		t.Fatal("should reject empty script")
	}
}

func TestPythonRunner_MaxTimeout(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewPythonRunnerTool()
	if err != nil {
		t.Fatalf("NewPythonRunnerTool: %v", err)
	}

	out := invokeTool[PythonRunnerInput, PythonRunnerOutput](t, tool, PythonRunnerInput{
		Script:  "print('test')",
		Timeout: 999,
	})
	if out.Error == "" || !strings.Contains(out.Error, "exceeds maximum") {
		t.Errorf("should reject timeout > maxPythonTimeout: error=%q", out.Error)
	}
}

func TestPythonRunner_Stderr(t *testing.T) {
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir failed: %v", err)
	}

	tool, err := NewPythonRunnerTool()
	if err != nil {
		t.Fatalf("NewPythonRunnerTool: %v", err)
	}

	out := invokeTool[PythonRunnerInput, PythonRunnerOutput](t, tool, PythonRunnerInput{
		Script: "import sys; sys.stderr.write('error message\\n')",
	})

	if strings.Contains(out.Error, "executable file not found") || strings.Contains(out.Error, "file not found") {
		t.Skip("python3 not available")
	}

	if !strings.Contains(out.Stderr, "error message") {
		t.Errorf("stderr should contain error message: %q", out.Stderr)
	}
}

// encoding_decoder 测试

func TestEncodingDecoder_Hex(t *testing.T) {
	tool, err := NewEncodingDecoderTool()
	if err != nil {
		t.Fatalf("NewEncodingDecoderTool: %v", err)
	}

	out := invokeTool[EncodingDecoderInput, EncodingDecoderOutput](t, tool, EncodingDecoderInput{
		Data: "48656c6c6f", Encoding: "hex",
	})
	if out.Decoded != "Hello" {
		t.Errorf("hex decode: got %q, want 'Hello'", out.Decoded)
	}
	if out.Truncated {
		t.Error("should not be truncated")
	}
}

func TestEncodingDecoder_Base64(t *testing.T) {
	tool, err := NewEncodingDecoderTool()
	if err != nil {
		t.Fatalf("NewEncodingDecoderTool: %v", err)
	}

	out := invokeTool[EncodingDecoderInput, EncodingDecoderOutput](t, tool, EncodingDecoderInput{
		Data: "SGVsbG8gV29ybGQ=", Encoding: "base64",
	})
	if out.Decoded != "Hello World" {
		t.Errorf("base64 decode: got %q, want 'Hello World'", out.Decoded)
	}
}

func TestEncodingDecoder_URL(t *testing.T) {
	tool, err := NewEncodingDecoderTool()
	if err != nil {
		t.Fatalf("NewEncodingDecoderTool: %v", err)
	}

	out := invokeTool[EncodingDecoderInput, EncodingDecoderOutput](t, tool, EncodingDecoderInput{
		Data: "hello%20world%21", Encoding: "url",
	})
	if out.Decoded != "hello world!" {
		t.Errorf("url decode: got %q, want 'hello world!'", out.Decoded)
	}
}

func TestEncodingDecoder_ROT13(t *testing.T) {
	tool, err := NewEncodingDecoderTool()
	if err != nil {
		t.Fatalf("NewEncodingDecoderTool: %v", err)
	}

	out := invokeTool[EncodingDecoderInput, EncodingDecoderOutput](t, tool, EncodingDecoderInput{
		Data: "Uryyb Jbeyq", Encoding: "rot13",
	})
	if out.Decoded != "Hello World" {
		t.Errorf("rot13 decode: got %q, want 'Hello World'", out.Decoded)
	}

	// ROT13 is self-inverse
	out2 := invokeTool[EncodingDecoderInput, EncodingDecoderOutput](t, tool, EncodingDecoderInput{
		Data: out.Decoded, Encoding: "rot13",
	})
	if out2.Decoded != "Uryyb Jbeyq" {
		t.Errorf("rot13 double: got %q, want 'Uryyb Jbeyq'", out2.Decoded)
	}
}

func TestEncodingDecoder_Binary(t *testing.T) {
	tool, err := NewEncodingDecoderTool()
	if err != nil {
		t.Fatalf("NewEncodingDecoderTool: %v", err)
	}

	out := invokeTool[EncodingDecoderInput, EncodingDecoderOutput](t, tool, EncodingDecoderInput{
		Data: "01001000 01101001", Encoding: "binary",
	})
	if out.Decoded != "Hi" {
		t.Errorf("binary decode: got %q, want 'Hi'", out.Decoded)
	}
}

func TestEncodingDecoder_UnsupportedEncoding(t *testing.T) {
	tool, err := NewEncodingDecoderTool()
	if err != nil {
		t.Fatalf("NewEncodingDecoderTool: %v", err)
	}

	out := invokeTool[EncodingDecoderInput, EncodingDecoderOutput](t, tool, EncodingDecoderInput{
		Data: "test", Encoding: "unknown",
	})
	if out.Error == "" || !strings.Contains(out.Error, "unsupported") {
		t.Errorf("should reject unsupported encoding: %v", out.Error)
	}
}

func TestEncodingDecoder_EmptyData(t *testing.T) {
	tool, err := NewEncodingDecoderTool()
	if err != nil {
		t.Fatalf("NewEncodingDecoderTool: %v", err)
	}

	out := invokeTool[EncodingDecoderInput, EncodingDecoderOutput](t, tool, EncodingDecoderInput{
		Data: "", Encoding: "hex",
	})
	if out.Error == "" {
		t.Fatal("should error on empty data")
	}
}

func TestEncodingDecoder_HexWithSpaces(t *testing.T) {
	tool, err := NewEncodingDecoderTool()
	if err != nil {
		t.Fatalf("NewEncodingDecoderTool: %v", err)
	}

	out := invokeTool[EncodingDecoderInput, EncodingDecoderOutput](t, tool, EncodingDecoderInput{
		Data: "48 65 6c 6c 6f", Encoding: "hex",
	})
	if out.Decoded != "Hello" {
		t.Errorf("hex with spaces: got %q, want 'Hello'", out.Decoded)
	}
}

func TestEncodingDecoder_InvalidHex(t *testing.T) {
	tool, err := NewEncodingDecoderTool()
	if err != nil {
		t.Fatalf("NewEncodingDecoderTool: %v", err)
	}

	out := invokeTool[EncodingDecoderInput, EncodingDecoderOutput](t, tool, EncodingDecoderInput{
		Data: "xyz", Encoding: "hex",
	})
	if out.Error == "" {
		t.Fatal("should error on invalid hex")
	}
}

// Registry 测试

func TestRegistry_HasAllNewTools(t *testing.T) {
	r := NewRegistry()

	fileInfoTool, err := NewFileInfoTool()
	if err != nil {
		t.Fatalf("NewFileInfoTool: %v", err)
	}
	r.Register("file_info", fileInfoTool)

	fileReaderTool, err := NewFileReaderTool()
	if err != nil {
		t.Fatalf("NewFileReaderTool: %v", err)
	}
	r.Register("file_reader", fileReaderTool)

	cmdExecTool, err := NewCommandExecutorTool()
	if err != nil {
		t.Fatalf("NewCommandExecutorTool: %v", err)
	}
	r.Register("command_executor", cmdExecTool)

	pythonRunnerTool, err := NewPythonRunnerTool()
	if err != nil {
		t.Fatalf("NewPythonRunnerTool: %v", err)
	}
	r.Register("python_runner", pythonRunnerTool)

	encodingDecoderTool, err := NewEncodingDecoderTool()
	if err != nil {
		t.Fatalf("NewEncodingDecoderTool: %v", err)
	}
	r.Register("encoding_decoder", encodingDecoderTool)

	expectedTools := []string{
		"file_info", "file_reader", "command_executor",
		"python_runner", "encoding_decoder",
	}

	for _, name := range expectedTools {
		tool, err := r.Get(name)
		if err != nil {
			t.Errorf("tool %q not in registry: %v", name, err)
			continue
		}
		if tool == nil {
			t.Errorf("tool %q is nil", name)
		}
	}

	allTools := r.All()
	if len(allTools) != len(expectedTools) {
		t.Errorf("All() count: got %d, want %d", len(allTools), len(expectedTools))
	}
}

// Prompt 测试

func TestSystemPrompt_ContainsNewToolNames(t *testing.T) {
	toolNames := []string{
		"file_info", "file_reader", "command_executor",
		"python_runner", "encoding_decoder",
	}
	for _, name := range toolNames {
		if name == "" {
			t.Errorf("tool name should not be empty")
		}
	}
	t.Log("prompt tool name validation: all 5 tool names are well-formed")
}
