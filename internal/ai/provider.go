package ai

import (
	"context"
	"fmt"
)

// Provider defines the interface for AI model providers
type Provider interface {
	// Name returns the provider name (e.g., "openai", "anthropic", "deepseek")
	Name() string
	
	// Chat sends a chat completion request
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	
	// SupportsTools returns true if the provider supports tool calls
	SupportsTools() bool
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model    string
	Messages []Message
	Tools    []ToolDefinition
	Thinking bool
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	Content    string
	ToolCalls  []ToolCall
	FinishReason string
}

// Message represents a chat message
type Message struct {
	Role    string // "system", "user", "assistant", "tool"
	Content string
}

// ToolDefinition defines a tool that can be called by the AI
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
}

// ToolCall represents a tool invocation request from the AI
type ToolCall struct {
	ID   string
	Name string
	Args map[string]interface{}
}

// Registry manages available AI providers
type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

func (r *Registry) Register(provider Provider) {
	r.providers[provider.Name()] = provider
}

func (r *Registry) Get(name string) (Provider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return provider, nil
}

func (r *Registry) List() []string {
	var names []string
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// MockProvider is a simple mock for testing
type MockProvider struct{}

func (p *MockProvider) Name() string { return "mock" }
func (p *MockProvider) SupportsTools() bool { return true }
func (p *MockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Simple echo response for testing
	return &ChatResponse{
		Content:     fmt.Sprintf("Mock response to: %v", req.Messages),
		FinishReason: "stop",
	}, nil
}