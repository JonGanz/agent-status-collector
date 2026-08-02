package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JonGanz/agent-status-collector/internal/render"
	"github.com/JonGanz/agent-status-collector/internal/store"
)

var showCmd = &cobra.Command{
	Use:   "show <session-id>",
	Short: "Show full detail for one tracked agent session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s := newStore()
		st, existed, err := s.Load(args[0])
		if err != nil {
			return err
		}
		if !existed {
			return fmt.Errorf("no session found with id %q", args[0])
		}
		entry := store.Entry{Status: st, Stale: s.IsStale(st)}
		if flagJSON {
			return render.JSON(cmd.OutOrStdout(), entry)
		}
		return render.Show(cmd.OutOrStdout(), entry)
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
