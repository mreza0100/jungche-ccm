package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pfmconfig "hostops/pfm/internal/config"
)

// ExpectedHook is one hook the installer converges and owns.
type ExpectedHook struct {
	Target  string
	File    string
	Event   string
	Matcher string
	Command string
	Name    string
}

// HookProbeResult keeps missing, broken, and ownership drift distinct.
type HookProbeResult struct {
	Hook  ExpectedHook
	State string
	Error string
}

// ExpectedHooks is the installer's exported source of truth for doctor. The
// installer writes configured account settings only; it never writes a
// project-level .claude/settings.json or invents an engine account.
func ExpectedHooks(home string, config pfmconfig.Config) []ExpectedHook {
	type target struct {
		name string
		file string
	}
	targets := make([]target, 0, len(config.Accounts))
	for _, account := range config.Accounts {
		targets = append(targets, target{
			name: fmt.Sprintf("claude[%d]", account.ID),
			file: filepath.Join(account.ConfigDir, "settings.json"),
		})
	}
	seen := map[string]bool{}
	// Codex's own SessionStart(source=clear) hook is retired, not migrated:
	// it fires on the new session's FIRST TURN, by which point every Codex
	// chat on the host shares one app-server daemon pid, so it could never
	// say which pane cleared (codex-clear-identity train). There is
	// therefore no per-account ~/.codex/hooks.json entry to expect anymore;
	// pfm install actively strips a leftover one (codex_hooks.go), and
	// doctor reports nothing about a hook it no longer wires — reporting
	// "missing: run pfm install" for a hook that was never coming back
	// would be the lie, not the silence.
	templates := claudeHookTemplates(home)
	result := make([]ExpectedHook, 0, len(targets)*len(templates))
	for _, target := range targets {
		physical := physicalSettingsPath(target.file)
		if seen[physical] {
			continue
		}
		seen[physical] = true
		for _, hook := range templates {
			hook.Target = target.name
			hook.File = target.file
			result = append(result, hook)
		}
	}
	return result
}

func claudeHookTemplates(home string) []ExpectedHook {
	binary := filepath.Join(home, ".local", "bin", "pfm")
	return []ExpectedHook{
		{Event: "SessionStart", Command: binary + " internal launcher-repair", Name: "launcher-repair"},
		{Event: "UserPromptSubmit", Command: binary + " usage-hook", Name: "usage"},
		{Event: "SessionEnd", Command: binary + " internal clear-kill", Name: "clear-kill"},
		{Event: "PreToolUse", Matcher: "Agent|Task", Command: binary + " internal explore-deny", Name: "explore-deny"},
		{Event: "UserPromptSubmit", Command: binary + " internal epic-inject", Name: "epic-inject"},
	}
}

func commandByName(hooks []ExpectedHook, name string) string {
	for _, hook := range hooks {
		if hook.Name == name {
			return hook.Command
		}
	}
	panic("installer expected hook is missing: " + name)
}

func physicalSettingsPath(path string) string {
	physical, err := filepath.EvalSymlinks(path)
	if err != nil {
		physical = path
	}
	return filepath.Clean(physical)
}

