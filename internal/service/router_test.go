package service

import (
	"encoding/json"
	"testing"

	"shop_server/pkg/mqtt"
)

type fakeMQTTClient struct {
	subscriptions map[string]mqtt.MessageHandler
	published     []publishedMessage
}

type publishedMessage struct {
	topic   string
	payload []byte
}

func newFakeMQTTClient() *fakeMQTTClient {
	return &fakeMQTTClient{
		subscriptions: make(map[string]mqtt.MessageHandler),
	}
}

func (f *fakeMQTTClient) SubscribePattern(pattern string, handler mqtt.MessageHandler) error {
	f.subscriptions[pattern] = handler
	return nil
}

func (f *fakeMQTTClient) Publish(topic string, payload []byte) error {
	copied := append([]byte(nil), payload...)
	f.published = append(f.published, publishedMessage{topic: topic, payload: copied})
	return nil
}

func TestRouterSubscribeTopics(t *testing.T) {
	initTestLogger(t)

	fakeClient := newFakeMQTTClient()
	router := NewRouter(testConfig(), fakeClient, NewDeviceManager(testConfig()))

	if err := router.subscribeTopics(); err != nil {
		t.Fatalf("subscribeTopics returned error: %v", err)
	}

	expectedShopPattern := "shinecar/shop/001/command/#"
	expectedStationPattern := "shinecar/shop/001/station/+/command/#"
	if fakeClient.subscriptions[expectedShopPattern] == nil {
		t.Fatalf("expected shop subscription %q", expectedShopPattern)
	}
	if fakeClient.subscriptions[expectedStationPattern] == nil {
		t.Fatalf("expected station subscription %q", expectedStationPattern)
	}
}

func TestRouterPublishToMQTTUsesConfiguredShop(t *testing.T) {
	initTestLogger(t)

	fakeClient := newFakeMQTTClient()
	router := NewRouter(testConfig(), fakeClient, NewDeviceManager(testConfig()))

	err := router.PublishToMQTT("001", "device_event", "gate_status", map[string]interface{}{
		"type":   "gate_status",
		"status": "open",
	})
	if err != nil {
		t.Fatalf("PublishToMQTT returned error: %v", err)
	}

	if len(fakeClient.published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(fakeClient.published))
	}

	got := fakeClient.published[0]
	if got.topic != "shinecar/shop/001/station/001/event" {
		t.Fatalf("unexpected topic: %s", got.topic)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(got.payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if payload["type"] != "device_event" || payload["sub_type"] != "gate_status" {
		t.Fatalf("unexpected payload type fields: %+v", payload)
	}
	if payload["shop_id"] != "001" || payload["station_id"] != "001" {
		t.Fatalf("unexpected payload station identity: %+v", payload)
	}
}

func TestRouterHandleMQTTMessageOpenGate(t *testing.T) {
	initTestLogger(t)

	fakeClient := newFakeMQTTClient()
	dm := NewDeviceManager(testConfig())
	router := NewRouter(testConfig(), fakeClient, dm)

	msg := MQTTMessage{
		Type:      MessageTypeDevice,
		Command:   CommandOpenGate,
		ShopID:    "001",
		StationID: "001",
		Data:      map[string]interface{}{},
		Timestamp: 1710000000,
	}
	payload, err := msg.ToJSON()
	if err != nil {
		t.Fatalf("failed to marshal mqtt message: %v", err)
	}

	err = router.handleMQTTMessage("shinecar/shop/001/station/001/command", payload)
	if err != nil {
		t.Fatalf("handleMQTTMessage returned error: %v", err)
	}

	if count := dm.GetPendingPlateResponseCount("001"); count != 1 {
		t.Fatalf("expected one pending plate response, got %d", count)
	}

	response, ok := dm.GetPendingPlateResponse("001")
	if !ok {
		t.Fatalf("expected pending plate response")
	}
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map response, got %T", response)
	}
	alarm, ok := responseMap["Response_AlarmInfoPlate"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected Response_AlarmInfoPlate in response: %+v", responseMap)
	}
	ioctrl, ok := alarm["ivs_ioctrl"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected ivs_ioctrl in response: %+v", alarm)
	}
	if ioctrl["io"] != 0 || ioctrl["value"] != 2 {
		t.Fatalf("unexpected open gate io control response: %+v", ioctrl)
	}
}

func TestRouterRejectsMismatchedShop(t *testing.T) {
	initTestLogger(t)

	router := NewRouter(testConfig(), newFakeMQTTClient(), NewDeviceManager(testConfig()))
	msg := MQTTMessage{
		Type:      MessageTypeDevice,
		Command:   CommandOpenGate,
		ShopID:    "002",
		StationID: "001",
		Data:      map[string]interface{}{},
		Timestamp: 1710000000,
	}
	payload, err := msg.ToJSON()
	if err != nil {
		t.Fatalf("failed to marshal mqtt message: %v", err)
	}

	err = router.handleMQTTMessage("shinecar/shop/001/station/001/command", payload)
	if err == nil {
		t.Fatalf("expected mismatched shop to be rejected")
	}
}
