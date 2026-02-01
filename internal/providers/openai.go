package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client *openai.Client
	config Config
}

type Config struct {
	APIKey string
	Model  string // Default model to use
}

func NewOpenAIProvider(config Config) (*OpenAIProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key required for OpenAI")
	}

	client := openai.NewClient(config.APIKey)
	return &OpenAIProvider{
		client: client,
		config: config,
	}, nil
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) SupportsTools() bool {
	return true
}

func (p *OpenAIProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
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
		model = "gpt-4o-mini" // Default
	}

	// Create completion request
	completionReq := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
	}

	// Add thinking mode if requested
	if req.Thinking {
		// For OpenAI, we might use a different approach
		// For now, just note it's requested
		completionReq.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeText,
		}
	}

	// Make API call
	resp, err := p.client.CreateChatCompletion(ctx, completionReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	// Convert response
	chatResp := &ai.ChatResponse{
		Content:    resp.Choices[0].Message.Content,
		FinishReason: string(resp.Choices[0].FinishReason),
	}

	// Extract tool calls if any
	if resp.Choices[0].Message.ToolCalls != nil {
		for _, toolCall := range resp.Choices[0].Message.ToolCalls {
			// Parse arguments (assuming JSON)
			args := make(map[string]interface{})
			// In real implementation, we'd parse the JSON arguments
			// For now, just store the raw string
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

// SimpleProvider is a fallback that doesn't require API key
type SimpleProvider struct{}

func NewSimpleProvider() *SimpleProvider {
	return &SimpleProvider{}
}

func (p *SimpleProvider) Name() string {
	return "simple"
}

func (p *SimpleProvider) SupportsTools() bool {
	return false
}

func (p *SimpleProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	// Simple echo with tool awareness
	response := "I'm a simple AI provider. "
	if len(req.Tools) > 0 {
		response += fmt.Sprintf("I see %d tools available: ", len(req.Tools))
		for _, tool := range req.Tools {
			response += tool.Name + ", "
		}
		response = strings.TrimSuffix(response, ", ") + ". "
	}
	
	response += fmt.Sprintf("You said: %s", req.Messages[len(req.Messages)-1].Content)
	
	return &ai.ChatResponse{
		Content:      response,
		FinishReason: "stop",
	}, nil
}