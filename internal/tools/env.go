package tools

import (
	"fmt"
	"os"
	"strings"
)

// EnvTool implements environment variable operations
type EnvTool struct{}

func (t *EnvTool) Name() string { return "env" }
func (t *EnvTool) Description() string {
	return "Manage environment variables. Arguments: operation (get, set, list), key (for get/set), value (for set)"
}
func (t *EnvTool) Execute(args map[string]interface{}) (interface{}, error) {
	operation, ok := args["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation argument required")
	}

	switch operation {
	case "list":
		// List all environment variables
		var envVars []string
		for _, env := range os.Environ() {
			envVars = append(envVars, env)
		}
		return strings.Join(envVars, "\n"), nil
		
	case "get":
		key, ok := args["key"].(string)
		if !ok {
			return nil, fmt.Errorf("key argument required for get")
		}
		value := os.Getenv(key)
		return fmt.Sprintf("%s=%s", key, value), nil
		
	case "set":
		key, ok := args["key"].(string)
		if !ok {
			return nil, fmt.Errorf("key argument required for set")
		}
		value, ok := args["value"].(string)
		if !ok {
			return nil, fmt.Errorf("value argument required for set")
		}
		err := os.Setenv(key, value)
		if err != nil {
			return nil, fmt.Errorf("failed to set environment variable: %w", err)
		}
		return fmt.Sprintf("Set %s=%s", key, value), nil
		
	default:
		return nil, fmt.Errorf("unsupported env operation: %s", operation)
	}
}