package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/neves/zen-claw/internal/ai"
)

type AIAgent struct {
	provider ai.Provider
	tools    *ToolManager
}

type ToolManager struct {
	tools map[string]Tool
}

type Tool interface {
	Name() string
	Description() string
	Execute(args map[string]interface{}) (string, error)
}

func NewAIAgent(provider ai.Provider) *AIAgent {
	return &AIAgent{
		provider: provider,
		tools:    &ToolManager{tools: make(map[string]Tool)},
	}
}

func (a *AIAgent) RegisterTool(tool Tool) {
	a.tools.tools[tool.Name()] = tool
}

func (a *AIAgent) Process(ctx context.Context, input string) (string, error) {
	// Convert tools to AI tool definitions
	var toolDefs []ai.ToolDefinition
	for _, tool := range a.tools.tools {
		toolDefs = append(toolDefs, ai.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					// Tool-specific parameters would go here
				},
			},
		})
	}

	// Create chat request
	req := ai.ChatRequest{
		Model: "default",
		Messages: []ai.Message{
			{Role: "user", Content: input},
		},
		Tools: toolDefs,
	}

	// Get AI response
	resp, err := a.provider.Chat(ctx, req)
	if err != nil {
		return "", fmt.Errorf("AI chat failed: %w", err)
	}

	// Handle tool calls if any
	if len(resp.ToolCalls) > 0 {
		return a.handleToolCalls(ctx, resp.ToolCalls)
	}

	return resp.Content, nil
}

func (a *AIAgent) handleToolCalls(ctx context.Context, toolCalls []ai.ToolCall) (string, error) {
	var results []string
	
	for _, call := range toolCalls {
		tool, ok := a.tools.tools[call.Name]
		if !ok {
			results = append(results, fmt.Sprintf("Tool not found: %s", call.Name))
			continue
		}

		result, err := tool.Execute(call.Args)
		if err != nil {
			results = append(results, fmt.Sprintf("Tool %s failed: %v", call.Name, err))
			continue
		}

		results = append(results, fmt.Sprintf("Tool %s: %s", call.Name, result))
	}

	return strings.Join(results, "\n"), nil
}

// Example tool implementations
type EchoTool struct{}

func (t *EchoTool) Name() string        { return "echo" }
func (t *EchoTool) Description() string { return "Echo back the input" }
func (t *EchoTool) Execute(args map[string]interface{}) (string, error) {
	text, _ := args["text"].(string)
	return fmt.Sprintf("Echo: %s", text), nil
}

type MathTool struct{}

func (t *MathTool) Name() string        { return "add" }
func (t *MathTool) Description() string { return "Add two numbers" }
func (t *MathTool) Execute(args map[string]interface{}) (string, error) {
	a, _ := args["a"].(float64)
	b, _ := args["b"].(float64)
	return fmt.Sprintf("%.2f", a+b), nil
}