package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SubagentStatus represents the status of a subagent run
type SubagentStatus string

const (
	SubagentPending   SubagentStatus = "pending"
	SubagentRunning   SubagentStatus = "running"
	SubagentCompleted SubagentStatus = "completed"
	SubagentFailed    SubagentStatus = "failed"
	SubagentCancelled SubagentStatus = "cancelled"
)

// SubagentRun represents a background agent run
type SubagentRun struct {
	ID           string
	Task         string
	Label        string
	Status       SubagentStatus
	Result       string
	Error        string
	StartTime    time.Time
	EndTime      *time.Time
	TokensUsed   int
	StepsUsed    int
	ParentID     string
	cancel       context.CancelFunc
	mu           sync.RWMutex
}

// SubagentManager manages background subagent runs
type SubagentManager struct {
	runs          map[string]*SubagentRun
	mu            sync.RWMutex
	nextID        int
	maxConcurrent int
	agentFactory  SubagentFactory
}

// SubagentFactory creates agents for subagent runs
type SubagentFactory func() (*Agent, error)

// NewSubagentManager creates a new subagent manager
func NewSubagentManager(maxConcurrent int, factory SubagentFactory) *SubagentManager {
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}
	return &SubagentManager{
		runs:          make(map[string]*SubagentRun),
		maxConcurrent: maxConcurrent,
		agentFactory:  factory,
	}
}

// Spawn starts a new subagent run
func (m *SubagentManager) Spawn(ctx context.Context, task, label, parentID string) (*SubagentRun, error) {
	m.mu.Lock()

	// Check concurrent limit
	running := 0
	for _, r := range m.runs {
		if r.Status == SubagentRunning {
			running++
		}
	}
	if running >= m.maxConcurrent {
		m.mu.Unlock()
		return nil, fmt.Errorf("max concurrent subagents reached (%d)", m.maxConcurrent)
	}

	m.nextID++
	id := fmt.Sprintf("sub-%d", m.nextID)

	runCtx, cancel := context.WithCancel(context.Background())

	run := &SubagentRun{
		ID:        id,
		Task:      task,
		Label:     label,
		Status:    SubagentPending,
		StartTime: time.Now(),
		ParentID:  parentID,
		cancel:    cancel,
	}

	m.runs[id] = run
	m.mu.Unlock()

	// Start the run in background
	go m.executeRun(runCtx, run)

	return run, nil
}

// executeRun runs the subagent task
func (m *SubagentManager) executeRun(ctx context.Context, run *SubagentRun) {
	run.mu.Lock()
	run.Status = SubagentRunning
	run.mu.Unlock()

	// Create agent for this run
	agent, err := m.agentFactory()
	if err != nil {
		run.mu.Lock()
		run.Status = SubagentFailed
		run.Error = fmt.Sprintf("failed to create agent: %v", err)
		now := time.Now()
		run.EndTime = &now
		run.mu.Unlock()
		return
	}

	// Create a session for this subagent run
	session := NewSession(run.ID)

	// Run the task
	session, result, err := agent.Run(ctx, session, run.Task)

	run.mu.Lock()
	defer run.mu.Unlock()

	now := time.Now()
	run.EndTime = &now

	if ctx.Err() == context.Canceled {
		run.Status = SubagentCancelled
		run.Error = "cancelled"
		return
	}

	if err != nil {
		run.Status = SubagentFailed
		run.Error = err.Error()
		return
	}

	run.Status = SubagentCompleted
	run.Result = result

	// Get stats from subagent session
	if session != nil {
		stats := session.GetStats()
		run.StepsUsed = stats.MessageCount
	}
}

// Get returns a subagent run by ID
func (m *SubagentManager) Get(id string) *SubagentRun {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runs[id]
}

// List returns all subagent runs, optionally filtered by parent
func (m *SubagentManager) List(parentID string) []*SubagentRun {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*SubagentRun
	for _, r := range m.runs {
		if parentID == "" || r.ParentID == parentID {
			result = append(result, r)
		}
	}
	return result
}

// Stop cancels a running subagent
func (m *SubagentManager) Stop(id string) error {
	m.mu.RLock()
	run := m.runs[id]
	m.mu.RUnlock()

	if run == nil {
		return fmt.Errorf("subagent not found: %s", id)
	}

	run.mu.Lock()
	defer run.mu.Unlock()

	if run.Status != SubagentRunning && run.Status != SubagentPending {
		return fmt.Errorf("subagent not running: %s", run.Status)
	}

	if run.cancel != nil {
		run.cancel()
	}
	return nil
}

// Remove removes a completed/failed subagent from the list
func (m *SubagentManager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	run, exists := m.runs[id]
	if !exists {
		return fmt.Errorf("subagent not found: %s", id)
	}

	if run.Status == SubagentRunning || run.Status == SubagentPending {
		return fmt.Errorf("cannot remove running subagent")
	}

	delete(m.runs, id)
	return nil
}

// GetInfo returns info about a run (thread-safe)
func (r *SubagentRun) GetInfo() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info := map[string]interface{}{
		"id":         r.ID,
		"task":       truncateSubagentTask(r.Task, 100),
		"status":     string(r.Status),
		"start_time": r.StartTime.Format(time.RFC3339),
	}

	if r.Label != "" {
		info["label"] = r.Label
	}
	if r.EndTime != nil {
		info["end_time"] = r.EndTime.Format(time.RFC3339)
		info["duration_ms"] = r.EndTime.Sub(r.StartTime).Milliseconds()
	}
	if r.Error != "" {
		info["error"] = r.Error
	}
	if r.TokensUsed > 0 {
		info["tokens_used"] = r.TokensUsed
	}
	if r.StepsUsed > 0 {
		info["steps_used"] = r.StepsUsed
	}

	return info
}

