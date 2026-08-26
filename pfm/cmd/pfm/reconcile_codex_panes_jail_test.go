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
		printWarn(&stderr),
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
		printWarn(&stderr),
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
		printWarn(&stderr),
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

// Duplicate display names are ordinary fleet state, not a broken tmux probe.
// When the pane is already bound to one of the matching threads, that binding
// is the only safe disambiguator: keep it steady without printing a warning
// when the picker releases the terminal.
func TestReconcileCodexPanesUsesExistingBindingForDuplicateName(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxTmpDir := filepath.Join(root, "tmuxtmp")
	const socket = "cx-1800000005-1-1"
	startCodexStatusPane(t, tmuxTmpDir, socket, `  FIX_HAND · /work/example · Full Access\n`)

	resolved := jailPaths(t)
	resolved.TmuxDir = filepath.Join(tmuxTmpDir, "tmux-"+strconv.Itoa(os.Getuid()))
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	const boundID = "99999999-9999-4999-8999-999999999999"
	const duplicateID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	codexJailRollout(t, database, root, boundID, 1)
	codexJailRollout(t, database, root, duplicateID, 1)
	if err := database.ReplaceCxNames(context.Background(), []store.CxName{
		{ID: boundID, ThreadName: "FIX_HAND"},
		{ID: duplicateID, ThreadName: "FIX_HAND"},
	}); err != nil {
		t.Fatal(err)
	}

	manager, err := kill.New(database, kill.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.AdvanceCodexPane(context.Background(), socket, "%0", boundID); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	reconcileCodexPanes(
		context.Background(),
		database,
		gather.Snapshot{Panes: []gather.Pane{codexPane(socket, "%0")}},
		commandRuntime{Paths: resolved},
		printWarn(&stderr),
	)

	if stderr.Len() != 0 {
		t.Fatalf("steady duplicate name printed a shutdown warning: %q", stderr.String())
	}
	got, found, err := manager.CodexPaneBinding(context.Background(), socket, "%0")
	if err != nil || !found || got != boundID {
		t.Fatalf("binding = (%q, %v, %v), want unchanged %q", got, found, err, boundID)
	}
	if _, found, err := database.Killed(context.Background(), boundID); err != nil || found {
		t.Fatalf("steady bound thread was killed: found=%v error=%v", found, err)
	}
}

func TestReconcileCodexPanesSkipsDuplicateNameWithoutUsableBindingQuietly(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	for _, test := range []struct {
		name      string
		boundID   string
		wantBound bool
	}{
		{name: "no incumbent binding", wantBound: false},
		{name: "incumbent is not one of the matches", boundID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", wantBound: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := jailTest(t)
			tmuxTmpDir := filepath.Join(root, "tmuxtmp")
			socket := "cx-1800000006-" + strings.ReplaceAll(test.name, " ", "-")
			startCodexStatusPane(t, tmuxTmpDir, socket, `  FIX_HAND · /work/example · Full Access\n`)

			resolved := jailPaths(t)
			resolved.TmuxDir = filepath.Join(tmuxTmpDir, "tmux-"+strconv.Itoa(os.Getuid()))
			database, err := store.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()

			const firstID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
			const secondID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
			codexJailRollout(t, database, root, firstID, 1)
			codexJailRollout(t, database, root, secondID, 1)
			if err := database.ReplaceCxNames(context.Background(), []store.CxName{
				{ID: firstID, ThreadName: "FIX_HAND"},
				{ID: secondID, ThreadName: "FIX_HAND"},
			}); err != nil {
				t.Fatal(err)
			}

			manager, err := kill.New(database, kill.Dependencies{})
			if err != nil {
				t.Fatal(err)
			}
			if test.boundID != "" {
				codexJailRollout(t, database, root, test.boundID, 1)
				if _, _, err := manager.AdvanceCodexPane(context.Background(), socket, "%0", test.boundID); err != nil {
					t.Fatal(err)
				}
			}

			var stderr bytes.Buffer
			reconcileCodexPanes(
				context.Background(),
				database,
				gather.Snapshot{Panes: []gather.Pane{codexPane(socket, "%0")}},
				commandRuntime{Paths: resolved},
				printWarn(&stderr),
			)
			if stderr.Len() != 0 {
				t.Fatalf("valid duplicate-name state printed a shutdown warning: %q", stderr.String())
			}
			bound, found, err := manager.CodexPaneBinding(context.Background(), socket, "%0")
			if err != nil {
				t.Fatal(err)
			}
			if found != test.wantBound || (found && bound != test.boundID) {
				t.Fatalf("binding = (%q, %v), want incumbent (%q, %v)", bound, found, test.boundID, test.wantBound)
			}
			for _, id := range []string{firstID, secondID} {
				if _, killed, err := database.Killed(context.Background(), id); err != nil || killed {
					t.Fatalf("duplicate-name ambiguity killed %s: killed=%v error=%v", id, killed, err)
				}
			}
		})
	}
}

