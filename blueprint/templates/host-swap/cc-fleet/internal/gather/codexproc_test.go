package gather

import (
	"path/filepath"
	"reflect"
	"testing"

	"hostops/cc-fleet/internal/resolve"
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
		func(exported, cwd string, birth int64) (string, string) {
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
		func(string, string, int64) (string, string) { return "", "" },
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
		func(string, string, int64) (string, string) {
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
