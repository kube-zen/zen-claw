// Package consensus implements multi-model AI consensus for better blueprint generation.
// All workers receive the SAME prompt with the SAME role.
// The arbiter gets the original question + all worker outputs, scores each worker,
// and synthesizes the best ideas into a unified blueprint.
//
// Optionally, an LLM Judge can be used to evaluate worker responses before synthesis,
// providing more nuanced selection based on configurable criteria.
package consensus

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
	"github.com/neves/zen-claw/internal/judge"
	"github.com/neves/zen-claw/internal/providers"
)

// Worker represents a single AI worker in the consensus pool
type Worker struct {
	Provider string // e.g., "deepseek", "qwen", "minimax"
	Model    string // specific model
}

// WorkerResult holds a worker's response
type WorkerResult struct {
	Worker   Worker
	Response string
	Duration time.Duration
	Error    error
	Score    int    // 1-10 score assigned by arbiter
	Feedback string // Brief feedback from arbiter
}

// ConsensusRequest represents a request for consensus
type ConsensusRequest struct {
	Prompt      string   // The original request/idea
	Role        string   // The expert role for ALL workers AND arbiter (e.g., "security_architect")
	Workers     []Worker // Optional custom workers (defaults used if empty)
	MaxTokens   int      // Max tokens per worker response (default 4000)
	Temperature float64  // Temperature for worker responses (default 0.7)
	UseJudge    bool     // Use LLM judge to evaluate responses before synthesis
	JudgeCriteria []string // Custom criteria for judge evaluation (optional)
}

// ConsensusResult holds the synthesized result
type ConsensusResult struct {
	Blueprint       string         // The synthesized blueprint
	WorkerResults   []WorkerResult // Individual worker responses with scores
	ArbiterModel    string         // Which model arbitrated
	TotalDuration   time.Duration  // Total processing time
	WorkerDuration  time.Duration  // Time for parallel worker calls
	ArbiterDuration time.Duration  // Time for arbiter synthesis
	Role            string         // The role used for this consensus
	JudgeResult     *judge.Result  // Optional: LLM judge evaluation result
}

// WorkerStats tracks long-term worker performance
type WorkerStats struct {
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	TotalTasks int       `json:"total_tasks"`
	TotalScore int       `json:"total_score"`
	AvgScore   float64   `json:"avg_score"`
	LastUsed   time.Time `json:"last_used"`
	BestRoles  []string  `json:"best_roles"` // Roles where this worker excelled
}

// Engine manages multi-model consensus
type Engine struct {
	cfg       *config.Config
	factory   *providers.Factory
	statsFile string
	stats     map[string]*WorkerStats // provider/model -> stats
	statsMu   sync.RWMutex
	useJudge  bool // Whether to use LLM judge for response evaluation
}

// NewEngine creates a new consensus engine
func NewEngine(cfg *config.Config) *Engine {
	e := &Engine{
		cfg:       cfg,
		factory:   providers.NewFactory(cfg),
		statsFile: filepath.Join(os.TempDir(), "zen-claw-consensus-stats.json"),
		stats:     make(map[string]*WorkerStats),
		useJudge:  false, // Disabled by default
	}
	e.loadStats()
	return e
}

// EnableJudge enables LLM judge for response evaluation
func (e *Engine) EnableJudge(enable bool) {
	e.useJudge = enable
}

// IsJudgeEnabled returns whether LLM judge is enabled
func (e *Engine) IsJudgeEnabled() bool {
	return e.useJudge
}

