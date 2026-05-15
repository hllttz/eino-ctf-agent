package tool

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
)

// ─── TCP host 校验测试 ───

func TestRemoteInteractor_TCP_Host_AllowLocalhost(t *testing.T) {
	err := validateTCPHost("127.0.0.1")
	if err != nil {
		t.Errorf("127.0.0.1 should be allowed: %v", err)
	}
}

func TestRemoteInteractor_TCP_Host_AllowPublicIP(t *testing.T) {
	err := validateTCPHost("1.2.3.4")
	if err != nil {
		t.Errorf("1.2.3.4 should be allowed: %v", err)
	}
}

func TestRemoteInteractor_TCP_Host_AllowDomain(t *testing.T) {
	err := validateTCPHost("challenge.example.com")
	if err != nil {
		t.Errorf("challenge.example.com should be allowed: %v", err)
	}
}

func TestRemoteInteractor_TCP_Host_AllowVPNInternalIP(t *testing.T) {
	for _, ip := range []string{"10.0.0.1", "172.16.0.1", "192.168.1.1"} {
		err := validateTCPHost(ip)
		if err != nil {
			t.Errorf("VPN internal IP %s should be allowed: %v", ip, err)
		}
	}
}

func TestRemoteInteractor_TCP_Host_AllowLocalhostName(t *testing.T) {
	err := validateTCPHost("localhost")
	if err != nil {
		t.Errorf("localhost should be allowed: %v", err)
	}
}

func TestRemoteInteractor_TCP_Host_RejectCIDR(t *testing.T) {
	err := validateTCPHost("192.168.1.0/24")
	if err == nil {
		t.Fatal("CIDR notation should be rejected")
	}
	if !strings.Contains(err.Error(), "CIDR") {
		t.Errorf("error should mention CIDR: %s", err.Error())
	}
}

func TestRemoteInteractor_TCP_Host_RejectMultipleHosts(t *testing.T) {
	err := validateTCPHost("1.1.1.1,2.2.2.2")
	if err == nil {
		t.Fatal("multiple hosts should be rejected")
	}
	if !strings.Contains(err.Error(), "multiple") && !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("error should mention multiple hosts: %s", err.Error())
	}
}

func TestRemoteInteractor_TCP_Host_RejectWithPortInHost(t *testing.T) {
	err := validateTCPHost("127.0.0.1:8080")
	if err == nil {
		t.Fatal("host with embedded port should be rejected")
	}
}

func TestRemoteInteractor_TCP_Host_RejectSpaceSeparated(t *testing.T) {
	err := validateTCPHost("10.0.0.1 10.0.0.2")
	if err == nil {
		t.Fatal("space-separated hosts should be rejected")
	}
}

// ─── TCP port 校验测试 ───

func TestRemoteInteractor_TCP_Port_RejectZero(t *testing.T) {
	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "tcp",
		Host: "127.0.0.1",
		Port: 0,
	})
	if out.Error == "" {
		t.Fatal("port 0 should be rejected")
	}
}

func TestRemoteInteractor_TCP_Port_Reject65536(t *testing.T) {
	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "tcp",
		Host: "127.0.0.1",
		Port: 65536,
	})
	if out.Error == "" {
		t.Fatal("port 65536 should be rejected")
	}
}

func TestRemoteInteractor_TCP_Port_RejectPortRange(t *testing.T) {
	// Port 字段是 int，无法直接表达 range (如 1000-2000)。
	// 但如果 Agent 错误传递了 1000 这样的值，它会被当单个端口接受。
	// 这个测试验证单个合法端口值不会触发 range 误判。
	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	// 单个端口 1000 应被接受（校验层不拒绝）
	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode:    "tcp",
		Host:    "127.0.0.1",
		Port:    1000,
		Timeout: 1,
	})
	// 连接可能被拒（没有服务监听），但不应是"非法参数"错误
	if out.Error != "" && strings.Contains(out.Error, "invalid port") {
		t.Errorf("single port 1000 should not be rejected as invalid: %s", out.Error)
	}
}

// ─── HTTP scheme 校验测试 ───

func TestRemoteInteractor_HTTP_Scheme_AllowHTTP(t *testing.T) {
	err := validateHTTPURL("http://challenge.example.com")
	if err != nil {
		t.Errorf("http scheme should be allowed: %v", err)
	}
}

