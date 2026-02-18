package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func stringPtr(s string) *string {
	return &s
}

const baseURL = "https://localhost:8443"

func TestMain(m *testing.M) {
	// 1. Setup Config
	cfg := Config{
		Addr:                 ":8443",
		CertFile:             "certs/cert.pem",
		KeyFile:              "certs/key.pem",
		HTTPMode:             false,
		InitialAdminPassword: stringPtr("UOOOWWW4"),
	}

	// 2. Start Server
	srv, err := run(cfg)
	if err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}

	// Run server in goroutine
	go func() {
		if _, err := os.Stat(cfg.CertFile); os.IsNotExist(err) {
			_ = generateSelfSignedCert(cfg.CertFile, cfg.KeyFile)
		}
		if err := srv.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server failed: %v\n", err)
		}
	}()

	// 3. Wait for readiness
	time.Sleep(2 * time.Second)

	// 4. Run Tests
	code := m.Run()

	// 5. Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("Shutdown failed: %v\n", err)
	}

	// Cleanup DB? Maybe not needed for E2E if we want persistence or if we just want to execute.
	// Ideally we clean up, but SQLite file might be locked.
	// For now, let's just exit.
	os.Exit(code)
}

// Helper to create HTTP client that ignores self-signed certs
func newClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// Helper to make JSON requests
func makeRequest(t *testing.T, method, path string, body interface{}, token string) (*http.Response, map[string]interface{}) {
	client := newClient()

	var reqBody *bytes.Buffer
	if body != nil {
		jsonData, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonData)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Logf("Warning: Failed to decode response body: %v", err)
	}
	defer resp.Body.Close()

	return resp, result
}

// Helper to make JSON requests that return arrays
func makeArrayRequest(t *testing.T, method, path string, body interface{}, token string) (*http.Response, []interface{}) {
	client := newClient()

	var reqBody *bytes.Buffer
	if body != nil {
		jsonData, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonData)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	var result []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Logf("Warning: Failed to decode response body: %v", err)
	}
	defer resp.Body.Close()

	return resp, result
}

// TestE2E_AdminCreatePublisherAndTopic tests the complete workflow:
// 1. Login as admin
// 2. Create publisher user
// 3. Get token for publisher
// 4. Try to publish to non-existent topic (should fail)
// 5. Create topic
// 6. Publish to topic (should succeed)
// 7. Verify message exists
func TestE2E_AdminCreatePublisherAndTopic(t *testing.T) {
	// NOTE: This test requires the server to be running
	// Run: go run . -fcm-creds firebase-credentials.json
	// Or: go run . -http (for HTTP mode)

	t.Log("Step 1: Login as admin")
	resp, body := makeRequest(t, "POST", "/admin/login", map[string]string{
		"username": "admin",
		"password": "UOOOWWW4", // Update this!
	}, "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Admin login failed: %v", body)
	}

	adminToken, ok := body["token"].(string)
	if !ok || adminToken == "" {
		t.Fatalf("No token in admin login response: %v", body)
	}
	t.Logf("✅ Admin logged in")

	// Step 2: Create publisher user
	t.Log("Step 2: Create publisher user")
	resp, body = makeRequest(t, "POST", "/admin/users", map[string]string{
		"username": "test-publisher",
		"password": "test123",
		"role":     "publisher",
	}, adminToken)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		t.Fatalf("Failed to create publisher: %v", body)
	}
	t.Logf("✅ Publisher user created/exists")

	// Step 3: Get token for publisher
	t.Log("Step 3: Get token for publisher")
	resp, body = makeRequest(t, "GET", "/admin/token?username=test-publisher", nil, adminToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get publisher token: %v", body)
	}

	publisherToken, ok := body["token"].(string)
	if !ok || publisherToken == "" {
		t.Fatalf("No token in response: %v", body)
	}
	t.Logf("✅ Publisher token retrieved")

	// Step 4: Try to publish to non-existent topic (should fail with 404)
	t.Log("Step 4: Try to publish to non-existent topic (should fail)")
	topicName := fmt.Sprintf("test-topic-%d", time.Now().Unix())
	resp, body = makeRequest(t, "POST", "/send", map[string]interface{}{
		"topic":   topicName,
		"payload": map[string]string{"message": "Hello World"},
	}, publisherToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d: %v", resp.StatusCode, body)
	}
	t.Logf("✅ Publish correctly failed with 404")

	// Step 5: Create the topic
	t.Log("Step 5: Create topic")
	resp, body = makeRequest(t, "POST", "/admin/topics", map[string]string{
		"name": topicName,
	}, adminToken)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create topic: %v", body)
	}
	t.Logf("✅ Topic created")

	// Step 6: Publish to the topic (should succeed)
	t.Log("Step 6: Publish to topic (should succeed)")
	resp, body = makeRequest(t, "POST", "/send", map[string]interface{}{
		"topic":   topicName,
		"payload": map[string]string{"message": "Hello World"},
	}, publisherToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Publish failed: %v", body)
	}
	t.Logf("✅ Publish succeeded")

	// Step 7: Verify message in topic
	t.Log("Step 7: Verify message in topic")
	resp, messages := makeArrayRequest(t, "GET", "/admin/topics/"+topicName+"/messages", nil, adminToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get messages: %v", messages)
	}

	if len(messages) == 0 {
		t.Fatalf("No messages found in topic")
	}
	t.Logf("✅ Message found in topic (count: %d)", len(messages))

	// Cleanup: Delete the topic
	t.Log("Cleanup: Delete topic")
	// First clear messages
	resp, body = makeRequest(t, "DELETE", "/admin/topics/"+topicName+"/messages", nil, adminToken)
	t.Logf("Clear messages response: %d - %v", resp.StatusCode, body)
	// Then delete topic
	resp, body = makeRequest(t, "DELETE", "/admin/topics/"+topicName, nil, adminToken)
	t.Logf("Delete topic response: %d - %v", resp.StatusCode, body)
	// Delete test user
	resp, body = makeRequest(t, "DELETE", "/admin/users/test-publisher", nil, adminToken)
	t.Logf("Delete user response: %d - %v", resp.StatusCode, body)

	t.Log("✅ All E2E tests passed")
}

