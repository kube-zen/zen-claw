package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "zen-claw",
	Short: "Zen Claw - Multi-AI assistant with consensus and factory modes",
	Long: `Zen Claw is a multi-model AI assistant for software development.

Modes:
  agent      Interactive AI agent with tool execution
  consensus  Multi-AI consensus for better blueprints (3+ AIs → arbiter → synthesis)
  factory    Software factory with AI specialists (Go, TypeScript, Infrastructure)

Features:
  • Multi-provider support (DeepSeek, Qwen, MiniMax, Kimi, OpenAI, GLM)
  • Parallel AI execution for speed
  • Consensus engine: Diverse AI perspectives → superior results
  • Factory mode: Coordinator + specialist workers for complex projects
  • Guardrails for safe autonomous execution
  • Session persistence and resumption`,
	Run: func(cmd *cobra.Command, args []string) {
		// Display capabilities at startup
		displayCapabilities()
		cmd.Help()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(newAgentCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newConsensusCmd())
	rootCmd.AddCommand(newFactoryCmd())
	rootCmd.AddCommand(newGatewayCmd())
	rootCmd.AddCommand(newSessionCmd())
	rootCmd.AddCommand(newSlackCmd())
	rootCmd.AddCommand(newToolsCmd())
}

// displayCapabilities shows the AI's capabilities at startup
func displayCapabilities() {
	fmt.Println("🚀 Zen Claw Capabilities")
	fmt.Println("========================")

	// Try to load capabilities from ~/.zen/zen-claw/capabilities
	capabilitiesDir := filepath.Join(os.Getenv("HOME"), ".zen", "zen-claw", "capabilities")

	if _, err := os.Stat(capabilitiesDir); os.IsNotExist(err) {
		// Directory doesn't exist, skip
		return
	}

	// Read all markdown files in capabilities directory
	files, err := os.ReadDir(capabilitiesDir)
	if err != nil {
		// Skip if we can't read the directory
		return
	}

	// Sort files for consistent display
	var mdFiles []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".md") {
			mdFiles = append(mdFiles, file.Name())
		}
	}

	// Display each capability file
	for _, fileName := range mdFiles {
		filePath := filepath.Join(capabilitiesDir, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Display the content (skip the first line which is the title)
		lines := strings.Split(string(content), "\n")
		if len(lines) > 1 {
			// Print the first line (title) as heading
			fmt.Printf("\n%s\n", strings.TrimSpace(lines[0]))
			fmt.Println(strings.Repeat("-", len(strings.TrimSpace(lines[0]))))

			// Print the rest of the content
			for i := 1; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) != "" {
					fmt.Printf("%s\n", strings.TrimSpace(lines[i]))
				}
			}
		}
	}

	fmt.Println()
}
