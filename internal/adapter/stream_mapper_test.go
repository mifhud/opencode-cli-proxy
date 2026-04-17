package adapter

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildStreamChunk(t *testing.T) {
	chunk, err := BuildStreamChunk("id1", "model1", 0, "hello", true, "")
	if err != nil {
		t.Fatal(err)
	}
	text := string(chunk)
	if !strings.Contains(text, "data: ") || !strings.Contains(text, "hello") {
		t.Fatalf("unexpected chunk %s", text)
	}
}

func TestBuildCompletionStreamChunk(t *testing.T) {
	chunk, err := BuildCompletionStreamChunk("id1", "model1", "hello", "")
	if err != nil {
		t.Fatal(err)
	}

	var payload struct {
		Object  string `json:"object"`
		Choices []struct {
			Text string `json:"text"`
		} `json:"choices"`
	}
	body := strings.TrimSuffix(strings.TrimPrefix(string(chunk), "data: "), "\n\n")
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Object != "text_completion" || payload.Choices[0].Text != "hello" {
		t.Fatalf("unexpected chunk %s", chunk)
	}

	finish, err := BuildCompletionStreamChunk("id1", "model1", "", "stop")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(finish), `"finish_reason":"stop"`) || strings.Contains(string(finish), `"text"`) {
		t.Fatalf("unexpected finish chunk %s", finish)
	}
}
