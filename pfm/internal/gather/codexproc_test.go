package gather

import (
	"path/filepath"
	"reflect"
	"testing"

	"hostops/pfm/internal/resolve"
)

// A Codex session that writes no rollout file holds no rollout descriptor
// either, so identity has to come from the state store. Without a resolver
// the process stays invisible, which is the pre-0.146 behavior every other
// caller still gets.
func TestDetectCodexThreadsIdentifiesSessionsWithoutRolloutFiles(t *testing.T) {
	codexRoot := "/jail/codex"
	declared := filepath.Join(
		codexRoot,
		"sessions",
		"2026",
		"rollout-2026-01-01T00-00-00-paginated.jsonl",
	)
	proc := &fakeProcFS{processes: map[int]fakeProcess{
		100: {stat: ProcStat{ParentPID: 1}},
		400: {
			cmdline: []string{"/usr/bin/codex"},
			environ: map[string]string{resolve.CodexThreadEnv: "paginated"},
			stat:    ProcStat{ParentPID: 100},
			birth:   1700,
		},
	}}
	panes := []Pane{{
		Socket:      "cx-1-2-3",
		PaneID:      "%4",
		PID:         100,
		CurrentPath: "/work/paginated",
	}}

	if invisible, err := DetectCodex(proc, codexRoot, panes); err != nil ||
		len(invisible) != 0 {
		t.Fatalf("DetectCodex() = %#v, error = %v; want no rollout-less rows", invisible, err)
	}

	var gotExported, gotCWD string
	var gotBirth int64
	got, err := DetectCodexThreads(
		proc,
		codexRoot,
		panes,
		func(exported, cwd string, birth int64, _, _ string) (string, string) {
			gotExported, gotCWD, gotBirth = exported, cwd, birth
			return "paginated", declared
		},
	)
	if err != nil {
		t.Fatalf("DetectCodexThreads() error = %v", err)
	}
	want := []LiveCodex{{
		PID:         400,
		PanePID:     100,
		Socket:      "cx-1-2-3",
		PaneID:      "%4",
		RolloutPath: declared,
		ThreadID:    "paginated",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DetectCodexThreads() = %#v, want %#v", got, want)
	}
	if gotExported != "paginated" ||
		gotCWD != "/work/paginated" ||
		gotBirth != 1700 {
		t.Fatalf(
			"resolver received (%q, %q, %d), want the exported identity, pane directory, and birth",
			gotExported,
			gotCWD,
			gotBirth,
		)
	}
}

// A pane resumed through the picker holds no rollout descriptor and exports
// no thread id, and its thread was created hours before the process — nothing
// a clock or a directory can reach. Its argv states the thread outright, and
// that is enough to make it a live chat with no state store consulted at all.
func TestDetectCodexThreadsIdentifiesResumedSessionsByArgv(t *testing.T) {
	const resumed = "019feb1b-1215-7f30-b18b-e227e5ca26e5"
	proc := &fakeProcFS{processes: map[int]fakeProcess{
		100: {stat: ProcStat{ParentPID: 1}},
		400: {
			cmdline: []string{
				"/opt/codex",
				"--dangerously-bypass-approvals-and-sandbox",
				"resume",
				resumed,
			},
			stat:  ProcStat{ParentPID: 100},
			birth: 1786403951,
		},
	}}
	panes := []Pane{{
		Socket:      "cx-1-2-3",
		PaneID:      "%4",
		PID:         100,
		CurrentPath: "/work/proja",
	}}

	live, err := DetectCodex(proc, "/jail/codex", panes)
	if err != nil {
		t.Fatalf("DetectCodex() error = %v", err)
	}
	want := []LiveCodex{{
		PID:      400,
		PanePID:  100,
		Socket:   "cx-1-2-3",
		PaneID:   "%4",
		ThreadID: resumed,
	}}
	if !reflect.DeepEqual(live, want) {
		t.Fatalf("DetectCodex() = %#v, want %#v", live, want)
	}
}

// CODEX_THREAD_ID is INHERITED: a `codex resume` launched from inside another
// Codex session's shell carries the parent's thread in its environment and its
// own in its argv. The argv is what this process is actually running.
func TestDetectCodexThreadsPrefersResumeArgvOverAnInheritedEnvironment(t *testing.T) {
	const resumed = "019feb1b-1215-7f30-b18b-e227e5ca26e5"
	proc := &fakeProcFS{processes: map[int]fakeProcess{
		100: {stat: ProcStat{ParentPID: 1}},
		400: {
			cmdline: []string{"/opt/codex", "resume", resumed},
			environ: map[string]string{resolve.CodexThreadEnv: "the-parent-thread"},
			stat:    ProcStat{ParentPID: 100},
		},
	}}
	panes := []Pane{{Socket: "cx-1-2-3", PaneID: "%4", PID: 100}}

	var asked string
	live, err := DetectCodexThreads(
		proc,
		"/jail/codex",
		panes,
		func(exported, _ string, _ int64, _, _ string) (string, string) {
			asked = exported
			return exported, ""
		},
	)
	if err != nil {
		t.Fatalf("DetectCodexThreads() error = %v", err)
	}
	if asked != resumed {
		t.Fatalf("resolver received %q, want the argv thread %q", asked, resumed)
	}
	if len(live) != 1 || live[0].ThreadID != resumed {
		t.Fatalf("DetectCodexThreads() = %#v, want the argv thread", live)
	}
}

// `codex resume --last` and `codex resume <name>` name no thread here, and a
// bare `resume` at the end of argv names nothing either.
func TestCodexResumeArgvTakesOnlyAUUID(t *testing.T) {
	const resumed = "019feb1b-1215-7f30-b18b-e227e5ca26e5"
	tests := []struct {
		name    string
		cmdline []string
		want    string
	}{
		{name: "no arguments", cmdline: []string{"/opt/codex"}},
		{name: "last", cmdline: []string{"/opt/codex", "resume", "--last"}},
		{name: "by name", cmdline: []string{"/opt/codex", "resume", "BUILDER_WF"}},
		{name: "trailing resume", cmdline: []string{"/opt/codex", "resume"}},
		{
			name:    "uuid",
			cmdline: []string{"/opt/codex", "resume", resumed},
			want:    resumed,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := codexResumeArgv(test.cmdline); got != test.want {
				t.Fatalf("codexResumeArgv() = %q, want %q", got, test.want)
			}
		})
	}
}

