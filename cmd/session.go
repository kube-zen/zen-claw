package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage sessions",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List active sessions",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Session listing coming soon")
		},
	})

	return cmd
}

// Helper function to truncate strings for display
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}