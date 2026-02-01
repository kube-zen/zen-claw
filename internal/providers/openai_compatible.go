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

	// Set default base URLs for known providers
	if config.BaseURL == "" {
		switch strings.ToLower(name) {
		case "openai":
			config.BaseURL = "https://api.openai.com/v1"
		case "deepseek":
			config.BaseURL = "https://api.deepseek.com"
		case "glm":
			config.BaseURL = "https://open.bigmodel.cn/api/paas/v4"
		case "minimax":
			config.BaseURL = "https://api.minimax.chat/v1"
		default:
			// Use OpenAI default if not specified
			config.BaseURL = "https://api.openai.com/v1"
		}
	}

	// Set default models for known providers
	if config.Model == "" {
		switch strings.ToLower(name) {
		case "openai":
			config.Model = "gpt-4o-mini"
		case "deepseek":
			config.Model = "deepseek-chat"
		case "glm":
			config.Model = "glm-4.7"
		case "minimax":
			config.Model = "minimax-M2.1"
		default:
			config.Model = "gpt-4o-mini"
		}
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

func (p *OpenAICompatibleProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	// Convert messages to OpenAI format
	var messages []openai.ChatCompletionMessage
	for _, msg := range req.Messages {
		openaiMsg := openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		
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
		
		messages = append(messages, openaiMsg)
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
		Messages: messages,
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
	}

	// Make API call
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