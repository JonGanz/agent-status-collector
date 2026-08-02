package mux

import "testing"

func TestDetect_NoMultiplexer(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("STY", "")
	got := Detect()
	if got != (Info{}) {
		t.Fatalf("Detect() = %+v, want zero value", got)
	}
}

func TestDetect_Screen(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("STY", "12345.pts-0.host")
	got := Detect()
	want := Info{Type: "screen", Session: "12345.pts-0.host"}
	if got != want {
		t.Fatalf("Detect() = %+v, want %+v", got, want)
	}
}

func TestDetect_Tmux_UsesInjectedQuerier(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")

	orig := tmuxQuerier
	tmuxQuerier = func() Info {
		return Info{Type: "tmux", Session: "main", Window: "2", Pane: "0"}
	}
	defer func() { tmuxQuerier = orig }()

	got := Detect()
	want := Info{Type: "tmux", Session: "main", Window: "2", Pane: "0"}
	if got != want {
		t.Fatalf("Detect() = %+v, want %+v", got, want)
	}
}

func TestDetect_TmuxEnvSetButQuerierUnavailable(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")

	orig := tmuxQuerier
	tmuxQuerier = func() Info { return Info{Type: "tmux"} } // simulates tmux binary missing
	defer func() { tmuxQuerier = orig }()

	got := Detect()
	want := Info{Type: "tmux"}
	if got != want {
		t.Fatalf("Detect() = %+v, want %+v (graceful degradation)", got, want)
	}
}
