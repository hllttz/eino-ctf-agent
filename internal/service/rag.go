package service

import (
	"context"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"eino_ctf_agent/internal/config"
	appmodel "eino_ctf_agent/internal/model"
	"eino_ctf_agent/internal/prompt"
	"eino_ctf_agent/internal/retriever"
	"eino_ctf_agent/internal/skill"
	"eino_ctf_agent/internal/vectorstore"
)

type RAGService struct {
	cfg         *config.Config
	chatModel   einomodel.BaseChatModel
	retriever   *retriever.KnowledgeRetriever
	skillRouter *skill.Router
}

func NewRAGService(
	cfg *config.Config,
	chatModel einomodel.BaseChatModel,
	retriever *retriever.KnowledgeRetriever,
	skillRouter *skill.Router,
) *RAGService {
	return &RAGService{
		cfg:         cfg,
		chatModel:   chatModel,
		retriever:   retriever,
		skillRouter: skillRouter,
	}
}

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
	query := lastUserMessage(req.Messages)
	results, err := s.retriever.Retrieve(ctx, query, vectorstore.SearchOptions{
		TopK:           s.cfg.RAG.TopK,
		ScoreThreshold: s.cfg.RAG.ScoreThreshold,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	matchedSkills := s.matchSkills(query)
	activeSkills := toSkillRefs(matchedSkills)

	citations := make([]appmodel.Citation, 0, len(results))
	for _, result := range results {
		citations = append(citations, appmodel.Citation{
			DocumentID: result.Record.DocumentID,
			Filename:   result.Record.Filename,
			ChunkIndex: result.Record.ChunkIndex,
			Score:      result.Score,
			PageNumber: result.Record.PageNumber,
		})
	}

	messages := make([]*schema.Message, 0, len(req.Messages)+1)
	messages = append(messages, schema.SystemMessage(prompt.BuildRAGSystemPrompt(results, matchedSkills, s.cfg.RAG.MaxContextChunks)))
	for _, msg := range req.Messages {
		messages = append(messages, &schema.Message{
			Role:    toSchemaRole(msg.Role),
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

func lastUserMessage(messages []appmodel.ChatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	if len(messages) == 0 {
		return ""
	}
	return messages[len(messages)-1].Content
}
