#!/bin/sh
set -e

# ============================================================
# ZenStats 容器入口脚本
# 1. 等待数据库就绪并自动执行迁移
# 2. 启动应用服务（GeoIP 数据库将在首次运行时自动下载）
# ============================================================

MAX_RETRIES=10
RETRY_INTERVAL=3

echo "=== ZenStats Entrypoint ==="

# ---- 等待 PostgreSQL 并执行迁移 ----
echo "Waiting for PostgreSQL (${ZENSTATS_DB_HOST}:${ZENSTATS_DB_PORT:-5432})..."
i=0
while [ $i -lt $MAX_RETRIES ]; do
	if /app/zenstats migrate 2>/dev/null; then
		echo "Migration completed successfully."
		break
	fi
	i=$((i + 1))
	if [ $i -lt $MAX_RETRIES ]; then
		echo "Migration attempt $i failed, retrying in ${RETRY_INTERVAL}s..."
		sleep $RETRY_INTERVAL
	else
		echo "ERROR: Migration failed after $MAX_RETRIES attempts."
		exit 1
	fi
done

echo "=== Starting ZenStats Server ==="
exec /app/zenstats server
