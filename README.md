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

`ask-up` is password-manager-agnostic. It never stores a secret; it only needs the key at runtime, resolved in this order:

1. `ANTHROPIC_API_KEY` from the environment (standard path; works with enterprise keys)
2. `ANTHROPIC_AUTH_TOKEN` from the environment (for gateway/token auth)
3. `api_key_command` from the config file: any command whose stdout is the key

The security properties that matter do not depend on which vault you use: keep the secret in a manager, get it into the process at runtime, and never write it to a file. `ask-up`'s config holds only the *command*, never the key.

**Option A: export from your manager** in your shell profile. Any tool that prints the secret works:

```sh
export ANTHROPIC_API_KEY="$(op read 'op://Vault/Anthropic/credential')"   # 1Password
export ANTHROPIC_API_KEY="$(pass anthropic/api-key)"                       # pass
export ANTHROPIC_API_KEY="$(aws secretsmanager get-secret-value --secret-id anthropic --query SecretString --output text)"
export ANTHROPIC_API_KEY="$(vault kv get -field=key secret/anthropic)"     # HashiCorp Vault
export ANTHROPIC_API_KEY="$(doppler secrets get ANTHROPIC_API_KEY --plain)"
```

**Option B: let `ask-up` fetch it on demand** (no long-lived exported key). Put the fetch command in `~/.ask-up/config.toml`:

```toml
api_key_command = "op read 'op://Vault/Anthropic/credential'"
```

`ask-up` runs it per call and uses the output as the key. Latency is negligible next to the model call, and the secret only ever lives in the short-lived process's memory. The command runs in your shell's trust boundary; on failure `ask-up` reports it without echoing the command's stderr, so secrets do not leak into logs.

If you do not know which manager your org uses, you do not need to: pick whichever of A/B fits, point it at your org's tool, and the key never touches disk either way.

For an enterprise gateway, also set `base_url` in the config (below).

## Usage

The prompt comes from stdin when piped, otherwise from the positional argument.

```sh
# quick one-liner
ask-up "what's the idiomatic Go way to cancel a fan-out on first error?"

# a curated brief via a quoted heredoc: multi-line, code-safe, no shell escaping
ask-up <<'EOF'
We deadlock under reentrancy in the lock manager. Relevant snippet from lock.go:

    mu.Lock()
    defer mu.Unlock()
    inner.Lock()   // acquired while still holding mu

Question: is taking `inner` while holding `mu` safe here, or do we need a tryLock path?
EOF

ask-up -continue cns_1a2b3c4d "what about across processes?"   # continue a thread
ask-up -continue cns_1a2b3c4d -force "..."                     # revive past its cache window
ask-up -list                                                   # list saved consultations
ask-up -v "question"                                           # also print token/cache usage
```

Flags must come before the prompt (Go's flag parser stops at the first non-flag argument). The answer goes to stdout; the pointer line (how to continue, how long the cache stays warm) and any usage detail go to stderr, so the answer pipes cleanly.

### Composing a good consultation

The upstream model gets one shot and cannot see your codebase. The quality of its answer is set entirely by how you frame the call, so do the work it cannot:

- **Curate, don't dump.** Pull the few relevant snippets, not whole files. A tight brief beats a pile of context.
- **Summarize the situation** and say what you have already tried or ruled out.
- **Ask one specific, decidable question.**
- Pipe it via a quoted heredoc (`<<'EOF'`) so code, quotes, and backticks pass through untouched.

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
# system          = "..."          # override the consult instruction
# api_key_command = "..."          # fetch the key at runtime (see Authentication)
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
