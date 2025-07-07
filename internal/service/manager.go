package service

import (
	"context"
	"fmt"
	"log"
	"shop_server/config"
	"shop_server/pkg/mqtt"
)

// Manager 服务管理器
type Manager struct {
	config     *config.Config
	mqttClient *mqtt.Client
	router     *Router
}

// NewManager 创建服务管理器
func NewManager(cfg *config.Config, mqttClient *mqtt.Client) *Manager {
	return &Manager{
		config:     cfg,
		mqttClient: mqttClient,
	}
}

// Start 启动所有服务
func (m *Manager) Start(ctx context.Context) error {
	log.Println("开始启动服务...")

	// 创建消息路由器
	m.router = NewRouter(m.config, m.mqttClient)

	// 启动消息路由器
	if err := m.router.Start(ctx); err != nil {
		return fmt.Errorf("启动消息路由器失败: %w", err)
	}

	log.Println("所有服务启动成功")
	return nil
}

// Stop 停止所有服务
func (m *Manager) Stop(ctx context.Context) error {
	log.Println("开始停止服务...")

	// 停止消息路由器
	if m.router != nil {
		if err := m.router.Stop(ctx); err != nil {
			log.Printf("停止消息路由器失败: %v", err)
		}
	}

	log.Println("所有服务已停止")
	return nil
}

// GetRouter 获取消息路由器
func (m *Manager) GetRouter() *Router {
	return m.router
}

// GetDeviceManager 获取设备管理器
func (m *Manager) GetDeviceManager() *DeviceManager {
	if m.router != nil {
		return m.router.GetDeviceManager()
	}
	return nil
}
