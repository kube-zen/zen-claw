// Package providers contains AI provider implementations
package providers

import (
	"fmt"
	"strings"
	
	"github.com/neves/zen-claw/internal/ai"
)

// Qwen-specific enhancements for context window optimization
func (p *OpenAICompatibleProvider) IsQwenProvider() bool {
	return strings.ToLower(p.name) == "qwen"
}

// GetQwenModelInfo returns information about Qwen models and their capabilities
func (p *OpenAICompatibleProvider) GetQwenModelInfo(modelName string) map[string]interface{} {
	if !p.IsQwenProvider() {
		return nil
	}
	
	// Map of Qwen models to their characteristics
	modelInfo := map[string]map[string]interface{}{
		"qwen3-coder-30b": {
			"name":           "qwen3-coder-30b",
			"context_window": 262144, // 262K tokens
			"capabilities":   []string{"coding", "large_context", "reasoning"},
			"description":    "Qwen3 Coder 30B with 262K context window - perfect for large codebases",
		},
		"qwen3-coder-480b": {
			"name":           "qwen3-coder-480b",
			"context_window": 262144,
			"capabilities":   []string{"coding", "large_context", "reasoning", "complex_tasks"},
			"description":    "Qwen3 Coder 480B with 262K context window - highest performance for complex tasks",
		},
		"qwen-plus": {
			"name":           "qwen-plus",
			"context_window": 131072,
			"capabilities":   []string{"balanced", "multimodal", "reasoning"},
			"description":    "Qwen Plus with 131K context window - balanced performance",
		},
		"qwen-max": {
			"name":           "qwen-max",
			"context_window": 131072,
			"capabilities":   []string{"maximum", "multimodal", "reasoning", "complex_tasks"},
			"description":    "Qwen Max with 131K context window - maximum capabilities",
		},
	}
	
	// Return specific model info or general Qwen info
	if info, exists := modelInfo[strings.ToLower(modelName)]; exists {
		return info
	}
	
	// Return default Qwen info
	return map[string]interface{}{
		"name":           "qwen",
		"context_window": 262144,
		"capabilities":   []string{"coding", "large_context", "reasoning"},
		"description":    "Qwen AI model with large context window",
	}
}

// OptimizeForQwenContext optimizes message handling for Qwen's large context window
func (p *OpenAICompatibleProvider) OptimizeForQwenContext(messages []ai.Message, maxTokens int) []ai.Message {
	if !p.IsQwenProvider() {
		return messages
	}
	
	// For Qwen, we can be more aggressive with context utilization
	// This is a placeholder for more sophisticated context management
	fmt.Printf("🔍 Optimizing for Qwen's large context window (%d tokens)\n", 262144)
	
	return messages
}

// ValidateQwenModel validates Qwen-specific model requirements
func (p *OpenAICompatibleProvider) ValidateQwenModel(modelName string) error {
	if !p.IsQwenProvider() {
		return nil
	}
	
	// Check if model is a valid Qwen model
	validModels := []string{
		"qwen3-coder-30b",
		"qwen3-coder-480b",
		"qwen-plus",
		"qwen-max",
		"qwen3-235b-a22b-instruct-2507",
		"qwen3-coder-480b-a35b-instruct",
		"qwen3-coder-30b-a3b-instruct",
	}
	
	modelName = strings.ToLower(modelName)
	for _, validModel := range validModels {
		if modelName == validModel {
			return nil
		}
	}
	
	return fmt.Errorf("invalid Qwen model: %s. Valid models: %v", modelName, validModels)
}
