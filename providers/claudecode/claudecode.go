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

// Provider implements provider.Provider and provider.StoreAware for
// Claude Code.
type Provider struct {
	store provider.SessionStore
	now   func() time.Time
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
	case "PreToolUse", "PostToolUse":
		st.State = status.StateActive
		if hp.ToolName != "" {
			st.Extra["last_tool"] = hp.ToolName
		}
	case "Notification":
		st.State = status.StateWaitingForInput
		if hp.Message != "" {
			st.Extra["notification_message"] = hp.Message
		}
	case "SubagentStop":
		// A subagent (Task-tool invocation) finished, not the main turn; no state change.
	case "Stop":
		st.State = status.StateIdle
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
