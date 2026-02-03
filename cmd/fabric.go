package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chzyer/readline"
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

Interactive mode (no task argument):
  Enter a fabric session where you can configure coordinator/worker dynamically
  and run multiple tasks with shared context.

One-shot mode (with task argument):
  Run a single task and exit.

Examples:
  # Interactive mode - enter fabric session
  zen-claw fabric

  # One-shot: DeepSeek coordinates Qwen for security analysis
  zen-claw fabric --coordinator deepseek --worker qwen --role security_architect \
    "Analyze zen-claw codebase for security vulnerabilities"`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			task := ""
			if len(args) > 0 {
				task = args[0]
			}

			// If no task provided, enter interactive mode
			if task == "" {
				// Check stdin for piped input
				stat, _ := os.Stdin.Stat()
				if (stat.Mode() & os.ModeCharDevice) == 0 {
					data, err := os.ReadFile("/dev/stdin")
					if err == nil && len(strings.TrimSpace(string(data))) > 0 {
						task = string(data)
					}
				}
			}

			if task == "" {
				// Interactive mode
				runFabricInteractive(coordinator, worker, role, workingDir, verbose)
			} else {
				// One-shot mode
				runFabric(task, coordinator, worker, role, workingDir, verbose)
			}
		},
	}

	cmd.Flags().StringVar(&coordinator, "coordinator", "deepseek", "Coordinator AI provider (plans and reviews)")
	cmd.Flags().StringVar(&worker, "worker", "qwen", "Worker AI provider (executes the task)")
	cmd.Flags().StringVar(&role, "role", "software_architect", "Expert role for both coordinator and worker")
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

// FabricSession maintains state for interactive fabric mode
type FabricSession struct {
	cfg              *config.Config
	factory          *providers.Factory
	coordinator      string
	worker           string
	role             string
	workingDir       string
	verbose          bool
	history          []FabricExchange // Conversation history for context
	coordinatorMsgs  []ai.Message     // Coordinator's message history
	workerMsgs       []ai.Message     // Worker's message history
}

// FabricExchange represents one task execution
type FabricExchange struct {
	Task           string
	Plan           string
	WorkerOutput   string
	CoordinatorReview string
	Timestamp      time.Time
}

// runFabricInteractive runs the fabric session in interactive mode
func runFabricInteractive(coordinator, worker, role, workingDir string, verbose bool) {
	fmt.Println("🧵 Zen Claw - Fabric Interactive Session")
	fmt.Println("═" + strings.Repeat("═", 78))
	fmt.Println("Coordinator + Worker mode with shared context.")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  /coordinator <name>  - Set coordinator AI (deepseek, qwen, kimi, etc.)")
	fmt.Println("  /worker <name>       - Set worker AI")
	fmt.Println("  /role <name>         - Set expert role (security_architect, api_designer, etc.)")
	fmt.Println("  /status              - Show current configuration")
	fmt.Println("  /history             - Show task history")
	fmt.Println("  /clear               - Clear conversation history")
	fmt.Println("  /verbose             - Toggle verbose output")
	fmt.Println("  /exit, /quit         - Exit fabric session")
	fmt.Println("  /help                - Show this help")
	fmt.Println()
	fmt.Println("Just type a task to run it through coordinator → worker → review.")
	fmt.Println("═" + strings.Repeat("═", 78))

	// Load config
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		return
	}

	// Initialize session
	session := &FabricSession{
		cfg:         cfg,
		factory:     providers.NewFactory(cfg),
		coordinator: coordinator,
		worker:      worker,
		role:        role,
		workingDir:  workingDir,
		verbose:     verbose,
		history:     []FabricExchange{},
		coordinatorMsgs: []ai.Message{},
		workerMsgs:      []ai.Message{},
	}

	// Verify providers work
	if _, err := session.factory.CreateProvider(session.coordinator); err != nil {
		fmt.Printf("⚠️ Coordinator '%s' not available: %v\n", session.coordinator, err)
	}
	if _, err := session.factory.CreateProvider(session.worker); err != nil {
		fmt.Printf("⚠️ Worker '%s' not available: %v\n", session.worker, err)
	}

	session.printStatus()

	// Setup readline
	historyFile := filepath.Join(os.Getenv("HOME"), ".zen-claw-fabric-history")
	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "\n🧵 > ",
		HistoryFile:       historyFile,
		HistoryLimit:      1000,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		fmt.Printf("Warning: readline not available: %v\n", err)
		return
	}
	defer rl.Close()

	// Interactive loop
	for {
		input, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				fmt.Println("\nInterrupted. Use /exit to quit.")
				continue
			}
			if err == io.EOF {
				fmt.Println("\nExiting...")
				return
			}
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle commands
		switch {
		case input == "/exit" || input == "/quit":
			fmt.Println("Exiting fabric session...")
			return

		case input == "/help":
			fmt.Println("Commands:")
			fmt.Println("  /coordinator <name>  - Set coordinator AI")
			fmt.Println("  /worker <name>       - Set worker AI")
			fmt.Println("  /role <name>         - Set expert role")
			fmt.Println("  /status              - Show current configuration")
			fmt.Println("  /history             - Show task history")
			fmt.Println("  /clear               - Clear conversation history")
			fmt.Println("  /verbose             - Toggle verbose output")
			fmt.Println("  /exit, /quit         - Exit")
			continue

		case strings.HasPrefix(input, "/coordinator"):
			parts := strings.Fields(input)
			if len(parts) < 2 {
				fmt.Printf("Current coordinator: %s\n", session.coordinator)
				fmt.Println("Usage: /coordinator <name> (deepseek, qwen, kimi, glm, minimax)")
				continue
			}
			newCoord := parts[1]
			if _, err := session.factory.CreateProvider(newCoord); err != nil {
				fmt.Printf("❌ Provider '%s' not available: %v\n", newCoord, err)
				continue
			}
			session.coordinator = newCoord
			fmt.Printf("✓ Coordinator set to: %s\n", session.coordinator)
			continue

		case strings.HasPrefix(input, "/worker"):
			parts := strings.Fields(input)
			if len(parts) < 2 {
				fmt.Printf("Current worker: %s\n", session.worker)
				fmt.Println("Usage: /worker <name> (deepseek, qwen, kimi, glm, minimax)")
				continue
			}
			newWorker := parts[1]
			if _, err := session.factory.CreateProvider(newWorker); err != nil {
				fmt.Printf("❌ Provider '%s' not available: %v\n", newWorker, err)
				continue
			}
			session.worker = newWorker
			fmt.Printf("✓ Worker set to: %s\n", session.worker)
			continue

		case strings.HasPrefix(input, "/role"):
			parts := strings.Fields(input)
			if len(parts) < 2 {
				fmt.Printf("Current role: %s\n", session.role)
				fmt.Println("Available: security_architect, software_architect, api_designer, database_architect, devops_engineer, frontend_architect, or custom")
				continue
			}
			session.role = strings.Join(parts[1:], "_")
			fmt.Printf("✓ Role set to: %s\n", session.role)
			continue

		case input == "/status":
			session.printStatus()
			continue

		case input == "/history":
			session.printHistory()
			continue

		case input == "/clear":
			session.history = []FabricExchange{}
			session.coordinatorMsgs = []ai.Message{}
			session.workerMsgs = []ai.Message{}
			fmt.Println("✓ Conversation history cleared")
			continue

		case input == "/verbose":
			session.verbose = !session.verbose
			fmt.Printf("✓ Verbose mode: %v\n", session.verbose)
			continue

		default:
			// It's a task - run through fabric
			session.runTask(input)
		}
	}
}

// printStatus shows current fabric configuration
func (s *FabricSession) printStatus() {
	fmt.Println()
	fmt.Println("📊 Current Configuration:")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("  Coordinator: %s\n", s.coordinator)
	fmt.Printf("  Worker:      %s\n", s.worker)
	fmt.Printf("  Role:        %s\n", s.role)
	fmt.Printf("  Working Dir: %s\n", s.workingDir)
	fmt.Printf("  Verbose:     %v\n", s.verbose)
	fmt.Printf("  History:     %d exchanges\n", len(s.history))
	fmt.Println(strings.Repeat("─", 40))
}

// printHistory shows past exchanges
func (s *FabricSession) printHistory() {
	if len(s.history) == 0 {
		fmt.Println("No history yet.")
		return
	}

	fmt.Println("\n📜 Task History:")
	fmt.Println(strings.Repeat("─", 60))
	for i, ex := range s.history {
		fmt.Printf("%d. [%s] %s\n", i+1, ex.Timestamp.Format("15:04:05"), truncateFabric(ex.Task, 60))
	}
	fmt.Println(strings.Repeat("─", 60))
}

// runTask executes a task through the fabric pipeline with context
func (s *FabricSession) runTask(task string) {
	fmt.Println()
	fmt.Printf("📋 Task: %s\n", truncateFabric(task, 70))
	fmt.Printf("   Coordinator: %s → Worker: %s (role: %s)\n", s.coordinator, s.worker, s.role)
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	totalStart := time.Now()

	// Create providers
	coordinator, err := s.factory.CreateProvider(s.coordinator)
	if err != nil {
		fmt.Printf("❌ Failed to create coordinator: %v\n", err)
		return
	}

	worker, err := s.factory.CreateProvider(s.worker)
	if err != nil {
		fmt.Printf("❌ Failed to create worker: %v\n", err)
		return
	}

	// Build context from history
	historyContext := s.buildHistoryContext()

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 1: Coordinator creates plan (with history context)
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("📋 Phase 1: Coordinator planning...")
	planStart := time.Now()

	planPrompt := fmt.Sprintf(`You are a %s acting as a coordinator.

