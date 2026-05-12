package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"shop_server/config"
	"shop_server/internal/service"
)

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 0},
		Shop: config.ShopConfig{
			ID:   "001",
			Name: "测试店铺",
			Stations: []config.StationConfig{
				{ID: "001", Name: "1号工位"},
			},
		},
		Log: config.LogConfig{Level: "info"},
	}
}

func newTestHTTPServer() (*HTTPServer, *service.DeviceManager) {
	cfg := testConfig()
	deviceManager := service.NewDeviceManager(cfg)
	manager := service.NewSimpleManager(cfg, deviceManager)
	return NewHTTPServer(cfg, manager), deviceManager
}

func TestHandleDeviceHeartbeatUpdatesStatus(t *testing.T) {
	initTestLogger(t)

	httpServer, deviceManager := newTestHTTPServer()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("device_name", "SIM-PLATE-DEVICE")
	_ = writer.WriteField("ipaddr", "192.168.1.100")
	_ = writer.WriteField("serialno", "sim-serial-001")
	_ = writer.WriteField("port", "80")
	_ = writer.WriteField("user_name", "admin")
	_ = writer.WriteField("pass_wd", "admin")
	_ = writer.WriteField("channel_num", "1")
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/device/heartbeat/001", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	httpServer.router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	status, err := deviceManager.GetDeviceStatus("001")
	if err != nil {
		t.Fatalf("expected device status to be updated: %v", err)
	}
	if status.DeviceType != service.DeviceTypePlate || !status.Online {
		t.Fatalf("unexpected device status: %+v", status)
	}
	if status.Data["serialno"] != "sim-serial-001" {
		t.Fatalf("expected serialno from heartbeat, got %+v", status.Data)
	}
}

func TestHandleDeviceHeartbeatReturnsPendingResponse(t *testing.T) {
	initTestLogger(t)

	httpServer, deviceManager := newTestHTTPServer()
	deviceManager.SetPendingPlateResponse("001", map[string]interface{}{
		"Response_AlarmInfoPlate": map[string]interface{}{
			"info": "ok",
		},
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/device/heartbeat/001", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	httpServer.router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if deviceManager.GetPendingPlateResponseCount("001") != 0 {
		t.Fatalf("expected pending response to be consumed")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if _, ok := payload["Response_AlarmInfoPlate"]; !ok {
		t.Fatalf("expected pending plate response body, got %+v", payload)
	}
}

func TestHandleGioMessageUpdatesGateStatus(t *testing.T) {
	initTestLogger(t)

	httpServer, deviceManager := newTestHTTPServer()
	body := []byte(`{
		"AlarmGioIn": {
			"result": {
				"TriggerResult": {
					"source": 1,
					"value": 1
				}
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/device/gio/001", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	httpServer.router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !deviceManager.IsGateOpen("001") {
		t.Fatalf("expected gate to be open after GIO source/value 1/1")
	}
}
