# Zen Claw Session Management Guide

## Overview
This guide explains how to use zen-claw with Byobu for multi-provider sessions that can be continued from terminal or Slack.

## Interactive Model Switching

One of the key features for your workflow is the ability to switch models during a session:

### Model Switching Commands
- **`/models`** - List all available models
- **`/model qwen/qwen3-coder-30b`** - Switch to Qwen Coder
- **`/model deepseek/deepseek-chat`** - Switch to DeepSeek
- **`/model openai/gpt-4o`** - Switch to OpenAI GPT-4

### Example Workflow
```bash
# Start with default model (DeepSeek)
zen-claw agent "analyze codebase"

# In the session, type:
/models                    # See available models
/model qwen/qwen3-coder-30b # Switch to Qwen
/code-review               # Continue with Qwen
```

## Session Creation Commands

### Qwen Session
```bash
# Start a Qwen session in Byobu pane
byobu new-window -n "qwen-session" "zen-claw agent --provider qwen"
```

### DeepSeek Session  
```bash
# Start a DeepSeek session in Byobu pane
byobu new-window -n "deepseek-session" "zen-claw agent --provider deepseek"
```

## Session Management

### List Sessions
```bash
# List all active sessions
zen-claw session list
```

### Continue Session from Terminal
```bash
# Resume a specific session
zen-claw agent --session-id <session-id> "continue from here"
```

### Tag Sessions for Organization
```bash
# Tag sessions for easy identification
zen-claw session tag <session-id> "code-review"
zen-claw session tag <session-id> "qwen-analysis"
```

## Byobu Setup

### Create Session Manager Script
```bash
#!/bin/bash
# ~/bin/zen-sessions.sh

echo "=== Zen Claw Session Manager ==="
echo "1. Qwen Session"
echo "2. DeepSeek Session" 
echo "3. List Sessions"
echo "4. Continue Session"
echo -n "Choose option: "
read choice

case $choice in
  1)
    byobu new-window -n "qwen" "zen-claw agent --provider qwen"
    ;;
  2)
    byobu new-window -n "deepseek" "zen-claw agent --provider deepseek"
    ;;
  3)
    zen-claw session list
    ;;
  4)
    echo "Enter session ID:"
    read session_id
    byobu new-window -n "continue-$session_id" "zen-claw agent --session-id $session_id"
    ;;
esac
```

### Set Up Byobu Profile
Create a Byobu profile for zen-claw sessions:
```bash
# ~/.byobu/profile.tmux
# Add these lines to customize your Byobu environment
set -g mouse on
set -g status-style bg=black,fg=white
set -g status-left-length 20
set -g status-right-length 40
```

## Slack Integration

### Session Attachment
Once Slack integration is implemented, you'll be able to:
1. Start a session in terminal
2. Get the session ID
3. Attach to the same session from Slack using the session ID
4. Continue work seamlessly

### Session Persistence
All sessions are automatically saved in:
```
~/.zen/zen-claw/sessions/
```

## Recommended Workflow

### Morning Setup
```bash
# Start byobu session
byobu new-session -s zen-work

# Open panes for different providers
byobu split-window -h
byobu split-window -v
# Customize panes for different tasks
```

### Session Organization
Use session tags to organize:
```bash
# Tag for code review
zen-claw session tag <session-id> "code-review"

# Tag for documentation
zen-claw session tag <session-id> "docs"

# Tag for architecture
zen-claw session tag <session-id> "architecture"
```

## Advanced Features

### Session Export for Sharing
```bash
# Export session for sharing
zen-claw session export <session-id> --output session-backup.json
```

### Session Cleanup
```bash
# List sessions with size information
zen-claw session list --verbose

# Delete unwanted sessions
zen-claw session delete <session-id>
```

## Byobu Navigation Shortcuts

- `Ctrl+B, C` - Create new window
- `Ctrl+B, N` - Next window  
- `Ctrl+B, P` - Previous window
- `Ctrl+B, O` - Rotate panes
- `Ctrl+B, [Arrow Keys]` - Switch panes
- `Ctrl+B, X` - Close pane
- `Ctrl+B, S` - Split window horizontally
- `Ctrl+B, V` - Split window vertically

## Best Practices

1. **Always tag sessions** with meaningful labels
2. **Use descriptive session names** when creating windows
3. **Regular cleanup** of old sessions to save space
4. **Document important sessions** with notes
5. **Backup critical sessions** before major changes