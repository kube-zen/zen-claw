# Zen Claw - Getting Started

## 🚀 Quick Start

```bash
# Build
cd ~/git/zen-claw
go build -o zen-claw .

# Check configuration
./zen-claw config check

# Test with mock provider (no API key needed)
./zen-claw agent --model mock --task "read README.md"

# Start interactive mode with mock
./zen-claw agent --model mock
```

## 📁 Configuration

Zen Claw uses YAML configuration at `~/.zen/zen-claw/config.yaml`:

```yaml
# Default settings
default:
  provider: deepseek  # Default provider
  model: deepseek-chat
  thinking: false

# Workspace
workspace:
  path: ~/.zen/zen-claw/workspace

# AI Providers
providers:
  deepseek:
    api_key: ${DEEPSEEK_API_KEY}  # Get from https://platform.deepseek.com
    model: deepseek-chat
    base_url: "https://api.deepseek.com"
  
  glm:
    api_key: ${GLM_API_KEY}  # Get from https://open.bigmodel.cn/
    model: glm-4.7
    base_url: "https://open.bigmodel.cn/api/paas/v4"
  
  minimax:
    api_key: ${MINIMAX_API_KEY}  # Get from https://api.minimax.chat/
    model: minimax-M2.1
    base_url: "https://api.minimax.chat/v1"
  
  openai:
    api_key: ${OPENAI_API_KEY}
    model: gpt-4o-mini
    base_url: "https://api.openai.com/v1"
```

## 🔑 Getting API Keys

### 1. **DeepSeek** (Recommended - Free tier)
- Visit: https://platform.deepseek.com
- Sign up and get API key
- Free tier: 1M tokens/month

### 2. **GLM-4.7** (Zhipu AI)
- Visit: https://open.bigmodel.cn
- Sign up and get API key
- Good Chinese support, OpenAI-compatible

### 3. **Minimax R2**
- Visit: https://api.minimax.chat
- Sign up and get API key
- Good Chinese/English balance

### 4. **OpenAI** (Fallback)
- Visit: https://platform.openai.com
- Get API key for GPT-4o/GPT-3.5

## 🛠️ Configuration Commands

```bash
# Initialize config
./zen-claw config init

# Show config
./zen-claw config show

# Check config status
./zen-claw config check

# Show config path
./zen-claw config path
```

## 🤖 Using Zen Claw

### With Mock Provider (Testing)
```bash
# Test tool calling
./zen-claw agent --model mock --task "read README.md"
./zen-claw agent --model mock --task "write hello.txt with content"

# Interactive mode
./zen-claw agent --model mock
```

### With Real AI Provider
```bash
# Set API key
export DEEPSEEK_API_KEY=your_key_here

# Use DeepSeek
./zen-claw agent --model deepseek --task "Write a Go function"

# Or use config default
./zen-claw agent --task "Help me with code"
```

## 🧪 Available Tools

Zen Claw provides these tools to AI:
- `read` - Read files
- `write` - Write files  
- `edit` - Edit files
- `exec` - Execute commands
- `process` - Manage processes

## 🏗️ Project Structure

```
~/.zen/zen-claw/
├── config.yaml          # Configuration
└── workspace/          # Agent workspace

~/git/zen-claw/
├── zen-claw           # Binary
├── cmd/               # CLI commands
├── internal/          # Core packages
│   ├── config/       # YAML config
│   ├── providers/    # AI providers
│   ├── agent/        # Agent logic
│   ├── tools/        # Tool implementations
│   └── session/      # Session management
└── *.md              # Documentation
```

## 🔧 Development

```bash
# Build
go build -o zen-claw .

# Run tests
go test ./...

# Add dependency
go get github.com/package/name

# Update dependencies
go mod tidy
```

## 🎯 Example Workflow

1. **Setup**
```bash
cd ~/git/zen-claw
go build -o zen-claw .
./zen-claw config init
# Edit ~/.zen/zen-claw/config.yaml with API keys
```

2. **Test**
```bash
# Test with mock
./zen-claw agent --model mock --task "List files in workspace"

# Test with real AI (after setting API key)
export DEEPSEEK_API_KEY=your_key
./zen-claw agent --task "Write a Python script"
```

3. **Use**
```bash
# Interactive mode
./zen-claw agent

# One-off tasks
./zen-claw agent --task "Analyze this code"
./zen-claw agent --task "Search for information about X"
```

## 📊 Provider Comparison

| Provider | Cost | Chinese | Tools | Best For |
|----------|------|---------|-------|----------|
| **DeepSeek** | Free tier | Excellent | ✅ | General use, testing |
| **GLM-4.7** | Low | Excellent | ✅ | Chinese tasks |
| **Minimax R2** | Medium | Good | ✅ | Balanced tasks |
| **OpenAI** | High | Good | ✅ | Fallback, English |

## 🚨 Troubleshooting

### "API key not found"
- Set API key in config file or environment variable
- Use mock provider for testing: `--model mock`

### "Tool calling not working"
- Mock provider only calls tools for specific patterns
- Real providers need tool support (all 4 providers support tools)

### "Configuration issues"
- Run `./zen-claw config check` to diagnose
- Ensure `~/.zen/zen-claw/config.yaml` exists

### "Build errors"
- Ensure Go 1.24+ is installed
- Run `go mod tidy` to fix dependencies

## 🎉 Next Steps

1. Get a DeepSeek API key (free)
2. Test with real AI: `export DEEPSEEK_API_KEY=key && ./zen-claw agent`
3. Explore tool capabilities
4. Customize configuration as needed

Zen Claw is now ready for production use with real AI providers! 🚀