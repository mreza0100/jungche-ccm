package archive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/paths"
)

// fakeHides is the hidden set with an audit trail of what was retired.
type fakeHides struct {
	rows     []HiddenChat
	unhidden []string
	failOn   string
}

func (hides *fakeHides) Hidden(context.Context) ([]HiddenChat, error) {
	return hides.rows, nil
}

func (hides *fakeHides) Unhide(_ context.Context, id string) error {
	if id == hides.failOn {
		return errors.New("store is busy")
	}
	hides.unhidden = append(hides.unhidden, id)
	return nil
}

// emptyProc reports no processes: the fixtures declare liveness through the
// sid crumbs instead, which is the reading that does not need a fake /proc.
type emptyProc struct{}

func (emptyProc) PIDs() ([]int, error)                   { return nil, nil }
func (emptyProc) Cmdline(int) ([]string, error)          { return nil, nil }
func (emptyProc) Environ(int) (map[string]string, error) { return nil, nil }
func (emptyProc) FDLinks(int) ([]gather.FDLink, error)   { return nil, nil }
func (emptyProc) Stat(int) (gather.ProcStat, error)      { return gather.ProcStat{}, nil }

func archiveJail(t *testing.T) paths.Values {
	t.Helper()
	root := t.TempDir()
	values := paths.Values{
		Home:          filepath.Join(root, "home"),
		ClaudeRoots:   []string{filepath.Join(root, "home", ".cc", "1", "projects")},
		CodexRoot:     filepath.Join(root, "home", ".codex"),
		SIDDir:        filepath.Join(root, "sid"),
		HiddenCarrier: filepath.Join(root, "home", ".claude", ".cc-ls-hidden"),
		ArchiveDir:    filepath.Join(root, "home", ".claude-archive"),
		TmuxDir:       filepath.Join(root, "tmux"),
		ProcRoot:      filepath.Join(root, "proc"),
	}
	for _, directory := range []string{
		filepath.Join(values.ClaudeRoots[0], "-home-user-work-x"),
		filepath.Join(values.CodexRoot, "sessions", "2026", "08", "15"),
		values.SIDDir,
		filepath.Join(values.Home, ".claude"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return values
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The whole hidden-chat contract in one run: the dry run moves nothing, the
// apply moves exactly the resolvable dead chats, a LIVE hidden chat is left on
// disk, and every decided id leaves the hidden list — including the live one,
// which belongs back in the picker.
func TestArchiveMovesOnlyResolvableDeadHiddenChats(t *testing.T) {
	values := archiveJail(t)
	const (
		deadID   = "11111111-1111-4111-8111-111111111111"
		liveID   = "22222222-2222-4222-8222-222222222222"
		ghostID  = "33333333-3333-4333-8333-333333333333"
		codexID  = "44444444-4444-4444-8444-444444444444"
		rollout  = "rollout-2026-08-15T10-00-00-44444444-4444-4444-8444-444444444444.jsonl"
		project  = "-home-user-work-x"
		sidecars = "history"
	)
	_ = sidecars
	transcripts := filepath.Join(values.ClaudeRoots[0], project)
	writeFile(t, filepath.Join(transcripts, deadID+".jsonl"), "{}\n")
	writeFile(t, filepath.Join(transcripts, liveID+".jsonl"), "{}\n")
	writeFile(
		t,
		filepath.Join(values.CodexRoot, "sessions", "2026", "08", "15", rollout),
		"{}\n",
	)
	// The live chat states itself through its socket crumb, exactly as a
	// running chat does.
	writeFile(
		t,
		filepath.Join(values.SIDDir, "cc-1800000001-42-1"),
		filepath.Join(transcripts, liveID+".jsonl")+"\n",
	)
	writeFile(
		t,
		filepath.Join(values.Home, ".claude", "history.jsonl"),
		`{"display":"one","sid":"`+deadID+`"}`+"\n"+
			`{"display":"two","sid":"`+liveID+`"}`+"\n",
	)
	writeFile(
		t,
		filepath.Join(values.CodexRoot, "session_index.jsonl"),
		`{"id":"`+codexID+`","thread_name":"gone"}`+"\n"+
			`{"id":"55555555-5555-4555-8555-555555555555","thread_name":"kept"}`+"\n",
	)
	writeFile(t, values.HiddenCarrier, deadID+"\n"+liveID+"\n")

	hides := &fakeHides{rows: []HiddenChat{
		{ID: deadID, Engine: "cc"},
		{ID: liveID, Engine: "cc"},
		{ID: ghostID, Engine: "cc"},
		{ID: codexID, Engine: "cx"},
	}}
	runner, err := New(Dependencies{Paths: values, Hides: hides, Proc: emptyProc{}})
	if err != nil {
		t.Fatal(err)
	}

	preview, err := runner.Run(context.Background(), Options{})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(preview.Moves) != 2 {
		t.Fatalf("dry run planned %d moves, want 2: %#v", len(preview.Moves), preview.Moves)
	}
	if _, err := os.Stat(filepath.Join(transcripts, deadID+".jsonl")); err != nil {
		t.Fatalf("the dry run moved a file: %v", err)
	}
	if len(hides.unhidden) != 0 {
		t.Fatalf("the dry run retired hides: %v", hides.unhidden)
	}

	report, err := runner.Run(context.Background(), Options{Apply: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(transcripts, deadID+".jsonl")); !os.IsNotExist(err) {
		t.Fatalf("the dead chat was not archived: %v", err)
	}
	if _, err := os.Stat(filepath.Join(transcripts, liveID+".jsonl")); err != nil {
		t.Fatalf("a LIVE chat's transcript was archived out from under it: %v", err)
	}
	if len(report.Live) != 1 || report.Live[0] != liveID {
		t.Fatalf("live set = %v, want [%s]", report.Live, liveID)
	}
	if len(report.Orphans) != 1 || report.Orphans[0] != ghostID {
		t.Fatalf("orphans = %v, want [%s]", report.Orphans, ghostID)
	}
	if report.Unhidden != 4 {
		t.Fatalf("hides retired = %d, want 4 (every decided id)", report.Unhidden)
	}

	archived := filepath.Join(values.ArchiveDir, "claude", project, deadID+".jsonl")
	if _, err := os.Stat(archived); err != nil {
		t.Fatalf("archived file is not where the manifest says: %v", err)
	}
	if _, err := os.Stat(
		filepath.Join(values.ArchiveDir, "codex", rollout),
	); err != nil {
		t.Fatalf("the codex rollout was not archived: %v", err)
	}

	history, err := os.ReadFile(filepath.Join(values.Home, ".claude", "history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(history), deadID) {
		t.Fatal("history.jsonl still carries the archived chat's prompts")
	}
	if !strings.Contains(string(history), liveID) {
		t.Fatal("history.jsonl lost a live chat's prompts")
	}
	index, err := os.ReadFile(filepath.Join(values.CodexRoot, "session_index.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(index), codexID) {
		t.Fatal("the codex index still points at a moved rollout")
	}
	if !strings.Contains(string(index), "kept") {
		t.Fatal("the codex index lost an untouched row")
	}
	if len(report.SidecarBackups) != 3 {
		t.Fatalf("sidecar backups = %v, want three files copied", report.SidecarBackups)
	}
}

// A second run finds nothing left to do — an archive you cannot re-run is a
// tool you have to remember the state of.
func TestArchiveIsIdempotent(t *testing.T) {
	values := archiveJail(t)
	const id = "11111111-1111-4111-8111-111111111111"
	transcripts := filepath.Join(values.ClaudeRoots[0], "-p")
	writeFile(t, filepath.Join(transcripts, id+".jsonl"), "{}\n")
	hides := &fakeHides{rows: []HiddenChat{{ID: id, Engine: "cc"}}}
	runner, err := New(Dependencies{Paths: values, Hides: hides, Proc: emptyProc{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background(), Options{Apply: true}); err != nil {
		t.Fatal(err)
	}
	hides.rows = nil
	report, err := runner.Run(context.Background(), Options{Apply: true})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(report.Moves) != 0 {
		t.Fatalf("second run planned %d moves", len(report.Moves))
	}
}

// Restore is the whole reason this is an archive and not a delete.
func TestRestorePutsAChatBackWhereItWas(t *testing.T) {
	values := archiveJail(t)
	const id = "11111111-1111-4111-8111-111111111111"
	transcripts := filepath.Join(values.ClaudeRoots[0], "-p")
	original := filepath.Join(transcripts, id+".jsonl")
	writeFile(t, original, "{\"a\":1}\n")
	hides := &fakeHides{rows: []HiddenChat{{ID: id, Engine: "cc"}}}
	runner, err := New(Dependencies{Paths: values, Hides: hides, Proc: emptyProc{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background(), Options{Apply: true}); err != nil {
		t.Fatal(err)
	}
	row, err := Restore(values.ArchiveDir, id)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if row.Original != original {
		t.Fatalf("restored to %s, want %s", row.Original, original)
	}
	content, err := os.ReadFile(original)
	if err != nil {
		t.Fatalf("the restored file is not readable: %v", err)
	}
	if string(content) != "{\"a\":1}\n" {
		t.Fatalf("restored content = %q", content)
	}
	if _, err := Restore(values.ArchiveDir, "not-in-the-manifest"); err == nil {
		t.Fatal("Restore() accepted an id the manifest never recorded")
	}
}

// Subagent mode is age-gated, because a running subagent appends to its
// transcript for as long as it runs.
func TestArchiveSubagentsRespectsTheAgeGate(t *testing.T) {
	values := archiveJail(t)
	project := filepath.Join(values.ClaudeRoots[0], "-p")
	old := filepath.Join(project, "aaaaaaaa-0000-4000-8000-000000000001.jsonl")
	fresh := filepath.Join(project, "bbbbbbbb-0000-4000-8000-000000000002.jsonl")
	chat := filepath.Join(project, "cccccccc-0000-4000-8000-000000000003.jsonl")
	writeFile(t, old, `{"isSidechain":true,"type":"user"}`+"\n")
	writeFile(t, fresh, `{"isSidechain":true,"type":"user"}`+"\n")
	writeFile(t, chat, `{"type":"user","message":"hello"}`+"\n")
	past := time.Now().Add(-72 * time.Hour)
	for _, path := range []string{old, chat} {
		if err := os.Chtimes(path, past, past); err != nil {
			t.Fatal(err)
		}
	}

	hides := &fakeHides{}
	runner, err := New(Dependencies{Paths: values, Hides: hides, Proc: emptyProc{}})
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(context.Background(), Options{
		Apply:     true,
		Subagents: true,
		OlderThan: 48 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Moves) != 1 || report.Moves[0].Source != old {
		t.Fatalf("moves = %#v, want only the aged sidechain", report.Moves)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("a fresh sidechain was archived: %v", err)
	}
	if _, err := os.Stat(chat); err != nil {
		t.Fatalf("a real chat transcript was archived as a sidechain: %v", err)
	}
	if len(hides.unhidden) != 0 {
		t.Fatalf("subagent mode touched the hidden list: %v", hides.unhidden)
	}
}

// A live subagent transcript is left alone even when its file is old enough:
// liveness is re-read at run time and outranks the age gate.
func TestArchiveSubagentsSkipsLiveTranscripts(t *testing.T) {
	values := archiveJail(t)
	project := filepath.Join(values.ClaudeRoots[0], "-p")
	const id = "aaaaaaaa-0000-4000-8000-000000000001"
	path := filepath.Join(project, id+".jsonl")
	writeFile(t, path, `{"isSidechain":true}`+"\n")
	past := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(values.SIDDir, "cc-1800000001-42-1"), path+"\n")

	runner, err := New(Dependencies{
		Paths: values,
		Hides: &fakeHides{},
		Proc:  emptyProc{},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(context.Background(), Options{
		Apply:     true,
		Subagents: true,
		OlderThan: 48 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Moves) != 0 {
		t.Fatalf("a live transcript was archived: %#v", report.Moves)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the live transcript is gone: %v", err)
	}
}
