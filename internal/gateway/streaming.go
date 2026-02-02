package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kube-zen/zen-sdk/pkg/logging"
)

// StreamResponse represents a single chunk of streamed content
type StreamResponse struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
}

// StreamManager manages active streaming connections
type StreamManager struct {
	mu       sync.RWMutex
	streams  map[string]*StreamConnection
	logger   *logging.Logger
}

// StreamConnection represents an active streaming connection
type StreamConnection struct {
	ID         string
	Conn       *websocket.Conn
	CreatedAt  time.Time
	LastActive time.Time
	Cancel     context.CancelFunc
}

// NewStreamManager creates a new stream manager
func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams: make(map[string]*StreamConnection),
		logger:  logging.NewLogger("stream-manager"),
	}
}

// AddStream adds a new streaming connection
func (sm *StreamManager) AddStream(id string, conn *websocket.Conn, cancel context.CancelFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.streams[id] = &StreamConnection{
		ID:         id,
		Conn:       conn,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Cancel:     cancel,
	}
	
	fmt.Printf("INFO: %s\n", "test")
}

// RemoveStream removes a streaming connection
func (sm *StreamManager) RemoveStream(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if stream, exists := sm.streams[id]; exists {
		if stream.Cancel != nil {
			stream.Cancel()
		}
		delete(sm.streams, id)
		fmt.Printf("INFO: %s\n", "test")
	}
}

// Broadcast sends a message to all connected streams
func (sm *StreamManager) Broadcast(id string, response StreamResponse) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	stream, exists := sm.streams[id]
	if !exists {
		return fmt.Errorf("stream not found: %s", id)
	}
	
	// Update last active time
	stream.LastActive = time.Now()
	
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	
	return stream.Conn.WriteMessage(websocket.TextMessage, data)
}

// GetStream returns a stream connection
func (sm *StreamManager) GetStream(id string) (*StreamConnection, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	stream, exists := sm.streams[id]
	return stream, exists
}

// CleanupInactiveStreams removes inactive streams
func (sm *StreamManager) CleanupInactiveStreams(maxAge time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	now := time.Now()
	for id, stream := range sm.streams {
		if now.Sub(stream.LastActive) > maxAge {
			if stream.Cancel != nil {
				stream.Cancel()
			}
			delete(sm.streams, id)
			fmt.Printf("INFO: %s\n", "test")
		}
	}
}
