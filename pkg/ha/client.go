package ha

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Client HomeAssistant客户端
type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

// Config HomeAssistant配置
type Config struct {
	BaseURL string            `json:"base_url"`
	Token   string            `json:"token"`
	Timeout int               `json:"timeout"`
	Headers map[string]string `json:"headers"`
}

// EntityState 实体状态
type EntityState struct {
	EntityID   string                 `json:"entity_id"`
	State      string                 `json:"state"`
	Attributes map[string]interface{} `json:"attributes"`
	LastSeen   time.Time              `json:"last_changed"`
}

// ServiceCallRequest 服务调用请求
type ServiceCallRequest struct {
	EntityID string                 `json:"entity_id,omitempty"`
	Data     map[string]interface{} `json:",inline"`
}

// ServiceCallResponse 服务调用响应
type ServiceCallResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// NewClient 创建HomeAssistant客户端
func NewClient(cfg *Config) *Client {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		baseURL: cfg.BaseURL,
		token:   cfg.Token,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// CallService 调用HomeAssistant服务
func (c *Client) CallService(ctx context.Context, domain, service string, entityID string, data map[string]interface{}) error {
	url := fmt.Sprintf("%s/api/services/%s/%s", c.baseURL, domain, service)

	// 构建请求数据
	requestData := make(map[string]interface{})
	if entityID != "" {
		requestData["entity_id"] = entityID
	}

	// 合并额外数据
	if data != nil {
		for k, v := range data {
			requestData[k] = v
		}
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("序列化请求数据失败: %w", err)
	}

	log.Printf("调用HomeAssistant服务: %s.%s, 实体: %s, 数据: %s", domain, service, entityID, string(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HomeAssistant API调用失败: %s, 响应: %s", resp.Status, string(body))
	}

	log.Printf("HomeAssistant服务调用成功: %s.%s resp: %s", domain, service, string(body))
	return nil
}

// GetEntityState 获取实体状态
func (c *Client) GetEntityState(ctx context.Context, entityID string) (*EntityState, error) {
	url := fmt.Sprintf("%s/api/states/%s", c.baseURL, entityID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("创建HA请求失败: %v", err)
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("实体不存在: %s", entityID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取实体状态失败: %s, 响应: %s", resp.Status, string(body))
	}

	var state EntityState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	log.Printf("获取实体状态成功: %s = %s", entityID, state.State)
	return &state, nil
}

// SetEntityState 设置实体状态
func (c *Client) SetEntityState(ctx context.Context, entityID string, state interface{}) error {
	url := fmt.Sprintf("%s/api/states/%s", c.baseURL, entityID)

	stateData := map[string]interface{}{
		"state": state,
	}

	jsonData, err := json.Marshal(stateData)
	if err != nil {
		return fmt.Errorf("序列化状态数据失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("设置实体状态失败: %s, 响应: %s", resp.Status, string(body))
	}

	log.Printf("设置实体状态成功: %s = %v", entityID, state)
	return nil
}

// GetAllStates 获取所有实体状态
func (c *Client) GetAllStates(ctx context.Context) ([]EntityState, error) {
	url := fmt.Sprintf("%s/api/states", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取所有状态失败: %s, 响应: %s", resp.Status, string(body))
	}

	var states []EntityState
	if err := json.NewDecoder(resp.Body).Decode(&states); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	log.Printf("获取所有状态成功: %d个实体", len(states))
	return states, nil
}

// CheckHealth 检查HomeAssistant健康状态
func (c *Client) CheckHealth(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HomeAssistant健康检查失败: %s", resp.Status)
	}

	log.Println("HomeAssistant健康检查成功")
	return nil
}

// TurnOn 打开设备
func (c *Client) TurnOn(ctx context.Context, entityID string) error {
	domain := c.getDomainFromEntityID(entityID)
	return c.CallService(ctx, domain, "turn_on", entityID, nil)
}

// TurnOff 关闭设备
func (c *Client) TurnOff(ctx context.Context, entityID string) error {
	domain := c.getDomainFromEntityID(entityID)
	return c.CallService(ctx, domain, "turn_off", entityID, nil)
}

// Toggle 切换设备状态
func (c *Client) Toggle(ctx context.Context, entityID string) error {
	domain := c.getDomainFromEntityID(entityID)
	return c.CallService(ctx, domain, "toggle", entityID, nil)
}

func (c *Client) PlayVoice(ctx context.Context, entityID string) error {
	domain := c.getDomainFromEntityID(entityID)
	return c.CallService(ctx, domain, "play_voice", entityID, nil)
}

func (c *Client) PlayText(ctx context.Context, entityID string, text string) error {
	domain := c.getDomainFromEntityID(entityID)
	return c.CallService(ctx, domain, "set_value", entityID, map[string]interface{}{
		"value": text,
	})
}

// setHeaders 设置请求头
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

// getDomainFromEntityID 从实体ID获取域名
func (c *Client) getDomainFromEntityID(entityID string) string {
	// 实体ID格式: domain.entity_name
	for i, char := range entityID {
		if char == '.' {
			return entityID[:i]
		}
	}
	return "switch" // 默认返回switch域
}

// IsEntityOn 检查实体是否为开启状态
func (c *Client) IsEntityOn(ctx context.Context, entityID string) (bool, error) {
	state, err := c.GetEntityState(ctx, entityID)
	if err != nil {
		return false, err
	}

	return state.State == "on", nil
}

// WaitForState 等待实体状态变化
func (c *Client) WaitForState(ctx context.Context, entityID string, expectedState string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待状态变化超时: %s", entityID)
		case <-ticker.C:
			state, err := c.GetEntityState(ctx, entityID)
			if err != nil {
				continue
			}

			if state.State == expectedState {
				return nil
			}
		}
	}
}
