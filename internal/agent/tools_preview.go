package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// DIFF PREVIEW TOOLS
// ═══════════════════════════════════════════════════════════════════════════════

// PreviewWriteTool shows diff before writing
type PreviewWriteTool struct {
	BaseTool
	workingDir string
}

// NewPreviewWriteTool creates a preview write tool
func NewPreviewWriteTool(workingDir string) *PreviewWriteTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path to preview writing to",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content that would be written",
			},
		},
		"required": []string{"path", "content"},
	}

	return &PreviewWriteTool{
		BaseTool: NewBaseTool(
			"preview_write",
			"Preview changes before writing a file. Shows unified diff of what would change without actually modifying the file.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *PreviewWriteTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content parameter is required")
	}

	// Resolve path
	fullPath := path
	if t.workingDir != "" && !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "~") {
		fullPath = filepath.Join(t.workingDir, path)
	}

	if strings.HasPrefix(fullPath, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			fullPath = filepath.Join(home, fullPath[1:])
		}
	}

	// Read existing content
	existingContent := ""
	existed := false
	if data, err := os.ReadFile(fullPath); err == nil {
		existingContent = string(data)
		existed = true
	}

	// Generate diff
	diff := generateUnifiedDiff(path, existingContent, content)

	// Summary
	oldLines := len(strings.Split(existingContent, "\n"))
	newLines := len(strings.Split(content, "\n"))
	if existingContent == "" {
		oldLines = 0
	}

	return map[string]interface{}{
		"path":        path,
		"existed":     existed,
		"diff":        diff,
		"old_lines":   oldLines,
		"new_lines":   newLines,
		"old_size":    len(existingContent),
		"new_size":    len(content),
		"action":      ternary(existed, "modify", "create"),
		"preview":     true,
		"not_written": "Use write_file to apply these changes",
	}, nil
}

// PreviewEditTool shows diff before editing
type PreviewEditTool struct {
	BaseTool
	workingDir string
}

// NewPreviewEditTool creates a preview edit tool
func NewPreviewEditTool(workingDir string) *PreviewEditTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path to preview editing",
			},
			"old_string": map[string]interface{}{
				"type":        "string",
				"description": "The exact string to find",
			},
			"new_string": map[string]interface{}{
				"type":        "string",
				"description": "The replacement string",
			},
			"replace_all": map[string]interface{}{
				"type":        "boolean",
				"description": "Preview replacing all occurrences",
			},
		},
		"required": []string{"path", "old_string", "new_string"},
	}

	return &PreviewEditTool{
		BaseTool: NewBaseTool(
			"preview_edit",
			"Preview string replacement changes without modifying the file. Shows what edit_file would do.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *PreviewEditTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter is required")
	}

	oldString, ok := args["old_string"].(string)
	if !ok {
		return nil, fmt.Errorf("old_string parameter is required")
	}

	newString, ok := args["new_string"].(string)
	if !ok {
		return nil, fmt.Errorf("new_string parameter is required")
	}

	replaceAll := false
	if ra, ok := args["replace_all"].(bool); ok {
		replaceAll = ra
	}

	// Resolve path
	fullPath := path
	if t.workingDir != "" && !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "~") {
		fullPath = filepath.Join(t.workingDir, path)
	}

	if strings.HasPrefix(fullPath, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			fullPath = filepath.Join(home, fullPath[1:])
		}
	}

	// Read file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return map[string]interface{}{
			"path":    path,
			"error":   fmt.Sprintf("failed to read file: %v", err),
			"preview": true,
		}, nil
	}

	contentStr := string(content)

	// Check occurrences
	count := strings.Count(contentStr, oldString)
	if count == 0 {
		return map[string]interface{}{
			"path":    path,
			"error":   "old_string not found in file",
			"preview": true,
		}, nil
	}

	if !replaceAll && count > 1 {
		return map[string]interface{}{
			"path":        path,
			"error":       fmt.Sprintf("old_string found %d times, must be unique (or use replace_all)", count),
			"occurrences": count,
			"preview":     true,
		}, nil
	}

	// Generate preview
	var newContent string
	var replacements int
	if replaceAll {
		newContent = strings.ReplaceAll(contentStr, oldString, newString)
		replacements = count
	} else {
		newContent = strings.Replace(contentStr, oldString, newString, 1)
		replacements = 1
	}

	diff := generateUnifiedDiff(path, contentStr, newContent)

	return map[string]interface{}{
		"path":         path,
		"diff":         diff,
		"replacements": replacements,
		"preview":      true,
		"not_written":  "Use edit_file to apply these changes",
	}, nil
}

// generateUnifiedDiff creates a simple unified diff
func generateUnifiedDiff(filename, oldContent, newContent string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- a/%s\n", filename))
	diff.WriteString(fmt.Sprintf("+++ b/%s\n", filename))

	// Simple line-by-line diff
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	// Find changed regions
	type hunk struct {
		oldStart, oldCount int
		newStart, newCount int
		lines              []string
	}

	var hunks []hunk
	var currentHunk *hunk
	context := 3

	for i := 0; i < maxLines; i++ {
		oldLine := ""
		newLine := ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine || (i < len(oldLines)) != (i < len(newLines)) {
			if currentHunk == nil {
				start := i - context
				if start < 0 {
					start = 0
				}
				currentHunk = &hunk{
					oldStart: start + 1,
					newStart: start + 1,
				}
				// Add context before
				for j := start; j < i; j++ {
					if j < len(oldLines) {
						currentHunk.lines = append(currentHunk.lines, " "+oldLines[j])
						currentHunk.oldCount++
						currentHunk.newCount++
					}
				}
			}

			if i < len(oldLines) {
				currentHunk.lines = append(currentHunk.lines, "-"+oldLine)
				currentHunk.oldCount++
			}
			if i < len(newLines) {
				currentHunk.lines = append(currentHunk.lines, "+"+newLine)
				currentHunk.newCount++
			}
		} else if currentHunk != nil {
			// Add context line
			currentHunk.lines = append(currentHunk.lines, " "+oldLine)
			currentHunk.oldCount++
			currentHunk.newCount++

			// Check if we should close this hunk
			contextCount := 0
			for j := len(currentHunk.lines) - 1; j >= 0 && currentHunk.lines[j][0] == ' '; j-- {
				contextCount++
			}
			if contextCount >= context*2 {
				currentHunk.lines = currentHunk.lines[:len(currentHunk.lines)-(contextCount-context)]
				currentHunk.oldCount -= (contextCount - context)
				currentHunk.newCount -= (contextCount - context)
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
		}
	}

	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	// Output hunks
	for _, h := range hunks {
		diff.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", h.oldStart, h.oldCount, h.newStart, h.newCount))
		for _, line := range h.lines {
			diff.WriteString(line + "\n")
		}
	}

	if len(hunks) == 0 {
		return "(no changes)"
	}

	return diff.String()
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
