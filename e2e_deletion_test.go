package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestE2E_TopicDeletionValidation tests that:
// 1. Topics with messages cannot be deleted
// 2. Topics can be deleted after clearing messages
func TestE2E_TopicDeletionValidation(t *testing.T) {
	t.Log("Step 1: Login as admin")
	resp, body := makeRequest(t, "POST", "/admin/login", map[string]string{
		"username": "admin",
		"password": "UOOOWWW4",
	}, "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Admin login failed: %v", body)
	}

	adminToken := body["token"].(string)

	// Create publisher
	makeRequest(t, "POST", "/admin/users", map[string]string{
		"username": "test-del-publisher",
		"password": "test123",
		"role":     "publisher",
	}, adminToken)

	// Get publisher token
	_, body = makeRequest(t, "GET", "/admin/token?username=test-del-publisher", nil, adminToken)
	publisherToken := body["token"].(string)

	// Create topic
	topicName := fmt.Sprintf("test-del-topic-%d", time.Now().Unix())
	t.Log("Step 2: Create topic")
	makeRequest(t, "POST", "/admin/topics", map[string]string{
		"name": topicName,
	}, adminToken)

	// Publish a message
	t.Log("Step 3: Publish message to topic")
	makeRequest(t, "POST", "/send", map[string]interface{}{
		"topic":   topicName,
		"payload": map[string]string{"message": "Test message"},
	}, publisherToken)

	// Try to delete topic with messages (should fail)
	t.Log("Step 4: Try to delete topic with messages (should fail)")
	resp, body = makeRequest(t, "DELETE", "/admin/topics/"+topicName, nil, adminToken)

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("Expected 409 Conflict, got %d: %v", resp.StatusCode, body)
	}

	errorMsg, ok := body["error"].(string)
	if !ok || !strings.Contains(errorMsg, "has") {
		t.Fatalf("Expected error message about messages/subscribers, got: %v", body)
	}
	t.Logf("✅ Topic deletion correctly failed: %s", errorMsg)

	// Clear messages
	t.Log("Step 5: Clear messages from topic")
	resp, body = makeRequest(t, "DELETE", "/admin/topics/"+topicName+"/messages", nil, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to clear messages: %v", body)
	}
	t.Logf("✅ Messages cleared")

	// Now delete topic (should succeed)
	t.Log("Step 6: Delete topic (should succeed now)")
	resp, body = makeRequest(t, "DELETE", "/admin/topics/"+topicName, nil, adminToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %v", resp.StatusCode, body)
	}
	t.Logf("✅ Topic deleted successfully")

	// Cleanup user
	makeRequest(t, "DELETE", "/admin/users/test-del-publisher", nil, adminToken)

	t.Log("✅ Topic deletion validation test passed")
}
