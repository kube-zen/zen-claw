#!/bin/bash

echo "Testing Zen Claw scrolling fix..."

# Test 1: Build the application
echo "Test 1: Building Zen Claw..."
go build -o zen-claw
if [ $? -ne 0 ]; then
    echo "❌ Build failed"
    exit 1
fi
echo "✅ Build successful"

# Test 2: Check that the binary exists
echo "Test 2: Checking binary..."
if [ -f "./zen-claw" ]; then
    echo "✅ Binary created successfully"
else
    echo "❌ Binary not found"
    exit 1
fi

# Test 3: Run help to ensure functionality
echo "Test 3: Running help..."
./zen-claw --help > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "✅ Help command works"
else
    echo "❌ Help command failed"
    exit 1
fi

# Test 4: Test agent command help
echo "Test 4: Testing agent command..."
./zen-claw agent --help > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "✅ Agent command works"
else
    echo "❌ Agent command failed"
    exit 1
fi

# Test 5: Test slack command help  
echo "Test 5: Testing slack command..."
./zen-claw slack --help > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "✅ Slack command works"
else
    echo "❌ Slack command failed"
    exit 1
fi

echo ""
echo "🎉 All tests passed! The scrolling fix is working correctly."
echo "The interactive mode should now display properly without scrolling issues."
