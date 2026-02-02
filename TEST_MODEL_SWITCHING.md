# Testing Model Switching Feature

This document demonstrates that the model switching feature has been successfully implemented.

## Feature Verification

### 1. Model Switching Commands
The following commands are now available within zen-claw sessions:

- **`/models`** - Lists all available AI models
- **`/model qwen/qwen3-coder-30b`** - Switches to Qwen Coder model
- **`/model deepseek/deepseek-chat`** - Switches to DeepSeek model
- **`/model openai/gpt-4o`** - Switches to OpenAI GPT-4

### 2. Usage Example

To test the feature, start a zen-claw agent session:

```bash
./zen-claw agent "test task"
```

Once the session is running, you can use:

```bash
/models              # Shows all available models
/model qwen/qwen3-coder-30b  # Switches to Qwen Coder
/model deepseek/deepseek-chat # Switches to DeepSeek
```

### 3. Implementation Details

The model switching feature has been implemented in:
- `internal/agent/agent.go`
- `internal/agent/agent_light.go`

Both agents now support:
- `/models` command to list available models
- `/model <model-name>` command to switch models during a session
- Proper model tracking and switching

### 4. Configuration

Default model is set to DeepSeek in the configuration:
```yaml
default:
  provider: "deepseek"
  model: "deepseek-chat"
```

But can be overridden with:
```bash
./zen-claw agent --model qwen/qwen3-coder-30b "task"
```

### 5. Session Persistence

All sessions are automatically saved in:
```
~/.zen/zen-claw/sessions/
```

Sessions retain their model preference and can be resumed with the same model context.

## Testing Results

The zen-claw binary has been verified to:
- Accept the `/models` command
- Accept the `/model <name>` command  
- Switch between different AI models during session
- Maintain session state properly

This completes all the requested functionality for interactive model switching within the zen-claw agent interface.