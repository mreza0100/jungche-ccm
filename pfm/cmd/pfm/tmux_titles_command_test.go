package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/paths"
)

func tmuxTitlesRuntime(t *testing.T, enabled bool) commandRuntime {
	t.Helper()
	home := t.TempDir()
	machine := pfmconfig.Defaults(home, nil)
	machine.Tmux.Titles.Enabled = enabled
	return commandRuntime{Config: machine, Paths: paths.Values{Home: home}}
}

func runTmuxTitles(runtime commandRuntime, args ...string) (string, string, int) {
	var stdout, stderr bytes.Buffer
	code := runInternalTmuxTitles(args, &stdout, &stderr, runtime)
	return stdout.String(), stderr.String(), code
}

// The shim never re-spells set-titles-string: it applies whatever pfm names,
// name first and value the rest of the line.
func TestTmuxTitlesPrintsTheOptionLinesWhenPfmOwnsTheTitle(t *testing.T) {
	stdout, stderr, code := runTmuxTitles(tmuxTitlesRuntime(t, true))
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q, want silent success", code, stderr)
	}
	want := "set-titles on\nset-titles-string " + pfmconfig.TmuxTitlesString + "\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

// Disabled prints NOTHING, so the shim applies nothing and the host's own
// title survives the spawn.
func TestTmuxTitlesPrintsNothingWhenTheHostOwnsTheTitle(t *testing.T) {
	stdout, stderr, code := runTmuxTitles(tmuxTitlesRuntime(t, false))
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("(stdout %q, stderr %q, code %d), want silent success with no options", stdout, stderr, code)
	}
}

// An unreadable config prints no stdout at all: half a line protocol would
// have the shim set one option and not the other.
func TestTmuxTitlesRefusesOnAnUnreadableConfig(t *testing.T) {
	runtime := tmuxTitlesRuntime(t, true)
	runtime.ConfigError = errors.New("boom")
	stdout, stderr, code := runTmuxTitles(runtime)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "config unreadable") {
		t.Fatalf("(stdout %q, stderr %q, code %d), want a one-line refusal", stdout, stderr, code)
	}
}

func TestTmuxTitlesRejectsArguments(t *testing.T) {
	_, stderr, code := runTmuxTitles(tmuxTitlesRuntime(t, true), "extra")
	if code != 2 || !strings.Contains(stderr, "usage:") {
		t.Fatalf("(stderr %q, code %d), want a usage refusal", stderr, code)
	}
}

// The shim's zsh loop and this command are one protocol; the option name it
// splits off is the one tmux takes. Running the printed lines against a probe
// server proves the two halves fit.
func TestTmuxTitlesLinesApplyToARealServer(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := t.TempDir()
	socketPath := filepath.Join(root, "probe-titles.sock")
	start := exec.Command(
		"tmux", "-S", socketPath, "-f", "/dev/null",
		"new-session", "-d", "-s", "probe-titles", "sleep 120",
	)
	if output, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start probe server: %v: %s", err, output)
	}
	t.Cleanup(func() { _ = exec.Command("tmux", "-S", socketPath, "kill-server").Run() })

	stdout, _, code := runTmuxTitles(tmuxTitlesRuntime(t, true))
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		name, value, found := strings.Cut(line, " ")
		if !found {
			t.Fatalf("line %q carries no value", line)
		}
		if output, err := exec.Command(
			"tmux", "-S", socketPath, "set", "-g", name, value,
		).CombinedOutput(); err != nil {
			t.Fatalf("tmux set -g %s: %v: %s", name, err, output)
		}
	}
	read, err := exec.Command(
		"tmux", "-S", socketPath, "show-options", "-g", "set-titles-string",
	).Output()
	if err != nil {
		t.Fatal(err)
	}
	want := `set-titles-string "` + pfmconfig.TmuxTitlesString + `"`
	if got := strings.TrimSpace(string(read)); got != want {
		t.Fatalf("set-titles-string = %q, want %q", got, want)
	}
}

// A knob nobody can read is a knob nobody has: both new keys must show up in
// `pfm config show` with their source, like every other resolved value.
func TestConfigShowDisplaysTheTmuxTitlesAndNameSyncKeys(t *testing.T) {
	home := t.TempDir()
	machine := pfmconfig.Defaults(home, nil)
	var stdout bytes.Buffer
	printResolvedConfig(&stdout, commandRuntime{Config: machine, Paths: paths.Values{Home: home}})
	for _, want := range []string{
		"config tmux.titles.enabled=true (default)",
		"config nameSync.interval=15m0s (default)",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("config show missing %q:\n%s", want, stdout.String())
		}
	}
}
