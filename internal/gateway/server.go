package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/neves/zen-claw/internal/config"
)

// Server represents the Zen Claw gateway server
type Server struct {
	config      *config.Config
	server      *http.Server
	mu          sync.RWMutex
	running     bool
	pidFile     string
	agentService *AgentService
}

// NewServer creates a new gateway server
func NewServer(cfg *config.Config) *Server {
	srv := &Server{
		config:      cfg,
		pidFile:     "/tmp/zen-claw-gateway.pid",
		agentService: NewAgentService(cfg),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.healthHandler)
	mux.HandleFunc("/chat", srv.chatHandler)
	mux.HandleFunc("/sessions", srv.sessionsHandler)
	mux.HandleFunc("/sessions/", srv.sessionHandler)
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

// sessionsHandler lists all sessions
func (s *Server) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get sessions from agent service
	sessions := s.agentService.ListSessions()
	
	// Convert to response format
	sessionList := make([]map[string]interface{}, 0, len(sessions))
	for _, stats := range sessions {
		sessionList = append(sessionList, map[string]interface{}{
			"id":            stats.SessionID,
			"created_at":    stats.CreatedAt.Format(time.RFC3339),
			"updated_at":    stats.UpdatedAt.Format(time.RFC3339),
			"message_count": stats.MessageCount,
			"user_messages": stats.UserMessages,
			"assistant_messages": stats.AssistantMessages,
			"tool_messages": stats.ToolMessages,
			"working_dir":   stats.WorkingDir,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": sessionList,
		"count":    len(sessionList),
	})
}

// sessionHandler handles individual session operations
func (s *Server) sessionHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Path[len("/sessions/"):]
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
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
			"id":            stats.SessionID,
			"created_at":    stats.CreatedAt.Format(time.RFC3339),
			"updated_at":    stats.UpdatedAt.Format(time.RFC3339),
			"message_count": stats.MessageCount,
			"user_messages": stats.UserMessages,
			"assistant_messages": stats.AssistantMessages,
			"tool_messages": stats.ToolMessages,
			"working_dir":   stats.WorkingDir,
			"messages":      session.GetMessages(),
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

// defaultHandler shows available endpoints
func (s *Server) defaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Zen Claw Gateway v0.1.0\n")
	fmt.Fprintf(w, "Available AI providers: %v\n", s.agentService.GetAvailableProviders())
	fmt.Fprintf(w, "\nEndpoints:\n")
	fmt.Fprintf(w, "  GET  /health           - Health check\n")
	fmt.Fprintf(w, "  POST /chat             - Chat with AI (JSON)\n")
	fmt.Fprintf(w, "  GET  /sessions         - List sessions\n")
	fmt.Fprintf(w, "  GET  /sessions/{id}    - Get session\n")
	fmt.Fprintf(w, "  DELETE /sessions/{id}  - Delete session\n")
}