package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hostops/pfm/internal/config"
	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/paths"
)

func TestClassifySpawnSeparatesInjectedOldAndBypassed(t *testing.T) {
	const layer = int64(1_700_000_000)
	for _, testCase := range []struct {
		name        string
		observation spawnObservation
		want        spawnVerdict
		wantReason  string
	}{
		{
			name: "professor prompt file in argv",
			observation: spawnObservation{
				Argv:        []string{"claude", "--resume", "abc", "--system-prompt-file", "/p.md"},
				Environ:     map[string]string{},
				StartedUnix: layer + 60,
			},
			want:       spawnInjected,
			wantReason: "--system-prompt-file",
		},
		{
			name: "lean arm in the environment",
			observation: spawnObservation{
				Argv:        []string{"claude"},
				Environ:     map[string]string{"CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT": "1"},
				StartedUnix: layer + 60,
			},
			want:       spawnInjected,
			wantReason: "lean prompt armed",
		},
		{
			name: "flagless chat older than the layer",
			observation: spawnObservation{
				Argv:        []string{"claude"},
				Environ:     map[string]string{},
				StartedUnix: layer - 3600,
			},
			want:       spawnPredatesLayer,
			wantReason: "before the prompt layer was staged",
		},
		{
			name: "flagless resume with no usable age signal",
			observation: spawnObservation{
				Argv:    []string{"claude", "--resume", "abc"},
				Environ: map[string]string{},
			},
			want:       spawnPredatesLayer,
			wantReason: "reborn before the door",
		},
		{
			name: "fresh flagless launch",
			observation: spawnObservation{
				Argv:        []string{"claude", "--name", "seat"},
				Environ:     map[string]string{},
				StartedUnix: layer + 3600,
			},
			want:       spawnViolation,
			wantReason: "bypassed the door",
		},
		{
			// The lean arm lives only in the environment, so an unreadable
			// environment cannot clear a seat. It must not read as clean.
			name: "fresh flagless launch with an unreadable environment",
			observation: spawnObservation{
				Argv:        []string{"claude", "--name", "seat"},
				EnvironErr:  errors.New("permission denied"),
				StartedUnix: layer + 3600,
			},
			want:       spawnViolation,
			wantReason: "verdict unproven",
		},
		{
			// An absent stamp must never turn an ordinary fresh chat into a
			// "predates the layer" excuse.
			name: "no layer stamp leaves the argv verdict standing",
			observation: spawnObservation{
				Argv:        []string{"claude", "--name", "seat"},
				Environ:     map[string]string{},
				StartedUnix: layer - 3600,
			},
			want:       spawnViolation,
			wantReason: "bypassed the door",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			stamp := layer
			if testCase.name == "no layer stamp leaves the argv verdict standing" {
				stamp = 0
			}
			verdict, reason := classifySpawn(testCase.observation, stamp)
			if verdict != testCase.want {
				t.Fatalf("classifySpawn = %s (%s), want %s", verdict, reason, testCase.want)
			}
			if !strings.Contains(reason, testCase.wantReason) {
				t.Fatalf("reason %q does not name the deciding signal %q", reason, testCase.wantReason)
			}
		})
	}
}

// Production configures no prompt material, so an audit that classified every
// seat would report a fleet-wide violation. The check must say it has nothing
// to assert instead — and must never print the clean-audit wording.
func TestSpawnAuditIsInertUnderProductionPolicy(t *testing.T) {
	var stdout bytes.Buffer
	machine := config.Config{Claude: config.ClaudePrefs{SystemPrompt: config.SystemPromptProduction}}
	if warnings := printSpawnAuditDoctor(context.Background(), &stdout, paths.Values{}, machine, 1); warnings != 0 {
		t.Fatalf("production policy warned %d times: %q", warnings, stdout.String())
	}
	line := stdout.String()
	if !strings.Contains(line, "nothing to audit") {
		t.Fatalf("production line = %q", line)
	}
	if strings.Contains(line, "VIOLATION") || strings.Contains(line, "no live Claude chats found") {
		t.Fatalf("production policy borrowed an audited verdict: %q", line)
	}
}

// A tmux directory that cannot be read is a FAILED audit, never an empty one.
func TestSpawnAuditFailureNeverReadsAsNoChats(t *testing.T) {
	var stdout bytes.Buffer
	machine := config.Config{Claude: config.ClaudePrefs{SystemPrompt: config.SystemPromptProfessor}}
	resolved := paths.Values{Home: t.TempDir(), TmuxDir: filepath.Join(t.TempDir(), "not-a-directory")}
	if err := os.WriteFile(resolved.TmuxDir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	warnings := printSpawnAuditDoctor(context.Background(), &stdout, resolved, machine, 1)
	line := stdout.String()
	if warnings == 0 {
		t.Fatalf("unreadable socket directory reported no warning: %q", line)
	}
	if !strings.Contains(line, "CHECK FAILED to run") {
		t.Fatalf("failed audit line = %q", line)
	}
	if strings.Contains(line, "no live Claude chats found") {
		t.Fatalf("a failed audit rendered as an empty one: %q", line)
	}
}

// fakeProcFS serves a hand-built process table to the resolver tests.
type fakeProcFS struct {
	cmdlines map[int][]string
	parents  map[int]int
}

func (fake fakeProcFS) PIDs() ([]int, error) {
	pids := make([]int, 0, len(fake.cmdlines))
	for pid := range fake.cmdlines {
		pids = append(pids, pid)
	}
	return pids, nil
}

func (fake fakeProcFS) Cmdline(pid int) ([]string, error) {
	argv, ok := fake.cmdlines[pid]
	if !ok {
		return nil, errors.New("no such process")
	}
	return argv, nil
}

func (fake fakeProcFS) Environ(int) (map[string]string, error) {
	return map[string]string{}, nil
}

func (fake fakeProcFS) FDLinks(int) ([]gather.FDLink, error) { return nil, nil }

func (fake fakeProcFS) Stat(pid int) (gather.ProcStat, error) {
	if _, ok := fake.cmdlines[pid]; !ok {
		return gather.ProcStat{}, errors.New("no such process")
	}
	return gather.ProcStat{ParentPID: fake.parents[pid]}, nil
}

// The claude launcher execs the version-named file
// (~/.local/share/claude/versions/2.1.250), so a live seat's argv[0] basename
// is a version string, never "claude". The resolver must identify it through
// the fleet's one matcher or the audit reports a live fleet as empty.
func TestResolveClaudeProcessMatchesVersionNamedBinary(t *testing.T) {
	proc := fakeProcFS{
		cmdlines: map[int][]string{
			100: {"zsh"},
			101: {"/srv/seat/.local/share/claude/versions/2.1.250", "--system-prompt-file", "/p.md"},
		},
		parents: map[int]int{101: 100},
	}
	pid, argv, found, err := resolveClaudeProcess(proc, 100, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("version-named claude binary was not recognized — a live seat renders as an empty pane")
	}
	if pid != 101 {
		t.Fatalf("resolved pid = %d, want 101", pid)
	}
	if len(argv) == 0 || argv[0] != "/srv/seat/.local/share/claude/versions/2.1.250" {
		t.Fatalf("resolved argv = %q", argv)
	}
}
