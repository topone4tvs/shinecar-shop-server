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
			"timestamp":   plateResult.TimeStamp.Timeval.Sec,
		},
	})

	// 发布车牌识别事件到MQTT
	err := h.publishPlateEvent(stationID, &plateResult)
	if err != nil {
		log.Printf("发布车牌识别事件失败: %v", err)
	}

	// 检查是否有待处理的响应（从设备管理器获取）
	deviceManager := h.manager.GetDeviceManager()
	if response, exists := deviceManager.GetPendingPlateResponse(stationID); exists {
		log.Printf("返回门禁响应: 工位=%s", stationID)
		return response, nil
	}

	// 默认响应
	return nil, nil
}

// processIOTrigger 处理IO触发事件
func (h *HTTPServer) processIOTrigger(stationID string, alarm *AlarmGioIn) (interface{}, error) {
	triggerResult := alarm.Result

	log.Printf("IO触发事件: 工位=%s, 源=%d, 值=%d, 设备=%s",
		stationID, triggerResult.Source, triggerResult.Value, alarm.IPAddr)

	// 发布IO触发事件到MQTT
	err := h.publishIOEvent(stationID, &triggerResult)
	if err != nil {
		log.Printf("发布IO触发事件失败: %v", err)
	}

	return nil, nil
}

// processSerialData 处理串口数据
func (h *HTTPServer) processSerialData(stationID string, serialData *SerialData) (interface{}, error) {
	log.Printf("收到串口数据: 工位=%s, 通道=%d, 长度=%d",
		stationID, serialData.SerialChannel, serialData.DataLen)

	// 发布串口数据事件到MQTT
	err := h.publishSerialEvent(stationID, serialData)
	if err != nil {
		log.Printf("发布串口数据事件失败: %v", err)
	}

	return nil, nil
}

// processTriggerImage 处理截图数据
func (h *HTTPServer) processTriggerImage(stationID string, triggerImage *TriggerImage) (interface{}, error) {
	log.Printf("收到截图数据: 工位=%s, 大小=%d", stationID, triggerImage.ImageFileLen)

	// 发布截图事件到MQTT
	err := h.publishImageEvent(stationID, triggerImage)
	if err != nil {
		log.Printf("发布截图事件失败: %v", err)
	}

	return nil, nil
}

// publishPlateEvent 发布车牌识别事件到MQTT
func (h *HTTPServer) publishPlateEvent(stationID string, plateResult *PlateResultDetail) error {
	router := h.manager.GetRouter()

	event := map[string]interface{}{
		"type": "plate_recognition",
		"data": map[string]interface{}{
			"license":      plateResult.License,
			"confidence":   plateResult.Confidence,
			"color_type":   plateResult.ColorType,
			"plate_type":   plateResult.Type,
			"direction":    plateResult.Direction,
			"plate_id":     plateResult.PlateID,
			"is_offline":   plateResult.IsOffline,
			"is_fake":      plateResult.IsFakePlate,
			"trigger_type": plateResult.TriggerType,
			"timestamp":    plateResult.TimeStamp.Timeval.Sec,
			"location":     plateResult.Location,
		},
	}

	return router.PublishToMQTT(stationID, "plate_event", event)
}

// publishIOEvent 发布IO触发事件到MQTT
func (h *HTTPServer) publishIOEvent(stationID string, triggerResult *TriggerResult) error {
	router := h.manager.GetRouter()

	event := map[string]interface{}{
		"type": "io_trigger",
		"data": map[string]interface{}{
			"source": triggerResult.Source,
			"value":  triggerResult.Value,
		},
	}

	return router.PublishToMQTT(stationID, "io_event", event)
}

// publishSerialEvent 发布串口数据事件到MQTT
func (h *HTTPServer) publishSerialEvent(stationID string, serialData *SerialData) error {
	router := h.manager.GetRouter()

	event := map[string]interface{}{
		"type": "serial_data",
		"data": map[string]interface{}{
			"channel":        serialData.Channel,
			"serial_channel": serialData.SerialChannel,
			"data":           serialData.Data,
			"data_len":       serialData.DataLen,
		},
	}

	return router.PublishToMQTT(stationID, "serial_event", event)
}

// publishImageEvent 发布截图事件到MQTT
func (h *HTTPServer) publishImageEvent(stationID string, triggerImage *TriggerImage) error {
	router := h.manager.GetRouter()

	event := map[string]interface{}{
		"type": "image_capture",
		"data": map[string]interface{}{
			"image_len": triggerImage.ImageFileLen,
			"ip_addr":   triggerImage.IPAddr,
		},
	}

	return router.PublishToMQTT(stationID, "image_event", event)
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
