package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

	// Check for cd command and update working directory
	trimmedCmd := strings.TrimSpace(command)
	if strings.HasPrefix(trimmedCmd, "cd ") {
		// Extract directory
		dir := strings.TrimSpace(strings.TrimPrefix(trimmedCmd, "cd "))
		// Handle ~ expansion
		if strings.HasPrefix(dir, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				dir = home + dir[1:]
			}
		}

		// Update working directory if it exists
		if _, err := os.Stat(dir); err == nil {
			t.workingDir = dir
			return map[string]interface{}{
				"command":         command,
				"output":          fmt.Sprintf("Changed directory to: %s", dir),
				"exit_code":       0,
				"new_working_dir": dir,
			}, nil
		} else {
			return map[string]interface{}{
				"command":   command,
				"output":    fmt.Sprintf("Error: Directory %s does not exist", dir),
				"exit_code": 1,
				"error":     err.Error(),
			}, nil
		}
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
			"name": entry.Name(),
			"type": getFileType(entry),
			"size": info.Size(),
			"mode": info.Mode().String(),
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
		"hostname":          hostname,
		"current_directory": wd,
		"time":              time.Now().Format(time.RFC3339),
		"pid":               os.Getpid(),
		"env_vars":          os.Environ(),
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

// WriteFileTool writes content to a file (create or overwrite)
type WriteFileTool struct {
	BaseTool
	workingDir string
}

// NewWriteFileTool creates a new write file tool
func NewWriteFileTool(workingDir string) *WriteFileTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path to write to",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write to the file",
			},
			"create_dirs": map[string]interface{}{
				"type":        "boolean",
				"description": "Create parent directories if they don't exist (default: true)",
			},
		},
		"required": []string{"path", "content"},
	}

	return &WriteFileTool{
		BaseTool: NewBaseTool(
			"write_file",
			"Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Use edit_file for partial updates.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content parameter is required")
	}

	// Default create_dirs to true
	createDirs := true
	if cd, ok := args["create_dirs"].(bool); ok {
		createDirs = cd
	}

	// Resolve path relative to working directory
	fullPath := path
	if t.workingDir != "" && !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "~") {
		fullPath = filepath.Join(t.workingDir, path)
	}

	// Handle ~ expansion
	if strings.HasPrefix(fullPath, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			fullPath = filepath.Join(home, fullPath[1:])
		}
	}

	// Create parent directories if needed
	if createDirs {
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return map[string]interface{}{
				"path":    path,
				"error":   fmt.Sprintf("failed to create directories: %v", err),
				"success": false,
			}, nil
		}
	}

	// Check if file exists (for reporting)
	existed := false
	if _, err := os.Stat(fullPath); err == nil {
		existed = true
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return map[string]interface{}{
			"path":    path,
			"error":   err.Error(),
			"success": false,
		}, nil
	}

	action := "created"
	if existed {
		action = "overwritten"
	}

	return map[string]interface{}{
		"path":    path,
		"action":  action,
		"size":    len(content),
		"success": true,
	}, nil
}

// EditFileTool performs string replacement in files (like Cursor's StrReplace)
type EditFileTool struct {
	BaseTool
	workingDir string
}

