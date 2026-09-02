package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/kill"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/store"
)

// reload is a public chat operation; keep the contract pinned at the CLI
// boundary. `swap` was the pre-port spelling and is gone — dispatch must say
// so rather than quietly accepting a name nothing documents.
func TestChatReloadAcceptsCacheOnlyRequest(t *testing.T) {
	jailTest(t)
	var stdout, stderr bytes.Buffer
	code := runChat(
		[]string{"reload", "--1h", "on"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code == 2 && strings.Contains(stderr.String(), `unknown command "reload"`) {
		t.Fatalf("reload dispatch is still missing: rc=%d stderr=%q", code, stderr.String())
	}
}

func TestChatReloadHelpIsPublicAndSuccessful(t *testing.T) {
	jailTest(t)
	var stdout, stderr bytes.Buffer
	code := runChat(
		[]string{"reload", "--help"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != 0 || !strings.Contains(stdout.String(), "usage: pfm chat reload") {
		t.Fatalf("reload help rc=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

// The retired `swap` alias must be refused by name. A dispatch that still
// answered it would keep a second public spelling alive that no help text,
// card, or doc mentions.
func TestChatSwapAliasIsRetired(t *testing.T) {
	jailTest(t)
	var stdout, stderr bytes.Buffer
	code := runChat(
		[]string{"swap", "--help"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != 2 || !strings.Contains(stderr.String(), `unknown command "swap"`) {
		t.Fatalf("retired swap alias still dispatches: rc=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestChatReloadRefusesAnOpenSelectorOnAProbeSocket(t *testing.T) {
	jailTest(t)
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	socket := probeReloadSocket(t, "selector")
	server := exec.Command(
		"tmux", "-S", socket, "-f", "/dev/null", "new-session", "-d", "-s", "probe",
		"printf '❯ 1. choose\\n'; sleep 120",
	)
	server.Env = append(server.Environ(), "TMUX=")
	if output, err := server.CombinedOutput(); err != nil {
		t.Fatalf("start probe socket: %v: %s", err, output)
	}
	cleanupProbeReloadSocket(t, socket)
	paneOutput, err := exec.Command("tmux", "-S", socket, "list-panes", "-F", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("read probe pane: %v", err)
	}
	resolved, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(resolved.SIDDir, 0o700); err != nil {
		t.Fatal(err)
	}
	crumb := filepath.Join(
		resolved.SIDDir,
		filepath.Base(socket)+"."+strings.TrimSpace(string(paneOutput)),
	)
	if err := os.WriteFile(
		crumb,
		[]byte(filepath.Join(t.TempDir(), "22222222-2222-4222-8222-222222222222.jsonl")+"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runChatReloadWorker([]string{"--sock", socket, "--1h", "on"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "open selector menu") {
		t.Fatalf("reload selector gate rc=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if output, err := exec.Command("tmux", "-S", socket, "list-panes", "-F", "#{pane_current_command}").Output(); err != nil || strings.TrimSpace(string(output)) == "" {
		t.Fatalf("selector gate lost the pane: err=%v output=%q", err, output)
	}
}

func TestChatReloadSchedulesADetachedWorker(t *testing.T) {
	root := jailTest(t)
	configPath := writeConfigFixture(t, root, `{
  "version": 1,
  "accounts": [
    {"id": 1, "configDir": "`+filepath.Join(root, "account-1")+`"},
    {"id": 2, "configDir": "`+filepath.Join(root, "account-2")+`"}
  ]
}`)
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	socket := probeReloadSocket(t, "schedule")
	server := exec.Command(
		"tmux", "-S", socket, "-f", "/dev/null", "new-session", "-d", "-s", "probe",
		"sleep 120",
	)
	server.Env = append(server.Environ(), "TMUX=")
	if output, err := server.CombinedOutput(); err != nil {
		t.Fatalf("start probe socket: %v: %s", err, output)
	}
	cleanupProbeReloadSocket(t, socket)
	old := startReloadWorker
	t.Cleanup(func() { startReloadWorker = old })
	var workerArgs []string
	detached := false
	startReloadWorker = func(command *exec.Cmd) error {
		workerArgs = append([]string(nil), command.Args...)
		detached = command.SysProcAttr != nil && command.SysProcAttr.Setsid &&
			command.Stdin != os.Stdin && command.Stdout == command.Stderr
		return nil
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--config", configPath, "chat", "reload", "2", "--sock", socket, "--1h", "on"}, &stdout, &stderr); code != 0 {
		t.Fatalf("schedule rc=%d stderr=%q", code, stderr.String())
	}
	joined := strings.Join(workerArgs, "\x00")
	if !strings.Contains(joined, "\x00--config\x00") ||
		!strings.Contains(joined, "\x00internal\x00reload-run\x002\x00") {
		t.Fatalf("worker argv = %q", workerArgs)
	}
	if !strings.Contains(stdout.String(), "reload scheduled") {
		t.Fatalf("schedule receipt = %q", stdout.String())
	}
	if !detached {
		t.Fatal("reload worker retained the caller's process group or stdio pipes")
	}
}

// reloadPromptFixture is a raw-tty pane the worker can actually /exit and
// respawn: it echoes every typed byte itself (like injectCLIUI in
// inject_cli_jail_test.go) so capture-pane sees a live "❯ …" composer line,
// and it exits 0 the moment a submitted line reads exactly "/exit" — the
// literal string reload.Run types and submits before ever touching Respawn.
const reloadPromptFixture = `import os, sys, tty
tty.setraw(0)
buf = bytearray()
sys.stdout.write("❯ ")
sys.stdout.flush()
while True:
    ch = os.read(0, 1)
    if not ch:
        break
    if ch == b"\x13":
        continue
    if ch in (b"\r", b"\n"):
        line = bytes(buf).decode("utf-8", "replace")
        buf.clear()
        if line == "/exit":
            sys.exit(0)
        sys.stdout.write("\r\n❯ ")
        sys.stdout.flush()
        continue
    buf.extend(ch)
    sys.stdout.buffer.write(ch)
    sys.stdout.flush()
`

// TestChatReloadWorkerFreshDropsSessionButKeepsTranscriptCWD is the
// regression for T1: `--fresh` must blank the resumed session id (so the
// respawned Claude never carries `--resume`) while the transcript-derived
// CWD still reaches the respawned pane untouched. It drives the worker
// through a REAL tmux pane end to end — cmd/pfm's worker owns no Tmux
// interface seam of its own (unlike internal/reload's fakeReloadTmux), so a
// live probe socket plus a substitute "claude" binary that records its own
// argv and working directory is this package's actual stand-in for a fake.
func TestChatReloadWorkerFreshDropsSessionButKeepsTranscriptCWD(t *testing.T) {
	root := jailTest(t)
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}

	captured := filepath.Join(t.TempDir(), "captured.txt")
	fixtureClaude := filepath.Join(t.TempDir(), "claude-fixture.sh")
	script := "#!/bin/sh\n{\n  pwd\n  for a in \"$@\"; do printf 'ARG:%s\\n' \"$a\"; done\n} >> '" + captured + "'\nexit 0\n"
	if err := os.WriteFile(fixtureClaude, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	targetCWD := t.TempDir()

	configPath := writeConfigFixture(t, root, `{
  "version": 1,
  "accounts": [
    {"id": 1, "configDir": "`+filepath.Join(root, "account-1")+`", "claude": {"binary": "`+fixtureClaude+`"}}
  ]
}`)

	promptScript := filepath.Join(t.TempDir(), "prompt.py")
	if err := os.WriteFile(promptScript, []byte(reloadPromptFixture), 0o700); err != nil {
		t.Fatal(err)
	}
	socket := probeReloadSocket(t, "fresh")
	server := exec.Command(
		"tmux", "-S", socket, "-f", "/dev/null", "new-session", "-d", "-s", "probe",
		"python3", promptScript,
	)
	server.Env = append(server.Environ(), "TMUX=")
	if output, err := server.CombinedOutput(); err != nil {
		t.Fatalf("start probe socket: %v: %s", err, output)
	}
	cleanupProbeReloadSocket(t, socket)

	paneOutput, err := exec.Command("tmux", "-S", socket, "list-panes", "-F", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("read probe pane: %v", err)
	}
	pane := strings.TrimSpace(string(paneOutput))

	resolved, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(resolved.SIDDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// The crumb's own filename supplies the session id (SessionFromCrumb
	// strips the extension off the transcript basename); the transcript
	// FILE supplies the cwd the worker must keep even under --fresh.
	transcript := filepath.Join(t.TempDir(), "44444444-4444-4444-8444-444444444444.jsonl")
	if err := os.WriteFile(transcript, []byte(`{"cwd":"`+targetCWD+`"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	crumb := filepath.Join(resolved.SIDDir, filepath.Base(socket)+"."+pane)
	if err := os.WriteFile(crumb, []byte(transcript+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PFM_RELOAD_DELAY_MS", "0")
	t.Setenv("PFM_RELOAD_POLL_MS", "20")
	t.Setenv("PFM_RELOAD_EXIT_TRIES", "50")

	runtime, err := loadCommandRuntime(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runChatReloadWorkerWithRuntime(
		[]string{"--sock", socket, "--fresh", "--account", "1"}, &stdout, &stderr, runtime,
	)
	if code != 0 {
		t.Fatalf("fresh reload rc=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "rebooted FRESH as requested") {
		t.Fatalf("stdout=%q missing the fresh-reboot receipt", stdout.String())
	}

	// Respawn returns as soon as the tmux server has forked the replacement
	// process; the fixture claude finishing its own write is a separate,
	// microseconds-scale race — poll rather than assume the write already
	// landed by the time the worker call above returned.
	var content string
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, readErr := os.ReadFile(captured)
		if readErr == nil && len(data) > 0 {
			content = string(data)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("fixture claude never ran within the timeout: err=%v stdout=%q stderr=%q", readErr, stdout.String(), stderr.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if strings.Contains(content, "--resume") {
		t.Fatalf("fresh reload still resumed a session:\n%s", content)
	}
	if !strings.HasPrefix(content, targetCWD+"\n") {
		t.Fatalf("fresh reload lost the transcript's cwd: got %q, want it to start with %q", content, targetCWD+"\n")
	}
}

func probeReloadSocket(t *testing.T, suffix string) string {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "tmux-"+strconv.Itoa(os.Getuid()))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "cc-probe-pfm-reload-"+strconv.Itoa(os.Getpid())+"-"+suffix)
}

func cleanupProbeReloadSocket(t *testing.T, socket string) {
	t.Helper()
	t.Cleanup(func() {
		command := exec.Command("tmux", "-S", socket, "kill-server")
		command.Env = append(command.Environ(), "TMUX=")
		if output, err := command.CombinedOutput(); err != nil {
			probe := exec.Command("tmux", "-S", socket, "list-panes")
			probe.Env = append(probe.Environ(), "TMUX=")
			if probe.Run() == nil {
				t.Errorf("stop probe server %s: %v: %s", socket, err, output)
			}
		}
		if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove stale probe socket %s: %v", socket, err)
		}
	})
}

func TestReloadTargetIdentityNeverFallsBackToTheCallerSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "11111111-1111-4111-8111-111111111111")
	resolved := paths.Values{
		SIDDir: filepath.Join(root, "sid"),
		Roots:  map[pfmengine.ID][]string{pfmengine.Claude: {filepath.Join(root, "claude")}},
	}
	_, _, err := resolveReloadSession(
		resolved,
		pfmconfig.Defaults(resolved.Home, resolved.Roots[pfmengine.Claude], firstRoot(resolved.Roots[pfmengine.Codex])),
		"/tmp/tmux-1000/probe-pfm-reload-target",
		"%7",
		false,
	)
	if err == nil || !strings.Contains(err.Error(), "couldn't identify") {
		t.Fatalf("target without its own breadcrumb borrowed caller identity: %v", err)
	}
}

func TestExplicitCodexReloadUsesPaneBindingWithoutBreadcrumb(t *testing.T) {
	jailTest(t)
	resolved, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	machine := pfmconfig.Defaults(
		resolved.Home,
		resolved.Roots[pfmengine.Claude],
		firstRoot(resolved.Roots[pfmengine.Codex]),
	)
	machine.CodexAccounts = []pfmconfig.CodexAccount{{
		ID:   1,
		Home: firstRoot(resolved.Roots[pfmengine.Codex]),
	}}
	const (
		socket = "cx-probe-reload-bound"
		pane   = "%7"
		wantID = "22222222-2222-4222-8222-222222222222"
	)
	rollout := filepath.Join(
		firstRoot(resolved.Roots[pfmengine.Codex]),
		"sessions",
		"rollout-2026-08-24T00-00-00-"+wantID+".jsonl",
	)
	if err := os.MkdirAll(filepath.Dir(rollout), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollout, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	manager, err := kill.New(database, killDependencies(commandRuntime{
		Config: machine,
		Paths:  resolved,
	}))
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if _, _, err := manager.AdvanceCodexPane(context.Background(), socket, pane, wantID); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	gotID, gotTranscript, err := resolveReloadSession(
		resolved,
		machine,
		filepath.Join(resolved.TmuxDir, socket),
		pane,
		false,
	)
	if err != nil || gotID != wantID || gotTranscript != rollout {
		t.Fatalf(
			"resolve bound Codex reload = (%q, %q, %v), want (%q, %q, nil)",
			gotID,
			gotTranscript,
			err,
			wantID,
			rollout,
		)
	}
}
