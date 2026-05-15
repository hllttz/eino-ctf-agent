package tool

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const (
	defaultRemoteTimeoutSec = 5
	maxRemoteTimeoutSec     = 20
	remoteOutputLimit       = 64 * 1024 // 64KB per response
)

// RemoteInteractorInput remote_interactor 工具的输入参数。
type RemoteInteractorInput struct {
	Mode    string            `json:"mode" jsonschema:"description=interaction mode: tcp or http"`
	Host    string            `json:"host,omitempty" jsonschema:"description=target host: single IP or single domain name (for tcp mode)"`
	Port    int               `json:"port,omitempty" jsonschema:"description=target port 1-65535 (for tcp mode)"`
	URL     string            `json:"url,omitempty" jsonschema:"description=target URL, http or https scheme only (for http mode)"`
	Method  string            `json:"method,omitempty" jsonschema:"description=HTTP method: GET or POST (for http mode, default GET)"`
	Headers map[string]string `json:"headers,omitempty" jsonschema:"description=HTTP headers (for http mode)"`
	Data    string            `json:"data,omitempty" jsonschema:"description=data to send, hex-encode binary data with 0x prefix"`
	Timeout int               `json:"timeout,omitempty" jsonschema:"description=timeout seconds, default 5, max 20"`
	// InsecureTLS 仅在 CTF 自签名证书场景下设为 true。默认 false，使用系统 CA 验证。
	InsecureTLS bool `json:"insecure_tls,omitempty" jsonschema:"description=skip TLS certificate verification, for CTF self-signed certs only (default false)"`
}

// RemoteInteractorOutput remote_interactor 工具的输出结果。
type RemoteInteractorOutput struct {
	Status    string `json:"status" jsonschema:"description=connection status: success, connected, http_NNN, error"`
	Response  string `json:"response" jsonschema:"description=received data as string, truncated if exceeds 64KB"`
	Truncated bool   `json:"truncated" jsonschema:"description=whether response was truncated"`
	Error     string `json:"error,omitempty" jsonschema:"description=error message if operation failed"`
}

// NewRemoteInteractorTool 创建 CTF 远程交互工具。
//
// 定位：CTF/pwn 靶机交互工具，用于连接用户明确指定的单个目标，发送 payload，读取响应。
// 不是扫描器——不扫描网段/端口范围，不枚举目录/子域名，不监听端口，不反连 shell。
func NewRemoteInteractorTool() (einotool.InvokableTool, error) {
	return utils.InferTool[RemoteInteractorInput, RemoteInteractorOutput](
		"remote_interactor",
		"Connect to a single CTF/pwn challenge target specified by the user. "+
			"TCP mode: connect to ONE host:port, send payload, receive response. "+
			"HTTP mode: send ONE GET or POST request to a single URL, receive response. "+
			"Send binary data by hex-encoding with 0x prefix (e.g. 0x0a0d). "+
			"NOT a scanner — do NOT use for port scanning, network sweeping, directory brute-forcing, "+
			"subdomain enumeration, or accessing multiple targets in one call. "+
			"Each call connects to exactly one target.",
		func(ctx context.Context, input RemoteInteractorInput) (RemoteInteractorOutput, error) {
			mode := strings.ToLower(strings.TrimSpace(input.Mode))
			timeout := input.Timeout
			if timeout <= 0 {
				timeout = defaultRemoteTimeoutSec
			}
			if timeout > maxRemoteTimeoutSec {
				return RemoteInteractorOutput{Error: fmt.Sprintf("timeout %d exceeds maximum %d", timeout, maxRemoteTimeoutSec)}, nil
			}

			log.Printf("[remote-tool] mode=%s host=%s port=%d url=%s timeout=%d data_len=%d insecure_tls=%v",
				mode, input.Host, input.Port, input.URL, timeout, len(input.Data), input.InsecureTLS)

			switch mode {
			case "tcp":
				return tcpInteract(input, timeout), nil
			case "http":
				return httpInteract(input, timeout), nil
			default:
				return RemoteInteractorOutput{Error: fmt.Sprintf(
					"unsupported mode: %s (supported: tcp, http)", mode)}, nil
			}
		},
	)
}

// ─── TCP 输入校验 ───

// validateTCPHost 校验 TCP host 字段：必须是单个 IP 或单个域名。
// 拒绝：CIDR、逗号分隔的多个 host、空字符串、含端口号的字符串。
func validateTCPHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("host is required for tcp mode")
	}

	// 拒绝包含 CIDR 前缀
	if strings.Contains(host, "/") {
		return fmt.Errorf("CIDR notation not allowed in host: %s (use a single IP, not a subnet)", host)
	}

	// 拒绝逗号分隔（多个 host）
	if strings.Contains(host, ",") {
		return fmt.Errorf("multiple hosts not allowed: %s (use a single IP or domain, not a list)", host)
	}

	// 拒绝空格分隔
	if strings.Contains(host, " ") {
		return fmt.Errorf("host must be a single IP or domain: %s", host)
	}

	// 拒绝包含冒号（可能误填了 host:port）
	if strings.Contains(host, ":") {
		return fmt.Errorf("host must not contain port; use the port field: %s", host)
	}

	// 尝试解析为 IP
	if ip := net.ParseIP(host); ip != nil {
		return nil
	}

	// 非 IP → 作为域名校验基本格式
	if len(host) > 253 {
		return fmt.Errorf("host too long (max 253 chars): %s", host)
	}

	// 域名至少包含一个点（允许 localhost）
	if host != "localhost" && !strings.Contains(host, ".") {
		return fmt.Errorf("host must be a valid IP, 'localhost', or a domain name with at least one dot: %s", host)
	}

	return nil
}

