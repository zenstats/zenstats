# ---- Build Stage ----
FROM golang:1.24-alpine AS builder

ARG APK_MIRROR=""
RUN if [ -n "$APK_MIRROR" ]; then \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories; \
    fi && \
    apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 配置 Go 模块代理（加速依赖下载）
ENV GOPROXY=https://goproxy.cn,direct

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bin/zenstats .

# ---- GeoIP download Stage ----
FROM alpine:3.20 AS geoip-downloader

ARG APK_MIRROR=""
ARG GEOIP_MIRROR=""
RUN if [ -n "$APK_MIRROR" ]; then \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories; \
    fi && \
    apk add --no-cache wget
RUN mkdir -p /geoip-seed && \
    if [ -n "$GEOIP_MIRROR" ]; then \
      URL="${GEOIP_MIRROR}https://raw.githubusercontent.com/Loyalsoldier/geoip/release/Country-without-asn.mmdb"; \
    else \
      URL="https://raw.githubusercontent.com/Loyalsoldier/geoip/release/Country-without-asn.mmdb"; \
    fi && \
    wget --timeout=30 -O /geoip-seed/GeoLite2-City-fallback.mmdb "$URL" || \
    echo "GeoIP fallback download failed, will download at runtime"

# ---- Runtime Stage ----
FROM alpine:3.20

ARG APK_MIRROR=""
RUN if [ -n "$APK_MIRROR" ]; then \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories; \
    fi && \
    apk add --no-cache ca-certificates tzdata

# 创建非 root 用户
RUN addgroup -S zenstats && adduser -S zenstats -G zenstats

WORKDIR /app

# 从构建阶段复制二进制文件和配置
COPY --from=builder /build/bin/zenstats /app/zenstats
COPY --from=builder /build/config/config_prod.yaml /app/config/config_prod.yaml

# 预下载的 GeoIP 数据库种子文件（入口脚本会在首次启动时复制到数据目录）
COPY --from=geoip-downloader /geoip-seed/ /app/geoip-seed/

# 入口脚本：自动 migrate + 启动服务
COPY deploy/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# 创建必要目录并设置权限
RUN mkdir -p /app/data /app/geoip-seed && chown -R zenstats:zenstats /app

USER zenstats

# 暴露 HTTP 端口
EXPOSE 8080

# 设置环境变量
ENV APP_ENV=prod
ENV GIN_MODE=release

ENTRYPOINT ["/app/entrypoint.sh"]
