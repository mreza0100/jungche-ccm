package gather

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"hostops/cc-fleet/internal/paths"
)

type tmuxJail struct {
	root      string
	tmuxDir   string
	sidDir    string
	codexRoot string
	home      string
	sockets   []string
}

type alwaysFailTmux struct{}

func (alwaysFailTmux) ListPanes(context.Context, string) ([]Pane, error) {
	return nil, fmt.Errorf("dead server")
}

type failOnceTmux struct {
	calls int
}

func (tmux *failOnceTmux) ListPanes(
	context.Context,
	string,
) ([]Pane, error) {
	tmux.calls++
	if tmux.calls == 1 {
		return nil, fmt.Errorf("transient server race")
	}
	return []Pane{{Socket: "cc-1-2-3", PaneID: "%1"}}, nil
}

func newTmuxJail(t *testing.T) *tmuxJail {
	t.Helper()

	root, err := os.MkdirTemp("/tmp", "ccf")
	if err != nil {
		t.Fatalf("create short tmux jail: %v", err)
	}
	jail := &tmuxJail{
		root:      root,
		tmuxDir:   filepath.Join(root, "tmux-"+strconv.Itoa(os.Getuid())),
		sidDir:    filepath.Join(root, "sid"),
		codexRoot: filepath.Join(root, "codex"),
		home:      filepath.Join(root, "home"),
	}
	setGatherTestEnv(t, jail.root, jail.tmuxDir)

	t.Cleanup(func() {
		for _, socket := range jail.sockets {
			command := jail.command("-L", socket, "kill-server")
			_ = command.Run()
		}
		if err := os.RemoveAll(jail.root); err != nil {
			t.Errorf("remove tmux jail %q: %v", jail.root, err)
		}
	})
	return jail
}

func setGatherTestEnv(t *testing.T, root, tmuxDir string) {
	t.Helper()

	home := filepath.Join(root, "home")
	sidDir := filepath.Join(root, "sid")
	codexRoot := filepath.Join(root, "codex")
	for _, directory := range []string{home, sidDir, codexRoot, tmuxDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("create gather jail directory %q: %v", directory, err)
		}
	}

	t.Setenv("HOME", home)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_TMPDIR", root)
	t.Setenv(paths.EnvHome, home)
	t.Setenv(paths.EnvDB, filepath.Join(root, "fleet.db"))
	t.Setenv(paths.EnvSIDDir, sidDir)
	t.Setenv(paths.EnvClaudeRoots, filepath.Join(root, "claude-projects"))
	t.Setenv(paths.EnvCodexRoot, codexRoot)
	t.Setenv(paths.EnvTmuxDir, tmuxDir)
}

func (jail *tmuxJail) command(arguments ...string) *exec.Cmd {
	command := exec.Command("tmux", arguments...)
	command.Env = append(
		os.Environ(),
		"HOME="+jail.home,
		"TMUX=",
		"TMUX_TMPDIR="+jail.root,
	)
	return command
}

func (jail *tmuxJail) startServer(
	t *testing.T,
	socket, session, window, title string,
) {
	t.Helper()

	command := jail.command(
		"-f",
		"/dev/null",
		"-L",
		socket,
		"new-session",
		"-d",
		"-s",
		session,
		"-n",
		window,
		"sleep 120",
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start jailed tmux server %q: %v: %s", socket, err, output)
	}
	jail.sockets = append(jail.sockets, socket)

	command = jail.command(
		"-L",
		socket,
		"select-pane",
		"-t",
		session+":0.0",
		"-T",
		title,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("set jailed pane title for %q: %v: %s", socket, err, output)
	}
}

func (jail *tmuxJail) killServer(socket string) error {
	return jail.command("-L", socket, "kill-server").Run()
}

func createCorpseSocket(t *testing.T, path string, modified time.Time) {
	t.Helper()

	listener, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: path,
		Net:  "unix",
	})
	if err != nil {
		t.Fatalf("create corpse socket %q: %v", path, err)
	}
	listener.SetUnlinkOnClose(false)
	if err := listener.Close(); err != nil {
		t.Fatalf("close corpse socket %q: %v", path, err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatalf("set corpse socket mtime %q: %v", path, err)
	}
}

