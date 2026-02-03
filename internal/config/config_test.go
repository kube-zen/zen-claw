package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()

	// Should contain .zen/zen-claw
	if !contains(path, ".zen") || !contains(path, "zen-claw") {
		t.Errorf("DefaultConfigPath() = %q, expected to contain .zen/zen-claw", path)
	}
}

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	// Check required defaults
	if cfg.Default.Provider != "deepseek" {
		t.Errorf("Default.Provider = %q, want %q", cfg.Default.Provider, "deepseek")
	}
	if cfg.Default.Model != "deepseek-chat" {
		t.Errorf("Default.Model = %q, want %q", cfg.Default.Model, "deepseek-chat")
	}
	if cfg.Sessions.MaxSessions != 5 {
		t.Errorf("Sessions.MaxSessions = %d, want 5", cfg.Sessions.MaxSessions)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("returns default when file not found", func(t *testing.T) {
		cfg, err := LoadConfig("/nonexistent/path/config.yaml")
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}

		if cfg.Default.Provider != "deepseek" {
			t.Errorf("Expected default config, got provider = %q", cfg.Default.Provider)
		}
	})

	t.Run("loads valid config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		configContent := `
providers:
  deepseek:
    api_key: "test-key"
    model: "deepseek-chat"
default:
  provider: "deepseek"
  model: "deepseek-chat"
sessions:
  max_sessions: 10
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		cfg, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}

		if cfg.Sessions.MaxSessions != 10 {
			t.Errorf("Sessions.MaxSessions = %d, want 10", cfg.Sessions.MaxSessions)
		}
		if cfg.Providers.DeepSeek == nil || cfg.Providers.DeepSeek.APIKey != "test-key" {
			t.Error("Expected DeepSeek provider with api_key")
		}
	})

	t.Run("returns error on invalid YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		invalidContent := `
providers:
  deepseek:
    - this is invalid yaml
    model: [should be string]
`
		if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		_, err := LoadConfig(configPath)
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}
	})
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.yaml")

	cfg := NewDefaultConfig()
	cfg.Sessions.MaxSessions = 20

	err := SaveConfig(cfg, configPath)
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file not created")
	}

	// Load and verify
	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.Sessions.MaxSessions != 20 {
		t.Errorf("Loaded MaxSessions = %d, want 20", loaded.Sessions.MaxSessions)
	}
}

func TestGetAPIKey(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Providers.DeepSeek = &ProviderConfig{
		APIKey: "test-key",
		Model:  "deepseek-chat",
	}
	cfg.Providers.OpenAI = &ProviderConfig{
		APIKey: "openai-key",
		Model:  "gpt-4o",
	}

	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{"deepseek has key", "deepseek", "test-key"},
		{"openai has key", "openai", "openai-key"},
		{"qwen not configured", "qwen", ""},
		{"unknown provider", "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetAPIKey(tt.provider)
			if got != tt.want {
				t.Errorf("GetAPIKey(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestGetModel(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Providers.DeepSeek = &ProviderConfig{
		APIKey: "key",
		Model:  "deepseek-coder",
	}

	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{"configured provider returns model", "deepseek", "deepseek-coder"},
		{"unconfigured provider returns default", "openai", "gpt-4o-mini"},
		{"unknown provider returns deepseek default", "unknown", "deepseek-chat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetModel(tt.provider)
			if got != tt.want {
				t.Errorf("GetModel(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestGetMaxSessions(t *testing.T) {
	t.Run("returns configured value", func(t *testing.T) {
		cfg := NewDefaultConfig()
		cfg.Sessions.MaxSessions = 15

		if got := cfg.GetMaxSessions(); got != 15 {
			t.Errorf("GetMaxSessions() = %d, want 15", got)
		}
	})

	t.Run("returns default when zero", func(t *testing.T) {
		cfg := &Config{}

		if got := cfg.GetMaxSessions(); got != 5 {
			t.Errorf("GetMaxSessions() = %d, want 5 (default)", got)
		}
	})
}

func TestContextTier(t *testing.T) {
	cfg := NewDefaultConfig()

	tests := []struct {
		tokens int
		want   ContextTier
	}{
		{1000, ContextTierSmall},
		{32000, ContextTierSmall},
		{32001, ContextTierMedium},
		{200000, ContextTierMedium},
		{200001, ContextTierLarge},
		{1000000, ContextTierLarge},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := cfg.GetContextTier(tt.tokens)
			if got != tt.want {
				t.Errorf("GetContextTier(%d) = %v, want %v", tt.tokens, got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid default config", func(t *testing.T) {
		cfg := NewDefaultConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("empty provider", func(t *testing.T) {
		cfg := NewDefaultConfig()
		cfg.Default.Provider = ""
		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() expected error for empty provider")
		}
	})

	t.Run("unknown provider", func(t *testing.T) {
		cfg := NewDefaultConfig()
		cfg.Default.Provider = "unknown"
		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() expected error for unknown provider")
		}
	})

	t.Run("negative max sessions", func(t *testing.T) {
		cfg := NewDefaultConfig()
		cfg.Sessions.MaxSessions = -1
		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() expected error for negative max_sessions")
		}
	})

	t.Run("empty consensus worker provider", func(t *testing.T) {
		cfg := NewDefaultConfig()
		cfg.Consensus.Workers = []WorkerConfig{{Provider: "", Model: "test"}}
		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() expected error for empty worker provider")
		}
	})

	t.Run("negative premium budget", func(t *testing.T) {
		cfg := NewDefaultConfig()
		cfg.Routing.PremiumBudget = -1
		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() expected error for negative premium_budget")
		}
	})

	t.Run("MCP server without name", func(t *testing.T) {
		cfg := NewDefaultConfig()
		cfg.MCP.Servers = []MCPServerConfig{{Name: "", Command: "test"}}
		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() expected error for MCP server without name")
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		cfg := NewDefaultConfig()
		cfg.Default.Provider = ""
		cfg.Sessions.MaxSessions = -1
		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() expected error")
		}
		// Should contain multiple errors
		if verrs, ok := err.(ValidationErrors); ok {
			if len(verrs) < 2 {
				t.Errorf("Expected at least 2 errors, got %d", len(verrs))
			}
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}
