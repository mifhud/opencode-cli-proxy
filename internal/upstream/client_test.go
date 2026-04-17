package upstream

import (
	"strings"
	"testing"

	"opencode-cli-proxy/internal/domain"
)

const multiStepRun = `{"type":"step_start","part":{"type":"step-start"}}
{"type":"tool_use","part":{"type":"tool","tool":"shell"}}
{"type":"step_finish","part":{"type":"step-finish","reason":"tool-calls","tokens":{"input":10,"output":5}}}
{"type":"text","part":{"type":"text","text":"module opencode-cli-proxy"}}
{"type":"step_finish","part":{"type":"step-finish","reason":"stop","tokens":{"input":20,"output":8}}}`

// A run that reads or edits before answering reports one step_finish per step.
// The text of the later step must survive streaming: the SSE response used to
// end at the first step_finish and deliver an empty reply.
func TestStreamOpencodeOutputKeepsTextFromLaterSteps(t *testing.T) {
	events := make(chan domain.StreamEvent, 8)
	if err := streamOpencodeOutput(strings.NewReader(multiStepRun), events); err != nil {
		t.Fatal(err)
	}
	close(events)

	var deltas []string
	var done bool
	for event := range events {
		if event.Delta != "" {
			deltas = append(deltas, event.Delta)
		}
		if event.Done {
			done = true
			if event.ErrMsg != "" {
				t.Fatalf("unexpected error %q", event.ErrMsg)
			}
		}
	}

	if len(deltas) != 1 || deltas[0] != "module opencode-cli-proxy" {
		t.Fatalf("dropped text from a later step: %#v", deltas)
	}
	if !done {
		t.Fatal("stream never reported Done")
	}
}

func TestStreamOpencodeOutputSurfacesUpstreamError(t *testing.T) {
	events := make(chan domain.StreamEvent, 4)
	if err := streamOpencodeOutput(strings.NewReader(`{"type":"error","error":{"message":"Free usage limit reached"}}`), events); err != nil {
		t.Fatal(err)
	}
	close(events)

	event := <-events
	if event.ErrMsg != "Free usage limit reached" || !event.Done {
		t.Fatalf("unexpected event %#v", event)
	}
}

func TestParseOpencodeOutputReadsMultiStepRunAndUsage(t *testing.T) {
	resp, err := parseOpencodeOutput(strings.NewReader(multiStepRun), false)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "module opencode-cli-proxy" || resp.FinishReason != "stop" {
		t.Fatalf("unexpected response %#v", resp)
	}
	if resp.Usage == nil || resp.Usage.PromptTokens != 20 || resp.Usage.CompletionTokens != 8 || resp.Usage.TotalTokens != 28 {
		t.Fatalf("unexpected usage %#v", resp.Usage)
	}
}

func TestParseOpencodeOutputFailsOnUpstreamError(t *testing.T) {
	if _, err := parseOpencodeOutput(strings.NewReader(`{"type":"error","error":{"message":"provider.free-limit"}}`), false); err == nil {
		t.Fatal("expected an error for an upstream error event")
	}
}
