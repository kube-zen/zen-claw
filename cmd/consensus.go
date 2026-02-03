package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/consensus"
	"github.com/spf13/cobra"
)

func newConsensusCmd() *cobra.Command {
	var role string
	var verbose bool
	var showStats bool

	cmd := &cobra.Command{
		Use:   "consensus [prompt]",
		Short: "Multi-AI consensus for better blueprints",
		Long: `Generate blueprints using multi-model AI consensus.

Sends your request to 3+ AI models in parallel with the SAME role,
then an arbiter synthesizes the best ideas and scores each worker.

The role defines the expert perspective for ALL workers AND the arbiter.
This ensures everyone approaches the problem with the same expertise.

Available roles:
  - security_architect    (security systems, zero-trust, compliance)
  - software_architect    (system design, scalability, patterns)
  - api_designer          (REST/gRPC design, schemas, versioning)
  - database_architect    (schema design, optimization, modeling)
  - devops_engineer       (CI/CD, Kubernetes, infrastructure)
  - frontend_architect    (components, state, performance)
  - Or any custom role    (e.g., "kubernetes_operator_expert")

Examples:
  # Security architecture (all AIs act as security architects)
  zen-claw consensus --role security_architect "Design zero-trust auth for microservices"

  # API design
  zen-claw consensus --role api_designer "REST API for user management with RBAC"

  # Custom role
  zen-claw consensus --role "kubernetes_operator_expert" "Design CRD for database management"

  # View worker performance stats
  zen-claw consensus --stats`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Show stats mode
			if showStats {
				showWorkerStats()
				return
			}

			prompt := ""
			if len(args) > 0 {
				prompt = args[0]
			} else {
				// Read from stdin
				stat, _ := os.Stdin.Stat()
				if (stat.Mode() & os.ModeCharDevice) == 0 {
					data, err := os.ReadFile("/dev/stdin")
					if err == nil {
						prompt = string(data)
					}
				}
			}

			if prompt == "" {
				fmt.Println("Error: Please provide a prompt or pipe input via stdin")
				fmt.Println("       Or use --stats to view worker performance")
				os.Exit(1)
			}

			runConsensus(prompt, role, verbose)
		},
	}

	cmd.Flags().StringVar(&role, "role", "software_architect", "Expert role for all workers (e.g., security_architect, api_designer)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show individual worker responses")
	cmd.Flags().BoolVar(&showStats, "stats", false, "Show worker performance statistics")

	return cmd
}

