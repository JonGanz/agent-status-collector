# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A local-only Go CLI (`agent-status`) that tracks the status of AI coding agents (Claude Code, and other providers as adapters are added) running on the developer's machine, exposing a single unified status format regardless of provider. See `GOAL.md` for the original problem statement/design philosophy and `README.md` for user-facing usage.

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

`provider.Provider` methods: `Name`, `HandleHook(ctx, event, payload io.Reader) (status.Status, error)`, `Detect`, `IsConfigured`, `Setup(dryRun bool)`. Adapters that need session persistence implement `provider.StoreAware.SetStore(SessionStore)` — `cmd/hook.go` calls this (with a `store.Store` built from the resolved `--state-dir`) before invoking `HandleHook`, since the store's location isn't known at provider-registration time.

### Unified status schema and merge model

`internal/status.Status` is the one shape every adapter produces (state enum: `active`/`idle`/`waiting_for_input`/`stopped`/`unknown`, plus context/cost/rate-limit/multiplexer/task-summary fields, and an `Extra map[string]any` escape hatch for provider-specific data that shouldn't require a core schema change).

Each hook/statusline call only carries a partial view. Adapters own their own load-merge-save cycle against their injected `SessionStore` — the core does not merge hook payloads itself. After an adapter returns a `Status`, `cmd/hook.go` stamps `Multiplexer` info (via `internal/mux`, tmux/screen detection) and re-saves, since multiplexer context is a core-owned, provider-agnostic concern.

### Claude Code adapter (`providers/claudecode/`)

Combines two independent Claude Code data sources into one stored record per `session_id`:
- **Hooks** (`SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Notification`, `Stop`, `SubagentStop`, `PreCompact`, `SessionEnd`) drive `State` transitions — see the switch in `claudecode.go`'s `handleLifecycleHook`. Notably `Notification` is the only "needs attention" signal Claude Code exposes (covers both permission prompts and idle nudges), and `Stop` (turn done) is distinct from `SessionEnd` (session terminated).
- **Statusline** invocations (`--event=StatusLine`, handled in `statusline.go`) carry `context_window`/`cost`/`rate_limits`/`model` and never touch `State`.
- A synthetic `--event=TaskSummary` (handled in `tasksummary.go`) backs the `report-status` skill (embedded from `assets/report-status.SKILL.md` via `go:embed`, installed by `Setup`). Since Claude Code doesn't expose `session_id` to the model, this event resolves the target session by matching current working directory against tracked sessions (`findActiveSessionByCwd`) when no explicit `session_id` is given — a documented best-effort heuristic, not a guarantee.
- PID capture (`pid.go`) uses `os.Getppid()`, relying on `Setup()` installing hook commands as direct child processes of Claude Code (no `sh -c` wrapper) so the parent PID is actually Claude Code's; sanity-checked against `/proc/<pid>/comm` on Linux. This is inherently best-effort — `internal/store`'s TTL-based staleness fallback covers the case where it's wrong.
- `Setup()`/`settings.go` read-modify-write `~/.claude/settings.json`: hook entries are merged idempotently (matched by a command substring marker, `hookCommandMarker`) without disturbing other tools' entries in the same event array; an existing third-party `statusLine` command is wrapped (not replaced) unless it's already ours. Every real write backs up `settings.json` first and dry-run must never touch the filesystem — this file is global, shared across all of the user's Claude Code projects.

### Storage (`internal/store/`)

One JSON file per session under `<state-dir>/sessions/<session-id>.json` (default XDG state dir, resolved in `internal/xdg`, overridable via `--state-dir`). Atomic writes (temp file in the same directory + rename, `atomic.go`) and per-session flock (`lock.go`) so concurrent hook invocations never corrupt a file. Staleness (`IsStale`) checks PID liveness first (`internal/procutil`), falls back to a `LastUpdated` TTL when no PID is known, and treats `StateStopped` as immediately stale but not immediately deletion-eligible — `Prune`/`ShouldDelete` respect a separate grace period so a `list --all` right after a session ends still shows it. **Use `ShouldDelete`, not `Stale`, when previewing what a real prune would remove** (there was a bug here once: `prune --dry-run` used `Stale` and reported removals during the grace period that `Prune()` wouldn't actually perform).

### Testability conventions

- `internal/clock.Clock` (real/fake) and injectable `isRunning func(pid int) bool` on `store.Store` make staleness/TTL logic deterministic in tests (`store.WithClock`, `store.WithIsRunning`).
- `internal/testutil.LoadFixture(t, relPath)` loads from the calling package's own `testdata/` dir — used by `providers/claudecode` tests against `testdata/hooks/*.json` and `testdata/statusline/*.json`.
- `providers/claudecode` tests use an in-memory `memStore` (implements `provider.SessionStore`) instead of the real file-backed store, for isolated adapter-logic tests.
- `cmd/integration_test.go` drives the actual cobra commands end-to-end (`rootCmd.SetArgs`/`SetOut` + `--state-dir` pointed at `t.TempDir()`).
- `providers/claudecode/setup_test.go` overrides `$HOME` (`t.Setenv`) to test the `~/.claude/settings.json` merge logic without touching the real file.

### Safety note for manual testing

This tool detects tmux/screen context and reads Claude Code's real `~/.claude/settings.json` when run for real (not under `go test`). When manually exercising `setup`/`hook`/`mux` behavior in a shell, override `$HOME` and `$XDG_STATE_HOME` to scratch directories first — do not point it at the real `~/.claude` config, and never run commands that could kill/disrupt the current tmux session (`internal/mux` only ever calls read-only `tmux display-message`, which is safe).
