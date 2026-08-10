package hide

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"hostops/cc-fleet/internal/compose"
	"hostops/cc-fleet/internal/gather"
	"hostops/cc-fleet/internal/index"
	"hostops/cc-fleet/internal/store"
)

type fakeProc struct {
	pids    []int
	cmdline map[int][]string
	links   map[int][]gather.FDLink
	stats   map[int]gather.ProcStat
	environ map[int]map[string]string
	pidsErr error
}

func (proc *fakeProc) PIDs() ([]int, error) {
	return append([]int(nil), proc.pids...), proc.pidsErr
}

func (proc *fakeProc) Cmdline(pid int) ([]string, error) {
	if value, found := proc.cmdline[pid]; found {
		return append([]string(nil), value...), nil
	}
	return nil, os.ErrNotExist
}

func (proc *fakeProc) Environ(pid int) (map[string]string, error) {
	if value, found := proc.environ[pid]; found {
		return value, nil
	}
	return nil, os.ErrNotExist
}

func (proc *fakeProc) FDLinks(pid int) ([]gather.FDLink, error) {
	if value, found := proc.links[pid]; found {
		return append([]gather.FDLink(nil), value...), nil
	}
	return nil, os.ErrNotExist
}

func (proc *fakeProc) Stat(pid int) (gather.ProcStat, error) {
	if value, found := proc.stats[pid]; found {
		return value, nil
	}
	return gather.ProcStat{}, os.ErrNotExist
}

type fakeTmux struct {
	mutex         sync.Mutex
	panePID       int
	existsFor     int
	existsCalls   int
	sent          []string
	killedPanes   []string
	killedServers []string
}

func (tmux *fakeTmux) PanePID(
	_ context.Context,
	_, _ string,
) (int, error) {
	if tmux.panePID == 0 {
		return 0, errors.New("no pane pid")
	}
	return tmux.panePID, nil
}

func (tmux *fakeTmux) PaneExists(
	_ context.Context,
	_, _ string,
) bool {
	tmux.mutex.Lock()
	defer tmux.mutex.Unlock()
	tmux.existsCalls++
	return tmux.existsCalls <= tmux.existsFor
}

func (tmux *fakeTmux) SendLine(
	_ context.Context,
	_, _ string,
	line string,
) error {
	tmux.mutex.Lock()
	defer tmux.mutex.Unlock()
	tmux.sent = append(tmux.sent, line)
	return nil
}

func (tmux *fakeTmux) KillPane(
	_ context.Context,
	socketPath, paneID string,
) error {
	tmux.mutex.Lock()
	defer tmux.mutex.Unlock()
	tmux.killedPanes = append(
		tmux.killedPanes,
		filepath.Base(socketPath)+"\t"+paneID,
	)
	return nil
}

func (tmux *fakeTmux) KillServer(
	_ context.Context,
	socketPath string,
) error {
	tmux.mutex.Lock()
	defer tmux.mutex.Unlock()
	tmux.killedServers = append(tmux.killedServers, filepath.Base(socketPath))
	return nil
}

type captureSpawner struct {
	args []ExitArgs
}

func (spawner *captureSpawner) Spawn(
	_ context.Context,
	args ExitArgs,
) error {
	spawner.args = append(spawner.args, args)
	return nil
}

type refreshFunc func(context.Context) error

func (function refreshFunc) Refresh(ctx context.Context) error {
	return function(ctx)
}

