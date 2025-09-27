package service

import (
	"encoding/json"
	"fmt"
	"time"
)

// MQTTMessage MQTT消息结构
type MQTTMessage struct {
	Type      string                 `json:"type"`       // 消息类型
	Command   string                 `json:"command"`    // 具体命令
	ShopID    string                 `json:"shop_id"`    // 店铺ID
	StationID string                 `json:"station_id"` // 工位ID
	Data      map[string]interface{} `json:"data"`       // 消息数据
	Timestamp int64                  `json:"timestamp"`  // 时间戳
}

// 支持的消息类型
const (
	MessageTypeDevice = "device" // 设备控制
	MessageTypeQuery  = "query"  // 状态查询
	MessageTypeConfig = "config" // 配置更新
)

// 支持的设备命令
const (
	CommandOpenGate           = "open_gate"            // 开闸
	CommandCloseGate          = "close_gate"           // 关闸
	CommandVoicePlay          = "voice_play"           // 语音播报
	CommandSnapshot           = "snapshot"             // 截图
	CommandTrigger            = "trigger"              // 手动触发
	CommandHAControl          = "ha_control"           // HomeAssistant控制
	CommandHANotice           = "ha_notice"            // HomeAssistant通知
	CommandHAPlayMusic        = "ha_play_music"        // HomeAssistant通知
	CommandHAPauseMusic       = "ha_pause_music"       // HomeAssistant通知
	CommandHASetVolume        = "ha_set_volume"        // HomeAssistant通知
	CommandHAExecuteDirective = "ha_execute_directive" // HomeAssistant通知
	CommandGetStatus          = "get_status"           // 获取状态
	CommandUnionStart         = "union_start"          // 联动订单开始
	CommandUnionFinish        = "union_finish"         // 联动订单结束
)

// 要操作的设备类型
const (
	DeviceTypeUnion = "union" // 联动类型
	DeviceTypePlate = "plate" // 门禁系统
	DeviceTypeHA    = "ha"    // HomeAssistant
)

// DeviceCommand 设备命令接口
type DeviceCommand interface {
	GetDeviceType() string
	GetCommand() string
	GetStationID() string
	GetData() interface{}
	Validate() error
}

// DeviceResponse 设备响应接口
type DeviceResponse interface {
	GetDeviceType() string
	GetCommand() string
	GetStationID() string
	GetData() interface{}
	GetError() error
	IsSuccess() bool
}

// BaseDeviceCommand 基础设备命令
type BaseDeviceCommand struct {
	DeviceType string      `json:"device_type"`
	Command    string      `json:"command"`
	StationID  string      `json:"station_id"`
	Data       interface{} `json:"data"`
}

func (b *BaseDeviceCommand) GetDeviceType() string {
	return b.DeviceType
}

func (b *BaseDeviceCommand) GetCommand() string {
	return b.Command
}

func (b *BaseDeviceCommand) GetStationID() string {
	return b.StationID
}

func (b *BaseDeviceCommand) GetData() interface{} {
	return b.Data
}

func (b *BaseDeviceCommand) Validate() error {
	if b.DeviceType == "" {
		return fmt.Errorf("设备类型不能为空")
	}
	if b.Command == "" {
		return fmt.Errorf("命令不能为空")
	}
	if b.StationID == "" {
		return fmt.Errorf("工位ID不能为空")
	}
	return nil
}

// BaseDeviceResponse 基础设备响应
type BaseDeviceResponse struct {
	DeviceType string      `json:"device_type"`
	Command    string      `json:"command"`
	StationID  string      `json:"station_id"`
	Data       interface{} `json:"data"`
	Error      error       `json:"error,omitempty"`
	Success    bool        `json:"success"`
}

func (b *BaseDeviceResponse) GetDeviceType() string {
	return b.DeviceType
}

func (b *BaseDeviceResponse) GetCommand() string {
	return b.Command
}

func (b *BaseDeviceResponse) GetStationID() string {
	return b.StationID
}

func (b *BaseDeviceResponse) GetData() interface{} {
	return b.Data
}

func (b *BaseDeviceResponse) GetError() error {
	return b.Error
}

func (b *BaseDeviceResponse) IsSuccess() bool {
	return b.Success
}

// ParseMQTTMessage 解析MQTT消息
func ParseMQTTMessage(payload []byte) (*MQTTMessage, error) {
	var msg MQTTMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	// 设置时间戳（如果没有）
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().Unix()
	}

	return &msg, nil
}

// ValidateMQTTMessage 验证MQTT消息
func ValidateMQTTMessage(msg *MQTTMessage) error {
	if msg.Type == "" {
		return fmt.Errorf("消息类型不能为空")
	}

	if msg.Command == "" {
		return fmt.Errorf("命令不能为空")
	}

	if msg.ShopID == "" {
		return fmt.Errorf("店铺ID不能为空")
	}

	if msg.StationID == "" {
		return fmt.Errorf("工位ID不能为空")
	}

	// 验证消息类型
	if !isValidMessageType(msg.Type) {
		return fmt.Errorf("无效的消息类型: %s", msg.Type)
	}

	// 验证命令
	if !isValidCommand(msg.Command) {
		return fmt.Errorf("无效的命令: %s", msg.Command)
	}

	return nil
}

// isValidMessageType 检查消息类型是否有效
func isValidMessageType(msgType string) bool {
	switch msgType {
	case MessageTypeDevice, MessageTypeQuery, MessageTypeConfig:
		return true
	default:
		return false
	}
}

// isValidCommand 检查命令是否有效
func isValidCommand(command string) bool {
	switch command {
	case CommandOpenGate, CommandCloseGate, CommandVoicePlay, CommandSnapshot, CommandTrigger,
		CommandHAControl, CommandHANotice, CommandGetStatus, CommandUnionStart, CommandUnionFinish:
		return true
	default:
		return false
	}
}

// ToJSON 将消息转换为JSON
func (m *MQTTMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// String 返回消息的字符串表示
func (m *MQTTMessage) String() string {
	return fmt.Sprintf("MQTTMessage{Type: %s, Command: %s, ShopID: %s, StationID: %s}",
		m.Type, m.Command, m.ShopID, m.StationID)
}
