package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PROCESS MANAGEMENT TOOLS
// ═══════════════════════════════════════════════════════════════════════════════

// BackgroundProcess represents a background running process
type BackgroundProcess struct {
	ID        string
	Command   string
	StartTime time.Time
	EndTime   *time.Time
	ExitCode  *int
	Output    *strings.Builder
	Cmd       *exec.Cmd
	Done      chan struct{}
	mu        sync.Mutex
}

// ProcessManager manages background processes
type ProcessManager struct {
	processes map[string]*BackgroundProcess
	mu        sync.RWMutex
	nextID    int
}

// Global process manager (per gateway instance)
var globalProcessManager = &ProcessManager{
	processes: make(map[string]*BackgroundProcess),
}

// GetProcessManager returns the global process manager
func GetProcessManager() *ProcessManager {
	return globalProcessManager
}

// Start starts a background process
func (pm *ProcessManager) Start(ctx context.Context, command, workingDir string) (*BackgroundProcess, error) {
	pm.mu.Lock()
	pm.nextID++
	id := fmt.Sprintf("proc-%d", pm.nextID)
	pm.mu.Unlock()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	proc := &BackgroundProcess{
		ID:        id,
		Command:   command,
		StartTime: time.Now(),
		Output:    &strings.Builder{},
		Cmd:       cmd,
		Done:      make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Capture output in background
	go func() {
		multi := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(multi)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			proc.mu.Lock()
			proc.Output.WriteString(scanner.Text() + "\n")
			proc.mu.Unlock()
		}
	}()

	// Wait for completion in background
	go func() {
		err := cmd.Wait()
		proc.mu.Lock()
		now := time.Now()
		proc.EndTime = &now
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				proc.ExitCode = &code
			} else {
				code := -1
				proc.ExitCode = &code
			}
		} else {
			code := 0
			proc.ExitCode = &code
		}
		proc.mu.Unlock()
		close(proc.Done)
	}()

	pm.mu.Lock()
	pm.processes[id] = proc
	pm.mu.Unlock()

	return proc, nil
}

// Get returns a process by ID
func (pm *ProcessManager) Get(id string) *BackgroundProcess {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.processes[id]
}

// List returns all processes
func (pm *ProcessManager) List() []*BackgroundProcess {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*BackgroundProcess, 0, len(pm.processes))
	for _, p := range pm.processes {
		result = append(result, p)
	}
	return result
}

// Kill kills a process
func (pm *ProcessManager) Kill(id string) error {
	pm.mu.RLock()
	proc := pm.processes[id]
	pm.mu.RUnlock()

	if proc == nil {
		return fmt.Errorf("process not found: %s", id)
	}

	if proc.Cmd.Process != nil {
		return proc.Cmd.Process.Kill()
	}
	return nil
}

// Remove removes a completed process from the list
func (pm *ProcessManager) Remove(id string) {
	pm.mu.Lock()
	delete(pm.processes, id)
	pm.mu.Unlock()
}

// ProcessTool manages background exec sessions
type ProcessTool struct {
	BaseTool
	workingDir string
}

