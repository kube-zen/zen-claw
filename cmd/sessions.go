package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/gateway"
	"github.com/spf13/cobra"
)

func newSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage session data",
		Long:  `View and manage persistent session storage.`,
	}

	cmd.AddCommand(newSessionsListCmd())
	cmd.AddCommand(newSessionsCleanCmd())
	cmd.AddCommand(newSessionsInfoCmd())

	return cmd
}

func newSessionsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all saved sessions",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := loadConfigForSessions()
			store, err := gateway.NewSessionStore(&gateway.SessionStoreConfig{
				DBPath:      cfg.GetSessionDBPath(),
				MaxSessions: 100, // Just for listing
			})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			defer store.Close()

			sessions := store.ListSessionsWithState()
			if len(sessions) == 0 {
				fmt.Println("No saved sessions")
				return
			}

			fmt.Printf("Sessions (%d):\n", len(sessions))
			fmt.Println(strings.Repeat("─", 60))
			for _, s := range sessions {
				age := time.Since(s.LastUsed)
				ageStr := formatDuration(age)
				fmt.Printf("  • %-20s  %3d msgs  %s ago\n",
					s.Stats.SessionID, s.Stats.MessageCount, ageStr)
			}
		},
	}
}

func newSessionsCleanCmd() *cobra.Command {
	var all bool
	var olderThan string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean session data",
		Long: `Delete session data to free space.

Examples:
  zen-claw sessions clean --all           # Delete ALL sessions
  zen-claw sessions clean --older 7d      # Delete sessions older than 7 days
  zen-claw sessions clean --older 24h     # Delete sessions older than 24 hours`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg := loadConfigForSessions()
			store, err := gateway.NewSessionStore(&gateway.SessionStoreConfig{
				DBPath:      cfg.GetSessionDBPath(),
				MaxSessions: 100,
			})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			defer store.Close()

			if all {
				count, err := store.CleanAllSessions()
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					return
				}
				fmt.Printf("Deleted %d sessions\n", count)
				return
			}

			if olderThan != "" {
				duration, err := parseDuration(olderThan)
				if err != nil {
					fmt.Printf("Invalid duration: %v\n", err)
					fmt.Println("Examples: 24h, 7d, 30d")
					return
				}
				count, err := store.CleanOldSessions(duration)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					return
				}
				fmt.Printf("Deleted %d sessions older than %s\n", count, olderThan)
				return
			}

			fmt.Println("Specify --all or --older <duration>")
			fmt.Println("Run 'zen-claw sessions clean --help' for examples")
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Delete all sessions")
	cmd.Flags().StringVar(&olderThan, "older", "", "Delete sessions older than duration (e.g., 7d, 24h)")

	return cmd
}

func newSessionsInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show session storage info",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := loadConfigForSessions()
			dbPath := cfg.GetSessionDBPath()
			if dbPath == "" {
				dbPath = gateway.DefaultSessionDBPath()
			}

			fmt.Println("Session Storage Info")
			fmt.Println(strings.Repeat("─", 50))
			fmt.Printf("Database: %s\n", dbPath)

			// Check if file exists
			info, err := os.Stat(dbPath)
			if err != nil {
				fmt.Println("Status: Not initialized")
				return
			}

			fmt.Printf("Size: %s\n", formatBytes(info.Size()))
			fmt.Printf("Modified: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))

			// Open to get session count
			store, err := gateway.NewSessionStore(&gateway.SessionStoreConfig{
				DBPath:      dbPath,
				MaxSessions: 100,
			})
			if err != nil {
				fmt.Printf("Status: Error - %v\n", err)
				return
			}
			defer store.Close()

			sessions := store.ListSessions()
			fmt.Printf("Sessions: %d\n", len(sessions))
		},
	}
}

func loadConfigForSessions() *config.Config {
	// Try default config path
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".zen", "zen-claw", "config.yaml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// Return empty config, will use defaults
		return &config.Config{}
	}
	return cfg
}

func parseDuration(s string) (time.Duration, error) {
	// Handle day suffix
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var d int
		_, err := fmt.Sscanf(days, "%d", &d)
		if err != nil {
			return 0, err
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// getDefaultDBDir returns the default data directory
func getDefaultDBDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/zen-claw"
	}
	return filepath.Join(home, ".zen", "zen-claw", "data")
}
