package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"hostops/pfm/internal/policy"
)

// TestRunAutonomyDefaultsToOff is the regression guard at the CLI surface:
// with no PFM_AUTONOMY and no config file, `pfm autonomy` must answer "off".
func TestRunAutonomyDefaultsToOff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(policy.EnvAutonomy, "")
	var stdout, stderr bytes.Buffer
	if code := runAutonomy(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("runAutonomy() code=%d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); got != "off\n" {
		t.Fatalf("runAutonomy() stdout=%q, want %q", got, "off\n")
	}
}

func TestRunAutonomyReadsEnvOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(policy.EnvAutonomy, "1")
	var stdout, stderr bytes.Buffer
	if code := runAutonomy(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("runAutonomy() code=%d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); got != "on\n" {
		t.Fatalf("runAutonomy() stdout=%q, want %q", got, "on\n")
	}
}

func TestRunAutonomyPathPrintsConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	if code := runAutonomy([]string{"--path"}, &stdout, &stderr); code != 0 {
		t.Fatalf("runAutonomy(--path) code=%d stderr=%q", code, stderr.String())
	}
	want := filepath.Join(home, ".config", "pfm", "config.json") + "\n"
	if got := stdout.String(); got != want {
		t.Fatalf("runAutonomy(--path) stdout=%q, want %q", got, want)
	}
}