%s

Your worker AI (%s) will execute the task. Create a DETAILED execution plan.

CURRENT TASK:
%s

WORKING DIRECTORY: %s

Create a specific execution plan. The worker will receive this plan.`, 
		s.role, historyContext, s.worker, task, s.workingDir)

	// Add to coordinator history
	s.coordinatorMsgs = append(s.coordinatorMsgs, ai.Message{Role: "user", Content: planPrompt})

	planResp, err := coordinator.Chat(ctx, ai.ChatRequest{
		Model:       s.cfg.GetModel(s.coordinator),
		Messages:    s.coordinatorMsgs,
		MaxTokens:   2000,
		Temperature: 0.7,
	})
	if err != nil {
		fmt.Printf("❌ Coordinator failed: %v\n", err)
		return
	}

	s.coordinatorMsgs = append(s.coordinatorMsgs, ai.Message{Role: "assistant", Content: planResp.Content})
	planDuration := time.Since(planStart)
	fmt.Printf("✓ Plan ready (%v)\n", planDuration.Round(time.Millisecond))

	if s.verbose {
		fmt.Println("\n" + strings.Repeat("─", 40))
		fmt.Println("PLAN:")
		fmt.Println(planResp.Content)
		fmt.Println(strings.Repeat("─", 40))
	}

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 2: Worker executes (with history context)
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("⚙️  Phase 2: Worker executing...")
	execStart := time.Now()

	execPrompt := fmt.Sprintf(`You are a %s executing a task.

