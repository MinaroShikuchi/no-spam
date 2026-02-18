package handlers

import (
	"net/http"
	"strings"

	"no-spam/middleware"
	"no-spam/store"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func CreateUserHandler(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
			Role     string `json:"role"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Validate Role
		if req.Role == "" {
			req.Role = "subscriber"
		}
		validRoles := map[string]bool{"admin": true, "publisher": true, "subscriber": true}
		if !validRoles[req.Role] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role. Must be admin, publisher, or subscriber"})
			return
		}

		// Hash password
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		if err := s.CreateUser(req.Username, string(hash), req.Role); err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint") {
				c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "User created", "username": req.Username, "role": req.Role})
	}
}

func DeleteUserHandler(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.Param("username")
		if username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username required"})
			return
		}

		// Prevent deleting self? Use middleware.GetUsername(c)
		operator := middleware.GetUsername(c)
		if operator == username {
			c.JSON(http.StatusConflict, gin.H{"error": "Cannot delete yourself"})
			return
		}

		if err := s.DeleteUser(username); err != nil {
			if strings.Contains(err.Error(), "user not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
	}
}

func ListUsersHandler(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		users, err := s.ListUsers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
			return
		}

		type UserResponse struct {
			Username string `json:"username"`
			Role     string `json:"role"`
		}

		var resp []UserResponse
		for _, u := range users {
			resp = append(resp, UserResponse{
				Username: u.Username,
				Role:     u.Role,
			})
		}

		c.JSON(http.StatusOK, resp)
	}
}

func LoginHandler(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		user, err := s.GetUser(req.Username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		// Generate Token
		token, err := middleware.GenerateToken(user.Username, user.Role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}

func RefreshHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		username := middleware.GetUsername(c)
		role := middleware.GetRole(c)

		if username == "" || role == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// Issue new token
		newToken, err := middleware.GenerateToken(username, role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": newToken})
	}
}
