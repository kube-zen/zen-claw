# Zen Claw - Complete Example

This shows how the Go clone of OpenClaw would work end-to-end.

## Architecture

```
User → CLI → Agent → AI Provider → Tools → Results
```

## 1. CLI Usage

```bash
# Run an AI agent with tools
zen-claw agent --model deepseek/deepseek-chat --task "Read README.md and summarize it"

# Interactive mode
zen-claw agent --thinking

# List available tools
zen-claw tools

# Manage sessions
zen-claw session list
zen-claw session spawn --task "Write a Go function"
```

## 2. Agent System

```go
// Create agent with workspace
config := agent.Config{
    Model:     "deepseek/deepseek-chat",
    Workspace: "/home/user/projects",
    Thinking:  true,
}

ag, err := agent.New(config)
if err != nil {
    log.Fatal(err)
}

// Run task
err = ag.RunTask("Write a hello world in Go")
```

## 3. Tool Integration

```go
// Tools are automatically available to the AI
tools := []string{
    "read",      // Read files
    "write",     // Write files  
    "edit",      // Edit files
    "exec",      // Run commands
    "process",   // Manage processes
    "web_search", // Search web
    "web_fetch",  // Fetch URLs
}

// AI can call tools like:
// "Read the file at src/main.go"
// "Run 'go test ./...'"
// "Search for 'Go concurrency patterns'"
```

## 4. AI Provider Interface

```go
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    SupportsTools() bool
}

// Multiple providers supported:
// - OpenAI (GPT-4, GPT-3.5)
// - Anthropic (Claude)
// - DeepSeek
// - Google (Gemini)
// - Local (Ollama, Llama)
```

## 5. Session Management

```go
// Sessions persist across runs
session := session.New(session.Config{
    Workspace: "/workspace",
    Model:     "deepseek/deepseek-chat",
})

// Save/load transcript
session.Save()
session.Load()

// Sub-agent sessions
parentSession.SpawnChild("Write documentation", "doc-agent")
```

## 6. Gateway (Future)

```go
// WebSocket server for remote access
gateway.Start(&gateway.Config{
    Port: 8080,
    Auth: gateway.TokenAuth("secret-token"),
})

// Clients can connect and use agent remotely
// Browser UI, mobile apps, other services
```

## Complete Flow Example

1. **User**: `zen-claw agent --task "Write a Go HTTP server"`
2. **Agent**: Creates session, loads tools
3. **AI**: "I'll write a Go HTTP server. Let me check if Go is installed."
4. **Tool Call**: `exec {command: "go version"}`
5. **Result**: "go version go1.23 linux/amd64"
6. **AI**: "Go is installed. Now writing server.go..."
7. **Tool Call**: `write {path: "server.go", content: "package main\n\nimport (...)"}`
8. **Result**: "File written: server.go (150 bytes)"
9. **AI**: "Server written. Let me test it..."
10. **Tool Call**: `exec {command: "go run server.go &"}`
11. **Final**: "HTTP server running on :8080. Use curl to test."

## Why Go?

- **Performance**: Native compilation, fast execution
- **Simplicity**: Clear syntax, minimal dependencies  
- **Concurrency**: Goroutines for parallel tool execution
- **Portability**: Single binary, cross-platform
- **Ecosystem**: Rich libraries for AI, web, CLI

## Next Steps

1. **AI Integration**: Connect to real providers (OpenAI, DeepSeek)
2. **Tool Completion**: Implement all tools from OpenClaw
3. **Gateway**: WebSocket server with auth
4. **Memory**: Persistent context across sessions
5. **Skills**: Plugin system for specialized tasks

## Philosophy in Action

- **Trunk-based**: This entire project was built in one session on `main`
- **Minimal**: No CI, no branches, just working code
- **Practical**: Structure ready for AI integration in hours
- **Atomic**: Each commit does one thing well