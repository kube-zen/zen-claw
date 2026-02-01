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
	case "openai":
		if apiKey == "" {
			// Try environment variable as fallback
			apiKey = os.Getenv("OPENAI_API_KEY")
			if apiKey == "" {
				return nil, fmt.Errorf("OpenAI API key not found. Set in config or OPENAI_API_KEY env")
			}
		}
		
		var baseURL string
		if f.config.Providers.OpenAI != nil {
			baseURL = f.config.Providers.OpenAI.BaseURL
		}
		
		return NewOpenAIProvider(ProviderConfig{
			APIKey:  apiKey,
			Model:   model,
			BaseURL: baseURL,
		})

	case "deepseek":
		if apiKey == "" {
			apiKey = os.Getenv("DEEPSEEK_API_KEY")
			if apiKey == "" {
				return nil, fmt.Errorf("DeepSeek API key not found. Set in config or DEEPSEEK_API_KEY env")
			}
		}
		
		var baseURL string
		if f.config.Providers.DeepSeek != nil {
			baseURL = f.config.Providers.DeepSeek.BaseURL
		}
		
		return NewDeepSeekProvider(ProviderConfig{
			APIKey:  apiKey,
			Model:   model,
			BaseURL: baseURL,
		})

	case "glm":
		if apiKey == "" {
			apiKey = os.Getenv("GLM_API_KEY")
			if apiKey == "" {
				return nil, fmt.Errorf("GLM API key not found. Set in config or GLM_API_KEY env")
			}
		}
		
		var baseURL string
		if f.config.Providers.GLM != nil {
			baseURL = f.config.Providers.GLM.BaseURL
		}
		
		return NewGLMProvider(ProviderConfig{
			APIKey:  apiKey,
			Model:   model,
			BaseURL: baseURL,
		})

	case "minimax":
		if apiKey == "" {
			apiKey = os.Getenv("MINIMAX_API_KEY")
			if apiKey == "" {
				return nil, fmt.Errorf("Minimax API key not found. Set in config or MINIMAX_API_KEY env")
			}
		}
		
		var baseURL string
		if f.config.Providers.Minimax != nil {
			baseURL = f.config.Providers.Minimax.BaseURL
		}
		
		return NewMinimaxProvider(ProviderConfig{
			APIKey:  apiKey,
			Model:   model,
			BaseURL: baseURL,
		})

	case "mock":
		// Mock provider for testing (always works)
		return NewMockProvider(true), nil

	case "simple":
		// Simple provider (always works, no tools)
		return NewSimpleProvider(), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s. Available: openai, deepseek, glm, minimax, mock, simple", name)
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
	
	return providers
}