// Command ask-up escalates a single question to a more capable ("up") model and
// prints the answer, persisting the exchange as a resumable consultation.
//
// Usage:
//
//	ask-up "question"                       quick one-liner
//	ask-up <<'EOF' ...curated brief... EOF  compose a fuller prompt on stdin
//	ask-up -continue cns_x "follow-up"      continue an existing consultation
//	ask-up -continue cns_x -force "..."     revive one past its cache window
//	ask-up -list                            list saved consultations
//	ask-up -v "question"                    also print token/cache usage
//
// The prompt comes from stdin when piped, otherwise from the positional
// arguments. Piping (a quoted heredoc) is the right path for anything with
// code, quotes, or newlines: it needs no shell escaping. Flags must precede
// the question (Go's flag package stops at the first non-flag argument).
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

// exitCodeColdCache is returned when the warmth guard blocks a -continue.
const exitCodeColdCache = 3

func main() {
	if err := run(os.Args[1:]); err != nil {
		var ce coldCacheError
		if errors.As(err, &ce) {
			fmt.Fprint(os.Stderr, ce.message)
			os.Exit(exitCodeColdCache)
		}
		fmt.Fprintln(os.Stderr, "ask-up: "+err.Error())
		os.Exit(1)
	}
}

// coldCacheError signals the warmth guard tripped; main maps it to a distinct
// exit code so callers can tell "cold, re-run with -force" from a real failure.
type coldCacheError struct{ message string }

func (e coldCacheError) Error() string { return "consultation cache is cold" }

func run(args []string) error {
	fs := flag.NewFlagSet("ask-up", flag.ContinueOnError)
	var (
		cont        = fs.String("continue", "", "continue an existing consultation by id")
		force       = fs.Bool("force", false, "revive a consultation even if its cache has likely expired")
		verbose     = fs.Bool("v", false, "print token and cache usage to stderr")
		list        = fs.Bool("list", false, "list saved consultations and exit")
		modelFlag   = fs.String("model", "", "override the configured target model (ignored with -continue)")
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

	st, err := store.New(config.Home())
	if err != nil {
		return err
	}

	if *list {
		return listCmd(st, cfg)
	}

	stdinData, err := readStdin()
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	question, err := resolveBody(stdinData, fs.Args())
	if err != nil {
		usage(fs)
		return err
	}

	c, err := resolveConsultation(st, cfg, *cont, *force, question)
	if err != nil {
		return err
	}

	res, err := consult.Ask(context.Background(), cfg, c.Messages, question)
	if err != nil {
		return err
	}

	c.Messages = append(c.Messages,
		store.Message{Role: "user", Text: question},
		store.Message{Role: "assistant", Text: res.Answer},
	)
	c.LastUsed = time.Now()
	c.PrefixTokens = res.PrefixTokens()
	if err := st.Save(c); err != nil {
		return fmt.Errorf("saving consultation: %w", err)
	}

	fmt.Println(res.Answer)
	printFooter(c, cfg)
	if *verbose {
		printUsage(c, cfg, res)
	}
	return nil
}

// readStdin returns piped/redirected stdin. It skips an interactive terminal so
// it never blocks waiting for input that isn't coming.
func readStdin() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", nil // can't stat; treat as no stdin rather than fail
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		return "", nil // interactive terminal, nothing piped
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

// resolveConsultation loads the consultation to continue (applying the warmth
// guard) or creates a fresh one.
func resolveConsultation(st *store.Store, cfg config.Config, cont string, force bool, question string) (*store.Consultation, error) {
	if cont == "" {
		return &store.Consultation{
			ID:        store.NewID(),
			Label:     store.Label(question),
			Model:     cfg.Model,
			CreatedAt: time.Now(),
		}, nil
	}

	c, err := st.Load(cont)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", cont, err)
	}

	w := c.Assess(time.Now(), config.FloorFor(c.Model), cfg.WarmthWindowDuration())
	if w.Cacheable && !w.Warm && !force {
		return nil, coldCacheError{message: warmthWarning(c, w)}
	}
	return c, nil
}
