package gateway

import (
	"context"
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
	config       *config.Config
	aiRouter     *AIRouter
	tools        []agent.Tool
	sessionStore *SessionStore
	fallbackSessions map[string]*agent.Session
	fallbackMu   sync.RWMutex
}

// NewAgentService creates a new agent service for the gateway
func NewAgentService(cfg *config.Config) *AgentService {
	aiRouter := NewAIRouter(cfg)
	
	// Create session store with persistence
	sessionStore, err := NewSessionStore(&SessionStoreConfig{
		DataDir: "/tmp/zen-claw-sessions",
	})
	if err != nil {
		// Fallback to in-memory if persistence fails
		log.Printf("Warning: Failed to create session store: %v", err)
		sessionStore = nil
	}
	
	// Create tools (working directory will be set per session)
	tools := []agent.Tool{
		agent.NewExecTool(""), // Working dir set per session
		agent.NewReadFileTool(""),
		agent.NewListDirTool(""),
		agent.NewSystemInfoTool(),
	}
	
	return &AgentService{
		config:       cfg,
		aiRouter:     aiRouter,
		tools:        tools,
		sessionStore: sessionStore,
		fallbackSessions: make(map[string]*agent.Session),
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
	
	// Set max steps
	maxSteps := req.MaxSteps
	if maxSteps == 0 {
		maxSteps = 50
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
	
	// Save session
	if s.sessionStore != nil {
		// Save to persistent store
		if err := s.sessionStore.SaveSession(updatedSession); err != nil {
			log.Printf("Warning: Failed to save session %s: %v", updatedSession.ID, err)
		}
	} else {
		// Save to fallback in-memory store
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
func (s *AgentService) getOrCreateSession(sessionID string) *agent.Session {
	// Try to get existing session
	if s.sessionStore != nil {
		if session, exists := s.sessionStore.GetSession(sessionID); exists {
			return session
		}
	} else {
		// Fallback to in-memory
		s.fallbackMu.RLock()
		session, exists := s.fallbackSessions[sessionID]
		s.fallbackMu.RUnlock()
		
		if exists {
			return session
		}
	}
	
	// Create new session
	session := agent.NewSession(sessionID)
	
	// Add system message to guide the AI
	session.AddMessage(ai.Message{
		Role: "system",
		Content: `You are a strategic AI assistant that helps with code analysis and development tasks.

CONTEXT MANAGEMENT:
- You have access to large context windows (up to 262K+ tokens)
- Keep conversation history for context
- Use tools when needed, but don't over-explore for simple tasks

INTELLIGENT WORKFLOW:
1. **For simple questions**: Answer directly without tool use
2. **For code analysis requests**: 
   - First list directory to understand structure
   - Then read key files (README, package.json, go.mod, main files)
   - Analyze and provide recommendations
3. **For development tasks**: 
   - Break down into logical steps
   - Use tools only when necessary
   - Be efficient with API calls

TOOL USAGE GUIDELINES:
- exec: For running commands, changing directories, git operations
- read_file: Read specific files when analysis requires it
- list_dir: Explore directory structure when context is needed
- system_info: Get system details when relevant

EFFICIENCY:
- Don't explore unnecessarily for simple tasks
- Read only key files, not everything
- Use large context window to maintain conversation history
- When task is complete, provide clear conclusion

You have a large context window (262K+ tokens), so you can maintain long conversations. Use tools judiciously.`,
	})
	
	// Store in fallback if no persistent store
	if s.sessionStore == nil {
		s.fallbackMu.Lock()
		s.fallbackSessions[sessionID] = session
		s.fallbackMu.Unlock()
	}
	
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
	}
	
	// Default to empty (will use default provider)
	return ""
}