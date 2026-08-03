package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/JonGanz/agent-status-collector/internal/status"
	"github.com/JonGanz/agent-status-collector/internal/store"
)

func TestList_ContainsExpectedColumns(t *testing.T) {
	var buf bytes.Buffer
	entries := []store.Entry{
		{
			Status: status.Status{
				SessionID:   "abcdef123456789",
				Provider:    "claudecode",
				State:       status.StateActive,
				TaskSummary: "Writing tests",
				Context:     &status.ContextUsage{PercentUsed: 42},
				LastUpdated: time.Now(),
			},
			Stale: false,
		},
	}
	if err := List(&buf, entries); err != nil {
		t.Fatalf("List: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"SESSION", "claudecode", "active", "42%", "Writing tests"} {
		if !strings.Contains(out, want) {
			t.Fatalf("List() output missing %q:\n%s", want, out)
		}
	}
}

func TestList_MarksStale(t *testing.T) {
	var buf bytes.Buffer
	entries := []store.Entry{
		{Status: status.Status{SessionID: "s1", Provider: "claudecode", State: status.StateActive}, Stale: true},
	}
	if err := List(&buf, entries); err != nil {
		t.Fatalf("List: %v", err)
	}
	if !strings.Contains(buf.String(), "stale") {
		t.Fatalf("List() output missing stale marker:\n%s", buf.String())
	}
}

func TestShow_IncludesDetail(t *testing.T) {
	var buf bytes.Buffer
	pid := 4242
	e := store.Entry{
		Status: status.Status{
			SessionID:   "sess-1",
			Provider:    "claudecode",
			State:       status.StateBlocked,
			PID:         &pid,
			TaskSummary: "Refactoring foo",
			StartedAt:   time.Now(),
			LastUpdated: time.Now(),
		},
	}
	if err := Show(&buf, e); err != nil {
		t.Fatalf("Show: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"sess-1", "claudecode", "blocked", "4242", "Refactoring foo"} {
		if !strings.Contains(out, want) {
			t.Fatalf("Show() output missing %q:\n%s", want, out)
		}
	}
}

func TestJSON_RoundTrips(t *testing.T) {
	var buf bytes.Buffer
	st := status.Status{SessionID: "s1", Provider: "claudecode"}
	if err := JSON(&buf, st); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if !strings.Contains(buf.String(), `"session_id": "s1"`) {
		t.Fatalf("JSON() output missing session_id:\n%s", buf.String())
	}
}
