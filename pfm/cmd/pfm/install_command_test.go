package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallGateScopesDryRunIdleAndRunningService(t *testing.T) {
	t.Run("dry run ignores reachable manager", func(t *testing.T) {
		home := t.TempDir()
		bin := filepath.Join(t.TempDir(), "systemctl")
		if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)
		t.Setenv("PATH", filepath.Dir(bin))
		var stdout, stderr bytes.Buffer
		if code := runInstall([]string{"--dry-run"}, &stdout, &stderr); code != 0 {
			t.Fatalf("dry-run code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
			t.Fatalf("dry-run wrote files: entries=%v err=%v", entries, err)
		}
	})

	t.Run("idle reachable manager applies", func(t *testing.T) {
		home := t.TempDir()
		bin := filepath.Join(t.TempDir(), "systemctl")
		script := "#!/bin/sh\nif [ \"$*\" = \"--user is-active --quiet pfm-name-sync.service\" ]; then exit 1; fi\nexit 0\n"
		if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)
		t.Setenv("PATH", filepath.Dir(bin))
		var stdout, stderr bytes.Buffer
		if code := runInstall([]string{"--apply"}, &stdout, &stderr); code != 0 {
			t.Fatalf("idle apply code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	t.Run("running service refuses actionably", func(t *testing.T) {
		home := t.TempDir()
		bin := filepath.Join(t.TempDir(), "systemctl")
		if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)
		t.Setenv("PATH", filepath.Dir(bin))

		var stdout, stderr bytes.Buffer
		if code := runInstall([]string{"--apply"}, &stdout, &stderr); code != 97 {
			t.Fatalf("runInstall() code=%d, want 97; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "systemctl --user stop pfm-name-sync.service") {
			t.Fatalf("stderr=%q, want actionable running-service refusal", stderr.String())
		}
		entries, err := os.ReadDir(home)
		if err != nil || len(entries) != 0 {
			t.Fatalf("rc 97 refusal wrote files: entries=%v err=%v", entries, err)
		}
	})
}
