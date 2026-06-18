# syntax=docker/dockerfile:1.7
#
# ZenStats Go Backend — 多阶段构建，最终镜像约 15MB
#
# 构建: docker build -t zenstats .
# 运行: docker compose -f deploy/docker-compose.yml up

# ---- Stage 1: Build Go binary ----
FROM golang:1.24-alpine AS builder

ARG GOPROXY=""
ARG APK_MIRROR=""

RUN if [ -n "$APK_MIRROR" ]; then \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories; \
    fi && \
    apk add --no-cache git ca-certificates tzdata

WORKDIR /build

ENV GOPROXY=${GOPROXY}
ENV CGO_ENABLED=0
ENV GOOS=linux

# 利用 Docker BuildKit 缓存加速依赖下载
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-s -w -X main.Version=1.0.0" -o /build/zenstats .

# ---- Stage 2: Minimal runtime ----
FROM alpine:3.21

ARG APK_MIRROR=""

RUN if [ -n "$APK_MIRROR" ]; then \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories; \
    fi && \
    apk add --no-cache ca-certificates tzdata

# 创建非 root 用户
RUN addgroup -S zenstats && adduser -S zenstats -G zenstats

WORKDIR /app

# 复制仅运行时需要的文件
COPY --from=builder /build/zenstats /app/zenstats
COPY --from=builder /build/config/config_prod.yaml /app/config/config_prod.yaml
COPY deploy/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# 数据持久化目录（volume 挂载）
RUN mkdir -p /app/data && chown -R zenstats:zenstats /app

USER zenstats

EXPOSE 8080
ENV APP_ENV=prod GIN_MODE=release

ENTRYPOINT ["/app/entrypoint.sh"]
