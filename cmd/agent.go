package cmd

import (
	"errors"

	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
)

// Global variables for thinking cursor
// Removed duplicate cleanupOldSessions function (duplicate with session_manager.go)

// Session ID validation function
func validateSessionID(sessionID string) string {
    if sessionID == "" {
        return fmt.Sprintf("sess-%d", time.Now().Unix())
    }
    if len(sessionID) < 5 {
        return fmt.Sprintf("sess-%d", time.Now().Unix())
    }
    return sessionID
}
var thinkingCursorActive = false
var thinkingCursorTicker *time.Ticker
var thinkingCursorStop chan bool

func newAgentCmd() *cobra.Command {
	var model string
	var provider string
	var workingDir string
	var sessionID string
	var showProgress bool
	var maxSteps int
	var verbose bool
	var think bool

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Lightweight AI agent (console → Slack/Telegram compatible)",
		Long: `Zen Agent - Lightweight AI agent with automatic tool chaining.

Designed for multi-client sessions:
- Start in console, continue in Slack/Telegram
- Minimal footprint: Agent only does tool execution
- Session-based: Save session ID to continue later
- No baked-in presentation logic

Architecture:
  Agent (tool execution) ← Session (conversation state)
       ↑
  Clients (Console, Slack, Telegram, HTTP)

Examples:
  # Start a new session
  zen-claw agent "analyze project"
  
  # Start interactive mode (no arguments)
  zen-claw agent
  
  # Start with session ID (save for continuing)
  zen-claw agent --session-id my-task "check codebase"
  
  # Run with verbose output for debugging
  zen-claw agent --verbose "debug this issue"
  
  # Switch models during session:
  # Type "/models" to see available models
  # Type "/model qwen/qwen3-coder-30b-a3b-instruct" to switch to Qwen
  
  # Continue session from another client (future):
  # Use same session ID in Slack/Telegram bot`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			task := ""
			if len(args) > 0 {
				task = args[0]
			}
			runAgent(task, model, provider, workingDir, sessionID, showProgress, maxSteps, verbose, think)
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "AI model (e.g., deepseek-chat)")
	cmd.Flags().StringVar(&provider, "provider", "", "AI provider (deepseek, openai, glm, minimax, qwen)")
	cmd.Flags().StringVar(&workingDir, "working-dir", ".", "Working directory for tools")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (for continuing sessions)")
	cmd.Flags().BoolVar(&showProgress, "progress", false, "Show progress in console (CLI only)")
	cmd.Flags().IntVar(&maxSteps, "max-steps", 50, "Maximum tool execution steps")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose output for debugging")
	cmd.Flags().BoolVar(&think, "think", false, "Show AI thinking process with cursor animation")

	return cmd
}

// createOrLoadSession creates a new session or loads an existing one
func createOrLoadSession(sessionID, workingDir, provider, model string) *Session {
	// In a real implementation, this would interact with the session store
	// For now, we just return a basic session object
	return &Session{
		ID:         sessionID,
		WorkingDir: workingDir,
		Provider:   provider,
		Model:      model,
	}
}

