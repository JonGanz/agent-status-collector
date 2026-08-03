package claudecode

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/JonGanz/agent-status-collector/internal/fsutil"
	"github.com/JonGanz/agent-status-collector/internal/provider"
)

//go:embed assets/report-status.SKILL.md
var skillTemplate []byte

const statuslineScriptMarker = "# agent-status-collector-managed"

// IsConfigured implements provider.Provider. It is deliberately strict: all
// of hooks, statusline, and the skill file must be present, since a partial
// install silently produces incomplete Status data (e.g. hooks wired but no
// statusline means Context/Cost/RateLimit never populate).
func (p *Provider) IsConfigured() (bool, error) {
	sp, err := settingsPath()
	if err != nil {
		return false, err
	}
	settings, existed, err := readSettings(sp)
	if err != nil {
		return false, err
	}
	if !existed {
		return false, nil
	}

	if !allHooksPresent(settings) {
		return false, nil
	}

	slScript, err := statuslineScriptPath()
	if err != nil {
		return false, err
	}
	if existingStatusLineCommand(settings) != slScript {
		return false, nil
	}

	sk, err := skillPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(sk); err != nil {
		return false, nil
	}
	return true, nil
}

// Setup implements provider.Provider. This mutates the user's global
// ~/.claude/settings.json, so every real write is preceded by a timestamped
// backup, and dryRun performs zero filesystem writes.
func (p *Provider) Setup(dryRun bool) (provider.SetupResult, error) {
	sp, err := settingsPath()
	if err != nil {
		return provider.SetupResult{}, err
	}
	settings, existed, err := readSettings(sp)
	if err != nil {
		return provider.SetupResult{}, err
	}

	hooksChanged, hooksRaw, err := mergeHooks(settings)
	if err != nil {
		return provider.SetupResult{}, err
	}

	slScript, err := statuslineScriptPath()
	if err != nil {
		return provider.SetupResult{}, err
	}
	existingSL := existingStatusLineCommand(settings)
	slChanged := existingSL != slScript
	var slAction string
	switch {
	case existingSL == "":
		slAction = fmt.Sprintf("install statusline script at %s", slScript)
	case existingSL == slScript:
		slAction = "statusline already managed by agent-status-collector"
	default:
		slAction = fmt.Sprintf("wrap existing statusline command (%q) with %s so both keep working", existingSL, slScript)
	}

	sk, err := skillPath()
	if err != nil {
		return provider.SetupResult{}, err
	}
	skillChanged := true
	if data, err := os.ReadFile(sk); err == nil && string(data) == string(skillTemplate) {
		skillChanged = false
	}

	changed := hooksChanged || slChanged || skillChanged

	var instructions string
	if !changed {
		instructions = "Claude Code integration is already fully configured; nothing to do."
	} else {
		instructions = fmt.Sprintf(
			"Will modify: %s (hooks: %v)\nStatusline: %s\nWill write skill: %s",
			sp, hooksChanged, slAction, sk)
	}

	result := provider.SetupResult{
		Changed:      changed,
		Instructions: instructions,
		FilesTouched: []string{sp, slScript, sk},
	}
	if dryRun || !changed {
		return result, nil
	}

	// 1. Back up settings.json before any write, if it already existed.
	if existed {
		backupPath := sp + ".bak-" + p.now().Format("20060102-150405")
		if err := copyFile(sp, backupPath); err != nil {
			return result, err
		}
		result.FilesTouched = append(result.FilesTouched, backupPath)
	}

	// 2. Write the statusline script (default relay, or wrapping the
	// user's existing command).
	if slChanged {
		if err := writeStatuslineScript(slScript, existingSL); err != nil {
			return result, err
		}
	}

	// 3. Merge hooks + statusLine into settings.json and write atomically.
	settings["hooks"] = hooksRaw
	slConf := statusLineConfig{Type: "command", Command: slScript}
	slRaw, err := json.MarshalIndent(slConf, "", "  ")
	if err != nil {
		return result, err
	}
	settings["statusLine"] = slRaw

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return result, err
	}
	if err := fsutil.WriteAtomic(sp, out, 0o600); err != nil {
		return result, err
	}

	// 4. Write the skill file.
	if skillChanged {
		if err := fsutil.WriteAtomic(sk, skillTemplate, 0o600); err != nil {
			return result, err
		}
	}

	return result, nil
}

func writeStatuslineScript(path, originalCommand string) error {
	var script string
	if originalCommand == "" {
		script = fmt.Sprintf(`#!/bin/sh
%s vN - default statusline relay for agent-status-collector
input=$(cat)
printf '%%s' "$input" | agent-status hook claudecode --event=StatusLine >/dev/null 2>&1 &
printf '%%s' "$input" | jq -r '"[\(.model.display_name // "claude")] ctx \(.context_window.used_percentage // 0)%% $\(.cost.total_cost_usd // 0)"'
`, statuslineScriptMarker)
	} else {
		script = fmt.Sprintf(`#!/bin/sh
%s vN - wraps the user's original statusline command while also relaying
# to agent-status-collector. Original command captured at setup time:
input=$(cat)
printf '%%s' "$input" | agent-status hook claudecode --event=StatusLine >/dev/null 2>&1 &
printf '%%s' "$input" | %s
`, statuslineScriptMarker, originalCommand)
	}
	return fsutil.WriteAtomic(path, []byte(script), 0o700)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(dst, data, 0o600)
}
