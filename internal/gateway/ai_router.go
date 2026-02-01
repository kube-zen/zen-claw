package gateway

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
)

// AIRouter handles AI provider selection, routing, and fallback
type AIRouter struct {
	config    *config.Config
	factory   *providers.Factory
	providers map[string]ai.Provider // Loaded providers
}

// NewAIRouter creates a new AI router
func NewAIRouter(cfg *config.Config) *AIRouter {
	factory := providers.NewFactory(cfg)
	
	// Load available providers
	providersMap := make(map[string]ai.Provider)
	
	// Try to load each configured provider
	providerConfigs := map[string]*config.ProviderConfig{
		"deepseek": cfg.Providers.DeepSeek,
		"glm":      cfg.Providers.GLM,
		"minimax":  cfg.Providers.Minimax,
		"openai":   cfg.Providers.OpenAI,
		"qwen":     cfg.Providers.Qwen,
	}
	
	for name, providerCfg := range providerConfigs {
		// Skip if API key is empty or template
		if providerCfg.APIKey == "" || 
		   strings.HasPrefix(providerCfg.APIKey, "${") {
			continue
		}
		
		provider, err := factory.CreateProvider(name)
		if err != nil {
			log.Printf("Warning: Failed to load provider %s: %v", name, err)
			continue
		}
		
		providersMap[name] = provider
		log.Printf("Loaded AI provider: %s", name)
	}
	
	if len(providersMap) == 0 {
		log.Printf("Warning: No AI providers loaded!")
	}
	
	return &AIRouter{
		config:    cfg,
		factory:   factory,
		providers: providersMap,
	}
}

// Chat sends a chat request through the router with automatic fallback
func (r *AIRouter) Chat(ctx context.Context, req ai.ChatRequest, preferredProvider string) (*ai.ChatResponse, error) {
	// Determine provider chain (cost-optimized)
	providerChain := r.getProviderChain(preferredProvider)
	
	var lastErr error
	
	for _, providerName := range providerChain {
		provider, exists := r.providers[providerName]
		if !exists {
			continue
		}
		
		log.Printf("[AIRouter] Trying provider: %s", providerName)
		
		// Try this provider
		resp, err := provider.Chat(ctx, req)
		if err == nil {
			log.Printf("[AIRouter] Provider %s succeeded", providerName)
			return resp, nil
		}
		
		log.Printf("[AIRouter] Provider %s failed: %v", providerName, err)
		lastErr = err
		
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// Continue to next provider
		}
	}
	
	return nil, fmt.Errorf("all providers failed. Last error: %w", lastErr)
}

// getProviderChain returns provider chain based on preference and cost optimization
func (r *AIRouter) getProviderChain(preferred string) []string {
	// Start with preferred provider if specified and available
	chain := []string{}
	if preferred != "" {
		if _, exists := r.providers[preferred]; exists {
			chain = append(chain, preferred)
		}
	}
	
	// Add default provider if not already in chain
	defaultProvider := r.config.Default.Provider
	if defaultProvider != "" && defaultProvider != preferred {
		if _, exists := r.providers[defaultProvider]; exists {
			chain = append(chain, defaultProvider)
		}
	}
	
	// Cost-optimized fallback chain (cheapest first)
	costOptimized := []string{"deepseek", "glm", "minimax", "qwen", "openai"}
	
	for _, provider := range costOptimized {
		// Skip if already in chain or not available
		if contains(chain, provider) {
			continue
		}
		if _, exists := r.providers[provider]; exists {
			chain = append(chain, provider)
		}
	}
	
	return chain
}

// GetAvailableProviders returns list of available providers
func (r *AIRouter) GetAvailableProviders() []string {
	var providers []string
	for name := range r.providers {
		providers = append(providers, name)
	}
	return providers
}

// GetProvider returns a specific provider
func (r *AIRouter) GetProvider(name string) (ai.Provider, bool) {
	provider, exists := r.providers[name]
	return provider, exists
}

// TestProviders tests all loaded providers
func (r *AIRouter) TestProviders(ctx context.Context) map[string]error {
	results := make(map[string]error)
	
	for name, provider := range r.providers {
		// Simple test request
		testReq := ai.ChatRequest{
			Model: "test",
			Messages: []ai.Message{
				{Role: "user", Content: "Hello"},
			},
			MaxTokens: 5,
		}
		
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := provider.Chat(ctx, testReq)
		cancel()
		
		results[name] = err
	}
	
	return results
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}