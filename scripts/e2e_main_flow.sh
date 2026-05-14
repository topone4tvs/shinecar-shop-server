#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${BACKEND_DIR:-$(cd "$ROOT_DIR/.." && pwd)/shinecar-backend}"
SHOP_URL="${SHOP_URL:-http://localhost:8080}"
BACKEND_URL="${BACKEND_URL:-http://localhost:8989}"
STATION_ID="${STATION_ID:-001}"
LICENSE_PLATE="${LICENSE_PLATE:-浙A73J2W}"
SCENARIO="${SCENARIO:-gate-open-close}"
START_BACKEND=false
START_SHOP=false
SKIP_HEALTH=false
BACKEND_PID=""
SHOP_PID=""
BACKEND_BIN="${BACKEND_BIN:-/tmp/shinecar_backend_e2e}"
SHOP_BIN="${SHOP_BIN:-/tmp/shop_server_e2e}"

export GOCACHE="${GOCACHE:-/tmp/shop_server_go_cache}"

usage() {
  cat <<EOF
用法: $0 [选项]

选项:
  --start-backend       脚本内启动 shinecar-backend combined 服务
  --start-shop          脚本内启动 shop_server
  --start-all           同时启动 backend 和 shop_server
  --scenario <name>     场景: heartbeat/plate-recognition/gate-open/gate-close/gate-open-close/main-flow/all
  --station <id>        工位ID，默认 001
  --license <plate>     车牌号，默认 浙A73J2W
  --shop-url <url>      shop_server 地址，默认 http://localhost:8080
  --backend-url <url>   backend 地址，默认 http://localhost:8989
  --backend-dir <path>  backend 项目路径，默认 ../shinecar-backend
  --skip-health         跳过健康检查
  -h, --help            显示帮助

示例:
  $0 --scenario all
  $0 --start-shop --scenario gate-open-close
  $0 --start-all --scenario main-flow
EOF
}

log() {
  printf '[e2e] %s\n' "$*"
}

fail() {
  printf '[e2e][ERROR] %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [[ -n "$SHOP_PID" ]] && kill -0 "$SHOP_PID" 2>/dev/null; then
    log "停止 shop_server: pid=$SHOP_PID"
    kill -TERM "$SHOP_PID" 2>/dev/null || true
    wait "$SHOP_PID" 2>/dev/null || true
  fi
  if [[ -n "$BACKEND_PID" ]] && kill -0 "$BACKEND_PID" 2>/dev/null; then
    log "停止 shinecar-backend: pid=$BACKEND_PID"
    kill -TERM "$BACKEND_PID" 2>/dev/null || true
    wait "$BACKEND_PID" 2>/dev/null || true
  fi
  cleanup_leftover_port 8080 "$SHOP_BIN" "shop_server"
  cleanup_leftover_port 8989 "$BACKEND_BIN" "shinecar-backend"
}
trap cleanup EXIT

cleanup_leftover_port() {
  local port="$1"
  local expected_cmd="$2"
  local name="$3"
  local pids

  pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  [[ -n "$pids" ]] || return 0

  while IFS= read -r pid; do
    [[ -n "$pid" ]] || continue
    local command
    command="$(ps -p "$pid" -o command= 2>/dev/null || true)"
    if [[ "$command" == *"$expected_cmd"* ]]; then
      log "清理残留 $name 进程: pid=$pid port=$port"
      kill -TERM "$pid" 2>/dev/null || true
    else
      log "端口 $port 仍被其他进程占用，跳过清理: pid=$pid command=$command"
    fi
  done <<< "$pids"
}

