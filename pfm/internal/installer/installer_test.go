package installer

import (
	"bytes"
	"context"
	"errors"
	"io"
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
	writeFixture(t, filepath.Join(home, ".codex", "hooks.json"), `{
  "hooks": {
    "SubagentStart": [{"matcher":"","hooks":[
      {"type":"command","command":"`+home+`/.local/bin/cc-fleet dream hook codex-subagent-inject"},
      {"type":"command","command":"fixture-codex-keep"}
    ]}]
  }
}`)
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
	secondarySettings := filepath.Join(home, ".cc", "2", "settings.json")
	writeFixture(t, secondarySettings, `{"hooks":{"UserPromptSubmit":[{"matcher":"","hooks":[{"type":"command","command":"pfm chat bb"},{"type":"command","command":"secondary-keep"}]}]}}`)
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
	if content := readFixture(t, bbTarget); content != "operator copy\n" {
		t.Fatalf("install changed operator bb.md: %q", content)
	}
	assertLink(t, filepath.Join(config, "commands", "chat", "group", "send.md"), filepath.Join(managed, "chat", "group", "send.command.md"))
	// One job, two schedulers: assert the one this platform actually installs,
	// and that it did NOT leave the other platform's files behind.
	if schedulerIsLaunchd {
		agent := filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
		info, err := os.Lstat(agent)
		if err != nil {
			t.Fatalf("launch agent was not installed: %v", err)
		}
		// launchd silently ignores a symlinked agent, so this must be a real file.
		if info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("launch agent is a symlink; launchd will never load it")
		}
		plist := readFixture(t, agent)
		if strings.Contains(plist, "__PFM_HOME__") {
			t.Fatalf("launch agent kept its placeholder:\n%s", plist)
		}
		if !strings.Contains(plist, home+"/.local/bin/pfm") ||
			!strings.Contains(plist, home+"/.codex/session_index.jsonl") {
			t.Fatalf("launch agent does not point at this home:\n%s", plist)
		}
		if _, err := os.Lstat(filepath.Join(managed, "systemd")); !os.IsNotExist(err) {
			t.Fatalf("systemd units were staged on a launchd host: %v", err)
		}
	} else {
		assertLink(t, filepath.Join(home, ".config", "systemd", "user", "pfm-name-sync.service"), filepath.Join(managed, "systemd", "pfm-name-sync.service"))
		assertLink(t, filepath.Join(unitDirectory, "default.target.wants", "pfm-name-sync.path"), filepath.Join(unitDirectory, "pfm-name-sync.path"))
		assertLink(t, filepath.Join(unitDirectory, "timers.target.wants", "pfm-name-sync.timer"), filepath.Join(unitDirectory, "pfm-name-sync.timer"))
	}
	// The predecessor's enablement links are a systemd concept; a launchd host
	// never wires systemd at all, so it has none to retire.
	if !schedulerIsLaunchd {
		for _, retired := range []string{legacyPathWant, legacyTimerWant} {
			if _, err := os.Lstat(retired); !os.IsNotExist(err) {
				t.Fatalf("retired enablement link remains at %s: %v", retired, err)
			}
		}
	}
	if _, err := os.Lstat(bbTarget + ".pre-professor-20300102-030405"); !os.IsNotExist(err) {
		t.Fatalf("install backed up an unowned bb.md: %v", err)
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
		home + "/.local/bin/pfm internal clear-hide",
		home + "/.local/bin/pfm chat group hook",
		home + "/.local/bin/pfm dream hook agent-inject",
	} {
		if !strings.Contains(settings, wanted) {
			t.Fatalf("settings missing %q:\n%s", wanted, settings)
		}
	}
	for _, retired := range []string{"chat bb", "pfm bb", "bb-hook.sh"} {
		if strings.Contains(settings, retired) {
			t.Fatalf("settings retained retired /bb wiring %q:\n%s", retired, settings)
		}
	}
	codexHooks := readFixture(t, filepath.Join(home, ".codex", "hooks.json"))
	for _, wanted := range []string{
		home + "/.local/bin/pfm internal clear-hide",
		home + "/.local/bin/pfm dream hook codex-subagent-inject",
		"fixture-codex-keep",
		`"SessionStart"`,
		`"matcher": "startup|resume|clear"`,
	} {
		if !strings.Contains(codexHooks, wanted) {
			t.Fatalf("Codex hooks missing %q:\n%s", wanted, codexHooks)
		}
	}
	if strings.Contains(codexHooks, "/cc-fleet ") {
		t.Fatalf("Codex hooks retained predecessor command:\n%s", codexHooks)
	}
	secondary := readFixture(t, secondarySettings)
	if strings.Contains(secondary, "chat bb") ||
		!strings.Contains(secondary, home+"/.local/bin/pfm chat group hook") ||
		!strings.Contains(secondary, home+"/.local/bin/pfm internal clear-hide") ||
		!strings.Contains(secondary, "secondary-keep") {
		t.Fatalf("secondary settings did not receive the complete hook wiring:\n%s", secondary)
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
	if content := readFixture(t, bbTarget); content != "operator copy\n" {
		t.Fatalf("uninstall changed operator bb.md = %q", content)
	}
	if settings := readFixture(t, filepath.Join(config, "settings.json")); strings.Contains(settings, "internal clear-hide") {
		t.Fatalf("uninstall retained clear-hide hook:\n%s", settings)
	}
	if codexHooks := readFixture(t, filepath.Join(home, ".codex", "hooks.json")); strings.Contains(codexHooks, "internal clear-hide") ||
		!strings.Contains(codexHooks, "fixture-codex-keep") ||
		!strings.Contains(codexHooks, "pfm dream hook codex-subagent-inject") {
		t.Fatalf("uninstall did not remove only the owned Codex clear hook:\n%s", codexHooks)
	}
	if _, err := os.Lstat(managed); !os.IsNotExist(err) {
		t.Fatalf("uninstall left managed asset root: %v", err)
	}
	if schedulerIsLaunchd {
		agent := filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
		if _, err := os.Lstat(agent); !os.IsNotExist(err) {
			t.Fatalf("uninstall left the launch agent at %s: %v", agent, err)
		}
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

func TestApplyRetiresInstalledBBCardsAndHook(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, ".claude")
	managed := filepath.Join(home, ".local", "share", "pfm", "install")
	commandTarget := filepath.Join(config, "commands", "bb.md")
	skillTarget := filepath.Join(home, ".agents", "skills", "bb")
	writeFixture(t, filepath.Join(managed, "bb.command.md"), "old managed command\n")
	writeFixture(t, filepath.Join(managed, "codex-skills", "bb", "SKILL.md"), "old managed skill\n")
	writeFixture(t, filepath.Join(managed, "codex-skills", "bb", "agents", "openai.yaml"), "old managed metadata\n")
	if err := os.MkdirAll(filepath.Dir(commandTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(managed, "bb.command.md"), commandTarget); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, commandTarget+".pre-professor-20300102-030405", "operator command\n")
	if err := os.MkdirAll(filepath.Dir(skillTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(managed, "codex-skills", "bb"), skillTarget); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(config, "settings.json"), `{
  "hooks": {
    "UserPromptSubmit": [
      {"matcher":"","hooks":[
        {"type":"command","command":"`+home+`/.local/bin/pfm chat bb"},
        {"type":"command","command":"fixture-keep"}
      ]}
    ]
  }
}`)

	now := func() time.Time { return time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC) }
	if _, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Now: now, Runner: &fakeRunner{},
	}); err != nil {
		t.Fatal(err)
	}
	if content := readFixture(t, commandTarget); content != "operator command\n" {
		t.Fatalf("retirement did not restore operator card: %q", content)
	}
	for _, retired := range []string{
		skillTarget,
		filepath.Join(managed, "bb.command.md"),
		filepath.Join(managed, "codex-skills"),
	} {
		if _, err := os.Lstat(retired); !os.IsNotExist(err) {
			t.Fatalf("retired /bb surface remains at %s: %v", retired, err)
		}
	}
	settings := readFixture(t, filepath.Join(config, "settings.json"))
	if strings.Contains(settings, "chat bb") || !strings.Contains(settings, "fixture-keep") ||
		!strings.Contains(settings, home+"/.local/bin/pfm internal clear-hide") {
		t.Fatalf("settings did not retire /bb and wire clear-hide:\n%s", settings)
	}

	var second bytes.Buffer
	report, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Now: now, Stdout: &second, Runner: &fakeRunner{},
	})
	if err != nil || report.Changed != 0 {
		t.Fatalf("second apply report=%#v err=%v\n%s", report, err, second.String())
	}
}

