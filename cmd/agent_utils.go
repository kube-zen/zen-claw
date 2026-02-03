package cmd

import (
	"fmt"
	"strings"

	"github.com/neves/zen-claw/internal/providers"
)

// inferProviderFromModel uses centralized provider detection
func inferProviderFromModel(modelName string) string {
	return providers.InferProviderFromModel(modelName)
}

// isModelCompatibleWithProvider checks if a model is likely compatible with a provider
func isModelCompatibleWithProvider(modelName, provider string) bool {
	modelName = strings.ToLower(modelName)
	provider = strings.ToLower(provider)

	switch provider {
	case "qwen":
		return strings.Contains(modelName, "qwen")
	case "deepseek":
		return strings.Contains(modelName, "deepseek")
	case "glm":
		return strings.Contains(modelName, "glm")
	case "minimax":
		return strings.Contains(modelName, "minimax") || strings.Contains(modelName, "abab")
	case "openai":
		return strings.Contains(modelName, "gpt")
	case "kimi":
		return strings.Contains(modelName, "kimi") || strings.Contains(modelName, "moonshot")
	}

	// Unknown provider, assume compatible
	return true
}

// displayProgressEvent prints a progress event to the console with minimal formatting
func displayProgressEvent(event ProgressEvent) {
	switch event.Type {
	case "start":
		// Skip - already shown in header
	case "step":
		// Show compact step indicator
		fmt.Printf("\n[%d] ", event.Step)
	case "thinking":
		// Skip - not useful to show
	case "ai_response":
		// Show brief AI intent (first line only, truncated)
		msg := event.Message
		if idx := strings.Index(msg, "\n"); idx > 0 {
			msg = msg[:idx]
		}
		if len(msg) > 80 {
			msg = msg[:77] + "..."
		}
		if msg != "" {
			fmt.Printf("%s\n", msg)
		}
	case "tool_call":
		// Show tool call compactly
		fmt.Printf("    %s\n", event.Message)
	case "tool_result":
		// Skip detailed results - tool_call already shows summary
	case "complete":
		fmt.Printf("\n✅ Done (%d steps)\n", event.Step)
	case "error":
		fmt.Printf("\n❌ %s\n", event.Message)
	case "done":
		// Final result will be displayed separately
	default:
		// Skip unknown events
	}
}
