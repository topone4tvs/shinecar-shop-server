package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeviceManagerUpdateDeviceStatus(t *testing.T) {
	initTestLogger(t)

	dm := NewDeviceManager(testConfig())
	firstSeen := time.Now().Add(-time.Minute)

	dm.UpdateDeviceStatus("001", &DeviceStatus{
		DeviceType: DeviceTypePlate,
		StationID:  "001",
		Online:     true,
		LastSeen:   firstSeen,
		Status:     "online",
		Data: map[string]interface{}{
			"serialno": "plate-001",
		},
	})

	status, err := dm.GetDeviceStatus("001")
	if err != nil {
		t.Fatalf("GetDeviceStatus returned error: %v", err)
	}
	if status.DeviceType != DeviceTypePlate || !status.Online || status.Data["serialno"] != "plate-001" {
		t.Fatalf("unexpected initial status: %+v", status)
	}

	secondSeen := time.Now()
	dm.UpdateDeviceStatus("001", &DeviceStatus{
		DeviceType: DeviceTypePlate,
		StationID:  "001",
		Online:     false,
		LastSeen:   secondSeen,
		Status:     "offline",
		Data: map[string]interface{}{
			"serialno": "plate-001-updated",
		},
	})

	status, err = dm.GetDeviceStatus("001")
	if err != nil {
		t.Fatalf("GetDeviceStatus returned error after update: %v", err)
	}
	if status.Online {
		t.Fatalf("expected device to be offline after update")
	}
	if status.Status != "offline" {
		t.Fatalf("expected status offline, got %q", status.Status)
	}
	if status.Data["serialno"] != "plate-001-updated" {
		t.Fatalf("expected updated data, got %+v", status.Data)
	}
}

func TestDeviceManagerPendingPlateResponsesAreFIFO(t *testing.T) {
	initTestLogger(t)

	dm := NewDeviceManager(testConfig())
	first := map[string]interface{}{"command": "open"}
	second := map[string]interface{}{"command": "power_on"}

	dm.SetPendingPlateResponse("001", first)
	dm.SetPendingPlateResponse("001", second)

	if count := dm.GetPendingPlateResponseCount("001"); count != 2 {
		t.Fatalf("expected 2 pending responses, got %d", count)
	}

	got, ok := dm.GetPendingPlateResponse("001")
	if !ok {
		t.Fatalf("expected first pending response")
	}
	if got.(map[string]interface{})["command"] != "open" {
		t.Fatalf("expected first response to be open, got %+v", got)
	}

	got, ok = dm.GetPendingPlateResponse("001")
	if !ok {
		t.Fatalf("expected second pending response")
	}
	if got.(map[string]interface{})["command"] != "power_on" {
		t.Fatalf("expected second response to be power_on, got %+v", got)
	}

	if _, ok = dm.GetPendingPlateResponse("001"); ok {
		t.Fatalf("expected pending response queue to be empty")
	}
}

func TestDeviceManagerApplyHardwareGateReading(t *testing.T) {
	initTestLogger(t)

	dm := NewDeviceManager(testConfig())

	dm.ApplyHardwareGateReading("001", 0, 0, GateChannelGioHTTP)
	if dm.IsGateOpen("001") {
		t.Fatalf("expected gate to be closed when source and value are zero")
	}

	status, exists := dm.GetGateStatus("001")
	if !exists {
		t.Fatalf("expected gate status to exist")
	}
	if status.IsOpen || status.Source != 0 || status.Value != 0 || status.Channel != GateChannelGioHTTP {
		t.Fatalf("unexpected closed gate status: %+v", status)
	}

	dm.ApplyHardwareGateReading("001", 1, 1, GateChannelPlateAlarmGioIn)
	if !dm.IsGateOpen("001") {
		t.Fatalf("expected gate to be open when source/value are non-zero")
	}

	status, exists = dm.GetGateStatus("001")
	if !exists {
		t.Fatalf("expected gate status to exist after open reading")
	}
	if !status.IsOpen || status.Source != 1 || status.Value != 1 || status.Channel != GateChannelPlateAlarmGioIn {
		t.Fatalf("unexpected open gate status: %+v", status)
	}

	snapshot, changed := dm.ApplyHardwareGateReading("001", 1, 0, GateChannelGioHTTP)
	if !changed {
		t.Fatalf("expected source=1,value=0 to change gate status")
	}
	if snapshot.IsOpen || snapshot.Source != 1 || snapshot.Value != 0 || snapshot.Channel != GateChannelGioHTTP {
		t.Fatalf("expected source=1,value=0 to be closed, got %+v", snapshot)
	}
}

func TestDeviceManagerUnionStartSkipsAirConditionerAndFinishTurnsItOff(t *testing.T) {
	initTestLogger(t)

	var serviceCalls []string
	haServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/states/climate.station_001_air_conditioner":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"entity_id": "climate.station_001_air_conditioner",
				"state":     "off",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/services/climate/turn_on":
			serviceCalls = append(serviceCalls, "turn_on")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/services/climate/turn_off":
			serviceCalls = append(serviceCalls, "turn_off")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("unexpected HA request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer haServer.Close()

	cfg := testConfig()
	cfg.Devices.HA.BaseURL = haServer.URL
	dm := NewDeviceManager(cfg)
	ctx := context.Background()

	startResp, err := dm.ExecuteCommand(ctx, &UnionCommand{
		BaseDeviceCommand: BaseDeviceCommand{
			DeviceType: DeviceTypeUnion,
			Command:    CommandUnionStart,
			StationID:  "001",
		},
	})
	if err != nil {
		t.Fatalf("union start returned error: %v", err)
	}
	if !startResp.IsSuccess() {
		t.Fatalf("expected union start success, got error: %v", startResp.GetError())
	}

	finishResp, err := dm.ExecuteCommand(ctx, &UnionCommand{
		BaseDeviceCommand: BaseDeviceCommand{
			DeviceType: DeviceTypeUnion,
			Command:    CommandUnionFinish,
			StationID:  "001",
		},
	})
	if err != nil {
		t.Fatalf("union finish returned error: %v", err)
	}
	if !finishResp.IsSuccess() {
		t.Fatalf("expected union finish success, got error: %v", finishResp.GetError())
	}

	if len(serviceCalls) != 1 {
		t.Fatalf("expected 1 HA service call, got %d: %+v", len(serviceCalls), serviceCalls)
	}
	if serviceCalls[0] != "turn_off" {
		t.Fatalf("unexpected HA service calls: %+v", serviceCalls)
	}
}
