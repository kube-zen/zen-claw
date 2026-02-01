package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
)

// Server represents the Zen Claw gateway server
type Server struct {
	config  *config.Config
	server  *http.Server
	mu      sync.RWMutex
	running bool
	pidFile string
	sessions map[string]*Session
}

// Session represents a user session
type Session struct {
	ID        string
	CreatedAt time.Time
	LastActive time.Time
	Messages  []ai.Message
}

// NewServer creates a new gateway server
func NewServer(cfg *config.Config) *Server {
	srv := &Server{
		config:   cfg,
		pidFile:  "/tmp/zen-claw-gateway.pid",
		sessions: make(map[string]*Session),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.healthHandler)
	mux.HandleFunc("/chat", srv.chatHandler)
	mux.HandleFunc("/chat/stream", srv.chatStreamHandler)
	mux.HandleFunc("/sessions", srv.sessionsHandler)
	mux.HandleFunc("/sessions/", srv.sessionHandler)
	mux.HandleFunc("/tools", srv.toolsHandler)
	mux.HandleFunc("/tools/execute", srv.toolsExecuteHandler)
	mux.HandleFunc("/", srv.defaultHandler)

	srv.server = &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	return srv
}

// Start starts the gateway server
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("gateway already running")
	}
	s.running = true
	s.mu.Unlock()

	// Write PID file
	if err := s.writePID(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting Zen Claw gateway on %s", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	s.waitForShutdown()

	return nil
}

// Stop stops the gateway server
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return fmt.Errorf("gateway not running")
	}
	s.running = false
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	// Remove PID file
	os.Remove(s.pidFile)

	log.Println("Gateway stopped")
	return nil
}

// Restart restarts the gateway server
func (s *Server) Restart() error {
	if err := s.Stop(); err != nil {
		return fmt.Errorf("failed to stop gateway: %w", err)
	}

	// Small delay before restart
	time.Sleep(1 * time.Second)

	return s.Start()
}

// Status returns the gateway status
func (s *Server) Status() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.running {
		return "running"
	}
	return "stopped"
}

// waitForShutdown waits for interrupt signals
func (s *Server) waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received")

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.server.Shutdown(ctx)
	os.Remove(s.pidFile)
}

// writePID writes the process ID to file
func (s *Server) writePID() error {
	pid := os.Getpid()
	return os.WriteFile(s.pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

// HTTP handlers
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","service":"zen-claw-gateway","version":"0.1.0"}`)
}

func (s *Server) chatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req struct {
		Message  string `json:"message"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		SessionID string `json:"session_id"`
		Stream   bool   `json:"stream"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Get or create session
	session := s.getOrCreateSession(req.SessionID)
	
	// Add user message to session
	session.Messages = append(session.Messages, ai.Message{
		Role:    "user",
		Content: req.Message,
	})

	// Get provider from request or config default
	provider := req.Provider
	if provider == "" {
		provider = s.config.Default.Provider
	}

	// Get model from request or config default
	model := req.Model
	if model == "" {
		model = s.config.GetModel(provider)
	}

	// Create AI provider
	factory := providers.NewFactory(s.config)
	aiProvider, err := factory.CreateProvider(provider)
	if err != nil {
		// Fall back to mock provider
		log.Printf("Failed to create provider %s: %v, falling back to mock", provider, err)
		aiProvider = providers.NewMockProvider(false)
	}

	// Get available tools
	availableTools := s.getAvailableTools()
	
	// Create chat request
	chatReq := ai.ChatRequest{
		Model:    model,
		Messages: session.Messages,
		Tools:    availableTools,
	}

	// Get AI response
	resp, err := aiProvider.Chat(context.Background(), chatReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("AI processing failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Handle tool calls if any
	if len(resp.ToolCalls) > 0 {
		// Execute tool calls
		toolResults := s.executeToolCalls(resp.ToolCalls)
		
		// Add tool call messages to session
		for range resp.ToolCalls {
			session.Messages = append(session.Messages, ai.Message{
				Role:    "assistant",
				Content: "",
				// Note: We need a way to store tool calls in messages
			})
		}
		
		// Add tool results to session
		for _, result := range toolResults {
			session.Messages = append(session.Messages, ai.Message{
				Role:    "tool",
				Content: result,
			})
		}
		
		// For now, just return the first tool call result
		// In a full implementation, we'd send results back to AI for follow-up
		if len(toolResults) > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response":   toolResults[0],
				"tool_calls": len(resp.ToolCalls),
				"provider":   provider,
				"model":      model,
				"session_id": session.ID,
				"finish_reason": "tool_calls",
			})
			return
		}
	}

	// Add assistant response to session
	session.Messages = append(session.Messages, ai.Message{
		Role:    "assistant",
		Content: resp.Content,
	})

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response":   resp.Content,
		"provider":   provider,
		"model":      model,
		"session_id": session.ID,
		"finish_reason": resp.FinishReason,
	})
}

