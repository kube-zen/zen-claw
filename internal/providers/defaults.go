// Package providers contains AI provider implementations and configuration.
package providers

import "strings"

// ProviderDefaults contains default configuration for each provider.
type ProviderDefaults struct {
	Model   string // Default model
	BaseURL string // Default API base URL
}

// Defaults maps provider names to their default configuration.
var Defaults = map[string]ProviderDefaults{
	"deepseek": {
		Model:   "deepseek-chat",
		BaseURL: "https://api.deepseek.com/v1",
	},
	"qwen": {
		Model:   "qwen3-coder-30b-a3b-instruct",
		BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
	},
	"glm": {
		Model:   "glm-4.7",
		BaseURL: "https://open.bigmodel.cn/api/paas/v4",
	},
	"minimax": {
		Model:   "minimax-M2.1",
		BaseURL: "https://api.minimax.chat/v1",
	},
	"openai": {
		Model:   "gpt-4o-mini",
		BaseURL: "https://api.openai.com/v1",
	},
	"kimi": {
		Model:   "kimi-k2-5",
		BaseURL: "https://api.moonshot.cn/v1",
	},
	"anthropic": {
		Model:   "claude-sonnet-4-20250514",
		BaseURL: "https://api.anthropic.com/v1",
	},
}

// ValidProviders returns a list of all valid provider names.
var ValidProviders = []string{"deepseek", "qwen", "glm", "minimax", "openai", "kimi", "anthropic"}

// DefaultProvider is the default provider when none is specified.
const DefaultProvider = "deepseek"

// GetDefaultModel returns the default model for a provider.
func GetDefaultModel(provider string) string {
	provider = strings.ToLower(provider)
	if def, ok := Defaults[provider]; ok {
		return def.Model
	}
	return Defaults[DefaultProvider].Model
}

// GetDefaultBaseURL returns the default base URL for a provider.
func GetDefaultBaseURL(provider string) string {
	provider = strings.ToLower(provider)
	if def, ok := Defaults[provider]; ok {
		return def.BaseURL
	}
	return ""
}

// InferProviderFromModel infers the provider from a model name.
func InferProviderFromModel(modelName string) string {
	modelName = strings.ToLower(modelName)

	switch {
	case strings.Contains(modelName, "qwen"):
		return "qwen"
	case strings.Contains(modelName, "deepseek"):
		return "deepseek"
	case strings.Contains(modelName, "glm"):
		return "glm"
	case strings.Contains(modelName, "minimax") || strings.Contains(modelName, "abab"):
		return "minimax"
	case strings.Contains(modelName, "gpt"):
		return "openai"
	case strings.Contains(modelName, "kimi") || strings.Contains(modelName, "moonshot"):
		return "kimi"
	case strings.Contains(modelName, "claude") || strings.Contains(modelName, "anthropic"):
		return "anthropic"
	default:
		return ""
	}
}

// IsValidProvider checks if a provider name is valid.
func IsValidProvider(provider string) bool {
	provider = strings.ToLower(provider)
	_, ok := Defaults[provider]
	return ok
}
