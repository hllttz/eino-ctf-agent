package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Writer SSE事件流写入器，封装HTTP ResponseWriter的SSE格式输出。
type Writer struct {
	w http.ResponseWriter
}

func NewWriter(w http.ResponseWriter) *Writer {
	return &Writer{w: w}
}

func SetHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
}

func (w *Writer) Event(event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal SSE event %s: %w", event, err)
	}
	if _, err := fmt.Fprintf(w.w, "event: %s\ndata: %s\n\n", event, payload); err != nil {
		return fmt.Errorf("write SSE event %s: %w", event, err)
	}
	if flusher, ok := w.w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}
