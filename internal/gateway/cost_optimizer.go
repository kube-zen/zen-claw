package gateway

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
)

// ToolPruneRule defines how to prune a specific tool's output
type ToolPruneRule struct {
	MaxTokens  int  // Max tokens before truncation (0 = use default)
	KeepRecent int  // Keep last N results untruncated (0 = truncate all)
	Aggressive bool // Use aggressive truncation (head only, no tail)
}

// CostOptimizer reduces token usage without losing functionality
type CostOptimizer struct {
	// Max tokens for tool results before truncation
	MaxToolResultTokens int
	// Max conversation history messages before summarization
	MaxHistoryMessages int
	// Hard limit on total messages (drop oldest beyond this)
	MaxHistoryTurns int
	// Max tokens for a single message before compression
	MaxMessageTokens int
	// Tool-specific pruning rules
	ToolRules map[string]ToolPruneRule
	// Keep last N assistant messages unmodified
	KeepLastAssistants int
}

// NewCostOptimizer creates a cost optimizer with sensible defaults
func NewCostOptimizer() *CostOptimizer {
	return NewCostOptimizerWithConfig(nil)
}

// NewCostOptimizerWithConfig creates a cost optimizer from config
func NewCostOptimizerWithConfig(cfg *config.CostOptimizationConfig) *CostOptimizer {
	// Start with defaults
	o := &CostOptimizer{
		MaxToolResultTokens: 8000,  // ~32KB of tool output
		MaxHistoryMessages:  20,    // Keep last 20 messages, summarize rest
		MaxHistoryTurns:     50,    // Hard cap at 50 messages
		MaxMessageTokens:    16000, // Compress messages over this
		KeepLastAssistants:  3,     // Keep last 3 assistant responses intact
		ToolRules: map[string]ToolPruneRule{
			// Code-related tools: keep more context
			"read_file":     {MaxTokens: 12000, KeepRecent: 2},
			"search_files":  {MaxTokens: 8000, KeepRecent: 1},
			"git_diff":      {MaxTokens: 10000, KeepRecent: 2},
			"git_log":       {MaxTokens: 4000, KeepRecent: 1},
			"preview_write": {MaxTokens: 6000, KeepRecent: 1},
			"preview_edit":  {MaxTokens: 6000, KeepRecent: 1},

			// Command output: prune aggressively
			"exec":    {MaxTokens: 4000, KeepRecent: 1, Aggressive: true},
			"process": {MaxTokens: 4000, KeepRecent: 1, Aggressive: true},

			// Web tools: moderate pruning
			"web_search": {MaxTokens: 4000, KeepRecent: 1},
			"web_fetch":  {MaxTokens: 8000, KeepRecent: 1},

			// Directory listings: small
			"list_dir": {MaxTokens: 2000, KeepRecent: 2},

			// System info: tiny
			"system_info": {MaxTokens: 1000, KeepRecent: 1},
		},
	}

	// Apply config overrides
	if cfg != nil {
		if cfg.MaxHistoryTurns > 0 {
			o.MaxHistoryTurns = cfg.MaxHistoryTurns
		}
		if cfg.MaxHistoryMessages > 0 {
			o.MaxHistoryMessages = cfg.MaxHistoryMessages
		}
		if cfg.KeepLastAssistants > 0 {
			o.KeepLastAssistants = cfg.KeepLastAssistants
		}
		if cfg.MaxToolResultTokens > 0 {
			o.MaxToolResultTokens = cfg.MaxToolResultTokens
		}

		// Apply tool-specific rules from config
		for toolName, rule := range cfg.ToolRules {
			o.ToolRules[toolName] = ToolPruneRule{
				MaxTokens:  rule.MaxTokens,
				KeepRecent: rule.KeepRecent,
				Aggressive: rule.Aggressive,
			}
		}
	}

	return o
}

// OptimizeRequest applies all cost optimizations to a request
func (o *CostOptimizer) OptimizeRequest(req ai.ChatRequest) ai.ChatRequest {
	optimized := req

	// 1. Hard limit on turns (drop oldest)
	if o.MaxHistoryTurns > 0 && len(optimized.Messages) > o.MaxHistoryTurns {
		optimized.Messages = o.EnforceHistoryTurnLimit(optimized.Messages)
	}

	// 2. Compress prompts (whitespace, redundancy)
	for i := range optimized.Messages {
		optimized.Messages[i].Content = o.CompressPrompt(optimized.Messages[i].Content)
	}

	// 3. Prune tool results with tool-specific rules
	optimized.Messages = o.PruneToolResults(optimized.Messages)

	// 4. Summarize old history if still too long
	if len(optimized.Messages) > o.MaxHistoryMessages {
		optimized.Messages = o.SummarizeHistory(optimized.Messages)
	}

	// 5. Set smart output limits based on task
	if optimized.MaxTokens == 0 {
		optimized.MaxTokens = o.EstimateOutputTokens(optimized.Messages)
	}

	return optimized
}

