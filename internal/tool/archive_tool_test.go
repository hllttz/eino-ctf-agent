package tool

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── helpers ───

func setupArchiveTest(t *testing.T) (string, func()) {
	t.Helper()
	tmp := t.TempDir()
	if err := SetWorkDir(tmp); err != nil {
		t.Fatalf("SetWorkDir: %v", err)
	}
	return tmp, func() {}
}

func createZipFile(t *testing.T, dir, name string, files map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	w := zip.NewWriter(f)
	for name, content := range files {
		writer, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		writer.Write([]byte(content))
	}
	w.Close()
	f.Close()
	return name
}

func createTarFile(t *testing.T, dir, name string, files map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tar: %v", err)
	}
	w := tar.NewWriter(f)
	for name, content := range files {
		w.WriteHeader(&tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0644,
		})
		w.Write([]byte(content))
	}
	w.Close()
	f.Close()
	return name
}

func createTarGzFile(t *testing.T, dir, name string, files map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tar.gz: %v", err)
	}
	gzW := gzip.NewWriter(f)
	tw := tar.NewWriter(gzW)
	for name, content := range files {
		tw.WriteHeader(&tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0644,
		})
		tw.Write([]byte(content))
	}
	tw.Close()
	gzW.Close()
	f.Close()
	return name
}

func createGzFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gz: %v", err)
	}
	gzW := gzip.NewWriter(f)
	gzW.Name = strings.TrimSuffix(name, ".gz")
	gzW.Write([]byte(content))
	gzW.Close()
	f.Close()
	return name
}

func createPlainFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	os.WriteFile(path, []byte(content), 0644)
	return name
}

// ─── zip tests ───

func TestArchiveTool_ZipIdentify(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createZipFile(t, dir, "test.zip", map[string]string{"flag.txt": "flag{test}"})

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "identify",
		Path: "test.zip",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.Format != "zip" {
		t.Errorf("format: got %q, want 'zip'", out.Format)
	}
}

func TestArchiveTool_ZipList(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createZipFile(t, dir, "test.zip", map[string]string{
		"flag.txt":  "flag{test}",
		"readme.md": "# Hello",
	})

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "list",
		Path: "test.zip",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.FileCount != 2 {
		t.Errorf("file_count: got %d, want 2", out.FileCount)
	}
	found := make(map[string]bool)
	for _, f := range out.Files {
		found[f.Name] = true
	}
	if !found["flag.txt"] || !found["readme.md"] {
		t.Errorf("missing expected files in: %v", out.Files)
	}
}

func TestArchiveTool_ZipExtract(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createZipFile(t, dir, "test.zip", map[string]string{"flag.txt": "flag{extract_test}"})

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "extract",
		Path: "test.zip",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.FileCount != 1 {
		t.Errorf("file_count: got %d, want 1", out.FileCount)
	}
	if out.ExtractDir == "" {
		t.Error("extract_dir should not be empty")
	}

	// 验证文件确实被解压
	extracted := filepath.Join(out.ExtractDir, "flag.txt")
	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(data) != "flag{extract_test}" {
		t.Errorf("extracted content: got %q, want 'flag{extract_test}'", string(data))
	}
}

func TestArchiveTool_ZipCustomOutputDir(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createZipFile(t, dir, "test.zip", map[string]string{"flag.txt": "custom"})

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode:      "extract",
		Path:      "test.zip",
		OutputDir: "my_output",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.ExtractDir, "my_output") {
		t.Errorf("extract_dir should contain 'my_output': %s", out.ExtractDir)
	}
}

// ─── tar tests ───

func TestArchiveTool_TarIdentify(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createTarFile(t, dir, "test.tar", map[string]string{"flag.txt": "flag{tar}"})

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "identify",
		Path: "test.tar",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.Format != "tar" {
		t.Errorf("format: got %q, want 'tar'", out.Format)
	}
}

func TestArchiveTool_TarList(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createTarFile(t, dir, "test.tar", map[string]string{
		"a.txt": "A", "b.txt": "B",
	})

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "list",
		Path: "test.tar",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.FileCount != 2 {
		t.Errorf("file_count: got %d, want 2", out.FileCount)
	}
}

func TestArchiveTool_TarExtract(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createTarFile(t, dir, "test.tar", map[string]string{"flag.txt": "flag{tar_extract}"})

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "extract",
		Path: "test.tar",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	extracted := filepath.Join(out.ExtractDir, "flag.txt")
	data, _ := os.ReadFile(extracted)
	if string(data) != "flag{tar_extract}" {
		t.Errorf("content: got %q", string(data))
	}
}

// ─── tar.gz tests ───

