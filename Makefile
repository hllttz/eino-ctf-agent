.PHONY: run test fmt vet lint redis-up redis-down redis-cli build clean

# 默认目标
run:
	go run ./cmd/server -config ./configs/config.yaml

# 以 example config 启动（用于验证配置格式）
run-example:
	go run ./cmd/server -config ./configs/config.example.yaml

# 编译检查
build:
	go build ./...

# 运行全部测试
test:
	go test ./...

# 代码格式化
fmt:
	gofmt -w .
	goimports -w . 2>/dev/null || true

# 静态分析
vet:
	go vet ./...

# 格式化 + 静态分析 + 测试
lint: fmt vet test

# 启动 Redis Stack
redis-up:
	docker compose up -d redis-stack

# 停止 Redis Stack
redis-down:
	docker compose down

# Redis CLI（连接本地 Redis Stack）
redis-cli:
	docker compose exec redis-stack redis-cli
