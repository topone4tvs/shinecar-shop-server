//go:build manual

package main

import (
	"time"

	"shop_server/pkg/logger"
)

func main() {
	// 测试按日期轮转的日志系统
	config := &logger.Config{
		Level:    "info",
		FilePath: "logs/test.log",
		MaxAge:   7,     // 保留7天
		Compress: false, // 不压缩
		Console:  true,  // 同时输出到控制台
	}

	if err := logger.Init(config); err != nil {
		panic("初始化日志失败: " + err.Error())
	}
	defer logger.Sync()

	logger.Infof("=== 日志按日期轮转测试 ===")
	logger.Infof("配置: FilePath=%s, MaxAge=%d天", config.FilePath, config.MaxAge)
	logger.Infof("当前时间: %s", time.Now().Format("2006-01-02 15:04:05"))

	// 写入一些测试日志
	for i := 1; i <= 10; i++ {
		logger.Infof("测试日志 #%d - 时间: %s", i, time.Now().Format("15:04:05.000"))
		time.Sleep(100 * time.Millisecond)
	}

	logger.Infof("=== 测试完成 ===")
	logger.Infof("日志文件格式: logs/test-2025-10-10.log")
	logger.Infof("说明:")
	logger.Infof("  1. 日志文件名包含日期后缀（test-YYYY-MM-DD.log）")
	logger.Infof("  2. 每天0点自动切换到新文件")
	logger.Infof("  3. 服务不重启时，后台协程每分钟检查一次日期变化")
	logger.Infof("  4. 自动删除超过%d天的旧日志", config.MaxAge)
}