// TestE2E_SubscriberFlow tests subscriber functionality
func TestE2E_SubscriberFlow(t *testing.T) {
	t.Log("Step 1: Login as admin")
	resp, body := makeRequest(t, "POST", "/admin/login", map[string]string{
		"username": "admin",
		"password": "UOOOWWW4", // Update this!
	}, "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Admin login failed: %v", body)
	}

	adminToken := body["token"].(string)

	// Create subscriber user
	t.Log("Step 2: Create subscriber user")
	// Create subscriber user
	t.Log("Step 2: Create subscriber user")
	_, _ = makeRequest(t, "POST", "/admin/users", map[string]string{
		"username": "test-subscriber",
		"password": "test123",
		"role":     "subscriber",
	}, adminToken)

	// Get subscriber token
	t.Log("Step 3: Get subscriber token")
	_, body = makeRequest(t, "GET", "/admin/token?username=test-subscriber", nil, adminToken)
	subscriberToken := body["token"].(string)

	// Create topic
	topicName := fmt.Sprintf("test-sub-topic-%d", time.Now().Unix())
	t.Log("Step 4: Create topic")
	_, _ = makeRequest(t, "POST", "/admin/topics", map[string]string{
		"name": topicName,
	}, adminToken)

	// Subscribe to topic
	t.Log("Step 5: Subscribe to topic")
	resp, body = makeRequest(t, "POST", "/subscribe", map[string]interface{}{
		"topic":    topicName,
		"token":    "test-device-token-123",
		"provider": "mock",
	}, subscriberToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Subscribe failed: %v", body)
	}
	t.Logf("✅ Subscribed to topic")

	// Try to subscribe again (should be idempotent)
	t.Log("Step 6: Subscribe again (should be idempotent)")
	resp, body = makeRequest(t, "POST", "/subscribe", map[string]interface{}{
		"topic":    topicName,
		"token":    "test-device-token-123",
		"provider": "mock",
	}, subscriberToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Idempotent subscribe failed: %v", body)
	}
	if msg, ok := body["message"].(string); !ok || msg != "Already subscribed" {
		t.Logf("Warning: Expected 'Already subscribed' message, got: %v", body)
	}
	t.Logf("✅ Idempotent subscribe worked")

	// Cleanup
	resp, body = makeRequest(t, "DELETE", "/admin/topics/"+topicName+"/subscribers", nil, adminToken)
	t.Logf("Clear subscribers response: %d - %v", resp.StatusCode, body)
	resp, body = makeRequest(t, "DELETE", "/admin/topics/"+topicName, nil, adminToken)
	t.Logf("Delete topic response: %d - %v", resp.StatusCode, body)
	// Delete test user
	resp, body = makeRequest(t, "DELETE", "/admin/users/test-subscriber", nil, adminToken)
	t.Logf("Delete user response: %d - %v", resp.StatusCode, body)

	t.Log("✅ Subscriber flow test passed")
}
