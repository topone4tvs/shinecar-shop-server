package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"shop_server/config"
	"shop_server/pkg/ha"
)

const (
	DeviceOpeTurnOn   = "turn_on"
	DeviceOpeTurnOff  = "turn_off"
	DeviceOpeToggle   = "toggle"
	DeviceOpeStateOn  = "state_on"
	DeviceOpeStateOff = "state_off"
)

// Service HomeAssistant服务实现
type HaService struct {
	config       *config.Config
	client       *ha.Client
	deviceStatus map[string]*DeviceStatus
	statusMutex  sync.RWMutex
}

// NewService 创建HomeAssistant服务
func NewHaService(cfg *config.Config) *HaService {
	haConfig := &ha.Config{
		BaseURL: cfg.Devices.HA.BaseURL,
		Token:   cfg.Devices.HA.Token,
		Timeout: cfg.Devices.HA.Timeout,
		Headers: cfg.Devices.HA.Headers,
	}

	return &HaService{
		config:       cfg,
		client:       ha.NewClient(haConfig),
		deviceStatus: make(map[string]*DeviceStatus),
	}
}

// GetDeviceType 获取设备类型
func (s *HaService) GetDeviceType() string {
	return DeviceTypeHA
}

// ExecuteCommand 执行设备命令
func (s *HaService) ExecuteCommand(ctx context.Context, cmd DeviceCommand) (DeviceResponse, error) {
	log.Printf("执行HomeAssistant命令: %s, 工位=%s", cmd.GetCommand(), cmd.GetStationID())

	haCmd, ok := cmd.(*HACommand)
	if !ok {
		return s.createErrorResponse(cmd, fmt.Errorf("无效的HomeAssistant命令类型")), nil
	}

	var err error
	var data interface{}

	switch haCmd.Command {
	case CommandHAControl:
		err = s.CallService(ctx, haCmd.Domain, haCmd.Service, haCmd.EntityID, haCmd.ServiceData)
	case CommandGetStatus:
		var state *ha.EntityState
		state, err = s.client.GetEntityState(ctx, haCmd.EntityID)
		if err == nil {
			data = map[string]interface{}{
				"entity_id":  haCmd.EntityID,
				"state":      state.State,
				"attributes": state.Attributes,
			}
		}
	default:
		err = fmt.Errorf("不支持的HomeAssistant命令: %s", haCmd.Command)
	}

	if err != nil {
		return s.createErrorResponse(cmd, err), nil
	}

	return s.createSuccessResponse(cmd, data), nil
}

// GetDeviceStatus 获取设备状态
func (s *HaService) GetDeviceStatus(ctx context.Context, stationID string) (DeviceStatus, error) {
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()

	if status, exists := s.deviceStatus[stationID]; exists {
		return *status, nil
	}

	return DeviceStatus{}, fmt.Errorf("工位 %s 的HomeAssistant设备状态不存在", stationID)
}

// IsHealthy 检查设备健康状态
func (s *HaService) IsHealthy(ctx context.Context) bool {
	// 检查与HomeAssistant的连接
	if err := s.client.CheckHealth(ctx); err != nil {
		log.Printf("HomeAssistant健康检查失败: %v", err)
		return false
	}

	return true
}

// CallService 调用HomeAssistant服务
func (s *HaService) CallService(ctx context.Context, domain, service string, entityID string, data map[string]interface{}) error {
	log.Printf("调用HomeAssistant服务: %s.%s, 实体: %s", domain, service, entityID)

	err := s.client.CallService(ctx, domain, service, entityID, data)
	if err != nil {
		log.Printf("HomeAssistant服务调用失败: %v", err)
		return err
	}

	// 更新设备状态
	s.updateDeviceStatusFromEntityID(entityID, true)
	return nil
}

// GetEntityState 获取实体状态
func (s *HaService) GetEntityState(ctx context.Context, entityID string) (interface{}, error) {
	state, err := s.client.GetEntityState(ctx, entityID)
	if err != nil {
		return nil, err
	}

	// 更新设备状态
	s.updateDeviceStatusFromEntityID(entityID, true)

	return map[string]interface{}{
		"entity_id":  state.EntityID,
		"state":      state.State,
		"attributes": state.Attributes,
		"last_seen":  state.LastSeen,
	}, nil
}

// SetEntityState 设置实体状态
func (s *HaService) SetEntityState(ctx context.Context, entityID string, state interface{}) error {
	err := s.client.SetEntityState(ctx, entityID, state)
	if err != nil {
		return err
	}

	// 更新设备状态
	s.updateDeviceStatusFromEntityID(entityID, true)
	return nil
}

// TurnOn 打开设备
func (s *HaService) TurnOn(ctx context.Context, entityID string) error {
	return s.client.TurnOn(ctx, entityID)
}

// TurnOff 关闭设备
func (s *HaService) TurnOff(ctx context.Context, entityID string) error {
	return s.client.TurnOff(ctx, entityID)
}

