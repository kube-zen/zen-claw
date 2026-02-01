package gateway

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
)

// AgentService manages agent sessions and tool execution via gateway
type AgentService struct {
	config     *config.Config
	factory    *providers.Factory
	tools      []agent.Tool
	sessions   map[string]*agent.Session
	sessionsMu sync.RWMutex
}

// NewAgentService creates a new agent service for the gateway
func NewAgentService(cfg *config.Config) *AgentService {
	factory := providers.NewFactory(cfg)
	
	// Create tools (working directory will be set per session)
	tools := []agent.Tool{
		agent.NewExecTool(""), // Working dir set per session
		agent.NewReadFileTool(""),
		agent.NewListDirTool(""),
		agent.NewSystemInfoTool(),
	}
	
	return &AgentService{
		config:   cfg,
		factory:  factory,
		tools:    tools,
		sessions: make(map[string]*agent.Session),
	}
}

// ChatRequest represents a chat request to the agent service
type ChatRequest struct {
	SessionID   string `json:"session_id"`
	UserInput   string `json:"user_input"`
	WorkingDir  string `json:"working_dir,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
	MaxSteps    int    `json:"max_steps,omitempty"`
}

// ChatResponse represents a chat response from the agent service
type ChatResponse struct {
	SessionID   string           `json:"session_id"`
	Result      string           `json:"result"`
	SessionInfo agent.SessionStats `json:"session_info"`
	Error       string           `json:"error,omitempty"`
}

// Chat handles a chat request using the agent service
func (s *AgentService) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Get or create session
	session := s.getOrCreateSession(req.SessionID)
	
	// Set working directory if provided
	if req.WorkingDir != "" {
		session.SetWorkingDir(req.WorkingDir)
	}
	
	// Determine provider
	providerName := req.Provider
	if providerName == "" {
		providerName = s.config.Default.Provider
	}
	
	// Determine model
	modelName := req.Model
	if modelName == "" {
		modelName = s.config.GetModel(providerName)
	}
	
	// Create AI provider
	aiProvider, err := s.factory.CreateProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider %s: %w", providerName, err)
	}
	
	// Set max steps
	maxSteps := req.MaxSteps
	if maxSteps == 0 {
		maxSteps = 10
	}
	
	// Create lightweight agent
	lightAgent := agent.NewLightAgent(aiProvider, s.tools, maxSteps)
	
	// Run agent
	startTime := time.Now()
	updatedSession, result, err := lightAgent.Run(ctx, session, req.UserInput)
	duration := time.Since(startTime)
	
	if err != nil {
		return &ChatResponse{
			SessionID: updatedSession.ID,
			Result:    "",
			SessionInfo: updatedSession.GetStats(),
			Error:     err.Error(),
		}, nil // Return error in response, not as Go error
	}
	
	// Update session in map
	s.sessionsMu.Lock()
	s.sessions[updatedSession.ID] = updatedSession
	s.sessionsMu.Unlock()
	
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
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	
	session, exists := s.sessions[sessionID]
	return session, exists
}

// ListSessions returns all sessions
func (s *AgentService) ListSessions() []agent.SessionStats {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	
	var stats []agent.SessionStats
	for _, session := range s.sessions {
		stats = append(stats, session.GetStats())
	}
	return stats
}

// DeleteSession deletes a session
func (s *AgentService) DeleteSession(sessionID string) bool {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	
	if _, exists := s.sessions[sessionID]; exists {
		delete(s.sessions, sessionID)
		return true
	}
	return false
}

// getOrCreateSession gets an existing session or creates a new one
func (s *AgentService) getOrCreateSession(sessionID string) *agent.Session {
	s.sessionsMu.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMu.RUnlock()
	
	if exists {
		return session
	}
	
	// Create new session
	session = agent.NewSession(sessionID)
	
	s.sessionsMu.Lock()
	s.sessions[sessionID] = session
	s.sessionsMu.Unlock()
	
	return session
}

// GetAvailableProviders returns available AI providers
func (s *AgentService) GetAvailableProviders() []string {
	// Check which providers have API keys
	var available []string
	
	// DeepSeek
	if s.config.Providers.DeepSeek.APIKey != "" {
		available = append(available, "deepseek")
	}
	
	// GLM
	if s.config.Providers.GLM.APIKey != "" {
		available = append(available, "glm")
	}
	
	// Minimax
	if s.config.Providers.Minimax.APIKey != "" {
		available = append(available, "minimax")
	}
	
	// OpenAI (check env var)
	if s.config.Providers.OpenAI.APIKey != "${OPENAI_API_KEY}" {
		// API key is set (not the template)
		available = append(available, "openai")
	}
	
	// Qwen (check env var)
	if s.config.Providers.Qwen.APIKey != "${QWEN_API_KEY}" {
		// API key is set (not the template)
		available = append(available, "qwen")
	}
	
	return available
}