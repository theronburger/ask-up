// Package consult wraps a single escalation call to the upstream model. It
// rebuilds the consultation history into a request, sets a cache breakpoint on
// the stable prefix, and returns the answer plus token usage.
package consult

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/theronburger/ask-up/internal/config"
	"github.com/theronburger/ask-up/internal/store"
)

// Result is the outcome of one escalation.
type Result struct {
	Answer                   string
	InputTokens              int64
	OutputTokens             int64
	CacheReadInputTokens     int64
	CacheCreationInputTokens int64
}

// PrefixTokens is the total prompt size: fresh input plus everything served
// from or written to cache. This is what we compare against the cache floor.
func (r Result) PrefixTokens() int64 {
	return r.InputTokens + r.CacheReadInputTokens + r.CacheCreationInputTokens
}

// Ask sends history plus the new question to the configured model and returns
// the assistant's reply. The API key is resolved by the SDK from
// ANTHROPIC_API_KEY (or ANTHROPIC_AUTH_TOKEN, when set).
func Ask(ctx context.Context, cfg config.Config, history []store.Message, question string) (Result, error) {
	client := newClient(cfg)

	cc := anthropic.NewCacheControlEphemeralParam()
	if cfg.TTLDuration() == time.Hour {
		cc.TTL = anthropic.CacheControlEphemeralTTLTTL1h
	}

	system := []anthropic.TextBlockParam{{Text: cfg.System, CacheControl: cc}}

	msgs := make([]anthropic.MessageParam, 0, len(history)+1)
	for _, m := range history {
		block := anthropic.NewTextBlock(m.Text)
		if m.Role == "assistant" {
			msgs = append(msgs, anthropic.NewAssistantMessage(block))
		} else {
			msgs = append(msgs, anthropic.NewUserMessage(block))
		}
	}
	// Cache breakpoint on the newest turn: the whole prefix up to here becomes
	// readable on the next continue.
	q := anthropic.NewTextBlock(question)
	q.OfText.CacheControl = cc
	msgs = append(msgs, anthropic.NewUserMessage(q))

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:        cfg.Model,
		MaxTokens:    cfg.MaxTokens,
		System:       system,
		Messages:     msgs,
		Thinking:     anthropic.ThinkingConfigParamUnion{OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{}},
		OutputConfig: anthropic.OutputConfigParam{Effort: effort(cfg.Effort)},
	})
	if err != nil {
		return Result{}, fmt.Errorf("calling %s: %w", cfg.Model, err)
	}

	var sb strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}

	return Result{
		Answer:                   strings.TrimSpace(sb.String()),
		InputTokens:              resp.Usage.InputTokens,
		OutputTokens:             resp.Usage.OutputTokens,
		CacheReadInputTokens:     resp.Usage.CacheReadInputTokens,
		CacheCreationInputTokens: resp.Usage.CacheCreationInputTokens,
	}, nil
}

func newClient(cfg config.Config) anthropic.Client {
	var opts []option.RequestOption
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	// SDK reads ANTHROPIC_API_KEY automatically; wire an auth token explicitly
	// if that's the enterprise credential in use.
	if tok := os.Getenv("ANTHROPIC_AUTH_TOKEN"); tok != "" {
		opts = append(opts, option.WithAuthToken(tok))
	}
	return anthropic.NewClient(opts...)
}

func effort(s string) anthropic.OutputConfigEffort {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return anthropic.OutputConfigEffortLow
	case "medium":
		return anthropic.OutputConfigEffortMedium
	case "xhigh":
		return anthropic.OutputConfigEffortXhigh
	case "max":
		return anthropic.OutputConfigEffortMax
	default:
		return anthropic.OutputConfigEffortHigh
	}
}