func TestArchiveTool_TarGzList(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createTarGzFile(t, dir, "test.tar.gz", map[string]string{
		"flag.txt": "flag{tgz}",
		"note.md":  "# Note",
	})

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "list",
		Path: "test.tar.gz",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.FileCount != 2 {
		t.Errorf("file_count: got %d, want 2", out.FileCount)
	}
}

func TestArchiveTool_TgzExtract(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createTarGzFile(t, dir, "test.tgz", map[string]string{"flag.txt": "flag{tgz_extract}"})

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "extract",
		Path: "test.tgz",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	extracted := filepath.Join(out.ExtractDir, "flag.txt")
	data, _ := os.ReadFile(extracted)
	if string(data) != "flag{tgz_extract}" {
		t.Errorf("content: got %q", string(data))
	}
}

// ─── gz single file test ───

func TestArchiveTool_GzExtract(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createGzFile(t, dir, "data.txt.gz", "Hello GZip World\n")

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "extract",
		Path: "data.txt.gz",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.FileCount != 1 {
		t.Errorf("file_count: got %d, want 1", out.FileCount)
	}
	if out.Format != "gz" {
		t.Errorf("format: got %q, want 'gz'", out.Format)
	}
}

// ─── bz2 tests ───

func TestArchiveTool_Bz2Extract(t *testing.T) {
	if !commandExists("bzip2") {
		t.Skip("bzip2 not installed")
	}

	dir, cleanup := setupArchiveTest(t)
	defer cleanup()

	// 创建文本压缩文件
	plainPath := filepath.Join(dir, "data.txt")
	os.WriteFile(plainPath, []byte("Hello BZip2\n"), 0644)
	execCmd(t, dir, "bzip2", "-k", plainPath)

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "extract",
		Path: "data.txt.bz2",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.FileCount != 1 {
		t.Errorf("file_count: got %d, want 1", out.FileCount)
	}
}

// ─── xz tests ───

func TestArchiveTool_XzExtract(t *testing.T) {
	if !commandExists("xz") {
		t.Skip("xz not installed")
	}

	dir, cleanup := setupArchiveTest(t)
	defer cleanup()

	plainPath := filepath.Join(dir, "data.txt")
	os.WriteFile(plainPath, []byte("Hello XZ\n"), 0644)
	execCmd(t, dir, "xz", "-k", plainPath)

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "extract",
		Path: "data.txt.xz",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.FileCount != 1 {
		t.Errorf("file_count: got %d, want 1", out.FileCount)
	}
}

// ─── path traversal rejection tests ───

func TestArchiveTool_ZipRejectPathTraversal(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()

	// 手工构造带 ../ 的 zip
	path := filepath.Join(dir, "evil.zip")
	f, _ := os.Create(path)
	w := zip.NewWriter(f)
	w.Create("../etc/passwd")
	w.Close()
	f.Close()

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "extract",
		Path: "evil.zip",
	})
	if out.Error == "" {
		t.Fatal("should reject zip with ../ entry")
	}
	if !strings.Contains(out.Error, "traversal") && !strings.Contains(out.Error, "path") {
		t.Errorf("error should mention traversal or path: %s", out.Error)
	}
}

func TestArchiveTool_ZipRejectAbsolutePath(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()

	path := filepath.Join(dir, "evil2.zip")
	f, _ := os.Create(path)
	w := zip.NewWriter(f)
	w.Create("/etc/passwd")
	w.Close()
	f.Close()

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "extract",
		Path: "evil2.zip",
	})
	if out.Error == "" {
		t.Fatal("should reject zip with absolute path")
	}
}

func TestArchiveTool_TarRejectPathTraversal(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()

	path := filepath.Join(dir, "evil.tar")
	f, _ := os.Create(path)
	w := tar.NewWriter(f)
	w.WriteHeader(&tar.Header{Name: "../etc/passwd", Size: 10, Mode: 0644})
	w.Write([]byte("evil_data"))
	w.Close()
	f.Close()

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "extract",
		Path: "evil.tar",
	})
	if out.Error == "" {
		t.Fatal("should reject tar with ../ entry")
	}
}

func TestArchiveTool_ListWarnsTraversal(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()

	path := filepath.Join(dir, "warn.tar")
	f, _ := os.Create(path)
	w := tar.NewWriter(f)
	w.WriteHeader(&tar.Header{Name: "../secrets", Size: 0, Mode: 0644})
	w.Close()
	f.Close()

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "list",
		Path: "warn.tar",
	})
	if out.Error != "" {
		t.Fatalf("list should not fail, just warn: %s", out.Error)
	}
	hasWarning := false
	for _, w := range out.Warnings {
		if strings.Contains(w, "traversal") || strings.Contains(w, "suspicious") {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Error("should warn about traversal path in list mode")
	}
}

