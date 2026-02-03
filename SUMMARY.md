# Zen Claw - Project Summary

## What It Is

Zen Claw is a Go-based AI agent system with:
- **6 AI providers** (DeepSeek, Kimi, Qwen, GLM, Minimax, OpenAI)
- **4 operation modes** (Agent, Consensus, Factory, Fabric)
- **24+ tools** for file, git, web, and system operations
- **Real-time streaming** via SSE and WebSocket
- **SQLite session persistence** with CLI management
- **Plugin system** for custom tools
- **RAG support** with codebase indexing
- **MCP protocol** for external tool integration

## Architecture

```
┌────────────┐   SSE/WS    ┌────────────┐    API    ┌────────────┐
│   CLI      │◄───────────►│  Gateway   │◄─────────►│ Providers  │
│  Client    │   Stream    │   :8080    │           │ 6 backends │
└────────────┘             └────────────┘           └────────────┘
                                 │
                    ┌────────────┼────────────┐
                    │            │            │
              ┌─────┴─────┐ ┌────┴────┐ ┌────┴────┐
              │  SQLite   │ │ Plugins │ │  RAG    │
              │ Sessions  │ │         │ │ Index   │
              └───────────┘ └─────────┘ └─────────┘
```

## Operation Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| **Agent** | Single AI with tools | Daily coding tasks |
| **Consensus** | 3+ AIs → arbiter | Architecture decisions |
| **Factory** | Coordinator + specialists | Multi-phase projects |
| **Fabric** | Interactive multi-worker | Complex team tasks |

## Key Features

### Tools (24+)
| Category | Tools |
|----------|-------|
| **File** | read_file, write_file, edit_file, append_file, list_dir, search_files |
| **Git** | git_status, git_diff, git_add, git_commit, git_push, git_log |
| **Preview** | preview_write, preview_edit |
| **Web** | web_search, web_fetch |
| **System** | exec, system_info, process |
| **Advanced** | apply_patch |
| **RAG** | code_search, find_symbol, get_context |
| **MCP** | External tools via MCP servers |
| **Plugins** | Custom script-based tools |

### Multi-Provider Support
| Provider | Model | Context | Best For |
|----------|-------|---------|----------|
| DeepSeek | deepseek-chat | 32K | Fast tasks |
| Kimi | kimi-k2-5 | 256K | Go/K8s |
| Qwen | qwen3-coder-30b | 262K | Large codebases |
| GLM | glm-4.7 | 128K | Chinese |
| Minimax | minimax-M2.1 | 128K | Balanced |
| OpenAI | gpt-4o-mini | 128K | Fallback |

### Session Management
- SQLite persistence (`~/.zen/zen-claw/data/sessions.db`)
- Named sessions survive gateway restarts
- CLI commands: `/session list`, `/session load`, `/session clean`

### Plugin System
```bash
zen-claw plugins init my-tool --lang bash
zen-claw plugins list
```

### RAG (Codebase Indexing)
```bash
zen-claw index build .
zen-claw index search "authentication"
```

## Quick Start

```bash
# Build
go build -o zen-claw .

# Set API key
export DEEPSEEK_API_KEY=sk-...

# Start gateway
./zen-claw gateway start &

# Interactive mode
./zen-claw agent
```

## Configuration

File: `~/.zen/zen-claw/config.yaml`

```yaml
providers:
  deepseek:
    api_key: sk-...
    model: deepseek-chat

default:
  provider: deepseek

sessions:
  max_sessions: 5

plugins:
  dir: ~/.zen/zen-claw/plugins
```

## Codebase Structure

```
zen-claw/
├── cmd/                    # CLI commands
├── internal/
│   ├── agent/             # Agent engine + tools
│   ├── gateway/           # HTTP server + SSE/WS
│   ├── providers/         # AI provider clients
│   ├── config/            # Configuration
│   ├── plugins/           # Plugin system
│   ├── rag/               # Codebase indexing
│   ├── mcp/               # MCP protocol client
│   ├── consensus/         # Multi-AI consensus
│   └── factory/           # Factory mode
├── main.go
└── go.mod
```

## Philosophy

- **Trunk-based**: Everything on `main`
- **Go-native**: Single binary, no external dependencies
- **Practical**: Get things done
- **Extensible**: Plugins, MCP, multiple providers
