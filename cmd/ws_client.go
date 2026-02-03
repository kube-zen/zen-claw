package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSClient handles WebSocket communication with the gateway
type WSClient struct {
	conn       *websocket.Conn
	done       chan struct{}
	msgID      int
	mu         sync.Mutex
	callbacks  map[string]func(WSMessage)
	callbackMu sync.Mutex
}

// WSMessage matches the server's message format
type WSMessage struct {
	Type string          `json:"type"`
	ID   string          `json:"id,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// WSChatRequest is the chat request format
type WSChatRequest struct {
	SessionID  string `json:"session_id,omitempty"`
	UserInput  string `json:"user_input"`
	WorkingDir string `json:"working_dir,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	MaxSteps   int    `json:"max_steps,omitempty"`
}

// NewWSClient creates a WebSocket client connection
func NewWSClient(url string) (*WSClient, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket dial failed: %w", err)
	}

	client := &WSClient{
		conn:      conn,
		done:      make(chan struct{}),
		callbacks: make(map[string]func(WSMessage)),
	}

	// Start message reader
	go client.readPump()

	return client, nil
}

// Close closes the WebSocket connection
func (c *WSClient) Close() error {
	close(c.done)
	return c.conn.Close()
}

// readPump reads messages from the WebSocket
func (c *WSClient) readPump() {
	defer c.conn.Close()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] Read error: %v", err)
			}
			return
		}

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		// Call registered callback if exists
		c.callbackMu.Lock()
		if callback, ok := c.callbacks[msg.ID]; ok {
			callback(msg)
		}
		c.callbackMu.Unlock()
	}
}

// Send sends a message over WebSocket
func (c *WSClient) Send(msg WSMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// NextMsgID generates a unique message ID
func (c *WSClient) NextMsgID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgID++
	return fmt.Sprintf("msg_%d", c.msgID)
}

// RegisterCallback registers a callback for a message ID
func (c *WSClient) RegisterCallback(id string, callback func(WSMessage)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.callbacks[id] = callback
}

// UnregisterCallback removes a callback
func (c *WSClient) UnregisterCallback(id string) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	delete(c.callbacks, id)
}

// Chat sends a chat request and streams progress events
func (c *WSClient) Chat(req WSChatRequest, onProgress func(ProgressEvent), onResult func(*ChatResponse, error)) {
	msgID := c.NextMsgID()
	done := make(chan struct{})

	// Register callback for this message
	c.RegisterCallback(msgID, func(msg WSMessage) {
		switch msg.Type {
		case "progress":
			var progressData map[string]interface{}
			if err := json.Unmarshal(msg.Data, &progressData); err == nil {
				event := ProgressEvent{
					Type:    getString(progressData, "type"),
					Step:    getInt(progressData, "step"),
					Message: getString(progressData, "message"),
					Data:    progressData["data"],
				}
				if onProgress != nil {
					onProgress(event)
				}
			}

		case "result":
			var result struct {
				SessionID   string                 `json:"session_id"`
				Result      string                 `json:"result"`
				SessionInfo map[string]interface{} `json:"session_info"`
			}
			if err := json.Unmarshal(msg.Data, &result); err == nil {
				if onResult != nil {
					onResult(&ChatResponse{
						SessionID:   result.SessionID,
						Result:      result.Result,
						SessionInfo: result.SessionInfo,
					}, nil)
				}
			}
			close(done)

		case "error":
			var errData struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(msg.Data, &errData); err == nil {
				if onResult != nil {
					onResult(nil, fmt.Errorf("%s", errData.Error))
				}
			}
			close(done)

		case "cancelled":
			if onResult != nil {
				onResult(nil, fmt.Errorf("task cancelled"))
			}
			close(done)
		}
	})

	// Send chat request
	reqData, _ := json.Marshal(req)
	err := c.Send(WSMessage{
		Type: "chat",
		ID:   msgID,
		Data: reqData,
	})

	if err != nil {
		c.UnregisterCallback(msgID)
		if onResult != nil {
			onResult(nil, err)
		}
		return
	}

	// Wait for completion
	<-done
	c.UnregisterCallback(msgID)
}

// Cancel cancels the current task
func (c *WSClient) Cancel() error {
	return c.Send(WSMessage{
		Type: "cancel",
	})
}

// Helper functions
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
