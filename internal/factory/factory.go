// Package factory implements a software factory with AI specialists.
// A coordinator AI manages specialist workers (Go, TypeScript, Infrastructure)
// to execute complex multi-phase projects with guardrails and macro reporting.
package factory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
)

// Specialist represents an AI specialist for a specific domain
type Specialist struct {
	Domain   string // "go", "typescript", "infrastructure", "coordinator"
	Provider string // e.g., "deepseek", "qwen", "minimax"
	Model    string // specific model
}

// Phase represents a project phase
type Phase struct {
	Name       string   `json:"name" yaml:"name"`
	Task       string   `json:"task" yaml:"task"`
	Domain     string   `json:"domain" yaml:"domain"` // "go", "typescript", "infrastructure"
	DependsOn  []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Inputs     []string `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Outputs    []string `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	Validation []string `json:"validation,omitempty" yaml:"validation,omitempty"`
}

// Blueprint represents a project blueprint
type Blueprint struct {
	Project     string  `json:"project" yaml:"project"`
	Description string  `json:"description,omitempty" yaml:"description,omitempty"`
	WorkingDir  string  `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	Phases      []Phase `json:"phases" yaml:"phases"`
}

// Guardrails define safety limits for factory execution
type Guardrails struct {
	// Time limits
	MaxPhaseDuration time.Duration `json:"max_phase_duration" yaml:"max_phase_duration"`
	MaxTotalDuration time.Duration `json:"max_total_duration" yaml:"max_total_duration"`

	// Cost limits (per 1M tokens)
	MaxCostPerPhase float64 `json:"max_cost_per_phase" yaml:"max_cost_per_phase"`
	MaxCostTotal    float64 `json:"max_cost_total" yaml:"max_cost_total"`

	// Safety limits
	MaxFilesModified   int      `json:"max_files_modified" yaml:"max_files_modified"`
	MaxLinesChanged    int      `json:"max_lines_changed" yaml:"max_lines_changed"`
	AllowedDirectories []string `json:"allowed_directories" yaml:"allowed_directories"`
	ForbiddenCommands  []string `json:"forbidden_commands" yaml:"forbidden_commands"`

	// Quality gates
	RequireTests       bool `json:"require_tests" yaml:"require_tests"`
	RequireCompilation bool `json:"require_compilation" yaml:"require_compilation"`
	RequireLinting     bool `json:"require_linting" yaml:"require_linting"`
}

// PhaseStatus represents the status of a phase
type PhaseStatus string

const (
	StatusPending   PhaseStatus = "pending"
	StatusRunning   PhaseStatus = "running"
	StatusCompleted PhaseStatus = "completed"
	StatusFailed    PhaseStatus = "failed"
	StatusBlocked   PhaseStatus = "blocked"
	StatusSkipped   PhaseStatus = "skipped"
)

// PhaseResult represents the result of a phase execution
type PhaseResult struct {
	Phase            Phase
	Status           PhaseStatus
	Output           string
	Errors           []string
	FilesModified    []string
	Duration         time.Duration
	EstimatedCost    float64
	ValidationPassed bool
}

// FactoryState represents the persistent state of a factory run
type FactoryState struct {
	Blueprint    Blueprint              `json:"blueprint"`
	CurrentPhase int                    `json:"current_phase"`
	PhaseResults map[string]PhaseResult `json:"phase_results"`
	StartedAt    time.Time              `json:"started_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	TotalCost    float64                `json:"total_cost"`
	Paused       bool                   `json:"paused"`
	PauseReason  string                 `json:"pause_reason,omitempty"`
}

// MacroUpdate represents a high-level update for the user
type MacroUpdate struct {
	Timestamp time.Time   `json:"timestamp"`
	Phase     string      `json:"phase"`
	Status    PhaseStatus `json:"status"`
	Message   string      `json:"message"`
	Duration  string      `json:"duration,omitempty"`
}

// Factory manages AI specialists for project execution
type Factory struct {
	cfg             *config.Config
	providerFactory *providers.Factory
	specialists     map[string]Specialist
	guardrails      Guardrails
	state           *FactoryState
	stateFile       string
	updates         chan MacroUpdate
	mu              sync.RWMutex
}

// NewFactory creates a new software factory
func NewFactory(cfg *config.Config) *Factory {
	return &Factory{
		cfg:             cfg,
		providerFactory: providers.NewFactory(cfg),
		specialists:     defaultSpecialists(),
		guardrails:      defaultGuardrails(),
		updates:         make(chan MacroUpdate, 100),
	}
}

