package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/neves/zen-claw/internal/agent"
	"gopkg.in/yaml.v3"
)

// PluginManifest defines a plugin's metadata and configuration
type PluginManifest struct {
	Name        string                 `yaml:"name"`
	Version     string                 `yaml:"version"`
	Description string                 `yaml:"description"`
	Author      string                 `yaml:"author,omitempty"`
	Command     string                 `yaml:"command"`    // e.g., "python", "bash", "./run.sh"
	Args        []string               `yaml:"args"`       // Additional args before input
	Timeout     string                 `yaml:"timeout"`    // e.g., "30s", "5m"
	Parameters  map[string]interface{} `yaml:"parameters"` // JSON Schema for tool parameters
	Env         map[string]string      `yaml:"env"`        // Environment variables
}

// Plugin represents a loaded plugin
type Plugin struct {
	Manifest PluginManifest
	Dir      string // Plugin directory path
	timeout  time.Duration
}

// LoadPlugin loads a plugin from a directory
func LoadPlugin(dir string) (*Plugin, error) {
	manifestPath := filepath.Join(dir, "plugin.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if manifest.Name == "" {
		return nil, fmt.Errorf("plugin name is required")
	}
	if manifest.Command == "" {
		return nil, fmt.Errorf("plugin command is required")
	}

	// Parse timeout (default 30s)
	timeout := 30 * time.Second
	if manifest.Timeout != "" {
		if t, err := time.ParseDuration(manifest.Timeout); err == nil {
			timeout = t
		}
	}

	// Default parameters schema
	if manifest.Parameters == nil {
		manifest.Parameters = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	return &Plugin{
		Manifest: manifest,
		Dir:      dir,
		timeout:  timeout,
	}, nil
}

// ToTool converts a plugin to an agent.Tool
func (p *Plugin) ToTool() agent.Tool {
	return &PluginTool{plugin: p}
}

// PluginTool wraps a plugin as an agent.Tool
type PluginTool struct {
	agent.BaseTool
	plugin *Plugin
}

// Name returns the tool name
func (t *PluginTool) Name() string {
	return t.plugin.Manifest.Name
}

// Description returns the tool description
func (t *PluginTool) Description() string {
	return t.plugin.Manifest.Description
}

// Parameters returns the JSON schema for parameters
func (t *PluginTool) Parameters() map[string]interface{} {
	return t.plugin.Manifest.Parameters
}

// Execute runs the plugin with given arguments
func (t *PluginTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	p := t.plugin

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Resolve command path (relative to plugin dir or absolute)
	cmdPath := p.Manifest.Command
	if !filepath.IsAbs(cmdPath) && !isExecutableInPath(cmdPath) {
		cmdPath = filepath.Join(p.Dir, cmdPath)
	}

	// Build command
	cmd := exec.CommandContext(execCtx, cmdPath, p.Manifest.Args...)
	cmd.Dir = p.Dir

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range p.Manifest.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Pass args as JSON via stdin
	inputJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal args: %w", err)
	}
	cmd.Stdin = bytes.NewReader(inputJSON)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	if err := cmd.Run(); err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("plugin timed out after %v", p.timeout)
		}
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("plugin error: %s", errMsg)
	}

	// Parse output as JSON
	output := stdout.Bytes()
	if len(output) == 0 {
		return map[string]interface{}{"result": "success"}, nil
	}

	var result interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		// Return raw output if not JSON
		return map[string]interface{}{"output": string(output)}, nil
	}

	return result, nil
}

// isExecutableInPath checks if a command is in PATH
func isExecutableInPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
