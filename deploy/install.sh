#!/bin/sh
set -e

# ================================================================
# ZenStats — 交互式一键部署脚本
#
# 用法:
#   curl -fsSL https://raw.githubusercontent.com/zenstats/zenstats/main/deploy/install.sh | sh -s -- -i
# 或:
#   chmod +x install.sh && ./install.sh -i
#
# 非交互模式（适合 CI/CD）:
#   ZENSTATS_DOMAIN=stats.example.com ./install.sh
# ================================================================

BOLD="\033[1m"
GREEN="\033[32m"
YELLOW="\033[33m"
RED="\033[31m"
CYAN="\033[36m"
RESET="\033[0m"

echo ""
echo "${BOLD}${GREEN}  ⚗  ZenStats — Privacy-First Web Analytics${RESET}"
echo "${BOLD}  Interactive Installer${RESET}"
echo ""

# ---- 系统检测 ----
if ! command -v docker >/dev/null 2>&1; then
    echo "${RED}✗ Docker 未安装。请先安装: https://docs.docker.com/get-docker/${RESET}"
    exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
    echo "${RED}✗ Docker Compose 未安装或版本过低（需要 Docker 24+）。${RESET}"
    exit 1
fi

INTERACTIVE=false
# 检测是否为交互式终端（stdin 是 tty）
if [ -t 0 ]; then
    INTERACTIVE=true
fi

# 支持 -i 强制交互
if [ "$1" = "-i" ]; then
    INTERACTIVE=true
fi

# ---- 交互式问答 ----
if $INTERACTIVE; then
    echo "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo "${BOLD}  基础配置${RESET}"
    echo "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo ""

    # 域名
    printf "${BOLD}  1. 域名${RESET} (例如 stats.example.com)\n"
    printf "     用于自动申请 SSL 证书，本地测试可直接回车使用 localhost\n"
    printf "     ${CYAN}域名:${RESET} "
    read DOMAIN_INPUT
    ZENSTATS_DOMAIN="${DOMAIN_INPUT:-localhost}"
    echo ""

    # MaxMind key
    printf "${BOLD}  2. MaxMind GeoIP License Key${RESET}\n"
    printf "     免费注册: ${CYAN}https://dev.maxmind.com/geoip/geolite2-free-geolocation-data${RESET}\n"
    printf "     用于精确的访客地理定位，跳过则使用免费备用数据库\n"
    printf "     ${CYAN}Key (回车跳过):${RESET} "
    read MAXMIND_INPUT
    if [ -n "$MAXMIND_INPUT" ]; then
        ZENSTATS_MAXMIND_LICENSE_KEY="$MAXMIND_INPUT"
    fi
    echo ""

    # 安装目录
    printf "${BOLD}  3. 安装目录${RESET} (默认 ./zenstats)\n"
    printf "     ${CYAN}目录:${RESET} "
    read DIR_INPUT
    INSTALL_DIR="${DIR_INPUT:-./zenstats}"
    echo ""

    echo "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo "${BOLD}  安全配置${RESET}"
    echo "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo ""

    # PG 密码
    printf "${BOLD}  4. PostgreSQL 密码${RESET} (回车随机生成)\n"
    printf "     ${CYAN}密码:${RESET} "
    read PG_INPUT
    if [ -n "$PG_INPUT" ]; then
        DB_PASSWORD="$PG_INPUT"
    else
        DB_PASSWORD=$(openssl rand -base64 24 2>/dev/null || cat /dev/urandom | tr -dc 'a-zA-Z0-9' | head -c 24)
        echo "     ${YELLOW}→ 已生成随机密码${RESET}"
    fi
    echo ""

    # JWT 密钥
    printf "${BOLD}  5. JWT 签名密钥${RESET} (回车随机生成)\n"
    printf "     用于加密用户会话 Token\n"
    printf "     ${CYAN}密钥:${RESET} "
    read JWT_INPUT
    if [ -n "$JWT_INPUT" ]; then
        ZENSTATS_SECRET_KEY="$JWT_INPUT"
    else
        ZENSTATS_SECRET_KEY=$(openssl rand -base64 32 2>/dev/null || cat /dev/urandom | tr -dc 'a-zA-Z0-9' | head -c 32)
        echo "     ${YELLOW}→ 已生成随机密钥${RESET}"
    fi
    echo ""

    # ---- 确认 ----
    echo "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo "${BOLD}  确认配置${RESET}"
    echo "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo "  域名:       ${GREEN}${ZENSTATS_DOMAIN}${RESET}"
    echo "  安装目录:   ${GREEN}${INSTALL_DIR}${RESET}"
    if [ -n "$ZENSTATS_MAXMIND_LICENSE_KEY" ]; then
        echo "  GeoIP:      ${GREEN}已配置${RESET}"
    else
        echo "  GeoIP:      ${YELLOW}未配置（免费备用数据库）${RESET}"
    fi
    echo "  PG 密码:    ${GREEN}$(echo "$DB_PASSWORD" | cut -c1-8)...${RESET}"
    echo "  JWT 密钥:   ${GREEN}$(echo "$ZENSTATS_SECRET_KEY" | cut -c1-8)...${RESET}"
    echo ""
    printf "${BOLD}  确认开始安装? [Y/n]:${RESET} "
    read CONFIRM
    if [ "$CONFIRM" = "n" ] || [ "$CONFIRM" = "N" ] || [ "$CONFIRM" = "no" ]; then
        echo "${YELLOW}  已取消。${RESET}"
        exit 0
    fi
    echo ""
