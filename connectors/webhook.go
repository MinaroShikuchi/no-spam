package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"no-spam/store"
	"time"
)

type WebhookConnector struct {
	client *http.Client
}

func NewWebhookConnector() *WebhookConnector {
	return &WebhookConnector{
		client: &http.Client{
			// Shorter timeout for webhooks
			Timeout: 5 * time.Second,
		},
	}
}

func (c *WebhookConnector) Send(ctx context.Context, token string, payload []byte) error {
	// For Webhook Connector, token is the Webhook URL
	webhookURL := token
	if webhookURL == "" {
		return fmt.Errorf("webhook url is missing")
	}

	// Unwrap the payload if it's wrapped in a store.Notification (from Hub)
	var notif store.Notification
	var body []byte
	if err := json.Unmarshal(payload, &notif); err == nil && len(notif.Payload) > 0 {
		// It's a wrapped notification, send the inner payload
		body = notif.Payload
	} else {
		// Fallback: send original payload (maybe not wrapped)
		body = payload
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Assume JSON payload
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send to webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook failed with status: %d", resp.StatusCode)
	}

	return nil
}
