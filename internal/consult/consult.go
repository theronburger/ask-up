// Package consult runs one advisor turn by invoking the local Claude Code CLI in
// print mode. It strips the agent harness (tools, MCP servers, dynamic context)
// so the call carries only our system prompt and the question, and uses Claude
// Code's own session for continuity via --resume.
package consult

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/theronburger/ask-up/internal/config"
)

// Result is the outcome of one advisor turn.
type Result struct {
	Answer                   string
	SessionID                string
	InputTokens              int64
	OutputTokens             int64
	CacheReadInputTokens     int64
	CacheCreationInputTokens int64
}

// PrefixTokens is the total prompt size for this turn.
func (r Result) PrefixTokens() int64 {
	return r.InputTokens + r.CacheReadInputTokens + r.CacheCreationInputTokens
}

// claudeOutput is the subset of `claude -p --output-format json` we read.
type claudeOutput struct {
	Result    string `json:"result"`
	SessionID string `json:"session_id"`
	IsError   bool   `json:"is_error"`
	Subtype   string `json:"subtype"`
	Usage     struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

// Ask sends prompt to the advisor. A non-empty sessionID resumes that Claude
// Code session; otherwise a fresh one is started. The prompt is passed on stdin
// so curated, multi-line briefs need no shell escaping.
func Ask(ctx context.Context, cfg config.Config, sessionID, prompt string) (Result, error) {
	cmd := exec.CommandContext(ctx, cfg.ClaudeBin, buildArgs(cfg, sessionID)...)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = childEnv(cfg)
	// Run from a neutral directory so no project CLAUDE.md leaks into context.
	cmd.Dir = os.TempDir()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return Result{}, fmt.Errorf("running %s: %w: %s", cfg.ClaudeBin, err, msg)
	}

	var out claudeOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return Result{}, fmt.Errorf("parsing claude output: %w", err)
	}
	if out.IsError {
		return Result{}, fmt.Errorf("claude reported an error (%s): %s", out.Subtype, out.Result)
	}

	return Result{
		Answer:                   strings.TrimSpace(out.Result),
		SessionID:                out.SessionID,
		InputTokens:              out.Usage.InputTokens,
		OutputTokens:             out.Usage.OutputTokens,
		CacheReadInputTokens:     out.Usage.CacheReadInputTokens,
		CacheCreationInputTokens: out.Usage.CacheCreationInputTokens,
	}, nil
}

// buildArgs constructs the lean print-mode invocation. `--tools` is given no
// values (it is immediately followed by another flag), which drops every tool
// schema; combined with --strict-mcp-config and a replaced system prompt this
// removes the agent harness, leaving just the persona and the question.
func buildArgs(cfg config.Config, sessionID string) []string {
	args := []string{
		"-p",
		"--model", cfg.Model,
		"--output-format", "json",
		"--strict-mcp-config",
	}
	if cfg.Effort != "" {
		// Set effort explicitly so it doesn't inherit the caller's CLAUDE_EFFORT.
		args = append(args, "--effort", cfg.Effort)
	}
	// `--tools` is given no values (immediately followed by another flag), which
	// drops every tool schema. Keep it directly before --system-prompt.
	args = append(args,
		"--tools",
		"--exclude-dynamic-system-prompt-sections",
		"--system-prompt", cfg.System,
	)
	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}
	return args
}

// childEnv selects the Claude Code account for the child via CLAUDE_CONFIG_DIR
// when configured, inheriting the rest of the environment.
func childEnv(cfg config.Config) []string {
	env := os.Environ()
	if cfg.ConfigDir != "" {
		env = append(env, "CLAUDE_CONFIG_DIR="+cfg.ConfigDir)
	}
	return env
}
