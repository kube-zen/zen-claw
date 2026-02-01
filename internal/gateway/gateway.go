package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
)

// Gateway represents the HTTP gateway server
type Gateway struct {
	config  *config.Config
	server  *http.Server
	mu      sync.RWMutex
	running bool
	pidFile string
}

// NewGateway creates a new gateway instance
func NewGateway(cfg *config.Config) *Gateway {
	// Create gateway
	gw := &Gateway{
		config:  cfg,
		pidFile: "/tmp/zen-claw-gateway.pid",
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", gw.healthHandler)
	mux.HandleFunc("/chat", gw.chatHandler)
	mux.HandleFunc("/", gw.defaultHandler)

	gw.server = &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	return gw
}

// Start starts the gateway server
func (g *Gateway) Start() error {
	g.mu.Lock()
	if g.running {
		g.mu.Unlock()
		return fmt.Errorf("gateway already running")
	}
	g.running = true
	g.mu.Unlock()

	// Write PID file
	if err := g.writePID(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting Zen Claw gateway on %s", g.server.Addr)
		if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	g.waitForShutdown()

	return nil
}

// Stop stops the gateway server
func (g *Gateway) Stop() error {
	g.mu.Lock()
	if !g.running {
		g.mu.Unlock()
		return fmt.Errorf("gateway not running")
	}
	g.running = false
	g.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := g.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	// Remove PID file
	os.Remove(g.pidFile)

	log.Println("Gateway stopped")
	return nil
}

// Restart restarts the gateway server
func (g *Gateway) Restart() error {
	if err := g.Stop(); err != nil {
		return fmt.Errorf("failed to stop gateway: %w", err)
	}

	// Small delay before restart
	time.Sleep(1 * time.Second)

	return g.Start()
}

// Status returns the gateway status
func (g *Gateway) Status() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.running {
		return "running"
	}
	return "stopped"
}

// waitForShutdown waits for interrupt signals
func (g *Gateway) waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received")

	g.mu.Lock()
	g.running = false
	g.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	g.server.Shutdown(ctx)
	os.Remove(g.pidFile)
}

// writePID writes the process ID to file
func (g *Gateway) writePID() error {
	pid := os.Getpid()
	return os.WriteFile(g.pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

// readPID reads the process ID from file
func (g *Gateway) readPID() (int, error) {
	data, err := os.ReadFile(g.pidFile)
	if err != nil {
		return 0, err
	}

	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	return pid, err
}

// HTTP handlers
func (g *Gateway) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","service":"zen-claw-gateway"}`)
}

func (g *Gateway) chatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	message := r.FormValue("message")
	if message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Get provider from query param or config default
	provider := r.FormValue("provider")
	if provider == "" {
		provider = g.config.Default.Provider
	}

	// Get model from query param or config default
	model := r.FormValue("model")
	if model == "" {
		model = g.config.GetModel(provider)
	}

	// Create AI provider
	factory := providers.NewFactory(g.config)
	aiProvider, err := factory.CreateProvider(provider)
	if err != nil {
		// Fall back to mock provider
		log.Printf("Failed to create provider %s: %v, falling back to mock", provider, err)
		aiProvider = providers.NewMockProvider(false)
	}

	// Simple chat without tools for now
	req := ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "user", Content: message},
		},
		Model: model,
	}

	resp, err := aiProvider.Chat(context.Background(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("AI processing failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"response":"%s","provider":"%s","model":"%s"}`, 
		resp.Content, provider, model)
}

func (g *Gateway) defaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Zen Claw Gateway\n")
	fmt.Fprintf(w, "Endpoints:\n")
	fmt.Fprintf(w, "  GET  /health - Health check\n")
	fmt.Fprintf(w, "  POST /chat   - Chat with AI (message, provider, model params)\n")
}