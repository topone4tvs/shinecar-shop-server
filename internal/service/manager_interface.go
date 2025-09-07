package service

import (
	"context"
)

// ManagerInterface 服务管理器接口
type ManagerInterface interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetRouter() *Router
	GetDeviceManager() *DeviceManager
}
