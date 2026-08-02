package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/JonGanz/agent-status-collector/internal/provider"
	"github.com/JonGanz/agent-status-collector/internal/render"
)

type providerSummary struct {
	Name         string `json:"name"`
	Installed    bool   `json:"installed"`
	Detail       string `json:"detail"`
	IsConfigured bool   `json:"is_configured"`
}

var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List known providers and whether each is installed/configured",
	RunE: func(cmd *cobra.Command, args []string) error {
		var summaries []providerSummary
		for _, p := range provider.All() {
			installed, detail := p.Detect()
			configured, err := p.IsConfigured()
			if err != nil {
				configured = false
			}
			summaries = append(summaries, providerSummary{
				Name:         p.Name(),
				Installed:    installed,
				Detail:       detail,
				IsConfigured: configured,
			})
		}

		if flagJSON {
			return render.JSON(cmd.OutOrStdout(), summaries)
		}

		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "PROVIDER\tINSTALLED\tCONFIGURED\tDETAIL")
		for _, s := range summaries {
			fmt.Fprintf(tw, "%s\t%t\t%t\t%s\n", s.Name, s.Installed, s.IsConfigured, s.Detail)
		}
		return tw.Flush()
	},
}

func init() {
	rootCmd.AddCommand(providersCmd)
}
