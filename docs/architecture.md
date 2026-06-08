# ZenStats 部署架构图

## 整体架构 (Mermaid)

```mermaid
graph TB
    subgraph "外部流量"
        User[用户浏览器]
        Visitor[访客浏览器<br/>三方网站]
    end

    subgraph "Caddy 统一网关 :80/:443"
        Caddy[Caddy v2<br/>TLS 终止 / HTTP3 QUIC<br/>路由分发 + 静态文件服务]
        TrackerJS[Tracker JS<br/>zenstats.js<br/>跨域脚本上报]
    end

    subgraph "Frontend 构建产物"
        React[React SPA<br/>Vite + TypeScript<br/>TailwindCSS + shadcn/ui]
    end

    subgraph "Backend 容器 (Go 1.24)"
        Gin[Gin Web 框架<br/>REST API :8080]
        subgraph "内部模块"
            API[API Router<br/>/api/sites<br/>/api/stats<br/>/api/event]
            EventWorker[Event Worker<br/>异步事件处理<br/>channel buffer: 1024]
            Auth[认证模块]
            Middleware[CORS / 恢复]
        end
    end

    subgraph "数据层"
        PostgreSQL[(PostgreSQL 16<br/>元数据 / 站点配置<br/>用户数据)]
        ClickHouse[(ClickHouse 24.12<br/>事件数据 / 分析<br/>时序查询)]
    end

    subgraph "数据持久化卷"
        PGVol[db-data]
        CHVol[event-data]
        CHLogVol[event-logs]
        AppVol[zenstats-data]
    end

    %% 请求路由
    User -->|"HTTPS /*"| Caddy
    Visitor -->|"HTTPS POST /api/event"| Caddy
    Visitor -->|"HTTPS GET /js/*"| Caddy

    Caddy -->|"/* → /srv (SPA)"| React
    Caddy -->|"/js/* → /srv/js (静态文件 + CORS)"| TrackerJS
    Caddy -->|"/api/*"| Gin
    Caddy -->|"/api/event (CORS)"| Gin

    Gin --> API
    Gin --> Middleware
    API --> Auth

    API -->|"ent ORM"| PostgreSQL
    EventWorker -->|"批量写入"| ClickHouse
    API -->|"事件入队"| EventWorker

    PostgreSQL -.-> PGVol
    ClickHouse -.-> CHVol
    ClickHouse -.-> CHLogVol
    Gin -.-> AppVol

    %% 样式
    classDef proxy fill:#4CAF50,color:#fff
    classDef frontend fill:#2196F3,color:#fff
    classDef backend fill:#FF9800,color:#fff
    classDef database fill:#9C27B0,color:#fff
    classDef storage fill:#607D8B,color:#fff

    class Caddy proxy
    class React,TrackerJS frontend
    class Gin,API,EventWorker,Auth,Middleware backend
    class PostgreSQL,ClickHouse database
    class PGVol,CHVol,CHLogVol,AppVol storage
```

## Docker Compose 服务拓扑（4 服务，已移除 nginx）

```mermaid
graph LR
    Caddy[Caddy v2<br/>:80 :443<br/>静态文件 + 反向代理]
    ZenStats[zenstats<br/>Go :8080]
    PG[zenstats_db<br/>PostgreSQL :5432]
    CH[zenstats_events_db<br/>ClickHouse :8123 :9000]

    Caddy --> ZenStats
    ZenStats --> PG
    ZenStats --> CH

    style Caddy fill:#4CAF50,color:#fff
    style ZenStats fill:#FF9800,color:#fff
    style PG fill:#9C27B0,color:#fff
    style CH fill:#9C27B0,color:#fff
```

## 目录结构

