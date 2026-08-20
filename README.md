# agent-status-collector

A single local CLI for checking the status of multiple AI coding agents (Claude Code, and other providers as adapters are added) running on your machine — session state, current task, context usage, and cost per session, plus account-level rate limits, regardless of how each agent reports its own status.

- Local only — no data leaves your machine.
- One unified output format across providers.
- Provider adapters are pluggable; v1 ships a Claude Code adapter.

## Disclaimer

This repository's contents — code, tests, and documentation — are 100% LLM-produced.

## Build / Install

Requires Go 1.25+.

### Step 1 — build the binary

```sh
./install.sh
```

This runs `go build -o ~/.local/bin/agent-status .` — make sure `~/.local/bin` is on your `$PATH` (check with `echo $PATH`; if it's missing, add `export PATH="$HOME/.local/bin:$PATH"` to your shell profile).

Alternatives, if you'd rather not use `~/.local/bin`:

```sh
go build -o agent-status .                              # build into the current directory instead
go install github.com/JonGanz/agent-status-collector@latest   # build via Go tooling into $GOBIN/$GOPATH/bin
```

`@latest` resolves to the HEAD of the default branch unless/until this repo starts tagging releases — pin a commit (`@<sha>`) if you need reproducibility.

Run the test suite:

```sh
go test ./...
```

### Step 2 — wire up an agent integration

The binary alone only gives you a local CLI over whatever session data already exists on disk. To actually get Claude Code reporting into it, run:

```sh
agent-status providers                    # see what's installed/configured
agent-status setup claudecode --dry-run   # preview changes, no writes — run this first
agent-status setup claudecode              # apply
```

`setup claudecode` edits `~/.claude/settings.json` to add hooks, a statusline command, and installs a `report-status` skill file. It's safe to re-run:
- It always backs up the existing file first, to `~/.claude/settings.json.bak-<timestamp>`.
- Existing hooks/statusline commands are preserved (appended alongside, or wrapped), never overwritten.
- It's idempotent — configuration is only considered complete once hooks, statusline, and the skill file are *all* present, so re-running after a partial/interrupted apply just fills in what's missing.

This is not a daemon or background service — there's nothing to enable at boot or keep running. `agent-status` is invoked on-demand by the hooks/statusline it installs, and by you directly.

## Usage

### List and inspect sessions

```sh
agent-status list              # live sessions
agent-status list --all        # include stale/stopped sessions
agent-status list --json       # machine-readable output
agent-status show <session-id>
```

### Rate limits

```sh
agent-status rate-limits                     # all providers
agent-status rate-limits --provider claudecode
agent-status rate-limits --json
```

Rate limits (e.g. Claude's 5h/7d usage windows) are account-level, not tied to any one session — this reads the most recently reported snapshot directly, so you don't need to find which session last updated it.

### Maintenance

```sh
agent-status prune --dry-run   # preview stale sessions that would be removed
agent-status prune             # remove them
```

## Storage

Session state is stored as flat JSON files under `$XDG_STATE_HOME/agent-status-collector/sessions` (defaults to `~/.local/state/agent-status-collector`), and account-level rate limit snapshots under `.../rate-limits`. Pass `--debug` (or persist it via `$XDG_CONFIG_HOME/agent-status-collector/config.json`) to retain raw hook/statusline payloads for troubleshooting.
