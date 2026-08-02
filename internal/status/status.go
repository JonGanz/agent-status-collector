// Package status defines the unified status shape every provider adapter
// produces, regardless of which agent (Claude Code, Copilot, etc.) reported it.
package status

import "time"

// CurrentSchemaVersion is written into every new Status. It lets the store
// detect old on-disk formats without a migration framework.
const CurrentSchemaVersion = 1

// State is the high-level activity state of an agent session.
type State string

const (
	StateActive          State = "active"
	StateIdle            State = "idle"
	StateWaitingForInput State = "waiting_for_input"
	StateStopped         State = "stopped"
	StateUnknown         State = "unknown"
)

// Status is the single unified shape every provider adapter must produce.
type Status struct {
	SchemaVersion int              `json:"schema_version"`
	SessionID     string           `json:"session_id"`
	Provider      string           `json:"provider"`
	PID           *int             `json:"pid,omitempty"`
	State         State            `json:"state"`
	TaskSummary   string           `json:"task_summary,omitempty"`
	Context       *ContextUsage    `json:"context,omitempty"`
	Cost          *CostInfo        `json:"cost,omitempty"`
	RateLimit     *RateLimitInfo   `json:"rate_limit,omitempty"`
	Multiplexer   *MultiplexerInfo `json:"multiplexer,omitempty"`
	StartedAt     time.Time        `json:"started_at"`
	LastUpdated   time.Time        `json:"last_updated"`
	WorkingDir    string           `json:"working_dir,omitempty"`
	DebugLogPath  string           `json:"debug_log_path,omitempty"`
	Extra         map[string]any   `json:"extra,omitempty"`
}

type ContextUsage struct {
	UsedTokens  int     `json:"used_tokens,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	PercentUsed float64 `json:"percent_used"`
}

type CostInfo struct {
	SessionUSD float64 `json:"session_usd,omitempty"`
	Currency   string  `json:"currency,omitempty"`
}

type RateLimitInfo struct {
	Windows []RateLimitWindow `json:"windows,omitempty"`
}

type RateLimitWindow struct {
	Label       string     `json:"label"`
	PercentUsed float64    `json:"percent_used"`
	ResetsAt    *time.Time `json:"resets_at,omitempty"`
}

type MultiplexerInfo struct {
	Type    string `json:"type,omitempty"`
	Session string `json:"session,omitempty"`
	Window  string `json:"window,omitempty"`
	Pane    string `json:"pane,omitempty"`
}

// IsStopped reports whether the session has reached a terminal state.
func (s Status) IsStopped() bool {
	return s.State == StateStopped
}
