#!/bin/bash

echo "=== Zen Claw Session Management Demo ==="
echo

echo "1. Building zen-claw..."
go build -o zen-claw .

echo
echo "2. Showing help with session features:"
./zen-claw agent --help | head -20

echo
echo "3. Testing session ID flag functionality:"
echo "   Command: ./zen-claw agent --session-id test-session \"do all 16 enhancements\""
echo "   (This would normally create a task list, but gateway is not running)"

echo
echo "4. Testing interactive mode:"
echo "   Command: ./zen-claw agent"
echo "   (Will enter interactive mode with session support)"

echo
echo "5. Available session flags:"
echo "   --session-id <id>    - Save session ID for continuation"
echo "   --working-dir <dir>  - Set working directory"
echo "   --provider <name>    - AI provider (deepseek, qwen, glm, etc.)"
echo "   --model <name>       - Specific AI model"

echo
echo "=== Session Management Features Implemented ==="
echo "- Session ID tracking for continuity across clients"
echo "- Task list generation for complex projects (like 16 enhancements)"
echo "- Working directory persistence"
echo "- Provider/model configuration per session"
echo "- Interactive mode with session awareness"
echo "- Gateway integration for distributed sessions"

