package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/ai"
)

const (
	AnthropicAPIURL      = "https://api.anthropic.com/v1/messages"
	AnthropicAPIVersion  = "2023-06-01"
	AnthropicBetaVersion = "prompt-caching-2024-07-31"
)

// AnthropicProvider implements the Anthropic Claude API with prompt caching
type AnthropicProvider struct {
	apiKey         string
	model          string
	client         *http.Client
	cacheRetention string // "none", "short" (5m), "long" (1h)
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey, model, cacheRetention string) (*AnthropicProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key required")
	}
	if model == "" {
		model = "claude-sonnet-4-20250514" // Claude Sonnet 4
	}
	if cacheRetention == "" {
		cacheRetention = "short"
	}

	return &AnthropicProvider{
		apiKey:         apiKey,
		model:          model,
		cacheRetention: cacheRetention,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}, nil
}

// Name returns the provider name
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// SupportsTools returns true since Anthropic supports tool use
func (p *AnthropicProvider) SupportsTools() bool {
	return true
}

// Chat implements ai.Provider
func (p *AnthropicProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	anthropicReq := p.buildRequest(req)

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", AnthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", AnthropicAPIVersion)
	httpReq.Header.Set("anthropic-beta", AnthropicBetaVersion)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return p.convertResponse(anthropicResp), nil
}

// ChatStream implements ai.Provider with streaming
func (p *AnthropicProvider) ChatStream(ctx context.Context, req ai.ChatRequest, callback ai.StreamCallback) (*ai.ChatResponse, error) {
	anthropicReq := p.buildRequest(req)
	anthropicReq.Stream = true

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", AnthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", AnthropicAPIVersion)
	httpReq.Header.Set("anthropic-beta", AnthropicBetaVersion)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Process SSE stream
	var fullContent strings.Builder
	var toolCalls []ai.ToolCall
	var stopReason string

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
			ContentBlock struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content_block"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		text := ""
		if event.Delta.Text != "" {
			text = event.Delta.Text
		} else if event.ContentBlock.Text != "" {
			text = event.ContentBlock.Text
		}

		if text != "" {
			fullContent.WriteString(text)
			if callback != nil {
				callback(text)
			}
		}
	}

	return &ai.ChatResponse{
		Content:      fullContent.String(),
		ToolCalls:    toolCalls,
		FinishReason: stopReason,
	}, nil
}

// Request/response types
type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Messages    []anthropicMessage `json:"messages"`
	System      []anthropicContent `json:"system,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

type cacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type  string                 `json:"type"`
		Text  string                 `json:"text,omitempty"`
		ID    string                 `json:"id,omitempty"`
		Name  string                 `json:"name,omitempty"`
		Input map[string]interface{} `json:"input,omitempty"`
	} `json:"content"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens        int `json:"input_tokens"`
		OutputTokens       int `json:"output_tokens"`
		CacheCreationInput int `json:"cache_creation_input_tokens,omitempty"`
		CacheReadInput     int `json:"cache_read_input_tokens,omitempty"`
	} `json:"usage"`
}

func (p *AnthropicProvider) buildRequest(req ai.ChatRequest) anthropicRequest {
	anthropicReq := anthropicRequest{
		Model:     p.model,
		MaxTokens: req.MaxTokens,
	}

	if anthropicReq.MaxTokens == 0 {
		anthropicReq.MaxTokens = 4096
	}

	if req.Temperature > 0 {
		anthropicReq.Temperature = req.Temperature
	}

	// Convert messages
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			content := anthropicContent{
				Type: "text",
				Text: msg.Content,
			}
			// Enable caching for system prompt (saves up to 90%!)
			if p.cacheRetention != "none" {
				content.CacheControl = &cacheControl{Type: "ephemeral"}
			}
			anthropicReq.System = append(anthropicReq.System, content)

		case "user", "assistant":
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
				Role: msg.Role,
				Content: []anthropicContent{{
					Type: "text",
					Text: msg.Content,
				}},
			})

		case "tool":
			// Tool results in Anthropic format
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
				Role: "user",
				Content: []anthropicContent{{
					Type: "tool_result",
					Text: msg.Content,
				}},
			})
		}
	}

	// Convert tools
	for _, tool := range req.Tools {
		anthropicReq.Tools = append(anthropicReq.Tools, anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.Parameters,
		})
	}

	return anthropicReq
}

func (p *AnthropicProvider) convertResponse(resp anthropicResponse) *ai.ChatResponse {
	var content string
	var toolCalls []ai.ToolCall

	for _, c := range resp.Content {
		switch c.Type {
		case "text":
			content += c.Text
		case "tool_use":
			toolCalls = append(toolCalls, ai.ToolCall{
				ID:   c.ID,
				Name: c.Name,
				Args: c.Input,
			})
		}
	}

	return &ai.ChatResponse{
		Content:      content,
		ToolCalls:    toolCalls,
		FinishReason: resp.StopReason,
	}
}
