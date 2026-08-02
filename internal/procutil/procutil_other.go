//go:build !unix

package procutil

import "os"

func isRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := os.FindProcess(pid)
	return err == nil
}
