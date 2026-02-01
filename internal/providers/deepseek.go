package providers

import (
	"context"
	"fmt"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/sashabaranov/go-openai"
)

type DeepSeekProvider struct {
	client *openai.Client
	config ProviderConfig
}

func NewDeepSeekProvider(config ProviderConfig) (*DeepSeekProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key required for DeepSeek")
	}

	// DeepSeek uses OpenAI-compatible API
	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	} else {
		clientConfig.BaseURL = "https://api.deepseek.com"
	}

	client := openai.NewClientWithConfig(clientConfig)
	return &DeepSeekProvider{
		client: client,
		config: config,
	}, nil
}

func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}

func (p *DeepSeekProvider) SupportsTools() bool {
	return true // DeepSeek supports tools
}

func (p *DeepSeekProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
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
		model = "deepseek-chat"
	}

	// Create completion request
	completionReq := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
	}

	// Make API call
	resp, err := p.client.CreateChatCompletion(ctx, completionReq)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek API error: %w", err)
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