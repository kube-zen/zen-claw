package providers

import (
	"context"
	"fmt"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/sashabaranov/go-openai"
)

// GLMProvider implements GLM-4.7 (Zhipu AI) which uses OpenAI-compatible API
type GLMProvider struct {
	client *openai.Client
	config ProviderConfig
}

func NewGLMProvider(config ProviderConfig) (*GLMProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key required for GLM")
	}

	// GLM uses OpenAI-compatible API with custom endpoint
	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	} else {
		clientConfig.BaseURL = "https://open.bigmodel.cn/api/paas/v4"
	}

	client := openai.NewClientWithConfig(clientConfig)
	return &GLMProvider{
		client: client,
		config: config,
	}, nil
}

func (p *GLMProvider) Name() string {
	return "glm"
}

func (p *GLMProvider) SupportsTools() bool {
	return true // GLM-4.7 supports tool calling
}

func (p *GLMProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
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
		model = "glm-4.7"
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
		return nil, fmt.Errorf("GLM API error: %w", err)
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