// Package mux detects whether the current process is running inside a
// terminal multiplexer (tmux, screen), so that info can be stamped onto a
// Status record for other tooling to key off of.
package mux

import (
	"os"

	"github.com/JonGanz/agent-status-collector/internal/status"
)

// Info mirrors status.MultiplexerInfo; kept as an alias so callers can use
// either name interchangeably.
type Info = status.MultiplexerInfo

// tmuxQuerier is overridable in tests so they never depend on a real tmux
// binary/session being present.
var tmuxQuerier = queryTmux

// Detect inspects the current process's environment (and, for tmux, shells
// out to `tmux display-message`) to determine multiplexer context. It is
// read-only and side-effect free: it never creates, kills, or modifies any
// multiplexer session/window/pane.
func Detect() Info {
	if os.Getenv("TMUX") != "" {
		return tmuxQuerier()
	}
	if sty := os.Getenv("STY"); sty != "" {
		return Info{Type: "screen", Session: sty}
	}
	return Info{}
}
