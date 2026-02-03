# zen-agent - Go Agent Library

Core agent library used by zen-claw's multiple operation modes.

## Features

- Tool execution loop with conversation continuation
- Session management (SQLite persistence)
- 6 AI providers with circuit breaker and fallback
- 20+ tools + MCP integration

## Architecture

```
Agent (core loop)
├── Session (SQLite persistence)
├── Tools (20+)
│   ├── File: exec, read_file, write_file, edit_file, append_file, list_dir, search_files, system_info
│   ├── Git: git_status, git_diff, git_add, git_commit, git_push, git_log
│   ├── Preview: preview_write, preview_edit
│   ├── Web: web_search, web_fetch
│   ├── Process: process (background exec management)
│   ├── Patch: apply_patch (multi-file)
│   └── MCP: External tools via Model Context Protocol
├── Providers (DeepSeek, OpenAI, GLM, Minimax, Qwen, Kimi)
└── Circuit Breaker (auto-disable unhealthy providers)
```

## Used By

- **Agent Mode**: Single AI with tools (main interface)
- **Consensus Mode**: Multi-AI → arbiter synthesis
- **Factory Mode**: Coordinator + specialists for multi-phase projects
- **Fabric Mode**: Interactive multi-worker sessions

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
