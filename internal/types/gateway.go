// Package types provides shared types used across zen-claw packages.
package types

// ChatRequest represents a chat request to the gateway.
// Used by CLI, Slack bot, and gateway service.
type ChatRequest struct {
	SessionID     string `json:"session_id,omitempty"`
	UserInput     string `json:"user_input"`
	WorkingDir    string `json:"working_dir,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	MaxSteps      int    `json:"max_steps,omitempty"`
	ThinkingLevel string `json:"thinking_level,omitempty"` // off, low, medium, high
	Stream        bool   `json:"stream,omitempty"`         // Enable token-by-token streaming
}

// ChatResponse represents a chat response from the gateway.
type ChatResponse struct {
	SessionID   string                 `json:"session_id"`
	Result      string                 `json:"result"`
	Error       string                 `json:"error,omitempty"`
	SessionInfo map[string]interface{} `json:"session_info,omitempty"`
}

// ProgressEvent represents a progress event during agent execution.
type ProgressEvent struct {
	Type    string      `json:"type"`    // start, step, thinking, ai_response, tool_call, complete, error, done
	Step    int         `json:"step"`    // Current step number
	Message string      `json:"message"` // Human-readable message
	Data    interface{} `json:"data,omitempty"`
}

// ProgressCallback is called with progress events during execution.
type ProgressCallback func(event ProgressEvent)
