package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withFakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func readSettingsFile(t *testing.T, home string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parsing settings.json: %v", err)
	}
	return m
}

func TestSetup_DryRun_MakesNoWrites(t *testing.T) {
	home := withFakeHome(t)
	p := New()

	res, err := p.Setup(true)
	if err != nil {
		t.Fatalf("Setup(dryRun=true): %v", err)
	}
	if !res.Changed {
		t.Fatalf("expected Changed=true on fresh install dry-run")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run must not create settings.json, stat err = %v", err)
	}
}

func TestSetup_FreshInstall(t *testing.T) {
	home := withFakeHome(t)
	p := New()

	res, err := p.Setup(false)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if !res.Changed {
		t.Fatalf("expected Changed=true on fresh install")
	}

	settings := readSettingsFile(t, home)

	var hooksByEvent map[string][]hookEntry
	if err := json.Unmarshal(settings["hooks"], &hooksByEvent); err != nil {
		t.Fatalf("parsing hooks: %v", err)
	}
	for _, event := range requiredHookEvents {
		if !hasOurHookEntry(hooksByEvent[event]) {
			t.Fatalf("event %s missing our hook entry: %+v", event, hooksByEvent[event])
		}
	}

	var sl statusLineConfig
	if err := json.Unmarshal(settings["statusLine"], &sl); err != nil {
		t.Fatalf("parsing statusLine: %v", err)
	}
	wantScript := filepath.Join(home, ".claude", "agent-status-statusline.sh")
	if sl.Command != wantScript {
		t.Fatalf("statusLine.Command = %q, want %q", sl.Command, wantScript)
	}
	if _, err := os.Stat(wantScript); err != nil {
		t.Fatalf("statusline script not written: %v", err)
	}

	skillFile := filepath.Join(home, ".claude", "skills", "report-status", "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		t.Fatalf("skill file not written: %v", err)
	}
}

func TestSetup_IdempotentReRun(t *testing.T) {
	withFakeHome(t)
	p := New()

	if _, err := p.Setup(false); err != nil {
		t.Fatalf("first Setup: %v", err)
	}
	res, err := p.Setup(false)
	if err != nil {
		t.Fatalf("second Setup: %v", err)
	}
	if res.Changed {
		t.Fatalf("re-running Setup reported Changed=true, want no-op")
	}

	configured, err := p.IsConfigured()
	if err != nil {
		t.Fatalf("IsConfigured: %v", err)
	}
	if !configured {
		t.Fatalf("IsConfigured() = false after Setup")
	}
}

func TestSetup_PreservesThirdPartyHookEntries(t *testing.T) {
	home := withFakeHome(t)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	existing := `{
  "hooks": {
    "PreToolUse": [
      { "hooks": [ { "type": "command", "command": "my-custom-linter-hook" } ] }
    ]
  }
}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := New()
	if _, err := p.Setup(false); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	settings := readSettingsFile(t, home)
	var hooksByEvent map[string][]hookEntry
	if err := json.Unmarshal(settings["hooks"], &hooksByEvent); err != nil {
		t.Fatalf("parsing hooks: %v", err)
	}
	found := false
	for _, e := range hooksByEvent["PreToolUse"] {
		for _, h := range e.Hooks {
			if h.Command == "my-custom-linter-hook" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("third-party PreToolUse hook entry was removed: %+v", hooksByEvent["PreToolUse"])
	}
	if !hasOurHookEntry(hooksByEvent["PreToolUse"]) {
		t.Fatalf("our PreToolUse hook entry was not added alongside the existing one")
	}
}

func TestSetup_BacksUpExistingSettings(t *testing.T) {
	home := withFakeHome(t)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	settingsFile := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsFile, []byte(`{"hooks":{}}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := New()
	if _, err := p.Setup(false); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	entries, err := os.ReadDir(claudeDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	foundBackup := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" && len(e.Name()) > len("settings.json.bak-") &&
			e.Name()[:len("settings.json.bak-")] == "settings.json.bak-" {
			foundBackup = true
		}
	}
	if !foundBackup {
		t.Fatalf("no backup file created in %s: %v", claudeDir, entries)
	}
}

func TestSetup_WrapsExistingStatusline(t *testing.T) {
	home := withFakeHome(t)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	existing := `{"statusLine": {"type": "command", "command": "/usr/local/bin/my-statusline.sh"}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := New()
	if _, err := p.Setup(false); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	scriptPath := filepath.Join(claudeDir, "agent-status-statusline.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("reading wrapper script: %v", err)
	}
	if !strings.Contains(string(data), "/usr/local/bin/my-statusline.sh") {
		t.Fatalf("wrapper script does not reference original command:\n%s", data)
	}
}
