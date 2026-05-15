package tool

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newIPv4TestServer forces tests to listen on 127.0.0.1 to avoid
// environments where httptest.NewServer falls back to [::1] and fails.
func newIPv4TestServer(t testing.TB, handler http.Handler) *httptest.Server {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4 test server: %v", err)
	}

	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}
	server.Start()
	return server
}