func runAgent(task, modelFlag, providerFlag, workingDir, sessionID string, showProgress bool, maxSteps int, verbose bool, think bool) {
	// Interactive mode if no task provided
	if task == "" {
		runInteractiveMode(modelFlag, providerFlag, workingDir, sessionID, showProgress, maxSteps, verbose, think)
		return
	}

	if verbose {
		fmt.Println("🔧 Verbose mode enabled")
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

	// Determine provider and model - in runAgent function
	providerName := providerFlag
	modelName := modelFlag

	// If model is specified but provider isn't, try to infer provider from model
	if modelName != "" && providerName == "" {
		providerName = inferProviderFromModel(modelName)
	}

	// If provider still not determined, use default
	if providerName == "" {
		providerName = "deepseek" // Default
	}

	// If model not specified, use default for provider
	if modelName == "" {
		// Default models per provider
		switch providerName {
		case "deepseek":
			modelName = "deepseek-chat"
		case "qwen":
			modelName = "qwen3-coder-30b-a3b-instruct"
		case "glm":
			modelName = "glm-4.7"
		case "minimax":
			modelName = "minimax-M2.1"
		case "openai":
			modelName = "gpt-4o-mini"
		default:
			modelName = "deepseek-chat"
		}
	}

	fmt.Printf("Provider: %s, Model: %s\n", providerName, modelName)
	if showProgress {
		fmt.Println()
	}

	// Prepare request
	req := ChatRequest{
		SessionID:  sessionID,
		UserInput:  task,
		WorkingDir: workingDir,
		Provider:   providerName,
		Model:      modelName,
		MaxSteps:   maxSteps,
	}

	if showProgress {
		fmt.Println("🤖 Sending request to gateway...")
		fmt.Println()
		fmt.Printf("📡 Gateway: http://localhost:8080\n")
		if sessionID != "" {
			fmt.Printf("   Session ID: %s\n", sessionID)
		}
		fmt.Printf("   Task: %s\n", task)
		fmt.Println()
	}

	// Show thinking cursor if enabled
	if think {
		go showThinkingCursor(task)
	}

	// Send request to gateway
	resp, err := client.Send(req)
	if err != nil {
		fmt.Printf("\n❌ Gateway request failed: %v\n", err)
		os.Exit(1)
	}

	// Stop thinking cursor if it was running
	if think {
		stopThinkingCursor()
	}

	// Check for error in response
	if resp.Error != "" {
		fmt.Printf("\n❌ Agent execution failed: %s\n", resp.Error)
		os.Exit(1)
	}

	// Print result
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("🎯 RESULT (via Gateway)")
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
		fmt.Printf("   zen-claw agent --session-id %s \"your next task\"\n", resp.SessionID)
	}
}

