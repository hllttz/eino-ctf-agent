package tool

import (
	"context"
	"encoding/json"
	"log"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// IDA 工具使用的 IDAMCPClient 实例，由 main.go 在启动时注入。
var idaClient IDAMCPClient

// SetIDAClient 注入 IDA MCP 客户端，供所有 IDA 工具使用。
// 生产环境传入 RealMCPClient 或 DisabledMCPClient，测试环境传入 MockMCPClient。
func SetIDAClient(c IDAMCPClient) {
	idaClient = c
}

// idaOutputLimit IDA 工具输出截断上限，100KB。
const idaOutputLimit = 100 * 1024

// ida_status

// IDAStatusInput ida_status 工具的输入参数（无）。
type IDAStatusInput struct{}

// IDAStatusOutput ida_status 工具的输出结果。
type IDAStatusOutput struct {
	Available bool   `json:"available" jsonschema:"description=whether the IDA MCP service is reachable"`
	Endpoint  string `json:"endpoint" jsonschema:"description=the IDA MCP endpoint being checked"`
	Error     string `json:"error,omitempty" jsonschema:"description=error message if the check failed"`
}

// NewIDAStatusTool 创建 IDA MCP 状态探测工具。
// 用于在调用其他 IDA 工具前确认服务是否可用。
func NewIDAStatusTool() (einotool.InvokableTool, error) {
	return utils.InferTool[IDAStatusInput, IDAStatusOutput](
		"ida_status",
		"Check if the IDA MCP service is reachable. "+
			"Call this before using any other ida_* tools. "+
			"If unavailable, fall back to command_executor (strings, readelf, objdump) for binary analysis.",
		func(ctx context.Context, input IDAStatusInput) (IDAStatusOutput, error) {
			if idaClient == nil {
				return IDAStatusOutput{Available: false, Error: "IDA MCP client not configured"}, nil
			}
			status, err := idaClient.Status(ctx)
			if err != nil {
				return IDAStatusOutput{Available: false, Error: err.Error()}, nil
			}
			return IDAStatusOutput{
				Available: status.Available,
				Endpoint:  status.Endpoint,
				Error:     status.Error,
			}, nil
		},
	)
}

// ida_functions

// IDAFunctionsInput ida_functions 工具的输入参数。
type IDAFunctionsInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"description=maximum number of functions to return, default 200"`
}

// IDAFunctionsOutput ida_functions 工具的输出结果。
type IDAFunctionsOutput struct {
	Functions string `json:"functions" jsonschema:"description=JSON array of function names, truncated if exceeds limit"`
	Total     int    `json:"total" jsonschema:"description=total number of functions found"`
	Truncated bool   `json:"truncated" jsonschema:"description=whether the output was truncated"`
	Error     string `json:"error,omitempty" jsonschema:"description=error message if the call failed"`
}

// NewIDAFunctionsTool 创建 IDA 函数列表查询工具。
func NewIDAFunctionsTool() (einotool.InvokableTool, error) {
	return utils.InferTool[IDAFunctionsInput, IDAFunctionsOutput](
		"ida_functions",
		"List functions identified by IDA in the loaded binary. "+
			"Use this to get an overview of function names before decompiling specific ones. "+
			"Returns a JSON array of function names, truncated if the list is large.",
		func(ctx context.Context, input IDAFunctionsInput) (IDAFunctionsOutput, error) {
			log.Printf("[ida-react-tool] logical=ida_functions handler=IDAMCPClient.Functions")
			if idaClient == nil {
				return IDAFunctionsOutput{Error: "IDA MCP client not configured"}, nil
			}
			limit := input.Limit
			if limit <= 0 {
				limit = 200
			}
			result, err := idaClient.Functions(ctx, limit)
			if err != nil {
				return IDAFunctionsOutput{Error: err.Error()}, nil
			}
			if result.Error != "" {
				return IDAFunctionsOutput{Error: result.Error}, nil
			}

			jsonBytes, _ := json.Marshal(result.Functions)
			content, truncated := truncateOutput(string(jsonBytes), idaOutputLimit)
			if truncated {
				content += "\n...[ida_functions output truncated]"
			}
			return IDAFunctionsOutput{
				Functions: content,
				Total:     result.Total,
				Truncated: truncated,
			}, nil
		},
	)
}

// ida_decompile

// IDADecompileInput ida_decompile 工具的输入参数。
type IDADecompileInput struct {
	Target string `json:"target" jsonschema:"description=function name or address to decompile, e.g. main or 0x401000"`
}

// IDADecompileOutput ida_decompile 工具的输出结果。
type IDADecompileOutput struct {
	Code      string `json:"code" jsonschema:"description=decompiled pseudocode, truncated if exceeds limit"`
	Truncated bool   `json:"truncated" jsonschema:"description=whether the output was truncated"`
	Error     string `json:"error,omitempty" jsonschema:"description=error message if decompilation failed"`
}

// NewIDADecompileTool 创建 IDA 反编译工具。
func NewIDADecompileTool() (einotool.InvokableTool, error) {
	return utils.InferTool[IDADecompileInput, IDADecompileOutput](
		"ida_decompile",
		"Decompile a specific function by name or address using IDA. "+
			"Target suspicious functions first: main, check, verify, auth, login, "+
			"read, gets, scanf, strcpy, system. "+
			"Do NOT decompile all functions — focus on security-relevant ones. "+
			"Call this AFTER ida_status confirms the service is available.",
		func(ctx context.Context, input IDADecompileInput) (IDADecompileOutput, error) {
			if idaClient == nil {
				return IDADecompileOutput{Error: "IDA MCP client not configured"}, nil
			}
			result, err := idaClient.Decompile(ctx, input.Target)
			if err != nil {
				return IDADecompileOutput{Error: err.Error()}, nil
			}
			if result.Error != "" {
				return IDADecompileOutput{Error: result.Error}, nil
			}

			code, truncated := truncateOutput(result.Code, idaOutputLimit)
			if truncated {
				code += "\n...[ida_decompile output truncated]"
			}
			return IDADecompileOutput{
				Code:      code,
				Truncated: truncated,
			}, nil
		},
	)
}

// ida_strings

// IDAStringsInput ida_strings 工具的输入参数。
type IDAStringsInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"description=maximum number of strings to return, default 200"`
}

