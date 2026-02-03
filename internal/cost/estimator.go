package cost

import (
	"fmt"
	"strings"
)

// ProviderPricing defines token costs per million tokens (USD)
type ProviderPricing struct {
	InputPerMillion  float64 // Cost per 1M input tokens
	OutputPerMillion float64 // Cost per 1M output tokens
	CachedPerMillion float64 // Cost per 1M cached input tokens (if supported)
}

// Known provider pricing (as of 2024)
var ProviderPrices = map[string]ProviderPricing{
	"deepseek": {
		InputPerMillion:  0.14,  // $0.14/M input
		OutputPerMillion: 0.28,  // $0.28/M output
		CachedPerMillion: 0.014, // 10x cheaper when cached
	},
	"qwen": {
		InputPerMillion:  0.50, // Approximate
		OutputPerMillion: 1.50,
		CachedPerMillion: 0.05,
	},
	"kimi": {
		InputPerMillion:  0.10, // $0.10/M - very cheap
		OutputPerMillion: 0.30,
		CachedPerMillion: 0.01,
	},
	"glm": {
		InputPerMillion:  0.50,
		OutputPerMillion: 0.50,
		CachedPerMillion: 0.05,
	},
	"minimax": {
		InputPerMillion:  0.15,
		OutputPerMillion: 0.60,
		CachedPerMillion: 0.015,
	},
	"openai": {
		InputPerMillion:  2.50,  // GPT-4o
		OutputPerMillion: 10.00,
		CachedPerMillion: 1.25,
	},
	"anthropic": {
		InputPerMillion:  3.00,  // Claude Sonnet
		OutputPerMillion: 15.00,
		CachedPerMillion: 0.30, // 90% discount with prompt caching!
	},
	"gemini": {
		InputPerMillion:  1.25,
		OutputPerMillion: 5.00,
		CachedPerMillion: 0.125,
	},
}

// Estimate represents a cost estimate for a task
type Estimate struct {
	Provider         string
	Model            string
	InputTokens      int
	OutputTokens     int // Estimated
	CachedTokens     int // Tokens that might be cached
	EstimatedCostUSD float64
	CostBreakdown    string
	Warning          string
}

// Estimator estimates costs before running tasks
type Estimator struct {
	prices map[string]ProviderPricing
}

// NewEstimator creates a cost estimator
func NewEstimator() *Estimator {
	return &Estimator{
		prices: ProviderPrices,
	}
}

// EstimateTask estimates cost for a task
func (e *Estimator) EstimateTask(provider, model, systemPrompt, userPrompt string, hasTools bool) Estimate {
	est := Estimate{
		Provider: provider,
		Model:    model,
	}

	// Estimate input tokens
	systemTokens := estimateTokens(systemPrompt)
	userTokens := estimateTokens(userPrompt)
	toolTokens := 0
	if hasTools {
		toolTokens = 2000 // ~2K tokens for tool definitions
	}

	est.InputTokens = systemTokens + userTokens + toolTokens

	// Estimate output tokens based on task type
	est.OutputTokens = estimateOutputTokens(userPrompt)

	// System prompt is often cacheable
	est.CachedTokens = systemTokens

	// Get pricing
	pricing, ok := e.prices[provider]
	if !ok {
		pricing = ProviderPricing{
			InputPerMillion:  1.0,
			OutputPerMillion: 2.0,
			CachedPerMillion: 0.1,
		}
		est.Warning = fmt.Sprintf("Unknown provider %s, using default pricing", provider)
	}

	// Calculate cost
	uncachedInput := est.InputTokens - est.CachedTokens
	inputCost := float64(uncachedInput) / 1_000_000 * pricing.InputPerMillion
	cachedCost := float64(est.CachedTokens) / 1_000_000 * pricing.CachedPerMillion
	outputCost := float64(est.OutputTokens) / 1_000_000 * pricing.OutputPerMillion

	est.EstimatedCostUSD = inputCost + cachedCost + outputCost

	// Build breakdown
	est.CostBreakdown = fmt.Sprintf(
		"Input: %d tokens ($%.4f) + Cached: %d tokens ($%.4f) + Output: ~%d tokens ($%.4f)",
		uncachedInput, inputCost,
		est.CachedTokens, cachedCost,
		est.OutputTokens, outputCost,
	)

	return est
}

