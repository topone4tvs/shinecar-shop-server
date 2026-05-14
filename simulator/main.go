package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	var (
		target        string
		baseURL       string
		mode          string
		scenario      string
		station       string
		license       string
		ioSource      int
		ioValue       int
		pauseDuration time.Duration
	)

	flag.StringVar(&target, "target", "", "目标业务系统URL, 如 http://localhost:8080/api/plate/station/001")
	flag.StringVar(&baseURL, "base-url", "http://localhost:8080", "shop_server HTTP地址, scenario模式使用")
	flag.StringVar(&mode, "mode", "plate", "推送类型: plate/heartbeat/io")
	flag.StringVar(&scenario, "scenario", "", "场景: heartbeat/plate-recognition/gate-open/gate-close/gate-open-close/main-flow/all")
	flag.StringVar(&station, "station", "001", "工位ID/设备ID")
	flag.StringVar(&license, "license", "浙A73J2W", "车牌号(plate模式)")
	flag.IntVar(&ioSource, "source", 1, "IO触发源(io模式)")
	flag.IntVar(&ioValue, "value", 1, "IO触发值(io模式)")
	flag.DurationVar(&pauseDuration, "pause", 800*time.Millisecond, "scenario模式下步骤间隔")
	flag.Parse()

	if scenario != "" {
		if err := runScenario(scenarioConfig{
			baseURL:       strings.TrimRight(baseURL, "/"),
			scenario:      scenario,
			station:       station,
			license:       license,
			ioSource:      ioSource,
			pauseDuration: pauseDuration,
		}); err != nil {
			log.Fatalf("执行场景失败: %v", err)
		}
		return
	}

	if target == "" {
		fmt.Println("请指定 -target 业务系统URL，或使用 -scenario 配合 -base-url")
		os.Exit(1)
	}

	if err := pushByMode(target, mode, station, license, ioSource, ioValue); err != nil {
		log.Fatal(err)
	}
}

type scenarioConfig struct {
	baseURL       string
	scenario      string
	station       string
	license       string
	ioSource      int
	pauseDuration time.Duration
}

func runScenario(cfg scenarioConfig) error {
	switch cfg.scenario {
	case "heartbeat":
		return pushByMode(endpoint(cfg.baseURL, "heartbeat", cfg.station), "heartbeat", cfg.station, cfg.license, cfg.ioSource, 1)
	case "plate-recognition", "plate":
		return pushByMode(endpoint(cfg.baseURL, "plate", cfg.station), "plate", cfg.station, cfg.license, cfg.ioSource, 1)
	case "gate-open":
		return pushByMode(endpoint(cfg.baseURL, "gio", cfg.station), "io", cfg.station, cfg.license, cfg.ioSource, 1)
	case "gate-close":
		return pushByMode(endpoint(cfg.baseURL, "gio", cfg.station), "io", cfg.station, cfg.license, cfg.ioSource, 0)
	case "gate-open-close":
		if err := pushByMode(endpoint(cfg.baseURL, "gio", cfg.station), "io", cfg.station, cfg.license, cfg.ioSource, 1); err != nil {
			return err
		}
		time.Sleep(cfg.pauseDuration)
		return pushByMode(endpoint(cfg.baseURL, "gio", cfg.station), "io", cfg.station, cfg.license, cfg.ioSource, 0)
	case "main-flow", "all":
		steps := []struct {
			name    string
			target  string
			mode    string
			ioValue int
		}{
			{name: "设备心跳", target: endpoint(cfg.baseURL, "heartbeat", cfg.station), mode: "heartbeat"},
			{name: "车牌识别", target: endpoint(cfg.baseURL, "plate", cfg.station), mode: "plate"},
			{name: "门禁打开", target: endpoint(cfg.baseURL, "gio", cfg.station), mode: "io", ioValue: 1},
			{name: "门禁关闭", target: endpoint(cfg.baseURL, "gio", cfg.station), mode: "io", ioValue: 0},
		}
		for i, step := range steps {
			fmt.Printf("\n==== 场景步骤 %d/%d: %s ====\n", i+1, len(steps), step.name)
			if err := pushByMode(step.target, step.mode, cfg.station, cfg.license, cfg.ioSource, step.ioValue); err != nil {
				return err
			}
			if i < len(steps)-1 {
				time.Sleep(cfg.pauseDuration)
			}
		}
		return nil
	default:
		return fmt.Errorf("不支持的scenario: %s", cfg.scenario)
	}
}

