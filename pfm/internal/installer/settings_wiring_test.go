package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pfmconfig "hostops/pfm/internal/config"
)

func TestEveryClaudeSettingsFileGetsCompleteHookWiring(t *testing.T) {
	home := t.TempDir()
	canonical := filepath.Join(home, ".claude", "settings.json")
	secondary := filepath.Join(home, ".cc", "4", "settings.json")
	writeFixture(t, canonical, `{"hooks":{"UserPromptSubmit":[{"matcher":"","hooks":[{"type":"command","command":"canonical-keep"}]}]}}`)
	writeFixture(t, secondary, `{"hooks":{"SessionEnd":[{"matcher":"","hooks":[{"type":"command","command":"secondary-keep"}]}]}}`)
	canonicalAlias := filepath.Join(home, ".cc", "1", "settings.json")
	if err := os.MkdirAll(filepath.Dir(canonicalAlias), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(canonical, canonicalAlias); err != nil {
		t.Fatal(err)
	}

	now := func() time.Time { return time.Date(2031, 2, 3, 4, 5, 6, 0, time.UTC) }
	runner := &fakeRunner{}
	if _, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, ConfigDirs: []string{filepath.Join(home, ".claude"), filepath.Join(home, ".cc", "4")}, Now: now, Runner: runner,
	}); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) == 0 || runner.calls[0] != "systemctl --user is-active --quiet pfm-name-sync.service" {
		t.Fatalf("installer did not probe the running-service gate before touching fixtures: %v", runner.calls)
	}

	for _, path := range []string{canonical, secondary} {
		for _, command := range []string{
			home + "/.local/bin/pfm chat group hook",
			home + "/.local/bin/pfm internal clear-kill",
		} {
			if got := hookCommandCount(t, readFixture(t, path), "", command); got != 1 {
				t.Fatalf("%s has %d copies of %q, want exactly one\n%s", path, got, command, readFixture(t, path))
			}
		}
	}

	var second bytes.Buffer
	report, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, ConfigDirs: []string{filepath.Join(home, ".claude"), filepath.Join(home, ".cc", "4")}, Now: now, Stdout: &second, Runner: &fakeRunner{},
	})
	if err != nil || report.Changed != 0 {
		t.Fatalf("second apply report=%#v err=%v\n%s", report, err, second.String())
	}

	codex := readFixture(t, filepath.Join(home, ".codex", "hooks.json"))
	if !strings.Contains(codex, `"matcher": "startup|resume|clear"`) ||
		!strings.Contains(codex, home+"/.local/bin/pfm internal clear-kill") {
		t.Fatalf("Codex clear-kill startup/resume/clear fixture regressed:\n%s", codex)
	}
}

