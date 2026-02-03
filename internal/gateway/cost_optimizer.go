package gateway

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/neves/zen-claw/internal/ai"
)

// CostOptimizer reduces token usage without losing functionality
type CostOptimizer struct {
	// Max tokens for tool results before truncation
	MaxToolResultTokens int
	// Max conversation history messages before summarization
	MaxHistoryMessages int
	// Max tokens for a single message before compression
	MaxMessageTokens int
}

// NewCostOptimizer creates a cost optimizer with sensible defaults
func NewCostOptimizer() *CostOptimizer {
	return &CostOptimizer{
		MaxToolResultTokens: 8000,  // ~32KB of tool output
		MaxHistoryMessages:  20,    // Keep last 20 messages, summarize rest
		MaxMessageTokens:    16000, // Compress messages over this
	}
}

// OptimizeRequest applies all cost optimizations to a request
func (o *CostOptimizer) OptimizeRequest(req ai.ChatRequest) ai.ChatRequest {
	optimized := req

	// 1. Compress prompts (whitespace, redundancy)
	for i := range optimized.Messages {
		optimized.Messages[i].Content = o.CompressPrompt(optimized.Messages[i].Content)
	}

	// 2. Truncate long tool results
	optimized.Messages = o.TruncateToolResults(optimized.Messages)

	// 3. Summarize old history if too long
	if len(optimized.Messages) > o.MaxHistoryMessages {
		optimized.Messages = o.SummarizeHistory(optimized.Messages)
	}

	// 4. Set smart output limits based on task
	if optimized.MaxTokens == 0 {
		optimized.MaxTokens = o.EstimateOutputTokens(optimized.Messages)
	}

	return optimized
}

// CompressPrompt reduces token count without losing meaning
func (o *CostOptimizer) CompressPrompt(content string) string {
	if content == "" {
		return content
	}

	// 1. Normalize whitespace (multiple spaces/newlines → single)
	content = normalizeWhitespace(content)

	// 2. Remove redundant markdown formatting
	content = simplifyMarkdown(content)

	// 3. Compress common verbose patterns
	content = compressVerbosePatterns(content)

	return strings.TrimSpace(content)
}

// TruncateToolResults limits tool output size
func (o *CostOptimizer) TruncateToolResults(messages []ai.Message) []ai.Message {
	result := make([]ai.Message, len(messages))
	copy(result, messages)

	for i, msg := range result {
		// Tool results are typically in assistant messages with specific patterns
		if msg.Role == "tool" || (msg.Role == "assistant" && isLikelyToolResult(msg.Content)) {
			if estimateTokens(msg.Content) > o.MaxToolResultTokens {
				result[i].Content = truncateWithContext(msg.Content, o.MaxToolResultTokens)
			}
		}
	}

	return result
}

// SummarizeHistory keeps recent messages, summarizes old ones
func (o *CostOptimizer) SummarizeHistory(messages []ai.Message) []ai.Message {
	if len(messages) <= o.MaxHistoryMessages {
		return messages
	}

	// Keep system message (if any) + last N messages
	keepCount := o.MaxHistoryMessages - 1 // Reserve 1 for summary

	var systemMsg *ai.Message
	var history []ai.Message

	for i, msg := range messages {
		if msg.Role == "system" && systemMsg == nil {
			systemMsg = &messages[i]
		} else {
			history = append(history, msg)
		}
	}

	// Split into old (to summarize) and recent (to keep)
	splitPoint := len(history) - keepCount
	if splitPoint <= 0 {
		return messages
	}

	oldMessages := history[:splitPoint]
	recentMessages := history[splitPoint:]

	// Create summary of old messages
	summary := createHistorySummary(oldMessages)

	// Build result: system + summary + recent
	result := []ai.Message{}
	if systemMsg != nil {
		result = append(result, *systemMsg)
	}
	result = append(result, ai.Message{
		Role:    "system",
		Content: "[Previous conversation summary]\n" + summary,
	})
	result = append(result, recentMessages...)

	return result
}

