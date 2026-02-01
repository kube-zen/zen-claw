package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
	"github.com/neves/zen-claw/internal/session"
	"github.com/neves/zen-claw/internal/tools"
	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run an AI agent session",
		RunE:  runAgent,
	}

	cmd.Flags().String("model", "", "Model to use (default: from config)")
	cmd.Flags().String("workspace", "", "Workspace directory (default: from config)")
	cmd.Flags().Bool("thinking", false, "Enable thinking mode")
	cmd.Flags().String("task", "", "Task to execute (if not interactive)")
	cmd.Flags().String("config", "", "Config file path (default: ~/.zen/zen-claw/config.yaml)")

	return cmd
}

func runAgent(cmd *cobra.Command, args []string) error {
	model, _ := cmd.Flags().GetString("model")
	workspace, _ := cmd.Flags().GetString("workspace")
	thinking, _ := cmd.Flags().GetBool("thinking")
	task, _ := cmd.Flags().GetString("task")
	configPath, _ := cmd.Flags().GetString("config")

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Determine workspace
	if workspace == "" {
		workspace = cfg.Workspace.Path
		if workspace == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			workspace = cwd
		}
	}

	// Expand ~ in workspace path
	if workspace[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home directory: %w", err)
		}
		workspace = filepath.Join(home, workspace[2:])
	}

	// Ensure workspace exists
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}

	// Determine provider and model
	// If model is specified, it could be just a model name or provider:model
	// For simplicity, we treat it as provider name unless it contains a slash
	providerName := cfg.Default.Provider
	modelToUse := cfg.GetModel(providerName)
	
	if model != "" && model != "default" {
		// Check if it's a provider name (simple, mock, or known provider)
		if model == "simple" || model == "mock" || 
		   model == "openai" || model == "deepseek" || 
		   model == "glm" || model == "minimax" {
			// It's a provider name
			providerName = model
			modelToUse = cfg.GetModel(providerName)
		} else if strings.Contains(model, "/") {
			// It's a model name like "deepseek-chat" or "gpt-4o-mini"
			// Try to guess provider from model name
			modelToUse = model
			// Keep default provider
		} else {
			// Unknown format, use as model name
			modelToUse = model
		}
	}

	// Create provider factory
	factory := providers.NewFactory(cfg)
	
	// Create AI provider
	aiProvider, err := factory.CreateProvider(providerName)
	if err != nil {
		// If provider fails, fall back to mock
		fmt.Printf("⚠️  Failed to create provider %s: %v\n", providerName, err)
		fmt.Println("🔧 Falling back to mock provider")
		aiProvider = providers.NewMockProvider(true)
	}

	// Create session
	sess := session.New(session.Config{
		Workspace: workspace,
		Model:     modelToUse,
	})

	// Create tool manager
	toolMgr, err := tools.NewManager(tools.Config{
		Workspace: workspace,
		Session:   sess,
	})
	if err != nil {
		return fmt.Errorf("create tool manager: %w", err)
	}

	// Create agent config
	agentConfig := agent.Config{
		Model:     modelToUse,
		Workspace: workspace,
		Thinking:  thinking || cfg.Default.Thinking,
	}

	// Initialize real agent
	ag := agent.NewRealAgent(agentConfig, aiProvider, toolMgr, sess)

	// Run agent
	if task != "" {
		return ag.RunTask(task)
	}

	return ag.RunInteractive()
}