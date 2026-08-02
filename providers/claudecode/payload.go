package claudecode

// hookPayload captures the fields common to every Claude Code hook event
// payload, plus the handful of event-specific fields this adapter reads.
// Unknown fields are ignored (forward-compatible with future Claude Code
// releases adding new fields).
type hookPayload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	HookEventName  string `json:"hook_event_name"`

	Message  string `json:"message,omitempty"`   // Notification
	Prompt   string `json:"prompt,omitempty"`    // UserPromptSubmit
	ToolName string `json:"tool_name,omitempty"` // PreToolUse / PostToolUse
}

// statusLinePayload captures the fields of a Claude Code statusline
// invocation payload that this adapter reports on.
type statusLinePayload struct {
	SessionID string `json:"session_id"`
	PromptID  string `json:"prompt_id"`
	Model     struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
	ContextWindow struct {
		UsedPercentage    float64 `json:"used_percentage"`
		TotalInputTokens  int     `json:"total_input_tokens"`
		ContextWindowSize int     `json:"context_window_size"`
	} `json:"context_window"`
	Cost struct {
		TotalCostUSD    float64 `json:"total_cost_usd"`
		TotalDurationMs int64   `json:"total_duration_ms"`
	} `json:"cost"`
	RateLimits struct {
		FiveHour *rateLimitWindowPayload `json:"five_hour,omitempty"`
		SevenDay *rateLimitWindowPayload `json:"seven_day,omitempty"`
	} `json:"rate_limits"`
}

type rateLimitWindowPayload struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       *int64  `json:"resets_at,omitempty"` // unix epoch seconds
}

// taskSummaryPayload is the synthetic payload used by the report-status
// skill (agent-status hook claudecode --event=TaskSummary --summary "...").
type taskSummaryPayload struct {
	SessionID string `json:"session_id,omitempty"`
	Summary   string `json:"summary"`
	Cwd       string `json:"cwd,omitempty"`
}
