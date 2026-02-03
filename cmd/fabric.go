package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
	"github.com/spf13/cobra"
)

// SpecializedWorker represents a worker with a specific role/domain
type SpecializedWorker struct {
	Name     string `json:"name"`     // e.g., "go_expert", "ts_expert"
	Provider string `json:"provider"` // e.g., "qwen", "minimax"
	Role     string `json:"role"`     // e.g., "go_developer", "typescript_developer"
}

// FabricProfile represents a saved fabric configuration
type FabricProfile struct {
	Name        string              `json:"name"`
	Coordinator string              `json:"coordinator"`
	Workers     []SpecializedWorker `json:"workers"`
	Description string              `json:"description,omitempty"`
}

func newFabricCmd() *cobra.Command {
	var profile string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "fabric [task]",
		Short: "Multi-worker fabric with coordinator",
		Long: `Run tasks with a coordinator AI that delegates to specialized workers.

Interactive mode (no task argument):
  Enter a fabric session with multiple workers and shared context.

Features:
  - Multiple workers with different specializations (Go, TypeScript, etc.)
  - Parallel execution for independent subtasks
  - Profiles to save/load configurations

Examples:
  # Interactive mode
  zen-claw fabric

  # Load a saved profile
  zen-claw fabric --profile fullstack`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			task := ""
			if len(args) > 0 {
				task = args[0]
			}

			if task == "" {
				runFabricInteractive(profile, verbose)
			} else {
				fmt.Println("For one-shot tasks, use interactive mode: zen-claw fabric")
			}
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "Load a saved profile")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed output")

	return cmd
}

// FabricSession maintains state for interactive fabric mode
type FabricSession struct {
	cfg         *config.Config
	factory     *providers.Factory
	coordinator string
	workers     []SpecializedWorker
	workingDir  string
	verbose     bool
	history     []FabricExchange
	workerMsgs  map[string][]ai.Message // worker name -> messages
	coordMsgs   []ai.Message
	profilesDir string
}

// FabricExchange represents one task execution
type FabricExchange struct {
	Task           string
	WorkerOutputs  map[string]string
	CoordReview    string
	Timestamp      time.Time
}

// runFabricInteractive runs the fabric session in interactive mode
func runFabricInteractive(profileName string, verbose bool) {
	fmt.Println("🧵 Zen Claw - Multi-Worker Fabric")
	fmt.Println("═" + strings.Repeat("═", 78))
	fmt.Println()
	fmt.Println("Team Commands:")
	fmt.Println("  /coordinator <provider>      - Set coordinator (deepseek, qwen, kimi)")
	fmt.Println("  /worker add <name> <provider> <role>  - Add a specialized worker")
	fmt.Println("  /worker remove <name>        - Remove a worker")
	fmt.Println("  /worker list                 - List all workers")
	fmt.Println()
	fmt.Println("Profile Commands:")
	fmt.Println("  /profile save <name>         - Save current configuration")
	fmt.Println("  /profile load <name>         - Load a saved profile")
	fmt.Println("  /profile list                - List saved profiles")
	fmt.Println("  /profile delete <name>       - Delete a profile")
	fmt.Println()
	fmt.Println("Session Commands:")
	fmt.Println("  /status                      - Show current team")
	fmt.Println("  /history                     - Show task history")
	fmt.Println("  /clear                       - Clear history")
	fmt.Println("  /verbose                     - Toggle verbose mode")
	fmt.Println("  /exit                        - Exit")
	fmt.Println()
	fmt.Println("Just type a task to run it through your team.")
	fmt.Println("═" + strings.Repeat("═", 78))

	// Load config
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		return
	}

	// Initialize session
	home, _ := os.UserHomeDir()
	session := &FabricSession{
		cfg:         cfg,
		factory:     providers.NewFactory(cfg),
		coordinator: "deepseek",
		workers:     []SpecializedWorker{},
		workingDir:  ".",
		verbose:     verbose,
		history:     []FabricExchange{},
		workerMsgs:  make(map[string][]ai.Message),
		coordMsgs:   []ai.Message{},
		profilesDir: filepath.Join(home, ".zen", "zen-claw", "fabric-profiles"),
	}

	// Ensure profiles directory exists
	os.MkdirAll(session.profilesDir, 0755)

	// Load profile if specified
	if profileName != "" {
		if err := session.loadProfile(profileName); err != nil {
			fmt.Printf("⚠️ Could not load profile '%s': %v\n", profileName, err)
		} else {
			fmt.Printf("✓ Loaded profile: %s\n", profileName)
		}
	}

	// If no workers, add defaults
	if len(session.workers) == 0 {
		fmt.Println("\n💡 No workers configured. Add workers with: /worker add <name> <provider> <role>")
		fmt.Println("   Example: /worker add go_expert qwen go_developer")
		fmt.Println("   Example: /worker add ts_expert minimax typescript_developer")
	}

	session.printStatus()

	// Setup readline
	historyFile := filepath.Join(home, ".zen-claw-fabric-history")
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
				fmt.Println("\nUse /exit to quit.")
				continue
			}
			if err == io.EOF {
				return
			}
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		session.handleCommand(input)
	}
}

