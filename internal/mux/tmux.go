package mux

import (
	"os/exec"
	"strings"
)

// queryTmux best-effort queries the current tmux session/window/pane via
// `tmux display-message -p`, a read-only informational command (it does not
// create, attach, kill, or otherwise mutate any tmux state). If the tmux
// binary isn't on PATH or the query fails for any reason, it degrades
// gracefully to just Type: "tmux" with empty session/window/pane.
func queryTmux() Info {
	info := Info{Type: "tmux"}
	if _, err := exec.LookPath("tmux"); err != nil {
		return info
	}
	out, err := exec.Command("tmux", "display-message", "-p",
		"#{session_name}\t#{window_index}\t#{pane_index}").Output()
	if err != nil {
		return info
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "\t")
	if len(parts) == 3 {
		info.Session, info.Window, info.Pane = parts[0], parts[1], parts[2]
	}
	return info
}
