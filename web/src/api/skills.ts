// TODO Phase 6: Skills API 模块
// 封装 Skills 管理接口：列表、详情、reload。

const API_BASE = '/api'

/**
 * 获取 skill 列表
 * GET /api/skills
 */
export async function listSkills(): Promise<any[]> {
  const res = await fetch(`${API_BASE}/skills`)
  if (!res.ok) throw new Error('Failed to list skills')
  return res.json()
}

/**
 * 获取 skill 详情
 * GET /api/skills/:name
 */
export async function getSkillDetail(_name: string): Promise<any> {
  // TODO Phase 6
}

/**
 * 重新加载 skills
 * POST /api/skills/reload
 */
export async function reloadSkills(): Promise<void> {
  // TODO Phase 6
}
