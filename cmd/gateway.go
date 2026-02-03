package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/gateway"
	"github.com/spf13/cobra"
)

const (
	pidFile     = "/tmp/zen-claw-gateway.pid"
	gatewayPort = "8080"
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

	// Create gateway server
	server := gateway.NewServer(cfg)

	// Start gateway
	fmt.Println("Starting Zen Claw gateway...")
	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}

	return nil
}

func runGatewayStop(cmd *cobra.Command, args []string) error {
	fmt.Println("Stopping Zen Claw gateway...")

	// Method 1: Try PID file
	if pidData, err := os.ReadFile(pidFile); err == nil {
		pidStr := strings.TrimSpace(string(pidData))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			if process, err := os.FindProcess(pid); err == nil {
				if err := process.Signal(syscall.SIGTERM); err == nil {
					fmt.Printf("Sent SIGTERM to PID %d\n", pid)
					os.Remove(pidFile)
					// Wait a moment for graceful shutdown
					time.Sleep(500 * time.Millisecond)
					return nil
				}
			}
		}
		os.Remove(pidFile) // Remove stale PID file
	}

	// Method 2: Find process by port using lsof
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%s", gatewayPort)).Output()
	if err == nil && len(out) > 0 {
		pids := strings.Fields(strings.TrimSpace(string(out)))
		for _, pidStr := range pids {
			if pid, err := strconv.Atoi(pidStr); err == nil {
				if process, err := os.FindProcess(pid); err == nil {
					if err := process.Signal(syscall.SIGTERM); err == nil {
						fmt.Printf("Sent SIGTERM to PID %d (found by port)\n", pid)
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
		return nil
	}

	// Method 3: Check if port is actually in use
	conn, err := net.DialTimeout("tcp", "localhost:"+gatewayPort, time.Second)
	if err != nil {
		fmt.Println("Gateway is not running")
		return nil
	}
	conn.Close()

	return fmt.Errorf("could not stop gateway - process found on port %s but unable to kill", gatewayPort)
}

func runGatewayRestart(cmd *cobra.Command, args []string) error {
	fmt.Println("Restarting Zen Claw gateway...")

	// Stop first (ignore errors - might not be running)
	_ = runGatewayStop(cmd, args)

	// Wait for port to be free
	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", "localhost:"+gatewayPort, 100*time.Millisecond)
		if err != nil {
			break // Port is free
		}
		conn.Close()
		time.Sleep(200 * time.Millisecond)
	}

	// Start
	return runGatewayStart(cmd, args)
}

func runGatewayStatus(cmd *cobra.Command, args []string) error {
	// Check if port is in use
	conn, err := net.DialTimeout("tcp", "localhost:"+gatewayPort, time.Second)
	if err != nil {
		fmt.Println("Gateway status: stopped")
		return nil
	}
	conn.Close()

	fmt.Println("Gateway status: running")
	fmt.Printf("Listening on: :%s\n", gatewayPort)

	// Check PID file
	if pidData, err := os.ReadFile(pidFile); err == nil {
		pidStr := strings.TrimSpace(string(pidData))
		fmt.Printf("PID: %s\n", pidStr)
	}

	// Try to get health info
	client := NewGatewayClient("http://localhost:" + gatewayPort)
	if err := client.HealthCheck(); err == nil {
		fmt.Println("Health: OK")
	}

	return nil
}
