package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"hostops/cc-fleet/internal/compose"
	"hostops/cc-fleet/internal/gather"
)

func TestSocketSquattersCountsOnlyDifferentlyNamedSessions(t *testing.T) {
	// cc-parked lost its chat but kept its tmux server, and three dev servers
	// were started on it. cc-chat is an ordinary chat: one session, named after
	// its socket, split across two panes.
	got := socketSquatters([]gather.Pane{
		{Socket: "cc-parked", SessionName: "projb-dev-backend-4100"},
		{Socket: "cc-parked", SessionName: "projb-dev-frontend-5273"},
		{Socket: "cc-parked", SessionName: "projb-dev-cortex-8100"},
		{Socket: "cc-chat", SessionName: "cc-chat"},
		{Socket: "cc-chat", SessionName: "cc-chat"},
		{Socket: "cc-mixed", SessionName: "cc-mixed"},
		{Socket: "cc-mixed", SessionName: "parked-server"},
		{Socket: "cc-mixed", SessionName: "parked-server"},
		{Socket: "cc-nameless", SessionName: ""},
	})
	want := map[string]int{"cc-parked": 3, "cc-mixed": 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("socketSquatters() = %#v, want %#v", got, want)
	}
}

func TestDormantAccountIDsIgnoreRootsThatCanonicaliseIntoAccountOne(t *testing.T) {
	rows := []compose.Row{
		{Kind: compose.ResumeClaude, ID: "on-one", Account: 1},
		{Kind: compose.ResumeClaude, ID: "on-shared-two", Account: 2},
		{Kind: compose.ResumeClaude, ID: "on-separate-three", Account: 3},
		{Kind: compose.ResumeCodex, ID: "codex-two", Account: 2},
	}
	// Account 2's root is the symlink shape this fleet actually has: it
	// canonicalises onto account 1's store, so the legacy picker DOES walk it.
	roots := []compose.AccountRoot{
		{Account: 1, Path: "/home/user/.claude/projects"},
		{Account: 2, Path: "/home/user/.claude/projects"},
		{Account: 3, Path: "/home/user/.cc/3/projects"},
	}
	got := dormantAccountIDs(rows, roots)
	want := map[string]struct{}{"on-separate-three": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dormantAccountIDs() = %#v, want %#v", got, want)
	}
}

func TestResolveCheckAllowlistOrder(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "bin")
	working := filepath.Join(root, "tree")
	override := filepath.Join(root, "override-allowlist.txt")
	beside := filepath.Join(executable, "testdata", checkAllowlistFile)
	underWorking := filepath.Join(working, "testdata", checkAllowlistFile)
	t.Setenv(checkAllowlistEnv, "")

	if _, err := resolveCheckAllowlist(executable, working); err == nil {
		t.Fatal("resolveCheckAllowlist() with no allowlist on disk error = nil, want error")
	} else if !strings.Contains(err.Error(), beside) ||
		!strings.Contains(err.Error(), underWorking) {
		t.Fatalf("resolveCheckAllowlist() error = %v, want both candidates named", err)
	}

	writeCheckAllowlist(t, underWorking)
	got, err := resolveCheckAllowlist(executable, working)
	if err != nil || got != underWorking {
		t.Fatalf("resolveCheckAllowlist() = %q, %v; want %q, nil", got, err, underWorking)
	}

	writeCheckAllowlist(t, beside)
	got, err = resolveCheckAllowlist(executable, working)
	if err != nil || got != beside {
		t.Fatalf(
			"resolveCheckAllowlist() with both candidates = %q, %v; want %q, nil",
			got,
			err,
			beside,
		)
	}

	writeCheckAllowlist(t, override)
	t.Setenv(checkAllowlistEnv, override)
	got, err = resolveCheckAllowlist(executable, working)
	if err != nil || got != override {
		t.Fatalf("resolveCheckAllowlist() with %s = %q, %v; want %q, nil",
			checkAllowlistEnv, got, err, override)
	}
}

func TestResolveCheckAllowlistSkipsMissingDirectories(t *testing.T) {
	root := t.TempDir()
	working := filepath.Join(root, "tree")
	underWorking := filepath.Join(working, "testdata", checkAllowlistFile)
	writeCheckAllowlist(t, underWorking)
	t.Setenv(checkAllowlistEnv, "")

	got, err := resolveCheckAllowlist("", working)
	if err != nil || got != underWorking {
		t.Fatalf(
			"resolveCheckAllowlist() with unknown executable = %q, %v; want %q, nil",
			got,
			err,
			underWorking,
		)
	}

	if _, err := resolveCheckAllowlist("", ""); err == nil {
		t.Fatal("resolveCheckAllowlist() with no directories error = nil, want error")
	}
}

func TestResolveCheckAllowlistIgnoresHome(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	writeCheckAllowlist(
		t,
		filepath.Join(home, "work", "host-ops", "cc-fleet", "testdata", checkAllowlistFile),
	)
	t.Setenv(checkAllowlistEnv, "")
	t.Setenv("CC_FLEET_HOME", home)
	t.Setenv("HOME", home)

	if got, err := resolveCheckAllowlist(
		filepath.Join(root, "bin"),
		filepath.Join(root, "tree"),
	); err == nil {
		t.Fatalf("resolveCheckAllowlist() = %q, want an error rather than a home-relative path", got)
	}
}

func writeCheckAllowlist(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# jailed allowlist\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
