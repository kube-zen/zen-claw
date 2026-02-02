package tools

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitTool implements Git operations
type GitTool struct{}

func (t *GitTool) Name() string { return "git" }
func (t *GitTool) Description() string {
	return "Perform Git operations. Arguments: operation (status, diff, log, branch, checkout), args (map of additional arguments)"
}
func (t *GitTool) Execute(args map[string]interface{}) (interface{}, error) {
	operation, ok := args["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation argument required")
	}

	// Prepare command
	var cmd *exec.Cmd
	switch operation {
	case "status":
		cmd = exec.Command("git", "status", "--porcelain")
	case "diff":
		cmd = exec.Command("git", "diff")
	case "log":
		cmd = exec.Command("git", "log", "--oneline", "-10")
	case "branch":
		cmd = exec.Command("git", "branch", "-a")
	case "checkout":
		branch, ok := args["branch"].(string)
		if !ok {
			return nil, fmt.Errorf("branch argument required for checkout")
		}
		cmd = exec.Command("git", "checkout", branch)
	default:
		return nil, fmt.Errorf("unsupported git operation: %s", operation)
	}

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s failed: %w\nOutput: %s", operation, err, output)
	}

	return string(output), nil
}