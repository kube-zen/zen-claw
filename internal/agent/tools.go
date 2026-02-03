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
