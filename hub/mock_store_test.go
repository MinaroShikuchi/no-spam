package hub

import (
	"errors"
	"no-spam/store"
	"sync"
)

// MockStore is an in-memory implementation of store.Store for testing
type MockStore struct {
	mu             sync.Mutex
	Topics         map[string]bool
	Subscriptions  map[string][]store.Subscriber // Key: Topic
	Users          map[string]store.User
	Messages       map[int64]store.Message
	MessageSeq     int64
	Queue          []store.QueueItem
	QueueSeq       int64
	DeliveredItems map[int64]bool // Key: QueueID

	// Error simulation
	FailAll bool
}

func NewMockStore() *MockStore {
	return &MockStore{
		Topics:         make(map[string]bool),
		Subscriptions:  make(map[string][]store.Subscriber),
		Users:          make(map[string]store.User),
		Messages:       make(map[int64]store.Message),
		DeliveredItems: make(map[int64]bool),
	}
}

func (m *MockStore) CreateTopic(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return errors.New("mock error")
	}
	if m.Topics == nil {
		m.Topics = make(map[string]bool)
	}
	m.Topics[name] = true
	return nil
}

func (m *MockStore) DeleteTopic(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return errors.New("mock error")
	}
	delete(m.Topics, name)
	delete(m.Subscriptions, name)
	return nil
}

func (m *MockStore) TopicExists(name string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return false, errors.New("mock error")
	}
	return m.Topics[name], nil
}

func (m *MockStore) ListTopics() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return nil, errors.New("mock error")
	}
	var topics []string
	for t := range m.Topics {
		topics = append(topics, t)
	}
	return topics, nil
}

func (m *MockStore) AddSubscription(topic, token, provider, username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return errors.New("mock error")
	}

	sub := store.Subscriber{
		Topic:    topic,
		Token:    token,
		Provider: provider,
		Username: username,
	}

	m.Subscriptions[topic] = append(m.Subscriptions[topic], sub)
	return nil
}

func (m *MockStore) RemoveSubscription(topic, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return errors.New("mock error")
	}

	subs := m.Subscriptions[topic]
	var newSubs []store.Subscriber
	for _, s := range subs {
		if s.Token != token {
			newSubs = append(newSubs, s)
		}
	}
	m.Subscriptions[topic] = newSubs
	return nil
}

func (m *MockStore) GetSubscribers(topic string) ([]store.Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return nil, errors.New("mock error")
	}
	return m.Subscriptions[topic], nil
}

// Users
func (m *MockStore) CreateUser(username, passwordHash, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Users[username] = store.User{Username: username, PasswordHash: passwordHash, Role: role}
	return nil
}
func (m *MockStore) DeleteUser(username string) error             { return nil }
func (m *MockStore) ListUsers() ([]store.User, error)             { return nil, nil }
func (m *MockStore) GetUser(username string) (*store.User, error) { return nil, nil }
func (m *MockStore) HasAdminUser() (bool, error)                  { return false, nil }
func (m *MockStore) UpdateUserRole(username, role string) error   { return nil }

// Messages and Queue
func (m *MockStore) SaveMessage(topic string, payload []byte) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return 0, errors.New("mock error")
	}
	m.MessageSeq++
	id := m.MessageSeq
	m.Messages[id] = store.Message{
		ID:      id,
		Topic:   topic,
		Payload: payload,
	}
	return id, nil
}

func (m *MockStore) EnqueueMessage(messageID int64, token string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return 0, errors.New("mock error")
	}

	msg, ok := m.Messages[messageID]
	if !ok {
		return 0, errors.New("message not found")
	}

	m.QueueSeq++
	id := m.QueueSeq
	item := store.QueueItem{
		ID:        id,
		MessageID: messageID,
		Token:     token,
		Status:    "pending",
		Payload:   msg.Payload,
	}
	m.Queue = append(m.Queue, item)
	return id, nil
}

func (m *MockStore) GetAllPendingMessages() ([]store.QueueItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return nil, errors.New("mock error")
	}

	var pending []store.QueueItem
	for _, item := range m.Queue {
		if item.Status == "pending" {
			pending = append(pending, item)
		}
	}
	return pending, nil
}

func (m *MockStore) MarkDelivered(queueID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return errors.New("mock error")
	}

	for i, item := range m.Queue {
		if item.ID == queueID {
			m.Queue[i].Status = "delivered"
			m.DeliveredItems[queueID] = true
			return nil
		}
	}
	return errors.New("queue item not found")
}

// Previously failing stubs - now implemented
func (m *MockStore) GetRecentMessages(topic string, limit int) ([]store.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return nil, errors.New("mock error")
	}
	var msgs []store.Message
	for _, msg := range m.Messages {
		if msg.Topic == topic {
			msgs = append(msgs, msg)
		}
	}
	return msgs, nil
}

func (m *MockStore) ClearTopicMessages(topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return errors.New("mock error")
	}
	// Simplified: no-op for mock if we don't need verification
	return nil
}

func (m *MockStore) ClearTopicSubscribers(topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return errors.New("mock error")
	}
	delete(m.Subscriptions, topic)
	return nil
}

func (m *MockStore) GetPendingMessages(token string) ([]store.QueueItem, error) {
	return nil, nil
}

func (m *MockStore) GetPendingMessagesByTopic(topic string) ([]store.QueueItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailAll {
		return nil, errors.New("mock error")
	}
	return m.Queue, nil
}

func (m *MockStore) GetTotalMessagesSent() (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.DeliveredItems)), nil
}

func (m *MockStore) GetSubscriptionCount() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, subs := range m.Subscriptions {
		count += len(subs)
	}
	return count, nil
}

func (m *MockStore) GetSubscriptionsByToken(token string) ([]store.Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []store.Subscriber
	for _, subs := range m.Subscriptions {
		for _, s := range subs {
			if s.Token == token {
				result = append(result, s)
			}
		}
	}
	return result, nil
}

func (m *MockStore) GetSubscriptionsByUser(username string) ([]store.Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []store.Subscriber
	for _, subs := range m.Subscriptions {
		for _, s := range subs {
			if s.Username == username {
				result = append(result, s)
			}
		}
	}
	return result, nil
}
