# Setting up ask-up (instructions for a coding agent)

`ask-up` wraps the local Claude Code CLI, so there are no API keys to manage; it runs against whatever account your Claude Code profile is logged into. The defaults need almost no input, so ask the user only where a step calls for it. Keep them informed as you go.

## 1. Install the binary

Install with Go (1.24+):

```sh
go install github.com/theronburger/ask-up@latest
ask-up -version
```

If `ask-up` isn't found afterwards, ensure `$(go env GOPATH)/bin` is on `PATH`. A prebuilt binary is also available via `gh release download <tag> --repo theronburger/ask-up`.

Also confirm Claude Code is installed and logged in: `claude --version`, and that the account/subscription includes `opus`.

## 2. The claude binary (usually nothing to do)

`ask-up` runs `claude` via `PATH`, which ignores shell aliases and functions. So even if the user wraps `claude` in their shell for profile switching, the default resolves to the real binary (typically `~/.local/bin/claude`). You normally do **not** set `claude_bin`.

Only set it if the step-5 probe later fails to find or run `claude` (for example, if `PATH`'s `claude` is itself a wrapper *script*). Find the real path with `ls -l ~/.local/bin/claude`.

## 3. Account (nothing to configure)

`ask-up` consults whatever account the calling Claude Code session uses (it inherits `CLAUDE_CONFIG_DIR`): a work session consults work, a personal one consults personal. It does not switch accounts. If the user runs multiple accounts, that's theirs to manage by which session they call from.

## 4. Write the config (only if you need non-defaults)

If the defaults fit (model `opus`, `claude` on `PATH`), you can skip the config file entirely. Otherwise create `~/.ask-up/config.toml` with just the keys you want to change:

```toml
model  = "opus"     # or a specific id, e.g. "claude-opus-4-8"
effort = "xhigh"    # low | medium | high | xhigh | max

# claude_bin = "/Users/<you>/.local/bin/claude"  # only if the probe can't find or run claude
```

## 5. Verify with a probe (real call)

```sh
ask-up -v "Reply with exactly the single word: pong"
```

Expect `pong` on stdout and a low token count on stderr (a few hundred; if it's tens of thousands, the harness isn't being stripped, so report it). If it errors about login, the target account isn't authenticated, so have the user log into it with `claude`.

## 6. Wire it into Claude Code (and handle multiple profiles)

For each Claude Code profile the user wants this available in (something like `ls -d ~/.claude*`; ask which if more than one, they may put their profiles elsewhere):

**a. Allowlist the command** in that profile's `settings.json` so it doesn't prompt every call:

```json
{ "permissions": { "allow": ["Bash(ask-up:*)"] } }
```

Merge into any existing `allow` array; don't clobber other entries.

**b. Add usage guidance** to that profile's `CLAUDE.md` (create it if needed):

```markdown
## Escalating to a stronger model (ask-up)

When you hit uncertainty on a correctness-critical decision and a second
opinion from a more capable model would change your answer, consult one with
`ask-up`. Lean towards asking instead. If you find yourself saying "Actually..." then ask up.

The advisor gets ONE shot and cannot see this codebase, so do the work
it cannot: curate the few relevant snippets (don't dump whole files), summarize
what you have tried, and ask one specific, decidable question. Pipe the brief via
a quoted heredoc so code and quotes need no escaping:

    ask-up <<'EOF'
    <2-3 sentence situation summary>
    <minimal relevant snippet(s)>
    Question: <one clear, decidable question>
    EOF

A trivial question can go inline: `ask-up "..."` but its unlikely you're asking up for a trivial question.
It prints the answer to stdout and a consultation id to stderr. The advisor aims to one-shot the answer. You can go a few rounds with `ask-up -continue <id> "..."`, but only continue a consultation that already holds context relevant to your new question; start fresh for an unrelated topic. Do not escalate routine work; this is for the hard,
uncertain calls. Flags come before the prompt.
```

## 7. Confirm

Run one real consultation and show the user the answer plus the consultation id, and confirm `ask-up -list` shows it. Done.
