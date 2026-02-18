package hub

import (
	"context"
	"errors"
	"sync"
)

// MockConnector implements connectors.Connector for testing
type MockConnector struct {
	mu           sync.Mutex
	SentMessages []SentMessage
	ShouldFail   bool
}

type SentMessage struct {
	Token   string
	Payload []byte
}

func NewMockConnector() *MockConnector {
	return &MockConnector{}
}

func (m *MockConnector) Send(ctx context.Context, token string, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ShouldFail {
		return errors.New("mock send error")
	}

	m.SentMessages = append(m.SentMessages, SentMessage{
		Token:   token,
		Payload: payload,
	})
	return nil
}
