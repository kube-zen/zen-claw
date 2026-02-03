# Zen Claw

A Go-based AI agent system with multi-provider support, real-time progress streaming, and powerful tool execution. Built for practical development workflows.

## Philosophy

- **Trunk-based**: Everything on `main`, atomic commits
- **Minimal**: Zero project management overhead  
- **Practical**: Get things done in hours, not days
- **Go-native**: Leverage Go's simplicity and performance

## Features

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

### Real-Time Progress Streaming
See exactly what the AI is doing as it works (via SSE or WebSocket).

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
- CLI management: `zen-claw sessions list/info/clean`

## Quick Start

```bash
# Build
go build -o zen-claw .

# Start gateway (required)
./zen-claw gateway start &

# Interactive mode (recommended)
./zen-claw agent
```

## Modes of Operation

### 1. Agent Mode (Single AI)
Standard single-AI agent with tool execution - the primary interface.

```bash
# Interactive
./zen-claw agent

# Single task
./zen-claw agent "analyze this project"

# Named session
./zen-claw agent --session my-project "set up database"
```

### 2. Consensus Mode (Multi-AI → Arbiter)
Multiple AI workers tackle the SAME prompt with the SAME role, then an arbiter synthesizes the best ideas into a unified blueprint.

```bash
# Security architecture review
zen-claw consensus --role security_architect "Design zero-trust auth for microservices"

# API design
zen-claw consensus --role api_designer "REST API for user management with RBAC"

# Custom role
zen-claw consensus --role "kubernetes_operator_expert" "Design CRD for database management"

# View worker performance stats
zen-claw consensus --stats
```

**Available roles**: `security_architect`, `software_architect`, `api_designer`, `database_architect`, `devops_engineer`, `frontend_architect`, or any custom role.

### 3. Factory Mode (Coordinator + Specialists)
A coordinator AI manages specialist workers (Go, TypeScript, Infrastructure) to execute multi-phase projects with guardrails.

```bash
# Start factory with blueprint
zen-claw factory start --blueprint blueprint.json

# Check status
zen-claw factory status --project my-project

# Resume paused factory
zen-claw factory resume --project my-project
```

**Blueprint example** (`blueprint.json`):
```json
{
  "project": "zen-platform-v3",
  "phases": [
    {
      "name": "core_types",
      "task": "Define shared Go structs",
      "domain": "go",
      "outputs": ["zen-sdk/go/types.go"]
    },
    {
      "name": "api",
      "task": "Implement HTTP handlers",
      "domain": "go",
      "depends_on": ["core_types"]
    }
  ]
}
```

### 4. Fabric Mode (Interactive Multi-Worker)
Interactive session with multiple specialized workers and a coordinator.

```bash
zen-claw fabric

# Commands in fabric:
/coordinator deepseek             # Set coordinator
/worker add go_expert qwen go_developer
/worker add ts_expert minimax typescript_developer
/worker list
/profile save fullstack           # Save configuration
/profile load fullstack           # Load later
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

# Consensus mode configuration
consensus:
  workers:
    - provider: deepseek
      model: deepseek-chat
    - provider: qwen
      model: qwen3-coder-30b
    - provider: minimax
      model: minimax-M2.1
  arbiter: [kimi, qwen, deepseek]  # Preference order

# Factory mode specialists
factory:
  specialists:
    coordinator:
      provider: kimi
      model: kimi-k2-5
    go:
      provider: deepseek
      model: deepseek-chat
    typescript:
      provider: qwen
      model: qwen3-coder-30b
    infrastructure:
      provider: minimax
      model: minimax-M2.1
  guardrails:
    max_phase_duration_mins: 10
    max_total_duration_mins: 240
    max_cost_per_phase: 0.50
    max_cost_total: 5.00

preferences:
  fallback_order: [deepseek, kimi, glm, minimax, qwen, openai]
```

## Interactive Commands (Agent Mode)

| Command | Description |
|---------|-------------|
| `/help` | Show all commands |
| `/sessions` | List saved sessions |
| `/sessions info` | Show storage info (path, size) |
| `/sessions clean` | Clean sessions (`--all` or `--older 7d`) |
| `/load <name>` | Load a saved session |
| `/clear` | Fresh context |
| `/provider <name>` | Switch provider |
| `/model <name>` | Switch model |
| `/think [level]` | Set reasoning depth (off/low/medium/high) |
| `/stats` | Show usage and cache statistics |
| `/exit` | Exit |

## CLI Commands

```bash
# Agent (main interface)
zen-claw agent                    # Interactive mode
zen-claw agent "task"             # Single task

# Consensus (multi-AI synthesis)
zen-claw consensus --role <role> "prompt"
zen-claw consensus --stats

# Factory (multi-phase projects)
zen-claw factory start --blueprint file.json
zen-claw factory status --project name
zen-claw factory resume --project name

# Fabric (interactive multi-worker)
zen-claw fabric

# Session management
zen-claw sessions list
zen-claw sessions info
zen-claw sessions clean --all

# Gateway
zen-claw gateway start
zen-claw gateway stop
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

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| POST | `/chat` | Blocking chat |
| POST | `/chat/stream` | SSE streaming chat |
| GET | `/ws` | WebSocket |
| GET | `/sessions` | List sessions |
| GET | `/stats` | Usage, cache, circuit stats |

## Troubleshooting

```bash
# Check gateway health
curl http://localhost:8080/health

# Check stats
curl http://localhost:8080/stats

# View sessions
zen-claw sessions list

# Restart gateway
pkill -f "zen-claw gateway"
./zen-claw gateway start &
```

---

## Roadmap

### Completed ✅

- [x] Multi-provider AI support (6 providers)
- [x] Real-time SSE/WebSocket streaming
- [x] SQLite session persistence
- [x] Session CLI management
- [x] Tool system (20+ tools)
- [x] Interactive CLI with readline
- [x] Provider fallback routing
- [x] Circuit breaker
- [x] Response caching
- [x] Retry/backoff
- [x] Token usage tracking
- [x] Git tools
- [x] Diff preview
- [x] Parallel tool execution
- [x] MCP protocol support
- [x] Web tools
- [x] Thinking levels
- [x] **Consensus mode** (multi-AI → arbiter synthesis)
- [x] **Factory mode** (coordinator + specialists)
- [x] **Fabric mode** (interactive multi-worker)
- [x] Guardrails (cost, time, file limits)

### Next

- [ ] Web UI
- [ ] Plugin system
- [ ] RAG support

---

## Documentation

- [GETTING_STARTED.md](GETTING_STARTED.md) - Quick start guide
- [API.md](API.md) - Gateway API documentation
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Common issues

## License

MIT (private use only)
