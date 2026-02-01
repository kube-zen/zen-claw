package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/ai"
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

	cmd.Flags().String("model", "deepseek/deepseek-chat", "Model to use")
	cmd.Flags().String("workspace", "", "Workspace directory (default: current)")
	cmd.Flags().Bool("thinking", false, "Enable thinking mode")
	cmd.Flags().String("task", "", "Task to execute (if not interactive)")

	return cmd
}

func runAgent(cmd *cobra.Command, args []string) error {
	model, _ := cmd.Flags().GetString("model")
	workspace, _ := cmd.Flags().GetString("workspace")
	thinking, _ := cmd.Flags().GetBool("thinking")
	task, _ := cmd.Flags().GetString("task")

	if workspace == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get current directory: %w", err)
		}
		workspace = cwd
	}

	// Ensure workspace exists
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}

	// Create AI provider based on model
	var aiProvider ai.Provider
	if strings.Contains(model, "openai") {
		// Try to get OpenAI API key from environment
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			fmt.Println("⚠️  OPENAI_API_KEY not set. Using mock provider with tool calls.")
			aiProvider = providers.NewMockProvider(true)
		} else {
			openaiProvider, err := providers.NewOpenAIProvider(providers.Config{
				APIKey: apiKey,
				Model:  model,
			})
			if err != nil {
				return fmt.Errorf("create OpenAI provider: %w", err)
			}
			aiProvider = openaiProvider
		}
	} else if strings.Contains(model, "mock") {
		// Use mock provider with tool calls
		fmt.Println("🔧 Using mock provider with tool calls")
		aiProvider = providers.NewMockProvider(true)
	} else {
		// Use mock provider without tool calls for other models
		fmt.Printf("🔧 Using mock provider for model: %s\n", model)
		aiProvider = providers.NewMockProvider(false)
	}

	// Create session
	sess := session.New(session.Config{
		Workspace: workspace,
		Model:     model,
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
	config := agent.Config{
		Model:     model,
		Workspace: workspace,
		Thinking:  thinking,
	}

	// Initialize real agent
	ag := agent.NewRealAgent(config, aiProvider, toolMgr, sess)

	// Run agent
	if task != "" {
		return ag.RunTask(task)
	}

	return ag.RunInteractive()
}