func (s *Server) chatStreamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req struct {
		Message  string `json:"message"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		SessionID string `json:"session_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Set headers for Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get or create session
	session := s.getOrCreateSession(req.SessionID)
	
	// Add user message to session
	session.Messages = append(session.Messages, ai.Message{
		Role:    "user",
		Content: req.Message,
	})

	// Get provider from request or config default
	provider := req.Provider
	if provider == "" {
		provider = s.config.Default.Provider
	}

	// Get model from request or config default
	model := req.Model
	if model == "" {
		model = s.config.GetModel(provider)
	}

	// For now, we'll simulate streaming with mock provider
	// In a real implementation, we'd stream from the AI provider
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Create AI provider
	factory := providers.NewFactory(s.config)
	aiProvider, err := factory.CreateProvider(provider)
	if err != nil {
		// Fall back to mock provider
		aiProvider = providers.NewMockProvider(false)
	}

	// Create chat request
	chatReq := ai.ChatRequest{
		Model:    model,
		Messages: session.Messages,
	}

	// Get AI response
	resp, err := aiProvider.Chat(context.Background(), chatReq)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	// Simulate streaming by sending response in chunks
	responseText := resp.Content
	chunkSize := 20
	for i := 0; i < len(responseText); i += chunkSize {
		end := i + chunkSize
		if end > len(responseText) {
			end = len(responseText)
		}
		
		chunk := responseText[i:end]
		fmt.Fprintf(w, "data: %s\n\n", jsonEscape(chunk))
		flusher.Flush()
		
		// Small delay to simulate streaming
		time.Sleep(50 * time.Millisecond)
	}

	// Send completion event
	fmt.Fprintf(w, "event: done\ndata: {\"finish_reason\":\"%s\"}\n\n", resp.FinishReason)
	flusher.Flush()

	// Add assistant response to session
	session.Messages = append(session.Messages, ai.Message{
		Role:    "assistant",
		Content: resp.Content,
	})
}

func (s *Server) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]map[string]interface{}, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, map[string]interface{}{
			"id":          session.ID,
			"created_at":  session.CreatedAt.Format(time.RFC3339),
			"last_active": session.LastActive.Format(time.RFC3339),
			"message_count": len(session.Messages),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

func (s *Server) sessionHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Path[len("/sessions/"):]
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	session, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	if r.Method == http.MethodGet {
		// Get session details
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          session.ID,
			"created_at":  session.CreatedAt.Format(time.RFC3339),
			"last_active": session.LastActive.Format(time.RFC3339),
			"messages":    session.Messages,
		})
	} else if r.Method == http.MethodDelete {
		// Delete session
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deleted": true,
			"id":      sessionID,
		})
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) toolsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// List available tools on gateway
	tools := []map[string]interface{}{
		{
			"name":        "exec",
			"description": "Execute shell command on gateway host",
			"parameters": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Shell command to execute",
				},
			},
		},
		{
			"name":        "list_dir",
			"description": "List directory contents on gateway host",
			"parameters": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory path (default: current directory)",
					"optional":    true,
				},
			},
		},
		{
			"name":        "read_file",
			"description": "Read file from gateway host",
			"parameters": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to read",
				},
			},
		},
		{
			"name":        "system_info",
			"description": "Get system information from gateway host",
			"parameters":   map[string]interface{}{},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": tools,
	})
}