func TestSettingsInstallAddsWaveHooksCleanupAndOwnsOnlyItsEntries(t *testing.T) {
	home := filepath.Join("neutral", "home")
	raw := []byte(`{"hooks":{"UserPromptSubmit":[{"matcher":"","hooks":[{"type":"command","command":"manual-keep"}]}]},"cleanupPeriodDays":42}`)
	updated, changed, owned, err := updateSettings(raw, home, false, nil)
	if err != nil || !changed {
		t.Fatalf("updateSettings changed=%v err=%v", changed, err)
	}
	var document map[string]any
	if err := json.Unmarshal(updated, &document); err != nil {
		t.Fatal(err)
	}
	if got := document["cleanupPeriodDays"]; got != float64(42) {
		t.Fatalf("cleanupPeriodDays=%v, want existing value 42", got)
	}
	prefix := home + "/.local/bin/pfm"
	for _, hook := range []struct {
		event, matcher, command string
	}{
		{"PreToolUse", "Agent|Task", prefix + " dream hook agent-inject"},
		{"PreToolUse", "Agent|Task", prefix + " internal explore-deny"},
		{"UserPromptSubmit", "", prefix + " dream hook nudge"},
		{"UserPromptSubmit", "", prefix + " internal epic-inject"},
		{"UserPromptSubmit", "", prefix + " chat group hook"},
		{"UserPromptSubmit", "", prefix + " usage-hook"},
		{"SessionStart", "", prefix + " internal launcher-repair"},
		{"SessionEnd", "", prefix + " internal clear-kill"},
	} {
		if got := hookCommandCount(t, string(updated), hook.event, hook.command); got != 1 {
			t.Fatalf("%s %s count=%d, want 1\n%s", hook.event, hook.command, got, updated)
		}
		if got := hookMatcherCount(t, string(updated), hook.event, hook.command, hook.matcher); got != 1 {
			t.Fatalf("%s %s matcher count=%d, want 1\n%s", hook.event, hook.command, got, updated)
		}
	}
	if len(owned) != 8 {
		t.Fatalf("owned hooks=%d, want 8: %#v", len(owned), owned)
	}

	withManual := append([]byte(`{"hooks":{"PreToolUse":[{"matcher":"Agent|Task","hooks":[{"type":"command","command":"operator-keep"}]}]}}`), '\n')
	updated, _, owned, err = updateSettings(withManual, home, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	removed, changed, _, err := updateSettings(updated, home, true, owned)
	if err != nil || !changed {
		t.Fatalf("uninstall changed=%v err=%v", changed, err)
	}
	if strings.Contains(string(removed), prefix+" ") || !strings.Contains(string(removed), "operator-keep") {
		t.Fatalf("uninstall ownership result=%s", removed)
	}
}

// TestInstallOwnershipLedgerClaimsHooksAlreadyPresentInSettings pins the
// deep-doctor defect: a host whose settings.json already carries every
// expected hook command — because it predates the ownership ledger, or a
// prior install ran before the ledger existed for that hook — never gets
// those hooks claimed. nextSettingsHookOwnership only credits a hook whose
// per-call count went UP (before[key] != after[key]); a hook that was already
// there and stays unchanged contributes zero "added" count forever, so the
// ledger stays empty for it and doctor reports permanent ownership drift
// (`ownership=0 file=1`) for a hook that is, in fact, correctly wired.
func TestInstallOwnershipLedgerClaimsHooksAlreadyPresentInSettings(t *testing.T) {
	home := t.TempDir()

	// Build the settings document a fully-wired install produces — this is
	// exactly what a real host's settings.json looks like today, regardless
	// of whether any ledger ever tracked it.
	preexisting, _, _, err := updateSettings([]byte("{}\n"), home, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Today's install run starts from an EMPTY ownership ledger against that
	// already-wired file — the exact shape `wireSettings` reads via
	// `ownership[physical]` when the ledger has no entry for this path yet.
	_, changed, owned, err := updateSettings(preexisting, home, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatalf("an already fully-wired settings.json was unexpectedly rewritten")
	}
	if len(owned) != 8 {
		t.Fatalf("owned hooks=%d, want 8 — every already-present expected hook must be claimed: %#v", len(owned), owned)
	}
	for key, count := range owned {
		if count != 1 {
			t.Fatalf("owned[%#v]=%d, want 1", key, count)
		}
	}

	// The doctor hook probe must agree: an already-owned hook is never drift.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	writeFixture(t, settingsPath, string(preexisting))
	encoded, err := encodeSettingsHookOwnership(map[string]settingsHookCounts{physicalSettingsPath(settingsPath): owned})
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(home, ".local", "share", "pfm", "install", "settings-hook-ownership.json"), string(encoded))
	machine := pfmconfig.Config{Accounts: []pfmconfig.Account{{ID: 1, ConfigDir: filepath.Join(home, ".claude")}}}
	for _, result := range ProbeExpectedHooks(home, machine) {
		if result.State == "drift" {
			t.Fatalf("hook probe reported drift for an already-owned hook: %#v", result)
		}
	}
}

// TestSettingsInstallRemovesRetiredClearHideAndKeepsOneClearKill pins the
// deep-doctor defect: the kill-rename retired `internal clear-hide` in favor
// of `internal clear-kill` (cmd/pfm/main.go's `internal` dispatch has no
// `clear-hide` case at all — it falls through to the usage error), but the
// installer's SessionEnd wiring only recognizes and dedups the CURRENT
// clear-kill command. A settings.json still carrying the pre-rename command
// keeps it forever; the installer neither removes it nor even notices it.
func TestSettingsInstallRemovesRetiredClearHideAndKeepsOneClearKill(t *testing.T) {
	home := filepath.Join("neutral", "home")
	binary := home + "/.local/bin/pfm"
	retired := binary + " internal clear-hide"
	raw := []byte(`{"hooks":{"SessionEnd":[{"matcher":"","hooks":[{"type":"command","command":"` + retired + `"}]}]}}`)

	updated, changed, owned, err := updateSettings(raw, home, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatalf("retired clear-hide hook was not rewritten at all")
	}
	if strings.Contains(string(updated), "clear-hide") {
		t.Fatalf("retired command %q survived installer wiring:\n%s", retired, updated)
	}
	clearKill := binary + " internal clear-kill"
	if got := hookCommandCount(t, string(updated), "SessionEnd", clearKill); got != 1 {
		t.Fatalf("clear-kill count=%d after wiring, want exactly 1:\n%s", got, updated)
	}
	if owned[settingsHookKey{Event: "SessionEnd", Command: clearKill}] != 1 {
		t.Fatalf("owned ledger did not claim the replacement clear-kill hook: %#v", owned)
	}
}

func TestShimAssetEmitsConfiguredClaudeAndCodexPosture(t *testing.T) {
	raw, err := readAsset("shim/pfm.zsh")
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(renderShimAsset(raw, Options{
		ClaudePrompted: map[int]bool{1: false, 2: true},
		CodexYolo:      map[int]bool{1: true, 2: false},
	}))
	for _, want := range []string{
		"[1]=0",
		"[2]=1",
		"PFM_CODEX_YOLO",
		"[2]=0",
		"autonomy_flags=(--allow-dangerously-skip-permissions --dangerously-skip-permissions)",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered shim missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "CC_AUTONOMY_FLAGS") || strings.Contains(rendered, "codex --dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("rendered shim retained unconditional posture flags:\n%s", rendered)
	}
}

func hookMatcherCount(t *testing.T, raw, event, command, matcher string) int {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		t.Fatal(err)
	}
	events, _ := document["hooks"].(map[string]any)
	count := 0
	entries, _ := events[event].([]any)
	for _, value := range entries {
		entry, _ := value.(map[string]any)
		if entry["matcher"] != matcher {
			continue
		}
		hooks, _ := entry["hooks"].([]any)
		for _, hookValue := range hooks {
			hook, _ := hookValue.(map[string]any)
			if hook["command"] == command {
				count++
			}
		}
	}
	return count
}

