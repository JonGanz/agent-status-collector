# agent-status-collector

A single local CLI for checking the status of multiple AI coding agents (Claude Code, and other providers as adapters are added) running on your machine — session state, current task, context usage, and cost per session, plus account-level rate limits, regardless of how each agent reports its own status.

- Local only — no data leaves your machine.
- One unified output format across providers.
- Provider adapters are pluggable; v1 ships a Claude Code adapter.

## Build / Install

Requires Go 1.25+.

```sh
go build -o agent-status .
```

Put the resulting `agent-status` binary somewhere on your `$PATH` (e.g. `~/.local/bin`).

Or install directly via Go tooling:

```sh
go install github.com/JonGanz/agent-status-collector@latest
```

Run the test suite:

```sh
go test ./...
```

## Usage

### List and inspect sessions

```sh
agent-status list              # live sessions
agent-status list --all        # include stale/stopped sessions
agent-status list --json       # machine-readable output
agent-status show <session-id>
```

### Set up an agent integration

```sh
agent-status providers                # see what's installed/configured
agent-status setup claudecode --dry-run   # preview changes, no writes
agent-status setup claudecode             # apply (backs up ~/.claude/settings.json first)
```

This wires up Claude Code's hooks and statusline to report into agent-status-collector, and installs a `report-status` skill so Claude Code can self-report what it's working on. Existing hooks/statusline commands are preserved (appended alongside, or wrapped), never overwritten.

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
