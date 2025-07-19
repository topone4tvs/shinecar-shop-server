package plate

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"sync"
	"time"

	"shop_server/config"
	"shop_server/internal/service"
)

// Service 门禁服务实现
type Service struct {
	config           *config.Config
	pendingResponses map[string][]*Response
	responseMutex    sync.RWMutex
	deviceStatus     map[string]*service.DeviceStatus
	statusMutex      sync.RWMutex
}

// Response 门禁响应结构
type Response struct {
	ResponseAlarmInfoPlate ResponseAlarmInfoPlate `json:"Response_AlarmInfoPlate"`
}

// ResponseAlarmInfoPlate 门禁响应主体
type ResponseAlarmInfoPlate struct {
	Info                  string                 `json:"info,omitempty"`                    // 开闸指令："ok"表示开闸
	Content               string                 `json:"content,omitempty"`                 // 附加内容
	IsPay                 string                 `json:"is_pay,omitempty"`                  // 是否付费
	ManualTrigger         string                 `json:"manualTrigger,omitempty"`           // 手动触发："ok"表示触发
	PlayServerJSONRequest *PlayServerJSONRequest `json:"playserver_json_request,omitempty"` // 语音播报
	IvsIOCtrl             *IvsIOCtrl             `json:"ivs_ioctrl,omitempty"`              // IO控制
	TriggerImage          *TriggerImage          `json:"TriggerImage,omitempty"`            // 截图
	SerialData            []SerialData           `json:"serialData,omitempty"`              // 串口数据
	ContinuePushOffline   *ContinuePushOffline   `json:"ContinuePushOffline,omitempty"`     // 离线重传控制
}

// PlayServerJSONRequest 语音播报请求
type PlayServerJSONRequest struct {
	Type          string `json:"type"`           // "ps_voice_play"
	Voice         string `json:"voice"`          // BASE64编码的语音内容
	VoiceInterval int    `json:"voice_interval"` // 播放间隔
	VoiceVolume   int    `json:"voice_volume"`   // 音量 1-100
	VoiceMale     int    `json:"voice_male"`     // 语音类型：0男声，1女声
}

// IvsIOCtrl IO控制
type IvsIOCtrl struct {
	Delay int `json:"delay"` // 延迟时间(ms)
	IO    int `json:"io"`    // IO序号 0-3
	Value int `json:"value"` // 状态值：0断，1通，2先通后断
}

// TriggerImage 截图
type TriggerImage struct {
	Port                   int    `json:"port,omitempty"`                   // 端口号
	SnapImageRelativeURL   string `json:"snapImageRelativeUrl,omitempty"`   // 相对路径
	SnapImageAbsolutelyURL string `json:"snapImageAbsolutelyUrl,omitempty"` // 绝对路径
}

// SerialData 串口数据
type SerialData struct {
	SerialChannel int    `json:"serialChannel"` // 串口通道号
	Data          string `json:"data"`          // BASE64编码的数据
	DataLen       int    `json:"dataLen"`       // 数据长度
}

// ContinuePushOffline 离线重传控制
type ContinuePushOffline struct {
	PlateID  int `json:"plateid"`  // 车牌记录ID
	Continue int `json:"continue"` // 是否继续：0否，1是
}

// NewService 创建门禁服务
func NewService(cfg *config.Config) *Service {
	return &Service{
		config:           cfg,
		pendingResponses: make(map[string][]*Response),
		deviceStatus:     make(map[string]*service.DeviceStatus),
	}
}

// GetDeviceType 获取设备类型
func (s *Service) GetDeviceType() string {
	return service.DeviceTypePlate
}

// ExecuteCommand 执行设备命令
func (s *Service) ExecuteCommand(ctx context.Context, cmd service.DeviceCommand) (service.DeviceResponse, error) {
	// 门禁系统命令通过响应机制实现，这里只是准备响应数据
	log.Printf("准备门禁设备响应: 命令=%s, 工位=%s", cmd.GetCommand(), cmd.GetStationID())
	return &service.BaseDeviceResponse{
		DeviceType: service.DeviceTypePlate,
		Command:    cmd.GetCommand(),
		StationID:  cmd.GetStationID(),
		Success:    true,
	}, nil
}

// GetDeviceStatus 获取设备状态
func (s *Service) GetDeviceStatus(ctx context.Context, stationID string) (service.DeviceStatus, error) {
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()

	if status, exists := s.deviceStatus[stationID]; exists {
		return *status, nil
	}

	return service.DeviceStatus{}, fmt.Errorf("工位 %s 的门禁设备状态不存在", stationID)
}

// IsHealthy 检查设备健康状态
func (s *Service) IsHealthy(ctx context.Context) bool {
	// 检查是否有任何在线设备
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()

	for _, status := range s.deviceStatus {
		if status.Online && time.Since(status.LastSeen) < 5*time.Minute {
			return true
		}
	}

	return false
}

// OpenGate 开闸
func (s *Service) OpenGate(ctx context.Context, stationID string) error {
	response := &Response{
		ResponseAlarmInfoPlate: ResponseAlarmInfoPlate{
			Info:    "ok",
			Content: "开闸成功",
			IsPay:   "true",
		},
	}

	return s.PrepareResponse(stationID, response)
}

