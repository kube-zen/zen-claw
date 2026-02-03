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
	var domain string
	var workers int
	var verbose bool

	cmd := &cobra.Command{
		Use:   "consensus [prompt]",
		Short: "Multi-AI consensus for better blueprints",
		Long: `Generate blueprints using multi-model AI consensus.

Sends your request to 3+ AI models in parallel, then synthesizes
the best ideas from each into a superior result.

This is ideal for:
- Architecture decisions
- API design
- Database schema design
- Code review
- Any complex technical decision

Examples:
  # Architecture consensus
  zen-claw consensus --domain architecture "Design a microservices architecture for e-commerce"

  # API design
  zen-claw consensus --domain api_design "REST API for user management with RBAC"

  # Code review
  zen-claw consensus --domain code_review "Review this implementation: $(cat main.go)"

  # Quick consensus (reads from stdin)
  cat requirements.md | zen-claw consensus --domain architecture`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
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
				os.Exit(1)
			}

			runConsensus(prompt, domain, workers, verbose)
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "architecture", "Domain type: architecture, api_design, database_schema, code_review")
	cmd.Flags().IntVar(&workers, "workers", 3, "Number of AI workers (default 3, requires API keys)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show individual worker responses")

	return cmd
}

func runConsensus(prompt, domain string, workers int, verbose bool) {
	fmt.Println("🤖 Zen Claw - Multi-AI Consensus")
	fmt.Println("═" + strings.Repeat("═", 78))
	fmt.Printf("Domain: %s\n", domain)
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

	fmt.Printf("🔄 Using %d AI workers:\n", len(availableWorkers))
	for _, w := range availableWorkers {
		fmt.Printf("   • %s/%s (%s)\n", w.Provider, w.Model, w.Role)
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
		Domain: domain,
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

	// Show individual worker responses if verbose
	if verbose {
		fmt.Println("\n" + strings.Repeat("─", 80))
		fmt.Println("INDIVIDUAL WORKER RESPONSES")
		fmt.Println(strings.Repeat("─", 80))

		for _, r := range result.WorkerResults {
			fmt.Printf("\n### %s/%s (%s) - %v\n",
				strings.ToUpper(r.Worker.Provider), r.Worker.Model, r.Worker.Role, r.Duration.Round(time.Millisecond))
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

func truncatePrompt(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
