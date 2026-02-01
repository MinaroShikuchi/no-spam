package store

import "time"

type Subscriber struct {
	Topic    string
	Token    string
	Provider string
}

type Message struct {
	ID        int64
	Topic     string
	Payload   []byte // JSON raw
	CreatedAt time.Time
}

type QueueItem struct {
	ID        int64
	MessageID int64
	Token     string
	Status    string // pending, delivered
	Payload   []byte // Joined from messages table for convenience
}

type Store interface {
	// Subscriptions
	AddSubscription(topic, token, provider string) error
	RemoveSubscription(topic, token string) error
	GetSubscribers(topic string) ([]Subscriber, error)
	GetSubscriptionCount() (int, error) // For stats

	// Save Message
	SaveMessage(topic string, payload []byte) (int64, error)

	// Queue
	EnqueueMessage(messageID int64, token string) (int64, error)
	GetPendingMessages(token string) ([]QueueItem, error)
	MarkDelivered(queueID int64) error

	// Stats
	GetTotalMessagesSent() (int64, error)
}
