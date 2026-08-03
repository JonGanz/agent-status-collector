package cmd

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/JonGanz/agent-status-collector/internal/ratelimit"
	"github.com/JonGanz/agent-status-collector/internal/render"
)

var rateLimitsProvider string

var rateLimitsCmd = &cobra.Command{
	Use:   "rate-limits",
	Short: "Show account-level rate limit usage reported by agents",
	Long: "Show account-level rate limit usage (e.g. Claude's 5h/7d windows).\n" +
		"Unlike `list`/`show`, this is not scoped to any individual session:\n" +
		"rate limits apply to the whole account, so this reads the most\n" +
		"recently reported snapshot directly, without needing to find which\n" +
		"session last updated it.",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := newRateLimitStore()

		var records []ratelimit.Record
		if rateLimitsProvider != "" {
			windows, lastUpdated, ok, err := s.LoadRateLimits(rateLimitsProvider)
			if err != nil {
				return err
			}
			if ok {
				records = append(records, ratelimit.Record{
					Provider: rateLimitsProvider, Windows: windows, LastUpdated: lastUpdated,
				})
			}
		} else {
			var err error
			records, err = s.LoadAll()
			if err != nil {
				return err
			}
		}

		if flagJSON {
			return render.JSON(cmd.OutOrStdout(), records)
		}

		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "PROVIDER\tWINDOW\tUSED\tRESETS\tLAST UPDATED")
		for _, rec := range records {
			for _, win := range rec.Windows {
				resets := "-"
				if win.ResetsAt != nil {
					resets = win.ResetsAt.Format(time.RFC3339)
				}
				fmt.Fprintf(tw, "%s\t%s\t%.1f%%\t%s\t%s\n",
					rec.Provider, win.Label, win.PercentUsed, resets, rec.LastUpdated.Format(time.RFC3339))
			}
		}
		return tw.Flush()
	},
}

func init() {
	rateLimitsCmd.Flags().StringVar(&rateLimitsProvider, "provider", "", "only show this provider's rate limits")
	rootCmd.AddCommand(rateLimitsCmd)
}
