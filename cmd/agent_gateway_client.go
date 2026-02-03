package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// GatewayClient handles communication with the Zen Claw gateway
type GatewayClient struct {
	baseURL string
	client  *http.Client
}

// NewGatewayClient creates a new gateway client
func NewGatewayClient(baseURL string) *GatewayClient {
	return &GatewayClient{
		baseURL: baseURL,
		client: &http.Client{
			// Large tasks can take a long time - similar to how Cursor handles them.
			// Complex multi-step tasks with large context models may need 30+ minutes.
			// We use 45 minutes to be generous and match the gateway's internal timeout.
			// Individual AI calls have their own 5-minute per-step timeout.
			Timeout: 45 * time.Minute,
		},
	}
}

// ChatRequest represents a chat request to the gateway
type ChatRequest struct {
	SessionID  string `json:"session_id"`
	UserInput  string `json:"user_input"`
	WorkingDir string `json:"working_dir"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	MaxSteps   int    `json:"max_steps"`
}

// ChatResponse represents a chat response from the gateway
type ChatResponse struct {
	SessionID   string                 `json:"session_id"`
	Result      string                 `json:"result"`
	Error       string                 `json:"error,omitempty"`
	SessionInfo map[string]interface{} `json:"session_info,omitempty"`
}

// HealthCheck checks if the gateway is reachable
func (gc *GatewayClient) HealthCheck() error {
	url := fmt.Sprintf("%s/health", gc.baseURL)

	resp, err := gc.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway unhealthy: %d", resp.StatusCode)
	}

	return nil
}

// Send sends a chat request to the gateway
func (gc *GatewayClient) Send(req ChatRequest) (*ChatResponse, error) {
	url := fmt.Sprintf("%s/chat", gc.baseURL)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := gc.client.Post(url, "application/json", bytes.NewBuffer(jsonReq))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway request failed: %d", resp.StatusCode)
	}

	var response ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

// StreamRequest represents a streaming request to the gateway
type StreamRequest struct {
	SessionID string `json:"session_id"`
	Task      string `json:"task"`
	Stream    bool   `json:"stream"`
}

// StreamResponse represents a streaming response from the gateway
type StreamResponse struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
}

// StartStream initiates a streaming session (deprecated, use SendWithProgress)
func (gc *GatewayClient) StartStream(sessionID, task string) (string, error) {
	url := fmt.Sprintf("%s/stream", gc.baseURL)

	req := StreamRequest{
		SessionID: sessionID,
		Task:      task,
		Stream:    true,
	}

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	resp, err := gc.client.Post(url, "application/json", bytes.NewBuffer(jsonReq))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to start stream: %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	streamID, ok := response["stream_id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid response: missing stream_id")
	}

	return streamID, nil
}

// ProgressEvent represents a streaming progress event
type ProgressEvent struct {
	Type        string                 `json:"type"`
	Step        int                    `json:"step,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Data        interface{}            `json:"data,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	Result      string                 `json:"result,omitempty"`
	SessionInfo map[string]interface{} `json:"session_info,omitempty"`
}

// SessionListResponse represents the response from /sessions endpoint
type SessionListResponse struct {
	Sessions    []SessionEntry `json:"sessions"`
	Count       int            `json:"count"`
	MaxSessions int            `json:"max_sessions"`
	ActiveCount int            `json:"active_count"`
}

// SessionEntry represents a session in the list
type SessionEntry struct {
	ID                string `json:"id"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	MessageCount      int    `json:"message_count"`
	UserMessages      int    `json:"user_messages"`
	AssistantMessages int    `json:"assistant_messages"`
	ToolMessages      int    `json:"tool_messages"`
	WorkingDir        string `json:"working_dir"`
	State             string `json:"state"`
	ClientID          string `json:"client_id"`
	LastUsed          string `json:"last_used"`
}

// ListSessions lists all sessions from the gateway
func (gc *GatewayClient) ListSessions() (*SessionListResponse, error) {
	url := fmt.Sprintf("%s/sessions", gc.baseURL)

	resp, err := gc.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list sessions: %d", resp.StatusCode)
	}

	var result SessionListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// BackgroundSession moves a session to background state
func (gc *GatewayClient) BackgroundSession(sessionID string) error {
	url := fmt.Sprintf("%s/sessions/%s/background", gc.baseURL, sessionID)

	resp, err := gc.client.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to background session: %d", resp.StatusCode)
	}

	return nil
}

// ActivateSession activates a session
func (gc *GatewayClient) ActivateSession(sessionID, clientID string) error {
	url := fmt.Sprintf("%s/sessions/%s/activate", gc.baseURL, sessionID)

	body := map[string]string{"client_id": clientID}
	jsonBody, _ := json.Marshal(body)

	resp, err := gc.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to activate session: %d", resp.StatusCode)
	}

	return nil
}

// DeleteSession deletes a session
func (gc *GatewayClient) DeleteSession(sessionID string) error {
	url := fmt.Sprintf("%s/sessions/%s", gc.baseURL, sessionID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := gc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete session: %d", resp.StatusCode)
	}

	return nil
}

// GetPreferences returns AI preferences
func (gc *GatewayClient) GetPreferences(category string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/preferences/%s", gc.baseURL, category)

	resp, err := gc.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get preferences: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// UpdatePreferences updates AI preferences
func (gc *GatewayClient) UpdatePreferences(updates map[string]interface{}) error {
	url := fmt.Sprintf("%s/preferences", gc.baseURL)

	jsonBody, err := json.Marshal(updates)
	if err != nil {
		return err
	}

	resp, err := gc.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update preferences: %d", resp.StatusCode)
	}

	return nil
}

// SendWithProgress sends a chat request with SSE streaming for progress
func (gc *GatewayClient) SendWithProgress(req ChatRequest, onProgress func(ProgressEvent)) (*ChatResponse, error) {
	url := fmt.Sprintf("%s/chat/stream", gc.baseURL)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonReq))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := gc.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway request failed: %d", resp.StatusCode)
	}

	// Read SSE stream
	scanner := bufio.NewScanner(resp.Body)
	var finalResponse *ChatResponse

	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: "data: {...}"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		var event ProgressEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		// Call progress callback for all events
		if onProgress != nil {
			onProgress(event)
		}

		// Check for final "done" event
		if event.Type == "done" {
			finalResponse = &ChatResponse{
				SessionID:   event.SessionID,
				Result:      event.Result,
				SessionInfo: event.SessionInfo,
			}
		}

		// Check for error
		if event.Type == "error" {
			return &ChatResponse{
				Error: event.Message,
			}, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	if finalResponse == nil {
		return nil, fmt.Errorf("stream ended without final response")
	}

	return finalResponse, nil
}
