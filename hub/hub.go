package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"no-spam/connectors"
	"no-spam/store"
)

var ErrTopicNotFound = errors.New("topic not found")

// Message represents a notification to be sent.

// Message represents a notification to be sent.
type Message struct {
	Token    string          `json:"token,omitempty"`
	Provider string          `json:"provider,omitempty"` // fcm, apns
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

// StartQueueProcessor starts a background goroutine that processes pending queue items every 10 seconds
func (h *Hub) StartQueueProcessor(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("[Queue] Processor stopped")
				return
			case <-ticker.C:
				h.processQueue()
			}
		}
	}()
	log.Println("[Queue] Processor started (10s interval)")
}

// processQueue processes all pending messages in the queue
func (h *Hub) processQueue() {
	// Get all pending queue items
	pending, err := h.store.GetAllPendingMessages()
	if err != nil {
		log.Printf("[Queue] Failed to get pending messages: %v", err)
		return
	}

	if len(pending) == 0 {
		return
	}

	log.Printf("[Queue] Processing %d pending messages", len(pending))

	for _, item := range pending {
		// Get the connector for this provider
		h.mu.RLock()
		conn, exists := h.connectors[item.Provider]
		h.mu.RUnlock()

		if !exists {
			log.Printf("[Queue] No connector for provider: %s", item.Provider)
			continue
		}

		// Attempt to send
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := conn.Send(ctx, item.Token, item.Payload)
		cancel()

		if err != nil {
			log.Printf("[Queue] Failed to deliver message %d to %s: %v", item.ID, item.Token, err)
			// Could implement retry logic here
		} else {
			// Mark as delivered
			if err := h.store.MarkDelivered(item.ID); err != nil {
				log.Printf("[Queue] Failed to mark message %d as delivered: %v", item.ID, err)
			} else {
				log.Printf("[Queue] Successfully delivered message %d to %s via %s", item.ID, item.Token, item.Provider)
			}
		}
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
		exists, err := h.store.TopicExists(msg.Topic)
		if err != nil {
			return fmt.Errorf("failed to check topic existence: %v", err)
		}
		if !exists {
			return ErrTopicNotFound
		}

		// Wrap Payload with Topic
		envelope := store.Notification{
			Topic:   msg.Topic,
			Payload: msg.Payload,
		}
		wrappedPayload, err := json.Marshal(envelope)
		if err != nil {
			return fmt.Errorf("failed to marshal notification envelope: %v", err)
		}
		msg.Payload = wrappedPayload

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
			log.Printf("No subscribers found for topic: %s", msg.Topic)
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
			h.attemptDelivery(ctx, sub, msg.Payload, queueID)
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

func (h *Hub) attemptDelivery(ctx context.Context, sub store.Subscriber, payload []byte, queueID int64) {
	connector, ok := h.GetConnector(sub.Provider)
	if !ok {
		return
	}

	go func(c connectors.Connector, t string, p []byte, qID int64) {
		// Store-and-Forward: If sent, mark delivered.
		if err := c.Send(ctx, t, p); err == nil {
			h.store.MarkDelivered(qID)
		}
	}(connector, sub.Token, payload, queueID)
}

func (h *Hub) GetConnector(name string) (connectors.Connector, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.connectors[name]
	return c, ok
}

// Subscribe adds a subscriber to a topic.
func (h *Hub) Subscribe(topic string, sub store.Subscriber) error {
	exists, err := h.store.TopicExists(topic)
	if err != nil {
		return err
	}
	if !exists {
		return ErrTopicNotFound
	}

	if err := h.store.AddSubscription(topic, sub.Token, sub.Provider, sub.Username); err != nil {
		return err
	}

	// History Replay: Get last 20 messages
	msgs, err := h.store.GetRecentMessages(topic, 20)
	if err != nil {
		log.Printf("Failed to get recent messages for replay: %v", err)
		return nil // Don't fail subscription if replay fails
	}

	if len(msgs) > 0 {
		log.Printf("[Hub] Replaying %d recent messages to new subscriber %s", len(msgs), sub.Token)
		go func() {
			ctx := context.Background()
			for _, m := range msgs {
				// Enqueue
				qID, err := h.store.EnqueueMessage(m.ID, sub.Token)
				if err != nil {
					log.Printf("Failed to enqueue replay message %d: %v", m.ID, err)
					continue
				}
				// Attempt Delivery
				h.attemptDelivery(ctx, sub, m.Payload, qID)
			}
		}()
	}
	return nil
}

func (h *Hub) CreateTopic(name string) error {
	return h.store.CreateTopic(name)
}

func (h *Hub) ListTopics() ([]string, error) {
	return h.store.ListTopics()
}

func (h *Hub) DeleteTopic(name string) error {
	return h.store.DeleteTopic(name)
}

// Unsubscribe removes a subscriber from a topic.

// Unsubscribe removes a subscriber from a topic.
func (h *Hub) Unsubscribe(topic string, token string) error {
	return h.store.RemoveSubscription(topic, token)
}

// GetQueue retrieves pending messages for a specific topic.
func (h *Hub) GetQueue(topic string) ([]store.QueueItem, error) {
	exists, err := h.store.TopicExists(topic)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrTopicNotFound
	}
	return h.store.GetPendingMessagesByTopic(topic)
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

// GetSubscriptions retrieves all subscriptions for a given token.
func (h *Hub) GetSubscriptions(token string) ([]store.Subscriber, error) {
	return h.store.GetSubscriptionsByToken(token)
}

func (h *Hub) GetSubscriptionsByUser(username string) ([]store.Subscriber, error) {
	return h.store.GetSubscriptionsByUser(username)
}

func (h *Hub) GetRecentMessages(topic string, limit int) ([]store.Message, error) {
	return h.store.GetRecentMessages(topic, limit)
}

func (h *Hub) GetSubscribers(topic string) ([]store.Subscriber, error) {
	return h.store.GetSubscribers(topic)
}

func (h *Hub) ClearTopicMessages(topic string) error {
	return h.store.ClearTopicMessages(topic)
}

func (h *Hub) ClearTopicSubscribers(topic string) error {
	return h.store.ClearTopicSubscribers(topic)
}
