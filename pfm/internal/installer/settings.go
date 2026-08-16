package installer

import (
	"encoding/json"
	"fmt"
	"strings"
)

func updateSettings(raw []byte, home string, full, uninstall bool) ([]byte, bool, error) {
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, false, err
	}
	oldBinary := home + "/.local/bin/cc-fleet"
	pfmBinary := home + "/.local/bin/pfm"
	clearCommand := pfmBinary + " internal clear-hide"
	groupCommand := pfmBinary + " chat group hook"
	statusCommand := pfmBinary + " statusline"
	usageCommand := pfmBinary + " usage-hook"

	changed := false
	if !uninstall {
		changed = rewriteCommandFields(document, func(command string) string {
			if command == oldBinary || strings.HasPrefix(command, oldBinary+" ") {
				return pfmBinary + strings.TrimPrefix(command, oldBinary)
			}
			return command
		})
	}

	status, _ := document["statusLine"].(map[string]any)
	currentStatus, _ := status["command"].(string)
	if uninstall {
		if currentStatus == statusCommand {
			delete(document, "statusLine")
			changed = true
		}
	} else if currentStatus == "" {
		document["statusLine"] = map[string]any{
			"type":                 "command",
			"command":              statusCommand,
			"padding":              float64(0),
			"refreshInterval":      float64(3),
			"hideVimModeIndicator": true,
		}
		changed = true
	} else if currentStatus != statusCommand && strings.Contains(currentStatus, "statusline-command.sh") {
		status["type"] = "command"
		status["command"] = statusCommand
		changed = true
	}

	entries := hookEntries(document, "UserPromptSubmit", !uninstall)
	for _, entry := range entries {
		hooks, _ := entry["hooks"].([]any)
		kept := hooks[:0]
		for _, hookValue := range hooks {
			hook, _ := hookValue.(map[string]any)
			command, _ := hook["command"].(string)
			original := command
			switch {
			case strings.Contains(command, "chat/group.sh") && strings.HasSuffix(command, " hook"):
				command = groupCommand
			case strings.Contains(command, "cc-usage-hook.sh"):
				command = usageCommand
			}
			if command != original {
				hook["command"] = command
				hook["type"] = "command"
				changed = true
			}
			if isRetiredBBCommand(command) || uninstall && (command == groupCommand || command == usageCommand) {
				changed = true
				continue
			}
			kept = append(kept, hookValue)
		}
		entry["hooks"] = kept
	}
	pruneEmptyHooks(document, "UserPromptSubmit")

	clearSeen := false
	for _, entry := range hookEntries(document, "SessionEnd", false) {
		hooks, _ := entry["hooks"].([]any)
		kept := hooks[:0]
		for _, hookValue := range hooks {
			hook, _ := hookValue.(map[string]any)
			command, _ := hook["command"].(string)
			if command == clearCommand {
				if uninstall || clearSeen {
					changed = true
					continue
				}
				clearSeen = true
			}
			kept = append(kept, hookValue)
		}
		entry["hooks"] = kept
	}
	pruneEmptyHooks(document, "SessionEnd")

	if !uninstall {
		if !hasHookCommand(hookEntries(document, "UserPromptSubmit", true), usageCommand) {
			appendHook(document, "UserPromptSubmit", usageCommand)
			changed = true
		}
		if full && !clearSeen {
			appendHook(document, "SessionEnd", clearCommand)
			changed = true
		}
	}

	if !changed {
		return raw, false, nil
	}
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, false, fmt.Errorf("encode settings: %w", err)
	}
	return append(updated, '\n'), true, nil
}

func rewriteCommandFields(value any, rewrite func(string) string) bool {
	changed := false
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "command" {
				if command, ok := child.(string); ok {
					updated := rewrite(command)
					if updated != command {
						typed[key] = updated
						changed = true
					}
				}
			}
			if rewriteCommandFields(child, rewrite) {
				changed = true
			}
		}
	case []any:
		for _, child := range typed {
			if rewriteCommandFields(child, rewrite) {
				changed = true
			}
		}
	}
	return changed
}

func isRetiredBBCommand(command string) bool {
	command = strings.TrimSpace(command)
	return command == "pfm bb" || command == "pfm chat bb" ||
		strings.HasSuffix(command, "/pfm bb") || strings.HasSuffix(command, "/pfm chat bb") ||
		strings.HasSuffix(command, "/cc-fleet bb") || strings.HasSuffix(command, "/cc-fleet chat bb") ||
		strings.Contains(command, "bb-hook.sh")
}

func hookEntries(document map[string]any, event string, create bool) []map[string]any {
	hooks, _ := document["hooks"].(map[string]any)
	if hooks == nil && create {
		hooks = map[string]any{}
		document["hooks"] = hooks
	}
	if hooks == nil {
		return nil
	}
	values, _ := hooks[event].([]any)
	if values == nil && create {
		values = []any{}
		hooks[event] = values
	}
	entries := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if entry, ok := value.(map[string]any); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func hasHookCommand(entries []map[string]any, wanted string) bool {
	for _, entry := range entries {
		hooks, _ := entry["hooks"].([]any)
		for _, value := range hooks {
			hook, _ := value.(map[string]any)
			if hook["command"] == wanted {
				return true
			}
		}
	}
	return false
}

func appendHook(document map[string]any, event, command string) {
	hooks, _ := document["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		document["hooks"] = hooks
	}
	values, _ := hooks[event].([]any)
	hooks[event] = append(values, map[string]any{
		"matcher": "",
		"hooks": []any{map[string]any{
			"type": "command", "command": command,
		}},
	})
}

func pruneEmptyHooks(document map[string]any, event string) {
	hooks, _ := document["hooks"].(map[string]any)
	if hooks == nil {
		return
	}
	values, _ := hooks[event].([]any)
	kept := values[:0]
	for _, value := range values {
		entry, _ := value.(map[string]any)
		entryHooks, _ := entry["hooks"].([]any)
		if len(entryHooks) > 0 {
			kept = append(kept, value)
		}
	}
	hooks[event] = kept
}
