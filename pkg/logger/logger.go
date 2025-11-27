package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Logger 全局日志实例
	Logger *zap.Logger
	Sugar  *zap.SugaredLogger

	// HeartbeatLogger heartbeat 专用日志实例
	HeartbeatLogger *zap.Logger
	HeartbeatSugar  *zap.SugaredLogger
)

// Config 日志配置
type Config struct {
	// 日志级别: debug, info, warn, error, fatal
	Level string
	// 日志文件路径（不含日期后缀）
	FilePath string
	// 保留的旧日志文件最大天数
	MaxAge int
	// 是否压缩旧日志文件
	Compress bool
	// 是否输出到控制台
	Console bool
}

// DailyRotateWriter 按日期轮转的日志写入器
type DailyRotateWriter struct {
	baseFilePath string
	maxAge       int
	compress     bool

	currentDate string
	currentFile *os.File
	mu          sync.Mutex
}

// NewDailyRotateWriter 创建按日期轮转的写入器
func NewDailyRotateWriter(baseFilePath string, maxAge int, compress bool) (*DailyRotateWriter, error) {
	w := &DailyRotateWriter{
		baseFilePath: baseFilePath,
		maxAge:       maxAge,
		compress:     compress,
	}

	if err := w.rotate(); err != nil {
		return nil, err
	}

	// 启动定时检查协程
	go w.checkRotate()

	return w, nil
}

// Write 实现 io.Writer 接口
func (w *DailyRotateWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 检查日期是否变化
	today := time.Now().Format("2006-01-02")
	if today != w.currentDate {
		if err := w.rotateNoLock(); err != nil {
			return 0, err
		}
	}

	return w.currentFile.Write(p)
}

// rotate 轮转日志文件
func (w *DailyRotateWriter) rotate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rotateNoLock()
}

// rotateNoLock 轮转日志文件（不加锁）
func (w *DailyRotateWriter) rotateNoLock() error {
	today := time.Now().Format("2006-01-02")

	// 关闭当前文件
	if w.currentFile != nil {
		w.currentFile.Close()
	}

	// 确保日志目录存在
	logDir := filepath.Dir(w.baseFilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// 新文件名：基础文件名 + 日期
	ext := filepath.Ext(w.baseFilePath)
	nameWithoutExt := w.baseFilePath[:len(w.baseFilePath)-len(ext)]
	newFilePath := fmt.Sprintf("%s-%s%s", nameWithoutExt, today, ext)

	// 打开新文件
	file, err := os.OpenFile(newFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	w.currentFile = file
	w.currentDate = today

	// 清理旧日志
	go w.cleanup()

	return nil
}

// checkRotate 定时检查是否需要轮转（每分钟检查一次）
func (w *DailyRotateWriter) checkRotate() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		today := time.Now().Format("2006-01-02")
		if today != w.currentDate {
			w.rotate()
		}
	}
}

// cleanup 清理过期的日志文件
func (w *DailyRotateWriter) cleanup() {
	if w.maxAge <= 0 {
		return
	}

	logDir := filepath.Dir(w.baseFilePath)
	ext := filepath.Ext(w.baseFilePath)
	nameWithoutExt := filepath.Base(w.baseFilePath[:len(w.baseFilePath)-len(ext)])

	// 计算过期时间
	cutoff := time.Now().AddDate(0, 0, -w.maxAge)

	// 遍历日志目录
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// 匹配日志文件格式：app-2025-10-10.log 或 app-2025-10-10.log.gz
		if !strings.HasPrefix(name, nameWithoutExt+"-") {
			continue
		}

		// 提取日期部分
		dateStr := ""
		if strings.HasSuffix(name, ext+".gz") {
			// 压缩文件：app-2025-10-10.log.gz
			dateStr = name[len(nameWithoutExt)+1 : len(name)-len(ext)-3]
		} else if strings.HasSuffix(name, ext) {
			// 普通文件：app-2025-10-10.log
			dateStr = name[len(nameWithoutExt)+1 : len(name)-len(ext)]
		} else {
			continue
		}

		// 解析日期
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// 删除过期文件
		if fileDate.Before(cutoff) {
			os.Remove(filepath.Join(logDir, name))
		}
	}
}

// Sync 同步文件
func (w *DailyRotateWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentFile != nil {
		return w.currentFile.Sync()
	}
	return nil
}

