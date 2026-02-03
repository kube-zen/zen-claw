package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/factory"
	"github.com/spf13/cobra"
)

func newFactoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "factory",
		Short: "Software factory with AI specialists",
		Long: `Run a software factory with coordinated AI specialists.

A coordinator AI manages specialist workers (Go, TypeScript, Infrastructure)
to execute complex multi-phase projects with guardrails and macro reporting.

Specialists:
  • Go: DeepSeek (fast, cost-effective, good for controllers)
  • TypeScript: Qwen (large context, coding-focused)
  • Infrastructure: MiniMax (Helm, K8s, CI/CD)
  • Coordinator: Kimi (synthesis, planning)

Commands:
  factory start    Start a new factory run with a blueprint
  factory status   Check the status of a running factory
  factory resume   Resume a paused factory run

Example blueprint (blueprint.json):
{
  "project": "zen-platform-v3",
  "phases": [
    {
      "name": "core_types",
      "task": "Define shared Go structs and TypeScript interfaces",
      "domain": "go",
      "outputs": ["zen-sdk/go/types.go"]
    },
    {
      "name": "platform_api",
      "task": "Implement HTTP handlers in zen-platform",
      "domain": "go",
      "depends_on": ["core_types"]
    }
  ]
}`,
	}

	// Add subcommands
	cmd.AddCommand(newFactoryStartCmd())
	cmd.AddCommand(newFactoryStatusCmd())
	cmd.AddCommand(newFactoryResumeCmd())

	return cmd
}

func newFactoryStartCmd() *cobra.Command {
	var blueprintFile string
	var workingDir string
	var maxDuration string
	var maxCost float64

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a factory run with a blueprint",
		Long: `Start a software factory run with a blueprint file.

The factory will:
1. Parse the blueprint phases
2. Execute each phase with the appropriate AI specialist
3. Report macro updates as phases complete
4. Pause for human intervention if errors occur

Example:
  zen-claw factory start --blueprint ./blueprint.json`,
		Run: func(cmd *cobra.Command, args []string) {
			runFactoryStart(blueprintFile, workingDir, maxDuration, maxCost)
		},
	}

	cmd.Flags().StringVar(&blueprintFile, "blueprint", "", "Path to blueprint JSON file (required)")
	cmd.Flags().StringVar(&workingDir, "working-dir", ".", "Working directory for execution")
	cmd.Flags().StringVar(&maxDuration, "max-duration", "4h", "Maximum total duration")
	cmd.Flags().Float64Var(&maxCost, "max-cost", 5.0, "Maximum total cost in dollars")
	cmd.MarkFlagRequired("blueprint")

	return cmd
}

func newFactoryStatusCmd() *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check factory status",
		Long: `Check the status of a factory run.

Shows:
- Current phase
- Completed phases
- Any errors or pauses
- Total cost so far`,
		Run: func(cmd *cobra.Command, args []string) {
			runFactoryStatus(project)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name to check status for")
	cmd.MarkFlagRequired("project")

	return cmd
}

func newFactoryResumeCmd() *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume a paused factory",
		Long: `Resume a factory run that was paused due to errors.

The factory pauses when:
- A phase fails and auto-fix fails
- Guardrails are exceeded
- Human intervention is needed`,
		Run: func(cmd *cobra.Command, args []string) {
			runFactoryResume(project)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name to resume")
	cmd.MarkFlagRequired("project")

	return cmd
}

