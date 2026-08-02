package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JonGanz/agent-status-collector/internal/status"
)

func seedSession(t *testing.T, stateDir string, st status.Status) {
	t.Helper()
	sessionsDir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	path := filepath.Join(sessionsDir, st.SessionID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return out.String(), err
}

func TestList_ShowsLiveHidesStale(t *testing.T) {
	stateDir := t.TempDir()
	livePID := os.Getpid()
	seedSession(t, stateDir, status.Status{
		SchemaVersion: status.CurrentSchemaVersion,
		SessionID:     "live-session",
		Provider:      "claudecode",
		PID:           &livePID,
		State:         status.StateActive,
		LastUpdated:   time.Now(),
	})
	deadPID := 999999999
	seedSession(t, stateDir, status.Status{
		SchemaVersion: status.CurrentSchemaVersion,
		SessionID:     "stale-session",
		Provider:      "claudecode",
		PID:           &deadPID,
		State:         status.StateActive,
		LastUpdated:   time.Now().Add(-time.Hour),
	})

	out, err := runCmd(t, "list", "--json", "--state-dir", stateDir)
	if err != nil {
		t.Fatalf("list --json: %v\n%s", err, out)
	}
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(entries) != 1 {
		t.Fatalf("list (default) returned %d entries, want 1 (live only):\n%s", len(entries), out)
	}

	out, err = runCmd(t, "list", "--all", "--json", "--state-dir", stateDir)
	if err != nil {
		t.Fatalf("list --all --json: %v\n%s", err, out)
	}
	entries = nil
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(entries) != 2 {
		t.Fatalf("list --all returned %d entries, want 2:\n%s", len(entries), out)
	}
}

func TestShow_ReturnsStaleSessionDetail(t *testing.T) {
	stateDir := t.TempDir()
	deadPID := 999999999
	seedSession(t, stateDir, status.Status{
		SchemaVersion: status.CurrentSchemaVersion,
		SessionID:     "stale-session",
		Provider:      "claudecode",
		PID:           &deadPID,
		State:         status.StateActive,
		LastUpdated:   time.Now().Add(-time.Hour),
	})

	out, err := runCmd(t, "show", "stale-session", "--json", "--state-dir", stateDir)
	if err != nil {
		t.Fatalf("show: %v\n%s", err, out)
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(out), &entry); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if entry["stale"] != true {
		t.Fatalf("expected stale=true in show output: %v", entry)
	}
}

func TestShow_UnknownSession_Errors(t *testing.T) {
	stateDir := t.TempDir()
	_, err := runCmd(t, "show", "nope", "--state-dir", stateDir)
	if err == nil {
		t.Fatalf("expected error for unknown session id")
	}
}

func TestPrune_DryRun_RespectsStoppedGracePeriod(t *testing.T) {
	// Regression test for a bug where `prune --dry-run` reported a
	// just-stopped session as removable (since Stale is immediately true
	// for stopped sessions) even though a real Prune() would retain it
	// until the grace period passed.
	stateDir := t.TempDir()
	seedSession(t, stateDir, status.Status{
		SchemaVersion: status.CurrentSchemaVersion,
		SessionID:     "just-stopped",
		Provider:      "claudecode",
		State:         status.StateStopped,
		LastUpdated:   time.Now(),
	})

	out, err := runCmd(t, "prune", "--dry-run", "--state-dir", stateDir)
	if err != nil {
		t.Fatalf("prune --dry-run: %v\n%s", err, out)
	}
	if out != "" {
		t.Fatalf("prune --dry-run reported a removal within the grace period:\n%s", out)
	}
}
