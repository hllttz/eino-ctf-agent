// TODO Phase 5.5: Knowledge API 模块
// 封装知识库管理接口：上传、列表、删除。

const API_BASE = '/api'

/**
 * 上传文档
 * POST /api/knowledge/upload
 */
export async function uploadDocument(_file: File): Promise<any> {
  // TODO Phase 5.5
}

/**
 * 获取文档列表
 * GET /api/knowledge/documents
 */
export async function listDocuments(): Promise<any[]> {
  const res = await fetch(`${API_BASE}/knowledge/documents`)
  if (!res.ok) throw new Error('Failed to list documents')
  return res.json()
}

/**
 * 删除文档
 * DELETE /api/knowledge/documents/:id
 */
export async function deleteDocument(_id: string): Promise<void> {
  // TODO Phase 5.5
}
