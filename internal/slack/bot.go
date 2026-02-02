package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Bot handles Slack integration for Zen Claw
type Bot struct {
	slackToken    string
	signingSecret string
	gatewayURL    string
	httpClient    *http.Client
	sessions      map[string]string // channelID:threadTS -> sessionID
	sessionsMu    sync.RWMutex
}

// Config holds Slack bot configuration
type Config struct {
	SlackToken    string
	SigningSecret string
	GatewayURL    string
}

// NewBot creates a new Slack bot
func NewBot(cfg Config) *Bot {
	return &Bot{
		slackToken:    cfg.SlackToken,
		signingSecret: cfg.SigningSecret,
		gatewayURL:    cfg.GatewayURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		sessions: make(map[string]string),
	}
}

// SlackEvent represents a Slack event from Events API
type SlackEvent struct {
	Type      string `json:"type"`
	Event     struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
		User    string `json:"user"`
		Text    string `json:"text"`
		TS      string `json:"ts"`
		ThreadTS string `json:"thread_ts,omitempty"`
	} `json:"event"`
	Challenge string `json:"challenge"`
}

// HandleEvent handles Slack events from Events API
func (b *Bot) HandleEvent(ctx context.Context, event SlackEvent) error {
	// Handle URL verification challenge
	if event.Type == "url_verification" {
		log.Printf("URL verification challenge received")
		return nil
	}

	// Handle message events
	if event.Event.Type == "message" {
		// Skip bot messages (user IDs starting with B) and empty messages
		if strings.HasPrefix(event.Event.User, "B") || event.Event.Text == "" {
			return nil
		}

		log.Printf("Received Slack message: user=%s channel=%s text=%s thread_ts=%s",
			event.Event.User, event.Event.Channel, event.Event.Text, event.Event.ThreadTS)

		// Process message with Zen Claw gateway
		return b.processMessage(ctx, event.Event.Channel, event.Event.User, 
			event.Event.Text, event.Event.ThreadTS)
	}

	return nil
}

// processMessage sends message to Zen Claw gateway and responds to Slack
func (b *Bot) processMessage(ctx context.Context, channelID, userID, text, threadTS string) error {
	// Get or create session for this channel/thread
	sessionKey := channelID
	if threadTS != "" {
		sessionKey = channelID + ":" + threadTS
	}

	b.sessionsMu.RLock()
	sessionID, hasSession := b.sessions[sessionKey]
	b.sessionsMu.RUnlock()
	
	// Prepare request to Zen Claw gateway
	gatewayReq := map[string]interface{}{
		"messages": []map[string]string{
			{
				"role": "user",
				"content": fmt.Sprintf("Slack user %s in channel %s says: %s", 
					userID, channelID, text),
			},
		},
	}

	if hasSession {
		gatewayReq["session_id"] = sessionID
	}

	// Send to gateway
	reqBody, err := json.Marshal(gatewayReq)
	if err != nil {
		return fmt.Errorf("failed to marshal gateway request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/chat", b.gatewayURL),
		bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create gateway request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway returned status %d", resp.StatusCode)
	}

	var gatewayResp struct {
		Response  string `json:"response"`
		SessionID string `json:"session_id,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gatewayResp); err != nil {
		return fmt.Errorf("failed to decode gateway response: %w", err)
	}

	// Store session ID
	if gatewayResp.SessionID != "" {
		b.sessionsMu.Lock()
		b.sessions[sessionKey] = gatewayResp.SessionID
		b.sessionsMu.Unlock()
	}

	// Send response back to Slack
	return b.sendSlackMessage(ctx, channelID, gatewayResp.Response, threadTS)
}

// sendSlackMessage sends a message to Slack using chat.postMessage API
func (b *Bot) sendSlackMessage(ctx context.Context, channelID, text, threadTS string) error {
	payload := map[string]interface{}{
		"channel": channelID,
		"text":    text,
	}
	if threadTS != "" {
		payload["thread_ts"] = threadTS
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	url := "https://slack.com/api/chat.postMessage"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create Slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.slackToken)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var slackResp struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&slackResp)
		return fmt.Errorf("Slack API error: %s", slackResp.Error)
	}

	log.Printf("Sent message to Slack: channel=%s thread_ts=%s", channelID, threadTS)
	return nil
}

// GetSlackTokenFromEnv gets Slack token from environment with fallbacks
// Similar to pattern used in zen-platform SlackNotificationService
func GetSlackTokenFromEnv() string {
	// Check environment variable first
	if token := os.Getenv("SLACK_BOT_TOKEN"); token != "" {
		return token
	}
	
	// Could add zen-lock or other fallbacks here
	// Following the pattern from SlackNotificationService
	return ""
}