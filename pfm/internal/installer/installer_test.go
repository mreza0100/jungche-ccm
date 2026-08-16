package installer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/shared"
)

type fakeRunner struct {
	reachable bool
	manager   bool
	calls     []string
}

func (runner *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	call := name + " " + strings.Join(args, " ")
	runner.calls = append(runner.calls, call)
	if call == "systemctl --user status" && runner.reachable {
		return nil
	}
	if call == "systemctl --user show-environment" && runner.manager {
		return nil
	}
	if strings.Contains(call, "is-active") || strings.Contains(call, "is-enabled") || strings.Contains(call, "is-failed") {
		return errors.New("not loaded")
	}
	if call != "systemctl --user status" && runner.manager {
		return nil
	}
	return errors.New("dead user bus")
}

func TestReachableUserBusRefusesBeforeWriting(t *testing.T) {
	home := t.TempDir()
	_, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Runner: &fakeRunner{reachable: true},
	})
	if !errors.Is(err, ErrReachableUserBus) {
		t.Fatalf("Run() error = %v, want ErrReachableUserBus", err)
	}
	if entries, readErr := os.ReadDir(home); readErr != nil || len(entries) != 0 {
		t.Fatalf("reachable-bus refusal wrote files: entries=%v err=%v", entries, readErr)
	}
}

func TestApplyIsSelfContainedIdempotentAndReversible(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, ".claude")
	writeFixture(t, filepath.Join(config, "settings.json"), `{
  "statusLine":{"type":"command","command":"bash ~/.claude/statusline-command.sh"},
  "hooks":{
    "PreToolUse":[{"matcher":"Agent","hooks":[{"type":"command","command":"`+home+`/.local/bin/cc-fleet dream hook agent-inject"}]}],
    "UserPromptSubmit":[
      {"matcher":"","hooks":[{"type":"command","command":"bash ~/.claude/bin/bb-hook.sh"}]},
      {"matcher":"","hooks":[{"type":"command","command":"bash ~/.claude/commands/chat/group.sh hook"}]},
      {"matcher":"","hooks":[{"type":"command","command":"bash /fixture/cc-usage-hook.sh"}]}
    ]
  }
}`)
	writeFixture(t, filepath.Join(home, ".cc", "2", "settings.json"), `{"hooks":{}}`)
	writeFixture(t, filepath.Join(config, ".cc-ls-hidden"), "hidden-b\nhidden-a\n")
	writeFixture(t, filepath.Join(config, "bin", "cc-hide.sh"), "retired\n")
	writeFixture(t, filepath.Join(home, ".zshrc"), "alias keep=yes\nsource /old/cc-fleet.zsh\n")
	unitDirectory := filepath.Join(home, ".config", "systemd", "user")
	legacyPathWant := filepath.Join(unitDirectory, "default.target.wants", "cc-name-sync.path")
	legacyTimerWant := filepath.Join(unitDirectory, "timers.target.wants", "cc-name-sync.timer")
	for _, target := range []string{legacyPathWant, legacyTimerWant} {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join("..", filepath.Base(target)), target); err != nil {
			t.Fatal(err)
		}
	}
	bbTarget := filepath.Join(config, "commands", "bb.md")
	writeFixture(t, bbTarget, "operator copy\n")
	seed := shared.Open(context.Background(), paths.Values{
		Home: home, SharedDB: filepath.Join(home, ".cc", "fleet.db"),
	})
	if err := seed.Hide(context.Background(), "hidden-a", 99); err != nil {
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{}
	now := func() time.Time { return time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC) }
	var preview bytes.Buffer
	previewReport, err := Run(context.Background(), Options{
		Mode: ModeDryRun, Home: home, Now: now, Stdout: &preview, Runner: runner,
	})
	if err != nil || previewReport.Changed == 0 {
		t.Fatalf("dry run report=%#v err=%v", previewReport, err)
	}
	if _, err := os.Lstat(filepath.Join(home, ".local", "share", "pfm", "install")); !os.IsNotExist(err) {
		t.Fatalf("dry run staged assets: %v", err)
	}
	if content := readFixture(t, bbTarget); content != "operator copy\n" {
		t.Fatalf("dry run changed bb.md: %q", content)
	}

	var applied bytes.Buffer
	report, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Now: now, Stdout: &applied, Runner: runner,
	})
	if err != nil || report.Changed == 0 {
		t.Fatalf("apply report=%#v err=%v\n%s", report, err, applied.String())
	}
	managed := filepath.Join(home, ".local", "share", "pfm", "install")
	assertLink(t, bbTarget, filepath.Join(managed, "bb.command.md"))
	assertLink(t, filepath.Join(config, "commands", "chat", "group", "send.md"), filepath.Join(managed, "chat", "group", "send.command.md"))
	assertLink(t, filepath.Join(home, ".agents", "skills", "bb"), filepath.Join(managed, "codex-skills", "bb"))
	assertLink(t, filepath.Join(home, ".config", "systemd", "user", "pfm-name-sync.service"), filepath.Join(managed, "systemd", "pfm-name-sync.service"))
	assertLink(t, filepath.Join(unitDirectory, "default.target.wants", "pfm-name-sync.path"), filepath.Join(unitDirectory, "pfm-name-sync.path"))
	assertLink(t, filepath.Join(unitDirectory, "timers.target.wants", "pfm-name-sync.timer"), filepath.Join(unitDirectory, "pfm-name-sync.timer"))
	for _, retired := range []string{legacyPathWant, legacyTimerWant} {
		if _, err := os.Lstat(retired); !os.IsNotExist(err) {
			t.Fatalf("retired enablement link remains at %s: %v", retired, err)
		}
	}
	if content := readFixture(t, bbTarget+".pre-professor-20300102-030405"); content != "operator copy\n" {
		t.Fatalf("bb backup = %q", content)
	}
	for _, retired := range []string{
		filepath.Join(config, ".cc-ls-hidden"),
		filepath.Join(config, "bin", "cc-hide.sh"),
	} {
		if _, err := os.Lstat(retired); !os.IsNotExist(err) {
			t.Fatalf("retired file remains at %s: %v", retired, err)
		}
	}
	settings := readFixture(t, filepath.Join(config, "settings.json"))
	for _, wanted := range []string{
		home + "/.local/bin/pfm statusline",
		home + "/.local/bin/pfm usage-hook",
		home + "/.local/bin/pfm chat bb",
		home + "/.local/bin/pfm chat group hook",
		home + "/.local/bin/pfm dream hook agent-inject",
	} {
		if !strings.Contains(settings, wanted) {
			t.Fatalf("settings missing %q:\n%s", wanted, settings)
		}
	}
	if zshrc := readFixture(t, filepath.Join(home, ".zshrc")); !strings.Contains(zshrc, sourceLine(filepath.Join(managed, "shim", "pfm.zsh"))) ||
		strings.Contains(zshrc, "cc-fleet.zsh") {
		t.Fatalf("zshrc was not converged:\n%s", zshrc)
	}

	state := shared.Open(context.Background(), paths.Values{
		Home: home, SharedDB: filepath.Join(home, ".cc", "fleet.db"),
	})
	hidden, err := state.HiddenAt(context.Background())
	closeErr := state.Close()
	if err != nil || closeErr != nil || len(hidden) != 2 || hidden["hidden-a"] != 99 || hidden["hidden-b"] != 0 {
		t.Fatalf("migrated hidden=%v err=%v close=%v", hidden, err, closeErr)
	}

	var second bytes.Buffer
	secondReport, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Now: now, Stdout: &second, Runner: runner,
	})
	if err != nil || secondReport.Changed != 0 {
		t.Fatalf("second apply report=%#v err=%v\n%s", secondReport, err, second.String())
	}

	var removed bytes.Buffer
	if _, err := Run(context.Background(), Options{
		Mode: ModeUninstall, Home: home, Now: now, Stdout: &removed, Runner: runner,
	}); err != nil {
		t.Fatalf("uninstall: %v\n%s", err, removed.String())
	}
	if _, err := os.Lstat(bbTarget); err != nil {
		t.Fatalf("uninstall did not restore bb.md: %v", err)
	}
	if content := readFixture(t, bbTarget); content != "operator copy\n" {
		t.Fatalf("restored bb.md = %q", content)
	}
	if _, err := os.Lstat(managed); !os.IsNotExist(err) {
		t.Fatalf("uninstall left managed asset root: %v", err)
	}
	for _, removed := range []string{
		filepath.Join(unitDirectory, "default.target.wants", "pfm-name-sync.path"),
		filepath.Join(unitDirectory, "timers.target.wants", "pfm-name-sync.timer"),
	} {
		if _, err := os.Lstat(removed); !os.IsNotExist(err) {
			t.Fatalf("uninstall left enablement link at %s: %v", removed, err)
		}
	}
}

