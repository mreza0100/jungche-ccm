package deps

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
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
	writeProbeStub(t, directory, "timeout", "exec /bin/sleep 30")
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
	results := Probe(context.Background(), entries, ProbeOptions{
		GOOS: "linux", Timeout: 2 * time.Second,
	})
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

func TestEngineDependenciesAreRequiredOnlyForPresentRosters(t *testing.T) {
	tests := []struct {
		name                  string
		claude, codex         int
		wantClaude, wantCodex bool
	}{
		{name: "zero zero"},
		{name: "claude only", claude: 2, wantClaude: true},
		{name: "codex only", codex: 3, wantCodex: true},
		{name: "both", claude: 2, codex: 1, wantClaude: true, wantCodex: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries := Registry(Options{
				Home: t.TempDir(), ClaudeAccounts: test.claude, CodexAccounts: test.codex,
			})
			got := map[string]bool{}
			for _, entry := range entries {
				if entry.Name == "claude" || entry.Name == "codex" {
					got[entry.Name] = entry.Required
				}
			}
			if got["claude"] != test.wantClaude || got["codex"] != test.wantCodex {
				t.Fatalf("required claude=%t codex=%t, want %t/%t", got["claude"], got["codex"], test.wantClaude, test.wantCodex)
			}
		})
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
exec /bin/sleep 30`)
	t.Setenv("PATH", directory)
	entries := []Entry{
		{Name: "unsupported", Command: "unsupported", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion, SelfDoctorArgs: []string{"doctor"}},
		{Name: "hung", Command: "hung", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion, SelfDoctorArgs: []string{"doctor"}},
	}
	// Package-level stress runs can delay a 50ms timer past a one-second
	// fixture process, making a deliberate timeout finish successfully before
	// the scheduler delivers cancellation. Preserve a wide separation between
	// the bound and the hung command.
	results := Probe(context.Background(), entries, ProbeOptions{
		GOOS:              "linux",
		Timeout:           2 * time.Second,
		SelfDoctorTimeout: 250 * time.Millisecond,
	})
	if results[0].State != StateOK || results[0].SelfDoctor != "unavailable" {
		t.Fatalf("unsupported self-doctor=%#v", results[0])
	}
	if results[1].State != StateBroken || results[1].SelfDoctor != "broken" {
		t.Fatalf("hung self-doctor=%#v", results[1])
	}
}

// Regression: real `go version` output is "go version go1.24.13 linux/amd64" —
// firstVersion's "vV"-only trim never strips the "go" prefix on the version
// field, so the go dependency read as broken on every real host. codex-cli
// and tmux are carried alongside as guards: both already parse correctly and
// must keep doing so.
func TestFirstVersionParsesRealCommandVersionStrings(t *testing.T) {
	directory := t.TempDir()
	writeProbeStub(t, directory, "go", "printf 'go version go1.24.13 linux/amd64\\n'")
	writeProbeStub(t, directory, "codex", "printf 'codex-cli 0.148.0\\n'")
	writeProbeStub(t, directory, "tmux", "printf 'tmux 3.5a\\n'")
	t.Setenv("PATH", directory)

	tests := []struct {
		name        string
		versionArgs []string
		parse       func(string) (string, error)
		wantVersion string
	}{
		{name: "go", versionArgs: []string{"version"}, parse: firstVersion, wantVersion: "1.24.13"},
		{name: "codex", versionArgs: []string{"--version"}, parse: firstVersion, wantVersion: "0.148.0"},
		{name: "tmux", versionArgs: []string{"-V"}, parse: prefixedVersion("tmux"), wantVersion: "3.5a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := Entry{Name: test.name, Command: test.name, Required: true, VersionArgs: test.versionArgs, Parse: test.parse}
			result := Probe(context.Background(), []Entry{entry}, ProbeOptions{GOOS: "linux"})[0]
			if result.State != StateOK {
				t.Fatalf("%s state=%s error=%q raw=%q, want ok", test.name, result.State, result.Error, result.Raw)
			}
			if result.Version != test.wantVersion {
				t.Fatalf("%s version=%q, want %q", test.name, result.Version, test.wantVersion)
			}
		})
	}
}

// Regression: probeSelfDoctor's second boundedOutput call (the summary
// itself, not the --help probe) maps every failure — including
// context.DeadlineExceeded — straight to "broken", the same bucket as an
// actually-broken binary. A codex doctor that legitimately runs past the
// probe timeout while scanning a large rollout corpus must not read as a
// broken engine and block install preflight; it must be named as a timeout
// distinct from a real failure.
func TestProbeSelfDoctorTimeoutIsNotConflatedWithBroken(t *testing.T) {
	// Subprocess startup competes with every other package during `go test
	// ./...`; 200ms made the quick --version/--help probes fail under ordinary
	// suite contention before this test ever reached the deliberate timeout.
	// Two seconds preserves a bounded test while leaving ample scheduling room.
	const timeout = 2 * time.Second
	durationPattern := regexp.MustCompile(`\d+(\.\d+)?\s*(ms|s|m)\b`)

	t.Run("slow but healthy self-doctor stays ok and is named as a timeout", func(t *testing.T) {
		directory := t.TempDir()
		writeProbeStub(t, directory, "codex", `
if [ "$1" = "--version" ]; then printf 'codex-cli 0.148.0\n'; exit 0; fi
if [ "$1" = "doctor" ] && [ "$2" = "--help" ]; then printf 'usage: codex doctor\n'; exit 0; fi
if [ "$1" = "doctor" ] && [ "$2" = "--summary" ]; then exec /bin/sleep 30; fi
exit 2`)
		t.Setenv("PATH", directory)
		entry := Entry{Name: "codex", Command: "codex", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion, SelfDoctorArgs: []string{"doctor", "--summary", "--ascii", "--no-color"}}
		result := Probe(context.Background(), []Entry{entry}, ProbeOptions{GOOS: "linux", Timeout: timeout})[0]
		if result.State != StateOK {
			t.Fatalf("state=%s error=%q, want ok — a self-doctor that outran the probe timeout must not read as a broken engine", result.State, result.Error)
		}
		if !strings.HasPrefix(result.SelfDoctor, "timeout") {
			t.Fatalf("self_doctor=%q, want it to start with %q", result.SelfDoctor, "timeout")
		}
		combined := result.SelfDoctor + " " + result.Error
		if !durationPattern.MatchString(combined) {
			t.Fatalf("self_doctor=%q error=%q, want the timeout duration named in one of them", result.SelfDoctor, result.Error)
		}
	})

	t.Run("quick non-zero self-doctor still reads broken", func(t *testing.T) {
		directory := t.TempDir()
		writeProbeStub(t, directory, "codex", `
if [ "$1" = "--version" ]; then printf 'codex-cli 0.148.0\n'; exit 0; fi
if [ "$1" = "doctor" ] && [ "$2" = "--help" ]; then printf 'usage: codex doctor\n'; exit 0; fi
if [ "$1" = "doctor" ] && [ "$2" = "--summary" ]; then printf 'boom\n'; exit 3; fi
exit 2`)
		t.Setenv("PATH", directory)
		entry := Entry{Name: "codex", Command: "codex", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion, SelfDoctorArgs: []string{"doctor", "--summary", "--ascii", "--no-color"}}
		result := Probe(context.Background(), []Entry{entry}, ProbeOptions{GOOS: "linux", Timeout: timeout})[0]
		if result.State != StateBroken || result.SelfDoctor != "broken" {
			t.Fatalf("result=%#v, want StateBroken with self_doctor=broken", result)
		}
	})
}

