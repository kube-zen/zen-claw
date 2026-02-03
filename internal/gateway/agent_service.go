package gateway

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
)

// GatewayAICaller implements agent.AICaller for gateway
type GatewayAICaller struct {
	aiRouter *AIRouter
	provider string
	model    string
}

func (c *GatewayAICaller) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	// Use provider/model from caller if specified, otherwise use defaults
	preferredProvider := c.provider

	// Override model if specified
	if c.model != "" {
		req.Model = c.model
	}

	return c.aiRouter.Chat(ctx, req, preferredProvider)
}

// AgentService manages agent sessions and tool execution via gateway
type AgentService struct {
	config           *config.Config
	aiRouter         *AIRouter
	tools            []agent.Tool
	sessionStore     *SessionStore
	fallbackSessions map[string]*agent.Session
	fallbackMu       sync.RWMutex
}

// NewAgentService creates a new agent service for the gateway
func NewAgentService(cfg *config.Config) *AgentService {
	aiRouter := NewAIRouter(cfg)

	// Create session store with persistence
	sessionStore, err := NewSessionStore(&SessionStoreConfig{
		DataDir:     "/tmp/zen-claw-sessions",
		MaxSessions: cfg.GetMaxSessions(),
	})
	if err != nil {
		// Fallback to in-memory if persistence fails
		log.Printf("Warning: Failed to create session store: %v", err)
		sessionStore = nil
	}

	// Create tools (working directory will be set per session)
	// Full toolset for code generation and editing
	tools := []agent.Tool{
		agent.NewExecTool(""),        // Shell commands
		agent.NewReadFileTool(""),    // Read files
		agent.NewWriteFileTool(""),   // Create/overwrite files
		agent.NewEditFileTool(""),    // String replacement (like Cursor's StrReplace)
		agent.NewAppendFileTool(""),  // Append to files
		agent.NewListDirTool(""),     // List directories
		agent.NewSearchFilesTool(""), // Grep-like search
		agent.NewSystemInfoTool(),    // System info
	}

	return &AgentService{
		config:           cfg,
		aiRouter:         aiRouter,
		tools:            tools,
		sessionStore:     sessionStore,
		fallbackSessions: make(map[string]*agent.Session),
	}
}

