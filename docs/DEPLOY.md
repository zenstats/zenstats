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
- `ZENSTATS_MAXMIND_LICENSE_KEY` — MaxMind GeoIP License Key
- `ZENSTATS_DOMAIN` — 你的域名（如 `stats.example.com`）

Caddy 会自动为你的域名申请 Let's Encrypt SSL 证书，无需手动配置证书。
如果 `ZENSTATS_DOMAIN` 未设置，默认使用 `localhost`（自签名证书）。

### 3. 启动全部服务

```bash
make docker-up
```

此命令会自动构建 Docker 镜像并启动以下服务：

| 服务 | 端口 | 说明 |
|------|------|------|
| caddy | 80, 443 | Caddy 反向代理（自动 SSL） |
| zenstats | 内部 8080 | ZenStats 应用服务 |
| zenstats_db | 内部 5432 | PostgreSQL 数据库 |
| zenstats_events_db | 内部 8123/9000 | ClickHouse 事件存储 |

如果设置了 `ZENSTATS_DOMAIN`，服务将通过 `https://your.domain.com` 访问，SSL 证书全自动管理。

### 4. 执行数据库迁移

```bash
make docker-migrate
```

此命令会：
- 创建数据库表结构
- 创建默认管理员账户（参见 `config/config_prod.yaml` 中的 `default_user` 配置）
- 初始化搜索引擎数据

### 5. 访问服务

- 如果设置了域名：`https://your.domain.com`（自动 SSL）
- 本地测试：`http://localhost`（通过 Caddy 代理到 zenstats:8080）
- 默认账户：参见 `config/config_prod.yaml`

> **SSL 说明**：Caddy 会自动为你的域名申请和续期 Let's Encrypt 证书。只需确保域名 DNS 已解析到服务器 IP，且 80/443 端口可从外网访问。

### 6. 查看日志

```bash
make docker-logs
```

### 7. 停止服务

```bash
make docker-down
```

---

## 手动部署

### 1. 安装依赖

```bash
go mod download
```

### 2. 配置文件

编辑 `config/config_prod.yaml` 或通过环境变量配置：

```yaml
scheme:
  address: 0.0.0.0
  http_port: 8080

db:
  host: localhost
  port: 5432
  username: postgres
  password: "your_password"
  database: "zenstats"

clickhouse:
  addr:
    - localhost:9000
  database: zenstats_events_db
  username: default
  password: ""
  ssl: false

maxmind_license_key: "your_license_key"
secret_key: "your_secret_key"
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

---

## 环境变量

所有配置项均可通过环境变量覆盖，前缀为 `ZENSTATS_`：

| 环境变量 | 说明 | 示例 |
|----------|------|------|
| `APP_ENV` | 运行环境（dev/prod） | `prod` |
| `ZENSTATS_MAXMIND_LICENSE_KEY` | **必须** MaxMind License Key | `xxx` |
| `ZENSTATS_SECRET_KEY` | JWT 签名密钥 | `your_key` |
| `ZENSTATS_DB_HOST` | PostgreSQL 主机 | `localhost` |
| `ZENSTATS_DB_PASSWORD` | PostgreSQL 密码 | `postgres` |
| `ZENSTATS_DB_USERNAME` | PostgreSQL 用户名 | `postgres` |
| `ZENSTATS_CLICKHOUSE_ADDR` | ClickHouse 地址（支持 JSON 数组或逗号分隔） | `localhost:9000` |
| `ZENSTATS_CLICKHOUSE_USERNAME` | ClickHouse 用户名 | `default` |
| `ZENSTATS_CLICKHOUSE_PASSWORD` | ClickHouse 密码 | |

---

## 数据库迁移

首次部署或升级时需要执行迁移：

```bash
# Docker 环境
make docker-migrate

# 本地环境
./bin/zenstats migrate
```

迁移操作会：
1. 自动创建/更新数据库表结构（PostgreSQL）
2. 创建默认管理员账户（如果不存在）
3. 初始化搜索引擎数据（如果表为空）

---

## 常见问题

### Q: ClickHouse 启动失败，提示 "Address family for hostname not supported"
A: Docker 默认不支持 IPv6。`deploy/clickhouse/ipv4-only.xml` 配置已解决此问题，确保使用 `deploy/docker-compose.yml` 启动。

### Q: MaxMind GeoIP 数据库下载失败
A: 需要有效的 MaxMind License Key。在 https://dev.maxmind.com/geoip/geolite2-free-geolocation-data 免费注册获取。

### Q: 如何修改默认管理员账户？
A: 编辑 `config/config_prod.yaml` 中的 `default_user` 配置，然后重新执行 `make docker-migrate` 或 `./bin/zenstats migrate`。

### Q: 如何查看 ZenStats 的 API 文档？
A: 启动后访问 Swagger UI：
```bash
go run main.go doc
# 然后打开 http://localhost:8081/swagger/index.html