// Package judge provides LLM-based evaluation and selection of AI responses.
// Used to pick the best response from multiple candidates using an LLM as judge.
package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/ai"
)

// Response represents a candidate response to be judged
type Response struct {
	Provider   string                 `json:"provider"`
	Model      string                 `json:"model"`
	Content    string                 `json:"content"`
	Duration   time.Duration          `json:"duration,omitempty"`
	Confidence float64                `json:"confidence,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// CriteriaScore represents score for a specific criterion
type CriteriaScore struct {
	Criterion string             `json:"criterion"`
	Scores    map[string]float64 `json:"scores"`     // Provider -> score
	Winner    string             `json:"winner"`     // Best for this criterion
	Reasoning string             `json:"reasoning,omitempty"`
}

// Evaluation contains the judge's evaluation details
type Evaluation struct {
	WinnerScore float64            `json:"winner_score"` // 0.0-1.0
	Confidence  float64            `json:"confidence"`   // Judge's confidence
	Reasoning   string             `json:"reasoning"`    // Why this was chosen
	Scores      map[string]float64 `json:"scores"`       // Provider -> score
	Criteria    []CriteriaScore    `json:"criteria,omitempty"`
}

// Result contains the judgment result
type Result struct {
	Winner       Response   `json:"winner"`
	AllResponses []Response `json:"all_responses"`
	Evaluation   Evaluation `json:"evaluation"`
	Metadata     struct {
		JudgeProvider    string        `json:"judge_provider"`
		JudgeModel       string        `json:"judge_model"`
		JudgedAt         time.Time     `json:"judged_at"`
		ProcessingTimeMs int64         `json:"processing_time_ms"`
	} `json:"metadata"`
}

// Request represents a request for LLM judgment
type Request struct {
	Responses []Response `json:"responses"`
	Criteria  []string   `json:"criteria,omitempty"` // Custom evaluation criteria
	Context   string     `json:"context,omitempty"`  // Additional context
	Task      string     `json:"task,omitempty"`     // Task description
}

// Judge uses an LLM to evaluate and compare multiple responses
type Judge struct {
	provider      ai.Provider
	providerName  string
	model         string
}

// NewJudge creates a new LLM judge
func NewJudge(provider ai.Provider, providerName, model string) *Judge {
	if model == "" {
		model = "deepseek-chat" // Default judge model
	}
	return &Judge{
		provider:     provider,
		providerName: providerName,
		model:        model,
	}
}

// Judge evaluates multiple responses and selects the best one
func (j *Judge) Judge(ctx context.Context, req Request) (*Result, error) {
	start := time.Now()

	if len(req.Responses) == 0 {
		return nil, fmt.Errorf("no responses to judge")
	}

	// Only one response - return it directly
	if len(req.Responses) == 1 {
		return &Result{
			Winner:       req.Responses[0],
			AllResponses: req.Responses,
			Evaluation: Evaluation{
				WinnerScore: 1.0,
				Confidence:  1.0,
				Reasoning:   "Only one response provided",
				Scores: map[string]float64{
					req.Responses[0].Provider: 1.0,
				},
			},
			Metadata: struct {
				JudgeProvider    string    `json:"judge_provider"`
				JudgeModel       string    `json:"judge_model"`
				JudgedAt         time.Time `json:"judged_at"`
				ProcessingTimeMs int64     `json:"processing_time_ms"`
			}{
				JudgeProvider:    j.providerName,
				JudgeModel:       j.model,
				JudgedAt:         time.Now(),
				ProcessingTimeMs: time.Since(start).Milliseconds(),
			},
		}, nil
	}

	// Build judgment prompt
	prompt := j.buildJudgmentPrompt(req)

	// Call judge provider with JSON output request
	resp, err := j.provider.Chat(ctx, ai.ChatRequest{
		Model: j.model,
		Messages: []ai.Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   2000,
		Temperature: 0.3, // Lower temperature for consistent judgments
	})

	if err != nil {
		return nil, fmt.Errorf("judge call failed: %w", err)
	}

	// Parse judgment from response
	judgment, err := j.parseJudgment(resp.Content, req.Responses)
	if err != nil {
		log.Printf("[Judge] Warning: failed to parse judgment JSON, falling back to first response: %v", err)
		// Fallback to first response
		return &Result{
			Winner:       req.Responses[0],
			AllResponses: req.Responses,
			Evaluation: Evaluation{
				WinnerScore: 0.5,
				Confidence:  0.3,
				Reasoning:   fmt.Sprintf("Parsing failed: %v. Raw: %s", err, truncate(resp.Content, 200)),
				Scores:      make(map[string]float64),
			},
			Metadata: struct {
				JudgeProvider    string    `json:"judge_provider"`
				JudgeModel       string    `json:"judge_model"`
				JudgedAt         time.Time `json:"judged_at"`
				ProcessingTimeMs int64     `json:"processing_time_ms"`
			}{
				JudgeProvider:    j.providerName,
				JudgeModel:       j.model,
				JudgedAt:         time.Now(),
				ProcessingTimeMs: time.Since(start).Milliseconds(),
			},
		}, nil
	}

	// Find winner response
	winner := req.Responses[0] // Default
	for _, resp := range req.Responses {
		if resp.Provider == judgment.Winner {
			winner = resp
			break
		}
	}

	result := &Result{
		Winner:       winner,
		AllResponses: req.Responses,
		Evaluation: Evaluation{
			WinnerScore: judgment.Scores[judgment.Winner],
			Confidence:  judgment.Confidence,
			Reasoning:   judgment.Reasoning,
			Scores:      judgment.Scores,
			Criteria:    judgment.CriteriaScores,
		},
		Metadata: struct {
			JudgeProvider    string    `json:"judge_provider"`
			JudgeModel       string    `json:"judge_model"`
			JudgedAt         time.Time `json:"judged_at"`
			ProcessingTimeMs int64     `json:"processing_time_ms"`
		}{
			JudgeProvider:    j.providerName,
			JudgeModel:       j.model,
			JudgedAt:         time.Now(),
			ProcessingTimeMs: time.Since(start).Milliseconds(),
		},
	}

	log.Printf("[Judge] Selected %s (score=%.2f, confidence=%.2f) in %v",
		winner.Provider, judgment.Scores[judgment.Winner], judgment.Confidence, time.Since(start))

	return result, nil
}

// parsedJudgment represents the parsed JSON from judge response
type parsedJudgment struct {
	Winner         string             `json:"winner"`
	Confidence     float64            `json:"confidence"`
	Reasoning      string             `json:"reasoning"`
	Scores         map[string]float64 `json:"scores"`
	CriteriaScores []CriteriaScore    `json:"criteria_scores,omitempty"`
}

// parseJudgment extracts the judgment from the LLM response
func (j *Judge) parseJudgment(content string, responses []Response) (*parsedJudgment, error) {
	// Find JSON block in response
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart < 0 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := content[jsonStart : jsonEnd+1]

	var judgment parsedJudgment
	if err := json.Unmarshal([]byte(jsonStr), &judgment); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	// Validate winner
	if judgment.Winner == "" {
		return nil, fmt.Errorf("judgment missing winner")
	}

	// Normalize confidence
	if judgment.Confidence < 0 || judgment.Confidence > 1 {
		judgment.Confidence = 0.5
	}

	// Initialize scores if nil
	if judgment.Scores == nil {
		judgment.Scores = make(map[string]float64)
	}

	return &judgment, nil
}

// buildJudgmentPrompt constructs the prompt for the judge LLM
func (j *Judge) buildJudgmentPrompt(req Request) string {
	var prompt strings.Builder

	prompt.WriteString("You are an AI judge evaluating multiple responses to select the best one.\n\n")

	if req.Task != "" {
		prompt.WriteString(fmt.Sprintf("TASK: %s\n\n", req.Task))
	}

	if req.Context != "" {
		prompt.WriteString(fmt.Sprintf("CONTEXT: %s\n\n", req.Context))
	}

	// Evaluation criteria
	prompt.WriteString("EVALUATION CRITERIA:\n")
	if len(req.Criteria) > 0 {
		for i, criterion := range req.Criteria {
			prompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, criterion))
		}
	} else {
		// Default criteria
		prompt.WriteString("1. Accuracy and correctness\n")
		prompt.WriteString("2. Completeness of response\n")
		prompt.WriteString("3. Clarity and coherence\n")
		prompt.WriteString("4. Relevance to the task\n")
		prompt.WriteString("5. Practicality and actionability\n")
	}
	prompt.WriteString("\n")

	// Responses to evaluate
	prompt.WriteString("RESPONSES TO EVALUATE:\n\n")
	for i, resp := range req.Responses {
		providerID := resp.Provider
		if resp.Model != "" {
			providerID = fmt.Sprintf("%s/%s", resp.Provider, resp.Model)
		}
		prompt.WriteString(fmt.Sprintf("=== RESPONSE %d: %s ===\n", i+1, providerID))
		prompt.WriteString(resp.Content)
		prompt.WriteString("\n\n")
	}

	// Instructions for output
	prompt.WriteString(`INSTRUCTIONS:
1. Evaluate each response against the criteria
2. Assign scores (0.0-1.0) to each provider
3. Select the winner with the highest overall quality
4. Provide clear reasoning for your decision
5. Indicate your confidence level (0.0-1.0)

OUTPUT FORMAT (JSON only, no other text):
{
  "winner": "<provider_name>",
  "confidence": 0.85,
  "reasoning": "Brief explanation of why this response was selected",
  "scores": {
`)

	// Add provider names to template
	for i, resp := range req.Responses {
		comma := ","
		if i == len(req.Responses)-1 {
			comma = ""
		}
		prompt.WriteString(fmt.Sprintf("    \"%s\": 0.0%s\n", resp.Provider, comma))
	}

	prompt.WriteString(`  }
}
`)

	return prompt.String()
}

// QuickJudge is a convenience function for simple judging
func (j *Judge) QuickJudge(ctx context.Context, responses []Response, task string) (*Result, error) {
	return j.Judge(ctx, Request{
		Responses: responses,
		Task:      task,
	})
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