func (s *Server) toolsExecuteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Tool string                 `json:"tool"`
		Args map[string]interface{} `json:"args"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Tool == "" {
		http.Error(w, "Tool name is required", http.StatusBadRequest)
		return
	}

	// Execute tool
	var result interface{}
	var err error

	switch req.Tool {
	case "exec":
		result, err = s.executeCommand(req.Args)
	case "list_dir":
		result, err = s.listDirectory(req.Args)
	case "read_file":
		result, err = s.readFile(req.Args)
	case "system_info":
		result, err = s.getSystemInfo(req.Args)
	default:
		http.Error(w, fmt.Sprintf("Unknown tool: %s", req.Tool), http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Tool execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"result": result,
		"tool":   req.Tool,
		"success": true,
	})
}

func (s *Server) executeCommand(args map[string]interface{}) (interface{}, error) {
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// ⚠️ DANGEROUS: Executing arbitrary commands!
	// In production, you should:
	// 1. Validate commands
	// 2. Use allowlists
	// 3. Run in sandbox
	// 4. Add timeouts
	// 5. Log all executions
	
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	
	result := map[string]interface{}{
		"command": command,
		"output":  string(output),
	}
	
	if err != nil {
		result["error"] = err.Error()
		result["exit_code"] = cmd.ProcessState.ExitCode()
	}
	
	return result, nil
}

func (s *Server) listDirectory(args map[string]interface{}) (interface{}, error) {
	path := "."
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}

	// Safety: prevent directory traversal
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("directory traversal not allowed")
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var files []map[string]interface{}
	for _, entry := range entries {
		info, _ := entry.Info()
		files = append(files, map[string]interface{}{
			"name":    entry.Name(),
			"is_dir":  entry.IsDir(),
			"size":    info.Size(),
			"mode":    info.Mode().String(),
			"mod_time": info.ModTime().Format(time.RFC3339),
		})
	}

	return map[string]interface{}{
		"path":  path,
		"files": files,
	}, nil
}

func (s *Server) readFile(args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Safety checks
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("directory traversal not allowed")
	}
	
	// Limit file size (1MB)
	const maxSize = 1 * 1024 * 1024
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	
	if info.Size() > maxSize {
		return nil, fmt.Errorf("file too large (%d bytes > %d bytes)", info.Size(), maxSize)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return map[string]interface{}{
		"path":    path,
		"content": string(content),
		"size":    len(content),
	}, nil
}

func (s *Server) getSystemInfo(args map[string]interface{}) (interface{}, error) {
	// Get hostname
	hostname, _ := os.Hostname()
	
	// Get current directory
	cwd, _ := os.Getwd()
	
	// Get environment info
	env := os.Environ()
	// Limit to first 20 env vars for brevity
	if len(env) > 20 {
		env = env[:20]
	}

	return map[string]interface{}{
		"hostname":    hostname,
		"current_dir": cwd,
		"gateway_pid": os.Getpid(),
		"env_vars":    env,
		"time":        time.Now().Format(time.RFC3339),
	}, nil
}

func (s *Server) defaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Zen Claw Gateway v0.1.0\n")
	fmt.Fprintf(w, "Endpoints:\n")
	fmt.Fprintf(w, "  GET  /health           - Health check\n")
	fmt.Fprintf(w, "  POST /chat             - Chat with AI (JSON)\n")
	fmt.Fprintf(w, "  POST /chat/stream      - Stream chat with SSE\n")
	fmt.Fprintf(w, "  GET  /sessions         - List sessions\n")
	fmt.Fprintf(w, "  GET  /sessions/{id}    - Get session\n")
	fmt.Fprintf(w, "  DELETE /sessions/{id}  - Delete session\n")
	fmt.Fprintf(w, "  GET  /tools            - List available tools\n")
	fmt.Fprintf(w, "  POST /tools/execute    - Execute tool on gateway host\n")
}

// Helper functions
func (s *Server) getOrCreateSession(sessionID string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sessionID == "" {
		// Create new session
		sessionID = generateSessionID()
	}

	session, exists := s.sessions[sessionID]
	if !exists {
		session = &Session{
			ID:        sessionID,
			CreatedAt: time.Now(),
			LastActive: time.Now(),
			Messages:  make([]ai.Message, 0),
		}
		s.sessions[sessionID] = session
	} else {
		session.LastActive = time.Now()
	}

	return session
}

func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

func (s *Server) getAvailableTools() []ai.ToolDefinition {
	return []ai.ToolDefinition{
		{
			Name:        "exec",
			Description: "Execute shell command on gateway host",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Shell command to execute",
					},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "list_dir",
			Description: "List directory contents on gateway host",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Directory path (default: current directory)",
					},
				},
			},
		},
		{
			Name:        "read_file",
			Description: "Read file from gateway host",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path to read",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "system_info",
			Description: "Get system information from gateway host",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func (s *Server) executeToolCalls(toolCalls []ai.ToolCall) []string {
	var results []string
	
	for _, toolCall := range toolCalls {
		var result interface{}
		var err error
		
		// Log tool call for debugging
		log.Printf("🔧 Tool call: %s, args: %v", toolCall.Name, toolCall.Args)
		
		// Handle different argument formats
		args := s.parseToolArgs(toolCall.Args)
		
		switch toolCall.Name {
		case "exec":
			result, err = s.executeCommand(args)
		case "list_dir":
			result, err = s.listDirectory(args)
		case "read_file":
			result, err = s.readFile(args)
		case "system_info":
			result, err = s.getSystemInfo(args)
		default:
			result = fmt.Sprintf("Unknown tool: %s", toolCall.Name)
		}
		
		if err != nil {
			results = append(results, fmt.Sprintf("Tool %s failed: %v", toolCall.Name, err))
		} else {
			// Convert result to string
			if str, ok := result.(string); ok {
				results = append(results, str)
			} else if bytes, err := json.Marshal(result); err == nil {
				results = append(results, string(bytes))
			} else {
				results = append(results, fmt.Sprintf("%v", result))
			}
		}
	}
	
	return results
}

func (s *Server) parseToolArgs(rawArgs map[string]interface{}) map[string]interface{} {
	args := make(map[string]interface{})
	
	// Check for _raw field (JSON string from some AI providers)
	if rawJSON, ok := rawArgs["_raw"].(string); ok {
		log.Printf("📝 Parsing raw JSON: %s", rawJSON)
		var parsedArgs map[string]interface{}
		if err := json.Unmarshal([]byte(rawJSON), &parsedArgs); err == nil {
			// Use parsed args
			for k, v := range parsedArgs {
				args[k] = v
			}
			return args
		}
	}
	
	// Otherwise use args as-is
	for k, v := range rawArgs {
		args[k] = v
	}
	
	return args
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}