package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JonGanz/agent-status-collector/internal/provider"
)

var (
	setupDryRun bool
	setupYes    bool
)

var setupCmd = &cobra.Command{
	Use:   "setup <provider>",
	Short: "Guided/automated integration setup for a provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		p, ok := provider.Get(name)
		if !ok {
			return fmt.Errorf("unknown provider %q (see `agent-status providers`)", name)
		}

		if setupDryRun {
			res, err := p.Setup(true)
			if err != nil {
				return err
			}
			printSetupResult(cmd, res)
			return nil
		}

		preview, err := p.Setup(true)
		if err != nil {
			return err
		}
		printSetupResult(cmd, preview)

		if !preview.Changed {
			return nil
		}

		if !setupYes {
			fmt.Fprint(cmd.OutOrStdout(), "Proceed? [y/N] ")
			reader := bufio.NewReader(cmd.InOrStdin())
			line, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(line)) != "y" {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted; no changes made.")
				return nil
			}
		}

		res, err := p.Setup(false)
		if err != nil {
			return err
		}
		printSetupResult(cmd, res)
		return nil
	},
}

func printSetupResult(cmd *cobra.Command, res provider.SetupResult) {
	if res.Instructions != "" {
		fmt.Fprintln(cmd.OutOrStdout(), res.Instructions)
	}
	for _, f := range res.FilesTouched {
		fmt.Fprintf(cmd.OutOrStdout(), "  file: %s\n", f)
	}
}

func init() {
	setupCmd.Flags().BoolVar(&setupDryRun, "dry-run", false, "preview changes without writing anything")
	setupCmd.Flags().BoolVar(&setupYes, "yes", false, "apply changes without interactive confirmation")
	rootCmd.AddCommand(setupCmd)
}
