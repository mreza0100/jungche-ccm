package action

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"hostops/cc-fleet/internal/compose"
)

type fakeActionTmux struct {
	mutex        sync.Mutex
	panes        map[string][]Pane
	alive        map[string]bool
	killedPanes  []string
	killedServer []string
	sized        []string
	selected     []string
	created      []CodexServer
}

func (tmux *fakeActionTmux) ListPanes(
	_ context.Context,
	socket string,
) ([]Pane, error) {
	tmux.mutex.Lock()
	defer tmux.mutex.Unlock()
	if !tmux.alive[socket] {
		return nil, errors.New("dead socket")
	}
	return append([]Pane(nil), tmux.panes[socket]...), nil
}

func (tmux *fakeActionTmux) SocketAlive(
	_ context.Context,
	socket string,
) bool {
	tmux.mutex.Lock()
	defer tmux.mutex.Unlock()
	return tmux.alive[socket]
}

func (tmux *fakeActionTmux) KillPane(
	_ context.Context,
	socket, paneID string,
) error {
	tmux.mutex.Lock()
	defer tmux.mutex.Unlock()
	tmux.killedPanes = append(tmux.killedPanes, socket+"\t"+paneID)
	panes := tmux.panes[socket]
	kept := panes[:0]
	for _, pane := range panes {
		if pane.PaneID != paneID {
			kept = append(kept, pane)
		}
	}
	tmux.panes[socket] = kept
	if len(kept) == 0 {
		tmux.alive[socket] = false
	}
	return nil
}

func (tmux *fakeActionTmux) KillServer(
	_ context.Context,
	socket string,
) error {
	tmux.mutex.Lock()
	defer tmux.mutex.Unlock()
	tmux.killedServer = append(tmux.killedServer, socket)
	tmux.alive[socket] = false
	delete(tmux.panes, socket)
	return nil
}

func (tmux *fakeActionTmux) SetWindowSizeLatest(
	_ context.Context,
	socket string,
) error {
	tmux.mutex.Lock()
	defer tmux.mutex.Unlock()
	tmux.sized = append(tmux.sized, socket)
	return nil
}

func (tmux *fakeActionTmux) SelectWindow(
	_ context.Context,
	socket string,
	windowIndex int,
) error {
	tmux.mutex.Lock()
	defer tmux.mutex.Unlock()
	tmux.selected = append(
		tmux.selected,
		socket+"\t"+string(rune('0'+windowIndex)),
	)
	return nil
}

func (tmux *fakeActionTmux) CreateCodexServer(
	_ context.Context,
	server CodexServer,
) error {
	tmux.mutex.Lock()
	defer tmux.mutex.Unlock()
	tmux.created = append(tmux.created, server)
	tmux.alive[server.Socket] = true
	return nil
}

type fakeProcesses struct {
	processes  []Process
	terminated []int
	mutex      sync.Mutex
}

func (processes *fakeProcesses) Processes(
	context.Context,
) ([]Process, error) {
	processes.mutex.Lock()
	defer processes.mutex.Unlock()
	return append([]Process(nil), processes.processes...), nil
}

func (processes *fakeProcesses) Terminate(pid int) error {
	processes.mutex.Lock()
	defer processes.mutex.Unlock()
	processes.terminated = append(processes.terminated, pid)
	return nil
}

type fixedGate bool

func (gate fixedGate) Confirm(
	context.Context,
	GateRequest,
) (bool, error) {
	return bool(gate), nil
}

type captureRunner struct {
	name string
	args []string
}

func (runner *captureRunner) Run(
	_ context.Context,
	name string,
	args ...string,
) error {
	runner.name = name
	runner.args = append([]string(nil), args...)
	return nil
}

