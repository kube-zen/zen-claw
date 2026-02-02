// runInteractiveMode runs the agent in interactive mode
func runInteractiveMode(modelFlag, providerFlag, workingDir, sessionID string, showProgress bool, maxSteps int, verbose bool) {
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

	// Simple interactive loop
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
