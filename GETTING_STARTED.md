# Zen Claw - Getting Started

## Quick Start (5 minutes)

```bash
# 1. Build
cd ~/zen/zen-claw
go build -o zen-claw .

# 2. Set up API key (get free key from https://platform.deepseek.com)
export DEEPSEEK_API_KEY=sk-your-key-here

# 3. Start gateway
./zen-claw gateway start &

# 4. Interactive mode (recommended)
./zen-claw agent
```

You'll see:
```
🚀 Zen Agent
════════════════════════════════════════════════════════════════════════════════
Fresh context (use --session <name> to save)
Working directory: .

Commands: /help, /sessions, /models, /provider, /stats, /clear, /exit
════════════════════════════════════════════════════════════════════════════════
Provider: deepseek, Model: deepseek-chat
════════════════════════════════════════════════════════════════════════════════
> 
```

## Configuration

### Option 1: Environment Variables (Quick)
```bash
export DEEPSEEK_API_KEY="sk-..."     # Recommended: Free tier available
export KIMI_API_KEY="sk-..."          # $0.10/M, great for Go/K8s
export QWEN_API_KEY="sk-..."          # 262K context window
```

### Option 2: Config File (Persistent)
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
  # db_path: ~/.zen/zen-claw/data/sessions.db  # Custom path (optional)

preferences:
  fallback_order: [deepseek, kimi, glm, minimax, qwen, openai]

# MCP Servers (optional) - auto-connect on gateway start
# mcp:
#   servers:
#     - name: myserver
#       command: /path/to/mcp-server
#       args: ["--flag"]
```

## Getting API Keys

| Provider | URL | Notes |
|----------|-----|-------|
| **DeepSeek** | https://platform.deepseek.com | Free tier, fast |
| **Kimi** | https://platform.moonshot.cn | $0.10/M, Go/K8s expert |
| **Qwen** | https://dashscope.aliyuncs.com | 262K context |
| **GLM** | https://open.bigmodel.cn | Chinese support |
| **Minimax** | https://api.minimax.chat | Balanced |
| **OpenAI** | https://platform.openai.com | Fallback |

## Interactive Mode (Primary Interface)

```bash
./zen-claw agent
```

### Available Commands

| Command | Description |
|---------|-------------|
| `/help` | Show all commands |
| `/sessions` | List saved sessions |
| `/sessions info` | Show storage info (path, size, count) |
| `/sessions clean --all` | Delete all sessions |
| `/sessions clean --older 7d` | Delete sessions older than 7 days |
| `/sessions delete <name>` | Delete specific session |
| `/load <name>` | Load a saved session |
| `/clear` | Fresh context (like Cursor Cmd+N) |
| `/provider <name>` | Switch provider (deepseek, kimi, qwen, etc.) |
| `/model <name>` | Switch model |
| `/models` | List models for current provider |
| `/think [level]` | Set reasoning depth (off/low/medium/high) |
| `/stats` | Show usage and cache statistics |
| `/prefs` | View AI routing preferences |
| `/exit` | Exit |

### Example Session

```
> /provider kimi
Switched to provider: kimi (model: kimi-k2-5)

> analyze main.go and suggest improvements

[1] 
    list_dir(path=".")
[2] 
    read_file(path="main.go")

════════════════════════════════════════════════════════════════════════════════
🎯 RESULT
════════════════════════════════════════════════════════════════════════════════
Here are my suggestions for main.go:
1. ...

> /sessions
📋 Saved Sessions:
──────────────────────────────────────────────────
  No saved sessions
  Use --session <name> to save a session
──────────────────────────────────────────────────
Commands: /sessions info, /sessions clean [--all|--older 7d]
```

## Session Management

### Named Sessions (Persistent)
```bash
# Start with a named session (will be saved)
./zen-claw agent --session my-project

# Continue later
./zen-claw agent --session my-project
```

### CLI Commands
```bash
# List all sessions
./zen-claw sessions list