func TestApplyLeavesUnrelatedBBSymlinkAlone(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, ".claude", "commands", "bb.md")
	operatorSource := filepath.Join(home, "operator", "bb.md")
	writeFixture(t, operatorSource, "operator command\n")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(operatorSource, target); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(home, ".claude", "settings.json"), `{}`)

	if _, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Runner: &fakeRunner{},
	}); err != nil {
		t.Fatal(err)
	}
	assertLink(t, target, operatorSource)
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
	wantCalls := []string{
		"systemctl --user status",
		"systemctl --user daemon-reload",
		"systemctl --user enable --now pfm-name-sync.path pfm-name-sync.timer",
	}
	if schedulerIsLaunchd {
		// launchd has no manager probe to fail: the agent is bootstrapped into
		// the caller's own gui domain, and bootout first so an edited plist
		// replaces the loaded job instead of being rejected as a duplicate.
		wantCalls = []string{
			"launchctl bootout gui/",
			"launchctl bootstrap gui/",
		}
		for _, unwanted := range []string{"daemon-reload", "enable --now"} {
			if strings.Contains(joined, unwanted) {
				t.Fatalf("systemd was driven on a launchd host:\n%s", joined)
			}
		}
	}
	for _, wanted := range wantCalls {
		if !strings.Contains(joined, wanted) {
			t.Fatalf("systemctl calls missing %q:\n%s", wanted, joined)
		}
	}
}