func TestSelfDoctorProbeDoesNotInventFailureFromDumbParentTerminal(t *testing.T) {
	directory := t.TempDir()
	writeProbeStub(t, directory, "codex", `
if [ "$1" = "--version" ]; then printf 'codex-cli 0.149.0\n'; exit 0; fi
if [ "$1" = "doctor" ] && [ "$2" = "--help" ]; then printf 'usage: codex doctor\n'; exit 0; fi
if [ "$1" = "doctor" ] && [ "$2" = "--summary" ]; then
  if [ "$TERM" = "dumb" ]; then printf 'TERM=dumb\n'; exit 1; fi
  printf 'healthy\n'; exit 0
fi
exit 2`)
	t.Setenv("PATH", directory)
	t.Setenv("TERM", "dumb")
	entry := Entry{
		Name: "codex", Command: "codex", Required: true,
		VersionArgs: []string{"--version"}, Parse: firstVersion,
		SelfDoctorArgs: []string{"doctor", "--summary", "--ascii", "--no-color"},
	}
	result := Probe(context.Background(), []Entry{entry}, ProbeOptions{GOOS: "linux"})[0]
	if result.State != StateOK || result.SelfDoctor != "ok" {
		t.Fatalf("result=%#v, want a healthy engine independent of PFM's non-interactive TERM", result)
	}
}

func writeProbeStub(t *testing.T, directory, name, body string) {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
}
