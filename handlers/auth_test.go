package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"no-spam/store"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// setupTestContext creates a test Gin context
func setupTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

// setupTestStore creates test store with sample data
func setupTestStore(t *testing.T) store.Store {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}

	// Create test users
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	s.CreateUser("testadmin", string(hash), "admin")
	s.CreateUser("testpublisher", string(hash), "publisher")
	s.CreateUser("testsubscriber", string(hash), "subscriber")

	return s
}

// TestLoginHandler tests the login handler
func TestLoginHandler(t *testing.T) {
	s := setupTestStore(t)
	handler := LoginHandler(s)

	tests := []struct {
		name           string
		body           map[string]string
		expectedStatus int
		expectToken    bool
	}{
		{
			name:           "Valid credentials",
			body:           map[string]string{"username": "testadmin", "password": "password123"},
			expectedStatus: http.StatusOK,
			expectToken:    true,
		},
		{
			name:           "Invalid password",
			body:           map[string]string{"username": "testadmin", "password": "wrongpassword"},
			expectedStatus: http.StatusUnauthorized,
			expectToken:    false,
		},
		{
			name:           "Non-existent user",
			body:           map[string]string{"username": "nonexistent", "password": "password123"},
			expectedStatus: http.StatusUnauthorized,
			expectToken:    false,
		},
		{
			name:           "Missing username",
			body:           map[string]string{"password": "password123"},
			expectedStatus: http.StatusBadRequest,
			expectToken:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupTestContext()

			bodyBytes, _ := json.Marshal(tt.body)
			c.Request = httptest.NewRequest("POST", "/login", bytes.NewBuffer(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			handler(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)

			if tt.expectToken {
				if _, ok := response["token"]; !ok {
					t.Error("Expected token in response")
				}
			}
		})
	}
}

// TestCreateUserHandler tests user creation
func TestCreateUserHandler(t *testing.T) {
	s := setupTestStore(t)
	handler := CreateUserHandler(s)

	tests := []struct {
		name           string
		body           map[string]string
		expectedStatus int
	}{
		{
			name:           "Create publisher",
			body:           map[string]string{"username": "newpublisher", "password": "pass123", "role": "publisher"},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "Create subscriber",
			body:           map[string]string{"username": "newsub", "password": "pass123", "role": "subscriber"},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "Invalid role",
			body:           map[string]string{"username": "newuser", "password": "pass123", "role": "invalid"},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing password",
			body:           map[string]string{"username": "newuser", "role": "publisher"},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Duplicate username",
			body:           map[string]string{"username": "testadmin", "password": "pass123", "role": "admin"},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupTestContext()

			bodyBytes, _ := json.Marshal(tt.body)
			c.Request = httptest.NewRequest("POST", "/admin/users", bytes.NewBuffer(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			handler(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestDeleteUserHandler tests user deletion
func TestDeleteUserHandler(t *testing.T) {
	s := setupTestStore(t)
	handler := DeleteUserHandler(s)

	tests := []struct {
		name           string
		username       string
		expectedStatus int
	}{
		{
			name:           "Delete existing user",
			username:       "testpublisher",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Delete non-existent user",
			username:       "nonexistent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupTestContext()

			c.Params = gin.Params{{Key: "username", Value: tt.username}}
			c.Request = httptest.NewRequest("DELETE", "/admin/users/"+tt.username, nil)

			handler(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestListUsersHandler tests listing users
func TestListUsersHandler(t *testing.T) {
	s := setupTestStore(t)
	handler := ListUsersHandler(s)

	c, w := setupTestContext()
	c.Request = httptest.NewRequest("GET", "/admin/users", nil)

	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var users []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &users)

	if len(users) < 3 {
		t.Errorf("Expected at least 3 users, got %d", len(users))
	}

	// Verify no password hashes in response
	for _, user := range users {
		if _, ok := user["password_hash"]; ok {
			t.Error("Password hash should not be in response")
		}
		if _, ok := user["username"]; !ok {
			t.Error("Username should be in response")
		}
		if _, ok := user["role"]; !ok {
			t.Error("Role should be in response")
		}
	}
}

// TestGetTokenHandler tests token generation for users
func TestGetTokenHandler(t *testing.T) {
	s := setupTestStore(t)
	handler := GetTokenHandler(s)

	tests := []struct {
		name           string
		username       string
		expectedStatus int
		expectToken    bool
	}{
		{
			name:           "Generate token for existing user",
			username:       "testpublisher",
			expectedStatus: http.StatusOK,
			expectToken:    true,
		},
		{
			name:           "Generate token for non-existent user",
			username:       "nonexistent",
			expectedStatus: http.StatusNotFound,
			expectToken:    false,
		},
		{
			name:           "Missing username parameter",
			username:       "",
			expectedStatus: http.StatusBadRequest,
			expectToken:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupTestContext()

			c.Request = httptest.NewRequest("GET", "/admin/token?username="+tt.username, nil)

			handler(c)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectToken {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)

				if _, ok := response["token"]; !ok {
					t.Error("Expected token in response")
				}
			}
		})
	}
}
