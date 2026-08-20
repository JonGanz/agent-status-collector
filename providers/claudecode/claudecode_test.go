package claudecode

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/JonGanz/agent-status-collector/internal/status"
	"github.com/JonGanz/agent-status-collector/internal/testutil"
)

// memStore is an in-memory provider.SessionStore for isolated adapter tests.
type memStore struct {
	sessions map[string]status.Status
}

func newMemStore() *memStore { return &memStore{sessions: map[string]status.Status{}} }

func (m *memStore) Load(id string) (status.Status, bool, error) {
	st, ok := m.sessions[id]
	return st, ok, nil
}

func (m *memStore) Save(st status.Status) error {
	m.sessions[st.SessionID] = st
	return nil
}

func (m *memStore) LoadAll() ([]status.Status, error) {
	out := make([]status.Status, 0, len(m.sessions))
	for _, st := range m.sessions {
		out = append(out, st)
	}
	return out, nil
}

// memRateLimitStore is an in-memory provider.RateLimitStore for isolated
// adapter tests.
type memRateLimitStore struct {
	windows     map[string][]status.RateLimitWindow
	lastUpdated map[string]time.Time
}

func newMemRateLimitStore() *memRateLimitStore {
	return &memRateLimitStore{
		windows:     map[string][]status.RateLimitWindow{},
		lastUpdated: map[string]time.Time{},
	}
}

func (m *memRateLimitStore) SaveRateLimits(providerName string, windows []status.RateLimitWindow, at time.Time) error {
	m.windows[providerName] = windows
	m.lastUpdated[providerName] = at
	return nil
}

func (m *memRateLimitStore) LoadRateLimits(providerName string) ([]status.RateLimitWindow, time.Time, bool, error) {
	w, ok := m.windows[providerName]
	if !ok {
		return nil, time.Time{}, false, nil
	}
	return w, m.lastUpdated[providerName], true, nil
}

func newTestProvider(t *testing.T) (*Provider, *memStore, *memRateLimitStore) {
	t.Helper()
	p := New()
	ms := newMemStore()
	rl := newMemRateLimitStore()
	p.SetStore(ms)
	p.SetRateLimitStore(rl)
	p.now = func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }
	return p, ms, rl
}

const testSessionID = "a1b2c3d4-0000-4444-8888-abcdef123456"

func TestHandleHook_StateTransitions(t *testing.T) {
	cases := []struct {
		fixture   string
		event     string
		wantState status.State
	}{
		{"hooks/session_start.json", "SessionStart", status.StateActive},
		{"hooks/user_prompt_submit.json", "UserPromptSubmit", status.StateActive},
		{"hooks/pre_tool_use.json", "PreToolUse", status.StateActive},
		{"hooks/post_tool_use.json", "PostToolUse", status.StateActive},
		{"hooks/notification.json", "Notification", status.StateDone},
		{"hooks/notification_permission.json", "Notification", status.StateBlocked},
		{"hooks/stop.json", "Stop", status.StateDone},
		{"hooks/pre_compact.json", "PreCompact", status.StateActive}, // no-op if already active
		{"hooks/session_end.json", "SessionEnd", status.StateStopped},
	}

	for _, tc := range cases {
		t.Run(tc.event, func(t *testing.T) {
			p, ms, _ := newTestProvider(t)
			// Seed an existing active session for events that shouldn't
			// change state (SubagentStop/PreCompact) or that transition
			// from active.
			ms.sessions[testSessionID] = status.Status{
				SessionID: testSessionID,
				Provider:  providerName,
				State:     status.StateActive,
			}

			payload := testutil.LoadFixture(t, tc.fixture)
			st, err := p.HandleHook(context.Background(), tc.event, bytes.NewReader(payload))
			if err != nil {
				t.Fatalf("HandleHook(%s): %v", tc.event, err)
			}
			if st.State != tc.wantState {
				t.Fatalf("HandleHook(%s) State = %q, want %q", tc.event, st.State, tc.wantState)
			}
			if st.SessionID != testSessionID {
				t.Fatalf("SessionID = %q, want %q", st.SessionID, testSessionID)
			}
			if st.WorkingDir != "/home/dev/myproj" {
				t.Fatalf("WorkingDir = %q, want /home/dev/myproj", st.WorkingDir)
			}

			saved, existed, err := ms.Load(testSessionID)
			if err != nil || !existed {
				t.Fatalf("expected session persisted to store: existed=%v err=%v", existed, err)
			}
			if saved.State != tc.wantState {
				t.Fatalf("persisted State = %q, want %q", saved.State, tc.wantState)
			}
		})
	}
}

