# ask-up

A quick word with a smarter model.

`ask-up` is a command-line tool that lets a coding agent escalate a single hard question *up* to a more capable model, get a direct second opinion, and carry on. The agent stays on its fast model for the main loop and only reaches up when it is genuinely uncertain. "Up" is a direction, not a product: the target model is configurable and defaults to Claude Opus 4.8.

It is built for agent harnesses like Claude Code, where the core agent runs on Sonnet and calls `ask-up` through Bash. There is no server and no daemon. Each call is a short-lived process that makes one API call and exits. State lives in plain files under `~/.ask-up`.

## Why a separate tool, not a model swap

Switching the main agent from Sonnet to Opus mid-session throws away the prompt cache (caches are per-model) and is expensive for every turn. `ask-up` keeps the escalation out of band: the main loop stays on one model, and only the targeted question goes up. Each consultation is its own small thread you can continue.

## Install

Prebuilt binaries are on the [Releases](https://github.com/theronburger/ask-up/releases) page. Download the archive for your platform, extract `ask-up`, and put it on your `PATH`.

Or build from source (Go 1.24+):

```sh
go install github.com/theronburger/ask-up@latest
```

## Authentication

`ask-up` reads the key from the environment; it never writes secrets to disk. Set one of:

- `ANTHROPIC_API_KEY` (the standard path; works with enterprise keys)
- `ANTHROPIC_AUTH_TOKEN` (used automatically when set)

Per house policy, source the key from 1Password rather than hardcoding it, for example in your shell profile:

```sh
export ANTHROPIC_API_KEY="$(op read 'op://Private/Anthropic API/credential')"
```

For an enterprise gateway, set `base_url` in the config file (below).

## Usage

```sh
ask-up "is this lock ordering deadlock-safe under reentrancy?"   # new consultation
ask-up -continue cns_1a2b3c4d "what about across processes?"     # continue it
ask-up -continue cns_1a2b3c4d -force "..."                       # revive past its cache window
ask-up -list                                                     # list saved consultations
ask-up -v "question"                                             # also print token/cache usage
```

Flags must come before the question (Go's flag parser stops at the first non-flag argument).

The answer is printed to stdout. The pointer line (how to continue, how long the cache stays warm) and any usage detail go to stderr, so `ask-up "..."` pipes a clean answer.

## How reuse and the cache work

Each consultation is one Opus thread. Continuing it re-sends the history, so the prompt cache can serve the repeated prefix at roughly a tenth of the input price. Two facts shape the behavior:

- **Cache TTL.** With an API key the breakpoint TTL is 5 minutes by default (set `ttl = "1h"` to keep threads warm across longer gaps, at a higher write cost). There is no way to query remaining time, so `ask-up` estimates warmth from when you last used a consultation and declares it "cold" a few seconds before the real expiry.
- **The cache floor.** Prompt caching only engages once the prefix reaches the model's minimum (4096 tokens on Opus 4.8, 2048 on Sonnet 4.6 / Fable 5). Below that the cache marker is ignored, so there is nothing to be warm or cold about.

The warmth guard follows from these: `-continue` on a consultation that is large enough to cache **and** likely cold will refuse and warn, rather than silently re-billing the whole history at full price. Re-run with `-force` to revive it, or omit `-continue` to start fresh. Consultations below the floor are reused on relevance alone; the guard does not apply.

Use `ask-up -v` to see the real outcome: `cache_read` greater than zero means you hit a warm cache.

## Configuration

Optional file at `~/.ask-up/config.toml` (override the directory with `ASK_UP_HOME`). Any key you omit falls back to the default.

```toml
model         = "claude-opus-4-8"  # the "up" model
effort        = "high"             # low | medium | high | xhigh | max
ttl           = "5m"               # cache breakpoint TTL: "5m" or "1h"
warmth_window = ""                 # duration override; default is ttl minus 10s
base_url      = ""                 # optional enterprise gateway
max_tokens    = 8192               # response cap
# system      = "..."              # override the consult instruction
```

## Claude Code integration

`ask-up` is meant to be called by your agent. To wire it into Claude Code (allowlist the command and teach the agent when to use it), see [SETUP.md](./SETUP.md), which is written so a coding agent can do the setup itself, including across multiple `~/.claude*` profiles.

## Development

```sh
go build ./...
go test -race -cover ./...
go vet ./...
golangci-lint run        # optional locally; enforced in CI
lefthook install         # enable the pre-commit / pre-push hooks
```

Releases are cut by tagging: `git tag v0.1.0 && git push --tags` runs goreleaser and publishes binaries to the Releases page.

## License

MIT. See [LICENSE](./LICENSE).