func TestSoloReapsPaneAloneServerAndStrayWithKeepTTY(t *testing.T) {
	jailAction(t)
	id := "33333333-3333-4333-8333-333333333333"
	sidDir := os.Getenv("CC_FLEET_SID_DIR")
	for name, content := range map[string]string{
		"cc-100-1-1":    "/tx/" + id + ".jsonl",
		"cc-200-1-1.%2": "/tx/" + id + ".jsonl",
		"cc-300-1-1":    "/tx/" + id + ".jsonl",
		"cc-400-1-1":    "/tx/" + id + ".jsonl",
		"cc-500-1-1":    "/tx/" + id + ".jsonl",
	} {
		writeActionFile(t, filepath.Join(sidDir, name), content, 0o600)
	}
	tmux := &fakeActionTmux{
		alive: map[string]bool{
			"cc-100-1-1": true,
			"cc-200-1-1": true,
			"cc-300-1-1": true,
			"cc-400-1-1": true,
		},
		panes: map[string][]Pane{
			"cc-100-1-1": {{PaneID: "%1", TTY: "pts/1"}},
			"cc-200-1-1": {
				{PaneID: "%2", TTY: "pts/2"},
				{PaneID: "%9", TTY: "pts/9"},
			},
			"cc-300-1-1": {{PaneID: "%3", TTY: "pts/3"}},
			"cc-400-1-1": {
				{PaneID: "%4", TTY: "pts/4"},
				{PaneID: "%5", TTY: "pts/5"},
			},
		},
	}
	processes := &fakeProcesses{processes: []Process{
		{PID: 10, Argv: []string{"claude", "--resume", id}, TTY: "pts/1"},
		{PID: 11, Argv: []string{"claude", "--resume", id}, TTY: "pts/8"},
		{PID: 12, Argv: []string{"claude", "--bg-pty-host", id}, TTY: "pts/8"},
		{PID: 13, Argv: []string{"node", id}, TTY: "pts/8"},
	}}
	var stderr bytes.Buffer
	executor, err := New(Dependencies{
		Tmux:      tmux,
		Processes: processes,
		Gate:      fixedGate(false),
		Runner:    &captureRunner{},
		Stderr:    &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := executor.Solo(
		context.Background(),
		id,
		"cc-100-1-1",
		false,
	); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(
		tmux.killedPanes,
		[]string{"cc-200-1-1\t%2"},
	) {
		t.Fatalf("killed panes = %q", tmux.killedPanes)
	}
	if !reflect.DeepEqual(tmux.killedServer, []string{"cc-300-1-1"}) {
		t.Fatalf("killed servers = %q", tmux.killedServer)
	}
	if !reflect.DeepEqual(processes.terminated, []int{11}) {
		t.Fatalf("terminated = %v", processes.terminated)
	}
	for _, name := range []string{
		"cc-200-1-1.%2",
		"cc-300-1-1",
		"cc-400-1-1",
		"cc-500-1-1",
	} {
		if _, err := os.Stat(filepath.Join(sidDir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("crumb %s remains: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(sidDir, "cc-100-1-1")); err != nil {
		t.Fatalf("keep crumb removed: %v", err)
	}
}

func TestExecutorGateSelfSwitchDeadFallbackAndCodexPrepare(t *testing.T) {
	jailAction(t)
	tmux := &fakeActionTmux{
		alive: map[string]bool{"cc-100-1-1": true},
		panes: map[string][]Pane{
			"cc-100-1-1": {
				{PaneID: "%1", WindowIndex: 0, CurrentCommand: "bash"},
				{PaneID: "%2", WindowIndex: 3, CurrentCommand: "claude"},
			},
		},
	}
	runner := &captureRunner{}
	var stderr bytes.Buffer
	executor, err := New(Dependencies{
		Tmux:      tmux,
		Processes: &fakeProcesses{},
		Gate:      fixedGate(true),
		Runner:    runner,
		Stderr:    &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := "44444444-4444-4444-8444-444444444444"
	request := Request{
		Row: compose.Row{
			Kind:        compose.LiveClaude,
			ID:          id,
			Socket:      "cc-100-1-1",
			SessionName: "live-session",
			Name:        "Live",
			Account:     2,
			C1H:         false,
		},
		PrimaryAccount: 1,
		Cache1H:        true,
		Home:           "/home/test",
		FreshSocket:    "cc-900-1-1",
		SwapScript:     "/jail/cc-swap-chat.sh",
	}
	line, err := executor.Open(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if line != "TMUX= tmux -L 'cc-100-1-1' attach -t 'live-session'" {
		t.Fatalf("live line = %q", line)
	}
	if runner.name != "bash" || !reflect.DeepEqual(runner.args, []string{
		"/jail/cc-swap-chat.sh",
		"--sock",
		"cc-100-1-1",
		"1",
		"--1h",
		"1",
	}) {
		t.Fatalf("swap runner = %q %q", runner.name, runner.args)
	}

	// Without an override the swap script resolves to the installed bundle;
	// the repo copy under work/host-ops/ is a pre-move path that is gone.
	bare := request
	bare.SwapScript = ""
	runner.args = nil
	if _, err := executor.Open(context.Background(), bare); err != nil {
		t.Fatal(err)
	}
	if len(runner.args) == 0 ||
		runner.args[0] != "/home/test/.claude/bin/cc-swap-chat.sh" {
		t.Fatalf("default swap script = %q", runner.args)
	}

	request.CurrentTMUX = "/tmp/jail/cc-100-1-1,1,0"
	line, err = executor.Open(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if line != "" || !reflect.DeepEqual(
		tmux.selected,
		[]string{"cc-100-1-1\t3"},
	) {
		t.Fatalf("selfswitch line=%q selected=%q", line, tmux.selected)
	}

	request.CurrentTMUX = ""
	request.Row.Socket = "cc-404-1-1"
	request.Row.CWD = "/work/dead"
	line, err = executor.Open(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !stringsContainsAll(
		line,
		"new-session",
		"'cc-900-1-1'",
		"claude",
		"--resume",
	) {
		t.Fatalf("dead fallback line = %q", line)
	}

	codexRequest := Request{
		Row: compose.Row{
			Kind: compose.ResumeCodex,
			ID:   "55555555-5555-4555-8555-555555555555",
			CWD:  "/work/codex",
		},
		PrimaryAccount: 1,
		Home:           "/home/test",
		FreshSocket:    "cx-901-1-1",
	}
	line, err = executor.Open(context.Background(), codexRequest)
	if err != nil {
		t.Fatal(err)
	}
	if line != "TMUX= tmux -L 'cx-901-1-1' attach -t 'cx-901-1-1:Codex'" ||
		len(tmux.created) != 1 ||
		tmux.created[0].Socket != "cx-901-1-1" {
		t.Fatalf("Codex line=%q created=%#v", line, tmux.created)
	}
}

func TestExecutorCodexWindowVerificationAndDeadFallback(t *testing.T) {
	jailAction(t)
	tmux := &fakeActionTmux{
		alive: map[string]bool{
			"cx-renamed": true,
			"cx-live":    true,
		},
		panes: map[string][]Pane{
			"cx-renamed": {{
				SessionName: "codex-session",
				WindowName:  "renamed-away",
			}},
			"cx-live": {{
				SessionName: "codex-session",
				WindowName:  "Expected",
			}},
		},
	}
	var stderr bytes.Buffer
	executor, err := New(Dependencies{
		Tmux:      tmux,
		Processes: &fakeProcesses{},
		Gate:      fixedGate(false),
		Runner:    &captureRunner{},
		Stderr:    &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{
		Row: compose.Row{
			Kind:        compose.LiveCodex,
			ID:          "codex-id",
			Socket:      "cx-renamed",
			SessionName: "codex-session",
			WindowName:  "Expected",
			CWD:         "/work/codex",
		},
		PrimaryAccount: 1,
		FreshSocket:    "cx-fresh",
		Home:           "/home/test",
	}
	line, err := executor.Open(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if line != "TMUX= tmux -L 'cx-renamed' attach -t 'codex-session'" {
		t.Fatalf("renamed-window attach = %q", line)
	}

	request.Row.Socket = "cx-live"
	line, err = executor.Open(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if line != "TMUX= tmux -L 'cx-live' attach -t 'codex-session:Expected'" {
		t.Fatalf("verified-window attach = %q", line)
	}

	request.Row.Socket = "cx-dead"
	line, err = executor.Open(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(line, "cx-dead") ||
		!strings.Contains(line, "cx-fresh:Codex") ||
		!strings.Contains(stderr.String(), "disappeared; resuming") {
		t.Fatalf("dead fallback line=%q stderr=%q", line, stderr.String())
	}
}

func jailAction(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{
		filepath.Join(root, "sid"),
		filepath.Join(root, "tmux"),
		filepath.Join(root, "home"),
		filepath.Join(root, "claude"),
		filepath.Join(root, "codex"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TMUX_TMPDIR", filepath.Join(root, "tmp"))
	t.Setenv("CC_FLEET_DB", filepath.Join(root, "fleet.db"))
	t.Setenv("CC_FLEET_SID_DIR", filepath.Join(root, "sid"))
	t.Setenv("CC_FLEET_CLAUDE_ROOTS", filepath.Join(root, "claude"))
	t.Setenv("CC_FLEET_CODEX_ROOT", filepath.Join(root, "codex"))
	t.Setenv("CC_FLEET_TMUX_DIR", filepath.Join(root, "tmux"))
	t.Setenv("CC_FLEET_HOME", filepath.Join(root, "home"))
	return root
}

func stringsContainsAll(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !bytes.Contains([]byte(value), []byte(fragment)) {
			return false
		}
	}
	return true
}
