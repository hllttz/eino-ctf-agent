package service

import (
	"context"

	einomodel "github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"eino_ctf_agent/internal/config"
	"eino_ctf_agent/internal/knowledge"
	appmodel "eino_ctf_agent/internal/model"
	"eino_ctf_agent/internal/prompt"
	"eino_ctf_agent/internal/skill"
)

// RAGService RAG 问答服务，编排检索→技能匹配→提示词构建→LLM 生成全链路。
type RAGService struct {
	cfg         *config.Config
	chatModel   einomodel.BaseChatModel
	retriever   einoretriever.Retriever
	skillRouter *skill.Router
}

// NewRAGService 创建 RAG 服务实例，接收 ChatModel、Retriever 和 SkillRouter 依赖。
func NewRAGService(
	cfg *config.Config,
	chatModel einomodel.BaseChatModel,
	retriever einoretriever.Retriever,
	skillRouter *skill.Router,
) *RAGService {
	return &RAGService{
		cfg:         cfg,
		chatModel:   chatModel,
		retriever:   retriever,
		skillRouter: skillRouter,
	}
}

// Generate 执行 RAG 问答，返回完整回复及引用和技能信息。
func (s *RAGService) Generate(ctx context.Context, req *appmodel.ChatRequest) (*appmodel.ChatResponse, error) {
	messages, citations, activeSkills, err := s.buildMessages(ctx, req)
	if err != nil {
		return nil, err
	}
	resp, err := s.chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, err
	}
	return &appmodel.ChatResponse{
		Reply:     resp.Content,
		Citations: citations,
		Skills:    activeSkills,
	}, nil
}

// Stream 执行 RAG 流式问答，返回消息流及引用和技能信息。
func (s *RAGService) Stream(ctx context.Context, req *appmodel.ChatRequest) (*ChatStream, error) {
	messages, citations, activeSkills, err := s.buildMessages(ctx, req)
	if err != nil {
		return nil, err
	}
	reader, err := s.chatModel.Stream(ctx, messages)
	if err != nil {
		return nil, err
	}
	return &ChatStream{Reader: reader, Citations: citations, Skills: activeSkills}, nil
}

func (s *RAGService) buildMessages(ctx context.Context, req *appmodel.ChatRequest) ([]*schema.Message, []appmodel.Citation, []appmodel.SkillRef, error) {
	query := appmodel.LastUserMessage(req.Messages)
	results, err := s.retriever.Retrieve(
		ctx,
		query,
		einoretriever.WithTopK(s.cfg.RAG.TopK),
		einoretriever.WithScoreThreshold(s.cfg.RAG.ScoreThreshold),
	)
	if err != nil {
		return nil, nil, nil, err
	}
	matchedSkills := s.matchSkills(query)
	activeSkills := toSkillRefs(matchedSkills)

	citations := make([]appmodel.Citation, 0, len(results))
	for _, doc := range results {
		citations = append(citations, appmodel.Citation{
			DocumentID: knowledge.MetadataString(doc, knowledge.MetaDocumentID),
			Filename:   knowledge.MetadataString(doc, knowledge.MetaFilename),
			ChunkIndex: knowledge.MetadataInt(doc, knowledge.MetaChunkIndex),
			Score:      doc.Score(),
			PageNumber: knowledge.MetadataInt(doc, knowledge.MetaPageNumber),
		})
	}

	messages := make([]*schema.Message, 0, len(req.Messages)+1)
	messages = append(messages, schema.SystemMessage(prompt.BuildRAGSystemPrompt(results, matchedSkills, s.cfg.RAG.MaxContextChunks)))
	for _, msg := range req.Messages {
		messages = append(messages, &schema.Message{
			Role:    appmodel.ToSchemaRole(msg.Role),
			Content: msg.Content,
		})
	}
	return messages, citations, activeSkills, nil
}

func (s *RAGService) matchSkills(query string) []skill.Skill {
	if s.skillRouter == nil || !s.cfg.Skills.Enabled {
		return nil
	}
	return s.skillRouter.Match(query)
}

func toSkillRefs(skills []skill.Skill) []appmodel.SkillRef {
	if len(skills) == 0 {
		return nil
	}
	refs := make([]appmodel.SkillRef, 0, len(skills))
	for _, s := range skills {
		refs = append(refs, appmodel.SkillRef{
			Name:        s.Name,
			Title:       s.Title,
			Description: s.Description,
		})
	}
	return refs
}
