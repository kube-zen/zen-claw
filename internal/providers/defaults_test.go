package providers

import (
	"testing"
)

func TestGetDefaultModel(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"deepseek", "deepseek-chat"},
		{"qwen", "qwen3-coder-30b-a3b-instruct"},
		{"glm", "glm-4.7"},
		{"minimax", "minimax-M2.1"},
		{"openai", "gpt-4o-mini"},
		{"kimi", "kimi-k2-5"},
		// Unknown providers fall back to default (deepseek)
		{"unknown", "deepseek-chat"},
		{"", "deepseek-chat"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := GetDefaultModel(tt.provider)
			if got != tt.want {
				t.Errorf("GetDefaultModel(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestGetDefaultBaseURL(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"deepseek", "https://api.deepseek.com/v1"},
		{"qwen", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		{"glm", "https://open.bigmodel.cn/api/paas/v4"},
		{"minimax", "https://api.minimax.chat/v1"},
		{"openai", "https://api.openai.com/v1"},
		{"kimi", "https://api.moonshot.cn/v1"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := GetDefaultBaseURL(tt.provider)
			if got != tt.want {
				t.Errorf("GetDefaultBaseURL(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestInferProviderFromModel(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		// DeepSeek models
		{"deepseek-chat", "deepseek"},
		{"deepseek-reasoner", "deepseek"},
		{"DeepSeek-V2", "deepseek"},

		// Qwen models
		{"qwen-plus", "qwen"},
		{"qwen3-coder-30b", "qwen"},
		{"Qwen-Max", "qwen"},

		// GLM models
		{"glm-4", "glm"},
		{"glm-4.7", "glm"},
		{"GLM-3-turbo", "glm"},

		// Minimax models
		{"minimax-M2.1", "minimax"},
		{"abab6.5s", "minimax"},
		{"abab6.5", "minimax"},

		// OpenAI models
		{"gpt-4o", "openai"},
		{"gpt-4o-mini", "openai"},
		{"GPT-3.5-turbo", "openai"},

		// Kimi models
		{"kimi-k2-5", "kimi"},
		{"moonshot-v1-8k", "kimi"},
		{"Moonshot-V1-128k", "kimi"},

		// Unknown models
		{"claude-3", ""},
		{"unknown-model", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := InferProviderFromModel(tt.model)
			if got != tt.want {
				t.Errorf("InferProviderFromModel(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsValidProvider(t *testing.T) {
	validProviders := []string{"deepseek", "qwen", "glm", "minimax", "openai", "kimi"}
	invalidProviders := []string{"claude", "anthropic", "unknown", ""}

	for _, p := range validProviders {
		t.Run("valid_"+p, func(t *testing.T) {
			if !IsValidProvider(p) {
				t.Errorf("IsValidProvider(%q) = false, want true", p)
			}
		})
	}

	for _, p := range invalidProviders {
		t.Run("invalid_"+p, func(t *testing.T) {
			if IsValidProvider(p) {
				t.Errorf("IsValidProvider(%q) = true, want false", p)
			}
		})
	}
}

func TestDefaultsMapCompleteness(t *testing.T) {
	// Ensure all providers in Defaults have both Model and BaseURL
	for provider, defaults := range Defaults {
		t.Run(provider, func(t *testing.T) {
			if defaults.Model == "" {
				t.Errorf("Provider %q has empty Model", provider)
			}
			if defaults.BaseURL == "" {
				t.Errorf("Provider %q has empty BaseURL", provider)
			}
		})
	}
}
