package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
	"github.com/spf13/cobra"
)

func newFabricCmd() *cobra.Command {
	var coordinator string
	var worker string
	var role string
	var workingDir string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "fabric [task]",
		Short: "Coordinator + Worker mode for delegated tasks",
		Long: `Run a task with a coordinator AI that delegates to a worker AI.

The coordinator:
1. Analyzes the task and creates a detailed plan
2. Delegates execution to the worker
3. Reviews the worker's output
4. Provides final synthesis and recommendations

This is simpler than full factory mode - just coordinator + worker for one task.

Examples:
  # DeepSeek coordinates Qwen for security analysis
  zen-claw fabric --coordinator deepseek --worker qwen --role security_architect \
    "Analyze zen-claw codebase for security vulnerabilities"

  # Kimi coordinates DeepSeek for architecture
  zen-claw fabric --coordinator kimi --worker deepseek --role software_architect \
    "Design caching layer for zen-platform"`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			task := ""
			if len(args) > 0 {
				task = args[0]
			} else {
				// Read from stdin
				stat, _ := os.Stdin.Stat()
				if (stat.Mode() & os.ModeCharDevice) == 0 {
					data, err := os.ReadFile("/dev/stdin")
					if err == nil {
						task = string(data)
					}
				}
			}

			if task == "" {
				fmt.Println("Error: Please provide a task or pipe input via stdin")
				os.Exit(1)
			}

			runFabric(task, coordinator, worker, role, workingDir, verbose)
		},
	}

	cmd.Flags().StringVar(&coordinator, "coordinator", "deepseek", "Coordinator AI provider (plans and reviews)")
	cmd.Flags().StringVar(&worker, "worker", "qwen", "Worker AI provider (executes the task)")
	cmd.Flags().StringVar(&role, "role", "security_architect", "Expert role for both coordinator and worker")
	cmd.Flags().StringVar(&workingDir, "working-dir", ".", "Working directory for context")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed output from each phase")

	return cmd
}

