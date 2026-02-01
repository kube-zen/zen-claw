# zen-agent - Go Agent Library

Inspired by pi-coding-agent architecture, providing:
- Tool execution loop with conversation continuation
- Session management
- Multiple AI provider support
- Extensible tool system

## Architecture

```
Agent (core loop)
├── Session (state management)
├── Tools (exec, read, write, edit, list_dir, etc.)
├── Providers (DeepSeek, OpenAI, GLM, Minimax, Qwen)
└── SessionManager (persistence)
```

## Usage

```go
// Create agent
agent := NewAgent(Config{
    Model: "deepseek/deepseek-chat",
    Tools: []Tool{execTool, readTool, writeTool},
    WorkingDir: "/path/to/project",
})

// Run task (handles multi-step tool execution)
result, err := agent.Run("check codebase and suggest improvements")
// Agent will: list_dir → read go.mod → read main.go → analyze
```

## Features

1. **Automatic tool chaining**: AI makes multiple tool calls, agent executes all
2. **Conversation continuation**: Tool results fed back to AI for follow-up
3. **Session persistence**: Save/load conversations
4. **Multi-provider**: DeepSeek, OpenAI, GLM, Minimax, Qwen
5. **Cost tracking**: Token usage and cost estimation
