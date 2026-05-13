package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"shop_server/config"
	"shop_server/pkg/logger"
)

// GateChannelGioHTTP GIO 专用 HTTP 推送路径。
const GateChannelGioHTTP = "gio_http"

// GateChannelPlateAlarmGioIn plate 推送中的 AlarmGioIn。
const GateChannelPlateAlarmGioIn = "plate_alarm_gio_in"

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

// GateStatus 门禁当前快照状态。
type GateStatus struct {
	StationID  string    `json:"station_id"`  // 工位ID
	IsOpen     bool      `json:"is_open"`     // 闸门是否开启
	LastUpdate time.Time `json:"last_update"` // 最后更新时间
	Source     int       `json:"source"`      // 触发源
	Value      int       `json:"value"`       // 触发值
	Channel    string    `json:"channel"`     // 最近一次读数来源
}

// DeviceManager 设备管理器
type DeviceManager struct {
	config         *config.Config
	deviceStatus   map[string]*DeviceStatus // 设备在线状态
	gateStatus     map[string]*GateStatus   // 门禁状态（独立管理，仅当前快照）
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
	logger.Infof("执行设备命令: %s, 设备类型: %s, 工位: %s, raw:%+v", cmd.GetCommand(), cmd.GetDeviceType(), cmd.GetStationID(), cmd)

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
	case DeviceTypeComposite:
		return dm.executeCompositeCommand(ctx, cmd)
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
	logger.Infof("准备门禁命令响应: %s", plateCmd.Command)

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
			logger.Infof("执行开门指令失败: 工位=%s, 错误=%v", plateCmd.StationID, err)
			return dm.createErrorResponse(cmd, err), nil
		}
	case CommandCloseGate:
		// 使用抽象的门禁控制方法
		if err := dm.executeGateCommand(ctx, plateCmd.StationID, CommandCloseGate); err != nil {
			logger.Infof("执行关门指令失败: 工位=%s, 错误=%v", plateCmd.StationID, err)
			return dm.createErrorResponse(cmd, err), nil
		}
	case CommandVoicePlay:
		dm.SetPendingPlateResponse(plateCmd.StationID, map[string]interface{}{
			"Response_AlarmInfoPlate": map[string]interface{}{
				"playserver_json_request": map[string]interface{}{
					"type":           "ps_voice_play",
					"voice":          "JXU2QjIyJXU4RkNFJXU1MTQ5JXU0RTM0JXU3MTE1JXU4RjY2JXU3NTFGJXU2RDNC",
					"voice_interval": 0,
					"voice_volume":   100,
					"voice_male":     1,
				},
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
	logger.Infof("准备联动命令响应: %s", unionCmd.Command)

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
			logger.Infof("执行开门指令失败: 工位=%s, 错误=%v", unionCmd.StationID, err)
			return dm.createErrorResponse(cmd, err), nil
		}
		// 2. 通电
		if err := dm.executePowerCommand(ctx, unionCmd.StationID, CommandOpenGate); err != nil {
			logger.Infof("执行通电指令失败: 工位=%s, 错误=%v", unionCmd.StationID, err)
			return dm.createErrorResponse(cmd, err), nil
		}

		// 3. 打开空调
		//dm.callHaCommand(ctx, unionCmd.StationID, CommandUnionStart, nil)

		// 4. 设置音量，并播放音乐
		//haCmd := &HACommand{
		//	PlayVolume:    0.45,
		//	TextDirective: "把音量调到45",
		//}
		//dm.callHaCommand(ctx, unionCmd.StationID, CommandHASetVolume, haCmd)
		//dm.callHaCommand(ctx, unionCmd.StationID, CommandHAPlayMusic, nil)

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
			logger.Infof("执行关门指令失败: 工位=%s, 错误=%v", unionCmd.StationID, err)
			return dm.createErrorResponse(cmd, err), nil
		}
		// 2. 断电
		if err := dm.executePowerCommand(ctx, unionCmd.StationID, CommandCloseGate); err != nil {
			logger.Infof("执行断电指令失败: 工位=%s, 错误=%v", unionCmd.StationID, err)
			return dm.createErrorResponse(cmd, err), nil
		}

		// 3. 关闭空调
		//dm.callHaCommand(ctx, unionCmd.StationID, CommandUnionFinish, nil)
		// 4. 关闭音乐
		//dm.callHaCommand(ctx, unionCmd.StationID, CommandHAPauseMusic, nil)
	}

	return dm.createSuccessResponse(cmd, map[string]interface{}{
		"prepared": true,
		"command":  unionCmd.Command,
	}), nil
}

