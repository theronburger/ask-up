// Command ask-up escalates a question to a more capable model by driving the
// local Claude Code CLI in print mode, and prints the answer. It persists each
// exchange as a resumable consultation (a Claude Code session).
//
// Usage:
//
//	ask-up "question"                       quick one-liner
//	ask-up <<'EOF' ...curated brief... EOF  compose a fuller prompt on stdin
//	ask-up -continue cns_x "follow-up"      continue an existing consultation
//	ask-up -list                            list saved consultations
//	ask-up -v "question"                    also print token/session info
//
// The prompt comes from stdin when piped, otherwise from the positional
// arguments. Flags must precede the question.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/theronburger/ask-up/internal/config"
	"github.com/theronburger/ask-up/internal/consult"
	"github.com/theronburger/ask-up/internal/store"
)

// Build metadata, injected by goreleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ask-up: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("ask-up", flag.ContinueOnError)
	var (
		cont        = fs.String("continue", "", "continue an existing consultation by id")
		verbose     = fs.Bool("v", false, "print token usage and session id to stderr")
		list        = fs.Bool("list", false, "list saved consultations and exit")
		modelFlag   = fs.String("model", "", "override the configured model (ignored with -continue)")
		effortFlag  = fs.String("effort", "", "reasoning effort: low|medium|high|xhigh|max (default from config)")
		showVersion = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() { usage(fs) }
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *showVersion {
		fmt.Printf("ask-up %s (commit %s, built %s)\n", version, commit, date)
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if *modelFlag != "" {
		cfg.Model = *modelFlag
	}
	if *effortFlag != "" {
		cfg.Effort = *effortFlag
	}

	st, err := store.New(config.Home())
	if err != nil {
		return err
	}

	if *list {
		return listCmd(st)
	}

	stdinData, err := readStdin()
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	prompt, err := resolveBody(stdinData, fs.Args())
	if err != nil {
		usage(fs)
		return err
	}

	c, resumeID := resolveConsultation(st, &cfg, *cont, prompt)
	if c == nil {
		return fmt.Errorf("loading %s: not found", *cont)
	}

	res, err := consult.Ask(context.Background(), cfg, resumeID, prompt)
	if err != nil {
		return err
	}

	if res.SessionID != "" {
		c.SessionID = res.SessionID
	}
	c.LastUsed = time.Now()
	if err := st.Save(c); err != nil {
		return fmt.Errorf("saving consultation: %w", err)
	}

	fmt.Println(res.Answer)
	printFooter(c)
	if *verbose {
		printUsage(c, res)
	}
	return nil
}

// resolveConsultation loads the consultation to continue (resuming its Claude
// Code session) or creates a fresh one. Returns the consultation and the
// session id to resume (empty for a new consultation). A nil consultation means
// the requested -continue id was not found.
func resolveConsultation(st *store.Store, cfg *config.Config, cont, prompt string) (*store.Consultation, string) {
	if cont == "" {
		return &store.Consultation{
			ID:        store.NewID(),
			Label:     store.Label(prompt),
			Model:     cfg.Model,
			CreatedAt: time.Now(),
		}, ""
	}
	c, err := st.Load(cont)
	if err != nil {
		return nil, ""
	}
	cfg.Model = c.Model // keep the resumed session on its original model
	return c, c.SessionID
}

// readStdin returns piped/redirected stdin, skipping an interactive terminal so
// it never blocks waiting for input that isn't coming.
func readStdin() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", nil
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		return "", nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// resolveBody picks the prompt: piped stdin wins (the path for curated,
// multi-line briefs), otherwise the positional arguments.
func resolveBody(stdinData string, args []string) (string, error) {
	if s := strings.TrimSpace(stdinData); s != "" {
		return s, nil
	}
	if q := strings.TrimSpace(strings.Join(args, " ")); q != "" {
		return q, nil
	}
	return "", errors.New("no prompt provided (pipe one via stdin or pass it as an argument)")
}
