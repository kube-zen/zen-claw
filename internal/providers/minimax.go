package providers

import (
	"context"
	"fmt"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/sashabaranov/go-openai"
)

// MinimaxProvider implements Minimax R2/M2.1 which uses OpenAI-compatible API
type MinimaxProvider struct {
	client *openai.Client
	config ProviderConfig
}

func NewMinimaxProvider(config ProviderConfig) (*MinimaxProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key required for Minimax")
	}

	// Minimax uses OpenAI-compatible API with custom endpoint
	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	} else {
		clientConfig.BaseURL = "https://api.minimax.chat/v1"
	}

	client := openai.NewClientWithConfig(clientConfig)
	return &MinimaxProvider{
		client: client,
		config: config,
	}, nil
}

func (p *MinimaxProvider) Name() string {
	return "minimax"
}

func (p *MinimaxProvider) SupportsTools() bool {
	return true // Minimax supports tool calling
}

func (p *MinimaxProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	// Convert messages to OpenAI format
	var messages []openai.ChatCompletionMessage
	for _, msg := range req.Messages {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
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
	if model == "" {
		model = "minimax-M2.1"
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

	// Make API call
	resp, err := p.client.CreateChatCompletion(ctx, completionReq)
	if err != nil {
		return nil, fmt.Errorf("Minimax API error: %w", err)
	}

	// Convert response
	chatResp := &ai.ChatResponse{
		Content:      resp.Choices[0].Message.Content,
		FinishReason: string(resp.Choices[0].FinishReason),
	}

	// Extract tool calls if any
	if resp.Choices[0].Message.ToolCalls != nil {
		for _, toolCall := range resp.Choices[0].Message.ToolCalls {
			args := make(map[string]interface{})
			args["_raw"] = toolCall.Function.Arguments
			
			chatResp.ToolCalls = append(chatResp.ToolCalls, ai.ToolCall{
				ID:   toolCall.ID,
				Name: toolCall.Function.Name,
				Args: args,
			})
		}
	}

	return chatResp, nil
}