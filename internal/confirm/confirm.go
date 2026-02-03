// Package confirm implements human-in-the-loop confirmation for dangerous operations.
// Provides a simple confirmation mechanism that can pause execution and ask for approval.
package confirm

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Level defines the confirmation requirement level
type Level string

const (
	LevelNone     Level = "none"     // No confirmation needed
	LevelDanger   Level = "danger"   // Confirm dangerous operations
	LevelAll      Level = "all"      // Confirm all write operations
)

// Operation represents an operation that may need confirmation
type Operation struct {
	Type        string                 // "exec", "write_file", "edit_file", "delete"
	Description string                 // Human-readable description
	Details     map[string]interface{} // Operation-specific details
	Risk        string                 // "low", "medium", "high", "critical"
}

// Confirmer handles confirmation requests
type Confirmer struct {
	level          Level
	alwaysYes      bool      // Auto-approve all (for non-interactive)
	dangerPatterns []string  // Patterns that trigger confirmation
	callback       ConfirmCallback
}

// ConfirmCallback is called to get user confirmation
// Returns (approved, error)
type ConfirmCallback func(op Operation) (bool, error)

// NewConfirmer creates a new confirmer
func NewConfirmer(level Level) *Confirmer {
	return &Confirmer{
		level:     level,
		alwaysYes: false,
		dangerPatterns: []string{
			// Destructive commands
			"rm -rf", "rm -r", "rmdir",
			"drop database", "DROP DATABASE",
			"truncate table", "TRUNCATE TABLE",
			"delete from", "DELETE FROM",
			// System modification
			"chmod 777", "chown root",
			"sudo rm", "sudo dd",
			// Git destructive
			"git push --force", "git push -f",
			"git reset --hard",
			"git clean -fd",
			// Network/system
			"curl | sh", "curl | bash",
			"wget -O - | sh",
		},
	}
}

// SetAlwaysYes sets auto-approve mode (for non-interactive)
func (c *Confirmer) SetAlwaysYes(yes bool) {
	c.alwaysYes = yes
}

// SetCallback sets a custom confirmation callback
func (c *Confirmer) SetCallback(cb ConfirmCallback) {
	c.callback = cb
}

// AddDangerPattern adds a pattern that triggers confirmation
func (c *Confirmer) AddDangerPattern(pattern string) {
	c.dangerPatterns = append(c.dangerPatterns, pattern)
}

// ShouldConfirm checks if an operation needs confirmation
func (c *Confirmer) ShouldConfirm(op Operation) bool {
	if c.level == LevelNone {
		return false
	}

	if c.level == LevelAll {
		// Confirm all write operations
		switch op.Type {
		case "exec", "write_file", "edit_file", "append_file", "delete":
			return true
		}
	}

	// Level == LevelDanger - check risk and patterns
	if op.Risk == "high" || op.Risk == "critical" {
		return true
	}

	// Check against danger patterns
	return c.matchesDangerPattern(op)
}

