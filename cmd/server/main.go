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
	"eino_ctf_agent/internal/llm"
	"eino_ctf_agent/internal/middleware"
	"eino_ctf_agent/internal/retriever"
	"eino_ctf_agent/internal/router"
	"eino_ctf_agent/internal/service"
	"eino_ctf_agent/internal/skill"
	"eino_ctf_agent/internal/store"
	"eino_ctf_agent/internal/vectorstore"
)

func main() {
	configPath := flag.String("config", "", "path to config.yaml")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] .env file not found, using system environment variables")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] failed to load config: %v", err)
	}

	ctx := context.Background()

	chatModel, err := llm.NewChatModel(ctx, cfg)
	if err != nil {
		log.Fatalf("[FATAL] failed to create chat model: %v", err)
	}

	embedder, err := embedding.NewEmbedder(ctx, cfg)
	if err != nil {
		log.Fatalf("[FATAL] failed to create embedder: %v", err)
	}

	metadataDB, err := store.Open(ctx, cfg.Storage.MetadataDB)
	if err != nil {
		log.Fatalf("[FATAL] failed to open metadata db: %v", err)
	}
	defer metadataDB.Close()

	vectorStore, err := vectorstore.NewSQLiteStore(ctx, cfg.Storage.VectorDB)
	if err != nil {
		log.Fatalf("[FATAL] failed to open vector db: %v", err)
	}
	defer vectorStore.Close()

	documentRepo := store.NewDocumentRepo(metadataDB)
	chunkRepo := store.NewChunkRepo(metadataDB)
	knowledgeService := service.NewKnowledgeService(cfg, embedder, vectorStore, documentRepo, chunkRepo)

	skillRegistry, err := skill.NewRegistry(skill.NewLoader(cfg.Storage.SkillsDir))
	if err != nil {
		log.Fatalf("[FATAL] failed to load skills: %v", err)
	}
	skillRouter := skill.NewRouter(skillRegistry, cfg.Skills.MaxActiveSkills)
	skillService := service.NewSkillService(skillRegistry)

	knowledgeRetriever := retriever.NewKnowledgeRetriever(embedder, vectorStore)
	ragService := service.NewRAGService(cfg, chatModel, knowledgeRetriever, skillRouter)
	chatService := service.NewChatService(chatModel, ragService)

	chatHandler := handler.NewChatHandler(chatService)
	knowledgeHandler := handler.NewKnowledgeHandler(knowledgeService)
	skillHandler := handler.NewSkillHandler(skillService)

	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery(), middleware.CORS(cfg.Server.CORS.AllowOrigins))
	engine.MaxMultipartMemory = 32 << 20

	router.Setup(engine, chatHandler, knowledgeHandler, skillHandler)

	log.Printf("[INFO] starting server at %s", cfg.Addr())
	if err := engine.Run(cfg.Addr()); err != nil {
		log.Fatalf("[FATAL] server failed: %v", err)
	}
}
