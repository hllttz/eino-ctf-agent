package tool

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const (
	archiveDefaultMaxFiles     = 500
	archiveDefaultMaxTotalSize = 100 << 20 // 100MB
)

// ArchiveFileEntry 压缩包内单个条目的元数据。
type ArchiveFileEntry struct {
	Name     string `json:"name" jsonschema:"description=file path within archive"`
	Size     int64  `json:"size" jsonschema:"description=uncompressed size in bytes"`
	IsDir    bool   `json:"is_dir" jsonschema:"description=whether the entry is a directory"`
	Modified string `json:"modified,omitempty" jsonschema:"description=modification time if available"`
}

// ArchiveToolInput archive_tool 工具的输入参数。
type ArchiveToolInput struct {
	Mode         string `json:"mode" jsonschema:"description=operation mode: identify, list, extract"`
	Path         string `json:"path" jsonschema:"description=path to archive file within work directory"`
	OutputDir    string `json:"output_dir,omitempty" jsonschema:"description=extract destination subdirectory under workdir (extract mode only, auto-created under .agent_tmp if empty)"`
	MaxFiles     int    `json:"max_files,omitempty" jsonschema:"description=max entries to extract, default 500"`
	MaxTotalSize int64  `json:"max_total_size,omitempty" jsonschema:"description=max total uncompressed bytes, default 100MB"`
}

// ArchiveToolOutput archive_tool 工具的输出结果。
type ArchiveToolOutput struct {
	Format     string             `json:"format,omitempty" jsonschema:"description=detected archive format"`
	FileCount  int                `json:"file_count,omitempty" jsonschema:"description=number of entries"`
	Files      []ArchiveFileEntry `json:"files,omitempty" jsonschema:"description=file entries in the archive"`
	ExtractDir string             `json:"extract_dir,omitempty" jsonschema:"description=directory where files were extracted"`
	Warnings   []string           `json:"warnings,omitempty" jsonschema:"description=non-fatal warnings"`
	Error      string             `json:"error,omitempty" jsonschema:"description=error message if operation failed"`
}

// NewArchiveTool 创建 CTF 附件压缩包识别/查看/解压工具。
func NewArchiveTool() (einotool.InvokableTool, error) {
	return utils.InferTool[ArchiveToolInput, ArchiveToolOutput](
		"archive_tool",
		"Identify, list contents, and safely extract CTF challenge archives. "+
			"Supports zip, tar, tar.gz/tgz, gz, bz2, xz, 7z, rar. "+
			"Detects archive format by magic bytes and extension. "+
			"Extract mode creates output in a controlled temp directory under the workdir. "+
			"Prevents Zip Slip / path traversal attacks. "+
			"Never executes extracted files. "+
			"Use this tool when handling challenge attachments in misc/forensics/reverse tasks.",
		func(ctx context.Context, input ArchiveToolInput) (ArchiveToolOutput, error) {
			mode := strings.ToLower(strings.TrimSpace(input.Mode))
			maxFiles := input.MaxFiles
			if maxFiles <= 0 {
				maxFiles = archiveDefaultMaxFiles
			}
			maxSize := input.MaxTotalSize
			if maxSize <= 0 {
				maxSize = archiveDefaultMaxTotalSize
			}

			log.Printf("[archive-tool] mode=%s path=%s max_files=%d max_size=%d",
				mode, input.Path, maxFiles, maxSize)

			absPath, err := resolvePath(input.Path)
			if err != nil {
				return ArchiveToolOutput{Error: err.Error()}, nil
			}

			format := detectFormat(absPath)
			if format == "" {
				return ArchiveToolOutput{Error: fmt.Sprintf("unrecognized archive format: %s", input.Path)}, nil
			}

			switch mode {
			case "identify":
				return identifyOnly(format, input.Path), nil
			case "list":
				return listArchive(format, absPath, maxFiles), nil
			case "extract":
				return extractArchive(format, absPath, input.OutputDir, maxFiles, maxSize), nil
			default:
				return ArchiveToolOutput{Error: fmt.Sprintf(
					"unsupported mode: %s (supported: identify, list, extract)", mode)}, nil
			}
		},
	)
}

