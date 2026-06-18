.PHONY: build run test test-cover clean lint swagger fmt ent-generate docker-build test-up test-down test-seed test-integration

# ============================================================================
#  Go 开发命令
# ============================================================================

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

# ============================================================================
#  Docker 镜像
# ============================================================================

# 构建 API 后端 Docker 镜像
docker-build:
	@docker build -t zenstats:latest .
	@echo "Docker image built: zenstats:latest"

# ============================================================================
#  集成测试（需先启动测试数据库: make test-up）
# ============================================================================

# 启动测试专用 PG + ClickHouse 容器
test-up:
	@cd ../zenstats-deploy && docker compose -f docker-compose.test.yml up -d --wait
	@echo "Test environment ready."
	@echo "  PostgreSQL: localhost:5433"
	@echo "  ClickHouse HTTP: http://localhost:8124"
	@echo "  ClickHouse Native: localhost:9001"
	@echo ""
	@echo "Run 'make test-seed' to generate test data."

# 停止测试环境
test-down:
	@cd ../zenstats-deploy && docker compose -f docker-compose.test.yml down -v
	@echo "Test environment removed."

# 生成确定性测试数据（需先 make test-up）
test-seed:
	@APP_ENV=test go run main.go migrate
	@APP_ENV=test go run main.go seed --test --clean
	@echo "Test data seeded (deterministic, fixed random seed=42, clean start)"

# 运行完整集成测试（启动容器 → 迁移 → 种子 → 测试 → 清理）
test-integration: test-up
	@echo "Waiting for databases..."
	@sleep 3
	@APP_ENV=test go run main.go migrate
	@APP_ENV=test go run main.go seed --test --clean
	@echo ""
	@echo "=== Running Unit Tests ==="
	@go test -short ./internal/service/stats/sql/... -count=1
	@echo ""
	@echo "=== Running E2E Tests ==="
	@APP_ENV=test go test ./internal/service/stats/... -run TestE2E -v -count=1
	@echo ""
	@echo "All tests passed."

# ============================================================================
#  部署 → 请使用独立的 zenstats-deploy 项目
#  https://github.com/zenstats/zenstats-deploy
# ============================================================================