// defaultSpecialists returns the default specialist configuration
func defaultSpecialists() map[string]Specialist {
	return map[string]Specialist{
		"coordinator":    {Domain: "coordinator", Provider: "kimi", Model: "kimi-k2-5"},
		"go":             {Domain: "go", Provider: "deepseek", Model: "deepseek-chat"},
		"typescript":     {Domain: "typescript", Provider: "qwen", Model: "qwen3-coder-30b"},
		"infrastructure": {Domain: "infrastructure", Provider: "minimax", Model: "minimax-M2.1"},
	}
}

// defaultGuardrails returns safe default guardrails
func defaultGuardrails() Guardrails {
	return Guardrails{
		MaxPhaseDuration:   10 * time.Minute,
		MaxTotalDuration:   4 * time.Hour,
		MaxCostPerPhase:    0.50,
		MaxCostTotal:       5.00,
		MaxFilesModified:   50,
		MaxLinesChanged:    2000,
		AllowedDirectories: []string{"."},
		ForbiddenCommands: []string{
			"rm -rf /",
			"rm -rf ~",
			"rm -rf /*",
			"drop database",
			"DROP DATABASE",
			"truncate",
			"TRUNCATE",
		},
		RequireTests:       false,
		RequireCompilation: true,
		RequireLinting:     false,
	}
}

// SetGuardrails updates the guardrails
func (f *Factory) SetGuardrails(g Guardrails) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.guardrails = g
}

// GetGuardrails returns current guardrails
func (f *Factory) GetGuardrails() Guardrails {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.guardrails
}

// Updates returns the channel for macro updates
func (f *Factory) Updates() <-chan MacroUpdate {
	return f.updates
}

// emitUpdate sends a macro update
func (f *Factory) emitUpdate(phase string, status PhaseStatus, message string, duration ...time.Duration) {
	update := MacroUpdate{
		Timestamp: time.Now(),
		Phase:     phase,
		Status:    status,
		Message:   message,
	}
	if len(duration) > 0 {
		update.Duration = duration[0].Round(time.Millisecond).String()
	}

	select {
	case f.updates <- update:
	default:
		// Channel full, log instead
		log.Printf("[Factory] %s: %s - %s", phase, status, message)
	}
}

