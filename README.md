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

## Core Features (Planned)

1. **AI Agent**: Run AI sessions with tool access
2. **Tool System**: Read, write, exec, edit, etc.
3. **Session Management**: Track and resume conversations
4. **Gateway**: WebSocket server for remote access
5. **Memory**: Persistent context across sessions

## Quick Start

```bash
# Build (when Go is installed)
go build -o zen-claw .

# Run agent
./zen-claw agent --model deepseek/deepseek-chat

# List tools
./zen-claw tools

# Manage sessions
./zen-claw session list

# Use gateway with arbitration
./zen-claw gateway start
./zen-claw agent --gateway
```

## Supported AI Providers

Zen Claw supports multiple AI providers with automatic failover:

1. **DeepSeek** (default, cheapest) - `deepseek/deepseek-chat` ($0.14/1M tokens)
2. **OpenAI** - `openai/gpt-4o`, `openai/gpt-4-turbo` ($5/1M tokens)
3. **GLM** (智谱清言) - `glm/glm-4`, `glm/glm-3-turbo`
4. **Minimax** - `minimax/abab6.5s`, `minimax/abab6.5`
5. **Qwen** (Alibaba) - `qwen/qwen-max`, `qwen/qwen-plus`, `qwen/qwen3-coder-30b`
6. **Mock** (for testing) - Always works

### 🎯 **Qwen3-Coder-30B Special Feature**
**📚 262K Context Window** - The only dedicated coder model with massive context under $1!
- **$0.216** for first 32K tokens
- **$0.538** even at 200K tokens
- **Perfect for**: Analyzing massive legacy codebases, providing 50-file context for architecture decisions

### Provider Arbitration
- **Cost-optimized**: Tries cheapest providers first
- **Automatic failover**: If one provider fails, tries next
- **User override**: Say `provider: openai` to force specific provider
- **Consensus voting**: Coming soon (ask multiple providers, take majority vote)

## Development Philosophy

1. **Atomic commits**: Each commit does one thing well
2. **Trunk-only**: No branches, push directly to main
3. **Working > Perfect**: Ship it, then improve
4. **Document as you go**: README and comments are mandatory
5. **Test in production**: Actually use what you build

## Status

🚧 **Work in Progress** - Core structure implemented, AI integration pending.

## License

MIT