// EstimateOutputTokens suggests max_tokens based on task type
func (o *CostOptimizer) EstimateOutputTokens(messages []ai.Message) int {
	if len(messages) == 0 {
		return 4096
	}

	// Look at last user message to understand task
	var lastUserMsg string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserMsg = strings.ToLower(messages[i].Content)
			break
		}
	}

	// Short answer tasks
	shortPatterns := []string{
		"what is", "how many", "which", "when", "where", "who",
		"yes or no", "true or false", "list", "name",
		"summarize", "tldr", "briefly",
	}
	for _, p := range shortPatterns {
		if strings.Contains(lastUserMsg, p) {
			return 1024 // Short response sufficient
		}
	}

	// Code generation tasks need more tokens
	codePatterns := []string{
		"write", "create", "implement", "generate", "build",
		"refactor", "add", "modify", "fix", "update",
	}
	for _, p := range codePatterns {
		if strings.Contains(lastUserMsg, p) {
			return 8192 // More for code
		}
	}

	// Default medium
	return 4096
}

// === Helper functions ===

func normalizeWhitespace(s string) string {
	// Replace multiple spaces with single space
	spaceRe := regexp.MustCompile(`[ \t]+`)
	s = spaceRe.ReplaceAllString(s, " ")

	// Replace 3+ newlines with 2
	newlineRe := regexp.MustCompile(`\n{3,}`)
	s = newlineRe.ReplaceAllString(s, "\n\n")

	// Trim each line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRightFunc(line, unicode.IsSpace)
	}
	return strings.Join(lines, "\n")
}

func simplifyMarkdown(s string) string {
	// Remove excessive emphasis (***bold*** → **bold**)
	s = regexp.MustCompile(`\*{3,}`).ReplaceAllString(s, "**")

	// Simplify horizontal rules
	s = regexp.MustCompile(`[-=]{4,}`).ReplaceAllString(s, "---")

	return s
}

func compressVerbosePatterns(s string) string {
	// Common verbose phrases that can be shortened
	replacements := map[string]string{
		"in order to":             "to",
		"due to the fact that":    "because",
		"at this point in time":   "now",
		"in the event that":       "if",
		"for the purpose of":      "for",
		"with regard to":          "regarding",
		"in accordance with":      "per",
		"in the process of":       "while",
		"on a daily basis":        "daily",
		"at the present time":     "now",
		"in the near future":      "soon",
		"a large number of":       "many",
		"a small number of":       "few",
		"the vast majority of":    "most",
		"in spite of the fact":    "although",
		"it is important to note": "note:",
		"please note that":        "note:",
	}

	result := s
	for verbose, concise := range replacements {
		result = strings.ReplaceAll(strings.ToLower(result), verbose, concise)
	}

	return result
}

func isLikelyToolResult(content string) bool {
	// Tool results often contain file paths, JSON, or structured output
	indicators := []string{
		"```", "{\n", "[\n", "/home/", "/usr/",
		"Error:", "Success:", "Output:",
	}
	for _, ind := range indicators {
		if strings.Contains(content, ind) {
			return true
		}
	}
	return false
}

func truncateWithContext(content string, maxTokens int) string {
	estimated := estimateTokens(content)
	if estimated <= maxTokens {
		return content
	}

	// Keep start and end, truncate middle
	lines := strings.Split(content, "\n")
	if len(lines) < 10 {
		// For short content, just truncate at end
		chars := maxTokens * 4 // ~4 chars per token
		if len(content) <= chars {
			return content
		}
		return content[:chars] + "\n... [truncated, " + string(rune(estimated-maxTokens)) + " tokens removed]"
	}

	// Keep first 1/3 and last 1/3 of lines
	keepLines := (maxTokens * 4) / 80 // Assume ~80 chars per line
	if keepLines < 6 {
		keepLines = 6
	}
	headLines := keepLines / 2
	tailLines := keepLines - headLines

	head := strings.Join(lines[:headLines], "\n")
	tail := strings.Join(lines[len(lines)-tailLines:], "\n")
	removed := len(lines) - headLines - tailLines

	return head + "\n\n... [" + string(rune(removed)) + " lines truncated] ...\n\n" + tail
}

func createHistorySummary(messages []ai.Message) string {
	// Create a brief summary of conversation turns
	var summary strings.Builder
	summary.WriteString("Conversation covered:\n")

	for _, msg := range messages {
		if msg.Role == "user" {
			// Extract key topic from user message
			topic := extractTopic(msg.Content)
			if topic != "" {
				summary.WriteString("- User asked about: " + topic + "\n")
			}
		}
	}

	return summary.String()
}

func extractTopic(content string) string {
	// Get first meaningful line/sentence as topic
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	// Take first line or first 100 chars
	lines := strings.SplitN(content, "\n", 2)
	topic := lines[0]

	if len(topic) > 100 {
		topic = topic[:100] + "..."
	}

	return topic
}

func estimateTokens(s string) int {
	return len(s) / 4 // Rough estimate: 4 chars per token
}
