package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/cache"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/cost"
	"github.com/neves/zen-claw/internal/providers"
	"github.com/neves/zen-claw/internal/retry"
)

// AIRouter handles AI provider selection, routing, and fallback
type AIRouter struct {
	config    *config.Config
	factory   *providers.Factory
	providers map[string]ai.Provider // Loaded providers

	// Response caching
	cache *cache.Cache

	// Cost tracking
	usageMu sync.Mutex
	usage   *cost.Usage
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
		"kimi":     cfg.Providers.Kimi,
	}

	for name, _ := range providerConfigs {
		// Skip if no API key available (check config and env vars)
		apiKey := cfg.GetAPIKey(name)
		if apiKey == "" {
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

	// Initialize response cache (1 hour TTL, 1000 entries max)
	responseCache := cache.New(1*time.Hour, 1000, true)

	return &AIRouter{
		config:    cfg,
		factory:   factory,
		providers: providersMap,
		cache:     responseCache,
		usage:     cost.NewUsage(),
	}
}

// Chat sends a chat request through the router with automatic fallback
func (r *AIRouter) Chat(ctx context.Context, req ai.ChatRequest, preferredProvider string) (*ai.ChatResponse, error) {
	// Determine provider chain (cost-optimized)
	providerChain := r.getProviderChain(preferredProvider)

	// Generate cache key from request
	cacheKey := r.computeCacheKey(preferredProvider, req)

	// Check cache first (skip for tool calls - they need fresh context)
	if len(req.Tools) == 0 {
		if cached, ok := r.cache.Get(cacheKey); ok {
			log.Printf("[AIRouter] Cache HIT - returning cached response")
			return &ai.ChatResponse{
				Content: cached,
			}, nil
		}
	}

	var lastErr error

	for _, providerName := range providerChain {
		provider, exists := r.providers[providerName]
		if !exists {
			continue
		}

		log.Printf("[AIRouter] Trying provider: %s", providerName)

		// Try this provider with retry
		resp, err := r.callWithRetry(ctx, provider, providerName, req)
		if err == nil {
			log.Printf("[AIRouter] Provider %s succeeded", providerName)

			// Track cost (estimate tokens from content length)
			// Rough estimate: 1 token ≈ 4 chars
			inputTokens := 0
			for _, msg := range req.Messages {
				inputTokens += len(msg.Content) / 4
			}
			outputTokens := len(resp.Content) / 4
			if outputTokens > 0 {
				r.usageMu.Lock()
				r.usage.Record(providerName, req.Model, inputTokens, outputTokens)
				r.usageMu.Unlock()
			}

			// Cache successful response (skip tool calls)
			if len(req.Tools) == 0 && resp.Content != "" {
				r.cache.Set(cacheKey, resp.Content)
			}

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

// callWithRetry wraps a provider call with retry logic
func (r *AIRouter) callWithRetry(ctx context.Context, provider ai.Provider, providerName string, req ai.ChatRequest) (*ai.ChatResponse, error) {
	cfg := retry.Config{
		Enabled:     true,
		MaxAttempts: 2, // 1 initial + 2 retries = 3 total attempts
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    10 * time.Second,
	}

	var resp *ai.ChatResponse

	_, _, err := retry.Do(ctx, cfg, func() (string, int, error) {
		var callErr error
		resp, callErr = provider.Chat(ctx, req)
		if callErr != nil {
			return "", 0, callErr
		}
		return resp.Content, 0, nil
	})

	return resp, err
}

// computeCacheKey generates a deterministic cache key from request
func (r *AIRouter) computeCacheKey(provider string, req ai.ChatRequest) string {
	// Only include messages in key (not tools - tool requests shouldn't be cached anyway)
	data := struct {
		Provider string       `json:"provider"`
		Model    string       `json:"model"`
		Messages []ai.Message `json:"messages"`
	}{
		Provider: provider,
		Model:    req.Model,
		Messages: req.Messages,
	}

	bytes, _ := json.Marshal(data)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

// GetUsageSummary returns current usage summary
func (r *AIRouter) GetUsageSummary() string {
	r.usageMu.Lock()
	defer r.usageMu.Unlock()
	return r.usage.Summary()
}

// GetCacheStats returns cache statistics
func (r *AIRouter) GetCacheStats() (hits, misses, size int, hitRate float64) {
	return r.cache.Stats()
}

// getProviderChain returns provider chain based on preference and cost optimization
func (r *AIRouter) getProviderChain(preferred string) []string {
	// If a specific provider is requested, use only that provider
	// Don't fallback to other providers with wrong model names
	if preferred != "" {
		if _, exists := r.providers[preferred]; exists {
			return []string{preferred}
		}
		// If requested provider doesn't exist, fall through to default
	}

	// No specific provider requested, use cost-optimized chain
	chain := []string{}

	// Start with default provider if available
	defaultProvider := r.config.Default.Provider
	if defaultProvider != "" {
		if _, exists := r.providers[defaultProvider]; exists {
			chain = append(chain, defaultProvider)
		}
	}

	// Fallback chain from config (user-defined order)
	costOptimized := r.config.GetFallbackOrder()

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
