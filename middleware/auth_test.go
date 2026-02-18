package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetJWTSecret(t *testing.T) {
	// Test default
	os.Unsetenv("JWT_SECRET")
	secret := GetJWTSecret()
	if string(secret) != "super-secret-key-change-me" {
		t.Errorf("Expected default secret, got %s", secret)
	}

	// Test env var
	os.Setenv("JWT_SECRET", "test-secret")
	secret = GetJWTSecret()
	if string(secret) != "test-secret" {
		t.Errorf("Expected test-secret, got %s", secret)
	}
	os.Unsetenv("JWT_SECRET") // Reset
}

func TestGenerateAndParseToken(t *testing.T) {
	username := "testuser"
	role := "admin"

	tokenString, err := GenerateToken(username, role)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ParseToken(tokenString)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	if claims.Subject != username {
		t.Errorf("Expected subject %s, got %s", username, claims.Subject)
	}
	if claims.Role != role {
		t.Errorf("Expected role %s, got %s", role, claims.Role)
	}
}

func TestJWTAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Missing Header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Authorization header missing",
		},
		{
			name:           "Invalid Format",
			authHeader:     "InvalidFormat",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid Authorization header format",
		},
		{
			name:           "Invalid Token",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid token",
		},
		{
			name:           "Valid Token",
			authHeader:     "Bearer " + generateTestToken("user", "user"),
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req, _ := http.NewRequest("GET", "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			c.Request = req

			// Middleware
			middleware := JWTAuthMiddleware()

			// Mock Next handler
			handler := func(c *gin.Context) {
				c.String(http.StatusOK, "OK")
			}

			// Chain them manually for unit test (or use router)
			// Using router is better to simulate middleware chain
			router := gin.New()
			router.Use(middleware)
			router.GET("/", handler)

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validToken := generateTestToken("admin", "admin")
	userToken := generateTestToken("user", "user")

	tests := []struct {
		name           string
		token          string
		requiredRole   string
		expectedStatus int
	}{
		{
			name:           "Authorized",
			token:          validToken,
			requiredRole:   "admin",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Forbidden - Wrong Role",
			token:          userToken,
			requiredRole:   "admin",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Forbidden - No Role (Bypass middleware simulation)",
			token:          "", // No token, so role not set in context
			requiredRole:   "admin",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			router := gin.New()
			// Only use JWT middleware if token is provided to simulate context setup
			if tt.token != "" {
				router.Use(JWTAuthMiddleware())
			}

			router.Use(RequireRole(tt.requiredRole))
			router.GET("/", func(c *gin.Context) {
				c.String(http.StatusOK, "OK")
			})

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestContextHelpers(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// Test empty
	if GetUsername(c) != "" {
		t.Error("Expected empty username")
	}
	if GetRole(c) != "" {
		t.Error("Expected empty role")
	}

	// Test with values
	c.Set("username", "testuser")
	c.Set("role", "admin")

	if GetUsername(c) != "testuser" {
		t.Errorf("Expected testuser, got %s", GetUsername(c))
	}
	if GetRole(c) != "admin" {
		t.Errorf("Expected admin, got %s", GetRole(c))
	}

	// Test invalid types
	c.Set("username", 123)
	if GetUsername(c) != "" {
		t.Error("Expected empty username for invalid type")
	}
}

func generateTestToken(username, role string) string {
	token, _ := GenerateToken(username, role)
	return token
}
