package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newAgentGatewayCmd() *cobra.Command {
	var gatewayURL string
	var sessionID string
	var workingDir string
	var provider string
	var model string
	var maxSteps int
	
	cmd := &cobra.Command{
		Use:   "agent-gateway",
		Short: "Connect to Zen Claw gateway (no direct AI calls)",
		Long: `Connect to Zen Claw gateway instead of making direct AI calls.

This client connects to the gateway running on :8080 and uses the
gateway's agent service with multi-provider support.

The gateway provides:
- Multiple AI providers (DeepSeek, GLM, Minimax, Qwen)
- Session management
- Tool execution via gateway
- No API keys needed on client side

Examples:
  # Start gateway first: zen-claw gateway start
  # Then use gateway client:
  zen-claw agent-gateway "list directory"
  zen-claw agent-gateway --session-id my-task "analyze project"`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runAgentGateway(args[0], gatewayURL, sessionID, workingDir, provider, model, maxSteps)
		},
	}
	
	cmd.Flags().StringVar(&gatewayURL, "gateway-url", "http://localhost:8080", "Gateway URL")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (for continuing sessions)")
	cmd.Flags().StringVar(&workingDir, "working-dir", ".", "Working directory for tools")
	cmd.Flags().StringVar(&provider, "provider", "", "AI provider (deepseek, glm, minimax, etc.)")
	cmd.Flags().StringVar(&model, "model", "", "AI model")
	cmd.Flags().IntVar(&maxSteps, "max-steps", 10, "Maximum tool execution steps")
	
	return cmd
}

func runAgentGateway(task, gatewayURL, sessionID, workingDir, provider, model string, maxSteps int) {
	fmt.Println("🚀 Zen Claw Gateway Client")
	fmt.Println("═" + strings.Repeat("═", 78))
	fmt.Printf("Task: %s\n", task)
	fmt.Printf("Gateway: %s\n", gatewayURL)
	if sessionID != "" {
		fmt.Printf("Session ID: %s\n", sessionID)
	}
	fmt.Printf("Working directory: %s\n", workingDir)
	
	// Check if gateway is reachable
	if !checkGatewayHealth(gatewayURL) {
		fmt.Printf("\n❌ Gateway not reachable at %s\n", gatewayURL)
		fmt.Println("   Start the gateway first: zen-claw gateway start")
		os.Exit(1)
	}
	
	// Create request
	req := map[string]interface{}{
		"session_id":  sessionID,
		"user_input":  task,
		"working_dir": workingDir,
		"max_steps":   maxSteps,
	}
	
	if provider != "" {
		req["provider"] = provider
	}
	if model != "" {
		req["model"] = model
	}
	
	// Send request to gateway
	startTime := time.Now()
	resp, err := sendToGateway(gatewayURL+"/chat", req)
	duration := time.Since(startTime)
	
	if err != nil {
		fmt.Printf("\n❌ Gateway error: %v\n", err)
		os.Exit(1)
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Printf("\n❌ Failed to parse gateway response: %v\n", err)
		os.Exit(1)
	}
	
	// Check for error in response
	if errorMsg, ok := result["error"].(string); ok && errorMsg != "" {
		fmt.Printf("\n❌ Agent error: %s\n", errorMsg)
		os.Exit(1)
	}
	
	// Get result
	finalResult, _ := result["result"].(string)
	respSessionID, _ := result["session_id"].(string)
	
	// Print result
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("🎯 RESULT (via Gateway)")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println(finalResult)
	fmt.Println(strings.Repeat("═", 80))
	
	// Print session info
	if sessionInfo, ok := result["session_info"].(map[string]interface{}); ok {
		fmt.Printf("\n📊 Session Information:\n")
		fmt.Printf("   Session ID: %s\n", respSessionID)
		fmt.Printf("   Duration: %v\n", duration.Round(time.Millisecond))
		
		if msgCount, ok := sessionInfo["message_count"].(float64); ok {
			fmt.Printf("   Messages: %.0f total\n", msgCount)
		}
		if userMsgs, ok := sessionInfo["user_messages"].(float64); ok {
			fmt.Printf("     - User: %.0f\n", userMsgs)
		}
		if assistantMsgs, ok := sessionInfo["assistant_messages"].(float64); ok {
			fmt.Printf("     - Assistant: %.0f\n", assistantMsgs)
		}
		if toolMsgs, ok := sessionInfo["tool_messages"].(float64); ok {
			fmt.Printf("     - Tool: %.0f\n", toolMsgs)
		}
		if wd, ok := sessionInfo["working_dir"].(string); ok {
			fmt.Printf("   Working directory: %s\n", wd)
		}
	}
	
	fmt.Printf("\n💡 To continue this session:\n")
	fmt.Printf("   zen-claw agent-gateway --session-id %s \"your next task\"\n", respSessionID)
}

func checkGatewayHealth(gatewayURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(gatewayURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func sendToGateway(url string, data interface{}) ([]byte, error) {
	// Convert to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Send request
	client := &http.Client{Timeout: 120 * time.Second} // Long timeout for agent tasks
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to gateway: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway returned error %d: %s", resp.StatusCode, string(body))
	}
	
	return body, nil
}