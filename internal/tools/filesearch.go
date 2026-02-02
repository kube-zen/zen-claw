package tools

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FileSearchTool implements a tool for searching files by name or content
type FileSearchTool struct {
	workspace string
}

func (t *FileSearchTool) Name() string { return "filesearch" }
func (t *FileSearchTool) Description() string {
	return "Search files by name or content. Arguments: pattern (string), recursive (bool, default true), content (bool, default false)"
}
func (t *FileSearchTool) Execute(args map[string]interface{}) (interface{}, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern argument required")
	}

	recursive, ok := args["recursive"].(bool)
	if !ok {
		recursive = true
	}

	contentSearch, ok := args["content"].(bool)
	if !ok {
		contentSearch = false
	}

	var results []string

	// Make path absolute relative to workspace if needed
	searchPath := t.workspace
	if !filepath.IsAbs(pattern) {
		searchPath = filepath.Join(t.workspace, pattern)
	}

	// If pattern is a file path, search in its directory
	if strings.Contains(pattern, string(os.PathSeparator)) {
		// Extract directory and filename pattern
		dir := filepath.Dir(pattern)
		filenamePattern := filepath.Base(pattern)
		if dir != "." {
			searchPath = dir
			pattern = filenamePattern
		}
	}

	// Walk the directory tree
	err := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			if !recursive && path != searchPath {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file name matches pattern
		filename := filepath.Base(path)
		if strings.Contains(filename, pattern) {
			if !contentSearch {
				results = append(results, path)
				return nil
			}
		}

		// If content search is requested, read file and search
		if contentSearch && strings.Contains(filename, pattern) {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil // Skip files we can't read
			}
			if strings.Contains(string(content), pattern) {
				results = append(results, path)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "No files found matching the pattern", nil
	}

	return fmt.Sprintf("Found %d files:\n%s", len(results), strings.Join(results, "\n")), nil
}