// Package render formats Status records for terminal display: a compact
// table for `list`, a detail view for `show`, and JSON passthrough for
// scripting (--json).
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/JonGanz/agent-status-collector/internal/store"
)

// JSON writes v as indented JSON to w.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// List writes a compact table of entries to w.
func List(w io.Writer, entries []store.Entry) error {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SESSION\tPROVIDER\tSTATE\tCONTEXT\tTASK\tAGE")
	for _, e := range entries {
		id := e.Status.SessionID
		if len(id) > 12 {
			id = id[:12]
		}
		ctx := "-"
		if e.Status.Context != nil {
			ctx = fmt.Sprintf("%.0f%%", e.Status.Context.PercentUsed)
		}
		task := e.Status.TaskSummary
		if task == "" {
			task = "-"
		}
		state := string(e.Status.State)
		if e.Stale {
			state += " (stale)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			id, e.Status.Provider, state, ctx, task, age(e.Status.LastUpdated))
	}
	return tw.Flush()
}

// Show writes a full detail view of a single Status to w.
func Show(w io.Writer, e store.Entry) error {
	st := e.Status
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "Session:\t%s\n", st.SessionID)
	fmt.Fprintf(tw, "Provider:\t%s\n", st.Provider)
	state := string(st.State)
	if e.Stale {
		state += " (stale)"
	}
	fmt.Fprintf(tw, "State:\t%s\n", state)
	if st.PID != nil {
		fmt.Fprintf(tw, "PID:\t%d\n", *st.PID)
	}
	if st.TaskSummary != "" {
		fmt.Fprintf(tw, "Task:\t%s\n", st.TaskSummary)
	}
	if st.Context != nil {
		fmt.Fprintf(tw, "Context:\t%.1f%% (%d/%d tokens)\n", st.Context.PercentUsed, st.Context.UsedTokens, st.Context.MaxTokens)
	}
	if st.Cost != nil {
		fmt.Fprintf(tw, "Cost:\t%.4f %s\n", st.Cost.SessionUSD, orDefault(st.Cost.Currency, "USD"))
	}
	if st.Multiplexer != nil && st.Multiplexer.Type != "" {
		fmt.Fprintf(tw, "Multiplexer:\t%s session=%s window=%s pane=%s\n",
			st.Multiplexer.Type, st.Multiplexer.Session, st.Multiplexer.Window, st.Multiplexer.Pane)
	}
	if st.WorkingDir != "" {
		fmt.Fprintf(tw, "Working dir:\t%s\n", st.WorkingDir)
	}
	fmt.Fprintf(tw, "Started:\t%s\n", st.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(tw, "Last updated:\t%s (%s ago)\n", st.LastUpdated.Format(time.RFC3339), age(st.LastUpdated))
	if st.DebugLogPath != "" {
		fmt.Fprintf(tw, "Debug log:\t%s\n", st.DebugLogPath)
	}
	return tw.Flush()
}

func age(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t).Round(time.Second)
	if d < 0 {
		d = 0
	}
	return d.String()
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
