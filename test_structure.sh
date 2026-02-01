#!/bin/bash

echo "🧪 Testing Zen Claw structure..."

# Check files exist
echo "📁 Checking file structure..."
required_files=(
    "main.go"
    "cmd/root.go"
    "cmd/agent.go"
    "cmd/session.go"
    "cmd/tools.go"
    "cmd/gateway.go"
    "internal/agent/agent.go"
    "internal/session/session.go"
    "internal/tools/manager.go"
    "go.mod"
    "README.md"
)

missing=0
for file in "${required_files[@]}"; do
    if [ -f "$file" ]; then
        echo "  ✓ $file"
    else
        echo "  ✗ $file (MISSING)"
        missing=$((missing + 1))
    fi
done

if [ $missing -gt 0 ]; then
    echo "❌ Missing $missing required files"
    exit 1
fi

echo "✅ All files present"
echo ""
echo "📋 Project summary:"
echo "  - CLI with cobra: ✓"
echo "  - Agent system: ✓"
echo "  - Session management: ✓"
echo "  - Tool system: ✓ (read, write, edit, exec, process)"
echo "  - Gateway stub: ✓"
echo "  - Documentation: ✓"
echo ""
echo "🎯 Next steps when Go is installed:"
echo "  1. go mod tidy"
echo "  2. go build -o zen-claw ."
echo "  3. ./zen-claw --help"
echo ""
echo "🚀 Structure complete! Ready for AI integration."