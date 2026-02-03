package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Config holds Slack bot configuration
type Config struct {
	BotToken   string // xoxb-...
	AppToken   string // xapp-... (for Socket Mode)
	GatewayURL string // WebSocket URL for zen-claw gateway
	DefaultDir string // Default working directory
	MaxSteps   int    // Max agent steps
	Provider   string // Default AI provider
	Model      string // Default AI model
	Debug      bool   // Enable debug logging
}

// Bot represents the Slack bot
type Bot struct {
	config       Config
	client       *slack.Client
	socketClient *socketmode.Client
	gateway      *GatewayClient
	sessions     map[string]*Session // thread_ts -> session
	sessionsMu   sync.RWMutex
	botUserID    string
	ctx          context.Context
	cancel       context.CancelFunc
}

// Session represents a conversation session tied to a Slack thread
type Session struct {
	ThreadTS     string // Slack thread timestamp
	ChannelID    string // Slack channel ID
	SessionID    string // zen-claw session ID
	WorkingDir   string // Working directory
	Provider     string // AI provider
	Model        string // AI model
	CreatedAt    time.Time
	LastUsedAt   time.Time
	MessageCount int
}

// GatewayClient handles WebSocket communication with zen-claw gateway
type GatewayClient struct {
	url        string
	conn       *websocket.Conn
	mu         sync.Mutex
	msgID      int
	callbacks  map[string]chan WSMessage
	callbackMu sync.Mutex
	done       chan struct{}
}

