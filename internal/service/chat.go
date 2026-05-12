package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"sync"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"eino_ctf_agent/internal/config"
	appmodel "eino_ctf_agent/internal/model"
	"eino_ctf_agent/internal/prompt"
	"eino_ctf_agent/internal/skill"
	"eino_ctf_agent/internal/tool"
)

type traceIDKey struct{}

// ContextWithTraceID 将 traceID 注入 context，供 handler 传入 X-Request-ID。
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// TraceIDFromContext 从 context 中提取 traceID。返回空字符串表示不存在。
func TraceIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(traceIDKey{}).(string)
	return v, ok
}

// ChatService 聊天服务，编排LLM调用与可选的RAG/Agent链路。
type ChatService struct {
	cfg         *config.Config
	chatModel   einomodel.BaseChatModel
	toolCalling einomodel.ToolCallingChatModel
	ragService  *RAGService
	skillRouter *skill.Router
	toolReg     *tool.Registry

	reactOnce  sync.Once
	reactAgent *react.Agent
	reactErr   error
}

// ChatStream 聊天流式响应封装，包含消息流读取器和预检索的引用与技能。
type ChatStream struct {
	Reader    *schema.StreamReader[*schema.Message]
	Citations []appmodel.Citation
	Skills    []appmodel.SkillRef
}

// NewChatService 创建聊天服务实例。
func NewChatService(
	cfg *config.Config,
	chatModel einomodel.BaseChatModel,
	toolCalling einomodel.ToolCallingChatModel,
	ragService *RAGService,
	skillRouter *skill.Router,
	toolReg *tool.Registry,
) *ChatService {
	return &ChatService{
		cfg:         cfg,
		chatModel:   chatModel,
		toolCalling: toolCalling,
		ragService:  ragService,
		skillRouter: skillRouter,
		toolReg:     toolReg,
	}
}

// Chat 处理聊天请求，按 agent.mode 分流到不同生成链路。
func (s *ChatService) Chat(ctx context.Context, req *appmodel.ChatRequest) (*appmodel.ChatResponse, error) {
	if err := validateChatRequest(req); err != nil {
		return nil, err
	}
	traceID, ok := TraceIDFromContext(ctx)
	if !ok {
		traceID = newTraceID()
	}
	log.Printf("[TRACE] %s Chat mode=%s messages=%d conversation=%q", traceID, s.cfg.Agent.Mode, len(req.Messages), req.ConversationID)
	if s.cfg.Agent.Mode == "react" {
		return s.reactChat(ctx, req, traceID)
	}
	if s.ragService != nil {
		return s.ragService.Generate(ctx, req)
	}

	messages := appmodel.ToSchemaMessages(req.Messages)
	resp, err := s.chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM generate failed: %w", err)
	}
	return &appmodel.ChatResponse{Reply: resp.Content}, nil
}

// Stream 处理流式聊天请求，按 agent.mode 分流。
func (s *ChatService) Stream(ctx context.Context, req *appmodel.ChatRequest) (*ChatStream, error) {
	if err := validateChatRequest(req); err != nil {
		return nil, err
	}
	traceID, ok := TraceIDFromContext(ctx)
	if !ok {
		traceID = newTraceID()
	}
	log.Printf("[TRACE] %s Stream mode=%s messages=%d conversation=%q", traceID, s.cfg.Agent.Mode, len(req.Messages), req.ConversationID)
	if s.cfg.Agent.Mode == "react" {
		return s.reactStream(ctx, req, traceID)
	}
	if s.ragService != nil {
		return s.ragService.Stream(ctx, req)
	}

	reader, err := s.chatModel.Stream(ctx, appmodel.ToSchemaMessages(req.Messages))
	if err != nil {
		return nil, fmt.Errorf("LLM stream failed: %w", err)
	}
	return &ChatStream{Reader: reader}, nil
}

func (s *ChatService) reactChat(ctx context.Context, req *appmodel.ChatRequest, traceID string) (*appmodel.ChatResponse, error) {
	agent, err := s.getReactAgent()
	if err != nil {
		log.Printf("[TRACE] %s reactChat error: agent not available: %v", traceID, err)
		return nil, err
	}
	log.Printf("[TRACE] %s reactChat start", traceID)
	messages := s.buildAgentInput(ctx, req)
	resp, err := agent.Generate(ctx, messages)
	if err != nil {
		log.Printf("[TRACE] %s reactChat error: %v", traceID, err)
		return nil, fmt.Errorf("react agent generate failed: %w", err)
	}
	log.Printf("[TRACE] %s reactChat done reply=%dchars", traceID, len(resp.Content))
	return &appmodel.ChatResponse{Reply: resp.Content}, nil
}