%s

ORIGINAL TASK: %s

COORDINATOR'S PLAN:
%s

Execute thoroughly. Provide detailed analysis and actionable recommendations.`,
		s.role, historyContext, task, planResp.Content)

	// Add to worker history
	s.workerMsgs = append(s.workerMsgs, ai.Message{Role: "user", Content: execPrompt})

	execResp, err := worker.Chat(ctx, ai.ChatRequest{
		Model:       s.cfg.GetModel(s.worker),
		Messages:    s.workerMsgs,
		MaxTokens:   6000,
		Temperature: 0.7,
	})
	if err != nil {
		fmt.Printf("❌ Worker failed: %v\n", err)
		return
	}

	s.workerMsgs = append(s.workerMsgs, ai.Message{Role: "assistant", Content: execResp.Content})
	execDuration := time.Since(execStart)
	fmt.Printf("✓ Execution complete (%v)\n", execDuration.Round(time.Millisecond))

	if s.verbose {
		fmt.Println("\n" + strings.Repeat("─", 40))
		fmt.Println("WORKER OUTPUT:")
		fmt.Println(execResp.Content)
		fmt.Println(strings.Repeat("─", 40))
	}

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 3: Coordinator reviews
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("🔍 Phase 3: Coordinator reviewing...")
	reviewStart := time.Now()

	reviewPrompt := fmt.Sprintf(`Review the worker's output for task: %s

