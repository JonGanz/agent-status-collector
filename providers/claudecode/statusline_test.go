package claudecode

import (
	"bytes"
	"context"
	"testing"

	"github.com/JonGanz/agent-status-collector/internal/status"
	"github.com/JonGanz/agent-status-collector/internal/testutil"
)

func TestHandleStatusLine_FullPayload(t *testing.T) {
	p, ms, rl := newTestProvider(t)
	ms.sessions[testSessionID] = status.Status{
		SessionID: testSessionID,
		Provider:  providerName,
		State:     status.StateActive,
	}

	payload := testutil.LoadFixture(t, "statusline/full.json")
	st, err := p.HandleHook(context.Background(), "StatusLine", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}

	if st.State != status.StateActive {
		t.Fatalf("StatusLine event must never change State; got %q", st.State)
	}
	if st.Context == nil || st.Context.PercentUsed != 42.5 {
		t.Fatalf("Context = %+v, want PercentUsed=42.5", st.Context)
	}
	if st.Context.MaxTokens != 200000 || st.Context.UsedTokens != 85000 {
		t.Fatalf("Context tokens = %+v", st.Context)
	}
	if st.Cost == nil || st.Cost.SessionUSD != 0.3421 {
		t.Fatalf("Cost = %+v", st.Cost)
	}
	if st.Extra["model"] != "Claude Sonnet 4.5" {
		t.Fatalf("Extra[model] = %v", st.Extra["model"])
	}

	// Rate limits are account-level, not session-level: they must land in
	// the rate limit store, not on the session Status.
	windows, _, ok, err := rl.LoadRateLimits(providerName)
	if err != nil {
		t.Fatalf("LoadRateLimits: %v", err)
	}
	if !ok || len(windows) != 2 {
		t.Fatalf("rate limit store windows = %+v, want 2", windows)
	}
}

func TestHandleStatusLine_NoSevenDayRateLimit(t *testing.T) {
	p, ms, rl := newTestProvider(t)
	ms.sessions[testSessionID] = status.Status{SessionID: testSessionID, Provider: providerName, State: status.StateActive}

	payload := testutil.LoadFixture(t, "statusline/no_seven_day_rate_limit.json")
	_, err := p.HandleHook(context.Background(), "StatusLine", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}
	windows, _, ok, err := rl.LoadRateLimits(providerName)
	if err != nil {
		t.Fatalf("LoadRateLimits: %v", err)
	}
	if !ok || len(windows) != 1 {
		t.Fatalf("rate limit store windows = %+v, want exactly 1 (five_hour only)", windows)
	}
	if windows[0].Label != "5h" {
		t.Fatalf("window label = %q, want 5h", windows[0].Label)
	}
}

func TestHandleStatusLine_MissingSessionID_Errors(t *testing.T) {
	p, _, _ := newTestProvider(t)
	_, err := p.HandleHook(context.Background(), "StatusLine", bytes.NewReader([]byte(`{}`)))
	if err == nil {
		t.Fatalf("expected error for missing session_id")
	}
}

func TestHandleStatusLine_NoRateLimitStoreConfigured_StillSucceeds(t *testing.T) {
	// A provider that never had SetRateLimitStore called (e.g. a future
	// provider that doesn't implement RateLimitStoreAware at all) must
	// still be able to handle statusline payloads for Context/Cost/model.
	p := New()
	ms := newMemStore()
	p.SetStore(ms)
	ms.sessions[testSessionID] = status.Status{SessionID: testSessionID, Provider: providerName, State: status.StateActive}

	payload := testutil.LoadFixture(t, "statusline/full.json")
	st, err := p.HandleHook(context.Background(), "StatusLine", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}
	if st.Context == nil {
		t.Fatalf("Context not set even without a rate limit store configured")
	}
}
