package hub

import (
	"no-spam/store"
	"testing"
	"time"
)

func TestPassthroughMethods(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)

	// Setup data
	topic := "stats-topic"
	h.CreateTopic(topic)
	sub := store.Subscriber{Topic: topic, Token: "t1", Provider: "p1", Username: "u1"}
	h.Subscribe(topic, sub)

	// Msg
	h.store.SaveMessage(topic, []byte("test"))
	// Queue item
	h.store.EnqueueMessage(1, "t1")
	h.store.MarkDelivered(1) // count as sent

	// GetTotalMessagesSent
	if count := h.GetTotalMessagesSent(); count != 1 {
		t.Errorf("Expected 1 total message sent, got %d", count)
	}

	// GetSubscriptionCount
	if count := h.GetSubscriptionCount(); count != 1 {
		t.Errorf("Expected 1 subscription, got %d", count)
	}

	// GetQueue
	q, err := h.GetQueue(topic)
	if err != nil {
		t.Errorf("GetQueue failed: %v", err)
	}
	if len(q) != 1 {
		t.Errorf("Expected 1 queue item, got %d", len(q))
	}

	// GetSubscriptions (by Token)
	subs, err := h.GetSubscriptions("t1")
	if err != nil || len(subs) != 1 {
		t.Error("Failed to get subscriptions by token")
	}

	// GetSubscriptionsByUser
	subs, err = h.GetSubscriptionsByUser("u1")
	if err != nil || len(subs) != 1 {
		t.Error("Failed to get subscriptions by user")
	}

	// GetSubscribers
	subs, err = h.GetSubscribers(topic)
	if err != nil || len(subs) != 1 {
		t.Error("Failed to get subscribers")
	}

	// GetRecentMessages
	msgs, err := h.GetRecentMessages(topic, 10)
	if err != nil || len(msgs) != 1 {
		t.Error("Failed to get recent messages")
	}

	// Clear functions
	if err := h.ClearTopicMessages(topic); err != nil {
		t.Error("ClearTopicMessages failed")
	}
	if err := h.ClearTopicSubscribers(topic); err != nil {
		t.Error("ClearTopicSubscribers failed")
	}
	// Verify clear
	subs, _ = h.GetSubscribers(topic)
	if len(subs) != 0 {
		t.Error("Subscribers not cleared")
	}
}

func TestSubscribe_HistoryReplay(t *testing.T) {
	mockStore := NewMockStore()
	h := NewHub(mockStore)
	topic := "replay-topic"
	h.CreateTopic(topic)

	mc := NewMockConnector()
	h.RegisterConnector("mock", mc)

	// 1. Save old messages to store directly (simulating history)
	h.store.SaveMessage(topic, []byte("msg1")) // ID 1
	h.store.SaveMessage(topic, []byte("msg2")) // ID 2

	// 2. Subscribe new user
	sub := store.Subscriber{
		Topic:    topic,
		Token:    "replay-token",
		Provider: "mock",
	}

	err := h.Subscribe(topic, sub)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// 3. Verify Replay
	// Subscribe spawns goroutine for replay. Wait a bit.
	time.Sleep(50 * time.Millisecond)

	// Check that 2 messages were enqueued/sent
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if len(mc.SentMessages) != 2 {
		t.Errorf("Expected 2 replayed messages, got %d", len(mc.SentMessages))
	}
}
