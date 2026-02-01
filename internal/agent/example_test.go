package agent_test

import (
	"context"
	"log"
	"strings"
	"testing"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/ai"
)

// MockProvider for testing
type MockProvider struct{}

func (p *MockProvider) Name() string { return "mock" }
func (p *MockProvider) SupportsTools() bool { return true }

func (p *MockProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	// Simple mock that returns tool calls based on user input
	lastMessage := ""
	if len(req.Messages) > 0 {
		lastMessage = req.Messages[len(req.Messages)-1].Content
	}
	
	// Check if we have tool results (tool role messages)
	hasToolResults := false
	for _, msg := range req.Messages {
		if msg.Role == "tool" {
			hasToolResults = true
			break
		}
	}
	
	if hasToolResults {
		// After tool results, return final analysis
		return &ai.ChatResponse{
			Content: "Based on the files I read, this is a Go project with go.mod. Suggestions: add tests, improve error handling.",
			FinishReason: "stop",
		}, nil
	}
	
	// First response: make tool calls
	if strings.Contains(lastMessage, "check codebase") {
		return &ai.ChatResponse{
			Content: "I'll check the codebase structure and read key files.",
			ToolCalls: []ai.ToolCall{
				{
					ID:   "1",
					Name: "list_dir",
					Args: map[string]interface{}{"path": "."},
				},
				{
					ID:   "2", 
					Name: "read_file",
					Args: map[string]interface{}{"path": "go.mod"},
				},
				{
					ID:   "3",
					Name: "read_file",
					Args: map[string]interface{}{"path": "README.md"},
				},
			},
			FinishReason: "tool_calls",
		}, nil
	}
	
	// Default response
	return &ai.ChatResponse{
		Content: "I can help you with that. What would you like me to do?",
		FinishReason: "stop",
	}, nil
}

func TestAgentBasic(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// Create mock provider
	provider := &MockProvider{}
	
	// Create tools
	tools := []agent.Tool{
		agent.NewListDirTool("."),
		agent.NewReadFileTool("."),
		agent.NewExecTool("."),
		agent.NewSystemInfoTool(),
	}
	
	// Create agent
	agent := agent.NewAgent(agent.Config{
		Provider:   provider,
		Tools:      tools,
		WorkingDir: ".",
		SessionID:  "test_session",
		MaxSteps:   5,
	})
	
	// Run agent
	ctx := context.Background()
	result, err := agent.Run(ctx, "check codebase in current directory and suggest improvements")
	
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}
	
	log.Printf("Agent result: %s", result)
	
	// Check session stats
	stats := agent.GetSession().GetStats()
	log.Printf("Session stats: %+v", stats)
	
	// Should have executed tool calls and gotten final response
	if stats.ToolMessages == 0 {
		t.Error("Expected tool messages but got none")
	}
	
	if !strings.Contains(result, "Go project") && !strings.Contains(result, "suggestions") {
		t.Error("Expected analysis in final response")
	}
}