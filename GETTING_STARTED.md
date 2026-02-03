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

# 4. Run your first task
./zen-claw agent "list files in the current directory"
```

You'll see real-time progress:
```
🚀 Starting with deepseek/deepseek-chat
📍 Step 1/100: Thinking...
   💭 Waiting for AI response...
   🔧 list_dir(path=".")
   ✓ list_dir → 18 items
✅ Task completed
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

## Basic Usage

### One-off Tasks
```bash
# Simple question
./zen-claw agent "what is 2+2?"

# Code analysis
./zen-claw agent "analyze the main.go file"

# With specific provider
./zen-claw agent --provider kimi "review this Go code"
```

### Interactive Mode
```bash
./zen-claw agent

# Commands available:
# /provider kimi    - Switch provider
# /model kimi-k2-5  - Switch model
# /think high       - Enable deep reasoning (off/low/medium/high)
# /stats            - Show usage and cache stats
# /clear            - Fresh context
# /help             - Show all commands
# /exit             - Exit
```

### Session Management
```bash
# Start with session ID (for continuation)
./zen-claw agent --session-id my-project "set up a Go project"

# Continue later
./zen-claw agent --session-id my-project "add error handling"

# List sessions
curl http://localhost:8080/sessions
```

## Provider Selection Guide

| Task | Provider | Why |
|------|----------|-----|
| Quick tasks | deepseek | Fast, cheap |
| Go/K8s code | kimi | Expert at Go idioms |
| Large codebase | qwen | 262K context |
| Complex reasoning | kimi | Strong analysis |

## Architecture

```
┌────────────┐    SSE     ┌────────────┐    API    ┌────────────┐
│   CLI      │◄──────────►│  Gateway   │◄─────────►│ Providers  │
│  Client    │  Stream    │  :8080     │           │ DS/Kimi/.. │
└────────────┘            └────────────┘           └────────────┘
```

1. **Gateway** - Required, manages AI connections and sessions
2. **CLI** - Connects to gateway, shows progress
3. **Providers** - Multiple AI backends with fallback

## Available Tools

The AI agent has access to 20 tools:

| Tool | Description |
|------|-------------|
| `exec` | Run shell commands |
| `read_file` | Read file contents |
| `write_file` | Create/overwrite files |
| `edit_file` | String replacement |
| `append_file` | Append to files |
| `list_dir` | List directory |
| `search_files` | Regex search |
| `system_info` | System information |
| `git_status` | Show branch, staged/unstaged/untracked files |
| `git_diff` | Show changes (staged, by file, by commit) |
| `git_add` | Stage files for commit |
| `git_commit` | Commit with message |
| `git_push` | Push to remote |
| `git_log` | Show commit history |
| `preview_write` | Preview diff before writing file |
| `preview_edit` | Preview diff before editing file |
| `web_search` | Search the web (Brave Search API) |
| `web_fetch` | Fetch URL and extract readable content |
| `process` | Background process management (start/poll/kill) |
| `apply_patch` | Multi-file structured patches |

## Troubleshooting

### Gateway Issues
```bash
# Check if running
curl http://localhost:8080/health

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

# Test API directly
curl -X POST https://api.deepseek.com/chat/completions \
  -H "Authorization: Bearer $DEEPSEEK_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}]}'
```

### Build Issues
```bash
# Ensure Go 1.24+
go version

# Update dependencies
go mod tidy

# Rebuild
go build -o zen-claw .
```

## Next Steps

1. Try different providers: `--provider kimi`
2. Use interactive mode for multi-step tasks
3. Explore the API: `curl http://localhost:8080/`
4. Read [EXAMPLE.md](EXAMPLE.md) for advanced usage
5. Check [API.md](API.md) for API documentation

## Key Files

```
~/.zen/zen-claw/
├── config.yaml              # Configuration
└── workspace/              # Agent workspace

/tmp/zen-claw-sessions/      # Session persistence
/tmp/gateway.log            # Gateway logs
```

Ready to go! 🚀
