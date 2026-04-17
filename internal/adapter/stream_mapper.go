package adapter

import (
	"encoding/json"
	"fmt"
	"time"

	"opencode-cli-proxy/internal/domain"
)

func BuildStreamChunk(id, model string, index int, delta string, includeRole bool, finishReason string) ([]byte, error) {
	msg := &domain.OpenAIChatMessage{Content: domain.MessageContent(delta)}
	if includeRole {
		msg.Role = "assistant"
	}

	resp := domain.OpenAIChatCompletionResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []domain.OpenAIChoice{{
			Index:        index,
			Delta:        msg,
			FinishReason: finishReason,
		}},
	}
	payload, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("data: %s\n\n", payload)), nil
}

// BuildCompletionStreamChunk emits a legacy /v1/completions SSE chunk: the
// payload carries choices[].text, not the chat-style choices[].delta.
func BuildCompletionStreamChunk(id, model, text, finishReason string) ([]byte, error) {
	choice := domain.OpenAIChoice{Index: 0, FinishReason: finishReason}
	if text != "" {
		choice.Text = text
	}

	resp := domain.OpenAIChatCompletionResponse{
		ID:      id,
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []domain.OpenAIChoice{choice},
	}
	payload, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("data: %s\n\n", payload)), nil
}

// BuildKeepAliveComment emits an SSE comment, which OpenAI clients ignore, so
// long silent steps (tool calls, thinking) do not trip an idle timeout.
func BuildKeepAliveComment() []byte {
	return []byte(": keep-alive\n\n")
}

// BuildErrorChunk reports an upstream failure that happened after the SSE
// response had already started.
func BuildErrorChunk(message string) []byte {
	resp := domain.OpenAIErrorResponse{Error: domain.OpenAIErrorBody{
		Message: message,
		Type:    "upstream_error",
		Code:    "502",
	}}
	payload, err := json.Marshal(resp)
	if err != nil {
		return []byte("data: {\"error\":{\"message\":\"upstream error\",\"code\":\"502\"}}\n\n")
	}
	return []byte(fmt.Sprintf("data: %s\n\n", payload))
}

func BuildDoneChunk() []byte {
	return []byte("data: [DONE]\n\n")
}

func NormalizeStreamEvent(event domain.StreamEvent) domain.StreamEvent {
	return event
}
