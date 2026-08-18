package dream

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrganRunnerLockExcludesThenReleasesWithoutDeleting(t *testing.T) {
	organRoot := t.TempDir()
	release, err := acquireRunnerLock(organRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquireRunnerLock(organRoot); err == nil {
		t.Fatal("second runner acquired the same organ lock")
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	secondRelease, err := acquireRunnerLock(organRoot)
	if err != nil {
		t.Fatalf("lock did not release: %v", err)
	}
	if err := secondRelease(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(organRoot, "tmp", "runner.lock"))
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("persistent lock evidence = %v, %v", info, err)
	}
}

func TestOrganRunnerLockRefusesSymlink(t *testing.T) {
	organRoot := t.TempDir()
	scratchRoot := filepath.Join(organRoot, "tmp")
	if err := os.Mkdir(scratchRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(organRoot, "outside")
	if err := os.WriteFile(target, []byte("untouched\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(scratchRoot, "runner.lock")); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireRunnerLock(organRoot); err == nil {
		t.Fatal("runner lock followed a symlink")
	}
	raw, err := os.ReadFile(target)
	if err != nil || string(raw) != "untouched\n" {
		t.Fatalf("runner lock changed symlink target: %q, %v", raw, err)
	}
}
