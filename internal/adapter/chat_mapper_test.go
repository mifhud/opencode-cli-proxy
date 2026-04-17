package adapter

import (
	"encoding/json"
	"testing"

	"opencode-cli-proxy/internal/config"
	"opencode-cli-proxy/internal/domain"
)

func TestMapTemperature(t *testing.T) {
	got := MapTemperature(1, 0, 1)
	if got != 0.5 {
		t.Fatalf("expected 0.5, got %v", got)
	}
}

func TestMapChatRequest(t *testing.T) {
	cfg := &config.Config{Models: map[string]string{"opencode-go-latest": "upstream-model"}}
	req := domain.OpenAIChatCompletionRequest{
		Model:    "opencode-go-latest",
		Messages: []domain.OpenAIChatMessage{{Role: "user", Content: "hello"}},
	}
	mapped, err := MapChatRequest(req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if mapped.Model != "upstream-model" {
		t.Fatalf("unexpected model %s", mapped.Model)
	}
	if len(mapped.Messages) != 1 || mapped.Messages[0].Text != "hello" {
		t.Fatalf("unexpected messages %#v", mapped.Messages)
	}
}

func TestMapChatRequestAcceptsContentParts(t *testing.T) {
	cfg := &config.Config{Models: map[string]string{"ocproxy/x": "upstream-model"}}
	body := []byte(`{"model":"ocproxy/x","messages":[{"role":"system","content":"sys"},{"role":"user","content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}]}`)

	var req domain.OpenAIChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}

	mapped, err := MapChatRequest(req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(mapped.Messages) != 2 {
		t.Fatalf("unexpected messages %#v", mapped.Messages)
	}
	if mapped.Messages[0].Text != "sys" || mapped.Messages[1].Text != "hello world" {
		t.Fatalf("unexpected message text %#v", mapped.Messages)
	}
}
