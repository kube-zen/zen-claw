# Zen Claw - Complete Examples

This document shows practical examples of using Zen Claw with real-time progress streaming.

## Architecture Overview

```
┌─────────────┐  SSE Stream  ┌─────────────┐    API Calls    ┌─────────────┐
│   Agent     │◄────────────►│  Gateway    │────────────────▶│ AI Providers │
│  (Client)   │  Progress    │ (Server)    │◀────────────────│ DeepSeek,   │
└─────────────┘  Events      └─────────────┘                 │ Kimi, Qwen  │
     │                              │                        └─────────────┘
     │                              │
     │                         Session Store
     │                         (Persistent)
     │
Tool Execution
     │
┌─────────────┐
│   Tools     │
│  exec, read │
│  write,edit │
│  search...  │
└─────────────┘
```

## 1. Basic Usage Examples

### Start the Gateway
```bash
# Start the gateway server (required for agents to work)
./zen-claw gateway start

# Check if gateway is running
curl http://localhost:8080/health
```

### Simple Agent Task
```bash
# Run a simple task with default provider (DeepSeek)
./zen-claw agent "What's in the current directory?"

# Output shows real-time progress:
# 🚀 Starting with deepseek/deepseek-chat
# 📍 Step 1/100: Thinking...
#    💭 Waiting for AI response...
#    🔧 list_dir(path=".")
#    ✓ list_dir → 34 items
# ✅ Task completed

# Run with specific provider (Kimi for Go/K8s)
./zen-claw agent --provider kimi "Analyze the Go code in this directory"

# Run with Qwen for large context
./zen-claw agent --provider qwen "Review the entire codebase"

# Run with session ID (for continuing later)
./zen-claw agent --session-id my-project "Set up a new Go project"
```

### Interactive Mode
```bash
# Start interactive session
./zen-claw agent

# In interactive mode, use these commands:
/providers           # List available AI providers
/provider qwen       # Switch to Qwen provider
/models             # List models for current provider
/model qwen-plus     # Switch to a different model
/help               # Show help
/exit               # Exit interactive mode
```

## 2. Code Analysis Examples

### Analyze a Project Structure
```bash
./zen-claw agent --provider qwen "cd to ~/zen/zen-claw and analyze the project structure"
```

### Review Specific Files
```bash
./zen-claw agent "Read main.go and go.mod, then suggest improvements"
```

### Performance Analysis
```bash
./zen-claw agent --provider qwen "Analyze this Go code for performance bottlenecks"
```

## 3. Development Workflow Examples

### Set Up a New Project
```bash
./zen-claw agent --session-id new-project "Set up a new Go web server with:
1. Create main.go with HTTP server
2. Create go.mod file
3. Create README.md
4. Create a simple test"
```

### Debug Issues
```bash
./zen-claw agent --verbose "Debug why this Go program won't compile"
```

### Refactor Code
```bash
./zen-claw agent "Refactor this function to be more readable and efficient"
```

## 4. Gateway API Examples

The gateway provides a REST API that agents and other clients can use:

### Health Check
```bash
curl http://localhost:8080/health
```

### Chat Endpoint
```bash
# Send a chat request to the gateway
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{
    "user_input": "What is in the current directory?",
    "session_id": "test-session",
    "provider": "qwen",
    "model": "qwen3-coder-30b-a3b-instruct",
    "working_dir": ".",
    "max_steps": 10
  }'
```

### List Sessions
```bash
curl http://localhost:8080/sessions
```

### Get Specific Session
```bash
curl http://localhost:8080/sessions/test-session
```

### Delete Session
```bash
curl -X DELETE http://localhost:8080/sessions/test-session
```

### Streaming Chat (SSE)
```bash
# Get real-time progress events
curl -N -X POST http://localhost:8080/chat/stream \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{
    "user_input": "analyze this codebase",
    "provider": "kimi",
    "working_dir": "."
  }'

# Output is Server-Sent Events:
# data: {"type":"start","message":"Starting with kimi/kimi-k2-5"}
# data: {"type":"step","step":1,"message":"Step 1/100: Thinking..."}
# data: {"type":"tool_call","step":1,"message":"🔧 list_dir(path=\".\")"}
# data: {"type":"tool_result","step":1,"message":"✓ list_dir → 34 items"}
# data: {"type":"done","session_id":"...","result":"..."}
```

## 5. Provider-Specific Examples

