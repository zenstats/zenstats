# ZenStats 系统架构说明

## 目录

- [项目概述](#项目概述)
- [技术栈](#技术栈)
- [目录结构](#目录结构)
- [数据架构](#数据架构)
- [核心流程](#核心流程)
- [API 架构](#api-架构)
- [前端架构](#前端架构)
- [部署架构](#部署架构)

---

## 项目概述

ZenStats 是一个隐私友好的网站分析平台，系统采用 Go 后端 + React 前端的架构，支持事件追踪、流量分析、漏斗分析等功能。

核心特性：
- 无 Cookie 追踪，保护用户隐私
- 轻量级追踪脚本（~3KB）
- 支持 SPA 单页应用追踪
- 实时数据分析
- 自定义事件与目标转化

---

## 技术栈

### 后端
| 技术 | 用途 |
|------|------|
| Go 1.24+ | 主要开发语言 |
| Gin | HTTP 框架 |
| Ent | ORM 框架（PostgreSQL） |
| ClickHouse Driver | ClickHouse 数据库驱动 |

### 前端
| 技术 | 用途 |
|------|------|
| React 18 | UI 框架 |
| TypeScript | 类型安全 |
| Vite | 构建工具 |
| React Router | 路由管理 |
| Tailwind CSS | 样式框架 |
| react-i18next | 国际化 |

### 数据库
| 数据库 | 用途 |
|--------|------|
| PostgreSQL 16+ | 用户、站点、目标等业务数据 |
| ClickHouse 24.12+ | 事件流、会话聚合等分析数据 |

### 部署
| 技术 | 用途 |
|------|------|
| Docker | 容器化 |
| Docker Compose | 服务编排 |
| Caddy | 反向代理、自动 SSL |

---

## 目录结构

```
zenstats/
├── cmd/                    # CLI 命令入口
│   ├── root.go            # 根命令
│   ├── server.go          # 启动 HTTP 服务
│   ├── migrate.go         # 数据库迁移
│   ├── seed.go            # 初始化数据
│   └── doc.go             # 生成 Swagger 文档
│
├── internal/               # 核心业务逻辑（不可外部导入）
│   ├── api/               # HTTP 路由与处理器
│   │   ├── router/        # 路由注册
│   │   ├── auth/          # 认证接口
│   │   ├── stats/         # 统计分析接口
│   │   ├── sites/         # 站点管理接口
│   │   ├── external/      # 事件采集接口
│   │   ├── goals/         # 目标管理接口
│   │   ├── funnels/       # 漏斗管理接口
│   │   └── apikeys/       # API Key 管理接口
│   │
│   ├── service/           # 业务服务层
│   │   ├── stats/         # 统计查询服务
│   │   │   └── sql/       # SQL 查询构建器
│   │   ├── funnel/        # 漏斗分析服务
│   │   ├── sites.go       # 站点服务
│   │   ├── goals.go       # 目标服务
│   │   └── users.go       # 用户服务
│   │
│   ├── store/             # 数据存储层
│   │   ├── postgresql/    # PostgreSQL 存储
│   │   │   └── ent/       # Ent ORM 生成代码
│   │   └── clickhouse/    # ClickHouse 存储
│   │       ├── repository/# 数据访问层
│   │       └── models/    # 数据模型
│   │
│   ├── event/             # 事件摄入处理
│   │   ├── event.go       # 事件处理主逻辑
│   │   ├── buffer.go      # 事件缓冲
│   │   └── cache/         # 缓存处理
│   │
│   ├── auth/              # 认证授权
│   ├── middleware/         # HTTP 中间件
│   ├── session/           # 会话管理
│   └── bootstrap/         # 应用初始化
│
├── pkg/                    # 可复用工具包
│   ├── geoip/             # GeoIP 地理定位
│   ├── ua_parser/         # User-Agent 解析
│   ├── response/          # 统一响应格式
│   ├── i18n/              # 国际化
│   ├── log/               # 日志工具
│   └── validator/         # 数据验证
│
├── tracker/                # 前端追踪脚本
│   ├── src/               # 脚本源码
│   └── compile.js         # 编译脚本
│
├── web/                    # React 前端
│   └── src/
│       ├── pages/         # 页面组件
│       ├── components/    # 通用组件
│       ├── store/         # 状态管理
│       ├── i18n/          # 国际化配置
│       └── utils/         # 工具函数
│
├── sql/                    # 数据库脚本
│   ├── clickhouse/        # ClickHouse 建表语句
│   └── migration.sql      # PostgreSQL 迁移脚本
│
├── config/                 # 配置文件
│   ├── config_prod.yaml   # 生产环境配置
│   └── config_dev.yaml    # 开发环境配置
│
├── deploy/                 # Docker 部署配置
│   ├── docker-compose.yml # 生产环境编排
│   ├── docker-compose.dev.yml # 开发环境编排
│   └── clickhouse/        # ClickHouse 配置
│
├── data/                   # 数据文件
│   └── geoip/             # GeoIP 数据库
│
└── docs/                   # 项目文档
    ├── ARCHITECTURE.md    # 架构说明（本文档）
    ├── api-stats.md       # 统计 API 文档
    ├── tracker.md         # 追踪脚本文档（英文）
    ├── tracker_zh.md      # 追踪脚本文档（中文）
    └── DEPLOY.md          # 部署指南
```

---

## 数据架构

### PostgreSQL 存储

PostgreSQL 存储业务配置数据，使用 Ent ORM 进行数据访问。

| 表名 | 说明 | 关键字段 |
|------|------|----------|
| `users` | 用户账户 | id, email, password_hash, name |
| `sites` | 站点配置 | id, domain, name, user_id |
| `goals` | 转化目标 | id, site_id, name, event_name, page_path |
| `funnels` | 漏斗配置 | id, site_id, name, steps |
| `api_keys` | API 密钥 | id, site_id, key, name |

### ClickHouse 存储

ClickHouse 存储事件流和分析数据，采用列式存储优化查询性能。

#### 核心表

| 表名 | 引擎 | 说明 |
|------|------|------|
| `events` | MergeTree | 原始事件流，按时间分区 |
| `sessions` | VersionedCollapsingMergeTree | 会话聚合数据 |
| `location_data` | MergeTree | 地理位置数据 |
| `imported_*` | MergeTree | 导入的历史数据 |

#### Events 表结构

```sql
CREATE TABLE events (
    -- 事件基本信息
    name            LowCardinality(String),  -- 事件名称
    timestamp       DateTime,                -- 事件时间
    site_id         UInt64,                  -- 站点 ID
    user_id         UInt64,                  -- 用户 ID
    session_id      UInt64,                  -- 会话 ID

    -- 页面信息
    url             String,                  -- 完整 URL
    hostname        String,                  -- 主机名
    pathname        String,                  -- 路径

    -- 来源信息
    referrer        String,                  -- 来源 URL
    referrer_source String,                  -- 来源名称
    utm_medium      String,                  -- UTM 参数
    utm_source      String,
    utm_campaign    String,

    -- 设备信息
    browser         LowCardinality(String),  -- 浏览器
    browser_version LowCardinality(String),  -- 浏览器版本
    operating_system LowCardinality(String), -- 操作系统
    screen_size     LowCardinality(String),  -- 设备类型（Desktop/Mobile/Tablet）
    user_agent      String,                  -- 原始 UA

    -- 地理信息
    country_code    FixedString(2),          -- 国家代码
    city_geoname_id String,                  -- 城市 ID
    ipv4            IPv4,

    -- 参与度
    engagement_time UInt32,                  -- 参与时间（毫秒）
    scroll_depth    UInt8,                   -- 滚动深度（%）

    -- 别名字段（兼容性）
    device          ALIAS screen_size,       -- 设备类型别名
    os              ALIAS operating_system,  -- 操作系统别名
    source          ALIAS referrer_source,   -- 来源别名
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(timestamp)
PRIMARY KEY (site_id, toDate(timestamp), name, user_id)
```

#### Sessions 表结构

Sessions 表通过物化视图从 Events 表聚合生成，存储会话级别的统计数据。

```sql
CREATE TABLE sessions (
    start           DateTime,    -- 会话开始时间
    timestamp       DateTime,    -- 最后活动时间
    session_id      UInt64,
    is_bounce       UInt8,       -- 是否跳出
    pageviews       Int32,       -- 页面浏览数
    events          Int32,       -- 事件数
    duration        UInt32,      -- 会话时长
    entry_page      String,      -- 入口页面
    exit_page       String,      -- 退出页面
    -- ... 其他字段同 events
)
ENGINE = VersionedCollapsingMergeTree(sign, version)
```

---

## 核心流程

### 1. 事件摄入流程

```
用户浏览器
    │
    ▼
[Tracker Script] ──POST /api/event──▶ [Event API]
                                          │
                                          ▼
                                   [Event Service]
                                          │
                            ┌─────────────┴─────────────┐
                            ▼                           ▼
                    [Buffer/Cache]              [GeoIP 解析]
                            │                   [UA 解析]
                            ▼                           │
                    [ClickHouse Events] ◀───────────────┘
                            │
                            ▼ (物化视图)
                    [ClickHouse Sessions]
```

**流程说明：**
1. Tracker 脚本收集页面浏览、参与度、自定义事件等数据
2. 通过 POST 请求发送到 `/api/event` 端点
3. Event Service 处理事件，进行 GeoIP 和 UA 解析
4. 事件写入 ClickHouse events 表
5. 物化视图自动聚合生成 sessions 数据

### 2. 统计查询流程

```
前端 Dashboard
    │
    ▼
[Stats API] ──▶ [Stats Service] ──▶ [Query Builder]
                                          │
                                          ▼
                                   [ClickHouse]
                                          │
                                          ▼
                                   [聚合结果]
                                          │
                                          ▼
                                   [对比计算]
                                          │
                                          ▼
                                   [JSON 响应]
```

**支持的查询类型：**
- `aggregate` — 总览指标（UV、PV、跳出率等）
- `main-graph` — 时序图表
- `breakdown` — 维度细分（来源、页面、设备等排行）
- `current-visitors` — 实时访客数

### 3. Session 聚合流程

```
Events 表
    │
    ▼ (MergeTree 后台合并)
[Session Dedup]
    │
    ▼
Sessions 表 (VersionedCollapsingMergeTree)
    │
    ▼
[统计查询]
```

---

## API 架构

### 路由结构

```
/api/
├── event                    # 事件采集（外部）
├── auth/                    # 认证
│   ├── login               # 登录
│   ├── register            # 注册
│   └── setup               # 初始设置
├── v1/                      # 管理 API
│   ├── sites/              # 站点管理
│   ├── goals/              # 目标管理
│   ├── funnels/            # 漏斗管理
│   └── apikeys/            # API Key 管理
├── stats/                   # 统计 API
│   └── :domain/
│       ├── aggregate       # 总览指标
│       ├── main-graph      # 时序图表
│       ├── breakdown       # 维度细分
│       ├── current-visitors # 实时访客
│       └── export          # 导出 CSV
└── external/                # 外部接口
```

### 认证方式

统计 API 支持两种认证方式：
1. **JWT Bearer Token** — 用于前端 Dashboard
2. **API Key** — 用于外部集成

### 统一响应格式

```json
{
  "code": 200,
  "message": "success",
  "data": { ... }
}
```

### 支持的统计维度

| 维度 | 说明 |
|------|------|
| `visit:source` | 流量来源 |
| `visit:country` | 国家 |
| `visit:browser` | 浏览器 |
| `visit:os` | 操作系统 |
| `visit:device` | 设备类型 |
| `visit:entry_page` | 入口页面 |
| `visit:exit_page` | 退出页面 |
| `event:page` | 页面路径 |
| `event:name` | 事件名称 |

---

## 前端架构

### 路由结构

```
/
├── /login                   # 登录页
├── /setup                   # 初始设置
├── /sites                   # 站点列表
│   ├── /new                 # 新建站点
│   ├── /:domain/
│   │   ├── /stats           # 统计面板
│   │   ├── /funnel-analysis # 漏斗分析
│   │   ├── /install         # 安装指南
│   │   └── /settings/       # 站点设置
│   │       ├── /general     # 基本设置
│   │       ├── /goals       # 目标管理
│   │       ├── /funnels     # 漏斗管理
│   │       └── /shields     # 屏蔽设置
├── /apikeys                 # API Key 管理
└── /settings/account        # 账户设置
```

### 组件结构

```
web/src/
├── pages/                   # 页面组件
│   ├── sites/stats/        # 统计面板
│   │   ├── stats.tsx       # 主页面
│   │   └── components/     # 子组件
│   │       ├── dimension-settings.tsx  # 维度设置
│   │       ├── custom-query.tsx        # 自定义查询
│   │       └── ...
│   └── ...
├── components/              # 通用组件
├── store/                   # 状态管理
├── i18n/                    # 国际化
└── utils/                   # 工具函数
```

---

## 部署架构

### Docker Compose 服务

```
                    ┌─────────────────────────────────────┐
                    │           Docker Compose            │
                    │                                     │
  Internet ──▶ [Caddy:80/443] ──▶ [ZenStats:8080]         │
                    │                    │                │
                    │         ┌──────────┴──────────┐     │
                    │         ▼                     ▼     │
                    │   [PostgreSQL:5432]   [ClickHouse:9000]
                    │         │                     │     │
                    │         ▼                     ▼     │
                    │   [db-data 卷]       [event-data 卷] │
                    └─────────────────────────────────────┘
```

### 服务说明

| 服务 | 端口 | 说明 |
|------|------|------|
| caddy | 80, 443 | 反向代理，自动 SSL 证书 |
| zenstats | 8080 | Go 应用服务 |
| zenstats_db | 5432 | PostgreSQL 数据库 |
| zenstats_events_db | 8123, 9000 | ClickHouse 分析数据库 |

### 数据持久化

| 卷名 | 用途 |
|------|------|
| db-data | PostgreSQL 数据 |
| event-data | ClickHouse 事件数据 |
| event-logs | ClickHouse 日志 |
| zenstats-data | 应用数据（GeoIP 等） |
| caddy-data | Caddy SSL 证书 |
| caddy-config | Caddy 配置 |