# Show storage info
./zen-claw sessions info
Session Storage Info
──────────────────────────────────────────────────
Database: /home/user/.zen/zen-claw/data/sessions.db
Size: 24.5 KB
Sessions: 3

# Clean old sessions
./zen-claw sessions clean --older 7d

# Delete all sessions
./zen-claw sessions clean --all
```

## Provider Selection Guide

| Task | Provider | Why |
|------|----------|-----|
| Quick tasks | deepseek | Fast, cheap |
| Go/K8s code | kimi | Expert at Go idioms |
| Large codebase | qwen | 262K context |
| Complex reasoning | kimi | Strong analysis |

## Available Tools (20+)

| Category | Tools |
|----------|-------|
| **File** | read_file, write_file, edit_file, append_file, list_dir, search_files |
| **Git** | git_status, git_diff, git_add, git_commit, git_push, git_log |
| **Preview** | preview_write, preview_edit |
| **Web** | web_search, web_fetch |
| **System** | exec, system_info, process |
| **Advanced** | apply_patch |
| **MCP** | External tools via Model Context Protocol |

## Architecture

```
┌────────────┐    SSE     ┌────────────┐    API    ┌────────────┐
│   CLI      │◄──────────►│  Gateway   │◄─────────►│ Providers  │
│  Client    │  Stream    │  :8080     │           │ +Circuit   │
└────────────┘            └─────┬──────┘           └────────────┘
                                │
                          ┌─────▼─────┐
                          │  SQLite   │
                          │ Sessions  │
                          └───────────┘
```

1. **Gateway** - Required, manages AI connections and sessions
2. **CLI** - Interactive mode is primary interface
3. **SQLite** - ACID-compliant session persistence

## Cost Optimization

The gateway automatically optimizes token usage. Configure in `config.yaml`:

```yaml
cost_optimization:
  # History limits
  max_history_turns: 50      # Hard cap on messages (drop oldest)
  max_history_messages: 20   # Summarize beyond this
  keep_last_assistants: 3    # Keep last N assistant msgs intact

  # Tool output pruning
  max_tool_result_tokens: 8000
  tool_rules:
    exec:           # Shell output: prune aggressively
      max_tokens: 4000
      keep_recent: 1
      aggressive: true
    read_file:      # Code: keep more context
      max_tokens: 12000
      keep_recent: 2

  # Anthropic prompt caching (when using Anthropic)
  anthropic_cache_retention: "short"  # "none", "short" (5m), "long" (1h)
```

### What gets optimized:

| Feature | Savings | How |
|---------|---------|-----|
| History turn limit | 20-40% | Drop messages beyond cap |
| Tool-specific pruning | 15-30% | Aggressive for exec, gentle for code |
| Memory flush | - | Extract decisions/TODOs before dropping |
| Prompt compression | 5-15% | Remove whitespace, verbose phrases |
| Request deduplication | Variable | Share results for identical requests |
| Semantic caching | 10-30% | Cache similar queries |

## Troubleshooting

### Gateway Issues
```bash
# Check if running
curl http://localhost:8080/health

# Check stats
curl http://localhost:8080/stats

# Restart gateway
pkill -f "zen-claw gateway"
./zen-claw gateway start &
```

### API Key Issues
```bash
# Check config
cat ~/.zen/zen-claw/config.yaml

# Check environment
echo $DEEPSEEK_API_KEY
```

### Build Issues
```bash
# Ensure Go 1.21+
go version

# Update dependencies
go mod tidy

# Rebuild
go build -o zen-claw .
```

## Key Files

```
~/.zen/zen-claw/
├── config.yaml                    # Configuration
└── data/
    └── sessions.db               # Session database (SQLite)

~/.zen-claw-history               # CLI history
```

## Next Steps

1. Try different providers: `/provider kimi`
2. Use `/think high` for complex reasoning
3. Save important sessions: `--session my-project`
4. Check `/stats` to see cache efficiency
5. Read [API.md](API.md) for gateway API

Ready to go! 🚀
