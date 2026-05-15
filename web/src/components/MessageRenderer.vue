<template>
  <div
    class="message-renderer markdown-renderer"
    @click="handleRendererClick"
    v-html="sanitizedHtml"
  ></div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { renderSafeMarkdown } from '../utils/markdown'

const props = defineProps<{
  content: string
}>()

const sanitizedHtml = computed(() => renderSafeMarkdown(props.content))

async function handleRendererClick(event: MouseEvent) {
  const button = (event.target as Element | null)?.closest<HTMLButtonElement>('.md-code-copy')
  if (!button) return

  const codeBlock = button.closest('.md-code-block')
  const code = codeBlock?.querySelector('code')?.textContent
  if (!code) return

  try {
    await copyCodeToClipboard(code)
    showCopyStatus(button, '已复制')
  } catch {
    showCopyStatus(button, '复制失败')
  }
}

async function copyCodeToClipboard(code: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(code)
    return
  }

  const textarea = document.createElement('textarea')
  textarea.value = code
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()

  const copied = document.execCommand('copy')
  document.body.removeChild(textarea)

  if (!copied) {
    throw new Error('Copy command failed')
  }
}

function showCopyStatus(button: HTMLButtonElement, label: string) {
  button.textContent = label
  window.setTimeout(() => {
    button.textContent = '复制'
  }, 1200)
}
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

.message-renderer :deep(.md-code-block) {
  overflow: hidden;
  margin: 0 0 0.9em;
  border: 1px solid #d0d7de;
  border-radius: 8px;
  background: #f6f8fa;
  box-shadow: 0 1px 2px rgba(27, 31, 36, 0.06);
}

.message-renderer :deep(.md-code-header) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 34px;
  padding: 6px 10px;
  border-bottom: 1px solid #d0d7de;
  background: #eef2f6;
}

.message-renderer :deep(.md-code-lang) {
  overflow: hidden;
  color: #57606a;
  font-family: ui-monospace, SFMono-Regular, Consolas, 'Liberation Mono', monospace;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
}

.message-renderer :deep(.md-code-copy) {
  flex: 0 0 auto;
  padding: 3px 9px;
  border: 1px solid #afb8c1;
  border-radius: 999px;
  background: #fff;
  color: #24292f;
  font-size: 12px;
  line-height: 1.5;
  cursor: pointer;
}

.message-renderer :deep(.md-code-copy:hover) {
  border-color: #8c959f;
  background: #f6f8fa;
}

.message-renderer :deep(.md-code-block pre) {
  overflow-x: auto;
  margin: 0;
  padding: 12px;
  background: #0f172a;
}

.message-renderer :deep(.md-code-block pre code) {
  display: block;
  padding: 0;
  color: #e5edf7;
  background: transparent;
  white-space: pre;
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
