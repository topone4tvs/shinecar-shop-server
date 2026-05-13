package service

import (
	"path/filepath"
	"sync"
	"testing"

	"shop_server/config"
	"shop_server/pkg/logger"
)

var initTestLoggerOnce sync.Once

func initTestLogger(t *testing.T) {
	t.Helper()

	initTestLoggerOnce.Do(func() {
		if err := logger.Init(&logger.Config{
			Level:    "error",
			FilePath: filepath.Join(t.TempDir(), "test.log"),
			Console:  false,
			MaxAge:   1,
		}); err != nil {
			t.Fatalf("failed to init test logger: %v", err)
		}
	})
}

func testConfig() *config.Config {
	return &config.Config{
		Shop: config.ShopConfig{
			ID:   "001",
			Name: "测试店铺",
			Stations: []config.StationConfig{
				{
					ID:   "001",
					Name: "1号工位",
					Devices: struct {
						Plate config.PlateDeviceConfig `yaml:"plate"`
						HA    config.HADeviceConfig    `yaml:"ha"`
					}{
						HA: config.HADeviceConfig{
							Enable:         true,
							AirConditioner: "climate.test_air_conditioner",
							Ventilation:    "switch.test_ventilation",
						},
					},
				},
				{ID: "002", Name: "2号工位"},
			},
		},
		Devices: config.DevicesConfig{
			HA: config.HAConfig{
				BaseURL: "http://127.0.0.1:1",
				Timeout: 1,
			},
		},
	}
}
