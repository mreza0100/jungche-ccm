package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallReachableBusReturns97BeforeWriting(t *testing.T) {
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
	if !strings.Contains(stderr.String(), "live user systemd bus is reachable") {
		t.Fatalf("stderr=%q, want reachable-bus refusal", stderr.String())
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rc 97 refusal wrote files: entries=%v err=%v", entries, err)
	}
}
