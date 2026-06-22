<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://img.shields.io/badge/ZenStats-%E2%9A%97%EF%B8%8F%20Privacy--First%20Analytics-6C5CE7?style=for-the-badge&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0MCIgaGVpZ2h0PSI0MCIgdmlld0JveD0iMCAwIDQwIDQwIj48cGF0aCBkPSJNMjAgM0wxIDMzbDE5LTEwIDE5IDEweiIgZmlsbD0iIzhCOEJGQiIvPjxjaXJjbGUgY3g9IjIwIiBjeT0iMjAiIHI9IjgiIGZpbGw9IiM2QzVDRTciLz48L3N2Zz4=&logoWidth=32">
    <img src="https://img.shields.io/badge/ZenStats-%E2%9A%97%EF%B8%8F%20Privacy--First%20Analytics-6C5CE7?style=for-the-badge&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0MCIgaGVpZ2h0PSI0MCIgdmlld0JveD0iMCAwIDQwIDQwIj48cGF0aCBkPSJNMjAgM0wxIDMzbDE5LTEwIDE5IDEweiIgZmlsbD0iIzhCOEJGQiIvPjxjaXJjbGUgY3g9IjIwIiBjeT0iMjAiIHI9IjgiIGZpbGw9IiM2QzVDRTciLz48L3N2Zz4=&logoWidth=32" alt="ZenStats">
  </picture>
</p>

<h3 align="center">
  Self-Hosted · Cookieless · Privacy-First Web Analytics
</h3>

<p align="center">
  Go API backend for the ZenStats analytics platform.
</p>

<p align="center">
  <a href="#prerequisites">Prerequisites</a> ·
  <a href="#quick-start">Quick Start</a> ·
  <a href="#local-development">Local Development</a> ·
  <a href="#architecture">Architecture</a> ·
  <a href="#tech-stack">Tech Stack</a> ·
  <a href="#commands">Commands</a> ·
  <a href="#troubleshooting">Troubleshooting</a> ·
  <a href="#documentation">Docs</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/PostgreSQL-16-4169E1?style=flat&logo=postgresql" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/ClickHouse-24.12-FCC624?style=flat&logo=clickhouse" alt="ClickHouse">
  <img src="https://img.shields.io/badge/license-AGPL--3.0-blue" alt="License">
</p>

---

## Prerequisites

| Tool | Version | Check | Notes |
|------|---------|-------|-------|
| **Go** | ≥ 1.24 | `go version` | Required to build & run the API |
| **Docker** | ≥ 24.0 | `docker --version` | For running databases (PG + CH) |
| **Docker Compose** | ≥ 2.0 | `docker compose version` | Included with Docker Desktop |

