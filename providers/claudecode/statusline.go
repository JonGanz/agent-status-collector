package claudecode

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/JonGanz/agent-status-collector/internal/status"
)

// handleStatusLine merges a Claude Code statusline payload into the stored
// Status for its session_id. Statusline payloads never carry lifecycle
// state; only Context/Cost/RateLimit/model fields are updated.
func (p *Provider) handleStatusLine(raw []byte) (status.Status, error) {
	var sp statusLinePayload
	if err := json.Unmarshal(raw, &sp); err != nil {
		return status.Status{}, fmt.Errorf("claudecode: decode statusline payload: %w", err)
	}
	if sp.SessionID == "" {
		return status.Status{}, fmt.Errorf("claudecode: statusline payload missing session_id")
	}

	st, err := p.loadOrInit(sp.SessionID)
	if err != nil {
		return status.Status{}, err
	}
	st.LastUpdated = p.now()
	st.Extra = ensureMap(st.Extra)

	if sp.ContextWindow.ContextWindowSize > 0 {
		st.Context = &status.ContextUsage{
			UsedTokens:  sp.ContextWindow.TotalInputTokens,
			MaxTokens:   sp.ContextWindow.ContextWindowSize,
			PercentUsed: sp.ContextWindow.UsedPercentage,
		}
	}
	st.Cost = &status.CostInfo{SessionUSD: sp.Cost.TotalCostUSD, Currency: "USD"}

	var windows []status.RateLimitWindow
	if w := toRateLimitWindow("5h", sp.RateLimits.FiveHour); w != nil {
		windows = append(windows, *w)
	}
	if w := toRateLimitWindow("7d", sp.RateLimits.SevenDay); w != nil {
		windows = append(windows, *w)
	}
	if len(windows) > 0 {
		st.RateLimit = &status.RateLimitInfo{Windows: windows}
	}

	if sp.Model.DisplayName != "" {
		st.Extra["model"] = sp.Model.DisplayName
	}

	if err := p.store.Save(st); err != nil {
		return status.Status{}, err
	}
	return st, nil
}

func toRateLimitWindow(label string, w *rateLimitWindowPayload) *status.RateLimitWindow {
	if w == nil {
		return nil
	}
	out := &status.RateLimitWindow{Label: label, PercentUsed: w.UsedPercentage}
	if w.ResetsAt != nil {
		t := time.Unix(*w.ResetsAt, 0).UTC()
		out.ResetsAt = &t
	}
	return out
}
