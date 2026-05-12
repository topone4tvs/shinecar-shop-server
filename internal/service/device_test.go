package service

import (
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
}
