# ZenStats 部署指南

## 目录

- [环境要求](#环境要求)
- [快速开始（Docker Compose）](#快速开始docker-compose)
- [手动部署](#手动部署)
- [环境变量](#环境变量)
- [数据库迁移](#数据库迁移)
- [常见问题](#常见问题)

---

## 环境要求

### Docker 部署
- Docker 20.10+
- Docker Compose v2+

### 手动部署
- Go 1.24+
- PostgreSQL 16+
- ClickHouse 24.12+
- MaxMind GeoLite2 License Key（免费注册获取：https://dev.maxmind.com/geoip/geolite2-free-geolocation-data）

---

## 快速开始（Docker Compose）

### 1. 克隆项目

```bash
git clone https://git.potawang.cn/zenstats/zenstats.git
cd zenstats
```

### 2. 配置环境变量

```bash
# 复制环境变量模板
cp deploy/.env.example deploy/.env

# 编辑 .env 文件
vi deploy/.env
```

必须配置的变量：
- `ZENSTATS_MAXMIND_LICENSE_KEY` — MaxMind GeoIP License Key（不填则使用免费 GeoIP 回退）
- `ZENSTATS_DOMAIN` — 你的域名（如 `stats.example.com`）
- `ZENSTATS_SECRET_KEY` — JWT 签名密钥（生成方式：`openssl rand -base64 32`）
- `DB_PASSWORD` — 数据库密码（生产务必修改）

### 3. 启动全部服务

```bash
make prod-up
```

此命令会自动构建 Docker 镜像并启动以下服务：

| 服务 | 端口 | 说明 |
|------|------|------|
| caddy | 80, 443 | Caddy 反向代理（自动 SSL）+ 前端静态文件 |
| zenstats | 内部 8080 | ZenStats 应用服务 |
| zenstats_db | 内部 5432 | PostgreSQL 数据库 |
| zenstats_events_db | 内部 8123/9000 | ClickHouse 事件存储 |

> **注意**：容器启动时会**自动执行数据库迁移**（通过 `entrypoint.sh`），无需手动运行 `make docker-migrate`。首次启动需要等待 PG 和 CH 健康检查通过。

如果设置了 `ZENSTATS_DOMAIN`，服务将通过 `https://your.domain.com` 访问，SSL 证书全自动管理。

### 4. 访问服务

- 如果设置了域名：`https://your.domain.com`（自动 SSL）
- 本地测试：`http://localhost`（通过 Caddy 代理）
- 首次访问会跳转到系统初始化页面，创建管理员账户

> **SSL 说明**：Caddy 会自动为你的域名申请和续期 Let's Encrypt 证书。只需确保域名 DNS 已解析到服务器 IP，且 80/443 端口可从外网访问。

### 5. 查看日志

```bash
make prod-logs
```

### 6. 停止服务

```bash
make prod-down
```

### 开发环境

开发环境会额外暴露数据库端口到宿主机：

```bash
make dev-up       # 启动（暴露 PG:5432, CH:8123/9000, App:8080）
make dev-logs     # 查看日志
make dev-down     # 停止
make dev-clean    # 停止并清理数据
```

---

## 手动部署

### 1. 安装依赖

```bash
go mod download
make submodule-init  # 初始化前端子模块
```

### 2. 配置文件

编辑 `config/config_prod.yaml` 或通过 `ZENSTATS_` 环境变量覆盖（推荐）：

```bash
export APP_ENV=prod
export ZENSTATS_DB_HOST=localhost
export ZENSTATS_DB_PORT=5432
export ZENSTATS_DB_USERNAME=postgres
export ZENSTATS_DB_PASSWORD=your_password
export ZENSTATS_DB_DATABASE=zenstats
export ZENSTATS_CLICKHOUSE_ADDR=localhost:9000
export ZENSTATS_MAXMIND_LICENSE_KEY=your_key
export ZENSTATS_SECRET_KEY=your_secret_key
```

### 3. 构建

```bash
make build
```

### 4. 数据库迁移

```bash
./bin/zenstats migrate
```

### 5. 启动服务

```bash
./bin/zenstats server
```

服务默认监听 `0.0.0.0:8080`。

---

## 环境变量

所有配置项均可通过环境变量覆盖，前缀为 `ZENSTATS_`。嵌套键用 `_` 分隔（如 `ZENSTATS_DB_HOST` → `db.host`）。

| 环境变量 | 说明 | 示例 |
|----------|------|------|
| `APP_ENV` | 运行环境（dev/prod） | `prod` |
| `GIN_MODE` | Gin 运行模式 | `release` |
| `ZENSTATS_MAXMIND_LICENSE_KEY` | MaxMind License Key | `xxx` |
| `ZENSTATS_SECRET_KEY` | JWT 签名密钥（生产务必修改） | `your_key` |
| `ZENSTATS_DOMAIN` | 站点域名（→ base_url） | `stats.example.com` |
| `ZENSTATS_DB_HOST` | PostgreSQL 主机 | `zenstats_db` |
| `ZENSTATS_DB_PORT` | PostgreSQL 端口 | `5432` |
| `ZENSTATS_DB_USERNAME` | PostgreSQL 用户名 | `postgres` |
| `ZENSTATS_DB_PASSWORD` | PostgreSQL 密码 | `postgres` |
| `ZENSTATS_DB_DATABASE` | PostgreSQL 数据库名 | `zenstats` |
| `ZENSTATS_CLICKHOUSE_ADDR` | ClickHouse 地址 | `zenstats_events_db:9000` |
| `ZENSTATS_CLICKHOUSE_USERNAME` | ClickHouse 用户名 | `default` |
| `ZENSTATS_CLICKHOUSE_PASSWORD` | ClickHouse 密码 | |
| `ZENSTATS_LOG_LEVEL` | 日志级别 | `info` |
| `ZENSTATS_POOL_SIZE` | 事件处理协程池大小 | `100` |
| `ZENSTATS_SCHEME_HTTP_PORT` | HTTP 监听端口 | `8080` |
| `ZENSTATS_SMTP_HOST` | SMTP 主机 | `smtp.example.com` |
| `ZENSTATS_SMTP_PORT` | SMTP 端口 | `587` |
| `ZENSTATS_SMTP_USERNAME` | SMTP 用户名 | |
| `ZENSTATS_SMTP_PASSWORD` | SMTP 密码 | |
| `ZENSTATS_SMTP_FROM` | 发件人地址 | `noreply@example.com` |

---

## 数据库迁移

**Docker 环境**：容器启动时通过 `entrypoint.sh` 自动执行迁移，无需手动操作。

**手动环境**：

```bash
./bin/zenstats migrate
```

迁移操作会：
1. 自动创建/更新数据库表结构（PostgreSQL）
2. 插入默认用户组/套餐数据（如果表为空）
3. 初始化搜索引擎数据（如果表为空）
4. 初始化系统配置项

---

## 常见问题

### Q: ClickHouse 启动失败，提示 "Address family for hostname not supported"
A: Docker 默认不支持 IPv6。`deploy/clickhouse/ipv4-only.xml` 配置已解决此问题。

### Q: MaxMind GeoIP 数据库下载失败
A: 需要有效的 MaxMind License Key。在 https://dev.maxmind.com/geoip/geolite2-free-geolocation-data 免费注册获取。也可以不配置，系统会自动使用免费的 Loyalsoldier/geoip 数据库。

### Q: 如何查看 API 文档？
A: 启动后访问 Swagger UI：
```bash
# 本地开发
go run main.go doc
# 然后打开 http://localhost:8081/swagger/index.html
```

### Q: 国内 Docker 构建慢？
A: 在构建时指定 APK 镜像源：
```bash
docker compose build --build-arg APK_MIRROR=mirrors.aliyun.com
```
