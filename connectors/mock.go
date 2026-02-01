package connectors

import (
	"context"
	"log"
)

// MockConnector is a connector that simply logs the message.
type MockConnector struct{}

// NewMockConnector creates a new MockConnector.
func NewMockConnector() *MockConnector {
	return &MockConnector{}
}

// Send logs the message payload.
func (m *MockConnector) Send(ctx context.Context, token string, payload []byte) error {
	log.Printf("[MockConnector] Sending to %s: %s", token, string(payload))
	return nil
}