// ─── 格式检测 ───

var formatSignatures = []struct {
	Format string
	Magic  []byte
	Offset int
}{
	{"7z", []byte("7z\xbc\xaf'\x1c"), 0},
	{"rar", []byte("Rar!\x1a\x07"), 0},
	{"xz", []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}, 0},
	{"gz", []byte{0x1f, 0x8b}, 0},
	{"bz2", []byte("BZh"), 0},
	{"zip", []byte("PK\x03\x04"), 0},
	{"zip", []byte("PK\x05\x06"), 0}, // empty zip
}

var extToFormat = map[string]string{
	".tar.gz":  "tar.gz",
	".tar.bz2": "tar.bz2",
	".tar.xz":  "tar.xz",
	".tgz":     "tar.gz",
	".tbz2":    "tar.bz2",
	".txz":     "tar.xz",
	".zip":     "zip",
	".tar":     "tar",
	".gz":      "gz",
	".bz2":     "bz2",
	".xz":      "xz",
	".7z":      "7z",
	".rar":     "rar",
}

func detectFormat(path string) string {
	lower := strings.ToLower(filepath.Base(path))

	// 先按 magic bytes
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		head := make([]byte, 8)
		n, _ := io.ReadFull(f, head)
		for _, sig := range formatSignatures {
			if len(sig.Magic) <= n {
				if matchMagic(head[sig.Offset:], sig.Magic) {
					format := sig.Format
					// gz/bz2/xz magic 可能是包裹 tar 的外层压缩
					if format == "gz" && (strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")) {
						return "tar.gz"
					}
					if format == "bz2" && (strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz2")) {
						return "tar.bz2"
					}
					if format == "xz" && (strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz")) {
						return "tar.xz"
					}
					return format
				}
			}
		}

		// tar: ustar magic at offset 257
		if n >= 264 {
			f.Seek(257, 0)
			ustar := make([]byte, 5)
			if _, err := io.ReadFull(f, ustar); err == nil && string(ustar) == "ustar" {
				return "tar"
			}
		}
	}

	// 回退到扩展名（按长度降序优先匹配双扩展名）
	sortedExts := sortedExtensionKeys()
	for _, ext := range sortedExts {
		if strings.HasSuffix(lower, ext) {
			return extToFormat[ext]
		}
	}
	return ""
}

func matchMagic(data, magic []byte) bool {
	if len(data) < len(magic) {
		return false
	}
	for i, b := range magic {
		if data[i] != b {
			return false
		}
	}
	return true
}

// ─── identify ───

func identifyOnly(format, origPath string) ArchiveToolOutput {
	return ArchiveToolOutput{
		Format: format,
		Warnings: []string{
			fmt.Sprintf("identified as %s by magic bytes or extension", format),
		},
	}
}

// ─── list ───

func listArchive(format, absPath string, maxFiles int) ArchiveToolOutput {
	switch format {
	case "zip":
		return listZip(absPath, maxFiles)
	case "tar":
		return listTar(absPath, maxFiles)
	case "tar.gz":
		return listTarGz(absPath, maxFiles)
	case "gz":
		return listSingle("gz", absPath)
	case "bz2":
		return listSingle("bz2", absPath)
	case "xz":
		return listSingle("xz", absPath)
	case "7z":
		return listExternal("7z", absPath, maxFiles)
	case "rar":
		return listExternal("unrar", absPath, maxFiles)
	default:
		return ArchiveToolOutput{Error: fmt.Sprintf("list not supported for format: %s", format)}
	}
}

func listZip(absPath string, maxFiles int) ArchiveToolOutput {
	r, err := zip.OpenReader(absPath)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("open zip: %v", err)}
	}
	defer r.Close()

	return buildEntryList(r.File, maxFiles, func(f *zip.File) ArchiveFileEntry {
		return ArchiveFileEntry{
			Name:     f.Name,
			Size:     int64(f.UncompressedSize64),
			IsDir:    f.FileInfo().IsDir(),
			Modified: f.Modified.Format("2006-01-02 15:04:05"),
		}
	})
}

