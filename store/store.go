package store

import (
	"encoding/json"
	"time"
)

type Subscriber struct {
	Topic    string `json:"topic"`
	Token    string `json:"token"`
	Provider string `json:"provider"`
	Username string `json:"-"` // Internal use, don't expose
}

type User struct {
	Username     string
	PasswordHash string
	Role         string
}

type Message struct {
	ID        int64
	Topic     string
	Payload   []byte // JSON raw
	CreatedAt time.Time
}

type Notification struct {
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload"`
}

type QueueItem struct {
	ID        int64  `json:"id"`
	MessageID int64  `json:"message_id"`
	Token     string `json:"token"`
	Provider  string `json:"provider"`
	Status    string `json:"status"`
	Payload   []byte `json:"payload"`
}

type Store interface {
	// Topics
	CreateTopic(name string) error
	DeleteTopic(name string) error
	TopicExists(name string) (bool, error)
	ListTopics() ([]string, error)

	// Subscriptions
	// username is now required
	AddSubscription(topic, token, provider, username string) error
	RemoveSubscription(topic, token string) error
	ClearTopicSubscribers(topic string) error
	GetSubscribers(topic string) ([]Subscriber, error)
	GetSubscriptionsByUser(username string) ([]Subscriber, error)
	GetSubscriptionsByToken(token string) ([]Subscriber, error)
	GetSubscriptionCount() (int, error) // For stats

	// Users
	CreateUser(username, passwordHash, role string) error
	GetUser(username string) (*User, error)
	HasAdminUser() (bool, error)
	UpdateUserRole(username, role string) error

	// Save Message
	SaveMessage(topic string, payload []byte) (int64, error)
	GetRecentMessages(topic string, limit int) ([]Message, error)
	ClearTopicMessages(topic string) error

	// Queue
	EnqueueMessage(messageID int64, token string) (int64, error)
	GetPendingMessages(token string) ([]QueueItem, error)
	GetAllPendingMessages() ([]QueueItem, error)
	MarkDelivered(queueID int64) error

	// Stats
	GetTotalMessagesSent() (int64, error)
}
