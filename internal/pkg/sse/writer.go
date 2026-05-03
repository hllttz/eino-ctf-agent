package sse

// TODO Phase 2: SSE Writer 工具。
// 封装 SSE 事件格式的写入逻辑：
//   event: message_delta
//   data: {"content":"..."}
//
//   event: done
//   data: {}
//
//   event: error
//   data: {"message":"..."}
//
// 设置流式响应头：
//   Content-Type: text/event-stream
//   Cache-Control: no-cache
//   Connection: keep-alive
