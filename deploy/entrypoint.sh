#!/bin/sh
set -e

# ============================================================
# ZenStats 容器入口脚本
# 1. 等待 PostgreSQL 就绪并执行数据库迁移（最多重试 10 次）
# 2. 下载 GeoIP 数据库种子（如不存在）
# 3. 启动应用
# ============================================================

MAX_RETRIES=10
RETRY_INTERVAL=3
DATA_DIR="${DATA_DIR:-/app/data}"

echo "=== ZenStats Entrypoint ==="

# ---- 下载 GeoIP 数据库（仅首次，并仅当 MaxMind key 已配置） ----
if [ -n "$ZENSTATS_MAXMIND_LICENSE_KEY" ]; then
    # MaxMind 会在程序启动时自动下载
    echo "MaxMind license key configured — GeoIP auto-download enabled."
fi

# ---- 等待 PostgreSQL 并执行迁移 ----
echo "Waiting for PostgreSQL (${ZENSTATS_DB_HOST}:${ZENSTATS_DB_PORT:-5432})..."
i=0
while [ $i -lt $MAX_RETRIES ]; do
    if /app/zenstats migrate 2>/dev/null; then
        echo "Migration completed."
        break
    fi
    i=$((i + 1))
    if [ $i -lt $MAX_RETRIES ]; then
        echo "  Retry $i/$MAX_RETRIES in ${RETRY_INTERVAL}s..."
        sleep $RETRY_INTERVAL
    else
        echo "ERROR: Migration failed after $MAX_RETRIES attempts."
        exit 1
    fi
done

echo "=== Starting ZenStats Server ==="
exec /app/zenstats server