else
    # ---- 非交互模式（CI / 环境变量） ----
    ZENSTATS_DOMAIN="${ZENSTATS_DOMAIN:-localhost}"
    INSTALL_DIR="${INSTALL_DIR:-./zenstats}"
    DB_PASSWORD="${DB_PASSWORD:-$(openssl rand -base64 24 2>/dev/null || cat /dev/urandom | tr -dc 'a-zA-Z0-9' | head -c 24)}"
    if [ -z "$ZENSTATS_SECRET_KEY" ]; then
        ZENSTATS_SECRET_KEY=$(openssl rand -base64 32 2>/dev/null || cat /dev/urandom | tr -dc 'a-zA-Z0-9' | head -c 32)
    fi
fi

# ---- 下载/更新项目 ----
if [ ! -d "$INSTALL_DIR" ]; then
    echo "${GREEN}→ 下载项目到 ${INSTALL_DIR}...${RESET}"
    git clone https://github.com/zenstats/zenstats.git "$INSTALL_DIR" 2>/dev/null || {
        echo "${RED}Git clone 失败。请确认网络连接或手动下载项目。${RESET}"
        exit 1
    }
else
    echo "${GREEN}→ 目录已存在，更新项目...${RESET}"
    cd "$INSTALL_DIR"
    if [ -d .git ]; then
        git pull --ff-only 2>/dev/null || true
    fi
fi

cd "$INSTALL_DIR"

# ---- 初始化子模块 ----
echo "${GREEN}→ 初始化前端子模块...${RESET}"
git submodule update --init --recursive 2>/dev/null || true

# ---- 写入 .env ----
echo "${GREEN}→ 生成环境配置...${RESET}"
mkdir -p deploy
cat > deploy/.env <<EOF
# ZenStats 部署环境配置 — 由 install.sh 生成 $(date -u +"%Y-%m-%dT%H:%M:%SZ")
ZENSTATS_DOMAIN=${ZENSTATS_DOMAIN}
ZENSTATS_SECRET_KEY=${ZENSTATS_SECRET_KEY}
ZENSTATS_MAXMIND_LICENSE_KEY=${ZENSTATS_MAXMIND_LICENSE_KEY:-}
DB_PASSWORD=${DB_PASSWORD}
EOF

echo "  ✓ deploy/.env 已生成"

# ---- 构建 & 启动 ----
echo ""
echo "${GREEN}→ 构建 Docker 镜像并启动服务...${RESET}"
cd deploy
docker compose up -d --build 2>&1 | while IFS= read -r line; do
    echo "  $line"
done

echo ""
echo "${GREEN}${BOLD}============================================${RESET}"
echo "${GREEN}${BOLD}  ✓ ZenStats 部署完成！${RESET}"
echo "${GREEN}${BOLD}============================================${RESET}"
echo ""
if [ "$ZENSTATS_DOMAIN" != "localhost" ]; then
    echo "  访问地址:  ${BOLD}https://${ZENSTATS_DOMAIN}${RESET}"
    echo "  (SSL 证书由 Caddy 自动申请，约 1-2 分钟生效)"
else
    echo "  访问地址:  ${BOLD}http://localhost${RESET}"
fi
echo ""
echo "  ${BOLD}管理命令:${RESET}"
echo "    cd ${INSTALL_DIR}/deploy"
echo "    docker compose logs -f          # 查看日志"
echo "    docker compose ps               # 服务状态"
echo "    docker compose restart          # 重启"
echo "    docker compose down             # 停止"
echo "    make prod-clean                 # 停止并清除数据"
echo ""
echo "  首次打开请完成设置向导创建管理员账号。"
echo ""
