// Package claudecode implements the agent-status-collector provider
// adapter for Claude Code, using its documented hooks and statusline
// integration surface. It never parses Claude Code's internal session
// transcripts (those are explicitly unstable/undocumented format).
package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/JonGanz/agent-status-collector/internal/provider"
	"github.com/JonGanz/agent-status-collector/internal/status"
)

const providerName = "claudecode"

// requiredHookEvents lists every hook event Setup wires up and IsConfigured
// checks for.
var requiredHookEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
	"Notification", "Stop", "SubagentStop", "PreCompact", "SessionEnd",
}

// Provider implements provider.Provider, provider.StoreAware, and
// provider.RateLimitStoreAware for Claude Code.
type Provider struct {
	store      provider.SessionStore
	rateLimits provider.RateLimitStore
	now        func() time.Time
}

// New constructs a Claude Code provider. Call SetStore before HandleHook.
func New() *Provider {
	return &Provider{now: time.Now}
}

func init() {
	provider.Register(New())
}

func (p *Provider) Name() string { return providerName }

// SetStore implements provider.StoreAware.
func (p *Provider) SetStore(s provider.SessionStore) { p.store = s }

// SetRateLimitStore implements provider.RateLimitStoreAware.
func (p *Provider) SetRateLimitStore(s provider.RateLimitStore) { p.rateLimits = s }

func (p *Provider) Detect() (installed bool, detail string) {
	if path, err := exec.LookPath("claude"); err == nil {
		return true, "found on PATH: " + path
	}
	home, err := os.UserHomeDir()
	if err == nil {
		if fi, statErr := os.Stat(home + "/.claude"); statErr == nil && fi.IsDir() {
			return true, "~/.claude directory present (claude binary not on PATH)"
		}
	}
	return false, "claude not found on PATH and ~/.claude does not exist"
}

// HandleHook implements provider.Provider. It loads any existing stored
// Status for the session, applies the event-specific partial update, and
// persists the merged result via the configured store.
func (p *Provider) HandleHook(ctx context.Context, event string, payload io.Reader) (status.Status, error) {
	if p.store == nil {
		return status.Status{}, fmt.Errorf("claudecode: no store configured (SetStore was not called)")
	}

	raw, err := io.ReadAll(payload)
	if err != nil {
		return status.Status{}, fmt.Errorf("claudecode: read payload: %w", err)
	}

	switch event {
	case "StatusLine":
		return p.handleStatusLine(raw)
	case "TaskSummary":
		return p.handleTaskSummary(raw)
	default:
		return p.handleLifecycleHook(event, raw)
	}
}

