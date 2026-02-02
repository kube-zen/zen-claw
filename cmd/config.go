package cmd

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/neves/zen-claw/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Initialize configuration file",
		RunE:  runConfigInit,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE:  runConfigShow,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Show config file path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.DefaultConfigPath())
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Check configuration and API keys",
		RunE:  runConfigCheck,
	})

	return cmd
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("path")
	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Config already exists at: %s\n", configPath)
		fmt.Println("Use 'zen-claw config show' to view it.")
		return nil
	}

	// Create default config
	cfg := config.NewDefaultConfig()

	// Save config
	if err := config.SaveConfig(cfg, configPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("✅ Configuration initialized at: %s\n", configPath)
	fmt.Println("\nNext steps:")
	fmt.Println("1. Edit the config file with your API keys")
	fmt.Println("2. Run 'zen-claw config check' to verify")
	fmt.Println("3. Run 'zen-claw agent' to start using Zen Claw")

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("path")
	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}

	// Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Convert to YAML for display
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	fmt.Printf("Configuration from: %s\n\n", configPath)
	fmt.Println(string(data))
	return nil
}

func runConfigCheck(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("path")
	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}

	// Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Println("🔍 Checking Zen Claw configuration...")
	fmt.Printf("Config file: %s\n\n", configPath)

	// Check default settings
	fmt.Println("📋 Default Settings:")
	fmt.Printf("  Provider: %s\n", cfg.Default.Provider)
	fmt.Printf("  Model: %s\n", cfg.Default.Model)
	fmt.Printf("  Thinking: %v\n", cfg.Default.Thinking)
	fmt.Printf("  Workspace: %s\n", cfg.Workspace.Path)

	// Check providers
	fmt.Println("\n🔑 API Key Status:")
	providers := []string{"openai", "deepseek", "glm", "minimax"}
	for _, provider := range providers {
		apiKey := cfg.GetAPIKey(provider)
		status := "❌ Missing"
		if apiKey != "" && !strings.HasPrefix(apiKey, "${") {
			status = "✅ Configured"
		} else if os.Getenv(fmt.Sprintf("%s_API_KEY", strings.ToUpper(provider))) != "" {
			status = "✅ From environment"
			apiKey = "(hidden)"
		} else if strings.HasPrefix(apiKey, "${") {
			status = "⚠️  Placeholder"
		}
		fmt.Printf("  %-10s: %s\n", provider, status)
	}

	// Check workspace
	fmt.Println("\n📁 Workspace:")
	if cfg.Workspace.Path == "" {
		fmt.Println("  ⚠️  Not configured, will use current directory")
	} else {
		if _, err := os.Stat(cfg.Workspace.Path); os.IsNotExist(err) {
			fmt.Printf("  ⚠️  Directory does not exist: %s\n", cfg.Workspace.Path)
			fmt.Println("    It will be created when you first run the agent.")
		} else {
			fmt.Printf("  ✅ Directory exists: %s\n", cfg.Workspace.Path)
		}
	}

	// Summary
	fmt.Println("\n🎯 Summary:")
	if cfg.GetAPIKey(cfg.Default.Provider) != "" || os.Getenv(fmt.Sprintf("%s_API_KEY", strings.ToUpper(cfg.Default.Provider))) != "" {
		fmt.Printf("✅ Ready to use with %s\n", cfg.Default.Provider)
		fmt.Println("   Run: zen-claw agent")
	} else {
		fmt.Printf("⚠️  Default provider '%s' needs API key\n", cfg.Default.Provider)
		fmt.Println("   Options:")
		fmt.Println("   1. Set API key in config file")
		fmt.Println("   2. Set environment variable")
		fmt.Println("   3. Use mock provider: zen-claw agent --model mock")
	}

	return nil
}