// EstimateSession estimates cost for a multi-turn session
func (e *Estimator) EstimateSession(provider string, turns int, avgInputTokens, avgOutputTokens int) Estimate {
	est := Estimate{
		Provider:     provider,
		InputTokens:  turns * avgInputTokens,
		OutputTokens: turns * avgOutputTokens,
	}

	pricing, ok := e.prices[provider]
	if !ok {
		pricing = ProviderPricing{InputPerMillion: 1.0, OutputPerMillion: 2.0}
	}

	inputCost := float64(est.InputTokens) / 1_000_000 * pricing.InputPerMillion
	outputCost := float64(est.OutputTokens) / 1_000_000 * pricing.OutputPerMillion
	est.EstimatedCostUSD = inputCost + outputCost

	est.CostBreakdown = fmt.Sprintf(
		"%d turns × (%d in + %d out) = $%.4f",
		turns, avgInputTokens, avgOutputTokens, est.EstimatedCostUSD,
	)

	return est
}

// FormatEstimate formats an estimate for display
func (e *Estimate) Format() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Cost Estimate (%s/%s)\n", e.Provider, e.Model))
	sb.WriteString(fmt.Sprintf("  Input:  %d tokens\n", e.InputTokens))
	sb.WriteString(fmt.Sprintf("  Output: ~%d tokens (estimated)\n", e.OutputTokens))
	if e.CachedTokens > 0 {
		sb.WriteString(fmt.Sprintf("  Cached: %d tokens (system prompt)\n", e.CachedTokens))
	}
	sb.WriteString(fmt.Sprintf("  Cost:   $%.4f USD\n", e.EstimatedCostUSD))
	sb.WriteString(fmt.Sprintf("  %s\n", e.CostBreakdown))
	if e.Warning != "" {
		sb.WriteString(fmt.Sprintf("  ⚠️  %s\n", e.Warning))
	}
	return sb.String()
}

// FormatCompact formats estimate in one line
func (e *Estimate) FormatCompact() string {
	return fmt.Sprintf("~%d tokens, ~$%.4f (%s)", e.InputTokens+e.OutputTokens, e.EstimatedCostUSD, e.Provider)
}

// estimateTokens estimates token count from text
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Rough estimate: ~4 chars per token
	return len(text) / 4
}

// estimateOutputTokens estimates output based on task type
func estimateOutputTokens(prompt string) int {
	promptLower := strings.ToLower(prompt)

	// Short answers
	shortPatterns := []string{
		"what is", "how many", "which", "when", "where", "who",
		"yes or no", "true or false", "list", "name",
		"summarize", "tldr", "briefly",
	}
	for _, p := range shortPatterns {
		if strings.Contains(promptLower, p) {
			return 500
		}
	}

	// Code generation
	codePatterns := []string{
		"write", "create", "implement", "generate", "build",
		"refactor", "add", "modify", "fix", "update",
		"function", "class", "method",
	}
	for _, p := range codePatterns {
		if strings.Contains(promptLower, p) {
			return 2000
		}
	}

	// Complex analysis
	analysisPatterns := []string{
		"analyze", "review", "explain", "compare", "evaluate",
		"architecture", "design", "security",
	}
	for _, p := range analysisPatterns {
		if strings.Contains(promptLower, p) {
			return 3000
		}
	}

	// Default
	return 1000
}

// CompareProviders shows cost comparison across providers
func (e *Estimator) CompareProviders(inputTokens, outputTokens int) string {
	var sb strings.Builder
	sb.WriteString("Provider Cost Comparison:\n")
	sb.WriteString(fmt.Sprintf("  (for %d input + %d output tokens)\n\n", inputTokens, outputTokens))

	providers := []string{"deepseek", "kimi", "qwen", "glm", "minimax", "anthropic", "openai"}

	for _, p := range providers {
		pricing, ok := e.prices[p]
		if !ok {
			continue
		}
		inputCost := float64(inputTokens) / 1_000_000 * pricing.InputPerMillion
		outputCost := float64(outputTokens) / 1_000_000 * pricing.OutputPerMillion
		total := inputCost + outputCost
		sb.WriteString(fmt.Sprintf("  %-10s: $%.4f\n", p, total))
	}

	return sb.String()
}