func (dm *DeviceManager) callHaCommand(ctx context.Context, stationID string, command string, haCmd *HACommand) error {
	var callCmdErr error
	switch command {
	case CommandUnionStart:
		callCmdErr = dm.haService.ExecuteAirConditionerCommand(ctx, stationID, DeviceOpeStateOn)
		if callCmdErr == nil {
			logger.Infof("跳过执行HomeAssistant命令成功: %s, 工位=%s", command, stationID)
		}
		return dm.haService.ExecuteAirConditionerCommand(ctx, stationID, DeviceOpeTurnOn)
	case CommandUnionFinish:
		callCmdErr = dm.haService.ExecuteAirConditionerCommand(ctx, stationID, DeviceOpeStateOff)
		if callCmdErr == nil {
			logger.Infof("应该可以跳过执行HomeAssistant命令成功: %s, 工位=%s", command, stationID)
		}
		return dm.haService.ExecuteAirConditionerCommand(ctx, stationID, DeviceOpeTurnOff)
	case CommandHANotice:
		playText := haCmd.PlayText
		if playText != "" {
			logger.Infof("执行HomeAssistant voice播放命令: %s, 工位=%s, 文本=%s", command, stationID, playText)
			return dm.haService.ExecuteSpeakerCommand(ctx, stationID, DeviceOpePlayText, playText)
		}
	case CommandHAPlayMusic:
		return dm.haService.ExecutePlayerCommand(ctx, stationID, DeviceOpePlayMusic)
	case CommandHAPauseMusic:
		return dm.haService.ExecuteMediaControlCommand(ctx, stationID, DeviceOpePauseMusic, 0)
	case CommandHASetVolume:
		return dm.haService.ExecuteMediaControlCommand(ctx, stationID, DeviceOpeSetVolume, haCmd.PlayVolume)
	case CommandHAExecuteDirective:
		return dm.haService.ExecuteExecuteDirectiveCommand(ctx, stationID, DeviceOpeExecuteDirective, haCmd.TextDirective)
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

func (dm *DeviceManager) executePowerCommand(ctx context.Context, stationID string, command string) error {
	switch command {
	case CommandOpenGate:
		return dm.powerOn(ctx, stationID, command)
	case CommandCloseGate:
		return dm.powerOff(ctx, stationID, command)
	}
	return nil
}

func (dm *DeviceManager) powerOn(ctx context.Context, stationID string, command string) error {
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

func (dm *DeviceManager) powerOff(ctx context.Context, stationID string, command string) error {
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

// openGate 开门操作
func (dm *DeviceManager) openGate(ctx context.Context, stationID string) error {
	// 检查门禁状态，如果门已经开启则不需要再次开门
	//if dm.IsGateOpen(stationID) {
	//	logger.Infof("门禁已开启，跳过开门指令: 工位=%s", stationID)
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
	logger.Infof("执行开门指令: 工位=%s", stationID)
	dm.SetPendingPlateResponse(stationID, map[string]interface{}{
		"Response_AlarmInfoPlate": map[string]interface{}{
			"ivs_ioctrl": map[string]interface{}{ // 通电
				"io":    0,
				"value": 2,
				"delay": 2000,
			},
		},
	})

	return nil
}

// closeGate 关门操作
func (dm *DeviceManager) closeGate(ctx context.Context, stationID string) error {
	logger.Infof("执行关门指令: 工位=%s", stationID)
	// 检查门禁状态，如果门已经开启则不需要再次开门
	//if dm.IsGateOpen(stationID) {
	//	logger.Infof("门禁已开启，跳过开门指令: 工位=%s", stationID)
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
	// 关门
	dm.SetPendingPlateResponse(stationID, map[string]interface{}{
		"Response_AlarmInfoPlate": map[string]interface{}{
			"ivs_ioctrl": map[string]interface{}{
				"io":    0,
				"value": 2,
				"delay": 2000,
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
	logger.Infof("准备HomeAssistant命令: %s, 实体: %s", haCmd.Command, haCmd.EntityID)

	// 更新设备状态
	dm.updateHADeviceStatus(haCmd.StationID, true, map[string]interface{}{
		"entity_id":    haCmd.EntityID,
		"last_command": haCmd.Command,
		"timestamp":    time.Now().Unix(),
		"domain":       haCmd.Domain,
		"service":      haCmd.Service,
	})

	// 根据命令类型设置待处理响应
	switch haCmd.Command {
	case CommandHANotice:
		//haCmd.TextDirective = "把音量调到99"
		//dm.callHaCommand(ctx, haCmd.StationID, CommandHAExecuteDirective, haCmd)
		haCmd.PlayVolume = 0.99
		dm.callHaCommand(ctx, haCmd.StationID, CommandHASetVolume, haCmd)
		//time.Sleep(1 * time.Second)
		dm.callHaCommand(ctx, haCmd.StationID, haCmd.Command, haCmd)
		//time.Sleep(36 * time.Second)
		//haCmd.PlayVolume = 0.45
		//dm.callHaCommand(ctx, haCmd.StationID, CommandHASetVolume, haCmd)
		//haCmd.TextDirective = "把音量调到45"
		//dm.callHaCommand(ctx, haCmd.StationID, CommandHAExecuteDirective, haCmd)
	}

	responseData := map[string]interface{}{
		"prepared":  true,
		"command":   haCmd.Command,
		"entity_id": haCmd.EntityID,
		"message":   "HomeAssistant命令已准备（简化实现）",
	}

	return dm.createSuccessResponse(cmd, responseData), nil
}

func (dm *DeviceManager) executeCompositeCommand(ctx context.Context, cmd DeviceCommand) (DeviceResponse, error) {
	compositeCmd, ok := cmd.(*CompositeCommand)
	if !ok {
		err := fmt.Errorf("无效的复合命令类型")
		return dm.createErrorResponse(cmd, err), nil
	}

	results := make([]map[string]interface{}, 0, len(compositeCmd.Commands))
	var firstErr error

	for _, subCmd := range compositeCmd.Commands {
		result := map[string]interface{}{
			"command": subCmd.Command,
			"target":  subCmd.Target,
			"action":  subCmd.Action,
			"success": true,
		}

		err := dm.executeCompositeSubCommand(ctx, compositeCmd.StationID, subCmd)
		if err != nil {
			result["success"] = false
			result["error"] = err.Error()
			if firstErr == nil {
				firstErr = err
			}
			results = append(results, result)
			if !compositeCmd.ContinueOnError {
				return dm.createErrorResponse(cmd, err), nil
			}
			continue
		}

		results = append(results, result)
	}

	responseData := map[string]interface{}{
		"scene":             compositeCmd.Scene,
		"continue_on_error": compositeCmd.ContinueOnError,
		"results":           results,
	}

	if firstErr != nil {
		responseData["partial_success"] = true
		responseData["error"] = firstErr.Error()
	}

	return dm.createSuccessResponse(cmd, responseData), nil
}

func (dm *DeviceManager) executeCompositeSubCommand(ctx context.Context, stationID string, subCmd CompositeSubCommand) error {
	switch subCmd.Command {
	case CommandHAControl:
		return dm.executeCompositeHAControl(ctx, stationID, subCmd)
	default:
		return fmt.Errorf("不支持的复合子命令: %s", subCmd.Command)
	}
}

func (dm *DeviceManager) executeCompositeHAControl(ctx context.Context, stationID string, subCmd CompositeSubCommand) error {
	entityID, err := dm.getHAEntityIDByTarget(stationID, subCmd.Target)
	if err != nil {
		return err
	}

	switch subCmd.Action {
	case DeviceOpeTurnOn:
		return dm.haService.TurnOn(ctx, entityID)
	case DeviceOpeTurnOff:
		return dm.haService.TurnOff(ctx, entityID)
	default:
		return fmt.Errorf("不支持的HA复合动作: target=%s action=%s", subCmd.Target, subCmd.Action)
	}
}

func (dm *DeviceManager) getHAEntityIDByTarget(stationID string, target string) (string, error) {
	station, err := dm.config.GetStationByID(stationID)
	if err != nil {
		return "", err
	}
	if !station.Devices.HA.Enable {
		return "", fmt.Errorf("工位 %s 的HomeAssistant设备未启用", stationID)
	}

	switch target {
	case "air_conditioner":
		if station.Devices.HA.AirConditioner == "" {
			return "", fmt.Errorf("工位 %s 未配置空调设备", stationID)
		}
		return station.Devices.HA.AirConditioner, nil
	case "ventilation":
		if station.Devices.HA.Ventilation == "" {
			return "", fmt.Errorf("工位 %s 未配置通风设备", stationID)
		}
		return station.Devices.HA.Ventilation, nil
	default:
		return "", fmt.Errorf("不支持的HA目标: %s", target)
	}
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
		//logger.Infof("设备状态已更新: 工位=%s, 在线=%t, 最后心跳=%s", stationID, status.Online, status.LastSeen.Format("2006-01-02 15:04:05"))
	} else {
		// 如果不存在，创建新的状态
		dm.deviceStatus[stationID] = status
		logger.Infof("设备状态已创建: 工位=%s, 在线=%t, 最后心跳=%s", stationID, status.Online, status.LastSeen.Format("2006-01-02 15:04:05"))
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
	logger.Infof("设置门禁待处理响应: 工位=%s 命令=%s", stationID, response)
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
	logger.Infof("清空门禁待处理响应: 工位=%s", stationID)
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
	logger.Infof("获取门禁待处理响应: 工位=%s", stationID)

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
			logger.Infof("updatePlateDeviceStatus: 工位=%s, 设备类型不匹配, 期望=%s, 实际=%s",
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
			logger.Infof("门禁设备状态已重新创建: 工位=%s, 在线=%t", stationID, online)
			return
		}

		// 如果已存在且设备类型匹配，只更新必要的字段
		existingStatus.Online = online
		existingStatus.LastSeen = time.Now()
		existingStatus.Status = "online"
		if data != nil {
			existingStatus.Data = data
		}
		logger.Infof("门禁设备状态已更新: 工位=%s, 在线=%t", stationID, online)
	} else {
		// 如果不存在，创建新的状态
		logger.Infof("updatePlateDeviceStatus: 工位=%s, 创建新状态", stationID)
		status := &DeviceStatus{
			DeviceType: DeviceTypePlate,
			StationID:  stationID,
			Online:     online,
			LastSeen:   time.Now(),
			Status:     "online",
			Data:       data,
		}
		dm.deviceStatus[stationID] = status
		logger.Infof("门禁设备状态已创建: 工位=%s, 在线=%t", stationID, online)
	}
}

// ApplyHardwareGateReading 根据硬件上报的 source/value 更新当前门禁快照；仅在开合语义变化（或首次有读数）时写 [门禁变迁] 日志落盘（由全局 logger 配置决定）。
func (dm *DeviceManager) ApplyHardwareGateReading(stationID string, source, value int, channel string) (GateStatus, bool) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	toOpen := !(source == 0 && value == 0)
	prev, hadPrev := dm.gateStatus[stationID]
	changed := !hadPrev || prev.IsOpen != toOpen

	now := time.Now()
	snapshot := GateStatus{
		StationID:  stationID,
		IsOpen:     toOpen,
		LastUpdate: now,
		Source:     source,
		Value:      value,
		Channel:    channel,
	}
	dm.gateStatus[stationID] = &snapshot

	if !changed {
		return snapshot, false
	}

	var fromOpen *bool
	if hadPrev {
		v := prev.IsOpen
		fromOpen = &v
	}

	if fromOpen == nil {
		logger.Infof("[门禁变迁] station_id=%s from_open=unknown to_open=%t source=%d value=%d channel=%s ts_unix=%d",
			stationID, toOpen, source, value, channel, now.Unix())
	} else {
		logger.Infof("[门禁变迁] station_id=%s from_open=%t to_open=%t source=%d value=%d channel=%s ts_unix=%d",
			stationID, *fromOpen, toOpen, source, value, channel, now.Unix())
	}

	return snapshot, true
}

// UpdateGateStatus 更新门禁状态（兼容旧调用；channel 为空）。
func (dm *DeviceManager) UpdateGateStatus(stationID string, source int, value int) {
	dm.ApplyHardwareGateReading(stationID, source, value, "")
}

// IsGateOpen 检查闸门是否开启
func (dm *DeviceManager) IsGateOpen(stationID string) bool {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	gateStatus, exists := dm.gateStatus[stationID]
	if !exists {
		logger.Infof("IsGateOpen: 工位=%s, 门禁状态不存在", stationID)
		return false
	}

	logger.Infof("IsGateOpen: 工位=%s, 门禁状态=%t, 源=%d, 值=%d",
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
	logger.Infof("更新HomeAssistant设备状态: 工位=%s, 在线=%v", stationID, online)
}

// StartHealthCheck 启动设备健康检查
func (dm *DeviceManager) StartHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Infof("设备健康检查已停止")
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
			logger.Infof("设备离线: 工位=%s, 设备类型=%s, 最后心跳=%s",
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
