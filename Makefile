.PHONY: swagger build run test clean lint docker-build fmt submodule-init ent-generate dev-up dev-down dev-logs dev-clean prod-up prod-down prod-logs docker-migrate docker-seed test-up test-down test-seed test-integration

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

# ---- Git Submodule ----

# 初始化/更新 submodule（首次克隆项目或 submodule 变更后执行）
submodule-init:
	@git submodule update --init --recursive
	@echo "Submodule initialized: web"

# ---- Docker 开发环境 ----

# 启动开发环境（暴露数据库端口）
dev-up:
	@cd deploy && docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build
	@echo "Dev environment started."
	@echo "  ClickHouse: http://localhost:8123"
	@echo "  PostgreSQL: localhost:5432"

# 停止开发环境
dev-down:
	@cd deploy && docker compose -f docker-compose.yml -f docker-compose.dev.yml down

# 查看开发环境日志
dev-logs:
	@cd deploy && docker compose -f docker-compose.yml -f docker-compose.dev.yml logs -f

# 清理开发环境数据
dev-clean:
	@cd deploy && docker compose -f docker-compose.yml -f docker-compose.dev.yml down -v
	@echo "Dev data cleaned"

# ---- Docker 生产环境 ----

# 启动生产环境（不暴露端口）
prod-up:
	@cd deploy && docker compose up -d
	@echo "Production environment started"

# 停止生产环境
prod-down:
	@cd deploy && docker compose down

# 查看生产环境日志
prod-logs:
	@cd deploy && docker compose logs -f

# ---- Docker 通用命令 ----

# 构建 Docker 镜像
docker-build:
	@docker build -t zenstats:latest .
	@echo "Docker image built: zenstats:latest"

# 数据库迁移（服务已运行时用 exec，否则用 run）
docker-migrate:
	@cd deploy && if docker compose ps zenstats --status running -q 2>/dev/null | grep -q .; then \
		docker compose exec zenstats /app/zenstats migrate; \
	else \
		docker compose run --rm zenstats migrate; \
	fi
	@echo "Migration complete"

docker-seed:
	@cd deploy && if docker compose ps zenstats --status running -q 2>/dev/null | grep -q .; then \
		docker compose exec zenstats /app/zenstats seed; \
	else \
		docker compose run --rm zenstats seed; \
	fi
	@echo "Seed complete"

# ---- 集成测试环境 ----

# 启动测试专用 PG + ClickHouse 容器（独立端口 5433/9001/8124）
test-up:
	@cd deploy && docker compose -f docker-compose.test.yml up -d --wait
	@echo "Test environment ready."
	@echo "  PostgreSQL: localhost:5433"
	@echo "  ClickHouse HTTP: http://localhost:8124"
	@echo "  ClickHouse Native: localhost:9001"
	@echo ""
	@echo "Run 'make test-seed' to generate test data."

# 停止测试环境
test-down:
	@cd deploy && docker compose -f docker-compose.test.yml down -v
	@echo "Test environment removed."

# 生成确定性测试数据（需先 make test-up + migrate）
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

# ---- Tracker ----

# 编译 tracker 脚本（本地开发用）
tracker-build:
	@rm -rf tracker/dist
	@cd tracker && npm ci && npm run deploy
	@echo "Tracker script compiled: tracker/dist/"

# 编译 tracker 脚本（开发模式）
tracker-dev:
	@rm -rf tracker/dist
	@cd tracker && npm run deploy
	@echo "Tracker script compiled (dev): tracker/dist/"