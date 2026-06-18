# Setting up ask-up (instructions for a coding agent)

You are a coding agent installing `ask-up` for your user. Do the steps yourself; only ask the human for the few things you cannot do (the API key, and which profiles to wire). Keep the human informed as you go. Do not write any secret to a file.

## 1. Install the binary

Prefer a prebuilt release; fall back to building from source.

```sh
# From source (Go 1.24+):
go install github.com/theronburger/ask-up@latest
# Confirm it is on PATH:
ask-up -version
```

If `ask-up` is not found after `go install`, ensure `$(go env GOPATH)/bin` is on the user's `PATH` and tell them the line to add to their shell profile.

## 2. Create the config

Write `~/.ask-up/config.toml` (or `$ASK_UP_HOME/config.toml`) only if the user wants non-default settings. Defaults are fine for most: model `claude-opus-4-8`, effort `high`, ttl `5m`. A starting file:

```toml
model      = "claude-opus-4-8"
effort     = "high"
ttl        = "5m"
# base_url = "https://your-enterprise-gateway/..."   # only if applicable
```

## 3. Wire the API key (do not write it to a file)

`ask-up` reads `ANTHROPIC_API_KEY` (or `ANTHROPIC_AUTH_TOKEN`) from the environment. Per policy the key must come from 1Password, not a file. Add a line like this to the user's shell profile (confirm the exact 1Password item path with them):

```sh
export ANTHROPIC_API_KEY="$(op read 'op://Private/Anthropic API/credential')"
```

Then verify end to end with a probe (this makes a real call):

```sh
ask-up -v "Reply with the single word: ready"
```

Check that an answer prints and the usage line appears on stderr. If it fails on auth, the key is not resolving; fix that before continuing.

## 4. Wire it into Claude Code (handle multiple profiles)

The user may have more than one Claude Code config directory. Detect them:

```sh
ls -d ~/.claude* 2>/dev/null
```

For each profile directory the user wants to enable (ask if there is more than one), do two things:

**a. Allowlist the command** so it does not prompt on every call. In that directory's `settings.json`, add `Bash(ask-up:*)` to `permissions.allow` (create the structure if absent):

```json
{
  "permissions": {
    "allow": ["Bash(ask-up:*)"]
  }
}
```

Merge into any existing `allow` array; do not clobber other entries.

**b. Teach the agent when to use it.** Append a section to that directory's `CLAUDE.md` (create the file if needed):

```markdown
## Escalating to a stronger model (ask-up)

When you hit genuine uncertainty on a correctness-critical decision and a second
opinion from a more capable model would change your answer, run `ask-up`:

    ask-up "your specific, self-contained question"

It prints the answer to stdout and a consultation id to stderr. To go back and
forth on the same thread, continue it:

    ask-up -continue <id> "follow-up"

Reuse a consultation only when it already holds context relevant to the new
question. If `ask-up` warns that a consultation's cache is cold, it is telling
you reviving it re-bills the full history; start a fresh consultation unless you
specifically need that prior context (then add -force). Do not escalate routine
work: this is for the hard, uncertain calls, not every question.

Flags must come before the question.
```

## 5. Confirm

Run one real escalation through the configured tool and show the user the result plus the consultation id. Confirm `ask-up -list` shows it. You are done.