// ChatRequest represents a chat request to the agent service
type ChatRequest struct {
	SessionID  string `json:"session_id"`
	UserInput  string `json:"user_input"`
	WorkingDir string `json:"working_dir,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	MaxSteps   int    `json:"max_steps,omitempty"`
}

// ChatResponse represents a chat response from the agent service
type ChatResponse struct {
	SessionID   string             `json:"session_id"`
	Result      string             `json:"result"`
	SessionInfo agent.SessionStats `json:"session_info"`
	Error       string             `json:"error,omitempty"`
}

// ProgressCallback is a function called for each progress event
type ProgressCallback func(event map[string]interface{})

// Chat handles a chat request using the agent service (no progress)
func (s *AgentService) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return s.ChatWithProgress(ctx, req, nil)
}

// ChatWithProgress handles a chat request with progress callback for streaming
func (s *AgentService) ChatWithProgress(ctx context.Context, req ChatRequest, progressCb ProgressCallback) (*ChatResponse, error) {
	// Get or create session
	session := s.getOrCreateSession(req.SessionID)

	// Set working directory if provided
	if req.WorkingDir != "" {
		session.SetWorkingDir(req.WorkingDir)
	}

	// Determine provider and model
	providerName := req.Provider
	modelName := req.Model

	// If model is specified but provider isn't, try to infer provider from model name
	if modelName != "" && providerName == "" {
		providerName = s.inferProviderFromModel(modelName)
	}

	// If provider still not determined, use default
	if providerName == "" {
		providerName = s.config.Default.Provider
	}

	// If model not specified, use default for provider
	if modelName == "" {
		modelName = s.config.GetModel(providerName)
	}

	// Emit initial progress
	if progressCb != nil {
		progressCb(map[string]interface{}{
			"type":     "start",
			"provider": providerName,
			"model":    modelName,
			"message":  fmt.Sprintf("Starting with %s/%s", providerName, modelName),
		})
	}

	// Set max steps - default 100 for complex multi-step tasks
	// Similar to Cursor which can handle large refactoring tasks
	maxSteps := req.MaxSteps
	if maxSteps == 0 {
		maxSteps = 100
	}

	// Create AI caller for gateway
	aiCaller := &GatewayAICaller{
		aiRouter: s.aiRouter,
		provider: providerName,
		model:    modelName,
	}

	// Create agent with progress callback
	agentInstance := agent.NewAgent(aiCaller, s.tools, maxSteps)

	// Set progress callback on agent if provided
	if progressCb != nil {
		agentInstance.SetProgressCallback(func(event agent.ProgressEvent) {
			progressCb(map[string]interface{}{
				"type":    event.Type,
				"step":    event.Step,
				"message": event.Message,
				"data":    event.Data,
			})
		})
	}

	// IMPORTANT: Use a detached context for agent execution
	// The HTTP request context has a shorter timeout (5 min from client)
	// but complex tasks may need 30+ minutes to complete.
	// We create a new context with a generous timeout for large tasks.
	// This is similar to how Cursor handles large tasks - they run in background
	// and are not tied to the HTTP request lifecycle.
	agentCtx, agentCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer agentCancel()

	// Also monitor HTTP context for client disconnection (graceful abort)
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			// HTTP client disconnected - but we let the current step finish
			log.Printf("[AgentService] HTTP context cancelled, agent will complete current step")
		case <-done:
			// Agent finished normally
		}
	}()
	defer close(done)

	// Run agent with detached context
	startTime := time.Now()
	updatedSession, result, err := agentInstance.Run(agentCtx, session, req.UserInput)
	duration := time.Since(startTime)

	if err != nil {
		return &ChatResponse{
			SessionID:   updatedSession.ID,
			Result:      "",
			SessionInfo: updatedSession.GetStats(),
			Error:       err.Error(),
		}, nil // Return error in response, not as Go error
	}

	// Save session - only persist explicitly named sessions
	// Auto-generated sessions (session_*) stay in memory only (like Cursor)
	if isNamedSession(updatedSession.ID) && s.sessionStore != nil {
		if err := s.sessionStore.SaveSession(updatedSession); err != nil {
			log.Printf("Warning: Failed to save session %s: %v", updatedSession.ID, err)
		}
	} else {
		// Keep in memory for conversation continuity within this run
		s.fallbackMu.Lock()
		s.fallbackSessions[updatedSession.ID] = updatedSession
		s.fallbackMu.Unlock()
	}

	// Get session stats
	stats := updatedSession.GetStats()

	log.Printf("[AgentService] Session %s: %d messages, %v duration",
		stats.SessionID, stats.MessageCount, duration.Round(time.Millisecond))

	return &ChatResponse{
		SessionID:   stats.SessionID,
		Result:      result,
		SessionInfo: stats,
	}, nil
}

// GetSession returns a session by ID
func (s *AgentService) GetSession(sessionID string) (*agent.Session, bool) {
	if s.sessionStore != nil {
		return s.sessionStore.GetSession(sessionID)
	}

	// Fallback to in-memory
	s.fallbackMu.RLock()
	defer s.fallbackMu.RUnlock()
	session, exists := s.fallbackSessions[sessionID]
	return session, exists
}

// ListSessions returns all sessions
func (s *AgentService) ListSessions() []agent.SessionStats {
	if s.sessionStore != nil {
		return s.sessionStore.ListSessions()
	}

	// Fallback to in-memory
	s.fallbackMu.RLock()
	defer s.fallbackMu.RUnlock()

	var stats []agent.SessionStats
	for _, session := range s.fallbackSessions {
		stats = append(stats, session.GetStats())
	}
	return stats
}

// DeleteSession deletes a session
func (s *AgentService) DeleteSession(sessionID string) bool {
	if s.sessionStore != nil {
		return s.sessionStore.DeleteSession(sessionID)
	}

	// Fallback to in-memory
	s.fallbackMu.Lock()
	defer s.fallbackMu.Unlock()

	if _, exists := s.fallbackSessions[sessionID]; exists {
		delete(s.fallbackSessions, sessionID)
		return true
	}
	return false
}

// getOrCreateSession gets an existing session or creates a new one
// IMPORTANT: Sessions are fresh by default (like Cursor).
// Only explicitly named sessions are persisted.
func (s *AgentService) getOrCreateSession(sessionID string) *agent.Session {
	// Check if this is an explicitly named session (not auto-generated)
	isNamedSession := sessionID != "" && !strings.HasPrefix(sessionID, "session_")

	// Try to get existing named session from persistent store
	if isNamedSession && s.sessionStore != nil {
		if session, exists := s.sessionStore.GetSession(sessionID); exists {
			return session
		}
	}

	// For unnamed sessions, check in-memory cache
	if sessionID != "" {
		s.fallbackMu.RLock()
		session, exists := s.fallbackSessions[sessionID]
		s.fallbackMu.RUnlock()
		if exists {
			return session
		}
	}

	// Create new session (fresh context, like Cursor "new chat")
	session := agent.NewSession(sessionID)

	// Add system message to guide the AI
	session.AddMessage(ai.Message{
		Role: "system",
		Content: `You are a software engineer assistant with full access to tools for reading, writing, and editing code.

AVAILABLE TOOLS:
- exec: Run shell commands (git, make, go, npm, etc.)
- read_file: Read file contents
- write_file: Create or overwrite files
- edit_file: Make precise string replacements in files
- append_file: Append content to files
- list_dir: List directory contents
- search_files: Search for patterns in files (grep-like)
- system_info: Get system information

WORKFLOW:
1. For simple questions: Answer directly
2. For code tasks: Use tools to read, analyze, then write/edit
3. Be efficient - don't over-explore

When editing files, use edit_file with unique string matches. For new files, use write_file.`,
	})

	// Only persist named sessions, not auto-generated ones
	// This keeps the session list clean (like Cursor's chat history)
	if isNamedSession && s.sessionStore != nil {
		// Will be saved after first interaction
	} else {
		// Store in memory for conversation continuity within this run
		s.fallbackMu.Lock()
		s.fallbackSessions[sessionID] = session
		s.fallbackMu.Unlock()
	}

	return session
}

// isNamedSession returns true if session was explicitly named (not auto-generated)
func isNamedSession(sessionID string) bool {
	return sessionID != "" && !strings.HasPrefix(sessionID, "session_")
}

// ListSessionsWithState returns all sessions with their state
func (s *AgentService) ListSessionsWithState() []SessionListEntry {
	if s.sessionStore != nil {
		return s.sessionStore.ListSessionsWithState()
	}

	// Fallback to in-memory (no state tracking)
	s.fallbackMu.RLock()
	defer s.fallbackMu.RUnlock()

	var entries []SessionListEntry
	for _, session := range s.fallbackSessions {
		entries = append(entries, SessionListEntry{
			Stats:    session.GetStats(),
			State:    SessionStateActive,
			LastUsed: time.Now(),
		})
	}
	return entries
}

// BackgroundSession moves a session to background state
func (s *AgentService) BackgroundSession(sessionID string) error {
	if s.sessionStore != nil {
		return s.sessionStore.BackgroundSession(sessionID)
	}
	return nil // No state tracking in fallback
}

// ActivateSession activates a session for a client
func (s *AgentService) ActivateSession(sessionID, clientID string) error {
	if s.sessionStore != nil {
		return s.sessionStore.ActivateSession(sessionID, clientID)
	}
	return nil // No state tracking in fallback
}

// CanCreateSession checks if a new session can be created
func (s *AgentService) CanCreateSession() bool {
	if s.sessionStore != nil {
		return s.sessionStore.CanCreateSession()
	}
	return true // No limit in fallback
}

// GetMaxSessions returns the max sessions limit
func (s *AgentService) GetMaxSessions() int {
	if s.sessionStore != nil {
		return s.sessionStore.GetMaxSessions()
	}
	return 0 // Unlimited in fallback
}

// GetActiveSessionCount returns the count of active sessions
func (s *AgentService) GetActiveSessionCount() int {
	if s.sessionStore != nil {
		return s.sessionStore.GetActiveSessionCount()
	}

	s.fallbackMu.RLock()
	defer s.fallbackMu.RUnlock()
	return len(s.fallbackSessions)
}

// GetAvailableProviders returns available AI providers
func (s *AgentService) GetAvailableProviders() []string {
	// Check which providers have API keys (including environment variables)
	var available []string

	providers := []string{"deepseek", "glm", "minimax", "openai", "qwen", "kimi"}

	for _, provider := range providers {
		if s.config.GetAPIKey(provider) != "" {
			available = append(available, provider)
		}
	}

	return available
}

// inferProviderFromModel tries to infer provider from model name
func (s *AgentService) inferProviderFromModel(modelName string) string {
	modelName = strings.ToLower(modelName)

	// Check for provider patterns in model name
	if strings.Contains(modelName, "qwen") {
		return "qwen"
	} else if strings.Contains(modelName, "deepseek") {
		return "deepseek"
	} else if strings.Contains(modelName, "glm") {
		return "glm"
	} else if strings.Contains(modelName, "minimax") || strings.Contains(modelName, "abab") {
		return "minimax"
	} else if strings.Contains(modelName, "gpt") {
		return "openai"
	} else if strings.Contains(modelName, "kimi") || strings.Contains(modelName, "moonshot") {
		return "kimi"
	}

	// Default to empty (will use default provider)
	return ""
}
