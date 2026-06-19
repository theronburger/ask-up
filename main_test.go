package main

import "testing"

func TestResolveBody(t *testing.T) {
	if got, _ := resolveBody("  piped brief  ", []string{"arg", "question"}); got != "piped brief" {
		t.Errorf("stdin should win: got %q", got)
	}
	if got, _ := resolveBody("", []string{"why", "deadlock"}); got != "why deadlock" {
		t.Errorf("positional fallback: got %q", got)
	}
	if got, _ := resolveBody("   \n\t ", []string{"fallback"}); got != "fallback" {
		t.Errorf("blank stdin should fall back to args: got %q", got)
	}
	if _, err := resolveBody("", nil); err == nil {
		t.Error("expected error when no prompt is provided")
	}
}
