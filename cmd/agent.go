package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/neves/zen-claw/internal/providers"
	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	var model string
	var provider string
	var workingDir string
	var sessionID string
	var showProgress bool
	var maxSteps int
	var verbose bool
	var useWebSocket bool
	var streamTokens bool

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "AI agent with read/write/edit tools",
		Long: `Zen Agent - AI assistant with full codebase access.

Fresh context by default (like Cursor). Named sessions are opt-in.

Examples:
  # Fresh session (no persistence)
  zen-claw agent "refactor this function"
  
  # Interactive mode
  zen-claw agent
  
  # Named session (persisted for later)
  zen-claw agent --session my-project "analyze codebase"
  
  # Resume a named session
  zen-claw agent --session my-project "continue from before"
  
  # Use WebSocket for bidirectional communication
  zen-claw agent --ws "analyze codebase"

Multi-AI modes (separate commands):
  zen-claw consensus   # 3 AIs → arbiter → better blueprints
  zen-claw factory     # Coordinator + specialist AIs`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			task := ""
			if len(args) > 0 {
				task = args[0]
			}
			runAgent(task, model, provider, workingDir, sessionID, showProgress, maxSteps, verbose, useWebSocket, streamTokens)
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "AI model (e.g., deepseek-chat)")
	cmd.Flags().StringVar(&provider, "provider", "", "AI provider (deepseek, openai, glm, minimax, qwen, kimi)")
	cmd.Flags().StringVar(&workingDir, "working-dir", ".", "Working directory for tools")
	cmd.Flags().StringVar(&sessionID, "session", "", "Named session to save/resume (omit for fresh context)")
	cmd.Flags().BoolVar(&showProgress, "progress", false, "Show progress in console (CLI only)")
	cmd.Flags().IntVar(&maxSteps, "max-steps", 100, "Maximum tool execution steps (default 100 for complex tasks)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose output for debugging")
	cmd.Flags().BoolVar(&useWebSocket, "ws", false, "Use WebSocket instead of SSE streaming")
	cmd.Flags().BoolVar(&streamTokens, "stream", false, "Stream AI response token-by-token")

	return cmd
}

func runAgent(task, modelFlag, providerFlag, workingDir, sessionID string, showProgress bool, maxSteps int, verbose bool, useWebSocket bool, streamTokens bool) {
	// Interactive mode if no task provided
	if task == "" {
		runInteractiveMode(modelFlag, providerFlag, workingDir, sessionID, showProgress, maxSteps, verbose, useWebSocket, streamTokens)
		return
	}
	_ = streamTokens // TODO: wire streaming through gateway SSE

	if verbose {
		fmt.Println("🔧 Verbose mode enabled")
	}

	// Use WebSocket if requested
	if useWebSocket {
		runAgentWebSocket(task, modelFlag, providerFlag, workingDir, sessionID, maxSteps, verbose)
		return
	}

	if showProgress {
		fmt.Println("🚀 Zen Agent - Gateway Client")
	} else {
		fmt.Println("🚀 Zen Agent - Gateway Client")
	}
	fmt.Println("═" + strings.Repeat("═", 78))
	fmt.Printf("Task: %s\n", task)
	if sessionID != "" {
		fmt.Printf("Session ID: %s (save for continuing in Slack/Telegram)\n", sessionID)
	}
	fmt.Printf("Working directory: %s\n", workingDir)

	// Create gateway client
	client := NewGatewayClient("http://localhost:8080")

	// Check if gateway is running
	if err := client.HealthCheck(); err != nil {
		fmt.Printf("\n❌ Gateway not available: %v\n", err)
		fmt.Println("   Start the gateway first: zen-claw gateway start")
		os.Exit(1)
	}

	if verbose {
		fmt.Println("✓ Gateway is healthy")
	}

	// Determine provider and model
	providerName := providerFlag
	modelName := modelFlag

	if modelName != "" && providerName == "" {
		providerName = inferProviderFromModel(modelName)
	}
	if providerName == "" {
		providerName = "deepseek"
	}
	if modelName == "" {
		modelName = providers.GetDefaultModel(providerName)
	}

	fmt.Printf("Provider: %s, Model: %s\n", providerName, modelName)

	// Prepare request
	req := ChatRequest{
		SessionID:  sessionID,
		UserInput:  task,
		WorkingDir: workingDir,
		Provider:   providerName,
		Model:      modelName,
		MaxSteps:   maxSteps,
	}

	fmt.Println()

	// Use streaming for better UX
	resp, err := client.SendWithProgress(req, func(event ProgressEvent) {
		displayProgressEvent(event)
	})
	if err != nil {
		fmt.Printf("\n❌ Gateway request failed: %v\n", err)
		os.Exit(1)
	}

	if resp.Error != "" {
		fmt.Printf("\n❌ Agent execution failed: %s\n", resp.Error)
		os.Exit(1)
	}

	// Print result
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("🎯 RESULT")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println(resp.Result)
	fmt.Println(strings.Repeat("═", 80))

	// Print session info from gateway response
	if sessionInfo := resp.SessionInfo; sessionInfo != nil {
		fmt.Printf("\n📊 Session Information:\n")
		if sid, ok := sessionInfo["session_id"].(string); ok {
			fmt.Printf("   Session ID: %s\n", sid)
		}
		if msgCount, ok := sessionInfo["message_count"].(float64); ok {
			fmt.Printf("   Messages: %.0f total\n", msgCount)
		}
		if userMsgs, ok := sessionInfo["user_messages"].(float64); ok {
			fmt.Printf("     - User: %.0f\n", userMsgs)
		}
		if assistantMsgs, ok := sessionInfo["assistant_messages"].(float64); ok {
			fmt.Printf("     - Assistant: %.0f\n", assistantMsgs)
		}
		if toolMsgs, ok := sessionInfo["tool_messages"].(float64); ok {
			fmt.Printf("     - Tool: %.0f\n", toolMsgs)
		}
		if wd, ok := sessionInfo["working_dir"].(string); ok {
			fmt.Printf("   Working directory: %s\n", wd)
		}
	}

	if showProgress {
		fmt.Printf("\n💡 To continue this session:\n")
		fmt.Printf("   zen-claw agent --session %s \"your next task\"\n", resp.SessionID)
	}
}

