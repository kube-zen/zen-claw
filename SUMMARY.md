# Zen Claw - Project Summary

## What It Is

Zen Claw is a Go-based AI agent system with:
- **6 AI providers** (DeepSeek, Kimi, Qwen, GLM, Minimax, OpenAI)
- **Real-time progress streaming** via SSE
- **8 tools** for file/system operations
- **Session persistence** with multi-session support
- **Gateway architecture** for scalable deployments

## Architecture

```
┌────────────┐    SSE     ┌────────────┐    API    ┌────────────┐
│   CLI      │◄──────────►│  Gateway   │◄─────────►│ Providers  │
│  Client    │  Stream    │  :8080     │           │ 6 backends │
└────────────┘            └────────────┘           └────────────┘
                                │
                          ┌─────┴─────┐
                          │  Session  │
                          │   Store   │
                          └───────────┘
```

## Key Features

### Real-Time Progress
```
🚀 Starting with deepseek/deepseek-chat
📍 Step 1/100: Thinking...
   💭 Waiting for AI response...
   🔧 list_dir(path=".")
   ✓ list_dir → 34 items
✅ Task completed
```

### Multi-Provider Support
| Provider | Model | Context | Best For |
|----------|-------|---------|----------|
| DeepSeek | deepseek-chat | 32K | Fast tasks |
| Kimi | kimi-k2-5 | 256K | Go/K8s |
| Qwen | qwen3-coder-30b | 262K | Large codebases |
| GLM | glm-4.7 | 128K | Chinese |
| Minimax | minimax-M2.1 | 128K | Balanced |
| OpenAI | gpt-4o-mini | 128K | Fallback |

### Tool System
- `exec` - Shell commands
- `read_file` / `write_file` / `edit_file` / `append_file` - File ops
- `list_dir` - Directory listing
- `search_files` - Regex search
- `system_info` - System info

### Session Management
- Max 5 concurrent sessions (configurable)
- Persistent to `/tmp/zen-claw-sessions/`
- Background/activate states
- API management

## Technical Details

### Timeouts
- HTTP Client: 45 min
- Agent Context: 30 min
- Per-Step: 5 min
- Max Steps: 100 (configurable)

### API Endpoints
- `POST /chat` - Blocking request
- `POST /chat/stream` - SSE streaming
- `GET /sessions` - List sessions
- `GET/DELETE /sessions/{id}` - Session ops

### Configuration
- File: `~/.zen/zen-claw/config.yaml`
- Env: `{PROVIDER}_API_KEY`
- Provider fallback ordering

## Codebase Structure

```
zen-claw/
├── cmd/                    # CLI commands
│   ├── agent.go           # Agent command + streaming client
│   ├── gateway.go         # Gateway server
│   └── ...
├── internal/
│   ├── agent/             # Agent engine + tools
│   ├── gateway/           # HTTP server + SSE
│   ├── providers/         # OpenAI-compatible providers
│   ├── config/            # YAML configuration
│   └── ...
├── main.go
└── go.mod
```

## Philosophy

- **Trunk-based**: Everything on `main`
- **Minimal**: No CI overhead
- **Practical**: Get things done
- **Go-native**: Single binary

## What's Next

See [README.md](README.md) roadmap for:
- WebSocket support
- Token tracking
- Consensus mode
- Factory mode
- Web UI
