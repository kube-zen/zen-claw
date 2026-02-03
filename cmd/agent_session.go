package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/gateway"
)

// displaySessionsInfo shows session storage info in interactive mode
func displaySessionsInfo() {
	cfg := loadConfigForAgent()
	dbPath := cfg.GetSessionDBPath()
	if dbPath == "" {
		dbPath = gateway.DefaultSessionDBPath()
	}

	fmt.Println("\n📦 Session Storage Info")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Database: %s\n", dbPath)

	info, err := os.Stat(dbPath)
	if err != nil {
		fmt.Println("Status: Not initialized")
		return
	}

	fmt.Printf("Size: %s\n", formatBytesAgent(info.Size()))
	fmt.Printf("Modified: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))

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
	fmt.Println(strings.Repeat("─", 50))
}

// cleanSessionsInteractive handles /sessions clean in interactive mode
func cleanSessionsInteractive(args string) {
	args = strings.TrimSpace(args)

	cfg := loadConfigForAgent()
	store, err := gateway.NewSessionStore(&gateway.SessionStoreConfig{
		DBPath:      cfg.GetSessionDBPath(),
		MaxSessions: 100,
	})
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	defer store.Close()

	if args == "--all" || args == "all" {
		count, err := store.CleanAllSessions()
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}
		fmt.Printf("✓ Deleted %d sessions\n", count)
		return
	}

	if strings.HasPrefix(args, "--older ") || strings.HasPrefix(args, "older ") {
		durationStr := strings.TrimPrefix(args, "--older ")
		durationStr = strings.TrimPrefix(durationStr, "older ")
		durationStr = strings.TrimSpace(durationStr)

		duration, err := parseDurationAgent(durationStr)
		if err != nil {
			fmt.Printf("❌ Invalid duration: %v\n", err)
			fmt.Println("Examples: 24h, 7d, 30d")
			return
		}
		count, err := store.CleanOldSessions(duration)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}
		fmt.Printf("✓ Deleted %d sessions older than %s\n", count, durationStr)
		return
	}

	fmt.Println("Usage: /sessions clean [--all | --older <duration>]")
	fmt.Println("Examples:")
	fmt.Println("  /sessions clean --all        # Delete all sessions")
	fmt.Println("  /sessions clean --older 7d   # Delete sessions older than 7 days")
	fmt.Println("  /sessions clean --older 24h  # Delete sessions older than 24 hours")
}

// deleteSessionByName deletes a specific session
func deleteSessionByName(name string) {
	cfg := loadConfigForAgent()
	store, err := gateway.NewSessionStore(&gateway.SessionStoreConfig{
		DBPath:      cfg.GetSessionDBPath(),
		MaxSessions: 100,
	})
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	defer store.Close()

	if store.DeleteSession(name) {
		fmt.Printf("✓ Deleted session: %s\n", name)
	} else {
		fmt.Printf("❌ Session not found: %s\n", name)
	}
}

// loadConfigForAgent loads config for agent commands
func loadConfigForAgent() *config.Config {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".zen", "zen-claw", "config.yaml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return &config.Config{}
	}
	return cfg
}

// parseDurationAgent parses duration string with day support
func parseDurationAgent(s string) (time.Duration, error) {
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

// formatBytesAgent formats bytes for display
func formatBytesAgent(b int64) string {
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