// EnforceHistoryTurnLimit drops oldest messages beyond limit
func (o *CostOptimizer) EnforceHistoryTurnLimit(messages []ai.Message) []ai.Message {
	if len(messages) <= o.MaxHistoryTurns {
		return messages
	}

	// Always keep system message if present
	var systemMsg *ai.Message
	var history []ai.Message

	for i, msg := range messages {
		if msg.Role == "system" && systemMsg == nil {
			systemMsg = &messages[i]
		} else {
			history = append(history, msg)
		}
	}

	// Keep only the most recent messages
	keepCount := o.MaxHistoryTurns
	if systemMsg != nil {
		keepCount-- // Reserve space for system message
	}

	if len(history) > keepCount {
		dropped := len(history) - keepCount
		history = history[dropped:]

		// Add note about dropped messages
		if len(history) > 0 {
			history[0].Content = fmt.Sprintf("[%d earlier messages dropped]\n\n%s", dropped, history[0].Content)
		}
	}

	// Rebuild with system message first
	result := []ai.Message{}
	if systemMsg != nil {
		result = append(result, *systemMsg)
	}
	result = append(result, history...)

	return result
}

// PruneToolResults applies tool-specific pruning rules
func (o *CostOptimizer) PruneToolResults(messages []ai.Message) []ai.Message {
	result := make([]ai.Message, len(messages))
	copy(result, messages)

	// Track tool result occurrences (newest first)
	toolCounts := make(map[string]int)

	// Process from newest to oldest
	for i := len(result) - 1; i >= 0; i-- {
		msg := &result[i]

		// Skip last N assistant messages
		if msg.Role == "assistant" {
			assistantCount := 0
			for j := len(result) - 1; j >= i; j-- {
				if result[j].Role == "assistant" {
					assistantCount++
				}
			}
			if assistantCount <= o.KeepLastAssistants {
				continue
			}
		}

		// Check if this is a tool result
		toolName := detectToolName(msg.Content)
		if toolName == "" {
			continue
		}

		toolCounts[toolName]++

		// Get rule for this tool
		rule, hasRule := o.ToolRules[toolName]
		if !hasRule {
			rule = ToolPruneRule{MaxTokens: o.MaxToolResultTokens}
		}

		// Skip if within KeepRecent
		if rule.KeepRecent > 0 && toolCounts[toolName] <= rule.KeepRecent {
			continue
		}

		// Apply truncation
		maxTokens := rule.MaxTokens
		if maxTokens == 0 {
			maxTokens = o.MaxToolResultTokens
		}

		if estimateTokens(msg.Content) > maxTokens {
			if rule.Aggressive {
				msg.Content = truncateAggressive(msg.Content, maxTokens)
			} else {
				msg.Content = truncateWithContext(msg.Content, maxTokens)
			}
		}
	}

	return result
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

// TruncateToolResults limits tool output size (legacy - use PruneToolResults)
func (o *CostOptimizer) TruncateToolResults(messages []ai.Message) []ai.Message {
	return o.PruneToolResults(messages)
}

// detectToolName extracts tool name from message content
func detectToolName(content string) string {
	// Common patterns for tool results
	patterns := []struct {
		prefix string
		tool   string
	}{
		{"read_file:", "read_file"},
		{"write_file:", "write_file"},
		{"edit_file:", "edit_file"},
		{"exec:", "exec"},
		{"list_dir:", "list_dir"},
		{"search_files:", "search_files"},
		{"git_status:", "git_status"},
		{"git_diff:", "git_diff"},
		{"git_log:", "git_log"},
		{"git_add:", "git_add"},
		{"git_commit:", "git_commit"},
		{"git_push:", "git_push"},
		{"web_search:", "web_search"},
		{"web_fetch:", "web_fetch"},
		{"process:", "process"},
		{"preview_write:", "preview_write"},
		{"preview_edit:", "preview_edit"},
		{"apply_patch:", "apply_patch"},
		{"system_info:", "system_info"},
		{"subagent:", "subagent"},
		// JSON-style tool results
		{`"tool": "read_file"`, "read_file"},
		{`"tool": "exec"`, "exec"},
		{`"tool": "list_dir"`, "list_dir"},
	}

	contentLower := strings.ToLower(content)
	for _, p := range patterns {
		if strings.Contains(contentLower, p.prefix) {
			return p.tool
		}
	}

	// Check for generic tool result patterns
	if strings.Contains(content, "✓") || strings.Contains(content, "→") {
		// Try to extract tool name from "✓ tool_name →" pattern
		if idx := strings.Index(content, "✓"); idx >= 0 {
			rest := content[idx+len("✓"):]
			rest = strings.TrimSpace(rest)
			if spaceIdx := strings.IndexAny(rest, " →("); spaceIdx > 0 {
				return strings.TrimSpace(rest[:spaceIdx])
			}
		}
	}

	return ""
}

// truncateAggressive keeps only the head (for noisy outputs like exec)
func truncateAggressive(content string, maxTokens int) string {
	estimated := estimateTokens(content)
	if estimated <= maxTokens {
		return content
	}

	chars := maxTokens * 4
	if len(content) <= chars {
		return content
	}

	// Just keep the head
	truncated := content[:chars]
	removed := len(content) - chars

	return fmt.Sprintf("%s\n\n... [%d chars truncated - use specific tools to see more]", truncated, removed)
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

	// Create memory flush - extract important items before dropping
	memoryFlush := createMemoryFlush(oldMessages)

	// Create summary of old messages
	summary := createHistorySummary(oldMessages)

	// Combine memory flush with summary
	fullSummary := summary
	if memoryFlush != "" {
		fullSummary = memoryFlush + "\n\n" + summary
	}

	// Build result: system + summary + recent
	result := []ai.Message{}
	if systemMsg != nil {
		result = append(result, *systemMsg)
	}
	result = append(result, ai.Message{
		Role:    "system",
		Content: "[Previous conversation summary]\n" + fullSummary,
	})
	result = append(result, recentMessages...)

	return result
}

// createMemoryFlush extracts important items from messages before they're dropped
func createMemoryFlush(messages []ai.Message) string {
	var items []string

	for _, msg := range messages {
		// Extract decisions, TODOs, constraints
		extracted := extractImportantItems(msg.Content)
		items = append(items, extracted...)
	}

	if len(items) == 0 {
		return ""
	}

	// Deduplicate and format
	seen := make(map[string]bool)
	var unique []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			unique = append(unique, item)
		}
	}

	if len(unique) > 10 {
		unique = unique[:10] // Cap at 10 items
	}

	return "Important context preserved:\n" + strings.Join(unique, "\n")
}