// handleCommand processes a command or task
func (s *FabricSession) handleCommand(input string) {
	switch {
	case input == "/exit" || input == "/quit":
		fmt.Println("Exiting fabric session...")
		os.Exit(0)

	case input == "/help":
		s.printHelp()

	case strings.HasPrefix(input, "/coordinator"):
		s.handleCoordinator(input)

	case strings.HasPrefix(input, "/worker"):
		s.handleWorker(input)

	case strings.HasPrefix(input, "/profile"):
		s.handleProfile(input)

	case input == "/status":
		s.printStatus()

	case input == "/history":
		s.printHistory()

	case input == "/clear":
		s.history = []FabricExchange{}
		s.workerMsgs = make(map[string][]ai.Message)
		s.coordMsgs = []ai.Message{}
		fmt.Println("✓ History cleared")

	case input == "/verbose":
		s.verbose = !s.verbose
		fmt.Printf("✓ Verbose: %v\n", s.verbose)

	default:
		// It's a task
		if len(s.workers) == 0 {
			fmt.Println("❌ No workers configured. Add workers first:")
			fmt.Println("   /worker add go_expert qwen go_developer")
			return
		}
		s.runTask(input)
	}
}

// handleCoordinator sets the coordinator
func (s *FabricSession) handleCoordinator(input string) {
	parts := strings.Fields(input)
	if len(parts) < 2 {
		fmt.Printf("Current coordinator: %s\n", s.coordinator)
		fmt.Println("Usage: /coordinator <provider>")
		return
	}

	provider := parts[1]
	if _, err := s.factory.CreateProvider(provider); err != nil {
		fmt.Printf("❌ Provider '%s' not available: %v\n", provider, err)
		return
	}

	s.coordinator = provider
	fmt.Printf("✓ Coordinator: %s\n", s.coordinator)
}

// handleWorker manages workers
func (s *FabricSession) handleWorker(input string) {
	parts := strings.Fields(input)
	if len(parts) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  /worker add <name> <provider> <role>")
		fmt.Println("  /worker remove <name>")
		fmt.Println("  /worker list")
		return
	}

	action := parts[1]

	switch action {
	case "add":
		if len(parts) < 5 {
			fmt.Println("Usage: /worker add <name> <provider> <role>")
			fmt.Println("Example: /worker add go_expert qwen go_developer")
			fmt.Println("Example: /worker add ts_expert minimax typescript_developer")
			fmt.Println("Example: /worker add security_expert deepseek security_architect")
			return
		}
		name := parts[2]
		provider := parts[3]
		role := strings.Join(parts[4:], "_")

		// Verify provider
		if _, err := s.factory.CreateProvider(provider); err != nil {
			fmt.Printf("❌ Provider '%s' not available: %v\n", provider, err)
			return
		}

		// Check for duplicate name
		for _, w := range s.workers {
			if w.Name == name {
				fmt.Printf("❌ Worker '%s' already exists\n", name)
				return
			}
		}

		s.workers = append(s.workers, SpecializedWorker{
			Name:     name,
			Provider: provider,
			Role:     role,
		})
		fmt.Printf("✓ Added worker: %s (%s, role: %s)\n", name, provider, role)

	case "remove":
		if len(parts) < 3 {
			fmt.Println("Usage: /worker remove <name>")
			return
		}
		name := parts[2]
		for i, w := range s.workers {
			if w.Name == name {
				s.workers = append(s.workers[:i], s.workers[i+1:]...)
				delete(s.workerMsgs, name)
				fmt.Printf("✓ Removed worker: %s\n", name)
				return
			}
		}
		fmt.Printf("❌ Worker '%s' not found\n", name)

	case "list":
		if len(s.workers) == 0 {
			fmt.Println("No workers configured.")
			return
		}
		fmt.Println("\n👥 Workers:")
		for _, w := range s.workers {
			fmt.Printf("   • %s: %s (role: %s)\n", w.Name, w.Provider, w.Role)
		}

	default:
		fmt.Println("Unknown action. Use: add, remove, list")
	}
}

