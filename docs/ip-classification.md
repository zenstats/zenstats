# IP 分类过滤配置指南

## 工作原理

`isDatacenterOrThreatIP()`（`internal/api/external/externalevent.go`）按优先级检查两个请求头：

```
X-Zenstats-Ip-Type  →  自定义，适用任何反向代理
       ↓ 为空则回退
Cf-Ip-Type           →  Cloudflare 自动设置
```

### 取值含义

| 值 | 含义 | 处理 |
|----|------|------|
| `dc_ip` | 数据中心/云服务商 IP（阿里云、腾讯云、AWS、GCP 等） | 丢弃，返回 `202 ok`，不写入 ClickHouse |
| `threat_ip` | 已知威胁 IP | 丢弃，返回 `202 ok`，不写入 ClickHouse |
| 其他值 / 空 | 普通访客 IP | 正常处理 |

---

## 环境配置对照

### ① Cloudflare 代理（生产推荐）

**无需任何配置**。Cloudflare 会在所有回源请求中自动附带 `Cf-Ip-Type` 请求头：

```
Cf-Ip-Type: dc_ip       # 数据中心 IP
Cf-Ip-Type: threat_ip   # 已知威胁 IP
```

只需确保域名在 Cloudflare 开启了代理（DNS 记录点亮橙色云朵 `☁️`）。

> **验证**：确认回源请求中包含 Cf-Ip-Type 头：
> ```bash
> curl -sI https://yourdomain.com/api/health | grep -i cf-ip-type
> ```

---

### ② 自定义反向代理

需要手动在反向代理层识别并设置 `X-Zenstats-Ip-Type` 头。

#### Caddy

编辑 `Caddyfile`，在 `reverse_proxy` 之前添加 `header_up`：

```caddy
# 使用 Caddy 内置的 remote_ip 匹配（CIDR 白名单/黑名单）
# 注意：remote_ip 只能做 CIDR 匹配，无法判断"是否是数据中心 IP"
# 完整的数据中心 IP 识别通常需要 Cloudflare 或其他 IP 分类服务
handle /api/event {
    @datacenter {
        # 已知的云服务商/数据中心 IP 范围（示例，按需维护）
        remote_ip 10.0.0.0/8 172.16.0.0/12 192.168.0.0/16
        # 更多 CIDR 可按行追加
    }
    header @datacenter X-Zenstats-Ip-Type "dc_ip"
    reverse_proxy zenstats:8080
}
```

#### Nginx

```nginx
location /api/event {
    # 如果使用 Cloudflare 回源，直接透传
    proxy_set_header X-Zenstats-Ip-Type $http_cf_ip_type;

    # 或使用 ngx_http_geo_module + 自定义 IP 库
    # geo $ip_type {
    #     include /etc/nginx/datacenter-ips.conf;  # 内容格式: 10.0.0.0/8 dc_ip;
    #     default "";
    # }
    # proxy_set_header X-Zenstats-Ip-Type $ip_type;

    proxy_pass http://zenstats:8080;
}
```

#### Traefik

```yaml
# docker-compose.yml labels 或动态配置
labels:
  - "traefik.http.middlewares.ip-filter.headers.customrequestheaders.X-Zenstats-Ip-Type=dc_ip"
  - "traefik.http.routers.zenstats.middlewares=ip-filter"
```

---

### ③ 本地开发 / Docker Compose

**默认不启用**。所有请求来自 localhost，无相关请求头，`isDatacenterOrThreatIP()` 返回 `false`，所有事件正常入库。

如需本地测试 IP 过滤功能：

```bash
# 模拟数据中心 IP（应返回 202，事件被静默丢弃）
curl -X POST http://localhost:8080/api/event \
  -H "X-Zenstats-Ip-Type: dc_ip" \
  -H "Content-Type: application/json" \
  -d '{"n":"pageview","u":"https://example.com/page","d":"example.com"}'

# 预期: HTTP 202
# 日志: "dropping datacenter/threat IP event"
```

---

### ④ Kubernetes

#### 使用 Cloudflare + k8s（标准方案）

```
用户 → Cloudflare（自动设 Cf-Ip-Type）→ k8s Ingress → Service → Zenstats Pod
```

Cloudflare 到 Ingress 的回源请求自动携带 `Cf-Ip-Type` 头，代码自动识别，**零配置**。

#### nginx-ingress + 自定义 IP 分类

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/configuration-snippet: |
      # Cloudflare 回源时透传
      proxy_set_header X-Zenstats-Ip-Type $http_cf_ip_type;
spec:
  rules:
  - host: yourdomain.com
    http:
      paths:
      - path: /api/event
        pathType: Prefix
        backend:
          service:
            name: zenstats
            port:
              number: 8080
```

---

## 配置决策树

```
你的生产环境使用什么？
│
├── Cloudflare（DNS 开启代理 ☁️）
│   └── ✅ 零配置。自动携带 Cf-Ip-Type，代码自动识别
│
├── 自定义反向代理（Caddy / Nginx / Traefik）
│   ├── 能访问 IP 分类服务（MaxMind / ipinfo.io 等）？
│   │   └── 在反向代理层编写规则，设置 X-Zenstats-Ip-Type
│   └── 无法分类 IP？
│       └── 不设该头即可，代码跳过过滤，所有事件正常入库
│
├── Kubernetes + Cloudflare
│   └── ✅ 零配置。Cf-Ip-Type 透传到 Pod
│
├── Kubernetes + 自定义 Ingress
│   └── 同自定义反向代理方案
│
└── 本地开发
    └── ✅ 默认不启用，可手动模拟测试
```

---

## 验证方法

无论哪种环境，验证逻辑一致：

```bash
# 1. 模拟数据中心 IP（应返回 202，事件被丢弃）
curl -v -X POST https://yourdomain.com/api/event \
  -H "X-Zenstats-Ip-Type: dc_ip" \
  -H "Content-Type: application/json" \
  -d '{"n":"pageview","u":"https://example.com/","d":"example.com"}'
# 预期: HTTP 202
# 日志: "dropping datacenter/threat IP event"

# 2. 模拟威胁 IP
curl -v -X POST https://yourdomain.com/api/event \
  -H "X-Zenstats-Ip-Type: threat_ip" \
  -H "Content-Type: application/json" \
  -d '{"n":"pageview","u":"https://example.com/","d":"example.com"}'
# 预期: HTTP 202
# 日志: "dropping datacenter/threat IP event"

# 3. 正常请求（应正常入库）
curl -v -X POST https://yourdomain.com/api/event \
  -H "Content-Type: application/json" \
  -d '{"n":"pageview","u":"https://example.com/","d":"example.com"}'
# 预期: HTTP 202，事件正常写入 ClickHouse

# 4. 查看实时日志
docker compose logs zenstats | grep -i "datacenter\|threat"
```