// DefaultWorkers returns the worker configuration from config (without roles - role is per-request)
func (e *Engine) DefaultWorkers() []Worker {
	workers := e.cfg.GetConsensusWorkers()
	result := make([]Worker, len(workers))
	for i, w := range workers {
		result[i] = Worker{
			Provider: w.Provider,
			Model:    w.Model,
		}
	}
	return result
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

	// Default role if not specified
	if req.Role == "" {
		req.Role = "senior_software_engineer"
	}

	log.Printf("[Consensus] Starting with %d workers, role: %s", len(workers), req.Role)

	// Set defaults
	if req.MaxTokens == 0 {
		req.MaxTokens = 4000
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	// Build worker prompt - ALL workers get the SAME prompt with the SAME role
	workerPrompt := e.buildWorkerPrompt(req)

	// Phase 1: Parallel worker calls (all with same role and prompt)
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
		return &ConsensusResult{
			Blueprint:      "",
			WorkerResults:  results,
			TotalDuration:  time.Since(start),
			WorkerDuration: workerDuration,
			Role:           req.Role,
		}, fmt.Errorf("only %d valid worker responses (need at least 2)", validCount)
	}

	// Optional Phase 1.5: LLM Judge evaluation
	var judgeResult *judge.Result
	if req.UseJudge || e.useJudge {
		judgeResult = e.evaluateWithJudge(ctx, req, results)
		if judgeResult != nil {
			log.Printf("[Consensus] Judge selected %s (confidence=%.2f): %s",
				judgeResult.Winner.Provider, judgeResult.Evaluation.Confidence,
				truncateString(judgeResult.Evaluation.Reasoning, 100))
		}
	}

	// Phase 2: Arbiter synthesis with CLEAN CONTEXT (no history, just the task)
	// Arbiter gets: original question + all worker outputs
	// Arbiter also scores each worker (optionally informed by judge)
	arbiterStart := time.Now()
	blueprint, arbiterModel, scoredResults, err := e.synthesizeWithArbiter(ctx, req, results, judgeResult)
	arbiterDuration := time.Since(arbiterStart)

	if err != nil {
		return &ConsensusResult{
			Blueprint:       "",
			WorkerResults:   results,
			TotalDuration:   time.Since(start),
			WorkerDuration:  workerDuration,
			ArbiterDuration: arbiterDuration,
			Role:            req.Role,
			JudgeResult:     judgeResult,
		}, fmt.Errorf("arbiter synthesis failed: %w", err)
	}

	// Update worker stats with scores
	e.updateWorkerStats(scoredResults, req.Role)

	log.Printf("[Consensus] Complete: %d workers in %v, arbiter in %v, total %v",
		len(workers), workerDuration.Round(time.Millisecond),
		arbiterDuration.Round(time.Millisecond), time.Since(start).Round(time.Millisecond))

	return &ConsensusResult{
		Blueprint:       blueprint,
		WorkerResults:   scoredResults,
		ArbiterModel:    arbiterModel,
		TotalDuration:   time.Since(start),
		WorkerDuration:  workerDuration,
		ArbiterDuration: arbiterDuration,
		Role:            req.Role,
		JudgeResult:     judgeResult,
	}, nil
}