func TestDreamHookMigrationIsMigrateOnlyAndUninstallPreservesManualHooks(t *testing.T) {
	home := t.TempDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	secondaryPath := filepath.Join(home, ".cc", "3", "settings.json")
	pfm := home + "/.local/bin/pfm"
	writeFixture(t, settingsPath, `{
  "hooks": {
    "PreToolUse": [{"matcher":"Agent","hooks":[
      {"type":"command","command":"bash `+home+`/.claude/hooks/dreamer-agent-inject.sh --fixture"},
      {"type":"command","command":"agent-neighbor"}
    ]}],
    "SessionStart": [{"matcher":"startup","hooks":[
      {"type":"command","command":"bash `+home+`/.claude/hooks/dreamer-nudge.sh"},
      {"type":"command","command":"nudge-neighbor"}
    ]}],
    "PostToolUse": [{"matcher":"Agent","hooks":[
      {"type":"command","command":"`+pfm+` dream hook agent-inject"},
      {"type":"command","command":"operator-keep"}
    ]}],
    "UserPromptSubmit": [{"matcher":"","hooks":[
      {"type":"command","command":"`+pfm+` chat group hook"},
      {"type":"command","command":"`+pfm+` usage-hook"}
    ]}],
    "SessionEnd": [{"matcher":"","hooks":[
      {"type":"command","command":"`+pfm+` internal clear-kill"}
    ]}]
  }
}`)
	writeFixture(t, secondaryPath, `{"hooks":{"PreToolUse":[{"matcher":"Agent","hooks":[{"type":"command","command":"secondary-keep"}]}]}}`)

	now := func() time.Time { return time.Date(2031, 2, 3, 4, 5, 6, 0, time.UTC) }
	if _, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, ConfigDirs: []string{filepath.Join(home, ".claude"), filepath.Join(home, ".cc", "3")}, Now: now, Runner: &fakeRunner{},
	}); err != nil {
		t.Fatal(err)
	}
	ownershipPath := filepath.Join(home, ".local", "share", "pfm", "install", "settings-hook-ownership.json")
	if info, err := os.Stat(ownershipPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("ownership ledger mode=%v err=%v, want 0600", info, err)
	}
	settings := readFixture(t, settingsPath)
	if strings.Contains(settings, "dreamer-agent-inject.sh") || strings.Contains(settings, "dreamer-nudge.sh") {
		t.Fatalf("legacy dream hook scripts survived migration:\n%s", settings)
	}
	if got := hookCommandCount(t, settings, "PreToolUse", pfm+" dream hook agent-inject"); got != 1 {
		t.Fatalf("PreToolUse agent-inject count=%d, want 1\n%s", got, settings)
	}
	if got := hookCommandCount(t, settings, "SessionStart", pfm+" dream hook nudge"); got != 1 {
		t.Fatalf("SessionStart nudge count=%d, want 1\n%s", got, settings)
	}
	if got := hookCommandCount(t, settings, "PostToolUse", pfm+" dream hook agent-inject"); got != 1 {
		t.Fatalf("manual PostToolUse hook moved or disappeared: count=%d\n%s", got, settings)
	}
	for _, neighbor := range []string{"agent-neighbor", "nudge-neighbor"} {
		if !strings.Contains(settings, neighbor) {
			t.Fatalf("dream migration dropped same-event sibling %q:\n%s", neighbor, settings)
		}
	}
	secondary := readFixture(t, secondaryPath)
	for _, command := range []string{
		pfm + " dream hook agent-inject",
		pfm + " dream hook nudge",
		pfm + " internal explore-deny",
		pfm + " internal epic-inject",
		pfm + " internal launcher-repair",
	} {
		if got := hookCommandCount(t, secondary, "", command); got != 1 {
			t.Fatalf("managed secondary settings missing new installer hook %q: count=%d\n%s", command, got, secondary)
		}
	}

	var second bytes.Buffer
	report, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, ConfigDirs: []string{filepath.Join(home, ".claude"), filepath.Join(home, ".cc", "3")}, Now: now, Stdout: &second, Runner: &fakeRunner{},
	})
	if err != nil || report.Changed != 0 {
		t.Fatalf("second apply report=%#v err=%v\n%s", report, err, second.String())
	}

	if _, err := Run(context.Background(), Options{
		Mode: ModeUninstall, Home: home, ConfigDirs: []string{filepath.Join(home, ".claude"), filepath.Join(home, ".cc", "3")}, Now: now, Runner: &fakeRunner{},
	}); err != nil {
		t.Fatal(err)
	}
	settings = readFixture(t, settingsPath)
	for event, command := range map[string]string{
		"PreToolUse":   pfm + " dream hook agent-inject",
		"SessionStart": pfm + " dream hook nudge",
	} {
		if got := hookCommandCount(t, settings, event, command); got != 0 {
			t.Fatalf("uninstall retained installer-migrated %s hook: count=%d\n%s", event, got, settings)
		}
	}
	for event, command := range map[string]string{
		"PostToolUse":      pfm + " dream hook agent-inject",
		"UserPromptSubmit": pfm + " chat group hook",
		"SessionEnd":       pfm + " internal clear-kill",
	} {
		if got := hookCommandCount(t, settings, event, command); got != 1 {
			t.Fatalf("uninstall deleted manually wired %s hook: count=%d want 1\n%s", event, got, settings)
		}
	}
	if got := hookCommandCount(t, settings, "UserPromptSubmit", pfm+" usage-hook"); got != 1 {
		t.Fatalf("uninstall deleted manually wired usage hook: count=%d want 1\n%s", got, settings)
	}
	if !strings.Contains(settings, "operator-keep") {
		t.Fatalf("uninstall deleted unrelated hook:\n%s", settings)
	}
	secondary = readFixture(t, secondaryPath)
	for _, command := range []string{
		pfm + " chat group hook",
		pfm + " usage-hook",
		pfm + " internal clear-kill",
		pfm + " dream hook agent-inject",
		pfm + " dream hook nudge",
		pfm + " internal explore-deny",
		pfm + " internal epic-inject",
		pfm + " internal launcher-repair",
	} {
		if got := hookCommandCount(t, secondary, "", command); got != 0 {
			t.Fatalf("uninstall retained installer-owned secondary hook %q: count=%d\n%s", command, got, secondary)
		}
	}
	if !strings.Contains(secondary, "secondary-keep") {
		t.Fatalf("uninstall deleted unrelated secondary hook:\n%s", secondary)
	}
	if _, err := os.Stat(ownershipPath); !os.IsNotExist(err) {
		t.Fatalf("uninstall retained hook ownership ledger: %v", err)
	}
}