func endpoint(baseURL, kind, station string) string {
	switch kind {
	case "heartbeat":
		return fmt.Sprintf("%s/api/device/heartbeat/%s", baseURL, station)
	case "gio":
		return fmt.Sprintf("%s/api/device/gio/%s", baseURL, station)
	case "plate":
		return fmt.Sprintf("%s/api/plate/station/%s", baseURL, station)
	default:
		return baseURL
	}
}

func pushByMode(target, mode, station, license string, ioSource, ioValue int) error {
	switch mode {
	case "plate":
		return pushPlate(target, license)
	case "heartbeat":
		return pushHeartbeat(target, station)
	case "io":
		return pushIO(target, ioSource, ioValue)
	default:
		return fmt.Errorf("不支持的mode类型: %s, 支持: plate/heartbeat/io", mode)
	}
}

// 推送车牌识别结果
func pushPlate(target, license string) error {
	msg := map[string]interface{}{
		"AlarmInfoPlate": map[string]interface{}{
			"channel":    0,
			"deviceName": "SIM-PLATE-DEVICE",
			"ipaddr":     "192.168.1.100",
			"serialno":   "sim-serial-001",
			"rule_id":    1,
			"user_data":  "",
			"result": map[string]interface{}{
				"PlateResult": map[string]interface{}{
					"license":       license,
					"confidence":    95,
					"colorType":     1,
					"type":          1,
					"direction":     4,
					"plateid":       12345,
					"isoffline":     0,
					"is_fake_plate": 0,
					"timeStamp": map[string]interface{}{
						"Timeval": map[string]interface{}{
							"decyear": 2025,
							"decmon":  7,
							"decday":  3,
							"dechour": 12,
							"decmin":  0,
							"decsec":  0,
							"sec":     time.Now().Unix(),
							"usec":    0,
						},
					},
					"location": map[string]interface{}{
						"RECT": map[string]interface{}{
							"left":   100,
							"top":    200,
							"right":  300,
							"bottom": 400,
						},
					},
				},
			},
		},
	}
	return postJSON(target, msg)
}

// 推送心跳
func pushHeartbeat(target, station string) error {
	msg := map[string]interface{}{
		"device_name": "SIM-PLATE-DEVICE",
		"ipaddr":      "192.168.1.100",
		"port":        80,
		"user_name":   "admin",
		"pass_wd":     "admin",
		"serialno":    "sim-serial-001",
		"channel_num": 1,
	}
	return postJSON(target, msg)
}

// 推送IO触发
func pushIO(target string, source, value int) error {
	msg := map[string]interface{}{
		"AlarmGioIn": map[string]interface{}{
			"deviceName": "SIM-PLATE-DEVICE",
			"ipaddr":     "192.168.1.100",
			"serialno":   "sim-serial-001",
			"result": map[string]interface{}{
				"TriggerResult": map[string]interface{}{
					"source": source,
					"value":  value,
				},
			},
		},
	}
	return postJSON(target, msg)
}

// 通用POST JSON
func postJSON(target string, msg interface{}) error {
	data, _ := json.MarshalIndent(msg, "", "  ")
	fmt.Printf("\n==== POST %s ====\n", target)
	fmt.Println("\n==== 发送请求 ====")
	fmt.Println(string(data))
	resp, err := http.Post(target, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("POST失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("\n==== 业务系统响应 (HTTP %d) ====\n%s\n", resp.StatusCode, string(body))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP状态异常: %d", resp.StatusCode)
	}
	return nil
}
