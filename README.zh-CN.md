<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://img.shields.io/badge/ZenStats-Privacy--First%20Analytics-A78BFA?style=for-the-badge&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0MCIgaGVpZ2h0PSI0MCIgdmlld0JveD0iMCAwIDcyIDcyIiBmaWxsPSJub25lIj4KICA8cmVjdCB4PSI2IiB5PSI2IiB3aWR0aD0iNjAiIGhlaWdodD0iNjAiIHJ4PSIxNiIgZmlsbD0iIzZDNEZFMCIvPgogIDxwYXRoIGQ9Ik0xNCA0MkMxNCAzNCAyMCAyMCAzNiAxOEM0OCAxNiA1NiAyNCA1NCAzNkM1MiA0NiA0NiA1MiAzOCA1MkMyOCA1MiAyNCA0OCAyNCA0MkMyNCAzNCAzMCAzMiAzOCAzNCIgc3Ryb2tlPSIjQTc4QkZBIiBzdHJva2Utd2lkdGg9IjQuNSIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBmaWxsPSJub25lIiBvcGFjaXR5PSIwLjgiLz4KICA8cmVjdCB4PSIyOCIgeT0iMzYiIHdpZHRoPSIzLjUiIGhlaWdodD0iMTIiIHJ4PSIxLjc1IiBmaWxsPSIjRERENkZFIiBvcGFjaXR5PSIwLjc1Ii8+CiAgPHJlY3QgeD0iMzQiIHk9IjI5IiB3aWR0aD0iMy41IiBoZWlnaHQ9IjE5IiByeD0iMS43NSIgZmlsbD0iI0VERTlGRSIgb3BhY2l0eT0iMC44Ii8+CiAgPHJlY3QgeD0iNDAiIHk9IjMyIiB3aWR0aD0iMy41IiBoZWlnaHQ9IjE2IiByeD0iMS43NSIgZmlsbD0iI0Y1RjNGRiIgb3BhY2l0eT0iMC43NSIvPgo8L3N2Zz4=&logoWidth=32">
    <img src="https://img.shields.io/badge/ZenStats-Privacy--First%20Analytics-6C4FE0?style=for-the-badge&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0MCIgaGVpZ2h0PSI0MCIgdmlld0JveD0iMCAwIDcyIDcyIiBmaWxsPSJub25lIj4KICA8cmVjdCB4PSI2IiB5PSI2IiB3aWR0aD0iNjAiIGhlaWdodD0iNjAiIHJ4PSIxNiIgZmlsbD0iIzZDNEZFMCIvPgogIDxwYXRoIGQ9Ik0xNCA0MkMxNCAzNCAyMCAyMCAzNiAxOEM0OCAxNiA1NiAyNCA1NCAzNkM1MiA0NiA0NiA1MiAzOCA1MkMyOCA1MiAyNCA0OCAyNCA0MkMyNCAzNCAzMCAzMiAzOCAzNCIgc3Ryb2tlPSIjQTc4QkZBIiBzdHJva2Utd2lkdGg9IjQuNSIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBmaWxsPSJub25lIiBvcGFjaXR5PSIwLjgiLz4KICA8cmVjdCB4PSIyOCIgeT0iMzYiIHdpZHRoPSIzLjUiIGhlaWdodD0iMTIiIHJ4PSIxLjc1IiBmaWxsPSIjRERENkZFIiBvcGFjaXR5PSIwLjc1Ii8+CiAgPHJlY3QgeD0iMzQiIHk9IjI5IiB3aWR0aD0iMy41IiBoZWlnaHQ9IjE5IiByeD0iMS43NSIgZmlsbD0iI0VERTlGRSIgb3BhY2l0eT0iMC44Ii8+CiAgPHJlY3QgeD0iNDAiIHk9IjMyIiB3aWR0aD0iMy41IiBoZWlnaHQ9IjE2IiByeD0iMS43NSIgZmlsbD0iI0Y1RjNGRiIgb3BhY2l0eT0iMC43NSIvPgo8L3N2Zz4=&logoWidth=32" alt="ZenStats">
  </picture>
