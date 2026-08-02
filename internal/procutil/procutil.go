// Package procutil provides best-effort process liveness checks used by the
// session store's PID-based staleness detection.
package procutil

// IsRunning reports whether a process with the given PID currently exists.
// It is best-effort: PID reuse by the OS means a "running" result does not
// guarantee it's still the *same* process the session recorded. Callers
// (internal/store) treat this as one signal among others (TTL fallback),
// never as sole authority.
func IsRunning(pid int) bool {
	return isRunning(pid)
}
