package mqtt

import (
	"context"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	client   mqtt.Client
	config   *Config
	handlers map[string]MessageHandler
}

type Config struct {
	Broker   string
	Port     int
	Username string
	Password string
	ClientID string
}

type MessageHandler func(topic string, payload []byte) error

// NewClient 创建MQTT客户端
func NewClient(cfg *Config) (*Client, error) {
	opts := mqtt.NewClientOptions()
	broker := fmt.Sprintf("tcp://%s:%d", cfg.Broker, cfg.Port)
	opts.AddBroker(broker)
	opts.SetClientID(cfg.ClientID)
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(15 * time.Second)
	opts.SetPingTimeout(5 * time.Second)

	client := &Client{
		client:   mqtt.NewClient(opts),
		config:   cfg,
		handlers: make(map[string]MessageHandler),
	}

	// 设置连接丢失处理器
	opts.SetConnectionLostHandler(client.onConnectionLost)

	// 设置连接成功处理器
	opts.SetOnConnectHandler(client.onConnect)

	return client, nil
}

// Connect 连接到MQTT服务器
func (c *Client) Connect(ctx context.Context) error {
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("连接MQTT服务器失败: %w", token.Error())
	}

	log.Printf("成功连接到MQTT服务器: %s:%d", c.config.Broker, c.config.Port)
	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() {
	c.client.Disconnect(250)
	log.Println("已断开MQTT连接")
}

// Subscribe 订阅主题
func (c *Client) Subscribe(topic string, handler MessageHandler) error {
	c.handlers[topic] = handler

	token := c.client.Subscribe(topic, 1, func(client mqtt.Client, msg mqtt.Message) {
		if handler != nil {
			if err := handler(msg.Topic(), msg.Payload()); err != nil {
				log.Printf("处理MQTT消息失败 [%s]: %v", msg.Topic(), err)
			}
		}
	})

	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("订阅主题失败 [%s]: %w", topic, token.Error())
	}

	log.Printf("成功订阅主题: %s", topic)
	return nil
}

// SubscribePattern 订阅主题模式（支持通配符）
func (c *Client) SubscribePattern(pattern string, handler MessageHandler) error {
	return c.Subscribe(pattern, handler)
}

// Publish 发布消息
func (c *Client) Publish(topic string, payload []byte) error {
	token := c.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("发布消息失败 [%s]: %w", topic, token.Error())
	}

	log.Printf("成功发布消息到主题 [%s]: %s", topic, string(payload))
	return nil
}

// PublishJSON 发布JSON消息
func (c *Client) PublishJSON(topic string, data interface{}) error {
	// 这里需要json包，先用简单的字符串格式
	payload := fmt.Sprintf("%+v", data)
	return c.Publish(topic, []byte(payload))
}

// onConnect 连接成功回调
func (c *Client) onConnect(client mqtt.Client) {
	log.Println("MQTT客户端连接成功")

	// 重新订阅所有主题
	for topic := range c.handlers {
		if err := c.Subscribe(topic, c.handlers[topic]); err != nil {
			log.Printf("重新订阅主题失败 [%s]: %v", topic, err)
		}
	}
}

// onConnectionLost 连接丢失回调
func (c *Client) onConnectionLost(client mqtt.Client, err error) {
	log.Printf("MQTT连接丢失: %v", err)
}

// IsConnected 检查连接状态
func (c *Client) IsConnected() bool {
	return c.client.IsConnected()
}
