package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/neves/zen-claw/internal/plugins"
	"github.com/spf13/cobra"
)

func newPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "Manage plugins",
		Long: `Manage zen-claw plugins.

Plugins are custom tools that extend zen-claw's capabilities.
Each plugin is a directory with a plugin.yaml manifest.

Plugin directory: ~/.zen/zen-claw/plugins/`,
	}

	cmd.AddCommand(newPluginsListCmd())
	cmd.AddCommand(newPluginsInitCmd())
	cmd.AddCommand(newPluginsInfoCmd())

	return cmd
}

func newPluginsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		Run: func(cmd *cobra.Command, args []string) {
			loader := plugins.NewLoader()
			if err := loader.LoadAll(); err != nil {
				fmt.Printf("Error loading plugins: %v\n", err)
				return
			}

			infos := loader.ListPlugins()
			if len(infos) == 0 {
				fmt.Println("No plugins installed")
				fmt.Printf("\nPlugin directory: %s\n", plugins.DefaultPluginDir())
				fmt.Println("Run 'zen-claw plugins init <name>' to create a new plugin")
				return
			}

			fmt.Printf("Installed plugins (%d):\n\n", len(infos))
			for _, info := range infos {
				fmt.Printf("  %s v%s\n", info.Name, info.Version)
				fmt.Printf("    %s\n", info.Description)
				if info.Author != "" {
					fmt.Printf("    Author: %s\n", info.Author)
				}
				fmt.Printf("    Dir: %s\n\n", info.Dir)
			}
		},
	}
}

func newPluginsInitCmd() *cobra.Command {
	var lang string

	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Create a new plugin from template",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			pluginDir := filepath.Join(plugins.DefaultPluginDir(), name)

			// Check if already exists
			if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
				fmt.Printf("Error: Plugin directory already exists: %s\n", pluginDir)
				return
			}

			// Create plugin directory
			if err := os.MkdirAll(pluginDir, 0755); err != nil {
				fmt.Printf("Error creating plugin directory: %v\n", err)
				return
			}

			// Generate files based on language
			switch lang {
			case "bash", "sh":
				createBashPlugin(pluginDir, name)
			case "python", "py":
				createPythonPlugin(pluginDir, name)
			default:
				createBashPlugin(pluginDir, name)
			}

			fmt.Printf("✓ Created plugin: %s\n", name)
			fmt.Printf("  Directory: %s\n", pluginDir)
			fmt.Println("\nNext steps:")
			fmt.Println("  1. Edit plugin.yaml to define parameters")
			fmt.Println("  2. Implement your logic in the script")
			fmt.Println("  3. Restart gateway to load the plugin")
		},
	}

	cmd.Flags().StringVarP(&lang, "lang", "l", "bash", "Script language (bash, python)")
	return cmd
}

func newPluginsInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show plugin system information",
		Run: func(cmd *cobra.Command, args []string) {
			pluginDir := plugins.DefaultPluginDir()

			fmt.Println("Plugin System Information")
			fmt.Println("─────────────────────────")
			fmt.Printf("Plugin directory: %s\n", pluginDir)

			// Check if directory exists
			if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
				fmt.Println("Status: Directory not created yet")
				fmt.Println("\nRun 'zen-claw plugins init <name>' to create your first plugin")
				return
			}

			// Count plugins
			loader := plugins.NewLoader()
			loader.LoadAll()
			fmt.Printf("Installed plugins: %d\n", loader.Count())

			fmt.Println("\nPlugin manifest format (plugin.yaml):")
			fmt.Println(`
  name: my-plugin
  version: 1.0.0
  description: What my plugin does
  command: ./run.sh       # or: python, bash
  args: []                # Additional arguments
  timeout: 30s            # Execution timeout
  parameters:             # JSON Schema
    type: object
    properties:
      input:
        type: string
        description: Input parameter
    required: [input]`)
		},
	}
}

func createBashPlugin(dir, name string) {
	// Create plugin.yaml
	manifest := fmt.Sprintf(`name: %s
version: 1.0.0
description: A custom plugin
command: ./run.sh
timeout: 30s
parameters:
  type: object
  properties:
    input:
      type: string
      description: Input to process
  required: [input]
`, name)

	os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(manifest), 0644)

	// Create run.sh
	script := `#!/bin/bash
# Plugin script - receives JSON via stdin, outputs JSON to stdout

# Read JSON input
INPUT=$(cat)

# Extract parameters (using jq if available, otherwise basic parsing)
if command -v jq &> /dev/null; then
    PARAM=$(echo "$INPUT" | jq -r '.input // empty')
else
    # Basic extraction without jq
    PARAM=$(echo "$INPUT" | grep -o '"input":"[^"]*"' | cut -d'"' -f4)
fi

# Your plugin logic here
RESULT="Processed: $PARAM"

# Output JSON result
echo "{\"result\": \"$RESULT\"}"
`
	os.WriteFile(filepath.Join(dir, "run.sh"), []byte(script), 0755)
}

func createPythonPlugin(dir, name string) {
	// Create plugin.yaml
	manifest := fmt.Sprintf(`name: %s
version: 1.0.0
description: A custom plugin
command: python
args: [run.py]
timeout: 30s
parameters:
  type: object
  properties:
    input:
      type: string
      description: Input to process
  required: [input]
`, name)

	os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(manifest), 0644)

	// Create run.py
	script := `#!/usr/bin/env python3
"""Plugin script - receives JSON via stdin, outputs JSON to stdout"""
import json
import sys

def main():
    # Read JSON input from stdin
    input_data = json.load(sys.stdin)
    
    # Extract parameters
    param = input_data.get('input', '')
    
    # Your plugin logic here
    result = f"Processed: {param}"
    
    # Output JSON result
    print(json.dumps({"result": result}))

if __name__ == "__main__":
    main()
`
	os.WriteFile(filepath.Join(dir, "run.py"), []byte(script), 0755)
}
