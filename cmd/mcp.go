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
  # Connect to an MCP server
  zen-claw mcp connect filesystem -- npx @modelcontextprotocol/server-filesystem /path

  # Connect to a Python MCP server  
  zen-claw mcp connect weather -- python weather_server.py

  # List connected servers and their tools
  zen-claw mcp list

  # Test a specific tool
  zen-claw mcp test filesystem read_file path=/etc/hostname`,
	}

	cmd.AddCommand(newMCPConnectCmd())
	cmd.AddCommand(newMCPListCmd())
	cmd.AddCommand(newMCPTestCmd())

	return cmd
}

func newMCPConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <name> -- <command> [args...]",
		Short: "Connect to an MCP server",
		Long: `Connect to an MCP server by name.

The command after -- is executed to start the MCP server.

Examples:
  zen-claw mcp connect fs -- npx @modelcontextprotocol/server-filesystem /home
  zen-claw mcp connect git -- npx @modelcontextprotocol/server-git
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
		Short: "List available MCP server examples",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Popular MCP Servers:")
			fmt.Println("═" + strings.Repeat("═", 70))
			fmt.Println()
			fmt.Println("  Filesystem - Read/write local files")
			fmt.Println("    npx @modelcontextprotocol/server-filesystem <allowed-path>")
			fmt.Println()
			fmt.Println("  Git - Git operations")
			fmt.Println("    npx @modelcontextprotocol/server-git")
			fmt.Println()
			fmt.Println("  GitHub - GitHub API access")
			fmt.Println("    npx @modelcontextprotocol/server-github")
			fmt.Println()
			fmt.Println("  Fetch - HTTP requests")
			fmt.Println("    npx @modelcontextprotocol/server-fetch")
			fmt.Println()
			fmt.Println("  Memory - Key-value storage")
			fmt.Println("    npx @modelcontextprotocol/server-memory")
			fmt.Println()
			fmt.Println("  Brave Search - Web search")
			fmt.Println("    npx @modelcontextprotocol/server-brave-search")
			fmt.Println()
			fmt.Println("Usage:")
			fmt.Println("  zen-claw mcp connect <name> -- <command from above>")
		},
	}
}

func newMCPTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <server> <tool> [key=value...]",
		Short: "Test an MCP tool",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Note: MCP tool testing requires an active connection.")
			fmt.Println("Use 'zen-claw mcp connect' first, then test in agent mode.")
			fmt.Println()
			fmt.Println("Example workflow:")
			fmt.Println("  1. zen-claw mcp connect fs -- npx @modelcontextprotocol/server-filesystem /home")
			fmt.Println("  2. zen-claw agent  # MCP tools will be available")
		},
	}
}