// matchesDangerPattern checks if operation matches any danger pattern
func (c *Confirmer) matchesDangerPattern(op Operation) bool {
	// Get command or content to check
	var toCheck string
	
	switch op.Type {
	case "exec":
		if cmd, ok := op.Details["command"].(string); ok {
			toCheck = cmd
		}
	case "write_file", "edit_file", "append_file":
		if path, ok := op.Details["path"].(string); ok {
			// Check sensitive paths
			sensitivePaths := []string{
				"/etc/", "/root/", "/var/",
				".ssh/", ".gnupg/", ".aws/",
				"credentials", "secrets", ".env",
				"id_rsa", "id_ed25519",
			}
			for _, sensitive := range sensitivePaths {
				if strings.Contains(path, sensitive) {
					return true
				}
			}
		}
		if content, ok := op.Details["content"].(string); ok {
			toCheck = content
		}
	}

	// Check against danger patterns
	for _, pattern := range c.dangerPatterns {
		if strings.Contains(strings.ToLower(toCheck), strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// Confirm asks for confirmation and returns whether to proceed
func (c *Confirmer) Confirm(op Operation) (bool, error) {
	if c.alwaysYes {
		return true, nil
	}

	if !c.ShouldConfirm(op) {
		return true, nil
	}

	// Use callback if set
	if c.callback != nil {
		return c.callback(op)
	}

	// Default: CLI confirmation
	return c.cliConfirm(op)
}

// cliConfirm prompts for confirmation via CLI
func (c *Confirmer) cliConfirm(op Operation) (bool, error) {
	// Build confirmation message
	fmt.Println()
	fmt.Println("⚠️  CONFIRMATION REQUIRED")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Operation: %s\n", op.Type)
	fmt.Printf("Risk: %s\n", op.Risk)
	fmt.Printf("Description: %s\n", op.Description)
	
	if len(op.Details) > 0 {
		fmt.Println("Details:")
		for k, v := range op.Details {
			// Truncate long values
			val := fmt.Sprintf("%v", v)
			if len(val) > 100 {
				val = val[:97] + "..."
			}
			fmt.Printf("  %s: %s\n", k, val)
		}
	}
	
	fmt.Println("═══════════════════════════════════════")
	fmt.Print("Proceed? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// ClassifyRisk classifies the risk level of an operation
func ClassifyRisk(op Operation) string {
	// Check for critical patterns first
	criticalPatterns := []string{
		"rm -rf /", "rm -rf ~", "rm -rf /*",
		"dd if=/dev/zero of=/dev/",
		":(){ :|:& };:", // Fork bomb
		"> /dev/sda",
		"mkfs.",
	}

	var content string
	if cmd, ok := op.Details["command"].(string); ok {
		content = cmd
	}

	for _, pattern := range criticalPatterns {
		if strings.Contains(content, pattern) {
			return "critical"
		}
	}

	// High risk patterns
	highRiskPatterns := []string{
		"rm -rf", "rm -r",
		"git push --force", "git push -f",
		"git reset --hard",
		"drop database", "DROP DATABASE",
		"sudo",
	}

	for _, pattern := range highRiskPatterns {
		if strings.Contains(strings.ToLower(content), strings.ToLower(pattern)) {
			return "high"
		}
	}

	// Medium risk: any exec command
	if op.Type == "exec" {
		return "medium"
	}

	// Low risk: file operations
	return "low"
}

// Helper functions for creating operations

// ExecOp creates an exec operation
func ExecOp(command, workDir string) Operation {
	op := Operation{
		Type:        "exec",
		Description: fmt.Sprintf("Execute command: %s", truncate(command, 60)),
		Details: map[string]interface{}{
			"command":     command,
			"working_dir": workDir,
		},
	}
	op.Risk = ClassifyRisk(op)
	return op
}

// WriteOp creates a write_file operation
func WriteOp(path string, contentLen int) Operation {
	op := Operation{
		Type:        "write_file",
		Description: fmt.Sprintf("Write to file: %s", path),
		Details: map[string]interface{}{
			"path":         path,
			"content_size": contentLen,
		},
		Risk: "low",
	}
	// Check sensitive paths
	sensitivePaths := []string{"/etc/", "/root/", ".ssh/", ".env", "credentials"}
	for _, sensitive := range sensitivePaths {
		if strings.Contains(path, sensitive) {
			op.Risk = "high"
			break
		}
	}
	return op
}

// EditOp creates an edit_file operation
func EditOp(path string) Operation {
	op := Operation{
		Type:        "edit_file",
		Description: fmt.Sprintf("Edit file: %s", path),
		Details: map[string]interface{}{
			"path": path,
		},
		Risk: "low",
	}
	return op
}

// DeleteOp creates a delete operation
func DeleteOp(path string) Operation {
	return Operation{
		Type:        "delete",
		Description: fmt.Sprintf("Delete: %s", path),
		Details: map[string]interface{}{
			"path": path,
		},
		Risk: "medium",
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
