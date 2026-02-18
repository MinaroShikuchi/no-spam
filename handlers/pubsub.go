package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"no-spam/hub"
	"no-spam/middleware"
	"no-spam/store"

	"github.com/gin-gonic/gin"
)

func SubscribeHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Topic    string `json:"topic" binding:"required"`
			Token    string `json:"token"`
			Webhook  string `json:"webhook"`
			Provider string `json:"provider" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields (topic, provider)"})
			return
		}

		// Alias webhook to token
		if req.Token == "" && req.Webhook != "" {
			req.Token = req.Webhook
		}

		// Validate token
		if req.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing token or webhook field"})
			return
		}

		username := middleware.GetUsername(c)
		if username == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No username in context"})
			return
		}

		if err := h.Subscribe(req.Topic, store.Subscriber{
			Token:    req.Token,
			Provider: req.Provider,
			Username: username,
		}); err != nil {
			log.Printf("Subscribe error: %v", err)
			if err == hub.ErrTopicNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Topic not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Subscribed"})
	}
}

func UnsubscribeHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Topic string `json:"topic" binding:"required"`
			Token string `json:"token" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields (topic, token)"})
			return
		}

		if err := h.Unsubscribe(req.Topic, req.Token); err != nil {
			log.Printf("Unsubscribe error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Unsubscribed"})
	}
}

func TopicsHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := middleware.GetUsername(c)
		if username == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		subs, err := h.GetSubscriptionsByUser(username)
		if err != nil {
			log.Printf("GetSubscriptions error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, subs)
	}
}

func SendHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		var msg hub.Message
		if err := c.ShouldBindJSON(&msg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := h.Route(ctx, msg); err != nil {
			log.Printf("Error routing message: %v", err)
			if err == hub.ErrTopicNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Topic not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Message sent"})
	}
}

func StatsHandler(h *hub.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats := gin.H{
			"total_messages_sent":  h.GetTotalMessagesSent(),
			"active_subscriptions": h.GetSubscriptionCount(),
		}
		c.JSON(http.StatusOK, stats)
	}
}
