package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorFreshTargetHomeIsClean(t *testing.T) {
	t.Setenv("HARVESTER_BROWSER", "") // the golden "clean" output must not depend on the ambient opt-in gate (review-2 S3)
	home := t.TempDir()
	canonicalDir := filepath.Join(home, ".local", "bin")
	hostShimDir := filepath.Join(t.TempDir(), "bin")
	for _, directory := range []string{
		canonicalDir,
		hostShimDir,
		filepath.Join(home, ".cc", "1", "projects"),
		filepath.Join(home, ".cc", "2", "projects"),
		filepath.Join(home, ".codex"),
		filepath.Join(home, ".local", "state", "pfm"),
		filepath.Join(home, "proc"),
		filepath.Join(home, "tmux"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	canonical := filepath.Join(canonicalDir, "pfm")
	if err := os.WriteFile(canonical, []byte("target-pfm"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostShimDir, "pfm"), []byte("host-pfm"), 0o700); err != nil {
		t.Fatal(err)
	}
	managedClaude := filepath.Join(home, ".local", "share", "pfm", "install", "bin", "claude")
	if err := os.MkdirAll(filepath.Dir(managedClaude), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedClaude, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managedClaude, filepath.Join(canonicalDir, "claude")); err != nil {
		t.Fatal(err)
	}
	// The pfm-statusline and tmux-title-renudge host overlays are contracted
	// pfm-install artifacts (issue #14 F1); a fixture representing a healthy
	// target HOME carries both, same managed-copy-then-symlink shape as the
	// Claude launcher above.
	for _, overlay := range []string{"pfm-statusline", "tmux-title-renudge"} {
		managedOverlay := filepath.Join(home, ".local", "share", "pfm", "install", "bin", overlay)
		if err := os.MkdirAll(filepath.Dir(managedOverlay), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(managedOverlay, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(managedOverlay, filepath.Join(canonicalDir, overlay)); err != nil {
			t.Fatal(err)
		}
	}
	stageHarnessPromptBaseline(t, home)

	t.Setenv("HOME", home)
	t.Setenv("PFM_HOME", home)
	t.Setenv("PFM_DB", filepath.Join(home, ".local", "state", "pfm", "fleet.db"))
	t.Setenv("PFM_SHARED_DB", filepath.Join(home, ".cc", "fleet.db"))
	t.Setenv("PFM_SID_DIR", filepath.Join(home, "sid"))
	t.Setenv("PFM_CLAUDE_ROOTS", strings.Join([]string{
		filepath.Join(home, ".cc", "1", "projects"),
		filepath.Join(home, ".cc", "2", "projects"),
	}, string(os.PathListSeparator)))
	t.Setenv("PFM_CODEX_ROOT", filepath.Join(home, ".codex"))
	t.Setenv("PFM_TMUX_DIR", filepath.Join(home, "tmux"))
	t.Setenv("PFM_TMUX_CONF", "/dev/null")
	t.Setenv("PFM_PROC_ROOT", filepath.Join(home, "proc"))
	t.Setenv("PATH", strings.Join([]string{canonicalDir, hostShimDir}, string(os.PathListSeparator)))

	runtime, err := loadCommandRuntime("")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runDoctor(nil, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("fresh target HOME doctor code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "doctor: clean") {
		t.Fatalf("fresh target HOME doctor output=%q", stdout.String())
	}
}

func TestPFMPathWarningsIgnoreHostShimsOutsideTargetHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PFM_HOME", home)
	t.Setenv("PFM_DEV_FENCE", "")
	canonicalDir := filepath.Join(home, ".local", "bin")
	hostShimDir := filepath.Join(t.TempDir(), "host-bin")
	for _, directory := range []string{canonicalDir, hostShimDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(canonicalDir, "pfm"), []byte("target-pfm"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostShimDir, "pfm"), []byte("host-pfm"), 0o700); err != nil {
		t.Fatal(err)
	}

	warnings := pfmPathWarnings(
		home,
		strings.Join([]string{canonicalDir, hostShimDir}, string(os.PathListSeparator)),
	)
	if len(warnings) != 0 {
		t.Fatalf("target HOME PATH warnings=%q, want none for host shim", warnings)
	}
}

func TestPFMPathWarningsReportHostShimsOutsideHomeWithoutAJail(t *testing.T) {
	t.Setenv("PFM_HOME", "")
	t.Setenv("PFM_DEV_FENCE", "")
	home := t.TempDir()
	canonicalDir := filepath.Join(home, ".local", "bin")
	hostShimDir := filepath.Join(t.TempDir(), "host-bin")
	for _, directory := range []string{canonicalDir, hostShimDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(canonicalDir, "pfm"), []byte("canonical-pfm"), 0o700); err != nil {
		t.Fatal(err)
	}
	hostShim := filepath.Join(hostShimDir, "pfm")
	if err := os.WriteFile(hostShim, []byte("shadowing-pfm"), 0o700); err != nil {
		t.Fatal(err)
	}

	warnings := pfmPathWarnings(
		home,
		strings.Join([]string{canonicalDir, hostShimDir}, string(os.PathListSeparator)),
	)
	if !strings.Contains(strings.Join(warnings, "\n"), hostShim) {
		t.Fatalf("PATH warnings=%q, want out-of-home shadow %q reported", warnings, hostShim)
	}
}

func TestCrumbHealthNonDirectoryRemainsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sid-file")
	if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := crumbHealth(path); err == nil {
		t.Fatal("crumbHealth returned nil for a non-directory probe target")
	}
}

func TestCrumbHealthPermissionDeniedRemainsAnError(t *testing.T) {
	path := t.TempDir()
	_, _, err := crumbHealthWith(
		path,
		os.Stat,
		func(string) ([]os.DirEntry, error) { return nil, os.ErrPermission },
	)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("crumbHealth permission error=%v, want permission denied", err)
	}
}

func TestCrumbHealthMissingDirectoryIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sid")
	entries, invalid, err := crumbHealth(path)
	if err != nil {
		t.Fatalf("crumbHealth missing directory error=%v, want nil", err)
	}
	if entries != 0 || invalid != 0 {
		t.Fatalf("crumbHealth missing directory entries=%d invalid=%d, want 0/0", entries, invalid)
	}
}
