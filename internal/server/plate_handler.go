package server

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"shop_server/internal/service"
)

// PlateMessage 门禁设备推送的消息结构
type PlateMessage struct {
	AlarmInfoPlate *AlarmInfoPlate `json:"AlarmInfoPlate,omitempty"`
	AlarmGioIn     *AlarmGioIn     `json:"AlarmGioIn,omitempty"`
	SerialData     *SerialData     `json:"SerialData,omitempty"`
	TriggerImage   *TriggerImage   `json:"TriggerImage,omitempty"`
}

// AlarmInfoPlate 车牌识别结果
type AlarmInfoPlate struct {
	Channel    int         `json:"channel"`
	DeviceName string      `json:"deviceName"`
	IPAddr     string      `json:"ipaddr"`
	SerialNo   string      `json:"serialno"`
	RuleID     int         `json:"rule_id"`
	UserData   string      `json:"user_data"`
	Result     PlateResult `json:"result"`
}

// PlateResult 车牌识别结果详情
type PlateResult struct {
	PlateResult PlateResultDetail `json:"PlateResult"`
}

// PlateResultDetail 车牌识别详细信息
type PlateResultDetail struct {
	License      string    `json:"license"`
	Confidence   int       `json:"confidence"`
	ColorType    int       `json:"colorType"`
	Type         int       `json:"type"`
	Direction    int       `json:"direction"`
	PlateID      int       `json:"plateid"`
	IsOffline    int       `json:"isoffline"`
	IsFakePlate  int       `json:"is_fake_plate"`
	TimeStamp    TimeStamp `json:"timeStamp"`
	Location     Location  `json:"location"`
	ImageFile    string    `json:"imageFile"`
	ImageFileLen int       `json:"imageFileLen"`
	TriggerType  int       `json:"triggerType"`
	GIOOuts      []GIOOut  `json:"gioouts"`
}

// TimeStamp 时间戳
type TimeStamp struct {
	Timeval Timeval `json:"Timeval"`
}

// Timeval 时间值
type Timeval struct {
	DecYear int   `json:"decyear"`
	DecMon  int   `json:"decmon"`
	DecDay  int   `json:"decday"`
	DecHour int   `json:"dechour"`
	DecMin  int   `json:"decmin"`
	DecSec  int   `json:"decsec"`
	Sec     int64 `json:"sec"`
	USec    int64 `json:"usec"`
}

// Location 位置信息
type Location struct {
	RECT RECT `json:"RECT"`
}

// RECT 矩形区域
type RECT struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
}

// GIOOut IO输出信息
type GIOOut struct {
	IONum    string `json:"ionum"`
	CtrlType int    `json:"ctrltype"`
}

// AlarmGioIn IO输入触发
type AlarmGioIn struct {
	DeviceName string        `json:"deviceName"`
	IPAddr     string        `json:"ipaddr"`
	SerialNo   string        `json:"serialno"`
	Result     TriggerResult `json:"result"`
}

// TriggerResult 触发结果
type TriggerResult struct {
	Source int `json:"source"`
	Value  int `json:"value"`
}

// SerialData 串口数据
type SerialData struct {
	Channel       int    `json:"channel"`
	SerialNo      string `json:"serialno"`
	IPAddr        string `json:"ipaddr"`
	DeviceName    string `json:"deviceName"`
	SerialChannel int    `json:"serialChannel"`
	Data          string `json:"data"`
	DataLen       int    `json:"dataLen"`
}

// TriggerImage 截图数据
type TriggerImage struct {
	IPAddr       string `json:"ipaddr"`
	ImageFile    string `json:"imageFile"`
	ImageFileLen int    `json:"imageFileLen"`
}

// LCD/广告/OSD相关结构体

type LCDConfig struct {
	Type     string      `json:"type"`
	Module   string      `json:"module"`
	ReplyURL string      `json:"reply_url,omitempty"`
	Body     interface{} `json:"body"`
}

type AdPushMessage struct {
	Type     string      `json:"type"`
	Module   string      `json:"module"`
	ReplyURL string      `json:"reply_url,omitempty"`
	Body     interface{} `json:"body"`
}

type OsdConfig struct {
	SetOsdPara string      `json:"set_osd_para"`
	Body       interface{} `json:"body"`
}

// 新增统一MQTT转发方法
type MqttEvent map[string]interface{}