// runAgentWebSocket runs the agent using WebSocket connection
func runAgentWebSocket(task, modelFlag, providerFlag, workingDir, sessionID string, maxSteps int, verbose bool) {
	fmt.Println("🚀 Zen Agent (WebSocket)")
	fmt.Println("═" + strings.Repeat("═", 78))
	fmt.Printf("Task: %s\n", task)
	fmt.Printf("Working directory: %s\n", workingDir)

	// Connect via WebSocket
	wsURL := "ws://localhost:8080/ws"
	if verbose {
		fmt.Printf("Connecting to %s...\n", wsURL)
	}

	client, err := NewWSClient(wsURL)
	if err != nil {
		fmt.Printf("\n❌ WebSocket connection failed: %v\n", err)
		fmt.Println("   Start the gateway first: zen-claw gateway start")
		os.Exit(1)
	}
	defer client.Close()

	fmt.Println("✓ WebSocket connected")

	// Determine provider and model
	providerName := providerFlag
	modelName := modelFlag

	if modelName != "" && providerName == "" {
		providerName = inferProviderFromModel(modelName)
	}
	if providerName == "" {
		providerName = "deepseek"
	}
	if modelName == "" {
		modelName = providers.GetDefaultModel(providerName)
	}

	fmt.Printf("Provider: %s, Model: %s\n", providerName, modelName)
	fmt.Println()

	// Create request
	req := WSChatRequest{
		SessionID:  sessionID,
		UserInput:  task,
		WorkingDir: workingDir,
		Provider:   providerName,
		Model:      modelName,
		MaxSteps:   maxSteps,
	}

	// Run chat with progress
	var finalResp *ChatResponse
	var finalErr error
	done := make(chan struct{})

	go func() {
		client.Chat(req, func(event ProgressEvent) {
			displayProgressEvent(event)
		}, func(resp *ChatResponse, err error) {
			finalResp = resp
			finalErr = err
			close(done)
		})
	}()

	<-done

	if finalErr != nil {
		fmt.Printf("\n❌ Agent error: %v\n", finalErr)
		os.Exit(1)
	}

	// Print result
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("🎯 RESULT (via WebSocket)")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println(finalResp.Result)
	fmt.Println(strings.Repeat("═", 80))

	// Print session info
	if finalResp.SessionInfo != nil {
		fmt.Printf("\n📊 Session Information:\n")
		if sid, ok := finalResp.SessionInfo["session_id"].(string); ok {
			fmt.Printf("   Session ID: %s\n", sid)
		}
		if msgCount, ok := finalResp.SessionInfo["message_count"].(float64); ok {
			fmt.Printf("   Messages: %.0f total\n", msgCount)
		}
	}
}
