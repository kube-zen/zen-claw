# Zen Claw - Getting Started

## Quick Start (5 minutes)

```bash
# 1. Build
cd ~/zen/zen-claw
go build -o zen-claw .

# 2. Set API key (get free from https://platform.deepseek.com)
export DEEPSEEK_API_KEY=sk-your-key-here

# 3. Start gateway
./zen-claw gateway start &

# 4. Interactive mode (recommended)
./zen-claw agent
```

## Configuration

### Environment Variables (Quick)
```bash
export DEEPSEEK_API_KEY="sk-..."     # Recommended: Free tier
export KIMI_API_KEY="sk-..."          # $0.10/M, Go/K8s expert
export QWEN_API_KEY="sk-..."          # 262K context
```

### Config File (Persistent)
Create `~/.zen/zen-claw/config.yaml`:

```yaml
providers:
  deepseek:
    api_key: sk-your-deepseek-key
    model: deepseek-chat
    base_url: "https://api.deepseek.com"
  
  kimi:
    api_key: sk-your-kimi-key
    model: kimi-k2-5
    base_url: "https://api.moonshot.cn/v1"
  
  qwen:
    api_key: sk-your-qwen-key
    model: qwen3-coder-30b-a3b-instruct
    base_url: "https://dashscope-us.aliyuncs.com/compatible-mode/v1"

default:
  provider: deepseek
  model: deepseek-chat

sessions:
  max_sessions: 5

# For consensus mode (requires 2+ API keys)
consensus:
  workers:
    - provider: deepseek
      model: deepseek-chat
    - provider: qwen
      model: qwen3-coder-30b
    - provider: kimi
      model: kimi-k2-5
  arbiter: [kimi, qwen, deepseek]

# For factory mode
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
```

## Modes of Operation

### 1. Agent Mode (Primary)

Single AI agent with tools - the main interface.

```bash
# Interactive (recommended)
./zen-claw agent

# Single task
./zen-claw agent "analyze this project"

# Named session (persisted)
./zen-claw agent --session my-project "set up database"
```

**Interactive Commands:**

| Command | Description |
|---------|-------------|
| `/help` | Show all commands |
| `/session list` | List saved sessions |
| `/session load <name>` | Load a saved session |
| `/session info` | Show storage info |
| `/session clean --all` | Delete all sessions |
| `/clear` | Fresh context |
| `/provider <name>` | Switch provider |
| `/model <name>` | Switch model |
| `/think [level]` | Set reasoning depth |
| `/stats` | Show statistics |
| `/exit` | Exit |

### 2. Consensus Mode (Multi-AI)

Multiple AI workers tackle the SAME prompt, then an arbiter synthesizes the best ideas.

**Requires**: At least 2 AI providers with API keys configured.

```bash
# Security review (all AIs act as security architects)
zen-claw consensus --role security_architect "Design zero-trust auth"

# API design
zen-claw consensus --role api_designer "REST API for user management"

# Database design
zen-claw consensus --role database_architect "Schema for multi-tenant SaaS"

# Custom role
zen-claw consensus --role "kubernetes_operator_expert" "Design CRD for databases"

# View worker performance history
zen-claw consensus --stats
```

**How it works:**
1. Your prompt goes to 3+ AI workers (in parallel)
2. All workers use the SAME role (e.g., security_architect)
3. An arbiter AI synthesizes the best ideas
4. Each worker gets scored (1-10)
5. You get a unified blueprint

**Built-in roles:** `security_architect`, `software_architect`, `api_designer`, `database_architect`, `devops_engineer`, `frontend_architect`

### 3. Factory Mode (Multi-Phase Projects)

Coordinator AI manages specialist workers for complex projects.

```bash
# Start with blueprint
zen-claw factory start --blueprint blueprint.json --max-cost 5.00

# Check progress
zen-claw factory status --project my-project

# Resume if paused
zen-claw factory resume --project my-project
```

