# ---- Build Stage ----
FROM golang:1.25-alpine AS builder

ARG TARGETARCH
ARG APK_MIRROR=""
RUN if [ -n "$APK_MIRROR" ]; then \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories; \
    fi && \
    apk add --no-cache git ca-certificates tzdata

WORKDIR /build

ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /build/bin/zenstats .

# ---- Runtime Stage ----
FROM --platform=$TARGETPLATFORM alpine:3.22

ARG APK_MIRROR=""
RUN if [ -n "$APK_MIRROR" ]; then \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories; \
    fi && \
    apk add --no-cache ca-certificates tzdata

RUN addgroup -S zenstats && adduser -S zenstats -G zenstats

WORKDIR /app

COPY --from=builder /build/bin/zenstats /app/zenstats
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

RUN mkdir -p /app/data && chown -R zenstats:zenstats /app

USER zenstats

EXPOSE 8080

ENV APP_ENV=prod
ENV GIN_MODE=release

ENTRYPOINT ["/app/entrypoint.sh"]