// NewProcessTool creates a process management tool
func NewProcessTool(workingDir string) *ProcessTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action: 'start', 'list', 'poll', 'log', 'kill', 'remove'",
			},
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Command to run (for 'start' action)",
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Process ID (for poll/log/kill/remove actions)",
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Line offset for log (omit for last N lines)",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Max lines to return (default 50)",
			},
		},
		"required": []string{"action"},
	}

	return &ProcessTool{
		BaseTool: NewBaseTool(
			"process",
			"Manage background processes. Actions: start (run command in background), list (show all), poll (check status + new output), log (get output), kill (terminate), remove (cleanup).",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *ProcessTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return nil, fmt.Errorf("action parameter is required")
	}

	pm := GetProcessManager()

	switch action {
	case "start":
		command, ok := args["command"].(string)
		if !ok || command == "" {
			return map[string]interface{}{
				"error":   "command is required for start action",
				"success": false,
			}, nil
		}

		proc, err := pm.Start(ctx, command, t.workingDir)
		if err != nil {
			return map[string]interface{}{
				"error":   fmt.Sprintf("failed to start: %v", err),
				"success": false,
			}, nil
		}

		return map[string]interface{}{
			"id":         proc.ID,
			"command":    proc.Command,
			"start_time": proc.StartTime.Format(time.RFC3339),
			"status":     "running",
			"success":    true,
		}, nil

	case "list":
		procs := pm.List()
		var result []map[string]interface{}
		for _, p := range procs {
			p.mu.Lock()
			status := "running"
			if p.ExitCode != nil {
				status = fmt.Sprintf("exited(%d)", *p.ExitCode)
			}
			result = append(result, map[string]interface{}{
				"id":         p.ID,
				"command":    truncateString(p.Command, 60),
				"start_time": p.StartTime.Format(time.RFC3339),
				"status":     status,
			})
			p.mu.Unlock()
		}
		return map[string]interface{}{
			"processes": result,
			"count":     len(result),
			"success":   true,
		}, nil

	case "poll":
		id, ok := args["id"].(string)
		if !ok || id == "" {
			return map[string]interface{}{
				"error":   "id is required for poll action",
				"success": false,
			}, nil
		}

		proc := pm.Get(id)
		if proc == nil {
			return map[string]interface{}{
				"error":   fmt.Sprintf("process not found: %s", id),
				"success": false,
			}, nil
		}

		proc.mu.Lock()
		status := "running"
		var exitCode *int
		if proc.ExitCode != nil {
			status = "completed"
			exitCode = proc.ExitCode
		}
		output := proc.Output.String()
		proc.mu.Unlock()

		// Return last 100 lines for poll
		lines := strings.Split(output, "\n")
		if len(lines) > 100 {
			lines = lines[len(lines)-100:]
		}

		result := map[string]interface{}{
			"id":      id,
			"status":  status,
			"output":  strings.Join(lines, "\n"),
			"success": true,
		}
		if exitCode != nil {
			result["exit_code"] = *exitCode
		}
		return result, nil

	case "log":
		id, ok := args["id"].(string)
		if !ok || id == "" {
			return map[string]interface{}{
				"error":   "id is required for log action",
				"success": false,
			}, nil
		}

		proc := pm.Get(id)
		if proc == nil {
			return map[string]interface{}{
				"error":   fmt.Sprintf("process not found: %s", id),
				"success": false,
			}, nil
		}

		limit := 50
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}

		proc.mu.Lock()
		output := proc.Output.String()
		proc.mu.Unlock()

		lines := strings.Split(output, "\n")

		// Handle offset
		if offset, ok := args["offset"].(float64); ok {
			start := int(offset)
			if start >= 0 && start < len(lines) {
				lines = lines[start:]
			}
		} else {
			// No offset - return last N lines
			if len(lines) > limit {
				lines = lines[len(lines)-limit:]
			}
		}

		// Apply limit
		if len(lines) > limit {
			lines = lines[:limit]
		}

		return map[string]interface{}{
			"id":          id,
			"output":      strings.Join(lines, "\n"),
			"total_lines": len(strings.Split(output, "\n")),
			"success":     true,
		}, nil

	case "kill":
		id, ok := args["id"].(string)
		if !ok || id == "" {
			return map[string]interface{}{
				"error":   "id is required for kill action",
				"success": false,
			}, nil
		}

		if err := pm.Kill(id); err != nil {
			return map[string]interface{}{
				"error":   err.Error(),
				"success": false,
			}, nil
		}

		return map[string]interface{}{
			"id":      id,
			"message": "process killed",
			"success": true,
		}, nil

	case "remove":
		id, ok := args["id"].(string)
		if !ok || id == "" {
			return map[string]interface{}{
				"error":   "id is required for remove action",
				"success": false,
			}, nil
		}

		pm.Remove(id)
		return map[string]interface{}{
			"id":      id,
			"message": "process removed",
			"success": true,
		}, nil

	default:
		return map[string]interface{}{
			"error":   fmt.Sprintf("unknown action: %s", action),
			"success": false,
		}, nil
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
