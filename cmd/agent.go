package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	var model string
	var provider string
	var workingDir string
	var showEvents bool
	var maxSteps int
	
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run AI agent with automatic tool chaining",
		Long: `Zen Agent - AI agent with automatic multi-step tool execution.

Features:
- Automatic tool chaining: AI makes multiple tool calls, agent executes all
- Conversation continuation: Tool results fed back to AI for follow-up
- Session management: Save/load conversations
- Multi-provider: DeepSeek, OpenAI, GLM, Minimax, Qwen

Examples:
  zen-claw agent --model deepseek/deepseek-chat "check codebase and suggest improvements"
  zen-claw agent --provider openai "build this project"
  zen-claw agent --working-dir ~/myproject "analyze architecture"
  zen-claw agent --events "show me what's in this directory"`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runAgent(args[0], model, provider, workingDir, showEvents, maxSteps)
		},
	}
	
	cmd.Flags().StringVar(&model, "model", "", "AI model (e.g., deepseek/deepseek-chat, openai/gpt-4o)")
	cmd.Flags().StringVar(&provider, "provider", "", "AI provider (deepseek, openai, glm, minimax, qwen)")
	cmd.Flags().StringVar(&workingDir, "working-dir", ".", "Working directory for tools")
	cmd.Flags().BoolVar(&showEvents, "events", false, "Show real-time events and progress")
	cmd.Flags().IntVar(&maxSteps, "max-steps", 10, "Maximum tool execution steps")
	
	return cmd
}

func runAgent(task, modelFlag, providerFlag, workingDir string, showEvents bool, maxSteps int) {
	if showEvents {
		fmt.Println("🚀 Zen Agent - Real-time Event Display")
	} else {
		fmt.Println("🚀 Zen Agent - Multi-step Tool Execution")
	}
	fmt.Println("═" + strings.Repeat("═", 78))
	fmt.Printf("Task: %s\n", task)
	fmt.Printf("Working directory: %s\n", workingDir)
	
	// Load config
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
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
	if showEvents {
		fmt.Println()
	}
	
	// Create AI provider
	factory := providers.NewFactory(cfg)
	aiProvider, err := factory.CreateProvider(providerName)
	if err != nil {
		log.Fatalf("Failed to create provider %s: %v", providerName, err)
	}
	
	// Create tools
	tools := []agent.Tool{
		agent.NewExecTool(workingDir),
		agent.NewReadFileTool(workingDir),
		agent.NewListDirTool(workingDir),
		agent.NewSystemInfoTool(),
	}
	
	// Create agent
	var result string
	var errRun error
	
	if showEvents {
		// Use enhanced agent with events
		enhancedAgent := agent.NewEnhancedAgent(agent.Config{
			Provider:   aiProvider,
			Tools:      tools,
			WorkingDir: workingDir,
			SessionID:  fmt.Sprintf("agent_%s", strings.ReplaceAll(task, " ", "_")),
			MaxSteps:   maxSteps,
		})
		
		// Subscribe to events for display
		unsubscribe := enhancedAgent.Subscribe(func(event agent.AgentEvent) {
			displayEvent(event)
		})
		defer unsubscribe()
		
		// Run agent
		ctx := context.Background()
		if showEvents {
			fmt.Println("🤖 Starting agent execution...")
			fmt.Println()
		}
		
		result, errRun = enhancedAgent.Run(ctx, task)
	} else {
		// Use basic agent
		basicAgent := agent.NewAgent(agent.Config{
			Provider:   aiProvider,
			Tools:      tools,
			WorkingDir: workingDir,
			SessionID:  fmt.Sprintf("agent_%s", strings.ReplaceAll(task, " ", "_")),
			MaxSteps:   maxSteps,
		})
		
		// Run agent
		ctx := context.Background()
		result, errRun = basicAgent.Run(ctx, task)
	}
	
	if errRun != nil {
		fmt.Printf("\n❌ Agent execution failed: %v\n", errRun)
		os.Exit(1)
	}
	
	// Print result
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("🎯 RESULT")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println(result)
	fmt.Println(strings.Repeat("═", 80))
}

func displayEvent(event agent.AgentEvent) {
	switch event.Type {
	case agent.EventAgentStart:
		data := event.Data.(agent.AgentStartEventData)
		fmt.Printf("📡 Agent started: %s\n", data.SessionID)
		fmt.Printf("   Task: %s\n", data.Task)
		
	case agent.EventTurnStart:
		data := event.Data.(agent.TurnStartEventData)
		fmt.Printf("\n🔄 Turn %d started\n", data.TurnNumber)
		
	case agent.EventToolStart:
		data := event.Data.(agent.ToolStartEventData)
		fmt.Printf("   🛠️  Executing tool: %s\n", data.ToolName)
		if len(data.Args) > 0 {
			fmt.Printf("     Args: %v\n", data.Args)
		}
		
	case agent.EventToolEnd:
		data := event.Data.(agent.ToolEndEventData)
		status := "✅"
		if data.IsError {
			status = "❌"
		}
		fmt.Printf("   %s Tool %s completed (%dms)\n", 
			status, data.ToolName, data.DurationMs)
		
	case agent.EventTurnEnd:
		data := event.Data.(agent.TurnEndEventData)
		fmt.Printf("   🔄 Turn %d completed\n", data.TurnNumber)
		if len(data.ToolResults) > 0 {
			fmt.Printf("     Tool results: %d\n", len(data.ToolResults))
		}
		
	case agent.EventAgentEnd:
		data := event.Data.(agent.AgentEndEventData)
		fmt.Printf("\n🎯 Agent completed: %s\n", data.SessionID)
		
	case agent.EventError:
		data := event.Data.(agent.ErrorEventData)
		fmt.Printf("\n❌ Error: %s\n", data.Error)
	}
}