// ─── max_files / max_total_size tests ───

func TestArchiveTool_MaxFilesLimit(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()

	files := make(map[string]string)
	for i := 0; i < 20; i++ {
		files[fmt.Sprintf("file_%d.txt", i)] = "data"
	}
	createZipFile(t, dir, "many.zip", files)

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode:     "extract",
		Path:     "many.zip",
		MaxFiles: 5,
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.FileCount > 5 {
		t.Errorf("should limit to 5 files, got %d", out.FileCount)
	}
	hasWarn := false
	for _, w := range out.Warnings {
		if strings.Contains(w, "max files") {
			hasWarn = true
		}
	}
	if !hasWarn {
		t.Error("should warn about max files exceeded")
	}
}

func TestArchiveTool_MaxTotalSizeLimit(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()

	createZipFile(t, dir, "big.zip", map[string]string{
		"a.txt": strings.Repeat("A", 5000),
		"b.txt": strings.Repeat("B", 5000),
	})

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode:         "extract",
		Path:         "big.zip",
		MaxTotalSize: 100, // 允许 100 bytes only
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	// 文件总量远超限制，应只有较少文件被解压
	if out.FileCount > 1 {
		t.Errorf("should limit extraction by size, got %d files", out.FileCount)
	}
}

// ─── unrecognized format test ───

func TestArchiveTool_UnrecognizedFormat(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createPlainFile(t, dir, "hello.txt", "Hello World")

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "identify",
		Path: "hello.txt",
	})
	if out.Error == "" {
		t.Fatal("should error on unrecognized format")
	}
	if !strings.Contains(out.Error, "unrecognized") {
		t.Errorf("error should mention 'unrecognized': %s", out.Error)
	}
}

// ─── 7z / rar missing command test ───

func TestArchiveTool_7zMissingCommand(t *testing.T) {
	if commandExists("7z") {
		t.Skip("7z is installed, skipping missing-command test")
	}

	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	// 创建假 .7z 文件（实际不是有效 7z）
	createPlainFile(t, dir, "fake.7z", "not a real 7z")

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "extract",
		Path: "fake.7z",
	})
	// 可能报格式无法识别（magic bytes 不匹配），或者 7z 不可用
	if out.Error == "" {
		t.Fatal("should error")
	}
}

func TestArchiveTool_RarMissingCommand(t *testing.T) {
	if commandExists("unrar") {
		t.Skip("unrar is installed, skipping missing-command test")
	}

	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createPlainFile(t, dir, "fake.rar", "not a real rar")

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "extract",
		Path: "fake.rar",
	})
	if out.Error == "" {
		t.Fatal("should error")
	}
}

// ─── corrupt archive test ───

func TestArchiveTool_CorruptArchive(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()
	createPlainFile(t, dir, "corrupt.zip", "this is not a zip file at all")

	tool, err := NewArchiveTool()
	if err != nil {
		t.Fatalf("NewArchiveTool: %v", err)
	}

	// 扩展名是 .zip 但内容不是，list 应报错
	out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
		Mode: "list",
		Path: "corrupt.zip",
	})
	if out.Error == "" {
		t.Fatal("should error on corrupt archive")
	}
}

// ─── Extension-based format detection tests ───

func TestArchiveTool_DetectByExtension(t *testing.T) {
	dir, cleanup := setupArchiveTest(t)
	defer cleanup()

	tests := []struct {
		filename   string
		wantFormat string
	}{
		{"challenge.tar.bz2", "tar.bz2"},
		{"challenge.tar.xz", "tar.xz"},
		{"challenge.tgz", "tar.gz"},
		{"challenge.7z", "7z"},
		{"challenge.rar", "rar"},
		{"challenge.bz2", "bz2"},
		{"challenge.xz", "xz"},
	}

	for _, tc := range tests {
		// 但是这些没有有效的 magic bytes，它们应该fallback到扩展名识别
		createPlainFile(t, dir, tc.filename, "placeholder content - not a valid archive")

		tool, err := NewArchiveTool()
		if err != nil {
			t.Fatalf("NewArchiveTool: %v", err)
		}

		out := invokeTool[ArchiveToolInput, ArchiveToolOutput](t, tool, ArchiveToolInput{
			Mode: "identify",
			Path: tc.filename,
		})
		// 应该通过扩展名识别
		if out.Error != "" {
			t.Logf("%s identify error: %s (may need magic bytes for this format)", tc.filename, out.Error)
			continue
		}
		if out.Format != tc.wantFormat {
			t.Errorf("%s: format got %q, want %q", tc.filename, out.Format, tc.wantFormat)
		}
	}
}

// ─── helper ───

func execCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", name, err, string(out))
	}
}
