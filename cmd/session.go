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
			fmt.Println("Sessions feature coming soon")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "spawn",
		Short: "Spawn a sub-agent session",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Sub-agent spawning coming soon")
		},
	})

	return cmd
}