// handleProfile manages profiles
func (s *FabricSession) handleProfile(input string) {
	parts := strings.Fields(input)
	if len(parts) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  /profile save <name>")
		fmt.Println("  /profile load <name>")
		fmt.Println("  /profile list")
		fmt.Println("  /profile delete <name>")
		return
	}

	action := parts[1]

	switch action {
	case "save":
		if len(parts) < 3 {
			fmt.Println("Usage: /profile save <name>")
			return
		}
		name := parts[2]
		if err := s.saveProfile(name); err != nil {
			fmt.Printf("❌ Failed to save: %v\n", err)
		} else {
			fmt.Printf("✓ Profile saved: %s\n", name)
		}

	case "load":
		if len(parts) < 3 {
			fmt.Println("Usage: /profile load <name>")
			return
		}
		name := parts[2]
		if err := s.loadProfile(name); err != nil {
			fmt.Printf("❌ Failed to load: %v\n", err)
		} else {
			fmt.Printf("✓ Profile loaded: %s\n", name)
			s.printStatus()
		}

	case "list":
		s.listProfiles()

	case "delete":
		if len(parts) < 3 {
			fmt.Println("Usage: /profile delete <name>")
			return
		}
		name := parts[2]
		path := filepath.Join(s.profilesDir, name+".json")
		if err := os.Remove(path); err != nil {
			fmt.Printf("❌ Failed to delete: %v\n", err)
		} else {
			fmt.Printf("✓ Profile deleted: %s\n", name)
		}

	default:
		fmt.Println("Unknown action. Use: save, load, list, delete")
	}
}

// saveProfile saves current configuration
func (s *FabricSession) saveProfile(name string) error {
	profile := FabricProfile{
		Name:        name,
		Coordinator: s.coordinator,
		Workers:     s.workers,
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(s.profilesDir, name+".json")
	return os.WriteFile(path, data, 0644)
}

// loadProfile loads a saved configuration
func (s *FabricSession) loadProfile(name string) error {
	path := filepath.Join(s.profilesDir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var profile FabricProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return err
	}

	s.coordinator = profile.Coordinator
	s.workers = profile.Workers
	s.workerMsgs = make(map[string][]ai.Message)
	return nil
}

// listProfiles shows saved profiles
func (s *FabricSession) listProfiles() {
	entries, err := os.ReadDir(s.profilesDir)
	if err != nil {
		fmt.Println("No profiles found.")
		return
	}

	fmt.Println("\n📁 Saved Profiles:")
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			name := strings.TrimSuffix(e.Name(), ".json")
			// Load and show details
			path := filepath.Join(s.profilesDir, e.Name())
			if data, err := os.ReadFile(path); err == nil {
				var p FabricProfile
				if json.Unmarshal(data, &p) == nil {
					workers := []string{}
					for _, w := range p.Workers {
						workers = append(workers, fmt.Sprintf("%s(%s)", w.Name, w.Provider))
					}
					fmt.Printf("   • %s: coord=%s, workers=[%s]\n", name, p.Coordinator, strings.Join(workers, ", "))
				}
			}
		}
	}
}

// printStatus shows current configuration
func (s *FabricSession) printStatus() {
	fmt.Println()
	fmt.Println("📊 Current Team:")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  Coordinator: %s\n", s.coordinator)
	fmt.Println("  Workers:")
	if len(s.workers) == 0 {
		fmt.Println("    (none)")
	}
	for _, w := range s.workers {
		fmt.Printf("    • %s: %s (role: %s)\n", w.Name, w.Provider, w.Role)
	}
	fmt.Printf("  History: %d tasks\n", len(s.history))
	fmt.Println(strings.Repeat("─", 50))
}

// printHistory shows past tasks
func (s *FabricSession) printHistory() {
	if len(s.history) == 0 {
		fmt.Println("No history yet.")
		return
	}
	fmt.Println("\n📜 Task History:")
	for i, ex := range s.history {
		fmt.Printf("%d. [%s] %s\n", i+1, ex.Timestamp.Format("15:04:05"), truncateFabric(ex.Task, 60))
	}
}

// printHelp shows help
func (s *FabricSession) printHelp() {
	fmt.Println("Team: /coordinator, /worker add/remove/list")
	fmt.Println("Profile: /profile save/load/list/delete")
	fmt.Println("Session: /status, /history, /clear, /verbose, /exit")
}