// extractImportantItems finds decisions, TODOs, and constraints in text
func extractImportantItems(content string) []string {
	var items []string

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		lineLower := strings.ToLower(line)

		// Decisions
		if strings.Contains(lineLower, "decided") ||
			strings.Contains(lineLower, "decision:") ||
			strings.Contains(lineLower, "we will") ||
			strings.Contains(lineLower, "let's go with") {
			items = append(items, "• "+strings.TrimSpace(line))
		}

		// TODOs
		if strings.Contains(lineLower, "todo") ||
			strings.Contains(lineLower, "todo:") ||
			strings.Contains(lineLower, "need to") ||
			strings.Contains(lineLower, "should") {
			if len(line) < 200 { // Skip long lines
				items = append(items, "• "+strings.TrimSpace(line))
			}
		}

		// Constraints
		if strings.Contains(lineLower, "constraint") ||
			strings.Contains(lineLower, "must not") ||
			strings.Contains(lineLower, "don't") ||
			strings.Contains(lineLower, "avoid") {
			if len(line) < 200 {
				items = append(items, "• "+strings.TrimSpace(line))
			}
		}

		// Files modified
		if strings.Contains(lineLower, "created") ||
			strings.Contains(lineLower, "modified") ||
			strings.Contains(lineLower, "updated") {
			if strings.Contains(line, "/") || strings.Contains(line, ".go") || strings.Contains(line, ".ts") {
				items = append(items, "• "+strings.TrimSpace(line))
			}
		}
	}

	return items
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
		removed := estimated - maxTokens
		return fmt.Sprintf("%s\n... [truncated, ~%d tokens removed]", content[:chars], removed)
	}

	// Keep first 1/3 and last 1/3 of lines
	keepLines := (maxTokens * 4) / 80 // Assume ~80 chars per line
	if keepLines < 6 {
		keepLines = 6
	}
	headLines := keepLines / 2
	tailLines := keepLines - headLines

	if headLines >= len(lines) {
		headLines = len(lines) / 2
	}
	if tailLines >= len(lines)-headLines {
		tailLines = len(lines) - headLines
	}

	head := strings.Join(lines[:headLines], "\n")
	tail := strings.Join(lines[len(lines)-tailLines:], "\n")
	removed := len(lines) - headLines - tailLines

	return fmt.Sprintf("%s\n\n... [%d lines truncated] ...\n\n%s", head, removed, tail)
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
