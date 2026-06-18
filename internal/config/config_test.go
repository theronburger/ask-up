package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	d := Default()
	if d.Model != "claude-opus-4-8" {
		t.Errorf("default model = %q", d.Model)
	}
	if d.Effort != "high" {
		t.Errorf("default effort = %q", d.Effort)
	}
	if d.System == "" {
		t.Error("default system prompt is empty")
	}
}

func TestFloorFor(t *testing.T) {
	cases := map[string]int64{
		"claude-opus-4-8":    4096,
		"claude-haiku-4-5":   4096,
		"claude-sonnet-4-6":  2048,
		"claude-fable-5":     2048,
		"claude-sonnet-4-5":  1024,
		"some-unknown-model": 4096,
	}
	for model, want := range cases {
		if got := FloorFor(model); got != want {
			t.Errorf("FloorFor(%q) = %d, want %d", model, got, want)
		}
	}
}

func TestWarmthWindowDuration(t *testing.T) {
	if got := (Config{TTL: "5m"}).WarmthWindowDuration(); got != 4*time.Minute+50*time.Second {
		t.Errorf("5m window = %s, want 4m50s", got)
	}
	if got := (Config{TTL: "1h"}).WarmthWindowDuration(); got != time.Hour-10*time.Second {
		t.Errorf("1h window = %s, want 59m50s", got)
	}
	if got := (Config{TTL: "5m", WarmthWindow: "90s"}).WarmthWindowDuration(); got != 90*time.Second {
		t.Errorf("explicit window = %s, want 90s", got)
	}
}

func TestTTLDuration(t *testing.T) {
	if got := (Config{TTL: "1h"}).TTLDuration(); got != time.Hour {
		t.Errorf("1h = %s", got)
	}
	if got := (Config{TTL: "garbage"}).TTLDuration(); got != 5*time.Minute {
		t.Errorf("fallback = %s, want 5m", got)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	t.Setenv("ASK_UP_HOME", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "claude-opus-4-8" {
		t.Errorf("model = %q, want default", cfg.Model)
	}
}

func TestLoadOverlay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ASK_UP_HOME", home)
	body := "model = \"claude-fable-5\"\nttl = \"1h\"\n"
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "claude-fable-5" {
		t.Errorf("model = %q, want overlaid value", cfg.Model)
	}
	if cfg.TTL != "1h" {
		t.Errorf("ttl = %q, want 1h", cfg.TTL)
	}
	if cfg.Effort != "high" {
		t.Errorf("effort = %q, want default preserved", cfg.Effort)
	}
	if cfg.System == "" {
		t.Error("system prompt should fall back to default when unset")
	}
}