func (s *ChatService) reactStream(ctx context.Context, req *appmodel.ChatRequest, traceID string) (*ChatStream, error) {
	agent, err := s.getReactAgent()
	if err != nil {
		log.Printf("[TRACE] %s reactStream error: agent not available: %v", traceID, err)
		return nil, err
	}
	log.Printf("[TRACE] %s reactStream start", traceID)
	messages := s.buildAgentInput(ctx, req)
	reader, err := agent.Stream(ctx, messages)
	if err != nil {
		log.Printf("[TRACE] %s reactStream error: %v", traceID, err)
		return nil, fmt.Errorf("react agent stream failed: %w", err)
	}
	log.Printf("[TRACE] %s reactStream stream-opened", traceID)
	return &ChatStream{Reader: reader}, nil
}

func (s *ChatService) buildAgentInput(ctx context.Context, req *appmodel.ChatRequest) []*schema.Message {
	query := appmodel.LastUserMessage(req.Messages)
	matchedSkills := s.matchSkills(query)
	systemPrompt := prompt.BuildAgentSystemPrompt(matchedSkills)

	messages := make([]*schema.Message, 0, len(req.Messages)+1)
	messages = append(messages, schema.SystemMessage(systemPrompt))
	for _, msg := range req.Messages {
		messages = append(messages, &schema.Message{
			Role:    appmodel.ToSchemaRole(msg.Role),
			Content: msg.Content,
		})
	}
	return messages
}

func (s *ChatService) getReactAgent() (*react.Agent, error) {
	s.reactOnce.Do(func() {
		tools := s.toolReg.All()
		if len(tools) == 0 {
			s.reactErr = fmt.Errorf("no tools registered for react agent")
			log.Printf("[ERROR] react agent init failed: %v", s.reactErr)
			return
		}
		log.Printf("[INFO] initializing react agent with %d tools", len(tools))
		s.reactAgent, s.reactErr = react.NewAgent(context.Background(), &react.AgentConfig{
			ToolCallingModel:      s.toolCalling,
			StreamToolCallChecker: streamToolCallChecker,
			ToolsConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
			MaxStep: s.cfg.Agent.MaxSteps,
		})
		if s.reactErr != nil {
			log.Printf("[ERROR] react agent init failed: %v", s.reactErr)
		} else {
			log.Printf("[INFO] react agent initialized successfully")
		}
	})
	return s.reactAgent, s.reactErr
}

// InitAgent 触发 React Agent 的惰性初始化，应在服务启动后、接受请求前调用。
// 将初始化错误暴露在启动阶段（fail-fast），避免运行时首个请求才发现。
func (s *ChatService) InitAgent() error {
	_, err := s.getReactAgent()
	return err
}

func (s *ChatService) matchSkills(query string) []skill.Skill {
	if s.skillRouter == nil || !s.cfg.Skills.Enabled {
		return nil
	}
	return s.skillRouter.Match(query)
}

// streamToolCallChecker 遍历模型流式输出中的所有 chunk，直到发现 tool_calls 或 EOF。
// 与 Eino 默认的 firstChunkStreamToolCallChecker 不同，此 checker 不因遇到 content 文本而提前终止，
// 兼容 DeepSeek thinking 模式下首 chunk 不含 tool_calls 的场景。
func streamToolCallChecker(ctx context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
	defer sr.Close()
	for {
		msg, err := sr.Recv()
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if len(msg.ToolCalls) > 0 {
			return true, nil
		}
	}
}

// newTraceID 生成 16 字符的 hex 编码追踪 ID，用于关联单次请求的所有日志。
func newTraceID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(buf[:])
}

func validateChatRequest(req *appmodel.ChatRequest) error {
	if req == nil || len(req.Messages) == 0 {
		return fmt.Errorf("messages are required")
	}
	for i, msg := range req.Messages {
		if msg.Content == "" {
			return fmt.Errorf("message %d content is empty", i)
		}
	}
	return nil
}
