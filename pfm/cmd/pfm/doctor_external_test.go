package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/deps"
	"hostops/pfm/internal/installer"
	"hostops/pfm/internal/paths"
)

func TestDoctorEnumeratesExternalDependenciesAndInstalledHooks(t *testing.T) {
	jailTest(t)
	runtime, err := loadCommandRuntime("")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runDoctor(nil, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("doctor code=%d, want clean injected probes\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	for _, wanted := range []string{
		"doctor: dep tmux ",
		"doctor: hook codex ",
	} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Fatalf("doctor output missing %q:\n%s", wanted, stdout.String())
		}
	}
}

func TestDependencyDoctorRowsKeepMissingBrokenAndSkippedDistinct(t *testing.T) {
	saved := dependencyProbeOverride
	t.Cleanup(func() { dependencyProbeOverride = saved })
	entries := []deps.Entry{
		{Name: "tmux", Required: true, MinVersion: "1.8"},
		{Name: "ps", Required: true, Platforms: []string{"darwin"}},
		{Name: "codex", Required: true, InstallHint: "install configured Codex"},
		{Name: "claude", Required: true},
	}
	dependencyProbeOverride = func(context.Context, []deps.Entry, deps.ProbeOptions) []deps.Result {
		return []deps.Result{
			{Entry: entries[0], State: deps.StateOK, Path: "/fixture/tmux", Version: "3.4"},
			{Entry: entries[1], State: deps.StateSkipped, Error: "not this platform"},
			{Entry: entries[2], State: deps.StateMissing},
			{Entry: entries[3], State: deps.StateBroken, Path: "/fixture/claude", Error: "exit status 1", Raw: "damaged install\nmore"},
		}
	}
	var output bytes.Buffer
	if warnings := printDependencyDoctor(context.Background(), &output, entries, deps.ProbeOptions{}); warnings != 2 {
		t.Fatalf("warnings=%d, want 2\n%s", warnings, output.String())
	}
	want := strings.Join([]string{
		"doctor: dep tmux path=/fixture/tmux version=3.4 min=1.8 ok",
		"doctor: dep ps platform=darwin skipped (not this platform)",
		"doctor: dep codex path=(none) MISSING required — install: install configured Codex",
		`doctor: dep claude path=/fixture/claude broken error=exit status 1 raw="damaged install"`,
		"",
	}, "\n")
	if output.String() != want {
		t.Fatalf("dependency rows:\n%s\nwant:\n%s", output.String(), want)
	}
}

func TestInstallPreflightRefusesRequiredDependencyEvenWithForce(t *testing.T) {
	savedProbe, savedInstaller := dependencyProbeOverride, runInstaller
	t.Cleanup(func() {
		dependencyProbeOverride = savedProbe
		runInstaller = savedInstaller
	})
	dependencyProbeOverride = func(_ context.Context, entries []deps.Entry, _ deps.ProbeOptions) []deps.Result {
		for _, entry := range entries {
			if entry.Name == "tmux" {
				return []deps.Result{{Entry: entry, State: deps.StateMissing}}
			}
		}
		t.Fatal("tmux registry entry missing")
		return nil
	}
	called := false
	runInstaller = func(context.Context, installer.Options) (installer.Report, error) {
		called = true
		return installer.Report{}, nil
	}
	home := t.TempDir()
	runtime := commandRuntime{
		Config: pfmconfig.Config{Claude: pfmconfig.Claude{Binary: "claude"}, Codex: pfmconfig.Codex{Binary: "codex"}},
		Paths:  paths.Values{Home: home, CodexRoot: filepath.Join(home, ".codex")},
	}
	var stdout, stderr bytes.Buffer
	if code := runInstall([]string{"--yes", "--force", "--skip-harvest"}, &stdout, &stderr, runtime); code != 1 {
		t.Fatalf("install code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if called || !strings.Contains(stdout.String(), "doctor: dep tmux path=(none) MISSING required") || !strings.Contains(stderr.String(), "required dependency preflight failed") {
		t.Fatalf("called=%t stdout=%s stderr=%s", called, stdout.String(), stderr.String())
	}
}

func TestHookDoctorRowsCountMissingBrokenAndDriftWarnings(t *testing.T) {
	saved := hookProbeOverride
	t.Cleanup(func() { hookProbeOverride = saved })
	home := t.TempDir()
	hookProbeOverride = func(string, pfmconfig.Config) []installer.HookProbeResult {
		return []installer.HookProbeResult{
			{Hook: installer.ExpectedHook{Target: "claude[1]", File: filepath.Join(home, ".claude", "settings.json"), Event: "SessionEnd", Name: "clear-kill"}, State: "ok"},
			{Hook: installer.ExpectedHook{Target: "codex", File: filepath.Join(home, ".codex", "hooks.json"), Event: "SessionStart", Name: "clear-kill"}, State: "missing"},
			{Hook: installer.ExpectedHook{Target: "claude[2]", File: filepath.Join(home, ".cc", "2", "settings.json"), Event: "UserPromptSubmit", Name: "usage"}, State: "broken", Error: "parse error"},
			{Hook: installer.ExpectedHook{Target: "ownership", File: filepath.Join(home, "ledger.json"), Event: "SessionEnd", Name: "unexpected"}, State: "drift", Error: "ledger owns 1 hook absent from expectations"},
		}
	}
	var output bytes.Buffer
	if warnings := printHookDoctor(&output, home, pfmconfig.Config{}); warnings != 3 {
		t.Fatalf("warnings=%d output=%s", warnings, output.String())
	}
	for _, wanted := range []string{
		"doctor: hook claude[1] settings.json SessionEnd clear-kill ok",
		"doctor: hook codex hooks.json SessionStart clear-kill MISSING — run pfm install",
		"doctor: hook claude[2] settings.json UserPromptSubmit usage broken error=parse error",
		"doctor: hook ownership ledger.json SessionEnd unexpected drift error=ledger owns 1 hook absent from expectations",
	} {
		if !strings.Contains(output.String(), wanted) {
			t.Errorf("output missing %q:\n%s", wanted, output.String())
		}
	}
}
