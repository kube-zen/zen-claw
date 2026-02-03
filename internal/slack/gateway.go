package slack

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// ChatRequest is the request format for the gateway
type ChatRequest struct {
	SessionID  string `json:"session_id,omitempty"`
	UserInput  string `json:"user_input"`
	WorkingDir string `json:"working_dir,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	MaxSteps   int    `json:"max_steps,omitempty"`
}

// ChatResult is the result from the gateway
type ChatResult struct {
	SessionID   string                 `json:"session_id"`
	Result      string                 `json:"result"`
	SessionInfo map[string]interface{} `json:"session_info"`
}

// ProgressEvent represents a progress event from the gateway
type ProgressEvent struct {
	Type    string      `json:"type"`
	Step    int         `json:"step"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// NewGatewayClient creates a new gateway WebSocket client
func NewGatewayClient(url string) (*GatewayClient, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket dial failed: %w", err)
	}

	client := &GatewayClient{
		url:       url,
		conn:      conn,
		callbacks: make(map[string]chan WSMessage),
		done:      make(chan struct{}),
	}

	// Start message reader
	go client.readPump()

	// Wait for connected message
	time.Sleep(100 * time.Millisecond)

	return client, nil
}

// Close closes the WebSocket connection
func (c *GatewayClient) Close() error {
	close(c.done)
	return c.conn.Close()
}

// readPump reads messages from the WebSocket
func (c *GatewayClient) readPump() {
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
				log.Printf("[Gateway] Read error: %v", err)
			}
			return
		}

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		// Route to callback channel
		c.callbackMu.Lock()
		if ch, ok := c.callbacks[msg.ID]; ok {
			select {
			case ch <- msg:
			default:
				// Channel full, skip
			}
		}
		c.callbackMu.Unlock()
	}
}

// Send sends a message over WebSocket
func (c *GatewayClient) Send(msg WSMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// NextMsgID generates a unique message ID
func (c *GatewayClient) NextMsgID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgID++
	return fmt.Sprintf("slack_msg_%d", c.msgID)
}

// Chat sends a chat request and returns the result
func (c *GatewayClient) Chat(req ChatRequest, onProgress func(ProgressEvent)) (*ChatResult, error) {
	msgID := c.NextMsgID()
	responseChan := make(chan WSMessage, 100)

	// Register callback
	c.callbackMu.Lock()
	c.callbacks[msgID] = responseChan
	c.callbackMu.Unlock()

	defer func() {
		c.callbackMu.Lock()
		delete(c.callbacks, msgID)
		c.callbackMu.Unlock()
		close(responseChan)
	}()

	// Send chat request
	reqData, _ := json.Marshal(req)
	err := c.Send(WSMessage{
		Type: "chat",
		ID:   msgID,
		Data: reqData,
	})
	if err != nil {
		return nil, fmt.Errorf("send failed: %w", err)
	}

	// Wait for response with timeout
	timeout := time.After(30 * time.Minute)
	for {
		select {
		case msg := <-responseChan:
			switch msg.Type {
			case "progress":
				if onProgress != nil {
					var progressData map[string]interface{}
					if err := json.Unmarshal(msg.Data, &progressData); err == nil {
						event := ProgressEvent{
							Type:    getString(progressData, "type"),
							Step:    getInt(progressData, "step"),
							Message: getString(progressData, "message"),
							Data:    progressData["data"],
						}
						onProgress(event)
					}
				}

			case "result":
				var result ChatResult
				if err := json.Unmarshal(msg.Data, &result); err != nil {
					return nil, fmt.Errorf("parse result failed: %w", err)
				}
				return &result, nil

			case "error":
				var errData struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(msg.Data, &errData); err == nil {
					return nil, fmt.Errorf("%s", errData.Error)
				}
				return nil, fmt.Errorf("unknown error")

			case "cancelled":
				return nil, fmt.Errorf("task cancelled")
			}

		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for response")

		case <-c.done:
			return nil, fmt.Errorf("connection closed")
		}
	}
}

// Cancel cancels the current task
func (c *GatewayClient) Cancel() error {
	return c.Send(WSMessage{
		Type: "cancel",
	})
}

// ListSessions lists all sessions from the gateway
func (c *GatewayClient) ListSessions() ([]map[string]interface{}, error) {
	msgID := c.NextMsgID()
	responseChan := make(chan WSMessage, 1)

	c.callbackMu.Lock()
	c.callbacks[msgID] = responseChan
	c.callbackMu.Unlock()

	defer func() {
		c.callbackMu.Lock()
		delete(c.callbacks, msgID)
		c.callbackMu.Unlock()
	}()

	err := c.Send(WSMessage{
		Type: "sessions",
		ID:   msgID,
	})
	if err != nil {
		return nil, err
	}

	select {
	case msg := <-responseChan:
		var data struct {
			Sessions []map[string]interface{} `json:"sessions"`
		}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			return nil, err
		}
		return data.Sessions, nil

	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout")
	}
}

// Reconnect attempts to reconnect to the gateway
func (c *GatewayClient) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close existing connection
	if c.conn != nil {
		c.conn.Close()
	}

	// Reconnect
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("reconnect failed: %w", err)
	}

	c.conn = conn
	c.done = make(chan struct{})

	// Restart reader
	go c.readPump()

	return nil
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

// HealthCheck checks if the gateway is healthy
func (c *GatewayClient) HealthCheck() error {
	msgID := c.NextMsgID()
	responseChan := make(chan WSMessage, 1)

	c.callbackMu.Lock()
	c.callbacks[msgID] = responseChan
	c.callbackMu.Unlock()

	defer func() {
		c.callbackMu.Lock()
		delete(c.callbacks, msgID)
		c.callbackMu.Unlock()
	}()

	err := c.Send(WSMessage{
		Type: "ping",
		ID:   msgID,
	})
	if err != nil {
		return err
	}

	select {
	case msg := <-responseChan:
		if msg.Type == "pong" {
			return nil
		}
		return fmt.Errorf("unexpected response: %s", msg.Type)
	case <-time.After(5 * time.Second):
		return fmt.Errorf("ping timeout")
	}
}
