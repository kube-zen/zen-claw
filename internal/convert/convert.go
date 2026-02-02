package convert

import (
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/session"
)

// ToAIMessage converts a session.Message to ai.Message
func ToAIMessage(sm session.Message) ai.Message {
	// Convert session.ToolCall to ai.ToolCall
	var aiToolCalls []ai.ToolCall
	for _, tc := range sm.ToolCalls {
		aiToolCalls = append(aiToolCalls, ai.ToolCall{
			ID:   tc.ID,
			Name: tc.Name,
			Args: tc.Args,
		})
	}
	
	return ai.Message{
		Role:       sm.Role,
		Content:    sm.Content,
		ToolCalls:  aiToolCalls,
		ToolCallID: sm.ToolCallID,
	}
}

// ToSessionMessage converts an ai.Message to session.Message
func ToSessionMessage(am ai.Message) session.Message {
	// Convert ai.ToolCall to session.ToolCall
	var sessionToolCalls []session.ToolCall
	for _, tc := range am.ToolCalls {
		sessionToolCalls = append(sessionToolCalls, session.ToolCall{
			ID:   tc.ID,
			Name: tc.Name,
			Args: tc.Args,
		})
	}
	
	return session.Message{
		Role:       am.Role,
		Content:    am.Content,
		Time:       time.Now(),
		ToolCalls:  sessionToolCalls,
		ToolCallID: am.ToolCallID,
	}
}

// ToAIMessages converts a slice of session.Message to ai.Message
func ToAIMessages(sms []session.Message) []ai.Message {
	aims := make([]ai.Message, len(sms))
	for i, sm := range sms {
		aims[i] = ToAIMessage(sm)
	}
	return aims
}

// ToSessionMessages converts a slice of ai.Message to session.Message
func ToSessionMessages(aims []ai.Message) []session.Message {
	sms := make([]session.Message, len(aims))
	for i, am := range aims {
		sms[i] = ToSessionMessage(am)
	}
	return sms
}