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
  A lightweight, GDPR-compliant alternative to Google Analytics — built with Go + ClickHouse for performance at scale.
</p>

<p align="center">
  <a href="#quick-start">Quick Start</a> ·
  <a href="#features">Features</a> ·
  <a href="#architecture">Architecture</a> ·
  <a href="#tech-stack">Tech Stack</a> ·
  <a href="#deployment">Deployment</a> ·
  <a href="#documentation">Docs</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/React-19-61DAFB?style=flat&logo=react" alt="React">
  <img src="https://img.shields.io/badge/PostgreSQL-16-4169E1?style=flat&logo=postgresql" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/ClickHouse-24.12-FCC624?style=flat&logo=clickhouse" alt="ClickHouse">
  <img src="https://img.shields.io/badge/license-Apache%202.0-blue" alt="License">
  <img src="https://img.shields.io/badge/status-beta-yellow" alt="Status">
</p>

---

## Quick Start

```bash
# Clone & enter
git clone https://github.com/zenstats/zenstats.git
cd zenstats

# Init frontend submodule
make submodule-init

# Configure (get a free GeoIP key at https://maxmind.com)
cp deploy/.env.example deploy/.env
# Edit deploy/.env → set ZENSTATS_MAXMIND_LICENSE_KEY

# Start databases (PostgreSQL + ClickHouse)
make dev-up

# Run migrations & start server
make docker-migrate
make run
```

Open **http://localhost:8080** and complete the initial setup wizard. Done.

---

## Features

| | Capability |
|---|---|
| 🍪 **Cookieless** | No cookie banners needed — GDPR compliant out of the box |
| ⚡ **Lightweight Tracker** | Tiny JS snippet (~3KB gzipped) — zero impact on Lighthouse scores |
| 📊 **Real-time Dashboard** | Live visitor counts, page views, and engagement metrics with interactive charts |
| 🌍 **GeoIP & Device Detection** | Visitor geography, browser, OS, device type, and screen size |
| 🧩 **SPA Support** | Automatic route tracking for React, Vue, Angular (pushState + hash-based routing) |
| 🎯 **Goals & Funnels** | Track conversions, define custom event goals, visualize multi-step funnels |
| 📈 **UTM Campaigns** | Built-in marketing attribution (source, medium, campaign, content, term) |
| 🔐 **Team Management** | Multi-user with role-based access (admin, user) and sub-accounts |
| 🔑 **API Access** | REST API with key-based authentication for external integrations |
| 🛡️ **Shield Rules** | Filter unwanted traffic by IP, hostname, country, or UA |
| 🐳 **One-Command Deploy** | Docker Compose with Caddy 2 + auto SSL |

### Tracker Variants

The JS tracker is compiled into **64 feature variants** (power set of 6 optional features), allowing you to serve the exact minimum code needed:

| Feature | Flag | Description |
|---|---|---|
| `ex` | `COMPILE_EXCLUSIONS` | Include/exclude paths via `data-include` / `data-exclude` |
| `fd` | `COMPILE_FILE_DOWNLOADS` | Track file download clicks (pdf, zip, docx, etc.) |
| `ha` | `COMPILE_HASH` | Hash-based SPA routing detection (for HashRouter) |
| `ma` | `COMPILE_MANUAL` | Manual pageview with custom URL override |
| `ol` | `COMPILE_OUTBOUND_LINKS` | Track outbound link clicks |
| `te` | `COMPILE_TAGGED_EVENTS` | Declarative event tracking via CSS class names |

The default `script.js` includes all features **except** hash (`ha`), and works with BrowserRouter-based SPAs. If your app uses HashRouter, serve `script.hash.js`.

---

## Architecture

```
┌──────────────┐    ┌──────────────────────────────────────┐
│   Browser    │───▶│  Caddy (Reverse Proxy + SSL + SPA)   │
│  (Tracker)   │    └──────────────┬───────────────────────┘
└──────────────┘                   │
                                  ▼
┌──────────────────────────────────────────────────────────┐
│               Zenstats (Go 1.24 / Gin)                   │
│  ┌─────────┐  ┌──────────┐  ┌────────────────────────┐   │
│  │  API    │─▶│ Service  │─▶│ Store                  │   │
│  │  Layer  │  │ (Cached) │  │  ┌────────┐ ┌────────┐ │   │
│  └─────────┘  └──────────┘  │  │PG (ent)│ │CH (SQL)│ │   │
│                             │  └────────┘ └────────┘ │   │
│                             └────────────────────────┘   │
└──────────────────────────────────────────────────────────┘
```

### Data Flow

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

PostgreSQL handles transactional business logic; ClickHouse provides columnar storage for high-throughput event aggregation and sub-second analytical queries.

