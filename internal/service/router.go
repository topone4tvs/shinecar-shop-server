package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"shop_server/config"
	"shop_server/pkg/mqtt"
)

// Router 消息路由器
type Router struct {
	config        *config.Config
	mqttClient    *mqtt.Client
	converter     MessageConverter
	deviceManager *DeviceManager
}

// NewRouter 创建消息路由器
func NewRouter(cfg *config.Config, mqttClient *mqtt.Client) *Router {
	router := &Router{
		config:        cfg,
		mqttClient:    mqttClient,
		converter:     NewMessageConverter(cfg),
		deviceManager: NewDeviceManager(cfg),
	}

	return router
}

// Start 启动路由器
func (r *Router) Start(ctx context.Context) error {
	log.Println("启动消息路由器...")

	// 订阅MQTT主题
	if err := r.subscribeTopics(); err != nil {
		return fmt.Errorf("订阅MQTT主题失败: %w", err)
	}

	// 启动设备健康检查
	go r.deviceManager.StartHealthCheck(ctx)

	log.Println("消息路由器启动成功")
	return nil
}

// Stop 停止路由器
func (r *Router) Stop(ctx context.Context) error {
	log.Println("停止消息路由器...")
	// 这里可以添加清理逻辑
	return nil
}

// subscribeTopics 订阅MQTT主题
func (r *Router) subscribeTopics() error {
	// 构建订阅主题模式 - 用于接收上游消息
	subscribePattern := fmt.Sprintf("shinecar/shop/%s/station/+/command", r.config.Shop.ID)

	// 订阅主题
	return r.mqttClient.SubscribePattern(subscribePattern, r.handleMQTTMessage)
}

// handleMQTTMessage 处理MQTT消息
func (r *Router) handleMQTTMessage(topic string, payload []byte) error {
	log.Printf("收到MQTT消息 - 主题: %s, 内容: %s", topic, string(payload))

	// 解析主题，提取工位ID
	stationID, err := r.parseStationID(topic)
	if err != nil {
		return fmt.Errorf("解析工位ID失败: %w", err)
	}

	// 解析消息内容
	mqttMsg, err := ParseMQTTMessage(payload)
	if err != nil {
		return fmt.Errorf("解析消息内容失败: %w", err)
	}

	// 验证消息
	if err := r.validateMessage(mqttMsg, stationID); err != nil {
		return fmt.Errorf("消息验证失败: %w", err)
	}

	// 处理消息
	return r.processMessage(mqttMsg)
}

// processMessage 处理消息
func (r *Router) processMessage(msg *MQTTMessage) error {
	log.Printf("处理消息: %s", msg.String())

	ctx := context.Background()

	switch msg.Type {
	case MessageTypeDevice:
		return r.processDeviceMessage(ctx, msg)
	case MessageTypeQuery:
		return r.processQueryMessage(ctx, msg)
	case MessageTypeConfig:
		return r.processConfigMessage(ctx, msg)
	default:
		return fmt.Errorf("未知消息类型: %s", msg.Type)
	}
}

// processDeviceMessage 处理设备消息
func (r *Router) processDeviceMessage(ctx context.Context, msg *MQTTMessage) error {
	// 转换为设备命令
	deviceCmd, err := r.converter.ConvertToDeviceCommand(msg)
	if err != nil {
		return fmt.Errorf("消息转换失败: %w", err)
	}

	// 执行设备命令
	response, err := r.deviceManager.ExecuteCommand(ctx, deviceCmd)
	if err != nil {
		log.Printf("执行设备命令失败: %v", err)
		return err
	}

	// 发布响应（如果需要）
	if err := r.publishResponse(response); err != nil {
		log.Printf("发布响应失败: %v", err)
		return err
	}

	log.Printf("设备命令执行成功: 命令=%s, 设备=%s, 工位=%s",
		response.GetCommand(), response.GetDeviceType(), response.GetStationID())

	return nil
}

// processQueryMessage 处理查询消息
func (r *Router) processQueryMessage(ctx context.Context, msg *MQTTMessage) error {
	log.Printf("处理查询消息: %s", msg.Command)

	var responseData interface{}
	var err error

	switch msg.Command {
	case CommandGetStatus:
		responseData, err = r.getDeviceStatus(msg.StationID)
	case "get_all_status":
		responseData = r.getAllDeviceStatus()
	default:
		return fmt.Errorf("不支持的查询命令: %s", msg.Command)
	}

	if err != nil {
		return err
	}

	// 发布查询结果
	return r.publishQueryResult(msg, responseData)
}

