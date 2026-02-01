package agentlib

import (
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/ai"
)

// EventType represents the type of agent event
type EventType string

const (
	// EventAgentStart is emitted when agent starts processing
	EventAgentStart EventType = "agent_start"
	
	// EventAgentEnd is emitted when agent finishes processing
	EventAgentEnd EventType = "agent_end"
	
	// EventTurnStart is emitted when a new turn starts
	EventTurnStart EventType = "turn_start"
	
	// EventTurnEnd is emitted when a turn ends
	EventTurnEnd EventType = "turn_end"
	
	// EventToolStart is emitted when tool execution starts
	EventToolStart EventType = "tool_execution_start"
	
	// EventToolEnd is emitted when tool execution ends
	EventToolEnd EventType = "tool_execution_end"
	
	// EventMessageStart is emitted when message generation starts
	EventMessageStart EventType = "message_start"
	
	// EventMessageEnd is emitted when message generation ends
	EventMessageEnd EventType = "message_end"
	
	// EventError is emitted when an error occurs
	EventError EventType = "error"
)

// AgentEvent represents an agent event
type AgentEvent struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// AgentStartEventData contains data for agent_start event
type AgentStartEventData struct {
	SessionID string `json:"session_id"`
	Task      string `json:"task"`
}

// AgentEndEventData contains data for agent_end event
type AgentEndEventData struct {
	SessionID string      `json:"session_id"`
	Result    string      `json:"result"`
	Messages  []ai.Message `json:"messages"`
}

// TurnStartEventData contains data for turn_start event
type TurnStartEventData struct {
	TurnNumber int `json:"turn_number"`
}

// TurnEndEventData contains data for turn_end event
type TurnEndEventData struct {
	TurnNumber  int          `json:"turn_number"`
	Message     ai.Message   `json:"message"`
	ToolResults []ai.Message `json:"tool_results"`
}

// ToolStartEventData contains data for tool_execution_start event
type ToolStartEventData struct {
	ToolCallID string                 `json:"tool_call_id"`
	ToolName   string                 `json:"tool_name"`
	Args       map[string]interface{} `json:"args"`
}

// ToolEndEventData contains data for tool_execution_end event
type ToolEndEventData struct {
	ToolCallID string      `json:"tool_call_id"`
	ToolName   string      `json:"tool_name"`
	Result     interface{} `json:"result"`
	IsError    bool        `json:"is_error"`
	DurationMs int64       `json:"duration_ms"`
}

// MessageStartEventData contains data for message_start event
type MessageStartEventData struct {
	Message ai.Message `json:"message"`
}

// MessageEndEventData contains data for message_end event
type MessageEndEventData struct {
	Message ai.Message `json:"message"`
}

// ErrorEventData contains data for error event
type ErrorEventData struct {
	Error string `json:"error"`
}

// EventListener is a function that handles agent events
type EventListener func(event AgentEvent)

// EventEmitter handles event subscription and emission
type EventEmitter struct {
	listeners []EventListener
	mu        sync.RWMutex
}

// NewEventEmitter creates a new event emitter
func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		listeners: make([]EventListener, 0),
	}
}

// Subscribe adds an event listener
func (e *EventEmitter) Subscribe(listener EventListener) func() {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.listeners = append(e.listeners, listener)
	
	// Return unsubscribe function
	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		
		for i, l := range e.listeners {
			if &l == &listener {
				e.listeners = append(e.listeners[:i], e.listeners[i+1:]...)
				break
			}
		}
	}
}

// Emit sends an event to all listeners
func (e *EventEmitter) Emit(event AgentEvent) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	for _, listener := range e.listeners {
		go listener(event)
	}
}

// Helper functions to create events
func newAgentStartEvent(sessionID, task string) AgentEvent {
	return AgentEvent{
		Type:      EventAgentStart,
		Timestamp: time.Now(),
		Data: AgentStartEventData{
			SessionID: sessionID,
			Task:      task,
		},
	}
}

func newAgentEndEvent(sessionID, result string, messages []ai.Message) AgentEvent {
	return AgentEvent{
		Type:      EventAgentEnd,
		Timestamp: time.Now(),
		Data: AgentEndEventData{
			SessionID: sessionID,
			Result:    result,
			Messages:  messages,
		},
	}
}

func newTurnStartEvent(turnNumber int) AgentEvent {
	return AgentEvent{
		Type:      EventTurnStart,
		Timestamp: time.Now(),
		Data: TurnStartEventData{
			TurnNumber: turnNumber,
		},
	}
}

func newTurnEndEvent(turnNumber int, message ai.Message, toolResults []ai.Message) AgentEvent {
	return AgentEvent{
		Type:      EventTurnEnd,
		Timestamp: time.Now(),
		Data: TurnEndEventData{
			TurnNumber:  turnNumber,
			Message:     message,
			ToolResults: toolResults,
		},
	}
}

func newToolStartEvent(toolCallID, toolName string, args map[string]interface{}) AgentEvent {
	return AgentEvent{
		Type:      EventToolStart,
		Timestamp: time.Now(),
		Data: ToolStartEventData{
			ToolCallID: toolCallID,
			ToolName:   toolName,
			Args:       args,
		},
	}
}

func newToolEndEvent(toolCallID, toolName string, result interface{}, isError bool, duration time.Duration) AgentEvent {
	return AgentEvent{
		Type:      EventToolEnd,
		Timestamp: time.Now(),
		Data: ToolEndEventData{
			ToolCallID: toolCallID,
			ToolName:   toolName,
			Result:     result,
			IsError:    isError,
			DurationMs: duration.Milliseconds(),
		},
	}
}

func newErrorEvent(err error) AgentEvent {
	return AgentEvent{
		Type:      EventError,
		Timestamp: time.Now(),
		Data: ErrorEventData{
			Error: err.Error(),
		},
	}
}