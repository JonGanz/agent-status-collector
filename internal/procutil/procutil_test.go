package procutil

import (
	"os"
	"testing"
)

func TestIsRunning_CurrentProcess(t *testing.T) {
	if !IsRunning(os.Getpid()) {
		t.Fatalf("IsRunning(os.Getpid()) = false, want true")
	}
}

func TestIsRunning_InvalidPID(t *testing.T) {
	if IsRunning(0) {
		t.Fatalf("IsRunning(0) = true, want false")
	}
	if IsRunning(-1) {
		t.Fatalf("IsRunning(-1) = true, want false")
	}
}

func TestIsRunning_UnlikelyPID(t *testing.T) {
	// A PID far beyond any real process on typical systems. Best-effort;
	// not guaranteed portable, but reasonable for CI Linux containers.
	const unlikely = 999999
	if IsRunning(unlikely) {
		t.Skipf("PID %d unexpectedly exists on this system; skipping", unlikely)
	}
}