// IDAStringsOutput ida_strings 工具的输出结果。
type IDAStringsOutput struct {
	Strings   string `json:"strings" jsonschema:"description=JSON array of identified strings, truncated if exceeds limit"`
	Total     int    `json:"total" jsonschema:"description=total number of strings found"`
	Truncated bool   `json:"truncated" jsonschema:"description=whether the output was truncated"`
	Error     string `json:"error,omitempty" jsonschema:"description=error message if the call failed"`
}

// NewIDAStringsTool 创建 IDA 字符串查询工具。
func NewIDAStringsTool() (einotool.InvokableTool, error) {
	return utils.InferTool[IDAStringsInput, IDAStringsOutput](
		"ida_strings",
		"List strings identified by IDA in the loaded binary. "+
			"IDA's string detection is more accurate than the command-line 'strings' tool "+
			"for binaries with non-standard encodings. "+
			"Call this AFTER ida_status confirms the service is available.",
		func(ctx context.Context, input IDAStringsInput) (IDAStringsOutput, error) {
			if idaClient == nil {
				return IDAStringsOutput{Error: "IDA MCP client not configured"}, nil
			}
			limit := input.Limit
			if limit <= 0 {
				limit = 200
			}
			result, err := idaClient.Strings(ctx, limit)
			if err != nil {
				return IDAStringsOutput{Error: err.Error()}, nil
			}
			if result.Error != "" {
				return IDAStringsOutput{Error: result.Error}, nil
			}

			jsonBytes, _ := json.Marshal(result.Strings)
			content, truncated := truncateOutput(string(jsonBytes), idaOutputLimit)
			if truncated {
				content += "\n...[ida_strings output truncated]"
			}
			return IDAStringsOutput{
				Strings:   content,
				Total:     result.Total,
				Truncated: truncated,
			}, nil
		},
	)
}

// ida_xrefs

// IDAXrefsInput ida_xrefs 工具的输入参数。
type IDAXrefsInput struct {
	Target string `json:"target" jsonschema:"description=function name, address, or string to find cross-references to"`
}

