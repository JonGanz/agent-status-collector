package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookEntry struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookCommand `json:"hooks"`
}

type statusLineConfig struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// hookCommandMarker is the recognizable substring used to detect our own
// hook entries for idempotent re-runs of Setup, without disturbing any
// other tool's entries in the same event's hook array.
const hookCommandMarker = "agent-status hook claudecode"

func ourHookCommand(event string) string {
	return fmt.Sprintf("agent-status hook claudecode --event=%s", event)
}

func claudeHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("claudecode: resolving home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

func settingsPath() (string, error) {
	home, err := claudeHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "settings.json"), nil
}

func statuslineScriptPath() (string, error) {
	home, err := claudeHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "agent-status-statusline.sh"), nil
}

func skillPath() (string, error) {
	home, err := claudeHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "skills", "report-status", "SKILL.md"), nil
}

// readSettings reads settings.json, returning an empty top-level object if
// the file doesn't exist yet.
func readSettings(path string) (map[string]json.RawMessage, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]json.RawMessage{}, false, nil
		}
		return nil, false, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, true, fmt.Errorf("claudecode: parsing %s: %w", path, err)
	}
	if m == nil {
		m = map[string]json.RawMessage{}
	}
	return m, true, nil
}

// mergeHooks returns whether any change is needed and the updated "hooks"
// raw value. It never removes or alters entries belonging to other tools.
func mergeHooks(settings map[string]json.RawMessage) (changed bool, newHooksRaw json.RawMessage, err error) {
	hooksByEvent := map[string][]hookEntry{}
	if raw, ok := settings["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooksByEvent); err != nil {
			return false, nil, fmt.Errorf("claudecode: parsing existing hooks config: %w", err)
		}
	}
	if hooksByEvent == nil {
		hooksByEvent = map[string][]hookEntry{}
	}

	for _, event := range requiredHookEvents {
		if hasOurHookEntry(hooksByEvent[event]) {
			continue
		}
		changed = true
		hooksByEvent[event] = append(hooksByEvent[event], hookEntry{
			Hooks: []hookCommand{{Type: "command", Command: ourHookCommand(event)}},
		})
	}

	out, err := json.MarshalIndent(hooksByEvent, "", "  ")
	if err != nil {
		return false, nil, err
	}
	return changed, out, nil
}

func hasOurHookEntry(entries []hookEntry) bool {
	for _, e := range entries {
		for _, h := range e.Hooks {
			if strings.Contains(h.Command, hookCommandMarker) {
				return true
			}
		}
	}
	return false
}

// allHooksPresent reports whether every required event already has an entry
// whose command contains our marker.
func allHooksPresent(settings map[string]json.RawMessage) bool {
	hooksByEvent := map[string][]hookEntry{}
	raw, ok := settings["hooks"]
	if !ok {
		return false
	}
	if err := json.Unmarshal(raw, &hooksByEvent); err != nil {
		return false
	}
	for _, event := range requiredHookEvents {
		if !hasOurHookEntry(hooksByEvent[event]) {
			return false
		}
	}
	return true
}

// existingStatusLineCommand returns the currently configured statusLine
// command, or "" if none is configured.
func existingStatusLineCommand(settings map[string]json.RawMessage) string {
	raw, ok := settings["statusLine"]
	if !ok {
		return ""
	}
	var sl statusLineConfig
	if err := json.Unmarshal(raw, &sl); err != nil {
		return ""
	}
	return sl.Command
}
