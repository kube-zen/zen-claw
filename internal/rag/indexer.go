package rag

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// FileInfo represents indexed file information
type FileInfo struct {
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Hash      string    `json:"hash"`
	Language  string    `json:"language"`
	Symbols   []string  `json:"symbols"` // Functions, classes, types
	Imports   []string  `json:"imports"` // Dependencies
	UpdatedAt time.Time `json:"updated_at"`
}

// Indexer manages codebase indexing
type Indexer struct {
	db       *sql.DB
	dbPath   string
	rootDir  string
	patterns []string // File patterns to index
	excludes []string // Patterns to exclude
}

// IndexerConfig configures the indexer
type IndexerConfig struct {
	DBPath   string   // Path to index database
	RootDir  string   // Root directory to index
	Patterns []string // File patterns (e.g., "*.go", "*.ts")
	Excludes []string // Exclude patterns (e.g., "vendor/*", "node_modules/*")
}

// DefaultIndexDBPath returns the default index database path
func DefaultIndexDBPath(projectName string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zen", "zen-claw", "index", projectName+".db")
}

// NewIndexer creates a new codebase indexer
func NewIndexer(cfg *IndexerConfig) (*Indexer, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0755); err != nil {
		return nil, fmt.Errorf("create index dir: %w", err)
	}

	// Open SQLite database
	db, err := sql.Open("sqlite3", cfg.DBPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Create tables
	if err := createIndexTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	// Default patterns
	patterns := cfg.Patterns
	if len(patterns) == 0 {
		patterns = []string{
			"*.go", "*.py", "*.js", "*.ts", "*.tsx", "*.jsx",
			"*.java", "*.rs", "*.rb", "*.c", "*.cpp", "*.h",
			"*.yaml", "*.yml", "*.json", "*.toml", "*.md",
		}
	}

	excludes := cfg.Excludes
	if len(excludes) == 0 {
		excludes = []string{
			"vendor/*", "node_modules/*", ".git/*", "dist/*", "build/*",
			"*.min.js", "*.min.css", "*.lock", "go.sum",
		}
	}

	return &Indexer{
		db:       db,
		dbPath:   cfg.DBPath,
		rootDir:  cfg.RootDir,
		patterns: patterns,
		excludes: excludes,
	}, nil
}

func createIndexTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT UNIQUE NOT NULL,
		size INTEGER,
		hash TEXT,
		language TEXT,
		symbols TEXT,
		imports TEXT,
		content_preview TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
	CREATE INDEX IF NOT EXISTS idx_files_language ON files(language);

	CREATE VIRTUAL TABLE IF NOT EXISTS files_fts USING fts5(
		path, symbols, imports, content_preview,
		content='files',
		content_rowid='id'
	);

	CREATE TRIGGER IF NOT EXISTS files_ai AFTER INSERT ON files BEGIN
		INSERT INTO files_fts(rowid, path, symbols, imports, content_preview)
		VALUES (new.id, new.path, new.symbols, new.imports, new.content_preview);
	END;

	CREATE TRIGGER IF NOT EXISTS files_ad AFTER DELETE ON files BEGIN
		INSERT INTO files_fts(files_fts, rowid, path, symbols, imports, content_preview)
		VALUES ('delete', old.id, old.path, old.symbols, old.imports, old.content_preview);
	END;

	CREATE TRIGGER IF NOT EXISTS files_au AFTER UPDATE ON files BEGIN
		INSERT INTO files_fts(files_fts, rowid, path, symbols, imports, content_preview)
		VALUES ('delete', old.id, old.path, old.symbols, old.imports, old.content_preview);
		INSERT INTO files_fts(rowid, path, symbols, imports, content_preview)
		VALUES (new.id, new.path, new.symbols, new.imports, new.content_preview);
	END;

	CREATE TABLE IF NOT EXISTS metadata (
		key TEXT PRIMARY KEY,
		value TEXT
	);
	`
	_, err := db.Exec(schema)
	return err
}

// Index indexes the codebase
func (idx *Indexer) Index() (int, error) {
	count := 0
	start := time.Now()

	err := filepath.WalkDir(idx.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Get relative path
		relPath, _ := filepath.Rel(idx.rootDir, path)

		// Skip excluded paths
		if idx.isExcluded(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// Check if matches patterns
		if !idx.matchesPattern(relPath) {
			return nil
		}

		// Index the file
		if err := idx.indexFile(path, relPath); err != nil {
			log.Printf("[RAG] Warning: failed to index %s: %v", relPath, err)
			return nil
		}

		count++
		return nil
	})

	if err != nil {
		return count, err
	}

	// Update metadata
	idx.db.Exec(`INSERT OR REPLACE INTO metadata (key, value) VALUES ('last_indexed', ?)`,
		time.Now().Format(time.RFC3339))
	idx.db.Exec(`INSERT OR REPLACE INTO metadata (key, value) VALUES ('file_count', ?)`,
		fmt.Sprintf("%d", count))
	idx.db.Exec(`INSERT OR REPLACE INTO metadata (key, value) VALUES ('index_duration', ?)`,
		time.Since(start).String())

	return count, nil
}

func (idx *Indexer) indexFile(absPath, relPath string) error {
	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	// Calculate hash
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:8]) // First 8 bytes

	// Detect language
	lang := detectLanguage(relPath)

	// Extract symbols and imports
	symbols := extractSymbols(string(content), lang)
	imports := extractImports(string(content), lang)

	// Create content preview (first 500 chars, cleaned)
	preview := createPreview(string(content), 500)

	// Upsert into database
	_, err = idx.db.Exec(`
		INSERT INTO files (path, size, hash, language, symbols, imports, content_preview, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			size = excluded.size,
			hash = excluded.hash,
			language = excluded.language,
			symbols = excluded.symbols,
			imports = excluded.imports,
			content_preview = excluded.content_preview,
			updated_at = excluded.updated_at
	`, relPath, info.Size(), hashStr, lang,
		strings.Join(symbols, " "),
		strings.Join(imports, " "),
		preview,
		time.Now())

	return err
}

func (idx *Indexer) matchesPattern(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range idx.patterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

func (idx *Indexer) isExcluded(path string) bool {
	for _, pattern := range idx.excludes {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Also check directory prefix
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(path, prefix+"/") || path == prefix {
				return true
			}
		}
	}
	return false
}

// Search searches the index for relevant files
func (idx *Indexer) Search(query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	// Use FTS5 for full-text search
	rows, err := idx.db.Query(`
		SELECT f.path, f.language, f.symbols, f.imports, f.content_preview,
		       bm25(files_fts) as score
		FROM files_fts
		JOIN files f ON f.id = files_fts.rowid
		WHERE files_fts MATCH ?
		ORDER BY score
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var symbols, imports string
		if err := rows.Scan(&r.Path, &r.Language, &symbols, &imports, &r.Preview, &r.Score); err != nil {
			continue
		}
		r.Symbols = strings.Fields(symbols)
		r.Imports = strings.Fields(imports)
		results = append(results, r)
	}

	return results, nil
}

