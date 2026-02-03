# Zen Claw

A Go-based AI agent system with multi-provider support, real-time progress streaming, and powerful tool execution. Built for practical development workflows.

## Philosophy

- **Trunk-based**: Everything on `main`, atomic commits
- **Minimal**: Zero project management overhead  
- **Practical**: Get things done in hours, not days
- **Go-native**: Leverage Go's simplicity and performance

## Features

### Real-Time Progress Streaming
See exactly what the AI is doing as it works (via SSE or WebSocket):
```
🚀 Starting with deepseek/deepseek-chat

📍 Step 1/100: Thinking...
   💭 Waiting for AI response...
   🤖 I'll analyze the codebase structure first.
   🔧 list_dir(path=".")
   ✓ list_dir → 34 items

📍 Step 2/100: Thinking...
   💭 Waiting for AI response...
   🤖 Now let me read the main configuration...
   🔧 read_file(path="go.mod")
   ✓ read_file → 79 lines

✅ Task completed
```

### WebSocket Support
Bidirectional communication for real-time interaction:
```bash
# Use WebSocket instead of SSE
./zen-claw agent --ws "analyze this codebase"
```

### Multi-Provider AI Support
Six AI providers with automatic fallback and circuit breaker:

| Provider | Default Model | Context | Best For |
|----------|--------------|---------|----------|
| **DeepSeek** | deepseek-chat | 32K | Fast, general tasks |
| **Kimi** | kimi-k2-5 | 256K | Go/K8s, $0.10/M input |
| **Qwen** | qwen3-coder-30b | 262K | Large codebases |
| **GLM** | glm-4.7 | 128K | Chinese/English |
| **Minimax** | minimax-M2.1 | 128K | Good balance |
| **OpenAI** | gpt-4o-mini | 128K | Fallback |

### Powerful Tool System (20+ tools)
- **File ops**: read_file, write_file, edit_file, append_file, list_dir, search_files
- **Git**: git_status, git_diff, git_add, git_commit, git_push, git_log
- **Preview**: preview_write, preview_edit (show changes before modifying)
- **Web**: web_search (Brave API), web_fetch (HTML→markdown)
- **System**: exec, system_info, process (background management)
- **Advanced**: apply_patch (multi-file patches)
- **MCP**: External tool servers via Model Context Protocol

### Session Management
- **SQLite persistence** at `~/.zen/zen-claw/data/sessions.db`
- ACID-compliant, crash-safe (WAL mode)
- Max 5 concurrent sessions (configurable)
- CLI management: `zen-claw sessions list/info/clean`

## Quick Start

```bash
# Build
go build -o zen-claw .

# Start gateway (required)
./zen-claw gateway start &

# Interactive mode (recommended)
./zen-claw agent

# Or run single task
./zen-claw agent "analyze this project"
```

## Configuration

Config file: `~/.zen/zen-claw/config.yaml`

```yaml
providers:
  deepseek:
    api_key: sk-your-key
    model: deepseek-chat
    base_url: "https://api.deepseek.com"
  
  kimi:
    api_key: sk-your-key
    model: kimi-k2-5
    base_url: "https://api.moonshot.cn/v1"
  
  qwen:
    api_key: sk-your-key
    model: qwen3-coder-30b-a3b-instruct
    base_url: "https://dashscope-us.aliyuncs.com/compatible-mode/v1"

default:
  provider: deepseek
  model: deepseek-chat

sessions:
  max_sessions: 5
  # db_path: ~/.zen/zen-claw/data/sessions.db  # Custom path (optional)

preferences:
  fallback_order: [deepseek, kimi, glm, minimax, qwen, openai]

# MCP Servers (optional)
# mcp:
#   servers:
#     - name: myserver
#       command: /path/to/mcp-server
#       args: ["--flag"]
```

Or use environment variables:
```bash
export DEEPSEEK_API_KEY="sk-..."
export KIMI_API_KEY="sk-..."
export QWEN_API_KEY="sk-..."
```

## Interactive Commands

| Command | Description |
|---------|-------------|
| `/help` | Show all commands |
| `/sessions` | List saved sessions |
| `/sessions info` | Show storage info (path, size) |
| `/sessions clean` | Clean sessions (`--all` or `--older 7d`) |
| `/sessions delete <n>` | Delete specific session |
| `/load <name>` | Load a saved session |
| `/clear` | Fresh context (like Cursor Cmd+N) |
| `/providers` | List all AI providers |
| `/provider <name>` | Switch provider |
| `/models` | Show models for current provider |
| `/model <name>` | Switch model |
| `/think [level]` | Set reasoning depth (off/low/medium/high) |
| `/stats` | Show usage and cache statistics |
| `/prefs` | View/edit AI routing preferences |
| `/exit` | Exit |

## CLI Commands