func runFabric(task, coordinatorName, workerName, role, workingDir string, verbose bool) {
	fmt.Println("🧵 Zen Claw - Fabric Mode")
	fmt.Println("═" + strings.Repeat("═", 78))
	fmt.Printf("Role: %s\n", role)
	fmt.Printf("Coordinator: %s (plans + reviews)\n", coordinatorName)
	fmt.Printf("Worker: %s (executes)\n", workerName)
	fmt.Printf("Task: %s\n", truncateFabric(task, 80))
	fmt.Println()

	// Load config
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create provider factory
	factory := providers.NewFactory(cfg)

	// Create coordinator provider
	coordinator, err := factory.CreateProvider(coordinatorName)
	if err != nil {
		fmt.Printf("❌ Failed to create coordinator (%s): %v\n", coordinatorName, err)
		os.Exit(1)
	}

	// Create worker provider
	worker, err := factory.CreateProvider(workerName)
	if err != nil {
		fmt.Printf("❌ Failed to create worker (%s): %v\n", workerName, err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	totalStart := time.Now()

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 1: Coordinator creates detailed plan
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("📋 Phase 1: Coordinator creating execution plan...")
	planStart := time.Now()

	planPrompt := fmt.Sprintf(`You are a %s acting as a coordinator.

Your team has a worker AI (%s) that will execute the task. Your job is to:
1. Analyze the task thoroughly
2. Create a DETAILED execution plan for the worker
3. Specify exactly what the worker should analyze/produce
4. Define success criteria

TASK FROM USER:
%s

WORKING DIRECTORY: %s

Create a detailed execution plan. Be specific about:
- What files/areas to analyze (if applicable)
- What aspects to focus on
- What format the output should be in
- Success criteria

Your plan will be given to the worker AI as instructions.`, role, workerName, task, workingDir)

	planResp, err := coordinator.Chat(ctx, ai.ChatRequest{
		Model: cfg.GetModel(coordinatorName),
		Messages: []ai.Message{
			{Role: "user", Content: planPrompt},
		},
		MaxTokens:   2000,
		Temperature: 0.7,
	})
	if err != nil {
		fmt.Printf("❌ Coordinator failed to create plan: %v\n", err)
		os.Exit(1)
	}

	planDuration := time.Since(planStart)
	fmt.Printf("✓ Plan created in %v\n", planDuration.Round(time.Millisecond))

	if verbose {
		fmt.Println("\n" + strings.Repeat("─", 60))
		fmt.Println("COORDINATOR'S PLAN:")
		fmt.Println(strings.Repeat("─", 60))
		fmt.Println(planResp.Content)
		fmt.Println(strings.Repeat("─", 60))
	}

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 2: Worker executes the plan
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("\n⚙️  Phase 2: Worker executing plan...")
	execStart := time.Now()

	execPrompt := fmt.Sprintf(`You are a %s executing a task assigned by your coordinator.

ORIGINAL TASK:
%s

COORDINATOR'S EXECUTION PLAN:
%s

WORKING DIRECTORY: %s

Execute the plan thoroughly. Provide:
1. Detailed analysis/findings
2. Specific recommendations with code examples where relevant
3. Prioritized action items
4. Any risks or concerns identified

Be comprehensive and actionable.`, role, task, planResp.Content, workingDir)

	execResp, err := worker.Chat(ctx, ai.ChatRequest{
		Model: cfg.GetModel(workerName),
		Messages: []ai.Message{
			{Role: "user", Content: execPrompt},
		},
		MaxTokens:   6000,
		Temperature: 0.7,
	})
	if err != nil {
		fmt.Printf("❌ Worker failed to execute: %v\n", err)
		os.Exit(1)
	}

	execDuration := time.Since(execStart)
	fmt.Printf("✓ Execution completed in %v\n", execDuration.Round(time.Millisecond))

	if verbose {
		fmt.Println("\n" + strings.Repeat("─", 60))
		fmt.Println("WORKER'S OUTPUT:")
		fmt.Println(strings.Repeat("─", 60))
		fmt.Println(execResp.Content)
		fmt.Println(strings.Repeat("─", 60))
	}

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 3: Coordinator reviews and synthesizes
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("\n🔍 Phase 3: Coordinator reviewing and synthesizing...")
	reviewStart := time.Now()

	reviewPrompt := fmt.Sprintf(`You are a %s reviewing work from your team.

ORIGINAL TASK:
%s

YOUR EXECUTION PLAN:
%s

WORKER'S OUTPUT:
%s

As the coordinator, provide:

1. QUALITY ASSESSMENT
   - Did the worker address all aspects of the plan?
   - Rate the quality: 1-10
   - What was done well?
   - What was missed or needs improvement?

2. FINAL SYNTHESIS
   - Key findings (prioritized)
   - Critical recommendations
   - Immediate action items

3. EXECUTIVE SUMMARY
   - 3-5 bullet points for leadership
   - Risk level assessment (Low/Medium/High/Critical)

Be decisive and actionable.`, role, task, planResp.Content, execResp.Content)

	reviewResp, err := coordinator.Chat(ctx, ai.ChatRequest{
		Model: cfg.GetModel(coordinatorName),
		Messages: []ai.Message{
			{Role: "user", Content: reviewPrompt},
		},
		MaxTokens:   4000,
		Temperature: 0.5,
	})
	if err != nil {
		fmt.Printf("❌ Coordinator failed to review: %v\n", err)
		os.Exit(1)
	}

	reviewDuration := time.Since(reviewStart)
	fmt.Printf("✓ Review completed in %v\n", reviewDuration.Round(time.Millisecond))

	// ═══════════════════════════════════════════════════════════════════════
	// FINAL OUTPUT
	// ═══════════════════════════════════════════════════════════════════════
	totalDuration := time.Since(totalStart)

	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("📊 TIMING SUMMARY")
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("  Phase 1 (Plan):    %v (%s)\n", planDuration.Round(time.Millisecond), coordinatorName)
	fmt.Printf("  Phase 2 (Execute): %v (%s)\n", execDuration.Round(time.Millisecond), workerName)
	fmt.Printf("  Phase 3 (Review):  %v (%s)\n", reviewDuration.Round(time.Millisecond), coordinatorName)
	fmt.Printf("  Total:             %v\n", totalDuration.Round(time.Millisecond))

	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("📋 COORDINATOR'S FINAL REPORT")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println()
	fmt.Println(reviewResp.Content)
	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))

	// Show raw worker output if not verbose (user might want it)
	if !verbose {
		fmt.Println("\n💡 Use --verbose to see detailed worker output")
	}
}

func truncateFabric(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
