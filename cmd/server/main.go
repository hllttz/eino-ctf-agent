package main

import (
	"context"
	"flag"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"eino_ctf_agent/internal/config"
	"eino_ctf_agent/internal/embedding"
	"eino_ctf_agent/internal/handler"
	"eino_ctf_agent/internal/knowledge"
	"eino_ctf_agent/internal/llm"
	"eino_ctf_agent/internal/middleware"
	"eino_ctf_agent/internal/router"
	"eino_ctf_agent/internal/service"
	"eino_ctf_agent/internal/skill"
	"eino_ctf_agent/internal/tool"
)

func main() {
	configPath := flag.String("config", "", "path to config.yaml")
	flag.Parse()

	// 从 .env 中读取关键参数
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] .env file not found, using system environment variables")
	}

	// 配置出错直接退出，避免后面组件用无效值初始化。
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] failed to load config: %v", err)
	}

	ctx := context.Background()

	// 两个 ChatModel：一个普通对话用，一个给 Agent 做工具调用用。
	// Eino React Agent 必须用 ToolCallingChatModel，不能混用。
	chatModel, err := llm.NewChatModel(ctx, cfg)
	if err != nil {
		log.Fatalf("[FATAL] failed to create chat model: %v", err)
	}

	toolCallingModel, err := llm.NewToolCallingChatModel(ctx, cfg)
	if err != nil {
		log.Fatalf("[FATAL] failed to create tool-calling chat model: %v", err)
	}

	// 向量化模型：文档入库时做 embedding，检索时把 query 转成向量。
	embedder, err := embedding.NewEmbedder(ctx, cfg)
	if err != nil {
		log.Fatalf("[FATAL] failed to create embedder: %v", err)
	}

	// Redis：先连上再建向量索引。Indexer 管写，Retriever 管查。
	redisClient := knowledge.NewRedisClient(cfg)
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("[FATAL] failed to connect redis: %v", err)
	}
	if err := knowledge.EnsureVectorIndex(ctx, redisClient, cfg); err != nil {
		log.Fatalf("[FATAL] failed to prepare redis vector index: %v", err)
	}

	knowledgeIndexer, err := knowledge.NewRedisIndexer(ctx, redisClient, cfg, embedder)
	if err != nil {
		log.Fatalf("[FATAL] failed to create redis indexer: %v", err)
	}
	knowledgeRetriever, err := knowledge.NewRedisRetriever(ctx, redisClient, cfg, embedder)
	if err != nil {
		log.Fatalf("[FATAL] failed to create redis retriever: %v", err)
	}
	documentRepo := knowledge.NewMetadataRepo(redisClient, cfg)
	knowledgeService := knowledge.NewService(cfg, redisClient, knowledgeIndexer, documentRepo)

	// Skills：从本地 .md 文件加载，靠关键词匹配，不走向量检索。
	skillRegistry, err := skill.NewRegistry(skill.NewLoader(cfg.Storage.SkillsDir))
	if err != nil {
		log.Fatalf("[FATAL] failed to load skills: %v", err)
	}
	skillRouter := skill.NewRouter(skillRegistry, cfg.Skills.MaxActiveSkills)
	skillService := service.NewSkillService(skillRegistry)

	// Agent 工具：注册后 React Agent 才能在推理时调用。
	toolRegistry := tool.NewRegistry()
	knowledgeSearchTool, err := tool.NewKnowledgeSearchTool(knowledgeRetriever, cfg.RAG.TopK, cfg.RAG.ScoreThreshold)
	if err != nil {
		log.Fatalf("[FATAL] failed to create knowledge_search tool: %v", err)
	}
	toolRegistry.Register("knowledge_search", knowledgeSearchTool)

	skillReaderTool, err := tool.NewSkillReaderTool(skillRegistry)
	if err != nil {
		log.Fatalf("[FATAL] failed to create skill_reader tool: %v", err)
	}
	toolRegistry.Register("skill_reader", skillReaderTool)

	// Phase 5 CTF 本地分析工具：设置工作目录并注册 5 个新工具。
	if err := tool.SetWorkDir(cfg.Storage.DocsDir); err != nil {
		log.Fatalf("[FATAL] failed to set CTF work dir: %v", err)
	}

	fileInfoTool, err := tool.NewFileInfoTool()
	if err != nil {
		log.Fatalf("[FATAL] failed to create file_info tool: %v", err)
	}
	toolRegistry.Register("file_info", fileInfoTool)

	fileReaderTool, err := tool.NewFileReaderTool()
	if err != nil {
		log.Fatalf("[FATAL] failed to create file_reader tool: %v", err)
	}
	toolRegistry.Register("file_reader", fileReaderTool)

	commandExecutorTool, err := tool.NewCommandExecutorTool()
	if err != nil {
		log.Fatalf("[FATAL] failed to create command_executor tool: %v", err)
	}
	toolRegistry.Register("command_executor", commandExecutorTool)

	pythonRunnerTool, err := tool.NewPythonRunnerTool()
	if err != nil {
		log.Fatalf("[FATAL] failed to create python_runner tool: %v", err)
	}
	toolRegistry.Register("python_runner", pythonRunnerTool)

	encodingDecoderTool, err := tool.NewEncodingDecoderTool()
	if err != nil {
		log.Fatalf("[FATAL] failed to create encoding_decoder tool: %v", err)
	}
	toolRegistry.Register("encoding_decoder", encodingDecoderTool)

	// Phase 6 IDA MCP 只读分析工具。
	// endpoint 非法时记录 warning 并注册 disabled client，不阻止服务启动。
	idaEndpoint := tool.EnvIDAEndpoint()
	idaTimeout := tool.EnvIDATimeout()
	var idaClient tool.IDAMCPClient
	if realClient, err := tool.NewRealMCPClient(idaEndpoint, idaTimeout); err != nil {
		log.Printf("[WARN] IDA MCP endpoint invalid (%s): %v — IDA tools will be unavailable", idaEndpoint, err)
		idaClient = tool.NewDisabledMCPClient(err.Error())
	} else {
		idaClient = realClient
		log.Printf("[INFO] IDA MCP client configured: endpoint=%s timeout=%ds", idaEndpoint, idaTimeout)
	}
	tool.SetIDAClient(idaClient)

	idaStatusTool, err := tool.NewIDAStatusTool()
	if err != nil {
		log.Fatalf("[FATAL] failed to create ida_status tool: %v", err)
	}
	toolRegistry.Register("ida_status", idaStatusTool)

	idaFunctionsTool, err := tool.NewIDAFunctionsTool()
	if err != nil {
		log.Fatalf("[FATAL] failed to create ida_functions tool: %v", err)
	}
	toolRegistry.Register("ida_functions", idaFunctionsTool)

	idaDecompileTool, err := tool.NewIDADecompileTool()
	if err != nil {
		log.Fatalf("[FATAL] failed to create ida_decompile tool: %v", err)
	}
	toolRegistry.Register("ida_decompile", idaDecompileTool)

	idaStringsTool, err := tool.NewIDAStringsTool()
	if err != nil {
		log.Fatalf("[FATAL] failed to create ida_strings tool: %v", err)
	}
	toolRegistry.Register("ida_strings", idaStringsTool)

	idaXrefsTool, err := tool.NewIDAXrefsTool()
	if err != nil {
		log.Fatalf("[FATAL] failed to create ida_xrefs tool: %v", err)
	}
	toolRegistry.Register("ida_xrefs", idaXrefsTool)

	// 组装服务：ragService 处理简单 RAG，chatService 按配置自动选 simple_rag 或 react 模式。
	ragService := service.NewRAGService(cfg, chatModel, knowledgeRetriever, skillRouter)
	chatService := service.NewChatService(cfg, chatModel, toolCallingModel, ragService, skillRouter, toolRegistry)
	if err := chatService.InitAgent(); err != nil {
		log.Printf("[WARN] react agent not available: %v (agent.mode=%s)", err, cfg.Agent.Mode)
	}

	// Handler：只做请求解析和响应写入，业务逻辑在 Service 层。
	chatHandler := handler.NewChatHandler(chatService)
	knowledgeHandler := handler.NewKnowledgeHandler(knowledgeService)
	skillHandler := handler.NewSkillHandler(skillService)

	// gin.New() 不带默认中间件，由项目自己控制。
	// 32MB 的 multipart 上限
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery(), middleware.CORS(cfg.Server.CORS.AllowOrigins))
	engine.MaxMultipartMemory = 32 << 20

	router.Setup(engine, chatHandler, knowledgeHandler, skillHandler)

	log.Printf("[INFO] starting server at %s", cfg.Addr())
	if err := engine.Run(cfg.Addr()); err != nil {
		log.Fatalf("[FATAL] server failed: %v", err)
	}
}
