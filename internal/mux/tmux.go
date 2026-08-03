package mux

import (
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
	out, err := exec.Command("tmux", "display-message", "-p",
		"#{session_name}\t#{window_index}\t#{pane_index}\t#{session_id}\t#{window_id}\t#{pane_id}").Output()
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