// ─── TCP 交互 ───

func tcpInteract(input RemoteInteractorInput, timeoutSec int) RemoteInteractorOutput {
	if err := validateTCPHost(input.Host); err != nil {
		return RemoteInteractorOutput{Error: err.Error()}
	}
	if input.Port < 1 || input.Port > 65535 {
		return RemoteInteractorOutput{Error: fmt.Sprintf("invalid port: %d (must be 1-65535)", input.Port)}
	}

	addr := fmt.Sprintf("%s:%d", input.Host, input.Port)
	timeout := time.Duration(timeoutSec) * time.Second

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return RemoteInteractorOutput{Status: "error", Error: fmt.Sprintf("tcp connect to %s: %v", addr, err)}
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	// 发送数据
	sendData := decodeHexOrRaw(input.Data)
	if len(sendData) > 0 {
		written, err := conn.Write(sendData)
		if err != nil {
			return RemoteInteractorOutput{Status: "error", Error: fmt.Sprintf("tcp send to %s: %v", addr, err)}
		}
		log.Printf("[remote-tool] tcp sent=%d bytes to %s", written, addr)
	}

	// 接收响应
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if len(buf) >= remoteOutputLimit {
				break
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			if len(buf) > 0 {
				break
			}
			return RemoteInteractorOutput{Status: "error", Error: fmt.Sprintf("tcp recv from %s: %v", addr, err)}
		}
		if len(buf) < 256 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	response := string(buf)
	result, truncated := truncateOutput(response, remoteOutputLimit)
	if truncated {
		result += "\n...[remote response truncated at 64KB]"
	}

	status := "success"
	if len(buf) == 0 {
		status = "connected"
	}

	return RemoteInteractorOutput{
		Status:    status,
		Response:  result,
		Truncated: truncated,
	}
}

// ─── HTTP URL 校验 ───

// validateHTTPURL 校验 HTTP URL：仅允许 http/https scheme。
// 拒绝：file://、gopher://、ftp:// 等非 http scheme。
func validateHTTPURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("url is required for http mode")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme == "" {
		return fmt.Errorf("URL must have an explicit scheme (http:// or https://)")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q — only http and https are allowed", u.Scheme)
	}

	if u.Host == "" {
		return fmt.Errorf("URL must contain a host: %s", rawURL)
	}

	return nil
}

// ─── HTTP 交互 ───

func httpInteract(input RemoteInteractorInput, timeoutSec int) RemoteInteractorOutput {
	if err := validateHTTPURL(input.URL); err != nil {
		return RemoteInteractorOutput{Error: err.Error()}
	}

	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = "GET"
	}
	if method != "GET" && method != "POST" {
		return RemoteInteractorOutput{Error: fmt.Sprintf("unsupported HTTP method: %s (supported: GET, POST)", method)}
	}

	timeout := time.Duration(timeoutSec) * time.Second
	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: input.InsecureTLS},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	sendData := decodeHexOrRaw(input.Data)
	var body io.Reader
	if len(sendData) > 0 {
		body = strings.NewReader(string(sendData))
	}

	req, err := http.NewRequest(method, input.URL, body)
	if err != nil {
		return RemoteInteractorOutput{Status: "error", Error: fmt.Sprintf("http create request: %v", err)}
	}

	for k, v := range input.Headers {
		req.Header.Set(k, v)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return RemoteInteractorOutput{Status: "error", Error: fmt.Sprintf("http request to %s: %v", input.URL, err)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, int64(remoteOutputLimit+1)))
	if err != nil {
		return RemoteInteractorOutput{Status: "error", Error: fmt.Sprintf("http read response: %v", err)}
	}

	truncated := len(respBody) > remoteOutputLimit
	response := string(respBody)
	if truncated {
		response = response[:remoteOutputLimit] + "\n...[http response truncated at 64KB]"
	}

	log.Printf("[remote-tool] http %s %s → %d (%d bytes)", method, input.URL, resp.StatusCode, len(respBody))

	// 3xx 时附带 Location 提示（但不跟随）
	status := "success"
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		status = fmt.Sprintf("http_%d", resp.StatusCode)
		if loc != "" {
			status += fmt.Sprintf(" (redirect to: %s, not followed)", loc)
		}
	} else if resp.StatusCode >= 400 {
		status = fmt.Sprintf("http_%d", resp.StatusCode)
	}

	return RemoteInteractorOutput{
		Status:    status,
		Response:  response,
		Truncated: truncated,
	}
}

// decodeHexOrRaw 如果 data 以 0x 开头则 hex 解码，否则原样返回。
func decodeHexOrRaw(data string) []byte {
	s := strings.TrimSpace(data)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		decoded, err := hex.DecodeString(s[2:])
		if err == nil {
			return decoded
		}
	}
	return []byte(s)
}