// SearchResult represents a search result
type SearchResult struct {
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Symbols  []string `json:"symbols"`
	Imports  []string `json:"imports"`
	Preview  string   `json:"preview"`
	Score    float64  `json:"score"`
}

// GetStats returns index statistics
func (idx *Indexer) GetStats() map[string]string {
	stats := make(map[string]string)

	rows, err := idx.db.Query("SELECT key, value FROM metadata")
	if err != nil {
		return stats
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err == nil {
			stats[key] = value
		}
	}

	// Count by language
	langRows, err := idx.db.Query(`
		SELECT language, COUNT(*) as count 
		FROM files 
		GROUP BY language 
		ORDER BY count DESC
	`)
	if err == nil {
		defer langRows.Close()
		var langs []string
		for langRows.Next() {
			var lang string
			var count int
			if langRows.Scan(&lang, &count) == nil {
				langs = append(langs, fmt.Sprintf("%s:%d", lang, count))
			}
		}
		stats["languages"] = strings.Join(langs, ", ")
	}

	return stats
}

// Close closes the indexer
func (idx *Indexer) Close() error {
	if idx.db != nil {
		return idx.db.Close()
	}
	return nil
}

// Helper functions

func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescript"
	case ".jsx":
		return "javascript"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc":
		return "cpp"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	case ".toml":
		return "toml"
	default:
		return "text"
	}
}

var symbolPatterns = map[string][]*regexp.Regexp{
	"go": {
		regexp.MustCompile(`func\s+(\w+)\s*\(`),
		regexp.MustCompile(`func\s+\([^)]+\)\s+(\w+)\s*\(`),
		regexp.MustCompile(`type\s+(\w+)\s+(?:struct|interface)`),
	},
	"python": {
		regexp.MustCompile(`def\s+(\w+)\s*\(`),
		regexp.MustCompile(`class\s+(\w+)`),
	},
	"javascript": {
		regexp.MustCompile(`function\s+(\w+)\s*\(`),
		regexp.MustCompile(`(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?\(`),
		regexp.MustCompile(`class\s+(\w+)`),
	},
	"typescript": {
		regexp.MustCompile(`function\s+(\w+)\s*[<(]`),
		regexp.MustCompile(`(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?\(`),
		regexp.MustCompile(`class\s+(\w+)`),
		regexp.MustCompile(`interface\s+(\w+)`),
		regexp.MustCompile(`type\s+(\w+)\s*=`),
	},
}

func extractSymbols(content, lang string) []string {
	patterns, ok := symbolPatterns[lang]
	if !ok {
		return nil
	}

	seen := make(map[string]bool)
	var symbols []string

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				sym := match[1]
				if !seen[sym] && len(sym) > 1 {
					seen[sym] = true
					symbols = append(symbols, sym)
				}
			}
		}
	}

	return symbols
}

var importPatterns = map[string]*regexp.Regexp{
	"go":         regexp.MustCompile(`import\s+(?:\w+\s+)?"([^"]+)"`),
	"python":     regexp.MustCompile(`(?:from\s+(\S+)\s+import|import\s+(\S+))`),
	"javascript": regexp.MustCompile(`(?:import|require)\s*\(?['"]([^'"]+)['"]`),
	"typescript": regexp.MustCompile(`(?:import|require)\s*\(?['"]([^'"]+)['"]`),
}

func extractImports(content, lang string) []string {
	pattern, ok := importPatterns[lang]
	if !ok {
		return nil
	}

	seen := make(map[string]bool)
	var imports []string

	matches := pattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		for i := 1; i < len(match); i++ {
			if match[i] != "" && !seen[match[i]] {
				seen[match[i]] = true
				imports = append(imports, match[i])
			}
		}
	}

	return imports
}

func createPreview(content string, maxLen int) string {
	// Remove comments and empty lines
	scanner := bufio.NewScanner(strings.NewReader(content))
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
		if len(strings.Join(lines, " ")) > maxLen {
			break
		}
	}

	preview := strings.Join(lines, " ")
	if len(preview) > maxLen {
		preview = preview[:maxLen] + "..."
	}

	return preview
}
