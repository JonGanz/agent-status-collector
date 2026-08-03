package claudecode

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JonGanz/agent-status-collector/internal/status"
)

func TestHandleTaskSummary_BySessionID(t *testing.T) {
	p, ms, _ := newTestProvider(t)
	ms.sessions[testSessionID] = status.Status{SessionID: testSessionID, Provider: providerName, State: status.StateActive}

	body, _ := json.Marshal(map[string]string{"session_id": testSessionID, "summary": "Writing tests"})
	st, err := p.HandleHook(context.Background(), "TaskSummary", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}
	if st.TaskSummary != "Writing tests" {
		t.Fatalf("TaskSummary = %q", st.TaskSummary)
	}
}

func TestHandleTaskSummary_FallsBackToCwdMatch(t *testing.T) {
	p, ms, _ := newTestProvider(t)
	now := time.Now()
	ms.sessions[testSessionID] = status.Status{
		SessionID:   testSessionID,
		Provider:    providerName,
		State:       status.StateActive,
		WorkingDir:  "/home/dev/myproj",
		LastUpdated: now,
	}
	ms.sessions["other-session"] = status.Status{
		SessionID:   "other-session",
		Provider:    providerName,
		State:       status.StateActive,
		WorkingDir:  "/home/dev/otherproj",
		LastUpdated: now,
	}

	body, _ := json.Marshal(map[string]string{"cwd": "/home/dev/myproj", "summary": "Refactoring foo"})
	st, err := p.HandleHook(context.Background(), "TaskSummary", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}
	if st.SessionID != testSessionID {
		t.Fatalf("matched wrong session %q, want %q", st.SessionID, testSessionID)
	}
	if st.TaskSummary != "Refactoring foo" {
		t.Fatalf("TaskSummary = %q", st.TaskSummary)
	}
}

func TestHandleTaskSummary_NoMatchingSession_Errors(t *testing.T) {
	p, _, _ := newTestProvider(t)
	body, _ := json.Marshal(map[string]string{"cwd": "/nowhere", "summary": "x"})
	_, err := p.HandleHook(context.Background(), "TaskSummary", bytes.NewReader(body))
	if err == nil {
		t.Fatalf("expected error when no session matches cwd")
	}
}

func TestHandleTaskSummary_MissingSummary_Errors(t *testing.T) {
	p, ms, _ := newTestProvider(t)
	ms.sessions[testSessionID] = status.Status{SessionID: testSessionID, Provider: providerName, State: status.StateActive}
	body, _ := json.Marshal(map[string]string{"session_id": testSessionID})
	_, err := p.HandleHook(context.Background(), "TaskSummary", bytes.NewReader(body))
	if err == nil {
		t.Fatalf("expected error for missing summary")
	}
}

func TestHandleTaskSummary_IgnoresStoppedSessions(t *testing.T) {
	p, ms, _ := newTestProvider(t)
	now := time.Now()
	ms.sessions[testSessionID] = status.Status{
		SessionID:   testSessionID,
		Provider:    providerName,
		State:       status.StateStopped,
		WorkingDir:  "/home/dev/myproj",
		LastUpdated: now,
	}
	body, _ := json.Marshal(map[string]string{"cwd": "/home/dev/myproj", "summary": "x"})
	_, err := p.HandleHook(context.Background(), "TaskSummary", bytes.NewReader(body))
	if err == nil {
		t.Fatalf("expected error: stopped sessions should not match cwd fallback")
	}
}