func TestHandleHook_SubagentStopDoesNotChangeState(t *testing.T) {
	p, ms, _ := newTestProvider(t)
	ms.sessions[testSessionID] = status.Status{SessionID: testSessionID, Provider: providerName, State: status.StateActive}

	payload := testutil.LoadFixture(t, "hooks/subagent_stop.json")
	st, err := p.HandleHook(context.Background(), "SubagentStop", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}
	if st.State != status.StateActive {
		t.Fatalf("State = %q, want unchanged %q", st.State, status.StateActive)
	}
}

func TestHandleHook_SessionStart_CreatesNewRecord(t *testing.T) {
	p, ms, _ := newTestProvider(t)
	payload := testutil.LoadFixture(t, "hooks/session_start.json")

	st, err := p.HandleHook(context.Background(), "SessionStart", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}
	if st.State != status.StateActive {
		t.Fatalf("State = %q, want active", st.State)
	}
	if st.Provider != providerName {
		t.Fatalf("Provider = %q, want %q", st.Provider, providerName)
	}
	if len(ms.sessions) != 1 {
		t.Fatalf("expected exactly 1 session created, got %d", len(ms.sessions))
	}
}

func TestHandleHook_UserPromptSubmit_SetsFallbackSummary(t *testing.T) {
	p, ms, _ := newTestProvider(t)
	ms.sessions[testSessionID] = status.Status{SessionID: testSessionID, Provider: providerName, State: status.StateActive}

	payload := testutil.LoadFixture(t, "hooks/user_prompt_submit.json")
	st, err := p.HandleHook(context.Background(), "UserPromptSubmit", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}
	if st.TaskSummary == "" {
		t.Fatalf("TaskSummary not set from prompt fallback")
	}
}

func TestHandleHook_Notification_RecordsMessage(t *testing.T) {
	p, ms, _ := newTestProvider(t)
	ms.sessions[testSessionID] = status.Status{SessionID: testSessionID, Provider: providerName, State: status.StateActive}

	payload := testutil.LoadFixture(t, "hooks/notification.json")
	st, err := p.HandleHook(context.Background(), "Notification", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}
	if st.Extra["notification_message"] != "Claude is waiting for your input" {
		t.Fatalf("Extra[notification_message] = %v", st.Extra["notification_message"])
	}
	if st.State != status.StateDone {
		t.Fatalf("State = %q, want done (idle nudge, not a permission request)", st.State)
	}
}

func TestHandleHook_Notification_PermissionRequest_SetsBlocked(t *testing.T) {
	p, ms, _ := newTestProvider(t)
	ms.sessions[testSessionID] = status.Status{SessionID: testSessionID, Provider: providerName, State: status.StateActive}

	payload := testutil.LoadFixture(t, "hooks/notification_permission.json")
	st, err := p.HandleHook(context.Background(), "Notification", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}
	if st.State != status.StateBlocked {
		t.Fatalf("State = %q, want blocked (permission request)", st.State)
	}
}

func TestHandleHook_Stop_AlwaysConcludesTurnEvenAfterBlocked(t *testing.T) {
	// Once the user resolves a permission request, Claude Code always
	// fires Stop when the turn concludes, so Stop should unconditionally
	// move a Blocked session to Done.
	p, ms, _ := newTestProvider(t)
	ms.sessions[testSessionID] = status.Status{SessionID: testSessionID, Provider: providerName, State: status.StateBlocked}

	payload := testutil.LoadFixture(t, "hooks/stop.json")
	st, err := p.HandleHook(context.Background(), "Stop", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}
	if st.State != status.StateDone {
		t.Fatalf("State = %q, want done (Stop always means the turn concluded)", st.State)
	}
}

