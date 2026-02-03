package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/neves/zen-claw/internal/cost"
	"github.com/neves/zen-claw/internal/providers"
)

// runInteractiveMode runs the agent in interactive mode
func runInteractiveMode(modelFlag, providerFlag, workingDir, sessionID string, showProgress bool, maxSteps int, verbose bool, useWebSocket bool, streamTokens bool) {
	// streamTokens is passed in requests below
	fmt.Println("🚀 Zen Agent")
	if useWebSocket {
		fmt.Println("(WebSocket mode)")
	}
	fmt.Println("═" + strings.Repeat("═", 78))

	// Show session info
	if sessionID != "" {
		fmt.Printf("Session: %s (will be saved)\n", sessionID)
	} else {
		fmt.Println("Fresh context (use --session <name> to save)")
	}
	fmt.Printf("Working directory: %s\n", workingDir)
	fmt.Println()
	fmt.Println("Commands: /help, /session list, /session load, /models, /provider, /exit")
	fmt.Println("═" + strings.Repeat("═", 78))

	// Thinking level for reasoning-capable models
	thinkingLevel := "" // off, low, medium, high (empty = model default)

	// Create gateway client
	client := NewGatewayClient("http://localhost:8080")

	// Check if gateway is running
	if err := client.HealthCheck(); err != nil {
		fmt.Printf("\n❌ Gateway not available: %v\n", err)
		fmt.Println("   Start the gateway first: zen-claw gateway start")
		return
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
	fmt.Println("═" + strings.Repeat("═", 78))

	// Setup readline for improved interactive mode
	historyFile := filepath.Join(os.Getenv("HOME"), ".zen-claw-history")
	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "> ",
		HistoryFile:       historyFile,
		HistoryLimit:      1000,
		AutoComplete:      nil,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
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
			printInteractiveHelp()
			continue

		case input == "/think" || strings.HasPrefix(input, "/think "):
			thinkingLevel = handleThinkCommand(input, thinkingLevel)
			continue

		case input == "/clear":
			sessionID = ""
			fmt.Println("✓ Cleared. Fresh context.")
			continue

		case input == "/stats":
			handleStatsCommand(client)
			continue

		case input == "/sessions" || input == "/sessions list" || input == "/session" || input == "/session list":
			handleSessionsListCommand(client, sessionID)
			continue

		case input == "/sessions info" || input == "/session info":
			displaySessionsInfo()
			continue

		case strings.HasPrefix(input, "/sessions clean") || strings.HasPrefix(input, "/session clean"):
			arg := strings.TrimPrefix(input, "/sessions clean")
			if arg == input {
				arg = strings.TrimPrefix(input, "/session clean")
			}
			cleanSessionsInteractive(arg)
			continue

		case strings.HasPrefix(input, "/sessions delete ") || strings.HasPrefix(input, "/session delete "):
			name := strings.TrimPrefix(input, "/sessions delete ")
			if name == input {
				name = strings.TrimPrefix(input, "/session delete ")
			}
			name = strings.TrimSpace(name)
			if name == "" {
				fmt.Println("Usage: /session delete <name>")
				continue
			}
			deleteSessionByName(name)
			continue

		case strings.HasPrefix(input, "/load ") || strings.HasPrefix(input, "/session load "):
			name := strings.TrimPrefix(input, "/load ")
			if name == input {
				name = strings.TrimPrefix(input, "/session load ")
			}
			name = strings.TrimSpace(name)
			if name == "" || strings.HasPrefix(name, "session_") {
				fmt.Println("Usage: /session load <name>")
				continue
			}
			sessionID = name
			fmt.Printf("✓ Switched to session: %s\n", sessionID)
			fmt.Println("  Next message will use this session's context")
			continue

		case input == "/prefs" || input == "/preferences":
			handlePrefsCommand(client)
			continue

		case strings.HasPrefix(input, "/prefs fallback"):
			handlePrefsFallbackCommand(client, input)
			continue

		case strings.HasPrefix(input, "/prefs consensus"):
			handlePrefsConsensusCommand(client)
			continue

		case strings.HasPrefix(input, "/prefs arbiter"):
			handlePrefsArbiterCommand(client, input)
			continue

		case strings.HasPrefix(input, "/prefs factory"):
			handlePrefsFactoryCommand(client)
			continue

		case input == "/providers" || input == "/provider":
			printProvidersList()
			continue

		case strings.HasPrefix(input, "/provider ") || strings.HasPrefix(input, "/providers "):
			providerName, modelName = handleProviderSwitchCommand(input, providerName)
			continue

		case input == "/models":
			printModelsForProvider(providerName)
			continue

		case strings.HasPrefix(input, "/model "):
			modelName = handleModelSwitchCommand(input, providerName)
			continue

		case strings.HasPrefix(input, "/context-limit"):
			handleContextLimitCommand(client, input, sessionID, workingDir, providerName, modelName, maxSteps)
			continue

		case strings.HasPrefix(input, "/qwen-large-context"):
			handleQwenLargeContextCommand(client, input, sessionID, workingDir, providerName, modelName, maxSteps)
			continue

		case input == "/cost" || strings.HasPrefix(input, "/cost "):
			handleCostCommand(input, providerName)
			continue

		case input == "/compare":
			handleCompareProvidersCommand()
			continue
		}

		// Process task
		req := ChatRequest{
			SessionID:     sessionID,
			UserInput:     input,
			WorkingDir:    workingDir,
			Provider:      providerName,
			Model:         modelName,
			MaxSteps:      maxSteps,
			ThinkingLevel: thinkingLevel,
			Stream:        streamTokens,
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

// Command handlers

func printInteractiveHelp() {
	fmt.Println("\nCommands:")
	fmt.Println("  /clear              - Clear conversation history (fresh start)")
	fmt.Println("  /stats              - Show usage and cache statistics")
	fmt.Println("  /cost [prompt]      - Estimate cost for a prompt")
	fmt.Println("  /compare            - Compare provider costs")
	fmt.Println("  /models             - List available models")
	fmt.Println("  /model <name>       - Switch to a different model")
	fmt.Println("  /provider <name>    - Switch provider (deepseek, qwen, minimax, kimi)")
	fmt.Println("  /think [level]      - Set thinking level (off, low, medium, high)")
	fmt.Println("  /context-limit [n]  - Set context limit (0=unlimited)")
	fmt.Println()
	fmt.Println("Session management:")
	fmt.Println("  /session list       - List saved sessions")
	fmt.Println("  /session load <n>   - Switch to a saved session")
	fmt.Println("  /session info       - Show storage info (path, size)")
	fmt.Println("  /session clean      - Clean sessions (--all or --older 7d)")
	fmt.Println("  /session delete <n> - Delete a specific session")
	fmt.Println()
	fmt.Println("  /exit               - Exit")
	fmt.Println()
	fmt.Println("Multi-AI modes (separate commands):")
	fmt.Println("  zen-claw consensus  - 3 AIs → arbiter → better blueprints")
	fmt.Println("  zen-claw factory    - Coordinator + specialist AIs")
}

func handleThinkCommand(input, currentLevel string) string {
	if input == "/think" {
		if currentLevel == "" {
			fmt.Println("Thinking: default (model decides)")
		} else {
			fmt.Printf("Thinking: %s\n", currentLevel)
		}
		fmt.Println("Usage: /think [off|low|medium|high]")
		return currentLevel
	}
	level := strings.TrimSpace(strings.TrimPrefix(input, "/think "))
	switch level {
	case "off", "low", "medium", "high":
		if level == "off" {
			fmt.Println("✓ Thinking disabled")
		} else {
			fmt.Printf("✓ Thinking level: %s\n", level)
		}
		return level
	default:
		fmt.Println("Invalid level. Use: off, low, medium, high")
		return currentLevel
	}
}

func handleStatsCommand(client *GatewayClient) {
	stats, err := client.GetStats()
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	fmt.Println("\n📊 Statistics:")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Usage: %s\n", stats.Usage)
	fmt.Printf("Cache: %d hits, %d misses (%.1f%% hit rate)\n",
		stats.CacheHits, stats.CacheMisses, stats.CacheHitRate*100)
	fmt.Printf("Cache size: %d entries\n", stats.CacheSize)
	fmt.Println(strings.Repeat("─", 50))
}

func handleSessionsListCommand(client *GatewayClient, currentSessionID string) {
	sessions, err := client.ListSessions()
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
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
		if s.ID == currentSessionID {
			current = " ← current"
		}
		fmt.Printf("  • %s (%d messages)%s\n", s.ID, s.MessageCount, current)
	}
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("Commands: /sessions info, /sessions clean [--all|--older 7d]")
}

func handlePrefsCommand(client *GatewayClient) {
	prefs, err := client.GetPreferences("all")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
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
}

func handlePrefsFallbackCommand(client *GatewayClient, input string) {
	parts := strings.Fields(input)
	if len(parts) == 2 {
		prefs, err := client.GetPreferences("fallback")
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}
		fmt.Printf("Fallback order: %v\n", prefs["fallback_order"])
		fmt.Println("To change: /prefs fallback deepseek,kimi,qwen,glm,minimax,openai")
	} else {
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
}

func handlePrefsConsensusCommand(client *GatewayClient) {
	prefs, err := client.GetPreferences("consensus")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
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
}

func handlePrefsArbiterCommand(client *GatewayClient, input string) {
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
}

func handlePrefsFactoryCommand(client *GatewayClient) {
	prefs, err := client.GetPreferences("factory")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
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
}

func printProvidersList() {
	fmt.Println("Available providers:")
	fmt.Println("  - deepseek  (default)")
	fmt.Println("  - qwen      (256K context)")
	fmt.Println("  - kimi      (256K context, $0.10/M)")
	fmt.Println("  - glm")
	fmt.Println("  - minimax")
	fmt.Println("  - openai")
	fmt.Println("\nUse '/provider <provider-name>' to switch providers")
	fmt.Println("Each provider has its own models. Use '/models' to see models for current provider.")
}

func handleProviderSwitchCommand(input, currentProvider string) (string, string) {
	var newProvider string
	if strings.HasPrefix(input, "/provider ") {
		newProvider = strings.TrimSpace(strings.TrimPrefix(input, "/provider "))
	} else {
		newProvider = strings.TrimSpace(strings.TrimPrefix(input, "/providers "))
	}

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
		return currentProvider, providers.GetDefaultModel(currentProvider)
	}

	modelName := providers.GetDefaultModel(newProvider)
	fmt.Printf("Switched to provider: %s (model: %s)\n", newProvider, modelName)
	return newProvider, modelName
}

