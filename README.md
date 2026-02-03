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

WebSocket enables:
- Cancel running tasks
- Multiple requests per connection
- Lower latency for interactive use

### Slack Integration
Interact with the AI agent directly from Slack:
```bash
# Start the Slack bot
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_APP_TOKEN="xapp-..."
./zen-claw slack
```

Slack features:
- Thread-based sessions (each thread = separate context)
- Real-time progress updates
- Session management commands (`/status`, `/sessions`, `/attach`, `/clear`)
- Provider/model switching (`/provider`, `/model`)

### Multi-Provider AI Support
Six AI providers with automatic fallback:

| Provider | Default Model | Context | Best For |
|----------|--------------|---------|----------|
| **DeepSeek** | deepseek-chat | 32K | Fast, general tasks |
| **Kimi** | kimi-k2-5 | 256K | Go/K8s, $0.10/M input |
| **Qwen** | qwen3-coder-30b | 262K | Large codebases |
| **GLM** | glm-4.7 | 128K | Chinese/English |
| **Minimax** | minimax-M2.1 | 128K | Good balance |
| **OpenAI** | gpt-4o-mini | 128K | Fallback |

### Powerful Tool System
- **exec**: Run shell commands with working directory tracking
- **read_file**: Read file contents
- **write_file**: Create/overwrite files  
- **edit_file**: String replacement (like Cursor's StrReplace)
- **append_file**: Append to files
- **list_dir**: List directory contents
- **search_files**: Regex search across files
- **system_info**: Get system information

### Session Management
- Persistent conversation state
- Max 5 concurrent sessions (configurable)
- Background/activate session states
- Session export capability

## Quick Start

```bash
# Build
go build -o zen-claw .

# Start gateway (required)
./zen-claw gateway start &

# Run agent with streaming progress
./zen-claw agent "analyze this project"

# Interactive mode
./zen-claw agent

# Use specific provider
./zen-claw agent --provider kimi "review this Go code"

# Increase max steps for complex tasks
./zen-claw agent --max-steps 200 "refactor the entire module"
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

preferences:
  fallback_order: [deepseek, kimi, glm, minimax, qwen, openai]
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
| `/providers` | List all available AI providers |
| `/provider <name>` | Switch provider (resets to default model) |
| `/models` | Show models for current provider |
| `/model <name>` | Switch model within provider |
| `/context-limit [n]` | Set context limit (0=unlimited, default 50) |
| `/qwen-large-context [on\|off]` | Toggle Qwen 256K context |
| `/exit`, `/quit` | Exit interactive mode |
| `/help` | Show all commands |

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
              └─────┬─────┘          └───────────┘          └─────┬─────┘
                    │                                             │
              ┌─────▼─────┐                                 ┌─────▼─────┐
              │   Tools   │                                 │ Providers │
              │ exec,read │                                 │ DS,Kimi.. │
              └───────────┘                                 └───────────┘
```

**Gateway Server** (`:8080`)
- REST API + SSE streaming
- Session persistence in `/tmp/zen-claw-sessions/`
- AI provider routing with fallback
- Tool execution coordination

**CLI Client**
- Real-time progress streaming
- Interactive mode with readline
- History persistence

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
| POST | `/sessions/{id}/background` | Move to background |
| POST | `/sessions/{id}/activate` | Activate session |
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

# Check active sessions
curl http://localhost:8080/sessions

# View gateway logs
tail -f /tmp/gateway.log

# Restart gateway
pkill -f "zen-claw gateway"
./zen-claw gateway start &
```

See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for detailed solutions.

## Shell Completion

```bash
# Bash
source <(zen-claw completion bash)

# Zsh  
source <(zen-claw completion zsh)

# Fish
zen-claw completion fish | source
```

---

## Roadmap

### Completed ✅

- [x] Multi-provider AI support (6 providers)
- [x] Real-time SSE progress streaming
- [x] WebSocket support - Bidirectional communication with cancel support
- [x] **Slack integration** - Thread-based sessions, progress streaming, commands
- [x] Session persistence and management
- [x] Tool system (21 tools: file, git, web, process, patch, subagent)
- [x] Interactive CLI with readline
- [x] Provider fallback routing
- [x] Context limit control
- [x] Kimi K2.5 integration
- [x] **Consensus mode** - 3 AIs → arbiter → better blueprints
- [x] **Factory mode** - Coordinator + specialist AIs for complex projects
- [x] **Guardrails** - Safety limits (cost, time, files, phases)
- [x] **Simplified sessions** - Fresh by default (like Cursor), named sessions opt-in
- [x] **Response caching** - In-memory cache with TTL (30-50% cost savings)
- [x] **Retry/backoff** - Auto-retry failed AI calls with exponential backoff
- [x] **Token usage tracking** - `/stats` command shows cost per session
- [x] **Git tools** - Built-in git_status, git_diff, git_add, git_commit, git_push, git_log
- [x] **Diff preview** - preview_write/preview_edit show changes before modifying files
- [x] **Parallel tool execution** - Read-only tools run concurrently (2-5x speedup)
- [x] **Circuit breaker** - Provider health tracking, auto-disable unhealthy providers
- [x] **Streaming responses** - Provider-level streaming with `--stream` flag
- [x] **MCP protocol support** - MCP servers auto-connect from config, tools available in agent
- [x] **Web tools** - web_search (Brave API) and web_fetch (HTML→markdown)
- [x] **Process management** - Background exec with poll/log/kill
- [x] **Apply patch** - Multi-file structured patches
- [x] **Thinking levels** - `/think off/low/medium/high` for model reasoning depth
- [x] **Subagents** - Spawn parallel background agent runs

### Short Term (Next)

- [ ] **MCP in agent** - Wire MCP tools into agent sessions

### Medium Term

### Long Term

- [ ] **Web UI** - Browser-based interface
- [ ] **Plugin system** - Custom tool packages
- [ ] **RAG support** - Vector search for large codebases

### Ideas

1. **Undo support** - Rollback file modifications
2. **Provider health monitoring** - Track latency and errors  
3. **Cost estimation** - Predict cost before execution
4. **Smart context** - Auto-include relevant files

---

## Documentation

- [API.md](API.md) - Gateway API documentation
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Common issues and solutions
- [EXAMPLE.md](EXAMPLE.md) - Usage examples

## License

MIT (private use only)
