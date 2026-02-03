package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/sashabaranov/go-openai"
)

// OpenAICompatibleProvider handles all OpenAI-compatible APIs
// Supports: OpenAI, DeepSeek, GLM-4.7, Minimax, etc.
type OpenAICompatibleProvider struct {
	client *openai.Client
	config ProviderConfig
	name   string
}

// NewOpenAICompatibleProvider creates a provider for any OpenAI-compatible API
func NewOpenAICompatibleProvider(name string, config ProviderConfig) (*OpenAICompatibleProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key required for %s", name)
	}

	// Use centralized defaults
	if config.BaseURL == "" {
		config.BaseURL = GetDefaultBaseURL(name)
		if config.BaseURL == "" {
			config.BaseURL = "https://api.openai.com/v1"
		}
	}

	if config.Model == "" {
		config.Model = GetDefaultModel(name)
	}

	clientConfig := openai.DefaultConfig(config.APIKey)
	clientConfig.BaseURL = config.BaseURL

	client := openai.NewClientWithConfig(clientConfig)
	return &OpenAICompatibleProvider{
		client: client,
		config: config,
		name:   name,
	}, nil
}

func (p *OpenAICompatibleProvider) Name() string {
	return p.name
}

func (p *OpenAICompatibleProvider) SupportsTools() bool {
	return true // All OpenAI-compatible APIs support tool calling
}

// Chat implements the AI provider interface
func (p *OpenAICompatibleProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	messages := req.Messages

	// Apply context limit (0 = unlimited, default 50 from session)
	// If ContextLimit is 0, it means unlimited was explicitly requested
	// Otherwise, use the value from the request (which comes from session, default 50)
	contextLimit := req.ContextLimit
	// Note: 0 means unlimited, non-zero means that many messages
	// The session defaults to 50, so if ContextLimit is 0 here, it means user set it to unlimited

	// truncateMessages truncates messages while preserving tool call sequences.
	// Tool messages must follow an assistant message with tool_calls, so we can't break that pattern.
	truncateMessages := func(msgs []ai.Message, maxCount int) []ai.Message {
		if len(msgs) <= maxCount {
			return msgs
		}

		// Keep system message if present
		hasSystem := len(msgs) > 0 && msgs[0].Role == "system"
		startIdx := 0
		if hasSystem {
			startIdx = 1
			maxCount-- // Reserve one slot for system message
		}

		// Calculate where to start truncation
		truncateStart := len(msgs) - maxCount
		if truncateStart <= startIdx {
			return msgs // No truncation needed
		}

		// Find the start of the first complete tool call sequence after truncateStart
		// A tool call sequence is: assistant (with tool_calls) -> tool -> tool -> ... -> assistant
		// We need to ensure we don't cut in the middle of a sequence
		actualStart := truncateStart
		for i := truncateStart; i < len(msgs); i++ {
			// If we find a tool message, we need to include its preceding assistant message
			if msgs[i].Role == "tool" {
				// Look backwards for the assistant message with tool_calls
				for j := i - 1; j >= startIdx; j-- {
					if msgs[j].Role == "assistant" && len(msgs[j].ToolCalls) > 0 {
						// Found the assistant with tool_calls, start from here
						actualStart = j
						break
					}
					if msgs[j].Role != "tool" {
						// Not part of this tool sequence, stop looking
						break
					}
				}
				break
			}
		}

		// Build result
		result := make([]ai.Message, 0, maxCount+1)
		if hasSystem {
			result = append(result, msgs[0])
		}
		result = append(result, msgs[actualStart:]...)

		return result
	}

	// Apply context limit with proper tool call sequence handling
	// For all providers including Qwen: respect user's context limit setting
	if contextLimit > 0 {
		messages = truncateMessages(messages, contextLimit)
	} else if p.name == "qwen" && !req.QwenLargeContextEnabled {
		// Qwen with large context disabled and no user limit: use safe default (10)
		// This only applies when user hasn't set a context limit
		// If user sets /context-limit, we respect it fully regardless of large context setting
		messages = truncateMessages(messages, 10)
	}

	// Convert messages to OpenAI format
	var openaiMessages []openai.ChatCompletionMessage
	for _, msg := range messages {
		// Ensure Content is always a string (Qwen requires string, not object)
		// Qwen API is strict: content must be string or array of objects, never a plain object
		contentStr := msg.Content

		// Handle empty content - use empty string, not nil
		if contentStr == "" {
			contentStr = ""
		}

		// For assistant messages with tool calls, content can be empty (tool calls are in separate field)
		// For other roles, ensure we have valid string content
		if msg.Role != "assistant" && contentStr == "" {
			// Non-assistant messages should have content, but if empty, use placeholder
			// This shouldn't happen, but handle gracefully
			contentStr = ""
		}

		openaiMsg := openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: contentStr, // Always a string, never an object
		}

		// Qwen is strict: content must be string, never an object
		// If content appears to be JSON object/array as string, that's fine
		// But ensure we're not accidentally passing a Go object/struct

		// Handle tool role messages (need tool_call_id)
		if msg.Role == "tool" && msg.ToolCallID != "" {
			openaiMsg.ToolCallID = msg.ToolCallID
		}

		// Handle assistant messages with tool calls
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			var toolCalls []openai.ToolCall
			for _, tc := range msg.ToolCalls {
				// Convert arguments to JSON string
				argsJSON := "{}"
				if len(tc.Args) > 0 {
					if jsonBytes, err := json.Marshal(tc.Args); err == nil {
						argsJSON = string(jsonBytes)
					}
				}

				toolCalls = append(toolCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: argsJSON,
					},
				})
			}
			openaiMsg.ToolCalls = toolCalls
		}

		openaiMessages = append(openaiMessages, openaiMsg)
	}

	// Convert tools to OpenAI format
	var tools []openai.Tool
	for _, tool := range req.Tools {
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	// Determine model to use
	model := p.config.Model
	if req.Model != "" && req.Model != "default" {
		model = req.Model
	}

	// Create completion request
	completionReq := openai.ChatCompletionRequest{
		Model:    model,
		Messages: openaiMessages,
		Tools:    tools,
	}

	// Add thinking mode if requested
	if req.Thinking {
		completionReq.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeText,
		}
	}

	// Add temperature if specified
	if req.Temperature > 0 {
		completionReq.Temperature = float32(req.Temperature)
	}

	// Add max tokens if specified
	if req.MaxTokens > 0 {
		completionReq.MaxTokens = req.MaxTokens
	} else if p.name == "qwen" && !req.QwenLargeContextEnabled {
		// Qwen: reduce max tokens when large context is disabled to speed up responses
		// Smaller responses = faster API calls = less timeout risk
		completionReq.MaxTokens = 1000 // Reduced from default 2000
	}

	// Add stream option for better UX
	completionReq.Stream = false // For now, disable streaming for simplicity

	// Make API call
	// Note: For Qwen, the context may be canceled if HTTP client times out (180s)
	// We use small message windows (10 messages) and reduced max tokens to keep it fast
	resp, err := p.client.CreateChatCompletion(ctx, completionReq)
	if err != nil {
		return nil, fmt.Errorf("%s API error: %w", p.name, err)
	}

	// Convert response
	chatResp := &ai.ChatResponse{
		Content:      resp.Choices[0].Message.Content,
		FinishReason: string(resp.Choices[0].FinishReason),
	}

	// Extract tool calls if any
	if resp.Choices[0].Message.ToolCalls != nil {
		for _, toolCall := range resp.Choices[0].Message.ToolCalls {
			// Parse JSON arguments
			args := make(map[string]interface{})
			if toolCall.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					// If parsing fails, store raw string
					args["_raw"] = toolCall.Function.Arguments
				}
			}

			chatResp.ToolCalls = append(chatResp.ToolCalls, ai.ToolCall{
				ID:   toolCall.ID,
				Name: toolCall.Function.Name,
				Args: args,
			})
		}
	}

	return chatResp, nil
}