func TestUninstallDoesNotMigratePredecessorCommands(t *testing.T) {
	home := t.TempDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	oldCommand := home + "/.local/bin/cc-fleet dream hook agent-inject"
	writeFixture(t, settingsPath, `{"hooks":{"PreToolUse":[{"hooks":[{"command":"`+oldCommand+`"}]}]}}`)

	if _, err := Run(context.Background(), Options{
		Mode: ModeUninstall, Home: home, Runner: &fakeRunner{},
	}); err != nil {
		t.Fatal(err)
	}
	if settings := readFixture(t, settingsPath); !strings.Contains(settings, oldCommand) {
		t.Fatalf("uninstall migrated predecessor command:\n%s", settings)
	}
}

func TestUnitTransitionsUseOnlyTheInjectedManager(t *testing.T) {
	home := t.TempDir()
	runner := &fakeRunner{manager: true}
	if _, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Runner: runner,
	}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(runner.calls, "\n")
	for _, wanted := range []string{
		"systemctl --user status",
		"systemctl --user daemon-reload",
		"systemctl --user enable --now pfm-name-sync.path pfm-name-sync.timer",
	} {
		if !strings.Contains(joined, wanted) {
			t.Fatalf("systemctl calls missing %q:\n%s", wanted, joined)
		}
	}
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func assertLink(t *testing.T, target, wanted string) {
	t.Helper()
	got, linked := resolvedLink(target)
	if !linked || got != wanted {
		t.Fatalf("link %s -> %q,%v, want %q,true", target, got, linked, wanted)
	}
}
