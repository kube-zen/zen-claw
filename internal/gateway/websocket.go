package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local development
	},
}

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type string          `json:"type"`
	ID   string          `json:"id,omitempty"`   // Message ID for request/response matching
	Data json.RawMessage `json:"data,omitempty"` // Payload
}

// WSChatRequest is the chat request sent over WebSocket
type WSChatRequest struct {
	SessionID  string `json:"session_id,omitempty"`
	UserInput  string `json:"user_input"`
	WorkingDir string `json:"working_dir,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	MaxSteps   int    `json:"max_steps,omitempty"`
}

// WSClient represents a connected WebSocket client
type WSClient struct {
	conn         *websocket.Conn
	server       *Server
	send         chan []byte
	done         chan struct{}
	mu           sync.Mutex
	cancelFunc   context.CancelFunc // Cancel current task
	currentMsgID string             // ID of current task
}

// NewWSClient creates a new WebSocket client handler
func NewWSClient(conn *websocket.Conn, server *Server) *WSClient {
	return &WSClient{
		conn:   conn,
		server: server,
		send:   make(chan []byte, 256),
		done:   make(chan struct{}),
	}
}

// Run handles the WebSocket connection
func (c *WSClient) Run() {
	// Start writer goroutine
	go c.writePump()

	// Send welcome message
	c.sendMessage(WSMessage{
		Type: "connected",
		Data: json.RawMessage(`{"message":"Connected to Zen Claw WebSocket","version":"0.1.0"}`),
	})

	// Read messages
	c.readPump()
}

// readPump reads messages from the WebSocket connection
func (c *WSClient) readPump() {
	defer func() {
		c.conn.Close()
		close(c.done)
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB max message size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Minute))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Minute))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WebSocket] Read error: %v", err)
			}
			return
		}

		c.handleMessage(message)
	}
}

// writePump writes messages to the WebSocket connection
func (c *WSClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("[WebSocket] Write error: %v", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			return
		}
	}
}

// handleMessage processes incoming WebSocket messages
func (c *WSClient) handleMessage(message []byte) {
	var msg WSMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		c.sendError("", "Invalid JSON: "+err.Error())
		return
	}

	switch msg.Type {
	case "chat":
		c.handleChat(msg)

	case "cancel":
		c.handleCancel(msg)

	case "ping":
		c.sendMessage(WSMessage{Type: "pong", ID: msg.ID})

	case "sessions":
		c.handleSessions(msg)

	case "session":
		c.handleSession(msg)

	default:
		c.sendError(msg.ID, "Unknown message type: "+msg.Type)
	}
}

// handleChat processes a chat request
func (c *WSClient) handleChat(msg WSMessage) {
	var req WSChatRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		c.sendError(msg.ID, "Invalid chat request: "+err.Error())
		return
	}

	if req.UserInput == "" {
		c.sendError(msg.ID, "user_input is required")
		return
	}

	if req.WorkingDir == "" {
		req.WorkingDir = "."
	}

	// Cancel any existing task
	c.mu.Lock()
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	c.cancelFunc = cancel
	c.currentMsgID = msg.ID
	c.mu.Unlock()

	// Convert to gateway ChatRequest
	chatReq := ChatRequest{
		SessionID:  req.SessionID,
		UserInput:  req.UserInput,
		WorkingDir: req.WorkingDir,
		Provider:   req.Provider,
		Model:      req.Model,
		MaxSteps:   req.MaxSteps,
	}

	// Run in goroutine
	go func() {
		defer func() {
			c.mu.Lock()
			if c.currentMsgID == msg.ID {
				c.cancelFunc = nil
				c.currentMsgID = ""
			}
			c.mu.Unlock()
			cancel()
		}()

		// Send progress events via WebSocket
		resp, err := c.server.agentService.ChatWithProgress(ctx, chatReq, func(event map[string]interface{}) {
			// Add message ID to event
			eventWithID := make(map[string]interface{})
			for k, v := range event {
				eventWithID[k] = v
			}
			eventWithID["id"] = msg.ID

			eventJSON, _ := json.Marshal(eventWithID)
			c.sendMessage(WSMessage{
				Type: "progress",
				ID:   msg.ID,
				Data: eventJSON,
			})
		})

		if err != nil {
			c.sendError(msg.ID, err.Error())
			return
		}

		if resp.Error != "" {
			c.sendError(msg.ID, resp.Error)
			return
		}

		// Send final result
		resultData, _ := json.Marshal(map[string]interface{}{
			"session_id":   resp.SessionID,
			"result":       resp.Result,
			"session_info": resp.SessionInfo,
		})

		c.sendMessage(WSMessage{
			Type: "result",
			ID:   msg.ID,
			Data: resultData,
		})
	}()
}

// handleCancel cancels the current task
func (c *WSClient) handleCancel(msg WSMessage) {
	c.mu.Lock()
	if c.cancelFunc != nil {
		c.cancelFunc()
		c.sendMessage(WSMessage{
			Type: "cancelled",
			ID:   c.currentMsgID,
			Data: json.RawMessage(`{"message":"Task cancelled"}`),
		})
	} else {
		c.sendMessage(WSMessage{
			Type: "info",
			ID:   msg.ID,
			Data: json.RawMessage(`{"message":"No task to cancel"}`),
		})
	}
	c.mu.Unlock()
}

// handleSessions lists all sessions
func (c *WSClient) handleSessions(msg WSMessage) {
	sessions := c.server.agentService.ListSessions()

	sessionsJSON, _ := json.Marshal(map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})

	c.sendMessage(WSMessage{
		Type: "sessions",
		ID:   msg.ID,
		Data: sessionsJSON,
	})
}

// handleSession gets or deletes a session
func (c *WSClient) handleSession(msg WSMessage) {
	var req struct {
		SessionID string `json:"session_id"`
		Action    string `json:"action"` // "get" or "delete"
	}

	if err := json.Unmarshal(msg.Data, &req); err != nil {
		c.sendError(msg.ID, "Invalid session request: "+err.Error())
		return
	}

	switch req.Action {
	case "get", "":
		session, exists := c.server.agentService.GetSession(req.SessionID)
		if !exists {
			c.sendError(msg.ID, "Session not found: "+req.SessionID)
			return
		}
		stats := session.GetStats()
		sessionJSON, _ := json.Marshal(stats)
		c.sendMessage(WSMessage{
			Type: "session",
			ID:   msg.ID,
			Data: sessionJSON,
		})

	case "delete":
		deleted := c.server.agentService.DeleteSession(req.SessionID)
		resultJSON, _ := json.Marshal(map[string]interface{}{
			"deleted": deleted,
			"id":      req.SessionID,
		})
		c.sendMessage(WSMessage{
			Type: "session_deleted",
			ID:   msg.ID,
			Data: resultJSON,
		})

	default:
		c.sendError(msg.ID, "Unknown action: "+req.Action)
	}
}

// sendMessage sends a message to the client
func (c *WSClient) sendMessage(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WebSocket] Marshal error: %v", err)
		return
	}

	select {
	case c.send <- data:
	default:
		log.Printf("[WebSocket] Send buffer full, dropping message")
	}
}

// sendError sends an error message to the client
func (c *WSClient) sendError(id, message string) {
	errorData, _ := json.Marshal(map[string]string{
		"error": message,
	})
	c.sendMessage(WSMessage{
		Type: "error",
		ID:   id,
		Data: errorData,
	})
}

// wsHandler handles WebSocket upgrade requests
func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade error: %v", err)
		return
	}

	log.Printf("[WebSocket] New connection from %s", r.RemoteAddr)

	client := NewWSClient(conn, s)
	client.Run()

	log.Printf("[WebSocket] Connection closed from %s", r.RemoteAddr)
}
