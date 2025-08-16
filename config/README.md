# 配置系统使用说明

## 概述

系统现在使用 `.env` 文件进行配置，不再需要通过环境变量 `ENV` 来指定配置文件。配置服务会直接读取项目根目录下的 `.env` 文件。

## 配置方式

### 1. 创建 .env 文件

在项目根目录下创建 `.env` 文件，可以参考 `config/env.template` 模板：

```bash
cp config/env.template .env
```

### 2. 环境标注

在 `.env` 文件中显式标注当前启动环境：

```bash
# 当前启动环境 (development/production)
ENV=development
```

### 3. 配置项说明

#### 基础配置
- `ENV`: 启动环境 (development/production)
- `SERVER_HOST`: 服务器监听地址
- `SERVER_PORT`: 服务器监听端口

#### MQTT配置
- `MQTT_BROKER`: MQTT服务器地址
- `MQTT_PORT`: MQTT服务器端口
- `MQTT_USERNAME`: MQTT用户名
- `MQTT_PASSWORD`: MQTT密码
- `MQTT_CLIENT_ID`: MQTT客户端ID

#### 店铺配置
- `SHOP_ID`: 店铺ID
- `SHOP_NAME`: 店铺名称

#### 工位配置
- `STATION_1_ID`: 工位1的ID
- `STATION_1_NAME`: 工位1的名称
- `STATION_1_PLATE_ENABLE`: 工位1门禁设备是否启用
- `STATION_1_PLATE_ENDPOINT`: 工位1门禁设备推送端点
- `STATION_1_HA_ENABLE`: 工位1 HA设备是否启用
- `STATION_1_HA_ENTITY_ID`: 工位1 HA设备实体ID

#### HomeAssistant配置
- `HA_BASE_URL`: HomeAssistant服务器地址
- `HA_TOKEN`: HomeAssistant访问令牌
- `HA_TIMEOUT`: 请求超时时间(秒)

#### 日志配置
- `LOG_LEVEL`: 日志级别 (debug/info/warn/error)
- `LOG_FORMAT`: 日志格式 (text/json)
- `LOG_OUTPUT`: 日志输出 (stdout/file)

## 使用方法

### 开发环境
```bash
# .env 文件
ENV=development
SERVER_PORT=8080
MQTT_BROKER=localhost
# ... 其他配置
```

### 生产环境
```bash
# .env 文件
ENV=production
SERVER_PORT=80
MQTT_BROKER=mqtt.production.com
# ... 其他配置
```

## 优势

1. **简化启动**: 不需要设置 `ENV` 环境变量
2. **环境明确**: 在配置文件中直接标注当前环境
3. **易于管理**: 所有配置集中在一个文件中
4. **版本控制友好**: 可以创建不同环境的模板文件

## 注意事项

1. `.env` 文件包含敏感信息，不要提交到版本控制系统
2. 可以创建 `.env.example` 作为模板文件
3. 系统启动时会自动读取 `.env` 文件
4. 如果 `.env` 文件不存在，系统会使用默认配置值 