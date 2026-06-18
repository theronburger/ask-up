package consult

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/theronburger/ask-up/internal/config"
	"github.com/theronburger/ask-up/internal/store"
)

// TestAsk wires the SDK to a stub server so we exercise request building and
// response/usage parsing without a network or a real key.
func TestAsk(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	const reply = `{
		"id": "msg_test",
		"type": "message",
		"role": "assistant",
		"model": "claude-opus-4-8",
		"stop_reason": "end_turn",
		"content": [{"type": "text", "text": "pong"}],
		"usage": {
			"input_tokens": 12,
			"output_tokens": 3,
			"cache_read_input_tokens": 7,
			"cache_creation_input_tokens": 0
		}
	}`

	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, reply)
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.BaseURL = srv.URL

	history := []store.Message{
		{Role: "user", Text: "earlier question"},
		{Role: "assistant", Text: "earlier answer"},
	}
	res, err := Ask(context.Background(), cfg, history, "ping")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}

	if res.Answer != "pong" {
		t.Errorf("answer = %q, want pong", res.Answer)
	}
	if res.InputTokens != 12 || res.CacheReadInputTokens != 7 || res.OutputTokens != 3 {
		t.Errorf("usage parsed wrong: %+v", res)
	}
	if got, want := res.PrefixTokens(), int64(19); got != want {
		t.Errorf("PrefixTokens = %d, want %d", got, want)
	}

	// The request should carry the system prompt, the history, and the new question.
	for _, frag := range []string{"earlier question", "earlier answer", "ping", "adaptive", "\"effort\":\"high\""} {
		if !strings.Contains(gotBody, frag) {
			t.Errorf("request body missing %q", frag)
		}
	}
}

func TestRunSecretCommand(t *testing.T) {
	got, err := runSecretCommand("printf 'sk-test-123'")
	if err != nil {
		t.Fatalf("runSecretCommand: %v", err)
	}
	if got != "sk-test-123" {
		t.Errorf("got %q, want sk-test-123", got)
	}

	if _, err := runSecretCommand("true"); err == nil {
		t.Error("expected error when command produces no output")
	}
	if _, err := runSecretCommand("exit 7"); err == nil {
		t.Error("expected error when command fails")
	}
}

func TestEffortMapping(t *testing.T) {
	cases := map[string]string{
		"low": "low", "medium": "medium", "high": "high",
		"xhigh": "xhigh", "max": "max", "": "high", "bogus": "high",
	}
	for in, want := range cases {
		if got := string(effort(in)); got != want {
			t.Errorf("effort(%q) = %q, want %q", in, got, want)
		}
	}
}
