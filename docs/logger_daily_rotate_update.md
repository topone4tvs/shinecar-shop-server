# 日志系统按日期切分更新

## 更新时间
2025-10-10

## 更新背景

用户需求：
1. 日志文件名不需要时间后缀，只保留日期
2. 暂时不需要根据文件大小拆分，按日期每日拆分即可
3. 即使服务没有重启，到了跨日期的时间点后，新的日志能够写入新的文件

## 实施方案

### 技术调整

**移除依赖**：
- ❌ 移除 `gopkg.in/natefinch/lumberjack.v2`（主要基于文件大小切分）

**自实现轮转**：
- ✅ 实现 `DailyRotateWriter`（严格按日期切分）

### 核心实现

#### 1. DailyRotateWriter 结构

```go
type DailyRotateWriter struct {
    baseFilePath string      // 基础文件路径
    maxAge       int          // 保留天数
    compress     bool         // 是否压缩
    
    currentDate  string       // 当前日期
    currentFile  *os.File     // 当前文件句柄
    mu           sync.Mutex   // 并发安全
}
```

#### 2. 关键功能

**写入检测**：
- 每次写入时检查日期是否变化
- 日期变化时自动轮转到新文件

```go
func (w *DailyRotateWriter) Write(p []byte) (n int, err error) {
    today := time.Now().Format("2006-01-02")
    if today != w.currentDate {
        w.rotate()  // 自动轮转
    }
    return w.currentFile.Write(p)
}
```

**后台检测**：
- 启动后台协程每分钟检查一次
- 确保即使写入量少也能及时轮转

```go
func (w *DailyRotateWriter) checkRotate() {
    ticker := time.NewTicker(1 * time.Minute)
    for range ticker.C {
        if time.Now().Format("2006-01-02") != w.currentDate {
            w.rotate()
        }
    }
}
```

**自动清理**：
- 每次轮转时清理过期文件
- 删除超过 maxAge 天数的日志

## 配置变化

### 之前的配置

```yaml
log:
  level: "info"
  file_path: "logs/app.log"
  max_size: 100      # ❌ 已移除
  max_age: 30
  max_backups: 10    # ❌ 已移除
  compress: true
  console: true
```

### 现在的配置

```yaml
log:
  level: "info"
  file_path: "logs/app.log"  # 基础文件名（不含日期）
  max_age: 30                # 保留天数
  compress: false            # 暂未实现
  console: true
```

## 文件命名变化

### 之前（使用 lumberjack）

- 当前日志：`app.log`
- 轮转日志：`app-2025-10-10T19-04-57.824.log`（含时间戳）
- 压缩日志：`app-2025-10-10T19-04-57.824.log.gz`

### 现在（按日期切分）

- 日志文件：`app-2025-10-10.log`（只含日期）
- 格式简洁：便于查找和管理

## 功能特点

### ✅ 严格按日期切分

- 每天 0 点自动创建新文件
- 文件名格式：`app-YYYY-MM-DD.log`
- 一天一个文件，清晰明了

### ✅ 服务不重启也生效

- 后台协程每分钟检查日期变化
- 写入时也会检测日期
- 双重保障，确保及时切换

### ✅ 自动清理过期文件

- 保留最近 N 天的日志（可配置）
- 每次轮转时自动清理
- 无需手动维护

### ✅ 并发安全

- 使用 Mutex 保护文件操作
- 支持高并发写入
- 确保数据完整性

## 测试验证

### 编译测试

```bash
✅ go build -o bin/shop_server ./cmd/main.go
✅ go build -o bin/http_server ./http_server.go
```

### 功能测试

```bash
$ go run test_daily_rotate.go
2025-10-10 21:14:30.522 INFO    === 日志按日期轮转测试 ===
2025-10-10 21:14:30.522 INFO    配置: FilePath=logs/test.log, MaxAge=7天
2025-10-10 21:14:30.522 INFO    当前时间: 2025-10-10 21:14:30
...
```

### 文件生成

```bash
$ ls -lh logs/
-rw-r--r--  1 user  staff   2.8K Oct 10 21:14 test-2025-10-10.log
```

### 文件内容

```json
{"level":"INFO","time":"2025-10-10 21:14:30.522","caller":"...","msg":"测试日志"}
```

## 代码变更

### 修改文件

1. `pkg/logger/logger.go` - 实现 DailyRotateWriter
2. `config/config.go` - 更新 LogConfig 结构
3. `config/env.yaml` - 更新配置
4. `cmd/main.go` - 调整日志初始化
5. `http_server.go` - 调整日志初始化

### 文档更新

1. `pkg/logger/README.md` - 更新使用说明
2. `docs/logger_optimization.md` - 更新优化总结
3. `docs/logger_daily_rotate_update.md` - 本更新文档

## 使用示例

### 日志写入

```go
logger.Infof("服务启动成功，端口: %d", port)
logger.Warnf("配置项 %s 未设置", key)
logger.Errorf("连接失败: %v", err)
```

### 查看日志

```bash
# 查看今天的日志
cat logs/app-2025-10-10.log

# 查看昨天的日志
cat logs/app-2025-10-09.log

# 查看最近3天的日志
cat logs/app-2025-10-{08,09,10}.log
```

## 优势对比

| 特性 | lumberjack | DailyRotateWriter |
|------|-----------|-------------------|
| 切分方式 | 主要按大小 | **严格按日期** ✅ |
| 文件命名 | 含时间戳 | **只含日期** ✅ |
| 跨日切换 | 需要写入触发 | **后台自动检测** ✅ |
| 配置简单性 | 较复杂 | **更简洁** ✅ |
| 文件查找 | 较困难 | **非常容易** ✅ |

## 注意事项

1. **时区设置**：使用 `Asia/Shanghai` 时区
2. **文件权限**：确保日志目录有写权限
3. **磁盘空间**：定期检查磁盘使用情况
4. **日志级别**：生产环境建议使用 `info` 或更高级别

## 后续优化建议

1. **日志压缩**：实现旧日志自动压缩功能
2. **远程备份**：定期备份到远程存储
3. **日志分析**：接入 ELK 等日志分析系统
4. **告警机制**：ERROR 级别日志触发告警

## 总结

✅ **完成目标**：
- 日志文件名只含日期（app-2025-10-10.log）
- 严格按日期每日拆分
- 服务不重启时跨日期自动切换

✅ **技术实现**：
- 自实现 DailyRotateWriter
- 移除 lumberjack 依赖
- 双重检测机制（写入时 + 后台协程）

✅ **用户体验**：
- 配置更简洁
- 文件名更直观
- 查找更方便
- 管理更轻松


