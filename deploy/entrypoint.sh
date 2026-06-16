#!/bin/sh
set -e

# ============================================================
# ZenStats 容器入口脚本
# 1. 将预下载的 GeoIP 种子数据复制到持久化数据目录（首次启动）
# 2. 等待数据库就绪并自动执行迁移
# 3. 启动应用服务
# ============================================================

MAX_RETRIES=10
RETRY_INTERVAL=3
DATA_DIR="/app/data"
GEOIP_SEED_DIR="/app/geoip-seed"

echo "=== ZenStats Entrypoint ==="

# ---- 初始化持久化数据目录 ----
# 将预下载的 GeoIP 数据库复制到 volume 挂载的数据目录（仅当文件不存在时）
if [ -d "$GEOIP_SEED_DIR" ] && [ -n "$(ls -A $GEOIP_SEED_DIR 2>/dev/null)" ]; then
	echo "Initializing data directory with seed files..."
	for f in "$GEOIP_SEED_DIR"/*; do
		fname=$(basename "$f")
		if [ ! -f "$DATA_DIR/$fname" ]; then
			cp "$f" "$DATA_DIR/$fname"
			echo "  Copied $fname to $DATA_DIR/"
		fi
	done
fi

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
