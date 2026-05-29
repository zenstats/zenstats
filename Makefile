.PHONY: swagger build run test clean lint docker-build docker-up docker-down docker-logs docker-migrate ent-generate fmt

# 构建项目
build:
	@go build -o bin/zenstats .
	@echo "Build complete: bin/zenstats"

# 运行服务器
run:
	@go run main.go server

# 运行测试
test:
	@go test ./... -v

# 运行测试并生成覆盖率报告
test-cover:
	@go test ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# 代码静态检查
lint:
	@go vet ./...

# 生成 Swagger 文档
swagger:
	@swag init -g docs/swagger.go --output docs
	@echo "Swagger documentation generated"

# 清理构建产物
clean:
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# 格式化代码
fmt:
	@go fmt ./...

# ent 代码生成
ent-generate:
	@cd internal/store/postgresql && go generate ./...
	@echo "Ent code generation complete"

# ---- Docker ----

# 构建 Docker 镜像
docker-build:
	@docker build -t zenstats:latest .
	@echo "Docker image built: zenstats:latest"

# 使用 docker compose 启动全部服务（含数据库）
docker-up:
	@cd deploy && docker compose up -d --build
	@echo "All services started. Run 'make docker-logs' to view logs."

# 停止全部服务
docker-down:
	@cd deploy && docker compose down

# 查看服务日志
docker-logs:
	@cd deploy && docker compose logs -f

# 数据库迁移（在 docker 环境中执行）
docker-migrate:
	@cd deploy && docker compose exec zenstats /app/zenstats migrate
	@echo "Migration complete"
