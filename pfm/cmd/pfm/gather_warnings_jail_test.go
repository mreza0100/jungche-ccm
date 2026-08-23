package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"hostops/pfm/internal/store"
	"hostops/pfm/internal/ui"
)

// createDeadSocket fabricates a real orphaned unix socket special file: a
// listener bound then closed without unlinking, so a live tmux client hangs
// nothing but still fails to connect — the fixture the gather package's own
// jail tests use to raise a tmux probe warning deterministically
// (internal/gather/tmux_jail_test.go:createCorpseSocket does the same thing
// for the same reason; duplicated here rather than exported across a
// package boundary a test-only helper has no other reason to cross).
func createDeadSocket(t *testing.T, path string) {
	t.Helper()
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatalf("create dead socket %q: %v", path, err)
	}
	listener.SetUnlinkOnClose(false)
	if err := listener.Close(); err != nil {
		t.Fatalf("close dead socket %q: %v", path, err)
	}
}

// stubFailingTmux puts a tmux on PATH that fails the way a REAL anomaly does —
// not "no server running", which is now the ordinary end of a chat and stays
// silent, but a failure an operator has to see.
func stubFailingTmux(t *testing.T, root string) {
	t.Helper()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatalf("create stub bin dir: %v", err)
	}
	stub := "#!/bin/sh\necho 'tmux: connect failed: permission denied' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "tmux"), []byte(stub), 0o700); err != nil {
		t.Fatalf("write tmux stub: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestInteractiveRefreshBuffersGatherWarningsUntilFlushed is BUG 4's
// red-first fixture: pipeline.go used to write a tmux probe warning straight
// to stderr from gatherFleet (cmd/pfm/pipeline.go, the loop over
// live.Warnings) regardless of who called it — including the background
// refresh goroutine streamFleetRefreshesWith runs WHILE the interactive
// picker owns the tty (runLS starts it right before Pick). That write
// corrupts Bubble Tea's alt-screen frame. The fix threads a warn callback
// through instead of the direct write: the interactive path buffers, the
// plain/tsv/check/one-shot paths keep printing immediately.
func TestInteractiveRefreshBuffersGatherWarningsUntilFlushed(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxDir := filepath.Join(root, "tmux")
	const socketName = "cc-dead-1-2-3"
	createDeadSocket(t, filepath.Join(tmuxDir, socketName))
	// A socket with no server behind it is swept in SILENCE now, so it can no
	// longer be the warning this test buffers. Stand in a tmux that fails for a
	// reason worth reading instead — the anomaly class buffering exists for.
	stubFailingTmux(t, root)

	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	// The interactive path: streamFleetRefreshesWith is exactly what runLS
	// starts in the background while BubblePicker.Pick owns the terminal.
	var warnings bufferedWarnings
	var interactiveStderr bytes.Buffer
	updates := make(chan ui.Snapshot, 4)
	refreshContext, cancelRefresh := context.WithCancel(context.Background())
	go streamFleetRefreshesWith(
		refreshContext,
		database,
		scanRequest{},
		warnings.add,
		&interactiveStderr,
		updates,
		refreshDependencies{
			newIndexer: func(*store.Store) (indexRunner, error) {
				return &immediateIndexRunner{}, nil
			},
		},
	)
	for {
		snapshot, ok := <-updates
		if !ok {
			t.Fatalf("interactive refresh stream ended early: %s", interactiveStderr.String())
		}
		if !snapshot.Refreshing {
			break
		}
	}
	cancelRefresh()
	for range updates {
	}
	if interactiveStderr.Len() != 0 {
		t.Fatalf(
			"interactive refresh wrote to stderr before the picker released the terminal: %q",
			interactiveStderr.String(),
		)
	}
	warnings.flush(&interactiveStderr)
	if got := interactiveStderr.String(); !strings.Contains(got, "tmux probe warning: "+socketName) {
		t.Fatalf("flush did not emit the buffered warning: %q", got)
	}

	// The plain/tsv/check/one-shot path: scanFleet must keep printing
	// immediately — buffering is specific to the interactive picker owning a
	// real tty, not a blanket behavior change.
	var directStderr bytes.Buffer
	if _, err := scanFleet(
		context.Background(),
		database,
		scanRequest{},
		&directStderr,
	); err != nil {
		t.Fatal(err)
	}
	if got := directStderr.String(); !strings.Contains(got, "tmux probe warning: "+socketName) {
		t.Fatalf("scanFleet stopped printing immediately: %q", got)
	}
}

// Codex clear reconciliation runs inside the same background refresh that
// gathers tmux state. Its warnings must use the same deferred sink: writing
// them directly to stderr while Bubble Tea owns the terminal corrupts the
// active frame with text such as "matches more than one thread".
func TestInteractiveRefreshBuffersCodexReconcileWarningsUntilFlushed(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxDir := filepath.Join(root, "tmux-"+strconv.Itoa(os.Getuid()))
	t.Setenv("PFM_TMUX_DIR", tmuxDir)
	const socket = "cx-1800000004-1-1"
	startCodexStatusPane(t, root, socket, `  DUPLICATE · /work/example · Full Access\n`)

	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	const firstID = "77777777-7777-4777-8777-777777777777"
	const secondID = "88888888-8888-4888-8888-888888888888"
	codexJailRollout(t, database, root, firstID, 1)
	codexJailRollout(t, database, root, secondID, 1)
	if err := database.ReplaceCxNames(context.Background(), []store.CxName{
		{ID: firstID, ThreadName: "DUPLICATE"},
		{ID: secondID, ThreadName: "DUPLICATE"},
	}); err != nil {
		t.Fatal(err)
	}

	var warnings bufferedWarnings
	var interactiveStderr bytes.Buffer
	updates := make(chan ui.Snapshot, 4)
	refreshContext, cancelRefresh := context.WithCancel(context.Background())
	go streamFleetRefreshesWith(
		refreshContext,
		database,
		scanRequest{},
		warnings.add,
		&interactiveStderr,
		updates,
		refreshDependencies{
			newIndexer: func(*store.Store) (indexRunner, error) {
				return &immediateIndexRunner{}, nil
			},
		},
	)
	for {
		snapshot, ok := <-updates
		if !ok {
			t.Fatalf("interactive refresh stream ended early: %s", interactiveStderr.String())
		}
		if !snapshot.Refreshing {
			break
		}
	}
	cancelRefresh()
	for range updates {
	}
	if interactiveStderr.Len() != 0 {
		t.Fatalf(
			"Codex reconcile wrote to stderr before the picker released the terminal: %q",
			interactiveStderr.String(),
		)
	}
	warnings.flush(&interactiveStderr)
	if got := interactiveStderr.String(); !strings.Contains(got, `"DUPLICATE" matches more than one thread`) {
		t.Fatalf("flush did not emit the buffered Codex warning: %q", got)
	}
}

func TestBufferedWarningsDeduplicateRepeatedRefreshFailures(t *testing.T) {
	var warnings bufferedWarnings
	warnings.add("same failure")
	warnings.add("same failure")

	var output bytes.Buffer
	warnings.flush(&output)
	if got := strings.Count(output.String(), "same failure"); got != 1 {
		t.Fatalf("same warning printed %d times, want once: %q", got, output.String())
	}
}
