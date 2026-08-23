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

// ExpectedHooks names no Codex SessionStart hook at all anymore: the hook is
// retired (see updateCodexHooks), and doctor reporting "missing: run pfm
// install" for a hook that is never coming back would be the lie, not the
// silence.
func TestExpectedHooksEmitsNoCodexHookEvenWithCodexAccountsConfigured(t *testing.T) {
	home := t.TempDir()
	machine := pfmconfig.Config{CodexAccounts: []pfmconfig.CodexAccount{
		{ID: 1, Home: filepath.Join(home, ".codex")},
		{ID: 2, Home: filepath.Join(home, ".codex-2")},
	}}
	hooks := ExpectedHooks(home, machine)
	for _, hook := range hooks {
		if strings.HasPrefix(hook.Target, "codex[") {
			t.Fatalf("ExpectedHooks() still names a Codex hook: %#v", hooks)
		}
	}
}

// A leftover Codex SessionStart clear-kill hook — canonical command, wrong
// type, or the old shell-parent shape — is stripped outright, never
// repaired or migrated forward: there is nothing left to converge it toward.
func TestCodexHookWiringStripsALeftoverClearKillHookInEveryShape(t *testing.T) {
	home := t.TempDir()
	canonical := codexHookTemplate(home).Command
	legacyParent := filepath.Join(home, ".local", "bin", "pfm") + ` internal clear-kill --parent "$PPID"`
	raw := []byte(fmt.Sprintf(
		`{"hooks":{"SessionStart":[`+
			`{"matcher":%q,"hooks":[{"type":"prompt","command":%q}]},`+
			`{"matcher":%q,"hooks":[{"type":"command","command":%q}]}`+
			`]}}`,
		codexClearMatcher, canonical,
		codexClearMatcher, legacyParent,
	))
	updated, changed, owned, err := updateCodexHooks(raw, home, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("Codex hook wiring did not strip the leftover clear-kill hook")
	}
	if len(owned) != 0 {
		t.Fatalf("Codex hook wiring claimed ownership of a hook it just retired: %#v", owned)
	}
	if got := hookCommandCount(t, string(updated), "SessionStart", canonical); got != 0 {
		t.Fatalf("canonical clear-kill count=%d, want zero:\n%s", got, updated)
	}
	if got := hookCommandCount(t, string(updated), "SessionStart", legacyParent); got != 0 {
		t.Fatalf("shell-parent clear-kill count=%d, want zero:\n%s", got, updated)
	}
	if strings.Contains(string(updated), `"SessionStart"`) {
		t.Fatalf("an empty SessionStart array was left behind:\n%s", updated)
	}

	// Idempotent: a second pass over the already-converged file changes
	// nothing further.
	again, changedAgain, _, err := updateCodexHooks(updated, home, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if changedAgain {
		t.Fatalf("a converged Codex hooks file was rewritten again:\n%s", again)
	}
}

func stageExpectedHookFixtures(t *testing.T) (string, pfmconfig.Config) {
	t.Helper()
	home := t.TempDir()
	machine := pfmconfig.Config{
		Accounts: []pfmconfig.Account{{ID: 2, ConfigDir: filepath.Join(home, ".cc", "2")}},
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

func assertHookState(t *testing.T, results []HookProbeResult, hook ExpectedHook, state string) {
	t.Helper()
	for _, result := range results {
		if result.Hook.File == hook.File && result.Hook.Event == hook.Event && result.Hook.Name == hook.Name && result.State == state {
			return
		}
	}
	t.Fatalf("hook %s/%s has no %s result: %#v", hook.Target, hook.Name, state, results)
}