func listTar(absPath string, maxFiles int) ArchiveToolOutput {
	f, err := os.Open(absPath)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("open tar: %v", err)}
	}
	defer f.Close()

	return readTarEntries(f, maxFiles, nil)
}

func listTarGz(absPath string, maxFiles int) ArchiveToolOutput {
	f, err := os.Open(absPath)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("open tar.gz: %v", err)}
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("gzip open: %v", err)}
	}
	defer gzReader.Close()

	return readTarEntries(gzReader, maxFiles, nil)
}

func listSingle(format, absPath string) ArchiveToolOutput {
	// gz/bz2/xz 单文件压缩，统计文件信息
	fi, err := os.Stat(absPath)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("stat %s: %v", format, err)}
	}
	return ArchiveToolOutput{
		Format:    format,
		FileCount: 1,
		Files: []ArchiveFileEntry{{
			Name: strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath)),
			Size: fi.Size(),
		}},
	}
}

func listExternal(toolName, absPath string, maxFiles int) ArchiveToolOutput {
	if !commandExists(toolName) {
		return ArchiveToolOutput{Error: fmt.Sprintf("external command %q not found; install %s to handle this format", toolName, toolName)}
	}

	var cmd *exec.Cmd
	switch toolName {
	case "7z":
		cmd = exec.Command("7z", "l", "-bd", absPath)
	case "unrar":
		cmd = exec.Command("unrar", "v", absPath)
	default:
		return ArchiveToolOutput{Error: fmt.Sprintf("unsupported external tool: %s", toolName)}
	}

	output, err := cmd.Output()
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("%s list failed: %v\n%s", toolName, err, string(output))}
	}
	return ArchiveToolOutput{
		Format: toolName,
		Warnings: []string{
			fmt.Sprintf("listing via external %s (parsed output limited)", toolName),
			string(output),
		},
	}
}

func readTarEntries(r io.Reader, maxFiles int, filter func(*tar.Header) bool) ArchiveToolOutput {
	tr := tar.NewReader(r)
	var files []ArchiveFileEntry
	var warnings []string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ArchiveToolOutput{Error: fmt.Sprintf("read tar entry: %v", err)}
		}

		if filter != nil && !filter(hdr) {
			continue
		}

		if len(files) >= maxFiles {
			warnings = append(warnings, fmt.Sprintf("archive has more than %d entries, output truncated", maxFiles))
			break
		}

		if warn := checkEntryPath(hdr.Name); warn != "" {
			warnings = append(warnings, warn)
		}

		files = append(files, ArchiveFileEntry{
			Name:     hdr.Name,
			Size:     hdr.Size,
			IsDir:    hdr.Typeflag == tar.TypeDir,
			Modified: hdr.ModTime.Format("2006-01-02 15:04:05"),
		})
	}

	return ArchiveToolOutput{
		Format:    "tar",
		FileCount: len(files),
		Files:     files,
		Warnings:  warnings,
	}
}

func buildEntryList[T any](entries []T, maxFiles int, convert func(T) ArchiveFileEntry) ArchiveToolOutput {
	var files []ArchiveFileEntry
	var warnings []string

	for i, e := range entries {
		if i >= maxFiles {
			warnings = append(warnings, fmt.Sprintf("archive has more than %d entries, output truncated", maxFiles))
			break
		}
		entry := convert(e)
		if warn := checkEntryPath(entry.Name); warn != "" {
			warnings = append(warnings, warn)
		}
		files = append(files, entry)
	}

	return ArchiveToolOutput{
		Format:    "archive",
		FileCount: len(files),
		Files:     files,
		Warnings:  warnings,
	}
}

// ─── extract ───

