package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Environment string        `yaml:"environment"` // 新增：当前启动环境
	Server      ServerConfig  `yaml:"server"`
	MQTT        MQTTConfig    `yaml:"mqtt"`
	Shop        ShopConfig    `yaml:"shop"`
	Devices     DevicesConfig `yaml:"devices"`
	Log         LogConfig     `yaml:"log"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type MQTTConfig struct {
	Broker   string `yaml:"broker"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	ClientID string `yaml:"client_id"`
	Topics   struct {
		Pattern string `yaml:"pattern"` // shinecar/shop/{shop_id}/station/{station_id}
	} `yaml:"topics"`
}

type ShopConfig struct {
	ID       string          `yaml:"id"`
	Name     string          `yaml:"name"`
	Stations []StationConfig `yaml:"stations"`
}

type StationConfig struct {
	ID      string `yaml:"id"`
	Name    string `yaml:"name"`
	Devices struct {
		Plate PlateDeviceConfig `yaml:"plate"`
		HA    HADeviceConfig    `yaml:"ha"`
	} `yaml:"devices"`
}

type PlateDeviceConfig struct {
	Enable   bool   `yaml:"enable"`
	Endpoint string `yaml:"endpoint"` // HTTP推送接收端点
}

type HADeviceConfig struct {
	Enable         bool   `yaml:"enable"`
	EntityID       string `yaml:"entity_id"`
	AirConditioner string `yaml:"air_conditioner"` // 空调设备ID
	Player         string `yaml:"player"`          // 播放器的设备ID
}

type DevicesConfig struct {
	HA HAConfig `yaml:"ha"`
}

type HAConfig struct {
	BaseURL string            `yaml:"base_url"`
	Token   string            `yaml:"token"`
	Timeout int               `yaml:"timeout"`
	Headers map[string]string `yaml:"headers"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

var globalConfig *Config

// Load 加载配置文件
func Load() (*Config, error) {
	// 固定读取 env.yaml 文件
	configFile := "config/env.yaml"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("配置文件不存在: %s", configFile)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证配置
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	globalConfig = &cfg
	return &cfg, nil
}

// Get 获取全局配置
func Get() *Config {
	return globalConfig
}

// GetEnvironment 获取当前环境
func (c *Config) GetEnvironment() string {
	return c.Environment
}

// SetEnvironment 设置当前环境
func (c *Config) SetEnvironment(env string) {
	c.Environment = env
}

// IsDevelopment 判断是否为开发环境
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction 判断是否为生产环境
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// GetConfigPath 获取配置文件路径 (已废弃，现在使用 env.yaml 文件)
func GetConfigPath(env string) string {
	return "config/env.yaml"
}

// validateConfig 验证配置
func validateConfig(cfg *Config) error {
	if cfg.Shop.ID == "" {
		return fmt.Errorf("店铺ID不能为空")
	}

	if len(cfg.Shop.Stations) == 0 {
		return fmt.Errorf("至少需要配置一个工位")
	}

	for _, station := range cfg.Shop.Stations {
		if station.ID == "" {
			return fmt.Errorf("工位ID不能为空")
		}
	}

	return nil
}

// GetMQTTTopic 获取MQTT主题
func (c *Config) GetMQTTTopic(stationID string) string {
	pattern := c.MQTT.Topics.Pattern
	if pattern == "" {
		pattern = "shinecar/shop/{shop_id}/station/{station_id}"
	}

	topic := pattern
	topic = fmt.Sprintf(topic, c.Shop.ID, stationID)
	return topic
}

// GetStationByID 根据工位ID获取工位配置
func (c *Config) GetStationByID(stationID string) (*StationConfig, error) {
	for _, station := range c.Shop.Stations {
		if station.ID == stationID {
			return &station, nil
		}
	}
	return nil, fmt.Errorf("工位ID不存在: %s", stationID)
}
