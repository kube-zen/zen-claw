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
	case "openai", "deepseek", "glm", "minimax", "qwen":
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
		}

		// Build config for the provider
		config := ProviderConfig{
			APIKey: apiKey,
			Model:  model,
		}

		if providerConfig != nil {
			if providerConfig.BaseURL != "" {
				config.BaseURL = providerConfig.BaseURL
			}
			if providerConfig.Model != "" && model == f.config.GetModel(name) {
				// Use config model if not overridden by command line
				config.Model = providerConfig.Model
			}
		}

		// Validate that we have a valid API key
		if apiKey == "" {
			return nil, fmt.Errorf("invalid API key for provider %s", name)
		}

		return NewOpenAICompatibleProvider(name, config)

	case "mock":
		// Mock provider for testing (always works)
		return NewMockProvider(true), nil

	case "simple":
		// Simple provider (always works, no tools)
		return NewSimpleProvider(), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s. Available: openai, deepseek, glm, minimax, qwen, mock, simple", name)
	}
}

// CreateDefaultProvider creates the default provider from config
func (f *Factory) CreateDefaultProvider() (ai.Provider, error) {
	return f.CreateProvider(f.config.Default.Provider)
}

// ListAvailableProviders returns a list of available providers
func (f *Factory) ListAvailableProviders() []string {
	providers := []string{"mock", "simple"}

	// Check which configured providers have API keys
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

	return providers
}

// ValidateProviderConfig validates that the provider configuration is valid
func (f *Factory) ValidateProviderConfig(name string) error {
	// Check if provider exists in our supported list
	supportedProviders := []string{"openai", "deepseek", "glm", "minimax", "qwen"}

	valid := false
	for _, provider := range supportedProviders {
		if provider == name {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("unsupported provider: %s", name)
	}

	// Check if API key is present
	apiKey := f.config.GetAPIKey(name)
	if apiKey == "" {
		// Check environment variable
		envVar := fmt.Sprintf("%s_API_KEY", strings.ToUpper(name))
		apiKey = os.Getenv(envVar)
	}

	if apiKey == "" {
		return fmt.Errorf("missing API key for provider %s", name)
	}

	return nil
}
