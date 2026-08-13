package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/store"
	"hostops/cc-fleet/internal/ui"
)

func jailPaths(t *testing.T) paths.Values {
	t.Helper()
	resolved, err := paths.Resolve()
	if err != nil {
		t.Fatalf("resolve jail paths: %v", err)
	}
	return resolved
}

// ⌃X on a LIVE row does not merely file the chat away — it ENDS it. These are
// the fixtures for that, run against real tmux servers on scratch sockets
// inside the jail, because a kill that only works in a mock is not a kill.

// startJailServer brings up a real tmux server on the jail's fleet socket dir,
// addressed the way action.CommandTmux addresses one (-S <dir>/<socket>).
func startJailServer(t *testing.T, tmuxDir, socket string) {
	t.Helper()
	if err := os.MkdirAll(tmuxDir, 0o700); err != nil {
		t.Fatalf("create jail tmux dir: %v", err)
	}
	command := exec.Command(
		"tmux", "-f", "/dev/null",
		"-S", filepath.Join(tmuxDir, socket),
		"new-session", "-d", "-s", "chat", "sleep 120",
	)
	command.Env = append(os.Environ(), "TMUX=")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start jail tmux server %q: %v: %s", socket, err, output)
	}
	t.Cleanup(func() {
		kill := exec.Command("tmux", "-S", filepath.Join(tmuxDir, socket), "kill-server")
		kill.Env = append(os.Environ(), "TMUX=")
		_ = kill.Run()
	})
}

func jailServerAlive(tmuxDir, socket string) bool {
	command := exec.Command(
		"tmux", "-S", filepath.Join(tmuxDir, socket), "list-panes", "-a",
	)
	command.Env = append(os.Environ(), "TMUX=")
	return command.Run() == nil
}

// TestHidingALiveChatEndsItAndClearsItsHandles is the whole contract of the
// destructive half: the store row lands, the server dies, and every handle
// that pointed at it — socket file, sid crumb — is gone, so nothing is left
// resolving the chat to a server that no longer exists.
func TestHidingALiveChatEndsItAndClearsItsHandles(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxDir := filepath.Join(root, "tmux")
	sidDir := filepath.Join(root, "sid")
	const socket = "cc-1800000001-42-7"
	startJailServer(t, tmuxDir, socket)
	if !jailServerAlive(tmuxDir, socket) {
		t.Fatal("setup: jail server did not come up")
	}
	if err := os.MkdirAll(sidDir, 0o700); err != nil {
		t.Fatal(err)
	}
	crumb := filepath.Join(sidDir, socket+".%0")
	if err := os.WriteFile(crumb, []byte("/transcript.jsonl\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	resolved := jailPaths(t)
	apply, err := hideApplier(context.Background(), database, resolved)
	if err != nil {
		t.Fatal(err)
	}

	const id = "11111111-1111-4111-8111-111111111111"
	if err := apply(ui.HideChange{
		ID:     id,
		Engine: "cc",
		Hidden: true,
		Socket: socket,
		Live:   true,
		Name:   "DOOMED",
	}); err != nil {
		t.Fatalf("hide a live chat: %v", err)
	}

	if jailServerAlive(tmuxDir, socket) {
		t.Fatal("⌃X hid the chat but left it running")
	}
	if _, err := os.Stat(filepath.Join(tmuxDir, socket)); !os.IsNotExist(err) {
		t.Fatalf("socket file outlived the kill: %v", err)
	}
	if _, err := os.Stat(crumb); !os.IsNotExist(err) {
		t.Fatalf("sid crumb still points at a dead server: %v", err)
	}
}

// TestHidingAChatThatIsNotRunningKillsNothing: the kill is scoped to rows the
// picker saw as live. A resumable row is filed away and nothing is touched.
func TestHidingAChatThatIsNotRunningKillsNothing(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxDir := filepath.Join(root, "tmux")
	const bystander = "cc-1800000002-43-8"
	startJailServer(t, tmuxDir, bystander)

	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	apply, err := hideApplier(context.Background(), database, jailPaths(t))
	if err != nil {
		t.Fatal(err)
	}

	if err := apply(ui.HideChange{
		ID:     "22222222-2222-4222-8222-222222222222",
		Engine: "cc",
		Hidden: true,
		Live:   false,
		Name:   "ARCHIVED",
	}); err != nil {
		t.Fatalf("hide a resumable chat: %v", err)
	}
	if !jailServerAlive(tmuxDir, bystander) {
		t.Fatal("hiding a resumable row killed a server that was not its own")
	}
}

// TestHidingAChatWhoseServerAlreadyDiedStillSucceeds: killing a corpse fails
// loudly at the tmux level for no reason an operator can act on. The goal is
// "not running", so a chat that died on its own still hides cleanly.
func TestHidingAChatWhoseServerAlreadyDiedStillSucceeds(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := jailTest(t)
	tmuxDir := filepath.Join(root, "tmux")
	const socket = "cc-1800000003-44-9"
	if err := os.MkdirAll(tmuxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	createDeadSocket(t, filepath.Join(tmuxDir, socket))

	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	apply, err := hideApplier(context.Background(), database, jailPaths(t))
	if err != nil {
		t.Fatal(err)
	}

	if err := apply(ui.HideChange{
		ID:     "33333333-3333-4333-8333-333333333333",
		Engine: "cc",
		Hidden: true,
		Socket: socket,
		Live:   true,
		Name:   "ALREADY-GONE",
	}); err != nil {
		t.Fatalf("hiding an already-dead chat reported a failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmuxDir, socket)); !os.IsNotExist(err) {
		t.Fatalf("corpse socket survived: %v", err)
	}
}
