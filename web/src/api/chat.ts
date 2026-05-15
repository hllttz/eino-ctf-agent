// Chat API 模块：封装 /api/chat 和 /api/chat/stream 调用逻辑。
// 流式接口使用 fetch + ReadableStream 手动解析 SSE，不使用 EventSource。

const API_BASE = '/api'

// 开发环境 delta 调试日志开关。
const DEBUG_STREAM = import.meta.env.DEV

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
    const err = await res.json().catch(() => ({ message: 'Chat request failed' }))
    throw new Error(err.message || 'Chat request failed')
  }
  const data = await res.json()
  return data.reply
}

/**
 * 流式聊天回调接口。
 */
export interface StreamCallbacks {
  /** 收到 message_delta 事件时调用，参数为增量文本内容 */
  onDelta: (content: string) => void | Promise<void>
  /** 收到 done 事件或流正常结束时调用 */
  onDone: () => void | Promise<void>
  /** 请求或解析出错时调用 */
  onError: (error: Error) => void | Promise<void>
}

/**
 * 流式聊天请求
 * POST /api/chat/stream
 *
 * 使用 fetch + ReadableStream 手动解析 SSE 事件流。
 * 支持 AbortController 用于中途停止生成。
 */
export async function streamChatMessage(
  messages: { role: string; content: string }[],
  callbacks: StreamCallbacks,
  signal?: AbortSignal
): Promise<void> {
  // 开发态日志：标记本次 stream 请求开始
  if (DEBUG_STREAM) {
    console.log('[stream] request start', new Date().toISOString())
  }

  try {
    const res = await fetch(`${API_BASE}/chat/stream`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ messages }),
      signal,
    })

    if (!res.ok) {
      const err = await res.json().catch(() => ({ message: 'Stream request failed' }))
      await callbacks.onError(new Error(err.message || `HTTP ${res.status}`))
      return
    }

    const reader = res.body?.getReader()
    if (!reader) {
      await callbacks.onError(new Error('Response body is not readable'))
      return
    }

    const decoder = new TextDecoder()
    let buffer = ''

    // SSE 持续读到流关闭
    while (true) {
      const { done, value } = await reader.read()
      if (done) {
        if (DEBUG_STREAM) {
          console.log('[stream] reader done', new Date().toISOString())
        }
        break
      }

      buffer += decoder.decode(value, { stream: true })

      // 兼容 LF/CRLF 两种换行格式，避免浏览器或代理改写后无法及时切分事件。
      const parts = buffer.split(/\r?\n\r?\n/)
      // 最后一段可能不完整，留到下次处理
      buffer = parts.pop() || ''

      for (const part of parts) {
        if (!part.trim()) continue
        const parsed = parseSSEEvent(part)
        if (!parsed) continue

        if (parsed.event === 'message_delta') {
          // data: {"content": "..."}
          const content = parsed.data?.content
          if (typeof content === 'string' && content.length > 0) {
            if (DEBUG_STREAM) {
              const preview = content.length > 30 ? content.slice(0, 30) + '…' : content
              console.log(`[stream] delta ${new Date().toISOString()} len=${content.length} content=${JSON.stringify(preview)}`)
            }
            await callbacks.onDelta(content)
          }
        } else if (parsed.event === 'error') {
          const msg = parsed.data?.message || 'Stream error'
          await callbacks.onError(new Error(msg))
          return
        } else if (parsed.event === 'done') {
          if (DEBUG_STREAM) {
            console.log('[stream] done event', new Date().toISOString())
          }
          await callbacks.onDone()
          return
        }
        // skill_used、citation 等事件当前不做 UI 展示，静默跳过
      }
    }

    const tail = decoder.decode()
    if (tail) {
      buffer += tail
    }
    if (buffer.trim()) {
      const parsed = parseSSEEvent(buffer)
      if (parsed?.event === 'message_delta') {
        const content = parsed.data?.content
        if (typeof content === 'string' && content.length > 0) {
          await callbacks.onDelta(content)
        }
      }
    }

    // 流结束但未收到 done 事件，仍视为正常结束
    await callbacks.onDone()
  } catch (err: any) {
    if (err.name === 'AbortError') {
      if (DEBUG_STREAM) {
        console.log('[stream] aborted', new Date().toISOString())
      }
      await callbacks.onDone()
      return
    }
    await callbacks.onError(err instanceof Error ? err : new Error(String(err)))
  }
}

/**
 * 解析单个 SSE 事件块，提取 event 类型和 data JSON。
 */
function parseSSEEvent(text: string): { event: string; data: any } | null {
  let eventType = ''
  const dataLines: string[] = []

  for (const line of text.split(/\r?\n/)) {
    if (line.startsWith('event: ')) {
      eventType = line.slice(7).trim()
    } else if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trimStart())
    }
  }

  if (!eventType) return null

  const dataStr = dataLines.join('\n')
  let data: any = null
  if (dataStr) {
    try {
      data = JSON.parse(dataStr)
    } catch {
      data = dataStr
    }
  }

  return { event: eventType, data }
}
