package store

import (
	"testing"
)

// setupTestStore creates an in-memory SQLite database for testing
func setupTestStore(t *testing.T) *SQLiteStore {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	return store
}

// TestCreateTopic tests topic creation
func TestCreateTopic(t *testing.T) {
	store := setupTestStore(t)

	// Test creating a topic
	err := store.CreateTopic("test-topic")
	if err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}

	// Test duplicate topic creation
	err = store.CreateTopic("test-topic")
	if err == nil {
		t.Fatal("Expected error for duplicate topic, got nil")
	}
}

// TestTopicExists tests checking if a topic exists
func TestTopicExists(t *testing.T) {
	store := setupTestStore(t)

	// Topic should not exist initially
	exists, err := store.TopicExists("test-topic")
	if err != nil {
		t.Fatalf("TopicExists failed: %v", err)
	}
	if exists {
		t.Fatal("Topic should not exist")
	}

	// Create topic
	store.CreateTopic("test-topic")

	// Topic should exist now
	exists, err = store.TopicExists("test-topic")
	if err != nil {
		t.Fatalf("TopicExists failed: %v", err)
	}
	if !exists {
		t.Fatal("Topic should exist")
	}
}

// TestListTopics tests listing all topics
func TestListTopics(t *testing.T) {
	store := setupTestStore(t)

	// Initially empty
	topics, err := store.ListTopics()
	if err != nil {
		t.Fatalf("ListTopics failed: %v", err)
	}
	if len(topics) != 0 {
		t.Fatalf("Expected 0 topics, got %d", len(topics))
	}

	// Create topics
	store.CreateTopic("topic1")
	store.CreateTopic("topic2")
	store.CreateTopic("topic3")

	topics, err = store.ListTopics()
	if err != nil {
		t.Fatalf("ListTopics failed: %v", err)
	}
	if len(topics) != 3 {
		t.Fatalf("Expected 3 topics, got %d", len(topics))
	}
}

// TestDeleteTopic tests topic deletion
func TestDeleteTopic(t *testing.T) {
	store := setupTestStore(t)

	// Create topic
	store.CreateTopic("test-topic")

	// Delete topic
	err := store.DeleteTopic("test-topic")
	if err != nil {
		t.Fatalf("Failed to delete topic: %v", err)
	}

	// Verify it's gone
	exists, _ := store.TopicExists("test-topic")
	if exists {
		t.Fatal("Topic should not exist after deletion")
	}
}

// TestCreateUser tests user creation
func TestCreateUser(t *testing.T) {
	store := setupTestStore(t)

	// Create user
	err := store.CreateUser("testuser", "hashedpassword", "admin")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Test duplicate user
	err = store.CreateUser("testuser", "hashedpassword", "admin")
	if err == nil {
		t.Fatal("Expected error for duplicate user, got nil")
	}
}

// TestGetUser tests retrieving a user
func TestGetUser(t *testing.T) {
	store := setupTestStore(t)

	// User should not exist
	user, err := store.GetUser("testuser")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if user != nil {
		t.Fatal("User should not exist")
	}

	// Create user
	store.CreateUser("testuser", "hashedpassword", "publisher")

	// Get user
	user, err = store.GetUser("testuser")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if user == nil {
		t.Fatal("User should exist")
	}
	if user.Username != "testuser" {
		t.Fatalf("Expected username 'testuser', got '%s'", user.Username)
	}
	if user.Role != "publisher" {
		t.Fatalf("Expected role 'publisher', got '%s'", user.Role)
	}
}

// TestListUsers tests listing all users
func TestListUsers(t *testing.T) {
	store := setupTestStore(t)

	// Initially empty (or just admin if auto-created)
	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	initialCount := len(users)

	// Create users
	store.CreateUser("user1", "hash1", "admin")
	store.CreateUser("user2", "hash2", "publisher")
	store.CreateUser("user3", "hash3", "subscriber")

	users, err = store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != initialCount+3 {
		t.Fatalf("Expected %d users, got %d", initialCount+3, len(users))
	}
}

// TestDeleteUser tests user deletion
func TestDeleteUser(t *testing.T) {
	store := setupTestStore(t)

	// Create user
	store.CreateUser("testuser", "hash", "admin")

	// Delete user
	err := store.DeleteUser("testuser")
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	// Verify it's gone
	user, _ := store.GetUser("testuser")
	if user != nil {
		t.Fatal("User should not exist after deletion")
	}

	// Try to delete non-existent user
	err = store.DeleteUser("nonexistent")
	if err == nil {
		t.Fatal("Expected error for deleting non-existent user")
	}
}

