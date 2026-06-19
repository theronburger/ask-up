<p align="center">
  <img src="assets/logo.png?v=3" alt="ask-up logo" width="300">
</p>

# ask-up

A quick word with a bigger model.

`ask-up` lets a coding agent running a smaller model escalate a single hard question *up* to a larger thinking model, get a direct second opinion, and carry on. The agent stays on its fast model for the main loop and only reaches up when it is genuinely uncertain.

It wraps the **local Claude Code CLI** in print mode, so it runs against whatever your Claude Code profile is already logged into. Each call is a short-lived process that asks once and exits; consultation state is a small file under `~/.ask-up`.

## Set up (have your agent do it)

Paste this into your coding agent (Claude Code, etc.):

```text
Please install and set up the `ask-up` CLI on my machine and wire it into, well, yourself. Read and follow the setup guide at:
https://raw.githubusercontent.com/theronburger/ask-up/refs/heads/main/SETUP.md
```

Prefer to do it by hand? See [Manual install](#manual-install) and [Configuration](#configuration).

## How it works

`ask-up` shells out to `claude -p` (print mode) with the agent harness stripped: no tools, no MCP servers, no project context. What's left is just an advisor system prompt plus your question, so a call costs roughly its own tokens (a few hundred) rather than the ~25k a default `claude -p` would carry. It authenticates however your Claude Code profile does, so there's nothing to provision. Continuing a consultation resumes the same Claude Code session, so the advisor keeps prior context.

Because it leans on Claude Code, the advisor is the more capable model run as a plain reasoner, not a second coding agent crawling your repo. The lower agent arrives with context and a question; the advisor brings reasoning and an answer.

## Manual install

Prebuilt binaries are on the [Releases](https://github.com/theronburger/ask-up/releases) page. Download the archive for your platform, extract `ask-up`, and put it on your `PATH`.

Or build from source (Go 1.24+):

```sh
go install github.com/theronburger/ask-up@latest
```

(`gh release download <tag> --repo theronburger/ask-up` also fetches a prebuilt binary.)

You also need Claude Code installed and logged in (`claude` on your `PATH`, a subscription that includes `opus`).

## Configuration

Optional file at `~/.ask-up/config.toml` (override the directory with `ASK_UP_HOME`). Any key you omit falls back to the default.

```toml
model      = "opus"     # Claude Code model alias or id (e.g. "opus", "claude-opus-4-8")
effort     = "xhigh"    # reasoning effort: low | medium | high | xhigh | max
claude_bin = "claude"   # path to the claude binary; set absolute only if claude isn't on PATH
# system   = "..."      # override the advisor system prompt
```

**Account.** `ask-up` runs under whatever account the calling Claude Code session uses: it inherits `CLAUDE_CONFIG_DIR` and doesn't switch accounts. A work session consults work, a personal one consults personal. If you run multiple accounts, you control which answers by which session you call from (see [Multiple accounts](#multiple-accounts)).

**Effort.** Defaults to `xhigh` (a notch above the standard `high`, since this is for hard calls); override per-call with `-effort low|medium|high|xhigh|max`. Setting it explicitly means the advisor's effort is deliberate rather than inherited from the caller.

**Binary.** Plain `claude` is resolved on `PATH`, and Go's exec skips shell aliases/functions, so a profile wrapper doesn't interfere. Set `claude_bin` to an absolute path only if `claude` isn't on `PATH` (or `PATH`'s `claude` is itself a wrapper script).

## Multiple accounts

If you keep separate Claude Code profiles for, say, work and personal use, `ask-up` should already route correctly: it inherits the account of whichever profile calls it, so a work session consults the work account and a personal session consults personal. Nothing to configure.

That said, it's helpful to add a guard to each profile's global `CLAUDE.md` anyway:

- **Work profile:** tell the agent to only operate inside your work repositories and to refuse anything outside them.
- **Non-work profiles:** add the inverse, AND an explicit rule never to read or touch work code.


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

ask-up -continue cns_1a2b3c4d "what about across processes?"   # resume the same advisor session
ask-up -list                                                   # list saved consultations
ask-up -v "question"                                           # also print token/session info
```

Flags must come before the prompt (Go's flag parser stops at the first non-flag argument). The answer goes to stdout; the continue pointer and any usage detail go to stderr, so the answer pipes cleanly.

### Composing a good consultation

The advisor gets one shot and cannot see your codebase. The quality of its answer is set entirely by how you frame the call, so do the work it cannot:

- **Curate, don't dump.** Pull the few relevant snippets, not whole files. A tight brief beats a pile of context.
- **Summarize the situation** and say what you have already tried or ruled out.
- **Ask one specific, decidable question.**
- Pipe it via a quoted heredoc (`<<'EOF'`) so code, quotes, and backticks pass through untouched.

`-continue` resumes the same session, so it keeps prior context and can go a few rounds. Only continue a consultation that already holds context relevant to the new question; start fresh for an unrelated topic.

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