</p>

<h3 align="center">
  自托管 · 无 Cookie · 隐私优先的网站分析
</h3>

<p align="center">
  ZenStats 分析平台的 Go API 后端。
</p>

<p align="center">
  <a href="#环境要求">环境要求</a> ·
  <a href="#快速开始">快速开始</a> ·
  <a href="#本地开发">本地开发</a> ·
  <a href="#架构">架构</a> ·
  <a href="#技术栈">技术栈</a> ·
  <a href="#命令">命令</a> ·
  <a href="#故障排除">故障排除</a> ·
  <a href="#文档">文档</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=flat&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Gin-1.12-00ADD8?style=flat&logo=go" alt="Gin">
  <img src="https://img.shields.io/badge/Ent-0.14-4B8BBE?style=flat" alt="Ent">
  <img src="https://img.shields.io/badge/PostgreSQL-18-4169E1?style=flat&logo=postgresql" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/ClickHouse-25.11-FCC624?style=flat&logo=clickhouse" alt="ClickHouse">
  <img src="https://img.shields.io/badge/license-AGPL--3.0-blue" alt="License">
</p>

---

## 环境要求

| 工具 | 版本 | 检查 | 说明 |
|------|---------|-------|-------|
| **Go** | ≥ 1.25 | `go version` | 构建和运行 API 所需 |
| **Docker** | ≥ 24.0 | `docker --version` | 用于运行数据库 (PG + CH) |
| **Docker Compose** | ≥ 2.0 | `docker compose version` | Docker Desktop 自带 |

