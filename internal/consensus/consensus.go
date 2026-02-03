// Package consensus implements multi-model AI consensus for better blueprint generation.
// This sends requests to 3+ AIs in parallel, then uses an arbiter to synthesize
// the best ideas from each into a superior result.
package consensus

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
	"github.com/neves/zen-claw/internal/providers"
)

// Worker represents a single AI worker in the consensus pool
type Worker struct {
	Provider string // e.g., "deepseek", "qwen", "minimax"
	Model    string // specific model
	Role     string // e.g., "systems_thinker", "implementation_realist", "edge_case_hunter"
}

// WorkerResult holds a worker's response
type WorkerResult struct {
	Worker   Worker
	Response string
	Duration time.Duration
	Error    error
}

// ConsensusRequest represents a request for consensus
type ConsensusRequest struct {
	Prompt      string   // The original request/idea
	Domain      string   // e.g., "architecture", "api_design", "database_schema", "code_review"
	Workers     []Worker // Optional custom workers (defaults used if empty)
	MaxTokens   int      // Max tokens per worker response (default 4000)
	Temperature float64  // Temperature for worker responses (default 0.7)
}

// ConsensusResult holds the synthesized result
type ConsensusResult struct {
	Blueprint       string         // The synthesized blueprint
	WorkerResults   []WorkerResult // Individual worker responses
	ArbiterModel    string         // Which model arbitrated
	TotalDuration   time.Duration  // Total processing time
	WorkerDuration  time.Duration  // Time for parallel worker calls
	ArbiterDuration time.Duration  // Time for arbiter synthesis
}

// Engine manages multi-model consensus
type Engine struct {
	cfg     *config.Config
	factory *providers.Factory
}

// NewEngine creates a new consensus engine
func NewEngine(cfg *config.Config) *Engine {
	return &Engine{
		cfg:     cfg,
		factory: providers.NewFactory(cfg),
	}
}

// DefaultWorkers returns the default 3-worker configuration
// Using diverse models for better consensus:
// - DeepSeek: Fast, cost-effective, good systems thinking
// - Qwen: Large context, coding-focused
// - MiniMax: Different perspective, 1M context capability
func (e *Engine) DefaultWorkers() []Worker {
	return []Worker{
		{Provider: "deepseek", Model: "deepseek-chat", Role: "systems_thinker"},
		{Provider: "qwen", Model: "qwen3-coder-30b", Role: "implementation_realist"},
		{Provider: "minimax", Model: "minimax-M2.1", Role: "edge_case_hunter"},
	}
}

// GetAvailableWorkers returns workers for which we have API keys
func (e *Engine) GetAvailableWorkers() []Worker {
	available := e.factory.ListAvailableProviders()
	availableMap := make(map[string]bool)
	for _, p := range available {
		availableMap[p] = true
	}

	var workers []Worker
	for _, w := range e.DefaultWorkers() {
		if availableMap[w.Provider] {
			workers = append(workers, w)
		}
	}
	return workers
}

