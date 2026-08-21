package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/paths"
)

// The swap verb is a public chat operation. Before its port, dispatch treated
// it as an unknown command; keep this contract pinned at the CLI boundary.
func TestChatSwapAcceptsCacheOnlyRequest(t *testing.T) {
	jailTest(t)
	var stdout, stderr bytes.Buffer
	code := runChat(
		[]string{"swap", "--1h", "on"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code == 2 && strings.Contains(stderr.String(), `unknown command "swap"`) {
		t.Fatalf("swap dispatch is still missing: rc=%d stderr=%q", code, stderr.String())
	}
}

func TestChatSwapHelpIsPublicAndSuccessful(t *testing.T) {
	jailTest(t)
	var stdout, stderr bytes.Buffer
	code := runChat(
		[]string{"swap", "--help"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != 0 || !strings.Contains(stdout.String(), "usage: pfm chat reload") {
		t.Fatalf("swap help rc=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestChatSwapRefusesAnOpenSelectorOnAProbeSocket(t *testing.T) {
	jailTest(t)
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	socket := probeSwapSocket(t, "selector")
	server := exec.Command(
		"tmux", "-S", socket, "-f", "/dev/null", "new-session", "-d", "-s", "probe",
		"printf '❯ 1. choose\\n'; sleep 120",
	)
	server.Env = append(server.Environ(), "TMUX=")
	if output, err := server.CombinedOutput(); err != nil {
		t.Fatalf("start probe socket: %v: %s", err, output)
	}
	cleanupProbeSwapSocket(t, socket)
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
		t.Fatalf("swap selector gate rc=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
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
	socket := probeSwapSocket(t, "schedule")
	server := exec.Command(
		"tmux", "-S", socket, "-f", "/dev/null", "new-session", "-d", "-s", "probe",
		"sleep 120",
	)
	server.Env = append(server.Environ(), "TMUX=")
	if output, err := server.CombinedOutput(); err != nil {
		t.Fatalf("start probe socket: %v: %s", err, output)
	}
	cleanupProbeSwapSocket(t, socket)
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
	if code := run([]string{"--config", configPath, "chat", "swap", "2", "--sock", socket, "--1h", "on"}, &stdout, &stderr); code != 0 {
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
		t.Fatal("swap worker retained the caller's process group or stdio pipes")
	}
}

func probeSwapSocket(t *testing.T, suffix string) string {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "tmux-"+strconv.Itoa(os.Getuid()))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "probe-pfm-swap-"+strconv.Itoa(os.Getpid())+"-"+suffix)
}

func cleanupProbeSwapSocket(t *testing.T, socket string) {
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

func TestSwapTargetIdentityNeverFallsBackToTheCallerSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "11111111-1111-4111-8111-111111111111")
	resolved := paths.Values{
		SIDDir:      filepath.Join(root, "sid"),
		ClaudeRoots: []string{filepath.Join(root, "claude")},
	}
	_, _, err := resolveReloadSession(
		resolved,
		pfmconfig.Defaults(resolved.Home, resolved.ClaudeRoots, resolved.CodexRoot),
		"/tmp/tmux-1000/probe-pfm-swap-target",
		"%7",
		false,
	)
	if err == nil || !strings.Contains(err.Error(), "couldn't identify") {
		t.Fatalf("target without its own breadcrumb borrowed caller identity: %v", err)
	}
}
