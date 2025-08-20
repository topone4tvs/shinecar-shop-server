package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"shop_server/config"
	"shop_server/internal/server"
	"shop_server/internal/service"
	"shop_server/pkg/mqtt"
)

func main() {

	// 设置时区为中国标准时间
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// 如果加载失败，使用系统本地时区
		loc = time.Local
	}
	time.Local = loc

	log.Println("启动洗车店智能设备管理系统...")

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	log.Printf("配置加载成功: 环境=%s, 店铺=%s, 工位数量=%d", cfg.GetEnvironment(), cfg.Shop.Name, len(cfg.Shop.Stations))

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建MQTT客户端
	mqttClient, err := mqtt.NewClient(&mqtt.Config{
		Broker:   cfg.MQTT.Broker,
		Port:     cfg.MQTT.Port,
		ClientID: cfg.MQTT.ClientID,
		Username: cfg.MQTT.Username,
		Password: cfg.MQTT.Password,
	})

	if err != nil {
		log.Fatalf("创建MQTT客户端失败: %v", err)
	}

	// 连接MQTT
	if err := mqttClient.Connect(ctx); err != nil {
		log.Fatalf("MQTT连接失败: %v", err)
	}
	defer mqttClient.Disconnect()

	log.Println("MQTT连接成功")

	// 创建共享的设备管理器实例
	deviceManager := service.NewDeviceManager(cfg)

	// 创建服务管理器，传入共享的设备管理器
	manager := service.NewManager(cfg, mqttClient, deviceManager)

	// 创建HTTP服务器，传入共享的设备管理器
	httpServer := server.NewHTTPServer(cfg, manager)

	// 启动所有服务
	var wg sync.WaitGroup

	// 启动服务管理器（MQTT服务）
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := manager.Start(ctx); err != nil {
			log.Printf("启动服务管理器失败: %v", err)
			cancel()
		}
	}()

	// 启动HTTP服务器
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := httpServer.Start(ctx); err != nil {
			log.Printf("启动HTTP服务器失败: %v", err)
			cancel()
		}
	}()

	log.Println("所有服务启动成功，系统运行中...")
	log.Printf("HTTP服务器地址: http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("API端点列表:")
	log.Printf("  GET  /health                           - 健康检查")
	log.Printf("  GET  /                                 - 系统信息")
	log.Printf("  POST /api/plate/station/{station_id}  - 门禁推送接收")
	log.Printf("  POST /api/device/heartbeat/{station_id} - 设备心跳")
	log.Printf("  POST /api/device/gio/{station_id}     - 门禁状态推送")
	log.Printf("  GET  /api/status                       - 系统状态")
	log.Printf("  GET  /api/status/station/{station_id}  - 工位状态")

	// 等待系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Println("收到退出信号...")
	case <-ctx.Done():
		log.Println("系统异常退出...")
	}

	// 优雅关闭
	log.Println("开始关闭系统...")
	cancel() // 取消上下文，通知所有goroutine停止

	// 创建关闭超时上下文
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// 停止HTTP服务器
	go func() {
		if err := httpServer.Stop(shutdownCtx); err != nil {
			log.Printf("停止HTTP服务器失败: %v", err)
		}
	}()

	// 停止服务管理器
	go func() {
		if err := manager.Stop(shutdownCtx); err != nil {
			log.Printf("停止服务管理器失败: %v", err)
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
		log.Println("所有服务已停止")
	case <-shutdownCtx.Done():
		log.Println("关闭超时，强制退出")
	}

	log.Println("洗车店智能设备管理系统已退出")
}
