# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Multi-agent AI-assisted development often runs several coding agents in parallel (multiple instances of the same agent, or a mix of Claude Code/Copilot/etc), each with its own way of surfacing status, context usage, and rate limits — making them hard to track together. `agent-status` is a single local CLI entrypoint that queries the status of every locally-running agent, session-level and aggregate (e.g. rate limits), in one unified format regardless of provider. See `README.md` for user-facing usage.

### Design constraints (non-negotiable, not just current behavior)

- **UNIX philosophy** — this tool does one thing (report agent status) well; it is not a dashboard, daemon, or notification system.
- **Local only, always** — no data is ever pulled from or pushed to a remote service. If a future change would phone home for any reason, that's out of scope for this project, not a config option to add.
- **No runtime dependencies** — the shipped binary needs nothing installed beyond itself (no Node/npm, no Python, etc). Shelling out to already-ubiquitous CLI tools (`jq`, `tmux`) is fine and already done (`providers/claudecode/setup.go`'s installed statusline script, `internal/mux/tmux.go`) — the constraint is about the *distributed* artifact, not build-time Go module deps.
- **XDG-compliant storage, always** — any new local storage or config path must go through `internal/xdg` (or `--state-dir`/`--config-dir` overrides), never a hardcoded `~/.foo`. This also means the on-disk format is a stable-ish contract: prefer additive changes to `status.Status`/`ratelimit.Record` and bump `status.CurrentSchemaVersion` for breaking ones, rather than silently reshaping what's already on disk.
- **Single, unified return value** — every provider adapter produces the same `status.Status` shape (see below); callers of `list`/`show` never need provider-specific branching.
- **Provider setup must be either automated-and-safe or explicit-and-manual** — `provider.Provider.Setup` returning `SetupResult{Changed: false, Instructions: "..."}` (manual-only path) is a legitimate, expected outcome for a provider that can't be safely automated, not a stopgap to eventually remove.

## Commands

```sh
go build -o agent-status .        # build the binary
go vet ./...                      # static checks — run before considering work done
go test ./...                     # full test suite
go test ./internal/store/...      # test a single package
go test ./providers/claudecode/... -run TestHandleHook_StateTransitions   # single test
gofmt -l .                        # list files needing formatting (gofmt -w . to fix)
```

There is no lint config beyond `go vet`/`gofmt`. No Node/other runtime is involved — this is a pure Go module (`github.com/JonGanz/agent-status-collector`, module-local, no remote CI configured).

## Architecture

### Provider-plugin boundary (the key structural rule)

`internal/` is the provider-agnostic core; `providers/<name>/` are adapters. The core **never imports a concrete provider package** except one blank import in `cmd/root.go` (`_ "github.com/JonGanz/agent-status-collector/providers/claudecode"`) that triggers each adapter's `init()` → `provider.Register(New())`. Everything else in `cmd/` and `internal/` talks only to the `provider.Provider` interface (`internal/provider/provider.go`) and the registry (`internal/provider/registry.go`). Adding a new agent provider means a new `providers/<name>` package plus one import line — nothing in `internal/` changes.

`provider.Provider` methods: `Name`, `HandleHook(ctx, event, payload io.Reader) (status.Status, error)`, `Detect`, `IsConfigured`, `Setup(dryRun bool)`. Adapters that need session persistence implement `provider.StoreAware.SetStore(SessionStore)`; adapters that report account-level rate limits implement `provider.RateLimitStoreAware.SetRateLimitStore(RateLimitStore)`. `cmd/hook.go` calls both (with a `store.Store`/`ratelimit.Store` built from the resolved `--state-dir`) before invoking `HandleHook`, since neither store's location is known at provider-registration time.

### Unified status schema and merge model

`internal/status.Status` is the one shape every adapter produces (state enum: `active`/`done`/`blocked`/`stopped`/`unknown`, plus context/cost/multiplexer/task-summary fields, and an `Extra map[string]any` escape hatch for provider-specific data that shouldn't require a core schema change). `done` means the turn finished and nothing is pending; `blocked` specifically means the agent needs a decision from you (e.g. a permission request) — keeping these distinct is the point, so don't collapse them back together.

Each hook/statusline call only carries a partial view. Adapters own their own load-merge-save cycle against their injected `SessionStore` — the core does not merge hook payloads itself. After an adapter returns a `Status`, `cmd/hook.go` stamps `Multiplexer` info (via `internal/mux`, tmux/screen detection) and re-saves, since multiplexer context is a core-owned, provider-agnostic concern. `internal/mux/tmux.go` always targets `tmux display-message -t "$TMUX_PANE"` (never a bare `-p` with no target) — without it, tmux resolves "current" to whatever pane the attached *client* currently has focused, not the pane this process is actually running in, so a session's stored pane/window would silently get overwritten with the wrong ids the moment the user looked at a different window. This was a real, previously-shipped bug (external consumers like tmux-asc-binder inherited the wrong window/pane); don't remove the explicit `-t`.

**Rate limits are deliberately not on `Status`.** They're account-level (e.g. Claude's 5h/7d windows apply to the whole account, not one session) even though an adapter may only be able to observe them via one session's integration. They're persisted separately by `internal/ratelimit` (one flat file per provider, keyed by provider name, not session id) and surfaced via `agent-status rate-limits`, so a caller never needs to find "a session that happened to report it." `status.RateLimitWindow` is the shared value type between the two.

### Claude Code adapter (`providers/claudecode/`)

Combines two independent Claude Code data sources into one stored record per `session_id`:
- **Hooks** (`SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Notification`, `Stop`, `SubagentStop`, `PreCompact`, `SessionEnd`) drive `State` transitions — see the switch in `claudecode.go`'s `handleLifecycleHook`. `Stop` (turn concluded) always sets `StateDone`, distinct from `SessionEnd` (session terminated, `StateStopped`). `Notification` is the only "needs attention" signal Claude Code exposes, but it fires for two unrelated reasons distinguishable only by free-text message content — `isPermissionRequest` does a best-effort substring match (`"permission"`) to set `StateBlocked` for genuine tool-permission requests, versus `StateDone` for the ~60s idle-waiting reminder that follows an already-completed `Stop`. This heuristic is inherently a little fragile (Claude Code doesn't document the message wording as a stable API) but it's the only signal available — don't collapse the two back into one state, that was the whole point of splitting them out.
- **Statusline** invocations (`--event=StatusLine`, handled in `statusline.go`) update the session's `Context`/`Cost`/model info, and separately persist `rate_limits` to the account-level rate limit store (`p.rateLimits`, nil-checked — a provider only implements `RateLimitStoreAware` if `cmd/hook.go` wired one up). Statusline never touches `State`.
- A synthetic `--event=TaskSummary` (handled in `tasksummary.go`) backs the `report-status` skill (embedded from `assets/report-status.SKILL.md` via `go:embed`, installed by `Setup`). Since Claude Code doesn't expose `session_id` to the model, this event resolves the target session by matching current working directory against tracked sessions (`findActiveSessionByCwd`) when no explicit `session_id` is given — a documented best-effort heuristic, not a guarantee.
- PID capture (`pid.go`) uses `os.Getppid()`, relying on `Setup()` installing hook commands as direct child processes of Claude Code (no `sh -c` wrapper) so the parent PID is actually Claude Code's; sanity-checked against `/proc/<pid>/comm` on Linux. This is inherently best-effort — `internal/store`'s TTL-based staleness fallback covers the case where it's wrong.
- `Setup()`/`settings.go` read-modify-write `~/.claude/settings.json`: hook entries are merged idempotently (matched by a command substring marker, `hookCommandMarker`) without disturbing other tools' entries in the same event array; an existing third-party `statusLine` command is wrapped (not replaced) unless it's already ours. Every real write backs up `settings.json` first and dry-run must never touch the filesystem — this file is global, shared across all of the user's Claude Code projects.

