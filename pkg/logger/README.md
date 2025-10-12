# 日志系统使用说明

## 概述

本系统使用 `zap` 日志库实现高性能的结构化日志记录，并通过自实现的 `DailyRotateWriter` 实现按日期自动轮转的日志文件管理。

## 功能特性

- ✅ **多级别日志**：支持 Debug, Info, Warn, Error, Fatal 五个级别
- ✅ **按日期切分**：每天0点自动切换到新的日志文件
- ✅ **服务不重启也生效**：后台协程每分钟检查日期变化，自动轮转
- ✅ **自动清理**：自动删除过期的日志文件（默认保留30天）
- ✅ **双输出**：同时输出到控制台和文件
- ✅ **结构化**：文件输出为 JSON 格式，便于日志分析
- ✅ **高性能**：基于 zap 的零内存分配设计

## 配置说明

在 `config/env.yaml` 中配置日志参数：

```yaml
log:
  level: "info"              # 日志级别: debug, info, warn, error, fatal
  file_path: "logs/app.log"  # 日志文件路径（不含日期后缀，实际文件为 app-2025-10-10.log）
  max_age: 30                # 保留天数
  compress: false            # 是否压缩旧日志（暂未实现）
  console: true              # 是否同时输出到控制台
```

## 使用方法

### 1. 初始化日志系统

在应用程序启动时初始化（已在 `cmd/main.go` 中完成）：

```go
import "shop_server/pkg/logger"

// 从配置加载
logConfig := &logger.Config{
    Level:      cfg.Log.Level,
    FilePath:   cfg.Log.FilePath,
    MaxSize:    cfg.Log.MaxSize,
    MaxAge:     cfg.Log.MaxAge,
    MaxBackups: cfg.Log.MaxBackups,
    Compress:   cfg.Log.Compress,
    Console:    cfg.Log.Console,
}

if err := logger.Init(logConfig); err != nil {
    panic("初始化日志系统失败: " + err.Error())
}
defer logger.Sync() // 程序退出前刷新日志缓冲
```

### 2. 记录日志

#### 格式化日志（推荐）

```go
logger.Debugf("调试信息: %s", debugInfo)
logger.Infof("服务启动成功，端口: %d", port)
logger.Warnf("配置项 %s 未设置，使用默认值: %v", key, defaultValue)
logger.Errorf("连接数据库失败: %v", err)
logger.Fatalf("致命错误: %v", err) // 会退出程序
```

#### 结构化日志

```go
import "go.uber.org/zap"

logger.Info("用户登录", 
    zap.String("user_id", userID),
    zap.Int("attempt", attemptCount),
    zap.Duration("latency", duration),
)
```

## 日志文件管理

### 文件命名规则

- 日志文件格式：`app-2025-10-10.log`（文件名 + 日期后缀）
- 每天一个文件，便于管理和归档

### 自动切分机制

1. **按日期切分**：每天 0 点自动切换到新文件
2. **实时检测**：后台协程每分钟检查一次日期变化
3. **无需重启**：即使服务长时间运行，跨日期时也会自动切换

### 自动清理规则

- 保留最近 `max_age` (30) 天的日志
- 每次轮转时自动清理过期文件
- 删除超过保留期限的所有日志文件

## 日志格式

### 控制台输出（便于查看）

```
2025-10-10 19:04:57.824 INFO    shop_server/main.go:51   系统启动成功
```

### 文件输出（JSON格式，便于分析）

```json
{
  "level": "INFO",
  "time": "2025-10-10 19:04:57.824",
  "caller": "shop_server/main.go:51",
  "msg": "系统启动成功"
}
```

## 最佳实践

1. **选择合适的日志级别**
   - Debug：开发调试信息
   - Info：重要的业务流程
   - Warn：警告但不影响运行
   - Error：错误但可以继续运行
   - Fatal：致命错误，需要退出

2. **避免敏感信息**
   - 不要记录密码、Token 等敏感信息
   - 用户信息脱敏处理

3. **适量记录**
   - 避免在循环中记录大量日志
   - 关键节点记录即可

4. **及时清理**
   - 定期检查日志文件大小
   - 根据磁盘空间调整保留策略

## 性能优化

- 使用 `zap` 的零分配设计，性能比标准库 `log` 提升 10+ 倍
- 日志写入异步化，不阻塞业务逻辑
- 按日期轮转，避免单个文件过大影响性能

## 故障排查

### 日志文件未创建

- 检查日志目录权限
- 检查磁盘空间是否充足
- 检查配置文件路径是否正确

### 日志未按日期切分

- 检查系统时区设置（使用 Asia/Shanghai）
- 确认后台轮转协程正常运行
- 查看日志文件是否按日期命名

### 日志丢失

- 确保程序退出前调用 `logger.Sync()`
- 检查是否有未捕获的 panic