// runInteractiveMode runs the agent in interactive mode
func runInteractiveMode(modelFlag, providerFlag, workingDir, sessionID string, showProgress bool, maxSteps int, verbose bool, think bool) {
	fmt.Println("🚀 Zen Agent - Interactive Mode")
	fmt.Println("═" + strings.Repeat("═", 78))
	fmt.Println("Entering interactive mode. Type your tasks, one per line.")
	fmt.Println("Special commands:")
	fmt.Println("  /providers          - List available AI providers")
	fmt.Println("  /provider <name>    - Switch to a specific provider")
	fmt.Println("  /models            - List models for current provider")
	fmt.Println("  /model <name>      - Switch model within current provider")
	fmt.Println("  /context-limit [n] - Set context limit (default 50, 0=unlimited)")
	fmt.Println("  /qwen-large-context [on|off|disable] - Enable/disable Qwen 256k context (default off)")
	fmt.Println("  /exit, /quit       - Exit interactive mode")
	fmt.Println("  /help              - Show this help")
	fmt.Println("═" + strings.Repeat("═", 78))

	if sessionID != "" {
		fmt.Printf("Session ID: %s\n", sessionID)
	}
	fmt.Printf("Working directory: %s\n", workingDir)

	// Create gateway client
	client := NewGatewayClient("http://localhost:8080")

	// Check if gateway is running
	if err := client.HealthCheck(); err != nil {
		fmt.Printf("\n❌ Gateway not available: %v\n", err)
		fmt.Println("   Start the gateway first: zen-claw gateway start")
		return
	}

	// Determine provider and model - in runInteractiveMode function
	providerName := providerFlag
	modelName := modelFlag

	// If model is specified but provider isn't, try to infer provider from model
	if modelName != "" && providerName == "" {
		providerName = inferProviderFromModel(modelName)
	}

	// If provider still not determined, use default
	if providerName == "" {
		providerName = "deepseek" // Default
	}

	// If model not specified, use default for provider
	if modelName == "" {
		// Default models per provider
		switch providerName {
		case "deepseek":
			modelName = "deepseek-chat"
		case "qwen":
			modelName = "qwen3-coder-30b-a3b-instruct"
		case "glm":
			modelName = "glm-4.7"
		case "minimax":
			modelName = "minimax-M2.1"
		case "openai":
			modelName = "gpt-4o-mini"
		default:
			modelName = "deepseek-chat"
		}
	}

	fmt.Printf("Provider: %s, Model: %s\n", providerName, modelName)
	fmt.Println("═" + strings.Repeat("═", 78))

	// Setup readline for improved interactive mode (history, editing, etc.)
	historyFile := filepath.Join(os.Getenv("HOME"), ".zen-claw-history")
	rl, err := readline.NewEx(&readline.Config{
		Prompt:            " > ",
		HistoryFile:       historyFile,
		HistoryLimit:      1000,
		AutoComplete:      nil, // Can add custom completer later
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		// Fallback to basic input if readline fails
		fmt.Printf("Warning: readline not available, using basic input: %v\n", err)
		runBasicInteractiveMode(client, sessionID, workingDir, providerName, modelName, maxSteps)
		return
	}
	defer rl.Close()

	// Interactive loop with readline
	for {
		input, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				fmt.Println("\nInterrupted. Use /exit to quit.")
				continue
			}
			// EOF or other error - exit
			fmt.Println("\nExiting...")
			return
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle special commands
		switch {
		case input == "/exit" || input == "/quit":
			fmt.Println("Exiting interactive mode...")
			return
		case input == "/help":
			fmt.Println("Special commands:")
			fmt.Println("  /providers          - List available AI providers")
			fmt.Println("  /provider <name>    - Switch to a specific provider")
			fmt.Println("  /models            - List models for current provider")
			fmt.Println("  /model <name>      - Switch model within current provider")
			fmt.Println("  /context-limit [n] - Set context limit (default 50, 0=unlimited)")
			fmt.Println("  /qwen-large-context [on|off|disable] - Enable/disable Qwen 256k context (default off)")
			fmt.Println("  /exit, /quit       - Exit interactive mode")
			fmt.Println("  /help              - Show this help")
			continue
		case input == "/providers" || input == "/provider":
			fmt.Println("Available providers:")
			fmt.Println("  - deepseek  (default)")
			fmt.Println("  - qwen")
			fmt.Println("  - glm")
			fmt.Println("  - minimax")
			fmt.Println("  - openai")
			fmt.Println("\nUse '/provider <provider-name>' to switch providers")
			fmt.Println("Each provider has its own models. Use '/models' to see models for current provider.")
			continue
		case strings.HasPrefix(input, "/provider ") || strings.HasPrefix(input, "/providers "):
			// Extract provider name, handling both prefixes
			var newProvider string
			if strings.HasPrefix(input, "/provider ") {
				newProvider = strings.TrimSpace(strings.TrimPrefix(input, "/provider "))
			} else {
				newProvider = strings.TrimSpace(strings.TrimPrefix(input, "/providers "))
			}

			// Validate provider
			validProviders := []string{"deepseek", "qwen", "glm", "minimax", "openai"}
			isValid := false
			for _, p := range validProviders {
				if p == newProvider {
					isValid = true
					break
				}
			}

			if !isValid {
				fmt.Printf("Unknown provider: %s. Valid providers: %v\n", newProvider, validProviders)
				continue
			}

			providerName = newProvider

			// Reset to default model for this provider
			switch providerName {
			case "deepseek":
				modelName = "deepseek-chat"
			case "qwen":
				modelName = "qwen3-coder-30b-a3b-instruct"
			case "glm":
				modelName = "glm-4.7"
			case "minimax":
				modelName = "minimax-M2.1"
			case "openai":
				modelName = "gpt-4o-mini"
			}

			fmt.Printf("Switched to provider: %s (model: %s)\n", providerName, modelName)
			continue
		case input == "/models":
			// Show models for current provider
			fmt.Printf("Models for provider '%s':\n", providerName)
			switch providerName {
			case "deepseek":
				fmt.Println("  - deepseek-chat (default)")
				fmt.Println("  - deepseek-reasoner")
			case "qwen":
				fmt.Println("  - qwen3-coder-30b-a3b-instruct (default)")
				fmt.Println("  - qwen-plus")
				fmt.Println("  - qwen-max")
				fmt.Println("  - qwen3-235b-a22b-instruct-2507")
				fmt.Println("  - qwen3-coder-480b-a35b-instruct")
			case "glm":
				fmt.Println("  - glm-4.7 (default)")
				fmt.Println("  - glm-4")
				fmt.Println("  - glm-3-turbo")
			case "minimax":
				fmt.Println("  - minimax-M2.1 (default)")
				fmt.Println("  - abab6.5s")
				fmt.Println("  - abab6.5")
			case "openai":
				fmt.Println("  - gpt-4o-mini (default)")
				fmt.Println("  - gpt-4o")
				fmt.Println("  - gpt-4-turbo")
				fmt.Println("  - gpt-3.5-turbo")
			default:
				fmt.Println("  (Unknown provider)")
			}
			fmt.Println("\nUse '/model <model-name>' to switch models within current provider")
			continue
		case strings.HasPrefix(input, "/model "):
			newModel := strings.TrimSpace(strings.TrimPrefix(input, "/model "))
			modelName = newModel

			// Verify model is compatible with current provider
			if !isModelCompatibleWithProvider(newModel, providerName) {
				fmt.Printf("Warning: Model '%s' may not be compatible with provider '%s'\n", newModel, providerName)
				fmt.Printf("Consider switching provider first with '/provider <provider-name>'\n")
			}

			fmt.Printf("Model switched to: %s (provider: %s)\n", modelName, providerName)
			continue
		case strings.HasPrefix(input, "/context-limit"):
			// This will be handled by the agent, but we can show usage here
			parts := strings.Fields(input)
			if len(parts) == 1 {
				fmt.Println("Usage: /context-limit [number]")
				fmt.Println("  Set context limit (number of messages to send)")
				fmt.Println("  Use 0 for unlimited, default is 50")
				fmt.Println("  Example: /context-limit 100")
			} else {
				// Forward to agent - it will handle it
				req := ChatRequest{
					SessionID:  sessionID,
					UserInput:  input,
					WorkingDir: workingDir,
					Provider:   providerName,
					Model:      modelName,
					MaxSteps:   maxSteps,
				}
				resp, err := client.Send(req)
				if err != nil {
					fmt.Printf("❌ Error: %v\n", err)
				} else if resp.Error != "" {
					fmt.Printf("❌ Error: %s\n", resp.Error)
				} else {
					fmt.Println(resp.Result)
				}
			}
			continue
		case strings.HasPrefix(input, "/qwen-large-context"):
			// Forward to agent - it will handle it
			req := ChatRequest{
				SessionID:  sessionID,
				UserInput:  input,
				WorkingDir: workingDir,
				Provider:   providerName,
				Model:      modelName,
				MaxSteps:   maxSteps,
			}
			resp, err := client.Send(req)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			} else if resp.Error != "" {
				fmt.Printf("❌ Error: %s\n", resp.Error)
			} else {
				fmt.Println(resp.Result)
			}
			continue
		}

		// Process task
		req := ChatRequest{
			SessionID:  sessionID,
			UserInput:  input,
			WorkingDir: workingDir,
			Provider:   providerName,
			Model:      modelName,
			MaxSteps:   maxSteps,
		}

		// Send request to gateway
		resp, err := client.Send(req)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		// Check for error in response
		if resp.Error != "" {
			fmt.Printf("❌ Agent error: %s\n", resp.Error)
			continue
		}

		// Print result
		fmt.Println("\n" + strings.Repeat("═", 80))
		fmt.Println("🎯 RESULT")
		fmt.Println(strings.Repeat("═", 80))
		fmt.Println(resp.Result)
		fmt.Println(strings.Repeat("═", 80))

		// Update session ID for continuation
		sessionID = resp.SessionID
	}
}

