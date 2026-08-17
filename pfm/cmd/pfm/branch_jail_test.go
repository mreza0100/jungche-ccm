package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/shared"
	"hostops/pfm/internal/store"
)

// TestChatBranchCreatesADetachedSeatWithoutTouchingTheCaller is the hard
// layout law for /chat:branch. Both the caller and the fork are real scratch
// tmux servers; no live fleet socket is visible inside this jail.
func TestChatBranchCreatesADetachedSeatWithoutTouchingTheCaller(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root, err := os.MkdirTemp("", "pbr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	tmuxDir := filepath.Join(root, "tmux-"+strconv.Itoa(os.Getuid()))
	claudeRoot := filepath.Join(root, "home", ".claude", "projects")
	binDir := filepath.Join(root, "bin")
	for _, directory := range []string{tmuxDir, claudeRoot, binDir, filepath.Join(root, "sid")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	const parentID = "11111111-1111-4111-8111-111111111111"
	const callerSocket = "probe-branch-caller"
	const branchSocket = "probe-branch-detached"
	argsPath := filepath.Join(root, "claude.args")
	fakeClaude := filepath.Join(binDir, "claude")
	if err := os.WriteFile(fakeClaude, []byte(
		"#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$PFM_TEST_BRANCH_ARGS\"\nexec sleep 120\n",
	), 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("PFM_HOME", filepath.Join(root, "home"))
	t.Setenv("PFM_DB", filepath.Join(root, "fleet.db"))
	t.Setenv("PFM_SHARED_DB", filepath.Join(root, "shared.db"))
	t.Setenv("PFM_SID_DIR", filepath.Join(root, "sid"))
	t.Setenv("PFM_CLAUDE_ROOTS", claudeRoot)
	t.Setenv("PFM_CODEX_ROOT", filepath.Join(root, "codex"))
	t.Setenv("PFM_TMUX_DIR", tmuxDir)
	t.Setenv("PFM_TMUX_CONF", "/dev/null")
	t.Setenv("PFM_TEST_FRESH_SOCKET", branchSocket)
	t.Setenv("PFM_TEST_BRANCH_ARGS", argsPath)
	t.Setenv("CLAUDE_CODE_SESSION_ID", parentID)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, "home", ".claude"))
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	projectSlug := strings.NewReplacer("/", "-", ".", "-").Replace(cwd)
	parentPath := filepath.Join(claudeRoot, projectSlug, parentID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(parentPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parentPath, []byte(
		`{"type":"assistant","message":{"model":"claude-opus-5","usage":{}}}`+"\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}

	database, err := store.Open(store.WithWarningWriter(&bytes.Buffer{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertTranscript(context.Background(), store.Transcript{
		UUID: parentID, Path: parentPath,
		CWD: "/work/project", CustomTitle: "Parent seat",
	}); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	callerPath := filepath.Join(tmuxDir, callerSocket)
	start := exec.Command(
		"tmux", "-f", "/dev/null", "-S", callerPath,
		"new-session", "-d", "-s", callerSocket, "-n", "Parent seat", "sleep 120",
	)
	start.Env = append(os.Environ(), "TMUX=")
	if output, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start caller: %v: %s", err, output)
	}
	for _, socket := range []string{callerSocket, branchSocket} {
		socket := socket
		t.Cleanup(func() {
			kill := exec.Command("tmux", "-S", filepath.Join(tmuxDir, socket), "kill-server")
			kill.Env = append(os.Environ(), "TMUX=")
			_ = kill.Run()
		})
	}
	t.Setenv("TMUX", fmt.Sprintf("%s,%d,0", callerPath, os.Getpid()))
	t.Setenv("TMUX_PANE", "%0")

	before := branchCallerShape(t, callerPath)
	started := time.Now()
	var stdout, stderr bytes.Buffer
	if code := runChatBranch(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("chat branch rc=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("detached branch blocked for %s", elapsed)
	}
	resolved, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	state := shared.Open(context.Background(), resolved)
	seats, seatsErr := state.BranchSeats(context.Background())
	closeErr := state.Close()
	if seatsErr != nil || closeErr != nil {
		t.Fatalf("read branch marker: %v", errors.Join(seatsErr, closeErr))
	}
	if got := seats[branchSocket]; got.Parent != parentID {
		t.Fatalf("branch marker = %#v", got)
	}
	after := branchCallerShape(t, callerPath)
	if after != before {
		t.Fatalf("branch changed caller layout/focus\nbefore: %s\nafter:  %s", before, after)
	}

	branchPath := filepath.Join(tmuxDir, branchSocket)
	window := branchTmuxOutput(t, branchPath, "list-windows", "-F", "#{window_name}|#{window_active}|#{window_layout}")
	if !strings.HasPrefix(window, "Parent seat-branch|") {
		t.Fatalf("detached branch window = %q", window)
	}
	clients := branchTmuxOutput(t, branchPath, "list-clients", "-F", "#{client_name}")
	if strings.TrimSpace(clients) != "" {
		t.Fatalf("branch was attached at birth: %q", clients)
	}
	// The seat is spawned DETACHED, so its process races this assertion: tmux
	// returns as soon as the server exists, before the pane's shell has exec'd
	// the engine. A single read passes on a fast host and fails on a slow exec,
	// which is a flake, not a platform difference — wait for the write instead
	// of assuming it already happened.
	arguments, err := waitForFile(t, argsPath, 10*time.Second)
	if err != nil {
		t.Fatalf("read claude argv: %v", err)
	}
	joined := strings.Join(strings.Fields(string(arguments)), " ")
	for _, want := range []string{
		"--resume " + parentID, "--fork-session", "--model opus[1m]", "--name Parent seat-branch",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("claude argv %q lacks %q", joined, want)
		}
	}
	if !strings.Contains(stdout.String(), "detached") || !strings.Contains(stdout.String(), "pfm ls") {
		t.Fatalf("receipt does not explain create-now/open-later: %q", stdout.String())
	}
}

func branchCallerShape(t *testing.T, socketPath string) string {
	t.Helper()
	return branchTmuxOutput(
		t, socketPath, "list-panes", "-a", "-F",
		"#{session_name}|#{window_id}|#{window_name}|#{pane_id}|#{pane_active}|#{window_active}|#{window_layout}",
	)
}

func branchTmuxOutput(t *testing.T, socketPath string, arguments ...string) string {
	t.Helper()
	command := exec.Command("tmux", append([]string{"-S", socketPath}, arguments...)...)
	command.Env = append(os.Environ(), "TMUX=")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("tmux %s: %v", strings.Join(arguments, " "), err)
	}
	return strings.TrimSpace(string(output))
}

// waitForFile polls until path has content or the deadline passes, returning
// the last error so a timeout reports why the file was unreadable.
func waitForFile(t *testing.T, path string, within time.Duration) ([]byte, error) {
	t.Helper()
	deadline := time.Now().Add(within)
	var err error
	for {
		var content []byte
		content, err = os.ReadFile(path)
		if err == nil && len(content) > 0 {
			return content, nil
		}
		if time.Now().After(deadline) {
			if err == nil {
				err = errors.New("file stayed empty")
			}
			return nil, err
		}
		time.Sleep(20 * time.Millisecond)
	}
}
