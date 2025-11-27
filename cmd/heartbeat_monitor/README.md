# Heartbeat Monitor - 心跳监控服务

## 功能说明

这是一个独立的监控服务，用于监控日志文件中的心跳记录。如果检测到心跳丢失（在指定时间内没有新的心跳记录），会自动执行 Docker 容器重启命令。

## 主要功能

- ✅ 监控指定目录下的日志文件（按日期自动识别）
- ✅ 解析 JSON 格式的日志记录
- ✅ 检测心跳消息（`[shop-heartbeat]: heartbeat`）
- ✅ 每分钟检查一次心跳状态
- ✅ 心跳丢失时自动执行 Docker 重启命令

## 使用方法

### 1. 直接运行

```bash
# 使用默认配置
go run cmd/heartbeat_monitor/main.go

# 或编译后运行
go build -o bin/heartbeat_monitor cmd/heartbeat_monitor/main.go
./bin/heartbeat_monitor
```

### 2. 通过环境变量配置

```bash
export HEARTBEAT_LOG_DIR="logs"                    # 日志目录
export HEARTBEAT_LOG_PREFIX="heartbeat"            # 日志文件前缀
export HEARTBEAT_MSG="[shop-heartbeat]: heartbeat" # 心跳消息内容
export HEARTBEAT_CHECK_INTERVAL="60s"              # 检查间隔
export HEARTBEAT_RESTART_CMD="docker compose up -d --force-recreate shop_server"  # 重启命令
export HEARTBEAT_WORK_DIR="/var/www"               # 重启命令工作目录

go run cmd/heartbeat_monitor/main.go
```

### 3. 作为系统服务运行（推荐）

创建 systemd 服务文件 `/etc/systemd/system/heartbeat-monitor.service`:

```ini
[Unit]
Description=Heartbeat Monitor Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/var/www
ExecStart=/path/to/heartbeat_monitor
Restart=always
RestartSec=10
Environment="HEARTBEAT_LOG_DIR=/var/www/logs"
Environment="HEARTBEAT_LOG_PREFIX=heartbeat"
Environment="HEARTBEAT_MSG=[shop-heartbeat]: heartbeat"
Environment="HEARTBEAT_CHECK_INTERVAL=60s"
Environment="HEARTBEAT_RESTART_CMD=docker compose up -d --force-recreate shop_server"
Environment="HEARTBEAT_WORK_DIR=/var/www"

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable heartbeat-monitor
sudo systemctl start heartbeat-monitor
sudo systemctl status heartbeat-monitor
```

## 默认配置

- **日志目录**: `logs`
- **日志文件前缀**: `heartbeat`
- **心跳消息**: `[shop-heartbeat]: heartbeat`
- **检查间隔**: 60 秒
- **重启命令**: `docker compose up -d --force-recreate shop_server`
- **工作目录**: `/var/www`

## 工作原理

1. **日志文件识别**: 根据当前日期自动识别要监控的日志文件
   - 例如：`logs/heartbeat-2025-11-27.log`

2. **心跳检测**: 
   - 每分钟检查一次日志文件
   - 查找包含指定心跳消息的日志记录
   - 检查最近的心跳记录是否在检查窗口内（默认 90 秒）

3. **自动重启**:
   - 如果检测到心跳丢失（超过检查窗口没有新心跳）
   - 自动执行配置的 Docker 重启命令
   - 在指定工作目录下执行命令

## 日志格式要求

监控服务期望的日志格式为 JSON，每行一条记录：

```json
{"level":"INFO","time":"2025-11-27 23:30:00.024","caller":"service/router.go:271","msg":"[shop-heartbeat]: heartbeat"}
```

## 输出示例

```
[2025-11-27 23:30:00] 启动心跳监控服务...
  日志目录: logs
  日志文件前缀: heartbeat
  心跳消息: [shop-heartbeat]: heartbeat
  检查间隔: 60 秒
  重启命令: docker compose up -d --force-recreate shop_server
  工作目录: /var/www
[2025-11-27 23:30:00] ✓ 心跳正常
[2025-11-27 23:31:00] ✓ 心跳正常
[2025-11-27 23:32:00] ✗ 心跳丢失，执行重启...
[2025-11-27 23:32:00] 检测到心跳丢失，执行重启命令...
[2025-11-27 23:32:05] 重启命令执行成功，输出: ...
```

## 注意事项

1. **权限要求**: 执行 Docker 重启命令需要相应的权限
2. **日志文件路径**: 确保日志文件路径正确，服务需要有读取权限
3. **工作目录**: 确保重启命令的工作目录存在且有执行权限
4. **检查间隔**: 建议设置为心跳发送间隔的 1.5-2 倍，避免误判
5. **容错窗口**: 服务使用 1.5 倍检查间隔作为容错窗口，允许一定的延迟

## 故障排查

### 问题：日志文件不存在

```
检查心跳失败: 日志文件不存在: logs/heartbeat-2025-11-27.log
```

**解决方案**: 检查日志目录和文件前缀配置是否正确

### 问题：重启命令执行失败

```
重启失败: 执行重启命令失败: permission denied
```

**解决方案**: 
- 确保服务有执行 Docker 命令的权限
- 检查工作目录是否存在
- 验证 Docker Compose 命令是否正确

### 问题：无法解析日志

如果日志格式不正确，服务会跳过无法解析的行，继续查找有效的心跳记录。

## 开发说明

服务使用 Go 标准库实现，无需额外依赖，可以独立编译和运行。