> **可选**: [Node.js](https://nodejs.org) ≥ 22 + [pnpm](https://pnpm.io) ≥ 10.13，用于构建前端。
> 安装 pnpm: `corepack enable && corepack prepare pnpm@10.13.1 --activate`

---

## 快速开始

三步在本地运行 API：

### 1. 启动数据库

```bash
cd zenstats
make test-up
```

将在 Docker 容器中启动 PostgreSQL (`localhost:5433`) 和 ClickHouse (`localhost:9001` / HTTP `localhost:8124`)。数据是临时的 (tmpfs) — 容器在 `make test-down` 后销毁。

### 2. 执行迁移

```bash
go run main.go migrate
```

在 PostgreSQL 和 ClickHouse 中创建所有表。内嵌配置默认值与 `make test-up` 端口一致 — 无需额外环境变量。

### 3. 启动服务

```bash
go run main.go server
```

API 现已运行在 **http://localhost:8080**。验证：

```bash
curl http://localhost:8080/api/health
```

### （可选）生成测试数据

```bash
go run main.go seed --test --clean
```

向数据库填充确定性演示数据（用户、站点、事件），可立即浏览仪表盘。

> **管理面板**: 由独立仓库 [zenstats-web](https://github.com/zenstats/zenstats-web) 维护
> **部署**: 使用 [zenstats-deploy](https://github.com/zenstats/zenstats-deploy) 一键部署完整服务栈

---

## 本地开发

### 仅后端

```bash
# 启动
make test-up
go run main.go migrate
go run main.go server        # → http://localhost:8080

# 完成后停止数据库
make test-down
```

### 使用不同环境配置

```bash
# 使用测试配置（与 make test-up 端口完全匹配）
APP_ENV=test go run main.go migrate
APP_ENV=test go run main.go server

# 使用生产配置
APP_ENV=prod go run main.go server
```

### 通过环境变量覆盖配置

所有 `config/*.yaml` 值可通过 `ZENSTATS_` 前缀的环境变量覆盖（自动从 `.env` 加载）：

```bash
# 示例：连接远程 ClickHouse
ZENSTATS_CLICKHOUSE_ADDR=192.168.1.100:9000 go run main.go server
```

在 `zenstats/` 创建 `.env` 文件持久化覆盖：

```env
# zenstats/.env
ZENSTATS_DB_HOST=127.0.0.1
ZENSTATS_DB_PORT=5433
ZENSTATS_DB_PASSWORD=postgres
ZENSTATS_CLICKHOUSE_ADDR=127.0.0.1:9001
ZENSTATS_MAXMIND_LICENSE_KEY=   # 可选 — 留空使用 GeoIP 回退
ZENSTATS_SECRET_KEY=            # 可选 — 未设置时自动生成
```

### 前端开发（同级仓库）

```bash
cd ../zenstats-web

# Mock 模式 — 无需后端，所有 API 返回模拟数据
VITE_USE_MOCK=true pnpm dev     # → http://localhost:5173

# 连接本地后端
pnpm dev                        # 代理 /api/* 到 localhost:8080
```

### 通过 Docker Compose 全栈开发

使用 `zenstats-deploy` 一键启动完整服务栈：

```bash
cd ../zenstats-deploy
cp .env.local .env        # 或 .env.example 用于生产
make local                # 一键启动全栈（前后端均本地构建）
```

开发时前端热重载：

```bash
# 终端 1: 启动数据库 + API
make db-up

# 终端 2: 启动前端 Vite 开发服务器
cd ../zenstats-web
pnpm dev                  # → http://localhost:5173, 代理 /api 到 :8080
```

### 运行测试

```bash
# 单元测试（无需 DB）
make test

# 完整集成测试（启动 DB、迁移、种子数据、E2E 测试、清理）
make test-integration
```

---

## 架构

```
API 请求 ──▶ Gin Router ──▶ Service Layer (LRU Cache)
                                    │
                           ┌────────┼────────┐
                           ▼        ▼        ▼
                    PostgreSQL  PostgreSQL  ClickHouse (SQL)
                    (业务数据)   (系统配置      (分析数据)
                                event_salt)
```

### 数据流（事件采集）

```
Tracker JS  ──▶  POST /api/event  ──▶  Origin 校验
                                           │
                                    ┌──────┴──────┐
                                    ▼              ▼
                              IP 过滤         域名白名单
                           (数据中心/威胁)
                                    │              │
                                    ▼              ▼
                              速率限制      屏蔽规则
                                          (UA/IP/地区/国家)
                                    │              │
                                    ▼              ▼
                              字段长度校验
                           (事件名≤120/URL≤2000/Props≤30)
                                    │              │
                                    ▼              ▼
                          事件缓冲 → 协程池处理
                                    │
                           ┌────────┴────────┐
                           ▼                  ▼
                     UA 解析              GeoIP 定位
                     (Bot 过滤)            (城市/国家)
                           │                  │
                           └────────┬─────────┘
                                    ▼
                           ┌────────────────┐
                           │  SPA Hash 路由  │
                           │  h=1 时拼接      │
                           │  pathname+fragment│
                           └────────────────┘
                                    │
                                    ▼
                              Source 解析
                              (UTM/Referrer)
                                    │
                                    ▼
                           Session 聚合
                           (30min 缓存 / 幂等写)
                                    │
                                    ▼
                           批量写入 ──▶ ClickHouse
```

### 存储策略

| 数据库 | 角色 | 技术 |
|---|---|---|
| **PostgreSQL** | 业务数据（用户、站点、目标、漏斗、API Key、配置） | Ent ORM |
| **PostgreSQL** | 系统配置（event_salt 等） | Ent ORM |
| **ClickHouse** | 分析数据（事件、Session、地理位置） | 手工 SQL |

---

## 技术栈

| 组件 | 版本 | 用途 |
|-----------|---------|---------|
| **Go** | 1.25 | 运行时 |
| **Gin** | 1.12 | HTTP 框架 |
| **Ent** | 0.14 | PostgreSQL ORM |
| **ClickHouse Go** | 2.33 | ClickHouse 客户端 |
| **JWT** | 5.2 | Access + refresh token 认证 |
| **Viper** | 1.20 | 配置管理 |
| **Cobra** | 1.9 | CLI（server / migrate / seed） |
| **ants** | 1.2 | 协程池（事件处理） |
| **golang-lru** | 2.0 | 多级 LRU 缓存 |
| **GeoIP2** | 1.11 | IP 地理定位（MaxMind） |
| **ua-parser** | latest | User-Agent 解析 |

数据库: **PostgreSQL 18**（业务数据） + **ClickHouse 25.11**（分析数据）

---

## 项目结构

```
zenstats/
├── cmd/                # CLI 入口（server, migrate, seed）
├── config/             # 内嵌 YAML 配置，ZENSTATS_ 环境变量覆盖
├── docs/               # Swagger API 文档 + 架构指南
├── internal/
│   ├── api/            # HTTP handlers + 按模块路由注册
│   ├── auth/           # JWT（access + refresh token）认证
│   ├── bootstrap/      # 应用初始化（DB, GeoIP, cron, 队列）
│   ├── event/          # 事件采集管道（缓冲 → 协程池 → 批量写）
│   ├── middleware/     # Gin 中间件（JWT, API Key, 语言检测）
│   ├── model/          # 共享领域类型（EventRequest 等）
│   ├── service/        # 业务逻辑层（LRU 缓存）
│   │   ├── funnel/     # 漏斗查询引擎
│   │   ├── seed/       # 测试数据生成器（设备、地理位置、来源）
│   │   └── stats/      # 统计查询引擎（SQL 构建器、ClickHouse 执行器）
│   ├── session/        # Session 聚合 + 去重
│   └── store/          # PG（ent ORM）+ CH（手工 SQL）仓储
├── pkg/                # 共享工具
│   ├── bcrypt/         # 密码哈希（Generate / Check）
│   ├── file/           # 文件复制工具
│   ├── generic/        # 泛型数据结构（DynamicQueue[T]）
│   ├── geoip/          # MaxMind GeoIP 查询
│   ├── globals/        # 全局单例（DB, Queue）
│   ├── i18n/           # 国际化工具
│   ├── iputil/         # IP 解析 / 客户端 IP 提取
│   ├── log/            # 结构化日志
│   ├── pool/           # 协程池封装
│   ├── response/       # HTTP 响应工具
│   ├── scheduler/      # 定时任务调度器
│   ├── ua_parser/      # User-Agent 解析
│   ├── utils/          # 哈希、I/O、切片工具
│   └── validator/      # 请求校验
├── sql/                # 数据库 DDL 脚本（参考用）
├── Dockerfile          # 多阶段构建（golang:1.25-alpine → alpine:3.22）
├── entrypoint.sh       # 容器启动脚本（自动迁移）
└── main.go
```

---

## 配置

配置来自内嵌 `config.yaml` + `ZENSTATS_` 环境变量覆盖。

启动时生效配置写入 `config/config_<env>.yaml`（自动生成，gitignore）。也可手动放置 `config/config_<env>.yaml` 文件覆盖内嵌默认值。

| 变量 | 必填 | 默认 | 说明 |
|---|---|---|---|
| `ZENSTATS_MAXMIND_LICENSE_KEY` | 否 | — | MaxMind GeoIP 许可证密钥（留空使用免费回退） |
| `ZENSTATS_DOMAIN` | 否 | `localhost` | 公开域名 |
| `ZENSTATS_SECRET_KEY` | 否 | 自动生成 | JWT 签名密钥 |
| `ZENSTATS_DB_HOST` | 否 | `127.0.0.1` | PostgreSQL 主机 |
| `ZENSTATS_DB_PORT` | 否 | `5433` | PostgreSQL 端口 |
| `ZENSTATS_DB_USERNAME` | 否 | `postgres` | PostgreSQL 用户名 |
| `ZENSTATS_DB_PASSWORD` | 否 | `postgres` | PostgreSQL 密码 |
| `ZENSTATS_DB_DATABASE` | 否 | `zenstats` | PostgreSQL 数据库名 |
| `ZENSTATS_CLICKHOUSE_ADDR` | 否 | `127.0.0.1:9001` | ClickHouse 地址 |
| `ZENSTATS_CLICKHOUSE_USERNAME` | 否 | `default` | ClickHouse 用户名 |
| `ZENSTATS_CLICKHOUSE_PASSWORD` | 否 | — | ClickHouse 密码 |
| `ZENSTATS_GEOIP_MIRROR` | 否 | — | GeoIP 数据库下载镜像 URL |
| `ZENSTATS_LOG_LEVEL` | 否 | `debug` | 日志级别（debug / info / warn） |
| `ZENSTATS_POOL_SIZE` | 否 | `100` | 事件协程池大小 |

> 以上默认值反映 **dev** 环境。完整配置见 `config/config.yaml`。
> 使用 `APP_ENV=test` 或 `APP_ENV=prod` 切换环境。

---

## 命令

| 命令 | 说明 |
|---|---|
| `make run` | 启动开发服务器 |
| `make build` | 构建二进制到 `bin/zenstats` |
| `make test` | 运行所有测试 |
| `make lint` | 静态分析 (`go vet ./...`) |
| `make docker-build` | 构建 API Docker 镜像 |
| `make test-up` / `make test-down` | 启动 / 停止测试数据库 |
| `make test-integration` | 完整集成测试套件 |
| `make swagger` | 生成 Swagger API 文档 |
| `make ent-generate` | 重新生成 Ent ORM 代码 |

---

## 故障排除

### "connection refused" — PostgreSQL 无法连接

| 原因 | 解决 |
|-------|-----|
| 数据库未启动 | 先运行 `make test-up` |
| 配置中主机错误 | 设置 `ZENSTATS_DB_HOST=127.0.0.1 ZENSTATS_DB_PORT=5433` |
| 配置未加载 | 创建 `.env` 文件或检查 `APP_ENV` 是否匹配 |

### "connection refused" — ClickHouse 无法连接

确保 ClickHouse 正在运行: `docker ps | grep clickhouse`。
确保端口匹配: `ZENSTATS_CLICKHOUSE_ADDR=127.0.0.1:9001`（开发配置默认为 `:9001`）。

### GeoIP 下载缓慢或失败

GeoIP 数据库在首次启动时从 MaxMind CDN 下载。如果缓慢，设置镜像：

```bash
export ZENSTATS_GEOIP_MIRROR=https://github.com/Loyalsoldier/geoip/releases/latest/download
```

或保持 `ZENSTATS_MAXMIND_LICENSE_KEY` 为空 — 应用将使用免费回退。

### 前端返回 404 或 CORS 错误

- 确保后端运行在 `:8080`
- 或使用 Mock 模式: `VITE_USE_MOCK=true pnpm dev`
- 检查 Caddy（如果使用）的上游代理配置是否正确

### "go: command not found" 或 Go 版本错误

安装 Go ≥ 1.24: [go.dev/dl](https://go.dev/dl/)
验证: `go version`

### 端口冲突

| 默认端口 | 服务 | 配置键 |
|---|---|---|
| 8080 | API HTTP | `scheme.http_port` |
| 5433 | PostgreSQL（测试） | `db.port` |
| 9001 | ClickHouse Native | `clickhouse.addr` |
| 8124 | ClickHouse HTTP | — |

如果与本地服务冲突，通过环境变量或 `.env` 文件覆盖。

---

## 部署

使用独立部署项目 [zenstats-deploy](https://github.com/zenstats/zenstats-deploy)：

```bash
git clone https://github.com/zenstats/zenstats-deploy.git
cd zenstats-deploy
cp .env.example .env
docker compose up -d
```

### 手动部署

```bash
# 环境要求: Go 1.25+, PostgreSQL 18+, ClickHouse 25.11+
go run main.go migrate
go run main.go server
```

---

## 文档

- [Swagger API 文档](http://localhost:8080/swagger/index.html)
- [统计 API](docs/api-stats.md)
- [IP 分类配置](docs/ip-classification.md)
- [优化路线图](docs/optimization-roadmap.md)
- [系统架构](https://github.com/zenstats/zenstats-deploy/blob/main/docs/architecture.md) (zenstats-deploy)
- [部署指南](https://github.com/zenstats/zenstats-deploy/blob/main/docs/DEPLOY.md) (zenstats-deploy)

---

## License

**AGPL-3.0** — 详见 [LICENSE.md](LICENSE.md)。
