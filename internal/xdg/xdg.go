// Package xdg resolves XDG Base Directory paths for this application,
// per the XDG Base Directory Specification.
package xdg

import (
	"os"
	"path/filepath"
)

// AppName is the directory segment appended under each XDG base dir.
const AppName = "agent-status-collector"

// homeDirFn is overridable in tests since os.UserHomeDir cannot be faked
// directly via t.Setenv on all platforms.
var homeDirFn = os.UserHomeDir

// StateDir returns $XDG_STATE_HOME/agent-status-collector, falling back to
// ~/.local/state/agent-status-collector.
func StateDir() string { return resolve("XDG_STATE_HOME", ".local/state") }

// ConfigDir returns $XDG_CONFIG_HOME/agent-status-collector, falling back to
// ~/.config/agent-status-collector.
func ConfigDir() string { return resolve("XDG_CONFIG_HOME", ".config") }

// DataDir returns $XDG_DATA_HOME/agent-status-collector, falling back to
// ~/.local/share/agent-status-collector.
func DataDir() string { return resolve("XDG_DATA_HOME", ".local/share") }

func resolve(envVar, fallbackRelToHome string) string {
	// Per spec: if the variable is set but not an absolute path, treat it as unset.
	if v := os.Getenv(envVar); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, AppName)
	}
	home, err := homeDirFn()
	if err != nil || home == "" {
		home = os.TempDir()
	}
	return filepath.Join(home, fallbackRelToHome, AppName)
}