// TestAddSubscription tests adding subscriptions
func TestAddSubscription(t *testing.T) {
	store := setupTestStore(t)

	// Create topic and user
	store.CreateTopic("test-topic")
	store.CreateUser("testuser", "hash", "subscriber")

	// Add subscription
	err := store.AddSubscription("test-topic", "device-token-123", "fcm", "testuser")
	if err != nil {
		t.Fatalf("Failed to add subscription: %v", err)
	}

	// Verify subscription exists
	subs, err := store.GetSubscribers("test-topic")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("Expected 1 subscriber, got %d", len(subs))
	}
	if subs[0].Token != "device-token-123" {
		t.Fatalf("Expected token 'device-token-123', got '%s'", subs[0].Token)
	}
}

// TestGetSubscribers tests retrieving subscribers for a topic
func TestGetSubscribers(t *testing.T) {
	store := setupTestStore(t)

	// Create topic
	store.CreateTopic("test-topic")
	store.CreateUser("user1", "hash", "subscriber")
	store.CreateUser("user2", "hash", "subscriber")

	// Add multiple subscriptions
	store.AddSubscription("test-topic", "token1", "fcm", "user1")
	store.AddSubscription("test-topic", "token2", "fcm", "user2")
	store.AddSubscription("test-topic", "token3", "mock", "user1")

	subs, err := store.GetSubscribers("test-topic")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}
	if len(subs) != 3 {
		t.Fatalf("Expected 3 subscribers, got %d", len(subs))
	}
}

// TestSaveMessage tests saving messages
func TestSaveMessage(t *testing.T) {
	store := setupTestStore(t)

	// Create topic
	store.CreateTopic("test-topic")

	// Save message
	payload := []byte(`{"message": "Hello World"}`)
	msgID, err := store.SaveMessage("test-topic", payload)
	if err != nil {
		t.Fatalf("Failed to save message: %v", err)
	}
	if msgID == 0 {
		t.Fatal("Expected non-zero message ID")
	}
}

