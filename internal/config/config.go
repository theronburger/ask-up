// Package config loads ask-up settings from ~/.ask-up/config.toml (overridable
// via ASK_UP_HOME), layered on top of sensible defaults. Secrets never live
// here: the API key is read from the environment, or fetched at runtime via the
// configured api_key_command (the config stores the command, not the secret).
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// DefaultSystem is the instruction given to the upstream model. It frames the
// call as a peer escalation and asks for a committed recommendation rather than
// an exhaustive survey.
const DefaultSystem = `You are a senior engineer being consulted by another AI agent that is mid-task and has hit a point of genuine uncertainty. Answer the specific question directly and precisely. Commit to a recommendation rather than listing every option; if you must weigh alternatives, say which you would pick and why. If the question is underspecified, state the single most reasonable assumption and answer under it rather than asking for clarification. Lead with the answer, then the reasoning. Be concise.`

// Config holds non-secret settings. Field names map to TOML keys.
type Config struct {
	Model         string `toml:"model"`           // target ("up") model id
	Effort        string `toml:"effort"`          // low|medium|high|xhigh|max
	TTL           string `toml:"ttl"`             // "5m" or "1h": cache breakpoint TTL
	WarmthWindow  string `toml:"warmth_window"`   // duration; empty => derived from TTL
	BaseURL       string `toml:"base_url"`        // optional enterprise gateway
	System        string `toml:"system"`          // override the consult system prompt
	MaxTokens     int64  `toml:"max_tokens"`      // response cap
	APIKeyCommand string `toml:"api_key_command"` // optional: shell command whose stdout is the API key
}

// Default returns the baseline configuration used when no file is present and
// for any field a config file leaves unset.
func Default() Config {
	return Config{
		Model:     "claude-opus-4-8",
		Effort:    "high",
		TTL:       "5m",
		MaxTokens: 8192,
		System:    DefaultSystem,
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
	return cfg, nil
}

// TTLDuration parses the configured TTL ("5m"/"1h"). Unknown values fall back
// to 5 minutes, matching the API-key default.
func (c Config) TTLDuration() time.Duration {
	switch strings.ToLower(strings.TrimSpace(c.TTL)) {
	case "1h":
		return time.Hour
	default:
		return 5 * time.Minute
	}
}

// WarmthWindowDuration is how long after last use a consultation is still
// assumed cache-warm. An explicit warmth_window wins; otherwise it's the TTL
// minus a 10s safety buffer, so we declare "cold" before the real expiry.
func (c Config) WarmthWindowDuration() time.Duration {
	if c.WarmthWindow != "" {
		if d, err := time.ParseDuration(c.WarmthWindow); err == nil {
			return d
		}
	}
	if w := c.TTLDuration() - 10*time.Second; w > 0 {
		return w
	}
	return c.TTLDuration()
}

// FloorFor returns the minimum prefix size (in tokens) at which prompt caching
// engages for a model. Below it the cache_control marker is silently ignored,
// so the warmth guard does not apply. Unknown models get the conservative
// 4096-token Opus-tier floor.
func FloorFor(model string) int64 {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "sonnet-4-6"), strings.Contains(m, "fable-5"),
		strings.Contains(m, "mythos-5"), strings.Contains(m, "haiku-3"):
		return 2048
	case strings.Contains(m, "sonnet-4-5"), strings.Contains(m, "sonnet-4-0"),
		strings.Contains(m, "sonnet-3-7"):
		return 1024
	default:
		return 4096
	}
}
