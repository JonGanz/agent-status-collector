package mux

import (
	"os"
	"os/exec"
	"strings"
)

// queryTmux best-effort queries the current tmux session/window/pane (both
// by name/index and by stable id) via `tmux display-message -p`, a read-only
// informational command (it does not create, attach, kill, or otherwise
// mutate any tmux state). If the tmux binary isn't on PATH or the query
// fails for any reason, it degrades gracefully to just Type: "tmux" with
// everything else empty.
func queryTmux() Info {
	info := Info{Type: "tmux"}
	if _, err := exec.LookPath("tmux"); err != nil {
		return info
	}

	// Without an explicit -t, `tmux display-message` resolves "current" to
	// the attached client's currently-focused pane, NOT the pane this
	// process is actually running in -- so as soon as the user looks at a
	// different window, the next query for an unrelated pane's hook/
	// statusline invocation would misreport that focused window/pane
	// instead of its own. $TMUX_PANE is set by tmux on every pane's shell
	// (and inherited by child processes, e.g. Claude Code's hook/statusline
	// subprocess) and always identifies the actual originating pane
	// regardless of focus, so target it explicitly when available.
	args := []string{"display-message", "-p"}
	if paneID := os.Getenv("TMUX_PANE"); paneID != "" {
		args = append(args, "-t", paneID)
	}
	args = append(args, "#{session_name}\t#{window_index}\t#{pane_index}\t#{session_id}\t#{window_id}\t#{pane_id}")

	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return info
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "\t")
	if len(parts) == 6 {
		info.Session, info.Window, info.Pane = parts[0], parts[1], parts[2]
		info.SessionID, info.WindowID, info.PaneID = parts[3], parts[4], parts[5]
	}
	return info
}
