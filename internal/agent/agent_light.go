package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/neves/zen-claw/internal/ai"
)

// AICaller interface for making AI calls
// This abstracts away whether we call AI directly or through gateway
type AICaller interface {
	Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error)
}

// LightAgent is a minimal agent focused only on tool execution
// Uses AICaller interface for AI communication
type LightAgent struct {
	aiCaller AICaller
	tools    map[string]Tool
	maxSteps int
}

// NewLightAgent creates a new lightweight agent
func NewLightAgent(aiCaller AICaller, tools []Tool, maxSteps int) *LightAgent {
	// Convert tools to map
	toolMap := make(map[string]Tool)
	for _, tool := range tools {
		toolMap[tool.Name()] = tool
	}

	return &LightAgent{
		aiCaller: aiCaller,
		tools:    toolMap,
		maxSteps: maxSteps,
	}
}

// Run executes a task with the given session
// Returns updated session and final result
func (a *LightAgent) Run(ctx context.Context, session *Session, userInput string) (*Session, string, error) {
	log.Printf("[LightAgent] Running: %s", userInput)
	
	// Add user message to session
	session.AddMessage(ai.Message{
		Role:    "user",
		Content: userInput,
	})

	// Execute agent loop
	for step := 0; step < a.maxSteps; step++ {
		log.Printf("[LightAgent] Step %d", step+1)
		
		// Get AI response
		resp, err := a.getAIResponse(ctx, session)
		if err != nil {
			return session, "", fmt.Errorf("AI response failed: %w", err)
		}

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 {
			session.AddMessage(ai.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
			return session, resp.Content, nil
		}

		log.Printf("[LightAgent] Executing %d tool calls", len(resp.ToolCalls))
		
		// Execute all tool calls
		toolResults, err := a.executeToolCalls(ctx, resp.ToolCalls)
		if err != nil {
			return session, "", fmt.Errorf("tool execution failed: %w", err)
		}

		// Add assistant message with tool calls
		session.AddMessage(ai.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Add tool results to session
		for _, result := range toolResults {
			session.AddMessage(ai.Message{
				Role:       "tool",
				Content:    result.Content,
				ToolCallID: result.ToolCallID,
			})
		}

		log.Printf("[LightAgent] Added %d tool results, continuing...", len(toolResults))
	}

	return session, "", fmt.Errorf("exceeded maximum steps (%d)", a.maxSteps)
}

// getAIResponse gets a response from the AI caller with session messages
func (a *LightAgent) getAIResponse(ctx context.Context, session *Session) (*ai.ChatResponse, error) {
	// Get current messages
	messages := session.GetMessages()
	
	// Convert tools to AI tool definitions
	toolDefs := a.getToolDefinitions()
	
	// Create chat request
	req := ai.ChatRequest{
		Model:       "deepseek-chat", // TODO: Make configurable via AICaller
		Messages:    messages,
		Tools:       toolDefs,
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	// Get response with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return a.aiCaller.Chat(ctx, req)
}

// executeToolCalls executes all tool calls and returns results
func (a *LightAgent) executeToolCalls(ctx context.Context, toolCalls []ai.ToolCall) ([]ToolResult, error) {
	var results []ToolResult

	for _, call := range toolCalls {
		log.Printf("[LightAgent] Executing tool: %s", call.Name)
		
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
		
		log.Printf("[LightAgent] Tool %s completed", call.Name)
	}

	return results, nil
}

// getToolDefinitions converts tools to AI tool definitions
func (a *LightAgent) getToolDefinitions() []ai.ToolDefinition {
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