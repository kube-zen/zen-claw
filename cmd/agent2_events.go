package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/agentlib"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
	"github.com/spf13/cobra"
)

func newAgent2EventsCmd() *cobra.Command {
	var model string
	var provider string
	var workingDir string
	
	cmd := &cobra.Command{
		Use:   "agent2-events",
		Short: "Agent 2.0 with real-time event display",
		Long: `Zen Agent 2.0 with real-time event display.
Shows tool execution, turns, and progress in real-time.

Example:
  zen-claw agent2-events "analyze this project"`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runAgent2Events(args[0], model, provider, workingDir)
		},
	}
	
	cmd.Flags().StringVar(&model, "model", "", "AI model (e.g., deepseek/deepseek-chat, openai/gpt-4o)")
	cmd.Flags().StringVar(&provider, "provider", "", "AI provider (deepseek, openai, glm, minimax, qwen)")
	cmd.Flags().StringVar(&workingDir, "working-dir", ".", "Working directory for tools")
	
	return cmd
}

func runAgent2Events(task, modelFlag, providerFlag, workingDir string) {
	fmt.Println("🚀 Zen Agent 2.0 - Real-time Event Display")
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
	fmt.Println()
	
	// Create AI provider
	factory := providers.NewFactory(cfg)
	aiProvider, err := factory.CreateProvider(providerName)
	if err != nil {
		log.Fatalf("Failed to create provider %s: %v", providerName, err)
	}
	
	// Create tools
	tools := []agentlib.Tool{
		agentlib.NewExecTool(workingDir),
		agentlib.NewReadFileTool(workingDir),
		agentlib.NewListDirTool(workingDir),
		agentlib.NewSystemInfoTool(),
	}
	
	// Create enhanced agent
	agent := agentlib.NewEnhancedAgent(agentlib.Config{
		Provider:   aiProvider,
		Tools:      tools,
		WorkingDir: workingDir,
		SessionID:  fmt.Sprintf("agent2_events_%s", strings.ReplaceAll(task, " ", "_")),
		MaxSteps:   10,
	})
	
	// Subscribe to events
	unsubscribe := agent.Subscribe(func(event agentlib.AgentEvent) {
		switch event.Type {
		case agentlib.EventAgentStart:
			data := event.Data.(agentlib.AgentStartEventData)
			fmt.Printf("📡 Agent started: %s\n", data.SessionID)
			fmt.Printf("   Task: %s\n", data.Task)
			
		case agentlib.EventTurnStart:
			data := event.Data.(agentlib.TurnStartEventData)
			fmt.Printf("\n🔄 Turn %d started\n", data.TurnNumber)
			
		case agentlib.EventToolStart:
			data := event.Data.(agentlib.ToolStartEventData)
			fmt.Printf("   🛠️  Executing tool: %s\n", data.ToolName)
			if len(data.Args) > 0 {
				fmt.Printf("     Args: %v\n", data.Args)
			}
			
		case agentlib.EventToolEnd:
			data := event.Data.(agentlib.ToolEndEventData)
			status := "✅"
			if data.IsError {
				status = "❌"
			}
			fmt.Printf("   %s Tool %s completed (%dms)\n", 
				status, data.ToolName, data.DurationMs)
			
		case agentlib.EventTurnEnd:
			data := event.Data.(agentlib.TurnEndEventData)
			fmt.Printf("   🔄 Turn %d completed\n", data.TurnNumber)
			if len(data.ToolResults) > 0 {
				fmt.Printf("     Tool results: %d\n", len(data.ToolResults))
			}
			
		case agentlib.EventAgentEnd:
			data := event.Data.(agentlib.AgentEndEventData)
			fmt.Printf("\n🎯 Agent completed: %s\n", data.SessionID)
			
		case agentlib.EventError:
			data := event.Data.(agentlib.ErrorEventData)
			fmt.Printf("\n❌ Error: %s\n", data.Error)
		}
	})
	defer unsubscribe()
	
	// Run agent
	ctx := context.Background()
	fmt.Println("\n🤖 Starting agent execution...")
	fmt.Println()
	
	startTime := time.Now()
	result, err := agent.Run(ctx, task)
	duration := time.Since(startTime)
	
	if err != nil {
		fmt.Printf("\n❌ Agent execution failed: %v\n", err)
		return
	}
	
	// Print result
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("🎯 FINAL RESULT")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println(result)
	fmt.Println(strings.Repeat("═", 80))
	
	// Print session stats
	stats := agent.GetSession().GetStats()
	fmt.Printf("\n📊 Session Statistics:\n")
	fmt.Printf("   Session ID: %s\n", stats.SessionID)
	fmt.Printf("   Duration: %v\n", duration.Round(time.Millisecond))
	fmt.Printf("   Messages: %d total\n", stats.MessageCount)
	fmt.Printf("     - User: %d\n", stats.UserMessages)
	fmt.Printf("     - Assistant: %d\n", stats.AssistantMessages)
	fmt.Printf("     - Tool: %d\n", stats.ToolMessages)
	fmt.Printf("   Working directory: %s\n", stats.WorkingDir)
}