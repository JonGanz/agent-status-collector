// Package config persists small, sticky user settings (currently just the
// debug logging flag) so that non-interactive invocations of this tool
// (hooks fired by an agent in the background) pick up settings a user
// configured interactively.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/JonGanz/agent-status-collector/internal/xdg"
)

// Config is the persisted user configuration.
type Config struct {
	Debug bool `json:"debug"`
}

// Path returns the config file path under $XDG_CONFIG_HOME.
func Path() string {
	return filepath.Join(xdg.ConfigDir(), "config.json")
}

// Load reads the config file, returning a zero-value Config if it doesn't
// exist yet.
func Load() (Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Save writes the config file, creating its directory if needed.
func Save(c Config) error {
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
