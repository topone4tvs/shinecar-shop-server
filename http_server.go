//go:build manual

package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"shop_server/config"
	"shop_server/internal/server"
	"shop_server/internal/service"
	"shop_server/pkg/logger"
	_ "time/tzdata" // 强制设置时区
)

func main() {
	// 设置时区为中国标准时间
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// 如果加载失败，使用系统本地时区
		loc = time.Local
	}
	time.Local = loc

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		// 配置加载失败时使用标准输出
		panic("加载配置失败: " + err.Error())
	}

	// 初始化日志系统
	logConfig := &logger.Config{
		Level:    cfg.Log.Level,
		FilePath: cfg.Log.FilePath,
		MaxAge:   cfg.Log.MaxAge,
		Compress: cfg.Log.Compress,
		Console:  cfg.Log.Console,
	}
	if err := logger.Init(logConfig); err != nil {
		panic("初始化日志系统失败: " + err.Error())
	}
	defer logger.Sync()

	// 初始化 heartbeat 专用日志系统
	heartbeatLogConfig := &logger.Config{
		Level:    cfg.Log.Level,
		FilePath: "logs/heartbeat.log", // heartbeat 日志文件路径
		MaxAge:   cfg.Log.MaxAge,
		Compress: cfg.Log.Compress,
		Console:  false, // heartbeat 日志不输出到控制台
	}
	if err := logger.InitHeartbeatLogger(heartbeatLogConfig); err != nil {
		panic("初始化 heartbeat 日志系统失败: " + err.Error())
	}

	logger.Infof("启动洗车店智能设备管理系统 - HTTP服务模式...")
	logger.Infof("配置加载成功: 环境=%s, 店铺=%s, 工位数量=%d", cfg.GetEnvironment(), cfg.Shop.Name, len(cfg.Shop.Stations))

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建共享的设备管理器实例
	deviceManager := service.NewDeviceManager(cfg)

	// 创建简化的服务管理器（不包含MQTT功能）
	manager := service.NewSimpleManager(cfg, deviceManager)

	// 创建HTTP服务器，传入共享的设备管理器
	httpServer := server.NewHTTPServer(cfg, manager)

	// 启动HTTP服务器
	var wg sync.WaitGroup

	// 启动HTTP服务器
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := httpServer.Start(ctx); err != nil {
			logger.Infof("启动HTTP服务器失败: %v", err)
			cancel()
		}
	}()

	logger.Infof("HTTP服务启动成功，系统运行中...")
	logger.Infof("HTTP服务器地址: http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Infof("API端点列表:")
	logger.Infof("  GET  /health                           - 健康检查")
	logger.Infof("  GET  /                                 - 系统信息")
	logger.Infof("  POST /api/plate/station/{station_id}  - 门禁推送接收")
	logger.Infof("  POST /api/device/heartbeat/{station_id} - 设备心跳")
	logger.Infof("  POST /api/device/gio/{station_id}     - 门禁状态推送")
	logger.Infof("  GET  /api/status                       - 系统状态")
	logger.Infof("  GET  /api/status/station/{station_id}  - 工位状态")

	// 等待系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		logger.Infof("收到退出信号...")
	case <-ctx.Done():
		logger.Infof("系统异常退出...")
	}

	// 优雅关闭
	logger.Infof("开始关闭系统...")
	cancel() // 取消上下文，通知所有goroutine停止

	// 创建关闭超时上下文
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// 停止HTTP服务器
	go func() {
		if err := httpServer.Stop(shutdownCtx); err != nil {
			logger.Infof("停止HTTP服务器失败: %v", err)
		}
	}()

	// 等待所有goroutine结束或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Infof("所有服务已停止")
	case <-shutdownCtx.Done():
		logger.Infof("关闭超时，强制退出")
	}

	logger.Infof("洗车店智能设备管理系统已退出")
}
