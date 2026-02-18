package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"no-spam/hub"
	"no-spam/store"

	"github.com/gin-gonic/gin"
)

// setupTestStoreForAdmin creates test store
func setupTestStoreForAdmin(t *testing.T) store.Store {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	return s
}

// setupTestHubForAdmin creates test hub
func setupTestHubForAdmin(t *testing.T) (*hub.Hub, store.Store) {
	s := setupTestStoreForAdmin(t)
	h := hub.NewHub(s)
	return h, s
}

// TestCreateTopicHandler tests topic creation
func TestCreateTopicHandler(t *testing.T) {
	h, _ := setupTestHubForAdmin(t)
	handler := CreateTopicHandler(h)

	tests := []struct {
		name           string
		body           map[string]string
		expectedStatus int
	}{
		{
			name:           "Create topic",
			body:           map[string]string{"name": "new-topic"},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "Missing name",
			body:           map[string]string{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Duplicate topic",
			body:           map[string]string{"name": "new-topic"},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupTestContext()

			bodyBytes, _ := json.Marshal(tt.body)
			c.Request = httptest.NewRequest("POST", "/admin/topics", bytes.NewBuffer(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			handler(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestDeleteTopicHandler tests topic deletion
func TestDeleteTopicHandler(t *testing.T) {
	h, s := setupTestHubForAdmin(t)
	handler := DeleteTopicHandler(h)

	// Create topic with message
	// Create topic with message
	_ = s.CreateTopic("topic-with-message")
	_, _ = s.SaveMessage("topic-with-message", []byte(`{"msg": "test"}`))

	// Create empty topic
	// Create empty topic
	_ = s.CreateTopic("empty-topic")

	tests := []struct {
		name           string
		topicName      string
		expectedStatus int
	}{
		{
			name:           "Delete topic with messages (should fail)",
			topicName:      "topic-with-message",
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "Delete empty topic",
			topicName:      "empty-topic",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupTestContext()

			c.Params = gin.Params{{Key: "name", Value: tt.topicName}}
			c.Request = httptest.NewRequest("DELETE", "/admin/topics/"+tt.topicName, nil)

			handler(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestListTopicsHandler tests listing topics
func TestListTopicsHandler(t *testing.T) {
	h, s := setupTestHubForAdmin(t)
	handler := ListTopicsHandler(h)

	// Create topics
	// Create topics
	_ = s.CreateTopic("topic1")
	_ = s.CreateTopic("topic2")

	c, w := setupTestContext()
	c.Request = httptest.NewRequest("GET", "/admin/topics", nil)

	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var topics []string
	json.Unmarshal(w.Body.Bytes(), &topics)

	if len(topics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(topics))
	}
}

// TestGetMessagesHandler tests retrieving messages
func TestGetMessagesHandler(t *testing.T) {
	h, s := setupTestHubForAdmin(t)
	handler := GetMessagesHandler(h)

	// Create topic and add messages
	// Create topic and add messages
	_ = s.CreateTopic("test-topic")
	_, _ = s.SaveMessage("test-topic", []byte(`{"msg": "1"}`))
	_, _ = s.SaveMessage("test-topic", []byte(`{"msg": "2"}`))

	c, w := setupTestContext()
	c.Params = gin.Params{{Key: "name", Value: "test-topic"}}
	c.Request = httptest.NewRequest("GET", "/admin/topics/test-topic/messages", nil)

	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var messages []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &messages); err != nil {
		t.Fatalf("Failed to unmarshal messages: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
}

// TestClearMessagesHandler tests clearing messages
func TestClearMessagesHandler(t *testing.T) {
	h, s := setupTestHubForAdmin(t)
	handler := ClearMessagesHandler(h)

	// Create topic and add messages
	// Create topic and add messages
	_ = s.CreateTopic("test-topic")
	_, _ = s.SaveMessage("test-topic", []byte(`{"msg": "1"}`))

	c, w := setupTestContext()
	c.Params = gin.Params{{Key: "name", Value: "test-topic"}}
	c.Request = httptest.NewRequest("DELETE", "/admin/topics/test-topic/messages", nil)

	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify messages are cleared
	messages, _ := s.GetRecentMessages("test-topic", 10)
	if len(messages) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(messages))
	}
}

// TestClearSubscribersHandler tests clearing subscribers
func TestClearSubscribersHandler(t *testing.T) {
	h, s := setupTestHubForAdmin(t)
	handler := ClearSubscribersHandler(h)

	// Create topic and add subscribers
	// Create topic and add subscribers
	_ = s.CreateTopic("test-topic")
	_ = s.CreateUser("user1", "hash", "subscriber")
	_ = s.AddSubscription("test-topic", "token1", "mock", "user1")

	c, w := setupTestContext()
	c.Params = gin.Params{{Key: "name", Value: "test-topic"}}
	c.Request = httptest.NewRequest("DELETE", "/admin/topics/test-topic/subscribers", nil)

	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify subscribers are cleared
	subs, _ := s.GetSubscribers("test-topic")
	if len(subs) != 0 {
		t.Errorf("Expected 0 subscribers after clear, got %d", len(subs))
	}
}

// TestGetSubscribersHandler tests getting topic subscribers
func TestGetSubscribersHandler(t *testing.T) {
	h, s := setupTestHubForAdmin(t)
	handler := GetSubscribersHandler(h)

	// Create topic and subscribers
	// Create topic and subscribers
	_ = s.CreateTopic("test-topic")
	_ = s.CreateUser("user1", "hash", "subscriber")
	_ = s.AddSubscription("test-topic", "token1", "mock", "user1")

	c, w := setupTestContext()
	c.Params = gin.Params{{Key: "name", Value: "test-topic"}}
	c.Request = httptest.NewRequest("GET", "/admin/topics/test-topic/subscribers", nil)

	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var subscribers []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &subscribers); err != nil {
		t.Fatalf("Failed to unmarshal subscribers: %v", err)
	}

	if len(subscribers) != 1 {
		t.Errorf("Expected 1 subscriber, got %d", len(subscribers))
	}
}
