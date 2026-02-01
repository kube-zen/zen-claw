package agent

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/neves/zen-claw/internal/session"
	"github.com/neves/zen-claw/internal/tools"
)

type Config struct {
	Model     string
	Workspace string
	Thinking  bool
	SkillsDir string
}

type Agent struct {
	config  Config
	session *session.Session
	tools   *tools.Manager
}

func New(config Config) (*Agent, error) {
	// Create session
	sess := session.New(session.Config{
		Workspace: config.Workspace,
		Model:     config.Model,
	})

	// Create tool manager
	toolMgr, err := tools.NewManager(tools.Config{
		Workspace: config.Workspace,
		Session:   sess,
	})
	if err != nil {
		return nil, fmt.Errorf("create tool manager: %w", err)
	}

	return &Agent{
		config:  config,
		session: sess,
		tools:   toolMgr,
	}, nil
}

func (a *Agent) RunTask(task string) error {
	fmt.Printf("🧠 Running task: %s\n\n", task)

	// TODO: Implement actual AI interaction
	// For now, just echo the task and show available tools
	fmt.Println("Available tools:")
	for _, tool := range a.tools.List() {
		fmt.Printf("  • %s\n", tool)
	}
	fmt.Println()

	return nil
}

func (a *Agent) RunInteractive() error {
	fmt.Println("🧠 Zen Claw Interactive Mode")
	fmt.Println("Type 'quit' or 'exit' to end, 'help' for commands")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch strings.ToLower(input) {
		case "quit", "exit":
			fmt.Println("Goodbye!")
			return nil
		case "help":
			printHelp()
		case "tools":
			printTools(a.tools)
		default:
			// Treat as a task
			if err := a.RunTask(input); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}
	}

	return scanner.Err()
}

func printHelp() {
	fmt.Println("Commands (use / prefix):")
	fmt.Println("  /exit    - Exit interactive mode")
	fmt.Println("  /stop    - Stop current request")
	fmt.Println("  /pause   - Pause current request (not yet implemented)")
	fmt.Println("  /resume  - Resume paused request (not yet implemented)")
	fmt.Println("  /help    - Show this help")
	fmt.Println("  /tools   - List available tools")
	fmt.Println("  /session - Show current session information")
	fmt.Println("  /sessions- List all saved sessions")
	fmt.Println("  /skills  - List available skills")
	fmt.Println("  /status  - Show system status")
	fmt.Println("  <task>   - Run a task (no prefix needed)")
	fmt.Println()
}

func printTools(toolMgr *tools.Manager) {
	fmt.Println("Available tools:")
	for _, tool := range toolMgr.List() {
		fmt.Printf("  • %s\n", tool)
	}
	fmt.Println()
}