func extractArchive(format, absPath, outputDir string, maxFiles int, maxSize int64) ArchiveToolOutput {
	// 创建受控解压目录
	extractDir, err := createExtractDir(outputDir)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("create extract dir: %v", err)}
	}

	switch format {
	case "zip":
		return extractZip(absPath, extractDir, maxFiles, maxSize)
	case "tar":
		return extractTar(absPath, extractDir, maxFiles, maxSize, false)
	case "tar.gz":
		return extractTar(absPath, extractDir, maxFiles, maxSize, true)
	case "gz":
		return extractSingleGz(absPath, extractDir)
	case "bz2":
		return extractExternal("bzip2", absPath, extractDir, format)
	case "xz":
		return extractExternal("xz", absPath, extractDir, format)
	case "7z":
		return extract7z(absPath, extractDir, maxFiles, maxSize)
	case "rar":
		return extractUnrar(absPath, extractDir, maxFiles, maxSize)
	default:
		return ArchiveToolOutput{Error: fmt.Sprintf("extract not supported for format: %s", format)}
	}
}

func createExtractDir(outputDir string) (string, error) {
	tmpDir, err := agentTmpDir()
	if err != nil {
		return "", err
	}

	if outputDir != "" {
		extractDir := filepath.Join(tmpDir, "archive_extract_"+outputDir)
		if err := os.MkdirAll(extractDir, 0755); err != nil {
			return "", err
		}
		return extractDir, nil
	}

	// 生成随机子目录
	var randBytes [6]byte
	if _, err := rand.Read(randBytes[:]); err != nil {
		return "", err
	}
	suffix := hex.EncodeToString(randBytes[:])
	extractDir := filepath.Join(tmpDir, "archive_extract_"+suffix)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", err
	}
	return extractDir, nil
}

func safeExtractPath(extractDir, entryName string) (string, error) {
	clean := filepath.Clean(entryName)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute path in archive: %s", clean)
	}
	if strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("path traversal in archive: %s", entryName)
	}

	target := filepath.Join(extractDir, clean)
	absExtract, _ := filepath.Abs(extractDir)
	absTarget, _ := filepath.Abs(target)
	if !strings.HasPrefix(absTarget, absExtract+string(os.PathSeparator)) && absTarget != absExtract {
		return "", fmt.Errorf("path escapes extract dir: %s → %s", entryName, absTarget)
	}
	return target, nil
}

func checkEntryPath(name string) string {
	if filepath.IsAbs(name) {
		return fmt.Sprintf("suspicious absolute path in archive: %s", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Sprintf("suspicious traversal path in archive: %s", name)
	}
	return ""
}

// ─── zip extract ───

func extractZip(absPath, extractDir string, maxFiles int, maxSize int64) ArchiveToolOutput {
	r, err := zip.OpenReader(absPath)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("open zip: %v", err)}
	}
	defer r.Close()

	var extracted []ArchiveFileEntry
	var warnings []string
	var totalSize int64

	for _, f := range r.File {
		if len(extracted) >= maxFiles {
			warnings = append(warnings, fmt.Sprintf("max files reached (%d), remaining entries skipped", maxFiles))
			break
		}

		if warn := checkEntryPath(f.Name); warn != "" {
			return ArchiveToolOutput{Error: fmt.Sprintf("refusing to extract: %s", warn)}
		}

		target, err := safeExtractPath(extractDir, f.Name)
		if err != nil {
			return ArchiveToolOutput{Error: fmt.Sprintf("unsafe path: %v", err)}
		}

		entrySize := int64(f.UncompressedSize64)
		if totalSize+entrySize > maxSize {
			warnings = append(warnings, fmt.Sprintf("max total size reached (%d bytes), remaining entries skipped", maxSize))
			break
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			extracted = append(extracted, ArchiveFileEntry{Name: f.Name, IsDir: true})
			continue
		}

		os.MkdirAll(filepath.Dir(target), 0755)
		src, err := f.Open()
		if err != nil {
			return ArchiveToolOutput{Error: fmt.Sprintf("open zip entry %s: %v", f.Name, err)}
		}

		dst, err := os.Create(target)
		if err != nil {
			src.Close()
			return ArchiveToolOutput{Error: fmt.Sprintf("create %s: %v", target, err)}
		}

		written, err := io.Copy(dst, src)
		src.Close()
		dst.Close()
		if err != nil {
			return ArchiveToolOutput{Error: fmt.Sprintf("write %s: %v", target, err)}
		}

		totalSize += written
		extracted = append(extracted, ArchiveFileEntry{
			Name:     f.Name,
			Size:     written,
			Modified: f.Modified.Format("2006-01-02 15:04:05"),
		})
	}

	return ArchiveToolOutput{
		Format:     "zip",
		FileCount:  len(extracted),
		Files:      extracted,
		ExtractDir: extractDir,
		Warnings:   warnings,
	}
}

