package connectors

import (
	"context"
	"fmt"
	"sync"

	"nhooyr.io/websocket"
)

// WebSocketConnector manages WebSocket connections and sends messages to them.
type WebSocketConnector struct {
	mu          sync.Mutex
	connections map[string]*websocket.Conn
}

// NewWebSocketConnector creates a new WebSocketConnector.
func NewWebSocketConnector() *WebSocketConnector {
	return &WebSocketConnector{
		connections: make(map[string]*websocket.Conn),
	}
}

// AddConnection registers a new websocket connection for a token.
func (w *WebSocketConnector) AddConnection(token string, conn *websocket.Conn) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.connections[token] = conn
}

// RemoveConnection removes a websocket connection.
func (w *WebSocketConnector) RemoveConnection(token string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.connections, token)
}

// Send sends a message to the connected websocket client.
func (w *WebSocketConnector) Send(ctx context.Context, token string, payload []byte) error {
	w.mu.Lock()
	conn, ok := w.connections[token]
	w.mu.Unlock()

	if !ok {
		return fmt.Errorf("no active connection for token: %s", token)
	}

	// In a real implementation, we should probably handle write timeouts and concurrent writes carefully.
	// nhooyr.io/websocket logic for writing:
	return conn.Write(ctx, websocket.MessageText, payload)
}

// ConnectionCount returns the number of active connections.
func (w *WebSocketConnector) ConnectionCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.connections)
}
