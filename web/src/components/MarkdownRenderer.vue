<template>
  <!-- TODO Phase 2.5: Markdown 渲染组件 -->
  <div class="markdown-renderer" v-html="renderedContent"></div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  content: string
}>()

// TODO Phase 2.5: 接入 markdown-it 或 marked 库
const renderedContent = computed(() => props.content)
</script>

<style scoped>
/* TODO Phase 2.5: Markdown 渲染样式，支持代码高亮 */
</style>