// TestEarlyFleetCallsNamesWhatRunsBeforeTheLaunchersExist fixtures the class the
// installer cannot let pass silently: a fleet command CALLED above the source
// line. `cc` is also the POSIX C compiler and `cx` is a name anything may claim,
// so instead of "command not found" the shell runs a stranger — which is how a
// terminal profile calling `cc` from ~/.zshrc greeted every new terminal with
// "clang: error: no input files" while the fleet loaded fine a few lines later.
func TestEarlyFleetCallsNamesWhatRunsBeforeTheLaunchersExist(t *testing.T) {
	const sourced = `[[ -r "/opt/fixture/pfm/install/shim/pfm.zsh" ]] && source "/opt/fixture/pfm/install/shim/pfm.zsh"`

	cases := []struct {
		name    string
		content string
		want    []string
	}{{
		name:    "the canonical bug: a profile hook above the source line",
		content: "export EDITOR=vim\n[[ -n \"$VSCODE_AUTO_CC\" ]] && cc\n" + sourced + "\n",
		want:    []string{"line 2: [[ -n \"$VSCODE_AUTO_CC\" ]] && cc"},
	}, {
		name:    "the same call below the source line is fine",
		content: sourced + "\n[[ -n \"$VSCODE_AUTO_CC\" ]] && cc\n",
		want:    nil,
	}, {
		name:    "no source line yet — the whole file is above the bottom",
		content: "cc-ls\n",
		want:    []string{"line 1: cc-ls"},
	}, {
		name:    "a commented-out call is not a call",
		content: "# cc-ls here would break\ncc  # but this one is real\n" + sourced + "\n",
		want:    []string{"line 2: cc  # but this one is real"},
	}, {
		name:    "definitions and paths are not calls",
		content: "alias cc='echo no'\nexport CCDIR=$HOME/.cc/2\nPATH=$PATH:/opt/cc\n" + sourced + "\n",
		want:    nil,
	}, {
		name: "every launcher, after every separator that starts a command",
		content: "cc1\nfoo; cc2\nfoo && cx\nfoo || cc-ls\n(cc-open)\nfoo | cc-revive\n" +
			"cc-swap 1\nvsct-revive\n" + sourced + "\n",
		want: []string{
			"line 1: cc1", "line 2: foo; cc2", "line 3: foo && cx", "line 4: foo || cc-ls",
			"line 5: (cc-open)", "line 6: foo | cc-revive", "line 7: cc-swap 1",
			"line 8: vsct-revive",
		},
	}, {
		name:    "an empty rc file reports nothing",
		content: "",
		want:    nil,
	}}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := earlyFleetCalls(testCase.content)
			if len(got) != len(testCase.want) {
				t.Fatalf("earlyFleetCalls() = %q, want %q", got, testCase.want)
			}
			for index, wanted := range testCase.want {
				if got[index] != wanted {
					t.Fatalf("earlyFleetCalls()[%d] = %q, want %q", index, got[index], wanted)
				}
			}
		})
	}
}

