#!/usr/bin/env bash
set -euo pipefail

DEFAULT_REMOTE_HOST="shinecar@120.26.33.155"
DEFAULT_REMOTE_DIR="/data/shinecar/deploy/prestage"

REMOTE_HOST="${PRESTAGE_REMOTE_HOST:-${DEFAULT_REMOTE_HOST}}"
REMOTE_DIR="${PRESTAGE_REMOTE_DIR:-${DEFAULT_REMOTE_DIR}}"
BINARY_NAME="shop_server"
LOCAL_DIST="dist/prestage"
LOCAL_BINARY="${LOCAL_DIST}/${BINARY_NAME}"

shops=(
  "001:pre-shop-server-001"
  "002:pre-shop-server-002"
)

log() {
  printf '[deploy-staging][shop_server] %s\n' "$*"
}

die() {
  printf '[deploy-staging][shop_server][ERROR] %s\n' "$*" >&2
  exit 1
}

remote_exec() {
  ssh "${REMOTE_HOST}" "$@"
}

log "检查远端 deploy 目录: ${REMOTE_HOST}:${REMOTE_DIR}"
remote_exec "test -d '${REMOTE_DIR}' && test -x '${REMOTE_DIR}/scripts/restart.sh'" \
  || die "远端 deploy 目录未初始化，请先部署 shinecar-deploy 到 ${REMOTE_DIR}"

version="$(git describe --tags --always --dirty 2>/dev/null || git rev-parse --short HEAD)"
build_time="$(date -u '+%Y-%m-%d_%H:%M:%S')"
ldflags="-X main.Version=${version} -X main.BuildTime=${build_time}"

log "构建 Linux 二进制: ${LOCAL_BINARY}, version=${version}"
mkdir -p "${LOCAL_DIST}"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${ldflags}" -o "${LOCAL_BINARY}" ./cmd

log "上传二进制到 ${REMOTE_HOST}:${REMOTE_DIR}/runtime/shop-server"
for item in "${shops[@]}"; do
  shop_id="${item%%:*}"
  service_name="${item##*:}"
  remote_binary="${REMOTE_DIR}/runtime/shop-server/${shop_id}/${BINARY_NAME}"

  log "上传 ${service_name}: ${remote_binary}"
  remote_exec "mkdir -p '${REMOTE_DIR}/runtime/shop-server/${shop_id}'"
  scp "${LOCAL_BINARY}" "${REMOTE_HOST}:${remote_binary}.new"
  remote_exec "chmod +x '${remote_binary}.new' && mv '${remote_binary}.new' '${remote_binary}'"
done

restart_services=()
for item in "${shops[@]}"; do
  restart_services+=("${item##*:}")
done

if remote_exec "cd '${REMOTE_DIR}' && [ -n \"\$(./scripts/compose.sh ps -q pre-shop-server-001 2>/dev/null)\" ]"; then
  log "重启远端服务: ${restart_services[*]}"
  remote_exec "cd '${REMOTE_DIR}' && ./scripts/restart.sh ${restart_services[*]} && ./scripts/ps.sh ${restart_services[*]}"
else
  log "远端 shop_server 容器尚未创建，已完成二进制上传；首次启动请在 deploy 项目执行 ./scripts/up.sh"
fi
