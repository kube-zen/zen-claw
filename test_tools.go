package main

import (
	"fmt"
	"os"

	"github.com/neves/zen-claw/internal/tools"
	"github.com/neves/zen-claw/internal/session"
)

func main() {
	// Create a test workspace
	workspace := "/tmp/zen-claw-test"
	os.RemoveAll(workspace)
	os.MkdirAll(workspace, 0755)

	// Create session
	sess := session.New(session.Config{
		Workspace: workspace,
		Model:     "test",
	})

	// Create tool manager
	toolMgr, err := tools.NewManager(tools.Config{
		Workspace: workspace,
		Session:   sess,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("🧪 Testing Zen Claw Tools")
	fmt.Println("=========================")

	// Test 1: Write a file
	fmt.Println("\n1. Testing write tool...")
	result, err := toolMgr.Execute("write", map[string]interface{}{
		"path":    "test.txt",
		"content": "Hello, Zen Claw!",
	})
	if err != nil {
		fmt.Printf("   ❌ Write failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Write success: %v\n", result)
	}

	// Test 2: Read the file
	fmt.Println("\n2. Testing read tool...")
	result, err = toolMgr.Execute("read", map[string]interface{}{
		"path": "test.txt",
	})
	if err != nil {
		fmt.Printf("   ❌ Read failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Read success: %v\n", result)
	}

	// Test 3: Edit the file
	fmt.Println("\n3. Testing edit tool...")
	result, err = toolMgr.Execute("edit", map[string]interface{}{
		"path":    "test.txt",
		"oldText": "Hello, Zen Claw!",
		"newText": "Hello, Zen Claw World!",
	})
	if err != nil {
		fmt.Printf("   ❌ Edit failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Edit success: %v\n", result)
	}

	// Test 4: Read again to verify edit
	fmt.Println("\n4. Verifying edit...")
	result, err = toolMgr.Execute("read", map[string]interface{}{
		"path": "test.txt",
	})
	if err != nil {
		fmt.Printf("   ❌ Read failed: %v\n", err)
	} else {
		content := result.(string)
		if content == "Hello, Zen Claw World!" {
			fmt.Printf("   ✅ Edit verified: %v\n", content)
		} else {
			fmt.Printf("   ❌ Edit mismatch: %v\n", content)
		}
	}

	// Test 5: Execute a command
	fmt.Println("\n5. Testing exec tool...")
	result, err = toolMgr.Execute("exec", map[string]interface{}{
		"command": "echo 'Exec test' && pwd",
	})
	if err != nil {
		fmt.Printf("   ❌ Exec failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Exec success: %v\n", result)
	}

	// Test 6: List available tools
	fmt.Println("\n6. Available tools:")
	for _, tool := range toolMgr.List() {
		fmt.Printf("   • %s\n", tool)
	}

	fmt.Println("\n🎉 All tests completed!")
	fmt.Printf("Workspace: %s\n", workspace)
}