func TestReconcileCodexPanesKeepsBoundThreadSilentWhenNameIsEmpty(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxTmpDir := filepath.Join(root, "tmuxtmp")
	const socket = "cx-1800000007-1-1"
	startCodexStatusPane(t, tmuxTmpDir, socket, `  · /work/example · Full Access\n`)

	resolved := jailPaths(t)
	resolved.TmuxDir = filepath.Join(tmuxTmpDir, "tmux-"+strconv.Itoa(os.Getuid()))
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	const boundID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	codexJailRollout(t, database, root, boundID, 3)
	manager, err := kill.New(database, kill.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.AdvanceCodexPane(context.Background(), socket, "%0", boundID); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	reconcileCodexPanes(
		context.Background(),
		database,
		gather.Snapshot{Panes: []gather.Pane{codexPane(socket, "%0")}},
		commandRuntime{Paths: resolved},
		printWarn(&stderr),
	)
	if stderr.Len() != 0 {
		t.Fatalf("empty Codex name printed a shutdown warning: %q", stderr.String())
	}
	got, found, err := manager.CodexPaneBinding(context.Background(), socket, "%0")
	if err != nil || !found || got != boundID {
		t.Fatalf("empty-name binding = (%q, %v, %v), want unchanged %q", got, found, err, boundID)
	}
	if _, killed, err := database.Killed(context.Background(), boundID); err != nil || killed {
		t.Fatalf("empty Codex name killed incumbent: killed=%v error=%v", killed, err)
	}
}

// codexJailChildRollout is codexJailRollout for a thread that CONTINUES
// another — a resume or a fork, which Codex lands as a child rollout in the
// same pane. Its lineage root is the parent's, and that is exactly what tells
// a resume apart from a clear.
func codexJailChildRollout(
	t *testing.T,
	database *store.Store,
	root, id, parent string,
	promptCount int64,
) {
	t.Helper()
	rolloutPath := filepath.Join(
		root, "codex", "sessions", "2030", "01", "02",
		"rollout-2030-01-02T03-04-06-"+id+".jsonl",
	)
	if err := os.MkdirAll(filepath.Dir(rolloutPath), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"session_meta","payload":{"id":"` + id +
		`","thread_source":"user","cwd":"/work/example"}}` + "\n"
	if err := os.WriteFile(rolloutPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertRollout(context.Background(), store.Rollout{
		ID: id, Path: rolloutPath, CWD: "/work/example", UserThread: true,
		PromptCount: promptCount, ParentThread: parent,
	}); err != nil {
		t.Fatal(err)
	}
}

// THE regression, at the pipeline level, over a real tmux server.
//
// A pane has already cleared: pfm retired the old thread, advanced the binding
// to the new one, and re-applied the chat's name to it. cx_names has not
// caught up — Codex's index is refreshed on its own schedule — so the display
// name still resolves to exactly ONE thread, the dead one.
//
// The old pass took that unique match as the pane's identity, moved the
// binding BACKWARD onto the corpse, and clear-killed the live thread that had
// replaced it. Both panes named ENGINE_BUILDER on a real host ended that way:
// bound to one already-killed thread, with `pfm chat resolve` answering the
// corpse and every inject aimed at a thread nobody was running.
//
// A name never moves a binding. Nothing here may change.
func TestReconcileCodexPanesNameNeverMovesTheBindingBackwards(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxTmpDir := filepath.Join(root, "tmuxtmp")
	const socket = "cx-1800000010-1-1"
	// The pane shows its NAME again, because the post-clear rename landed.
	startCodexStatusPane(t, tmuxTmpDir, socket, `  W5_TESTER · /work/example · Full Access\n`)

	resolved := jailPaths(t)
	resolved.TmuxDir = filepath.Join(tmuxTmpDir, "tmux-"+strconv.Itoa(os.Getuid()))
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	const clearedID = "11111111-1111-4111-8111-111111111111"
	const liveID = "22222222-2222-4222-8222-222222222222"
	codexJailRollout(t, database, root, clearedID, 4)
	codexJailRollout(t, database, root, liveID, 1)
	// Only the DEAD thread carries the name: the index lag this test is about.
	if err := database.ReplaceCxNames(context.Background(), []store.CxName{
		{ID: clearedID, ThreadName: "W5_TESTER"},
	}); err != nil {
		t.Fatal(err)
	}

	manager, err := kill.New(database, kill.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.AdvanceCodexPane(context.Background(), socket, "%0", liveID); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	reconcileCodexPanes(
		context.Background(),
		database,
		gather.Snapshot{Panes: []gather.Pane{codexPane(socket, "%0")}},
		commandRuntime{Paths: resolved},
		printWarn(&stderr),
	)

	bound, found, err := manager.CodexPaneBinding(context.Background(), socket, "%0")
	if err != nil || !found || bound != liveID {
		t.Fatalf(
			"binding = (%q, %v, %v), want it to STAY on the live post-clear thread %q: stderr=%q",
			bound, found, err, liveID, stderr.String(),
		)
	}
	if _, killed, err := database.Killed(context.Background(), liveID); err != nil || killed {
		t.Fatalf(
			"a lagging display name clear-killed the LIVE thread %q: killed=%v error=%v",
			liveID, killed, err,
		)
	}
}

// Two live panes paint the same display name and cx_names knows exactly one
// thread by it. A name may seed at most one pane; the second keeps nothing,
// and the collision is reported rather than left to be discovered months later
// in the meta table.
func TestReconcileCodexPanesNeverBindsTwoPanesToOneThread(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxTmpDir := filepath.Join(root, "tmuxtmp")
	const first = "cx-1800000011-1-1"
	const second = "cx-1800000011-2-2"
	for _, socket := range []string{first, second} {
		startCodexStatusPane(t, tmuxTmpDir, socket, `  ENGINE_BUILDER · /work/example · Full Access\n`)
	}

	resolved := jailPaths(t)
	resolved.TmuxDir = filepath.Join(tmuxTmpDir, "tmux-"+strconv.Itoa(os.Getuid()))
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	const sharedID = "33333333-3333-4333-8333-333333333333"
	codexJailRollout(t, database, root, sharedID, 2)
	if err := database.ReplaceCxNames(context.Background(), []store.CxName{
		{ID: sharedID, ThreadName: "ENGINE_BUILDER"},
	}); err != nil {
		t.Fatal(err)
	}

	manager, err := kill.New(database, kill.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	reconcileCodexPanes(
		context.Background(),
		database,
		gather.Snapshot{Panes: []gather.Pane{codexPane(first, "%0"), codexPane(second, "%0")}},
		commandRuntime{Paths: resolved},
		printWarn(&stderr),
	)

	bindings := 0
	for _, socket := range []string{first, second} {
		bound, found, err := manager.CodexPaneBinding(context.Background(), socket, "%0")
		if err != nil {
			t.Fatal(err)
		}
		if found && bound == sharedID {
			bindings++
		}
	}
	if bindings != 1 {
		t.Fatalf("%d panes bound to one thread, want exactly 1: stderr=%q", bindings, stderr.String())
	}
	if !strings.Contains(stderr.String(), codexPaneNameTaken) {
		t.Fatalf("the refused pane was silent: stderr=%q", stderr.String())
	}
}

// A resume or fork lands a CHILD rollout in the same pane, so the pane's own
// status line shows a bare id it was not bound to — the same shape a clear
// produces. Retiring the previous thread there kills the chat's own lineage
// root and the chat vanishes from the fleet. The lineage is what tells them
// apart, so the binding advances and nothing is retired.
func TestReconcileCodexPanesTreatsASameLineageResumeAsNoClear(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxTmpDir := filepath.Join(root, "tmuxtmp")
	const socket = "cx-1800000012-1-1"
	const parentID = "44444444-4444-4444-8444-444444444444"
	const childID = "55555555-5555-4555-8555-555555555555"
	startCodexStatusPane(t, tmuxTmpDir, socket, "  "+childID+` · /work/example · Full Access\n`)

	resolved := jailPaths(t)
	resolved.TmuxDir = filepath.Join(tmuxTmpDir, "tmux-"+strconv.Itoa(os.Getuid()))
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	codexJailRollout(t, database, root, parentID, 6)
	codexJailChildRollout(t, database, root, childID, parentID, 7)

	manager, err := kill.New(database, kill.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.AdvanceCodexPane(context.Background(), socket, "%0", parentID); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	reconcileCodexPanes(
		context.Background(),
		database,
		gather.Snapshot{Panes: []gather.Pane{codexPane(socket, "%0")}},
		commandRuntime{Paths: resolved},
		printWarn(&stderr),
	)

	bound, found, err := manager.CodexPaneBinding(context.Background(), socket, "%0")
	if err != nil || !found || bound != childID {
		t.Fatalf("binding = (%q, %v, %v), want the child %q", bound, found, err, childID)
	}
	if _, killed, err := database.Killed(context.Background(), parentID); err != nil || killed {
		t.Fatalf("a same-lineage resume retired the chat's own root: killed=%v error=%v", killed, err)
	}
}