// runTask executes a task through the fabric pipeline
func (s *FabricSession) runTask(task string) {
	fmt.Println()
	fmt.Printf("📋 Task: %s\n", truncateFabric(task, 70))
	fmt.Printf("   Team: %s (coord) + %d workers\n", s.coordinator, len(s.workers))
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	totalStart := time.Now()

	// Create coordinator
	coordinator, err := s.factory.CreateProvider(s.coordinator)
	if err != nil {
		fmt.Printf("❌ Coordinator not available: %v\n", err)
		return
	}

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 1: Coordinator creates plan and assigns subtasks to workers
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("📋 Phase 1: Coordinator planning and assigning...")
	planStart := time.Now()

	// Build worker descriptions for coordinator
	var workerDesc strings.Builder
	for _, w := range s.workers {
		workerDesc.WriteString(fmt.Sprintf("- %s (%s): specialized in %s\n", w.Name, w.Provider, w.Role))
	}

	planPrompt := fmt.Sprintf(`You are a technical coordinator managing a team of specialized AI workers.

YOUR TEAM:
%s

TASK:
%s

Create a work plan that:
1. Analyzes the task requirements
2. Assigns specific subtasks to each worker based on their specialization
3. Identifies dependencies (which tasks can run in parallel)

OUTPUT FORMAT (JSON):
{
  "overview": "Brief task analysis",
  "assignments": [
    {"worker": "worker_name", "subtask": "specific task description", "parallel": true/false}
  ],
  "synthesis_notes": "What to focus on when combining results"
}

Assign work to ALL workers. Be specific about what each worker should do.`, workerDesc.String(), task)

	s.coordMsgs = append(s.coordMsgs, ai.Message{Role: "user", Content: planPrompt})

	planResp, err := coordinator.Chat(ctx, ai.ChatRequest{
		Model:       s.cfg.GetModel(s.coordinator),
		Messages:    s.coordMsgs,
		MaxTokens:   2000,
		Temperature: 0.7,
	})
	if err != nil {
		fmt.Printf("❌ Coordinator failed: %v\n", err)
		return
	}

	s.coordMsgs = append(s.coordMsgs, ai.Message{Role: "assistant", Content: planResp.Content})
	fmt.Printf("✓ Plan ready (%v)\n", time.Since(planStart).Round(time.Millisecond))

	if s.verbose {
		fmt.Println("\n" + strings.Repeat("─", 40))
		fmt.Println("COORDINATOR PLAN:")
		fmt.Println(planResp.Content)
		fmt.Println(strings.Repeat("─", 40))
	}

	// Parse assignments (best effort)
	assignments := s.parseAssignments(planResp.Content, task)

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 2: Workers execute in parallel
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("\n⚙️  Phase 2: Workers executing in parallel...")
	execStart := time.Now()

	workerOutputs := s.executeWorkersParallel(ctx, assignments)

	fmt.Printf("✓ All workers done (%v)\n", time.Since(execStart).Round(time.Millisecond))

	// Show individual results if verbose
	if s.verbose {
		for name, output := range workerOutputs {
			fmt.Println("\n" + strings.Repeat("─", 40))
			fmt.Printf("WORKER %s:\n", strings.ToUpper(name))
			fmt.Println(output)
		}
		fmt.Println(strings.Repeat("─", 40))
	}

	// ═══════════════════════════════════════════════════════════════════════
	// PHASE 3: Coordinator synthesizes
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("\n🔍 Phase 3: Coordinator synthesizing...")
	synthStart := time.Now()

	var outputsSummary strings.Builder
	for name, output := range workerOutputs {
		worker := s.getWorker(name)
		role := "unknown"
		if worker != nil {
			role = worker.Role
		}
		outputsSummary.WriteString(fmt.Sprintf("\n=== %s (%s) ===\n%s\n", strings.ToUpper(name), role, output))
	}

	synthPrompt := fmt.Sprintf(`ORIGINAL TASK: %s

WORKER OUTPUTS:
%s

Synthesize a comprehensive final report:
1. KEY FINDINGS - Combined insights from all workers
2. RECOMMENDATIONS - Prioritized action items
3. QUALITY ASSESSMENT - Rate each worker's contribution (1-10)
4. EXECUTIVE SUMMARY - 3-5 bullet points

Be decisive and actionable.`, task, outputsSummary.String())

	s.coordMsgs = append(s.coordMsgs, ai.Message{Role: "user", Content: synthPrompt})

	synthResp, err := coordinator.Chat(ctx, ai.ChatRequest{
		Model:       s.cfg.GetModel(s.coordinator),
		Messages:    s.coordMsgs,
		MaxTokens:   4000,
		Temperature: 0.5,
	})
	if err != nil {
		fmt.Printf("❌ Synthesis failed: %v\n", err)
		return
	}

	s.coordMsgs = append(s.coordMsgs, ai.Message{Role: "assistant", Content: synthResp.Content})

	// Save to history
	s.history = append(s.history, FabricExchange{
		Task:          task,
		WorkerOutputs: workerOutputs,
		CoordReview:   synthResp.Content,
		Timestamp:     time.Now(),
	})

	// Print results
	totalDuration := time.Since(totalStart)
	synthDuration := time.Since(synthStart)

	fmt.Printf("✓ Synthesis complete (%v)\n", synthDuration.Round(time.Millisecond))

	fmt.Println("\n" + strings.Repeat("═", 70))
	fmt.Printf("⏱️  Total: %v\n", totalDuration.Round(time.Millisecond))
	fmt.Println(strings.Repeat("═", 70))
	fmt.Println()
	fmt.Println(synthResp.Content)
	fmt.Println()
	fmt.Println(strings.Repeat("═", 70))
}

