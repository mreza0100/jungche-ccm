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

	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/kill"
	"hostops/pfm/internal/store"
)

// startCodexStatusPane brings up a real tmux server on a scratch socket
// under tmuxTmpDir — gather.CommandTmux's own addressing (-L socket plus
// TMUX_TMPDIR, landing the socket at tmuxTmpDir/tmux-<uid>/socket) — with
// one pane that paints statusLine and holds. A codex pane's identity lives
// on its own screen and nowhere else, so reconcileCodexPanes must read a
// REAL capture, not a mock.
// statusLine is printf's OWN format string (raw, run through a shell), so it
// must escape a literal "%" as "%%" and a newline as "\\n", exactly the way
// TestClaudeWindowNameConvergesOnARealServer (internal/gather) does for the
// claude half of the same mechanism.
func startCodexStatusPane(t *testing.T, tmuxTmpDir, socket, statusLine string) {
	t.Helper()
	if err := os.MkdirAll(tmuxTmpDir, 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(
		"tmux", "-f", "/dev/null", "-L", socket,
		"new-session", "-d", "-s", socket, "-n", "codex",
		"printf '"+statusLine+"'; sleep 120",
	)
	command.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+tmuxTmpDir)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start jailed codex pane %q: %v: %s", socket, err, output)
	}
	t.Cleanup(func() {
		kill := exec.Command("tmux", "-L", socket, "kill-server")
		kill.Env = append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+tmuxTmpDir)
		_ = kill.Run()
	})
}

func codexJailRollout(
	t *testing.T,
	database *store.Store,
	root, id string,
	promptCount int64,
) string {
	t.Helper()
	rolloutPath := filepath.Join(
		root, "codex", "sessions", "2030", "01", "02",
		"rollout-2030-01-02T03-04-05-"+id+".jsonl",
	)
	if err := os.MkdirAll(filepath.Dir(rolloutPath), 0o700); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		`{"type":"session_meta","payload":{"id":"` + id + `","thread_source":"user","cwd":"/work/example"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"first"}]}}`,
		"",
	}, "\n")
	if err := os.WriteFile(rolloutPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertRollout(context.Background(), store.Rollout{
		ID: id, Path: rolloutPath, CWD: "/work/example", UserThread: true,
		PromptCount: promptCount,
	}); err != nil {
		t.Fatal(err)
	}
	return rolloutPath
}

func codexPane(socket, paneID string) gather.Pane {
	return gather.Pane{
		Socket:         socket,
		SessionName:    socket,
		WindowID:       "@1",
		PaneID:         paneID,
		CurrentCommand: "codex",
	}
}