// evaluateWithJudge uses an LLM judge to evaluate worker responses
func (e *Engine) evaluateWithJudge(ctx context.Context, req ConsensusRequest, results []WorkerResult) *judge.Result {
	// Convert worker results to judge responses
	var responses []judge.Response
	for _, r := range results {
		if r.Error != nil || r.Response == "" {
			continue
		}
		responses = append(responses, judge.Response{
			Provider: r.Worker.Provider,
			Model:    r.Worker.Model,
			Content:  r.Response,
			Duration: r.Duration,
		})
	}

	if len(responses) < 2 {
		return nil
	}

	// Get judge provider (use arbiter order)
	arbiterOrder := e.cfg.GetArbiterOrder()
	var judgeProvider ai.Provider
	var judgeName string

	for _, name := range arbiterOrder {
		p, err := e.factory.CreateProvider(name)
		if err == nil {
			judgeProvider = p
			judgeName = name
			break
		}
	}

	if judgeProvider == nil {
		log.Printf("[Consensus] No judge provider available")
		return nil
	}

	// Create judge
	j := judge.NewJudge(judgeProvider, judgeName, e.cfg.GetModel(judgeName))

	// Run judgment
	judgeReq := judge.Request{
		Responses: responses,
		Task:      req.Prompt,
		Context:   fmt.Sprintf("Role: %s", req.Role),
		Criteria:  req.JudgeCriteria,
	}

	result, err := j.Judge(ctx, judgeReq)
	if err != nil {
		log.Printf("[Consensus] Judge evaluation failed: %v", err)
		return nil
	}

	return result
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// buildWorkerPrompt constructs the prompt - same for ALL workers
func (e *Engine) buildWorkerPrompt(req ConsensusRequest) string {
	roleDescription := getRoleDescription(req.Role)

	return fmt.Sprintf(`You are a %s.

%s

TASK:
%s

INSTRUCTIONS:
1. Analyze this request thoroughly from your expert perspective
2. Provide a detailed, actionable technical recommendation
3. Include specific implementation details and code examples where helpful
4. Identify potential risks, edge cases, and mitigation strategies
5. Be decisive and specific - your recommendation will be reviewed

Format your response as a clear technical specification.`, req.Role, roleDescription, req.Prompt)
}

// getRoleDescription returns a description for the given role
func getRoleDescription(role string) string {
	descriptions := map[string]string{
		"security_architect": `Your expertise includes:
- Zero-trust architecture and defense in depth
- Authentication, authorization, and access control
- Encryption, key management, and secure communications
- Threat modeling and vulnerability assessment
- Compliance frameworks (SOC2, GDPR, HIPAA)
- Secure coding practices and security testing`,

		"software_architect": `Your expertise includes:
- System design and component organization
- Scalability, reliability, and performance patterns
- Microservices and distributed systems
- Data flow and integration patterns
- Technology selection and trade-off analysis`,

		"api_designer": `Your expertise includes:
- RESTful API design and OpenAPI specifications
- gRPC and protocol buffer design
- API versioning and backward compatibility
- Error handling and response schemas
- Rate limiting and API security`,

		"database_architect": `Your expertise includes:
- Schema design and normalization
- Query optimization and indexing strategies
- Data modeling for different access patterns
- Replication, sharding, and consistency models
- Database selection (SQL vs NoSQL)`,

		"devops_engineer": `Your expertise includes:
- CI/CD pipeline design and automation
- Container orchestration (Kubernetes)
- Infrastructure as Code (Terraform, Helm)
- Monitoring, logging, and observability
- Deployment strategies and rollback procedures`,

		"frontend_architect": `Your expertise includes:
- Component architecture and state management
- Performance optimization and lazy loading
- Accessibility and responsive design
- Testing strategies (unit, integration, E2E)
- Build tooling and bundler configuration`,
	}

	if desc, ok := descriptions[role]; ok {
		return desc
	}

	// Default for unknown roles
	return fmt.Sprintf("You have deep expertise in %s. Apply your specialized knowledge to provide the best possible recommendation.", strings.ReplaceAll(role, "_", " "))
}

// callWorkersParallel calls all workers in parallel with the SAME prompt
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

			// ALL workers get the SAME prompt (no role prefix - role is in the prompt)
			// CLEAN CONTEXT: only the user message, no system message or history
			resp, err := provider.Chat(ctx, ai.ChatRequest{
				Model: w.Model,
				Messages: []ai.Message{
					{Role: "user", Content: prompt},
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

// synthesizeWithArbiter uses an arbiter model to synthesize and score worker responses
// CLEAN CONTEXT: arbiter gets only the original question + worker outputs, no history
// If judgeResult is provided, the arbiter is informed of the judge's evaluation
func (e *Engine) synthesizeWithArbiter(ctx context.Context, req ConsensusRequest, results []WorkerResult, judgeResult *judge.Result) (string, string, []WorkerResult, error) {
	// Choose arbiter based on config preference order
	arbiterOrder := e.cfg.GetArbiterOrder()
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
		return "", "", results, fmt.Errorf("no arbiter provider available (tried: %v)", arbiterOrder)
	}

	// Build worker responses section
	var workerResponses strings.Builder
	workerIDs := []string{}
	for i, r := range results {
		if r.Error != nil || r.Response == "" {
			continue
		}
		workerID := fmt.Sprintf("WORKER_%d_%s", i+1, strings.ToUpper(r.Worker.Provider))
		workerIDs = append(workerIDs, workerID)
		workerResponses.WriteString(fmt.Sprintf("\n\n=== %s (%s) ===\n%s\n",
			workerID, r.Worker.Model, r.Response))
	}

	// Arbiter prompt - SAME ROLE as workers, CLEAN CONTEXT
	roleDescription := getRoleDescription(req.Role)
	
	// Include judge evaluation if available
	judgeSection := ""
	if judgeResult != nil {
		judgeSection = fmt.Sprintf(`
PRELIMINARY EVALUATION (from LLM Judge):
- Recommended winner: %s (score: %.2f, confidence: %.2f)
- Reasoning: %s
- All scores: %v

Consider this evaluation in your synthesis, but make your own independent assessment.

`, judgeResult.Winner.Provider, judgeResult.Evaluation.WinnerScore,
			judgeResult.Evaluation.Confidence, judgeResult.Evaluation.Reasoning,
			judgeResult.Evaluation.Scores)
	}

	arbiterPrompt := fmt.Sprintf(`You are a %s acting as the lead reviewer.

%s

ORIGINAL TASK (this was given to all team members):
%s
%s
TEAM RESPONSES:
%s

YOUR RESPONSIBILITIES:

1. SYNTHESIZE: Create ONE comprehensive technical specification that:
   - Takes the best insights from each response
   - Resolves any contradictions with clear rationale
   - Produces a unified, actionable blueprint
   - Maintains your expert perspective as a %s

2. SCORE EACH WORKER: At the END of your response, include a JSON block with scores:
   - Rate each worker 1-10 based on: accuracy, completeness, practicality, insight
   - Provide brief feedback (1 sentence) for each

OUTPUT FORMAT:

[Your synthesized blueprint here - be comprehensive and actionable]

---SCORES---
`+"```json\n"+`{
  "scores": [
%s
  ]
}
`+"```",
		req.Role, roleDescription, req.Prompt, judgeSection, workerResponses.String(), req.Role,
		buildScoreTemplate(workerIDs))

	log.Printf("[Consensus] Arbiter %s (role: %s) synthesizing %d responses...", arbiterName, req.Role, len(results))

	// Call arbiter with CLEAN CONTEXT - only this one message, no history
	resp, err := arbiter.Chat(ctx, ai.ChatRequest{
		Model: e.cfg.GetModel(arbiterName),
		Messages: []ai.Message{
			{Role: "user", Content: arbiterPrompt},
		},
		MaxTokens:   8000,
		Temperature: 0.5,
	})

	if err != nil {
		return "", arbiterName, results, fmt.Errorf("arbiter call failed: %w", err)
	}

	// Parse response and extract scores
	blueprint, scoredResults := parseArbiterResponse(resp.Content, results)

	return blueprint, arbiterName, scoredResults, nil
}

// buildScoreTemplate creates the JSON template for scores
func buildScoreTemplate(workerIDs []string) string {
	var lines []string
	for _, id := range workerIDs {
		lines = append(lines, fmt.Sprintf(`    {"worker": "%s", "score": 0, "feedback": ""}`, id))
	}
	return strings.Join(lines, ",\n")
}

// parseArbiterResponse extracts blueprint and scores from arbiter response
func parseArbiterResponse(response string, results []WorkerResult) (string, []WorkerResult) {
	// Split at ---SCORES--- marker
	parts := strings.Split(response, "---SCORES---")
	blueprint := strings.TrimSpace(parts[0])

	// Try to parse scores if present
	if len(parts) > 1 {
		scoresSection := parts[1]

		// Find JSON block
		jsonStart := strings.Index(scoresSection, "{")
		jsonEnd := strings.LastIndex(scoresSection, "}")

		if jsonStart >= 0 && jsonEnd > jsonStart {
			jsonStr := scoresSection[jsonStart : jsonEnd+1]

			var scoreData struct {
				Scores []struct {
					Worker   string `json:"worker"`
					Score    int    `json:"score"`
					Feedback string `json:"feedback"`
				} `json:"scores"`
			}

			if err := json.Unmarshal([]byte(jsonStr), &scoreData); err == nil {
				// Match scores to results
				for i := range results {
					workerID := fmt.Sprintf("WORKER_%d_%s", i+1, strings.ToUpper(results[i].Worker.Provider))
					for _, s := range scoreData.Scores {
						if strings.Contains(s.Worker, workerID) || strings.Contains(workerID, s.Worker) {
							results[i].Score = s.Score
							results[i].Feedback = s.Feedback
							break
						}
					}
				}
			} else {
				log.Printf("[Consensus] Failed to parse scores JSON: %v", err)
			}
		}
	}

	return blueprint, results
}

// updateWorkerStats updates long-term worker statistics
func (e *Engine) updateWorkerStats(results []WorkerResult, role string) {
	e.statsMu.Lock()
	defer e.statsMu.Unlock()

	for _, r := range results {
		if r.Error != nil || r.Score == 0 {
			continue
		}

		key := fmt.Sprintf("%s/%s", r.Worker.Provider, r.Worker.Model)
		stats, exists := e.stats[key]
		if !exists {
			stats = &WorkerStats{
				Provider:  r.Worker.Provider,
				Model:     r.Worker.Model,
				BestRoles: []string{},
			}
			e.stats[key] = stats
		}

		stats.TotalTasks++
		stats.TotalScore += r.Score
		stats.AvgScore = float64(stats.TotalScore) / float64(stats.TotalTasks)
		stats.LastUsed = time.Now()

		// Track best roles (score >= 8)
		if r.Score >= 8 && !contains(stats.BestRoles, role) {
			stats.BestRoles = append(stats.BestRoles, role)
		}
	}

	e.saveStats()
}

// GetWorkerStats returns current worker statistics
func (e *Engine) GetWorkerStats() map[string]*WorkerStats {
	e.statsMu.RLock()
	defer e.statsMu.RUnlock()

	// Return a copy
	result := make(map[string]*WorkerStats)
	for k, v := range e.stats {
		statsCopy := *v
		result[k] = &statsCopy
	}
	return result
}

// loadStats loads worker statistics from disk
func (e *Engine) loadStats() {
	data, err := os.ReadFile(e.statsFile)
	if err != nil {
		return
	}

	var stats map[string]*WorkerStats
	if err := json.Unmarshal(data, &stats); err == nil {
		e.stats = stats
	}
}

// saveStats saves worker statistics to disk
func (e *Engine) saveStats() {
	data, err := json.MarshalIndent(e.stats, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(e.statsFile, data, 0644)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// QuickConsensus is a convenience method for simple consensus requests
func (e *Engine) QuickConsensus(ctx context.Context, prompt, role string) (*ConsensusResult, error) {
	return e.Generate(ctx, ConsensusRequest{
		Prompt: prompt,
		Role:   role,
	})
}