WORKER'S OUTPUT:
%s

Provide:
1. Quality assessment (1-10)
2. Key findings summary
3. Action items
4. Executive summary (3-5 bullets)`,
		task, execResp.Content)

	s.coordinatorMsgs = append(s.coordinatorMsgs, ai.Message{Role: "user", Content: reviewPrompt})

	reviewResp, err := coordinator.Chat(ctx, ai.ChatRequest{
		Model:       s.cfg.GetModel(s.coordinator),
		Messages:    s.coordinatorMsgs,
		MaxTokens:   3000,
		Temperature: 0.5,
	})
	if err != nil {
		fmt.Printf("❌ Review failed: %v\n", err)
		return
	}

	s.coordinatorMsgs = append(s.coordinatorMsgs, ai.Message{Role: "assistant", Content: reviewResp.Content})
	reviewDuration := time.Since(reviewStart)

	// Save to history
	s.history = append(s.history, FabricExchange{
		Task:              task,
		Plan:              planResp.Content,
		WorkerOutput:      execResp.Content,
		CoordinatorReview: reviewResp.Content,
		Timestamp:         time.Now(),
	})

	// Print results
	totalDuration := time.Since(totalStart)
	fmt.Printf("✓ Review complete (%v)\n", reviewDuration.Round(time.Millisecond))

	fmt.Println("\n" + strings.Repeat("═", 60))
	fmt.Printf("⏱️  Total: %v (plan: %v, exec: %v, review: %v)\n",
		totalDuration.Round(time.Millisecond),
		planDuration.Round(time.Millisecond),
		execDuration.Round(time.Millisecond),
		reviewDuration.Round(time.Millisecond))
	fmt.Println(strings.Repeat("═", 60))
	fmt.Println()
	fmt.Println(reviewResp.Content)
	fmt.Println()
	fmt.Println(strings.Repeat("═", 60))
}

// buildHistoryContext creates context from past exchanges
func (s *FabricSession) buildHistoryContext() string {
	if len(s.history) == 0 {
		return ""
	}

	var ctx strings.Builder
	ctx.WriteString("PREVIOUS CONTEXT (from earlier in this session):\n")
	
	// Include last 2-3 exchanges for context
	start := 0
	if len(s.history) > 3 {
		start = len(s.history) - 3
	}

	for i := start; i < len(s.history); i++ {
		ex := s.history[i]
		ctx.WriteString(fmt.Sprintf("\n--- Previous Task %d ---\n", i+1))
		ctx.WriteString(fmt.Sprintf("Task: %s\n", truncateFabric(ex.Task, 100)))
		ctx.WriteString(fmt.Sprintf("Key findings: %s\n", truncateFabric(ex.CoordinatorReview, 500)))
	}
	ctx.WriteString("\n--- Current Task ---\n")

	return ctx.String()
}
