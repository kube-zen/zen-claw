package cmd

import (
	"fmt"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/spf13/cobra"
)

func newToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List available tools",
		Run: func(cmd *cobra.Command, args []string) {
			// Create the actual tools that agents use
			tools := []agent.Tool{
				// File operations
				agent.NewExecTool("."),
				agent.NewReadFileTool("."),
				agent.NewWriteFileTool("."),
				agent.NewEditFileTool("."),
				agent.NewAppendFileTool("."),
				agent.NewListDirTool("."),
				agent.NewSearchFilesTool("."),
				agent.NewSystemInfoTool(),
				// Git operations
				agent.NewGitStatusTool("."),
				agent.NewGitDiffTool("."),
				agent.NewGitAddTool("."),
				agent.NewGitCommitTool("."),
				agent.NewGitPushTool("."),
				agent.NewGitLogTool("."),
				// Preview (diff before write)
				agent.NewPreviewWriteTool("."),
				agent.NewPreviewEditTool("."),
			}

			fmt.Println("Available Tools:")
			fmt.Println("════════════════════════════════════════════════════════════════════════════════")
			for _, tool := range tools {
				fmt.Printf("  • %-15s - %s\n", tool.Name(), tool.Description())
			}
			fmt.Println()
			fmt.Println("Usage: These tools are automatically available to the AI agent.")
			fmt.Println("       Use 'zen-claw agent' to start an interactive session.")
		},
	}

	return cmd
}
