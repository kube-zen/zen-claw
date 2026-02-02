package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/config"
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
  
  # Start with session ID (save for continuing)
  zen-claw agent --session-id my-task "check codebase"
  
  # Show progress in console
  zen-claw agent --progress "list directory"
  
  # Run with verbose output for debugging
  zen-claw agent --verbose "debug this issue"
  
  # Continue session from another client (future):
  # Use same session ID in Slack/Telegram bot`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runAgent(args[0], model, provider, workingDir, sessionID, showProgress, maxSteps, verbose)
		},
	}
	
	cmd.Flags().StringVar(&model, "model", "", "AI model (e.g., deepseek/deepseek-chat)")
	cmd.Flags().StringVar(&provider, "provider", "", "AI provider (deepseek, openai, glm, minimax, qwen)")
	cmd.Flags().StringVar(&workingDir, "working-dir", ".", "Working directory for tools")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (for continuing sessions)")
	cmd.Flags().BoolVar(&showProgress, "progress", false, "Show progress in console (CLI only)")
	cmd.Flags().IntVar(&maxSteps, "max-steps", 10, "Maximum tool execution steps")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose output for debugging")
	
	return cmd
}

func runAgent(task, modelFlag, providerFlag, workingDir, sessionID string, showProgress bool, maxSteps int, verbose bool) {
	if verbose {
		fmt.Println("🔧 Verbose mode enabled")
	}
	
	if showProgress {
		fmt.Println("🚀 Zen Agent - Lightweight (with console progress)")
	} else {
		fmt.Println("🚀 Zen Agent - Lightweight")
	}
	fmt.Println("═" + strings.Repeat("═", 78))
	fmt.Printf("Task: %s\n", task)
	if sessionID != "" {
		fmt.Printf("Session ID: %s (save for continuing in Slack/Telegram)\n", sessionID)
	}
	fmt.Printf("Working directory: %s\n", workingDir)
	
	// Load config
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	if verbose {
		fmt.Printf("Loaded config from: %s\n", config.DefaultConfigPath())
	}
	
	// Determine provider and model
	providerName := providerFlag
	if providerName == "" {
		providerName = cfg.Default.Provider
	}
	
	modelName := modelFlag
	if modelName == "" {
		modelName = cfg.GetModel(providerName)
	}
	
	fmt.Printf("Provider: %s, Model: %s\n", providerName, modelName)
	if showProgress {
		fmt.Println()
	}
	
	// Create AI provider
	factory := providers.NewFactory(cfg)
	aiProvider, err := factory.CreateProvider(providerName)
	if err != nil {
		log.Fatalf("Failed to create provider %s: %v", providerName, err)
	}
	
	if verbose {
		fmt.Printf("Using AI provider: %s\n", aiProvider.Name())
	}
	
	// Create tools
	tools := []agent.Tool{
		agent.NewExecTool(workingDir),
		agent.NewReadFileTool(workingDir),
		agent.NewListDirTool(workingDir),
		agent.NewSystemInfoTool(),
	}
	
	// Create or load session
	session := agent.NewSession(sessionID)
	session.SetWorkingDir(workingDir)
	
	// Create lightweight agent (stateless, session passed as parameter)
	lightAgent := agent.NewLightAgent(aiProvider, tools, maxSteps)
	
	// Run agent
	ctx := context.Background()
	if showProgress {
		fmt.Println("🤖 Starting agent execution...")
		fmt.Println()
		fmt.Printf("📡 Session: %s\n", session.ID)
		fmt.Printf("   Task: %s\n", task)
		fmt.Println()
	}
	
	startTime := time.Now()
	updatedSession, result, err := lightAgent.Run(ctx, session, task)
	duration := time.Since(startTime)
	
	if err != nil {
		fmt.Printf("\n❌ Agent execution failed: %v\n", err)
		if verbose {
			fmt.Printf("Detailed error information:\n")
			fmt.Printf("  - Task: %s\n", task)
			fmt.Printf("  - Provider: %s\n", providerName)
			fmt.Printf("  - Model: %s\n", modelName)
			fmt.Printf("  - Working directory: %s\n", workingDir)
			fmt.Printf("  - Session ID: %s\n", sessionID)
		}
		os.Exit(1)
	}
	
	// Print result
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("🎯 RESULT")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println(result)
	fmt.Println(strings.Repeat("═", 80))
	
	// Print session info (important for multi-client)
	stats := updatedSession.GetStats()
	fmt.Printf("\n📊 Session Information:\n")
	fmt.Printf("   Session ID: %s\n", stats.SessionID)
	fmt.Printf("   Duration: %v\n", duration.Round(time.Millisecond))
	fmt.Printf("   Messages: %d total\n", stats.MessageCount)
	fmt.Printf("     - User: %d\n", stats.UserMessages)
	fmt.Printf("     - Assistant: %d\n", stats.AssistantMessages)
	fmt.Printf("     - Tool: %d\n", stats.ToolMessages)
	fmt.Printf("   Working directory: %s\n", stats.WorkingDir)
	
	if showProgress {
		fmt.Printf("\n💡 To continue this session from another client:\n")
		fmt.Printf("   Use session ID: %s\n", stats.SessionID)
	}
}