// The rewriter and the early-call scan must never disagree about where the
// source line is: one decides where the launchers start existing, the other
// reports what runs before they do.
func TestEarlyCallScanStopsWhereTheRewriterFindsTheSourceLine(t *testing.T) {
	home := t.TempDir()
	zshrc := filepath.Join(home, ".zshrc")
	writeFixture(t, zshrc, "cc\nsource /old/cc-fleet.zsh\ncc-ls\n")

	var output bytes.Buffer
	if _, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Stdout: &output, Runner: &fakeRunner{},
	}); err != nil {
		t.Fatalf("apply: %v\n%s", err, output.String())
	}
	report := output.String()
	// Line 1 runs before the launchers exist; line 3 does not.
	if !strings.Contains(report, "a fleet command runs ABOVE the source line") ||
		!strings.Contains(report, "line 1: cc\n") {
		t.Fatalf("installer did not report the early call:\n%s", report)
	}
	if strings.Contains(report, "line 3: cc-ls") {
		t.Fatalf("installer reported a call below the source line:\n%s", report)
	}
	// It reports; it never rewrites a line of the operator's shell that is not ours.
	if zshrcContent := readFixture(t, zshrc); !strings.HasPrefix(zshrcContent, "cc\n") {
		t.Fatalf("installer rewrote the operator's own line:\n%s", zshrcContent)
	}
}

// outputRunner is a fakeRunner that can also answer a probe, which is what the
// launch-agent gate needs: launchctl reports a job's state in its output and
// exits zero either way.
type outputRunner struct {
	fakeRunner
	printOutput string
	printErr    error
}

func (runner *outputRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	runner.calls = append(runner.calls, name+" "+strings.Join(args, " "))
	if runner.printErr != nil {
		return nil, runner.printErr
	}
	return []byte(runner.printOutput), nil
}

// TestLaunchAgentGateRefusesOnlyMidExecution pins the macOS half of the rc 97
// refusal. It is deliberately NOT the dead-bus gate: launchd is always live for
// a logged-in user, so the only window worth refusing is an apply that would
// rewrite the agent and its binary while that agent is running.
func TestLaunchAgentGateRefusesOnlyMidExecution(t *testing.T) {
	if !schedulerIsLaunchd {
		t.Skip("launch-agent gate is macOS-only")
	}
	cases := []struct {
		name       string
		output     string
		outputErr  error
		wantRefuse bool
	}{
		{name: "mid-execution refuses", output: "\tstate = running\n", wantRefuse: true},
		// "not running" CONTAINS "running": a substring match here would refuse
		// every install on a perfectly idle agent.
		{name: "idle proceeds", output: "\tstate = not running\n"},
		{name: "unknown label proceeds", outputErr: errors.New("could not find service")},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runner := &outputRunner{printOutput: testCase.output, printErr: testCase.outputErr}
			_, err := Run(context.Background(), Options{
				Mode: ModeDryRun, Home: t.TempDir(), Stdout: io.Discard, Runner: runner,
			})
			if testCase.wantRefuse {
				if !errors.Is(err, ErrLaunchAgentRunning) {
					t.Fatalf("Run() error = %v, want ErrLaunchAgentRunning", err)
				}
				return
			}
			if errors.Is(err, ErrLaunchAgentRunning) {
				t.Fatalf("Run() refused an agent that was not mid-execution: %v", err)
			}
		})
	}
}

// A runner that cannot be probed must not be read as "safe": the installer says
// the gate did not run rather than implying it passed.
func TestLaunchAgentGateAnnouncesWhenItCannotProbe(t *testing.T) {
	if !schedulerIsLaunchd {
		t.Skip("launch-agent gate is macOS-only")
	}
	var output bytes.Buffer
	if _, err := Run(context.Background(), Options{
		Mode: ModeDryRun, Home: t.TempDir(), Stdout: &output, Runner: &fakeRunner{},
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "launch-agent gate NOT probed") {
		t.Fatalf("an unprobed gate was silent:\n%s", output.String())
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
