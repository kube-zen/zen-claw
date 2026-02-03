package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/neves/zen-claw/internal/config"
)

// Server represents the Zen Claw gateway server
type Server struct {
	config       *config.Config
	server       *http.Server
	mu           sync.RWMutex
	running      bool
	pidFile      string
	agentService *AgentService
}

// NewServer creates a new gateway server
func NewServer(cfg *config.Config) *Server {
	srv := &Server{
		config:       cfg,
		pidFile:      "/tmp/zen-claw-gateway.pid",
		agentService: NewAgentService(cfg),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.healthHandler)
	mux.HandleFunc("/chat", srv.chatHandler)
	mux.HandleFunc("/chat/stream", srv.streamChatHandler) // SSE streaming endpoint
	mux.HandleFunc("/ws", srv.wsHandler)                  // WebSocket endpoint
	mux.HandleFunc("/sessions", srv.sessionsHandler)
	mux.HandleFunc("/sessions/", srv.sessionHandler)
	mux.HandleFunc("/preferences", srv.preferencesHandler)
	mux.HandleFunc("/preferences/", srv.preferencesHandler)
	mux.HandleFunc("/stats", srv.statsHandler) // Usage and cache stats
	mux.HandleFunc("/", srv.defaultHandler)

	srv.server = &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	return srv
}

// Start starts the gateway server (blocks until shutdown)
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	// Write PID file
	if err := s.writePID(); err != nil {
		log.Printf("Warning: Failed to write PID file: %v", err)
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Starting Zen Claw gateway on %s", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or server error
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-shutdownChan:
		log.Printf("Shutdown signal received: %v", sig)
		s.Stop()
		return nil
	case err := <-serverErr:
		return fmt.Errorf("server failed: %w", err)
	}
}

// Stop stops the gateway server
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return fmt.Errorf("server not running")
	}
	s.running = false
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown failed: %v", err)
	}

	// Remove PID file
	os.Remove(s.pidFile)

	log.Println("Gateway stopped")
	return nil
}

// Restart restarts the gateway server
func (s *Server) Restart() error {
	if err := s.Stop(); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	return s.Start()
}

// Status returns the server status
func (s *Server) Status() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.running {
		return "running"
	}
	return "stopped"
}

// waitForShutdown is now integrated into Start() method

// writePID writes the PID to file
func (s *Server) writePID() error {
	pid := os.Getpid()
	return os.WriteFile(s.pidFile, []byte(fmt.Sprintf("%d\n", pid)), 0644)
}

// healthHandler handles health checks
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"gateway":   "zen-claw",
		"version":   "0.1.0",
	})
}

// statsHandler returns usage and cache statistics
func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hits, misses, size, hitRate := s.agentService.GetCacheStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"usage": s.agentService.GetUsageSummary(),
		"cache": map[string]interface{}{
			"hits":     hits,
			"misses":   misses,
			"size":     size,
			"hit_rate": hitRate,
		},
		"circuits":  s.agentService.GetCircuitStats(),
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// chatHandler handles chat requests
func (s *Server) chatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.UserInput == "" {
		http.Error(w, "user_input is required", http.StatusBadRequest)
		return
	}

	// Set default working directory if not provided
	if req.WorkingDir == "" {
		req.WorkingDir = "."
	}

	// Process with agent service
	ctx := r.Context()
	resp, err := s.agentService.Chat(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Agent service error: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// sessionsHandler lists all sessions with state info
func (s *Server) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get sessions with state from agent service
	sessions := s.agentService.ListSessionsWithState()

	// Convert to response format
	sessionList := make([]map[string]interface{}, 0, len(sessions))
	for _, entry := range sessions {
		sessionList = append(sessionList, map[string]interface{}{
			"id":                 entry.Stats.SessionID,
			"created_at":         entry.Stats.CreatedAt.Format(time.RFC3339),
			"updated_at":         entry.Stats.UpdatedAt.Format(time.RFC3339),
			"message_count":      entry.Stats.MessageCount,
			"user_messages":      entry.Stats.UserMessages,
			"assistant_messages": entry.Stats.AssistantMessages,
			"tool_messages":      entry.Stats.ToolMessages,
			"working_dir":        entry.Stats.WorkingDir,
			"state":              string(entry.State),
			"client_id":          entry.ClientID,
			"last_used":          entry.LastUsed.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions":     sessionList,
		"count":        len(sessionList),
		"max_sessions": s.agentService.GetMaxSessions(),
		"active_count": s.agentService.GetActiveSessionCount(),
	})
}

// sessionHandler handles individual session operations
func (s *Server) sessionHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/sessions/"):]
	if path == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Parse path for actions: /sessions/{id}/background, /sessions/{id}/activate
	parts := splitPath(path)
	sessionID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	// Handle session actions
	if action != "" {
		s.handleSessionAction(w, r, sessionID, action)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Get session from agent service
		session, exists := s.agentService.GetSession(sessionID)
		if !exists {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}

		stats := session.GetStats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":                 stats.SessionID,
			"created_at":         stats.CreatedAt.Format(time.RFC3339),
			"updated_at":         stats.UpdatedAt.Format(time.RFC3339),
			"message_count":      stats.MessageCount,
			"user_messages":      stats.UserMessages,
			"assistant_messages": stats.AssistantMessages,
			"tool_messages":      stats.ToolMessages,
			"working_dir":        stats.WorkingDir,
			"messages":           session.GetMessages(),
		})

	case http.MethodDelete:
		// Delete session via agent service
		deleted := s.agentService.DeleteSession(sessionID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deleted": deleted,
			"id":      sessionID,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSessionAction handles session actions (background, activate)
func (s *Server) handleSessionAction(w http.ResponseWriter, r *http.Request, sessionID, action string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	switch action {
	case "background":
		if err := s.agentService.BackgroundSession(sessionID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     sessionID,
			"state":  "background",
			"status": "ok",
		})

	case "activate":
		// Get client ID from request body (optional)
		var req struct {
			ClientID string `json:"client_id"`
		}
		json.NewDecoder(r.Body).Decode(&req) // Ignore error, clientID is optional

		if err := s.agentService.ActivateSession(sessionID, req.ClientID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     sessionID,
			"state":  "active",
			"status": "ok",
		})

	default:
		http.Error(w, "Unknown action: "+action, http.StatusBadRequest)
	}
}

