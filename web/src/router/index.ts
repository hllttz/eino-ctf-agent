import { createRouter, createWebHistory } from 'vue-router'
import ChatView from '../views/ChatView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'chat',
      component: ChatView,
    },
    {
      path: '/knowledge',
      name: 'knowledge',
      // 懒加载：Phase 5.5 实现
      component: () => import('../views/KnowledgeView.vue'),
    },
    {
      path: '/skills',
      name: 'skills',
      // 懒加载：Phase 8 实现
      component: () => import('../views/SkillsView.vue'),
    },
  ],
})

export default router
