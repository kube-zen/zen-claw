package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/ai"
)

// AICaller interface for making AI calls
// This abstracts away whether we call AI directly or through gateway
type AICaller interface {
	Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error)
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolCallID string
	Content    string
	IsError    bool
}

// Agent is a minimal agent focused only on tool execution
// Uses AICaller interface for AI communication
type Agent struct {
	aiCaller AICaller
	tools    map[string]Tool
	maxSteps int
	currentModel string
}

// NewAgent creates a new agent
func NewAgent(aiCaller AICaller, tools []Tool, maxSteps int) *Agent {
	// Convert tools to map
	toolMap := make(map[string]Tool)
	for _, tool := range tools {
		toolMap[tool.Name()] = tool
	}

	return &Agent{
		aiCaller: aiCaller,
		tools:    toolMap,
		maxSteps: maxSteps,
		currentModel: "deepseek-chat", // Default model
	}
}

// Run executes a task with the given session
// Returns updated session and final result
func (a *Agent) Run(ctx context.Context, session *Session, userInput string) (*Session, string, error) {
	log.Printf("[Agent] Running: %s", userInput)
	
	// Handle model switching commands
	if userInput == "/models" {
		availableModels := []string{
			"deepseek-chat",
			"qwen3-coder-30b-a3b-instruct",
			"qwen-plus",
			"qwen-max",
			"gpt-4o",
			"gpt-4-turbo",
			"glm-4",
			"glm-3-turbo",
			"abab6.5s",
			"abab6.5",
		}
		
		var sb strings.Builder
		sb.WriteString("Available models:\n")
		for _, model := range availableModels {
			if model == a.currentModel {
				sb.WriteString(fmt.Sprintf("  ✓ %s (current)\n", model))
			} else {
				sb.WriteString(fmt.Sprintf("  ○ %s\n", model))
			}
		}
		sb.WriteString("\nUse '/model <model-name>' to switch models")
		return session, sb.String(), nil
	}
	
	if strings.HasPrefix(userInput, "/model ") {
		model := strings.TrimSpace(strings.TrimPrefix(userInput, "/model "))
		a.currentModel = model
		return session, fmt.Sprintf("Model switched to: %s", model), nil
	}
	
	// Add user message to session
	session.AddMessage(ai.Message{
		Role:    "user",
		Content: userInput,
	})

	// Execute agent loop
	for step := 0; step < a.maxSteps; step++ {
		log.Printf("[Agent] Step %d", step+1)
		
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

		log.Printf("[Agent] Executing %d tool calls", len(resp.ToolCalls))
		
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

		log.Printf("[Agent] Added %d tool results, continuing...", len(toolResults))
		
		// Check if we should stop early (e.g., task completed)
		if a.shouldStopEarly(resp.Content, toolResults) {
			log.Printf("[Agent] Early stop condition met at step %d", step+1)
			// Get final response
			finalResp, err := a.getAIResponse(ctx, session)
			if err != nil {
				return session, "", fmt.Errorf("final AI response failed: %w", err)
			}
			session.AddMessage(ai.Message{
				Role:    "assistant",
				Content: finalResp.Content,
			})
			return session, finalResp.Content, nil
		}
	}

	return session, "", fmt.Errorf("exceeded maximum steps (%d)", a.maxSteps)
}

// getAIResponse gets a response from the AI caller with session messages
func (a *Agent) getAIResponse(ctx context.Context, session *Session) (*ai.ChatResponse, error) {
	// Get current messages
	allMessages := session.GetMessages()
	
	// Limit context window to last 20 messages to avoid token overflow
	// Keep system messages and recent conversation
	var messages []ai.Message
	systemMessages := 0
	
	for _, msg := range allMessages {
		if msg.Role == "system" {
			messages = append(messages, msg)
			systemMessages++
		}
	}
	
	// Add recent messages (last 20 - system messages)
	recentCount := 20 - systemMessages
	if recentCount > 0 && len(allMessages) > recentCount {
		messages = append(messages, allMessages[len(allMessages)-recentCount:]...)
	} else {
		messages = allMessages
	}
	
	// Convert tools to AI tool definitions
	toolDefs := a.getToolDefinitions()
	
	// Create chat request
	req := ai.ChatRequest{
		Model:       a.currentModel, // Use current model
		Messages:    messages,
		Tools:       toolDefs,
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	// Get response with timeout
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	return a.aiCaller.Chat(ctx, req)
}

// executeToolCalls executes all tool calls and returns results
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

// shouldStopEarly determines if we should stop execution early
func (a *Agent) shouldStopEarly(lastAssistantMessage string, toolResults []ToolResult) bool {
	// Check if assistant message indicates completion
	lowerMsg := strings.ToLower(lastAssistantMessage)
	completionIndicators := []string{
		"completed",
		"finished",
		"done",
		"conclusion",
		"summary",
		"final",
		"result:",
		"answer:",
		"recommendation:",
		"analysis complete",
		"task complete",
	}
	
	for _, indicator := range completionIndicators {
		if strings.Contains(lowerMsg, indicator) {
			return true
		}
	}
	
	// Check if tool results indicate we have enough information
	if len(toolResults) > 0 {
		// If we got file contents or directory listings, we might have enough
		for _, result := range toolResults {
			if !result.IsError && len(result.Content) > 100 {
				// Got substantial non-error result
				return false // Continue to process
			}
		}
	}
	
	return false
}