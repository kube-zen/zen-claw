package ai

import "context"

// Message represents a single message in the conversation
type Message struct {
	Role       string                 `json:"role"`
	Content    string                 `json:"content"`
	ToolCalls  []ToolCall             `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCall represents a tool call in a message
type ToolCall struct {
	ID   string                 `json:"id"`
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// Tool represents a function tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ChatRequest represents a chat request to an AI provider
type ChatRequest struct {
	Messages              []Message              `json:"messages"`
	Tools                 []Tool                 `json:"tools,omitempty"`
	Model                 string                 `json:"model,omitempty"`
	Temperature           float64                `json:"temperature,omitempty"`
	MaxTokens             int                    `json:"max_tokens,omitempty"`
	ContextLimit          int                    `json:"context_limit,omitempty"`
	Thinking              bool                   `json:"thinking,omitempty"`
	QwenLargeContextEnabled bool               `json:"qwen_large_context_enabled,omitempty"`
}

// ChatResponse represents a chat response from an AI provider
type ChatResponse struct {
	Content      string      `json:"content"`
	FinishReason string      `json:"finish_reason"`
	ToolCalls    []ToolCall  `json:"tool_calls,omitempty"`
}

// Provider interface that all AI providers must implement
type Provider interface {
	Name() string
	SupportsTools() bool
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
