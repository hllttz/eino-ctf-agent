<template>
  <div class="message-renderer markdown-renderer" v-html="sanitizedHtml"></div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { renderSafeMarkdown } from '../utils/markdown'

const props = defineProps<{
  content: string
}>()

const sanitizedHtml = computed(() => renderSafeMarkdown(props.content))
</script>

<style scoped>
.message-renderer {
  word-break: break-word;
  line-height: 1.6;
  font-size: 14px;
}

.message-renderer :deep(:first-child) {
  margin-top: 0;
}

.message-renderer :deep(:last-child) {
  margin-bottom: 0;
}

.message-renderer :deep(p),
.message-renderer :deep(ul),
.message-renderer :deep(ol),
.message-renderer :deep(blockquote),
.message-renderer :deep(pre),
.message-renderer :deep(table) {
  margin: 0 0 0.75em;
}

.message-renderer :deep(ul),
.message-renderer :deep(ol) {
  padding-left: 1.4em;
}

.message-renderer :deep(blockquote) {
  padding-left: 0.9em;
  color: #555;
  border-left: 3px solid #d0d7de;
}

.message-renderer :deep(code) {
  padding: 0.12em 0.35em;
  border-radius: 4px;
  background: rgba(27, 31, 36, 0.08);
  font-family: ui-monospace, SFMono-Regular, Consolas, 'Liberation Mono', monospace;
  font-size: 0.92em;
}

.message-renderer :deep(pre) {
  overflow-x: auto;
  padding: 10px 12px;
  border-radius: 6px;
  background: #1f2328;
}

.message-renderer :deep(pre code) {
  display: block;
  padding: 0;
  color: #f6f8fa;
  background: transparent;
}

.message-renderer :deep(a) {
  color: #1976d2;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.message-renderer :deep(table) {
  display: block;
  max-width: 100%;
  overflow-x: auto;
  border-collapse: collapse;
}

.message-renderer :deep(th),
.message-renderer :deep(td) {
  padding: 6px 8px;
  border: 1px solid #d0d7de;
}

.message-renderer :deep(th) {
  font-weight: 600;
  background: rgba(27, 31, 36, 0.05);
}
</style>
