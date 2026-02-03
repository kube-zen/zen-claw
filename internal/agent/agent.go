package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/ai"
)

// AICaller interface for making AI calls
// This abstracts away whether we call AI directly or through gateway
type AICaller interface {
	Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error)
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolCallID string
	Content    string
	IsError    bool
}

// ProgressCallback is called during agent execution to report progress
type ProgressCallback func(event ProgressEvent)

// ProgressEvent represents a progress event during agent execution
type ProgressEvent struct {
	Type    string      `json:"type"`    // "step", "thinking", "tool_call", "tool_result", "complete", "error"
	Step    int         `json:"step"`    // Current step number
	Message string      `json:"message"` // Human-readable message
	Data    interface{} `json:"data,omitempty"`
}

// Agent is a minimal agent focused only on tool execution
// Uses AICaller interface for AI communication
type Agent struct {
	aiCaller         AICaller
	tools            map[string]Tool
	maxSteps         int
	currentModel     string
	progressCallback ProgressCallback
}

// AgentEvent represents a progress event during agent execution (deprecated, use ProgressEvent)
type AgentEvent struct {
	Type    string      `json:"type"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Step    int         `json:"step,omitempty"`
}

// NewAgent creates a new agent
func NewAgent(aiCaller AICaller, tools []Tool, maxSteps int) *Agent {
	// Convert tools to map
	toolMap := make(map[string]Tool)
	for _, tool := range tools {
		toolMap[tool.Name()] = tool
	}

	return &Agent{
		aiCaller:     aiCaller,
		tools:        toolMap,
		maxSteps:     maxSteps,
		currentModel: "deepseek-chat", // Default model
	}
}

// SetProgressCallback sets a callback for progress events
func (a *Agent) SetProgressCallback(cb ProgressCallback) {
	a.progressCallback = cb
}

// emitProgress sends a progress event if callback is set
func (a *Agent) emitProgress(eventType string, step int, message string, data interface{}) {
	if a.progressCallback != nil {
		a.progressCallback(ProgressEvent{
			Type:    eventType,
			Step:    step,
			Message: message,
			Data:    data,
		})
	}
}

// Run executes a task with the given session
// Returns updated session and final result
func (a *Agent) Run(ctx context.Context, session *Session, userInput string) (*Session, string, error) {
	log.Printf("[Agent] Running: %s", userInput)

	// Handle model switching commands
	if userInput == "/models" {
		availableModels := []string{
			"deepseek-chat",
			"qwen3-coder-30b-a3b-instruct",
			"qwen-plus",
			"qwen-max",
			"gpt-4o",
			"gpt-4-turbo",
			"glm-4",
			"glm-3-turbo",
			"abab6.5s",
			"abab6.5",
		}

		var sb strings.Builder
		sb.WriteString("Available models:\n")
		for _, model := range availableModels {
			if model == a.currentModel {
				sb.WriteString(fmt.Sprintf("  ✓ %s (current)\n", model))
			} else {
				sb.WriteString(fmt.Sprintf("  ○ %s\n", model))
			}
		}
		sb.WriteString("\nUse '/model <model-name>' to switch models")
		return session, sb.String(), nil
	}

	if strings.HasPrefix(userInput, "/model ") {
		model := strings.TrimSpace(strings.TrimPrefix(userInput, "/model "))
		a.currentModel = model
		return session, fmt.Sprintf("Model switched to: %s", model), nil
	}

	// Handle context limit command: /context-limit <number> or /context-limit (show current)
	if strings.HasPrefix(userInput, "/context-limit") {
		parts := strings.Fields(userInput)
		if len(parts) == 1 {
			// Show current limit
			limit := session.GetContextLimit()
			if limit == 0 {
				return session, "Context limit: unlimited (0)", nil
			}
			return session, fmt.Sprintf("Context limit: %d messages", limit), nil
		} else if len(parts) == 2 {
			// Set limit
			var limit int
			if _, err := fmt.Sscanf(parts[1], "%d", &limit); err != nil {
				return session, fmt.Sprintf("Invalid context limit: %s. Use a number or 0 for unlimited.", parts[1]), nil
			}
			if limit < 0 {
				return session, "Context limit must be >= 0 (0 = unlimited)", nil
			}
			session.SetContextLimit(limit)
			if limit == 0 {
				return session, "Context limit set to: unlimited (0)", nil
			}
			return session, fmt.Sprintf("Context limit set to: %d messages", limit), nil
		} else {
			return session, "Usage: /context-limit [number] (0 = unlimited, default 50)", nil
		}
	}

	// Handle Qwen large context command: /qwen-large-context on|off|status
	if strings.HasPrefix(userInput, "/qwen-large-context") {
		parts := strings.Fields(userInput)
		if len(parts) == 1 {
			// Show current status
			enabled := session.GetQwenLargeContextEnabled()
			status := "disabled"
			if enabled {
				status = "enabled (256k context window)"
			} else {
				status = "disabled (using small window to avoid crashes)"
			}
			return session, fmt.Sprintf("Qwen large context: %s", status), nil
		} else if len(parts) == 2 {
			// Set status
			arg := strings.ToLower(parts[1])
			switch arg {
			case "on", "enable", "true", "1":
				session.SetQwenLargeContextEnabled(true)
				return session, "Qwen large context enabled (256k context window). Warning: may cause crashes with some Qwen models.", nil
			case "off", "disable", "disabled", "false", "0":
				session.SetQwenLargeContextEnabled(false)
				return session, "Qwen large context disabled (using small window to avoid crashes)", nil
			case "status":
				enabled := session.GetQwenLargeContextEnabled()
				status := "disabled"
				if enabled {
					status = "enabled (256k context window)"
				} else {
					status = "disabled (using small window to avoid crashes)"
				}
				return session, fmt.Sprintf("Qwen large context: %s", status), nil
			default:
				return session, "Usage: /qwen-large-context [on|off|status]", nil
			}
		} else {
			return session, "Usage: /qwen-large-context [on|off|status]", nil
		}
	}

	// Add user message to session
	session.AddMessage(ai.Message{
		Role:    "user",
		Content: userInput,
	})

	// Execute agent loop
	for step := 0; step < a.maxSteps; step++ {
		stepNum := step + 1
		log.Printf("[Agent] Step %d", stepNum)
		a.emitProgress("step", stepNum, fmt.Sprintf("Step %d/%d: Thinking...", stepNum, a.maxSteps), nil)

		// Get AI response
		a.emitProgress("thinking", stepNum, "Waiting for AI response...", nil)
		resp, err := a.getAIResponse(ctx, session)
		if err != nil {
			a.emitProgress("error", stepNum, fmt.Sprintf("AI error: %v", err), nil)
			return session, "", fmt.Errorf("AI response failed: %w", err)
		}

		// Parse tool calls from response content (text-based tool calling)
		toolCalls := a.parseToolCallsFromText(resp.Content)

		// If no tool calls, we're done
		if len(toolCalls) == 0 && len(resp.ToolCalls) == 0 {
			session.AddMessage(ai.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
			a.emitProgress("complete", stepNum, "Task completed", map[string]interface{}{
				"total_steps": stepNum,
			})
			return session, resp.Content, nil
		}

		// Combine structured tool calls with parsed text tool calls
		allToolCalls := resp.ToolCalls
		if len(toolCalls) > 0 {
			allToolCalls = append(allToolCalls, toolCalls...)
		}

		// Clean content - remove XML tool call tags for cleaner display
		cleanedContent := a.cleanToolCallTags(resp.Content)

		// Emit AI response if there's content
		if cleanedContent != "" {
			a.emitProgress("ai_response", stepNum, cleanedContent, nil)
		}

		log.Printf("[Agent] Executing %d tool calls (%d from text parsing)", len(allToolCalls), len(toolCalls))

		// Execute all tool calls with progress
		toolResults, err := a.executeToolCallsWithProgress(ctx, allToolCalls, stepNum)
		if err != nil {
			a.emitProgress("error", stepNum, fmt.Sprintf("Tool error: %v", err), nil)
			return session, "", fmt.Errorf("tool execution failed: %w", err)
		}

		// Add assistant message with tool calls
		session.AddMessage(ai.Message{
			Role:      "assistant",
			Content:   cleanedContent,
			ToolCalls: allToolCalls,
		})

		// Add tool results to session
		for _, result := range toolResults {
			session.AddMessage(ai.Message{
				Role:       "tool",
				Content:    result.Content,
				ToolCallID: result.ToolCallID,
			})

			// Check if this was a cd command that changed working directory
			// The ExecTool returns new_working_dir in the result for cd commands
			if result.Content != "" && !result.IsError {
				// Parse JSON result
				var toolResult map[string]interface{}
				if err := json.Unmarshal([]byte(result.Content), &toolResult); err == nil {
					// Check for new_working_dir field
					if newDir, ok := toolResult["new_working_dir"].(string); ok && newDir != "" {
						session.SetWorkingDir(newDir)
						log.Printf("[Agent] Updated session working directory to: %s", newDir)
					}
				}
			}
		}

		log.Printf("[Agent] Added %d tool results, continuing...", len(toolResults))

		// Check if we should stop early (e.g., task completed)
		if a.shouldStopEarly(cleanedContent, toolResults) {
			log.Printf("[Agent] Early stop condition met at step %d", step+1)
			// Get final response
			finalResp, err := a.getAIResponse(ctx, session)
			if err != nil {
				return session, "", fmt.Errorf("final AI response failed: %w", err)
			}
			finalCleaned := a.cleanToolCallTags(finalResp.Content)
			session.AddMessage(ai.Message{
				Role:    "assistant",
				Content: finalCleaned,
			})
			return session, finalCleaned, nil
		}
	}

	return session, "", fmt.Errorf("exceeded maximum steps (%d). For complex tasks: 1) Try --max-steps 200 for large refactoring, 2) Break task into smaller pieces, 3) Use /context-limit to reduce context if AI is exploring too much", a.maxSteps)
}

// getAIResponse gets a response from the AI caller with session messages
func (a *Agent) getAIResponse(ctx context.Context, session *Session) (*ai.ChatResponse, error) {
	// Get all messages - use full context window for large context models
	// Models like Qwen 3 Coder (262K), Gemini 3 Flash (1M) can handle long conversations
	messages := session.GetMessages()

	// Convert tools to AI tool definitions
	toolDefs := a.getToolDefinitions()

	// Create chat request with generous max tokens for complex responses
	req := ai.ChatRequest{
		Model:                   a.currentModel, // Use current model
		Messages:                messages,
		Tools:                   toolDefs,
		Temperature:             0.7,
		MaxTokens:               4000, // Increased for complex analysis tasks
		ContextLimit:            session.GetContextLimit(),
		QwenLargeContextEnabled: session.GetQwenLargeContextEnabled(),
	}

	// Per-step timeout: Each AI call gets its own generous timeout
	// This is per-step, not per-task, so large tasks with many steps work fine.
	// Similar to Cursor's approach where each step has time to complete.
	// 5 minutes per step is generous even for large context models.
	stepCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	resp, err := a.aiCaller.Chat(stepCtx, req)
	if err != nil {
		// Check if it's a context deadline exceeded error
		if ctx.Err() == context.DeadlineExceeded || stepCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("AI response timed out after 5 minutes (this step). Large models with long context may need more time. Try reducing context with /context-limit command")
		}
		return nil, err
	}
	return resp, nil
}

// executeToolCalls executes all tool calls and returns results (without progress)
func (a *Agent) executeToolCalls(ctx context.Context, toolCalls []ai.ToolCall) ([]ToolResult, error) {
	return a.executeToolCallsWithProgress(ctx, toolCalls, 0)
}

// isReadOnlyTool returns true if the tool is safe for parallel execution
func isReadOnlyTool(name string) bool {
	readOnly := map[string]bool{
		"read_file":    true,
		"list_dir":     true,
		"search_files": true,
		"system_info":  true,
	}
	return readOnly[name]
}

// executeToolCallsWithProgress executes tool calls with parallel execution for read-only tools
func (a *Agent) executeToolCallsWithProgress(ctx context.Context, toolCalls []ai.ToolCall, step int) ([]ToolResult, error) {
	if len(toolCalls) == 0 {
		return nil, nil
	}

	// Separate into parallel-safe (read-only) and sequential (write) tools
	var parallelCalls []ai.ToolCall
	var sequentialCalls []ai.ToolCall

	for _, call := range toolCalls {
		if isReadOnlyTool(call.Name) {
			parallelCalls = append(parallelCalls, call)
		} else {
			sequentialCalls = append(sequentialCalls, call)
		}
	}

	results := make([]ToolResult, len(toolCalls))
	resultIndex := make(map[string]int) // Map tool call ID to result index
	for i, call := range toolCalls {
		resultIndex[call.ID] = i
	}

	// Execute read-only tools in parallel
	if len(parallelCalls) > 0 {
		log.Printf("[Agent] Executing %d read-only tools in parallel", len(parallelCalls))
		a.emitProgress("tool_call", step, fmt.Sprintf("⚡ Running %d tools in parallel...", len(parallelCalls)), nil)

		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, call := range parallelCalls {
			wg.Add(1)
			go func(call ai.ToolCall) {
				defer wg.Done()

				result := a.executeSingleTool(ctx, call, step)

				mu.Lock()
				results[resultIndex[call.ID]] = result
				mu.Unlock()
			}(call)
		}

		wg.Wait()
		log.Printf("[Agent] Parallel execution complete")
	}

	// Execute write tools sequentially
	for _, call := range sequentialCalls {
		result := a.executeSingleTool(ctx, call, step)
		results[resultIndex[call.ID]] = result
	}

	return results, nil
}

// executeSingleTool executes a single tool call and returns the result
func (a *Agent) executeSingleTool(ctx context.Context, call ai.ToolCall, step int) ToolResult {
	log.Printf("[Agent] Executing tool: %s", call.Name)

	// Build argument summary for display
	argSummary := a.summarizeArgs(call.Args)
	a.emitProgress("tool_call", step, fmt.Sprintf("🔧 %s(%s)", call.Name, argSummary), map[string]interface{}{
		"tool": call.Name,
		"args": call.Args,
	})

	tool, exists := a.tools[call.Name]
	if !exists {
		errMsg := fmt.Sprintf("Tool '%s' not found", call.Name)
		a.emitProgress("tool_result", step, fmt.Sprintf("❌ %s", errMsg), nil)
		return ToolResult{
			ToolCallID: call.ID,
			Content:    fmt.Sprintf("Error: %s", errMsg),
			IsError:    true,
		}
	}

	// Execute tool
	result, err := tool.Execute(ctx, call.Args)
	if err != nil {
		errMsg := fmt.Sprintf("Error executing %s: %v", call.Name, err)
		a.emitProgress("tool_result", step, fmt.Sprintf("❌ %s", errMsg), nil)
		errorResult := map[string]interface{}{
			"error": errMsg,
		}
		errorJSON, _ := json.Marshal(errorResult)
		return ToolResult{
			ToolCallID: call.ID,
			Content:    string(errorJSON),
			IsError:    true,
		}
	}

	// Convert result to JSON string to preserve structure
	resultJSON, err := json.Marshal(result)
	if err != nil {
		// Fallback to string representation
		resultStr := fmt.Sprintf("%v", result)
		return ToolResult{
			ToolCallID: call.ID,
			Content:    resultStr,
			IsError:    false,
		}
	}

	// Emit success with truncated result
	resultSummary := a.summarizeResult(string(resultJSON))
	a.emitProgress("tool_result", step, fmt.Sprintf("✓ %s → %s", call.Name, resultSummary), nil)

	log.Printf("[Agent] Tool %s completed", call.Name)

	return ToolResult{
		ToolCallID: call.ID,
		Content:    string(resultJSON),
		IsError:    false,
	}
}

// summarizeArgs creates a short summary of tool arguments for display
func (a *Agent) summarizeArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}

	var parts []string
	for k, v := range args {
		str := fmt.Sprintf("%v", v)
		if len(str) > 40 {
			str = str[:37] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s=%q", k, str))
	}

	result := strings.Join(parts, ", ")
	if len(result) > 80 {
		result = result[:77] + "..."
	}
	return result
}

// summarizeResult creates a short summary of tool result for display
func (a *Agent) summarizeResult(result string) string {
	// Try to extract meaningful info from JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err == nil {
		// For exec results, show exit code and truncated output
		if exitCode, ok := data["exit_code"]; ok {
			if output, ok := data["output"].(string); ok {
				lines := strings.Split(strings.TrimSpace(output), "\n")
				if len(lines) > 0 {
					firstLine := lines[0]
					if len(firstLine) > 50 {
						firstLine = firstLine[:47] + "..."
					}
					return fmt.Sprintf("exit=%v: %s", exitCode, firstLine)
				}
				return fmt.Sprintf("exit=%v", exitCode)
			}
			return fmt.Sprintf("exit=%v", exitCode)
		}
		// For file reads, show file size or content preview
		if content, ok := data["content"].(string); ok {
			lines := len(strings.Split(content, "\n"))
			return fmt.Sprintf("%d lines", lines)
		}
		// For directory listing, show count
		if entries, ok := data["entries"].([]interface{}); ok {
			return fmt.Sprintf("%d items", len(entries))
		}
	}

	// Fallback: truncate raw result
	if len(result) > 60 {
		return result[:57] + "..."
	}
	return result
}

// getToolDefinitions converts tools to AI tool definitions
func (a *Agent) getToolDefinitions() []ai.Tool {
	var defs []ai.Tool

	for _, tool := range a.tools {
		defs = append(defs, ai.Tool{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		})
	}

	return defs
}

// cleanToolCallTags removes XML tool call tags from content for cleaner display
func (a *Agent) cleanToolCallTags(content string) string {
	// Remove XML-like tool call tags
	lines := strings.Split(content, "\n")
	var cleanedLines []string

	inToolCall := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip tool call tags
		if strings.HasPrefix(trimmed, "<function=") && strings.HasSuffix(trimmed, ">") {
			inToolCall = true
			continue
		}
		if trimmed == "</function>" || trimmed == "</tool_call>" {
			inToolCall = false
			continue
		}
		if strings.HasPrefix(trimmed, "<parameter=") && strings.Contains(trimmed, ">") {
			continue
		}

		// Keep non-tool-call lines
		if !inToolCall {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, "\n")
}

// parseToolCallsFromText parses text-based tool calls like <function=name>...</function>
func (a *Agent) parseToolCallsFromText(content string) []ai.ToolCall {
	var toolCalls []ai.ToolCall

	// Look for patterns like <function=name>...</function>
	// Example: <function=list_dir><parameter=path>~/git</parameter></function>

	// Simple regex-based parsing (could be more sophisticated)
	lines := strings.Split(content, "\n")
	inToolCall := false
	var currentToolCall *ai.ToolCall
	var currentArgs map[string]interface{}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for function start
		if strings.HasPrefix(line, "<function=") && strings.HasSuffix(line, ">") {
			inToolCall = true
			// Extract function name
			funcName := strings.TrimPrefix(line, "<function=")
			funcName = strings.TrimSuffix(funcName, ">")

			currentToolCall = &ai.ToolCall{
				ID:   fmt.Sprintf("call_%d", len(toolCalls)+1),
				Name: funcName,
				Args: make(map[string]interface{}),
			}
			currentArgs = currentToolCall.Args
			continue
		}

		// Check for parameter
		if inToolCall && strings.HasPrefix(line, "<parameter=") && strings.Contains(line, ">") {
			// Extract parameter name and value
			paramLine := strings.TrimPrefix(line, "<parameter=")
			parts := strings.SplitN(paramLine, ">", 2)
			if len(parts) == 2 {
				paramName := strings.TrimSuffix(parts[0], "</parameter")
				paramValue := parts[1]
				paramValue = strings.TrimSuffix(paramValue, "</parameter>")
				currentArgs[paramName] = paramValue
			}
			continue
		}

		// Check for function end
		if inToolCall && line == "</function>" {
			inToolCall = false
			if currentToolCall != nil {
				toolCalls = append(toolCalls, *currentToolCall)
			}
			currentToolCall = nil
			currentArgs = nil
			continue
		}

		// Check for </tool_call> (alternative format)
		if inToolCall && line == "</tool_call>" {
			inToolCall = false
			if currentToolCall != nil {
				toolCalls = append(toolCalls, *currentToolCall)
			}
			currentToolCall = nil
			currentArgs = nil
			continue
		}
	}

	// If we're still in a tool call at the end, add it
	if inToolCall && currentToolCall != nil {
		toolCalls = append(toolCalls, *currentToolCall)
	}

	log.Printf("[Agent] Parsed %d tool calls from text", len(toolCalls))
	return toolCalls
}

// shouldStopEarly determines if we should stop execution early
func (a *Agent) shouldStopEarly(lastAssistantMessage string, toolResults []ToolResult) bool {
	// Check if assistant message indicates completion
	lowerMsg := strings.ToLower(lastAssistantMessage)
	completionIndicators := []string{
		"completed",
		"finished",
		"done",
		"conclusion",
		"summary",
		"final",
		"result:",
		"answer:",
		"recommendation:",
		"analysis complete",
		"task complete",
	}

	for _, indicator := range completionIndicators {
		if strings.Contains(lowerMsg, indicator) {
			return true
		}
	}

	// Check if tool results indicate we have enough information
	if len(toolResults) > 0 {
		// If we got file contents or directory listings, we might have enough
		for _, result := range toolResults {
			if !result.IsError && len(result.Content) > 100 {
				// Got substantial non-error result
				return false // Continue to process
			}
		}
	}

	return false
}