// The pipeline-level shape of the whole fix: a pane bound to T1 whose status
// line now shows a DIFFERENT bare thread id T2 — a clear happened — is
// killed with a prompt baseline, exactly like Claude's SessionEnd hook gets,
// and the binding advances to T2.
func TestReconcileCodexPanesKillsThePreviousBoundThreadAndAdvancesTheBinding(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxTmpDir := filepath.Join(root, "tmuxtmp")
	const socket = "cx-1800000001-1-1"
	const newID = "22222222-2222-4222-8222-222222222222"
	startCodexStatusPane(t, tmuxTmpDir, socket, "  "+newID+` · /work/example · Full Access\n`)

	resolved := jailPaths(t)
	resolved.TmuxDir = filepath.Join(tmuxTmpDir, "tmux-"+strconv.Itoa(os.Getuid()))

	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	const oldID = "11111111-1111-4111-8111-111111111111"
	codexJailRollout(t, database, root, oldID, 1)

	manager, err := kill.New(database, kill.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.AdvanceCodexPane(context.Background(), socket, "%0", oldID); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	reconcileCodexPanes(
		context.Background(),
		database,
		gather.Snapshot{Panes: []gather.Pane{codexPane(socket, "%0")}},
		commandRuntime{Paths: resolved},
		&stderr,
	)

	bound, found, err := manager.CodexPaneBinding(context.Background(), socket, "%0")
	if err != nil || !found || bound != newID {
		t.Fatalf("binding = (%q, %v, %v), want the new thread: stderr=%q", bound, found, err, stderr.String())
	}
	killed, found, err := database.Killed(context.Background(), oldID)
	if err != nil || !found || killed.Engine != pfmengine.Codex ||
		killed.BaselinePrompts == nil || *killed.BaselinePrompts != 1 {
		t.Fatalf("previous thread killed = %#v found=%v error=%v: stderr=%q", killed, found, err, stderr.String())
	}
}

// A capture that FAILED — the pane's socket carries no server at all — must
// never be read as "this pane runs nothing": nothing is killed, the
// existing binding stands, and the failure is named on stderr in words
// distinct from an ordinary unnamed thread.
func TestReconcileCodexPanesCaptureFailedKillsNothingAndNamesTheFailure(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxTmpDir := filepath.Join(root, "tmuxtmp")
	if err := os.MkdirAll(tmuxTmpDir, 0o700); err != nil {
		t.Fatal(err)
	}
	const socket = "cx-1800000002-1-1"

	resolved := jailPaths(t)
	resolved.TmuxDir = filepath.Join(tmuxTmpDir, "tmux-"+strconv.Itoa(os.Getuid()))

	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	const oldID = "33333333-3333-4333-8333-333333333333"
	codexJailRollout(t, database, root, oldID, 1)

	manager, err := kill.New(database, kill.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.AdvanceCodexPane(context.Background(), socket, "%0", oldID); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	// No server was ever started on this socket: capture-pane fails.
	reconcileCodexPanes(
		context.Background(),
		database,
		gather.Snapshot{Panes: []gather.Pane{codexPane(socket, "%0")}},
		commandRuntime{Paths: resolved},
		&stderr,
	)

	if !strings.Contains(stderr.String(), "capture failed") {
		t.Fatalf("stderr = %q, want a capture-failed message", stderr.String())
	}
	if _, found, err := database.Killed(context.Background(), oldID); err != nil || found {
		t.Fatalf("a failed capture killed the bound thread: found=%v error=%v", found, err)
	}
	bound, found, err := manager.CodexPaneBinding(context.Background(), socket, "%0")
	if err != nil || !found || bound != oldID {
		t.Fatalf("binding moved on a failed capture: bound=%q found=%v error=%v", bound, found, err)
	}
}

// The regression the retired process-slot design could not pass: two panes
// share a cwd, only one clears. Only that pane's previous thread is killed;
// the other pane's own binding, and thread, stay untouched.
func TestReconcileCodexPanesOnlyKillsTheClearingPaneInASharedCWD(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxTmpDir := filepath.Join(root, "tmuxtmp")
	const clearingSocket = "cx-1800000003-1-1"
	const steadySocket = "cx-1800000003-2-2"
	const clearedID = "44444444-4444-4444-8444-444444444444"
	startCodexStatusPane(t, tmuxTmpDir, clearingSocket, "  "+clearedID+` · /work/shared · Full Access\n`)
	startCodexStatusPane(t, tmuxTmpDir, steadySocket, `  STEADY_CHAT · /work/shared · Full Access\n`)

	resolved := jailPaths(t)
	resolved.TmuxDir = filepath.Join(tmuxTmpDir, "tmux-"+strconv.Itoa(os.Getuid()))

	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	const clearingOldID = "55555555-5555-4555-8555-555555555555"
	const steadyID = "66666666-6666-4666-8666-666666666666"
	codexJailRollout(t, database, root, clearingOldID, 1)
	codexJailRollout(t, database, root, steadyID, 1)
	if err := database.ReplaceCxNames(context.Background(), []store.CxName{
		{ID: steadyID, ThreadName: "STEADY_CHAT"},
	}); err != nil {
		t.Fatal(err)
	}

	manager, err := kill.New(database, kill.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.AdvanceCodexPane(context.Background(), clearingSocket, "%0", clearingOldID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.AdvanceCodexPane(context.Background(), steadySocket, "%0", steadyID); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	reconcileCodexPanes(
		context.Background(),
		database,
		gather.Snapshot{Panes: []gather.Pane{
			codexPane(clearingSocket, "%0"),
			codexPane(steadySocket, "%0"),
		}},
		commandRuntime{Paths: resolved},
		&stderr,
	)

	if _, found, err := database.Killed(context.Background(), clearingOldID); err != nil || !found {
		t.Fatalf("clearing pane's previous thread was not killed: found=%v error=%v stderr=%q", found, err, stderr.String())
	}
	if _, found, err := database.Killed(context.Background(), steadyID); err != nil || found {
		t.Fatalf("the steady pane's own thread was killed too: found=%v error=%v stderr=%q", found, err, stderr.String())
	}
	bound, found, err := manager.CodexPaneBinding(context.Background(), steadySocket, "%0")
	if err != nil || !found || bound != steadyID {
		t.Fatalf("the steady pane's own binding moved: bound=%q found=%v error=%v", bound, found, err)
	}
}
