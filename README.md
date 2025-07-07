# Shop Server - 洗车店智能设备管理系统

## 项目概述

这是一个连锁自助洗车店的智能设备管理系统，作为上游MQTT服务和下游智能设备（门禁系统、HomeAssistant）之间的中间代理。

## 第一阶段完成情况 ✅

### 1. 配置管理系统
- ✅ 完成了店铺和工位的配置结构设计
- ✅ 支持多环境配置（local, pre, online）
- ✅ 实现了配置验证和工位管理功能

### 2. MQTT客户端
- ✅ 实现了MQTT连接、订阅、发布功能
- ✅ 支持自动重连和连接状态监控
- ✅ 支持主题模式订阅（通配符）

### 3. 消息路由器
- ✅ 实现了MQTT消息的接收和主题解析
- ✅ 支持根据店铺ID和工位ID进行消息路由
- ✅ 完成了基础的消息处理框架

### 4. 服务管理器
- ✅ 实现了统一的服务启动和停止管理
- ✅ 支持优雅关闭和信号处理

## 项目结构

```
shop_server/
├── cmd/
│   └── main.go              # 程序入口
├── config/
│   ├── config.go           # 配置管理
│   └── env.local.yaml      # 本地环境配置
├── internal/
│   └── service/            # 核心服务
│       ├── manager.go      # 服务管理器
│       └── router.go       # 消息路由器
├── pkg/
│   └── mqtt/               # MQTT客户端
│       └── client.go
├── go.mod                  # Go模块定义
└── README.md              # 项目说明
```

## 配置说明

### 配置文件示例 (`config/env.local.yaml`)

```yaml
shop:
  id: "shop_001"
  name: "示例洗车店"
  stations:
    - id: "001"
      name: "洗车位1"
      devices:
        plate:
          enable: true
          endpoint: "/api/plate/station/001"
        ha:
          enable: true
          entity_id: "switch.station_001"

mqtt:
  broker: "localhost"
  port: 1883
  client_id: "shop_server_local"
  topics:
    pattern: "shinecar/shop/%s/station/%s"
```

## 运行项目

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 启动服务

```bash
# 使用本地环境配置
go run cmd/main.go

# 或指定环境
APP_ENV=local go run cmd/main.go
```

### 3. 测试MQTT通信

程序启动后会：
1. 连接到MQTT服务器
2. 订阅主题：`shinecar/shop/shop_001/station/+`
3. 等待上游MQTT消息

## 第二阶段规划 🚧

### 上游命令处理框架
- [ ] 实现消息内容解析和验证
- [ ] 创建消息转换器（MQTT → 设备协议）
- [ ] 实现下游服务调用接口
- [ ] 添加门禁系统调用逻辑
- [ ] 添加HomeAssistant调用逻辑

### 下游推送处理框架
- [ ] 创建HTTP服务器接收门禁推送
- [ ] 实现门禁消息处理器
- [ ] 创建设备状态管理
- [ ] 实现上游MQTT通知机制

## 技术栈

- **语言**: Go 1.24
- **MQTT客户端**: eclipse/paho.mqtt.golang
- **配置管理**: YAML
- **HTTP服务**: gin-gonic/gin（第二阶段）
- **日志**: 标准库log（后续可升级为logrus）

## 开发规范

### 目录结构说明
- `cmd/`: 程序入口点
- `config/`: 配置管理相关
- `internal/`: 内部业务逻辑（不对外暴露）
- `pkg/`: 可复用的包（可对外暴露）
- `docs/`: 文档

### 配置管理
- 使用环境变量`APP_ENV`指定环境
- 支持local、pre、online三种环境
- 配置文件格式：`config/env.{环境}.yaml`

## 日志输出示例

```
2024/01/01 10:00:00 启动shop_server - 环境: local, 店铺: 示例洗车店 (shop_001)
2024/01/01 10:00:00 成功连接到MQTT服务器: localhost:1883
2024/01/01 10:00:00 开始启动服务...
2024/01/01 10:00:00 启动消息路由器...
2024/01/01 10:00:00 成功订阅主题: shinecar/shop/shop_001/station/+
2024/01/01 10:00:00 消息路由器启动成功
2024/01/01 10:00:00 所有服务启动成功
2024/01/01 10:00:00 服务已启动，等待信号...
```

## 下一步计划

1. **完善第二阶段**：上游命令处理框架
2. **实现第三阶段**：下游推送处理框架
3. **添加门禁系统对接**：基于`docs/device_api.md`
4. **添加HomeAssistant对接**：REST API调用
5. **完善错误处理和日志**：更好的监控和调试 