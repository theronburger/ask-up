package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/theronburger/ask-up/internal/consult"
	"github.com/theronburger/ask-up/internal/store"
)

// usage prints help. The answer goes to stdout; everything advisory (the
// continue pointer, usage) goes to stderr so piping `ask-up "..."` is clean.
func usage(fs *flag.FlagSet) {
	fmt.Fprint(os.Stderr, `ask-up: escalate a question to a more capable model (via the Claude Code CLI)

  ask-up "question"                     quick one-liner
  ask-up <<'EOF' ...brief... EOF        compose a curated prompt on stdin
  ask-up -continue cns_x "follow-up"    continue an existing consultation
  ask-up -list                          list saved consultations
  ask-up -v "question"                  also print token/session info

The prompt comes from stdin when piped, else from the arguments. Pipe a quoted
heredoc for anything with code, quotes, or newlines. Flags come before the prompt.

Flags:
`)
	fs.PrintDefaults()
}

// printFooter is the pointer written after the answer: how to continue.
func printFooter(c *store.Consultation) {
	fmt.Fprintf(os.Stderr, "\n→ %s · continue: ask-up -continue %s \"…\"\n", c.ID, c.ID)
}

// printUsage shows the token breakdown and the underlying session id.
func printUsage(c *store.Consultation, res consult.Result) {
	fmt.Fprintf(os.Stderr, "usage: input=%d cache_read=%d cache_write=%d output=%d · prefix %d tok · session %s\n",
		res.InputTokens, res.CacheReadInputTokens, res.CacheCreationInputTokens,
		res.OutputTokens, res.PrefixTokens(), c.SessionID)
}

// listCmd prints saved consultations, most recent first.
func listCmd(st *store.Store) error {
	cs, err := st.List()
	if err != nil {
		return err
	}
	if len(cs) == 0 {
		fmt.Fprintln(os.Stderr, "no consultations yet")
		return nil
	}
	now := time.Now()
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tAGE\tMODEL\tLABEL")
	for _, c := range cs {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			c.ID, now.Sub(c.LastUsed).Round(time.Second), c.Model, c.Label)
	}
	return tw.Flush()
}