// WSMessage matches the gateway WebSocket message format
type WSMessage struct {
	Type string          `json:"type"`
	ID   string          `json:"id,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// NewBot creates a new Slack bot
func NewBot(cfg Config) (*Bot, error) {
	if cfg.BotToken == "" {
		cfg.BotToken = os.Getenv("SLACK_BOT_TOKEN")
	}
	if cfg.AppToken == "" {
		cfg.AppToken = os.Getenv("SLACK_APP_TOKEN")
	}
	if cfg.GatewayURL == "" {
		cfg.GatewayURL = "ws://localhost:8080/ws"
	}
	if cfg.DefaultDir == "" {
		cfg.DefaultDir = "."
	}
	if cfg.MaxSteps == 0 {
		cfg.MaxSteps = 100
	}

	if cfg.BotToken == "" {
		return nil, fmt.Errorf("SLACK_BOT_TOKEN is required")
	}
	if cfg.AppToken == "" {
		return nil, fmt.Errorf("SLACK_APP_TOKEN is required for Socket Mode")
	}

	client := slack.New(
		cfg.BotToken,
		slack.OptionDebug(cfg.Debug),
		slack.OptionAppLevelToken(cfg.AppToken),
	)

	socketClient := socketmode.New(
		client,
		socketmode.OptionDebug(cfg.Debug),
	)

	ctx, cancel := context.WithCancel(context.Background())

	bot := &Bot{
		config:       cfg,
		client:       client,
		socketClient: socketClient,
		sessions:     make(map[string]*Session),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Get bot user ID
	authTest, err := client.AuthTest()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("auth test failed: %w", err)
	}
	bot.botUserID = authTest.UserID
	log.Printf("[Slack] Bot user ID: %s", bot.botUserID)

	return bot, nil
}

// Start starts the Slack bot
func (b *Bot) Start() error {
	// Connect to zen-claw gateway
	gateway, err := NewGatewayClient(b.config.GatewayURL)
	if err != nil {
		return fmt.Errorf("failed to connect to gateway: %w", err)
	}
	b.gateway = gateway
	log.Printf("[Slack] Connected to gateway at %s", b.config.GatewayURL)

	// Start event handler
	go b.handleEvents()

	// Start socket mode
	log.Printf("[Slack] Starting Socket Mode...")
	return b.socketClient.Run()
}

// Stop stops the bot
func (b *Bot) Stop() {
	b.cancel()
	if b.gateway != nil {
		b.gateway.Close()
	}
}

// handleEvents handles incoming Slack events
func (b *Bot) handleEvents() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case evt := <-b.socketClient.Events:
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				b.handleEventAPI(evt)
			case socketmode.EventTypeSlashCommand:
				b.handleSlashCommand(evt)
			case socketmode.EventTypeInteractive:
				b.handleInteractive(evt)
			}
		}
	}
}

// handleEventAPI handles Events API events
func (b *Bot) handleEventAPI(evt socketmode.Event) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	// Acknowledge the event
	b.socketClient.Ack(*evt.Request)

	switch eventsAPIEvent.Type {
	case slackevents.CallbackEvent:
		b.handleCallbackEvent(eventsAPIEvent)
	}
}

// handleCallbackEvent handles callback events
func (b *Bot) handleCallbackEvent(event slackevents.EventsAPIEvent) {
	switch ev := event.InnerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		b.handleMention(ev)
	case *slackevents.MessageEvent:
		// Only handle messages in threads we're tracking or DMs
		if ev.ThreadTimeStamp != "" || ev.ChannelType == "im" {
			b.handleMessage(ev)
		}
	}
}

// handleMention handles @mentions of the bot
func (b *Bot) handleMention(ev *slackevents.AppMentionEvent) {
	// Remove the bot mention from the text
	text := strings.TrimSpace(strings.Replace(ev.Text, fmt.Sprintf("<@%s>", b.botUserID), "", 1))

	if text == "" {
		b.sendHelp(ev.Channel, ev.TimeStamp)
		return
	}

	// Check for commands
	if strings.HasPrefix(text, "/") {
		b.handleBotCommand(ev.Channel, ev.TimeStamp, ev.User, text)
		return
	}

	// Process as AI request
	b.processAIRequest(ev.Channel, ev.TimeStamp, ev.User, text)
}

// handleMessage handles messages (in threads or DMs)
func (b *Bot) handleMessage(ev *slackevents.MessageEvent) {
	// Ignore bot messages
	if ev.BotID != "" || ev.User == b.botUserID {
		return
	}

	// Get thread context
	threadTS := ev.ThreadTimeStamp
	if threadTS == "" {
		threadTS = ev.TimeStamp
	}

	// Check if this is a tracked session
	b.sessionsMu.RLock()
	session, exists := b.sessions[threadTS]
	b.sessionsMu.RUnlock()

	if !exists && ev.ChannelType != "im" {
		// Not a tracked thread and not a DM, ignore
		return
	}

	text := strings.TrimSpace(ev.Text)
	if text == "" {
		return
	}

	// Check for commands
	if strings.HasPrefix(text, "/") {
		b.handleBotCommand(ev.Channel, threadTS, ev.User, text)
		return
	}

	// Process as continuation of conversation
	b.processAIRequest(ev.Channel, threadTS, ev.User, text)

	// Update session last used
	if session != nil {
		b.sessionsMu.Lock()
		session.LastUsedAt = time.Now()
		session.MessageCount++
		b.sessionsMu.Unlock()
	}
}

// handleBotCommand handles bot commands
func (b *Bot) handleBotCommand(channel, threadTS, user, text string) {
	parts := strings.Fields(text)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help":
		b.sendHelp(channel, threadTS)

	case "/status":
		b.sendStatus(channel, threadTS)

	case "/sessions":
		b.sendSessionList(channel, threadTS)

	case "/attach":
		if len(parts) < 2 {
			b.sendMessage(channel, threadTS, "❌ Usage: `/attach <session_id>`")
			return
		}
		b.attachSession(channel, threadTS, parts[1])

	case "/detach":
		b.detachSession(channel, threadTS)

	case "/provider":
		if len(parts) < 2 {
			b.sendMessage(channel, threadTS, "❌ Usage: `/provider <name>` (deepseek, openai, qwen, glm, minimax, kimi)")
			return
		}
		b.setProvider(channel, threadTS, parts[1])

	case "/model":
		if len(parts) < 2 {
			b.sendMessage(channel, threadTS, "❌ Usage: `/model <name>`")
			return
		}
		b.setModel(channel, threadTS, parts[1])

	case "/dir":
		if len(parts) < 2 {
			b.sendMessage(channel, threadTS, "❌ Usage: `/dir <path>`")
			return
		}
		b.setWorkingDir(channel, threadTS, parts[1])

	case "/cancel":
		b.cancelTask(channel, threadTS)

	case "/clear":
		b.clearSession(channel, threadTS)

	default:
		b.sendMessage(channel, threadTS, fmt.Sprintf("❌ Unknown command: `%s`. Try `/help`", cmd))
	}
}

// processAIRequest sends a request to the AI agent
func (b *Bot) processAIRequest(channel, threadTS, user, text string) {
	// Get or create session
	session := b.getOrCreateSession(channel, threadTS)

	// Send typing indicator
	b.client.SendMessage(channel, slack.MsgOptionTS(threadTS), slack.MsgOptionText("🤔 Thinking...", false))

	// Create progress message
	progressMsgTS := b.sendProgressStart(channel, threadTS, session)

	// Send request to gateway
	go func() {
		result, err := b.gateway.Chat(ChatRequest{
			SessionID:  session.SessionID,
			UserInput:  text,
			WorkingDir: session.WorkingDir,
			Provider:   session.Provider,
			Model:      session.Model,
			MaxSteps:   b.config.MaxSteps,
		}, func(event ProgressEvent) {
			// Update progress message
			b.updateProgress(channel, progressMsgTS, event)
		})

		if err != nil {
			b.sendMessage(channel, threadTS, fmt.Sprintf("❌ Error: %s", err.Error()))
			return
		}

		// Update session ID if new
		if result.SessionID != "" && session.SessionID == "" {
			b.sessionsMu.Lock()
			session.SessionID = result.SessionID
			b.sessionsMu.Unlock()
		}

		// Send final result
		b.sendResult(channel, threadTS, result)
	}()
}

// getOrCreateSession gets or creates a session for a thread
func (b *Bot) getOrCreateSession(channel, threadTS string) *Session {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()

	if session, exists := b.sessions[threadTS]; exists {
		return session
	}

	session := &Session{
		ThreadTS:   threadTS,
		ChannelID:  channel,
		WorkingDir: b.config.DefaultDir,
		Provider:   b.config.Provider,
		Model:      b.config.Model,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
	}
	b.sessions[threadTS] = session
	return session
}

// sendProgressStart sends the initial progress message
func (b *Bot) sendProgressStart(channel, threadTS string, session *Session) string {
	provider := session.Provider
	if provider == "" {
		provider = "deepseek"
	}
	model := session.Model
	if model == "" {
		model = "default"
	}

	_, ts, _ := b.client.PostMessage(
		channel,
		slack.MsgOptionTS(threadTS),
		slack.MsgOptionBlocks(
			slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("🚀 *Starting* with `%s/%s`...", provider, model), false, false),
				nil, nil,
			),
		),
	)
	return ts
}

// updateProgress updates the progress message
func (b *Bot) updateProgress(channel, msgTS string, event ProgressEvent) {
	var text string
	switch event.Type {
	case "step":
		text = fmt.Sprintf("📍 %s", event.Message)
	case "thinking":
		text = fmt.Sprintf("💭 %s", event.Message)
	case "ai_response":
		msg := event.Message
		if len(msg) > 200 {
			msg = msg[:197] + "..."
		}
		text = fmt.Sprintf("🤖 %s", msg)
	case "tool_call":
		text = fmt.Sprintf("🔧 %s", event.Message)
	case "tool_result":
		text = fmt.Sprintf("✓ %s", event.Message)
	case "complete":
		text = fmt.Sprintf("✅ %s", event.Message)
	case "error":
		text = fmt.Sprintf("❌ %s", event.Message)
	default:
		return
	}

	b.client.UpdateMessage(
		channel,
		msgTS,
		slack.MsgOptionBlocks(
			slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn", text, false, false),
				nil, nil,
			),
		),
	)
}

// sendResult sends the final result
func (b *Bot) sendResult(channel, threadTS string, result *ChatResult) {
	// Truncate long results
	text := result.Result
	if len(text) > 3000 {
		text = text[:2997] + "..."
	}

	blocks := []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text", "🎯 Result", false, false),
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", text, false, false),
			nil, nil,
		),
	}

	// Add session info
	if result.SessionInfo != nil {
		var fields []*slack.TextBlockObject
		if sid, ok := result.SessionInfo["session_id"].(string); ok {
			fields = append(fields, slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Session:* `%s`", sid), false, false))
		}
		if msgCount, ok := result.SessionInfo["message_count"].(float64); ok {
			fields = append(fields, slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Messages:* %.0f", msgCount), false, false))
		}
		if len(fields) > 0 {
			blocks = append(blocks, slack.NewSectionBlock(nil, fields, nil))
		}
	}

	b.client.PostMessage(channel, slack.MsgOptionTS(threadTS), slack.MsgOptionBlocks(blocks...))
}

// sendMessage sends a simple message
func (b *Bot) sendMessage(channel, threadTS, text string) {
	b.client.PostMessage(channel, slack.MsgOptionTS(threadTS), slack.MsgOptionText(text, false))
}

// sendHelp sends help information
func (b *Bot) sendHelp(channel, threadTS string) {
	blocks := []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text", "🤖 Zen Claw - AI Agent", false, false),
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "*Usage:* Mention me with a task, or continue in a thread.", false, false),
			nil, nil,
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "*Commands:*\n"+
				"• `/help` - Show this help\n"+
				"• `/status` - Show current session status\n"+
				"• `/sessions` - List all sessions\n"+
				"• `/attach <id>` - Attach to existing session\n"+
				"• `/detach` - Detach from current session\n"+
				"• `/provider <name>` - Set AI provider\n"+
				"• `/model <name>` - Set AI model\n"+
				"• `/dir <path>` - Set working directory\n"+
				"• `/cancel` - Cancel current task\n"+
				"• `/clear` - Clear session history", false, false),
			nil, nil,
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "*Providers:* deepseek, openai, qwen, glm, minimax, kimi", false, false),
			nil, nil,
		),
	}

	b.client.PostMessage(channel, slack.MsgOptionTS(threadTS), slack.MsgOptionBlocks(blocks...))
}

// sendStatus sends session status
func (b *Bot) sendStatus(channel, threadTS string) {
	b.sessionsMu.RLock()
	session, exists := b.sessions[threadTS]
	b.sessionsMu.RUnlock()

	if !exists {
		b.sendMessage(channel, threadTS, "ℹ️ No active session in this thread. Start by sending a message.")
		return
	}

	provider := session.Provider
	if provider == "" {
		provider = "deepseek (default)"
	}
	model := session.Model
	if model == "" {
		model = "default"
	}

	text := fmt.Sprintf("*Session Status*\n"+
		"• *Session ID:* `%s`\n"+
		"• *Provider:* %s\n"+
		"• *Model:* %s\n"+
		"• *Working Dir:* `%s`\n"+
		"• *Messages:* %d\n"+
		"• *Created:* %s\n"+
		"• *Last Used:* %s",
		session.SessionID,
		provider,
		model,
		session.WorkingDir,
		session.MessageCount,
		session.CreatedAt.Format(time.RFC3339),
		session.LastUsedAt.Format(time.RFC3339),
	)

	b.sendMessage(channel, threadTS, text)
}

// sendSessionList lists all sessions
func (b *Bot) sendSessionList(channel, threadTS string) {
	b.sessionsMu.RLock()
	defer b.sessionsMu.RUnlock()

	if len(b.sessions) == 0 {
		b.sendMessage(channel, threadTS, "ℹ️ No active sessions.")
		return
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("*Active Sessions:* %d", len(b.sessions)))
	for _, s := range b.sessions {
		lines = append(lines, fmt.Sprintf("• `%s` - %s (%d msgs)", s.SessionID, s.WorkingDir, s.MessageCount))
	}

	b.sendMessage(channel, threadTS, strings.Join(lines, "\n"))
}

// attachSession attaches to an existing session
func (b *Bot) attachSession(channel, threadTS, sessionID string) {
	session := b.getOrCreateSession(channel, threadTS)
	session.SessionID = sessionID
	b.sendMessage(channel, threadTS, fmt.Sprintf("✅ Attached to session `%s`", sessionID))
}

// detachSession detaches from current session
func (b *Bot) detachSession(channel, threadTS string) {
	b.sessionsMu.Lock()
	delete(b.sessions, threadTS)
	b.sessionsMu.Unlock()
	b.sendMessage(channel, threadTS, "✅ Session detached. Next message will start fresh.")
}

// setProvider sets the AI provider for a session
func (b *Bot) setProvider(channel, threadTS, provider string) {
	session := b.getOrCreateSession(channel, threadTS)
	session.Provider = provider
	b.sendMessage(channel, threadTS, fmt.Sprintf("✅ Provider set to `%s`", provider))
}

// setModel sets the AI model for a session
func (b *Bot) setModel(channel, threadTS, model string) {
	session := b.getOrCreateSession(channel, threadTS)
	session.Model = model
	b.sendMessage(channel, threadTS, fmt.Sprintf("✅ Model set to `%s`", model))
}

// setWorkingDir sets the working directory for a session
func (b *Bot) setWorkingDir(channel, threadTS, dir string) {
	session := b.getOrCreateSession(channel, threadTS)
	session.WorkingDir = dir
	b.sendMessage(channel, threadTS, fmt.Sprintf("✅ Working directory set to `%s`", dir))
}

// cancelTask cancels the current task
func (b *Bot) cancelTask(channel, threadTS string) {
	if err := b.gateway.Cancel(); err != nil {
		b.sendMessage(channel, threadTS, fmt.Sprintf("❌ Cancel failed: %s", err.Error()))
		return
	}
	b.sendMessage(channel, threadTS, "✅ Task cancelled.")
}

// clearSession clears session history
func (b *Bot) clearSession(channel, threadTS string) {
	b.sessionsMu.Lock()
	if session, exists := b.sessions[threadTS]; exists {
		session.SessionID = "" // Clear session ID to start fresh
		session.MessageCount = 0
	}
	b.sessionsMu.Unlock()
	b.sendMessage(channel, threadTS, "✅ Session cleared. Next message will start fresh context.")
}

// handleSlashCommand handles slash commands
func (b *Bot) handleSlashCommand(evt socketmode.Event) {
	cmd, ok := evt.Data.(slack.SlashCommand)
	if !ok {
		return
	}

	b.socketClient.Ack(*evt.Request)

	// Process slash command
	switch cmd.Command {
	case "/zen", "/zenclaw", "/zen-claw":
		if cmd.Text == "" {
			b.sendHelp(cmd.ChannelID, "")
			return
		}
		// Process as AI request (create new thread)
		b.processAIRequest(cmd.ChannelID, "", cmd.UserID, cmd.Text)
	}
}

// handleInteractive handles interactive components
func (b *Bot) handleInteractive(evt socketmode.Event) {
	callback, ok := evt.Data.(slack.InteractionCallback)
	if !ok {
		return
	}

	b.socketClient.Ack(*evt.Request)

	// Handle button actions, etc.
	log.Printf("[Slack] Interactive callback: %s", callback.Type)
}
