package main

import (
	"context"
	"flag"
	"log"
	"os"

	"eino_ctf_agent/internal/config"
	"eino_ctf_agent/internal/handler"
	"eino_ctf_agent/internal/llm"
	"eino_ctf_agent/internal/router"
	"eino_ctf_agent/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// 命令行参数：可选指定配置文件路径
	configPath := flag.String("config", "", "path to config.yaml (default: configs/config.yaml)")
	flag.Parse()

	// 加载 .env（如果存在），不存在不报错
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] .env file not found, relying on system environment variables")
	}

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] failed to load config: %v", err)
		os.Exit(1)
	}

	log.Printf("[INFO] config loaded successfully, server will listen on %s", cfg.Addr())

	ctx := context.Background()

	// ---------- Phase 1: 初始化 LLM ----------
	chatModel, err := llm.NewChatModel(ctx, cfg)
	if err != nil {
		log.Fatalf("[FATAL] failed to create ChatModel: %v", err)
	}
	log.Printf("[INFO] ChatModel created: provider=%s, model=%s", cfg.LLM.Provider, cfg.LLM.Model)

	// ---------- 初始化 Service ----------
	chatService := service.NewChatService(chatModel)

	// ---------- 初始化 Handler ----------
	chatHandler := handler.NewChatHandler(chatService)

	// ---------- 初始化 Gin ----------
	engine := gin.Default()

	// ---------- 注册路由 ----------
	router.Setup(engine, chatHandler)

	// ---------- 启动服务 ----------
	log.Printf("[INFO] starting server at %s", cfg.Addr())
	if err := engine.Run(cfg.Addr()); err != nil {
		log.Fatalf("[FATAL] server failed: %v", err)
	}
}
