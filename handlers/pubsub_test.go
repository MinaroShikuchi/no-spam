package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"no-spam/hub"
	"no-spam/store"
)

// setupTestHubAndStore creates test hub and store
func setupTestHubAndStore(t *testing.T) (*hub.Hub, store.Store) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	h := hub.NewHub(s)
	return h, s
}

// TestSubscribeHandler tests subscription functionality
func TestSubscribeHandler(t *testing.T) {
	h, s := setupTestHubAndStore(t)
	handler := SubscribeHandler(h)

	// Create topic and user
	s.CreateTopic("test-topic")
	s.CreateUser("testuser", "hash", "subscriber")

	tests := []struct {
		name           string
		body           map[string]interface{}
		username       string
		expectedStatus int
	}{
		{
			name: "Valid subscription",
			body: map[string]interface{}{
				"topic":    "test-topic",
				"token":    "device-token-123",
				"provider": "mock",
			},
			username:       "testuser",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Duplicate subscription (idempotent)",
			body: map[string]interface{}{
				"topic":    "test-topic",
				"token":    "device-token-123",
				"provider": "mock",
			},
			username:       "testuser",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Non-existent topic",
			body: map[string]interface{}{
				"topic":    "not-found",
				"token":    "device-token-456",
				"provider": "mock",
			},
			username:       "testuser",
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "Missing token",
			body: map[string]interface{}{
				"topic":    "test-topic",
				"provider": "mock",
			},
			username:       "testuser",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupTestContext()

			c.Set("username", tt.username)

			bodyBytes, _ := json.Marshal(tt.body)
			c.Request = httptest.NewRequest("POST", "/subscribe", bytes.NewBuffer(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			handler(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestUnsubscribeHandler tests unsubscription
func TestUnsubscribeHandler(t *testing.T) {
	h, s := setupTestHubAndStore(t)
	handler := UnsubscribeHandler(h)

	// Setup
	s.CreateTopic("test-topic")
	s.CreateUser("testuser", "hash", "subscriber")
	s.AddSubscription("test-topic", "device-token-123", "mock", "testuser")

	tests := []struct {
		name           string
		body           map[string]string
		username       string
		expectedStatus int
	}{
		{
			name: "Valid unsubscription",
			body: map[string]string{
				"topic": "test-topic",
				"token": "device-token-123",
			},
			username:       "testuser",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Missing token",
			body: map[string]string{
				"topic": "test-topic",
			},
			username:       "testuser",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupTestContext()

			c.Set("username", tt.username)

			bodyBytes, _ := json.Marshal(tt.body)
			c.Request = httptest.NewRequest("POST", "/unsubscribe", bytes.NewBuffer(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			handler(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestSendHandler tests message publishing
func TestSendHandler(t *testing.T) {
	h, s := setupTestHubAndStore(t)
	handler := SendHandler(h)

	// Create topic
	s.CreateTopic("test-topic")
	s.CreateUser("publisher", "hash", "publisher")

	tests := []struct {
		name           string
		body           map[string]interface{}
		username       string
		expectedStatus int
	}{
		{
			name: "Valid message",
			body: map[string]interface{}{
				"topic":   "test-topic",
				"message": map[string]string{"text": "Hello World"},
			},
			username:       "publisher",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Non-existent topic",
			body: map[string]interface{}{
				"topic":   "not-found",
				"message": map[string]string{"text": "Hello"},
			},
			username:       "publisher",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupTestContext()

			c.Set("username", tt.username)
			c.Set("role", "publisher")

			bodyBytes, _ := json.Marshal(tt.body)
			c.Request = httptest.NewRequest("POST", "/send", bytes.NewBuffer(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			handler(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestTopicsHandler tests getting user subscriptions
func TestTopicsHandler(t *testing.T) {
	h, s := setupTestHubAndStore(t)
	handler := TopicsHandler(h)

	// Setup
	s.CreateTopic("topic1")
	s.CreateTopic("topic2")
	s.CreateUser("testuser", "hash", "subscriber")
	s.AddSubscription("topic1", "token1", "mock", "testuser")
	s.AddSubscription("topic2", "token2", "mock", "testuser")

	c, w := setupTestContext()
	c.Set("username", "testuser")
	c.Request = httptest.NewRequest("GET", "/topics", nil)

	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

// TopicsHandler returns array of subscription objects directly
var subs []map[string]interface{}
if err := json.Unmarshal(w.Body.Bytes(), &subs); err != nil {
t.Fatalf("Failed to unmarshal: %v", err)
}
if len(subs) != 2 {
t.Errorf("Expected 2 subscriptions,got %d", len(subs))
}
}








// TestStatsHandler tests statistics endpoint
func TestStatsHandler(t *testing.T) {
	h, s := setupTestHubAndStore(t)
	handler := StatsHandler(h)

	// Create some data
	s.CreateTopic("topic1")
	s.CreateUser("user1", "hash", "subscriber")
	s.AddSubscription("topic1", "token1", "mock", "user1")
	s.SaveMessage("topic1", []byte(`{"msg": "test"}`))

	c, w := setupTestContext()
	c.Request = httptest.NewRequest("GET", "/stats", nil)

	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if _, ok := response["total_messages_sent"]; !ok {
		t.Error("Expected total_messages_sent in response")
	}
	if _, ok := response["active_subscriptions"]; !ok {
		t.Error("Expected active_subscriptions in response")
	}
}
