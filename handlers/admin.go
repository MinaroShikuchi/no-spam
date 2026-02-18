package handlers

import (
	"net/http"
	"strings"

	"no-spam/hub"
	"no-spam/middleware"
	"no-spam/store"

	"github.com/gin-gonic/gin"
)

func ListTopicsHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		topics, err := h.ListTopics()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list topics"})
			return
		}
		c.JSON(http.StatusOK, topics)
	}
}

func CreateTopicHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name string `json:"name" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing topic name"})
			return
		}

		if err := h.CreateTopic(req.Name); err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint") {
				c.JSON(http.StatusConflict, gin.H{"error": "Topic already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create topic"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "Topic created"})
	}
}

func DeleteTopicHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		if err := h.DeleteTopic(name); err != nil {
			if strings.Contains(err.Error(), "cannot delete topic") {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete topic"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Topic deleted"})
	}
}

func GetMessagesHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		msgs, err := h.GetRecentMessages(name, 100)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get messages"})
			return
		}

		c.JSON(http.StatusOK, msgs)
	}
}

func ClearMessagesHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		if err := h.ClearTopicMessages(name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear messages"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Messages cleared"})
	}
}

func GetSubscribersHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		subs, err := h.GetSubscribers(name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscribers"})
			return
		}

		c.JSON(http.StatusOK, subs)
	}
}

func ClearSubscribersHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		if err := h.ClearTopicSubscribers(name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear subscribers"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Subscribers cleared"})
	}
}

func GetTokenHandler(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.Query("username")
		if username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username parameter is required"})
			return
		}

		// Verify user exists
		user, err := s.GetUser(username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user"})
			return
		}
		if user == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Generate token with user's stored role
		token, err := middleware.GenerateToken(user.Username, user.Role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token":    token,
			"role":     user.Role,
			"username": user.Username,
		})
	}
}

func GetQueueHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		queue, err := h.GetQueue(name)
		if err != nil {
			if err == hub.ErrTopicNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Topic not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get queue"})
			return
		}

		c.JSON(http.StatusOK, queue)
	}
}
