package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neves/zen-claw/internal/rag"
)

// CodeSearchTool searches the codebase index for relevant files
type CodeSearchTool struct {
	BaseTool
	workingDir string
}

// NewCodeSearchTool creates a new code search tool
func NewCodeSearchTool(workingDir string) *CodeSearchTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query (function names, class names, keywords)",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results (default 10)",
			},
		},
		"required": []string{"query"},
	}

	return &CodeSearchTool{
		BaseTool: NewBaseTool(
			"code_search",
			"Search indexed codebase for relevant files by function names, classes, or keywords",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *CodeSearchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	// Determine project name from working directory
	projectName := filepath.Base(t.workingDir)
	if projectName == "" || projectName == "." {
		cwd, _ := os.Getwd()
		projectName = filepath.Base(cwd)
	}

	dbPath := rag.DefaultIndexDBPath(projectName)

	// Check if index exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return map[string]interface{}{
			"error":   "No index found",
			"message": fmt.Sprintf("Run 'zen-claw index build %s' to create an index", t.workingDir),
		}, nil
	}

	indexer, err := rag.NewIndexer(&rag.IndexerConfig{
		DBPath:  dbPath,
		RootDir: t.workingDir,
	})
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	defer indexer.Close()

	results, err := indexer.Search(query, limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		return map[string]interface{}{
			"results": []interface{}{},
			"message": "No matching files found",
		}, nil
	}

	// Format results
	var formatted []map[string]interface{}
	for _, r := range results {
		formatted = append(formatted, map[string]interface{}{
			"path":     r.Path,
			"language": r.Language,
			"symbols":  r.Symbols,
			"imports":  r.Imports,
			"preview":  r.Preview,
		})
	}

	return map[string]interface{}{
		"results": formatted,
		"count":   len(formatted),
	}, nil
}

// FindSymbolTool finds where a specific symbol is defined
type FindSymbolTool struct {
	BaseTool
	workingDir string
}

// NewFindSymbolTool creates a tool to find symbol definitions
func NewFindSymbolTool(workingDir string) *FindSymbolTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "Symbol name (function, class, type, interface)",
			},
		},
		"required": []string{"symbol"},
	}

	return &FindSymbolTool{
		BaseTool: NewBaseTool(
			"find_symbol",
			"Find where a function, class, or type is defined in the codebase",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *FindSymbolTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, _ := args["symbol"].(string)
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	// Search for the symbol
	projectName := filepath.Base(t.workingDir)
	if projectName == "" || projectName == "." {
		cwd, _ := os.Getwd()
		projectName = filepath.Base(cwd)
	}

	dbPath := rag.DefaultIndexDBPath(projectName)

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return map[string]interface{}{
			"error":   "No index found",
			"message": fmt.Sprintf("Run 'zen-claw index build %s' to create an index", t.workingDir),
		}, nil
	}

	indexer, err := rag.NewIndexer(&rag.IndexerConfig{
		DBPath:  dbPath,
		RootDir: t.workingDir,
	})
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	defer indexer.Close()

	// Search for exact symbol match
	results, err := indexer.Search(symbol, 20)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	// Filter to files that actually contain the symbol
	var matches []map[string]interface{}
	for _, r := range results {
		for _, s := range r.Symbols {
			if strings.EqualFold(s, symbol) {
				matches = append(matches, map[string]interface{}{
					"path":     r.Path,
					"language": r.Language,
					"symbols":  r.Symbols,
				})
				break
			}
		}
	}

	if len(matches) == 0 {
		return map[string]interface{}{
			"found":   false,
			"message": fmt.Sprintf("Symbol '%s' not found in index. Try code_search for broader search.", symbol),
		}, nil
	}

	return map[string]interface{}{
		"found":   true,
		"matches": matches,
		"count":   len(matches),
	}, nil
}

// ContextTool provides relevant context for a query
type ContextTool struct {
	BaseTool
	workingDir string
}

// NewContextTool creates a tool to get relevant context
func NewContextTool(workingDir string) *ContextTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "What you need context about (e.g., 'authentication', 'database models')",
			},
		},
		"required": []string{"query"},
	}

	return &ContextTool{
		BaseTool: NewBaseTool(
			"get_context",
			"Get relevant code context for a topic - finds and reads the most relevant files",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *ContextTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	projectName := filepath.Base(t.workingDir)
	if projectName == "" || projectName == "." {
		cwd, _ := os.Getwd()
		projectName = filepath.Base(cwd)
	}

	dbPath := rag.DefaultIndexDBPath(projectName)

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return map[string]interface{}{
			"error":   "No index found",
			"message": "Index not built. Use read_file or search_files instead.",
		}, nil
	}

	indexer, err := rag.NewIndexer(&rag.IndexerConfig{
		DBPath:  dbPath,
		RootDir: t.workingDir,
	})
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	defer indexer.Close()

	// Search for relevant files
	results, err := indexer.Search(query, 5)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		return map[string]interface{}{
			"context": "No relevant files found in index",
			"files":   []string{},
		}, nil
	}

	// Read contents of top files
	var contextParts []string
	var filePaths []string

	for _, r := range results {
		filePath := filepath.Join(t.workingDir, r.Path)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Truncate large files
		contentStr := string(content)
		if len(contentStr) > 4000 {
			contentStr = contentStr[:4000] + "\n... [truncated]"
		}

		contextParts = append(contextParts, fmt.Sprintf("=== %s ===\n%s", r.Path, contentStr))
		filePaths = append(filePaths, r.Path)
	}

	result := map[string]interface{}{
		"files":   filePaths,
		"context": strings.Join(contextParts, "\n\n"),
	}

	// Pretty print for AI
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonBytes), nil
}
