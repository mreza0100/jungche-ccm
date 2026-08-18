package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		Mode: ModeApply, Home: home, Now: now, Runner: runner,
	}); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) == 0 || runner.calls[0] != "systemctl --user is-active --quiet pfm-name-sync.service" {
		t.Fatalf("installer did not probe the running-service gate before touching fixtures: %v", runner.calls)
	}

	for _, path := range []string{canonical, secondary} {
		for _, command := range []string{
			home + "/.local/bin/pfm chat group hook",
			home + "/.local/bin/pfm internal clear-hide",
		} {
			if got := hookCommandCount(t, readFixture(t, path), "", command); got != 1 {
				t.Fatalf("%s has %d copies of %q, want exactly one\n%s", path, got, command, readFixture(t, path))
			}
		}
	}

	var second bytes.Buffer
	report, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Now: now, Stdout: &second, Runner: &fakeRunner{},
	})
	if err != nil || report.Changed != 0 {
		t.Fatalf("second apply report=%#v err=%v\n%s", report, err, second.String())
	}

	codex := readFixture(t, filepath.Join(home, ".codex", "hooks.json"))
	if !strings.Contains(codex, `"matcher": "startup|resume|clear"`) ||
		!strings.Contains(codex, home+"/.local/bin/pfm internal clear-hide") {
		t.Fatalf("Codex clear-hide startup/resume/clear fixture regressed:\n%s", codex)
	}
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
      {"type":"command","command":"`+pfm+` internal clear-hide"}
    ]}]
  }
}`)
	writeFixture(t, secondaryPath, `{"hooks":{"PreToolUse":[{"matcher":"Agent","hooks":[{"type":"command","command":"secondary-keep"}]}]}}`)

	now := func() time.Time { return time.Date(2031, 2, 3, 4, 5, 6, 0, time.UTC) }
	if _, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Now: now, Runner: &fakeRunner{},
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
	if secondary := readFixture(t, secondaryPath); strings.Contains(secondary, " dream hook ") {
		t.Fatalf("migrate-only install invented a dream hook:\n%s", secondary)
	}

	var second bytes.Buffer
	report, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Now: now, Stdout: &second, Runner: &fakeRunner{},
	})
	if err != nil || report.Changed != 0 {
		t.Fatalf("second apply report=%#v err=%v\n%s", report, err, second.String())
	}

	if _, err := Run(context.Background(), Options{
		Mode: ModeUninstall, Home: home, Now: now, Runner: &fakeRunner{},
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
		"SessionEnd":       pfm + " internal clear-hide",
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
	secondary := readFixture(t, secondaryPath)
	for _, command := range []string{
		pfm + " chat group hook",
		pfm + " usage-hook",
		pfm + " internal clear-hide",
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
