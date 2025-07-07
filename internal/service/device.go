package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"shop_server/config"
)

// DeviceService 设备服务接口
type DeviceService interface {
	GetDeviceType() string
	ExecuteCommand(ctx context.Context, cmd DeviceCommand) (DeviceResponse, error)
	GetDeviceStatus(ctx context.Context, stationID string) (DeviceStatus, error)
	IsHealthy(ctx context.Context) bool
}

// DeviceStatus 设备状态
type DeviceStatus struct {
	DeviceType string                 `json:"device_type"`
	StationID  string                 `json:"station_id"`
	Online     bool                   `json:"online"`
	LastSeen   time.Time              `json:"last_seen"`
	Status     string                 `json:"status"`
	Data       map[string]interface{} `json:"data"`
}

// DeviceManager 设备管理器
type DeviceManager struct {
	config         *config.Config
	deviceStatus   map[string]*DeviceStatus
	plateResponses map[string]interface{} // 待处理的门禁响应
	mutex          sync.RWMutex
}

// NewDeviceManager 创建设备管理器
func NewDeviceManager(cfg *config.Config) *DeviceManager {
	return &DeviceManager{
		config:         cfg,
		deviceStatus:   make(map[string]*DeviceStatus),
		plateResponses: make(map[string]interface{}),
		mutex:          sync.RWMutex{},
	}
}

// ExecuteCommand 执行设备命令
func (dm *DeviceManager) ExecuteCommand(ctx context.Context, cmd DeviceCommand) (DeviceResponse, error) {
	log.Printf("执行设备命令: %s, 设备类型: %s, 工位: %s", cmd.GetCommand(), cmd.GetDeviceType(), cmd.GetStationID())

	// 验证命令
	if err := cmd.Validate(); err != nil {
		return dm.createErrorResponse(cmd, err), nil
	}

	// 根据设备类型执行命令
	switch cmd.GetDeviceType() {
	case DeviceTypePlate:
		return dm.executePlateCommand(ctx, cmd)
	case DeviceTypeHA:
		return dm.executeHACommand(ctx, cmd)
	default:
		err := fmt.Errorf("不支持的设备类型: %s", cmd.GetDeviceType())
		return dm.createErrorResponse(cmd, err), nil
	}
}

// executePlateCommand 执行门禁命令
func (dm *DeviceManager) executePlateCommand(ctx context.Context, cmd DeviceCommand) (DeviceResponse, error) {
	plateCmd, ok := cmd.(*PlateCommand)
	if !ok {
		err := fmt.Errorf("无效的门禁命令类型")
		return dm.createErrorResponse(cmd, err), nil
	}

	// 门禁命令的处理逻辑
	log.Printf("准备门禁命令响应: %s", plateCmd.Command)

	// 更新设备状态
	dm.updatePlateDeviceStatus(plateCmd.StationID, true, map[string]interface{}{
		"last_command": plateCmd.Command,
		"timestamp":    time.Now().Unix(),
	})

	// 根据命令类型设置待处理响应
	switch plateCmd.Command {
	case CommandOpenGate:
		dm.setPendingPlateResponse(plateCmd.StationID, map[string]interface{}{
			"IO": map[string]interface{}{
				"IONum":    "1",
				"CtrlType": 1,
			},
		})
	case CommandVoicePlay:
		dm.setPendingPlateResponse(plateCmd.StationID, map[string]interface{}{
			"Voice": map[string]interface{}{
				"Text": plateCmd.VoiceText,
			},
		})
	case CommandSnapshot:
		dm.setPendingPlateResponse(plateCmd.StationID, map[string]interface{}{
			"Capture": map[string]interface{}{
				"Enable": true,
			},
		})
	}

	return dm.createSuccessResponse(cmd, map[string]interface{}{
		"prepared": true,
		"command":  plateCmd.Command,
	}), nil
}

// executeHACommand 执行HomeAssistant命令 (简化版本避免导入循环)
func (dm *DeviceManager) executeHACommand(ctx context.Context, cmd DeviceCommand) (DeviceResponse, error) {
	haCmd, ok := cmd.(*HACommand)
	if !ok {
		err := fmt.Errorf("无效的HomeAssistant命令类型")
		return dm.createErrorResponse(cmd, err), nil
	}

	// 简化的HomeAssistant命令处理
	log.Printf("准备HomeAssistant命令: %s, 实体: %s", haCmd.Command, haCmd.EntityID)

	// 更新设备状态
	dm.updateHADeviceStatus(haCmd.StationID, true, map[string]interface{}{
		"entity_id":    haCmd.EntityID,
		"last_command": haCmd.Command,
		"timestamp":    time.Now().Unix(),
		"domain":       haCmd.Domain,
		"service":      haCmd.Service,
	})

	responseData := map[string]interface{}{
		"prepared":  true,
		"command":   haCmd.Command,
		"entity_id": haCmd.EntityID,
		"message":   "HomeAssistant命令已准备（简化实现）",
	}

	return dm.createSuccessResponse(cmd, responseData), nil
}

