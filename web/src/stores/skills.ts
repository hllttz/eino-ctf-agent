// TODO Phase 8: Skills Store (Pinia)
// 管理 skill 列表和详情。

import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Skill {
  name: string
  description: string
  triggers: string[]
  priority: number
}

export const useSkillsStore = defineStore('skills', () => {
  const skills = ref<Skill[]>([])
  const loading = ref(false)

  return {
    skills,
    loading,
  }
})