// ProbeExpectedHooks parses each expected file once, validates the canonical
// command path, then cross-checks the same ownership ledger uninstall reads.
func ProbeExpectedHooks(home string, config pfmconfig.Config) []HookProbeResult {
	expected := ExpectedHooks(home, config)
	ownership, _, ownershipErr := readSettingsHookOwnership(filepath.Join(home, ".local", "share", "pfm", "install", "settings-hook-ownership.json"))
	if ownershipErr != nil {
		return []HookProbeResult{{
			Hook:  ExpectedHook{Target: "ownership", File: filepath.Join(home, ".local", "share", "pfm", "install", "settings-hook-ownership.json")},
			State: "broken", Error: ownershipErr.Error(),
		}}
	}
	type fileProbe struct {
		counts      settingsHookCounts
		allCounts   settingsHookCounts
		globalIssue string
		eventIssues map[string]string
		err         error
	}
	files := map[string]fileProbe{}
	fileTargets := map[string]string{}
	fileDisplayPaths := map[string]string{}
	for _, hook := range expected {
		physical := physicalSettingsPath(hook.File)
		if _, done := files[physical]; done {
			continue
		}
		fileTargets[physical] = hook.Target
		fileDisplayPaths[physical] = hook.File
		raw, err := os.ReadFile(hook.File)
		if err != nil {
			files[physical] = fileProbe{err: err}
			continue
		}
		var document map[string]any
		if err := json.Unmarshal(raw, &document); err != nil {
			files[physical] = fileProbe{err: fmt.Errorf("parse %s: %w", hook.File, err)}
			continue
		}
		counts, allCounts, globalIssue, eventIssues := inspectExpectedHookDocument(document)
		files[physical] = fileProbe{
			counts: counts, allCounts: allCounts,
			globalIssue: globalIssue, eventIssues: eventIssues,
		}
	}

	results := make([]HookProbeResult, 0, len(expected)*2)
	expectedKeys := map[string]bool{}
	for _, hook := range expected {
		physical := physicalSettingsPath(hook.File)
		key := settingsHookKey{Event: hook.Event, Matcher: hook.Matcher, Command: hook.Command}
		expectedKeys[physical+"\x00"+hook.Event+"\x00"+hook.Matcher+"\x00"+hook.Command] = true
		file := files[physical]
		if file.err != nil {
			state := "broken"
			if errors.Is(file.err, os.ErrNotExist) {
				state = "missing"
			}
			results = append(results, HookProbeResult{Hook: hook, State: state, Error: file.err.Error()})
			if ownership[physical][key] != 0 {
				results = append(results, HookProbeResult{
					Hook: hook, State: "drift",
					Error: fmt.Sprintf("ownership=%d file=unreadable", ownership[physical][key]),
				})
			}
		} else if file.globalIssue != "" {
			results = append(results, HookProbeResult{Hook: hook, State: "broken", Error: file.globalIssue})
		} else if issue := file.eventIssues[hook.Event]; issue != "" {
			results = append(results, HookProbeResult{Hook: hook, State: "broken", Error: issue})
		} else if file.counts[key] == 0 {
			state, detail := missingOrStaleHook(file.allCounts, hook)
			results = append(results, HookProbeResult{Hook: hook, State: state, Error: detail})
		} else {
			results = append(results, HookProbeResult{Hook: hook, State: "ok"})
		}
		if file.err == nil && ownership[physical][key] != file.counts[key] {
			results = append(results, HookProbeResult{
				Hook: hook, State: "drift",
				Error: fmt.Sprintf("ownership=%d file=%d", ownership[physical][key], file.counts[key]),
			})
		}
	}
	// A retired command sitting in a file is invisible to the loop above —
	// it matches no expected hook — so walk every probed file's raw command
	// inventory once for any hook that matches the shared retired-command
	// table, regardless of whether the installer ever wrote or owned it.
	for physical, file := range files {
		if file.err != nil {
			continue
		}
		for key, count := range file.allCounts {
			if count == 0 {
				continue
			}
			name, retired := retiredHookCommandName(key.Command)
			if !retired {
				continue
			}
			results = append(results, HookProbeResult{
				Hook: ExpectedHook{
					Target: fileTargets[physical], File: fileDisplayPaths[physical],
					Event: key.Event, Matcher: key.Matcher, Command: key.Command, Name: name,
				},
				State: "stale", Error: "retired hook command is still present",
			})
		}
	}
	for path, counts := range ownership {
		for key, count := range counts {
			identity := path + "\x00" + key.Event + "\x00" + key.Matcher + "\x00" + key.Command
			if expectedKeys[identity] {
				continue
			}
			results = append(results, HookProbeResult{
				Hook:  ExpectedHook{Target: "ownership", File: path, Event: key.Event, Matcher: key.Matcher, Command: key.Command, Name: "unexpected"},
				State: "drift", Error: fmt.Sprintf("ledger owns %d hook(s) absent from installer expectations", count),
			})
		}
	}
	sort.SliceStable(results, func(left, right int) bool {
		l, r := results[left], results[right]
		return l.Hook.Target+"\x00"+l.Hook.File+"\x00"+l.Hook.Event+"\x00"+l.Hook.Name+"\x00"+l.State <
			r.Hook.Target+"\x00"+r.Hook.File+"\x00"+r.Hook.Event+"\x00"+r.Hook.Name+"\x00"+r.State
	})
	return results
}

