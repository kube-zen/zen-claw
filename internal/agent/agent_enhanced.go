package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/neves/zen-claw/internal/ai"
)

// EnhancedAgent adds event system and better error handling to Agent
type EnhancedAgent struct {
	*Agent
	eventEmitter *EventEmitter
}

// NewEnhancedAgent creates a new enhanced agent
func NewEnhancedAgent(cfg Config) *EnhancedAgent {
	baseAgent := NewAgent(cfg)
	
	return &EnhancedAgent{
		Agent:        baseAgent,
		eventEmitter: NewEventEmitter(),
	}
}

// Run executes a user request with event emission
func (a *EnhancedAgent) Run(ctx context.Context, userInput string) (string, error) {
	// Emit agent start event
	a.eventEmitter.Emit(newAgentStartEvent(a.session.ID, userInput))
	
	// Add user message to session
	a.session.AddMessage(ai.Message{
		Role:    "user",
		Content: userInput,
	})

	// Execute agent loop with events
	for step := 0; step < a.maxSteps; step++ {
		// Emit turn start event
		a.eventEmitter.Emit(newTurnStartEvent(step + 1))
		
		// Get AI response
		resp, err := a.getAIResponse(ctx)
		if err != nil {
			a.eventEmitter.Emit(newErrorEvent(err))
			return "", fmt.Errorf("AI response failed: %w", err)
		}

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 {
			a.session.AddMessage(ai.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
			
			// Emit turn end and agent end events
			a.eventEmitter.Emit(newTurnEndEvent(step+1, ai.Message{
				Role:    "assistant",
				Content: resp.Content,
			}, nil))
			
			a.eventEmitter.Emit(newAgentEndEvent(a.session.ID, resp.Content, a.session.GetMessages()))
			return resp.Content, nil
		}

		// Execute all tool calls with events
		toolResults, err := a.executeToolCallsWithEvents(ctx, resp.ToolCalls)
		if err != nil {
			a.eventEmitter.Emit(newErrorEvent(err))
			return "", fmt.Errorf("tool execution failed: %w", err)
		}

		// Add assistant message with tool calls
		assistantMsg := ai.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		a.session.AddMessage(assistantMsg)

		// Add tool results to session
		var toolResultMessages []ai.Message
		for _, result := range toolResults {
			msg := ai.Message{
				Role:       "tool",
				Content:    result.Content,
				ToolCallID: result.ToolCallID,
			}
			a.session.AddMessage(msg)
			toolResultMessages = append(toolResultMessages, msg)
		}

		// Emit turn end event
		a.eventEmitter.Emit(newTurnEndEvent(step+1, assistantMsg, toolResultMessages))
	}

	// If we get here, we exceeded max steps
	err := fmt.Errorf("exceeded maximum steps (%d)", a.maxSteps)
	a.eventEmitter.Emit(newErrorEvent(err))
	return "", err
}

// executeToolCallsWithEvents executes tool calls with event emission
func (a *EnhancedAgent) executeToolCallsWithEvents(ctx context.Context, toolCalls []ai.ToolCall) ([]ToolResult, error) {
	var results []ToolResult

	for _, call := range toolCalls {
		// Emit tool start event
		a.eventEmitter.Emit(newToolStartEvent(call.ID, call.Name, call.Args))
		
		startTime := time.Now()
		
		tool, exists := a.tools[call.Name]
		if !exists {
			result := ToolResult{
				ToolCallID: call.ID,
				Content:    fmt.Sprintf("Error: Tool '%s' not found", call.Name),
				IsError:    true,
			}
			results = append(results, result)
			
			// Emit tool end event with error
			a.eventEmitter.Emit(newToolEndEvent(
				call.ID, call.Name, result.Content, true, time.Since(startTime),
			))
			continue
		}

		// Execute tool
		execResult, err := tool.Execute(ctx, call.Args)
		if err != nil {
			result := ToolResult{
				ToolCallID: call.ID,
				Content:    fmt.Sprintf("Error executing %s: %v", call.Name, err),
				IsError:    true,
			}
			results = append(results, result)
			
			// Emit tool end event with error
			a.eventEmitter.Emit(newToolEndEvent(
				call.ID, call.Name, result.Content, true, time.Since(startTime),
			))
			continue
		}

		// Convert result to string
		resultStr := fmt.Sprintf("%v", execResult)
		result := ToolResult{
			ToolCallID: call.ID,
			Content:    resultStr,
			IsError:    false,
		}
		results = append(results, result)
		
		// Emit tool end event with success
		a.eventEmitter.Emit(newToolEndEvent(
			call.ID, call.Name, resultStr, false, time.Since(startTime),
		))
	}

	return results, nil
}

// Subscribe adds an event listener
func (a *EnhancedAgent) Subscribe(listener EventListener) func() {
	return a.eventEmitter.Subscribe(listener)
}

// GetEventEmitter returns the event emitter
func (a *EnhancedAgent) GetEventEmitter() *EventEmitter {
	return a.eventEmitter
}