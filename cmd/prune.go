package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pruneDryRun bool

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete stale session records from local storage",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := newStore()
		if pruneDryRun {
			entries, err := s.List()
			if err != nil {
				return err
			}
			for _, e := range entries {
				if s.ShouldDelete(e.Status) {
					fmt.Fprintf(cmd.OutOrStdout(), "would remove: %s (%s)\n", e.Status.SessionID, e.Status.Provider)
				}
			}
			return nil
		}
		removed, err := s.Prune()
		if err != nil {
			return err
		}
		for _, id := range removed {
			fmt.Fprintf(cmd.OutOrStdout(), "removed: %s\n", id)
		}
		return nil
	},
}

func init() {
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "show what would be removed without deleting")
	rootCmd.AddCommand(pruneCmd)
}