func TestRemoteInteractor_HTTP_Scheme_AllowHTTPS(t *testing.T) {
	err := validateHTTPURL("https://challenge.example.com:8443/path?q=1")
	if err != nil {
		t.Errorf("https scheme should be allowed: %v", err)
	}
}

func TestRemoteInteractor_HTTP_Scheme_RejectFile(t *testing.T) {
	err := validateHTTPURL("file:///etc/passwd")
	if err == nil {
		t.Fatal("file:// scheme should be rejected")
	}
}

func TestRemoteInteractor_HTTP_Scheme_RejectGopher(t *testing.T) {
	err := validateHTTPURL("gopher://127.0.0.1:70/_hello")
	if err == nil {
		t.Fatal("gopher:// scheme should be rejected")
	}
}

func TestRemoteInteractor_HTTP_Scheme_RejectFTP(t *testing.T) {
	err := validateHTTPURL("ftp://ftp.example.com/file")
	if err == nil {
		t.Fatal("ftp:// scheme should be rejected")
	}
}

func TestRemoteInteractor_HTTP_Scheme_RejectNoScheme(t *testing.T) {
	err := validateHTTPURL("challenge.example.com/path")
	if err == nil {
		t.Fatal("URL without scheme should be rejected")
	}
}

// ─── TCP echo 交互测试 ───

func TestRemoteInteractor_TCP_Echo(t *testing.T) {
	listener, err := newLocalListener()
	if err != nil {
		t.Fatalf("create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				buf := make([]byte, 1024)
				n, _ := conn.Read(buf)
				conn.Write(buf[:n])
			}()
		}
	}()

	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "tcp",
		Host: addr.IP.String(),
		Port: addr.Port,
		Data: "Hello Server",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.Status != "success" {
		t.Errorf("status: got %q, want 'success'", out.Status)
	}
	if !strings.Contains(out.Response, "Hello Server") {
		t.Errorf("response should echo 'Hello Server', got: %s", out.Response)
	}
	if out.Truncated {
		t.Error("should not be truncated")
	}
}

func TestRemoteInteractor_TCP_HexData(t *testing.T) {
	listener, err := newLocalListener()
	if err != nil {
		t.Fatalf("create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				buf := make([]byte, 1024)
				n, _ := conn.Read(buf)
				conn.Write(buf[:n])
			}()
		}
	}()

	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "tcp",
		Host: addr.IP.String(),
		Port: addr.Port,
		Data: "0x414243",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Response, "ABC") {
		t.Errorf("should decode hex 0x414243 to 'ABC', got response: %s", out.Response)
	}
}

func TestRemoteInteractor_TCP_ConnectionRefused(t *testing.T) {
	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode:    "tcp",
		Host:    "127.0.0.1",
		Port:    19999,
		Timeout: 1,
	})
	if out.Error == "" {
		t.Fatal("should error on refused connection")
	}
	if out.Status != "error" {
		t.Errorf("status should be 'error', got %q", out.Status)
	}
}

func TestRemoteInteractor_TCP_MissingHost(t *testing.T) {
	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "tcp",
		Port: 80,
	})
	if out.Error == "" {
		t.Fatal("should error on missing host")
	}
}

// ─── HTTP 交互测试 ───

func TestRemoteInteractor_HTTP_GET(t *testing.T) {
	ts := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Write([]byte("Hello HTTP"))
	}))
	defer ts.Close()

	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "http",
		URL:  ts.URL + "/test",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.Status != "success" {
		t.Errorf("status: got %q, want 'success'", out.Status)
	}
	if !strings.Contains(out.Response, "Hello HTTP") {
		t.Errorf("response should contain 'Hello HTTP', got: %s", out.Response)
	}
}

func TestRemoteInteractor_HTTP_POST(t *testing.T) {
	var receivedBody string
	ts := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.Write([]byte("OK: " + receivedBody))
	}))
	defer ts.Close()

	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode:   "http",
		URL:    ts.URL + "/submit",
		Method: "POST",
		Data:   "flag=ctf{test}",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Response, "OK: flag=ctf{test}") {
		t.Errorf("response should contain 'OK: flag=ctf{test}', got: %s", out.Response)
	}
	_ = receivedBody
}

