package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hostops/pfm/internal/action"
	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/paths"
)

func promptArgsRuntime(t *testing.T, mode string) commandRuntime {
	t.Helper()
	home := t.TempDir()
	machine := pfmconfig.Defaults(home, nil)
	machine.Claude.SystemPrompt = mode
	return commandRuntime{Config: machine, Paths: paths.Values{Home: home}}
}

func runPromptArgs(runtime commandRuntime, args ...string) (string, string, int) {
	var stdout, stderr bytes.Buffer
	code := runInternalPromptArgs(args, &stdout, &stderr, runtime)
	return stdout.String(), stderr.String(), code
}

func TestPromptArgsProductionPrintsNothing(t *testing.T) {
	for _, mode := range []string{"", pfmconfig.SystemPromptProduction} {
		stdout, stderr, code := runPromptArgs(promptArgsRuntime(t, mode), "1")
		if code != 0 || stdout != "" || stderr != "" {
			t.Fatalf("mode %q = (stdout %q, stderr %q, code %d), want silent success", mode, stdout, stderr, code)
		}
	}
}

func TestPromptArgsLeanPrintsTheEnvLine(t *testing.T) {
	stdout, stderr, code := runPromptArgs(promptArgsRuntime(t, pfmconfig.SystemPromptLean), "1")
	if code != 0 || stderr != "" || stdout != "env CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT=1\n" {
		t.Fatalf("lean = (stdout %q, stderr %q, code %d)", stdout, stderr, code)
	}
}

func TestPromptArgsProfessorPrintsTheFlagPair(t *testing.T) {
	runtime := promptArgsRuntime(t, pfmconfig.SystemPromptProfessor)
	staged := action.ProfessorPromptPath(runtime.Paths.Home)
	if err := os.MkdirAll(filepath.Dir(staged), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("prompt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runPromptArgs(runtime, "1")
	if code != 0 || stderr != "" {
		t.Fatalf("professor = (stderr %q, code %d), want success", stderr, code)
	}
	if stdout != "arg --system-prompt-file\narg "+staged+"\n" {
		t.Fatalf("professor stdout = %q, want the two-line flag pair", stdout)
	}
}

func TestPromptArgsProfessorMissingStagedFileFailsWithoutPartialOutput(t *testing.T) {
	stdout, stderr, code := runPromptArgs(promptArgsRuntime(t, pfmconfig.SystemPromptProfessor), "1")
	if code == 0 {
		t.Fatal("missing staged prompt must exit nonzero")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want EMPTY on failure — a partial pair would inject half a flag", stdout)
	}
	if !strings.Contains(stderr, "run pfm install") || !strings.Contains(stderr, "professor prompt") {
		t.Fatalf("stderr = %q, want the one-line staged-file reason", stderr)
	}
}

func TestPromptArgsConfigErrorFailsClosed(t *testing.T) {
	runtime := promptArgsRuntime(t, pfmconfig.SystemPromptLean)
	runtime.ConfigError = errors.New("mangled json")
	stdout, stderr, code := runPromptArgs(runtime, "1")
	if code == 0 || stdout != "" || !strings.Contains(stderr, "config unreadable") {
		t.Fatalf("config error = (stdout %q, stderr %q, code %d), want nonzero + reason + no output", stdout, stderr, code)
	}
}