// IDAXrefsOutput ida_xrefs 工具的输出结果。
type IDAXrefsOutput struct {
	Xrefs     string `json:"xrefs" jsonschema:"description=JSON array of cross-reference descriptions, truncated if exceeds limit"`
	Truncated bool   `json:"truncated" jsonschema:"description=whether the output was truncated"`
	Error     string `json:"error,omitempty" jsonschema:"description=error message if the call failed"`
}

// NewIDAXrefsTool 创建 IDA 交叉引用查询工具。
func NewIDAXrefsTool() (einotool.InvokableTool, error) {
	return utils.InferTool[IDAXrefsInput, IDAXrefsOutput](
		"ida_xrefs",
		"Find cross-references to a function, address, or string within the binary. "+
			"Use this to trace where a suspicious function is called from, "+
			"or where a sensitive string (like 'flag' or 'password') is referenced. "+
			"Call this AFTER ida_status confirms the service is available.",
		func(ctx context.Context, input IDAXrefsInput) (IDAXrefsOutput, error) {
			if idaClient == nil {
				return IDAXrefsOutput{Error: "IDA MCP client not configured"}, nil
			}
			result, err := idaClient.Xrefs(ctx, input.Target)
			if err != nil {
				return IDAXrefsOutput{Error: err.Error()}, nil
			}
			if result.Error != "" {
				return IDAXrefsOutput{Error: result.Error}, nil
			}

			jsonBytes, _ := json.Marshal(result.Xrefs)
			content, truncated := truncateOutput(string(jsonBytes), idaOutputLimit)
			if truncated {
				content += "\n...[ida_xrefs output truncated]"
			}
			return IDAXrefsOutput{
				Xrefs:     content,
				Truncated: truncated,
			}, nil
		},
	)
}

// ida_disasm

// IDADisasmInput ida_disasm 工具的输入参数。
type IDADisasmInput struct {
	Address string `json:"address" jsonschema:"description=start address in hex (e.g. 0x401000) or function name to disassemble"`
	End     string `json:"end,omitempty" jsonschema:"description=end address in hex, optional"`
	Count   int    `json:"count,omitempty" jsonschema:"description=maximum number of instructions to return, default 100"`
}

// IDADisasmOutput ida_disasm 工具的输出结果。
type IDADisasmOutput struct {
	Instructions string `json:"instructions" jsonschema:"description=disassembled instructions, truncated if exceeds limit"`
	Truncated    bool   `json:"truncated" jsonschema:"description=whether the output was truncated"`
	Error        string `json:"error,omitempty" jsonschema:"description=error message if disassembly failed"`
}

// NewIDADisasmTool 创建 IDA 反汇编工具。
func NewIDADisasmTool() (einotool.InvokableTool, error) {
	return utils.InferTool[IDADisasmInput, IDADisasmOutput](
		"ida_disasm",
		"Disassemble instructions at a given address using IDA. "+
			"Use this to examine assembly code around a specific location, "+
			"check JUMPOUT targets, analyze cmp/test/jmp/jcc patterns, "+
			"or understand control flow at the instruction level. "+
			"Provide address in hex (e.g. 0x401000) or as a function name. "+
			"Call this AFTER ida_status confirms the service is available.",
		func(ctx context.Context, input IDADisasmInput) (IDADisasmOutput, error) {
			log.Printf("[ida-react-tool] logical=ida_disasm handler=IDAMCPClient.Disasm")
			if idaClient == nil {
				return IDADisasmOutput{Error: "IDA MCP client not configured"}, nil
			}
			count := input.Count
			if count <= 0 {
				count = 100
			}
			result, err := idaClient.Disasm(ctx, input.Address, input.End, count)
			if err != nil {
				return IDADisasmOutput{Error: err.Error()}, nil
			}
			if result.Error != "" {
				return IDADisasmOutput{Error: result.Error}, nil
			}

			content, truncated := truncateOutput(result.Instructions, idaOutputLimit)
			if truncated {
				content += "\n...[ida_disasm output truncated]"
			}
			return IDADisasmOutput{
				Instructions: content,
				Truncated:    truncated,
			}, nil
		},
	)
}
