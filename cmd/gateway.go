package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newGatewayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Gateway management",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start gateway server",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Gateway starting... (coming soon)")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop gateway server",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Gateway stopping... (coming soon)")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "restart",
		Short: "Restart gateway server",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Gateway restarting... (coming soon)")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Check gateway status",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Gateway status: not implemented yet")
		},
	})

	return cmd
}