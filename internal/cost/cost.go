// Package cost implements token cost tracking for AI providers.
// Adapted from zen-brain's cost.go
package cost

import (
	"fmt"
	"sync"
)

// Pricing represents cost per million tokens (in USD cents * 100 for precision)
// e.g., $0.14/M = 14 cents = 1400 (cents * 100)
type Pricing struct {
	InputPerMillion  int // cents * 100 per million tokens
	OutputPerMillion int // cents * 100 per million tokens
}

// Costs per provider:model (prices as of Feb 2026)
var providerCosts = map[string]Pricing{
	// DeepSeek - very cheap
	"deepseek:deepseek-chat": {InputPerMillion: 14, OutputPerMillion: 28}, // $0.14/$0.28 per M

	// Qwen - also cheap
	"qwen:qwen3-coder-30b":             {InputPerMillion: 7, OutputPerMillion: 45},  // $0.07/$0.45 per M
	"qwen:qwen3-coder-30b-a3b-instruct": {InputPerMillion: 7, OutputPerMillion: 45},

	// MiniMax
	"minimax:minimax-M2.1": {InputPerMillion: 30, OutputPerMillion: 30}, // $0.30/$0.30 per M

	// Kimi - cheap with cache
	"kimi:kimi-k2-5": {InputPerMillion: 10, OutputPerMillion: 20}, // $0.10/$0.20 per M (cached: $0.02)

	// GLM
	"glm:glm-4.7": {InputPerMillion: 50, OutputPerMillion: 50}, // estimate

	// OpenAI - more expensive
	"openai:gpt-4o-mini":  {InputPerMillion: 15, OutputPerMillion: 60},   // $0.15/$0.60 per M
	"openai:gpt-4o":       {InputPerMillion: 250, OutputPerMillion: 1000}, // $2.50/$10 per M
	"openai:gpt-4-turbo":  {InputPerMillion: 1000, OutputPerMillion: 3000},

	// Anthropic
	"anthropic:claude-3-haiku": {InputPerMillion: 25, OutputPerMillion: 125},  // $0.25/$1.25 per M
	"anthropic:claude-3-sonnet": {InputPerMillion: 300, OutputPerMillion: 1500}, // $3/$15 per M
}

// Usage tracks token usage for a session
type Usage struct {
	mu sync.Mutex

	InputTokens  int
	OutputTokens int
	TotalCost    int // cents * 100

	// Per-provider breakdown
	ByProvider map[string]*ProviderUsage
}

// ProviderUsage tracks usage per provider
type ProviderUsage struct {
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	Calls        int
	Cost         int // cents * 100
}

// NewUsage creates a new usage tracker
func NewUsage() *Usage {
	return &Usage{
		ByProvider: make(map[string]*ProviderUsage),
	}
}

// Record records token usage for a call
func (u *Usage) Record(provider, model string, inputTokens, outputTokens int) {
	u.mu.Lock()
	defer u.mu.Unlock()

	cost := Calculate(provider, model, inputTokens, outputTokens)

	u.InputTokens += inputTokens
	u.OutputTokens += outputTokens
	u.TotalCost += cost

	key := provider + ":" + model
	if u.ByProvider[key] == nil {
		u.ByProvider[key] = &ProviderUsage{
			Provider: provider,
			Model:    model,
		}
	}

	pu := u.ByProvider[key]
	pu.InputTokens += inputTokens
	pu.OutputTokens += outputTokens
	pu.Calls++
	pu.Cost += cost
}

// Calculate returns cost in cents * 100 for given tokens
func Calculate(provider, model string, inputTokens, outputTokens int) int {
	key := provider + ":" + model
	pricing, ok := providerCosts[key]
	if !ok {
		// Unknown model - use conservative estimate
		pricing = Pricing{InputPerMillion: 100, OutputPerMillion: 300}
	}

	// Cost = (tokens / 1M) * price_per_M
	// In cents * 100: (tokens * price_per_M) / 1M
	costIn := (inputTokens * pricing.InputPerMillion) / 1000000
	costOut := (outputTokens * pricing.OutputPerMillion) / 1000000

	// Minimum 1 unit if any tokens used
	if costIn == 0 && inputTokens > 0 {
		costIn = 1
	}
	if costOut == 0 && outputTokens > 0 {
		costOut = 1
	}

	return costIn + costOut
}

// FormatCost formats cost in cents * 100 to human readable
func FormatCost(cost int) string {
	cents := float64(cost) / 100.0
	if cents < 1 {
		return fmt.Sprintf("$0.%04d", cost)
	}
	return fmt.Sprintf("$%.4f", cents/100.0)
}

// Summary returns a formatted summary of usage
func (u *Usage) Summary() string {
	u.mu.Lock()
	defer u.mu.Unlock()

	return fmt.Sprintf(
		"Tokens: %d in / %d out | Cost: %s",
		u.InputTokens,
		u.OutputTokens,
		FormatCost(u.TotalCost),
	)
}

// GetProviderCosts returns the cost table for display
func GetProviderCosts() map[string]Pricing {
	return providerCosts
}