// Generate runs the consensus process
func (e *Engine) Generate(ctx context.Context, req ConsensusRequest) (*ConsensusResult, error) {
	start := time.Now()

	// Use default workers if not specified
	workers := req.Workers
	if len(workers) == 0 {
		workers = e.GetAvailableWorkers()
	}

	if len(workers) < 2 {
		return nil, fmt.Errorf("consensus requires at least 2 workers, got %d (check API keys)", len(workers))
	}

	log.Printf("[Consensus] Starting with %d workers for domain: %s", len(workers), req.Domain)

	// Set defaults
	if req.MaxTokens == 0 {
		req.MaxTokens = 4000
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	// Build worker prompt based on domain
	workerPrompt := e.buildWorkerPrompt(req)

	// Phase 1: Parallel worker calls
	workerStart := time.Now()
	results := e.callWorkersParallel(ctx, workers, workerPrompt, req.MaxTokens, req.Temperature)
	workerDuration := time.Since(workerStart)

	// Check if we got enough valid responses
	validCount := 0
	for _, r := range results {
		if r.Error == nil && r.Response != "" {
			validCount++
		}
	}

	if validCount < 2 {
		// Return partial results with error
		return &ConsensusResult{
			Blueprint:      "",
			WorkerResults:  results,
			TotalDuration:  time.Since(start),
			WorkerDuration: workerDuration,
		}, fmt.Errorf("only %d valid worker responses (need at least 2)", validCount)
	}

	// Phase 2: Arbiter synthesis
	arbiterStart := time.Now()
	blueprint, arbiterModel, err := e.synthesizeWithArbiter(ctx, req, results)
	arbiterDuration := time.Since(arbiterStart)

	if err != nil {
		return &ConsensusResult{
			Blueprint:       "",
			WorkerResults:   results,
			TotalDuration:   time.Since(start),
			WorkerDuration:  workerDuration,
			ArbiterDuration: arbiterDuration,
		}, fmt.Errorf("arbiter synthesis failed: %w", err)
	}

	log.Printf("[Consensus] Complete: %d workers in %v, arbiter in %v, total %v",
		len(workers), workerDuration.Round(time.Millisecond),
		arbiterDuration.Round(time.Millisecond), time.Since(start).Round(time.Millisecond))

	return &ConsensusResult{
		Blueprint:       blueprint,
		WorkerResults:   results,
		ArbiterModel:    arbiterModel,
		TotalDuration:   time.Since(start),
		WorkerDuration:  workerDuration,
		ArbiterDuration: arbiterDuration,
	}, nil
}

// buildWorkerPrompt constructs the prompt for workers based on domain
func (e *Engine) buildWorkerPrompt(req ConsensusRequest) string {
	domainContext := ""
	switch req.Domain {
	case "architecture":
		domainContext = `You are an expert software architect. Focus on:
- System structure and component organization
- Data flow and dependencies
- Scalability and maintainability patterns
- Trade-offs and decision rationale`
	case "api_design":
		domainContext = `You are an expert API designer. Focus on:
- RESTful principles or appropriate protocol choice
- Endpoint structure and naming conventions
- Request/response schemas
- Error handling and versioning`
	case "database_schema":
		domainContext = `You are an expert database designer. Focus on:
- Schema normalization and denormalization trade-offs
- Indexing strategy
- Query patterns and performance
- Data integrity constraints`
	case "code_review":
		domainContext = `You are an expert code reviewer. Focus on:
- Code correctness and bug detection
- Performance and efficiency
- Maintainability and readability
- Best practices and idiomatic patterns`
	default:
		domainContext = `You are a senior software engineer. Provide comprehensive technical analysis.`
	}

	return fmt.Sprintf(`%s

Analyze the following request and provide your detailed technical recommendation:

---
%s
---

Be specific, actionable, and thorough. Include code examples where helpful.
Format your response as a clear technical specification.`, domainContext, req.Prompt)
}

// callWorkersParallel calls all workers in parallel
func (e *Engine) callWorkersParallel(ctx context.Context, workers []Worker, prompt string, maxTokens int, temperature float64) []WorkerResult {
	results := make([]WorkerResult, len(workers))
	var wg sync.WaitGroup

	for i, worker := range workers {
		wg.Add(1)
		go func(idx int, w Worker) {
			defer wg.Done()
			start := time.Now()

			// Create provider
			provider, err := e.factory.CreateProvider(w.Provider)
			if err != nil {
				results[idx] = WorkerResult{
					Worker:   w,
					Error:    fmt.Errorf("create provider: %w", err),
					Duration: time.Since(start),
				}
				return
			}

			// Build request with role context
			rolePrompt := fmt.Sprintf("[Your role: %s]\n\n%s", w.Role, prompt)

			// Make AI call
			resp, err := provider.Chat(ctx, ai.ChatRequest{
				Model: w.Model,
				Messages: []ai.Message{
					{Role: "user", Content: rolePrompt},
				},
				MaxTokens:   maxTokens,
				Temperature: temperature,
			})

			if err != nil {
				results[idx] = WorkerResult{
					Worker:   w,
					Error:    err,
					Duration: time.Since(start),
				}
				log.Printf("[Consensus] Worker %s/%s failed: %v", w.Provider, w.Model, err)
				return
			}

			results[idx] = WorkerResult{
				Worker:   w,
				Response: resp.Content,
				Duration: time.Since(start),
			}
			log.Printf("[Consensus] Worker %s/%s completed in %v (%d chars)",
				w.Provider, w.Model, time.Since(start).Round(time.Millisecond), len(resp.Content))
		}(i, worker)
	}

	wg.Wait()
	return results
}

// synthesizeWithArbiter uses an arbiter model to synthesize worker responses
func (e *Engine) synthesizeWithArbiter(ctx context.Context, req ConsensusRequest, results []WorkerResult) (string, string, error) {
	// Choose arbiter: prefer Kimi (best for synthesis), then Qwen, then DeepSeek
	arbiterOrder := []string{"kimi", "qwen", "deepseek"}
	var arbiter ai.Provider
	var arbiterName string

	for _, name := range arbiterOrder {
		p, err := e.factory.CreateProvider(name)
		if err == nil {
			arbiter = p
			arbiterName = name
			break
		}
	}

	if arbiter == nil {
		return "", "", fmt.Errorf("no arbiter provider available (tried: %v)", arbiterOrder)
	}

	// Build arbiter prompt
	var workerResponses strings.Builder
	for _, r := range results {
		if r.Error != nil || r.Response == "" {
			continue
		}
		workerResponses.WriteString(fmt.Sprintf("\n## [%s - %s]\n%s\n",
			strings.ToUpper(r.Worker.Provider), r.Worker.Role, r.Response))
	}

	arbiterPrompt := fmt.Sprintf(`You are the Chief Technical Officer synthesizing proposals from your engineering team.

ORIGINAL REQUEST:
%s

DOMAIN: %s

TEAM PROPOSALS:
%s

YOUR TASK:
Synthesize ONE comprehensive technical specification that:
1. Takes the best structural insights from each proposal
2. Uses the most practical implementation details
3. Addresses all flagged risks and edge cases
4. Resolves any contradictions explicitly with clear rationale
5. Produces a unified, actionable blueprint

OUTPUT FORMAT:
- Start with a 2-3 sentence executive summary
- Provide clear sections with headers
- Include specific technical details and code examples
- End with "Next Steps" actionable items

Be decisive. This blueprint will be implemented.`,
		req.Prompt, req.Domain, workerResponses.String())

	log.Printf("[Consensus] Arbiter %s synthesizing %d responses...", arbiterName, len(results))

	// Call arbiter with generous token limit
	resp, err := arbiter.Chat(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "user", Content: arbiterPrompt},
		},
		MaxTokens:   8000, // Arbiter needs more space for synthesis
		Temperature: 0.5,  // Lower temp for more focused synthesis
	})

	if err != nil {
		return "", arbiterName, fmt.Errorf("arbiter call failed: %w", err)
	}

	return resp.Content, arbiterName, nil
}

// QuickConsensus is a convenience method for simple consensus requests
func (e *Engine) QuickConsensus(ctx context.Context, prompt, domain string) (*ConsensusResult, error) {
	return e.Generate(ctx, ConsensusRequest{
		Prompt: prompt,
		Domain: domain,
	})
}