// UpdateDeviceStatus 更新设备状态
func (dm *DeviceManager) UpdateDeviceStatus(stationID string, status *DeviceStatus) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.deviceStatus[stationID] = status
	log.Printf("设备状态已更新: 工位=%s, 设备类型=%s, 在线=%t",
		stationID, status.DeviceType, status.Online)
}

// GetDeviceStatus 获取设备状态
func (dm *DeviceManager) GetDeviceStatus(stationID string) (*DeviceStatus, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	status, exists := dm.deviceStatus[stationID]
	if !exists {
		return nil, fmt.Errorf("设备状态不存在: 工位=%s", stationID)
	}

	return status, nil
}

// GetAllDeviceStatus 获取所有设备状态
func (dm *DeviceManager) GetAllDeviceStatus() map[string]*DeviceStatus {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	// 创建副本以避免并发访问问题
	result := make(map[string]*DeviceStatus)
	for k, v := range dm.deviceStatus {
		result[k] = v
	}

	return result
}

// setPendingPlateResponse 设置待处理的门禁响应
func (dm *DeviceManager) setPendingPlateResponse(stationID string, response interface{}) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.plateResponses[stationID] = response
	log.Printf("设置门禁待处理响应: 工位=%s", stationID)
}

// GetPendingPlateResponse 获取待处理的门禁响应
func (dm *DeviceManager) GetPendingPlateResponse(stationID string) (interface{}, bool) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	response, exists := dm.plateResponses[stationID]
	if exists {
		// 获取后删除
		delete(dm.plateResponses, stationID)
		log.Printf("获取门禁待处理响应: 工位=%s", stationID)
	}

	return response, exists
}

// updatePlateDeviceStatus 更新门禁设备状态
func (dm *DeviceManager) updatePlateDeviceStatus(stationID string, online bool, data map[string]interface{}) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	status := &DeviceStatus{
		DeviceType: DeviceTypePlate,
		StationID:  stationID,
		Online:     online,
		LastSeen:   time.Now(),
		Status:     "online",
		Data:       data,
	}

	if !online {
		status.Status = "offline"
	}

	dm.deviceStatus[stationID] = status
	log.Printf("更新门禁设备状态: 工位=%s, 在线=%v", stationID, online)
}

// updateHADeviceStatus 更新HomeAssistant设备状态
func (dm *DeviceManager) updateHADeviceStatus(stationID string, online bool, data map[string]interface{}) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	status := &DeviceStatus{
		DeviceType: DeviceTypeHA,
		StationID:  stationID,
		Online:     online,
		LastSeen:   time.Now(),
		Status:     "online",
		Data:       data,
	}

	if !online {
		status.Status = "offline"
	}

	dm.deviceStatus[stationID] = status
	log.Printf("更新HomeAssistant设备状态: 工位=%s, 在线=%v", stationID, online)
}

// StartHealthCheck 启动设备健康检查
func (dm *DeviceManager) StartHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("设备健康检查已停止")
			return
		case <-ticker.C:
			dm.performHealthCheck()
		}
	}
}

// performHealthCheck 执行设备健康检查
func (dm *DeviceManager) performHealthCheck() {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	now := time.Now()
	offlineThreshold := 2 * time.Minute

	for stationID, status := range dm.deviceStatus {
		if status.Online && now.Sub(status.LastSeen) > offlineThreshold {
			status.Online = false
			status.Status = "offline"
			log.Printf("设备离线: 工位=%s, 设备类型=%s, 最后心跳=%s",
				stationID, status.DeviceType, status.LastSeen.Format("2006-01-02 15:04:05"))
		}
	}
}

// createSuccessResponse 创建成功响应
func (dm *DeviceManager) createSuccessResponse(cmd DeviceCommand, data interface{}) DeviceResponse {
	return &BaseDeviceResponse{
		DeviceType: cmd.GetDeviceType(),
		Command:    cmd.GetCommand(),
		StationID:  cmd.GetStationID(),
		Data:       data,
		Success:    true,
	}
}

// createErrorResponse 创建错误响应
func (dm *DeviceManager) createErrorResponse(cmd DeviceCommand, err error) DeviceResponse {
	return &BaseDeviceResponse{
		DeviceType: cmd.GetDeviceType(),
		Command:    cmd.GetCommand(),
		StationID:  cmd.GetStationID(),
		Error:      err,
		Success:    false,
	}
}
