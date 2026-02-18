package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"no-spam/store"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// FCMSender defines the interface for sending messages to FCM.
// This allows mocking the firebase messaging client.
type FCMSender interface {
	Send(ctx context.Context, message *messaging.Message) (string, error)
}

// FCMConnector sends messages via Google's Firebase Cloud Messaging.
type FCMConnector struct {
	client FCMSender
}

// NewFCMConnector creates a new FCMConnector.
func NewFCMConnector(credentialsFile string) *FCMConnector {
	ctx := context.Background()
	var opts []option.ClientOption

	if credentialsFile != "" {
		data, err := os.ReadFile(credentialsFile)
		if err != nil {
			log.Printf("[FCM] Failed to read credentials file: %v", err)
			return nil
		}
		opts = append(opts, option.WithCredentialsJSON(data))
	} else {
		// Use default credentials (GOOGLE_APPLICATION_CREDENTIALS)
		log.Println("[FCM] Initializing with default credentials...")
	}

	config := &firebase.Config{}
	app, err := firebase.NewApp(ctx, config, opts...)
	if err != nil {
		log.Printf("[FCM] Failed to initialize Firebase app: %v", err)
		return nil
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		log.Printf("[FCM] Failed to get Messaging client: %v", err)
		return nil
	}

	log.Println("[FCM] Connector initialized successfully")
	return &FCMConnector{client: client}
}

// Send sends a message via FCM.
func (f *FCMConnector) Send(ctx context.Context, token string, payload []byte) error {
	if f.client == nil {
		return fmt.Errorf("FCM client is not initialized")
	}

	var notif store.Notification
	if err := json.Unmarshal(payload, &notif); err != nil {
		return fmt.Errorf("failed to unmarshal notification for FCM: %v", err)
	}

	// Map Notification fields to FCM Message
	// We use "Data" payload for custom handling in the client
	message := &messaging.Message{
		Token: token,
		Data: map[string]string{
			"topic":   notif.Topic,
			"payload": string(notif.Payload),
		},
	}

	response, err := f.client.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("FCM send failed: %v", err)
	}

	log.Printf("[FCM] Successfully sent message: %s", response)
	return nil
}