// runBasicInteractiveMode is a fallback when readline is not available
func runBasicInteractiveMode(client *GatewayClient, sessionID, workingDir, providerName, modelName string, maxSteps int) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\n> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nExiting...")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "/exit" || input == "/quit" {
			fmt.Println("Exiting interactive mode...")
			return
		}

		// Process task
		req := ChatRequest{
			SessionID:  sessionID,
			UserInput:  input,
			WorkingDir: workingDir,
			Provider:   providerName,
			Model:      modelName,
			MaxSteps:   maxSteps,
		}

		resp, err := client.Send(req)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		if resp.Error != "" {
			fmt.Printf("❌ Agent error: %s\n", resp.Error)
			continue
		}

		fmt.Println("\n" + strings.Repeat("═", 80))
		fmt.Println("🎯 RESULT")
		fmt.Println(strings.Repeat("═", 80))
		fmt.Println(resp.Result)
		fmt.Println(strings.Repeat("═", 80))

		sessionID = resp.SessionID
	}
}

// inferProviderFromModel tries to infer provider from model name
func inferProviderFromModel(modelName string) string {
	modelName = strings.ToLower(modelName)

	// Check for provider patterns in model name
	if strings.Contains(modelName, "qwen") {
		return "qwen"
	} else if strings.Contains(modelName, "deepseek") {
		return "deepseek"
	} else if strings.Contains(modelName, "glm") {
		return "glm"
	} else if strings.Contains(modelName, "minimax") || strings.Contains(modelName, "abab") {
		return "minimax"
	} else if strings.Contains(modelName, "gpt") {
		return "openai"
	}

	// Could not infer provider
	return ""
}

