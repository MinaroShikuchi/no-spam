package connectors

import "context"

// Connector defines the interface that all notification providers must implement.
// It allows the Hub to route messages without knowing the underlying implementation details.
type Connector interface {
	// Send sends a payload to a specific device identified by the token.
	Send(ctx context.Context, token string, payload []byte) error
}
