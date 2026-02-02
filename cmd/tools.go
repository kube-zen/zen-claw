package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List available tools",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Available tools:")
			fmt.Println("  • read - Read file contents")
			fmt.Println("  • write - Create or overwrite files")
			fmt.Println("  • edit - Make precise edits to files")
			fmt.Println("  • exec - Run shell commands")
			fmt.Println("  • process - Manage background exec sessions")
			fmt.Println("  • web_search - Search the web")
			fmt.Println("  • web_fetch - Fetch content from URLs")
			fmt.Println("  • memory_search - Search memory")
			fmt.Println("  • memory_get - Read memory snippets")
			fmt.Println("  • sessions_spawn - Spawn sub-agents")
			fmt.Println("  • sessions_list - List sessions")
			fmt.Println("  • sessions_send - Send message to session")
			fmt.Println("  • cron - Manage cron jobs")
			fmt.Println("  • message - Send messages")
			fmt.Println("  • tts - Text to speech")
		},
	}

	return cmd
}
