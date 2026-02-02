# Zen Claw Troubleshooting Guide

This guide helps you diagnose and fix common issues with Zen Claw.

## Quick Start Checklist

If Zen Claw isn't working, check these first:

1. ✅ **Gateway running**: `curl http://localhost:8080/health`
2. ✅ **API keys configured**: Check `~/.zen/zen-claw/config.yaml` or environment variables
3. ✅ **Binary compiled**: `go build -o zen-claw .` after code changes
4. ✅ **Port 8080 available**: No other service using port 8080

## Common Issues and Solutions

### 1. Gateway Won't Start

**Symptoms:**
- `./zen-claw gateway start` fails or exits immediately
- Port 8080 is already in use
- Permission errors

**Solutions:**

**Port 8080 already in use:**
```bash
# Check what's using port 8080
sudo lsof -i :8080

# Kill the process
sudo kill -9 <PID>

# Or use a different port (future feature)
```

**Permission denied:**
```bash
# Make sure binary is executable
chmod +x zen-claw

# Check if you can write to /tmp
ls -la /tmp/
```

**Gateway starts but immediately stops:**
```bash
# Check logs
tail -f /tmp/zen-gateway-*.log

# Common causes:
# - Missing API keys
# - Invalid config file
# - Port conflict
```

### 2. Agent Can't Connect to Gateway

**Symptoms:**
- `❌ Gateway not available` error
- Agent times out waiting for gateway
- `curl http://localhost:8080/health` fails

**Solutions:**

**Gateway not running:**
```bash
# Start gateway first
./zen-claw gateway start

# Wait a few seconds, then check
sleep 3
curl http://localhost:8080/health
```

**Firewall blocking port:**
```bash
# Check if port is open locally
nc -z localhost 8080

# If using a different machine, check firewall rules
```

**Gateway crashed:**
```bash
# Check gateway logs
tail -50 /tmp/zen-gateway-*.log

# Restart gateway
pkill -f "zen-claw gateway"
./zen-claw gateway start
```

### 3. AI Provider Errors

**Symptoms:**
- `❌ Agent error: AI response failed`
- `Model Not Exist` or `Model access denied`
- `API error: 401 Unauthorized`

**Solutions:**

**Missing API key:**
```bash
# Check if API key is set
echo $QWEN_API_KEY
echo $DEEPSEEK_API_KEY

# Or check config file
cat ~/.zen/zen-claw/config.yaml
```

**Invalid API key:**
```bash
# Test API key directly
curl -X POST https://api.deepseek.com/chat/completions \
  -H "Authorization: Bearer $DEEPSEEK_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}]}'
```

**Wrong model name:**
```bash
# Use correct model names:
# - Qwen: qwen3-coder-30b-a3b-instruct
# - DeepSeek: deepseek-chat
# - GLM: glm-4.7
# - Minimax: minimax-M2.1
# - OpenAI: gpt-4o-mini

# List models for current provider in interactive mode:
/providers
/provider qwen
/models
```

**Provider not configured:**
```bash
# Check which providers are configured
grep -A3 "providers:" ~/.zen/zen-claw/config.yaml

# Add missing provider to config
```

### 4. Model Switching Issues

**Symptoms:**
- `/model` command doesn't switch providers
- `Model Not Exist` when switching models
- Provider doesn't change with model

**Solutions:**

**Use correct commands:**
```bash
# Wrong: /models qwen (shows all models)
# Correct: /provider qwen then /models

# Wrong: /model deepseek-chat (when provider is qwen)
# Correct: /provider deepseek then /model deepseek-chat
```

**Rebuild after code changes:**
```bash
# Always rebuild after git pull or code changes
go build -o zen-claw .
```

### 5. Slow Performance

**Symptoms:**
- Agent takes 30+ seconds for simple tasks
- API calls time out
- Gateway becomes unresponsive

**Solutions:**

**API latency:**
```bash
# Some APIs are slower than others
# Qwen: 2-3 seconds per call
# DeepSeek: 1-2 seconds per call

# Use --timeout flag (future feature)
# Or switch to faster provider
```

**Too many steps:**
```bash
# Complex tasks can take many steps
# Limit steps with --max-steps
./zen-claw agent --max-steps 5 "simple task"
```

**Large context:**
```bash
# Sessions with many messages get slow
# Start fresh session for new tasks
./zen-claw agent --session-id new-task "task"
```

### 6. Tool Execution Issues