// Close 关闭写入器
func (w *DailyRotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentFile != nil {
		return w.currentFile.Close()
	}
	return nil
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:    "info",
		FilePath: "logs/app.log",
		MaxAge:   30,   // 30 天
		Compress: true, // 压缩旧日志（暂未实现）
		Console:  true, // 输出到控制台
	}
}

// Init 初始化日志系统
func Init(cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 确保日志目录存在
	logDir := filepath.Dir(cfg.FilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// 解析日志级别
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "fatal":
		level = zapcore.FatalLevel
	}

	// 编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 创建按日期轮转的文件写入器
	rotateWriter, err := NewDailyRotateWriter(cfg.FilePath, cfg.MaxAge, cfg.Compress)
	if err != nil {
		return err
	}

	var fileWriter io.Writer = rotateWriter

	var core zapcore.Core
	if cfg.Console {
		// 同时输出到控制台和文件
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

		consoleCore := zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			level,
		)

		fileCore := zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(fileWriter),
			level,
		)

		core = zapcore.NewTee(consoleCore, fileCore)
	} else {
		// 只输出到文件
		fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
		core = zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(fileWriter),
			level,
		)
	}

	// 创建 logger
	Logger = zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(1), // 跳过封装函数，显示真实调用位置
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	// 创建 Sugar logger
	Sugar = Logger.Sugar()

	return nil
}

// customTimeEncoder 自定义时间格式
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

// Sync 刷新日志缓冲
func Sync() error {
	var err error
	if Logger != nil {
		if e := Logger.Sync(); e != nil {
			err = e
		}
	}
	if HeartbeatLogger != nil {
		if e := HeartbeatLogger.Sync(); e != nil {
			err = e
		}
	}
	return err
}

// InitHeartbeatLogger 初始化 heartbeat 专用日志实例
func InitHeartbeatLogger(cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
		// 默认 heartbeat 日志文件路径
		cfg.FilePath = "logs/heartbeat.log"
		cfg.Console = false // heartbeat 日志默认不输出到控制台
	}

	// 确保日志目录存在
	logDir := filepath.Dir(cfg.FilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// 解析日志级别
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "fatal":
		level = zapcore.FatalLevel
	}

	// 编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 创建按日期轮转的文件写入器
	rotateWriter, err := NewDailyRotateWriter(cfg.FilePath, cfg.MaxAge, cfg.Compress)
	if err != nil {
		return err
	}

	var fileWriter io.Writer = rotateWriter

	var core zapcore.Core
	if cfg.Console {
		// 同时输出到控制台和文件
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

		consoleCore := zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			level,
		)

		fileCore := zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(fileWriter),
			level,
		)

		core = zapcore.NewTee(consoleCore, fileCore)
	} else {
		// 只输出到文件
		fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
		core = zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(fileWriter),
			level,
		)
	}

	// 创建 logger
	HeartbeatLogger = zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(1), // 跳过封装函数，显示真实调用位置
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	// 创建 Sugar logger
	HeartbeatSugar = HeartbeatLogger.Sugar()

	return nil
}

// HeartbeatInfof heartbeat 专用格式化信息日志
func HeartbeatInfof(format string, args ...interface{}) {
	if HeartbeatSugar != nil {
		HeartbeatSugar.Infof(format, args...)
	}
}

// 以下是封装的常用日志方法

// Debug 调试日志
func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

// Debugf 格式化调试日志
func Debugf(format string, args ...interface{}) {
	Sugar.Debugf(format, args...)
}

// Info 信息日志
func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

// Infof 格式化信息日志
func Infof(format string, args ...interface{}) {
	Sugar.Infof(format, args...)
}

// Warn 警告日志
func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

// Warnf 格式化警告日志
func Warnf(format string, args ...interface{}) {
	Sugar.Warnf(format, args...)
}

// Error 错误日志
func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

// Errorf 格式化错误日志
func Errorf(format string, args ...interface{}) {
	Sugar.Errorf(format, args...)
}

// Fatal 致命错误日志（会退出程序）
func Fatal(msg string, fields ...zap.Field) {
	Logger.Fatal(msg, fields...)
}

// Fatalf 格式化致命错误日志（会退出程序）
func Fatalf(format string, args ...interface{}) {
	Sugar.Fatalf(format, args...)
}

// With 创建带字段的子logger
func With(fields ...zap.Field) *zap.Logger {
	return Logger.With(fields...)
}

// Named 创建命名的子logger
func Named(name string) *zap.Logger {
	return Logger.Named(name)
}
