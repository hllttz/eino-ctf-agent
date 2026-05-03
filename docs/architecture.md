# 项目架构说明

> 详见 plan.md 第 7 节。

## 目录结构

```
eino_ctf_agent/
├── cmd/server/main.go                   # 程序入口
├── internal/
│   ├── config/                          # 配置加载
│   ├── model/                           # 数据模型（HTTP 请求/响应、Document、Citation）
│   ├── llm/                             # LLM 封装（DeepSeek via OpenAI-compatible）
│   ├── embedding/                       # Embedding 封装（Qwen via DashScope）
│   ├── parser/                          # 文档解析（Markdown / PDF）
│   ├── splitter/                        # 文本切分
│   ├── vectorstore/                     # 向量存储抽象 + SQLite MVP 实现
│   ├── store/                           # 元数据存储（SQLite）
│   ├── retriever/                       # 知识库检索器
│   ├── skill/                           # Skill 加载 / 路由 / 注册
│   ├── tool/                            # Eino Tool 封装
│   ├── prompt/                          # Prompt 模板
│   ├── agent/                           # Agent Runner（Simple RAG / Tool Agent / ReAct）
│   ├── service/                         # 业务逻辑层
│   ├── handler/                         # HTTP Handler 层
│   ├── middleware/                       # 中间件（CORS / Logger / Recovery）
│   ├── errors/                          # 统一错误定义
│   └── pkg/                             # 内部工具包
│       ├── sse/                          # SSE 写入工具
│       ├── response/                     # 统一响应工具
│       └── security/                     # 安全工具
├── web/                                 # Vue 3 + Vite 前端
│   └── src/
│       ├── api/                          # 后端 API 调用模块
│       ├── components/                   # 可复用组件
│       ├── views/                        # 页面视图
│       ├── stores/                       # Pinia 状态管理
│       └── router/                       # Vue Router
├── configs/                             # 配置文件
├── data/                                # 知识库数据目录
│   ├── docs/markdown/                   # Markdown 文档
│   ├── docs/pdf/                        # PDF 文档
│   └── skills/                          # Skill 文件
├── vector_db/                           # 向量数据库（SQLite）
├── metadata_db/                         # 元数据数据库（SQLite）
└── docs/                               # 项目文档
```

## 依赖方向

```
handler → service → {llm, embedding, vectorstore, skill, retriever, agent}
                           ↓
                    Eino Framework (interface)
                           ↓
                    eino-ext (concrete implementation)
```
