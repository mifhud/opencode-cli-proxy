package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

type OpenAIChatMessage struct {
	Role    string         `json:"role,omitempty"`
	Content MessageContent `json:"content,omitempty"`
}

// MessageContent holds a message body. OpenAI clients send either a plain
// string or an array of content parts ({type,text}); both are flattened to
// text, and output always marshals back as a string.
type MessageContent string

func (c *MessageContent) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*c = MessageContent(text)
		return nil
	}

	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &parts); err != nil {
		return fmt.Errorf("message content must be a string or an array of content parts")
	}

	var builder strings.Builder
	for _, part := range parts {
		builder.WriteString(part.Text)
	}
	*c = MessageContent(builder.String())
	return nil
}

func (c MessageContent) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(c))
}

type OpenAIChatCompletionRequest struct {
	Model       string              `json:"model"`
	Messages    []OpenAIChatMessage `json:"messages"`
	Temperature *float64            `json:"temperature,omitempty"`
	MaxTokens   *int                `json:"max_tokens,omitempty"`
	Stop        []string            `json:"stop,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
}

type OpenAICompletionRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	Stop        []string `json:"stop,omitempty"`
	Stream      bool     `json:"stream,omitempty"`
}

type OpenAIModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type OpenAIModelsResponse struct {
	Object string            `json:"object"`
	Data   []OpenAIModelInfo `json:"data"`
}

type OpenAIChoice struct {
	Index        int                `json:"index"`
	Message      *OpenAIChatMessage `json:"message,omitempty"`
	Text         string             `json:"text,omitempty"`
	Delta        *OpenAIChatMessage `json:"delta,omitempty"`
	FinishReason string             `json:"finish_reason,omitempty"`
}

type OpenAIChatCompletionResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   interface{}    `json:"usage,omitempty"`
}

type OpenAIErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   any    `json:"param"`
	Code    string `json:"code"`
}

type OpenAIErrorResponse struct {
	Error OpenAIErrorBody `json:"error"`
}
