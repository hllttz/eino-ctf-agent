package prompt

import (
	"fmt"
	"strings"

	"eino_ctf_agent/internal/skill"
)

const agentSystemIntro = `你是一个具备工具调用能力的知识库问答助手。你可以使用以下工具：

- knowledge_search：检索本地知识库中的文档内容。当用户的问题可能涉及已上传的文档时，主动调用此工具搜索。
- skill_reader：读取指定 skill 的完整方法论和操作步骤。当你需要某个 skill 的详细解题步骤时，调用此工具获取完整内容。

使用原则：先用 knowledge_search 查事实依据，如果上下文提示需要某个 skill 的详细方法，再用 skill_reader 获取步骤。如果检索结果不足以回答问题，如实说明。回答要清晰、可操作。`

// BuildAgentSystemPrompt 构建 Agent 模式的 system prompt，注入已匹配的 skills 作为解题方法论。
func BuildAgentSystemPrompt(activeSkills []skill.Skill) string {
	var b strings.Builder
	b.WriteString(agentSystemIntro)

	if len(activeSkills) == 0 {
		return b.String()
	}

	b.WriteString("\n\n[Active Skills]\n")
	b.WriteString("以下 Skills 是任务方法论和操作流程，只能作为解题步骤指导，不等同于知识库事实依据。\n\n")
	for _, s := range activeSkills {
		b.WriteString(fmt.Sprintf(
			"<skill name=%q title=%q priority=%d>\nDescription: %s\n%s\n</skill>\n\n",
			s.Name,
			s.Title,
			s.Priority,
			s.Description,
			trimSkillBody(s.Body, s.MaxTokens),
		))
	}
	return b.String()
}