### Using Kimi (256K Context, Go/K8s Expert)
```bash
# Kimi K2.5 is excellent for Go and Kubernetes code
./zen-claw agent --provider kimi "Review this Kubernetes operator code"

# $0.10/M input tokens, cached system prompts get 90% discount
./zen-claw agent --provider kimi --model kimi-k2-5 \
  "Analyze this controller-runtime implementation"
```

### Using Qwen (262K Context)
```bash
# Qwen is excellent for large codebases
./zen-claw agent --provider qwen --model qwen3-coder-30b-a3b-instruct \
  "Analyze the entire codebase and suggest architectural improvements"
```

### Using DeepSeek (Fast and Cheap)
```bash
# DeepSeek is great for quick tasks
./zen-claw agent --provider deepseek "Write a simple Go function"
```

## 6. Tool Usage Examples

The AI can use these tools automatically:

### File Operations
```bash
# The AI will use read_file, list_dir tools automatically
./zen-claw agent "Read the README.md file"
```

### Shell Commands
```bash
# The AI can run commands with exec tool
./zen-claw agent "Check Go version and list Go files"
```

### Directory Navigation
```bash
# The AI can change directories
./zen-claw agent "Go to the src directory and list files"
```

## 7. Session Management Examples

### Continue Previous Session
```bash
# Start a session with an ID
./zen-claw agent --session-id my-task "Set up a database connection"

# Later, continue the same session
./zen-claw agent --session-id my-task "Now add authentication"
```

### List All Sessions
```bash
# Sessions are stored in /tmp/zen-claw-sessions/
ls -la /tmp/zen-claw-sessions/
```

## 8. Configuration Examples

### Environment Variables
```bash
# Set API keys
export QWEN_API_KEY="sk-..."
export DEEPSEEK_API_KEY="sk-..."

# Or use config file: ~/.zen/zen-claw/config.yaml
```

### Config File
```yaml
# ~/.zen/zen-claw/config.yaml
providers:
  deepseek:
    api_key: "${DEEPSEEK_API_KEY}"
    model: "deepseek-chat"
    base_url: "https://api.deepseek.com"
  
  qwen:
    api_key: "${QWEN_API_KEY}"
    model: "qwen3-coder-30b-a3b-instruct"
    base_url: "https://dashscope-us.aliyuncs.com/compatible-mode/v1"

default:
  provider: "deepseek"
  model: "deepseek-chat"
```

## 9. Real-World Workflow

### Complete Development Session
```bash
# 1. Start gateway
./zen-claw gateway start

# 2. Analyze existing code
./zen-claw agent --session-id project-analysis \
  "Analyze the current Go project structure and identify issues"

# 3. Implement fixes
./zen-claw agent --session-id project-analysis \
  "Implement the first improvement: add error handling"

# 4. Test changes
./zen-claw agent --session-id project-analysis \
  "Run tests and check if everything works"

# 5. Document changes
./zen-claw agent --session-id project-analysis \
  "Update the README with the new features"
```

## 10. Tips and Best Practices

1. **Use session IDs** for multi-step tasks
2. **Use Kimi** for Go/K8s code ($0.10/M, great for DevOps)
3. **Use Qwen** for large codebases (262K context)
4. **Use DeepSeek** for quick tasks (fast response)
5. **Watch the progress** - streaming shows exactly what's happening
6. **Be specific** in your requests
7. **Use --max-steps** for complex refactoring tasks
8. **Monitor session files** in `/tmp/zen-claw-sessions/`

## Provider Selection Guide

| Use Case | Recommended Provider | Why |
|----------|---------------------|-----|
| Go/K8s code | kimi | Excellent at Go idioms, cheap |
| Large codebase | qwen | 262K context window |
| Quick tasks | deepseek | Fast response time |
| Complex analysis | kimi or qwen | Large context, reasoning |
| Fallback | openai | Most reliable |

## Troubleshooting

If something doesn't work:
1. Check if gateway is running: `curl http://localhost:8080/health`
2. Check API keys are configured
3. Watch streaming progress for errors
4. Check `/tmp/gateway.log` for errors
5. Restart gateway: `pkill -f "zen-claw gateway" && ./zen-claw gateway start`

## Next Steps

Explore more advanced features:
- Multiple concurrent agents
- Custom tool development  
- Web UI (coming soon)
- Consensus mode (multiple AI providers)
- Factory mode (domain specialists)