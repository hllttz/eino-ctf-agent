import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'
import type { RenderRule } from 'markdown-it/lib/renderer.mjs'

const markdown = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
  typographer: false,
})

const renderCodeBlock: RenderRule = (tokens, idx) => {
  const token = tokens[idx]
  const language = getCodeLanguage(token.info)
  const escapedCode = markdown.utils.escapeHtml(token.content)

  return [
    `<div class="md-code-block">`,
    `<div class="md-code-header">`,
    `<span class="md-code-lang">${markdown.utils.escapeHtml(language.label)}</span>`,
    `<button type="button" class="md-code-copy" aria-label="复制代码">复制</button>`,
    `</div>`,
    `<pre><code class="${language.className}">${escapedCode}</code></pre>`,
    `</div>`,
  ].join('')
}

markdown.renderer.rules.fence = renderCodeBlock
markdown.renderer.rules.code_block = renderCodeBlock

const sanitizeOptions = {
  ALLOWED_TAGS: [
    'a',
    'blockquote',
    'br',
    'button',
    'code',
    'del',
    'div',
    'em',
    'h1',
    'h2',
    'h3',
    'h4',
    'h5',
    'h6',
    'hr',
    'li',
    'ol',
    'p',
    'pre',
    'span',
    'strong',
    'table',
    'tbody',
    'td',
    'th',
    'thead',
    'tr',
    'ul',
  ],
  ALLOWED_ATTR: ['aria-label', 'class', 'href', 'title', 'type'],
  ALLOW_DATA_ATTR: false,
}

export function renderSafeMarkdown(content: string): string {
  const renderedHtml = markdown.render(content)
  return DOMPurify.sanitize(renderedHtml, sanitizeOptions)
}

function getCodeLanguage(info: string): { label: string; className: string } {
  const rawLanguage = info.trim().split(/\s+/)[0] || 'text'
  const classSuffix = rawLanguage.toLowerCase().replace(/[^a-z0-9_-]+/g, '-')

  return {
    label: rawLanguage,
    className: classSuffix ? `language-${classSuffix}` : 'language-text',
  }
}
