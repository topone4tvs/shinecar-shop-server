package service

import (
	"context"
	"fmt"

	"shop_server/config"
	"shop_server/pkg/logger"
	"shop_server/pkg/mqtt"
)

// Manager 服务管理器
type Manager struct {
	config        *config.Config
	mqttClient    *mqtt.Client
	router        *Router
	deviceManager *DeviceManager
}

// NewManager 创建服务管理器
func NewManager(cfg *config.Config, mqttClient *mqtt.Client, deviceManager *DeviceManager) *Manager {
	return &Manager{
		config:        cfg,
		mqttClient:    mqttClient,
		deviceManager: deviceManager,
	}
}

// Start 启动所有服务
func (m *Manager) Start(ctx context.Context) error {
	logger.Infof("开始启动服务...")

	// 创建消息路由器，传入共享的设备管理器
	m.router = NewRouter(m.config, m.mqttClient, m.deviceManager)

	// 启动消息路由器
	if err := m.router.Start(ctx); err != nil {
		return fmt.Errorf("启动消息路由器失败: %w", err)
	}

	logger.Infof("所有服务启动成功")
	return nil
}

// Stop 停止所有服务
func (m *Manager) Stop(ctx context.Context) error {
	logger.Infof("开始停止服务...")

	// 停止消息路由器
	if m.router != nil {
		if err := m.router.Stop(ctx); err != nil {
			logger.Errorf("停止消息路由器失败: %v", err)
		}
	}

	logger.Infof("所有服务已停止")
	return nil
}

// GetRouter 获取消息路由器
func (m *Manager) GetRouter() *Router {
	return m.router
}

// GetEventPublisher 获取事件发布器。
func (m *Manager) GetEventPublisher() EventPublisher {
	return m.router
}

// GetDeviceManager 获取设备管理器
func (m *Manager) GetDeviceManager() *DeviceManager {
	return m.deviceManager
}
