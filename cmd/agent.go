package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
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

Multi-AI modes (separate commands):
  zen-claw consensus   # 3 AIs → arbiter → better blueprints
  zen-claw factory     # Coordinator + specialist AIs`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			task := ""
			if len(args) > 0 {
				task = args[0]
			}
			runAgent(task, model, provider, workingDir, sessionID, showProgress, maxSteps, verbose)
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "AI model (e.g., deepseek-chat)")
	cmd.Flags().StringVar(&provider, "provider", "", "AI provider (deepseek, openai, glm, minimax, qwen, kimi)")
	cmd.Flags().StringVar(&workingDir, "working-dir", ".", "Working directory for tools")
	cmd.Flags().StringVar(&sessionID, "session", "", "Named session to save/resume (omit for fresh context)")
	cmd.Flags().BoolVar(&showProgress, "progress", false, "Show progress in console (CLI only)")
	cmd.Flags().IntVar(&maxSteps, "max-steps", 100, "Maximum tool execution steps (default 100 for complex tasks)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose output for debugging")

	return cmd
}

func runAgent(task, modelFlag, providerFlag, workingDir, sessionID string, showProgress bool, maxSteps int, verbose bool) {
	// Interactive mode if no task provided
	if task == "" {
		runInteractiveMode(modelFlag, providerFlag, workingDir, sessionID, showProgress, maxSteps, verbose)
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
		case "kimi":
			modelName = "kimi-k2-5"
		default:
			modelName = "deepseek-chat"
		}
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

	// Use streaming for better UX - shows progress in real-time
	resp, err := client.SendWithProgress(req, func(event ProgressEvent) {
		displayProgressEvent(event)
	})
	if err != nil {
		fmt.Printf("\n❌ Gateway request failed: %v\n", err)
		os.Exit(1)
	}

	// Check for error in response
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

// runInteractiveMode runs the agent in interactive mode
func runInteractiveMode(modelFlag, providerFlag, workingDir, sessionID string, showProgress bool, maxSteps int, verbose bool) {
	fmt.Println("🚀 Zen Agent")
	fmt.Println("═" + strings.Repeat("═", 78))

	// Show session info
	if sessionID != "" {
		fmt.Printf("Session: %s (will be saved)\n", sessionID)
	} else {
		fmt.Println("Fresh context (use --session <name> to save)")
	}
	fmt.Printf("Working directory: %s\n", workingDir)
	fmt.Println()
	fmt.Println("Commands: /models, /model <name>, /provider <name>, /clear, /help, /exit")
	fmt.Println("═" + strings.Repeat("═", 78))

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
		case "kimi":
			modelName = "kimi-k2-5"
		default:
			modelName = "deepseek-chat"
		}
	}

	fmt.Printf("Provider: %s, Model: %s\n", providerName, modelName)
	fmt.Println("═" + strings.Repeat("═", 78))

	// Setup readline for improved interactive mode (history, editing, etc.)
	historyFile := filepath.Join(os.Getenv("HOME"), ".zen-claw-history")
	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "\n> ",
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
			fmt.Println("\nCommands:")
			fmt.Println("  /clear              - Clear conversation history (fresh start)")
			fmt.Println("  /models             - List available models")
			fmt.Println("  /model <name>       - Switch to a different model")
			fmt.Println("  /provider <name>    - Switch provider (deepseek, qwen, minimax, kimi)")
			fmt.Println("  /context-limit [n]  - Set context limit (0=unlimited)")
			fmt.Println("  /exit               - Exit")
			fmt.Println()
			fmt.Println("Multi-AI modes (separate commands):")
			fmt.Println("  zen-claw consensus  - 3 AIs → arbiter → better blueprints")
			fmt.Println("  zen-claw factory    - Coordinator + specialist AIs")
			continue

		case input == "/clear":
			// Clear conversation - start fresh (like Cursor Cmd+N)
			sessionID = "" // Force new session
			fmt.Println("✓ Cleared. Fresh context.")
			continue

		// Simplified session commands - only show saved (named) sessions
		case input == "/sessions":
			sessions, err := client.ListSessions()
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				continue
			}
			// Filter to only show named sessions (not auto-generated session_* ones)
			var namedSessions []SessionEntry
			for _, s := range sessions.Sessions {
				if !strings.HasPrefix(s.ID, "session_") {
					namedSessions = append(namedSessions, s)
				}
			}
			fmt.Println("\n📋 Saved Sessions:")
			fmt.Println(strings.Repeat("─", 50))
			if len(namedSessions) == 0 {
				fmt.Println("  No saved sessions")
				fmt.Println("  Use --session <name> to save a session")
			}
			for _, s := range namedSessions {
				current := ""
				if s.ID == sessionID {
					current = " ← current"
				}
				fmt.Printf("  • %s (%d messages)%s\n", s.ID, s.MessageCount, current)
			}
			fmt.Println(strings.Repeat("─", 50))
			continue

		case strings.HasPrefix(input, "/load "):
			// Load a saved session
			name := strings.TrimSpace(strings.TrimPrefix(input, "/load "))
			if name == "" || strings.HasPrefix(name, "session_") {
				fmt.Println("Usage: /load <session-name>")
				continue
			}
			sessionID = name
			fmt.Printf("✓ Loaded session: %s\n", sessionID)
			continue

		// Preferences commands
		case input == "/prefs" || input == "/preferences":
			prefs, err := client.GetPreferences("all")
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				continue
			}
			fmt.Println("\n⚙️ AI Preferences:")
			fmt.Println(strings.Repeat("─", 60))
			if def, ok := prefs["default"].(map[string]interface{}); ok {
				fmt.Printf("Default: %v/%v\n", def["provider"], def["model"])
			}
			if fo, ok := prefs["fallback_order"].([]interface{}); ok {
				fmt.Printf("Fallback order: %v\n", fo)
			}
			if cons, ok := prefs["consensus"].(map[string]interface{}); ok {
				if arb, ok := cons["arbiter"].([]interface{}); ok {
					fmt.Printf("Consensus arbiter: %v\n", arb)
				}
			}
			fmt.Println(strings.Repeat("─", 60))
			fmt.Println("Use /prefs fallback, /prefs consensus, /prefs factory for details")
			continue

		case strings.HasPrefix(input, "/prefs fallback"):
			parts := strings.Fields(input)
			if len(parts) == 2 {
				// Just show current fallback order
				prefs, err := client.GetPreferences("fallback")
				if err != nil {
					fmt.Printf("❌ Error: %v\n", err)
					continue
				}
				fmt.Printf("Fallback order: %v\n", prefs["fallback_order"])
				fmt.Println("To change: /prefs fallback deepseek,kimi,qwen,glm,minimax,openai")
			} else {
				// Set new fallback order
				orderStr := strings.TrimPrefix(input, "/prefs fallback ")
				order := strings.Split(orderStr, ",")
				for i := range order {
					order[i] = strings.TrimSpace(order[i])
				}
				updates := map[string]interface{}{"fallback_order": order}
				if err := client.UpdatePreferences(updates); err != nil {
					fmt.Printf("❌ Error: %v\n", err)
				} else {
					fmt.Printf("✓ Fallback order updated: %v\n", order)
				}
			}
			continue

		case strings.HasPrefix(input, "/prefs consensus"):
			prefs, err := client.GetPreferences("consensus")
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				continue
			}
			fmt.Println("\n🤝 Consensus Settings:")
			fmt.Println(strings.Repeat("─", 60))
			if arb, ok := prefs["arbiter"].([]interface{}); ok {
				fmt.Printf("Arbiter preference: %v\n", arb)
			}
			if workers, ok := prefs["workers"].([]interface{}); ok {
				fmt.Println("Workers:")
				for _, w := range workers {
					if wm, ok := w.(map[string]interface{}); ok {
						fmt.Printf("  - %v/%v (%v)\n", wm["Provider"], wm["Model"], wm["Role"])
					}
				}
			}
			fmt.Println(strings.Repeat("─", 60))
			fmt.Println("To change arbiter: /prefs arbiter kimi,qwen,deepseek")
			continue

		case strings.HasPrefix(input, "/prefs arbiter"):
			parts := strings.Fields(input)
			if len(parts) == 2 {
				prefs, _ := client.GetPreferences("consensus")
				fmt.Printf("Current arbiter order: %v\n", prefs["arbiter"])
			} else {
				orderStr := strings.TrimPrefix(input, "/prefs arbiter ")
				order := strings.Split(orderStr, ",")
				for i := range order {
					order[i] = strings.TrimSpace(order[i])
				}
				updates := map[string]interface{}{"arbiter": order}
				if err := client.UpdatePreferences(updates); err != nil {
					fmt.Printf("❌ Error: %v\n", err)
				} else {
					fmt.Printf("✓ Arbiter order updated: %v\n", order)
				}
			}
			continue

		case strings.HasPrefix(input, "/prefs factory"):
			prefs, err := client.GetPreferences("factory")
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				continue
			}
			fmt.Println("\n🏭 Factory Specialists:")
			fmt.Println(strings.Repeat("─", 60))
			if specs, ok := prefs["specialists"].(map[string]interface{}); ok {
				for domain, spec := range specs {
					if sm, ok := spec.(map[string]interface{}); ok {
						fmt.Printf("  %s: %v/%v\n", domain, sm["Provider"], sm["Model"])
					}
				}
			}
			fmt.Println(strings.Repeat("─", 60))
			continue

		case input == "/providers" || input == "/provider":
			fmt.Println("Available providers:")
			fmt.Println("  - deepseek  (default)")
			fmt.Println("  - qwen      (256K context)")
			fmt.Println("  - kimi      (256K context, $0.10/M)")
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
			validProviders := []string{"deepseek", "qwen", "glm", "minimax", "openai", "kimi"}
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
			case "kimi":
				modelName = "kimi-k2-5"
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
			case "kimi":
				fmt.Println("  - kimi-k2-5 (default, 256K context)")
				fmt.Println("  - kimi-k2-5-long-context (2M context)")
				fmt.Println("  - moonshot-v1-8k")
				fmt.Println("  - moonshot-v1-32k")
				fmt.Println("  - moonshot-v1-128k")
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

		// Send request to gateway with streaming progress
		resp, err := client.SendWithProgress(req, func(event ProgressEvent) {
			displayProgressEvent(event)
		})
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

		resp, err := client.SendWithProgress(req, func(event ProgressEvent) {
			displayProgressEvent(event)
		})
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
	} else if strings.Contains(modelName, "kimi") || strings.Contains(modelName, "moonshot") {
		return "kimi"
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
	case "kimi":
		return strings.Contains(modelName, "kimi") || strings.Contains(modelName, "moonshot")
	}

	// Unknown provider, assume compatible
	return true
}

// displayProgressEvent prints a progress event to the console with nice formatting
func displayProgressEvent(event ProgressEvent) {
	switch event.Type {
	case "start":
		fmt.Printf("🚀 %s\n", event.Message)
	case "step":
		fmt.Printf("\n📍 %s\n", event.Message)
	case "thinking":
		fmt.Printf("   💭 %s\n", event.Message)
	case "ai_response":
		// Truncate long AI responses in progress display
		msg := event.Message
		if len(msg) > 200 {
			msg = msg[:197] + "..."
		}
		fmt.Printf("   🤖 %s\n", msg)
	case "tool_call":
		fmt.Printf("   %s\n", event.Message)
	case "tool_result":
		fmt.Printf("   %s\n", event.Message)
	case "complete":
		fmt.Printf("\n✅ %s\n", event.Message)
	case "error":
		fmt.Printf("\n❌ %s\n", event.Message)
	case "done":
		// Final result will be displayed separately
	default:
		if event.Message != "" {
			fmt.Printf("   %s\n", event.Message)
		}
	}
}
