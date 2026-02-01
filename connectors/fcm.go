package connectors

import (
	"context"
	"fmt"
)

// FCMConnector is a skeleton for Google's Firebase Cloud Messaging.
type FCMConnector struct {
	// APIKey or Credentials would go here
}

// NewFCMConnector creates a new FCMConnector.
func NewFCMConnector() *FCMConnector {
	return &FCMConnector{}
}

// Send sends a message via FCM.
func (f *FCMConnector) Send(ctx context.Context, token string, payload []byte) error {
	// TODO: Implement actual FCM sending logic here (e.g. HTTP call to FCM API)
	fmt.Printf("[FCMConnector] (Skeleton) Sending to %s: %s\n", token, string(payload))
	return nil
}
