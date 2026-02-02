package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/neves/zen-claw/internal/ai"
)

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
