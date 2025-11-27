#!/bin/bash

# Heartbeat Monitor 启动脚本

# 设置默认配置（可通过环境变量覆盖）
export HEARTBEAT_LOG_DIR="${HEARTBEAT_LOG_DIR:-logs}"
export HEARTBEAT_LOG_PREFIX="${HEARTBEAT_LOG_PREFIX:-heartbeat}"
export HEARTBEAT_MSG="${HEARTBEAT_MSG:-[shop-heartbeat]: heartbeat}"
export HEARTBEAT_CHECK_INTERVAL="${HEARTBEAT_CHECK_INTERVAL:-60s}"
export HEARTBEAT_RESTART_CMD="${HEARTBEAT_RESTART_CMD:-docker compose up -d --force-recreate shop_server}"
export HEARTBEAT_WORK_DIR="${HEARTBEAT_WORK_DIR:-/var/www}"

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 切换到项目根目录
cd "$PROJECT_ROOT"

# 运行监控服务
echo "启动心跳监控服务..."
echo "配置信息:"
echo "  日志目录: $HEARTBEAT_LOG_DIR"
echo "  日志文件前缀: $HEARTBEAT_LOG_PREFIX"
echo "  心跳消息: $HEARTBEAT_MSG"
echo "  检查间隔: $HEARTBEAT_CHECK_INTERVAL"
echo "  重启命令: $HEARTBEAT_RESTART_CMD"
echo "  工作目录: $HEARTBEAT_WORK_DIR"
echo ""

go run cmd/heartbeat_monitor/main.go

