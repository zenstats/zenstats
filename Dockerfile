# ---- Build Stage ----
FROM golang:1.24-alpine AS builder

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
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

# ---- Runtime Stage ----
FROM alpine:3.20

# 配置镜像源以提升网络稳定性
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata wget

# 创建非 root 用户
RUN addgroup -S zenstats && adduser -S zenstats -G zenstats

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/bin/zenstats /app/zenstats
COPY --from=builder /build/config/config_prod.yaml /app/config/config_prod.yaml

# 创建数据目录
RUN mkdir -p /app/data && chown -R zenstats:zenstats /app

USER zenstats

# 暴露 HTTP 端口
EXPOSE 8080

# 设置环境变量
ENV APP_ENV=prod
ENV GIN_MODE=release

ENTRYPOINT ["/app/zenstats"]
CMD ["server"]