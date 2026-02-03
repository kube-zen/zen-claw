package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Providers        ProvidersConfig        `yaml:"providers"`
	Default          DefaultConfig          `yaml:"default"`
	Workspace        WorkspaceConfig        `yaml:"workspace"`
	Sessions         SessionsConfig         `yaml:"sessions"`
	Consensus        ConsensusConfig        `yaml:"consensus"`
	Factory          FactoryConfig          `yaml:"factory"`
	Preferences      PreferencesConfig      `yaml:"preferences"`
	Web              WebConfig              `yaml:"web"`
	MCP              MCPConfig              `yaml:"mcp"`
	Routing          RoutingConfig          `yaml:"routing"`
	CostOptimization CostOptimizationConfig `yaml:"cost_optimization"`
}

// MCPConfig configures Model Context Protocol servers
type MCPConfig struct {
	Servers []MCPServerConfig `yaml:"servers"`
}

// MCPServerConfig defines an MCP server connection
type MCPServerConfig struct {
	Name    string   `yaml:"name"`    // Display name
	Command string   `yaml:"command"` // Command to run (e.g., "npx", "python")
	Args    []string `yaml:"args"`    // Arguments
	Env     []string `yaml:"env"`     // Environment variables (optional)
	Enabled bool     `yaml:"enabled"` // Whether to auto-connect (default true if not specified)
}

// WebConfig configures web tools (search, fetch)
type WebConfig struct {
	Search WebSearchConfig `yaml:"search"`
}

// WebSearchConfig configures web search
type WebSearchConfig struct {
	Enabled  bool   `yaml:"enabled"`
	APIKey   string `yaml:"api_key"`  // Brave Search API key
	Provider string `yaml:"provider"` // "brave" (default)
}

type SessionsConfig struct {
	MaxSessions int    `yaml:"max_sessions"` // Maximum concurrent sessions (default 5)
	DBPath      string `yaml:"db_path"`      // Path to session database (default ~/.zen/zen-claw/data/sessions.db)
}

// ConsensusConfig configures the consensus engine
type ConsensusConfig struct {
	Workers []WorkerConfig `yaml:"workers"` // Worker definitions for parallel calls
	Arbiter []string       `yaml:"arbiter"` // Arbiter preference order (first available used)
}

// WorkerConfig defines a consensus worker
// Note: Role is NOT defined per-worker, it's defined per-request
// All workers receive the SAME role for a given consensus task
type WorkerConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
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

// RoutingConfig configures context-aware smart routing
type RoutingConfig struct {
	SmartRouting   bool               `yaml:"smart_routing"`   // Enable context-aware routing (default true)
	ContextTiers   ContextTiersConfig `yaml:"context_tiers"`   // Context size tiers
	PremiumBudget  float64            `yaml:"premium_budget"`  // Daily budget for premium models (USD)
	RequireConfirm bool               `yaml:"require_confirm"` // Require confirmation for premium tier
}

// CostOptimizationConfig configures token-saving features
type CostOptimizationConfig struct {
	// History management
	MaxHistoryTurns    int `yaml:"max_history_turns"`    // Hard cap on messages (default 50)
	MaxHistoryMessages int `yaml:"max_history_messages"` // Summarize beyond this (default 20)
	KeepLastAssistants int `yaml:"keep_last_assistants"` // Keep last N assistant msgs intact (default 3)

	// Tool output pruning
	MaxToolResultTokens int                       `yaml:"max_tool_result_tokens"` // Default max tokens per tool result
	ToolRules           map[string]ToolRuleConfig `yaml:"tool_rules"`             // Per-tool pruning rules

	// Anthropic prompt caching (when using Anthropic provider)
	AnthropicCacheRetention string `yaml:"anthropic_cache_retention"` // "none", "short" (5m), "long" (1h)
}

// ToolRuleConfig defines pruning rules for a specific tool
type ToolRuleConfig struct {
	MaxTokens  int  `yaml:"max_tokens"`  // Max tokens before truncation
	KeepRecent int  `yaml:"keep_recent"` // Keep last N results untruncated
	Aggressive bool `yaml:"aggressive"`  // Use aggressive truncation (head only)
}

// ContextTiersConfig defines context size thresholds
type ContextTiersConfig struct {
	SmallMax  int `yaml:"small_max"`  // Max tokens for "small" tier (default 32000)
	MediumMax int `yaml:"medium_max"` // Max tokens for "medium" tier (default 200000)
	// Anything above MediumMax goes to "large" tier (premium models)
}

// ProviderContextInfo holds known context window sizes (tokens)
var ProviderContextInfo = map[string]int{
	"deepseek": 128000,  // 128K - cost-effective workhorse
	"qwen":     262000,  // 262K - large context, good balance
	"kimi":     200000,  // 200K - K2.5 context
	"glm":      128000,  // 128K
	"minimax":  4000000, // 4M - massive context (premium)
	"openai":   128000,  // GPT-4o: 128K (GPT-4.1 would be 1M+)
	"gemini":   1000000, // Gemini 2.5 Pro: 1M (premium)
	"claude":   200000,  // Claude Sonnet: 200K
}