// NewEditFileTool creates a new edit file tool
func NewEditFileTool(workingDir string) *EditFileTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path to edit",
			},
			"old_string": map[string]interface{}{
				"type":        "string",
				"description": "The exact string to find and replace (must be unique in the file)",
			},
			"new_string": map[string]interface{}{
				"type":        "string",
				"description": "The replacement string",
			},
			"replace_all": map[string]interface{}{
				"type":        "boolean",
				"description": "Replace all occurrences instead of just the first (default: false)",
			},
		},
		"required": []string{"path", "old_string", "new_string"},
	}

	return &EditFileTool{
		BaseTool: NewBaseTool(
			"edit_file",
			"Edit a file by replacing a specific string with a new string. The old_string must be unique in the file unless replace_all is true. For creating new files, use write_file instead.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *EditFileTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
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

	// Resolve path relative to working directory
	fullPath := path
	if t.workingDir != "" && !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "~") {
		fullPath = filepath.Join(t.workingDir, path)
	}

	// Handle ~ expansion
	if strings.HasPrefix(fullPath, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			fullPath = filepath.Join(home, fullPath[1:])
		}
	}

	// Read existing file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return map[string]interface{}{
			"path":    path,
			"error":   fmt.Sprintf("failed to read file: %v", err),
			"success": false,
		}, nil
	}

	contentStr := string(content)

	// Check if old_string exists
	count := strings.Count(contentStr, oldString)
	if count == 0 {
		return map[string]interface{}{
			"path":    path,
			"error":   "old_string not found in file",
			"success": false,
		}, nil
	}

	// If not replace_all, ensure old_string is unique
	if !replaceAll && count > 1 {
		return map[string]interface{}{
			"path":    path,
			"error":   fmt.Sprintf("old_string found %d times in file, must be unique (or use replace_all: true)", count),
			"success": false,
		}, nil
	}

	// Perform replacement
	var newContent string
	var replacements int
	if replaceAll {
		newContent = strings.ReplaceAll(contentStr, oldString, newString)
		replacements = count
	} else {
		newContent = strings.Replace(contentStr, oldString, newString, 1)
		replacements = 1
	}

	// Write back
	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return map[string]interface{}{
			"path":    path,
			"error":   fmt.Sprintf("failed to write file: %v", err),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"path":         path,
		"replacements": replacements,
		"success":      true,
	}, nil
}

// SearchFilesTool searches for patterns in files (grep-like)
type SearchFilesTool struct {
	BaseTool
	workingDir string
}

// NewSearchFilesTool creates a new search files tool
func NewSearchFilesTool(workingDir string) *SearchFilesTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Search pattern (regex supported)",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File or directory path to search in (default: current directory)",
			},
			"file_pattern": map[string]interface{}{
				"type":        "string",
				"description": "Glob pattern to filter files (e.g., '*.go', '*.ts')",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results (default: 50)",
			},
		},
		"required": []string{"pattern"},
	}

	return &SearchFilesTool{
		BaseTool: NewBaseTool(
			"search_files",
			"Search for a pattern in files. Supports regex patterns. Returns matching lines with file paths and line numbers.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *SearchFilesTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern parameter is required")
	}

	// Compile regex
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return map[string]interface{}{
			"pattern": pattern,
			"error":   fmt.Sprintf("invalid regex pattern: %v", err),
		}, nil
	}

	// Get search path
	searchPath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		searchPath = p
	}

	// Resolve path
	fullPath := searchPath
	if t.workingDir != "" && !strings.HasPrefix(searchPath, "/") {
		fullPath = filepath.Join(t.workingDir, searchPath)
	}

	// Get file pattern
	filePattern := "*"
	if fp, ok := args["file_pattern"].(string); ok && fp != "" {
		filePattern = fp
	}

	// Get max results
	maxResults := 50
	if mr, ok := args["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	var results []map[string]interface{}

	// Walk directory
	err = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip directories
		if info.IsDir() {
			// Skip hidden directories and common non-code directories
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check file pattern
		if filePattern != "*" {
			matched, _ := filepath.Match(filePattern, info.Name())
			if !matched {
				return nil
			}
		}

		// Skip binary files (simple heuristic: skip files > 1MB or with certain extensions)
		if info.Size() > 1024*1024 {
			return nil
		}

		// Search in file
		matches := searchInFile(path, regex, maxResults-len(results))
		results = append(results, matches...)

		if len(results) >= maxResults {
			return filepath.SkipAll
		}

		return nil
	})

	// Make paths relative
	for i := range results {
		if relPath, err := filepath.Rel(fullPath, results[i]["file"].(string)); err == nil {
			results[i]["file"] = relPath
		}
	}

	return map[string]interface{}{
		"pattern":      pattern,
		"path":         searchPath,
		"file_pattern": filePattern,
		"results":      results,
		"count":        len(results),
		"truncated":    len(results) >= maxResults,
	}, nil
}

