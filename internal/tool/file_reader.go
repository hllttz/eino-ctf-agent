package tool

import (
	"context"
	"fmt"
	"os"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// FileReaderInput 文件读取工具的输入参数。
type FileReaderInput struct {
	Path     string `json:"path" jsonschema:"description=relative path to the file within the working directory"`
	MaxBytes int    `json:"max_bytes,omitempty" jsonschema:"description=maximum bytes to read, defaults to 1MB"`
}

// FileReaderOutput 文件读取工具的输出结果。
type FileReaderOutput struct {
	Content   string `json:"content" jsonschema:"description=file content, truncated if exceeds max_bytes"`
	Size      int    `json:"size" jsonschema:"description=number of bytes read"`
	Truncated bool   `json:"truncated" jsonschema:"description=whether the content was truncated"`
	Error     string `json:"error,omitempty" jsonschema:"description=error message if any"`
}

// NewFileReaderTool 创建文件读取工具，读取工作目录内的文本/源码/题目文件。
func NewFileReaderTool() (einotool.InvokableTool, error) {
	return utils.InferTool[FileReaderInput, FileReaderOutput](
		"file_reader",
		"Read the content of a text, source code, or challenge file within the working directory. "+
			"Supports setting a maximum byte limit with max_bytes (defaults to 1MB). "+
			"Returns truncated=true if the file was larger than the limit. "+
			"Use this after file_info to examine the contents of interesting files.",
		func(ctx context.Context, input FileReaderInput) (FileReaderOutput, error) {
			absPath, err := resolvePath(input.Path)
			if err != nil {
				return FileReaderOutput{Error: err.Error()}, nil
			}

			info, err := os.Stat(absPath)
			if err != nil {
				return FileReaderOutput{Error: fmt.Sprintf("file not accessible: %s", err.Error())}, nil
			}
			if info.IsDir() {
				return FileReaderOutput{Error: fmt.Sprintf("path is a directory, not a file: %s", input.Path)}, nil
			}

			maxBytes := input.MaxBytes
			if maxBytes <= 0 {
				maxBytes = defaultFileMaxBytes
			}

			data, err := os.ReadFile(absPath)
			if err != nil {
				return FileReaderOutput{Error: fmt.Sprintf("read failed: %s", err.Error())}, nil
			}

			content, truncated := truncateOutput(string(data), maxBytes)
			return FileReaderOutput{
				Content:   content,
				Size:      len(content),
				Truncated: truncated,
			}, nil
		},
	)
}
