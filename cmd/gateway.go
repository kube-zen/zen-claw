package cmd

import (
	"fmt"
	"os"

	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/gateway"
	"github.com/spf13/cobra"
)

func newGatewayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Gateway management",
	}

	// Add config flag to all subcommands
	cmd.PersistentFlags().String("config", "", "Config file path (default: ~/.zen/zen-claw/config.yaml)")

	cmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start gateway server",
		RunE:  runGatewayStart,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop gateway server",
		RunE:  runGatewayStop,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "restart",
		Short: "Restart gateway server",
		RunE:  runGatewayRestart,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Check gateway status",
		RunE:  runGatewayStatus,
	})

	return cmd
}

func runGatewayStart(cmd *cobra.Command, args []string) error {
	// Get config path from flag
	configPath, _ := cmd.Flags().GetString("config")
	
	// Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create gateway
	gw := gateway.NewGateway(cfg)

	// Start gateway
	fmt.Println("Starting Zen Claw gateway...")
	if err := gw.Start(); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}
	
	return nil
}

func runGatewayStop(cmd *cobra.Command, args []string) error {
	// Get config path from flag
	configPath, _ := cmd.Flags().GetString("config")
	
	// Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create gateway
	gw := gateway.NewGateway(cfg)

	// Stop gateway
	fmt.Println("Stopping Zen Claw gateway...")
	if err := gw.Stop(); err != nil {
		return fmt.Errorf("failed to stop gateway: %w", err)
	}
	fmt.Println("Gateway stopped")
	return nil
}

func runGatewayRestart(cmd *cobra.Command, args []string) error {
	// Get config path from flag
	configPath, _ := cmd.Flags().GetString("config")
	
	// Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create gateway
	gw := gateway.NewGateway(cfg)

	// Restart gateway
	fmt.Println("Restarting Zen Claw gateway...")
	if err := gw.Restart(); err != nil {
		return fmt.Errorf("failed to restart gateway: %w", err)
	}
	return nil
}

func runGatewayStatus(cmd *cobra.Command, args []string) error {
	// Get config path from flag
	configPath, _ := cmd.Flags().GetString("config")
	
	// Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create gateway
	gw := gateway.NewGateway(cfg)

	// Get status
	status := gw.Status()
	fmt.Printf("Gateway status: %s\n", status)

	// Check if PID file exists
	pidFile := "/tmp/zen-claw-gateway.pid"
	if _, err := os.Stat(pidFile); err == nil {
		fmt.Println("PID file exists")
	} else {
		fmt.Println("PID file not found")
	}
	return nil
}