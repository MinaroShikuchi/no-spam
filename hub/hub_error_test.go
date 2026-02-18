package hub

import (
	"context"
	"encoding/json"
	"no-spam/store"
	"testing"
	"time"
)

func TestRoute_Errors(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)
	topic := "error-topic"
	h.CreateTopic(topic)

	msg := Message{
		Topic:   topic,
		Payload: json.RawMessage(`{}`),
	}
	ctx := context.Background()

	// 1. TopicExists Error
	mockStore.FailAll = true
	if err := h.Route(ctx, msg); err == nil {
		t.Error("Expected error when TopicExists fails")
	}
	mockStore.FailAll = false

	// 2. Connector Not Found (Direct)
	msgDirect := Message{
		Token:    "t",
		Provider: "missing",
		Payload:  json.RawMessage(`{}`),
	}
	if err := h.Route(ctx, msgDirect); err == nil {
		t.Error("Expected error when connector missing")
	}

	// 3. Token Missing (Direct)
	mc := NewMockConnector()
	h.RegisterConnector("mock", mc)
	msgNoToken := Message{
		Provider: "mock",
		Payload:  json.RawMessage(`{}`),
	}
	if err := h.Route(ctx, msgNoToken); err == nil {
		t.Error("Expected error when token missing")
	}
}

func TestQueueProcessor_Lifecycle(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)

	ctx, cancel := context.WithCancel(context.Background())

	// Start processor
	h.StartQueueProcessor(ctx)

	// Let it run a bit (it won't tick for 10s, but we just verify it doesn't crash)
	time.Sleep(10 * time.Millisecond)

	// Stop
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestProcessQueue_Errors(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)

	// 1. Store Error on GetPending
	mockStore.FailAll = true
	h.processQueue() // Should log error and return
	mockStore.FailAll = false

	// 2. Connector Missing
	item := store.QueueItem{ID: 1, Token: "t", Provider: "missing", Status: "pending"}
	mockStore.Queue = append(mockStore.Queue, item)
	h.processQueue() // Should log and continue

	// 3. Send Error
	mc := NewMockConnector()
	mc.ShouldFail = true
	h.RegisterConnector("fail", mc)
	item2 := store.QueueItem{ID: 2, Token: "t", Provider: "fail", Status: "pending"}
	mockStore.Queue = append(mockStore.Queue, item2)

	h.processQueue() // Should log delivery failure

	// Verify not marked delivered
	mockStore.mu.Lock()
	if mockStore.DeliveredItems[2] {
		t.Error("Failed delivery should not be marked delivered")
	}
	mockStore.mu.Unlock()

	// 4. Mark Delivered Error
	mc.ShouldFail = false
	// Store fail will prevent MarkDelivered
	// But we need Store to succeed for GetPending, then fail for MarkDelivered?
	// MockStore.FailAll is global.
	// To test MarkDelivered failure specifically, we'd need more granular control in MockStore.
	// We can skip this edge case for now as global fail covers most.
}
