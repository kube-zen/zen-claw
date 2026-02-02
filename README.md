# Zen Claw

A Go clone of OpenClaw focusing on AI interaction and minimal tooling. No branches, no CI overhead, just results.

## Philosophy

- **Trunk-based**: Everything on `main`, atomic commits
- **Minimal**: Zero project management overhead
- **Practical**: Get things done in hours, not days
- **Go-native**: Leverage Go's simplicity and performance

## Structure

```
zen-claw/
├── cmd/           # CLI commands
├── internal/      # Internal packages
│   ├── agent/     # AI agent core
│   ├── session/   # Session management
│   └── tools/     # Tool implementations
├── main.go        # Entry point
└── go.mod         # Go module
```

## Quick Start

```bash
# Build (when Go is installed)
go build -o zen-claw .

# Run agent with default provider
./zen-claw agent "analyze project"

# Run agent with Qwen specifically
./zen-claw agent --provider qwen "analyze codebase"

# Run agent with specific Qwen model
./zen-claw agent --model qwen/qwen3-coder-30b "code review"

# List tools
./zen-claw tools

# Manage sessions
./zen-claw session list

# Use verbose mode for debugging
./zen-claw agent --verbose "debug this issue"
```

## Core Features

### AI Agent
- Run AI sessions with tool access
- Automatic tool chaining
- Conversation continuation
- Multi-provider support (DeepSeek, OpenAI, Qwen, etc.)

### Model Switching
- **Default model**: DeepSeek (configured in settings)
- **Switch models during session**:
  - `/models` - List all available models
  - `/model qwen/qwen3-coder-30b` - Switch to Qwen Coder
  - `/model deepseek/deepseek-chat` - Switch to DeepSeek

### Tool System
- **Read**: Read file contents
- **Write**: Create or overwrite files  
- **Edit**: Make precise edits to files
- **Exec**: Run shell commands
- **File Search**: Find files by name or content
- **Git Operations**: Git status, diff, log, etc.
- **Environment**: Manage environment variables

### Session Management
- Persistent conversation state
- Session ID tracking
- Session tagging for organization
- Session export capability

## Qwen Integration

### 🎯 **Qwen3-Coder-30B Special Feature**
**📚 262K Context Window** - The only dedicated coder model with massive context under $1!
- **$0.216** for first 32K tokens
- **$0.538** even at 200K tokens
- **Perfect for**: Analyzing massive legacy codebases, providing 50-file context for architecture decisions

## Configuration

Zen Claw comes with sensible defaults. Configure via environment variables or config file:

```bash
# Environment variables
export QWEN_API_KEY="your-qwen-api-key-here"
export DEEPSEEK_API_KEY="your-deepseek-api-key-here"

# Or create config file: ~/.zen/zen-claw/config.yaml
providers:
  qwen:
    api_key: "${QWEN_API_KEY}"
    model: "qwen3-coder-30b"
    base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
  deepseek:
    api_key: "${DEEPSEEK_API_KEY}"
    model: "deepseek-chat"

default:
  provider: "qwen"
  model: "qwen3-coder-30b"
  thinking: false

workspace:
  path: "~/.zen/zen-claw/workspace"
```

## Interactive Model Switching

While working in a session, you can switch between different AI models:

### Commands:
- **`/models`** - Show all available models
- **`/model qwen/qwen3-coder-30b`** - Switch to Qwen Coder
- **`/model deepseek/deepseek-chat`** - Switch to DeepSeek
- **`/model openai/gpt-4o`** - Switch to OpenAI GPT-4

### Example Workflow:
```bash
# Start with default model (DeepSeek)
zen-claw agent "analyze codebase"

# In the session, type:
/models                    # See available models
/model qwen/qwen3-coder-30b # Switch to Qwen
/code-review               # Continue with Qwen
```

## Byobu Integration

Perfect for terminal-based workflow with Byobu:
- **F2**: Create new session window
- **F3**: Previous window (left)
- **F4**: Next window (right) 
- **F8**: Rename window

## Development Philosophy

1. **Atomic commits**: Each commit does one thing well
2. **Trunk-only**: No branches, push directly to main
3. **Working > Perfect**: Ship it, then improve
4. **Document as you go**: README and comments are mandatory
5. **Test in production**: Actually use what you build

## Private Testing Notes

This is designed for private/internal use. No OSS considerations, just pure functionality evaluation.

## License

MIT (private use only)