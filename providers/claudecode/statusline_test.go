package claudecode

import (
	"bytes"
	"context"
	"testing"

	"github.com/JonGanz/agent-status-collector/internal/status"
	"github.com/JonGanz/agent-status-collector/internal/testutil"
)

func TestHandleStatusLine_FullPayload(t *testing.T) {
	p, ms := newTestProvider(t)
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
	if st.RateLimit == nil || len(st.RateLimit.Windows) != 2 {
		t.Fatalf("RateLimit = %+v, want 2 windows", st.RateLimit)
	}
	if st.Extra["model"] != "Claude Sonnet 4.5" {
		t.Fatalf("Extra[model] = %v", st.Extra["model"])
	}
}

func TestHandleStatusLine_NoSevenDayRateLimit(t *testing.T) {
	p, ms := newTestProvider(t)
	ms.sessions[testSessionID] = status.Status{SessionID: testSessionID, Provider: providerName, State: status.StateActive}

	payload := testutil.LoadFixture(t, "statusline/no_seven_day_rate_limit.json")
	st, err := p.HandleHook(context.Background(), "StatusLine", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("HandleHook: %v", err)
	}
	if st.RateLimit == nil || len(st.RateLimit.Windows) != 1 {
		t.Fatalf("RateLimit = %+v, want exactly 1 window (five_hour only)", st.RateLimit)
	}
	if st.RateLimit.Windows[0].Label != "5h" {
		t.Fatalf("window label = %q, want 5h", st.RateLimit.Windows[0].Label)
	}
}

func TestHandleStatusLine_MissingSessionID_Errors(t *testing.T) {
	p, _ := newTestProvider(t)
	_, err := p.HandleHook(context.Background(), "StatusLine", bytes.NewReader([]byte(`{}`)))
	if err == nil {
		t.Fatalf("expected error for missing session_id")
	}
}
