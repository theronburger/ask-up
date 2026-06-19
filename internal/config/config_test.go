package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	d := Default()
	if d.Model != "opus" {
		t.Errorf("default model = %q", d.Model)
	}
	if d.ClaudeBin != "claude" {
		t.Errorf("default claude_bin = %q", d.ClaudeBin)
	}
	if d.Effort != "xhigh" {
		t.Errorf("default effort = %q, want xhigh", d.Effort)
	}
	if d.System == "" {
		t.Error("default system prompt is empty")
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	t.Setenv("ASK_UP_HOME", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "opus" || cfg.ClaudeBin != "claude" || cfg.System == "" {
		t.Errorf("missing-file load did not return defaults: %+v", cfg)
	}
}

func TestLoadOverlay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ASK_UP_HOME", home)
	body := "model = \"claude-opus-4-8\"\nconfig_dir = \"/Users/x/.claude-work\"\nclaude_bin = \"/opt/claude\"\n"
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "claude-opus-4-8" {
		t.Errorf("model = %q, want overlaid value", cfg.Model)
	}
	if cfg.ConfigDir != "/Users/x/.claude-work" {
		t.Errorf("config_dir = %q", cfg.ConfigDir)
	}
	if cfg.ClaudeBin != "/opt/claude" {
		t.Errorf("claude_bin = %q", cfg.ClaudeBin)
	}
	if cfg.System == "" {
		t.Error("system prompt should fall back to default when unset")
	}
}