// isModelCompatibleWithProvider checks if a model is likely compatible with a provider
func isModelCompatibleWithProvider(modelName, provider string) bool {
	modelName = strings.ToLower(modelName)
	provider = strings.ToLower(provider)

	switch provider {
	case "qwen":
		return strings.Contains(modelName, "qwen")
	case "deepseek":
		return strings.Contains(modelName, "deepseek")
	case "glm":
		return strings.Contains(modelName, "glm")
	case "minimax":
		return strings.Contains(modelName, "minimax") || strings.Contains(modelName, "abab")
	case "openai":
		return strings.Contains(modelName, "gpt")
	}

	// Unknown provider, assume compatible
	return true
}

// showThinkingCursor displays an animated cursor while processing
func showThinkingCursor(task string) {
	thinkingCursorActive = true
	thinkingCursorStop = make(chan bool, 1)

	cursor := []string{"|", "/", "-", "\\"}
	cursorIndex := 0

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	fmt.Printf("\n🧠 Thinking: %s ", task)

	for {
		select {
		case <-ticker.C:
			if !thinkingCursorActive {
				return
			}
			// Move to next cursor character
			cursorIndex = (cursorIndex + 1) % len(cursor)
			// Clear the line and print new cursor
			fmt.Printf("\r🧠 Thinking: %s %s", task, cursor[cursorIndex])
		case <-thinkingCursorStop:
			fmt.Printf("\r🧠 Thinking: %s ✅\n", task)
			return
		}
	}
}

// stopThinkingCursor stops the thinking cursor
func stopThinkingCursor() {
	if thinkingCursorActive {
		thinkingCursorActive = false
		thinkingCursorStop <- true
	}
}

// Enhanced tool error handling
func handleToolError(toolName string, err error) error {
    errorMsg := fmt.Sprintf("Tool '%s' failed: %v", toolName, err)
    if strings.Contains(err.Error(), "permission denied") {
        errorMsg += "\n💡 Try running with appropriate permissions"
    } else if strings.Contains(err.Error(), "no such file") {
        errorMsg += "\n💡 Check that the file/path exists"
    }
    return errors.New(errorMsg)
}

// Tool input validation
func validateToolArgs(toolName string, args []string) error {
    switch toolName {
    case "read":
        if len(args) != 1 {
            return fmt.Errorf("read requires exactly one argument (file path)")
        }
    case "write":
        if len(args) < 2 {
            return fmt.Errorf("write requires at least two arguments (file path and content)")
        }
    case "exec":
        if len(args) == 0 {
            return fmt.Errorf("exec requires at least one command argument")
        }
    }
    return nil
}

// Enhanced tool help display
func showToolHelp() {
    fmt.Println("Available tools:")
    fmt.Println("  read     - Read file contents")
    fmt.Println("  write    - Create/overwrite files")
    fmt.Println("  edit     - Edit files precisely")
    fmt.Println("  exec     - Run shell commands")
    fmt.Println("  search   - Find files by name/content")
    fmt.Println("  git      - Git operations")
    fmt.Println("  env      - Environment variables")
    fmt.Println("  tools    - List all tools")
    fmt.Println("  session  - Session management")
    fmt.Println("\nUse 'toolname --help' for detailed usage")
}
