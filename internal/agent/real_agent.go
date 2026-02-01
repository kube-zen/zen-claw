package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/session"
	"github.com/neves/zen-claw/internal/tools"
)

type RealAgent struct {
	provider ai.Provider
	toolMgr  *tools.Manager
	session  *session.Session
	config   Config
}

func NewRealAgent(config Config, provider ai.Provider, toolMgr *tools.Manager, sess *session.Session) *RealAgent {
	return &RealAgent{
		provider: provider,
		toolMgr:  toolMgr,
		session:  sess,
		config:   config,
	}
}

func (a *RealAgent) RunTask(task string) error {
	return a.RunTaskWithContext(context.Background(), task)
}

func (a *RealAgent) RunTaskWithContext(ctx context.Context, task string) error {
	fmt.Printf("🧠 Running task: %s\n\n", task)

	// Add user message to session
	a.session.AddMessage("user", task)

	// Get AI response with context
	response, err := a.processWithAIWithContext(ctx, task)
	if err != nil {
		return fmt.Errorf("AI processing failed: %w", err)
	}

	// Print the response
	a.printFormattedResponse(response)

	// Add assistant response to session
	a.session.AddMessage("assistant", response)

	// Save session
	if err := a.session.Save(); err != nil {
		fmt.Printf("Warning: failed to save session: %v\n", err)
	}

	return nil
}

func (a *RealAgent) processWithAI(input string) (string, error) {
	return a.processWithAIWithContext(context.Background(), input)
}

func (a *RealAgent) processWithAIWithContext(ctx context.Context, input string) (string, error) {
	// Get available tools
	toolList := a.toolMgr.List()
	
	// Convert tools to AI tool definitions
	var toolDefs []ai.ToolDefinition
	for _, toolName := range toolList {
		tool, ok := a.toolMgr.Get(toolName)
		if !ok {
			continue
		}
		
		toolDefs = append(toolDefs, ai.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					// Basic structure - would be expanded per tool
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Command to execute",
					},
				},
				"required": []string{"command"},
			},
		})
	}

	// Get session transcript
	transcript := a.session.GetTranscript()
	var messages []ai.Message
	
	// Add system message
	messages = append(messages, ai.Message{
		Role: "system",
		Content: `You are Zen Claw, a helpful AI assistant with access to tools.
You can read files, write files, edit files, and execute commands.
Be concise and helpful. Use tools when needed.`,
	})
	
	// Add transcript history (last 10 messages)
	start := len(transcript) - 10
	if start < 0 {
		start = 0
	}
	for i := start; i < len(transcript); i++ {
		messages = append(messages, ai.Message{
			Role:    transcript[i].Role,
			Content: transcript[i].Content,
		})
	}
	
	// Add current user input
	messages = append(messages, ai.Message{
		Role:    "user",
		Content: input,
	})

	// Create chat request
	req := ai.ChatRequest{
		Model:       a.config.Model,
		Messages:    messages,
		Tools:       toolDefs,
		Thinking:    a.config.Thinking,
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	// Get AI response
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := a.provider.Chat(ctx, req)
	if err != nil {
		return "", fmt.Errorf("AI chat failed: %w", err)
	}

	// Handle tool calls if any
	if len(resp.ToolCalls) > 0 {
		return a.handleToolCalls(resp.ToolCalls)
	}

	return resp.Content, nil
}

func (a *RealAgent) handleToolCalls(toolCalls []ai.ToolCall) (string, error) {
	var results []string
	
	for _, call := range toolCalls {
		fmt.Printf("🔧 Calling tool: %s\n", call.Name)
		
		// Execute tool
		result, err := a.toolMgr.Execute(call.Name, call.Args)
		if err != nil {
			results = append(results, fmt.Sprintf("Tool %s failed: %v", call.Name, err))
			// Add error to session
			a.session.AddMessage("tool", fmt.Sprintf("Error: %v", err))
			continue
		}

		resultStr := fmt.Sprintf("%v", result)
		results = append(results, fmt.Sprintf("Tool %s: %s", call.Name, resultStr))
		
		// Add tool result to session
		a.session.AddMessage("tool", resultStr)
		
		// If it was a file operation, show it
		if strings.Contains(call.Name, "read") || strings.Contains(call.Name, "write") || strings.Contains(call.Name, "edit") {
			fmt.Printf("   📝 %s\n", resultStr)
		}
	}

	return strings.Join(results, "\n"), nil
}