// TestGetRecentMessages tests retrieving recent messages
func TestGetRecentMessages(t *testing.T) {
	store := setupTestStore(t)

	// Create topic
	store.CreateTopic("test-topic")

	// Save multiple messages
	store.SaveMessage("test-topic", []byte(`{"msg": "1"}`))
	store.SaveMessage("test-topic", []byte(`{"msg": "2"}`))
	store.SaveMessage("test-topic", []byte(`{"msg": "3"}`))

	// Get recent messages
	messages, err := store.GetRecentMessages("test-topic", 10)
	if err != nil {
		t.Fatalf("GetRecentMessages failed: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(messages))
	}

	// Messages should be in reverse chronological order (newest first)
	if string(messages[0].Payload) != `{"msg": "3"}` {
		t.Fatalf("Expected first message to be msg:3 (newest), got %s", messages[0].Payload)
	}
	if string(messages[2].Payload) != `{"msg": "1"}` {
		t.Fatalf("Expected last message to be msg:1 (oldest), got %s", messages[2].Payload)
	}
}

// TestClearTopicMessages tests clearing messages from a topic
func TestClearTopicMessages(t *testing.T) {
	store := setupTestStore(t)

	// Create topic and add messages
	store.CreateTopic("test-topic")
	store.SaveMessage("test-topic", []byte(`{"msg": "1"}`))
	store.SaveMessage("test-topic", []byte(`{"msg": "2"}`))

	// Clear messages
	err := store.ClearTopicMessages("test-topic")
	if err != nil {
		t.Fatalf("Failed to clear messages: %v", err)
	}

	// Verify messages are gone
	messages, _ := store.GetRecentMessages("test-topic", 10)
	if len(messages) != 0 {
		t.Fatalf("Expected 0 messages after clear, got %d", len(messages))
	}
}

// TestHasAdminUser tests checking for admin user
func TestHasAdminUser(t *testing.T) {
	store := setupTestStore(t)

	// Should not have admin user initially
	hasAdmin, err := store.HasAdminUser()
	if err != nil {
		t.Fatalf("HasAdminUser failed: %v", err)
	}
	if hasAdmin {
		t.Fatal("Should not have admin user initially")
	}

	// Create admin user
	store.CreateUser("admin", "hash", "admin")

	// Should have admin user now
	hasAdmin, err = store.HasAdminUser()
	if err != nil {
		t.Fatalf("HasAdminUser failed: %v", err)
	}
	if !hasAdmin {
		t.Fatal("Should have admin user")
	}
}

// TestGetSubscriptionCount tests counting subscriptions
func TestGetSubscriptionCount(t *testing.T) {
	store := setupTestStore(t)

	// Initially 0
	count, err := store.GetSubscriptionCount()
	if err != nil {
		t.Fatalf("GetSubscriptionCount failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("Expected 0 subscriptions, got %d", count)
	}

	// Add subscriptions
	store.CreateTopic("topic1")
	store.CreateUser("user1", "hash", "subscriber")
	store.AddSubscription("topic1", "token1", "fcm", "user1")
	store.AddSubscription("topic1", "token2", "fcm", "user1")

	count, err = store.GetSubscriptionCount()
	if err != nil {
		t.Fatalf("GetSubscriptionCount failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("Expected 2 subscriptions, got %d", count)
	}
}

// TestRemoveSubscription tests removing a subscription
func TestRemoveSubscription(t *testing.T) {
	store := setupTestStore(t)

	// Create topic and add subscription
	store.CreateTopic("test-topic")
	store.CreateUser("user1", "hash", "subscriber")
	store.AddSubscription("test-topic", "token1", "fcm", "user1")

	// Verify subscription exists
	subs, _ := store.GetSubscribers("test-topic")
	if len(subs) != 1 {
		t.Fatalf("Expected 1 subscriber, got %d", len(subs))
	}

	// Remove subscription
	err := store.RemoveSubscription("test-topic", "token1")
	if err != nil {
		t.Fatalf("Failed to remove subscription: %v", err)
	}

	// Verify it's gone
	subs, _ = store.GetSubscribers("test-topic")
	if len(subs) != 0 {
		t.Fatalf("Expected 0 subscribers, got %d", len(subs))
	}
}

// TestClearTopicSubscribers tests clearing all subscribers from a topic
func TestClearTopicSubscribers(t *testing.T) {
	store := setupTestStore(t)

	// Create topic and add multiple subscriptions
	store.CreateTopic("test-topic")
	store.CreateUser("user1", "hash", "subscriber")
	store.CreateUser("user2", "hash", "subscriber")
	store.AddSubscription("test-topic", "token1", "fcm", "user1")
	store.AddSubscription("test-topic", "token2", "fcm", "user2")

	// Clear all subscribers
	err := store.ClearTopicSubscribers("test-topic")
	if err != nil {
		t.Fatalf("Failed to clear subscribers: %v", err)
	}

	// Verify they're gone
	subs, _ := store.GetSubscribers("test-topic")
	if len(subs) != 0 {
		t.Fatalf("Expected 0 subscribers, got %d", len(subs))
	}
}

// TestGetSubscriptionsByUser tests getting all subscriptions for a user
func TestGetSubscriptionsByUser(t *testing.T) {
	store := setupTestStore(t)

	// Create topics and add subscriptions
	store.CreateTopic("topic1")
	store.CreateTopic("topic2")
	store.CreateUser("user1", "hash", "subscriber")
	store.AddSubscription("topic1", "token1", "fcm", "user1")
	store.AddSubscription("topic2", "token2", "fcm", "user1")

	// Get user's subscriptions
	subs, err := store.GetSubscriptionsByUser("user1")
	if err != nil {
		t.Fatalf("GetSubscriptionsByUser failed: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("Expected 2 subscriptions, got %d", len(subs))
	}
}

// TestGetSubscriptionsByToken tests getting subscriptions by device token
func TestGetSubscriptionsByToken(t *testing.T) {
	store := setupTestStore(t)

	// Create topics and add subscriptions with same token
	store.CreateTopic("topic1")
	store.CreateTopic("topic2")
	store.CreateUser("user1", "hash", "subscriber")
	store.AddSubscription("topic1", "shared-token", "fcm", "user1")
	store.AddSubscription("topic2", "shared-token", "fcm", "user1")

	// Get subscriptions by token
	subs, err := store.GetSubscriptionsByToken("shared-token")
	if err != nil {
		t.Fatalf("GetSubscriptionsByToken failed: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("Expected 2 subscriptions, got %d", len(subs))
	}
}

// TestEnqueueMessage tests message queueing
func TestEnqueueMessage(t *testing.T) {
	store := setupTestStore(t)

	// Create topic and save message first
	store.CreateTopic("test-topic")
	msgID, _ := store.SaveMessage("test-topic", []byte(`{"message": "test"}`))

	// Enqueue message for delivery
	queueID, err := store.EnqueueMessage(msgID, "device-token-1")
	if err != nil {
		t.Fatalf("Failed to enqueue message: %v", err)
	}
	if queueID == 0 {
		t.Fatal("Expected non-zero queue ID")
	}

	// Verify message was queued
	pending, _ := store.GetPendingMessages("device-token-1")
	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending message, got %d", len(pending))
	}
}

// TestGetPendingMessages tests retrieving pending messages
func TestGetPendingMessages(t *testing.T) {
	store := setupTestStore(t)

	// Create topic and save messages
	store.CreateTopic("test-topic")
	msgID1, _ := store.SaveMessage("test-topic", []byte(`{"msg": "1"}`))
	msgID2, _ := store.SaveMessage("test-topic", []byte(`{"msg": "2"}`))

	// Enqueue messages for same token
	store.EnqueueMessage(msgID1, "device-token-1")
	store.EnqueueMessage(msgID2, "device-token-1")

	// Get pending messages
	pending, err := store.GetPendingMessages("device-token-1")
	if err != nil {
		t.Fatalf("GetPendingMessages failed: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("Expected 2 pending messages, got %d", len(pending))
	}
}

// TestMarkDelivered tests marking messages as delivered
func TestMarkDelivered(t *testing.T) {
	store := setupTestStore(t)

	// Create topic, save message, and enqueue it
	store.CreateTopic("test-topic")
	msgID, _ := store.SaveMessage("test-topic", []byte(`{"msg": "test"}`))
	store.EnqueueMessage(msgID, "device-token-1")

	// Get pending messages (should be 1)
	pending, _ := store.GetPendingMessages("device-token-1")
	if len(pending) != 1 {
		t.Fatal("Expected 1 pending message")
	}

	// Mark as delivered
	err := store.MarkDelivered(pending[0].ID)
	if err != nil {
		t.Fatalf("Failed to mark delivered: %v", err)
	}

	// Get pending messages (should be 0)
	pending, _ = store.GetPendingMessages("device-token-1")
	if len(pending) != 0 {
		t.Fatalf("Expected 0 pending messages, got %d", len(pending))
	}
}

// TestGetAllPending Messages tests getting all pending messages
func TestGetAllPendingMessages(t *testing.T) {
	store := setupTestStore(t)

	// Create topics, users, and subscriptions
	store.CreateTopic("topic1")
	store.CreateTopic("topic2")
	store.CreateUser("user1", "hash", "subscriber")
	store.AddSubscription("topic1", "token1", "fcm", "user1")
	store.AddSubscription("topic2", "token2", "fcm", "user1")

	// Save and enqueue messages
	msgID1, _ := store.SaveMessage("topic1", []byte(`{"msg": "1"}`))
	msgID2, _ := store.SaveMessage("topic2", []byte(`{"msg": "2"}`))
	store.EnqueueMessage(msgID1, "token1")
	store.EnqueueMessage(msgID2, "token2")

	// Get all pending messages
	pending, err := store.GetAllPendingMessages()
	if err != nil {
		t.Fatalf("GetAllPendingMessages failed: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("Expected 2 pending messages, got %d", len(pending))
	}
}

// TestGetPendingMessagesByTopic tests getting pending messages for a topic
func TestGetPendingMessagesByTopic(t *testing.T) {
	store := setupTestStore(t)

	// Create topics, users, and subscriptions
	store.CreateTopic("topic1")
	store.CreateTopic("topic2")
	store.CreateUser("user1", "hash", "subscriber")
	store.AddSubscription("topic1", "token1", "fcm", "user1")
	store.AddSubscription("topic2", "token2", "fcm", "user1")

	// Save and enqueue messages
	msg1, _ := store.SaveMessage("topic1", []byte(`{"msg": "1"}`))
	msg2, _ := store.SaveMessage("topic1", []byte(`{"msg": "2"}`))
	msg3, _ := store.SaveMessage("topic2", []byte(`{"msg": "3"}`))
	store.EnqueueMessage(msg1, "token1")
	store.EnqueueMessage(msg2, "token1")
	store.EnqueueMessage(msg3, "token2")

	// Get pending messages for topic1
	pending, err := store.GetPendingMessagesByTopic("topic1")
	if err != nil {
		t.Fatalf("GetPendingMessagesByTopic failed: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("Expected 2 pending messages for topic1, got %d", len(pending))
	}
}

// TestGetTotalMessagesSent tests getting total messages sent count
func TestGetTotalMessagesSent(t *testing.T) {
	store := setupTestStore(t)

	// Initially should be 0
	count, err := store.GetTotalMessagesSent()
	if err != nil {
		t.Fatalf("GetTotalMessagesSent failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("Expected 0 messages, got %d", count)
	}

	// Save some messages
	store.CreateTopic("topic1")
	store.SaveMessage("topic1", []byte(`{"msg": "1"}`))
	store.SaveMessage("topic1", []byte(`{"msg": "2"}`))

	// Count should be 2
	count, err = store.GetTotalMessagesSent()
	if err != nil {
		t.Fatalf("GetTotalMessagesSent failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("Expected 2 messages, got %d", count)
	}
}