// ─── tar extract ───

func extractTar(absPath, extractDir string, maxFiles int, maxSize int64, isGz bool) ArchiveToolOutput {
	f, err := os.Open(absPath)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("open archive: %v", err)}
	}
	defer f.Close()

	var r io.Reader = f
	if isGz {
		gzReader, err := gzip.NewReader(f)
		if err != nil {
			return ArchiveToolOutput{Error: fmt.Sprintf("gzip open: %v", err)}
		}
		defer gzReader.Close()
		r = gzReader
	}

	tr := tar.NewReader(r)
	var extracted []ArchiveFileEntry
	var warnings []string
	var totalSize int64

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ArchiveToolOutput{Error: fmt.Sprintf("read tar entry: %v", err)}
		}

		if len(extracted) >= maxFiles {
			warnings = append(warnings, fmt.Sprintf("max files reached (%d), remaining entries skipped", maxFiles))
			break
		}

		if warn := checkEntryPath(hdr.Name); warn != "" {
			return ArchiveToolOutput{Error: fmt.Sprintf("refusing to extract: %s", warn)}
		}

		target, err := safeExtractPath(extractDir, hdr.Name)
		if err != nil {
			return ArchiveToolOutput{Error: fmt.Sprintf("unsafe path: %v", err)}
		}

		if totalSize+hdr.Size > maxSize {
			warnings = append(warnings, fmt.Sprintf("max total size reached (%d bytes), remaining entries skipped", maxSize))
			break
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
			extracted = append(extracted, ArchiveFileEntry{Name: hdr.Name, IsDir: true})
		case tar.TypeSymlink:
			warnings = append(warnings, fmt.Sprintf("symlink skipped: %s", hdr.Name))
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			dst, err := os.Create(target)
			if err != nil {
				return ArchiveToolOutput{Error: fmt.Sprintf("create %s: %v", target, err)}
			}
			written, _ := io.Copy(dst, tr)
			dst.Close()
			totalSize += written
			extracted = append(extracted, ArchiveFileEntry{
				Name:     hdr.Name,
				Size:     written,
				Modified: hdr.ModTime.Format("2006-01-02 15:04:05"),
			})
		default:
			warnings = append(warnings, fmt.Sprintf("skipped unsupported tar entry type %c: %s", hdr.Typeflag, hdr.Name))
		}
	}

	return ArchiveToolOutput{
		Format:     "tar",
		FileCount:  len(extracted),
		Files:      extracted,
		ExtractDir: extractDir,
		Warnings:   warnings,
	}
}

// ─── gz 单文件解压 ───

func extractSingleGz(absPath, extractDir string) ArchiveToolOutput {
	f, err := os.Open(absPath)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("open gz: %v", err)}
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("gzip open: %v", err)}
	}
	defer gzReader.Close()

	outName := gzReader.Name
	if outName == "" {
		outName = strings.TrimSuffix(filepath.Base(absPath), ".gz")
	}
	target := filepath.Join(extractDir, filepath.Base(outName))

	dst, err := os.Create(target)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("create %s: %v", target, err)}
	}
	defer dst.Close()

	written, err := io.Copy(dst, gzReader)
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("decompress gz: %v", err)}
	}

	return ArchiveToolOutput{
		Format:     "gz",
		FileCount:  1,
		Files:      []ArchiveFileEntry{{Name: filepath.Base(outName), Size: written}},
		ExtractDir: extractDir,
	}
}

// ─── external extract (bz2/xz) ───

