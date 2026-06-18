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
  <a href="#quick-start">Quick Start</a> ·
  <a href="#architecture">Architecture</a> ·
  <a href="#tech-stack">Tech Stack</a> ·
  <a href="#commands">Commands</a> ·
  <a href="#documentation">Docs</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/PostgreSQL-16-4169E1?style=flat&logo=postgresql" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/ClickHouse-24.12-FCC624?style=flat&logo=clickhouse" alt="ClickHouse">
  <img src="https://img.shields.io/badge/license-Apache%202.0-blue" alt="License">
</p>

---

## Quick Start

```bash
git clone https://github.com/zenstats/zenstats.git
cd zenstats

# Start databases
make test-up

# Run migrations & start server
go run main.go migrate
make run
```

Open **http://localhost:8080/api/health** to verify the API is running.

> **前端面板**: 由独立仓库 [zenstats-web](https://github.com/zenstats/zenstats-web) 维护  
> **部署**: 使用 [zenstats-deploy](https://github.com/zenstats/zenstats-deploy) 一键部署完整服务栈

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
| `ZENSTATS_MAXMIND_LICENSE_KEY` | **Yes** | — | MaxMind GeoIP license key (free) |
| `ZENSTATS_DOMAIN` | No | `localhost` | Public domain |
| `ZENSTATS_SECRET_KEY` | No | Auto-generated | JWT signing secret |
| `ZENSTATS_DB_HOST` | No | `localhost` | PostgreSQL host |
| `ZENSTATS_DB_PORT` | No | `5432` | PostgreSQL port |
| `ZENSTATS_DB_USERNAME` | No | `postgres` | PostgreSQL user |
| `ZENSTATS_DB_PASSWORD` | **Yes** | — | PostgreSQL password |
| `ZENSTATS_DB_DATABASE` | No | `zenstats` | PostgreSQL database name |
| `ZENSTATS_CLICKHOUSE_ADDR` | No | `localhost:9000` | ClickHouse address |
| `ZENSTATS_CLICKHOUSE_USERNAME` | No | `default` | ClickHouse user |
| `ZENSTATS_CLICKHOUSE_PASSWORD` | No | — | ClickHouse password |
| `ZENSTATS_LOG_LEVEL` | No | `info` | Log level |
| `ZENSTATS_POOL_SIZE` | No | `100` | Event goroutine pool size |

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
- [Architecture Overview](docs/ARCHITECTURE.md)
- [Deployment Guide](docs/DEPLOY.md)
- [Statistics API](docs/api-stats.md)

---

## License

**Apache 2.0** — See [LICENSE](https://www.apache.org/licenses/LICENSE-2.0) for details.
