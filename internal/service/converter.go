package service

import (
	"fmt"
	"shop_server/config"
)

// MessageConverter 消息转换器接口
type MessageConverter interface {
	// 转换MQTT消息为设备命令
	ConvertToDeviceCommand(msg *MQTTMessage) (DeviceCommand, error)
	// 转换设备响应为MQTT消息
	ConvertToMQTTMessage(resp DeviceResponse) (*MQTTMessage, error)
}

// DefaultMessageConverter 默认消息转换器
type DefaultMessageConverter struct {
	config *config.Config
}

// NewMessageConverter 创建消息转换器
func NewMessageConverter(cfg *config.Config) MessageConverter {
	return &DefaultMessageConverter{
		config: cfg,
	}
}

// ConvertToDeviceCommand 转换MQTT消息为设备命令
func (c *DefaultMessageConverter) ConvertToDeviceCommand(msg *MQTTMessage) (DeviceCommand, error) {
	// 根据命令类型决定设备类型
	deviceType, err := c.getDeviceTypeFromCommand(msg.Command)
	if err != nil {
		return nil, err
	}

	// 创建基础设备命令
	baseCmd := &BaseDeviceCommand{
		DeviceType: deviceType,
		Command:    msg.Command,
		StationID:  msg.StationID,
		Data:       msg.Data,
	}

	// 根据设备类型创建特定的设备命令
	switch deviceType {
	case DeviceTypePlate:
		return c.createPlateCommand(baseCmd, msg)
	case DeviceTypeHA:
		return c.createHACommand(baseCmd, msg)
	default:
		return baseCmd, nil
	}
}

// ConvertToMQTTMessage 转换设备响应为MQTT消息
func (c *DefaultMessageConverter) ConvertToMQTTMessage(resp DeviceResponse) (*MQTTMessage, error) {
	msg := &MQTTMessage{
		Type:      MessageTypeDevice,
		Command:   resp.GetCommand(),
		ShopID:    c.config.Shop.ID,
		StationID: resp.GetStationID(),
		Data:      make(map[string]interface{}),
	}

	// 设置响应数据
	msg.Data["device_type"] = resp.GetDeviceType()
	msg.Data["success"] = resp.IsSuccess()
	msg.Data["data"] = resp.GetData()

	if resp.GetError() != nil {
		msg.Data["error"] = resp.GetError().Error()
	}

	return msg, nil
}

// getDeviceTypeFromCommand 根据命令获取设备类型
func (c *DefaultMessageConverter) getDeviceTypeFromCommand(command string) (string, error) {
	switch command {
	case CommandOpenGate, CommandCloseGate, CommandVoicePlay, CommandSnapshot, CommandTrigger:
		return DeviceTypePlate, nil
	case CommandHAControl:
		return DeviceTypeHA, nil
	case CommandGetStatus:
		// 状态查询需要从数据中获取设备类型
		return "", fmt.Errorf("状态查询命令需要指定设备类型")
	default:
		return "", fmt.Errorf("未知命令: %s", command)
	}
}

// createPlateCommand 创建门禁设备命令
func (c *DefaultMessageConverter) createPlateCommand(baseCmd *BaseDeviceCommand, msg *MQTTMessage) (DeviceCommand, error) {
	cmd := &PlateCommand{
		BaseDeviceCommand: *baseCmd,
	}

	// 根据不同的门禁命令设置特定数据
	switch msg.Command {
	case CommandVoicePlay:
		if text, ok := msg.Data["text"].(string); ok {
			cmd.VoiceText = text
		} else {
			return nil, fmt.Errorf("语音播报命令缺少text参数")
		}
	case CommandSnapshot:
		if url, ok := msg.Data["callback_url"].(string); ok {
			cmd.CallbackURL = url
		}
	}

	return cmd, nil
}

// createHACommand 创建HomeAssistant命令
func (c *DefaultMessageConverter) createHACommand(baseCmd *BaseDeviceCommand, msg *MQTTMessage) (DeviceCommand, error) {
	cmd := &HACommand{
		BaseDeviceCommand: *baseCmd,
	}

	// 从配置中获取实体ID
	station, err := c.config.GetStationByID(msg.StationID)
	if err != nil {
		return nil, fmt.Errorf("获取工位配置失败: %w", err)
	}

	cmd.EntityID = station.Devices.HA.EntityID

	// 设置HomeAssistant特定数据
	if domain, ok := msg.Data["domain"].(string); ok {
		cmd.Domain = domain
	} else {
		cmd.Domain = "switch" // 默认为开关域
	}

	if service, ok := msg.Data["service"].(string); ok {
		cmd.Service = service
	} else {
		cmd.Service = "toggle" // 默认为切换服务
	}

	if serviceData, ok := msg.Data["service_data"].(map[string]interface{}); ok {
		cmd.ServiceData = serviceData
	}

	return cmd, nil
}

// PlateCommand 门禁系统命令
type PlateCommand struct {
	BaseDeviceCommand
	VoiceText   string `json:"voice_text,omitempty"`
	CallbackURL string `json:"callback_url,omitempty"`
}

// Validate 验证门禁命令
func (p *PlateCommand) Validate() error {
	if err := p.BaseDeviceCommand.Validate(); err != nil {
		return err
	}

	switch p.Command {
	case CommandVoicePlay:
		if p.VoiceText == "" {
			return fmt.Errorf("语音播报命令缺少文本内容")
		}
	}

	return nil
}

// HACommand HomeAssistant命令
type HACommand struct {
	BaseDeviceCommand
	EntityID    string                 `json:"entity_id"`
	Domain      string                 `json:"domain"`
	Service     string                 `json:"service"`
	ServiceData map[string]interface{} `json:"service_data,omitempty"`
}

// Validate 验证HomeAssistant命令
func (h *HACommand) Validate() error {
	if err := h.BaseDeviceCommand.Validate(); err != nil {
		return err
	}

	if h.EntityID == "" {
		return fmt.Errorf("HomeAssistant命令缺少entity_id")
	}

	if h.Domain == "" {
		return fmt.Errorf("HomeAssistant命令缺少domain")
	}

	if h.Service == "" {
		return fmt.Errorf("HomeAssistant命令缺少service")
	}

	return nil
}

// GetEntityID 获取实体ID
func (h *HACommand) GetEntityID() string {
	return h.EntityID
}

// GetDomain 获取域
func (h *HACommand) GetDomain() string {
	return h.Domain
}

// GetService 获取服务
func (h *HACommand) GetService() string {
	return h.Service
}

// GetServiceData 获取服务数据
func (h *HACommand) GetServiceData() map[string]interface{} {
	return h.ServiceData
}
