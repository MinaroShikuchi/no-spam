package hub

import (
	"context"
	"encoding/json"
	"no-spam/store"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)
	if h == nil {
		t.Fatal("NewHub returned nil")
	}
}

func TestRegisterConnector(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)
	mc := NewMockConnector()

	h.RegisterConnector("test-provider", mc)

	c, ok := h.GetConnector("test-provider")
	if !ok {
		t.Error("Failed to retrieve registered connector")
	}
	if c != mc {
		t.Error("Retrieved connector does not match registered one")
	}
}

func TestTopics(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)
	topic := "unit-test-topic"

	// Exists False
	exists, _ := mockStore.TopicExists(topic)
	if exists {
		t.Error("Topic should not exist initially")
	}

	// Create
	if err := h.CreateTopic(topic); err != nil {
		t.Errorf("CreateTopic failed: %v", err)
	}

	// Exists True
	exists, _ = mockStore.TopicExists(topic)
	if !exists {
		t.Error("Topic should exist after creation")
	}

	// List
	list, err := h.ListTopics()
	if err != nil {
		t.Errorf("ListTopics failed: %v", err)
	}
	found := false
	for _, name := range list {
		if name == topic {
			found = true
			break
		}
	}
	if !found {
		t.Error("Created topic not found in list")
	}

	// Delete
	if err := h.DeleteTopic(topic); err != nil {
		t.Errorf("DeleteTopic failed: %v", err)
	}
	exists, _ = mockStore.TopicExists(topic)
	if exists {
		t.Error("Topic should not exist after deletion")
	}
}

func TestSubscriptions(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)
	topic := "sub-topic"
	h.CreateTopic(topic)

	sub := store.Subscriber{
		Topic:    topic,
		Token:    "token-123",
		Provider: "mock",
		Username: "user",
	}

	// Subscribe
	if err := h.Subscribe(topic, sub); err != nil {
		t.Errorf("Subscribe failed: %v", err)
	}

	// Verify in store
	subs, _ := mockStore.GetSubscribers(topic)
	if len(subs) != 1 {
		t.Errorf("Expected 1 subscriber, got %d", len(subs))
	}

	// Unsubscribe
	if err := h.Unsubscribe(topic, sub.Token); err != nil {
		t.Errorf("Unsubscribe failed: %v", err)
	}

	subs, _ = mockStore.GetSubscribers(topic)
	if len(subs) != 0 {
		t.Errorf("Expected 0 subscribers, got %d", len(subs))
	}
}

func TestRoute_Broadcast(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)
	mc := NewMockConnector()

	h.RegisterConnector("mock", mc)

	topic := "broadcast-topic"
	h.CreateTopic(topic)

	// Add subscriber
	sub := store.Subscriber{
		Topic:    topic,
		Token:    "sub-token-1",
		Provider: "mock",
	}
	h.Subscribe(topic, sub)

	// Route Message
	msg := Message{
		Topic:   topic,
		Payload: json.RawMessage(`{"data":"hello"}`),
	}

	ctx := context.Background()
	if err := h.Route(ctx, msg); err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Verify Message Saved
	if len(mockStore.Messages) != 1 {
		t.Error("Message was not saved to store")
	}

	// Verify Queued
	if len(mockStore.Queue) != 1 {
		t.Error("Message was not queued")
	}

	// Wait for async delivery attempt (called in Route loop via attemptDelivery goroutine)
	// Actually attemptDelivery spawns a goroutine.
	time.Sleep(50 * time.Millisecond)

	// Verify Delivery
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if len(mc.SentMessages) != 1 {
		t.Errorf("Expected 1 sent message, got %d", len(mc.SentMessages))
	} else {
		if mc.SentMessages[0].Token != sub.Token {
			t.Errorf("Expected token %s, got %s", sub.Token, mc.SentMessages[0].Token)
		}
	}

	// Verify Marked Delivered
	mockStore.mu.Lock()
	defer mockStore.mu.Unlock()
	if !mockStore.DeliveredItems[1] { // QueueID 1
		t.Error("Queue item was not marked as delivered")
	}
}

func TestProcessQueue(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)
	mc := NewMockConnector()
	h.RegisterConnector("mock", mc)

	// Manually inject a pending queue item
	item := store.QueueItem{
		ID:       100,
		Token:    "retry-token",
		Provider: "mock",
		Status:   "pending",
		Payload:  []byte("retry-payload"),
	}
	mockStore.Queue = append(mockStore.Queue, item)

	// Trigger processing
	h.processQueue()

	// Verify sent
	mc.mu.Lock()
	if len(mc.SentMessages) != 1 {
		t.Errorf("Expected 1 sent message from queue, got %d", len(mc.SentMessages))
	}
	mc.mu.Unlock()

	// Verify status updated in store
	mockStore.mu.Lock()
	if !mockStore.DeliveredItems[100] {
		t.Error("Queue item 100 was not marked delivered")
	}
	mockStore.mu.Unlock()
}

func TestRoute_Direct(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)
	mc := NewMockConnector()
	h.RegisterConnector("fcm", mc)

	msg := Message{
		Token:    "device-123",
		Provider: "fcm",
		Payload:  json.RawMessage(`"hello"`),
	}

	if err := h.Route(context.Background(), msg); err != nil {
		t.Fatalf("Route direct failed: %v", err)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()
	if len(mc.SentMessages) != 1 {
		t.Errorf("Expected 1 sent message, got %d", len(mc.SentMessages))
	}
}