// GetResult returns the result (thread-safe)
func (r *SubagentRun) GetResult() (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.Status == SubagentRunning || r.Status == SubagentPending {
		return "", fmt.Errorf("subagent still running")
	}
	if r.Status == SubagentFailed {
		return "", fmt.Errorf("subagent failed: %s", r.Error)
	}
	if r.Status == SubagentCancelled {
		return "", fmt.Errorf("subagent was cancelled")
	}
	return r.Result, nil
}

func truncateSubagentTask(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// SubagentTool allows spawning parallel background agent runs
type SubagentTool struct {
	BaseTool
	manager  *SubagentManager
	parentID string
}

// NewSubagentTool creates a subagent tool
func NewSubagentTool(manager *SubagentManager, parentID string) *SubagentTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action: 'spawn', 'list', 'status', 'result', 'stop', 'remove'",
			},
			"task": map[string]interface{}{
				"type":        "string",
				"description": "Task for the subagent to perform (for 'spawn')",
			},
			"label": map[string]interface{}{
				"type":        "string",
				"description": "Optional label for the subagent run",
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Subagent ID (for status/result/stop/remove)",
			},
			"wait": map[string]interface{}{
				"type":        "boolean",
				"description": "Wait for completion (for 'spawn', default false)",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds when waiting (default 300)",
			},
		},
		"required": []string{"action"},
	}

	return &SubagentTool{
		BaseTool: NewBaseTool(
			"subagent",
			"Spawn and manage parallel background agent runs. Use for long tasks, research, or parallel work.",
			params,
		),
		manager:  manager,
		parentID: parentID,
	}
}

func (t *SubagentTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return nil, fmt.Errorf("action parameter is required")
	}

	if t.manager == nil {
		return map[string]interface{}{
			"error":   "subagent manager not available",
			"success": false,
		}, nil
	}

	switch action {
	case "spawn":
		return t.spawn(ctx, args)
	case "list":
		return t.list()
	case "status":
		return t.status(args)
	case "result":
		return t.result(args)
	case "stop":
		return t.stop(args)
	case "remove":
		return t.remove(args)
	default:
		return map[string]interface{}{
			"error":   fmt.Sprintf("unknown action: %s", action),
			"success": false,
		}, nil
	}
}

func (t *SubagentTool) spawn(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	task, ok := args["task"].(string)
	if !ok || task == "" {
		return map[string]interface{}{
			"error":   "task is required for spawn action",
			"success": false,
		}, nil
	}

	label := ""
	if l, ok := args["label"].(string); ok {
		label = l
	}

	run, err := t.manager.Spawn(ctx, task, label, t.parentID)
	if err != nil {
		return map[string]interface{}{
			"error":   err.Error(),
			"success": false,
		}, nil
	}

	// Check if we should wait
	wait := false
	if w, ok := args["wait"].(bool); ok {
		wait = w
	}

	if !wait {
		return map[string]interface{}{
			"id":      run.ID,
			"label":   label,
			"status":  "spawned",
			"message": "Subagent started. Use 'status' or 'result' to check progress.",
			"success": true,
		}, nil
	}

	// Wait for completion
	timeout := 300 * time.Second
	if to, ok := args["timeout_seconds"].(float64); ok {
		timeout = time.Duration(to) * time.Second
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return map[string]interface{}{
				"id":      run.ID,
				"status":  "timeout",
				"error":   "timed out waiting for subagent",
				"success": false,
			}, nil
		case <-ticker.C:
			info := run.GetInfo()
			status := info["status"].(string)
			if status == "completed" || status == "failed" || status == "cancelled" {
				result, _ := run.GetResult()
				return map[string]interface{}{
					"id":      run.ID,
					"status":  status,
					"result":  result,
					"info":    info,
					"success": status == "completed",
				}, nil
			}
		}
	}
}

func (t *SubagentTool) list() (interface{}, error) {
	runs := t.manager.List(t.parentID)

	var results []map[string]interface{}
	for _, r := range runs {
		results = append(results, r.GetInfo())
	}

	return map[string]interface{}{
		"subagents": results,
		"count":     len(results),
		"success":   true,
	}, nil
}

func (t *SubagentTool) status(args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return map[string]interface{}{
			"error":   "id is required for status action",
			"success": false,
		}, nil
	}

	run := t.manager.Get(id)
	if run == nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("subagent not found: %s", id),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"info":    run.GetInfo(),
		"success": true,
	}, nil
}

func (t *SubagentTool) result(args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return map[string]interface{}{
			"error":   "id is required for result action",
			"success": false,
		}, nil
	}

	run := t.manager.Get(id)
	if run == nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("subagent not found: %s", id),
			"success": false,
		}, nil
	}

	result, err := run.GetResult()
	if err != nil {
		return map[string]interface{}{
			"id":      id,
			"info":    run.GetInfo(),
			"error":   err.Error(),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"id":      id,
		"result":  result,
		"info":    run.GetInfo(),
		"success": true,
	}, nil
}

func (t *SubagentTool) stop(args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return map[string]interface{}{
			"error":   "id is required for stop action",
			"success": false,
		}, nil
	}

	if err := t.manager.Stop(id); err != nil {
		return map[string]interface{}{
			"error":   err.Error(),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"id":      id,
		"message": "stop signal sent",
		"success": true,
	}, nil
}

func (t *SubagentTool) remove(args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return map[string]interface{}{
			"error":   "id is required for remove action",
			"success": false,
		}, nil
	}

	if err := t.manager.Remove(id); err != nil {
		return map[string]interface{}{
			"error":   err.Error(),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"id":      id,
		"message": "subagent removed",
		"success": true,
	}, nil
}