func (p *Provider) handleLifecycleHook(event string, raw []byte) (status.Status, error) {
	var hp hookPayload
	if err := json.Unmarshal(raw, &hp); err != nil {
		return status.Status{}, fmt.Errorf("claudecode: decode %s payload: %w", event, err)
	}
	if hp.SessionID == "" {
		return status.Status{}, fmt.Errorf("claudecode: %s payload missing session_id", event)
	}

	st, err := p.loadOrInit(hp.SessionID)
	if err != nil {
		return status.Status{}, err
	}

	if hp.Cwd != "" {
		st.WorkingDir = hp.Cwd
	}
	st.LastUpdated = p.now()
	st.Extra = ensureMap(st.Extra)
	if hp.TranscriptPath != "" {
		st.Extra["transcript_path"] = hp.TranscriptPath
	}

	switch event {
	case "SessionStart":
		st.State = status.StateActive
		st.StartedAt = p.now()
		if pid, ok := capturePID(); ok {
			st.PID = &pid
		}
	case "UserPromptSubmit":
		st.State = status.StateActive
		if hp.Prompt != "" {
			st.TaskSummary = summarize(hp.Prompt)
		}
		delete(st.Extra, "awaiting_plan_approval")
	case "PreToolUse", "PostToolUse":
		st.State = status.StateActive
		if hp.ToolName != "" {
			st.Extra["last_tool"] = hp.ToolName
		}
		if hp.ToolName == "ExitPlanMode" {
			st.Extra["awaiting_plan_approval"] = true
		} else {
			delete(st.Extra, "awaiting_plan_approval")
		}
		if event == "PreToolUse" && hp.ToolName == "Task" {
			// A subagent is being launched. Its own SubagentStop hook fires
			// whenever it truly finishes, which for a background-run
			// subagent can be well after this turn's Stop, so track it as
			// still in flight until then rather than letting Stop declare
			// the parent done just because its own turn ended.
			st.Extra["pending_subagents"] = pendingSubagents(st) + 1
		}
	case "Notification":
		// Claude Code fires Notification for two very different reasons,
		// distinguishable only by the free-text message (there's no
		// structured flag): a genuine permission request (StateBlocked —
		// something only you can decide), or an idle reminder ~60s after
		// Stop already completed the turn (not actually blocked on
		// anything — just still StateDone). This heuristic is inherently
		// a little fragile since Claude Code doesn't document the message
		// wording as a stable API, but it's the only signal available.
		if isPermissionRequest(hp.Message) {
			st.State = status.StateBlocked
		} else {
			st.State = status.StateDone
		}
		if hp.Message != "" {
			st.Extra["notification_message"] = hp.Message
		}
	case "SubagentStop":
		// The subagent that just finished never directly sets State (its
		// own completion isn't the parent turn concluding) - but it may be
		// the last one Stop was waiting on, so clear it from the pending
		// count.
		if n := pendingSubagents(st) - 1; n > 0 {
			st.Extra["pending_subagents"] = n
		} else {
			delete(st.Extra, "pending_subagents")
		}
	case "Stop":
		// Normally Stop means the turn genuinely concluded (a permission
		// prompt, if any, already got resolved before Stop fires - see
		// TestHandleHook_Stop_AlwaysConcludesTurnEvenAfterBlocked). ExitPlanMode
		// doesn't fit that: presenting a plan IS the end of the turn, and
		// only the user's next action (approve or give feedback) resumes
		// anything, so Stop must not downgrade an awaiting-approval plan to
		// StateDone. Likewise, a subagent launched to run in the background
		// can still be working well after this turn's Stop fires - that
		// isn't the parent being done either, since its own completion
		// hasn't been reported yet.
		awaitingPlan, _ := st.Extra["awaiting_plan_approval"].(bool)
		switch {
		case awaitingPlan:
			st.State = status.StateBlocked
		case pendingSubagents(st) > 0:
			st.State = status.StateActive
		default:
			st.State = status.StateDone
		}
	case "PreCompact":
		st.Extra["last_compact_at"] = p.now().Format(time.RFC3339)
	case "SessionEnd":
		st.State = status.StateStopped
	default:
		// Unknown/future event: still record the touch (cwd/timestamp above)
		// rather than failing the hook chain.
	}

	if err := p.store.Save(st); err != nil {
		return status.Status{}, err
	}
	return st, nil
}

func (p *Provider) loadOrInit(sessionID string) (status.Status, error) {
	prev, existed, err := p.store.Load(sessionID)
	if err != nil {
		return status.Status{}, err
	}
	if !existed {
		prev = status.Status{
			SchemaVersion: status.CurrentSchemaVersion,
			SessionID:     sessionID,
			Provider:      providerName,
			State:         status.StateUnknown,
			StartedAt:     p.now(),
		}
	}
	return prev, nil
}

func ensureMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// pendingSubagents reads the "pending_subagents" counter out of st.Extra.
// It's stored as an int when set in-process, but round-trips through the
// JSON-backed store as a float64, so both are handled.
func pendingSubagents(st status.Status) int {
	switch n := st.Extra["pending_subagents"].(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

// summarize truncates a raw user prompt into a short fallback task summary,
// used until (or unless) the report-status skill provides an authoritative
// one.
func summarize(prompt string) string {
	const maxLen = 80
	s := prompt
	for i, r := range s {
		if r == '\n' {
			s = s[:i]
			break
		}
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-1]) + "…"
	}
	return s
}

// isPermissionRequest reports whether a Notification message looks like a
// tool-permission request rather than an idle-waiting reminder. Best-effort
// substring match on undocumented message text — see the Notification case
// in handleLifecycleHook for why this is the only signal available.
func isPermissionRequest(message string) bool {
	return strings.Contains(strings.ToLower(message), "permission")
}