**Symptoms:**
- `cd` command doesn't change directory
- File operations fail
- `exec` commands don't work

**Solutions:**

**cd command issues:**
```bash
# cd only affects subsequent commands in same session
# Use absolute paths for reliability
./zen-claw agent "cd to ~/zen/zen-claw and list files"
```

**File permission errors:**
```bash
# Check file permissions
ls -la file.txt

# Agent runs as your user, needs read/write access
```

**exec command failures:**
```bash
# Some commands need full path
# Use which to find path
which go
which python
```

### 7. Session Issues

**Symptoms:**
- Sessions not persisting after restart
- Can't continue previous session
- Session files corrupted

**Solutions:**

**Session directory:**
```bash
# Check session directory exists
ls -la /tmp/zen-claw-sessions/

# Create if missing
mkdir -p /tmp/zen-claw-sessions/
```

**Session ID:**
```bash
# Use --session-id to continue sessions
./zen-claw agent --session-id my-project "continue work"

# List all sessions
curl http://localhost:8080/sessions
```

**Corrupted session:**
```bash
# Delete corrupted session
rm /tmp/zen-claw-sessions/corrupted-session.json

# Or via API
curl -X DELETE http://localhost:8080/sessions/corrupted-session
```

### 8. Compilation Errors

**Symptoms:**
- `go build` fails
- Missing dependencies
- Type errors

**Solutions:**

**Go version:**
```bash
# Check Go version (needs 1.24+)
go version

# Update Go if needed
```

**Missing dependencies:**
```bash
# Download dependencies
go mod download

# Tidy module
go mod tidy
```

**Type errors after changes:**
```bash
# Check for syntax errors
go vet ./...

# Build to see errors
go build -o zen-claw . 2>&1
```

## Debugging Techniques

### Enable Verbose Logging
```bash
# Start gateway with verbose logging
./zen-claw gateway start --verbose

# Run agent with verbose flag
./zen-claw agent --verbose "task"
```

### Check Logs
```bash
# Gateway logs
tail -f /tmp/zen-gateway-*.log

# Agent output (with --verbose)
./zen-claw agent --verbose "task" 2>&1 | tee agent.log
```

### Test API Directly
```bash
# Test gateway API
curl http://localhost:8080/health
curl http://localhost:8080/sessions

# Test AI provider API
curl -X POST https://api.deepseek.com/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-chat","messages":[{"role":"user","content":"test"}]}'
```

### Monitor Processes
```bash
# Check if gateway is running
ps aux | grep "zen-claw gateway"

# Check port usage
sudo netstat -tlnp | grep :8080
```

## Performance Optimization

### For Faster Responses
1. **Use DeepSeek** for simple tasks (fastest)
2. **Limit steps** with `--max-steps 5`
3. **Be specific** in prompts
4. **Use fresh sessions** for new tasks

### For Large Codebases
1. **Use Qwen** with 262K context
2. **Break tasks** into smaller steps
3. **Use absolute paths** in requests
4. **Monitor memory usage**

### Gateway Tuning
1. **Increase timeouts** in code (future feature)
2. **Add caching** (future feature)
3. **Use connection pooling** (future feature)

## Common Error Messages

### `❌ Gateway not available`
- Gateway not running
- Port 8080 blocked
- Network issues

### `❌ Agent error: AI response failed`
- API key invalid
- Model doesn't exist
- Provider not configured
- Network timeout

### `Model Not Exist`
- Wrong model name for provider
- Typo in model name
- Provider doesn't support model

### `API error: 401 Unauthorized`
- Invalid API key
- Expired API key
- Wrong API endpoint

### `API error: 403 Forbidden`
- No access to model
- Rate limited
- Account issues

### `API error: 404 Not Found`
- Wrong API endpoint
- Model doesn't exist
- Provider API changed

### `context deadline exceeded`
- API call too slow
- Network issues
- Increase timeout in code

## Getting Help

If you still have issues:

1. **Check logs**: `/tmp/zen-gateway-*.log`
2. **Test manually**: Use `curl` to test APIs
3. **Simplify**: Try minimal test case
4. **Search issues**: Check GitHub issues (future)
5. **Ask for help**: Provide logs and error messages

## Prevention Tips

1. **Always rebuild** after code changes
2. **Check gateway health** before using agent
3. **Use session IDs** for important work
4. **Backup session files** regularly
5. **Monitor logs** for early warnings
6. **Test API keys** separately
7. **Keep Go updated**
8. **Use version control** for config files