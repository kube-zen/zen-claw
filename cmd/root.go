package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "zen-claw",
	Short: "Zen Claw - AI assistant CLI in Go",
	Long: `Zen Claw is a Go clone of OpenClaw focusing on AI interaction
and minimal tooling. No branches, no CI overhead, just results.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(newAgentCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newSessionCmd())
	rootCmd.AddCommand(newToolsCmd())
}