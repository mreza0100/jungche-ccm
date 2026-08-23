package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
)

func TestProbeExpectedHooksStatesAndOwnership(t *testing.T) {
	t.Run("all present", func(t *testing.T) {
		home, machine := stageExpectedHookFixtures(t)
		results := ProbeExpectedHooks(home, machine)
		if len(results) != len(ExpectedHooks(home, machine)) {
			t.Fatalf("results=%d expected=%d: %#v", len(results), len(ExpectedHooks(home, machine)), results)
		}
		for _, result := range results {
			if result.State != "ok" {
				t.Errorf("unexpected hook result=%#v", result)
			}
		}
	})

	t.Run("missing owned hook", func(t *testing.T) {
		home, machine := stageExpectedHookFixtures(t)
		hook := findExpectedHook(t, home, machine, "claude[2]", "usage")
		removeHookFixture(t, hook)
		assertHookState(t, ProbeExpectedHooks(home, machine), hook, "missing")
		assertHookState(t, ProbeExpectedHooks(home, machine), hook, "drift")
	})

	t.Run("stale binary", func(t *testing.T) {
		home, machine := stageExpectedHookFixtures(t)
		hook := findExpectedHook(t, home, machine, "claude[2]", "clear-kill")
		raw, err := os.ReadFile(hook.File)
		if err != nil {
			t.Fatal(err)
		}
		stale := strings.ReplaceAll(string(raw), hook.Command, "/usr/local/bin/pfm internal clear-kill")
		if err := os.WriteFile(hook.File, []byte(stale), 0o600); err != nil {
			t.Fatal(err)
		}
		assertHookState(t, ProbeExpectedHooks(home, machine), hook, "broken")
	})

	t.Run("corrupt Codex hooks", func(t *testing.T) {
		home, machine := stageExpectedHookFixtures(t)
		hook := findExpectedHook(t, home, machine, "codex[1]", "clear-kill")
		if err := os.WriteFile(hook.File, []byte("{broken\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		assertHookState(t, ProbeExpectedHooks(home, machine), hook, "broken")
	})

	t.Run("canonical command with wrong hook type", func(t *testing.T) {
		home, machine := stageExpectedHookFixtures(t)
		hook := findExpectedHook(t, home, machine, "codex[1]", "clear-kill")
		setHookTypeFixture(t, hook, "prompt")
		assertHookState(t, ProbeExpectedHooks(home, machine), hook, "broken")
	})
}

// TestProbeExpectedHooksFlagsRetiredHookCommandsAsStale pins the deep-doctor
// defect: a settings.json that still carries the pre-rename `internal
// clear-hide` command (retired in favor of `internal clear-kill`; `internal
// clear-hide` is not a recognized subcommand at all — cmd/pfm/main.go's
// `internal` dispatch falls through to its usage error for it) produces NO
// probe row at all. ProbeExpectedHooks only ever walks the installer's own
// `expected` hook list; a command that matches no expected hook AND is not
// (and never was) claimed by the ownership ledger is invisible end to end —
// doctor prints nothing about a stale command sitting in a live host's hooks.
func TestProbeExpectedHooksFlagsRetiredHookCommandsAsStale(t *testing.T) {
	home, machine := stageExpectedHookFixtures(t)
	hook := findExpectedHook(t, home, machine, "claude[2]", "clear-kill")
	retired := strings.Replace(hook.Command, "internal clear-kill", "internal clear-hide", 1)
	if retired == hook.Command {
		t.Fatalf("fixture command %q did not contain internal clear-kill", hook.Command)
	}

	raw, err := os.ReadFile(hook.File)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	appendHookWithMatcher(document, hook.Event, hook.Matcher, retired)
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook.File, append(updated, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	results := ProbeExpectedHooks(home, machine)
	var retiredResult *HookProbeResult
	warnings := 0
	for index, result := range results {
		if result.State != "ok" {
			warnings++
		}
		if strings.Contains(result.Hook.Command, "clear-hide") {
			retiredResult = &results[index]
		}
	}
	if retiredResult == nil {
		t.Fatalf("doctor hook probe said nothing about the retired command %q: %#v", retired, results)
	}
	if !strings.Contains(strings.ToLower(retiredResult.State), "stale") {
		t.Fatalf("retired command result state=%q, want it to say stale: %#v", retiredResult.State, retiredResult)
	}
	if warnings == 0 {
		t.Fatalf("retired command did not surface as a warning-worthy probe result: %#v", results)
	}
}

func TestExpectedHooksIteratesEveryConfiguredCodexHome(t *testing.T) {
	home := t.TempDir()
	machine := pfmconfig.Config{CodexAccounts: []pfmconfig.CodexAccount{
		{ID: 1, Home: filepath.Join(home, ".codex")},
		{ID: 2, Home: filepath.Join(home, ".codex-2")},
	}}
	hooks := ExpectedHooks(home, machine)
	got := map[string]string{}
	for _, hook := range hooks {
		if strings.HasPrefix(hook.Target, "codex[") {
			got[hook.Target] = hook.File
		}
	}
	want := map[string]string{
		"codex[1]": filepath.Join(home, ".codex", "hooks.json"),
		"codex[2]": filepath.Join(home, ".codex-2", "hooks.json"),
	}
	if len(hooks) != 2 || len(got) != 2 || got["codex[1]"] != want["codex[1]"] || got["codex[2]"] != want["codex[2]"] {
		t.Fatalf("ExpectedHooks()=%#v, want only both configured Codex homes", hooks)
	}
}

func TestHookWiringRepairsCanonicalCommandType(t *testing.T) {
	home := t.TempDir()
	expected := codexHookTemplate(home)
	raw := []byte(fmt.Sprintf(`{"hooks":{"SessionStart":[{"matcher":%q,"hooks":[{"type":"prompt","command":%q}]}]}}`, expected.Matcher, expected.Command))
	updated, changed, _, err := updateCodexHooks(raw, home, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("Codex hook wiring did not repair the canonical command type")
	}
	var document map[string]any
	if err := json.Unmarshal(updated, &document); err != nil {
		t.Fatal(err)
	}
	typed, _, globalIssue, eventIssues := inspectExpectedHookDocument(document)
	key := settingsHookKey{Event: expected.Event, Matcher: expected.Matcher, Command: expected.Command}
	if globalIssue != "" || eventIssues[expected.Event] != "" || typed[key] != 1 {
		t.Fatalf("repaired hook is not a typed command: typed=%#v global=%q events=%#v", typed, globalIssue, eventIssues)
	}
}

func TestCodexHookWiringMigratesParentlessClearKill(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, ".local", "bin", "pfm") + " internal clear-kill"
	raw := []byte(fmt.Sprintf(`{"hooks":{"SessionStart":[{"matcher":%q,"hooks":[{"type":"command","command":%q}]}]}}`, codexClearMatcher, legacy))
	updated, changed, _, err := updateCodexHooks(raw, home, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("Codex hook wiring did not migrate the parentless clear-kill command")
	}
	canonical := codexHookTemplate(home).Command
	if got := hookCommandCount(t, string(updated), "SessionStart", canonical); got != 1 {
		t.Fatalf("canonical clear-kill count=%d, want one:\n%s", got, updated)
	}
	if got := hookCommandCount(t, string(updated), "SessionStart", legacy); got != 0 {
		t.Fatalf("parentless clear-kill count=%d, want zero:\n%s", got, updated)
	}
}

func stageExpectedHookFixtures(t *testing.T) (string, pfmconfig.Config) {
	t.Helper()
	home := t.TempDir()
	machine := pfmconfig.Config{
		Accounts:      []pfmconfig.Account{{ID: 2, ConfigDir: filepath.Join(home, ".cc", "2")}},
		CodexAccounts: []pfmconfig.CodexAccount{{ID: 1, Home: filepath.Join(home, ".codex")}},
	}
	ownership := map[string]settingsHookCounts{}
	seen := map[string]bool{}
	for _, hook := range ExpectedHooks(home, machine) {
		physical := physicalSettingsPath(hook.File)
		if seen[physical] {
			continue
		}
		seen[physical] = true
		raw := []byte("{\"hooks\":{}}\n")
		if strings.HasPrefix(hook.Target, "codex[") {
			updated, _, owned, err := updateCodexHooks(raw, home, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			writeFixture(t, hook.File, string(updated))
			ownership[physical] = owned
			continue
		}
		updated, _, owned, err := updateSettings(raw, home, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, hook.File, string(updated))
		ownership[physical] = owned
	}
	encoded, err := encodeSettingsHookOwnership(ownership)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(home, ".local", "share", "pfm", "install", "settings-hook-ownership.json"), string(encoded))
	return home, machine
}

func findExpectedHook(t *testing.T, home string, machine pfmconfig.Config, target, name string) ExpectedHook {
	t.Helper()
	for _, hook := range ExpectedHooks(home, machine) {
		if hook.Target == target && hook.Name == name {
			return hook
		}
	}
	t.Fatalf("expected hook %s %s not found", target, name)
	return ExpectedHook{}
}

func removeHookFixture(t *testing.T, hook ExpectedHook) {
	t.Helper()
	raw, err := os.ReadFile(hook.File)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	key := settingsHookKey{Event: hook.Event, Matcher: hook.Matcher, Command: hook.Command}
	if !removeOwnedSettingsHooks(document, settingsHookCounts{key: 1}) {
		t.Fatalf("fixture did not contain %#v", key)
	}
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook.File, append(updated, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func setHookTypeFixture(t *testing.T, expected ExpectedHook, hookType string) {
	t.Helper()
	raw, err := os.ReadFile(expected.File)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	events, _ := document["hooks"].(map[string]any)
	entries, _ := events[expected.Event].([]any)
	found := false
	for _, entryValue := range entries {
		entry, _ := entryValue.(map[string]any)
		if matcher, _ := entry["matcher"].(string); matcher != expected.Matcher {
			continue
		}
		hooks, _ := entry["hooks"].([]any)
		for _, hookValue := range hooks {
			hook, _ := hookValue.(map[string]any)
			if hook["command"] == expected.Command {
				hook["type"] = hookType
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("fixture did not contain %s", expected.Command)
	}
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expected.File, append(updated, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertHookState(t *testing.T, results []HookProbeResult, hook ExpectedHook, state string) {
	t.Helper()
	for _, result := range results {
		if result.Hook.File == hook.File && result.Hook.Event == hook.Event && result.Hook.Name == hook.Name && result.State == state {
			return
		}
	}
	t.Fatalf("hook %s/%s has no %s result: %#v", hook.Target, hook.Name, state, results)
}
