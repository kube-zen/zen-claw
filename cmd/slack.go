package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	slackbot "github.com/neves/zen-claw/internal/slack"
	"github.com/spf13/cobra"
)

func newSlackCmd() *cobra.Command {
	var botToken string
	var appToken string
	var gatewayURL string
	var workingDir string
	var provider string
	var model string
	var maxSteps int
	var debug bool

	cmd := &cobra.Command{
		Use:   "slack",
		Short: "Start the Slack bot",
		Long: `Start the Zen Claw Slack bot that connects to the gateway via WebSocket.

The Slack bot allows users to interact with the AI agent directly from Slack.
Each thread becomes a separate session with its own context.

Required environment variables:
  SLACK_BOT_TOKEN  - Bot User OAuth Token (xoxb-...)
  SLACK_APP_TOKEN  - App-Level Token for Socket Mode (xapp-...)

Or use the --bot-token and --app-token flags.

Setup:
  1. Create a Slack App at https://api.slack.com/apps
  2. Enable Socket Mode (Settings > Socket Mode)
  3. Add Bot Token Scopes: app_mentions:read, chat:write, im:history, im:read, im:write
  4. Subscribe to Events: app_mention, message.im
  5. Install to workspace
  6. Copy the Bot Token and App Token

Examples:
  # Start with environment variables
  export SLACK_BOT_TOKEN="xoxb-..."
  export SLACK_APP_TOKEN="xapp-..."
  zen-claw slack

  # Start with flags
  zen-claw slack --bot-token xoxb-... --app-token xapp-...

  # Custom gateway and working directory
  zen-claw slack --gateway ws://localhost:8080/ws --dir /home/user/projects`,
		Run: func(cmd *cobra.Command, args []string) {
			runSlackBot(slackbot.Config{
				BotToken:   botToken,
				AppToken:   appToken,
				GatewayURL: gatewayURL,
				DefaultDir: workingDir,
				Provider:   provider,
				Model:      model,
				MaxSteps:   maxSteps,
				Debug:      debug,
			})
		},
	}

	cmd.Flags().StringVar(&botToken, "bot-token", "", "Slack Bot Token (xoxb-...) [env: SLACK_BOT_TOKEN]")
	cmd.Flags().StringVar(&appToken, "app-token", "", "Slack App Token for Socket Mode (xapp-...) [env: SLACK_APP_TOKEN]")
	cmd.Flags().StringVar(&gatewayURL, "gateway", "ws://localhost:8080/ws", "zen-claw gateway WebSocket URL")
	cmd.Flags().StringVar(&workingDir, "dir", ".", "Default working directory for tools")
	cmd.Flags().StringVar(&provider, "provider", "", "Default AI provider (deepseek, openai, qwen, glm, minimax, kimi)")
	cmd.Flags().StringVar(&model, "model", "", "Default AI model")
	cmd.Flags().IntVar(&maxSteps, "max-steps", 100, "Maximum tool execution steps")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")

	return cmd
}

func runSlackBot(cfg slackbot.Config) {
	fmt.Println("🤖 Zen Claw Slack Bot")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")

	// Validate tokens
	if cfg.BotToken == "" {
		cfg.BotToken = os.Getenv("SLACK_BOT_TOKEN")
	}
	if cfg.AppToken == "" {
		cfg.AppToken = os.Getenv("SLACK_APP_TOKEN")
	}

	if cfg.BotToken == "" {
		fmt.Println("❌ SLACK_BOT_TOKEN is required")
		fmt.Println("   Set via environment variable or --bot-token flag")
		os.Exit(1)
	}
	if cfg.AppToken == "" {
		fmt.Println("❌ SLACK_APP_TOKEN is required for Socket Mode")
		fmt.Println("   Set via environment variable or --app-token flag")
		os.Exit(1)
	}

	fmt.Printf("Gateway: %s\n", cfg.GatewayURL)
	fmt.Printf("Working Dir: %s\n", cfg.DefaultDir)
	if cfg.Provider != "" {
		fmt.Printf("Provider: %s\n", cfg.Provider)
	}
	if cfg.Model != "" {
		fmt.Printf("Model: %s\n", cfg.Model)
	}
	fmt.Printf("Max Steps: %d\n", cfg.MaxSteps)
	fmt.Println()

	// Create bot
	bot, err := slackbot.NewBot(cfg)
	if err != nil {
		fmt.Printf("❌ Failed to create bot: %v\n", err)
		os.Exit(1)
	}

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n⏹️  Shutting down...")
		bot.Stop()
		os.Exit(0)
	}()

	// Start bot
	fmt.Println("🚀 Starting Slack bot (Socket Mode)...")
	fmt.Println("   Mention me in Slack or DM me to start!")
	fmt.Println()

	if err := bot.Start(); err != nil {
		log.Fatalf("Bot error: %v", err)
	}
}
