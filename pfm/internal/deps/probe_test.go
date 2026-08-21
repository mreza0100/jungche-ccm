package deps

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProbeDistinguishesOKMinimumGarbageMissingAndTimeout(t *testing.T) {
	directory := t.TempDir()
	writeProbeStub(t, directory, "tmux-ok", "printf 'tmux 3.4\\n'")
	writeProbeStub(t, directory, "tmux-old", "printf 'tmux 1.7\\n'")
	writeProbeStub(t, directory, "garbage", "printf 'not-a-version\\n'")
	writeProbeStub(t, directory, "timeout", "/bin/sleep 1")
	writeProbeStub(t, directory, "failed", "printf 'permission denied by fixture\\n'; exit 7")
	t.Setenv("PATH", directory)

	entries := []Entry{
		{Name: "ok", Command: "tmux-ok", Required: true, VersionArgs: []string{"-V"}, MinVersion: "1.8", Parse: prefixedVersion("tmux")},
		{Name: "old", Command: "tmux-old", Required: true, VersionArgs: []string{"-V"}, MinVersion: "1.8", Parse: prefixedVersion("tmux")},
		{Name: "garbage", Command: "garbage", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion},
		{Name: "missing", Command: "absent", Required: true},
		{Name: "timeout", Command: "timeout", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion},
		{Name: "failed", Command: "failed", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion},
	}
	results := Probe(context.Background(), entries, ProbeOptions{GOOS: "linux", Timeout: 50 * time.Millisecond})
	want := []State{StateOK, StateBroken, StateBroken, StateMissing, StateBroken, StateBroken}
	for index := range want {
		if results[index].State != want[index] {
			t.Errorf("%s state=%s error=%q raw=%q, want %s", entries[index].Name, results[index].State, results[index].Error, results[index].Raw, want[index])
		}
	}
	if results[0].Version != "3.4" || results[1].Version != "1.7" {
		t.Fatalf("parsed versions ok=%q old=%q", results[0].Version, results[1].Version)
	}
	if results[4].Error != context.DeadlineExceeded.Error() {
		t.Fatalf("timeout error=%q, want %q", results[4].Error, context.DeadlineExceeded)
	}
}

func TestProbePlatformAndHarvestFiltering(t *testing.T) {
	entries := []Entry{
		{Name: "ps", Command: "ps", Required: true, Platforms: []string{"darwin"}},
		{Name: "uv", Command: "/managed/uv", Required: true, Harvest: true},
	}
	linux := Probe(context.Background(), entries, ProbeOptions{GOOS: "linux", SkipHarvest: true})
	if linux[0].State != StateSkipped || linux[0].Error != "not this platform" || linux[1].State != StateSkipped || linux[1].Error != "--skip-harvest" {
		t.Fatalf("linux filters=%#v", linux)
	}
	darwin := Probe(context.Background(), entries[:1], ProbeOptions{
		GOOS:     "darwin",
		LookPath: func(string) (string, error) { return "/usr/bin/ps", nil },
	})
	if darwin[0].State != StateOK || darwin[0].Path != "/usr/bin/ps" {
		t.Fatalf("darwin ps=%#v", darwin[0])
	}
	provision := Probe(context.Background(), entries[1:], ProbeOptions{GOOS: "linux", Provisioning: true})
	if provision[0].State != StateSkipped || provision[0].Error != "provisioned by install" {
		t.Fatalf("install harvest filter=%#v", provision[0])
	}
}

func TestResolveRejectsRegisteredOffPlatformCommand(t *testing.T) {
	var command string
	switch runtime.GOOS {
	case "linux":
		command = "launchctl"
	case "darwin":
		command = "setsid"
	default:
		t.Skip("registry platform contract is Linux/Darwin only")
	}
	directory := t.TempDir()
	writeProbeStub(t, directory, command, "exit 0")
	t.Setenv("PATH", directory)
	_, err := Resolve(command)
	if err == nil || !strings.Contains(err.Error(), "not supported on "+runtime.GOOS) {
		t.Fatalf("Resolve(%q) error=%v", command, err)
	}
}

func TestProbeDelegatesSupportedEngineSelfDoctor(t *testing.T) {
	directory := t.TempDir()
	writeProbeStub(t, directory, "claude", `
if [ "$1" = "--version" ]; then printf '2.1.238 (Claude Code)\n'; exit 0; fi
if [ "$1" = "doctor" ] && [ "$2" = "--help" ]; then printf 'usage: claude doctor\n'; exit 0; fi
if [ "$1" = "doctor" ]; then printf 'healthy\n'; exit 0; fi
exit 2`)
	t.Setenv("PATH", directory)
	entry := Entry{Name: "claude", Command: "claude", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion, SelfDoctorArgs: []string{"doctor"}}
	result := Probe(context.Background(), []Entry{entry}, ProbeOptions{GOOS: "linux"})[0]
	if result.State != StateOK || result.Version != "2.1.238" || result.SelfDoctor != "ok" {
		t.Fatalf("self doctor result=%#v", result)
	}
}

func TestProbeWritesSuccessfulRawOutputOnlyWhenVerbose(t *testing.T) {
	directory := t.TempDir()
	writeProbeStub(t, directory, "tool", "printf 'tool 4.2\\n'")
	t.Setenv("PATH", directory)
	verboseDir := filepath.Join(t.TempDir(), "tmp", "doctor")
	entry := Entry{Name: "tool", Command: "tool", VersionArgs: []string{"--version"}, Parse: firstVersion}
	result := Probe(context.Background(), []Entry{entry}, ProbeOptions{GOOS: "linux", VerboseDir: verboseDir})[0]
	if result.State != StateOK || result.VerboseErr != "" {
		t.Fatalf("verbose probe result=%#v", result)
	}
	raw, err := os.ReadFile(filepath.Join(verboseDir, "tool-version.log"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "tool 4.2\n" {
		t.Fatalf("verbose output=%q", raw)
	}
}

func TestProbeSelfDoctorUnsupportedAndTimeoutAreHonest(t *testing.T) {
	directory := t.TempDir()
	writeProbeStub(t, directory, "unsupported", `
if [ "$1" = "--version" ]; then printf '1.0\n'; exit 0; fi
exit 2`)
	writeProbeStub(t, directory, "hung", `
if [ "$1" = "--version" ]; then printf '1.0\n'; exit 0; fi
/bin/sleep 1`)
	t.Setenv("PATH", directory)
	entries := []Entry{
		{Name: "unsupported", Command: "unsupported", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion, SelfDoctorArgs: []string{"doctor"}},
		{Name: "hung", Command: "hung", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion, SelfDoctorArgs: []string{"doctor"}},
	}
	results := Probe(context.Background(), entries, ProbeOptions{GOOS: "linux", Timeout: 50 * time.Millisecond})
	if results[0].State != StateOK || results[0].SelfDoctor != "unavailable" {
		t.Fatalf("unsupported self-doctor=%#v", results[0])
	}
	if results[1].State != StateBroken || results[1].SelfDoctor != "broken" {
		t.Fatalf("hung self-doctor=%#v", results[1])
	}
}

func writeProbeStub(t *testing.T, directory, name, body string) {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
}