// searchInFile searches for pattern matches in a single file
func searchInFile(path string, regex *regexp.Regexp, maxMatches int) []map[string]interface{} {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var matches []map[string]interface{}
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if regex.MatchString(line) {
			matches = append(matches, map[string]interface{}{
				"file":    path,
				"line":    lineNum,
				"content": strings.TrimSpace(line),
			})

			if len(matches) >= maxMatches {
				break
			}
		}
	}

	return matches
}

// AppendFileTool appends content to a file
type AppendFileTool struct {
	BaseTool
	workingDir string
}

// NewAppendFileTool creates a new append file tool
func NewAppendFileTool(workingDir string) *AppendFileTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path to append to",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to append to the file",
			},
		},
		"required": []string{"path", "content"},
	}

	return &AppendFileTool{
		BaseTool: NewBaseTool(
			"append_file",
			"Append content to the end of a file. Creates the file if it doesn't exist.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *AppendFileTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content parameter is required")
	}

	// Resolve path relative to working directory
	fullPath := path
	if t.workingDir != "" && !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "~") {
		fullPath = filepath.Join(t.workingDir, path)
	}

	// Handle ~ expansion
	if strings.HasPrefix(fullPath, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			fullPath = filepath.Join(home, fullPath[1:])
		}
	}

	// Create parent directories if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return map[string]interface{}{
			"path":    path,
			"error":   fmt.Sprintf("failed to create directories: %v", err),
			"success": false,
		}, nil
	}

	// Open file for appending
	file, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return map[string]interface{}{
			"path":    path,
			"error":   err.Error(),
			"success": false,
		}, nil
	}
	defer file.Close()

	// Write content
	if _, err := file.WriteString(content); err != nil {
		return map[string]interface{}{
			"path":    path,
			"error":   err.Error(),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"path":          path,
		"bytes_written": len(content),
		"success":       true,
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// GIT TOOLS
// ═══════════════════════════════════════════════════════════════════════════════

// GitStatusTool shows git working tree status
type GitStatusTool struct {
	BaseTool
	workingDir string
}

// NewGitStatusTool creates a git status tool
func NewGitStatusTool(workingDir string) *GitStatusTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"short": map[string]interface{}{
				"type":        "boolean",
				"description": "Use short format output (default: false)",
			},
		},
	}

	return &GitStatusTool{
		BaseTool: NewBaseTool(
			"git_status",
			"Show git working tree status. Returns staged, unstaged, and untracked files.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitStatusTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// Build command
	cmdArgs := []string{"status", "--porcelain=v1"}

	short := false
	if s, ok := args["short"].(bool); ok {
		short = s
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git status failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	// Parse porcelain output
	var staged, unstaged, untracked []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		x, y := line[0], line[1]
		file := strings.TrimSpace(line[3:])

		if x == '?' {
			untracked = append(untracked, file)
		} else {
			if x != ' ' {
				staged = append(staged, file)
			}
			if y != ' ' {
				unstaged = append(unstaged, file)
			}
		}
	}

	// Get branch info
	branchCmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	if t.workingDir != "" {
		branchCmd.Dir = t.workingDir
	}
	branchOut, _ := branchCmd.Output()
	branch := strings.TrimSpace(string(branchOut))

	result := map[string]interface{}{
		"branch":    branch,
		"staged":    staged,
		"unstaged":  unstaged,
		"untracked": untracked,
		"clean":     len(staged) == 0 && len(unstaged) == 0 && len(untracked) == 0,
		"success":   true,
	}

	if short {
		result["output"] = string(output)
	}

	return result, nil
}

// GitDiffTool shows git diff
type GitDiffTool struct {
	BaseTool
	workingDir string
}

// NewGitDiffTool creates a git diff tool
func NewGitDiffTool(workingDir string) *GitDiffTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"staged": map[string]interface{}{
				"type":        "boolean",
				"description": "Show staged changes (--cached)",
			},
			"file": map[string]interface{}{
				"type":        "string",
				"description": "Specific file to diff",
			},
			"commit": map[string]interface{}{
				"type":        "string",
				"description": "Compare with specific commit (e.g., HEAD~1, main)",
			},
			"stat": map[string]interface{}{
				"type":        "boolean",
				"description": "Show diffstat only (file changes summary)",
			},
		},
	}

	return &GitDiffTool{
		BaseTool: NewBaseTool(
			"git_diff",
			"Show changes between commits, commit and working tree, etc.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitDiffTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	cmdArgs := []string{"diff", "--no-color"}

	if staged, ok := args["staged"].(bool); ok && staged {
		cmdArgs = append(cmdArgs, "--cached")
	}

	if stat, ok := args["stat"].(bool); ok && stat {
		cmdArgs = append(cmdArgs, "--stat")
	}

	if commit, ok := args["commit"].(string); ok && commit != "" {
		cmdArgs = append(cmdArgs, commit)
	}

	if file, ok := args["file"].(string); ok && file != "" {
		cmdArgs = append(cmdArgs, "--", file)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git diff failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	// Count changed files from stat
	diffOutput := string(output)
	changedFiles := 0
	additions := 0
	deletions := 0

	// Parse stat output if present
	lines := strings.Split(diffOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, " | ") {
			changedFiles++
		}
		if strings.Contains(line, "insertion") || strings.Contains(line, "deletion") {
			// Parse summary line
			parts := strings.Fields(line)
			for i, p := range parts {
				if strings.HasPrefix(p, "insertion") && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &additions)
				}
				if strings.HasPrefix(p, "deletion") && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &deletions)
				}
			}
		}
	}

	return map[string]interface{}{
		"diff":      diffOutput,
		"files":     changedFiles,
		"additions": additions,
		"deletions": deletions,
		"success":   true,
	}, nil
}

