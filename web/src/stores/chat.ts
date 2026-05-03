// TODO Phase 2.5: Chat Store (Pinia)
// 管理消息列表、会话状态、加载状态。

import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Message {
  role: 'user' | 'assistant' | 'system'
  content: string
}

export const useChatStore = defineStore('chat', () => {
  const messages = ref<Message[]>([])
  const loading = ref(false)

  function addMessage(msg: Message) {
    messages.value.push(msg)
  }

  function clearMessages() {
    messages.value = []
  }

  return {
    messages,
    loading,
    addMessage,
    clearMessages,
  }
})
