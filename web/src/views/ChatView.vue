<template>
  <div class="chat-view">
    <!-- 消息列表 -->
    <div class="message-list" ref="messageListRef">
      <div v-for="(msg, idx) in store.messages" :key="idx" :class="['message', msg.role]">
        <div class="role-label">{{ msg.role === 'user' ? 'You' : 'Assistant' }}</div>
        <MessageRenderer :content="msg.content" />
      </div>

      <!-- 加载指示器 -->
      <div v-if="store.loading" class="message assistant loading">
        <div class="role-label">Assistant</div>
        <div class="typing-indicator">...</div>
      </div>

      <!-- 错误提示 -->
      <div v-if="errorMsg" class="error-banner">
        {{ errorMsg }}
        <button class="dismiss-btn" @click="errorMsg = ''">x</button>
      </div>
    </div>

    <!-- 输入区域 -->
    <div class="input-area">
      <textarea
        v-model="inputText"
        :disabled="store.loading"
        placeholder="输入消息..."
        rows="2"
        @keydown.enter.exact.prevent="handleSend"
      />
      <div class="button-row">
        <button class="send-btn" :disabled="!canSend" @click="handleSend">
          发送
        </button>
        <button v-if="store.loading" class="stop-btn" @click="handleStop">
          停止
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import { useChatStore, type Message } from '../stores/chat'
import { streamChatMessage } from '../api/chat'
import MessageRenderer from '../components/MessageRenderer.vue'

const store = useChatStore()
const inputText = ref('')
const errorMsg = ref('')
const messageListRef = ref<HTMLElement>()

let abortController: AbortController | null = null

const canSend = computed(() => inputText.value.trim().length > 0 && !store.loading)

function handleSend() {
  if (!canSend.value) return

  const content = inputText.value.trim()
  inputText.value = ''
  errorMsg.value = ''

  // 添加用户消息
  const userMsg: Message = { role: 'user', content }
  store.addMessage(userMsg)

  // 创建助手消息占位
  const assistantMsg: Message = { role: 'assistant', content: '' }
  store.addMessage(assistantMsg)
  const msgIndex = store.messages.length - 1

  store.loading = true
  abortController = new AbortController()

  // 构建消息历史（流式接口需要完整 messages 数组）
  const apiMessages = store.messages.slice(0, -1).map(m => ({
    role: m.role,
    content: m.content,
  }))

  streamChatMessage(
    apiMessages,
    {
      async onDelta(content: string) {
        // 替换数组元素而非直接修改属性，确保 Vue 响应式系统检测到变更
        store.messages[msgIndex] = {
          ...store.messages[msgIndex],
          content: store.messages[msgIndex].content + content,
        }
        // 等待 Vue 完成 DOM patch，再让浏览器过一帧，确保增量内容真正可见。
        await nextTick()
        await waitForPaint()
        scrollToBottom()
      },
      onDone() {
        store.loading = false
        abortController = null
        // 如果助手消息为空（异常情况），给个提示
        if (!store.messages[msgIndex].content) {
          store.messages[msgIndex].content = '(empty response)'
        }
      },
      onError(err: Error) {
        errorMsg.value = err.message || '请求失败，请重试'
        store.loading = false
        abortController = null
        if (!store.messages[msgIndex].content) {
          store.messages[msgIndex].content = '(error)'
        }
      },
    },
    abortController.signal
  )
}

function handleStop() {
  if (abortController) {
    abortController.abort()
    abortController = null
  }
}

function scrollToBottom() {
  nextTick(() => {
    if (messageListRef.value) {
      messageListRef.value.scrollTop = messageListRef.value.scrollHeight
    }
  })
}

function waitForPaint() {
  return new Promise<void>((resolve) => {
    requestAnimationFrame(() => resolve())
  })
}
</script>

<style scoped>
.chat-view {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 64px);
  max-width: 800px;
  margin: 0 auto;
  padding: 16px;
}

.message-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}

.message {
  margin-bottom: 16px;
  padding: 12px;
  border-radius: 8px;
}

.message.user {
  background: #e8f0fe;
}

.message.assistant {
  background: #f5f5f5;
}

.role-label {
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 4px;
  color: #555;
}

.typing-indicator {
  color: #999;
  font-style: italic;
}

.error-banner {
  background: #fdecea;
  color: #b71c1c;
  padding: 8px 12px;
  border-radius: 6px;
  margin: 8px 0;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
}

.dismiss-btn {
  background: none;
  border: none;
  color: #b71c1c;
  cursor: pointer;
  font-size: 16px;
  padding: 0 4px;
}

.input-area {
  border-top: 1px solid #e0e0e0;
  padding-top: 12px;
}

.input-area textarea {
  width: 100%;
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
  resize: none;
  font-family: inherit;
  box-sizing: border-box;
}

.input-area textarea:disabled {
  background: #fafafa;
}

.button-row {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}

.send-btn {
  padding: 8px 24px;
  background: #1976d2;
  color: #fff;
  border: none;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
}

.send-btn:disabled {
  background: #90caf9;
  cursor: not-allowed;
}

.stop-btn {
  padding: 8px 16px;
  background: #c62828;
  color: #fff;
  border: none;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
}
</style>
