package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/ai"
)

// GatewayProvider uses the Zen Claw HTTP gateway
type GatewayProvider struct {
	baseURL string
	timeout time.Duration
}

// NewGatewayProvider creates a new gateway provider
func NewGatewayProvider(baseURL string) *GatewayProvider {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	
	return &GatewayProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		timeout: 30 * time.Second,
	}
}

func (p *GatewayProvider) Name() string {
	return "gateway"
}

func (p *GatewayProvider) SupportsTools() bool {
	// Gateway doesn't support tool calls yet
	// The HTTP gateway endpoint doesn't accept tool definitions
	return false
}

func (p *GatewayProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	// Determine provider from model or use default
	provider := "deepseek" // default
	model := req.Model
	
	// Try to extract provider from model string
	if strings.Contains(model, "/") {
		parts := strings.SplitN(model, "/", 2)
		if len(parts) == 2 {
			provider = parts[0]
			model = parts[1]
		}
	}
	
	// Get the last user message (simplified for now)
	var lastUserMessage string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUserMessage = req.Messages[i].Content
			break
		}
	}
	
	if lastUserMessage == "" && len(req.Messages) > 0 {
		lastUserMessage = req.Messages[len(req.Messages)-1].Content
	}
	
	// Prepare form data
	formData := url.Values{}
	formData.Set("message", lastUserMessage)
	formData.Set("provider", provider)
	if model != "" && model != "default" {
		formData.Set("model", model)
	}
	
	// Create HTTP request
	client := &http.Client{Timeout: p.timeout}
	gatewayURL := fmt.Sprintf("%s/chat", p.baseURL)
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", gatewayURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w", err)
	}
	
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
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
	var gatewayResp struct {
		Response string `json:"response"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	
	if err := json.Unmarshal(body, &gatewayResp); err != nil {
		return nil, fmt.Errorf("parse JSON response: %w", err)
	}
	
	// Check for tool calls in the response
	// For now, we'll just return the text response
	// In a real implementation, we'd parse tool calls from the response
	
	return &ai.ChatResponse{
		Content:      gatewayResp.Response,
		FinishReason: "stop",
	}, nil
}

// GatewayProviderWithTools extends GatewayProvider to handle tool calls
type GatewayProviderWithTools struct {
	*GatewayProvider
}

// NewGatewayProviderWithTools creates a gateway provider that can handle tool calls
func NewGatewayProviderWithTools(baseURL string) *GatewayProviderWithTools {
	return &GatewayProviderWithTools{
		GatewayProvider: NewGatewayProvider(baseURL),
	}
}

func (p *GatewayProviderWithTools) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	// For tool-enabled requests, we need to send the full conversation
	// This is a simplified version - in production, we'd need to handle
	// the full tool calling protocol through the gateway
	
	if len(req.Tools) == 0 {
		// No tools, use simple chat
		return p.GatewayProvider.Chat(ctx, req)
	}
	
	// For now, we'll use a simplified approach
	// In a real implementation, we'd need to:
	// 1. Send the full conversation with tools to the gateway
	// 2. Parse tool calls from the response
	// 3. Handle tool execution and follow-up
	
	// For simplicity, we'll just use the basic chat for now
	return p.GatewayProvider.Chat(ctx, req)
}