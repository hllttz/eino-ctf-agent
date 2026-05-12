package tool

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// IDAMCPStatus Status 探测结果。
type IDAMCPStatus struct {
	Available bool   `json:"available"`
	Endpoint  string `json:"endpoint"`
	Error     string `json:"error,omitempty"`
}

// IDAMCPFunctionsResult 函数列表查询结果。
type IDAMCPFunctionsResult struct {
	Functions []string `json:"functions"`
	Total     int      `json:"total"`
	Error     string   `json:"error,omitempty"`
}

// IDAMCPDecompileResult 反编译结果。
type IDAMCPDecompileResult struct {
	Code  string `json:"code"`
	Error string `json:"error,omitempty"`
}

// IDAMCPStringsResult 字符串查询结果。
type IDAMCPStringsResult struct {
	Strings []string `json:"strings"`
	Total   int      `json:"total"`
	Error   string   `json:"error,omitempty"`
}

// IDAMCPXrefsResult 交叉引用查询结果。
type IDAMCPXrefsResult struct {
	Xrefs []string `json:"xrefs"`
	Error string   `json:"error,omitempty"`
}

// IDAMCPClient 封装对 IDA MCP 服务的只读分析调用。
// 工具层只依赖此接口，不关心底层是 SSE、HTTP 还是 Mock 传输。
type IDAMCPClient interface {
	Status(ctx context.Context) (*IDAMCPStatus, error)
	Functions(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error)
	Decompile(ctx context.Context, target string) (*IDAMCPDecompileResult, error)
	Strings(ctx context.Context, limit int) (*IDAMCPStringsResult, error)
	Xrefs(ctx context.Context, target string) (*IDAMCPXrefsResult, error)
}

// 默认配置
const (
	defaultIDAEndpoint        = "http://127.0.0.1:13337/sse"
	defaultIDATimeoutSec      = 5
	idaTransportPendingSuffix = " transport not fully implemented; IDA MCP SSE client pending"
)

// validateIDAEndpoint 校验 IDA MCP endpoint 只允许 IPv4 localhost。
// 当前阶段仅支持 IPv4 localhost，不支持 [::1]。
// 必须拒绝 0.0.0.0、外网 IP、远程域名、空 scheme、非 http/https scheme。
func validateIDAEndpoint(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid IDA MCP endpoint URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("IDA MCP endpoint scheme must be http or https, got %q", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("IDA MCP endpoint has empty host")
	}

	// 允许 IPv4 localhost
	if host == "localhost" || host == "127.0.0.1" {
		return raw, nil
	}

	// 拒绝 0.0.0.0、远程 IP、远程域名
	if host == "0.0.0.0" {
		return "", fmt.Errorf("IDA MCP endpoint 0.0.0.0 not allowed, use 127.0.0.1 or localhost")
	}
	if ip := net.ParseIP(host); ip != nil {
		return "", fmt.Errorf("IDA MCP endpoint must be localhost or 127.0.0.1, got %s", host)
	}
	return "", fmt.Errorf("IDA MCP endpoint must be localhost or 127.0.0.1, got %s", host)
}

// RealMCPClient IDA MCP 生产客户端。
// 当前阶段只实现 Status 真实探测，其余方法返回 transport pending 错误。
type RealMCPClient struct {
	endpoint string
	timeout  time.Duration
}

// NewRealMCPClient 创建 IDA MCP 客户端，校验 endpoint 安全性。
// endpoint 非法时返回 nil + error，调用方应记录 warning 并使用 disabled client。
func NewRealMCPClient(endpoint string, timeoutSec int) (*RealMCPClient, error) {
	validated, err := validateIDAEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	t := time.Duration(timeoutSec) * time.Second
	return &RealMCPClient{endpoint: validated, timeout: t}, nil
}

// Status 探测 IDA MCP 服务是否可达。
// SSE 是长连接，收到响应头后立即关闭 body，不读取完整 stream。
// 必须受 timeout 控制。
func (c *RealMCPClient) Status(ctx context.Context) (*IDAMCPStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return &IDAMCPStatus{Available: false, Endpoint: c.endpoint, Error: err.Error()}, nil
	}

	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return &IDAMCPStatus{Available: false, Endpoint: c.endpoint, Error: err.Error()}, nil
	}
	// 收到响应头即认为 reachable，立即关闭 body，不读取 SSE stream
	resp.Body.Close()

	return &IDAMCPStatus{Available: true, Endpoint: c.endpoint}, nil
}

// Functions 获取函数列表。当前 transport 未实现。
func (c *RealMCPClient) Functions(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error) {
	return &IDAMCPFunctionsResult{
		Error: "ida_functions" + idaTransportPendingSuffix,
	}, nil
}

// Decompile 反编译指定函数。当前 transport 未实现。
func (c *RealMCPClient) Decompile(ctx context.Context, target string) (*IDAMCPDecompileResult, error) {
	return &IDAMCPDecompileResult{
		Error: "ida_decompile" + idaTransportPendingSuffix,
	}, nil
}