func printModelsForProvider(providerName string) {
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
}

func handleModelSwitchCommand(input, providerName string) string {
	newModel := strings.TrimSpace(strings.TrimPrefix(input, "/model "))
	if !isModelCompatibleWithProvider(newModel, providerName) {
		fmt.Printf("Warning: Model '%s' may not be compatible with provider '%s'\n", newModel, providerName)
		fmt.Printf("Consider switching provider first with '/provider <provider-name>'\n")
	}
	fmt.Printf("Model switched to: %s (provider: %s)\n", newModel, providerName)
	return newModel
}

func handleContextLimitCommand(client *GatewayClient, input, sessionID, workingDir, providerName, modelName string, maxSteps int) {
	parts := strings.Fields(input)
	if len(parts) == 1 {
		fmt.Println("Usage: /context-limit [number]")
		fmt.Println("  Set context limit (number of messages to send)")
		fmt.Println("  Use 0 for unlimited, default is 50")
		fmt.Println("  Example: /context-limit 100")
	} else {
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
}

func handleQwenLargeContextCommand(client *GatewayClient, input, sessionID, workingDir, providerName, modelName string, maxSteps int) {
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

func handleCostCommand(input, provider string) {
	estimator := cost.NewEstimator()

	// If just /cost, show pricing info
	if input == "/cost" {
		fmt.Println("\n💰 Provider Pricing (per 1M tokens)")
		fmt.Println(strings.Repeat("─", 55))
		fmt.Printf("%-12s %10s %10s %10s\n", "Provider", "Input", "Output", "Cached")
		fmt.Println(strings.Repeat("─", 55))

		providers := []string{"deepseek", "kimi", "qwen", "glm", "minimax", "anthropic", "openai"}
		for _, p := range providers {
			if pricing, ok := cost.ProviderPrices[p]; ok {
				fmt.Printf("%-12s $%8.2f $%8.2f $%8.2f\n",
					p, pricing.InputPerMillion, pricing.OutputPerMillion, pricing.CachedPerMillion)
			}
		}
		fmt.Println(strings.Repeat("─", 55))
		fmt.Println("Usage: /cost <prompt> - estimate cost for a specific prompt")
		return
	}

	// Estimate for specific prompt
	prompt := strings.TrimSpace(strings.TrimPrefix(input, "/cost "))
	if prompt == "" {
		fmt.Println("Usage: /cost <prompt>")
		return
	}

	// Default system prompt estimate
	systemPrompt := "You are a helpful AI assistant with access to code tools."

	estimate := estimator.EstimateTask(provider, "", systemPrompt, prompt, true)
	fmt.Println()
	fmt.Println(estimate.Format())
}

func handleCompareProvidersCommand() {
	estimator := cost.NewEstimator()

	// Compare for a typical coding task
	inputTokens := 5000  // System + context
	outputTokens := 2000 // Code response

	fmt.Println()
	fmt.Println(estimator.CompareProviders(inputTokens, outputTokens))
}