---

## Tech Stack

**Backend** — Go 1.24, Gin, Ent ORM (PostgreSQL), ClickHouse client, JWT (access + refresh tokens), Viper (config), Cobra (CLI), ants (goroutine pool), hashicorp/golang-lru (multi-level cache), GeoIP2, ua-parser

**Frontend** — React 19, TypeScript, Vite 7, Tailwind CSS 4, shadcn/ui, ECharts + Recharts, Zustand, Zustand, react-i18next, Zod, react-hook-form

**Tracker** — Vanilla JS (~3KB-6KB depending on variants), UglifyJS (minifier), Handlebars (template), Playwright (E2E testing)

**Infrastructure** — Docker / Docker Compose, Caddy 2 (auto TLS via Let's Encrypt), PostgreSQL 16, ClickHouse 24.12

---

## Project Structure

```
zenstats/
├── cmd/                # CLI entry points (server, migrate, seed, doc)
├── config/             # Embedded YAML config with ZENSTATS_ env overrides
├── deploy/             # Docker Compose + Caddy configs
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
├── tracker/            # Frontend tracker JS SDK (compiled to dist/)
└── web/                # React SPA (git submodule)
```

---

## Configuration

Settings are managed via embedded YAML with `ZENSTATS_` environment variable overrides.

| Variable | Required | Default | Description |
|---|---|---|---|
| `ZENSTATS_MAXMIND_LICENSE_KEY` | **Yes** | — | MaxMind GeoIP license key (free) |
| `ZENSTATS_DOMAIN` | No | `localhost` | Public domain (for Caddy SSL + self-tracking) |
| `ZENSTATS_SECRET_KEY` | No | Auto-generated | JWT signing secret (`openssl rand -base64 32`) |
| `ZENSTATS_DB_HOST` | No | `localhost` | PostgreSQL host |
| `ZENSTATS_DB_PORT` | No | `5432` | PostgreSQL port |
| `ZENSTATS_DB_USERNAME` | No | `postgres` | PostgreSQL user |
| `ZENSTATS_DB_PASSWORD` | **Yes** | — | PostgreSQL password |
| `ZENSTATS_DB_DATABASE` | No | `zenstats` | PostgreSQL database name |
| `ZENSTATS_CLICKHOUSE_ADDR` | No | `localhost:9000` | ClickHouse address(es) |
| `ZENSTATS_CLICKHOUSE_USERNAME` | No | `default` | ClickHouse user |
| `ZENSTATS_CLICKHOUSE_PASSWORD` | No | — | ClickHouse password |
| `ZENSTATS_LOG_LEVEL` | No | `info` | Log level (debug/info/warn/error) |
| `ZENSTATS_POOL_SIZE` | No | `100` | Event processing goroutine pool size |

Copy `deploy/.env.example` → `deploy/.env` and fill in your values.

---

## Commands

| Command | Description |
|---|---|
| `make run` | Start development server |
| `make build` | Build binary to `bin/zenstats` |
| `make test` | Run all tests (requires Docker PG + CH) |
| `make lint` | Static analysis (`go vet ./...`) |
| `make dev-up` / `make dev-down` | Start / stop Docker dev environment |
| `make prod-up` / `make prod-down` | Start / stop Docker production environment |
| `make docker-migrate` | Run DB migrations in Docker |
| `make docker-seed` | Seed test data in ClickHouse |
| `make swagger` | Generate Swagger API docs |
| `make ent-generate` | Regenerate Ent ORM code after schema changes |
| `make tracker-build` | Compile tracker JS SDK (all 64 variants) |
| `make submodule-init` | Initialize web (React SPA) submodule |

---

## Deployment

### Docker (Recommended)

```bash
cp deploy/.env.example deploy/.env
# Edit deploy/.env with your settings
make prod-up
```

Caddy automatically provisions Let's Encrypt SSL certificates for non-localhost domains.

### Manual

```bash
# Prerequisites: Go 1.24+, PostgreSQL 16+, ClickHouse 24.12+
go run main.go migrate
go run main.go server
```

---

## Documentation

- [Swagger API Docs](http://localhost:8081/swagger/index.html) (run `go run main.go doc`)
- [Architecture Overview](docs/ARCHITECTURE.md) (zh-CN)
- [Deployment Guide](docs/DEPLOY.md) (zh-CN)
- [Tracker SDK Reference](docs/tracker.md) (en) / [中文版](docs/tracker_zh.md)
- [Statistics API](docs/api-stats.md) (zh-CN)

---

## License

**Apache 2.0** — See [LICENSE](https://www.apache.org/licenses/LICENSE-2.0) for details.

The tracker SDK (`tracker/`) is additionally available under the MIT License.
