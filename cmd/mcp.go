package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcpclient "github.com/neves/zen-claw/internal/mcp"
	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP (Model Context Protocol) servers",
		Long: `Connect to and manage MCP servers to extend zen-claw's capabilities.

MCP servers provide tools, resources, and prompts that can be used by the AI agent.

Examples:
  # Connect to a Go MCP server
  zen-claw mcp connect mytools -- ./my-mcp-server

  # Connect to a Python MCP server  
  zen-claw mcp connect weather -- python weather_server.py

  # List connected servers and their tools
  zen-claw mcp list`,
	}

	cmd.AddCommand(newMCPConnectCmd())
	cmd.AddCommand(newMCPListCmd())

	return cmd
}

func newMCPConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <name> -- <command> [args...]",
		Short: "Connect to an MCP server",
		Long: `Connect to an MCP server by name.

The command after -- is executed to start the MCP server.

Examples:
  zen-claw mcp connect tools -- ./my-mcp-server --port 0
  zen-claw mcp connect weather -- python weather_mcp.py`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Find the "--" separator
			dashIdx := -1
			for i, arg := range args {
				if arg == "--" {
					dashIdx = i
					break
				}
			}

			if dashIdx == -1 || dashIdx >= len(args)-1 {
				fmt.Println("Error: Usage: zen-claw mcp connect <name> -- <command> [args...]")
				return
			}

			name := args[0]
			command := args[dashIdx+1]
			var cmdArgs []string
			if dashIdx+2 < len(args) {
				cmdArgs = args[dashIdx+2:]
			}

			fmt.Printf("Connecting to MCP server '%s'...\n", name)
			fmt.Printf("  Command: %s %s\n", command, strings.Join(cmdArgs, " "))

			client := mcpclient.NewClient()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := client.Connect(ctx, mcpclient.ServerConfig{
				Name:    name,
				Command: command,
				Args:    cmdArgs,
			})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Get tools
			tools := client.GetTools()
			fmt.Printf("\n✅ Connected! Found %d tools:\n", len(tools))
			for _, tool := range tools {
				fmt.Printf("  • %s - %s\n", tool.Name(), tool.Description())
			}

			// Keep connection open for testing
			fmt.Println("\nPress Ctrl+C to disconnect...")
			select {}
		},
	}
}

func newMCPListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show MCP info and how to build servers",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("MCP (Model Context Protocol) - Extend zen-claw with external tools")
			fmt.Println("═" + strings.Repeat("═", 60))
			fmt.Println()
			fmt.Println("MCP servers communicate via stdio (stdin/stdout JSON-RPC).")
			fmt.Println()
			fmt.Println("Build a Go MCP server with github.com/mark3labs/mcp-go:")
			fmt.Println()
			fmt.Println("  package main")
			fmt.Println()
			fmt.Println("  import (")
			fmt.Println("      \"github.com/mark3labs/mcp-go/mcp\"")
			fmt.Println("      \"github.com/mark3labs/mcp-go/server\"")
			fmt.Println("  )")
			fmt.Println()
			fmt.Println("  func main() {")
			fmt.Println("      s := server.NewMCPServer(\"MyTools\", \"1.0.0\")")
			fmt.Println("      s.AddTool(mcp.NewTool(\"hello\", ...))")
			fmt.Println("      server.ServeStdio(s)")
			fmt.Println("  }")
			fmt.Println()
			fmt.Println("Then connect:")
			fmt.Println("  zen-claw mcp connect mytools -- ./my-mcp-server")
			fmt.Println()
			fmt.Println("Or add to config.yaml:")
			fmt.Println("  mcp:")
			fmt.Println("    servers:")
			fmt.Println("      - name: mytools")
			fmt.Println("        command: ./my-mcp-server")
		},
	}
}

