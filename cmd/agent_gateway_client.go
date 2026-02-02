package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GatewayClient handles communication with the Zen Claw gateway
type GatewayClient struct {
	baseURL  string
	client   *http.Client
}

// NewGatewayClient creates a new gateway client
func NewGatewayClient(baseURL string) *GatewayClient {
	return &GatewayClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
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
	SessionID  string                 `json:"session_id"`
	Result     string                 `json:"result"`
	Error      string                 `json:"error,omitempty"`
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

// StartStream initiates a streaming session
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
