package providers

import (
	"fmt"
	"os"
	"strings"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
)

// Factory creates AI providers based on configuration
type Factory struct {
	config *config.Config
}

func NewFactory(cfg *config.Config) *Factory {
	return &Factory{config: cfg}
}

// CreateProvider creates an AI provider based on name and configuration
func (f *Factory) CreateProvider(name string) (ai.Provider, error) {
	// Normalize provider name
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = f.config.Default.Provider
	}

	// Get API key from config or environment
	apiKey := f.config.GetAPIKey(name)
	model := f.config.GetModel(name)

	// Check if API key is a placeholder (starts with ${)
	if strings.HasPrefix(apiKey, "${") && strings.HasSuffix(apiKey, "}") {
		apiKey = "" // Treat as not configured
	}

	switch name {
	case "openai", "deepseek", "glm", "minimax", "qwen", "kimi":
		// All these use OpenAI-compatible API
		if apiKey == "" {
			// Try environment variable as fallback
			envVar := fmt.Sprintf("%s_API_KEY", strings.ToUpper(name))
			apiKey = os.Getenv(envVar)
			if apiKey == "" {
				return nil, fmt.Errorf("%s API key not found. Set in config or %s env", name, envVar)
			}
		}

		// Get provider-specific config
		var providerConfig *config.ProviderConfig
		switch name {
		case "openai":
			providerConfig = f.config.Providers.OpenAI
		case "deepseek":
			providerConfig = f.config.Providers.DeepSeek
		case "glm":
			providerConfig = f.config.Providers.GLM
		case "minimax":
			providerConfig = f.config.Providers.Minimax
		case "qwen":
			providerConfig = f.config.Providers.Qwen
		case "kimi":
			providerConfig = f.config.Providers.Kimi
		}

		// Create provider with proper configuration
		baseURL := ""
		if providerConfig != nil {
			baseURL = providerConfig.BaseURL
		}

		// For Kimi, set the default base URL
		if name == "kimi" && baseURL == "" {
			baseURL = "https://api.moonshot.cn/v1"
		}

		// Create OpenAI-compatible provider
		provider, err := NewOpenAICompatibleProvider(name, ProviderConfig{
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   model,
		})
		if err != nil {
			return nil, fmt.Errorf("create %s provider: %w", name, err)
		}

		return provider, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", name)
	}
}

// ListAvailableProviders returns a list of all available providers
func (f *Factory) ListAvailableProviders() []string {
	var providers []string

	// Check which providers have API keys configured
	if f.config.GetAPIKey("openai") != "" || os.Getenv("OPENAI_API_KEY") != "" {
		providers = append(providers, "openai")
	}
	if f.config.GetAPIKey("deepseek") != "" || os.Getenv("DEEPSEEK_API_KEY") != "" {
		providers = append(providers, "deepseek")
	}
	if f.config.GetAPIKey("glm") != "" || os.Getenv("GLM_API_KEY") != "" {
		providers = append(providers, "glm")
	}
	if f.config.GetAPIKey("minimax") != "" || os.Getenv("MINIMAX_API_KEY") != "" {
		providers = append(providers, "minimax")
	}
	if f.config.GetAPIKey("qwen") != "" || os.Getenv("QWEN_API_KEY") != "" {
		providers = append(providers, "qwen")
	}
	if f.config.GetAPIKey("kimi") != "" || os.Getenv("KIMI_API_KEY") != "" {
		providers = append(providers, "kimi")
	}

	// Always include the default provider
	providers = append(providers, f.config.Default.Provider)

	// Remove duplicates
	uniqueProviders := make(map[string]bool)
	var unique []string
	for _, p := range providers {
		if !uniqueProviders[p] {
			uniqueProviders[p] = true
			unique = append(unique, p)
		}
	}

	return unique
}

// SupportedProviders returns a list of all supported providers
func (f *Factory) SupportedProviders() []string {
	supportedProviders := []string{
		"openai",
		"deepseek",
		"glm",
		"minimax",
		"qwen",
		"kimi",
	}

	return supportedProviders
}
