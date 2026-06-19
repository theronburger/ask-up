package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/theronburger/ask-up/internal/config"
	"github.com/theronburger/ask-up/internal/consult"
	"github.com/theronburger/ask-up/internal/store"
)

// usage prints help text. The question goes to stdout; everything advisory
// (pointers, usage, warnings) goes to stderr so piping `ask-up "..."` yields a
// clean answer.
func usage(fs *flag.FlagSet) {
	fmt.Fprint(os.Stderr, `ask-up: escalate a question to a more capable model

  ask-up "question"                     quick one-liner
  ask-up <<'EOF' ...brief... EOF        compose a curated prompt on stdin
  ask-up -continue cns_x "follow-up"    continue an existing consultation
  ask-up -continue cns_x -force "..."   revive one past its cache window
  ask-up -list                          list saved consultations
  ask-up -v "question"                  also print token/cache usage

The prompt comes from stdin when piped, else from the arguments. Pipe a quoted
heredoc for anything with code, quotes, or newlines. Flags come before the prompt.

Flags:
`)
	fs.PrintDefaults()
}

// printFooter is the pointer/padding written after the answer: how to continue,
// and how long the cache is assumed warm.
func printFooter(c *store.Consultation, cfg config.Config) {
	window := cfg.WarmthWindowDuration()
	floor := config.FloorFor(c.Model)
	if c.PrefixTokens >= floor {
		fmt.Fprintf(os.Stderr, "\n→ %s · continue: ask-up -continue %s \"…\" · warm ~%s\n",
			c.ID, c.ID, window.Round(time.Second))
	} else {
		fmt.Fprintf(os.Stderr, "\n→ %s · continue: ask-up -continue %s \"…\" · reuse on relevance (prefix below cache floor)\n",
			c.ID, c.ID)
	}
}

// printUsage shows the token breakdown and whether the prefix is cacheable.
func printUsage(c *store.Consultation, _ config.Config, res consult.Result) {
	floor := config.FloorFor(c.Model)
	note := fmt.Sprintf("cacheable (≥ %d floor)", floor)
	if res.PrefixTokens() < floor {
		note = fmt.Sprintf("not cacheable (prefix %d < %d floor)", res.PrefixTokens(), floor)
	}
	fmt.Fprintf(os.Stderr, "usage: input=%d cache_read=%d cache_write=%d output=%d · prefix %d tok · %s\n",
		res.InputTokens, res.CacheReadInputTokens, res.CacheCreationInputTokens,
		res.OutputTokens, res.PrefixTokens(), note)
}

// warmthWarning is shown when -continue targets a consultation whose cache has
// likely expired. It explains the cost and the escape hatch.
func warmthWarning(c *store.Consultation, w store.Warmth) string {
	return fmt.Sprintf(
		"ask-up: %s last used %s ago; its cache has likely expired.\n"+
			"Continuing re-bills the full history (~%d tokens) at standard input price.\n"+
			"Re-run with -force to revive it, or omit -continue to start fresh.\n",
		c.ID, w.Age.Round(time.Second), c.PrefixTokens)
}

// listCmd prints saved consultations, most recent first.
func listCmd(st *store.Store, cfg config.Config) error {
	cs, err := st.List()
	if err != nil {
		return err
	}
	if len(cs) == 0 {
		fmt.Fprintln(os.Stderr, "no consultations yet")
		return nil
	}
	now := time.Now()
	window := cfg.WarmthWindowDuration()
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	// Intermediate writes buffer into the tabwriter; the real error surfaces on Flush.
	_, _ = fmt.Fprintln(tw, "ID\tAGE\tSTATE\tMODEL\tLABEL")
	for _, c := range cs {
		w := c.Assess(now, config.FloorFor(c.Model), window)
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			c.ID, w.Age.Round(time.Second), state(w), c.Model, c.Label)
	}
	return tw.Flush()
}

func state(w store.Warmth) string {
	switch {
	case !w.Cacheable:
		return "uncached"
	case w.Warm:
		return "warm"
	default:
		return "cold"
	}
}
