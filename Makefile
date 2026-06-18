.PHONY: swagger build run test clean lint docker-build fmt submodule-init ent-generate dev-up dev-down dev-logs dev-clean prod-up prod-down prod-logs docker-migrate docker-seed

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

# 清理生产环境（含数据卷）
prod-clean:
	@cd deploy && docker compose down -v
	@echo "Production environment cleaned (all data removed)"

# ---- 部署脚本 ----

# 一键安装（交互式）
install:
	@cd deploy && ./install.sh

# ---- Docker 通用命令 ----

# 构建 Docker 镜像
docker-build:
	@docker build -t zenstats:latest .
	@echo "Docker image built: zenstats:latest"

# 数据库迁移
docker-migrate:
	@cd deploy && docker compose run --rm zenstats migrate
	@echo "Migration complete"

docker-seed:
	@cd deploy && docker compose run --rm zenstats seed
	@echo "Seed complete"

# ---- Tracker ----

# 编译 tracker 脚本（本地开发用）
tracker-build:
	@cd tracker && npm ci && npm run deploy
	@echo "Tracker script compiled: zenstats.js"

# 编译 tracker 脚本（开发模式）
tracker-dev:
	@cd tracker && npm run deploy
	@echo "Tracker script compiled (dev): zenstats.js"