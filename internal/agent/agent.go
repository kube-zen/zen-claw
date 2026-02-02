package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/session"
)

// Agent represents an AI agent that can execute tools and continue conversations
type Agent struct {
	provider   ai.Provider
	tools      map[string]Tool
	session    *session.Session
	workingDir string
	maxSteps   int // Maximum tool execution steps to prevent infinite loops
}

// Config for creating a new Agent
type Config struct {
	Provider   ai.Provider
	Tools      []Tool
	WorkingDir string
	SessionID  string
	MaxSteps   int
}

// NewAgent creates a new agent with the given configuration
func NewAgent(cfg Config) *Agent {
	if cfg.MaxSteps == 0 {
		cfg.MaxSteps = 10 // Default max steps
	}

	// Convert tools to map
	tools := make(map[string]Tool)
	for _, tool := range cfg.Tools {
		tools[tool.Name()] = tool
	}

	// Create session
	sess := session.NewSession(cfg.SessionID)
	if cfg.WorkingDir != "" {
		sess.SetWorkingDir(cfg.WorkingDir)
	}

	return &Agent{
		provider:   cfg.Provider,
		tools:      tools,
		session:    sess,
		workingDir: cfg.WorkingDir,
		maxSteps:   cfg.MaxSteps,
	}
}

// Run executes a user request with automatic tool chaining and conversation continuation
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	log.Printf("[Agent] Running: %s", userInput)
	
	// Add user message to session
	a.session.AddMessage(session.Message{
		Role:    "user",
		Content: userInput,
		Time:    time.Now(),
	})

	// Execute agent loop
	for step := 0; step < a.maxSteps; step++ {
		log.Printf("[Agent] Step %d", step+1)
		
		// Get AI response
		resp, err := a.getAIResponse(ctx)
		if err != nil {
			return "", fmt.Errorf("AI response failed on step %d: %w", step+1, err)
		}

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 {
			log.Printf("[Agent] No tool calls, returning final response")
			a.session.AddMessage(session.Message{
				Role:    "assistant",
				Content: resp.Content,
				Time:    time.Now(),
			})
			return resp.Content, nil
		}

		log.Printf("[Agent] Executing %d tool calls", len(resp.ToolCalls))
		
		// Execute all tool calls
		toolResults, err := a.executeToolCalls(ctx, resp.ToolCalls)
		if err != nil {
			return "", fmt.Errorf("tool execution failed on step %d: %w", step+1, err)
		}

		// Add assistant message with tool calls (for conversation history)
		a.session.AddMessage(session.Message{
			Role:    "assistant",
			Content: resp.Content,
			Time:    time.Now(),
		})

		// Add tool results to session
		for _, result := range toolResults {
			a.session.AddMessage(session.Message{
				Role:    "tool",
				Content: result.Content,
				Time:    time.Now(),
			})
		}

		log.Printf("[Agent] Added %d tool results, continuing...", len(toolResults))
	}

	return "", fmt.Errorf("exceeded maximum steps (%d)", a.maxSteps)
}

// getAIResponse gets a response from the AI provider with current session messages
func (a *Agent) getAIResponse(ctx context.Context) (*ai.ChatResponse, error) {
	// Get current messages
	messages := a.session.GetMessages()
	
	// Convert tools to AI tool definitions
	toolDefs := a.getToolDefinitions()
	
	// Create chat request
	req := ai.ChatRequest{
		Model:       a.provider.Name() + "/default", // Use provider name as default model
		Messages:    messages,
		Tools:       toolDefs,
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	// Get response with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, err := a.provider.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("AI provider %s failed: %w", a.provider.Name(), err)
	}

	return response, nil
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolCallID string
	Content    string
	IsError    bool
}

// executeToolCalls executes all tool calls and returns results with tool call IDs
func (a *Agent) executeToolCalls(ctx context.Context, toolCalls []ai.ToolCall) ([]ToolResult, error) {
	var results []ToolResult

	for _, call := range toolCalls {
		log.Printf("[Agent] Executing tool: %s", call.Name)
		
		tool, exists := a.tools[call.Name]
		if !exists {
			results = append(results, ToolResult{
				ToolCallID: call.ID,
				Content:    fmt.Sprintf("Error: Tool '%s' not found", call.Name),
				IsError:    true,
			})
			continue
		}

		// Execute tool
		result, err := tool.Execute(ctx, call.Args)
		if err != nil {
			results = append(results, ToolResult{
				ToolCallID: call.ID,
				Content:    fmt.Sprintf("Error executing %s: %v", call.Name, err),
				IsError:    true,
			})
			continue
		}

		// Convert result to string
		resultStr := fmt.Sprintf("%v", result)
		results = append(results, ToolResult{
			ToolCallID: call.ID,
			Content:    resultStr,
			IsError:    false,
		})
		
		log.Printf("[Agent] Tool %s completed", call.Name)
	}

	return results, nil
}

// getToolDefinitions converts tools to AI tool definitions
func (a *Agent) getToolDefinitions() []ai.ToolDefinition {
	var defs []ai.ToolDefinition
	
	for _, tool := range a.tools {
		defs = append(defs, ai.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		})
	}
	
	return defs
}

// GetSession returns the agent's session
func (a *Agent) GetSession() *Session {
	return a.session
}

// AddTool adds a tool to the agent
func (a *Agent) AddTool(tool Tool) {
	a.tools[tool.Name()] = tool
}