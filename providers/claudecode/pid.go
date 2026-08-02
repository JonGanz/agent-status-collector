package claudecode

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// capturePID returns the calling process's parent PID, on the assumption
// that Setup() installs hook commands as direct child processes of Claude
// Code (no intervening shell) — see setup.go's ourHookCommand, which
// deliberately avoids "sh -c" wrapping so this holds. This is best-effort:
// if Claude Code ever invokes hooks through an intermediate process on some
// platform, this would capture the wrong PID. The store's TTL-based
// staleness fallback (internal/store) degrades gracefully if so.
//
// On Linux, /proc/<ppid>/comm is checked as a sanity guard: if the parent
// doesn't look like Claude Code's process (a Node binary, typically named
// "node" or "claude"), the PID is not reported, and the store falls back to
// TTL-based staleness instead of PID liveness.
func capturePID() (pid int, ok bool) {
	ppid := os.Getppid()
	if ppid <= 0 {
		return 0, false
	}
	if runtime.GOOS == "linux" {
		if !parentLooksLikeClaudeCode(ppid) {
			return 0, false
		}
	}
	return ppid, true
}

func parentLooksLikeClaudeCode(ppid int) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(ppid) + "/comm")
	if err != nil {
		// /proc unavailable or process gone by the time we checked; don't
		// block PID capture over an inconclusive sanity check.
		return true
	}
	comm := strings.ToLower(strings.TrimSpace(string(data)))
	return strings.Contains(comm, "node") || strings.Contains(comm, "claude")
}
