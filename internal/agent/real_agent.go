package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/session"
	"github.com/neves/zen-claw/internal/skills"
	"github.com/neves/zen-claw/internal/tools"
)

type RealAgent struct {
	provider  ai.Provider
	toolMgr   *tools.Manager
	session   *session.Session
	config    Config
	skillMgr  *skills.Manager
}

func NewRealAgent(config Config, provider ai.Provider, toolMgr *tools.Manager, sess *session.Session, skillMgr *skills.Manager) *RealAgent {
	return &RealAgent{
		provider: provider,
		toolMgr:  toolMgr,
		session:  sess,
		config:   config,
		skillMgr: skillMgr,
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

CRITICAL TOOL USAGE RULES:
1. When user asks to "build", "compile", "run", "execute", or "test" something:
   - FIRST use "list_dir" to check current directory
   - THEN decide appropriate command
   - FINALLY use "exec" to run the command
   
2. For Go projects: If you see go.mod, use "go build", "go test", or "go run"
3. For Make projects: If you see Makefile, use "make" or "make build"
4. For Node.js: If you see package.json, check scripts with "read_file"
5. For direct commands: If user says "run <command>", use "exec" immediately

DIRECT ACTION REQUIRED:
- "build <project>" → Check directory, then build
- "run <command>" → Execute command directly
- "test" → Run tests
- "compile" → Compile project

Be proactive. Use tools aggressively when asked to perform actions.`,
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
	// Try to parse as JSON first (for tool results)
	var jsonObj map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonObj); err == nil {
		// It's JSON from a tool call
		a.printToolResult(jsonObj)
		return
	}
	
	// Not JSON, print as-is (regular AI response)
	fmt.Printf("🤖 %s\n\n", response)
}

func (a *RealAgent) printToolResult(result map[string]interface{}) {
	// Handle different tool result formats
	
	// exec tool result
	if command, ok := result["command"].(string); ok {
		output, _ := result["output"].(string)
		errorMsg, hasError := result["error"].(string)
		exitCode, hasExitCode := result["exit_code"].(float64)
		
		fmt.Println("🔧 Command executed:")
		fmt.Printf("   $ %s\n", command)
		
		if output != "" {
			fmt.Println("\n📤 Output:")
			fmt.Print(output)
			if !strings.HasSuffix(output, "\n") {
				fmt.Println()
			}
		}
		
		if hasError {
			fmt.Printf("❌ Error: %s\n", errorMsg)
		}
		
		if hasExitCode {
			fmt.Printf("📊 Exit code: %.0f\n", exitCode)
		}
		
		fmt.Println()
		return
	}
	
	// list_dir tool result
	if path, ok := result["path"].(string); ok {
		if files, ok := result["files"].([]interface{}); ok {
			fmt.Printf("📁 Directory: %s\n\n", path)
			for i, file := range files {
				if fileMap, ok := file.(map[string]interface{}); ok {
					name, _ := fileMap["name"].(string)
					isDir, _ := fileMap["is_dir"].(bool)
					size, _ := fileMap["size"].(float64)
					mode, _ := fileMap["mode"].(string)
					
					// Format like ls -la
					dirChar := "-"
					if isDir {
						dirChar = "d"
					}
					sizeStr := fmt.Sprintf("%.0f", size)
					if isDir {
						sizeStr = "-"
					}
					
					// Safely get mode string
					modeStr := ""
					if len(mode) > 1 {
						modeStr = mode[1:]
					} else {
						modeStr = mode
					}
					
					fmt.Printf("%s%s  %8s  %s\n", 
						dirChar, modeStr,
						sizeStr,
						name)
					
					// Show only first 20 files
					if i >= 19 && len(files) > 20 {
						fmt.Printf("... and %d more files\n", len(files)-20)
						break
					}
				}
			}
			fmt.Println()
			return
		}
	}
	
	// read_file tool result
	if content, ok := result["content"].(string); ok {
		path, _ := result["path"].(string)
		size, _ := result["size"].(float64)
		
		fmt.Printf("📄 File: %s (%.0f bytes)\n", path, size)
		fmt.Println(strings.Repeat("─", 50))
		fmt.Println(content)
		if !strings.HasSuffix(content, "\n") {
			fmt.Println()
		}
		fmt.Println(strings.Repeat("─", 50))
		fmt.Println()
		return
	}
	
	// system_info tool result
	if hostname, ok := result["hostname"].(string); ok {
		currentDir, _ := result["current_dir"].(string)
		gatewayPid, _ := result["gateway_pid"].(float64)
		
		fmt.Println("🖥️  System Information:")
		fmt.Printf("   Hostname: %s\n", hostname)
		fmt.Printf("   Current directory: %s\n", currentDir)
		fmt.Printf("   Gateway PID: %.0f\n", gatewayPid)
		fmt.Printf("   Time: %v\n", result["time"])
		
		// Show first few env vars
		if envVars, ok := result["env_vars"].([]interface{}); ok && len(envVars) > 0 {
			fmt.Println("\n   Environment variables:")
			for i := 0; i < min(5, len(envVars)); i++ {
				if env, ok := envVars[i].(string); ok {
					fmt.Printf("   • %s\n", env)
				}
			}
			if len(envVars) > 5 {
				fmt.Printf("   ... and %d more\n", len(envVars)-5)
			}
		}
		
		fmt.Println()
		return
	}
	
	// Unknown JSON format, pretty print it
	fmt.Println("🤖 Tool result:")
	prettyJSON, err := json.MarshalIndent(result, "  ", "  ")
	if err != nil {
		fmt.Printf("🤖 %v\n\n", result)
		return
	}
	fmt.Println(string(prettyJSON))
	fmt.Println()
}

func (a *RealAgent) listModels() {
	fmt.Println("🤖 Available Models")
	fmt.Println("─" + strings.Repeat("─", 40))
	
	// DeepSeek models
	fmt.Println("DeepSeek:")
	fmt.Println("  • deepseek-chat (default)")
	fmt.Println("  • deepseek-reasoner")
	fmt.Println("  • deepseek-coder")
	
	// OpenAI models (if configured)
	fmt.Println("\nOpenAI:")
	fmt.Println("  • gpt-4o")
	fmt.Println("  • gpt-4-turbo")
	fmt.Println("  • gpt-3.5-turbo")
	
	// GLM models
	fmt.Println("\nGLM:")
	fmt.Println("  • glm-4")
	fmt.Println("  • glm-3-turbo")
	
	// Minimax models
	fmt.Println("\nMinimax:")
	fmt.Println("  • abab6.5s")
	fmt.Println("  • abab6.5")
	
	// Qwen models
	fmt.Println("\nQwen:")
	fmt.Println("  • qwen-max")
	fmt.Println("  • qwen-plus")
	fmt.Println("  • qwen-turbo")
	fmt.Println("  • qwen2.5-72b")
	fmt.Println("  • qwen2.5-32b")
	fmt.Println("  • qwen2.5-14b")
	fmt.Println("  • qwen2.5-7b")
	fmt.Println("  • qwen3-coder-30b 📚 262K context, $0.216/32K, $0.538/200K")
	
	fmt.Println("\n💡 Usage: /models <model-name>")
	fmt.Println("   Example: /models deepseek-reasoner")
	fmt.Println("   Note: Model switching requires restart for full effect")
	fmt.Println()
}

func (a *RealAgent) buildProject(project string) {
	if project == "" {
		project = "current project"
	}
	
	fmt.Printf("🔨 Building: %s\n", project)
	fmt.Println("💡 Tip: For immediate build, use explicit command:")
	fmt.Println("   'Run go build command' or 'Use exec tool: go build'")
	fmt.Println()
	
	// Run a task that should trigger build
	task := fmt.Sprintf("Build %s using appropriate build command. Check if it's a Go project and run go build if it is.", project)
	
	if err := a.RunTask(task); err != nil {
		fmt.Printf("❌ Build failed: %v\n", err)
	}
}

func (a *RealAgent) switchModel(modelName string) {
	fmt.Printf("🔄 Switching to model: %s\n", modelName)
	
	// Update config
	a.config.Model = modelName
	
	// Update session config
	a.session.Save() // Save current session with old model
	
	// Note: For full effect, we'd need to recreate the provider
	// For now, just update the config
	fmt.Println("✅ Model updated in config")
	fmt.Println("⚠️  Note: Full model switch requires agent restart")
	fmt.Printf("   Current model: %s\n", a.config.Model)
	fmt.Println()
}

func (a *RealAgent) spawnSubAgent(task string) {
	fmt.Printf("👶 Spawning sub-agent for task: %s\n", task)
	
	// In a real implementation, we would:
	// 1. Create a new agent process
	// 2. Pass the task as argument
	// 3. Capture output
	// 4. Return results
	
	// For now, just show what we would do
	fmt.Println("🚧 Sub-agent spawning not fully implemented yet")
	fmt.Println("   This would run: zen-claw agent --task \"" + task + "\"")
	fmt.Println("   In background with isolated session")
	fmt.Println()
	
	// Actually, let me implement a simple version
	// that runs the task in the current agent but marks it as sub-task
	fmt.Println("📝 Running task in current agent (simulated sub-agent):")
	fmt.Println()
	
	// Run the task directly
	if err := a.RunTask(task); err != nil {
		fmt.Printf("❌ Sub-agent failed: %v\n", err)
	}
}

func (a *RealAgent) listSkills() {
	if a.skillMgr == nil {
		fmt.Println("⚠️  Skills system not initialized")
		return
	}
	
	skills := a.skillMgr.List()
	fmt.Printf("🎯 Skills: %d available\n", len(skills))
	fmt.Println("─" + strings.Repeat("─", 60))
	
	for i, skill := range skills {
		fmt.Printf("%2d. %s\n", i+1, skill.Name)
		fmt.Printf("    %s\n", skill.Description)
		if i < len(skills)-1 {
			fmt.Println()
		}
	}
	
	if len(skills) == 0 {
		fmt.Println("No skills found. Create skills in:", a.config.SkillsDir)
		fmt.Println("  Example: Create ~/.zen/skills/code-review/SKILL.md")
	}
	fmt.Println()
}

func (a *RealAgent) listSessions() {
	sessions, err := session.ListSessions(a.config.Workspace)
	if err != nil {
		fmt.Printf("❌ Error listing sessions: %v\n", err)
		return
	}
	
	fmt.Printf("📚 Sessions: %d total\n", len(sessions))
	fmt.Println("─" + strings.Repeat("─", 60))
	
	for i, s := range sessions {
		if i >= 10 {
			fmt.Printf("... and %d more sessions\n", len(sessions)-10)
			break
		}
		
		id := s["id"].(string)
		createdAt, _ := time.Parse(time.RFC3339, s["created_at"].(string))
		messageCount, _ := s["message_count"].(float64)
		model, hasModel := s["model"].(string)
		lastUpdated, _ := time.Parse(time.RFC3339, s["last_updated"].(string))
		
		// Format time
		timeStr := createdAt.Format("Jan 02 15:04")
		if time.Since(createdAt) > 24*time.Hour {
			timeStr = createdAt.Format("Jan 02")
		}
		
		// Current session marker
		current := ""
		if id == a.session.ID() {
			current = " ← current"
		}
		
		fmt.Printf("%2d. %s%s\n", i+1, id[:8], current)
		fmt.Printf("    Messages: %.0f", messageCount)
		if hasModel && model != "" {
			fmt.Printf(" | Model: %s", model)
		}
		fmt.Printf(" | Created: %s", timeStr)
		
		// Show age if not today
		if time.Since(lastUpdated) > 24*time.Hour {
			days := int(time.Since(lastUpdated).Hours() / 24)
			fmt.Printf(" (%d days ago)", days)
		}
		fmt.Println()
	}
	fmt.Println()
}

func (a *RealAgent) printStatus() {
	fmt.Println("📊 Zen Claw Status")
	fmt.Println("─" + strings.Repeat("─", 40))
	
	// Session info
	fmt.Printf("Session: %s\n", a.session.ID())
	fmt.Printf("  Messages: %d\n", len(a.session.GetTranscript()))
	fmt.Printf("  Model: %s\n", a.config.Model)
	fmt.Printf("  Provider: %s\n", a.provider.Name())
	
	// Workspace info
	workspace := a.config.Workspace
	if _, err := os.Stat(workspace); err == nil {
		fmt.Printf("Workspace: %s\n", workspace)
		fmt.Printf("  Size: %v\n", formatBytes(getDirSize(workspace)))
	}
	
	// Tools info
	tools := a.toolMgr.List()
	fmt.Printf("Tools: %d available\n", len(tools))
	for i, tool := range tools {
		if i < 5 {
			fmt.Printf("  • %s\n", tool)
		}
	}
	if len(tools) > 5 {
		fmt.Printf("  ... and %d more\n", len(tools)-5)
	}
	
	// Memory info (if we implement it)
	sessionDir := filepath.Join(workspace, ".zen-claw", "sessions")
	if _, err := os.Stat(sessionDir); err == nil {
		if entries, err := os.ReadDir(sessionDir); err == nil {
			fmt.Printf("Sessions: %d stored\n", len(entries))
		}
	}
	
	fmt.Println("─" + strings.Repeat("─", 40))
	fmt.Println()
}

func getDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
			parts := strings.Fields(input)
			cmd := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
			args := parts[1:]
			
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
			case "sessions":
				a.listSessions()
			case "skills":
				a.listSkills()
			case "status":
				a.printStatus()
			case "spawn":
				if len(args) > 1 {
					task := strings.Join(args[1:], " ")
					a.spawnSubAgent(task)
				} else {
					fmt.Println("❓ Usage: /spawn <task>")
					fmt.Println("   Example: /spawn Review the README file")
				}
			case "models":
				if len(args) > 0 {
					modelName := strings.Join(args, " ")
					a.switchModel(modelName)
				} else {
					a.listModels()
				}
			case "build":
				if len(args) > 0 {
					project := strings.Join(args, " ")
					a.buildProject(project)
				} else {
					a.buildProject("")
				}
			default:
				fmt.Printf("❓ Unknown command: %s\n", input)
				fmt.Println("   Available: /exit, /stop, /pause, /resume, /help, /tools, /session, /sessions, /skills, /status, /spawn, /models, /build")
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