func TestHandleHook_Stop_KeepsBlockedAfterExitPlanMode(t *testing.T) {
	// Presenting a plan is normally the last thing that happens in a turn -
	// Stop fires right after, well before the user has approved or rejected
	// it, so Stop must not downgrade this to Done the way it would for a
	// genuinely-concluded turn.
	p, _, _ := newTestProvider(t)

	preToolUse := testutil.LoadFixture(t, "hooks/pre_tool_use_exit_plan_mode.json")
	if _, err := p.HandleHook(context.Background(), "PreToolUse", bytes.NewReader(preToolUse)); err != nil {
		t.Fatalf("HandleHook(PreToolUse): %v", err)
	}

	stop := testutil.LoadFixture(t, "hooks/stop.json")
	st, err := p.HandleHook(context.Background(), "Stop", bytes.NewReader(stop))
	if err != nil {
		t.Fatalf("HandleHook(Stop): %v", err)
	}
	if st.State != status.StateBlocked {
		t.Fatalf("State = %q, want blocked (plan awaiting approval)", st.State)
	}
}

func TestHandleHook_Stop_ResolvesDoneAfterPlanApprovedAndToolRuns(t *testing.T) {
	// Once the user approves the plan, Claude Code resumes with real tool
	// calls; the next PreToolUse for a different tool should clear the
	// awaiting-approval flag so a later Stop concludes the turn normally.
	p, _, _ := newTestProvider(t)

	preToolUse := testutil.LoadFixture(t, "hooks/pre_tool_use_exit_plan_mode.json")
	if _, err := p.HandleHook(context.Background(), "PreToolUse", bytes.NewReader(preToolUse)); err != nil {
		t.Fatalf("HandleHook(PreToolUse ExitPlanMode): %v", err)
	}

	nextTool := testutil.LoadFixture(t, "hooks/pre_tool_use.json")
	if _, err := p.HandleHook(context.Background(), "PreToolUse", bytes.NewReader(nextTool)); err != nil {
		t.Fatalf("HandleHook(PreToolUse Bash): %v", err)
	}

	stop := testutil.LoadFixture(t, "hooks/stop.json")
	st, err := p.HandleHook(context.Background(), "Stop", bytes.NewReader(stop))
	if err != nil {
		t.Fatalf("HandleHook(Stop): %v", err)
	}
	if st.State != status.StateDone {
		t.Fatalf("State = %q, want done (plan was approved, real work resumed)", st.State)
	}
}

func TestHandleHook_Stop_ResolvesDoneAfterPlanFeedback(t *testing.T) {
	// If the user replies with feedback instead of approving, the next
	// UserPromptSubmit should clear the awaiting-approval flag too.
	p, _, _ := newTestProvider(t)

	preToolUse := testutil.LoadFixture(t, "hooks/pre_tool_use_exit_plan_mode.json")
	if _, err := p.HandleHook(context.Background(), "PreToolUse", bytes.NewReader(preToolUse)); err != nil {
		t.Fatalf("HandleHook(PreToolUse ExitPlanMode): %v", err)
	}

	prompt := testutil.LoadFixture(t, "hooks/user_prompt_submit.json")
	if _, err := p.HandleHook(context.Background(), "UserPromptSubmit", bytes.NewReader(prompt)); err != nil {
		t.Fatalf("HandleHook(UserPromptSubmit): %v", err)
	}

	stop := testutil.LoadFixture(t, "hooks/stop.json")
	st, err := p.HandleHook(context.Background(), "Stop", bytes.NewReader(stop))
	if err != nil {
		t.Fatalf("HandleHook(Stop): %v", err)
	}
	if st.State != status.StateDone {
		t.Fatalf("State = %q, want done (plan feedback given, awaiting flag cleared)", st.State)
	}
}

