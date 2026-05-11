package tool

import (
	"context"
	"fmt"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"eino_ctf_agent/internal/pkg/security"
	"eino_ctf_agent/internal/skill"
)

// SkillReader 技能读取工具，按名称读取技能的完整内容。
// 保留原有结构以兼容已有调用方。
type SkillReader struct {
	registry *skill.Registry
}

// NewSkillReader 创建技能读取器实例。
func NewSkillReader(registry *skill.Registry) *SkillReader {
	return &SkillReader{registry: registry}
}

// Read 按名称读取技能完整 body，校验 skill_name 合法性。
func (r *SkillReader) Read(skillName string) (string, error) {
	if !security.ValidSkillName(skillName) {
		return "", fmt.Errorf("invalid skill name")
	}
	s, ok := r.registry.GetByName(skillName)
	if !ok {
		return "", fmt.Errorf("skill not found: %s", skillName)
	}
	if !s.Enabled {
		return "", fmt.Errorf("skill is disabled: %s", skillName)
	}
	return s.Body, nil
}

// SkillReaderInput 技能读取工具的输入参数。
type SkillReaderInput struct {
	SkillName string `json:"skill_name" jsonschema:"description=name of the skill to read full content for"`
}

// SkillReaderOutput 技能读取工具的输出结果。
type SkillReaderOutput struct {
	Content string `json:"content" jsonschema:"description=full body content of the requested skill"`
}

// NewSkillReaderTool 创建适配 Eino InvokableTool 接口的技能读取工具。
// 内部复用 SkillReader.Read 的校验和读取逻辑。
func NewSkillReaderTool(registry *skill.Registry) (einotool.InvokableTool, error) {
	reader := NewSkillReader(registry)
	return utils.InferTool[SkillReaderInput, SkillReaderOutput](
		"skill_reader",
		"Read the full content of a skill by its name. "+
			"Use this when you need detailed step-by-step methodology from a skill. "+
			"The skill name must match exactly as listed in the available skills.",
		func(ctx context.Context, input SkillReaderInput) (SkillReaderOutput, error) {
			body, err := reader.Read(input.SkillName)
			if err != nil {
				return SkillReaderOutput{}, err
			}
			return SkillReaderOutput{Content: body}, nil
		},
	)
}
