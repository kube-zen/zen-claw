package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/neves/zen-claw/internal/rag"
	"github.com/spf13/cobra"
)

func newIndexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index codebase for RAG (retrieval-augmented generation)",
		Long: `Index a codebase to enable intelligent code search.

The index allows the AI to quickly find relevant files based on
function names, class names, imports, and content.

Examples:
  # Index current directory
  zen-claw index .

  # Index a specific project
  zen-claw index ~/projects/my-app

  # Search the index
  zen-claw index search "authentication"`,
	}

	cmd.AddCommand(newIndexBuildCmd())
	cmd.AddCommand(newIndexSearchCmd())
	cmd.AddCommand(newIndexStatsCmd())

	return cmd
}

func newIndexBuildCmd() *cobra.Command {
	var patterns, excludes []string

	cmd := &cobra.Command{
		Use:   "build [path]",
		Short: "Build index for a directory",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			absDir, err := filepath.Abs(dir)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Use directory name as project name
			projectName := filepath.Base(absDir)
			dbPath := rag.DefaultIndexDBPath(projectName)

			fmt.Printf("Indexing: %s\n", absDir)
			fmt.Printf("Database: %s\n", dbPath)

			indexer, err := rag.NewIndexer(&rag.IndexerConfig{
				DBPath:   dbPath,
				RootDir:  absDir,
				Patterns: patterns,
				Excludes: excludes,
			})
			if err != nil {
				fmt.Printf("Error creating indexer: %v\n", err)
				return
			}
			defer indexer.Close()

			count, err := indexer.Index()
			if err != nil {
				fmt.Printf("Error indexing: %v\n", err)
				return
			}

			fmt.Printf("✓ Indexed %d files\n", count)

			// Show stats
			stats := indexer.GetStats()
			if langs, ok := stats["languages"]; ok {
				fmt.Printf("  Languages: %s\n", langs)
			}
			if duration, ok := stats["index_duration"]; ok {
				fmt.Printf("  Duration: %s\n", duration)
			}
		},
	}

	cmd.Flags().StringSliceVarP(&patterns, "pattern", "p", nil, "File patterns to include (e.g., *.go)")
	cmd.Flags().StringSliceVarP(&excludes, "exclude", "e", nil, "Patterns to exclude (e.g., vendor/*)")

	return cmd
}

func newIndexSearchCmd() *cobra.Command {
	var limit int
	var project string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the index",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			query := args[0]

			// Determine project
			if project == "" {
				cwd, _ := os.Getwd()
				project = filepath.Base(cwd)
			}

			dbPath := rag.DefaultIndexDBPath(project)

			// Check if index exists
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Printf("No index found for project '%s'\n", project)
				fmt.Println("Run 'zen-claw index build' first")
				return
			}

			indexer, err := rag.NewIndexer(&rag.IndexerConfig{
				DBPath:  dbPath,
				RootDir: ".", // Not used for search
			})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			defer indexer.Close()

			results, err := indexer.Search(query, limit)
			if err != nil {
				fmt.Printf("Search error: %v\n", err)
				return
			}

			if len(results) == 0 {
				fmt.Println("No results found")
				return
			}

			fmt.Printf("Found %d results for '%s':\n\n", len(results), query)
			for i, r := range results {
				fmt.Printf("%d. %s [%s]\n", i+1, r.Path, r.Language)
				if len(r.Symbols) > 0 {
					syms := r.Symbols
					if len(syms) > 5 {
						syms = syms[:5]
					}
					fmt.Printf("   Symbols: %v\n", syms)
				}
				if r.Preview != "" {
					preview := r.Preview
					if len(preview) > 100 {
						preview = preview[:100] + "..."
					}
					fmt.Printf("   Preview: %s\n", preview)
				}
				fmt.Println()
			}
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "Max results to return")
	cmd.Flags().StringVar(&project, "project", "", "Project name (defaults to current directory name)")

	return cmd
}

func newIndexStatsCmd() *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show index statistics",
		Run: func(cmd *cobra.Command, args []string) {
			// Determine project
			if project == "" {
				cwd, _ := os.Getwd()
				project = filepath.Base(cwd)
			}

			dbPath := rag.DefaultIndexDBPath(project)

			// Check if index exists
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Printf("No index found for project '%s'\n", project)
				fmt.Println("Run 'zen-claw index build' first")
				return
			}

			indexer, err := rag.NewIndexer(&rag.IndexerConfig{
				DBPath:  dbPath,
				RootDir: ".",
			})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			defer indexer.Close()

			stats := indexer.GetStats()

			fmt.Printf("Index Statistics for '%s'\n", project)
			fmt.Println("─────────────────────────────")
			fmt.Printf("Database: %s\n", dbPath)

			if count, ok := stats["file_count"]; ok {
				fmt.Printf("Files indexed: %s\n", count)
			}
			if langs, ok := stats["languages"]; ok {
				fmt.Printf("Languages: %s\n", langs)
			}
			if lastIndexed, ok := stats["last_indexed"]; ok {
				fmt.Printf("Last indexed: %s\n", lastIndexed)
			}
			if duration, ok := stats["index_duration"]; ok {
				fmt.Printf("Index duration: %s\n", duration)
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name (defaults to current directory name)")

	return cmd
}
