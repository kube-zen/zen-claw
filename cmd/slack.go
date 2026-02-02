package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/neves/zen-claw/internal/slack"
	"github.com/spf13/cobra"
)

// newSlackCmd creates the Slack bot command
func newSlackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slack",
		Short: "Slack bot integration for Zen Claw",
		RunE:  runSlackBot,
	}

	cmd.Flags().String("port", "3000", "Port for Slack bot server")
	cmd.Flags().String("slack-token", "", "Slack bot token (or SLACK_BOT_TOKEN env)")
	cmd.Flags().String("signing-secret", "", "Slack signing secret (or SLACK_SIGNING_SECRET env)")
	cmd.Flags().String("gateway-url", "http://localhost:8080", "Zen Claw gateway URL")

	return cmd
}

func runSlackBot(cmd *cobra.Command, args []string) error {
	// Get Slack credentials
	slackToken, _ := cmd.Flags().GetString("slack-token")
	if slackToken == "" {
		slackToken = slack.GetSlackTokenFromEnv()
	}
	if slackToken == "" {
		return fmt.Errorf("Slack bot token required. Set --slack-token or SLACK_BOT_TOKEN env")
	}

	signingSecret, _ := cmd.Flags().GetString("signing-secret")
	if signingSecret == "" {
		signingSecret = os.Getenv("SLACK_SIGNING_SECRET")
	}

	gatewayURL, _ := cmd.Flags().GetString("gateway-url")
	port, _ := cmd.Flags().GetString("port")

	// Create Slack bot
	bot := slack.NewBot(slack.Config{
		SlackToken:    slackToken,
		SigningSecret: signingSecret,
		GatewayURL:    gatewayURL,
	})

	// Create HTTP server
	http.HandleFunc("/slack/events", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Verify Slack signature (optional but recommended)
		if signingSecret != "" {
			// TODO: Implement Slack signature verification
			// https://api.slack.com/authentication/verifying-requests-from-slack
		}

		var event slack.SlackEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Handle URL verification challenge
		if event.Type == "url_verification" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, event.Challenge)
			return
		}

		// Handle event in goroutine to respond quickly to Slack
		go func() {
			if err := bot.HandleEvent(ctx, event); err != nil {
				log.Printf("Error handling Slack event: %v", err)
			}
		}()

		w.WriteHeader(http.StatusOK)
	})

	// Health endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
			"service": "zen-claw-slack",
		})
	})

	// Root endpoint
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, `Zen Claw Slack Bot
		
Endpoints:
  POST /slack/events  - Slack Events API
  GET  /health        - Health check

Configuration:
  Gateway URL: %s
  Server port: %s
`, gatewayURL, port)
	})

	log.Printf("Starting Zen Claw Slack bot server on :%s", port)
	log.Printf("  Gateway URL: %s", gatewayURL)
	log.Printf("  Health check: http://localhost:%s/health", port)

	return http.ListenAndServe(":"+port, nil)
}