### Storage (`internal/store/`, `internal/ratelimit/`)

Sessions: one JSON file per session under `<state-dir>/sessions/<session-id>.json` (default XDG state dir, resolved in `internal/xdg`, overridable via `--state-dir`), with per-session flock (`lock.go`) so concurrent hook invocations never corrupt a file. Staleness (`IsStale`) checks PID liveness first (`internal/procutil`), falls back to a `LastUpdated` TTL when no PID is known, and treats `StateStopped` as immediately stale but not immediately deletion-eligible — `Prune`/`ShouldDelete` respect a separate grace period so a `list --all` right after a session ends still shows it. **Use `ShouldDelete`, not `Stale`, when previewing what a real prune would remove** (there was a bug here once: `prune --dry-run` used `Stale` and reported removals during the grace period that `Prune()` wouldn't actually perform).

Rate limits: one JSON file per provider under `<state-dir>/rate-limits/<provider>.json` (`internal/ratelimit.Store`) — no session id involved at all, since these snapshots are account-level.

Both stores (plus `providers/claudecode/setup.go`'s settings.json/skill/statusline-script writes) share `internal/fsutil.WriteAtomic` (temp file in the same directory + rename) so nothing ever leaves a torn/partial file behind — don't reintroduce a package-local copy of this when adding a new storage location.

### Debug logging (`internal/eventlog/`)

`--debug` (one-shot flag) or a persisted `{"debug": true}` in `$XDG_CONFIG_HOME/agent-status-collector/config.json` (`internal/config`, since a hook fired in the background by Claude Code never gets an interactive flag) makes `cmd/hook.go` append every raw hook/statusline payload it receives to `<state-dir>/logs/events.jsonl` via `eventlog.Append` — one JSON line per invocation (timestamp, provider, event, raw payload), with a basic size-based rotation guard. This exists specifically to debug "the tool lost track of an agent" — when that happens, diff what the log actually received against what `HandleHook` did with it, rather than guessing. `debugEnabled()` (`cmd/root.go`) resolves the flag/config precedence; `--debug` always wins.

### Testability conventions

- `internal/clock.Clock` (real/fake) and injectable `isRunning func(pid int) bool` on `store.Store` make staleness/TTL logic deterministic in tests (`store.WithClock`, `store.WithIsRunning`).
- `internal/testutil.LoadFixture(t, relPath)` loads from the calling package's own `testdata/` dir — used by `providers/claudecode` tests against `testdata/hooks/*.json` and `testdata/statusline/*.json`.
- `providers/claudecode` tests use in-memory `memStore`/`memRateLimitStore` (implement `provider.SessionStore`/`provider.RateLimitStore`) instead of the real file-backed stores, for isolated adapter-logic tests.
- `cmd/integration_test.go` drives the actual cobra commands end-to-end (`rootCmd.SetArgs`/`SetOut` + `--state-dir` pointed at `t.TempDir()`).
- `providers/claudecode/setup_test.go` overrides `$HOME` (`t.Setenv`) to test the `~/.claude/settings.json` merge logic without touching the real file.

### Safety note for manual testing

This tool detects tmux/screen context and reads Claude Code's real `~/.claude/settings.json` when run for real (not under `go test`). When manually exercising `setup`/`hook`/`mux` behavior in a shell, override `$HOME` and `$XDG_STATE_HOME` to scratch directories first — do not point it at the real `~/.claude` config, and never run commands that could kill/disrupt the current tmux session (`internal/mux` only ever calls read-only `tmux display-message`, which is safe).
