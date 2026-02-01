package cmd

import (
	"fmt"
	"os"

	"github.com/neves/zen-claw/internal/agent"
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

	// Create agent config
	config := agent.Config{
		Model:     model,
		Workspace: workspace,
		Thinking:  thinking,
	}

	// Initialize agent
	ag, err := agent.New(config)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// Run agent
	if task != "" {
		return ag.RunTask(task)
	}

	return ag.RunInteractive()
}