func missingOrStaleHook(counts settingsHookCounts, hook ExpectedHook) (string, string) {
	if counts[settingsHookKey{Event: hook.Event, Matcher: hook.Matcher, Command: hook.Command}] > 0 {
		return "broken", "hook type is not command"
	}
	_, arguments, foundArguments := strings.Cut(hook.Command, " ")
	if !foundArguments {
		return "missing", "expected command absent"
	}
	wantedSuffix := " " + arguments
	for key, count := range counts {
		if count == 0 || key.Event != hook.Event || key.Matcher != hook.Matcher {
			continue
		}
		fields := strings.Fields(key.Command)
		if len(fields) > 0 && strings.HasSuffix(key.Command, wantedSuffix) {
			return "broken", fmt.Sprintf("command points at %s", fields[0])
		}
	}
	return "missing", "expected command absent"
}

// inspectExpectedHookDocument validates the JSON shape doctor relies on. The
// installer's ownership counter intentionally remains tolerant because it must
// also remove historical entries; doctor is stricter and only counts typed
// command hooks as healthy.
func inspectExpectedHookDocument(document map[string]any) (settingsHookCounts, settingsHookCounts, string, map[string]string) {
	typed := settingsHookCounts{}
	all := settingsHookCounts{}
	issues := map[string]string{}
	hooksValue, present := document["hooks"]
	if !present {
		return typed, all, "", issues
	}
	events, ok := hooksValue.(map[string]any)
	if !ok {
		return typed, all, "hooks is not an object", issues
	}
	for event, eventValue := range events {
		entries, ok := eventValue.([]any)
		if !ok {
			issues[event] = fmt.Sprintf("event %s is not an array", event)
			continue
		}
		for entryIndex, entryValue := range entries {
			entry, ok := entryValue.(map[string]any)
			if !ok {
				issues[event] = fmt.Sprintf("event %s entry %d is not an object", event, entryIndex)
				continue
			}
			matcher := ""
			if matcherValue, exists := entry["matcher"]; exists {
				var matcherOK bool
				matcher, matcherOK = matcherValue.(string)
				if !matcherOK {
					issues[event] = fmt.Sprintf("event %s entry %d matcher is not a string", event, entryIndex)
					continue
				}
			}
			hookValues, ok := entry["hooks"].([]any)
			if !ok {
				issues[event] = fmt.Sprintf("event %s entry %d hooks is not an array", event, entryIndex)
				continue
			}
			for hookIndex, hookValue := range hookValues {
				hook, ok := hookValue.(map[string]any)
				if !ok {
					issues[event] = fmt.Sprintf("event %s entry %d hook %d is not an object", event, entryIndex, hookIndex)
					continue
				}
				command, ok := hook["command"].(string)
				if !ok || strings.TrimSpace(command) == "" {
					issues[event] = fmt.Sprintf("event %s entry %d hook %d command is not a non-empty string", event, entryIndex, hookIndex)
					continue
				}
				key := settingsHookKey{Event: event, Matcher: matcher, Command: command}
				all[key]++
				if hookType, ok := hook["type"].(string); ok && hookType == "command" {
					typed[key]++
				}
			}
		}
	}
	return typed, all, "", issues
}
