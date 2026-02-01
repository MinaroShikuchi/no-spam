package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"no-spam/connectors"
	"no-spam/middleware"
	"no-spam/store"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
)

func main() {
	certFile := flag.String("cert", "cert.pem", "Path to TLS certificate file")
	keyFile := flag.String("key", "key.pem", "Path to TLS key file")
	addr := flag.String("addr", ":8443", "Address to listen on")
	flag.Parse()

	// Initialize Store
	s, err := store.NewSQLiteStore("no-spam.db")
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Initialize Hub
	hub := NewHub(s)

	// Initialize Connectors
	mockConn := connectors.NewMockConnector()
	fcmConn := connectors.NewFCMConnector()
	apnsConn := connectors.NewAPNSConnector()
	wsConn := connectors.NewWebSocketConnector()

	// Register Connectors
	hub.RegisterConnector("mock", mockConn)
	hub.RegisterConnector("fcm", fcmConn)
	hub.RegisterConnector("apns", apnsConn)
	hub.RegisterConnector("websocket", wsConn)

	// Define Handlers
	mux := http.NewServeMux()

	// /subscribe endpoint - Secured by JWT Middleware
	mux.Handle("/subscribe", middleware.JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if middleware.GetRole(r.Context()) != "subscriber" {
			http.Error(w, "Forbidden: Only subscribers can subscribe", http.StatusForbidden)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		type SubscriptionRequest struct {
			Topic    string `json:"topic"`
			Token    string `json:"token"`
			Provider string `json:"provider"`
		}

		var req SubscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Topic == "" || req.Token == "" || req.Provider == "" {
			http.Error(w, "Missing required fields (topic, token, provider)", http.StatusBadRequest)
			return
		}

		if err := hub.Subscribe(req.Topic, store.Subscriber{
			Token:    req.Token,
			Provider: req.Provider,
		}); err != nil {
			log.Printf("Subscribe error: %v", err)
			// Simple check for "constraint" or "unique" in error string (SQLite specific)
			// In prod, check specific sqlite error code.
			http.Error(w, "Already subscribed or error: "+err.Error(), http.StatusConflict)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Subscribed"))
	})))

	// /unsubscribe endpoint - Secured by JWT Middleware
	mux.Handle("/unsubscribe", middleware.JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if middleware.GetRole(r.Context()) != "subscriber" {
			http.Error(w, "Forbidden: Only subscribers can unsubscribe", http.StatusForbidden)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		type UnsubscriptionRequest struct {
			Topic string `json:"topic"`
			Token string `json:"token"`
		}

		var req UnsubscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Topic == "" || req.Token == "" {
			http.Error(w, "Missing required fields (topic, token)", http.StatusBadRequest)
			return
		}

		if err := hub.Unsubscribe(req.Topic, req.Token); err != nil {
			log.Printf("Unsubscribe error: %v", err)
			http.Error(w, fmtError(err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Unsubscribed"))
	})))

	// /stats endpoint - Secured by JWT Middleware
	mux.Handle("/stats", middleware.JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only publishers (admins) should see stats
		if middleware.GetRole(r.Context()) != "publisher" {
			http.Error(w, "Forbidden: Only publishers can view stats", http.StatusForbidden)
			return
		}

		stats := map[string]interface{}{
			"total_messages_sent":          hub.GetTotalMessagesSent(),
			"active_websocket_connections": wsConn.ConnectionCount(),
			"active_subscriptions":         hub.GetSubscriptionCount(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})))

	// /send endpoint - Secured by JWT Middleware
	mux.Handle("/send", middleware.JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if middleware.GetRole(r.Context()) != "publisher" {
			http.Error(w, "Forbidden: Only publishers can send messages", http.StatusForbidden)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var msg Message
		// Limit request body to 1MB
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := hub.Route(ctx, msg); err != nil {
			log.Printf("Error routing message: %v", err)
			http.Error(w, fmtError(err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Message sent"))
	})))

	// /ws endpoint - Secured by Query Param Token
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		role, err := middleware.ValidateWSToken(token)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}

		if role != "subscriber" {
			http.Error(w, "Forbidden: Only subscribers can connect to WebSocket", http.StatusForbidden)
			return
		}

		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: false, // Enforce same origin by default, or adjust as needed
			// OriginPatterns: []string{"*"}, // Uncomment to allow all origins during dev
		})
		if err != nil {
			log.Printf("Failed to accept websocket: %v", err)
			return
		}

		// Generate Audit ID for logs
		connID := uuid.New().String()

		// Add connection to connector
		log.Printf("New WebSocket connection: %s (User: ...%s)", connID, token[len(token)-6:])
		wsConn.AddConnection(token, c)

		// Replay Pending Messages (Offline Queue)
		go func() {
			pending, err := s.GetPendingMessages(token)
			if err != nil {
				log.Printf("[%s] Failed to fetch pending messages: %v", connID, err)
				return
			}
			if len(pending) > 0 {
				log.Printf("[%s] Replaying %d pending messages", connID, len(pending))
				ctx := context.Background()
				for _, item := range pending {
					if err := wsConn.Send(ctx, token, item.Payload); err == nil {
						s.MarkDelivered(item.ID)
					} else {
						log.Printf("[%s] Failed to replay message %d: %v", connID, item.ID, err)
						// Stop replaying if sending fails? Yes, assume broken connection
						break
					}
				}
			}
		}()

		// Ensure we remove the connection when it closes
		defer func() {
			wsConn.RemoveConnection(token)
			c.Close(websocket.StatusInternalError, "server stopped")
			log.Printf("WebSocket connection closed: %s", connID)
		}()

		// Read loop to detect disconnect
		ctx := r.Context()
		for {
			_, _, err := c.Read(ctx)
			if err != nil {
				// Normal closure or error triggers return, executing defer
				break
			}
		}
	})

	// Configure TLS 1.3 Strict
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}

	server := &http.Server{
		Addr:      *addr,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	log.Printf("Server listening on %s (TLS 1.3 strict)", *addr)
	log.Printf("Ensure you have %s and %s, or provide via flags", *certFile, *keyFile)

	// Check if cert files exist, if not, warn but try to start (will fail)
	// For dev experience, we could generate them, but prompt didn't ask.
	if _, err := os.Stat(*certFile); os.IsNotExist(err) {
		log.Printf("WARNING: Certificate file %s not found. Server will fail to start.", *certFile)
	}

	err = server.ListenAndServeTLS(*certFile, *keyFile)
	if err != nil {
		log.Fatal("Server failed: ", err)
	}
}

func fmtError(err error) string {
	return "Error: " + err.Error()
}
