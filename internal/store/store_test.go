package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/JonGanz/agent-status-collector/internal/clock"
	"github.com/JonGanz/agent-status-collector/internal/status"
)

func newTestStore(t *testing.T, opts ...Option) (*Store, *clock.Fake) {
	t.Helper()
	fc := clock.NewFake(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	base := []Option{WithClock(fc), WithIsRunning(func(pid int) bool { return false })}
	base = append(base, opts...)
	return New(t.TempDir(), base...), fc
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	s, fc := newTestStore(t)
	st := status.Status{
		SchemaVersion: status.CurrentSchemaVersion,
		SessionID:     "sess-1",
		Provider:      "claudecode",
		State:         status.StateActive,
		StartedAt:     fc.Now(),
		LastUpdated:   fc.Now(),
	}
	if err := s.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, existed, err := s.Load("sess-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !existed {
		t.Fatalf("Load: existed = false, want true")
	}
	if got.SessionID != st.SessionID || got.Provider != st.Provider || got.State != st.State {
		t.Fatalf("Load() = %+v, want %+v", got, st)
	}
}

func TestLoad_NotExist(t *testing.T) {
	s, _ := newTestStore(t)
	_, existed, err := s.Load("nope")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if existed {
		t.Fatalf("existed = true, want false")
	}
}

func TestList_ReturnsAllSessions(t *testing.T) {
	s, fc := newTestStore(t)
	for _, id := range []string{"a", "b", "c"} {
		st := status.Status{SessionID: id, Provider: "claudecode", State: status.StateActive, LastUpdated: fc.Now()}
		if err := s.Save(st); err != nil {
			t.Fatalf("Save(%s): %v", id, err)
		}
	}
	entries, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("List() returned %d entries, want 3", len(entries))
	}
}

func TestIsStale_PIDBased(t *testing.T) {
	running := 12345
	s, fc := newTestStore(t, WithIsRunning(func(pid int) bool { return pid == running }))
	live := status.Status{SessionID: "live", PID: &running, State: status.StateActive, LastUpdated: fc.Now()}
	deadPID := 99999
	dead := status.Status{SessionID: "dead", PID: &deadPID, State: status.StateActive, LastUpdated: fc.Now()}

	if s.IsStale(live) {
		t.Fatalf("live session with running PID reported stale")
	}
	if !s.IsStale(dead) {
		t.Fatalf("session with dead PID not reported stale")
	}
}

func TestIsStale_TTLFallbackWhenNoPID(t *testing.T) {
	s, fc := newTestStore(t, WithNoUpdateTTL(10*time.Minute))
	st := status.Status{SessionID: "no-pid", State: status.StateActive, LastUpdated: fc.Now()}

	if s.IsStale(st) {
		t.Fatalf("freshly updated PID-less session reported stale")
	}
	fc.Advance(20 * time.Minute)
	if !s.IsStale(st) {
		t.Fatalf("PID-less session past TTL not reported stale")
	}
}

func TestIsStale_PIDReuseBackstop(t *testing.T) {
	// Regression test: a killed session's PID can get recycled by the OS
	// onto an unrelated live process. isRunning(pid) would then report
	// true forever, so IsStale must not trust it as sole authority — once
	// LastUpdated is older than maxPIDTrustAge, it's stale regardless of
	// what isRunning says.
	reused := 12345
	s, fc := newTestStore(t, WithIsRunning(func(pid int) bool { return pid == reused }), WithMaxPIDTrustAge(1*time.Hour))
	st := status.Status{SessionID: "ghost", PID: &reused, State: status.StateActive, LastUpdated: fc.Now()}

	if s.IsStale(st) {
		t.Fatalf("freshly updated session with live PID reported stale")
	}
	fc.Advance(2 * time.Hour)
	if !s.IsStale(st) {
		t.Fatalf("session with live-but-stale PID past maxPIDTrustAge not reported stale")
	}
}

func TestIsStale_StoppedAlwaysStale(t *testing.T) {
	s, fc := newTestStore(t)
	st := status.Status{SessionID: "done", State: status.StateStopped, LastUpdated: fc.Now()}
	if !s.IsStale(st) {
		t.Fatalf("stopped session not reported stale")
	}
}

func TestShouldDelete_RespectsGraceWhileIsStaleIsAlreadyTrue(t *testing.T) {
	// Regression test: a stopped session is immediately Stale (per
	// IsStale), but must NOT be ShouldDelete-eligible until the grace
	// period passes. Callers previewing a prune (dry-run) must use
	// ShouldDelete, not Stale, or they'll report removals that a real
	// Prune() wouldn't actually perform yet.
	s, fc := newTestStore(t, WithStoppedGrace(5*time.Minute))
	st := status.Status{SessionID: "just-stopped", State: status.StateStopped, LastUpdated: fc.Now()}

	if !s.IsStale(st) {
		t.Fatalf("stopped session should be immediately Stale")
	}
	if s.ShouldDelete(st) {
		t.Fatalf("ShouldDelete = true within grace period, want false")
	}

	fc.Advance(10 * time.Minute)
	if !s.ShouldDelete(st) {
		t.Fatalf("ShouldDelete = false past grace period, want true")
	}
}

func TestPrune_RespectsStoppedGrace(t *testing.T) {
	s, fc := newTestStore(t, WithStoppedGrace(5*time.Minute))
	st := status.Status{SessionID: "just-stopped", State: status.StateStopped, LastUpdated: fc.Now()}
	if err := s.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	removed, err := s.Prune()
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("Prune removed %v within grace period, want none", removed)
	}

	fc.Advance(10 * time.Minute)
	removed, err = s.Prune()
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(removed) != 1 || removed[0] != "just-stopped" {
		t.Fatalf("Prune() = %v, want [just-stopped] after grace period", removed)
	}
	if _, existed, _ := s.Load("just-stopped"); existed {
		t.Fatalf("session file still exists after Prune")
	}
}

func TestPrune_DeletesDeadPIDImmediately(t *testing.T) {
	s, fc := newTestStore(t, WithIsRunning(func(pid int) bool { return false }))
	deadPID := 99999
	st := status.Status{SessionID: "dead", PID: &deadPID, State: status.StateActive, LastUpdated: fc.Now()}
	if err := s.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}
	removed, err := s.Prune()
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(removed) != 1 || removed[0] != "dead" {
		t.Fatalf("Prune() = %v, want [dead]", removed)
	}
}

func TestDelete_NonExistentIsNoop(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Delete("nope"); err != nil {
		t.Fatalf("Delete of nonexistent session returned error: %v", err)
	}
}

func TestSave_ConcurrentWritesProduceValidJSON(t *testing.T) {
	s, fc := newTestStore(t)
	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			st := status.Status{
				SessionID:   "concurrent",
				Provider:    "claudecode",
				State:       status.StateActive,
				LastUpdated: fc.Now(),
				Extra:       map[string]any{"writer": i},
			}
			if err := s.Save(st); err != nil {
				t.Errorf("Save from goroutine %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	got, existed, err := s.Load("concurrent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !existed {
		t.Fatalf("session missing after concurrent writes")
	}
	// Re-marshal to confirm the on-disk content was well-formed JSON (Load
	// already unmarshals, but double check the raw bytes too).
	raw, err := os.ReadFile(filepath.Join(s.dir, "concurrent.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("on-disk file is not valid JSON: %v", err)
	}
	_ = got
}
