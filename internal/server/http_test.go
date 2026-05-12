package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"shop_server/config"
	"shop_server/internal/service"
)

type publishedEvent struct {
	stationID   string
	messageType string
	subType     string
	data        interface{}
}

type fakeEventPublisher struct {
	events []publishedEvent
}

func (p *fakeEventPublisher) PublishToMQTT(stationID, messageType, subType string, data interface{}) error {
	p.events = append(p.events, publishedEvent{
		stationID:   stationID,
		messageType: messageType,
		subType:     subType,
		data:        data,
	})
	return nil
}

type fakeManager struct {
	deviceManager *service.DeviceManager
	publisher     service.EventPublisher
}

func (m *fakeManager) Start(ctx context.Context) error { return nil }
func (m *fakeManager) Stop(ctx context.Context) error  { return nil }
func (m *fakeManager) GetRouter() *service.Router      { return nil }
func (m *fakeManager) GetEventPublisher() service.EventPublisher {
	return m.publisher
}
func (m *fakeManager) GetDeviceManager() *service.DeviceManager {
	return m.deviceManager
}

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

func newTestHTTPServerWithPublisher() (*HTTPServer, *service.DeviceManager, *fakeEventPublisher) {
	cfg := testConfig()
	deviceManager := service.NewDeviceManager(cfg)
	publisher := &fakeEventPublisher{}
	manager := &fakeManager{
		deviceManager: deviceManager,
		publisher:     publisher,
	}
	return NewHTTPServer(cfg, manager), deviceManager, publisher
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

func TestHandlePlateMessagePublishesPlateRecognitionEvent(t *testing.T) {
	initTestLogger(t)

	httpServer, _, publisher := newTestHTTPServerWithPublisher()
	body := []byte(`{
		"AlarmInfoPlate": {
			"channel": 0,
			"deviceName": "SIM-PLATE-DEVICE",
			"ipaddr": "192.168.1.100",
			"serialno": "sim-serial-001",
			"result": {
				"PlateResult": {
					"license": "浙A73J2W",
					"confidence": 95,
					"colorType": 1,
					"type": 1,
					"direction": 4,
					"plateid": 12345,
					"isoffline": 0,
					"is_fake_plate": 0,
					"timeStamp": {
						"Timeval": {
							"sec": 1710000000
						}
					},
					"location": {
						"RECT": {
							"left": 100,
							"top": 200,
							"right": 300,
							"bottom": 400
						}
					}
				}
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/plate/station/001", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	httpServer.router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected one published event, got %d", len(publisher.events))
	}

	event := publisher.events[0]
	if event.stationID != "001" || event.messageType != "device_event" || event.subType != "plate_recognition" {
		t.Fatalf("unexpected event envelope: %+v", event)
	}
	data, ok := event.data.(MqttEvent)
	if !ok {
		t.Fatalf("expected MqttEvent data, got %T", event.data)
	}
	if data["type"] != "plate_recognition" || data["station_id"] != "001" {
		t.Fatalf("unexpected event data: %+v", data)
	}
	plate, ok := data["plate"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected plate data, got %+v", data["plate"])
	}
	if plate["license"] != "浙A73J2W" {
		t.Fatalf("unexpected plate license: %+v", plate)
	}
}

func TestHandlePlateMessagePublishesIOTriggerEvent(t *testing.T) {
	initTestLogger(t)

	httpServer, deviceManager, publisher := newTestHTTPServerWithPublisher()
	body := []byte(`{
		"AlarmGioIn": {
			"deviceName": "SIM-PLATE-DEVICE",
			"ipaddr": "192.168.1.100",
			"serialno": "sim-serial-001",
			"result": {
				"source": 1,
				"value": 1
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/plate/station/001", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	httpServer.router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected one published event, got %d", len(publisher.events))
	}

	event := publisher.events[0]
	if event.stationID != "001" || event.messageType != "device_event" || event.subType != "io_trigger" {
		t.Fatalf("unexpected event envelope: %+v", event)
	}
	if !deviceManager.IsGateOpen("001") {
		t.Fatalf("expected IO trigger to update gate status")
	}

	data, ok := event.data.(MqttEvent)
	if !ok {
		t.Fatalf("expected MqttEvent data, got %T", event.data)
	}
	ioData, ok := data["io"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected io data, got %+v", data["io"])
	}
	if ioData["source"] != 1 || ioData["value"] != 1 {
		t.Fatalf("unexpected io data: %+v", ioData)
	}
}
