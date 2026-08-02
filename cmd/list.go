package cmd

import (
	"github.com/spf13/cobra"

	"github.com/JonGanz/agent-status-collector/internal/render"
	"github.com/JonGanz/agent-status-collector/internal/store"
)

var (
	listAll      bool
	listStale    bool
	listProvider string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List locally-tracked agent sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := newStore()
		entries, err := s.List()
		if err != nil {
			return err
		}

		filtered := make([]store.Entry, 0, len(entries))
		for _, e := range entries {
			if listProvider != "" && e.Status.Provider != listProvider {
				continue
			}
			if listStale && !e.Stale {
				continue
			}
			if !listAll && !listStale && e.Stale {
				continue
			}
			filtered = append(filtered, e)
		}

		if flagJSON {
			return render.JSON(cmd.OutOrStdout(), filtered)
		}
		return render.List(cmd.OutOrStdout(), filtered)
	},
}

func init() {
	listCmd.Flags().BoolVar(&listAll, "all", false, "include stale/stopped sessions still on disk")
	listCmd.Flags().BoolVar(&listStale, "stale", false, "only show stale sessions")
	listCmd.Flags().StringVar(&listProvider, "provider", "", "only show sessions from this provider")
	rootCmd.AddCommand(listCmd)
}
