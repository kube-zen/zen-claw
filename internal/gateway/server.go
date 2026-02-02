package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// Server represents the gateway server
type Server struct {
	addr        string
	router      *mux.Router
	streamMgr   *StreamManager
	logger      *logging.Logger
	aiRouter    *AIRouter
	sessionStore *SessionStore
}

// NewServer creates a new gateway server
func NewServer(addr string, aiRouter *AIRouter, sessionStore *SessionStore) *Server {
	return &Server{
		addr:         addr,
		router:       mux.NewRouter(),
		streamMgr:    NewStreamManager(),
		logger:       logging.NewLogger("gateway-server"),
		aiRouter:     aiRouter,
		sessionStore: sessionStore,
	}
}

// Start starts the gateway server
func (s *Server) Start(ctx context.Context) error {
	s.setupRoutes()
	
	fmt.Printf("INFO: %s\n", "test")
	
	// Start cleanup routine for inactive streams
	go s.startCleanupRoutine(ctx)
	
	return http.ListenAndServe(s.addr, s.router)
}

func (s *Server) setupRoutes() {
	// API routes
	s.router.HandleFunc("/api/v1/health", s.healthHandler).Methods("GET")
	s.router.HandleFunc("/api/v1/chat", s.chatHandler).Methods("POST")
	s.router.HandleFunc("/api/v1/stream", s.streamHandler).Methods("POST")
	s.router.HandleFunc("/api/v1/stream/{id}", s.streamWSHandler).Methods("GET")
	s.router.HandleFunc("/api/v1/sessions", s.sessionsHandler).Methods("GET")
	s.router.HandleFunc("/api/v1/sessions/{id}", s.sessionHandler).Methods("GET", "PUT", "DELETE")
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (s *Server) chatHandler(w http.ResponseWriter, r *http.Request) {
	// Existing chat handler logic
}

func (s *Server) streamHandler(w http.ResponseWriter, r *http.Request) {
	var req StreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// Create a unique stream ID
	streamID := fmt.Sprintf("stream-%d", time.Now().Unix())
	
	// Respond with stream ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"stream_id": streamID})
}

func (s *Server) streamWSHandler(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// Allow connections from any origin for simplicity
			return true
		},
	}
	
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("ERROR: %s\n", "test")
		return
	}
	defer conn.Close()
	
	// Extract stream ID from URL
	vars := mux.Vars(r)
	streamID := vars["id"]
	
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Register stream
	s.streamMgr.AddStream(streamID, conn, cancel)
	defer s.streamMgr.RemoveStream(streamID)
	
	fmt.Printf("INFO: %s\n", "test")
	
	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			fmt.Printf("INFO: %s\n", "test")
			break
		}
	}
}

func (s *Server) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	// Existing sessions handler logic
}

func (s *Server) sessionHandler(w http.ResponseWriter, r *http.Request) {
	// Existing session handler logic
}

func (s *Server) startCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("INFO: %s\n", "test")
			return
		case <-ticker.C:
			s.streamMgr.CleanupInactiveStreams(30 * time.Minute)
		}
	}
}

// StreamRequest represents a streaming request
type StreamRequest struct {
	SessionID string `json:"session_id"`
	Task      string `json:"task"`
	Stream    bool   `json:"stream"`
}