// WorkerAssignment represents work assigned to a worker
type WorkerAssignment struct {
	WorkerName string
	Subtask    string
}

// parseAssignments extracts worker assignments from coordinator's plan
func (s *FabricSession) parseAssignments(plan, originalTask string) []WorkerAssignment {
	assignments := []WorkerAssignment{}

	// Try to parse JSON
	jsonStart := strings.Index(plan, "{")
	jsonEnd := strings.LastIndex(plan, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonStr := plan[jsonStart : jsonEnd+1]
		var parsed struct {
			Assignments []struct {
				Worker  string `json:"worker"`
				Subtask string `json:"subtask"`
			} `json:"assignments"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			for _, a := range parsed.Assignments {
				assignments = append(assignments, WorkerAssignment{
					WorkerName: a.Worker,
					Subtask:    a.Subtask,
				})
			}
		}
	}

	// If parsing failed or incomplete, assign the original task to all workers
	if len(assignments) < len(s.workers) {
		for _, w := range s.workers {
			found := false
			for _, a := range assignments {
				if a.WorkerName == w.Name {
					found = true
					break
				}
			}
			if !found {
				assignments = append(assignments, WorkerAssignment{
					WorkerName: w.Name,
					Subtask:    fmt.Sprintf("Analyze from your perspective as %s: %s", w.Role, originalTask),
				})
			}
		}
	}

	return assignments
}

// executeWorkersParallel runs all workers in parallel
func (s *FabricSession) executeWorkersParallel(ctx context.Context, assignments []WorkerAssignment) map[string]string {
	outputs := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, assignment := range assignments {
		worker := s.getWorker(assignment.WorkerName)
		if worker == nil {
			continue
		}

		wg.Add(1)
		go func(w *SpecializedWorker, subtask string) {
			defer wg.Done()

			fmt.Printf("   → %s starting...\n", w.Name)
			start := time.Now()

			provider, err := s.factory.CreateProvider(w.Provider)
			if err != nil {
				mu.Lock()
				outputs[w.Name] = fmt.Sprintf("ERROR: %v", err)
				mu.Unlock()
				return
			}

			prompt := fmt.Sprintf(`You are a %s.

YOUR TASK:
%s

Provide a detailed, actionable response from your specialized perspective.
Include specific recommendations and examples where relevant.`, w.Role, subtask)

			// Get or create worker message history
			mu.Lock()
			msgs := s.workerMsgs[w.Name]
			msgs = append(msgs, ai.Message{Role: "user", Content: prompt})
			mu.Unlock()

			resp, err := provider.Chat(ctx, ai.ChatRequest{
				Model:       s.cfg.GetModel(w.Provider),
				Messages:    msgs,
				MaxTokens:   4000,
				Temperature: 0.7,
			})

			mu.Lock()
			if err != nil {
				outputs[w.Name] = fmt.Sprintf("ERROR: %v", err)
			} else {
				outputs[w.Name] = resp.Content
				msgs = append(msgs, ai.Message{Role: "assistant", Content: resp.Content})
				s.workerMsgs[w.Name] = msgs
			}
			mu.Unlock()

			fmt.Printf("   ✓ %s done (%v)\n", w.Name, time.Since(start).Round(time.Millisecond))
		}(worker, assignment.Subtask)
	}

	wg.Wait()
	return outputs
}

// getWorker finds a worker by name
func (s *FabricSession) getWorker(name string) *SpecializedWorker {
	for i := range s.workers {
		if s.workers[i].Name == name {
			return &s.workers[i]
		}
	}
	return nil
}

func truncateFabric(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
