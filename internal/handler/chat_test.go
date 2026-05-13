package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"eino_ctf_agent/internal/middleware"
	"eino_ctf_agent/internal/pkg/response"
)

func init() { gin.SetMode(gin.TestMode) }

// TestRecoveryMiddleware 验证 panic 被中间件恢复并返回统一 500 错误。
func TestRecoveryMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(middleware.Recovery())
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic, got %d", w.Code)
	}

	var body response.APIErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response should be valid JSON: %v", err)
	}
	if body.Error == "" {
		t.Error("error response should have error code")
	}
}

// TestLoggerMiddleware 验证日志中间件不 panic 且正常传递请求。
func TestLoggerMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(middleware.Logger())
	router.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ok", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestCORS 验证 CORS 中间件设置了正确的响应头。
func TestCORS(t *testing.T) {
	router := gin.New()
	router.Use(middleware.CORS([]string{"http://localhost:5173"}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	router.ServeHTTP(w, req)

	acao := w.Header().Get("Access-Control-Allow-Origin")
	if acao != "http://localhost:5173" {
		t.Errorf("expected http://localhost:5173, got %q", acao)
	}
}

// TestPreflight 验证 OPTIONS 预检请求返回 204。
func TestPreflight(t *testing.T) {
	router := gin.New()
	router.Use(middleware.CORS([]string{"*"}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", w.Code)
	}
}
