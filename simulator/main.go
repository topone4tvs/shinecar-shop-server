package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	var (
		target   string
		mode     string
		station  string
		license  string
		ioSource int
		ioValue  int
	)

	flag.StringVar(&target, "target", "", "目标业务系统URL, 如 http://localhost:8080/api/plate/station/001")
	flag.StringVar(&mode, "mode", "plate", "推送类型: plate/heartbeat/io")
	flag.StringVar(&station, "station", "001", "工位ID/设备ID")
	flag.StringVar(&license, "license", "粤A12345", "车牌号(plate模式)")
	flag.IntVar(&ioSource, "source", 1, "IO触发源(io模式)")
	flag.IntVar(&ioValue, "value", 1, "IO触发值(io模式)")
	flag.Parse()

	if target == "" {
		fmt.Println("请指定 --target 业务系统URL")
		os.Exit(1)
	}

	switch mode {
	case "plate":
		pushPlate(target, license)
	case "heartbeat":
		pushHeartbeat(target, station)
	case "io":
		pushIO(target, ioSource, ioValue)
	default:
		fmt.Println("不支持的mode类型: plate/heartbeat/io")
		os.Exit(1)
	}
}

// 推送车牌识别结果
func pushPlate(target, license string) {
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
					"direction":     0,
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
	postJSON(target, msg)
}

// 推送心跳
func pushHeartbeat(target, station string) {
	msg := map[string]interface{}{
		"device_name": "SIM-PLATE-DEVICE",
		"ipaddr":      "192.168.1.100",
		"port":        80,
		"user_name":   "admin",
		"pass_wd":     "admin",
		"serialno":    "sim-serial-001",
		"channel_num": 1,
	}
	postJSON(target, msg)
}

// 推送IO触发
func pushIO(target string, source, value int) {
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
	postJSON(target, msg)
}

// 通用POST JSON
func postJSON(target string, msg interface{}) {
	data, _ := json.MarshalIndent(msg, "", "  ")
	fmt.Println("\n==== 发送请求 ====")
	fmt.Println(string(data))
	resp, err := http.Post(target, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Fatalf("POST失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("\n==== 业务系统响应 (HTTP %d) ====\n%s\n", resp.StatusCode, string(body))
}