wait_http() {
  local url="$1"
  local name="$2"
  local max_attempts="${3:-30}"

  log "等待 $name 健康检查: $url"
  for ((i = 1; i <= max_attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      log "$name 已就绪"
      return 0
    fi
    sleep 1
  done

  return 1
}

probe_url() {
  local url="$1"
  local name="$2"

  log "检查 $name: $url"
  if body="$(curl -fsS "$url" 2>/dev/null)"; then
    printf '%s\n' "$body"
  else
    log "$name 当前不可用或请求失败"
  fi
}

check_command() {
  command -v "$1" >/dev/null 2>&1 || fail "缺少命令: $1"
}

start_backend() {
  [[ -d "$BACKEND_DIR" ]] || fail "backend目录不存在: $BACKEND_DIR"

  log "构建 shinecar-backend E2E 二进制: $BACKEND_BIN"
  (
    cd "$BACKEND_DIR"
    go build -o "$BACKEND_BIN" ./cmd
  )

  log "启动 shinecar-backend combined: $BACKEND_DIR"
  (
    cd "$BACKEND_DIR"
    exec env APP_ENV=dev "$BACKEND_BIN" combined
  ) &
  BACKEND_PID="$!"
}

start_shop() {
  log "构建 shop_server E2E 二进制: $SHOP_BIN"
  (
    cd "$ROOT_DIR"
    go build -o "$SHOP_BIN" ./cmd
  )

  log "启动 shop_server: $ROOT_DIR"
  (
    cd "$ROOT_DIR"
    exec env APP_ENV=dev "$SHOP_BIN"
  ) &
  SHOP_PID="$!"
}

run_simulator() {
  log "执行模拟器场景: scenario=$SCENARIO station=$STATION_ID license=$LICENSE_PLATE"
  (
    cd "$ROOT_DIR/simulator"
    go run . \
      -base-url "$SHOP_URL" \
      -scenario "$SCENARIO" \
      -station "$STATION_ID" \
      -license "$LICENSE_PLATE"
  )
}

print_status_tips() {
  cat <<EOF

[e2e] 后续检查建议:
  shop_server 健康状态:
    curl -s ${SHOP_URL}/health

  shop_server 工位状态:
    curl -s ${SHOP_URL}/api/status/station/${STATION_ID}

  backend 健康状态:
    curl -s ${BACKEND_URL}/health

  如果 backend/shop_server 是手动启动的，请观察对应终端日志里的 MQTT 上报和指令下发。
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --start-backend)
      START_BACKEND=true
      shift
      ;;
    --start-shop)
      START_SHOP=true
      shift
      ;;
    --start-all)
      START_BACKEND=true
      START_SHOP=true
      shift
      ;;
    --scenario)
      SCENARIO="${2:-}"
      [[ -n "$SCENARIO" ]] || fail "--scenario 缺少值"
      shift 2
      ;;
    --station)
      STATION_ID="${2:-}"
      [[ -n "$STATION_ID" ]] || fail "--station 缺少值"
      shift 2
      ;;
    --license)
      LICENSE_PLATE="${2:-}"
      [[ -n "$LICENSE_PLATE" ]] || fail "--license 缺少值"
      shift 2
      ;;
    --shop-url)
      SHOP_URL="${2:-}"
      [[ -n "$SHOP_URL" ]] || fail "--shop-url 缺少值"
      shift 2
      ;;
    --backend-url)
      BACKEND_URL="${2:-}"
      [[ -n "$BACKEND_URL" ]] || fail "--backend-url 缺少值"
      shift 2
      ;;
    --backend-dir)
      BACKEND_DIR="${2:-}"
      [[ -n "$BACKEND_DIR" ]] || fail "--backend-dir 缺少值"
      shift 2
      ;;
    --skip-health)
      SKIP_HEALTH=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "未知参数: $1"
      ;;
  esac
done

check_command go
check_command curl

if [[ "$START_BACKEND" == true ]]; then
  start_backend
fi
if [[ "$START_SHOP" == true ]]; then
  start_shop
fi

if [[ "$SKIP_HEALTH" == false ]]; then
  if [[ "$START_BACKEND" == true ]]; then
    wait_http "${BACKEND_URL}/health" "shinecar-backend" 45 || log "backend 健康检查失败，继续执行 shop_server 设备上报场景"
  else
    wait_http "${BACKEND_URL}/health" "shinecar-backend" 3 || log "backend 未就绪，继续执行仅 shop_server 设备上报场景"
  fi

  if [[ "$START_SHOP" == true ]]; then
    wait_http "${SHOP_URL}/health" "shop_server" 45 || fail "shop_server 健康检查失败"
  else
    wait_http "${SHOP_URL}/health" "shop_server" 3 || fail "shop_server 未就绪，请先启动或使用 --start-shop"
  fi
fi

run_simulator
probe_url "${SHOP_URL}/health" "shop_server 健康状态"
probe_url "${SHOP_URL}/api/status/station/${STATION_ID}" "shop_server 工位状态"
probe_url "${BACKEND_URL}/health" "shinecar-backend 健康状态"
print_status_tips
