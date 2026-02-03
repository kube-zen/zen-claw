package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// APPLY PATCH TOOL
// ═══════════════════════════════════════════════════════════════════════════════

// ApplyPatchTool applies structured patches to files
type ApplyPatchTool struct {
	BaseTool
	workingDir string
}

// NewApplyPatchTool creates an apply patch tool
func NewApplyPatchTool(workingDir string) *ApplyPatchTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"input": map[string]interface{}{
				"type": "string",
				"description": `Patch content in structured format:

*** Begin Patch
*** Add File: path/to/new.txt
+line 1
+line 2

*** Update File: path/to/existing.txt
@@
-old line
+new line

*** Delete File: path/to/remove.txt
*** End Patch`,
			},
		},
		"required": []string{"input"},
	}

	return &ApplyPatchTool{
		BaseTool: NewBaseTool(
			"apply_patch",
			"Apply structured patches to create, update, or delete multiple files. Better than multiple edit_file calls for complex changes.",
			params,
		),
		workingDir: workingDir,
	}
}

// PatchOperation represents a single file operation
type PatchOperation struct {
	Type     string // "add", "update", "delete"
	Path     string
	NewPath  string // for rename
	Content  string // for add
	Hunks    []PatchHunk
}

// PatchHunk represents a diff hunk
type PatchHunk struct {
	OldLines []string
	NewLines []string
}

func (t *ApplyPatchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	input, ok := args["input"].(string)
	if !ok || input == "" {
		return nil, fmt.Errorf("input parameter is required")
	}

	// Parse patch
	ops, err := parsePatch(input)
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("failed to parse patch: %v", err),
			"success": false,
		}, nil
	}

	if len(ops) == 0 {
		return map[string]interface{}{
			"error":   "no operations found in patch",
			"success": false,
		}, nil
	}

	// Apply operations
	var results []map[string]interface{}
	var errors []string

	for _, op := range ops {
		// Resolve path
		fullPath := op.Path
		if t.workingDir != "" && !filepath.IsAbs(op.Path) {
			fullPath = filepath.Join(t.workingDir, op.Path)
		}

		switch op.Type {
		case "add":
			result := t.applyAdd(fullPath, op)
			results = append(results, result)
			if !result["success"].(bool) {
				errors = append(errors, fmt.Sprintf("%s: %v", op.Path, result["error"]))
			}

		case "update":
			result := t.applyUpdate(fullPath, op)
			results = append(results, result)
			if !result["success"].(bool) {
				errors = append(errors, fmt.Sprintf("%s: %v", op.Path, result["error"]))
			}

		case "delete":
			result := t.applyDelete(fullPath, op)
			results = append(results, result)
			if !result["success"].(bool) {
				errors = append(errors, fmt.Sprintf("%s: %v", op.Path, result["error"]))
			}
		}
	}

	success := len(errors) == 0
	response := map[string]interface{}{
		"operations": results,
		"total":      len(ops),
		"success":    success,
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	return response, nil
}

func (t *ApplyPatchTool) applyAdd(fullPath string, op PatchOperation) map[string]interface{} {
	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return map[string]interface{}{
			"path":    op.Path,
			"action":  "add",
			"error":   fmt.Sprintf("failed to create directory: %v", err),
			"success": false,
		}
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(op.Content), 0644); err != nil {
		return map[string]interface{}{
			"path":    op.Path,
			"action":  "add",
			"error":   fmt.Sprintf("failed to write file: %v", err),
			"success": false,
		}
	}

	return map[string]interface{}{
		"path":    op.Path,
		"action":  "add",
		"success": true,
	}
}

func (t *ApplyPatchTool) applyUpdate(fullPath string, op PatchOperation) map[string]interface{} {
	// Read existing file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return map[string]interface{}{
			"path":    op.Path,
			"action":  "update",
			"error":   fmt.Sprintf("failed to read file: %v", err),
			"success": false,
		}
	}

	contentStr := string(content)

	// Apply each hunk
	appliedHunks := 0
	for _, hunk := range op.Hunks {
		oldBlock := strings.Join(hunk.OldLines, "\n")
		newBlock := strings.Join(hunk.NewLines, "\n")

		if strings.Contains(contentStr, oldBlock) {
			contentStr = strings.Replace(contentStr, oldBlock, newBlock, 1)
			appliedHunks++
		}
	}

	if appliedHunks == 0 && len(op.Hunks) > 0 {
		return map[string]interface{}{
			"path":    op.Path,
			"action":  "update",
			"error":   "no hunks could be applied (old content not found)",
			"success": false,
		}
	}

	// Handle rename
	targetPath := fullPath
	if op.NewPath != "" {
		newFullPath := op.NewPath
		if t.workingDir != "" && !filepath.IsAbs(op.NewPath) {
			newFullPath = filepath.Join(t.workingDir, op.NewPath)
		}
		targetPath = newFullPath

		// Create new directory if needed
		if err := os.MkdirAll(filepath.Dir(newFullPath), 0755); err != nil {
			return map[string]interface{}{
				"path":    op.Path,
				"action":  "update",
				"error":   fmt.Sprintf("failed to create directory for rename: %v", err),
				"success": false,
			}
		}

		// Remove old file after writing new one
		defer os.Remove(fullPath)
	}

	// Write file
	if err := os.WriteFile(targetPath, []byte(contentStr), 0644); err != nil {
		return map[string]interface{}{
			"path":    op.Path,
			"action":  "update",
			"error":   fmt.Sprintf("failed to write file: %v", err),
			"success": false,
		}
	}

	result := map[string]interface{}{
		"path":          op.Path,
		"action":        "update",
		"hunks_applied": appliedHunks,
		"success":       true,
	}

	if op.NewPath != "" {
		result["renamed_to"] = op.NewPath
	}

	return result
}

