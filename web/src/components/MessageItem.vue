<template>
  <!-- TODO Phase 2.5: 单条消息展示组件 -->
  <div class="message-item">
    <span class="role">{{ role }}</span>
    <div class="content">{{ content }}</div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  role: string
  content: string
}>()
</script>

<style scoped>
/* TODO Phase 2.5 样式 */
</style>