func runConsensus(prompt, role string, verbose bool) {
	fmt.Println("🤖 Zen Claw - Multi-AI Consensus")
	fmt.Println("═" + strings.Repeat("═", 78))
	fmt.Printf("Role: %s (all workers + arbiter)\n", role)
	fmt.Printf("Prompt: %s\n", truncatePrompt(prompt, 100))
	fmt.Println()

	// Load config
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create consensus engine
	engine := consensus.NewEngine(cfg)

	// Check available workers
	availableWorkers := engine.GetAvailableWorkers()
	if len(availableWorkers) < 2 {
		fmt.Printf("❌ Consensus requires at least 2 AI providers with API keys\n")
		fmt.Printf("   Available: %d\n", len(availableWorkers))
		fmt.Println("   Set API keys via environment variables:")
		fmt.Println("   - DEEPSEEK_API_KEY")
		fmt.Println("   - QWEN_API_KEY")
		fmt.Println("   - MINIMAX_API_KEY")
		fmt.Println("   - KIMI_API_KEY")
		os.Exit(1)
	}

	fmt.Printf("🔄 Using %d AI workers (all as %s):\n", len(availableWorkers), role)
	for _, w := range availableWorkers {
		fmt.Printf("   • %s/%s\n", w.Provider, w.Model)
	}
	fmt.Println()

	// Create context with generous timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Run consensus
	fmt.Println("⏳ Querying AI workers in parallel...")
	start := time.Now()

	result, err := engine.Generate(ctx, consensus.ConsensusRequest{
		Prompt: prompt,
		Role:   role,
	})

	if err != nil {
		fmt.Printf("\n❌ Consensus failed: %v\n", err)
		if result != nil && len(result.WorkerResults) > 0 {
			fmt.Println("\nPartial results from workers:")
			for _, r := range result.WorkerResults {
				if r.Error != nil {
					fmt.Printf("  • %s: ❌ %v\n", r.Worker.Provider, r.Error)
				} else {
					fmt.Printf("  • %s: ✓ %d chars in %v\n", r.Worker.Provider, len(r.Response), r.Duration.Round(time.Millisecond))
				}
			}
		}
		os.Exit(1)
	}

	// Show timing
	fmt.Printf("\n✓ Workers completed in %v (parallel)\n", result.WorkerDuration.Round(time.Millisecond))
	fmt.Printf("✓ Arbiter (%s) synthesized in %v\n", result.ArbiterModel, result.ArbiterDuration.Round(time.Millisecond))
	fmt.Printf("✓ Total time: %v\n", time.Since(start).Round(time.Millisecond))

	// Show worker scores
	fmt.Println("\n📊 Worker Scores (by arbiter):")
	for _, r := range result.WorkerResults {
		if r.Error != nil {
			fmt.Printf("   • %s/%s: ❌ error\n", r.Worker.Provider, r.Worker.Model)
		} else {
			scoreEmoji := "⭐"
			if r.Score >= 8 {
				scoreEmoji = "🌟"
			} else if r.Score < 5 {
				scoreEmoji = "⚠️"
			}
			fmt.Printf("   • %s/%s: %s %d/10", r.Worker.Provider, r.Worker.Model, scoreEmoji, r.Score)
			if r.Feedback != "" {
				fmt.Printf(" - %s", r.Feedback)
			}
			fmt.Println()
		}
	}

	// Show individual worker responses if verbose
	if verbose {
		fmt.Println("\n" + strings.Repeat("─", 80))
		fmt.Println("INDIVIDUAL WORKER RESPONSES")
		fmt.Println(strings.Repeat("─", 80))

		for _, r := range result.WorkerResults {
			fmt.Printf("\n### %s/%s - %v (score: %d/10)\n",
				strings.ToUpper(r.Worker.Provider), r.Worker.Model, r.Duration.Round(time.Millisecond), r.Score)
			if r.Error != nil {
				fmt.Printf("ERROR: %v\n", r.Error)
			} else {
				fmt.Println(r.Response)
			}
			fmt.Println()
		}
	}

	// Show synthesized blueprint
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("📋 SYNTHESIZED BLUEPRINT")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println()
	fmt.Println(result.Blueprint)
	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))
}

func showWorkerStats() {
	fmt.Println("📊 Worker Performance Statistics")
	fmt.Println("═" + strings.Repeat("═", 78))

	// Load config and engine
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	engine := consensus.NewEngine(cfg)
	stats := engine.GetWorkerStats()

	if len(stats) == 0 {
		fmt.Println("No worker statistics yet. Run some consensus tasks first.")
		return
	}

	fmt.Printf("\n%-25s %8s %8s %8s   %s\n", "WORKER", "TASKS", "AVG", "TOTAL", "BEST ROLES")
	fmt.Println(strings.Repeat("─", 80))

	for key, s := range stats {
		rolesStr := ""
		if len(s.BestRoles) > 0 {
			rolesStr = strings.Join(s.BestRoles, ", ")
		}
		fmt.Printf("%-25s %8d %8.1f %8d   %s\n",
			key, s.TotalTasks, s.AvgScore, s.TotalScore, rolesStr)
	}

	fmt.Println("\n💡 Higher average score = better performance in consensus tasks")
	fmt.Println("   Best roles = roles where worker scored 8+")
}

func truncatePrompt(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
