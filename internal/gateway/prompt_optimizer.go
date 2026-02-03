package gateway

import (
	"github.com/neves/zen-claw/internal/ai"
)

// PromptOptimizer handles prompt structuring for large context windows
// Addresses the "Lost in the Middle" problem where models struggle with
// information placed in the middle of long prompts
type PromptOptimizer struct {
	// Threshold above which to apply optimization (tokens)
	OptimizationThreshold int
}

// NewPromptOptimizer creates a new prompt optimizer
func NewPromptOptimizer() *PromptOptimizer {
	return &PromptOptimizer{
		OptimizationThreshold: 50000, // Start optimizing at 50K tokens
	}
}

// OptimizeMessages restructures messages for better recall in large contexts
// Strategy: Place critical instructions at START and END, data in middle
//
// Pattern for large contexts:
// 1. [START] System prompt with task instructions
// 2. [MIDDLE] Context data (files, docs, history) - can be "lost"
// 3. [END] Reminder of task + user's actual question
func (o *PromptOptimizer) OptimizeMessages(messages []ai.Message) []ai.Message {
	tokenCount := EstimateTokens(messages)

	// Only optimize large contexts
	if tokenCount < o.OptimizationThreshold {
		return messages
	}

	// Find system message and last user message
	var systemIdx = -1
	var lastUserIdx = -1
	for i, msg := range messages {
		if msg.Role == "system" {
			systemIdx = i
		}
		if msg.Role == "user" {
			lastUserIdx = i
		}
	}

	// No optimization needed if structure is simple
	if systemIdx == -1 || lastUserIdx == -1 || len(messages) < 4 {
		return messages
	}

	// Build optimized message list
	optimized := make([]ai.Message, 0, len(messages)+1)

	// 1. Start with system message (contains instructions)
	if systemIdx >= 0 {
		optimized = append(optimized, messages[systemIdx])
	}

	// 2. Add reminder at start for very large contexts
	if tokenCount > 100000 {
		optimized = append(optimized, ai.Message{
			Role:    "user",
			Content: "[CONTEXT FOLLOWS - Focus on the FINAL USER MESSAGE for the actual task]",
		})
	}

	// 3. Middle: all messages except system and last user
	for i, msg := range messages {
		if i == systemIdx || i == lastUserIdx {
			continue
		}
		optimized = append(optimized, msg)
	}

	// 4. End: Reminder + last user message
	if tokenCount > 100000 {
		optimized = append(optimized, ai.Message{
			Role:    "user",
			Content: "[END OF CONTEXT - Please address the following request:]",
		})
	}

	// Last user message at the very end (most important position)
	optimized = append(optimized, messages[lastUserIdx])

	return optimized
}

// PrioritizeContent helps structure content for large documents
// Critical info should be at start/end, less important in middle
type ContentPriority int

const (
	PriorityHigh   ContentPriority = iota // Place at start or end
	PriorityMedium                        // Can be in middle
	PriorityLow                           // Middle is fine
)

// ContentBlock represents a prioritized content block
type ContentBlock struct {
	Content  string
	Priority ContentPriority
}

// StructureContent organizes content blocks for optimal recall
func StructureContent(blocks []ContentBlock) string {
	var high, medium, low []string

	for _, b := range blocks {
		switch b.Priority {
		case PriorityHigh:
			high = append(high, b.Content)
		case PriorityMedium:
			medium = append(medium, b.Content)
		case PriorityLow:
			low = append(low, b.Content)
		}
	}

	// Structure: HIGH (start) -> LOW -> MEDIUM -> HIGH (end)
	result := ""

	// First high-priority items at start
	if len(high) > 0 {
		result += high[0] + "\n\n"
	}

	// Low priority in the deep middle (most likely to be "lost")
	for _, content := range low {
		result += content + "\n\n"
	}

	// Medium priority (less likely to be lost than deep middle)
	for _, content := range medium {
		result += content + "\n\n"
	}

	// Remaining high-priority items at end
	for i := 1; i < len(high); i++ {
		result += high[i] + "\n\n"
	}

	return result
}