// GitCommitTool commits changes
type GitCommitTool struct {
	BaseTool
	workingDir string
}

// NewGitCommitTool creates a git commit tool
func NewGitCommitTool(workingDir string) *GitCommitTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Commit message (required)",
			},
			"files": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Specific files to commit (default: all staged)",
			},
			"all": map[string]interface{}{
				"type":        "boolean",
				"description": "Stage all modified files before commit (-a)",
			},
		},
		"required": []string{"message"},
	}

	return &GitCommitTool{
		BaseTool: NewBaseTool(
			"git_commit",
			"Record changes to the repository. Stages files and commits with message.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitCommitTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	message, ok := args["message"].(string)
	if !ok || message == "" {
		return nil, fmt.Errorf("message parameter is required")
	}

	// Stage files if specified
	if files, ok := args["files"].([]interface{}); ok && len(files) > 0 {
		addArgs := []string{"add"}
		for _, f := range files {
			if s, ok := f.(string); ok {
				addArgs = append(addArgs, s)
			}
		}
		addCmd := exec.CommandContext(ctx, "git", addArgs...)
		if t.workingDir != "" {
			addCmd.Dir = t.workingDir
		}
		if out, err := addCmd.CombinedOutput(); err != nil {
			return map[string]interface{}{
				"error":   fmt.Sprintf("git add failed: %v", err),
				"output":  string(out),
				"success": false,
			}, nil
		}
	}

	// Build commit command
	cmdArgs := []string{"commit", "-m", message}

	if all, ok := args["all"].(bool); ok && all {
		cmdArgs = append(cmdArgs, "-a")
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git commit failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	// Get commit hash
	hashCmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	if t.workingDir != "" {
		hashCmd.Dir = t.workingDir
	}
	hashOut, _ := hashCmd.Output()
	hash := strings.TrimSpace(string(hashOut))

	return map[string]interface{}{
		"message": message,
		"hash":    hash,
		"output":  string(output),
		"success": true,
	}, nil
}

// GitPushTool pushes commits to remote
type GitPushTool struct {
	BaseTool
	workingDir string
}

// NewGitPushTool creates a git push tool
func NewGitPushTool(workingDir string) *GitPushTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"remote": map[string]interface{}{
				"type":        "string",
				"description": "Remote name (default: origin)",
			},
			"branch": map[string]interface{}{
				"type":        "string",
				"description": "Branch name (default: current branch)",
			},
			"set_upstream": map[string]interface{}{
				"type":        "boolean",
				"description": "Set upstream tracking (-u)",
			},
		},
	}

	return &GitPushTool{
		BaseTool: NewBaseTool(
			"git_push",
			"Push commits to remote repository.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitPushTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	remote := "origin"
	if r, ok := args["remote"].(string); ok && r != "" {
		remote = r
	}

	cmdArgs := []string{"push"}

	if setUpstream, ok := args["set_upstream"].(bool); ok && setUpstream {
		cmdArgs = append(cmdArgs, "-u")
	}

	cmdArgs = append(cmdArgs, remote)

	if branch, ok := args["branch"].(string); ok && branch != "" {
		cmdArgs = append(cmdArgs, branch)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git push failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"remote":  remote,
		"output":  string(output),
		"success": true,
	}, nil
}

// GitLogTool shows commit history
type GitLogTool struct {
	BaseTool
	workingDir string
}

// NewGitLogTool creates a git log tool
func NewGitLogTool(workingDir string) *GitLogTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of commits to show (default: 10)",
			},
			"oneline": map[string]interface{}{
				"type":        "boolean",
				"description": "Show one line per commit (default: true)",
			},
			"file": map[string]interface{}{
				"type":        "string",
				"description": "Show commits affecting specific file",
			},
		},
	}

	return &GitLogTool{
		BaseTool: NewBaseTool(
			"git_log",
			"Show commit history.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitLogTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	count := 10
	if c, ok := args["count"].(float64); ok {
		count = int(c)
	}

	oneline := true
	if o, ok := args["oneline"].(bool); ok {
		oneline = o
	}

	cmdArgs := []string{"log", fmt.Sprintf("-n%d", count)}

	if oneline {
		cmdArgs = append(cmdArgs, "--oneline")
	} else {
		cmdArgs = append(cmdArgs, "--format=%H|%an|%ar|%s")
	}

	if file, ok := args["file"].(string); ok && file != "" {
		cmdArgs = append(cmdArgs, "--", file)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git log failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	// Parse commits
	var commits []map[string]interface{}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if oneline {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) >= 2 {
				commits = append(commits, map[string]interface{}{
					"hash":    parts[0],
					"message": parts[1],
				})
			}
		} else {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) >= 4 {
				commits = append(commits, map[string]interface{}{
					"hash":    parts[0],
					"author":  parts[1],
					"date":    parts[2],
					"message": parts[3],
				})
			}
		}
	}

	return map[string]interface{}{
		"commits": commits,
		"count":   len(commits),
		"success": true,
	}, nil
}

// GitAddTool stages files
type GitAddTool struct {
	BaseTool
	workingDir string
}

// NewGitAddTool creates a git add tool
func NewGitAddTool(workingDir string) *GitAddTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"files": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Files to stage (use ['.'] for all)",
			},
			"all": map[string]interface{}{
				"type":        "boolean",
				"description": "Stage all changes (-A)",
			},
		},
	}

	return &GitAddTool{
		BaseTool: NewBaseTool(
			"git_add",
			"Stage files for commit.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitAddTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	cmdArgs := []string{"add"}

	if all, ok := args["all"].(bool); ok && all {
		cmdArgs = append(cmdArgs, "-A")
	} else if files, ok := args["files"].([]interface{}); ok && len(files) > 0 {
		for _, f := range files {
			if s, ok := f.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	} else {
		return map[string]interface{}{
			"error":   "either 'files' or 'all: true' is required",
			"success": false,
		}, nil
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git add failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"output":  string(output),
		"success": true,
	}, nil
}

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

	// Simple line-by-line diff (not a full unified diff algorithm, but useful)
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
				// Trim extra context and close hunk
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