func (h *HTTPServer) publishToMQTTEvent(stationID string, event MqttEvent) error {
	router := h.manager.GetRouter()
	// 使用统一的发布topic格式
	return router.PublishToMQTT(stationID, "device_event", event)
}

// parsePlateMessage 解析门禁设备推送消息
func (h *HTTPServer) parsePlateMessage(body []byte) (*PlateMessage, error) {
	var plateMsg PlateMessage

	if err := json.Unmarshal(body, &plateMsg); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	return &plateMsg, nil
}

// processPlateMessage 处理门禁设备消息
func (h *HTTPServer) processPlateMessage(stationID string, plateMsg *PlateMessage) (interface{}, error) {
	// 先判断是否为LCD/广告/OSD相关消息
	var raw map[string]interface{}
	if plateMsg != nil {
		b, _ := json.Marshal(plateMsg)
		_ = json.Unmarshal(b, &raw)
		// 检查顶层是否有LCD/广告/OSD相关字段
		if v, ok := raw["type"]; ok {
			typeStr, _ := v.(string)
			if typeStr == "set_ad_config" || typeStr == "get_ad_config" || typeStr == "set_ad_voice_config" || typeStr == "get_ad_voice_config" || typeStr == "ad_push_message" || typeStr == "set_osd_para" || typeStr == "AVS_SET_PIC_OSD_PRM" {
				h.processLCDRelatedMessage(stationID, raw)
				return nil, nil
			}
		}
		// 兼容 Response_AlarmInfoPlate 下的 type/module
		if resp, ok := raw["Response_AlarmInfoPlate"]; ok {
			if m, ok := resp.(map[string]interface{}); ok {
				if v, ok := m["type"]; ok {
					typeStr, _ := v.(string)
					if typeStr == "set_ad_config" || typeStr == "get_ad_config" || typeStr == "set_ad_voice_config" || typeStr == "get_ad_voice_config" || typeStr == "ad_push_message" || typeStr == "set_osd_para" || typeStr == "AVS_SET_PIC_OSD_PRM" {
						h.processLCDRelatedMessage(stationID, m)
						return nil, nil
					}
				}
			}
		}
	}
	// 根据消息类型进行处理
	if plateMsg.AlarmInfoPlate != nil {
		return h.processPlateRecognition(stationID, plateMsg.AlarmInfoPlate)
	} else if plateMsg.AlarmGioIn != nil {
		return h.processIOTrigger(stationID, plateMsg.AlarmGioIn)
	} else if plateMsg.SerialData != nil {
		return h.processSerialData(stationID, plateMsg.SerialData)
	} else if plateMsg.TriggerImage != nil {
		return h.processTriggerImage(stationID, plateMsg.TriggerImage)
	}

	return nil, fmt.Errorf("未知的门禁消息类型")
}

// processPlateRecognition 处理车牌识别结果
func (h *HTTPServer) processPlateRecognition(stationID string, alarm *AlarmInfoPlate) (interface{}, error) {
	plateResult := alarm.Result.PlateResult
	timestamp := plateResult.TimeStamp.Timeval.Sec

	log.Printf("车牌识别结果: 工位=%s, 车牌=%s, 置信度=%d, 设备=%s",
		stationID, plateResult.License, plateResult.Confidence, alarm.IPAddr)

	// 更新设备状态
	h.manager.GetDeviceManager().UpdateDeviceStatus(stationID, &service.DeviceStatus{
		DeviceType: service.DeviceTypePlate,
		StationID:  stationID,
		Online:     true,
		LastSeen:   time.Now(),
		Status:     "online",
		Data: map[string]interface{}{
			"device_name": alarm.DeviceName,
			"ip_address":  alarm.IPAddr,
			"serial_no":   alarm.SerialNo,
			"last_plate":  plateResult.License,
			"confidence":  plateResult.Confidence,
			"timestamp":   timestamp,
		},
	})

	// 统一格式化并转发到MQTT
	event := MqttEvent{
		"type":        "plate_recognition",
		"station_id":  stationID,
		"timestamp":   timestamp,
		"device_name": alarm.DeviceName,
		"ipaddr":      alarm.IPAddr,
		"serialno":    alarm.SerialNo,
		"plate": map[string]interface{}{
			"license":        plateResult.License,
			"confidence":     plateResult.Confidence,
			"color_type":     plateResult.ColorType,
			"type":           plateResult.Type,
			"direction":      plateResult.Direction,
			"plateid":        plateResult.PlateID,
			"isoffline":      plateResult.IsOffline,
			"is_fake_plate":  plateResult.IsFakePlate,
			"image_file":     plateResult.ImageFile,
			"image_file_len": plateResult.ImageFileLen,
			"location":       plateResult.Location.RECT,
		},
		"raw": alarm,
	}
	err := h.publishToMQTTEvent(stationID, event)
	if err != nil {
		log.Printf("发布车牌识别事件失败: %v", err)
	}

	return nil, nil
}