func hookCommandCount(t *testing.T, raw, event, wanted string) int {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		t.Fatalf("decode settings fixture: %v\n%s", err, raw)
	}
	events, _ := document["hooks"].(map[string]any)
	count := 0
	for eventName, eventValue := range events {
		if event != "" && eventName != event {
			continue
		}
		entries, _ := eventValue.([]any)
		for _, entryValue := range entries {
			entry, _ := entryValue.(map[string]any)
			hooks, _ := entry["hooks"].([]any)
			for _, hookValue := range hooks {
				hook, _ := hookValue.(map[string]any)
				if hook["command"] == wanted {
					count++
				}
			}
		}
	}
	return count
}

func TestUninstallRefusesToStrandOwnedHookInInvalidCodexJSON(t *testing.T) {
	home := t.TempDir()
	managed := filepath.Join(home, ".local", "share", "pfm", "install")
	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	writeFixture(t, hooksPath, "{broken\n")
	expected := codexHookTemplate(home)
	physical := physicalSettingsPath(hooksPath)
	encoded, err := encodeSettingsHookOwnership(map[string]settingsHookCounts{
		physical: {{Event: expected.Event, Matcher: expected.Matcher, Command: expected.Command}: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(managed, "settings-hook-ownership.json"), string(encoded))
	installer := engine{options: Options{Mode: ModeUninstall, Home: home}, managedRoot: managed}
	err = installer.wireCodexHooks()
	if err == nil || !strings.Contains(err.Error(), "refuse to strand owned hooks in invalid Codex hooks JSON") {
		t.Fatalf("wireCodexHooks error=%v", err)
	}
}

func TestCodexHookWiringIteratesConfiguredHomes(t *testing.T) {
	home := t.TempDir()
	homes := []string{filepath.Join(home, ".codex"), filepath.Join(home, ".codex-2")}
	installer := engine{
		options: Options{Mode: ModeApply, Home: home, CodexHomes: homes, Stdout: io.Discard},
		apply:   true, managedRoot: filepath.Join(home, ".local", "share", "pfm", "install"),
	}
	if err := installer.wireCodexHooks(); err != nil {
		t.Fatal(err)
	}
	for _, codexHome := range homes {
		raw := readFixture(t, filepath.Join(codexHome, "hooks.json"))
		if got := hookCommandCount(t, raw, "SessionStart", home+"/.local/bin/pfm internal clear-kill"); got != 1 {
			t.Fatalf("%s has %d canonical clear-kill hooks, want one\n%s", codexHome, got, raw)
		}
	}
}