func TestProbeTmuxReadOnlyLeavesOldCorpse(t *testing.T) {
	tmuxDir := t.TempDir()
	path := filepath.Join(tmuxDir, "cc-7-8-9")
	now := time.Now()
	createCorpseSocket(t, path, now.Add(-2*time.Hour))
	probe, err := ProbeTmuxReadOnly(
		context.Background(),
		tmuxDir,
		alwaysFailTmux{},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(probe.CorpseSwept) != 0 || len(probe.ProbeWarnings) != 1 {
		t.Fatalf("read-only probe = %#v", probe)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("read-only probe removed old corpse: %v", err)
	}
}

func TestProbeTmuxRetriesOneTransientReadFailure(t *testing.T) {
	tmuxDir := t.TempDir()
	path := filepath.Join(tmuxDir, "cc-1-2-3")
	createCorpseSocket(t, path, time.Now())
	client := &failOnceTmux{}
	probe, err := ProbeTmuxReadOnly(
		context.Background(),
		tmuxDir,
		client,
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 2 ||
		len(probe.Panes) != 1 ||
		len(probe.ProbeWarnings) != 0 {
		t.Fatalf("retry probe=%#v calls=%d", probe, client.calls)
	}
}

func TestParseLegacyPaneFallback(t *testing.T) {
	output := strings.Join([]string{
		"session\tpane title\t/work/project\t4\t1\t100\t%7\t/dev/pts/9\tClaude\t321",
		"",
	}, "\n")
	panes, err := parseLegacyPaneOutput("cc-1-2-3", []byte(output))
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 1 ||
		panes[0].SessionName != "session" ||
		panes[0].PaneID != "%7" ||
		panes[0].WindowName != "Claude" ||
		panes[0].WindowID != "" ||
		panes[0].PID != 321 ||
		!panes[0].Attached {
		t.Fatalf("legacy fallback panes = %#v", panes)
	}
}

func TestJailedTmuxProbeAndGather(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux binary is not installed")
	}

	jail := newTmuxJail(t)
	const (
		ccSocket = "cc-101-202-303"
		cxSocket = "cx-404-505-606"
	)
	jail.startServer(t, ccSocket, "cc-session", "claude-old", "claude-pane")
	jail.startServer(t, cxSocket, "cx-session", "codex-old", "codex-pane")
	jail.startServer(t, "vsct-test", "vsct-session", "ignored", "ignored")
	jail.startServer(t, "revive-test", "revive-session", "ignored", "ignored")

	now := time.Now()
	oldCorpse := "cc-707-808-909"
	freshCorpse := "cx-111-222-333"
	oldCorpsePath := filepath.Join(jail.tmuxDir, oldCorpse)
	freshCorpsePath := filepath.Join(jail.tmuxDir, freshCorpse)
	createCorpseSocket(t, oldCorpsePath, now.Add(-2*time.Hour))
	createCorpseSocket(t, freshCorpsePath, now.Add(-30*time.Minute))

	client := CommandTmux{Binary: "tmux", TmuxTmpDir: jail.root}
	probe, err := ProbeTmux(context.Background(), jail.tmuxDir, client, now)
	if err != nil {
		t.Fatalf("ProbeTmux() error = %v", err)
	}
	if got := paneSockets(probe.Panes); !reflect.DeepEqual(
		got,
		[]string{ccSocket, cxSocket},
	) {
		entries, _ := os.ReadDir(jail.tmuxDir)
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			info, _ := entry.Info()
			names = append(names, fmt.Sprintf("%s:%v", entry.Name(), info.Mode()))
		}
		t.Fatalf(
			"ProbeTmux() sockets = %q, want only chat sockets; warnings=%q entries=%q",
			got,
			probe.ProbeWarnings,
			names,
		)
	}
	if !reflect.DeepEqual(probe.CorpseSwept, []string{oldCorpse}) {
		t.Fatalf("ProbeTmux().CorpseSwept = %q, want old corpse", probe.CorpseSwept)
	}
	if len(probe.ProbeWarnings) != 2 {
		t.Fatalf("ProbeTmux().ProbeWarnings = %q, want two corpse warnings", probe.ProbeWarnings)
	}
	if _, err := os.Stat(oldCorpsePath); !os.IsNotExist(err) {
		t.Fatalf("old corpse still exists: %v", err)
	}
	if _, err := os.Stat(freshCorpsePath); err != nil {
		t.Fatalf("fresh corpse was removed: %v", err)
	}

	paneBySocket := make(map[string]Pane)
	for _, pane := range probe.Panes {
		paneBySocket[pane.Socket] = pane
	}
	ccPane := paneBySocket[ccSocket]
	cxPane := paneBySocket[cxSocket]
	if ccPane.SessionName != "cc-session" ||
		ccPane.WindowName != "claude-old" ||
		ccPane.PaneTitle != "claude-pane" ||
		ccPane.CurrentPath == "" ||
		ccPane.TTY == "" ||
		ccPane.PID <= 0 ||
		ccPane.PaneID == "" {
		t.Fatalf("ProbeTmux() cc pane = %+v, want exact jailed fields", ccPane)
	}
	if cxPane.SessionName != "cx-session" ||
		cxPane.WindowName != "codex-old" ||
		cxPane.PaneTitle != "codex-pane" ||
		cxPane.TTY == "" ||
		cxPane.PID <= 0 ||
		cxPane.PaneID == "" {
		t.Fatalf("ProbeTmux() cx pane = %+v, want exact jailed fields", cxPane)
	}
	for name, content := range map[string]string{
		ccSocket:                       "/transcripts/socket.jsonl",
		cxSocket + "." + cxPane.PaneID: "/transcripts/pane.jsonl",
		ccSocket + ".%999":             "/transcripts/stale.jsonl",
		ccSocket + ".then-failed":      "/transcripts/sentinel.jsonl",
	} {
		if err := os.WriteFile(filepath.Join(jail.sidDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write jailed crumb %q: %v", name, err)
		}
	}

	const (
		codexPID = 900001
		agentPID = 900002
		session  = "01234567-89ab-cdef-0123-456789abcdef"
	)
	rolloutPath := filepath.Join(jail.codexRoot, "sessions", "2026", "rollout-live.jsonl")
	proc := &fakeProcFS{processes: map[int]fakeProcess{
		cxPane.PID: {},
		ccPane.PID: {},
		codexPID: {
			cmdline: []string{"/usr/bin/codex"},
			fdLinks: []FDLink{
				{FD: 9, Target: filepath.Join(jail.codexRoot, "sessions", "rollout-later.jsonl")},
				{FD: 3, Target: rolloutPath},
			},
			stat: ProcStat{ParentPID: cxPane.PID, StartTime: 10},
		},
		agentPID: {
			cmdline: []string{"/usr/bin/claude", "--session-id", session},
			environ: map[string]string{
				"CLAUDE_CONFIG_DIR":        filepath.Join(jail.home, ".cc", "2"),
				"ENABLE_PROMPT_CACHING_1H": "1",
			},
			stat: ProcStat{ParentPID: ccPane.PID, StartTime: 20},
		},
	}}
	longName := strings.Repeat("界", 25)
	gatherer, err := New(Dependencies{
		ProcFS:    proc,
		Tmux:      client,
		Now:       func() time.Time { return now },
		CodexName: func(string) string { return longName },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	snapshot, err := gatherer.Gather(context.Background())
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	if got := paneSockets(snapshot.Panes); !reflect.DeepEqual(
		got,
		[]string{ccSocket, cxSocket},
	) {
		t.Fatalf("Gather().Panes sockets = %q", got)
	}
	if len(snapshot.Crumbs) != 2 {
		t.Fatalf("Gather().Crumbs = %#v, want socket and pane crumbs", snapshot.Crumbs)
	}
	if len(snapshot.Codex) != 1 ||
		snapshot.Codex[0].Socket != cxSocket ||
		snapshot.Codex[0].RolloutPath != rolloutPath {
		t.Fatalf("Gather().Codex = %#v", snapshot.Codex)
	}
	if len(snapshot.Agents) != 1 ||
		snapshot.Agents[0].Socket != ccSocket ||
		snapshot.Agents[0].SessionID != session {
		t.Fatalf("Gather().Agents = %#v", snapshot.Agents)
	}
	if len(snapshot.ClaudeProcesses) != 1 ||
		snapshot.ClaudeProcesses[0].PID != agentPID ||
		snapshot.ClaudeProcesses[0].Socket != ccSocket {
		t.Fatalf("Gather().ClaudeProcesses = %#v", snapshot.ClaudeProcesses)
	}
	if !reflect.DeepEqual(snapshot.Cache1HSockets, []string{ccSocket}) {
		t.Fatalf("Gather().Cache1HSockets = %q", snapshot.Cache1HSockets)
	}
	if len(snapshot.Renames) != 1 ||
		snapshot.Renames[0].TargetName != strings.Repeat("界", 24) {
		t.Fatalf("Gather().Renames = %#v", snapshot.Renames)
	}
	if !reflect.DeepEqual(snapshot.StaleSwept, []string{ccSocket + ".%999"}) {
		t.Fatalf("Gather().StaleSwept = %q", snapshot.StaleSwept)
	}
	if _, err := os.Stat(filepath.Join(jail.sidDir, ccSocket+".then-failed")); err != nil {
		t.Fatalf("sentinel was touched by Gather(): %v", err)
	}
}

func paneSockets(panes []Pane) []string {
	sockets := make([]string, 0, len(panes))
	for _, pane := range panes {
		sockets = append(sockets, pane.Socket)
	}
	return sockets
}

func TestTmuxJailUsesShortPrivateSocketPaths(t *testing.T) {
	jail := newTmuxJail(t)
	socketPath := filepath.Join(jail.tmuxDir, "cc-1-2-3")
	if len(socketPath) >= 100 {
		t.Fatalf("jailed socket path is too long (%d): %q", len(socketPath), socketPath)
	}
	if !strings.HasPrefix(filepath.Clean(jail.root), "/tmp/ccf") {
		t.Fatalf("tmux jail escaped short private root: %q", jail.root)
	}
}