func TestRemoteInteractor_HTTP_CustomHeaders(t *testing.T) {
	var receivedUA, receivedAuth string
	ts := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		receivedAuth = r.Header.Get("Authorization")
		w.Write([]byte(fmt.Sprintf("UA=%s Auth=%s", receivedUA, receivedAuth)))
	}))
	defer ts.Close()

	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "http",
		URL:  ts.URL + "/headers",
		Headers: map[string]string{
			"User-Agent":    "CTF-Agent/1.0",
			"Authorization": "Bearer fake-token",
		},
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Response, "UA=CTF-Agent/1.0") {
		t.Errorf("should receive custom User-Agent header, got: %s", out.Response)
	}
	if !strings.Contains(out.Response, "Auth=Bearer fake-token") {
		t.Errorf("should receive custom Authorization header, got: %s", out.Response)
	}
}

func TestRemoteInteractor_HTTP_ErrorStatus(t *testing.T) {
	ts := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	}))
	defer ts.Close()

	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "http",
		URL:  ts.URL + "/404",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Status, "http_404") {
		t.Errorf("status should be 'http_404', got %q", out.Status)
	}
	if !strings.Contains(out.Response, "not found") {
		t.Errorf("response should contain body, got: %s", out.Response)
	}
}

func TestRemoteInteractor_HTTP_NoRedirect(t *testing.T) {
	ts := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", 302)
			return
		}
		w.Write([]byte("final destination"))
	}))
	defer ts.Close()

	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "http",
		URL:  ts.URL + "/redirect",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	// 不应跟随重定向，应收到 302 + Location
	if strings.Contains(out.Response, "final destination") {
		t.Error("should NOT follow redirects")
	}
	if !strings.Contains(out.Status, "http_302") {
		t.Errorf("status should reflect 302 redirect, got: %s", out.Status)
	}
}

// ─── TLS 测试 ───

func TestRemoteInteractor_InsecureTLS_DefaultFalse(t *testing.T) {
	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	// 不设置 insecure_tls 时，对自签名 HTTPS 应报 TLS 错误
	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode:    "http",
		URL:     "https://self-signed.badssl.com/",
		Timeout: 3,
	})
	// 可能超时或 TLS 错误，status 应为 error
	if out.Status == "success" {
		t.Error("self-signed cert without insecure_tls should fail")
	}
}

// ─── max_bytes / timeout 测试 ───

func TestRemoteInteractor_MaxBytesTruncation(t *testing.T) {
	ts := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回超过 64KB 的数据
		large := strings.Repeat("A", 70*1024)
		w.Write([]byte(large))
	}))
	defer ts.Close()

	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "http",
		URL:  ts.URL + "/large",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !out.Truncated {
		t.Error("response should be truncated at 64KB")
	}
	if len(out.Response) < 64*1024 {
		t.Errorf("truncated response should be ~64KB, got %d bytes", len(out.Response))
	}
}

func TestRemoteInteractor_TimeoutExceeded(t *testing.T) {
	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode:    "tcp",
		Host:    "127.0.0.1",
		Port:    80,
		Timeout: 999,
	})
	if out.Error == "" {
		t.Fatal("should error on excessive timeout")
	}
	if !strings.Contains(out.Error, "exceeds maximum") {
		t.Errorf("error should mention 'exceeds maximum': %s", out.Error)
	}
}

// ─── 通用校验测试 ───

func TestRemoteInteractor_InvalidMode(t *testing.T) {
	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "ftp",
		URL:  "http://example.com",
	})
	if out.Error == "" {
		t.Fatal("should error on invalid mode")
	}
	if !strings.Contains(out.Error, "unsupported mode") {
		t.Errorf("error should mention 'unsupported mode': %s", out.Error)
	}
}

func TestRemoteInteractor_HTTP_MissingURL(t *testing.T) {
	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode: "http",
	})
	if out.Error == "" {
		t.Fatal("should error on missing URL")
	}
}

func TestRemoteInteractor_HTTP_InvalidMethod(t *testing.T) {
	tool, err := NewRemoteInteractorTool()
	if err != nil {
		t.Fatalf("NewRemoteInteractorTool: %v", err)
	}

	out := invokeTool[RemoteInteractorInput, RemoteInteractorOutput](t, tool, RemoteInteractorInput{
		Mode:   "http",
		URL:    "http://127.0.0.1/",
		Method: "DELETE",
	})
	if out.Error == "" {
		t.Fatal("should error on unsupported HTTP method")
	}
}

// ─── helper ───

func newLocalListener() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}