// splitPath splits a URL path by /
func splitPath(path string) []string {
	var parts []string
	for _, p := range splitString(path, '/') {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// splitString splits a string by a separator
func splitString(s string, sep rune) []string {
	var result []string
	current := ""
	for _, r := range s {
		if r == sep {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// streamChatHandler handles streaming chat requests via SSE
func (s *Server) streamChatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.UserInput == "" {
		http.Error(w, "user_input is required", http.StatusBadRequest)
		return
	}

	// Set default working directory if not provided
	if req.WorkingDir == "" {
		req.WorkingDir = "."
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get flusher for streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Create event channel
	eventChan := make(chan map[string]interface{}, 100)

	// Process with agent service (with progress callback)
	ctx := r.Context()
	go func() {
		defer close(eventChan)
		resp, err := s.agentService.ChatWithProgress(ctx, req, func(event map[string]interface{}) {
			select {
			case eventChan <- event:
			case <-ctx.Done():
				return
			}
		})
		if err != nil {
			eventChan <- map[string]interface{}{
				"type":    "error",
				"message": err.Error(),
			}
			return
		}
		// Send final result
		eventChan <- map[string]interface{}{
			"type":         "done",
			"session_id":   resp.SessionID,
			"result":       resp.Result,
			"session_info": resp.SessionInfo,
		}
	}()

	// Stream events to client
	for event := range eventChan {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		// Check if client disconnected
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// preferencesHandler handles AI preferences viewing and modification
func (s *Server) preferencesHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/preferences"):]
	path = strings.TrimPrefix(path, "/")

	switch r.Method {
	case http.MethodGet:
		// Return preferences based on path
		prefs := make(map[string]interface{})

		switch path {
		case "", "all":
			prefs["fallback_order"] = s.config.GetFallbackOrder()
			prefs["consensus"] = map[string]interface{}{
				"workers": s.config.GetConsensusWorkers(),
				"arbiter": s.config.GetArbiterOrder(),
			}
			prefs["factory"] = map[string]interface{}{
				"specialists": s.config.Factory.Specialists,
				"guardrails":  s.config.Factory.Guardrails,
			}
			prefs["default"] = map[string]interface{}{
				"provider": s.config.Default.Provider,
				"model":    s.config.Default.Model,
			}
		case "fallback":
			prefs["fallback_order"] = s.config.GetFallbackOrder()
		case "consensus":
			prefs["workers"] = s.config.GetConsensusWorkers()
			prefs["arbiter"] = s.config.GetArbiterOrder()
		case "factory":
			prefs["specialists"] = s.config.Factory.Specialists
			prefs["guardrails"] = s.config.Factory.Guardrails
		default:
			http.Error(w, "Unknown preference: "+path, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prefs)

	case http.MethodPost, http.MethodPut:
		// Update preferences
		var update map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Update fallback order
		if fo, ok := update["fallback_order"].([]interface{}); ok {
			order := make([]string, len(fo))
			for i, v := range fo {
				order[i] = v.(string)
			}
			s.config.Preferences.FallbackOrder = order
		}

		// Update default provider/model
		if def, ok := update["default"].(map[string]interface{}); ok {
			if p, ok := def["provider"].(string); ok {
				s.config.Default.Provider = p
			}
			if m, ok := def["model"].(string); ok {
				s.config.Default.Model = m
			}
		}

		// Update consensus arbiter
		if arb, ok := update["arbiter"].([]interface{}); ok {
			order := make([]string, len(arb))
			for i, v := range arb {
				order[i] = v.(string)
			}
			s.config.Consensus.Arbiter = order
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// defaultHandler shows available endpoints
func (s *Server) defaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Zen Claw Gateway v0.1.0\n")
	fmt.Fprintf(w, "Available AI providers: %v\n", s.agentService.GetAvailableProviders())
	fmt.Fprintf(w, "Max sessions: %d, Active: %d\n", s.agentService.GetMaxSessions(), s.agentService.GetActiveSessionCount())
	fmt.Fprintf(w, "\nEndpoints:\n")
	fmt.Fprintf(w, "  GET  /health                    - Health check\n")
	fmt.Fprintf(w, "  POST /chat                      - Chat with AI (JSON)\n")
	fmt.Fprintf(w, "  POST /chat/stream               - Chat with AI (SSE streaming)\n")
	fmt.Fprintf(w, "  GET  /ws                        - WebSocket (bidirectional)\n")
	fmt.Fprintf(w, "  GET  /sessions                  - List sessions with state\n")
	fmt.Fprintf(w, "  GET  /sessions/{id}             - Get session details\n")
	fmt.Fprintf(w, "  DELETE /sessions/{id}           - Delete session\n")
	fmt.Fprintf(w, "  POST /sessions/{id}/background  - Move session to background\n")
	fmt.Fprintf(w, "  POST /sessions/{id}/activate    - Activate a session\n")
	fmt.Fprintf(w, "  GET  /preferences               - View AI preferences\n")
	fmt.Fprintf(w, "  POST /preferences               - Update AI preferences\n")
}
