package agentlib

import (
	"context"

	"github.com/neves/zen-claw/internal/ai"
)

// Transport abstracts AI provider communication
// Similar to pi-agent's AgentTransport interface
type Transport interface {
	// Run executes an agent turn with the given messages and configuration
	// Returns a channel of events for real-time updates
	Run(ctx context.Context, messages []ai.Message, config RunConfig) (<-chan TransportEvent, error)
}

// RunConfig contains configuration for running an agent turn
type RunConfig struct {
	SystemPrompt string
	Tools        []ai.ToolDefinition
	Model        string
	Temperature  float64
	MaxTokens    int
}

// TransportEvent represents events from the transport layer
type TransportEvent struct {
	Type TransportEventType
	Data interface{}
}

// TransportEventType represents the type of transport event
type TransportEventType string

const (
	// TransportEventMessageStart indicates message generation started
	TransportEventMessageStart TransportEventType = "message_start"
	
	// TransportEventMessageUpdate indicates message content updated
	TransportEventMessageUpdate TransportEventType = "message_update"
	
	// TransportEventMessageEnd indicates message generation ended
	TransportEventMessageEnd TransportEventType = "message_end"
	
	// TransportEventToolCall indicates a tool call was made
	TransportEventToolCall TransportEventType = "tool_call"
	
	// TransportEventDone indicates the turn is complete
	TransportEventDone TransportEventType = "done"
	
	// TransportEventError indicates an error occurred
	TransportEventError TransportEventType = "error"
)

// TransportMessageStartEventData contains data for message_start event
type TransportMessageStartEventData struct {
	Message ai.Message
}

// TransportMessageUpdateEventData contains data for message_update event
type TransportMessageUpdateEventData struct {
	Message ai.Message
}

// TransportMessageEndEventData contains data for message_end event
type TransportMessageEndEventData struct {
	Message ai.Message
}

// TransportToolCallEventData contains data for tool_call event
type TransportToolCallEventData struct {
	ToolCalls []ai.ToolCall
}

// TransportDoneEventData contains data for done event
type TransportDoneEventData struct {
	Message ai.Message
}

// TransportErrorEventData contains data for error event
type TransportErrorEventData struct {
	Error error
}

// ProviderTransport implements Transport using our existing AI providers
type ProviderTransport struct {
	provider ai.Provider
}

// NewProviderTransport creates a new provider transport
func NewProviderTransport(provider ai.Provider) *ProviderTransport {
	return &ProviderTransport{
		provider: provider,
	}
}

// Run executes an agent turn using the AI provider
func (t *ProviderTransport) Run(ctx context.Context, messages []ai.Message, config RunConfig) (<-chan TransportEvent, error) {
	eventChan := make(chan TransportEvent, 10)
	
	go func() {
		defer close(eventChan)
		
		// Emit message start
		eventChan <- TransportEvent{
			Type: TransportEventMessageStart,
			Data: TransportMessageStartEventData{
				Message: ai.Message{
					Role: "assistant",
				},
			},
		}
		
		// Create chat request
		req := ai.ChatRequest{
			Model:       config.Model,
			Messages:    messages,
			Tools:       config.Tools,
			Temperature: config.Temperature,
			MaxTokens:   config.MaxTokens,
			// System field doesn't exist in ai.ChatRequest, need to add as message
		}
		
		// Add system prompt as first message if provided
		if config.SystemPrompt != "" {
			messages = append([]ai.Message{
				{Role: "system", Content: config.SystemPrompt},
			}, messages...)
			req.Messages = messages
		}
		
		// Get response
		resp, err := t.provider.Chat(ctx, req)
		if err != nil {
			eventChan <- TransportEvent{
				Type: TransportEventError,
				Data: TransportErrorEventData{Error: err},
			}
			return
		}
		
		// Emit tool calls if any
		if len(resp.ToolCalls) > 0 {
			eventChan <- TransportEvent{
				Type: TransportEventToolCall,
				Data: TransportToolCallEventData{
					ToolCalls: resp.ToolCalls,
				},
			}
		}
		
		// Create final message
		finalMessage := ai.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		
		// Emit message end
		eventChan <- TransportEvent{
			Type: TransportEventMessageEnd,
			Data: TransportMessageEndEventData{
				Message: finalMessage,
			},
		}
		
		// Emit done
		eventChan <- TransportEvent{
			Type: TransportEventDone,
			Data: TransportDoneEventData{
				Message: finalMessage,
			},
		}
	}()
	
	return eventChan, nil
}