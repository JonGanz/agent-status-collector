// Package provider defines the boundary every agent adapter (Claude Code,
// and future providers such as Copilot) must implement. The core (cmd/,
// internal/store) only ever depends on this interface — never on a
// concrete provider package — except for the single blank import in
// cmd/root.go that registers each built-in provider.
package provider

import (
	"context"
	"io"

	"github.com/JonGanz/agent-status-collector/internal/status"
)

// Provider is implemented by each agent-specific adapter package.
type Provider interface {
	// Name returns the stable provider identifier, e.g. "claudecode".
	Name() string

	// HandleHook parses a raw hook/statusline payload (as delivered by the
	// provider's own integration mechanism, piped via
	// `agent-status hook <name> [--event=...]`) and returns the resulting
	// unified Status for that session. Implementations are responsible for
	// loading/merging with any previously stored Status themselves.
	HandleHook(ctx context.Context, event string, payload io.Reader) (status.Status, error)

	// Detect reports whether this provider appears to be installed/usable
	// on this machine at all (e.g. its CLI binary is on PATH).
	Detect() (installed bool, detail string)

	// IsConfigured reports whether this tool's integration (hooks,
	// statusline, skills, etc.) has already been wired up for the provider.
	IsConfigured() (bool, error)

	// Setup performs (dryRun=false) or describes (dryRun=true) the
	// automated integration steps. dryRun must never touch the filesystem.
	Setup(dryRun bool) (SetupResult, error)
}

// SetupResult describes the outcome of a Setup call.
type SetupResult struct {
	// Changed is true if Setup made (or, for dry runs, would make) any
	// changes.
	Changed bool
	// Instructions is human-readable text describing what was done, would
	// be done, or must be done manually.
	Instructions string
	// FilesTouched lists paths created, modified, or backed up.
	FilesTouched []string
}

// SessionStore is the narrow persistence interface adapters need to
// load-then-merge a partial hook/statusline payload against the
// previously stored Status for a session. internal/store.Store satisfies
// this interface; adapters never depend on internal/store directly.
type SessionStore interface {
	Load(sessionID string) (status.Status, bool, error)
	Save(status.Status) error
	// LoadAll returns every stored Status. Used by adapters that need to
	// correlate a payload lacking a session id (e.g. a skill invocation)
	// against existing sessions via some other signal (working directory).
	LoadAll() ([]status.Status, error)
}

// StoreAware is implemented by providers that need access to the
// configured SessionStore (e.g. to honor a --state-dir override that isn't
// known at provider-registration time). The core calls SetStore once,
// after resolving the store for this invocation, before calling
// HandleHook.
type StoreAware interface {
	SetStore(SessionStore)
}
