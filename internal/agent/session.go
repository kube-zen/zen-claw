package agent

import (
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/ai"
)

// Session manages conversation state and history
type Session struct {
	ID                      string
	messages                []ai.Message
	createdAt               time.Time
	updatedAt               time.Time
	workingDir              string
	contextLimit            int  // Limit on messages sent (0 = no limit, default 50)
	qwenLargeContextEnabled bool // Enable 256k context for Qwen (default false)
	mu                      sync.RWMutex
}

// NewSession creates a new session
func NewSession(id string) *Session {
	if id == "" {
		id = generateSessionID()
	}

	now := time.Now()
	return &Session{
		ID:                      id,
		createdAt:               now,
		updatedAt:               now,
		messages:                []ai.Message{},
		contextLimit:            50,    // Default context limit
		qwenLargeContextEnabled: false, // Default: disabled to avoid crashes
	}
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(msg ai.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, msg)
	s.updatedAt = time.Now()
}

// GetMessages returns all messages in the session
func (s *Session) GetMessages() []ai.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent modification
	messages := make([]ai.Message, len(s.messages))
	copy(messages, s.messages)
	return messages
}

// ClearMessages clears all messages except system messages
func (s *Session) ClearMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Keep system messages
	var keep []ai.Message
	for _, msg := range s.messages {
		if msg.Role == "system" {
			keep = append(keep, msg)
		}
	}

	s.messages = keep
	s.updatedAt = time.Now()
}

// SetWorkingDir sets the working directory for the session
func (s *Session) SetWorkingDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.workingDir = dir
}

// GetWorkingDir returns the working directory
func (s *Session) GetWorkingDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.workingDir
}

// SetContextLimit sets the context limit (0 = no limit)
func (s *Session) SetContextLimit(limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contextLimit = limit
}

// GetContextLimit returns the context limit
func (s *Session) GetContextLimit() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.contextLimit
}

// SetQwenLargeContextEnabled enables/disables Qwen large context window
func (s *Session) SetQwenLargeContextEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.qwenLargeContextEnabled = enabled
}

// GetQwenLargeContextEnabled returns whether Qwen large context is enabled
func (s *Session) GetQwenLargeContextEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.qwenLargeContextEnabled
}

// GetStats returns session statistics
func (s *Session) GetStats() SessionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := SessionStats{
		SessionID:    s.ID,
		CreatedAt:    s.createdAt,
		UpdatedAt:    s.updatedAt,
		MessageCount: len(s.messages),
		WorkingDir:   s.workingDir,
	}

	// Count message types
	for _, msg := range s.messages {
		switch msg.Role {
		case "user":
			stats.UserMessages++
		case "assistant":
			stats.AssistantMessages++
		case "tool":
			stats.ToolMessages++
		case "system":
			stats.SystemMessages++
		}
	}

	return stats
}

// SessionStats contains session statistics
type SessionStats struct {
	SessionID         string    `json:"session_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	MessageCount      int       `json:"message_count"`
	UserMessages      int       `json:"user_messages"`
	AssistantMessages int       `json:"assistant_messages"`
	ToolMessages      int       `json:"tool_messages"`
	SystemMessages    int       `json:"system_messages"`
	WorkingDir        string    `json:"working_dir"`
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return "session_" + time.Now().Format("20060102_150405")
}