// Strings 获取 IDA 识别的字符串。当前 transport 未实现。
func (c *RealMCPClient) Strings(ctx context.Context, limit int) (*IDAMCPStringsResult, error) {
	return &IDAMCPStringsResult{
		Error: "ida_strings" + idaTransportPendingSuffix,
	}, nil
}

// Xrefs 查询交叉引用。当前 transport 未实现。
func (c *RealMCPClient) Xrefs(ctx context.Context, target string) (*IDAMCPXrefsResult, error) {
	return &IDAMCPXrefsResult{
		Error: "ida_xrefs" + idaTransportPendingSuffix,
	}, nil
}

// MockMCPClient 用于单元测试的 IDA MCP 客户端，不依赖真实 IDA 服务。
// 所有方法返回预设数据，测试代码可注入自定义返回值。
type MockMCPClient struct {
	StatusFn    func(ctx context.Context) (*IDAMCPStatus, error)
	FunctionsFn func(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error)
	DecompileFn func(ctx context.Context, target string) (*IDAMCPDecompileResult, error)
	StringsFn   func(ctx context.Context, limit int) (*IDAMCPStringsResult, error)
	XrefsFn     func(ctx context.Context, target string) (*IDAMCPXrefsResult, error)
}

func (m *MockMCPClient) Status(ctx context.Context) (*IDAMCPStatus, error) {
	if m.StatusFn != nil {
		return m.StatusFn(ctx)
	}
	return &IDAMCPStatus{Available: true, Endpoint: "mock://ida"}, nil
}

func (m *MockMCPClient) Functions(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error) {
	if m.FunctionsFn != nil {
		return m.FunctionsFn(ctx, limit)
	}
	return &IDAMCPFunctionsResult{Functions: []string{"main", "check_password", "verify"}, Total: 3}, nil
}

func (m *MockMCPClient) Decompile(ctx context.Context, target string) (*IDAMCPDecompileResult, error) {
	if m.DecompileFn != nil {
		return m.DecompileFn(ctx, target)
	}
	return &IDAMCPDecompileResult{Code: "int " + target + "() { return 0; }"}, nil
}

func (m *MockMCPClient) Strings(ctx context.Context, limit int) (*IDAMCPStringsResult, error) {
	if m.StringsFn != nil {
		return m.StringsFn(ctx, limit)
	}
	return &IDAMCPStringsResult{Strings: []string{"password", "flag{", "admin"}, Total: 3}, nil
}

func (m *MockMCPClient) Xrefs(ctx context.Context, target string) (*IDAMCPXrefsResult, error) {
	if m.XrefsFn != nil {
		return m.XrefsFn(ctx, target)
	}
	return &IDAMCPXrefsResult{Xrefs: []string{"call " + target + " from main+0x42", "ref to " + target + " at data+0x10"}}, nil
}

// DisabledMCPClient IDA MCP 配置异常时使用，所有方法返回配置错误。
// 用于 endpoint 非法但不想阻止服务启动的场景。
type DisabledMCPClient struct {
	configError string
}

// NewDisabledMCPClient 创建 disabled client，configError 描述配置问题。
func NewDisabledMCPClient(configError string) *DisabledMCPClient {
	return &DisabledMCPClient{configError: configError}
}

func (d *DisabledMCPClient) Status(ctx context.Context) (*IDAMCPStatus, error) {
	return &IDAMCPStatus{Available: false, Endpoint: "", Error: d.configError}, nil
}

func (d *DisabledMCPClient) Functions(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error) {
	return &IDAMCPFunctionsResult{Error: d.configError}, nil
}

func (d *DisabledMCPClient) Decompile(ctx context.Context, target string) (*IDAMCPDecompileResult, error) {
	return &IDAMCPDecompileResult{Error: d.configError}, nil
}

func (d *DisabledMCPClient) Strings(ctx context.Context, limit int) (*IDAMCPStringsResult, error) {
	return &IDAMCPStringsResult{Error: d.configError}, nil
}

func (d *DisabledMCPClient) Xrefs(ctx context.Context, target string) (*IDAMCPXrefsResult, error) {
	return &IDAMCPXrefsResult{Error: d.configError}, nil
}

// EnvIDAEndpoint 读取 IDA_MCP_ENDPOINT 环境变量。
// 读不到时返回默认值，不做校验——校验由 NewRealMCPClient 完成。
func EnvIDAEndpoint() string {
	s := strings.TrimSpace(os.Getenv("IDA_MCP_ENDPOINT"))
	if s == "" {
		return defaultIDAEndpoint
	}
	return s
}

// EnvIDATimeout 读取 IDA_MCP_TIMEOUT_SECONDS 环境变量。
func EnvIDATimeout() int {
	s := strings.TrimSpace(os.Getenv("IDA_MCP_TIMEOUT_SECONDS"))
	if s == "" {
		return defaultIDATimeoutSec
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultIDATimeoutSec
	}
	return v
}
