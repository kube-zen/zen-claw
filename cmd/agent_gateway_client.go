package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GatewayClient sends requests to the Zen Claw gateway
type GatewayClient struct {
	baseURL string
	timeout time.Duration
}

// NewGatewayClient creates a new gateway client
func NewGatewayClient(baseURL string) *GatewayClient {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	
	return &GatewayClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		timeout: 120 * time.Second, // Longer timeout for agent tasks
	}
}

// ChatRequest represents a chat request to the gateway
type ChatRequest struct {
	SessionID  string `json:"session_id"`
	UserInput  string `json:"user_input"`
	WorkingDir string `json:"working_dir,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	MaxSteps   int    `json:"max_steps,omitempty"`
}

// ChatResponse represents a chat response from the gateway
type ChatResponse struct {
	SessionID   string                 `json:"session_id"`
	Result      string                 `json:"result"`
	SessionInfo map[string]interface{} `json:"session_info"`
	Error       string                 `json:"error,omitempty"`
}

// Send sends a chat request to the gateway
func (c *GatewayClient) Send(req ChatRequest) (*ChatResponse, error) {
	// Prepare JSON request body
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}
	
	// Create HTTP request
	client := &http.Client{Timeout: c.timeout}
	gatewayURL := fmt.Sprintf("%s/chat", c.baseURL)
	
	httpReq, err := http.NewRequest("POST", gatewayURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w", err)
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	
	// Send request
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse JSON response
	var gatewayResp ChatResponse
	if err := json.Unmarshal(body, &gatewayResp); err != nil {
		return nil, fmt.Errorf("parse JSON response: %w", err)
	}
	
	return &gatewayResp, nil
}

// HealthCheck checks if the gateway is healthy
func (c *GatewayClient) HealthCheck() error {
	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := fmt.Sprintf("%s/health", c.baseURL)
	
	resp, err := client.Get(healthURL)
	if err != nil {
		return fmt.Errorf("gateway not reachable: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway health check failed with status %d", resp.StatusCode)
	}
	
	return nil
}