// CloseGate 关闸
func (s *Service) CloseGate(ctx context.Context, stationID string) error {
	response := &Response{
		ResponseAlarmInfoPlate: ResponseAlarmInfoPlate{
			IvsIOCtrl: &IvsIOCtrl{
				Delay: 0,
				IO:    0,
				Value: 0, // 断开
			},
		},
	}

	return s.PrepareResponse(stationID, response)
}

// PlayVoice 语音播报
func (s *Service) PlayVoice(ctx context.Context, stationID string, text string) error {
	// 将文本编码为BASE64
	voice := base64.StdEncoding.EncodeToString([]byte(text))

	response := &Response{
		ResponseAlarmInfoPlate: ResponseAlarmInfoPlate{
			PlayServerJSONRequest: &PlayServerJSONRequest{
				Type:          "ps_voice_play",
				Voice:         voice,
				VoiceInterval: 0,
				VoiceVolume:   100,
				VoiceMale:     1, // 女声
			},
		},
	}

	log.Printf("准备语音播报响应: 工位=%s, 文本=%s", stationID, text)
	return s.PrepareResponse(stationID, response)
}

// TriggerRecognition 手动触发识别
func (s *Service) TriggerRecognition(ctx context.Context, stationID string) error {
	response := &Response{
		ResponseAlarmInfoPlate: ResponseAlarmInfoPlate{
			ManualTrigger: "ok",
		},
	}

	return s.PrepareResponse(stationID, response)
}

// TakeSnapshot 截图
func (s *Service) TakeSnapshot(ctx context.Context, stationID string, callbackURL string) error {
	response := &Response{
		ResponseAlarmInfoPlate: ResponseAlarmInfoPlate{
			TriggerImage: &TriggerImage{
				Port:                   8080,
				SnapImageAbsolutelyURL: callbackURL,
			},
		},
	}

	return s.PrepareResponse(stationID, response)
}

// PrepareResponse 准备响应数据
func (s *Service) PrepareResponse(stationID string, response interface{}) error {
	s.responseMutex.Lock()
	defer s.responseMutex.Unlock()

	if plateResponse, ok := response.(*Response); ok {
		s.pendingResponses[stationID] = append(s.pendingResponses[stationID], plateResponse)
		log.Printf("准备门禁响应数据: 工位=%s", stationID)
		return nil
	}

	return fmt.Errorf("无效的响应数据类型")
}

// GetPendingResponse 获取待处理的响应
func (s *Service) GetPendingResponse(stationID string) (interface{}, bool) {
	s.responseMutex.Lock()
	defer s.responseMutex.Unlock()

	responses, exists := s.pendingResponses[stationID]
	if exists && len(responses) > 0 {
		response := responses[0]
		if len(responses) == 1 {
			delete(s.pendingResponses, stationID)
		} else {
			s.pendingResponses[stationID] = responses[1:]
		}
		log.Printf("获取门禁响应数据: 工位=%s", stationID)
		return response, true
	}

	return nil, false
}

// UpdateDeviceStatus 更新设备状态
func (s *Service) UpdateDeviceStatus(stationID string, online bool, data map[string]interface{}) {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()

	status := &service.DeviceStatus{
		DeviceType: service.DeviceTypePlate,
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
	log.Printf("更新门禁设备状态: 工位=%s, 在线=%v", stationID, online)
}

// GetAllStations 获取所有工位状态
func (s *Service) GetAllStations() map[string]*service.DeviceStatus {
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()

	result := make(map[string]*service.DeviceStatus)
	for k, v := range s.deviceStatus {
		result[k] = v
	}

	return result
}

// ClearPendingResponses 清理过期的待处理响应
func (s *Service) ClearPendingResponses() {
	s.responseMutex.Lock()
	defer s.responseMutex.Unlock()

	cleared := 0
	for _, responses := range s.pendingResponses {
		cleared += len(responses)
	}
	// 清空所有队列
	s.pendingResponses = make(map[string][]*Response)

	if cleared > 0 {
		log.Printf("清理了 %d 个待处理的门禁响应", cleared)
	}
}

// GetStationEndpoint 获取工位的HTTP端点
func (s *Service) GetStationEndpoint(stationID string) (string, error) {
	station, err := s.config.GetStationByID(stationID)
	if err != nil {
		return "", err
	}

	if !station.Devices.Plate.Enable {
		return "", fmt.Errorf("工位 %s 的门禁设备未启用", stationID)
	}

	return station.Devices.Plate.Endpoint, nil
}

// CreateCombinedResponse 创建组合响应（支持多个命令）
func (s *Service) CreateCombinedResponse(stationID string, commands []string, data map[string]interface{}) error {
	response := &Response{
		ResponseAlarmInfoPlate: ResponseAlarmInfoPlate{},
	}

	for _, command := range commands {
		switch command {
		case service.CommandOpenGate:
			response.ResponseAlarmInfoPlate.Info = "ok"
		case service.CommandVoicePlay:
			if text, ok := data["text"].(string); ok {
				voice := base64.StdEncoding.EncodeToString([]byte(text))
				response.ResponseAlarmInfoPlate.PlayServerJSONRequest = &PlayServerJSONRequest{
					Type:          "ps_voice_play",
					Voice:         voice,
					VoiceInterval: 0,
					VoiceVolume:   100,
					VoiceMale:     1,
				}
			}
		case service.CommandTrigger:
			response.ResponseAlarmInfoPlate.ManualTrigger = "ok"
		}
	}

	return s.PrepareResponse(stationID, response)
}
