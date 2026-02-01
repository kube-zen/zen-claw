package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/neves/zen-claw/internal/agentlib"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
	"github.com/spf13/cobra"
)

func newAgent2Cmd() *cobra.Command {
	var model string
	var provider string
	var workingDir string
	
	cmd := &cobra.Command{
		Use:   "agent2",
		Short: "New agent with automatic tool chaining (inspired by pi-coding-agent)",
		Long: `Zen Agent 2.0 - Next-generation agent with automatic multi-step tool execution.

Features:
- Automatic tool chaining: AI makes multiple tool calls, agent executes all
- Conversation continuation: Tool results fed back to AI for follow-up
- Session management: Save/load conversations
- Multi-provider: DeepSeek, OpenAI, GLM, Minimax, Qwen

Examples:
  zen-claw agent2 --model deepseek/deepseek-chat "check codebase and suggest improvements"
  zen-claw agent2 --provider openai "build this project"
  zen-claw agent2 --working-dir ~/myproject "analyze architecture"`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runAgent2(args[0], model, provider, workingDir)
		},
	}
	
	cmd.Flags().StringVar(&model, "model", "", "AI model (e.g., deepseek/deepseek-chat, openai/gpt-4o)")
	cmd.Flags().StringVar(&provider, "provider", "", "AI provider (deepseek, openai, glm, minimax, qwen)")
	cmd.Flags().StringVar(&workingDir, "working-dir", ".", "Working directory for tools")
	
	return cmd
}

func runAgent2(task, modelFlag, providerFlag, workingDir string) {
	log.Printf("🚀 Zen Agent 2.0 - Multi-step tool execution")
	log.Printf("Task: %s", task)
	log.Printf("Working directory: %s", workingDir)
	
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
	
	log.Printf("Using provider: %s, model: %s", providerName, modelName)
	
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
	
	// Create agent
	agent := agentlib.NewAgent(agentlib.Config{
		Provider:   aiProvider,
		Tools:      tools,
		WorkingDir: workingDir,
		SessionID:  fmt.Sprintf("agent2_%s", strings.ReplaceAll(task, " ", "_")),
		MaxSteps:   10,
	})
	
	// Run agent
	ctx := context.Background()
	log.Println("🤖 Starting agent execution...")
	
	result, err := agent.Run(ctx, task)
	if err != nil {
		log.Fatalf("Agent execution failed: %v", err)
	}
	
	// Print result
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("🎯 RESULT")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println(result)
	fmt.Println(strings.Repeat("═", 80))
	
	// Print session stats
	stats := agent.GetSession().GetStats()
	fmt.Printf("\n📊 Session: %s\n", stats.SessionID)
	fmt.Printf("   Messages: %d total (%d user, %d assistant, %d tool)\n",
		stats.MessageCount, stats.UserMessages, stats.AssistantMessages, stats.ToolMessages)
	fmt.Printf("   Working directory: %s\n", stats.WorkingDir)
}