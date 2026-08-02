package xdg

import (
	"path/filepath"
	"testing"
)

func TestStateDir_UsesEnvWhenAbsolute(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdgtest/state")
	got := StateDir()
	want := filepath.Join("/tmp/xdgtest/state", AppName)
	if got != want {
		t.Fatalf("StateDir() = %q, want %q", got, want)
	}
}

func TestStateDir_IgnoresRelativeEnv(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "relative/path")
	origHomeDirFn := homeDirFn
	homeDirFn = func() (string, error) { return "/home/tester", nil }
	defer func() { homeDirFn = origHomeDirFn }()

	got := StateDir()
	want := filepath.Join("/home/tester", ".local/state", AppName)
	if got != want {
		t.Fatalf("StateDir() = %q, want %q (relative env var must be ignored per XDG spec)", got, want)
	}
}

func TestStateDir_FallsBackToHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	origHomeDirFn := homeDirFn
	homeDirFn = func() (string, error) { return "/home/tester", nil }
	defer func() { homeDirFn = origHomeDirFn }()

	got := StateDir()
	want := filepath.Join("/home/tester", ".local/state", AppName)
	if got != want {
		t.Fatalf("StateDir() = %q, want %q", got, want)
	}
}

func TestConfigDir_FallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	origHomeDirFn := homeDirFn
	homeDirFn = func() (string, error) { return "/home/tester", nil }
	defer func() { homeDirFn = origHomeDirFn }()

	got := ConfigDir()
	want := filepath.Join("/home/tester", ".config", AppName)
	if got != want {
		t.Fatalf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestDataDir_FallsBackToHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	origHomeDirFn := homeDirFn
	homeDirFn = func() (string, error) { return "/home/tester", nil }
	defer func() { homeDirFn = origHomeDirFn }()

	got := DataDir()
	want := filepath.Join("/home/tester", ".local/share", AppName)
	if got != want {
		t.Fatalf("DataDir() = %q, want %q", got, want)
	}
}
