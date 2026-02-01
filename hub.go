package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	"no-spam/connectors"
	"no-spam/store"
)

// Message represents a notification to be sent.
type Message struct {
	Token    string          `json:"token,omitempty"`
	Provider string          `json:"provider,omitempty"` // fcm, apns, websocket
	Topic    string          `json:"topic,omitempty"`    // If set, broadcasts to subscribers
	Payload  json.RawMessage `json:"payload"`
}

// Hub manages the routing of messages to the appropriate connectors.
type Hub struct {
	mu         sync.RWMutex
	connectors map[string]connectors.Connector
	store      store.Store
}

// NewHub initializes a new Hub.
func NewHub(s store.Store) *Hub {
	return &Hub{
		connectors: map[string]connectors.Connector{},
		store:      s,
	}
}

// RegisterConnector adds a connector to the hub.
func (h *Hub) RegisterConnector(name string, c connectors.Connector) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connectors[name] = c
}

// Route directs the message to the requested provider's connector.
func (h *Hub) Route(ctx context.Context, msg Message) error {
	// Case 1: Broadcast to Topic
	if msg.Topic != "" {
		// 1. Save Message
		msgID, err := h.store.SaveMessage(msg.Topic, msg.Payload)
		if err != nil {
			return fmt.Errorf("failed to save message: %v", err)
		}

		// 2. Get Subscribers
		subscribers, err := h.store.GetSubscribers(msg.Topic)
		if err != nil {
			return fmt.Errorf("failed to get subscribers: %v", err)
		}

		if len(subscribers) == 0 {
			return nil
		}

		var wg sync.WaitGroup
		for _, sub := range subscribers {
			// 3. Enqueue for each subscriber
			queueID, err := h.store.EnqueueMessage(msgID, sub.Token)
			if err != nil {
				log.Printf("Failed to enqueue message for %s: %v", sub.Token, err)
				continue
			}

			// 4. Attempt Delivery
			connector, ok := h.GetConnector(sub.Provider)
			if !ok {
				continue
			}

			wg.Add(1)
			go func(c connectors.Connector, t string, p []byte, qID int64) {
				defer wg.Done()
				// Store-and-Forward: If sent, mark delivered.
				if err := c.Send(ctx, t, p); err == nil {
					h.store.MarkDelivered(qID)
				}
			}(connector, sub.Token, msg.Payload, queueID)
		}
		wg.Wait()
		return nil
	}

	// Case 2: Direct Message (Ephemeral, no DB?)
	// User asked for "queue logic", typically for topics.
	// But let's support direct message queueing too?
	// The prompt said "queue logic with sql".
	// Implementation Plan says: "Publish: Save message -> Find subscribers -> Insert queue".
	// This implies Topics.
	// Direct message support might be out of scope for the queue or just same logic.
	// I'll stick to Route for Topics having queue support as per plan.

	connector, ok := h.GetConnector(msg.Provider)
	if !ok {
		return fmt.Errorf("connector not found for provider: %s", msg.Provider)
	}

	if msg.Token == "" {
		return errors.New("target token is required for direct message")
	}

	return connector.Send(ctx, msg.Token, msg.Payload)
}

func (h *Hub) GetConnector(name string) (connectors.Connector, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.connectors[name]
	return c, ok
}

// Subscribe adds a subscriber to a topic.
func (h *Hub) Subscribe(topic string, sub store.Subscriber) error {
	return h.store.AddSubscription(topic, sub.Token, sub.Provider)
}

// Unsubscribe removes a subscriber from a topic.
func (h *Hub) Unsubscribe(topic string, token string) error {
	return h.store.RemoveSubscription(topic, token)
}

// Stats tracking proxies to store
func (h *Hub) GetTotalMessagesSent() int64 {
	count, _ := h.store.GetTotalMessagesSent()
	return count
}

func (h *Hub) GetSubscriptionCount() int {
	count, _ := h.store.GetSubscriptionCount()
	return count
}