func runFactoryStart(blueprintFile, workingDir, maxDuration string, maxCost float64) {
	fmt.Println("🏭 Zen Claw - Software Factory")
	fmt.Println("═" + strings.Repeat("═", 78))

	// Parse duration
	duration, err := time.ParseDuration(maxDuration)
	if err != nil {
		fmt.Printf("❌ Invalid duration: %v\n", err)
		os.Exit(1)
	}

	// Read blueprint
	data, err := os.ReadFile(blueprintFile)
	if err != nil {
		fmt.Printf("❌ Failed to read blueprint: %v\n", err)
		os.Exit(1)
	}

	blueprint, err := factory.ParseBlueprint(data)
	if err != nil {
		fmt.Printf("❌ Failed to parse blueprint: %v\n", err)
		os.Exit(1)
	}

	if blueprint.WorkingDir == "" {
		blueprint.WorkingDir = workingDir
	}

	fmt.Printf("Project: %s\n", blueprint.Project)
	fmt.Printf("Phases: %d\n", len(blueprint.Phases))
	fmt.Printf("Working Dir: %s\n", blueprint.WorkingDir)
	fmt.Printf("Max Duration: %v\n", duration)
	fmt.Printf("Max Cost: $%.2f\n", maxCost)
	fmt.Println()

	// Show phases
	fmt.Println("📋 Blueprint Phases:")
	for i, phase := range blueprint.Phases {
		deps := ""
		if len(phase.DependsOn) > 0 {
			deps = fmt.Sprintf(" (depends: %s)", strings.Join(phase.DependsOn, ", "))
		}
		fmt.Printf("  %d. [%s] %s%s\n", i+1, phase.Domain, phase.Name, deps)
	}
	fmt.Println()

	// Load config
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create factory
	f := factory.NewFactory(cfg)

	// Set guardrails
	guardrails := f.GetGuardrails()
	guardrails.MaxTotalDuration = duration
	guardrails.MaxCostTotal = maxCost
	f.SetGuardrails(guardrails)

	// Start listening for updates in background
	go func() {
		for update := range f.Updates() {
			statusEmoji := "📌"
			switch update.Status {
			case factory.StatusRunning:
				statusEmoji = "⏳"
			case factory.StatusCompleted:
				statusEmoji = "✅"
			case factory.StatusFailed:
				statusEmoji = "❌"
			case factory.StatusBlocked:
				statusEmoji = "🚫"
			}

			durationStr := ""
			if update.Duration != "" {
				durationStr = fmt.Sprintf(" (%s)", update.Duration)
			}

			fmt.Printf("[%s] %s %s: %s%s\n",
				update.Timestamp.Format("15:04:05"),
				statusEmoji,
				update.Phase,
				truncateString(update.Message, 60),
				durationStr)
		}
	}()

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// Run factory
	fmt.Println("🚀 Starting factory execution...")
	fmt.Println(strings.Repeat("─", 80))

	if err := f.Run(ctx, *blueprint); err != nil {
		fmt.Printf("\n❌ Factory stopped: %v\n", err)

		state := f.GetState()
		if state != nil && state.Paused {
			fmt.Printf("\n⏸️  Factory paused: %s\n", state.PauseReason)
			fmt.Printf("   Resume with: zen-claw factory resume --project %s\n", blueprint.Project)
		}
		os.Exit(1)
	}

	// Print summary
	state := f.GetState()
	if state != nil {
		fmt.Println("\n" + strings.Repeat("═", 80))
		fmt.Println("📊 FACTORY SUMMARY")
		fmt.Println(strings.Repeat("═", 80))
		fmt.Printf("Total Duration: %v\n", time.Since(state.StartedAt).Round(time.Second))
		fmt.Printf("Estimated Cost: $%.4f\n", state.TotalCost)
		fmt.Printf("Phases Completed: %d/%d\n", state.CurrentPhase, len(blueprint.Phases))
	}
}

