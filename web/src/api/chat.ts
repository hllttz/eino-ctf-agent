// TODO Phase 2.5: Chat API 模块
// 封装 /api/chat 和 /api/chat/stream 的调用逻辑。
// 流式接口使用 fetch + ReadableStream，不使用 EventSource。

const API_BASE = '/api'

/**
 * 非流式聊天请求
 * POST /api/chat
 */
export async function sendChatMessage(messages: { role: string; content: string }[]): Promise<string> {
  const res = await fetch(`${API_BASE}/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ messages }),
  })
  if (!res.ok) {
    const err = await res.json()
    throw new Error(err.message || 'Chat request failed')
  }
  const data = await res.json()
  return data.reply
}

/**
 * 流式聊天请求
 * POST /api/chat/stream
 * TODO Phase 2: 实现 SSE 流式读取
 */
export async function* streamChatMessage(
  messages: { role: string; content: string }[],
  signal?: AbortSignal
): AsyncGenerator<{ event: string; data: any }> {
  // TODO Phase 2: 使用 fetch + ReadableStream 读取 SSE 事件流
  void messages
  void signal
}
