package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