> **Optional**: [Node.js](https://nodejs.org) ≥ 22 + [pnpm](https://pnpm.io) ≥ 10.13 for building the frontend.
> Install pnpm: `corepack enable && corepack prepare pnpm@10.13.1 --activate`

---

## Quick Start

Three steps to get the API running locally:

### 1. Start databases

```bash
cd zenstats
make test-up
```

This starts PostgreSQL (`localhost:5433`) and ClickHouse (`localhost:9001` / HTTP `localhost:8124`) in Docker containers. Data is ephemeral (tmpfs) — containers are destroyed on `make test-down`.

### 2. Run migrations

```bash
go run main.go migrate
```

This creates all tables in both PostgreSQL and ClickHouse. The dev config (`config/config_dev.yaml`) now defaults to the same ports as `make test-up`, so no extra env vars are needed.

### 3. Start the server

```bash
go run main.go server
```

API is now running at **http://localhost:8080**. Verify:

```bash
curl http://localhost:8080/api/health
```

### (Optional) Generate test data

```bash
go run main.go seed --test --clean
```

This populates the database with deterministic demo data (users, sites, events) so you can immediately explore the dashboard.

> **前端面板**: 由独立仓库 [zenstats-web](https://github.com/zenstats/zenstats-web) 维护
> **部署**: 使用 [zenstats-deploy](https://github.com/zenstats/zenstats-deploy) 一键部署完整服务栈

---

## Local Development

### Backend only (what you just did)

```bash
# Start
make test-up
go run main.go migrate
go run main.go server        # → http://localhost:8080

# Stop databases when done
make test-down
```

### Run with a different config environment

```bash
# Use test config (matches make test-up ports exactly)
APP_ENV=test go run main.go migrate
APP_ENV=test go run main.go server

# Use prod config
APP_ENV=prod go run main.go server
```

### Override config via environment variables

All `config/*.yaml` values can be overridden with `ZENSTATS_`-prefixed env vars (auto-loaded from `.env` if present):

```bash
# Example: connect to a remote ClickHouse
ZENSTATS_CLICKHOUSE_ADDR=192.168.1.100:9000 go run main.go server
```

Create a `.env` file in `zenstats/` for persistent overrides:

```env
# zenstats/.env
ZENSTATS_DB_HOST=127.0.0.1
ZENSTATS_DB_PORT=5433
ZENSTATS_DB_PASSWORD=postgres
ZENSTATS_CLICKHOUSE_ADDR=127.0.0.1:9001
ZENSTATS_MAXMIND_LICENSE_KEY=   # optional — leave empty for GeoIP fallback
ZENSTATS_SECRET_KEY=            # optional — auto-generated if not set
```

### Frontend development (sibling repo)

```bash
cd ../zenstats-web

# Mock mode — no backend needed, all API calls return fake data
VITE_USE_MOCK=true pnpm dev     # → http://localhost:5173

# Connect to local backend
pnpm dev                        # proxies /api/* to localhost:8080
```

### Full-stack via Docker Compose

Use `zenstats-deploy` to spin up the whole stack at once:

```bash
cd ../zenstats-deploy
cp .env.local .env        # 或 .env.example 用于生产
make local                # 一键启动全栈（前后端均本地构建）
```

For frontend hot-reload during development:

```bash
# 终端 1: 仅启动数据库 + API
make db-up

# 终端 2: 启动前端 Vite 开发服务器
cd ../zenstats-web
pnpm dev                  # → http://localhost:5173, 代理 /api 到 :8080
```

### Running tests

```bash
# Unit tests (no DB needed)
make test

# Full integration tests (starts DB, migrates, seeds, runs E2E, tears down)
make test-integration
```

---

## Architecture

```
API 请求  ──▶  Gin Router  ──▶  Service Layer (LRU Cache)
                                    │
                           ┌────────┴────────┐
                           ▼                 ▼
                    PostgreSQL (ent)    ClickHouse (SQL)
                     业务数据             事件/分析数据
```

### Data Flow (Event Ingestion)

```
Tracker JS  ──▶  POST /api/event  ──▶  Event Buffer
                                           │
                                    ┌──────┴──────┐
                                    ▼              ▼
                              Rate Limiter    Shield Rules
                                              (UA/IP/Geo)
                                    │              │
                                    ▼              ▼
                              Session Aggregation
                                    │
                                    ▼
                           Batch Write ──▶ ClickHouse
```

### Storage Strategy

| Database | Role | Technology |
|---|---|---|
| **PostgreSQL** | Business data (users, sites, goals, funnels, API keys, settings) | Ent ORM |
| **ClickHouse** | Analytics data (events, sessions, geolocation) | Hand-written SQL |

---

## Tech Stack

Go 1.24, Gin, Ent ORM (PostgreSQL), ClickHouse client, JWT (access + refresh tokens), Viper (config), Cobra (CLI), ants (goroutine pool), hashicorp/golang-lru (multi-level cache), GeoIP2, ua-parser

---

## Project Structure

```
zenstats/
├── cmd/                # CLI entry points (server, migrate, seed, doc)
├── config/             # Embedded YAML config with ZENSTATS_ env overrides
├── docs/               # Swagger API docs + architecture guides
├── internal/
│   ├── api/            # HTTP handlers + route registration per module
│   ├── auth/           # JWT (access + refresh token) authentication
│   ├── bootstrap/      # App initialization (DB, GeoIP, cron, queues)
│   ├── event/          # Event ingestion pipeline (buffer → pool → write to CH)
│   ├── middleware/      # Gin middleware (JWT, API key, locale detection)
│   ├── service/        # Business logic layer with LRU caching
│   │   ├── funnel/     # Funnel query engine
│   │   └── stats/      # Stats query engine (SQL builder, ClickHouse runner)
│   ├── session/        # Session aggregation + deduplication
│   └── store/          # PG (ent ORM) + CH (hand-written SQL) repositories
├── pkg/                # Shared utilities (geoip, ua_parser, response helpers)
├── sql/                # Database DDL scripts
├── Dockerfile          # API backend Docker image
├── entrypoint.sh       # Container startup script (auto-migrate)
└── main.go
```

---

## Configuration

Settings via embedded YAML + `ZENSTATS_` environment variable overrides.

| Variable | Required | Default | Description |
|---|---|---|---|
| `ZENSTATS_MAXMIND_LICENSE_KEY` | No | — | MaxMind GeoIP license key (leave empty for free fallback) |
| `ZENSTATS_DOMAIN` | No | `localhost` | Public domain |
| `ZENSTATS_SECRET_KEY` | No | Auto-generated | JWT signing secret |
| `ZENSTATS_DB_HOST` | No | `127.0.0.1` | PostgreSQL host |
| `ZENSTATS_DB_PORT` | No | `5433` | PostgreSQL port |
| `ZENSTATS_DB_USERNAME` | No | `postgres` | PostgreSQL user |
| `ZENSTATS_DB_PASSWORD` | No | `postgres` | PostgreSQL password |
| `ZENSTATS_DB_DATABASE` | No | `zenstats` | PostgreSQL database name |
| `ZENSTATS_CLICKHOUSE_ADDR` | No | `127.0.0.1:9001` | ClickHouse address |
| `ZENSTATS_CLICKHOUSE_USERNAME` | No | `default` | ClickHouse user |
| `ZENSTATS_CLICKHOUSE_PASSWORD` | No | — | ClickHouse password |
| `ZENSTATS_GEOIP_MIRROR` | No | — | GeoIP DB download mirror URL |
| `ZENSTATS_LOG_LEVEL` | No | `debug` | Log level (debug / info / warn) |
| `ZENSTATS_POOL_SIZE` | No | `100` | Event goroutine pool size |

> The defaults above reflect the **dev** config after the recent fix. See `config/config_dev.yaml` for all settings.
> `config/config_test.yaml` is used when `APP_ENV=test`.

---

## Commands

| Command | Description |
|---|---|
| `make run` | Start development server |
| `make build` | Build binary to `bin/zenstats` |
| `make test` | Run all tests |
| `make lint` | Static analysis (`go vet ./...`) |
| `make docker-build` | Build API Docker image |
| `make test-up` / `make test-down` | Start / stop test databases |
| `make test-integration` | Full integration test suite |
| `make swagger` | Generate Swagger API docs |
| `make ent-generate` | Regenerate Ent ORM code |

---

## Troubleshooting

### "connection refused" — PostgreSQL not reachable

| Cause | Fix |
|-------|-----|
| Databases not started | Run `make test-up` first |
| Wrong host in config | Set `ZENSTATS_DB_HOST=127.0.0.1 ZENSTATS_DB_PORT=5433` |
| Old dev config (pre-fix) | Update `config/config_dev.yaml` or create a `.env` file |

### "connection refused" — ClickHouse not reachable

Make sure ClickHouse is running: `docker ps | grep clickhouse`.
Ensure the port matches: `ZENSTATS_CLICKHOUSE_ADDR=127.0.0.1:9001` (the dev config now defaults to `:9001`).

### GeoIP download slow or failing

The GeoIP database is downloaded on first startup from MaxMind's CDN. If it's slow, set a mirror:

```bash
export ZENSTATS_GEOIP_MIRROR=https://github.com/Loyalsoldier/geoip/releases/latest/download
```

Or leave `ZENSTATS_MAXMIND_LICENSE_KEY` empty — the app will use a free fallback.

### Frontend returns 404 or CORS errors

- Make sure the backend is running on `:8080`
- Or use mock mode: `VITE_USE_MOCK=true pnpm dev`
- Check that Caddy (if using) has the correct upstream proxy config

### "go: command not found" or wrong Go version

Install Go ≥ 1.24: [go.dev/dl](https://go.dev/dl/)
Verify: `go version`

### Port conflicts

| Default port | Service | Config key |
|---|---|---|
| 8080 | API HTTP | `scheme.http_port` |
| 5433 | PostgreSQL (test) | `db.port` |
| 9001 | ClickHouse native | `clickhouse.addr` |
| 8124 | ClickHouse HTTP | — |

If these conflict with local services, override via environment variables or edit `config/config_dev.yaml`.

---

## Deployment

使用独立部署项目 [zenstats-deploy](https://github.com/zenstats/zenstats-deploy)：

```bash
git clone https://github.com/zenstats/zenstats-deploy.git
cd zenstats-deploy
cp .env.example .env
docker compose up -d
```

### 手动部署

```bash
# Prerequisites: Go 1.24+, PostgreSQL 16+, ClickHouse 24.12+
go run main.go migrate
go run main.go server
```

---

## Documentation

- [Swagger API Docs](http://localhost:8081/swagger/index.html) (`go run main.go doc`)
- [Statistics API](docs/api-stats.md)
- [系统架构](https://github.com/zenstats/zenstats-deploy/blob/main/docs/architecture.md) (zenstats-deploy)
- [部署指南](https://github.com/zenstats/zenstats-deploy/blob/main/docs/DEPLOY.md) (zenstats-deploy)

---

## License

**AGPL-3.0** — See [LICENSE.md](LICENSE.md) for details.