```bash
# Agent (main interface)
zen-claw agent                    # Interactive mode
zen-claw agent "task"             # Single task
zen-claw agent --session my-proj  # Named session (persisted)
zen-claw agent --provider kimi    # Specific provider
zen-claw agent --max-steps 200    # Complex tasks

# Session management
zen-claw sessions list            # List all sessions
zen-claw sessions info            # Storage info
zen-claw sessions clean --all     # Delete all sessions
zen-claw sessions clean --older 7d # Delete old sessions

# Gateway
zen-claw gateway start            # Start server
zen-claw gateway stop             # Stop server

# MCP (Model Context Protocol)
zen-claw mcp list                 # Show MCP server examples
zen-claw mcp connect <n> -- cmd   # Connect to MCP server
```

## Architecture

```
┌─────────────┐     SSE Stream      ┌─────────────┐
│   CLI       │◄──────────────────►│   Gateway   │
│   Client    │    Progress Events  │   Server    │
└─────────────┘                     └──────┬──────┘
                                           │
                    ┌──────────────────────┼──────────────────────┐
                    │                      │                      │
              ┌─────▼─────┐          ┌─────▼─────┐          ┌─────▼─────┐
              │  Agent    │          │  Session  │          │    AI     │
              │  Engine   │          │   Store   │          │  Router   │
              └─────┬─────┘          │  (SQLite) │          └─────┬─────┘
                    │                └───────────┘                │
              ┌─────▼─────┐                                 ┌─────▼─────┐
              │   Tools   │                                 │ Providers │
              │ +MCP      │                                 │ +Circuit  │
              └───────────┘                                 └───────────┘
```

**Gateway Server** (`:8080`)
- REST API + SSE streaming + WebSocket
- SQLite session persistence (`~/.zen/zen-claw/data/sessions.db`)
- AI provider routing with fallback and circuit breaker
- MCP server integration

**CLI Client**
- Real-time progress streaming
- Interactive mode with readline
- Session management commands

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| POST | `/chat` | Send chat request (blocking) |
| POST | `/chat/stream` | Chat with SSE progress streaming |
| GET | `/ws` | WebSocket (bidirectional) |
| GET | `/sessions` | List all sessions |
| GET | `/sessions/{id}` | Get session details |
| DELETE | `/sessions/{id}` | Delete session |
| GET | `/stats` | Usage, cache, circuit breaker stats |
| GET | `/preferences` | Get AI routing preferences |

See [API.md](API.md) for detailed documentation.

## Timeout Configuration

| Component | Timeout | Purpose |
|-----------|---------|---------|
| HTTP Client | 45 min | Total request timeout |
| Agent Context | 30 min | Agent execution limit |
| Per-Step AI Call | 5 min | Individual AI call |
| Max Steps | 100 | Tool execution limit |

## Troubleshooting

```bash
# Check gateway health
curl http://localhost:8080/health

# Check stats (cache, circuits, MCP)
curl http://localhost:8080/stats

# View sessions
zen-claw sessions list

# Restart gateway
pkill -f "zen-claw gateway"
./zen-claw gateway start &
```

See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for detailed solutions.

---

## Roadmap

### Completed ✅

- [x] Multi-provider AI support (6 providers)
- [x] Real-time SSE progress streaming
- [x] WebSocket support with cancel
- [x] Session persistence (SQLite, ACID-compliant)
- [x] Session CLI management (`zen-claw sessions list/info/clean`)
- [x] Tool system (21 tools: file, git, web, process, patch, subagent)
- [x] Interactive CLI with readline
- [x] Provider fallback routing
- [x] Circuit breaker (auto-disable unhealthy providers)
- [x] Response caching (30-50% cost savings)
- [x] Retry/backoff for failed AI calls
- [x] Token usage tracking (`/stats`)
- [x] Git tools (status, diff, add, commit, push, log)
- [x] Diff preview (preview_write/preview_edit)
- [x] Parallel tool execution (2-5x speedup for read-only)
- [x] Streaming responses (provider-level)
- [x] MCP protocol support (external tool servers)
- [x] Web tools (search, fetch)
- [x] Thinking levels (`/think off/low/medium/high`)
- [x] Consensus mode (3 AIs → arbiter)
- [x] Factory mode (Coordinator + specialists)
- [x] Guardrails (cost, time, file limits)
- [x] Subagents (parallel background runs)
- [x] Smart context routing (size-based tier selection)
- [x] Cost optimizations (prompt compression, dedup, output limits)

### Next

- [ ] **Web UI** - Browser-based interface
- [ ] **Plugin system** - Custom tool packages
- [ ] **RAG support** - Vector search for large codebases

### Ideas

1. Undo support - Rollback file modifications
2. Cost estimation - Predict cost before execution
3. Smart context - Auto-include relevant files

---

## Documentation

- [GETTING_STARTED.md](GETTING_STARTED.md) - Quick start guide
- [API.md](API.md) - Gateway API documentation
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Common issues and solutions

## License

MIT (private use only)