func runFactoryStatus(project string) {
	fmt.Println("🏭 Zen Claw - Factory Status")
	fmt.Println("═" + strings.Repeat("═", 78))

	// Load config
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create factory and load state
	f := factory.NewFactory(cfg)
	if err := f.LoadState(project); err != nil {
		fmt.Printf("❌ Failed to load factory state: %v\n", err)
		fmt.Printf("   (State file may not exist for project: %s)\n", project)
		os.Exit(1)
	}

	state := f.GetState()
	if state == nil {
		fmt.Println("No factory state found")
		os.Exit(1)
	}

	// Print status
	fmt.Printf("Project: %s\n", state.Blueprint.Project)
	fmt.Printf("Started: %s\n", state.StartedAt.Format(time.RFC3339))
	fmt.Printf("Updated: %s\n", state.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("Duration: %v\n", time.Since(state.StartedAt).Round(time.Second))
	fmt.Printf("Cost: $%.4f\n", state.TotalCost)
	fmt.Printf("Paused: %v\n", state.Paused)
	if state.PauseReason != "" {
		fmt.Printf("Pause Reason: %s\n", state.PauseReason)
	}
	fmt.Println()

	fmt.Println("📋 Phase Status:")
	for i, phase := range state.Blueprint.Phases {
		status := "⏳ Pending"
		if result, ok := state.PhaseResults[phase.Name]; ok {
			switch result.Status {
			case factory.StatusCompleted:
				status = fmt.Sprintf("✅ Completed (%v)", result.Duration.Round(time.Millisecond))
			case factory.StatusFailed:
				status = "❌ Failed"
			case factory.StatusBlocked:
				status = "🚫 Blocked"
			}
		} else if i == state.CurrentPhase {
			status = "🔄 Current"
		}
		fmt.Printf("  %d. [%s] %s: %s\n", i+1, phase.Domain, phase.Name, status)
	}
}

func runFactoryResume(project string) {
	fmt.Println("🏭 Zen Claw - Factory Resume")
	fmt.Println("═" + strings.Repeat("═", 78))

	// Load config
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create factory and load state
	f := factory.NewFactory(cfg)
	if err := f.LoadState(project); err != nil {
		fmt.Printf("❌ Failed to load factory state: %v\n", err)
		os.Exit(1)
	}

	state := f.GetState()
	if state == nil {
		fmt.Println("No factory state found")
		os.Exit(1)
	}

	if !state.Paused {
		fmt.Println("Factory is not paused")
		os.Exit(1)
	}

	fmt.Printf("Resuming project: %s\n", state.Blueprint.Project)
	fmt.Printf("From phase: %d/%d\n", state.CurrentPhase+1, len(state.Blueprint.Phases))
	fmt.Println()

	// Listen for updates
	go func() {
		for update := range f.Updates() {
			fmt.Printf("[%s] %s: %s\n",
				update.Timestamp.Format("15:04:05"),
				update.Phase,
				update.Message)
		}
	}()

	// Resume
	ctx := context.Background()
	if err := f.Resume(ctx); err != nil {
		fmt.Printf("\n❌ Resume failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Factory completed successfully")
}

// Example blueprint for quick start
func printExampleBlueprint() {
	example := factory.Blueprint{
		Project:     "zen-platform-v3",
		Description: "Refactor zen-platform with new types",
		Phases: []factory.Phase{
			{
				Name:       "core_types",
				Task:       "Define shared Go structs and TypeScript interfaces from proto",
				Domain:     "go",
				Inputs:     []string{"api/proto/entities.proto"},
				Outputs:    []string{"zen-sdk/go/types.go", "zen-sdk/ts/types.ts"},
				Validation: []string{"go build ./...", "tsc --noEmit"},
			},
			{
				Name:       "platform_api",
				Task:       "Implement HTTP handlers for new endpoints",
				Domain:     "go",
				DependsOn:  []string{"core_types"},
				Outputs:    []string{"zen-platform/api/handlers/"},
				Validation: []string{"go test ./api/..."},
			},
			{
				Name:       "frontend_client",
				Task:       "Generate TypeScript client for zen-watcher",
				Domain:     "typescript",
				DependsOn:  []string{"core_types"},
				Outputs:    []string{"zen-watcher/src/client.ts"},
				Validation: []string{"npm run build"},
			},
			{
				Name:       "integration",
				Task:       "Wire services together and update Helm values",
				Domain:     "infrastructure",
				DependsOn:  []string{"platform_api", "frontend_client"},
				Outputs:    []string{"helm-charts/values.yaml"},
				Validation: []string{"helm template ."},
			},
		},
	}

	data, _ := json.MarshalIndent(example, "", "  ")
	fmt.Println(string(data))
}
