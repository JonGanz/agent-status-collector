package mux

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// installFakeTmux puts a fake `tmux` executable on PATH that records its
// argv to argsFile (one line, tab-separated) and prints a fixed
// display-message-shaped response, so queryTmux's actual `exec.Command`
// invocation can be inspected without depending on a real tmux server.
func installFakeTmux(t *testing.T) (argsFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake tmux shim uses a POSIX shell script")
	}

	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args.txt")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %s
printf 'sess\t3\t1\t$9\t@8\t%%%%7\n'
`, argsFile)
	scriptPath := filepath.Join(dir, "tmux")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

func TestQueryTmux_TargetsTmuxPaneWhenSet(t *testing.T) {
	argsFile := installFakeTmux(t)
	t.Setenv("TMUX_PANE", "%42")

	got := queryTmux()

	want := Info{Type: "tmux", Session: "sess", Window: "3", Pane: "1", SessionID: "$9", WindowID: "@8", PaneID: "%7"}
	if got != want {
		t.Fatalf("queryTmux() = %+v, want %+v", got, want)
	}

	recorded, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("reading recorded args: %v", err)
	}
	args := strings.TrimSpace(string(recorded))
	if !strings.Contains(args, "-t %42") {
		t.Fatalf("expected tmux invocation to target -t %%42 (the actual originating pane, not\n"+
			"whatever the attached client currently has focused), got args: %q", args)
	}
}

func TestQueryTmux_OmitsTargetWhenTmuxPaneUnset(t *testing.T) {
	argsFile := installFakeTmux(t)
	t.Setenv("TMUX_PANE", "")

	_ = queryTmux()

	recorded, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("reading recorded args: %v", err)
	}
	args := strings.TrimSpace(string(recorded))
	if strings.Contains(args, "-t") {
		t.Fatalf("expected no -t flag when TMUX_PANE is unset, got args: %q", args)
	}
}
