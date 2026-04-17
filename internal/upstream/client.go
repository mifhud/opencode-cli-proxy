package upstream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"opencode-cli-proxy/internal/config"
	"opencode-cli-proxy/internal/domain"
)

type Client struct {
	cfg *config.Config
}

// opencodeEvent is the subset of `opencode run --format json` events this
// proxy consumes; the CLI writes one JSON object per line.
type opencodeEvent struct {
	Type  string `json:"type"`
	Part  struct {
		Text   string `json:"text"`
		Tokens struct {
			Input     int `json:"input"`
			Output    int `json:"output"`
			Total     int `json:"total"`
			Reasoning int `json:"reasoning"`
		} `json:"tokens"`
	} `json:"part"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewClient(cfg *config.Config, _ any) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) Chat(ctx context.Context, _ string, req domain.ChatRequest) (*domain.ChatResponse, int, error) {
	cmd := c.buildCommand(ctx, req)
	output, err := cmd.Output()
	if err != nil {
		// opencode reports provider failures as an `error` event on stdout and
		// exits non-zero; prefer that message over "exit status 1".
		if _, eventErr := parseOpencodeOutput(strings.NewReader(string(output)), false); eventErr != nil {
			return nil, 502, eventErr
		}
		return nil, 502, commandError(err)
	}

	resp, err := parseOpencodeOutput(strings.NewReader(string(output)), false)
	if err != nil {
		return nil, 502, err
	}
	return resp, 200, nil
}

func (c *Client) ChatStream(ctx context.Context, _ string, req domain.ChatRequest) (<-chan domain.StreamEvent, <-chan error, int, error) {
	cmd := c.buildCommand(ctx, req)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, 500, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, 500, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, 502, commandError(err)
	}

	events := make(chan domain.StreamEvent)
	errCh := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errCh)
		defer stdout.Close()
		defer stderr.Close()

		if err := streamOpencodeOutput(stdout, events); err != nil {
			errCh <- err
			_ = cmd.Wait()
			return
		}
		if err := cmd.Wait(); err != nil {
			msg, _ := io.ReadAll(stderr)
			if len(strings.TrimSpace(string(msg))) > 0 {
				errCh <- fmt.Errorf(strings.TrimSpace(string(msg)))
				return
			}
			errCh <- commandError(err)
		}
	}()

	return events, errCh, 200, nil
}

func (c *Client) buildCommand(ctx context.Context, req domain.ChatRequest) *exec.Cmd {
	args := []string{"run", "--format", "json", "--model", req.Model}
	if c.cfg.Upstream.Attach != "" {
		args = append(args, "--attach", c.cfg.Upstream.Attach)
	}
	args = append(args, buildPrompt(req))
	return exec.CommandContext(ctx, c.cfg.Upstream.Binary, args...)
}

func buildPrompt(req domain.ChatRequest) string {
	var parts []string
	for _, msg := range req.Messages {
		role := strings.ToUpper(strings.TrimSpace(msg.Role))
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("[%s]\n%s", role, text))
	}
	return strings.Join(parts, "\n\n")
}

// maxEventBytes caps a single JSONL event: tool results and echoed file
// contents can be large.
const maxEventBytes = 8 << 20

func newEventScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventBytes)
	return scanner
}

func parseOpencodeOutput(r io.Reader, _ bool) (*domain.ChatResponse, error) {
	scanner := newEventScanner(r)
	resp := &domain.ChatResponse{}
	var text strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event opencodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, err
		}

		switch event.Type {
		case "text":
			text.WriteString(event.Part.Text)
		case "step_finish":
			// A run reports one step_finish per step; later steps can still
			// emit text, so only the reason is recorded here.
			resp.FinishReason = "stop"
			resp.Usage = usageFrom(event)
		case "error":
			return nil, fmt.Errorf("%s", event.Error.Message)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	resp.Content = text.String()
	if resp.FinishReason == "" {
		resp.FinishReason = "stop"
	}
	return resp, nil
}

// streamOpencodeOutput forwards every text delta of the run. `opencode run`
// emits a step_finish after each step, so the stream may only end at EOF:
// returning at the first step_finish drops the answer of any run that reads a
// file, edits, or thinks before replying.
func streamOpencodeOutput(r io.Reader, events chan<- domain.StreamEvent) error {
	scanner := newEventScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event opencodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return err
		}

		switch event.Type {
		case "text":
			if event.Part.Text != "" {
				events <- domain.StreamEvent{Delta: event.Part.Text}
			}
		case "error":
			events <- domain.StreamEvent{ErrMsg: event.Error.Message, Done: true}
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	events <- domain.StreamEvent{FinishReason: "stop", Done: true}
	return nil
}

func usageFrom(event opencodeEvent) *domain.Usage {
	total := event.Part.Tokens.Total
	if total == 0 {
		// opencode reports input/output only, so a missing total is derived.
		total = event.Part.Tokens.Input + event.Part.Tokens.Output
	}
	return &domain.Usage{
		PromptTokens:     event.Part.Tokens.Input,
		CompletionTokens: event.Part.Tokens.Output,
		TotalTokens:      total,
	}
}

func commandError(err error) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
		msg := strings.TrimSpace(string(exitErr.Stderr))
		if msg != "" {
			return fmt.Errorf(msg)
		}
	}
	return err
}
