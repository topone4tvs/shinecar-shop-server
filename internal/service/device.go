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
	// 移除GioStatus字段，门禁状态单独管理
}

// GioStatus 门禁状态
type GateStatus struct {
	StationID  string    `json:"station_id"`  // 工位ID
	IsOpen     bool      `json:"is_open"`     // 闸门是否开启
	LastUpdate time.Time `json:"last_update"` // 最后更新时间
	Source     int       `json:"source"`      // 触发源
	Value      int       `json:"value"`       // 触发值
}

// DeviceManager 设备管理器
type DeviceManager struct {
	config         *config.Config
	deviceStatus   map[string]*DeviceStatus // 设备在线状态
	gateStatus     map[string]*GateStatus   // 门禁状态（独立管理）
	plateResponses map[string][]interface{} // 待处理的门禁响应队列
	mutex          sync.RWMutex
	haService      *HaService
}

// NewDeviceManager 创建设备管理器
func NewDeviceManager(cfg *config.Config) *DeviceManager {
	return &DeviceManager{
		config:         cfg,
		deviceStatus:   make(map[string]*DeviceStatus),
		gateStatus:     make(map[string]*GateStatus),
		plateResponses: make(map[string][]interface{}),
		mutex:          sync.RWMutex{},
		haService:      NewHaService(cfg),
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
	case DeviceTypeUnion:
		return dm.executeUnionCommand(ctx, cmd)
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
		// 使用抽象的门禁控制方法
		if err := dm.executeGateCommand(ctx, plateCmd.StationID, CommandOpenGate); err != nil {
			log.Printf("执行开门指令失败: 工位=%s, 错误=%v", plateCmd.StationID, err)
			return dm.createErrorResponse(cmd, err), nil
		}
	case CommandCloseGate:
		// 使用抽象的门禁控制方法
		if err := dm.executeGateCommand(ctx, plateCmd.StationID, CommandCloseGate); err != nil {
			log.Printf("执行关门指令失败: 工位=%s, 错误=%v", plateCmd.StationID, err)
			return dm.createErrorResponse(cmd, err), nil
		}
	case CommandVoicePlay:
		dm.SetPendingPlateResponse(plateCmd.StationID, map[string]interface{}{
			"Voice": map[string]interface{}{
				"Text": plateCmd.VoiceText,
			},
		})
	case CommandSnapshot:
		dm.SetPendingPlateResponse(plateCmd.StationID, map[string]interface{}{
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

func (dm *DeviceManager) executeUnionCommand(ctx context.Context, cmd DeviceCommand) (DeviceResponse, error) {
	unionCmd, ok := cmd.(*UnionCommand)
	if !ok {
		err := fmt.Errorf("无效的联动命令类型")
		return dm.createErrorResponse(cmd, err), nil
	}

	// 联动命令的处理逻辑
	log.Printf("准备联动命令响应: %s", unionCmd.Command)

	// 更新设备状态
	dm.updatePlateDeviceStatus(unionCmd.StationID, true, map[string]interface{}{
		"last_command": unionCmd.Command,
		"timestamp":    time.Now().Unix(),
	})

	switch unionCmd.Command {
	case CommandUnionStart:
		// 订单开启后要执行的命令
		// 1. 开闸
		if err := dm.executeGateCommand(ctx, unionCmd.StationID, CommandOpenGate); err != nil {
			log.Printf("执行开门指令失败: 工位=%s, 错误=%v", unionCmd.StationID, err)
			return dm.createErrorResponse(cmd, err), nil
		}

		// 2. 打开空调
		dm.callHaCommand(ctx, unionCmd.StationID, CommandUnionStart)

		// 2. 语音播报 (暂时没有实现)
		// 3. 截图保存
		//dm.setPendingPlateResponse(unionCmd.StationID, map[string]interface{}{
		//	"Response_AlarmInfoPlate": map[string]interface{}{
		//		"TriggerImage": map[string]interface{}{
		//			"snapImageRelativeUrl": "/device/snapshot/" + unionCmd.StationID,
		//		},
		//	},
		//})

	case CommandUnionFinish:
		// 订单关闭后要执行的命令
		// 1. 关闸
		if err := dm.executeGateCommand(ctx, unionCmd.StationID, CommandCloseGate); err != nil {
			log.Printf("执行关门指令失败: 工位=%s, 错误=%v", unionCmd.StationID, err)
			return dm.createErrorResponse(cmd, err), nil
		}
		// 2. 关闭空调
		dm.callHaCommand(ctx, unionCmd.StationID, CommandUnionFinish)
	}

	return dm.createSuccessResponse(cmd, map[string]interface{}{
		"prepared": true,
		"command":  unionCmd.Command,
	}), nil
}

func (dm *DeviceManager) callHaCommand(ctx context.Context, stationID string, command string) error {
	var callCmdErr error
	switch command {
	case CommandUnionStart:
		callCmdErr = dm.haService.ExecuteStationCommand(ctx, stationID, DeviceOpeStateOn)
		if callCmdErr == nil {
			log.Printf("跳过执行HomeAssistant命令成功: %s, 工位=%s", command, stationID)
		}
		return dm.haService.ExecuteStationCommand(ctx, stationID, DeviceOpeTurnOn)
	case CommandUnionFinish:
		callCmdErr = dm.haService.ExecuteStationCommand(ctx, stationID, DeviceOpeStateOff)
		if callCmdErr == nil {
			log.Printf("应该可以跳过执行HomeAssistant命令成功: %s, 工位=%s", command, stationID)
		}
		return dm.haService.ExecuteStationCommand(ctx, stationID, DeviceOpeTurnOff)
	}
	return fmt.Errorf("不支持的HomeAssistant命令: %s", command)
}

// executeGateCommand 执行门禁控制命令
func (dm *DeviceManager) executeGateCommand(ctx context.Context, stationID string, command string) error {
	switch command {
	case CommandOpenGate:
		return dm.openGate(ctx, stationID)
	case CommandCloseGate:
		return dm.closeGate(ctx, stationID)
	default:
		return fmt.Errorf("不支持的门禁命令: %s", command)
	}
}

// openGate 开门操作
func (dm *DeviceManager) openGate(ctx context.Context, stationID string) error {
	// 检查门禁状态，如果门已经开启则不需要再次开门
	//if dm.IsGateOpen(stationID) {
	//	log.Printf("门禁已开启，跳过开门指令: 工位=%s", stationID)
	//	// 通电指令还是要继续
	//	dm.setPendingPlateResponse(stationID, map[string]interface{}{
	//		"Response_AlarmInfoPlate": map[string]interface{}{
	//			"ivs_ioctrl": map[string]interface{}{ // 通电
	//				"io":    1,
	//				"value": 1,
	//			},
	//		},
	//	})
	//	return nil
	//}

	// 执行开门指令
	log.Printf("执行开门指令: 工位=%s", stationID)
	dm.SetPendingPlateResponse(stationID, map[string]interface{}{
		"Response_AlarmInfoPlate": map[string]interface{}{
			"ivs_ioctrl": map[string]interface{}{ // 通电
				"io":    0,
				"value": 2,
				"delay": 2000,
			},
		},
	})
	// 通电
	dm.SetPendingPlateResponse(stationID, map[string]interface{}{
		"Response_AlarmInfoPlate": map[string]interface{}{
			"ivs_ioctrl": map[string]interface{}{ // 通电
				"io":    1,
				"value": 1,
			},
		},
	})
	return nil
}

// closeGate 关门操作
func (dm *DeviceManager) closeGate(ctx context.Context, stationID string) error {
	log.Printf("执行关门指令: 工位=%s", stationID)
	// 检查门禁状态，如果门已经开启则不需要再次开门
	//if dm.IsGateOpen(stationID) {
	//	log.Printf("门禁已开启，跳过开门指令: 工位=%s", stationID)
	//	// 断电指令还是要继续
	//	dm.setPendingPlateResponse(stationID, map[string]interface{}{
	//		"Response_AlarmInfoPlate": map[string]interface{}{
	//			"ivs_ioctrl": map[string]interface{}{ // 断电
	//				"io":    1,
	//				"value": 0,
	//			},
	//		},
	//	})
	//	return nil
	//}
	dm.SetPendingPlateResponse(stationID, map[string]interface{}{
		"Response_AlarmInfoPlate": map[string]interface{}{
			"ivs_ioctrl": map[string]interface{}{ // 通电
				"io":    0,
				"value": 2,
				"delay": 2000,
			},
		},
	})
	// 断电
	dm.SetPendingPlateResponse(stationID, map[string]interface{}{
		"Response_AlarmInfoPlate": map[string]interface{}{
			"ivs_ioctrl": map[string]interface{}{ // 断电
				"io":    1,
				"value": 0,
			},
		},
	})
	return nil
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

	// 检查是否已存在状态，如果存在且设备类型匹配，保留GioStatus
	existingStatus, exists := dm.deviceStatus[stationID]
	if exists && existingStatus.DeviceType == status.DeviceType {
		// 如果已存在，只更新必要的字段
		existingStatus.Online = status.Online
		existingStatus.LastSeen = status.LastSeen
		existingStatus.Status = status.Status
		if status.Data != nil {
			existingStatus.Data = status.Data
		}
		//log.Printf("设备状态已更新: 工位=%s, 在线=%t, 最后心跳=%s", stationID, status.Online, status.LastSeen.Format("2006-01-02 15:04:05"))
	} else {
		// 如果不存在，创建新的状态
		dm.deviceStatus[stationID] = status
		log.Printf("设备状态已创建: 工位=%s, 在线=%t, 最后心跳=%s", stationID, status.Online, status.LastSeen.Format("2006-01-02 15:04:05"))
	}
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

// SetPendingPlateResponse 设置待处理的门禁响应
func (dm *DeviceManager) SetPendingPlateResponse(stationID string, response interface{}) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.plateResponses[stationID] = append(dm.plateResponses[stationID], response)
	log.Printf("设置门禁待处理响应: 工位=%s 命令=%s", stationID, response)
}

// GetPendingPlateResponseCount 获取指定工位待处理响应的数量
func (dm *DeviceManager) GetPendingPlateResponseCount(stationID string) int {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	return len(dm.plateResponses[stationID])
}

// GetAllPendingPlateResponses 获取指定工位所有待处理的响应（不移除）
func (dm *DeviceManager) GetAllPendingPlateResponses(stationID string) []interface{} {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	if len(dm.plateResponses[stationID]) == 0 {
		return nil
	}

	// 返回副本，避免外部修改
	responses := make([]interface{}, len(dm.plateResponses[stationID]))
	copy(responses, dm.plateResponses[stationID])
	return responses
}

// ClearPendingPlateResponses 清空指定工位的所有待处理响应
func (dm *DeviceManager) ClearPendingPlateResponses(stationID string) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	delete(dm.plateResponses, stationID)
	log.Printf("清空门禁待处理响应: 工位=%s", stationID)
}

// GetPendingPlateResponse 获取待处理的门禁响应
func (dm *DeviceManager) GetPendingPlateResponse(stationID string) (interface{}, bool) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if len(dm.plateResponses[stationID]) == 0 {
		return nil, false
	}

	// 获取并删除队列中的第一个元素
	response := dm.plateResponses[stationID][0]
	dm.plateResponses[stationID] = dm.plateResponses[stationID][1:]
	log.Printf("获取门禁待处理响应: 工位=%s", stationID)

	return response, true
}

// updatePlateDeviceStatus 更新门禁设备状态
func (dm *DeviceManager) updatePlateDeviceStatus(stationID string, online bool, data map[string]interface{}) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	// 检查是否已存在状态
	existingStatus, exists := dm.deviceStatus[stationID]

	if exists {
		// 如果已存在，检查设备类型是否匹配
		if existingStatus.DeviceType != DeviceTypePlate {
			log.Printf("updatePlateDeviceStatus: 工位=%s, 设备类型不匹配, 期望=%s, 实际=%s",
				stationID, DeviceTypePlate, existingStatus.DeviceType)
			// 如果设备类型不匹配，创建新的门禁设备状态
			status := &DeviceStatus{
				DeviceType: DeviceTypePlate,
				StationID:  stationID,
				Online:     online,
				LastSeen:   time.Now(),
				Status:     "online",
				Data:       data,
			}
			dm.deviceStatus[stationID] = status
			log.Printf("门禁设备状态已重新创建: 工位=%s, 在线=%t", stationID, online)
			return
		}

		// 如果已存在且设备类型匹配，只更新必要的字段
		existingStatus.Online = online
		existingStatus.LastSeen = time.Now()
		existingStatus.Status = "online"
		if data != nil {
			existingStatus.Data = data
		}
		log.Printf("门禁设备状态已更新: 工位=%s, 在线=%t", stationID, online)
	} else {
		// 如果不存在，创建新的状态
		log.Printf("updatePlateDeviceStatus: 工位=%s, 创建新状态", stationID)
		status := &DeviceStatus{
			DeviceType: DeviceTypePlate,
			StationID:  stationID,
			Online:     online,
			LastSeen:   time.Now(),
			Status:     "online",
			Data:       data,
		}
		dm.deviceStatus[stationID] = status
		log.Printf("门禁设备状态已创建: 工位=%s, 在线=%t", stationID, online)
	}
}

// UpdateGateStatus 更新门禁状态
func (dm *DeviceManager) UpdateGateStatus(stationID string, source int, value int) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	// 判断门禁状态：source=0 && value=0 表示门已关闭
	isOpen := !(source == 0 && value == 0)

	// 更新门禁状态
	gateStatus := &GateStatus{
		StationID:  stationID,
		IsOpen:     isOpen,
		LastUpdate: time.Now(),
		Source:     source,
		Value:      value,
	}

	dm.gateStatus[stationID] = gateStatus

	log.Printf("门禁状态已更新: 工位=%s, 开启=%t, 源=%d, 值=%d, 指针=%p",
		stationID, isOpen, source, value, gateStatus)
}

// IsGateOpen 检查闸门是否开启
func (dm *DeviceManager) IsGateOpen(stationID string) bool {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	gateStatus, exists := dm.gateStatus[stationID]
	if !exists {
		log.Printf("IsGateOpen: 工位=%s, 门禁状态不存在", stationID)
		return false
	}

	log.Printf("IsGateOpen: 工位=%s, 门禁状态=%t, 源=%d, 值=%d",
		stationID, gateStatus.IsOpen, gateStatus.Source, gateStatus.Value)
	return gateStatus.IsOpen
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

// GetGateStatus 获取门禁状态
func (dm *DeviceManager) GetGateStatus(stationID string) (*GateStatus, bool) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	gateStatus, exists := dm.gateStatus[stationID]
	return gateStatus, exists
}

// GetAllGateStatus 获取所有门禁状态
func (dm *DeviceManager) GetAllGateStatus() map[string]*GateStatus {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	result := make(map[string]*GateStatus)
	for k, v := range dm.gateStatus {
		result[k] = v
	}
	return result
}
