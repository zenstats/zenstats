# AGENTS.md — Zenstats AI 协作指南

自托管网站分析服务。Go 1.24+ / Gin v1.10 后端 + React (Vite/TS) 前端，PostgreSQL（业务数据，ent ORM）+ ClickHouse（事件/Session 分析数据）。

## 项目结构速查

```
cmd/                     # CLI 入口 (server, migrate, seed)
config/                  # 内嵌 YAML，ZENSTATS_ 前缀环境变量可覆盖
data/geoip/              # GeoLite2-City.mmdb
deploy/                  # Docker Compose (PG + CH + Caddy)
docs/                    # Swagger API 文档
internal/
  api/                   # HTTP API 层 (每模块 handle.go + 路由注册)
    admin/ apikeys/ auth/ external/ funnels/ goals/ router/ sites/ stats/ types/ user/
  auth/                  # JWT v5 认证 (access + refresh 双 Token)
  bootstrap/             # 启动初始化 (DB/GeoIP/Cron)
  event/                 # 事件管道: 内存队列 → ants 协程池 → 批写 CH
    cache/               # 事件缓存接口
  middleware/             # Gin 中间件 (JWT/ApiKey/语言检测)
  service/               # 业务逻辑层, sync.Once 单例, GetXxxService()
    funnel/ stats/       # 子引擎 (stats/ 内含 sql/ types/ config/)
  session/               # Session 聚合 + 负载均衡 + 幂等写 CH
    cache/
  store/
    clickhouse/          # 手工 SQL 仓储 (model/ + repository/)
    postgresql/ent/      # ent schema + 自动生成 CRUD (禁止手动修改生成文件!)
  common/                # 公共常量/类型
pkg/                     # utils, globals, response 等工具
sql/                     # 手工 SQL 脚本
tracker/                 # 前端埋点 SDK (npm → zenstats.js)
web/                     # React SPA (git submodule)
```

## 分层架构

```
API 层: 校验参数 → 调用 service → 返回 JSON
Service 层: 业务逻辑 + 多级 LRU 缓存 (hashicorp/golang-lru/v2, 30min TTL)
Store 层: ent ORM (PG) / 手工 SQL (CH)
```

## 外部事件采集流程

```
POST /api/v1/external/event
  → verifyRequestOrigin() 检查 Origin/Referer (无头服务端请求直接放行)
  → 域名缓存查询站点配置 (站点 Domain 始终允许, AllowedOrigins 支持精确/通配符匹配)
  → 限流检查 (per-site)
  → JSON 字段校验
  → 事件缓冲池 → 协程池 → 屏蔽规则过滤 → UA/IP/Geo 解析 → Session 聚合 → 批写 CH
```

## 启动顺序

```
config.init() → InitLog → InitWorkQueue → InitClickhouseTable → InitGeoIP
  → InitPostgres → InitSystemConfig → (server 模式额外) InitCron
```

优雅关闭: 捕获 SIGINT/SIGTERM。

## 常用命令

| 用途 | 命令 |
|------|------|
| 启动开发 DB | `make dev-up` (docker-compose 启动 PG + CH) |
| 运行服务 | `make run` → `go run main.go server` |
| 运行测试 | `make test` (依赖 Docker PG + CH, 无 mock) |
| 静态检查 | `make lint` → `go vet ./...` |
| 构建 | `make build` |
| 数据库迁移 | `go run main.go migrate` |
| 生成种子数据 | `go run main.go seed` (写入 ClickHouse) |
| 修改 ent schema 后 | 改 `.go` → `make ent-generate` → `go run main.go migrate` |
| 编译 tracker SDK | `make tracker-build` |
| 初始化 web 子模块 | `make submodule-init` |
| Swagger 文档 | `make swagger` (需 swag CLI) |

## AI 协作准则

1. **语言**: 简体中文回复，代码/命令/错误保留英文
2. **最小改动**: 优先最小改动，避免无关重构；若最小方案会导致功能割裂或返工风险，说明根因并扩展范围
3. **先读后写**: 修改前理清调用链、入口、依赖顺序
4. **禁止手动修改 ent 生成代码**: `internal/store/postgresql/ent/*_create.go` 等自动生成文件不得手动编辑
5. **Schema 变更三步骤**: 改 ent schema 文件 → `make ent-generate` → `go run main.go migrate`
6. **进度播报格式** (可选):
   > 🧩 步骤：{描述} / 🎯 目的：{原因} / ▶️ 执行：{操作} / ✅ 结果：{状态} / 🧾 证据：{可验证证据}
7. **无额外 linter**: 项目不含 golangci-lint/staticcheck/revive，仅用 `go vet`
8. **环境变量前缀**: `ZENSTATS_`，如 `ZENSTATS_DB_HOST`, `ZENSTATS_CLICKHOUSE_ADDR`