// processConfigMessage 处理配置消息
func (r *Router) processConfigMessage(ctx context.Context, msg *MQTTMessage) error {
	log.Printf("处理配置消息: %s", msg.Command)

	switch msg.Command {
	case "reload_config":
		return r.reloadConfig()
	case "update_station":
		return r.updateStationConfig(msg)
	default:
		return fmt.Errorf("不支持的配置命令: %s", msg.Command)
	}
}

// getDeviceStatus 获取设备状态
func (r *Router) getDeviceStatus(stationID string) (interface{}, error) {
	status, err := r.deviceManager.GetDeviceStatus(stationID)
	if err != nil {
		return nil, err
	}

	return status, nil
}

// getAllDeviceStatus 获取所有设备状态
func (r *Router) getAllDeviceStatus() interface{} {
	return r.deviceManager.GetAllDeviceStatus()
}

// publishResponse 发布设备响应
func (r *Router) publishResponse(response DeviceResponse) error {
	if !response.IsSuccess() {
		// 只在失败时发布响应
		responseMsg, err := r.converter.ConvertToMQTTMessage(response)
		if err != nil {
			return err
		}

		return r.PublishToMQTT(response.GetStationID(), "device_response", responseMsg)
	}

	return nil
}

// publishQueryResult 发布查询结果
func (r *Router) publishQueryResult(originalMsg *MQTTMessage, data interface{}) error {
	responseMsg := &MQTTMessage{
		Type:      MessageTypeQuery,
		Command:   originalMsg.Command + "_result",
		ShopID:    r.config.Shop.ID,
		StationID: originalMsg.StationID,
		Data: map[string]interface{}{
			"result": data,
		},
	}

	return r.PublishToMQTT(originalMsg.StationID, "query_result", responseMsg)
}

// reloadConfig 重新加载配置
func (r *Router) reloadConfig() error {
	log.Println("重新加载配置...")
	// 这里可以实现配置重新加载逻辑
	// 目前只是日志记录
	return nil
}

// updateStationConfig 更新工位配置
func (r *Router) updateStationConfig(msg *MQTTMessage) error {
	log.Printf("更新工位配置: %s", msg.StationID)
	// 这里可以实现工位配置更新逻辑
	// 目前只是日志记录
	return nil
}

// parseStationID 从主题中解析工位ID
func (r *Router) parseStationID(topic string) (string, error) {
	// 主题格式: shinecar/shop/{shop_id}/station/{station_id}/command
	parts := strings.Split(topic, "/")
	if len(parts) != 6 {
		return "", fmt.Errorf("主题格式错误: %s", topic)
	}

	if parts[0] != "shinecar" || parts[1] != "shop" || parts[3] != "station" || parts[5] != "command" {
		return "", fmt.Errorf("主题格式错误: %s", topic)
	}

	shopID := parts[2]
	stationID := parts[4]

	// 验证店铺ID
	if shopID != r.config.Shop.ID {
		return "", fmt.Errorf("店铺ID不匹配: 期望 %s, 实际 %s", r.config.Shop.ID, shopID)
	}

	return stationID, nil
}

// validateMessage 验证消息
func (r *Router) validateMessage(msg *MQTTMessage, stationID string) error {
	// 使用协议模块的验证功能
	if err := ValidateMQTTMessage(msg); err != nil {
		return err
	}

	// 验证工位ID匹配
	if msg.StationID != stationID {
		return fmt.Errorf("消息中的工位ID不匹配: 期望 %s, 实际 %s", stationID, msg.StationID)
	}

	// 验证店铺ID
	if msg.ShopID != r.config.Shop.ID {
		return fmt.Errorf("店铺ID不匹配: 期望 %s, 实际 %s", r.config.Shop.ID, msg.ShopID)
	}

	// 验证工位是否存在于配置中
	if _, err := r.config.GetStationByID(stationID); err != nil {
		return fmt.Errorf("工位不存在: %w", err)
	}

	return nil
}

// PublishToMQTT 发布消息到MQTT
func (r *Router) PublishToMQTT(stationID string, messageType string, data interface{}) error {
	// 构建发布主题 - 用于向上游发送消息
	topic := fmt.Sprintf("shinecar/shop/%s/station/%s/event", r.config.Shop.ID, stationID)

	// 构建消息
	var payload []byte
	var err error

	if mqttMsg, ok := data.(*MQTTMessage); ok {
		// 如果是MQTTMessage，直接序列化
		payload, err = mqttMsg.ToJSON()
	} else {
		// 否则构建标准消息格式
		message := map[string]interface{}{
			"type":       messageType,
			"shop_id":    r.config.Shop.ID,
			"station_id": stationID,
			"data":       data,
		}
		payload, err = json.Marshal(message)
	}

	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	return r.mqttClient.Publish(topic, payload)
}

// GetDeviceManager 获取设备管理器
func (r *Router) GetDeviceManager() *DeviceManager {
	return r.deviceManager
}
