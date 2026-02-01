package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Providers ProvidersConfig `yaml:"providers"`
	Default   DefaultConfig   `yaml:"default"`
	Workspace WorkspaceConfig `yaml:"workspace"`
}

type ProvidersConfig struct {
	OpenAI   *ProviderConfig `yaml:"openai,omitempty"`
	DeepSeek *ProviderConfig `yaml:"deepseek,omitempty"`
	GLM      *ProviderConfig `yaml:"glm,omitempty"`
	Minimax  *ProviderConfig `yaml:"minimax,omitempty"`
	Qwen     *ProviderConfig `yaml:"qwen,omitempty"`
}

type ProviderConfig struct {
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url,omitempty"`
}

type DefaultConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	Thinking bool   `yaml:"thinking"`
}

type WorkspaceConfig struct {
	Path string `yaml:"path"`
}

// DefaultConfigPath returns the default config path
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./zen-claw.yaml"
	}
	return filepath.Join(home, ".zen", "zen-claw", "config.yaml")
}

// LoadConfig loads configuration from file
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	// Check if config exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return default config
		return NewDefaultConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config YAML: %w", err)
	}

	return &config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config, path string) error {
	if path == "" {
		path = DefaultConfigPath()
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal config to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// NewDefaultConfig returns a default configuration
func NewDefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	workspace := filepath.Join(home, ".zen", "zen-claw", "workspace")

	return &Config{
		Default: DefaultConfig{
			Provider: "deepseek",
			Model:    "deepseek-chat",
			Thinking: false,
		},
		Workspace: WorkspaceConfig{
			Path: workspace,
		},
	}
}

// GetAPIKey returns the API key for a provider from config or environment
func (c *Config) GetAPIKey(provider string) string {
	// First check environment variables
	envKey := os.Getenv(fmt.Sprintf("%s_API_KEY", strings.ToUpper(provider)))
	if envKey != "" {
		return envKey
	}

	// Then check config
	switch provider {
	case "openai":
		if c.Providers.OpenAI != nil {
			return c.Providers.OpenAI.APIKey
		}
	case "deepseek":
		if c.Providers.DeepSeek != nil {
			return c.Providers.DeepSeek.APIKey
		}
	case "glm":
		if c.Providers.GLM != nil {
			return c.Providers.GLM.APIKey
		}
	case "minimax":
		if c.Providers.Minimax != nil {
			return c.Providers.Minimax.APIKey
		}
	case "qwen":
		if c.Providers.Qwen != nil {
			return c.Providers.Qwen.APIKey
		}
	}

	return ""
}

// GetModel returns the model for a provider
func (c *Config) GetModel(provider string) string {
	switch provider {
	case "openai":
		if c.Providers.OpenAI != nil && c.Providers.OpenAI.Model != "" {
			return c.Providers.OpenAI.Model
		}
		return "gpt-4o-mini"
	case "deepseek":
		if c.Providers.DeepSeek != nil && c.Providers.DeepSeek.Model != "" {
			return c.Providers.DeepSeek.Model
		}
		return "deepseek-chat"
	case "glm":
		if c.Providers.GLM != nil && c.Providers.GLM.Model != "" {
			return c.Providers.GLM.Model
		}
		return "glm-4.7"
	case "minimax":
		if c.Providers.Minimax != nil && c.Providers.Minimax.Model != "" {
			return c.Providers.Minimax.Model
		}
		return "minimax-M2.1"
	case "qwen":
		if c.Providers.Qwen != nil && c.Providers.Qwen.Model != "" {
			return c.Providers.Qwen.Model
		}
		return "qwen3-coder-30b"  // Default Qwen model: 262K context, great for coding
	default:
		return c.Default.Model
	}
}