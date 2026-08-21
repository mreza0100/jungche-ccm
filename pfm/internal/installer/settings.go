package installer

import (
	"encoding/json"
	"fmt"
	"strings"
)

func updateSettings(
	raw []byte,
	home string,
	uninstall bool,
	owned settingsHookCounts,
) ([]byte, bool, settingsHookCounts, error) {
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, false, nil, err
	}
	oldBinary := home + "/.local/bin/cc-fleet"
	pfmBinary := home + "/.local/bin/pfm"
	expected := claudeHookTemplates(home)
	clearCommand := commandByName(expected, "clear-kill")
	groupCommand := commandByName(expected, "group")
	statusCommand := pfmBinary + " statusline"
	usageCommand := commandByName(expected, "usage")
	agentInjectCommand := commandByName(expected, "agent-inject")
	nudgeCommand := commandByName(expected, "nudge")
	exploreDenyCommand := commandByName(expected, "explore-deny")
	epicInjectCommand := commandByName(expected, "epic-inject")
	launcherRepairCommand := commandByName(expected, "launcher-repair")

	changed := false
	before := countSettingsHookCommands(document)
	if uninstall {
		if removeOwnedSettingsHooks(document, owned) {
			changed = true
		}
	} else {
		changed = rewriteCommandFields(document, func(command string) string {
			switch {
			case strings.Contains(command, "dreamer-agent-inject.sh"):
				return agentInjectCommand
			case strings.Contains(command, "dreamer-nudge.sh"):
				return nudgeCommand
			case strings.Contains(command, "explore-deny.sh"):
				return exploreDenyCommand
			case command == oldBinary || strings.HasPrefix(command, oldBinary+" "):
				return pfmBinary + strings.TrimPrefix(command, oldBinary)
			default:
				return command
			}
		})
	}
	if _, present := document["cleanupPeriodDays"]; !present && !uninstall {
		document["cleanupPeriodDays"] = float64(36500)
		changed = true
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
	seenUserPromptCommands := map[string]bool{}
	for _, entry := range entries {
		hooks, _ := entry["hooks"].([]any)
		kept := hooks[:0]
		for _, hookValue := range hooks {
			hook, _ := hookValue.(map[string]any)
			command, _ := hook["command"].(string)
			original := command
			if !uninstall {
				switch {
				case strings.Contains(command, "chat/group.sh") && strings.HasSuffix(command, " hook"):
					command = groupCommand
				case strings.Contains(command, "cc-usage-hook.sh"):
					command = usageCommand
				}
			}
			if command != original {
				hook["command"] = command
				hook["type"] = "command"
				changed = true
			}
			if isRetiredBBCommand(command) {
				changed = true
				continue
			}
			if !uninstall && (command == groupCommand || command == usageCommand || command == nudgeCommand || command == epicInjectCommand) {
				if seenUserPromptCommands[command] {
					changed = true
					continue
				}
				seenUserPromptCommands[command] = true
			}
			kept = append(kept, hookValue)
		}
		entry["hooks"] = kept
	}
	pruneEmptyHooks(document, "UserPromptSubmit")
	if !uninstall {
		for _, entry := range hookEntries(document, "PreToolUse", true) {
			hooks, _ := entry["hooks"].([]any)
			for _, hookValue := range hooks {
				hook, _ := hookValue.(map[string]any)
				command, _ := hook["command"].(string)
				if command == agentInjectCommand || command == exploreDenyCommand {
					if entry["matcher"] != "Agent|Task" {
						entry["matcher"] = "Agent|Task"
						changed = true
					}
				}
			}
		}
	}

	clearSeen := false
	for _, entry := range hookEntries(document, "SessionEnd", false) {
		hooks, _ := entry["hooks"].([]any)
		kept := hooks[:0]
		for _, hookValue := range hooks {
			hook, _ := hookValue.(map[string]any)
			command, _ := hook["command"].(string)
			if command == clearCommand {
				if !uninstall && clearSeen {
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
		if !hasHookCommandWithMatcher(hookEntries(document, "SessionStart", true), launcherRepairCommand, "") {
			appendHookWithMatcher(document, "SessionStart", "", launcherRepairCommand)
			changed = true
		}
		if !hasHookCommand(hookEntries(document, "UserPromptSubmit", true), groupCommand) {
			appendHook(document, "UserPromptSubmit", groupCommand)
			changed = true
		}
		if !hasHookCommand(hookEntries(document, "UserPromptSubmit", true), usageCommand) {
			appendHook(document, "UserPromptSubmit", usageCommand)
			changed = true
		}
		if !clearSeen {
			appendHook(document, "SessionEnd", clearCommand)
			changed = true
		}
		if !hasHookCommandWithMatcher(hookEntries(document, "PreToolUse", true), agentInjectCommand, "Agent|Task") {
			appendHookWithMatcher(document, "PreToolUse", "Agent|Task", agentInjectCommand)
			changed = true
		}
		if !hasHookCommandWithMatcher(hookEntries(document, "PreToolUse", true), exploreDenyCommand, "Agent|Task") {
			appendHookWithMatcher(document, "PreToolUse", "Agent|Task", exploreDenyCommand)
			changed = true
		}
		if !hasHookCommandWithMatcher(hookEntries(document, "UserPromptSubmit", true), nudgeCommand, "") {
			appendHookWithMatcher(document, "UserPromptSubmit", "", nudgeCommand)
			changed = true
		}
		if !hasHookCommandWithMatcher(hookEntries(document, "UserPromptSubmit", true), epicInjectCommand, "") {
			appendHookWithMatcher(document, "UserPromptSubmit", "", epicInjectCommand)
			changed = true
		}
		if normalizeExpectedHookTypes(document, expected) {
			changed = true
		}
	}
	nextOwned := nextSettingsHookOwnership(
		before,
		countSettingsHookCommands(document),
		owned,
		pfmBinary,
		uninstall,
	)

	if !changed {
		return raw, false, nextOwned, nil
	}
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, false, nil, fmt.Errorf("encode settings: %w", err)
	}
	return append(updated, '\n'), true, nextOwned, nil
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

func hasHookCommandWithMatcher(entries []map[string]any, wanted, matcher string) bool {
	for _, entry := range entries {
		if entry["matcher"] != matcher {
			continue
		}
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

func normalizeExpectedHookTypes(document map[string]any, expected []ExpectedHook) bool {
	changed := false
	for _, wanted := range expected {
		for _, entry := range hookEntries(document, wanted.Event, false) {
			matcher, _ := entry["matcher"].(string)
			if matcher != wanted.Matcher {
				continue
			}
			hooks, _ := entry["hooks"].([]any)
			for _, hookValue := range hooks {
				hook, _ := hookValue.(map[string]any)
				if hook["command"] == wanted.Command && hook["type"] != "command" {
					hook["type"] = "command"
					changed = true
				}
			}
		}
	}
	return changed
}

func appendHook(document map[string]any, event, command string) {
	appendHookWithMatcher(document, event, "", command)
}

func appendHookWithMatcher(document map[string]any, event, matcher, command string) {
	hooks, _ := document["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		document["hooks"] = hooks
	}
	values, _ := hooks[event].([]any)
	hooks[event] = append(values, map[string]any{
		"matcher": matcher,
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
