package service

import (
	"context"

	"shop_server/config"
	"shop_server/pkg/logger"
)

// SimpleManager 简化的服务管理器（不包含MQTT功能）
type SimpleManager struct {
	config        *config.Config
	deviceManager *DeviceManager
}

// NewSimpleManager 创建简化的服务管理器
func NewSimpleManager(cfg *config.Config, deviceManager *DeviceManager) *SimpleManager {
	return &SimpleManager{
		config:        cfg,
		deviceManager: deviceManager,
	}
}

// Start 启动服务（简化版本，只做初始化）
func (m *SimpleManager) Start(ctx context.Context) error {
	logger.Infof("简化服务管理器启动成功（无MQTT功能）")
	return nil
}

// Stop 停止服务（简化版本）
func (m *SimpleManager) Stop(ctx context.Context) error {
	logger.Infof("简化服务管理器已停止")
	return nil
}

// GetRouter 获取消息路由器（返回nil，因为简化版本不包含MQTT）
func (m *SimpleManager) GetRouter() *Router {
	return nil
}

// GetEventPublisher 获取事件发布器（简化版本不包含MQTT）。
func (m *SimpleManager) GetEventPublisher() EventPublisher {
	return nil
}

// GetDeviceManager 获取设备管理器
func (m *SimpleManager) GetDeviceManager() *DeviceManager {
	return m.deviceManager
}
