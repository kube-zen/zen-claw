package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ExecTool executes shell commands
type ExecTool struct {
	BaseTool
	workingDir string
}

// NewExecTool creates a new exec tool
func NewExecTool(workingDir string) *ExecTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Shell command to execute",
			},
		},
		"required": []string{"command"},
	}
	
	return &ExecTool{
		BaseTool: NewBaseTool(
			"exec",
			"Execute shell command",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *ExecTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	command, ok := args["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command parameter is required")
	}
	
	// Create command with context
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}
	
	// Execute with timeout
	output, err := cmd.CombinedOutput()
	result := map[string]interface{}{
		"command":   command,
		"output":    string(output),
		"exit_code": cmd.ProcessState.ExitCode(),
	}
	
	if err != nil {
		result["error"] = err.Error()
	}
	
	return result, nil
}

// ReadFileTool reads files
type ReadFileTool struct {
	BaseTool
	workingDir string
}

// NewReadFileTool creates a new read file tool
func NewReadFileTool(workingDir string) *ReadFileTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path to read",
			},
		},
		"required": []string{"path"},
	}
	
	return &ReadFileTool{
		BaseTool: NewBaseTool(
			"read_file",
			"Read file contents",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter is required")
	}
	
	// Resolve path relative to working directory
	fullPath := path
	if t.workingDir != "" && !strings.HasPrefix(path, "/") {
		fullPath = t.workingDir + "/" + path
	}
	
	// Read file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return map[string]interface{}{
			"path":  path,
			"error": err.Error(),
		}, nil // Return error as result, not as Go error
	}
	
	return map[string]interface{}{
		"path":    path,
		"content": string(content),
		"size":    len(content),
	}, nil
}

// ListDirTool lists directory contents
type ListDirTool struct {
	BaseTool
	workingDir string
}

// NewListDirTool creates a new list directory tool
func NewListDirTool(workingDir string) *ListDirTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Directory path (default: current directory)",
			},
		},
	}
	
	return &ListDirTool{
		BaseTool: NewBaseTool(
			"list_dir",
			"List directory contents",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *ListDirTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path := "."
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}
	
	// Resolve path relative to working directory
	fullPath := path
	if t.workingDir != "" && !strings.HasPrefix(path, "/") {
		fullPath = t.workingDir + "/" + path
	}
	
	// List directory
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return map[string]interface{}{
			"path":  path,
			"error": err.Error(),
		}, nil
	}
	
	// Format entries
	var files []map[string]interface{}
	for _, entry := range entries {
		info, _ := entry.Info()
		files = append(files, map[string]interface{}{
			"name":  entry.Name(),
			"type":  getFileType(entry),
			"size":  info.Size(),
			"mode":  info.Mode().String(),
		})
	}
	
	return map[string]interface{}{
		"path":  path,
		"files": files,
		"count": len(files),
	}, nil
}

// SystemInfoTool gets system information
type SystemInfoTool struct {
	BaseTool
}

// NewSystemInfoTool creates a new system info tool
func NewSystemInfoTool() *SystemInfoTool {
	params := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
	
	return &SystemInfoTool{
		BaseTool: NewBaseTool(
			"system_info",
			"Get system information",
			params,
		),
	}
}

func (t *SystemInfoTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	hostname, _ := os.Hostname()
	wd, _ := os.Getwd()
	
	return map[string]interface{}{
		"hostname":         hostname,
		"current_directory": wd,
		"time":             time.Now().Format(time.RFC3339),
		"pid":              os.Getpid(),
		"env_vars":         os.Environ(),
	}, nil
}

// Helper function to get file type
func getFileType(entry os.DirEntry) string {
	if entry.IsDir() {
		return "directory"
	}
	
	info, err := entry.Info()
	if err != nil {
		return "file"
	}
	
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		return "symlink"
	}
	
	return "file"
}