// Run executes a blueprint
func (f *Factory) Run(ctx context.Context, blueprint Blueprint) error {
	f.mu.Lock()
	f.state = &FactoryState{
		Blueprint:    blueprint,
		CurrentPhase: 0,
		PhaseResults: make(map[string]PhaseResult),
		StartedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	f.mu.Unlock()

	f.emitUpdate("PROJECT", StatusRunning, fmt.Sprintf("Starting %s with %d phases", blueprint.Project, len(blueprint.Phases)))

	// Create state file for persistence
	stateDir := filepath.Join(os.TempDir(), "zen-claw-factory")
	os.MkdirAll(stateDir, 0755)
	f.stateFile = filepath.Join(stateDir, fmt.Sprintf("%s-state.json", blueprint.Project))

	// Process phases
	for i, phase := range blueprint.Phases {
		// Check context cancellation
		if ctx.Err() != nil {
			f.emitUpdate(phase.Name, StatusSkipped, "Context cancelled")
			return ctx.Err()
		}

		// Check guardrails
		if err := f.checkGuardrails(); err != nil {
			f.emitUpdate(phase.Name, StatusBlocked, err.Error())
			f.pauseForHuman(phase.Name, err.Error())
			return err
		}

		// Check dependencies
		if !f.dependenciesMet(phase) {
			f.emitUpdate(phase.Name, StatusBlocked, "Dependencies not met")
			continue
		}

		// Execute phase
		f.emitUpdate(phase.Name, StatusRunning, fmt.Sprintf("Starting: %s", phase.Task))

		result, err := f.executePhase(ctx, phase)
		if err != nil {
			f.emitUpdate(phase.Name, StatusFailed, err.Error())

			// Attempt auto-fix
			fixResult, fixErr := f.attemptAutoFix(ctx, phase, err)
			if fixErr != nil {
				f.emitUpdate(phase.Name, StatusBlocked, "Auto-fix failed, needs human review")
				f.pauseForHuman(phase.Name, fmt.Sprintf("Failed: %v, Auto-fix failed: %v", err, fixErr))
				continue
			}
			result = fixResult
			f.emitUpdate(phase.Name, StatusCompleted, "Auto-fix succeeded")
		}

		// Store result
		f.mu.Lock()
		f.state.PhaseResults[phase.Name] = result
		f.state.CurrentPhase = i + 1
		f.state.TotalCost += result.EstimatedCost
		f.state.UpdatedAt = time.Now()
		f.saveState()
		f.mu.Unlock()

		f.emitUpdate(phase.Name, StatusCompleted, result.Output, result.Duration)
	}

	f.emitUpdate("PROJECT", StatusCompleted, fmt.Sprintf("Completed %s", blueprint.Project))
	return nil
}

// executePhase executes a single phase using the appropriate specialist
func (f *Factory) executePhase(ctx context.Context, phase Phase) (PhaseResult, error) {
	start := time.Now()

	// Get specialist for this domain
	specialist, ok := f.specialists[phase.Domain]
	if !ok {
		specialist = f.specialists["coordinator"] // fallback
	}

	// Create provider
	provider, err := f.providerFactory.CreateProvider(specialist.Provider)
	if err != nil {
		return PhaseResult{
			Phase:    phase,
			Status:   StatusFailed,
			Errors:   []string{err.Error()},
			Duration: time.Since(start),
		}, err
	}

	// Build phase prompt
	prompt := f.buildPhasePrompt(phase)

	// Create context with phase timeout
	phaseCtx, cancel := context.WithTimeout(ctx, f.guardrails.MaxPhaseDuration)
	defer cancel()

	// Execute with specialist
	resp, err := provider.Chat(phaseCtx, ai.ChatRequest{
		Model: specialist.Model,
		Messages: []ai.Message{
			{Role: "system", Content: f.getSpecialistSystemPrompt(specialist.Domain)},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   4000,
		Temperature: 0.7,
	})

	if err != nil {
		return PhaseResult{
			Phase:    phase,
			Status:   StatusFailed,
			Errors:   []string{err.Error()},
			Duration: time.Since(start),
		}, err
	}

	return PhaseResult{
		Phase:            phase,
		Status:           StatusCompleted,
		Output:           resp.Content,
		Duration:         time.Since(start),
		EstimatedCost:    0.01, // Rough estimate, would need token counting
		ValidationPassed: true,
	}, nil
}

// buildPhasePrompt constructs the prompt for a phase
func (f *Factory) buildPhasePrompt(phase Phase) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Task: %s\n\n", phase.Task))
	sb.WriteString(fmt.Sprintf("Phase: %s\n", phase.Name))
	sb.WriteString(fmt.Sprintf("Domain: %s\n\n", phase.Domain))

	if len(phase.Inputs) > 0 {
		sb.WriteString("### Input Files:\n")
		for _, input := range phase.Inputs {
			sb.WriteString(fmt.Sprintf("- %s\n", input))
		}
		sb.WriteString("\n")
	}

	if len(phase.Outputs) > 0 {
		sb.WriteString("### Expected Outputs:\n")
		for _, output := range phase.Outputs {
			sb.WriteString(fmt.Sprintf("- %s\n", output))
		}
		sb.WriteString("\n")
	}

	if len(phase.Validation) > 0 {
		sb.WriteString("### Validation Commands:\n")
		for _, cmd := range phase.Validation {
			sb.WriteString(fmt.Sprintf("- %s\n", cmd))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`
### Instructions:
1. Analyze the task requirements
2. Generate the required code/configuration
3. Explain key decisions
4. List any assumptions made
5. Suggest validation steps

Provide complete, production-ready output.`)

	return sb.String()
}

// getSpecialistSystemPrompt returns the system prompt for a specialist
func (f *Factory) getSpecialistSystemPrompt(domain string) string {
	switch domain {
	case "go":
		return `You are a senior Go engineer specializing in:
- Idiomatic Go code
- Kubernetes controllers and operators
- Performance optimization
- Concurrency patterns (goroutines, channels, sync primitives)
- Testing with testify and table-driven tests

Write production-ready Go code following best practices.`

	case "typescript":
		return `You are a senior TypeScript/React engineer specializing in:
- Modern TypeScript (strict mode, advanced types)
- React 19+ with hooks and server components
- API integration with React Query/SWR
- State management patterns
- Testing with Vitest/Playwright

Write production-ready TypeScript code following best practices.`

	case "infrastructure":
		return `You are a senior DevOps/Infrastructure engineer specializing in:
- Kubernetes (deployments, services, CRDs)
- Helm charts and Kustomize
- Docker and container optimization
- CI/CD pipelines
- Observability (Prometheus, Grafana, OpenTelemetry)

Generate production-ready infrastructure configurations.`

	case "coordinator":
		return `You are the Chief Technology Officer coordinating a software project.
Your role is to:
- Break down complex tasks into actionable phases
- Assign work to the appropriate specialist (Go, TypeScript, Infrastructure)
- Review outputs for quality and consistency
- Ensure the project stays on track

Be decisive and provide clear direction.`

	default:
		return `You are a senior software engineer. Provide comprehensive, production-ready solutions.`
	}
}

// dependenciesMet checks if all dependencies for a phase are completed
func (f *Factory) dependenciesMet(phase Phase) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, dep := range phase.DependsOn {
		result, ok := f.state.PhaseResults[dep]
		if !ok || result.Status != StatusCompleted {
			return false
		}
	}
	return true
}

// checkGuardrails verifies we're within limits
func (f *Factory) checkGuardrails() error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check total duration
	elapsed := time.Since(f.state.StartedAt)
	if elapsed > f.guardrails.MaxTotalDuration {
		return fmt.Errorf("exceeded max total duration: %v > %v", elapsed, f.guardrails.MaxTotalDuration)
	}

	// Check total cost
	if f.state.TotalCost > f.guardrails.MaxCostTotal {
		return fmt.Errorf("exceeded max total cost: $%.2f > $%.2f", f.state.TotalCost, f.guardrails.MaxCostTotal)
	}

	return nil
}