func TestHandleHook_Stop_StaysActiveWhileSubagentStillRunning(t *testing.T) {
	// A subagent launched to run in the background can still be working
	// after the parent turn's own Stop fires - that isn't the parent being
	// done, since the subagent's own completion (SubagentStop) hasn't
	// happened yet.
	p, _, _ := newTestProvider(t)

	preToolUse := testutil.LoadFixture(t, "hooks/pre_tool_use_task.json")
	if _, err := p.HandleHook(context.Background(), "PreToolUse", bytes.NewReader(preToolUse)); err != nil {
		t.Fatalf("HandleHook(PreToolUse Task): %v", err)
	}

	stop := testutil.LoadFixture(t, "hooks/stop.json")
	st, err := p.HandleHook(context.Background(), "Stop", bytes.NewReader(stop))
	if err != nil {
		t.Fatalf("HandleHook(Stop): %v", err)
	}
	if st.State != status.StateActive {
		t.Fatalf("State = %q, want active (subagent still in flight)", st.State)
	}
}

func TestHandleHook_Stop_ResolvesDoneAfterSubagentCompletes(t *testing.T) {
	// Once the subagent's own SubagentStop fires, the pending count clears
	// and a later Stop can conclude the turn normally.
	p, _, _ := newTestProvider(t)

	preToolUse := testutil.LoadFixture(t, "hooks/pre_tool_use_task.json")
	if _, err := p.HandleHook(context.Background(), "PreToolUse", bytes.NewReader(preToolUse)); err != nil {
		t.Fatalf("HandleHook(PreToolUse Task): %v", err)
	}

	subagentStop := testutil.LoadFixture(t, "hooks/subagent_stop.json")
	if _, err := p.HandleHook(context.Background(), "SubagentStop", bytes.NewReader(subagentStop)); err != nil {
		t.Fatalf("HandleHook(SubagentStop): %v", err)
	}

	stop := testutil.LoadFixture(t, "hooks/stop.json")
	st, err := p.HandleHook(context.Background(), "Stop", bytes.NewReader(stop))
	if err != nil {
		t.Fatalf("HandleHook(Stop): %v", err)
	}
	if st.State != status.StateDone {
		t.Fatalf("State = %q, want done (subagent finished before Stop)", st.State)
	}
}

func TestHandleHook_Stop_StaysActiveWithMultipleSubagentsUntilAllComplete(t *testing.T) {
	// Two subagents launched, only one reports SubagentStop before Stop:
	// the parent must not be marked done while the other is still running.
	p, _, _ := newTestProvider(t)

	preToolUse := testutil.LoadFixture(t, "hooks/pre_tool_use_task.json")
	for i := 0; i < 2; i++ {
		if _, err := p.HandleHook(context.Background(), "PreToolUse", bytes.NewReader(preToolUse)); err != nil {
			t.Fatalf("HandleHook(PreToolUse Task #%d): %v", i, err)
		}
	}

	subagentStop := testutil.LoadFixture(t, "hooks/subagent_stop.json")
	if _, err := p.HandleHook(context.Background(), "SubagentStop", bytes.NewReader(subagentStop)); err != nil {
		t.Fatalf("HandleHook(SubagentStop): %v", err)
	}

	stop := testutil.LoadFixture(t, "hooks/stop.json")
	st, err := p.HandleHook(context.Background(), "Stop", bytes.NewReader(stop))
	if err != nil {
		t.Fatalf("HandleHook(Stop): %v", err)
	}
	if st.State != status.StateActive {
		t.Fatalf("State = %q, want active (one of two subagents still in flight)", st.State)
	}
}

func TestHandleHook_MissingSessionID_Errors(t *testing.T) {
	p, _, _ := newTestProvider(t)
	_, err := p.HandleHook(context.Background(), "SessionStart", bytes.NewReader([]byte(`{"hook_event_name":"SessionStart"}`)))
	if err == nil {
		t.Fatalf("expected error for missing session_id")
	}
}

func TestHandleHook_NoStoreConfigured_Errors(t *testing.T) {
	p := New()
	_, err := p.HandleHook(context.Background(), "SessionStart", bytes.NewReader(testutil.LoadFixture(t, "hooks/session_start.json")))
	if err == nil {
		t.Fatalf("expected error when store is not configured")
	}
}

func TestDetect_ReturnsBool(t *testing.T) {
	p := New()
	installed, detail := p.Detect()
	if detail == "" {
		t.Fatalf("Detect() detail is empty")
	}
	_ = installed
}
