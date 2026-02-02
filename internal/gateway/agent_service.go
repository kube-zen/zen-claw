package gateway

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
)

// GatewayAICaller implements agent.AICaller for gateway
type GatewayAICaller struct {
	aiRouter     *AIRouter
	provider     string
	model        string
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
	config     *config.Config
	aiRouter   *AIRouter
	tools      []agent.Tool
	sessions   map[string]*agent.Session
	sessionsMu sync.RWMutex
}

// NewAgentService creates a new agent service for the gateway
func NewAgentService(cfg *config.Config) *AgentService {
	aiRouter := NewAIRouter(cfg)
	
	// Create tools (working directory will be set per session)
	tools := []agent.Tool{
		agent.NewExecTool(""), // Working dir set per session
		agent.NewReadFileTool(""),
		agent.NewListDirTool(""),
		agent.NewSystemInfoTool(),
	}
	
	return &AgentService{
		config:   cfg,
		aiRouter: aiRouter,
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
	
	// Set max steps
	maxSteps := req.MaxSteps
	if maxSteps == 0 {
		maxSteps = 10
	}
	
	// Create AI caller for gateway
	aiCaller := &GatewayAICaller{
		aiRouter: s.aiRouter,
		provider: providerName,
		model:    modelName,
	}
	
	// Create agent
	agentInstance := agent.NewAgent(aiCaller, s.tools, maxSteps)
	
	// Run agent
	startTime := time.Now()
	updatedSession, result, err := agentInstance.Run(ctx, session, req.UserInput)
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
	// Check which providers have API keys (including environment variables)
	var available []string
	
	providers := []string{"deepseek", "glm", "minimax", "openai", "qwen"}
	
	for _, provider := range providers {
		if s.config.GetAPIKey(provider) != "" {
			available = append(available, provider)
		}
	}
	
	return available
}