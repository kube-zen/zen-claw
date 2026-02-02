package agent

import (
	"context"
)

// Tool represents an executable tool that the AI can call
type Tool interface {
	// Name returns the tool name (used by AI to call it)
	Name() string

	// Description returns a human-readable description for the AI
	Description() string

	// Parameters returns the JSON schema for the tool parameters
	Parameters() map[string]interface{}

	// Execute runs the tool with the given arguments
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// BaseTool provides common functionality for tools
type BaseTool struct {
	name        string
	description string
	parameters  map[string]interface{}
}

// NewBaseTool creates a new base tool
func NewBaseTool(name, description string, parameters map[string]interface{}) BaseTool {
	return BaseTool{
		name:        name,
		description: description,
		parameters:  parameters,
	}
}

func (t BaseTool) Name() string {
	return t.name
}

func (t BaseTool) Description() string {
	return t.description
}

func (t BaseTool) Parameters() map[string]interface{} {
	return t.parameters
}
