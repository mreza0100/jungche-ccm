package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaudeLauncherInstallDisplacementAndRepair(t *testing.T) {
	home := t.TempDir()
	canonical := filepath.Join(home, ".local", "bin", "claude")
	nativeOne := filepath.Join(home, ".local", "share", "claude", "versions", "1.0.0")
	nativeTwo := filepath.Join(home, ".local", "share", "claude", "versions", "2.0.0")
	for _, binary := range []string{nativeOne, nativeTwo} {
		if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(canonical), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(nativeOne, canonical); err != nil {
		t.Fatal(err)
	}

	apply := func() {
		t.Helper()
		if _, err := Run(context.Background(), Options{
			Mode: ModeApply, Home: home, Runner: &fakeRunner{},
		}); err != nil {
			t.Fatal(err)
		}
	}
	apply()
	managed := filepath.Join(home, ".local", "share", "pfm", "install", "bin", "claude")
	assertLink(t, canonical, managed)
	content, err := os.ReadFile(filepath.Join(home, ".local", "share", "pfm", "install", "launcher.state"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), nativeOne) {
		t.Fatalf("launcher.state=%q, want displaced target %q", content, nativeOne)
	}

	if err := os.Remove(canonical); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(nativeTwo, canonical); err != nil {
		t.Fatal(err)
	}
	status, err := InspectClaudeLauncher(home)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != LauncherDisplaced || status.Target != nativeTwo {
		t.Fatalf("displaced status=%#v, want target %s", status, nativeTwo)
	}

	apply()
	assertLink(t, canonical, managed)
	if changed, err := RepairClaudeLauncher(home); err != nil || changed {
		t.Fatalf("idempotent repair changed=%t err=%v", changed, err)
	}
	if err := os.Remove(canonical); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(nativeTwo, canonical); err != nil {
		t.Fatal(err)
	}
	if changed, err := RepairClaudeLauncher(home); err != nil || !changed {
		t.Fatalf("displaced repair changed=%t err=%v", changed, err)
	}
	assertLink(t, canonical, managed)
}

func TestRenderedClaudeLauncherUsesConfiguredAbsoluteBinaryThenSkipsItself(t *testing.T) {
	raw, err := readAsset("bin/claude")
	if err != nil {
		t.Fatal(err)
	}
	configured := filepath.Join(t.TempDir(), "configured claude")
	rendered := string(renderClaudeLauncherAsset(raw, Options{ClaudeBinary: configured}))
	if !strings.Contains(rendered, configured) {
		t.Fatalf("rendered launcher omitted configured binary %q:\n%s", configured, rendered)
	}
	for _, want := range []string{"internal launch --real", `"$@"`, ".local/share/claude/versions", "command -v"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered launcher omitted %q:\n%s", want, rendered)
		}
	}
}

func TestRenderedClaudeLauncherChoosesNewestVersionByFreshness(t *testing.T) {
	home := t.TempDir()
	raw, err := readAsset("bin/claude")
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(home, ".local", "share", "pfm", "install", "bin", "claude")
	if err := os.MkdirAll(filepath.Dir(launcher), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, renderClaudeLauncherAsset(raw, Options{}), 0o700); err != nil {
		t.Fatal(err)
	}
	versions := filepath.Join(home, ".local", "share", "claude", "versions")
	if err := os.MkdirAll(versions, 0o700); err != nil {
		t.Fatal(err)
	}
	lexicallyLast := filepath.Join(versions, "9.9.9")
	newest := filepath.Join(versions, "10.0.0")
	for _, path := range []string{lexicallyLast, newest} {
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	stamp := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lexicallyLast, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newest, stamp.Add(time.Minute), stamp.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	pfm := filepath.Join(home, ".local", "bin", "pfm")
	if err := os.MkdirAll(filepath.Dir(pfm), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pfm, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(launcher, "--resume", "fixture")
	command.Env = []string{"HOME=" + home, "PATH=" + filepath.Join(home, ".local", "bin") + ":/usr/bin:/bin"}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("launcher failed: %v: %s", err, output)
	}
	want := "internal launch --real " + newest + " -- --resume fixture\n"
	if string(output) != want {
		t.Fatalf("launcher output=%q, want newest-by-mtime %q", output, want)
	}
}

func TestInspectClaudeLauncherRejectsBrokenManagedTarget(t *testing.T) {
	home := t.TempDir()
	canonical := canonicalClaudeLauncher(home)
	if err := os.MkdirAll(filepath.Dir(canonical), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managedClaudeLauncher(home), canonical); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectClaudeLauncher(home); err == nil || !strings.Contains(err.Error(), "inspect managed Claude launcher") {
		t.Fatalf("InspectClaudeLauncher broken target error=%v", err)
	}
}
