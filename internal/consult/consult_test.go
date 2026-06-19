package consult

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theronburger/ask-up/internal/config"
)

// fakeClaude writes a script that records its args and stdin, then prints the
// given JSON, standing in for the real `claude` binary.
func fakeClaude(t *testing.T, replyJSON string) (bin, argsFile, stdinFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	stdinFile = filepath.Join(dir, "stdin")
	bin = filepath.Join(dir, "claude")
	script := "#!/bin/sh\n" +
		"for a in \"$@\"; do printf '%s\\n' \"$a\"; done > " + argsFile + "\n" +
		"cat > " + stdinFile + "\n" +
		"cat <<'JSON'\n" + replyJSON + "\nJSON\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, argsFile, stdinFile
}

func TestAsk(t *testing.T) {
	reply := `{"result":"use a tryLock","session_id":"sess-123","is_error":false,"usage":{"input_tokens":304,"output_tokens":12,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`
	bin, argsFile, stdinFile := fakeClaude(t, reply)

	cfg := config.Default()
	cfg.ClaudeBin = bin

	res, err := Ask(context.Background(), cfg, "", "is this lock ordering deadlock-safe?")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if res.Answer != "use a tryLock" {
		t.Errorf("answer = %q", res.Answer)
	}
	if res.SessionID != "sess-123" {
		t.Errorf("session id = %q", res.SessionID)
	}
	if res.InputTokens != 304 || res.OutputTokens != 12 {
		t.Errorf("usage parsed wrong: %+v", res)
	}

	args, _ := os.ReadFile(argsFile)
	for _, want := range []string{"-p", "--model", "opus", "--effort", "xhigh", "--output-format", "json", "--strict-mcp-config", "--tools", "--exclude-dynamic-system-prompt-sections", "--system-prompt"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("args missing %q; got:\n%s", want, args)
		}
	}
	if strings.Contains(string(args), "--resume") {
		t.Error("new consultation should not pass --resume")
	}
	stdin, _ := os.ReadFile(stdinFile)
	if strings.TrimSpace(string(stdin)) != "is this lock ordering deadlock-safe?" {
		t.Errorf("prompt not passed on stdin; got %q", stdin)
	}
}

func TestAskResume(t *testing.T) {
	reply := `{"result":"yes, across processes too","session_id":"sess-123","is_error":false,"usage":{"input_tokens":50,"output_tokens":8}}`
	bin, argsFile, _ := fakeClaude(t, reply)
	cfg := config.Default()
	cfg.ClaudeBin = bin

	if _, err := Ask(context.Background(), cfg, "sess-123", "and across processes?"); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	args, _ := os.ReadFile(argsFile)
	if !strings.Contains(string(args), "--resume") || !strings.Contains(string(args), "sess-123") {
		t.Errorf("resume should pass --resume sess-123; got:\n%s", args)
	}
}

func TestAskReportsError(t *testing.T) {
	reply := `{"result":"login required","session_id":"","is_error":true,"subtype":"auth_error"}`
	bin, _, _ := fakeClaude(t, reply)
	cfg := config.Default()
	cfg.ClaudeBin = bin

	if _, err := Ask(context.Background(), cfg, "", "anything"); err == nil {
		t.Error("expected error when claude reports is_error")
	}
}