```
zenstats/
├── cmd/                    # CLI 入口 (cobra)
│   ├── root.go            # 根命令
│   ├── server.go          # server 子命令
│   ├── migrate.go         # 数据库迁移
│   └── seed.go            # 种子数据
├── config/                 # 配置文件 (YAML)
├── internal/               # 内部包
│   ├── api/               # API 路由 & 处理器
│   │   ├── router/        # 路由注册
│   │   └── stats/         # 统计相关 API
│   ├── auth/              # 认证
│   ├── bootstrap/         # 启动初始化
│   ├── event/             # 事件处理 (ClickHouse)
│   ├── middleware/         # 中间件
│   ├── service/           # 业务逻辑层
│   │   └── stats/         # 统计服务
│   ├── session/           # 会话管理
│   └── store/             # 数据访问层 (ent)
│       └── postgresql/    # ent schema 生成
├── pkg/                    # 公共包
├── deploy/                 # 部署配置
│   ├── docker-compose.yml       # 生产环境
│   ├── docker-compose.dev.yml   # 开发环境覆盖
│   ├── Caddyfile                # Caddy 路由 + 静态文件配置
│   ├── .env / .env.example      # 环境变量
│   └── clickhouse/              # ClickHouse 配置
├── web/                    # 前端子模块 (React SPA)
│   ├── src/               # 源码 (TypeScript)
│   ├── public/            # 静态资源 (含 tracker JS)
│   └── dist/              # 构建产物
├── tracker/               # Tracker JS 源码 (npm)
├── sql/                   # SQL 迁移脚本
├── Dockerfile             # 后端 Go 镜像
├── Dockerfile.caddy       # Caddy 网关镜像 (3 阶段构建：Tracker → React → Caddy)
├── Makefile               # 构建/部署命令
└── main.go                # 程序入口
```

## 关键数据流

```
访客浏览器 (三方网站)
    │
    ├── GET  /js/zenstats.js ──→ Caddy ──→ /srv/js 静态文件 (file_server + CORS)
    │                                         │
    │                                    返回 tracker 脚本
    │
    └── POST /api/event ──────→ Caddy ──→ Gin ──→ Event Worker Queue
                                                      │
                                                 批量写入 ClickHouse
                                                      │
管理后台                                         统计分析查询
    │                                                 ↑
    ├── /api/sites (CRUD) ──→ Gin ──→ PostgreSQL      │
    │                                                 │
    └── /api/stats/* ────────→ Gin ──→ ClickHouse ────┘
```

## 开发环境 vs 生产环境

| 特性 | 开发环境 | 生产环境 |
|------|---------|---------|
| 命令 | `make dev-up` | `make prod-up` |
| 数据库端口暴露 | ✅ (5432, 8123, 9000) | ❌ (仅内部) |
| Caddy TLS | 自签名证书 | Let's Encrypt |
| 后端端口 | 8080 暴露 | 8080 暴露 (通过 Caddy) |
| 热重载 | 本地 `go run` | 无 |

## 镜像构建

### 后端 (Dockerfile)
- 多阶段构建: `golang:1.24-alpine` → `alpine:3.20`
- 非 root 用户运行
- 二进制: `/app/zenstats server`
- 端口: 8080

### Caddy 网关 (Dockerfile.caddy)
- 三阶段构建:
  1. `node:20-alpine` - 编译 Tracker JS
  2. `node:22-alpine` - 构建 React SPA (pnpm)
  3. `caddy:2-alpine` - 运行时：静态文件服务 + 反向代理
- 前端产物部署到 `/srv`，Caddy 通过 `file_server` 直接 serve
- 无需额外 Web Server（已移除 nginx）
- 端口: 80 / 443 / 443/udp (HTTP/3 QUIC)

## 架构变更说明

### 已移除 nginx
原架构中 frontend 容器运行 nginx 提供静态文件服务，现由 Caddy 直接 `file_server` 替代：

| 原 nginx 职责 | 新方案（Caddy） |
|---|---|
| SPA 路由 `try_files $uri /index.html` | `try_files {path} /index.html` |
| Gzip 压缩 | `encode gzip` |
| 静态资源缓存 | `header Cache-Control` |
| Tracker JS CORS | `header Access-Control-Allow-Origin "*"` |

### 服务数量
- **之前**：5 服务 (caddy + frontend + backend + postgres + clickhouse)
- **之后**：4 服务 (caddy + backend + postgres + clickhouse)