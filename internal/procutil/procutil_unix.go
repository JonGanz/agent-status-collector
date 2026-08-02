//go:build unix

package procutil

import (
	"os"
	"syscall"
)

func isRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, os.FindProcess always succeeds; sending signal 0 is the
	// actual liveness check (no signal delivered, just existence/permission
	// checked).
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
