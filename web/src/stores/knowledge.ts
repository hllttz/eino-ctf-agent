// TODO Phase 5.5: Knowledge Store (Pinia)
// 管理文档列表、上传状态。

import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Document {
  id: string
  filename: string
  file_type: string
  status: string
  chunk_count: number
  error_message?: string
}

export const useKnowledgeStore = defineStore('knowledge', () => {
  const documents = ref<Document[]>([])
  const uploading = ref(false)

  return {
    documents,
    uploading,
  }
})
