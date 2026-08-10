package gather

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestRealProcFSSmokeOwnProcessOnly(t *testing.T) {
	proc := RealProcFS{}
	pid := os.Getpid()

	cmdline, err := proc.Cmdline(pid)
	if err != nil {
		t.Fatalf("Cmdline(own pid) error = %v", err)
	}
	if len(cmdline) == 0 {
		t.Fatal("Cmdline(own pid) is empty")
	}
	if _, err := proc.Environ(pid); err != nil {
		t.Fatalf("Environ(own pid) error = %v", err)
	}
	if _, err := proc.FDLinks(pid); err != nil {
		t.Fatalf("FDLinks(own pid) error = %v", err)
	}
	stat, err := proc.Stat(pid)
	if err != nil {
		t.Fatalf("Stat(own pid) error = %v", err)
	}
	if stat.ParentPID <= 0 || stat.StartTime == 0 {
		t.Fatalf("Stat(own pid) = %+v, want parent and start time", stat)
	}
	birth, err := proc.Birth(pid)
	if err != nil {
		t.Fatalf("Birth(own pid) error = %v", err)
	}
	if birth <= 0 || birth > time.Now().Unix() {
		t.Fatalf("Birth(own pid) = %d, want a past epoch second", birth)
	}
}

func TestDetectCodexNumericFDAndAncestorMatching(t *testing.T) {
	codexRoot := "/jail/codex"
	first := filepath.Join(codexRoot, "sessions", "2026", "rollout-first.jsonl")
	second := filepath.Join(codexRoot, "sessions", "2026", "rollout-second.jsonl")
	proc := &fakeProcFS{processes: map[int]fakeProcess{
		100: {stat: ProcStat{ParentPID: 1}},
		200: {stat: ProcStat{ParentPID: 100}},
		300: {stat: ProcStat{ParentPID: 200}},
		400: {
			cmdline: []string{"/usr/bin/codex"},
			fdLinks: []FDLink{
				{FD: 8, Target: second},
				{FD: 3, Target: first},
				{FD: 2, Target: "/outside/rollout-ignore.jsonl"},
			},
			stat: ProcStat{ParentPID: 300},
		},
		401: {
			cmdline: []string{"/usr/bin/not-codex"},
			fdLinks: []FDLink{{FD: 1, Target: first}},
		},
	}}
	panes := []Pane{{Socket: "cx-1-2-3", PaneID: "%4", PID: 100}}

	got, err := DetectCodex(proc, codexRoot, panes)
	if err != nil {
		t.Fatalf("DetectCodex() error = %v", err)
	}
	want := []LiveCodex{{
		PID:         400,
		PanePID:     100,
		Socket:      "cx-1-2-3",
		PaneID:      "%4",
		RolloutPath: first,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DetectCodex() = %#v, want %#v", got, want)
	}
}

func TestDetectAgentsAndCache1H(t *testing.T) {
	home := "/jail/home"
	sessionOne := "01234567-89ab-cdef-0123-456789abcdef"
	sessionTwo := "fedcba98-7654-3210-fedc-ba9876543210"
	proc := &fakeProcFS{processes: map[int]fakeProcess{
		500: {stat: ProcStat{ParentPID: 1}},
		700: {
			cmdline: []string{
				"/opt/claude",
				"--session-id",
				sessionOne,
				"--resume",
				"/transcripts/" + sessionTwo + ".jsonl",
			},
			environ: map[string]string{
				"CLAUDE_CONFIG_DIR":        "/jail/home/.cc/2",
				"ENABLE_PROMPT_CACHING_1H": "1",
			},
			stat: ProcStat{ParentPID: 500, StartTime: 99},
		},
		701: {
			cmdline: []string{"/opt/claude", "--resume", sessionOne},
			environ: map[string]string{
				"CLAUDE_CONFIG_DIR":        filepath.Join(home, ".claude"),
				"ENABLE_PROMPT_CACHING_1H": "1",
			},
			stat: ProcStat{ParentPID: 500, StartTime: 100},
		},
		702: {
			cmdline: []string{"/opt/claude", "--resume", "../../not-a-uuid"},
			environ: map[string]string{
				"CLAUDE_CONFIG_DIR": "/jail/home/.cc/3",
			},
			stat: ProcStat{ParentPID: 500},
		},
	}}
	panes := []Pane{{Socket: "cc-1-2-3", PaneID: "%5", PID: 500}}

	agents, err := DetectAgents(proc, home, panes)
	if err != nil {
		t.Fatalf("DetectAgents() error = %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("DetectAgents() = %#v, want two strict IDs", agents)
	}
	if agents[0].SessionID != sessionOne ||
		agents[1].SessionID != sessionTwo ||
		agents[0].ConfigDir != "/jail/home/.cc/2" ||
		agents[0].Socket != "cc-1-2-3" ||
		agents[0].StartTime != 99 {
		t.Fatalf("DetectAgents() = %#v", agents)
	}

	cacheSockets, err := DetectCache1H(proc, panes)
	if err != nil {
		t.Fatalf("DetectCache1H() error = %v", err)
	}
	if !reflect.DeepEqual(cacheSockets, []string{"cc-1-2-3"}) {
		t.Fatalf("DetectCache1H() = %q, want cc socket", cacheSockets)
	}

	claudeProcesses, err := DetectClaudeProcesses(proc, panes)
	if err != nil {
		t.Fatalf("DetectClaudeProcesses() error = %v", err)
	}
	if len(claudeProcesses) != 3 {
		t.Fatalf(
			"DetectClaudeProcesses() = %#v, want every Claude process",
			claudeProcesses,
		)
	}
	for _, process := range claudeProcesses {
		if process.Socket != "cc-1-2-3" ||
			process.PaneID != "%5" ||
			process.PanePID != 500 {
			t.Fatalf("DetectClaudeProcesses() process = %#v", process)
		}
	}
}

func TestWindowConvergenceClipsRunesOnlyHere(t *testing.T) {
	longName := strings.Repeat("界", 25)
	panes := []Pane{{
		Socket:      "cx-1-2-3",
		SessionName: "cx-session",
		WindowID:    "@1",
		WindowName:  "old",
		PaneID:      "%1",
	}}
	codex := []LiveCodex{{
		Socket:      "cx-1-2-3",
		PaneID:      "%1",
		RolloutPath: "/codex/rollout.jsonl",
	}}

	renames := computeWindowRenames(panes, codex, func(string) string { return longName })
	if len(renames) != 1 {
		t.Fatalf("computeWindowRenames() = %#v, want one", renames)
	}
	if got, want := renames[0].TargetName, strings.Repeat("界", 24); got != want {
		t.Fatalf("TargetName = %q, want %q", got, want)
	}
	if !utf8.ValidString(renames[0].TargetName) {
		t.Fatalf("TargetName is invalid UTF-8: %x", renames[0].TargetName)
	}
}
