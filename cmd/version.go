package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags, e.g.:
//
//	go build -ldflags "-X github.com/JonGanz/agent-status-collector/cmd.Version=v0.1.0"
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the agent-status version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), Version)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