type ProvidersConfig struct {
	Kimi      *ProviderConfig `yaml:"kimi,omitempty"`
	OpenAI    *ProviderConfig `yaml:"openai,omitempty"`
	DeepSeek  *ProviderConfig `yaml:"deepseek,omitempty"`
	GLM       *ProviderConfig `yaml:"glm,omitempty"`
	Minimax   *ProviderConfig `yaml:"minimax,omitempty"`
	Qwen      *ProviderConfig `yaml:"qwen,omitempty"`
	Anthropic *ProviderConfig `yaml:"anthropic,omitempty"`
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
				{Provider: "deepseek", Model: "deepseek-chat"},
				{Provider: "qwen", Model: "qwen3-coder-30b"},
				{Provider: "minimax", Model: "minimax-M2.1"},
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
		Routing: RoutingConfig{
			SmartRouting: true,
			ContextTiers: ContextTiersConfig{
				SmallMax:  32000,  // <32K: use cheapest (deepseek)
				MediumMax: 200000, // 32K-200K: use balanced (qwen, kimi)
				// >200K: use premium (minimax 4M, gemini 1M)
			},
			PremiumBudget:  5.0,  // $5/day for premium models
			RequireConfirm: true, // Require confirmation for large context
		},
		CostOptimization: CostOptimizationConfig{
			MaxHistoryTurns:         50,      // Hard cap at 50 messages
			MaxHistoryMessages:      20,      // Summarize beyond 20
			KeepLastAssistants:      3,       // Keep last 3 assistant msgs intact
			MaxToolResultTokens:     8000,    // Default ~32KB
			AnthropicCacheRetention: "short", // 5-minute cache for Anthropic
			ToolRules: map[string]ToolRuleConfig{
				"exec":    {MaxTokens: 4000, KeepRecent: 1, Aggressive: true},
				"process": {MaxTokens: 4000, KeepRecent: 1, Aggressive: true},
			},
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

// GetSessionDBPath returns the session database path
func (c *Config) GetSessionDBPath() string {
	return c.Sessions.DBPath // Empty means use default
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
	case "anthropic":
		if c.Providers.Anthropic != nil {
			return c.Providers.Anthropic.APIKey
		}
	}

	return ""
}

// GetBraveAPIKey returns the Brave Search API key from config or environment
func (c *Config) GetBraveAPIKey() string {
	// Check environment first
	if key := os.Getenv("BRAVE_API_KEY"); key != "" {
		return key
	}
	// Then config
	return c.Web.Search.APIKey
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
	case "anthropic":
		if c.Providers.Anthropic != nil && c.Providers.Anthropic.Model != "" {
			return c.Providers.Anthropic.Model
		}
		return "claude-sonnet-4-20250514" // Claude Sonnet 4: 200K context, prompt caching
	default:
		return c.Default.Model
	}
}

// GetMCPServers returns enabled MCP server configs
func (c *Config) GetMCPServers() []MCPServerConfig {
	var enabled []MCPServerConfig
	for _, srv := range c.MCP.Servers {
		// Default to enabled if not explicitly disabled
		if srv.Command != "" && srv.Name != "" {
			enabled = append(enabled, srv)
		}
	}
	return enabled
}

// ContextTier represents a context size tier for smart routing
type ContextTier string

const (
	ContextTierSmall  ContextTier = "small"  // <32K - cheapest providers
	ContextTierMedium ContextTier = "medium" // 32K-200K - balanced providers
	ContextTierLarge  ContextTier = "large"  // >200K - premium providers
)

// GetContextTier returns the appropriate context tier for a token count
func (c *Config) GetContextTier(tokenCount int) ContextTier {
	smallMax := c.Routing.ContextTiers.SmallMax
	mediumMax := c.Routing.ContextTiers.MediumMax

	// Use defaults if not configured
	if smallMax == 0 {
		smallMax = 32000
	}
	if mediumMax == 0 {
		mediumMax = 200000
	}

	if tokenCount <= smallMax {
		return ContextTierSmall
	}
	if tokenCount <= mediumMax {
		return ContextTierMedium
	}
	return ContextTierLarge
}

// GetProvidersForTier returns providers suitable for a context tier
func (c *Config) GetProvidersForTier(tier ContextTier) []string {
	// Known provider capabilities by tier
	smallProviders := []string{"deepseek", "glm"}                             // 128K max, cheapest
	mediumProviders := []string{"qwen", "kimi", "deepseek", "glm"}            // 200K-262K, balanced
	largeProviders := []string{"minimax", "qwen", "kimi", "gemini", "claude"} // 1M+, premium

	switch tier {
	case ContextTierSmall:
		return smallProviders
	case ContextTierMedium:
		return mediumProviders
	case ContextTierLarge:
		return largeProviders
	default:
		return smallProviders
	}
}

// IsSmartRoutingEnabled returns whether smart context-aware routing is enabled
func (c *Config) IsSmartRoutingEnabled() bool {
	// Default to true if not explicitly disabled
	return c.Routing.SmartRouting || c.Routing.ContextTiers.SmallMax == 0
}

// GetProviderContextLimit returns the known context limit for a provider
func GetProviderContextLimit(provider string) int {
	if limit, ok := ProviderContextInfo[provider]; ok {
		return limit
	}
	return 128000 // Default assumption
}

// CanProviderHandleContext returns whether a provider can handle the given context size
func CanProviderHandleContext(provider string, tokenCount int) bool {
	limit := GetProviderContextLimit(provider)
	return tokenCount <= limit
}
