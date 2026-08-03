// Package cmd implements the agent-status CLI.
package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/JonGanz/agent-status-collector/internal/config"
	"github.com/JonGanz/agent-status-collector/internal/ratelimit"
	"github.com/JonGanz/agent-status-collector/internal/store"
	"github.com/JonGanz/agent-status-collector/internal/xdg"

	_ "github.com/JonGanz/agent-status-collector/providers/claudecode"
)

var (
	flagJSON     bool
	flagDebug    bool
	flagStateDir string
	flagNoColor  bool
)

var rootCmd = &cobra.Command{
	Use:   "agent-status",
	Short: "Query the status of locally-running AI coding agents",
	Long: "agent-status is a single local entrypoint for querying the status of\n" +
		"multiple locally-running AI coding agent sessions (Claude Code, and\n" +
		"other providers as adapters are added), regardless of how each agent\n" +
		"reports its own status.",
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "output machine-readable JSON")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "enable verbose logging and retain hook/statusline payloads for this invocation")
	rootCmd.PersistentFlags().StringVar(&flagStateDir, "state-dir", "", "override the state directory (default: $XDG_STATE_HOME/agent-status-collector)")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable ANSI table coloring")
}

// sessionsDir resolves the sessions directory, honoring --state-dir.
func sessionsDir() string {
	if flagStateDir != "" {
		return filepath.Join(flagStateDir, "sessions")
	}
	return filepath.Join(xdg.StateDir(), "sessions")
}

// eventLogPath resolves the debug event log path, honoring --state-dir.
func eventLogPath() string {
	if flagStateDir != "" {
		return filepath.Join(flagStateDir, "logs", "events.jsonl")
	}
	return filepath.Join(xdg.StateDir(), "logs", "events.jsonl")
}

// newStore builds the Store used by all commands.
func newStore() *store.Store {
	return store.New(sessionsDir())
}

// rateLimitsDir resolves the account-level rate limit snapshot directory,
// honoring --state-dir.
func rateLimitsDir() string {
	if flagStateDir != "" {
		return filepath.Join(flagStateDir, "rate-limits")
	}
	return filepath.Join(xdg.StateDir(), "rate-limits")
}

// newRateLimitStore builds the rate limit Store used by all commands.
func newRateLimitStore() *ratelimit.Store {
	return ratelimit.New(rateLimitsDir())
}

// debugEnabled reports whether debug logging should be active for this
// invocation: the --debug flag takes precedence, then the persisted config.
func debugEnabled() bool {
	if flagDebug {
		return true
	}
	cfg, err := config.Load()
	if err != nil {
		return false
	}
	return cfg.Debug
}

// isTerminal reports whether f appears to be an interactive terminal.
func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
