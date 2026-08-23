package installer

import (
	"encoding/json"
	"fmt"
	"strings"
)

// codexClearMatcher is the matcher the retired Codex SessionStart clear-kill
// hook used to carry — kept as the shape a leftover installer-owned entry is
// still recognized by (installerOwnedHookKey), never written again.
const codexClearMatcher = "startup|resume|clear"

// updateCodexHooks converges ~/.codex/hooks.json two ways: an ordinary
// binary-path migration (cc-fleet → pfm) for whatever hooks a host still
// carries, exactly as before; and, unconditionally in both install and
// uninstall passes, the REMOVAL of the SessionStart clear-kill hook this
// installer used to wire. That hook is retired, not migrated: Codex's own
// SessionStart(source=clear) fires on the new session's FIRST TURN, by
// which point every Codex chat on the host shares one app-server daemon
// pid, so it could never say which pane cleared (codex-clear-identity
// train). There is no install-mode branch left that writes it back.
func updateCodexHooks(raw []byte, home string, uninstall bool, owned settingsHookCounts) ([]byte, bool, settingsHookCounts, error) {
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, false, nil, err
	}
	oldBinary := home + "/.local/bin/cc-fleet"
	pfmBinary := home + "/.local/bin/pfm"
	retiredCommands := map[string]bool{
		pfmBinary + " internal clear-kill":                  true,
		oldBinary + " internal clear-kill":                  true,
		pfmBinary + ` internal clear-kill --parent "$PPID"`: true,
		oldBinary + ` internal clear-kill --parent "$PPID"`: true,
	}

	changed := false
	if uninstall {
		changed = removeOwnedSettingsHooks(document, owned)
	} else {
		changed = rewriteCommandFields(document, func(command string) string {
			if command == oldBinary || strings.HasPrefix(command, oldBinary+" ") {
				return pfmBinary + strings.TrimPrefix(command, oldBinary)
			}
			return command
		})
	}
	if removeRetiredHookCommands(document) {
		changed = true
	}

	for _, entry := range hookEntries(document, "SessionStart", false) {
		hooks, _ := entry["hooks"].([]any)
		kept := hooks[:0]
		for _, hookValue := range hooks {
			hook, _ := hookValue.(map[string]any)
			command, _ := hook["command"].(string)
			if isRetiredHookCommand(command) || retiredCommands[command] {
				changed = true
				continue
			}
			kept = append(kept, hookValue)
		}
		entry["hooks"] = kept
	}
	pruneEmptyHooks(document, "SessionStart")
	// pruneEmptyHooks only trims entries, and unconditionally writes the
	// (possibly nil, possibly now-empty) array back — the shape every OTHER
	// hook it prunes still needs, since a claude settings.json keeps other
	// SessionStart hooks alongside clear-kill. A Codex hooks.json has NO
	// other SessionStart hook, so an event key holding nothing is deleted
	// outright rather than left behind as a bare `"SessionStart": []` (or
	// `null`, when the key never existed at all — pruneEmptyHooks writes
	// one anyway). This never affects whether the file NEEDS rewriting:
	// deleting a key pruneEmptyHooks only just introduced restores exactly
	// the document's own prior shape.
	if hooks, ok := document["hooks"].(map[string]any); ok {
		if values, _ := hooks["SessionStart"].([]any); len(values) == 0 {
			delete(hooks, "SessionStart")
		}
	}

	// Nothing in this file is ever installer-owned going forward: the one
	// hook the installer used to claim here is retired, and no code path
	// writes a fresh claim.
	nextOwned := settingsHookCounts{}
	if !changed {
		return raw, false, nextOwned, nil
	}
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, false, nil, fmt.Errorf("encode Codex hooks: %w", err)
	}
	return append(updated, '\n'), true, nextOwned, nil
}
