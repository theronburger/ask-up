// Package config loads ask-up settings from ~/.ask-up/config.toml (overridable
// via ASK_UP_HOME), layered on defaults. ask-up wraps the local Claude Code CLI
// in print mode, so there are no API keys or endpoints here: authentication is
// whatever the chosen Claude Code profile already uses.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// DefaultSystem is the advisor persona handed to the upstream model. It frames
// the call as a one-shot consultation from another Claude agent.
const DefaultSystem = `You are a senior advisor being consulted by another Claude agent that is working on a task and has escalated a question because it hit something it is unsure about. You are the more capable model; give it a clear, correct, decisive answer.

Be concise but complete: cover what matters and cut what doesn't. Do not assume an ongoing conversation and do not end by inviting follow-ups; aim to fully resolve the question in this one response. Only ask a clarifying question if the question genuinely cannot be answered without it; otherwise state the assumption you are making and answer under it. Lead with the answer, then the essential reasoning, and commit to a recommendation rather than listing every option.`

// Config holds the (few) settings. Field names map to TOML keys.
type Config struct {
	Model     string `toml:"model"`      // Claude Code model alias/id (default "opus")
	Effort    string `toml:"effort"`     // reasoning effort: low|medium|high|xhigh|max (default "xhigh")
	System    string `toml:"system"`     // advisor system prompt
	ClaudeBin string `toml:"claude_bin"` // path to the claude binary (default "claude" on PATH)
}

// Default returns the baseline configuration. Effort defaults to xhigh: a notch
// above the standard "high", since ask-up is for the hard, escalated calls.
func Default() Config {
	return Config{
		Model:     "opus",
		Effort:    "xhigh",
		System:    DefaultSystem,
		ClaudeBin: "claude",
	}
}

// Home returns the ask-up state directory: $ASK_UP_HOME, or ~/.ask-up.
func Home() string {
	if h := os.Getenv("ASK_UP_HOME"); h != "" {
		return h
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return ".ask-up"
	}
	return filepath.Join(dir, ".ask-up")
}

// Load reads Home()/config.toml over the defaults. A missing file is not an
// error; defaults are returned. Only keys present in the file override.
func Load() (Config, error) {
	cfg := Default()
	path := filepath.Join(Home(), "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.System == "" {
		cfg.System = DefaultSystem
	}
	if cfg.Model == "" {
		cfg.Model = "opus"
	}
	if cfg.Effort == "" {
		cfg.Effort = "xhigh"
	}
	if cfg.ClaudeBin == "" {
		cfg.ClaudeBin = "claude"
	}
	return cfg, nil
}
