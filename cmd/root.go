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
	Short: "AI assistant - single AI (like Cursor) or multi-AI for consensus/factory",
	Long: `Zen Claw - AI assistant for software development.

Modes:
  agent      Single AI, fresh context (like Cursor) - DEFAULT
  consensus  3 AIs → arbiter → better blueprints
  factory    Coordinator + specialist AIs for complex projects

Quick start:
  zen-claw agent                    # Interactive mode, single AI
  zen-claw agent "fix this bug"     # One-shot task
  zen-claw consensus "design X"     # Get 3 AI perspectives + synthesis
  zen-claw factory start plan.yaml  # Run multi-phase project

Providers: DeepSeek, Qwen, MiniMax, Kimi, OpenAI, GLM`,
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
	rootCmd.AddCommand(newFabricCmd())
	rootCmd.AddCommand(newFactoryCmd())
	rootCmd.AddCommand(newGatewayCmd())
	rootCmd.AddCommand(newMCPCmd())
	rootCmd.AddCommand(newSessionsCmd())
	rootCmd.AddCommand(newSessionCmd())
	rootCmd.AddCommand(newPluginsCmd())
	rootCmd.AddCommand(newIndexCmd())
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