func (a *RealAgent) printFormattedResponse(response string) {
	// Try to parse as JSON first
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(response), &jsonObj); err == nil {
		// It's JSON, pretty print it
		fmt.Println("🤖 Tool result:")
		prettyJSON, err := json.MarshalIndent(jsonObj, "  ", "  ")
		if err != nil {
			fmt.Printf("🤖 %s\n\n", response)
			return
		}
		fmt.Println(string(prettyJSON))
		fmt.Println()
		return
	}
	
	// Not JSON, print as-is
	fmt.Printf("🤖 %s\n\n", response)
}

func (a *RealAgent) RunInteractive() error {
	fmt.Println("🧠 Zen Claw Interactive Mode")
	fmt.Printf("Model: %s\n", a.config.Model)
	fmt.Printf("Workspace: %s\n", a.config.Workspace)
	fmt.Printf("Provider: %s\n", a.provider.Name())
	fmt.Println("Commands: /exit, /stop, /pause, /resume, /help, /tools, /session")
	fmt.Println("Type '/exit' to end session")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	
	// Create a cancellable context for the current task
	var cancel context.CancelFunc
	var currentTaskRunning bool
	
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Check for commands (start with /)
		if strings.HasPrefix(input, "/") {
			cmd := strings.ToLower(strings.TrimPrefix(input, "/"))
			switch cmd {
			case "exit", "quit":
				// Cancel any running task
				if cancel != nil {
					cancel()
				}
				fmt.Println("👋 Goodbye!")
				return nil
			case "stop":
				if currentTaskRunning && cancel != nil {
					fmt.Println("⏹️  Stopping current task...")
					cancel()
					currentTaskRunning = false
				} else {
					fmt.Println("ℹ️  No task is currently running")
				}
			case "pause":
				fmt.Println("⏸️  Pause command received (not yet implemented)")
				// TODO: Implement pausing
			case "resume":
				fmt.Println("▶️  Resume command received (not yet implemented)")
				// TODO: Implement resuming
			case "help":
				printHelp()
			case "tools":
				printTools(a.toolMgr)
			case "session":
				fmt.Printf("📋 Session ID: %s\n", a.session.ID())
				fmt.Printf("   Messages: %d\n", len(a.session.GetTranscript()))
				fmt.Printf("   Model: %s\n", a.config.Model)
				fmt.Printf("   Provider: %s\n", a.provider.Name())
			default:
				fmt.Printf("❓ Unknown command: %s\n", input)
				fmt.Println("   Available: /exit, /stop, /pause, /resume, /help, /tools, /session")
			}
			continue
		}

		// Run as task with cancellable context
		ctx, ctxCancel := context.WithCancel(context.Background())
		cancel = ctxCancel
		currentTaskRunning = true
		
		// Run task in goroutine so we can cancel it
		taskErr := make(chan error, 1)
		go func() {
			taskErr <- a.RunTaskWithContext(ctx, input)
		}()
		
		// Wait for task to complete or be cancelled
		select {
		case err := <-taskErr:
			currentTaskRunning = false
			if err != nil {
				if err == context.Canceled {
					fmt.Println("🛑 Task cancelled")
				} else {
					fmt.Printf("❌ Error: %v\n", err)
				}
			}
		case <-time.After(30 * time.Second):
			// Timeout for safety
			currentTaskRunning = false
			fmt.Println("⏰ Task timeout (30s)")
		}
	}

	return scanner.Err()
}