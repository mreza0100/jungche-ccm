package installer

import (
	"encoding/json"
	"fmt"
	"strings"
)

const codexClearMatcher = "startup|resume|clear"

func updateCodexHooks(raw []byte, home string, uninstall bool) ([]byte, bool, error) {
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, false, err
	}
	oldBinary := home + "/.local/bin/cc-fleet"
	pfmBinary := home + "/.local/bin/pfm"
	clearCommand := pfmBinary + " internal clear-kill"
	changed := false
	if !uninstall {
		changed = rewriteCommandFields(document, func(command string) string {
			if command == oldBinary || strings.HasPrefix(command, oldBinary+" ") {
				return pfmBinary + strings.TrimPrefix(command, oldBinary)
			}
			return command
		})
	}

	clearSeen := false
	for _, entry := range hookEntries(document, "SessionStart", false) {
		matcher, _ := entry["matcher"].(string)
		hooks, _ := entry["hooks"].([]any)
		kept := hooks[:0]
		for _, hookValue := range hooks {
			hook, _ := hookValue.(map[string]any)
			command, _ := hook["command"].(string)
			if command != clearCommand {
				kept = append(kept, hookValue)
				continue
			}
			if uninstall || clearSeen || matcher != codexClearMatcher {
				changed = true
				continue
			}
			clearSeen = true
			kept = append(kept, hookValue)
		}
		entry["hooks"] = kept
	}
	pruneEmptyHooks(document, "SessionStart")
	if !uninstall && !clearSeen {
		appendCodexClearHook(document, clearCommand)
		changed = true
	}
	if !changed {
		return raw, false, nil
	}
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, false, fmt.Errorf("encode Codex hooks: %w", err)
	}
	return append(updated, '\n'), true, nil
}

func appendCodexClearHook(document map[string]any, command string) {
	hooks, _ := document["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		document["hooks"] = hooks
	}
	values, _ := hooks["SessionStart"].([]any)
	hooks["SessionStart"] = append(values, map[string]any{
		"matcher": codexClearMatcher,
		"hooks": []any{map[string]any{
			"type": "command", "command": command, "timeout": float64(3),
		}},
	})
}