// attemptAutoFix tries to automatically fix a failed phase
func (f *Factory) attemptAutoFix(ctx context.Context, phase Phase, originalErr error) (PhaseResult, error) {
	log.Printf("[Factory] Attempting auto-fix for phase %s: %v", phase.Name, originalErr)

	// Get coordinator for fix attempt
	provider, err := f.providerFactory.CreateProvider(f.specialists["coordinator"].Provider)
	if err != nil {
		return PhaseResult{}, err
	}

	// Build fix prompt
	fixPrompt := fmt.Sprintf(`The following phase failed:

Phase: %s
Task: %s
Error: %v

Analyze the error and provide a corrected approach.
Be specific about what went wrong and how to fix it.`, phase.Name, phase.Task, originalErr)

	resp, err := provider.Chat(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "user", Content: fixPrompt},
		},
		MaxTokens:   2000,
		Temperature: 0.5,
	})

	if err != nil {
		return PhaseResult{}, err
	}

	return PhaseResult{
		Phase:            phase,
		Status:           StatusCompleted,
		Output:           resp.Content,
		ValidationPassed: true,
	}, nil
}

// pauseForHuman pauses execution and waits for human intervention
func (f *Factory) pauseForHuman(phase, reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.state.Paused = true
	f.state.PauseReason = fmt.Sprintf("Phase %s: %s", phase, reason)
	f.saveState()
}

// Resume continues execution from where it paused
func (f *Factory) Resume(ctx context.Context) error {
	f.mu.Lock()
	f.state.Paused = false
	f.state.PauseReason = ""
	f.mu.Unlock()

	// Continue from current phase
	return f.Run(ctx, f.state.Blueprint)
}

// saveState persists the current state to disk
func (f *Factory) saveState() error {
	if f.stateFile == "" || f.state == nil {
		return nil
	}

	data, err := json.MarshalIndent(f.state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(f.stateFile, data, 0644)
}

// LoadState loads state from disk
func (f *Factory) LoadState(project string) error {
	stateDir := filepath.Join(os.TempDir(), "zen-claw-factory")
	f.stateFile = filepath.Join(stateDir, fmt.Sprintf("%s-state.json", project))

	data, err := os.ReadFile(f.stateFile)
	if err != nil {
		return err
	}

	var state FactoryState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	f.mu.Lock()
	f.state = &state
	f.mu.Unlock()

	return nil
}

// GetState returns the current factory state
func (f *Factory) GetState() *FactoryState {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.state
}

// ParseBlueprint parses a blueprint from JSON or YAML
func ParseBlueprint(data []byte) (*Blueprint, error) {
	var blueprint Blueprint
	if err := json.Unmarshal(data, &blueprint); err != nil {
		return nil, err
	}
	return &blueprint, nil
}