// Toggle 切换设备状态
func (s *HaService) Toggle(ctx context.Context, entityID string) error {
	return s.client.Toggle(ctx, entityID)
}

// IsEntityOn 检查实体是否开启
func (s *HaService) IsEntityOn(ctx context.Context, entityID string) (bool, error) {
	return s.client.IsEntityOn(ctx, entityID)
}

// GetAllStations 获取所有工位的HomeAssistant状态
func (s *HaService) GetAllStations() map[string]*DeviceStatus {
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()

	result := make(map[string]*DeviceStatus)
	for k, v := range s.deviceStatus {
		result[k] = v
	}

	return result
}

// UpdateStationStatus 更新工位状态
func (s *HaService) UpdateStationStatus(ctx context.Context) error {
	// 遍历所有配置的工位，检查其HomeAssistant实体状态
	for _, station := range s.config.Shop.Stations {
		if !station.Devices.HA.Enable {
			continue
		}

		entityID := station.Devices.HA.EntityID
		state, err := s.client.GetEntityState(ctx, entityID)
		if err != nil {
			log.Printf("获取工位 %s 的HomeAssistant状态失败: %v", station.ID, err)
			s.updateDeviceStatus(station.ID, false, nil)
			continue
		}

		// 更新状态
		data := map[string]interface{}{
			"entity_id":  state.EntityID,
			"state":      state.State,
			"attributes": state.Attributes,
		}
		s.updateDeviceStatus(station.ID, true, data)
	}

	return nil
}

// StartStatusSync 启动状态同步
func (s *HaService) StartStatusSync(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("HomeAssistant状态同步已停止")
			return
		case <-ticker.C:
			if err := s.UpdateStationStatus(ctx); err != nil {
				log.Printf("HomeAssistant状态同步失败: %v", err)
			}
		}
	}
}

// updateDeviceStatusFromEntityID 根据实体ID更新设备状态
func (s *HaService) updateDeviceStatusFromEntityID(entityID string, online bool) {
	// 根据实体ID找到对应的工位
	for _, station := range s.config.Shop.Stations {
		if station.Devices.HA.Enable && station.Devices.HA.EntityID == entityID {
			s.updateDeviceStatus(station.ID, online, map[string]interface{}{
				"entity_id": entityID,
			})
			break
		}
	}
}

// updateDeviceStatus 更新设备状态
func (s *HaService) updateDeviceStatus(stationID string, online bool, data map[string]interface{}) {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()

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

	s.deviceStatus[stationID] = status
	log.Printf("更新HomeAssistant设备状态: 工位=%s, 在线=%v", stationID, online)
}

// createSuccessResponse 创建成功响应
func (s *HaService) createSuccessResponse(cmd DeviceCommand, data interface{}) DeviceResponse {
	return &BaseDeviceResponse{
		DeviceType: cmd.GetDeviceType(),
		Command:    cmd.GetCommand(),
		StationID:  cmd.GetStationID(),
		Data:       data,
		Success:    true,
	}
}

// createErrorResponse 创建错误响应
func (s *HaService) createErrorResponse(cmd DeviceCommand, err error) DeviceResponse {
	return &BaseDeviceResponse{
		DeviceType: cmd.GetDeviceType(),
		Command:    cmd.GetCommand(),
		StationID:  cmd.GetStationID(),
		Error:      err,
		Success:    false,
	}
}

// GetEntityIDForStation 获取工位对应的实体ID
func (s *HaService) GetEntityIDForStation(stationID string) (string, error) {
	station, err := s.config.GetStationByID(stationID)
	if err != nil {
		return "", err
	}

	if !station.Devices.HA.Enable {
		return "", fmt.Errorf("工位 %s 的HomeAssistant设备未启用", stationID)
	}

	return station.Devices.HA.EntityID, nil
}

// ExecuteStationCommand 执行工位相关的HomeAssistant命令
func (s *HaService) ExecuteStationCommand(ctx context.Context, stationID, command string) error {
	entityID, err := s.GetEntityIDForStation(stationID)
	if err != nil {
		return err
	}

	switch command {
	case DeviceOpeTurnOn:
		return s.TurnOn(ctx, entityID)
	case DeviceOpeTurnOff:
		return s.TurnOff(ctx, entityID)
	case DeviceOpeToggle:
		return s.Toggle(ctx, entityID)
	case DeviceOpeStateOn:
		state, err := s.client.GetEntityState(ctx, entityID)
		if err != nil {
			return err
		}
		if state.State == "on" {
			return nil
		}
		return fmt.Errorf("实体状态不是on")
	case DeviceOpeStateOff:
		state, err := s.client.GetEntityState(ctx, entityID)
		if err != nil {
			return err
		}
		if state.State == "off" {
			return nil
		}
		return fmt.Errorf("实体状态不是off")
	default:
		return fmt.Errorf("不支持的HomeAssistant命令: %s", command)
	}
}
