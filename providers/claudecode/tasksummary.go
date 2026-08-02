package claudecode

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/JonGanz/agent-status-collector/internal/status"
)

// handleTaskSummary updates TaskSummary from the report-status skill
// invocation (agent-status hook claudecode --event=TaskSummary --summary
// "..."). The skill has no way to obtain its own session_id (Claude Code
// does not expose it to the model), so when session_id is absent this
// falls back to matching the current working directory against tracked
// sessions — a best-effort heuristic that can be ambiguous if multiple
// Claude Code sessions share the same cwd.
func (p *Provider) handleTaskSummary(raw []byte) (status.Status, error) {
	var tp taskSummaryPayload
	if err := json.Unmarshal(raw, &tp); err != nil {
		return status.Status{}, fmt.Errorf("claudecode: decode TaskSummary payload: %w", err)
	}
	if tp.Summary == "" {
		return status.Status{}, fmt.Errorf("claudecode: TaskSummary payload missing summary")
	}

	var st status.Status
	if tp.SessionID != "" {
		loaded, existed, err := p.store.Load(tp.SessionID)
		if err != nil {
			return status.Status{}, err
		}
		if !existed {
			return status.Status{}, fmt.Errorf("claudecode: no session %q to attach summary to", tp.SessionID)
		}
		st = loaded
	} else {
		cwd := tp.Cwd
		if cwd == "" {
			var err error
			cwd, err = os.Getwd()
			if err != nil {
				return status.Status{}, fmt.Errorf("claudecode: resolving cwd for TaskSummary: %w", err)
			}
		}
		match, err := p.findActiveSessionByCwd(cwd)
		if err != nil {
			return status.Status{}, err
		}
		if match == nil {
			return status.Status{}, fmt.Errorf("claudecode: no active session found for working directory %q", cwd)
		}
		st = *match
	}

	st.TaskSummary = tp.Summary
	st.LastUpdated = p.now()
	if err := p.store.Save(st); err != nil {
		return status.Status{}, err
	}
	return st, nil
}

// findActiveSessionByCwd returns the most recently updated, non-stopped
// session whose WorkingDir matches cwd, or nil if none match.
func (p *Provider) findActiveSessionByCwd(cwd string) (*status.Status, error) {
	all, err := p.store.LoadAll()
	if err != nil {
		return nil, err
	}
	var best *status.Status
	for i := range all {
		st := all[i]
		if st.Provider != providerName || st.WorkingDir != cwd || st.State == status.StateStopped {
			continue
		}
		if best == nil || st.LastUpdated.After(best.LastUpdated) {
			best = &all[i]
		}
	}
	return best, nil
}