// An unidentifiable codex process — the app-server daemon, for instance —
// never becomes a live chat row.
func TestDetectCodexThreadsSkipsUnidentifiedProcesses(t *testing.T) {
	codexRoot := "/jail/codex"
	proc := &fakeProcFS{processes: map[int]fakeProcess{
		100: {stat: ProcStat{ParentPID: 1}},
		400: {
			cmdline: []string{"/usr/bin/codex"},
			stat:    ProcStat{ParentPID: 100},
			birth:   1700,
		},
	}}
	panes := []Pane{{Socket: "cx-1-2-3", PaneID: "%4", PID: 100}}

	live, err := DetectCodexThreads(
		proc,
		codexRoot,
		panes,
		func(string, string, int64, string, string) (string, string) { return "", "" },
	)
	if err != nil {
		t.Fatalf("DetectCodexThreads() error = %v", err)
	}
	if len(live) != 0 {
		t.Fatalf("DetectCodexThreads() = %#v, want no rows", live)
	}
}

// A session holding a rollout descriptor keeps its file identity and gains the
// exported thread id, without consulting the state store at all.
func TestDetectCodexThreadsKeepsRolloutDescriptorIdentity(t *testing.T) {
	codexRoot := "/jail/codex"
	rollout := filepath.Join(codexRoot, "sessions", "2026", "rollout-live.jsonl")
	proc := &fakeProcFS{processes: map[int]fakeProcess{
		100: {stat: ProcStat{ParentPID: 1}},
		400: {
			cmdline: []string{"/usr/bin/codex"},
			environ: map[string]string{resolve.CodexThreadEnv: "live-thread"},
			fdLinks: []FDLink{{FD: 3, Target: rollout}},
			stat:    ProcStat{ParentPID: 100},
		},
	}}
	panes := []Pane{{Socket: "cx-1-2-3", PaneID: "%4", PID: 100}}

	live, err := DetectCodexThreads(
		proc,
		codexRoot,
		panes,
		func(string, string, int64, string, string) (string, string) {
			t.Fatal("the resolver was consulted for a session holding a rollout descriptor")
			return "", ""
		},
	)
	if err != nil {
		t.Fatalf("DetectCodexThreads() error = %v", err)
	}
	want := []LiveCodex{{
		PID:         400,
		PanePID:     100,
		Socket:      "cx-1-2-3",
		PaneID:      "%4",
		RolloutPath: rollout,
		ThreadID:    "live-thread",
	}}
	if !reflect.DeepEqual(live, want) {
		t.Fatalf("DetectCodexThreads() = %#v, want %#v", live, want)
	}
}

func TestDetectCodexThreadsMatchesRolloutsUnderEveryConfiguredRoot(t *testing.T) {
	roots := []string{"/jail/codex-1", "/jail/codex-2"}
	rollout := filepath.Join(roots[1], "sessions", "2026", "rollout-second.jsonl")
	proc := &fakeProcFS{processes: map[int]fakeProcess{
		100: {stat: ProcStat{ParentPID: 1}},
		400: {
			cmdline: []string{"/usr/bin/codex"},
			fdLinks: []FDLink{{FD: 9, Target: rollout}},
			stat:    ProcStat{ParentPID: 100},
		},
	}}
	panes := []Pane{{Socket: "cx-1-2-3", PaneID: "%4", PID: 100}}
	live, err := DetectCodexThreadsInRoots(proc, roots, panes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 1 || live[0].RolloutPath != rollout {
		t.Fatalf("multi-root live Codex=%#v, want rollout %q", live, rollout)
	}
}