func extractExternal(toolName, absPath, extractDir, format string) ArchiveToolOutput {
	if !commandExists(toolName) {
		return ArchiveToolOutput{Error: fmt.Sprintf(
			"external command %q not found; install %s to handle .%s files (try: apt install %s or brew install %s)",
			toolName, toolName, format, toolName, toolName)}
	}

	outName := strings.TrimSuffix(filepath.Base(absPath), "."+format)
	target := filepath.Join(extractDir, outName)

	var cmd *exec.Cmd
	switch toolName {
	case "bzip2":
		// bzip2 -d -c 解压到 stdout，写入目标文件
		cmd = exec.Command("bzip2", "-d", "-c", absPath)
	case "xz":
		cmd = exec.Command("xz", "-d", "-c", absPath)
	default:
		return ArchiveToolOutput{Error: fmt.Sprintf("unsupported external tool: %s", toolName)}
	}

	cmd.Stderr = nil
	output, err := cmd.Output()
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("%s decompress failed: %v", toolName, err)}
	}

	if err := os.WriteFile(target, output, 0644); err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("write %s: %v", target, err)}
	}

	return ArchiveToolOutput{
		Format:     format,
		FileCount:  1,
		Files:      []ArchiveFileEntry{{Name: outName, Size: int64(len(output))}},
		ExtractDir: extractDir,
	}
}

// ─── 7z extract ───

func extract7z(absPath, extractDir string, maxFiles int, maxSize int64) ArchiveToolOutput {
	if !commandExists("7z") {
		return ArchiveToolOutput{Error: "external command \"7z\" not found; install p7zip-full to handle .7z files"}
	}

	cmd := exec.Command("7z", "x", fmt.Sprintf("-o%s", extractDir), "-y", absPath)
	output, err := cmd.Output()
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("7z extract failed: %v\n%s", err, string(output))}
	}

	// 扫描解压目录统计结果
	files, _, _ := scanExtractDir(extractDir, maxFiles, maxSize)
	var warnings []string
	if len(files) >= maxFiles {
		warnings = append(warnings, fmt.Sprintf("output truncated at %d files", maxFiles))
	}

	return ArchiveToolOutput{
		Format:     "7z",
		FileCount:  len(files),
		Files:      files,
		ExtractDir: extractDir,
		Warnings:   warnings,
	}
}

// ─── unrar extract ───

func extractUnrar(absPath, extractDir string, maxFiles int, maxSize int64) ArchiveToolOutput {
	if !commandExists("unrar") {
		return ArchiveToolOutput{Error: "external command \"unrar\" not found; install unrar to handle .rar files"}
	}

	cmd := exec.Command("unrar", "x", "-y", absPath, extractDir+string(os.PathSeparator))
	output, err := cmd.Output()
	if err != nil {
		return ArchiveToolOutput{Error: fmt.Sprintf("unrar extract failed: %v\n%s", err, string(output))}
	}

	files, _, _ := scanExtractDir(extractDir, maxFiles, maxSize)
	var warnings []string
	if len(files) >= maxFiles {
		warnings = append(warnings, fmt.Sprintf("output truncated at %d files", maxFiles))
	}

	return ArchiveToolOutput{
		Format:     "rar",
		FileCount:  len(files),
		Files:      files,
		ExtractDir: extractDir,
		Warnings:   warnings,
	}
}

// sortedExtensionKeys 按长度降序返回扩展名，确保 .tar.gz 优先于 .gz 匹配。
func sortedExtensionKeys() []string {
	keys := make([]string, 0, len(extToFormat))
	for k := range extToFormat {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	return keys
}

// ─── helpers ───

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func scanExtractDir(dir string, maxFiles int, maxSize int64) ([]ArchiveFileEntry, int64, error) {
	var files []ArchiveFileEntry
	var totalSize int64

	entries, err := os.ReadDir(dir)
	if err != nil {
		return files, totalSize, err
	}

	for _, entry := range entries {
		if len(files) >= maxFiles {
			break
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		totalSize += info.Size()
		if totalSize > maxSize {
			break
		}
		files = append(files, ArchiveFileEntry{
			Name:     entry.Name(),
			Size:     info.Size(),
			IsDir:    entry.IsDir(),
			Modified: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	return files, totalSize, nil
}
