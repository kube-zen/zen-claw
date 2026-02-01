package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neves/zen-claw/internal/session"
)

type Config struct {
	Workspace string
	Session   *session.Session
}

type Manager struct {
	config Config
	tools  map[string]Tool
}

type Tool interface {
	Name() string
	Description() string
	Execute(args map[string]interface{}) (interface{}, error)
}

func NewManager(config Config) (*Manager, error) {
	mgr := &Manager{
		config: config,
		tools:  make(map[string]Tool),
	}

	// Register core tools
	mgr.registerCoreTools()

	return mgr, nil
}

func (m *Manager) registerCoreTools() {
	m.tools["read"] = &ReadTool{workspace: m.config.Workspace}
	m.tools["write"] = &WriteTool{workspace: m.config.Workspace}
	m.tools["edit"] = &EditTool{workspace: m.config.Workspace}
	m.tools["exec"] = &ExecTool{}
	m.tools["process"] = &ProcessTool{}
}

func (m *Manager) List() []string {
	var names []string
	for name := range m.tools {
		names = append(names, name)
	}
	return names
}

func (m *Manager) Get(name string) (Tool, bool) {
	tool, ok := m.tools[name]
	return tool, ok
}

func (m *Manager) Execute(name string, args map[string]interface{}) (interface{}, error) {
	tool, ok := m.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return tool.Execute(args)
}

// ReadTool implementation
type ReadTool struct {
	workspace string
}

func (t *ReadTool) Name() string { return "read" }
func (t *ReadTool) Description() string {
	return "Read file contents"
}
func (t *ReadTool) Execute(args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path argument required")
	}

	// Make path absolute relative to workspace if needed
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workspace, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return string(content), nil
}

// WriteTool implementation
type WriteTool struct {
	workspace string
}

func (t *WriteTool) Name() string { return "write" }
func (t *WriteTool) Description() string {
	return "Create or overwrite files"
}
func (t *WriteTool) Execute(args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path argument required")
	}
	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content argument required")
	}

	// Make path absolute relative to workspace if needed
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workspace, path)
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create parent directories: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	return fmt.Sprintf("File written: %s (%d bytes)", path, len(content)), nil
}

// EditTool implementation
type EditTool struct {
	workspace string
}

func (t *EditTool) Name() string { return "edit" }
func (t *EditTool) Description() string {
	return "Make precise edits to files"
}
func (t *EditTool) Execute(args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path argument required")
	}
	oldText, ok := args["oldText"].(string)
	if !ok {
		return nil, fmt.Errorf("oldText argument required")
	}
	newText, ok := args["newText"].(string)
	if !ok {
		return nil, fmt.Errorf("newText argument required")
	}

	// Make path absolute relative to workspace if needed
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workspace, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	contentStr := string(content)
	// Simple exact replacement for now
	newContent := strings.Replace(contentStr, oldText, newText, 1)
	if newContent == contentStr {
		return nil, fmt.Errorf("oldText not found in file")
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	return fmt.Sprintf("File edited: %s", path), nil
}

// ExecTool implementation
type ExecTool struct{}

func (t *ExecTool) Name() string { return "exec" }
func (t *ExecTool) Description() string {
	return "Run shell commands"
}
func (t *ExecTool) Execute(args map[string]interface{}) (interface{}, error) {
	command, ok := args["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command argument required")
	}

	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("command failed: %w\nOutput: %s", err, output)
	}

	return string(output), nil
}

// ProcessTool implementation
type ProcessTool struct{}

func (t *ProcessTool) Name() string { return "process" }
func (t *ProcessTool) Description() string {
	return "Manage background exec sessions"
}
func (t *ProcessTool) Execute(args map[string]interface{}) (interface{}, error) {
	// TODO: Implement process management
	return "Process management coming soon", nil
}