// Package store persists agent Status records as flat JSON files under a
// directory (in production, $XDG_STATE_HOME/agent-status-collector/sessions).
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JonGanz/agent-status-collector/internal/clock"
	"github.com/JonGanz/agent-status-collector/internal/fsutil"
	"github.com/JonGanz/agent-status-collector/internal/procutil"
	"github.com/JonGanz/agent-status-collector/internal/status"
)

// Default staleness tuning. See IsStale for how these are applied.
const (
	DefaultNoUpdateTTL    = 15 * time.Minute
	DefaultStoppedGrace   = 5 * time.Minute
	DefaultMaxPIDTrustAge = 24 * time.Hour
	filePerm              = 0o600
)

// Store persists Status records as one JSON file per session under Dir.
type Store struct {
	dir            string
	clock          clock.Clock
	isRunning      func(pid int) bool
	noUpdateTTL    time.Duration
	stoppedGrace   time.Duration
	maxPIDTrustAge time.Duration
}

// Option configures a Store.
type Option func(*Store)

// WithClock overrides the clock used for staleness calculations (for tests).
func WithClock(c clock.Clock) Option { return func(s *Store) { s.clock = c } }

// WithIsRunning overrides the PID-liveness checker (for tests).
func WithIsRunning(f func(pid int) bool) Option { return func(s *Store) { s.isRunning = f } }

// WithNoUpdateTTL overrides how long a PID-less session can go without an
// update before being considered stale.
func WithNoUpdateTTL(d time.Duration) Option { return func(s *Store) { s.noUpdateTTL = d } }

// WithStoppedGrace overrides how long a stopped session is retained (visible
// via List with includeStale) before Prune removes it.
func WithStoppedGrace(d time.Duration) Option { return func(s *Store) { s.stoppedGrace = d } }

// WithMaxPIDTrustAge overrides how long a PID reported as "running" is
// trusted without any session update before staleness falls back to the
// LastUpdated TTL anyway (guards against OS PID reuse masking a dead
// session forever).
func WithMaxPIDTrustAge(d time.Duration) Option { return func(s *Store) { s.maxPIDTrustAge = d } }

// New creates a Store rooted at dir (the "sessions" directory itself, not
// its parent). The directory is created lazily on first write.
func New(dir string, opts ...Option) *Store {
	s := &Store{
		dir:            dir,
		clock:          clock.Real{},
		isRunning:      procutil.IsRunning,
		noUpdateTTL:    DefaultNoUpdateTTL,
		stoppedGrace:   DefaultStoppedGrace,
		maxPIDTrustAge: DefaultMaxPIDTrustAge,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Store) sessionPath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *Store) lockPath(id string) string {
	return filepath.Join(s.dir, "."+id+".lock")
}

// Save writes st to disk atomically, under an exclusive lock for its
// session id.
func (s *Store) Save(st status.Status) error {
	if st.SessionID == "" {
		return fmt.Errorf("store: Save: SessionID must not be empty")
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal status: %w", err)
	}
	path := s.sessionPath(st.SessionID)
	return withLock(s.lockPath(st.SessionID), func() error {
		return fsutil.WriteAtomic(path, data, filePerm)
	})
}

// Load reads the Status for id. existed is false (with a zero Status and nil
// error) if no record is on disk.
func (s *Store) Load(id string) (st status.Status, existed bool, err error) {
	data, err := os.ReadFile(s.sessionPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return status.Status{}, false, nil
		}
		return status.Status{}, false, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return status.Status{}, false, fmt.Errorf("store: unmarshal %s: %w", id, err)
	}
	return st, true, nil
}

// Entry pairs a Status with the store's computed (non-persisted) staleness.
type Entry struct {
	Status status.Status `json:"status"`
	Stale  bool          `json:"stale"`
}

// List returns every session on disk, each annotated with whether it's
// currently considered stale. It never deletes anything.
func (s *Store) List() ([]Entry, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	for _, de := range entries {
		name := de.Name()
		if de.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		st, existed, err := s.Load(id)
		if err != nil || !existed {
			continue // skip unreadable/racily-deleted entries rather than failing List entirely
		}
		out = append(out, Entry{Status: st, Stale: s.IsStale(st)})
	}
	return out, nil
}

// LoadAll returns every stored Status, without staleness annotation. It
// satisfies provider.SessionStore for adapters that need to scan all
// sessions (e.g. to correlate a payload lacking a session id).
func (s *Store) LoadAll() ([]status.Status, error) {
	entries, err := s.List()
	if err != nil {
		return nil, err
	}
	out := make([]status.Status, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Status)
	}
	return out, nil
}

// Delete removes the on-disk record for id, if present.
func (s *Store) Delete(id string) error {
	err := os.Remove(s.sessionPath(id))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	os.Remove(s.lockPath(id)) // best-effort cleanup, ignore error
	return err
}

// IsStale reports whether st should be considered no-longer-live.
//
// Rules (checked in order):
//  1. State == StateStopped: always stale, but see Prune for the grace
//     period before deletion.
//  2. PID known: stale if the process no longer exists (highest-confidence
//     signal). If it does exist, it's still stale once LastUpdated is
//     older than maxPIDTrustAge — the OS can recycle a PID onto an
//     unrelated process, so a dead-but-reused PID must not be trusted as
//     "alive" forever. StateBlocked is exempt from this time check: sitting
//     idle waiting on the user is the expected, correct behavior of that
//     state, not evidence of death, so only the liveness check applies.
//  3. PID unknown: stale iff LastUpdated is older than noUpdateTTL.
//     StateBlocked instead uses maxPIDTrustAge, since a session waiting
//     indefinitely on the user can easily outlive the short default TTL
//     with zero new hook traffic.
func (s *Store) IsStale(st status.Status) bool {
	if st.State == status.StateStopped {
		return true
	}
	if st.PID != nil {
		if !s.isRunning(*st.PID) {
			return true
		}
		if st.State == status.StateBlocked {
			return false
		}
		return s.clock.Now().Sub(st.LastUpdated) > s.maxPIDTrustAge
	}
	ttl := s.noUpdateTTL
	if st.State == status.StateBlocked {
		ttl = s.maxPIDTrustAge
	}
	return s.clock.Now().Sub(st.LastUpdated) > ttl
}

// ShouldDelete reports whether a stale entry has passed its grace period and
// is eligible for actual deletion by Prune. Callers previewing a prune
// (e.g. `prune --dry-run`) should use this rather than Stale/IsStale, since
// a stopped session is immediately Stale but retained for a grace period
// before it's actually deletion-eligible.
func (s *Store) ShouldDelete(st status.Status) bool {
	if !s.IsStale(st) {
		return false
	}
	if st.State == status.StateStopped {
		return s.clock.Now().Sub(st.LastUpdated) > s.stoppedGrace
	}
	return true
}

// Prune deletes stale session files (past any grace period) and returns the
// session ids removed.
func (s *Store) Prune() ([]string, error) {
	entries, err := s.List()
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, e := range entries {
		if s.ShouldDelete(e.Status) {
			if err := s.Delete(e.Status.SessionID); err != nil {
				return removed, err
			}
			removed = append(removed, e.Status.SessionID)
		}
	}
	return removed, nil
}