func TestManagerIdentifiesClaudeAndCodexSelf(t *testing.T) {
	jail := newHideJail(t)
	database := jail.open(t)
	defer database.Close()
	ctx := context.Background()

	claudeID := "11111111-1111-4111-8111-111111111111"
	claudePath := filepath.Join(jail.claudeRoot, claudeID+".jsonl")
	if err := database.UpsertTranscript(ctx, store.Transcript{
		UUID:        claudeID,
		Path:        claudePath,
		PromptCount: 7,
	}); err != nil {
		t.Fatal(err)
	}
	codexID := "22222222-2222-4222-8222-222222222222"
	rolloutPath := filepath.Join(
		jail.codexRoot,
		"sessions",
		"2026",
		"07",
		"rollout-2026-07-27T10-00-00-"+codexID+".jsonl",
	)
	if err := database.UpsertRollout(ctx, store.Rollout{
		ID:          codexID,
		Path:        rolloutPath,
		PromptCount: 9,
		UserThread:  true,
	}); err != nil {
		t.Fatal(err)
	}

	tmux := &fakeTmux{panePID: 10}
	proc := &fakeProc{
		pids:    []int{20},
		cmdline: map[int][]string{20: {"codex"}},
		links: map[int][]gather.FDLink{
			20: {{FD: 12, Target: rolloutPath}},
		},
		stats:   map[int]gather.ProcStat{20: {ParentPID: 10}},
		environ: map[int]map[string]string{},
	}
	spawner := &captureSpawner{}
	now := time.Unix(1_700_000_000, 0)
	manager, err := New(database, Dependencies{
		ProcFS:  proc,
		Tmux:    tmux,
		Spawner: spawner,
		Now:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	socketPath := filepath.Join(jail.tmuxDir, "cc-100-1-1")
	target, err := manager.Hide(ctx, Request{
		Self: true,
		Environment: SelfEnvironment{
			TMUX:            socketPath + ",1,0",
			TMUXPane:        "%3",
			ClaudeSessionID: claudeID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.ID != claudeID || target.Engine != ClaudeEngine {
		t.Fatalf("Claude target = %#v", target)
	}
	assertHidden(t, database, claudeID, ClaudeEngine, now.Unix())

	cxSocket := filepath.Join(jail.tmuxDir, "cx-200-1-1")
	target, err = manager.Hide(ctx, Request{
		Self: true,
		Exit: true,
		Environment: SelfEnvironment{
			TMUX:     cxSocket + ",2,0",
			TMUXPane: "%8",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.ID != codexID || target.Engine != CodexEngine {
		t.Fatalf("Codex target = %#v", target)
	}
	assertHidden(t, database, codexID, CodexEngine, now.Unix())
	if len(spawner.args) != 1 ||
		spawner.args[0].ID != codexID ||
		spawner.args[0].PaneID != "%8" ||
		spawner.args[0].SocketName != "cx-200-1-1" {
		t.Fatalf("spawned args = %#v", spawner.args)
	}

	if err := manager.Unhide(ctx, claudeID); err != nil {
		t.Fatal(err)
	}
	if _, found, err := database.Hidden(ctx, claudeID); err != nil || found {
		t.Fatalf("Hidden(Claude) found=%v err=%v, want absent", found, err)
	}
}

func TestManagerClaudeCrumbPrecedenceWritesNullBaseline(t *testing.T) {
	jail := newHideJail(t)
	database := jail.open(t)
	defer database.Close()
	socketName := "cc-300-1-1"
	paneID := "%5"
	socketID := "33333333-3333-4333-8333-333333333333"
	paneIDValue := "44444444-4444-4444-8444-444444444444"
	writeTestFile(
		t,
		filepath.Join(jail.sidDir, socketName),
		"/jail/"+socketID+".jsonl\n",
	)
	writeTestFile(
		t,
		filepath.Join(jail.sidDir, socketName+"."+paneID),
		"/jail/"+paneIDValue+".jsonl\n",
	)
	manager, err := New(database, Dependencies{
		ProcFS:  &fakeProc{},
		Tmux:    &fakeTmux{},
		Spawner: &captureSpawner{},
		Now:     func() time.Time { return time.Unix(25, 0) },
	})
	if err != nil {
		t.Fatal(err)
	}
	target, err := manager.Hide(context.Background(), Request{
		Self: true,
		Environment: SelfEnvironment{
			TMUX:     filepath.Join(jail.tmuxDir, socketName) + ",1,0",
			TMUXPane: paneID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.ID != paneIDValue {
		t.Fatalf("crumb selected %q, want pane crumb %q", target.ID, paneIDValue)
	}
	hidden, found, err := database.Hidden(context.Background(), paneIDValue)
	if err != nil || !found {
		t.Fatalf("Hidden() found=%v err=%v", found, err)
	}
	if hidden.BaselinePrompts != nil {
		t.Fatalf("baseline = %v, want the retired column left NULL", *hidden.BaselinePrompts)
	}
}

// A hide is permanent: new prompts on the transcript never bring the chat
// back, and only an explicit unhide does.
func TestHiddenChatStaysHiddenAsItGrowsUntilUnhide(t *testing.T) {
	jail := newHideJail(t)
	database := jail.open(t)
	defer database.Close()
	ctx := context.Background()

	id := "77777777-7777-4777-8777-777777777777"
	transcriptPath := filepath.Join(jail.claudeRoot, "project-alpha", id+".jsonl")
	writeTestFile(
		t,
		transcriptPath,
		`{"type":"user","cwd":"/work/alpha","message":{"content":"first"}}`+"\n",
	)
	reindex(t, database)

	manager, err := New(database, Dependencies{
		ProcFS:  &fakeProc{},
		Tmux:    &fakeTmux{},
		Spawner: &captureSpawner{},
		Now:     func() time.Time { return time.Unix(50, 0) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !listedByDefault(t, database, id) {
		t.Fatal("indexed chat is missing from the default listing")
	}
	if _, err := manager.Hide(ctx, Request{ID: id}); err != nil {
		t.Fatal(err)
	}
	assertHidden(t, database, id, ClaudeEngine, 50)
	if listedByDefault(t, database, id) {
		t.Fatal("hidden chat is still listed")
	}

	appendTestFile(
		t,
		transcriptPath,
		`{"type":"user","cwd":"/work/alpha","message":{"content":"second"}}`+"\n"+
			`{"type":"user","cwd":"/work/alpha","message":{"content":"third"}}`+"\n",
	)
	reindex(t, database)
	transcript, found, err := database.Transcript(ctx, id)
	if err != nil || !found || transcript.PromptCount != 3 {
		t.Fatalf("regrown transcript = %#v found=%v err=%v", transcript, found, err)
	}
	if listedByDefault(t, database, id) {
		t.Fatal("a hidden chat returned once its prompt count grew")
	}
	assertHidden(t, database, id, ClaudeEngine, 50)

	if err := manager.Unhide(ctx, id); err != nil {
		t.Fatal(err)
	}
	if !listedByDefault(t, database, id) {
		t.Fatal("unhide did not bring the chat back")
	}
}

func reindex(t *testing.T, database *store.Store) {
	t.Helper()
	indexer, err := index.New(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := indexer.Run(context.Background(), index.Options{}); err != nil {
		t.Fatal(err)
	}
}

// listedByDefault answers the visibility question twice — through the cached
// SQL candidate query and through compose — and fails if they disagree.
func listedByDefault(t *testing.T, database *store.Store, id string) bool {
	t.Helper()
	ctx := context.Background()
	candidates, _, _, err := database.DefaultCandidates(ctx, 50, 50)
	if err != nil {
		t.Fatal(err)
	}
	cached := false
	for _, candidate := range candidates {
		cached = cached || candidate.UUID == id
	}
	transcripts, err := database.Transcripts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hidden, err := database.HiddenChats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	output := compose.Compose(compose.Input{
		Transcripts: transcripts,
		Hidden:      hidden,
		Options:     compose.Options{View: compose.DefaultView},
	})
	composed := false
	for _, row := range output.Rows {
		composed = composed || row.ID == id
	}
	if cached != composed {
		t.Fatalf(
			"cached candidates (%t) and compose (%t) disagree about %q",
			cached,
			composed,
			id,
		)
	}
	return composed
}

func TestFinisherChoreographyAndTeammateReaping(t *testing.T) {
	jail := newHideJail(t)
	database := jail.open(t)
	defer database.Close()
	ctx := context.Background()
	id := "55555555-5555-4555-8555-555555555555"
	transcriptPath := filepath.Join(jail.claudeRoot, id+".jsonl")
	if err := database.UpsertTranscript(ctx, store.Transcript{
		UUID:        id,
		Path:        transcriptPath,
		PromptCount: 2,
	}); err != nil {
		t.Fatal(err)
	}
	baseline := int64(2)
	if err := database.Hide(ctx, store.Hidden{
		ID:              id,
		Engine:          ClaudeEngine,
		HiddenAt:        100,
		BaselinePrompts: &baseline,
	}); err != nil {
		t.Fatal(err)
	}

	socketName := "cc-400-1-1"
	paneID := "%9"
	writeTestFile(t, filepath.Join(jail.sidDir, socketName), transcriptPath)
	writeTestFile(
		t,
		filepath.Join(jail.sidDir, socketName+"."+paneID),
		transcriptPath,
	)
	detached := filepath.Join(
		jail.home,
		".claude",
		".cc-new-children",
		id,
	)
	paneChildren := filepath.Join(
		jail.home,
		".claude",
		".cc-pane-children",
		id,
	)
	writeTestFile(t, detached, "cc-401-1-1\n")
	writeTestFile(t, paneChildren, "cc-402-1-1\t%12\n")

	tmux := &fakeTmux{existsFor: 2}
	refresher := refreshFunc(func(ctx context.Context) error {
		return database.UpsertTranscript(ctx, store.Transcript{
			UUID:        id,
			Path:        transcriptPath,
			PromptCount: 3,
		})
	})
	finisher, err := NewFinisher(database, Dependencies{
		Tmux:         tmux,
		Refresher:    refresher,
		Delay:        time.Millisecond,
		PollEvery:    time.Millisecond,
		PollAttempts: 5,
		Now:          func() time.Time { return time.Unix(200, 0) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := finisher.Run(ctx, ExitArgs{
		Engine:     ClaudeEngine,
		ID:         id,
		DataPath:   transcriptPath,
		SocketPath: filepath.Join(jail.tmuxDir, socketName),
		SocketName: socketName,
		PaneID:     paneID,
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tmux.sent, []string{"/exit"}) {
		t.Fatalf("sent = %q, want /exit", tmux.sent)
	}
	if !reflect.DeepEqual(tmux.killedServers, []string{"cc-401-1-1"}) {
		t.Fatalf("killed servers = %q", tmux.killedServers)
	}
	if got := tmux.killedPanes; !reflect.DeepEqual(
		got,
		[]string{socketName + "\t" + paneID, "cc-402-1-1\t%12"},
	) {
		t.Fatalf("killed panes = %q", got)
	}
	for _, path := range []string{
		filepath.Join(jail.sidDir, socketName),
		filepath.Join(jail.sidDir, socketName+"."+paneID),
		detached,
		paneChildren,
	} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s remains after cleanup: %v", path, err)
		}
	}
	assertHidden(t, database, id, ClaudeEngine, 100)
}

func TestFinisherCodexUsesQuit(t *testing.T) {
	jail := newHideJail(t)
	database := jail.open(t)
	defer database.Close()
	ctx := context.Background()
	id := "66666666-6666-4666-8666-666666666666"
	path := filepath.Join(jail.codexRoot, "sessions", "rollout-"+id+".jsonl")
	if err := database.UpsertRollout(ctx, store.Rollout{
		ID:          id,
		Path:        path,
		PromptCount: 4,
		UserThread:  true,
	}); err != nil {
		t.Fatal(err)
	}
	tmux := &fakeTmux{}
	finisher, err := NewFinisher(database, Dependencies{
		Tmux:         tmux,
		Refresher:    refreshFunc(func(context.Context) error { return nil }),
		Delay:        time.Millisecond,
		PollEvery:    time.Millisecond,
		PollAttempts: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := finisher.Run(ctx, ExitArgs{
		Engine:     CodexEngine,
		ID:         id,
		DataPath:   path,
		SocketPath: filepath.Join(jail.tmuxDir, "cx-500-1-1"),
		SocketName: "cx-500-1-1",
		PaneID:     "%10",
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tmux.sent, []string{"/quit"}) {
		t.Fatalf("sent = %q, want /quit", tmux.sent)
	}
	assertHidden(t, database, id, CodexEngine, tmuxHiddenAt(t, database, id))
}

func TestCommandSpawnerUsesSetsidSelfReexec(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is not installed")
	}
	root := t.TempDir()
	argvPath := filepath.Join(root, "argv")
	setsidPath := filepath.Join(root, "setsid")
	writeTestFile(
		t,
		setsidPath,
		"#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CC_FLEET_SPAWN_ARGV\"\n",
	)
	if err := os.Chmod(setsidPath, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CC_FLEET_SPAWN_ARGV", argvPath)
	spawner := CommandSpawner{
		Executable: "/jail/cc-fleet",
		Setsid:     setsidPath,
	}
	args := ExitArgs{
		Engine:     ClaudeEngine,
		ID:         "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		DataPath:   "/jail/transcript.jsonl",
		SocketPath: "/jail/tmux/cc-1-1-1",
		SocketName: "cc-1-1-1",
		PaneID:     "%3",
	}
	if err := spawner.Spawn(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(argvPath)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"-f",
		"/jail/cc-fleet",
		"internal",
		"hide-exit",
		"--engine",
		args.Engine,
		"--id",
		args.ID,
		"--path",
		args.DataPath,
		"--socket",
		args.SocketPath,
		"--socket-name",
		args.SocketName,
		"--pane",
		args.PaneID,
		"",
	}, "\n")
	if string(content) != want {
		t.Fatalf("setsid argv = %q, want %q", content, want)
	}
}

type hideJail struct {
	root       string
	home       string
	sidDir     string
	tmuxDir    string
	claudeRoot string
	codexRoot  string
	dbPath     string
}

func newHideJail(t *testing.T) hideJail {
	t.Helper()
	root := t.TempDir()
	jail := hideJail{
		root:       root,
		home:       filepath.Join(root, "home"),
		sidDir:     filepath.Join(root, "sid"),
		tmuxDir:    filepath.Join(root, "tmux"),
		claudeRoot: filepath.Join(root, "claude"),
		codexRoot:  filepath.Join(root, "codex"),
		dbPath:     filepath.Join(root, "fleet.db"),
	}
	for _, directory := range []string{
		jail.home,
		jail.sidDir,
		jail.tmuxDir,
		jail.claudeRoot,
		jail.codexRoot,
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TMUX_TMPDIR", filepath.Join(root, "tmp"))
	t.Setenv("CC_FLEET_DB", jail.dbPath)
	t.Setenv("CC_FLEET_SID_DIR", jail.sidDir)
	t.Setenv("CC_FLEET_CLAUDE_ROOTS", jail.claudeRoot)
	t.Setenv("CC_FLEET_CODEX_ROOT", jail.codexRoot)
	t.Setenv("CC_FLEET_TMUX_DIR", jail.tmuxDir)
	t.Setenv("CC_FLEET_HOME", jail.home)
	return jail
}

func (jail hideJail) open(t *testing.T) *store.Store {
	t.Helper()
	database, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func appendTestFile(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// assertHidden also pins the retired ratchet column to NULL: a hide is
// permanent, so no prompt baseline is ever written.
func assertHidden(
	t *testing.T,
	database *store.Store,
	id, engine string,
	hiddenAt int64,
) {
	t.Helper()
	hidden, found, err := database.Hidden(context.Background(), id)
	if err != nil || !found {
		t.Fatalf("Hidden(%q) found=%v err=%v", id, found, err)
	}
	if hidden.Engine != engine ||
		hidden.HiddenAt != hiddenAt ||
		hidden.BaselinePrompts != nil {
		t.Fatalf("Hidden(%q) = %#v", id, hidden)
	}
}

func tmuxHiddenAt(t *testing.T, database *store.Store, id string) int64 {
	t.Helper()
	hidden, found, err := database.Hidden(context.Background(), id)
	if err != nil || !found {
		t.Fatalf("Hidden(%q) found=%v err=%v", id, found, err)
	}
	return hidden.HiddenAt
}
