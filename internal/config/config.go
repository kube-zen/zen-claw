package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Providers  ProvidersConfig  `yaml:"providers"`
	Default    DefaultConfig    `yaml:"default"`
	Workspace  WorkspaceConfig  `yaml:"workspace"`
	Sessions   SessionsConfig   `yaml:"sessions"`
	Consensus  ConsensusConfig  `yaml:"consensus"`
	Factory    FactoryConfig    `yaml:"factory"`
	Preferences PreferencesConfig `yaml:"preferences"`
}

type SessionsConfig struct {
	MaxSessions int `yaml:"max_sessions"` // Maximum concurrent sessions (default 5)
}

// ConsensusConfig configures the consensus engine
type ConsensusConfig struct {
	Workers []WorkerConfig `yaml:"workers"` // Worker definitions for parallel calls
	Arbiter []string       `yaml:"arbiter"` // Arbiter preference order (first available used)
}

// WorkerConfig defines a consensus worker
type WorkerConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	Role     string `yaml:"role"` // e.g., "systems_thinker", "implementation_realist"
}

// FactoryConfig configures the factory mode specialists
type FactoryConfig struct {
	Specialists map[string]SpecialistConfig `yaml:"specialists"` // domain -> specialist
	Guardrails  GuardrailsConfig            `yaml:"guardrails"`
}

// SpecialistConfig defines a factory specialist
type SpecialistConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// GuardrailsConfig defines factory safety limits
type GuardrailsConfig struct {
	MaxPhaseDurationMins int      `yaml:"max_phase_duration_mins"`
	MaxTotalDurationMins int      `yaml:"max_total_duration_mins"`
	MaxCostPerPhase      float64  `yaml:"max_cost_per_phase"`
	MaxCostTotal         float64  `yaml:"max_cost_total"`
	MaxFilesModified     int      `yaml:"max_files_modified"`
	RequireTests         bool     `yaml:"require_tests"`
	RequireCompilation   bool     `yaml:"require_compilation"`
	ForbiddenCommands    []string `yaml:"forbidden_commands"`
}

// PreferencesConfig configures AI routing preferences
type PreferencesConfig struct {
	FallbackOrder []string `yaml:"fallback_order"` // Provider fallback order for routing
}

type ProvidersConfig struct {
	Kimi     *ProviderConfig `yaml:"kimi,omitempty"`
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
		Sessions: SessionsConfig{
			MaxSessions: 5,
		},
		Consensus: ConsensusConfig{
			Workers: []WorkerConfig{
				{Provider: "deepseek", Model: "deepseek-chat", Role: "systems_thinker"},
				{Provider: "qwen", Model: "qwen3-coder-30b", Role: "implementation_realist"},
				{Provider: "minimax", Model: "minimax-M2.1", Role: "edge_case_hunter"},
			},
			Arbiter: []string{"kimi", "qwen", "deepseek"},
		},
		Factory: FactoryConfig{
			Specialists: map[string]SpecialistConfig{
				"coordinator":    {Provider: "kimi", Model: "kimi-k2-5"},
				"go":             {Provider: "deepseek", Model: "deepseek-chat"},
				"typescript":     {Provider: "qwen", Model: "qwen3-coder-30b"},
				"infrastructure": {Provider: "minimax", Model: "minimax-M2.1"},
			},
			Guardrails: GuardrailsConfig{
				MaxPhaseDurationMins: 10,
				MaxTotalDurationMins: 240,
				MaxCostPerPhase:      0.50,
				MaxCostTotal:         5.00,
				MaxFilesModified:     50,
				RequireTests:         false,
				RequireCompilation:   true,
				ForbiddenCommands:    []string{"rm -rf /", "rm -rf ~", "drop database", "DROP DATABASE"},
			},
		},
		Preferences: PreferencesConfig{
			FallbackOrder: []string{"deepseek", "kimi", "glm", "minimax", "qwen", "openai"},
		},
	}
}

// GetConsensusWorkers returns consensus workers (from config or defaults)
func (c *Config) GetConsensusWorkers() []WorkerConfig {
	if len(c.Consensus.Workers) > 0 {
		return c.Consensus.Workers
	}
	return NewDefaultConfig().Consensus.Workers
}

// GetArbiterOrder returns arbiter preference order
func (c *Config) GetArbiterOrder() []string {
	if len(c.Consensus.Arbiter) > 0 {
		return c.Consensus.Arbiter
	}
	return NewDefaultConfig().Consensus.Arbiter
}

// GetFactorySpecialist returns specialist config for a domain
func (c *Config) GetFactorySpecialist(domain string) SpecialistConfig {
	if c.Factory.Specialists != nil {
		if spec, ok := c.Factory.Specialists[domain]; ok {
			return spec
		}
	}
	defaults := NewDefaultConfig().Factory.Specialists
	if spec, ok := defaults[domain]; ok {
		return spec
	}
	return SpecialistConfig{Provider: "deepseek", Model: "deepseek-chat"}
}

// GetFallbackOrder returns provider fallback order
func (c *Config) GetFallbackOrder() []string {
	if len(c.Preferences.FallbackOrder) > 0 {
		return c.Preferences.FallbackOrder
	}
	return NewDefaultConfig().Preferences.FallbackOrder
}

// GetMaxSessions returns the max sessions limit (defaults to 5)
func (c *Config) GetMaxSessions() int {
	if c.Sessions.MaxSessions <= 0 {
		return 5
	}
	return c.Sessions.MaxSessions
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
	case "kimi":
		if c.Providers.Kimi != nil {
			return c.Providers.Kimi.APIKey
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
		return "qwen3-coder-30b" // Default Qwen model: 262K context, great for coding
	case "kimi":
		if c.Providers.Kimi != nil && c.Providers.Kimi.Model != "" {
			return c.Providers.Kimi.Model
		}
		return "kimi-k2-5" // Kimi K2.5: 256K context, $0.10/M input, excellent for Go/K8s
	default:
		return c.Default.Model
	}
}
