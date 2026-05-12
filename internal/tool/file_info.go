package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// FileInfoInput 文件信息工具的输入参数。
type FileInfoInput struct {
	Path string `json:"path" jsonschema:"description=relative path to the file or directory within the working directory"`
}

// FileInfoOutput 文件信息工具的输出结果。
type FileInfoOutput struct {
	Exists    bool   `json:"exists" jsonschema:"description=whether the file exists"`
	Size      int64  `json:"size" jsonschema:"description=file size in bytes"`
	IsDir     bool   `json:"is_dir" jsonschema:"description=whether the path is a directory"`
	Extension string `json:"extension" jsonschema:"description=file extension, e.g. .py .txt .c"`
	Type      string `json:"type" jsonschema:"description=simple file type description based on extension"`
	Error     string `json:"error,omitempty" jsonschema:"description=error message if any"`
}

// NewFileInfoTool 创建文件信息工具，查看文件是否存在、大小、类型等基本信息。
func NewFileInfoTool() (einotool.InvokableTool, error) {
	return utils.InferTool[FileInfoInput, FileInfoOutput](
		"file_info",
		"Inspect a file or directory within the working directory. "+
			"Returns whether it exists, its size, whether it is a directory, extension, and a simple type description. "+
			"Use this to check files before reading them or to explore directory contents.",
		func(ctx context.Context, input FileInfoInput) (FileInfoOutput, error) {
			absPath, err := resolvePath(input.Path)
			if err != nil {
				return FileInfoOutput{Error: err.Error()}, nil
			}

			info, err := os.Stat(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					return FileInfoOutput{Exists: false, Error: fmt.Sprintf("file not found: %s", input.Path)}, nil
				}
				return FileInfoOutput{Error: err.Error()}, nil
			}

			return FileInfoOutput{
				Exists:    true,
				Size:      info.Size(),
				IsDir:     info.IsDir(),
				Extension: strings.ToLower(filepath.Ext(info.Name())),
				Type:      classifyFile(info),
			}, nil
		},
	)
}

func classifyFile(info os.FileInfo) string {
	if info.IsDir() {
		return "directory"
	}

	name := strings.ToLower(info.Name())
	ext := filepath.Ext(name)

	switch ext {
	case ".py":
		return "Python script"
	case ".c":
		return "C source"
	case ".h":
		return "C header"
	case ".cpp", ".cc", ".cxx":
		return "C++ source"
	case ".hpp", ".hh", ".hxx":
		return "C++ header"
	case ".go":
		return "Go source"
	case ".rs":
		return "Rust source"
	case ".js":
		return "JavaScript"
	case ".ts":
		return "TypeScript"
	case ".java":
		return "Java source"
	case ".sh", ".bash":
		return "Shell script"
	case ".txt":
		return "Plain text"
	case ".md":
		return "Markdown"
	case ".pdf":
		return "PDF document"
	case ".png":
		return "PNG image"
	case ".jpg", ".jpeg":
		return "JPEG image"
	case ".gif":
		return "GIF image"
	case ".bmp":
		return "BMP image"
	case ".svg":
		return "SVG image"
	case ".elf", ".o", ".so":
		return "ELF binary"
	case ".exe", ".dll":
		return "PE binary"
	case ".zip":
		return "ZIP archive"
	case ".tar", ".gz", ".bz2", ".xz":
		return "Archive"
	case ".pcap", ".pcapng":
		return "Packet capture"
	case ".json":
		return "JSON data"
	case ".xml":
		return "XML data"
	case ".html", ".htm":
		return "HTML"
	case ".css":
		return "CSS stylesheet"
	case ".yaml", ".yml":
		return "YAML config"
	case ".toml":
		return "TOML config"
	case ".sql":
		return "SQL script"
	case ".db", ".sqlite", ".sqlite3":
		return "SQLite database"
	default:
		if info.Size() == 0 {
			return "empty file"
		}
		return "unknown type"
	}
}
