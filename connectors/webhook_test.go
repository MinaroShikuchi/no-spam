package connectors

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"no-spam/store"
	"testing"
	"time"
)

func TestNewWebhookConnector(t *testing.T) {
	wc := NewWebhookConnector()
	if wc == nil {
		t.Fatal("NewWebhookConnector returned nil")
	}
	if wc.client == nil {
		t.Error("Client was not initialized")
	}
	if wc.client.Timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", wc.client.Timeout)
	}
}

func TestWebhookSend_Success(t *testing.T) {
	// 1. Setup Mock Server
	var receivedBody []byte
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		receivedContentType = r.Header.Get("Content-Type")

		body, _ := io.ReadAll(r.Body)
		receivedBody = body

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 2. Connector
	wc := NewWebhookConnector()

	// 3. Send Payload (Raw)
	payload := []byte(`{"message":"hello"}`)
	ctx := context.Background()
	err := wc.Send(ctx, server.URL, payload)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// 4. Verify
	if receivedContentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", receivedContentType)
	}
	if string(receivedBody) != string(payload) {
		t.Errorf("Expected body %s, got %s", payload, receivedBody)
	}
}

func TestWebhookSend_WrappedNotification(t *testing.T) {
	// Verify unwrapping logic
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	wc := NewWebhookConnector()

	// Create wrapped payload
	innerPayload := json.RawMessage(`{"data":"inner"}`)
	notif := store.Notification{
		Topic:   "test-topic",
		Payload: innerPayload,
	}
	wrappedPayload, _ := json.Marshal(notif) // {"topic":"test-topic","payload":{"data":"inner"}}

	ctx := context.Background()
	err := wc.Send(ctx, server.URL, wrappedPayload)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should receive only inner payload
	expected := `{"data":"inner"}`
	if string(receivedBody) != expected {
		t.Errorf("Expected unwrapped body %s, got %s", expected, receivedBody)
	}
}

func TestWebhookSend_Errors(t *testing.T) {
	wc := NewWebhookConnector()
	ctx := context.Background()

	// 1. Missing URL
	if err := wc.Send(ctx, "", []byte{}); err == nil {
		t.Error("Expected error for missing URL")
	}

	// 2. Server Error (500)
	server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server500.Close()

	if err := wc.Send(ctx, server500.URL, []byte{}); err == nil {
		t.Error("Expected error for 500 response")
	} else if err.Error() != "webhook failed with status: 500" {
		t.Errorf("Unexpected error message: %v", err)
	}

	// 3. Network Error (Closed server)
	// Create a listener but close it immediately or use invalid port
	// Easier: Use invalid URL
	if err := wc.Send(ctx, "http://invalid-url-that-fails.local", []byte{}); err == nil {
		t.Error("Expected error for network failure")
	}
}
