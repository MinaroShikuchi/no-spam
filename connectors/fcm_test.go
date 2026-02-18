package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"no-spam/store"
	"testing"

	"firebase.google.com/go/v4/messaging"
)

// MockFCMSender implements FCMSender interface
type MockFCMSender struct {
	SentMessages []*messaging.Message
	ShouldFail   bool
}

func (m *MockFCMSender) Send(ctx context.Context, message *messaging.Message) (string, error) {
	if m.ShouldFail {
		return "", errors.New("mock fcm error")
	}
	m.SentMessages = append(m.SentMessages, message)
	return "projects/test/messages/123", nil
}

func TestFCMSend_Success(t *testing.T) {
	mock := &MockFCMSender{}
	connector := &FCMConnector{client: mock}

	// Prepare payload
	notif := store.Notification{
		Topic:   "news",
		Payload: json.RawMessage(`{"alert":"breaking"}`),
	}
	payload, _ := json.Marshal(notif)

	// Send
	if err := connector.Send(context.Background(), "device-token-123", payload); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify
	if len(mock.SentMessages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(mock.SentMessages))
	}
	msg := mock.SentMessages[0]
	if msg.Token != "device-token-123" {
		t.Errorf("Expected token device-token-123, got %s", msg.Token)
	}
	if msg.Data["topic"] != "news" {
		t.Errorf("Expected topic news, got %s", msg.Data["topic"])
	}
	expectedPayload := `{"alert":"breaking"}`
	if msg.Data["payload"] != expectedPayload {
		t.Errorf("Expected payload %s, got %s", expectedPayload, msg.Data["payload"])
	}
}

func TestFCMSend_Errors(t *testing.T) {
	mock := &MockFCMSender{}
	connector := &FCMConnector{client: mock}
	ctx := context.Background()

	// 1. Uninitialized
	emptyConnector := &FCMConnector{client: nil}
	if err := emptyConnector.Send(ctx, "t", []byte("{}")); err == nil {
		t.Error("Expected error for uninitialized client")
	}

	// 2. Invalid Payload (Unmarshal fail)
	if err := connector.Send(ctx, "t", []byte("invalid-json")); err == nil {
		t.Error("Expected error for invalid payload")
	}

	// 3. Send Error (Mock fail)
	mock.ShouldFail = true
	notif := store.Notification{Topic: "t", Payload: []byte("{}")}
	payload, _ := json.Marshal(notif)

	if err := connector.Send(ctx, "t", payload); err == nil {
		t.Error("Expected error when underlying send fails")
	} else if err.Error() != "FCM send failed: mock fcm error" {
		t.Errorf("Unexpected error message: %v", err)
	}
}
