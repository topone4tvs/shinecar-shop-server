package service

import (
	"context"
)

// EventPublisher 定义 HTTP 层发布设备事件所需的最小能力。
type EventPublisher interface {
	PublishToMQTT(stationID, messageType, subType string, data interface{}) error
}

// ManagerInterface 服务管理器接口
type ManagerInterface interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetRouter() *Router
	GetEventPublisher() EventPublisher
	GetDeviceManager() *DeviceManager
}