// ChatStream implements streaming chat with token-by-token callback
func (p *OpenAICompatibleProvider) ChatStream(ctx context.Context, req ai.ChatRequest, callback ai.StreamCallback) (*ai.ChatResponse, error) {
	// If no tools are requested, we can stream
	// Tool calls require non-streaming to get complete response
	if len(req.Tools) > 0 {
		// Fall back to non-streaming for tool calls
		return p.Chat(ctx, req)
	}

	// Convert messages
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	model := p.config.Model
	if req.Model != "" {
		model = req.Model
	}

	maxTokens := 8192
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	completionReq := openai.ChatCompletionRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
		Stream:    true,
	}

	if req.Temperature > 0 {
		completionReq.Temperature = float32(req.Temperature)
	}

	// Create streaming request
	stream, err := p.client.CreateChatCompletionStream(ctx, completionReq)
	if err != nil {
		return nil, fmt.Errorf("stream request failed: %w", err)
	}
	defer stream.Close()

	var content strings.Builder
	var finishReason string

	for {
		response, err := stream.Recv()
		if err != nil {
			// Check for normal stream end
			if err.Error() == "EOF" {
				break
			}
			// Check if context cancelled
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("stream receive error: %w", err)
		}

		if len(response.Choices) > 0 {
			delta := response.Choices[0].Delta.Content
			if delta != "" {
				content.WriteString(delta)
				if callback != nil {
					callback(delta)
				}
			}
			if response.Choices[0].FinishReason != "" {
				finishReason = string(response.Choices[0].FinishReason)
			}
		}
	}

	return &ai.ChatResponse{
		Content:      content.String(),
		FinishReason: finishReason,
	}, nil
}
