# 主流程 E2E 自测脚本

## 目标

这个脚本用于把本地 `shop_server`、可选的 `shinecar-backend`、设备模拟器串起来，快速模拟门店设备上报主流程消息。

它不是单测替代品，而是用于本地或测试环境快速确认 HTTP/MQTT 主链路有没有断。

## 文件

```text
scripts/e2e_main_flow.sh
simulator/main.go
```

## 支持场景

模拟器支持：

```bash
go run ./simulator -scenario heartbeat
go run ./simulator -scenario plate-recognition
go run ./simulator -scenario gate-open
go run ./simulator -scenario gate-close
go run ./simulator -scenario gate-open-close
go run ./simulator -scenario all
```

`all` 会依次发送：

1. 设备心跳
2. 车牌识别
3. 门禁打开
4. 门禁关闭

## 推荐用法

### 1. 使用已经启动好的服务

先在其他终端启动：

```bash
cd /Users/topone4tvs/Projects/own/shinecar/shop_server
APP_ENV=dev go run ./cmd
```

如果需要验证 backend MQTT 处理，也启动 backend：

```bash
cd /Users/topone4tvs/Projects/own/shinecar/shinecar-backend
APP_ENV=dev go run ./cmd combined
```

然后执行：

```bash
cd /Users/topone4tvs/Projects/own/shinecar/shop_server
scripts/e2e_main_flow.sh --scenario all
```

### 2. 由脚本启动 shop_server

```bash
scripts/e2e_main_flow.sh --start-shop --scenario all
```

### 3. 由脚本同时启动 backend 和 shop_server

```bash
scripts/e2e_main_flow.sh --start-all --scenario all
```

这个模式要求本地已经具备 backend 依赖：

- MySQL
- Redis
- MQTT broker
- backend dev 配置可连通

## 常用参数

```bash
scripts/e2e_main_flow.sh \
  --scenario gate-open-close \
  --station 001 \
  --license 浙A73J2W \
  --shop-url http://localhost:8080 \
  --backend-url http://localhost:7896
```

## 验证点

脚本会：

- 检查 `shop_server /health`
- 尝试检查 `shinecar-backend /health`
- 调用模拟器发送设备消息
- 读取 `shop_server /api/status/station/{stationID}`

如果 backend 和 MQTT 都在线，应在日志中看到：

- `shop_server` 接收设备 HTTP 推送
- `shop_server` 发布 MQTT 事件
- `shinecar-backend` 接收 MQTT 事件
- 门禁状态变化时 backend 下发复合指令
- `shop_server` 接收并执行 `composite_command`

## 注意事项

- 当前脚本不自动初始化数据库测试数据。
- `gate_status` 触发 backend 复合指令需要数据库中对应工位存在运行中订单。
- 如果只是验证 `shop_server` 的设备 HTTP 接入，可不启动 backend。
- 如果要验证 HomeAssistant 真实设备动作，需要本地或测试环境 HA 可用。

