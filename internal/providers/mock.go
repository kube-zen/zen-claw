package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/neves/zen-claw/internal/ai"
)

// MockProvider simulates tool calls for testing
type MockProvider struct {
	callTools bool // Whether to simulate tool calls
}

func NewMockProvider(callTools bool) *MockProvider {
	return &MockProvider{callTools: callTools}
}

func (p *MockProvider) Name() string {
	return "mock"
}

func (p *MockProvider) SupportsTools() bool {
	return p.callTools
}

func (p *MockProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	// Check if user is asking to read a file
	lastMessage := req.Messages[len(req.Messages)-1].Content
	lower := strings.ToLower(lastMessage)

	if p.callTools && strings.Contains(lower, "read") && strings.Contains(lower, "test.txt") {
		// Simulate a tool call to read
		return &ai.ChatResponse{
			ToolCalls: []ai.ToolCall{
				{
					ID:   "call_123",
					Name: "read",
					Args: map[string]interface{}{
						"path": "test.txt",
					},
				},
			},
			FinishReason: "tool_calls",
		}, nil
	}

	if p.callTools && strings.Contains(lower, "write") {
		// Simulate a tool call to write
		return &ai.ChatResponse{
			ToolCalls: []ai.ToolCall{
				{
					ID:   "call_456",
					Name: "write",
					Args: map[string]interface{}{
						"path":    "output.txt",
						"content": "This was written by the AI",
					},
				},
			},
			FinishReason: "tool_calls",
		}, nil
	}

	if p.callTools && strings.Contains(lower, "exec") {
		// Simulate a tool call to exec
		return &ai.ChatResponse{
			ToolCalls: []ai.ToolCall{
				{
					ID:   "call_789",
					Name: "exec",
					Args: map[string]interface{}{
						"command": "echo 'Hello from exec'",
					},
				},
			},
			FinishReason: "tool_calls",
		}, nil
	}

	// Default response
	response := fmt.Sprintf("Mock response to: %s", lastMessage)
	if len(req.Tools) > 0 {
		response += fmt.Sprintf("\nI see %d tools available.", len(req.Tools))
	}

	return &ai.ChatResponse{
		Content:      response,
		FinishReason: "stop",
	}, nil
}