func (t *ApplyPatchTool) applyDelete(fullPath string, op PatchOperation) map[string]interface{} {
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{
				"path":    op.Path,
				"action":  "delete",
				"error":   "file does not exist",
				"success": false,
			}
		}
		return map[string]interface{}{
			"path":    op.Path,
			"action":  "delete",
			"error":   fmt.Sprintf("failed to delete: %v", err),
			"success": false,
		}
	}

	return map[string]interface{}{
		"path":    op.Path,
		"action":  "delete",
		"success": true,
	}
}

// parsePatch parses a structured patch format
func parsePatch(input string) ([]PatchOperation, error) {
	var ops []PatchOperation

	lines := strings.Split(input, "\n")
	var currentOp *PatchOperation
	var currentHunk *PatchHunk
	inPatch := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Start/end markers
		if trimmed == "*** Begin Patch" {
			inPatch = true
			continue
		}
		if trimmed == "*** End Patch" {
			if currentOp != nil {
				if currentHunk != nil && (len(currentHunk.OldLines) > 0 || len(currentHunk.NewLines) > 0) {
					currentOp.Hunks = append(currentOp.Hunks, *currentHunk)
				}
				ops = append(ops, *currentOp)
			}
			break
		}

		if !inPatch {
			continue
		}

		// File operation markers
		if strings.HasPrefix(trimmed, "*** Add File:") {
			if currentOp != nil {
				if currentHunk != nil && (len(currentHunk.OldLines) > 0 || len(currentHunk.NewLines) > 0) {
					currentOp.Hunks = append(currentOp.Hunks, *currentHunk)
				}
				ops = append(ops, *currentOp)
			}
			path := strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Add File:"))
			currentOp = &PatchOperation{Type: "add", Path: path}
			currentHunk = nil
			continue
		}

		if strings.HasPrefix(trimmed, "*** Update File:") {
			if currentOp != nil {
				if currentHunk != nil && (len(currentHunk.OldLines) > 0 || len(currentHunk.NewLines) > 0) {
					currentOp.Hunks = append(currentOp.Hunks, *currentHunk)
				}
				ops = append(ops, *currentOp)
			}
			path := strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Update File:"))
			currentOp = &PatchOperation{Type: "update", Path: path}
			currentHunk = nil
			continue
		}

		if strings.HasPrefix(trimmed, "*** Delete File:") {
			if currentOp != nil {
				if currentHunk != nil && (len(currentHunk.OldLines) > 0 || len(currentHunk.NewLines) > 0) {
					currentOp.Hunks = append(currentOp.Hunks, *currentHunk)
				}
				ops = append(ops, *currentOp)
			}
			path := strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Delete File:"))
			currentOp = &PatchOperation{Type: "delete", Path: path}
			currentHunk = nil
			continue
		}

		if strings.HasPrefix(trimmed, "*** Move to:") {
			if currentOp != nil {
				currentOp.NewPath = strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Move to:"))
			}
			continue
		}

		if currentOp == nil {
			continue
		}

		// Content for add operation
		if currentOp.Type == "add" {
			if strings.HasPrefix(line, "+") {
				currentOp.Content += strings.TrimPrefix(line, "+") + "\n"
			} else if line != "" && !strings.HasPrefix(trimmed, "@@") {
				currentOp.Content += line + "\n"
			}
			continue
		}

		// Hunk markers for update
		if currentOp.Type == "update" {
			if strings.HasPrefix(trimmed, "@@") {
				if currentHunk != nil && (len(currentHunk.OldLines) > 0 || len(currentHunk.NewLines) > 0) {
					currentOp.Hunks = append(currentOp.Hunks, *currentHunk)
				}
				currentHunk = &PatchHunk{}
				continue
			}

			if currentHunk != nil {
				if strings.HasPrefix(line, "-") {
					currentHunk.OldLines = append(currentHunk.OldLines, strings.TrimPrefix(line, "-"))
				} else if strings.HasPrefix(line, "+") {
					currentHunk.NewLines = append(currentHunk.NewLines, strings.TrimPrefix(line, "+"))
				} else if strings.HasPrefix(line, " ") {
					// Context line - add to both
					content := strings.TrimPrefix(line, " ")
					currentHunk.OldLines = append(currentHunk.OldLines, content)
					currentHunk.NewLines = append(currentHunk.NewLines, content)
				}
			}
		}
	}

	// Handle last operation if no End marker
	if currentOp != nil && !inPatch {
		if currentHunk != nil && (len(currentHunk.OldLines) > 0 || len(currentHunk.NewLines) > 0) {
			currentOp.Hunks = append(currentOp.Hunks, *currentHunk)
		}
		ops = append(ops, *currentOp)
	}

	return ops, nil
}
