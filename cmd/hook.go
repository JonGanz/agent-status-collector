package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/JonGanz/agent-status-collector/internal/eventlog"
	"github.com/JonGanz/agent-status-collector/internal/mux"
	"github.com/JonGanz/agent-status-collector/internal/provider"
	"github.com/JonGanz/agent-status-collector/internal/render"
)

var (
	hookEvent   string
	hookSummary string
)

var hookCmd = &cobra.Command{
	Use:    "hook <provider>",
	Short:  "Internal: receive a hook/statusline payload on stdin and update the session store",
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		p, ok := provider.Get(name)
		if !ok {
			return fmt.Errorf("unknown provider %q (see `agent-status providers`)", name)
		}

		var raw []byte
		if hookSummary != "" {
			// The report-status skill invokes with --summary directly, so
			// the model never has to construct JSON itself.
			var err error
			raw, err = json.Marshal(struct {
				Summary string `json:"summary"`
			}{Summary: hookSummary})
			if err != nil {
				return fmt.Errorf("encoding summary payload: %w", err)
			}
		} else {
			var err error
			raw, err = io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("reading hook payload: %w", err)
			}
		}

		s := newStore()
		if sa, ok := p.(provider.StoreAware); ok {
			sa.SetStore(s)
		}

		st, err := p.HandleHook(cmd.Context(), hookEvent, bytes.NewReader(raw))
		if err != nil {
			if debugEnabled() {
				_ = eventlog.Append(eventLogPath(), name, hookEvent+" (error: "+err.Error()+")", raw)
			}
			return err
		}

		info := mux.Detect()
		st.Multiplexer = &info

		debug := debugEnabled()
		if debug {
			st.DebugLogPath = eventLogPath()
		}
		if err := s.Save(st); err != nil {
			return err
		}
		if debug {
			if err := eventlog.Append(eventLogPath(), name, hookEvent, raw); err != nil {
				return err
			}
		}

		if flagJSON {
			return render.JSON(cmd.OutOrStdout(), st)
		}
		return nil // hooks run non-interactively; no output by default
	},
}

func init() {
	hookCmd.Flags().StringVar(&hookEvent, "event", "", "name of the hook/statusline event this payload represents")
	hookCmd.Flags().StringVar(&hookSummary, "summary", "", "task summary text (used by the report-status skill instead of a stdin JSON payload)")
	rootCmd.AddCommand(hookCmd)
}