// processIOTrigger 处理IO触发事件
func (h *HTTPServer) processIOTrigger(stationID string, alarm *AlarmGioIn) (interface{}, error) {
	triggerResult := alarm.Result
	timestamp := time.Now().Unix()

	log.Printf("IO触发事件: 工位=%s, 源=%d, 值=%d, 设备=%s",
		stationID, triggerResult.Source, triggerResult.Value, alarm.IPAddr)

	event := MqttEvent{
		"type":        "io_trigger",
		"station_id":  stationID,
		"timestamp":   timestamp,
		"device_name": alarm.DeviceName,
		"ipaddr":      alarm.IPAddr,
		"serialno":    alarm.SerialNo,
		"io": map[string]interface{}{
			"source": triggerResult.Source,
			"value":  triggerResult.Value,
		},
		"raw": alarm,
	}
	err := h.publishToMQTTEvent(stationID, event)
	if err != nil {
		log.Printf("发布IO触发事件失败: %v", err)
	}

	return nil, nil
}

// processSerialData 处理串口数据
func (h *HTTPServer) processSerialData(stationID string, serialData *SerialData) (interface{}, error) {
	timestamp := time.Now().Unix()

	log.Printf("收到串口数据: 工位=%s, 通道=%d, 长度=%d",
		stationID, serialData.SerialChannel, serialData.DataLen)

	event := MqttEvent{
		"type":        "serial_data",
		"station_id":  stationID,
		"timestamp":   timestamp,
		"device_name": serialData.DeviceName,
		"ipaddr":      serialData.IPAddr,
		"serialno":    serialData.SerialNo,
		"serial": map[string]interface{}{
			"serial_channel": serialData.SerialChannel,
			"data":           serialData.Data,
			"data_len":       serialData.DataLen,
		},
		"raw": serialData,
	}
	err := h.publishToMQTTEvent(stationID, event)
	if err != nil {
		log.Printf("发布串口数据事件失败: %v", err)
	}

	return nil, nil
}

// processTriggerImage 处理截图数据
func (h *HTTPServer) processTriggerImage(stationID string, triggerImage *TriggerImage) (interface{}, error) {
	timestamp := time.Now().Unix()

	log.Printf("收到截图数据: 工位=%s, 大小=%d", stationID, triggerImage.ImageFileLen)

	event := MqttEvent{
		"type":       "image_capture",
		"station_id": stationID,
		"timestamp":  timestamp,
		"ipaddr":     triggerImage.IPAddr,
		"image": map[string]interface{}{
			"image_file":     triggerImage.ImageFile,
			"image_file_len": triggerImage.ImageFileLen,
		},
		"raw": triggerImage,
	}
	err := h.publishToMQTTEvent(stationID, event)
	if err != nil {
		log.Printf("发布截图事件失败: %v", err)
	}

	return nil, nil
}

// publishSnapshotEvent 发布截图上传事件到MQTT
func (h *HTTPServer) publishSnapshotEvent(stationID string, size int) error {
	router := h.manager.GetRouter()

	event := map[string]interface{}{
		"type": "snapshot_upload",
		"data": map[string]interface{}{
			"size":      size,
			"timestamp": time.Now().Unix(),
		},
	}

	return router.PublishToMQTT(stationID, "snapshot_event", event)
}

// 统一处理LCD/广告/OSD相关下发和转发
func (h *HTTPServer) processLCDRelatedMessage(stationID string, msg map[string]interface{}) {
	timestamp := time.Now().Unix()
	event := MqttEvent{
		"type":       "lcd_config",
		"station_id": stationID,
		"timestamp":  timestamp,
		"lcd_config": msg,
	}
	h.publishToMQTTEvent(stationID, event)
}