**Blueprint format** (`blueprint.json`):
```json
{
  "project": "zen-platform-v3",
  "description": "Refactor with new types",
  "phases": [
    {
      "name": "core_types",
      "task": "Define shared Go structs and TypeScript interfaces",
      "domain": "go",
      "outputs": ["zen-sdk/go/types.go"]
    },
    {
      "name": "api",
      "task": "Implement HTTP handlers",
      "domain": "go",
      "depends_on": ["core_types"]
    },
    {
      "name": "frontend",
      "task": "Generate TypeScript client",
      "domain": "typescript",
      "depends_on": ["core_types"]
    }
  ]
}
```

**Domains:** `go`, `typescript`, `infrastructure`, `coordinator`

**Guardrails:** Factory has built-in limits for cost, time, and safety.

### 4. Fabric Mode (Interactive Multi-Worker)

Interactive session with customizable workers and coordinator.

```bash
zen-claw fabric
```

**Fabric commands:**
```
/coordinator deepseek              # Set coordinator AI
/worker add go_expert qwen go_developer
/worker add ts_expert minimax typescript_developer
/worker add sec_expert kimi security_architect
/worker list                       # Show workers
/worker remove go_expert           # Remove worker

/profile save fullstack            # Save configuration
/profile load fullstack            # Load later
/profile list                      # Show saved profiles

/status                            # Show current team
/history                           # Past tasks
/verbose                           # Toggle verbose output
/exit
```

Then just type a task - it goes through all workers in parallel, then coordinator synthesizes.

## Provider Selection

| Task | Provider | Why |
|------|----------|-----|
| Quick tasks | DeepSeek | Fast, cheap |
| Go/K8s code | Kimi | Expert at Go idioms |
| Large codebase | Qwen | 262K context |
| Complex reasoning | Kimi | Strong analysis |

## Session Management

```bash
# List sessions
./zen-claw sessions list

# Storage info
./zen-claw sessions info

# Clean old sessions
./zen-claw sessions clean --older 7d

# Clean all
./zen-claw sessions clean --all
```

## Plugins

Extend zen-claw with custom tools using scripts.

```bash
# Create plugin from template
zen-claw plugins init my-tool --lang bash
zen-claw plugins init my-tool --lang python

# List installed plugins
zen-claw plugins list

# Plugin info
zen-claw plugins info
```

Plugins live in `~/.zen/zen-claw/plugins/<name>/` with a `plugin.yaml` manifest.

## RAG (Codebase Indexing)

Index your codebase for intelligent code search.

```bash
# Index current project
zen-claw index build .

# Search indexed code
zen-claw index search "authentication"

# View stats
zen-claw index stats
```

When indexed, the AI gains tools: `code_search`, `find_symbol`, `get_context`.

## Available Tools (24+)

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
| **Plugins** | Custom tools from ~/.zen/zen-claw/plugins/ |

## Troubleshooting

```bash
# Check gateway
curl http://localhost:8080/health

# Check stats
curl http://localhost:8080/stats

# Restart gateway
pkill -f "zen-claw gateway"
./zen-claw gateway start &
```

## Key Files

```
~/.zen/zen-claw/
├── config.yaml                    # Configuration
├── data/
│   └── sessions.db               # Session database
├── plugins/                       # Custom plugins
│   └── <name>/
│       ├── plugin.yaml           # Plugin manifest
│       └── run.sh                # Plugin script
├── index/                         # RAG indexes
│   └── <project>.db              # Project index (SQLite FTS5)
└── fabric-profiles/              # Saved fabric configurations

~/.zen-claw-history               # CLI history
~/.zen-claw-fabric-history        # Fabric history
```

## Next Steps

1. Try `/provider kimi` for Go code
2. Try `/think high` for complex reasoning
3. Try `zen-claw consensus --role security_architect "review auth"` for multi-AI
4. Save sessions: `--session my-project`
5. Index your codebase: `zen-claw index build .`
6. Create a custom plugin: `zen-claw plugins init my-tool`
7. Check `/stats` for cache efficiency
