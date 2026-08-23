package reload

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/gather"
)

type fakeReloadTmux struct {
	dead     bool
	literal  string
	respawn  string
	displays []string
}

func (tmux *fakeReloadTmux) ListPanes(context.Context, string) ([]Pane, error) {
	return []Pane{{ID: "%7", Dead: tmux.dead, PID: 700}}, nil
}
func (*fakeReloadTmux) SetRemain(context.Context, string, string, bool) error { return nil }
func (*fakeReloadTmux) PaneInMode(context.Context, string, string) (bool, error) {
	return false, nil
}
func (*fakeReloadTmux) CancelMode(context.Context, string, string) error { return nil }
func (*fakeReloadTmux) Capture(context.Context, string, string) (string, error) {
	return "Claude\n❯ ", nil
}
func (tmux *fakeReloadTmux) SendKey(_ context.Context, _, _, key string) error {
	if key == "Enter" && tmux.literal == "/exit" {
		tmux.dead = true
	}
	return nil
}
func (tmux *fakeReloadTmux) SendLiteral(_ context.Context, _, _, value string) error {
	tmux.literal = value
	return nil
}
func (tmux *fakeReloadTmux) Respawn(_ context.Context, _, _, _, command string) error {
	tmux.respawn = command
	tmux.dead = false
	return nil
}
func (tmux *fakeReloadTmux) Display(_ context.Context, _, _, message string) error {
	tmux.displays = append(tmux.displays, message)
	return nil
}

type fakeReloadProc struct {
	pids   []int
	argv   map[int][]string
	cmdErr map[int]error
	stat   map[int]gather.ProcStat
}

func (proc fakeReloadProc) PIDs() ([]int, error) { return proc.pids, nil }
func (proc fakeReloadProc) Cmdline(pid int) ([]string, error) {
	if err := proc.cmdErr[pid]; err != nil {
		return nil, err
	}
	return proc.argv[pid], nil
}
func (fakeReloadProc) Environ(int) (map[string]string, error)     { return nil, nil }
func (proc fakeReloadProc) Stat(pid int) (gather.ProcStat, error) { return proc.stat[pid], nil }

type delayedThenTmux struct {
	fakeReloadTmux
	ready     bool
	submitted bool
}

func (tmux *delayedThenTmux) Capture(context.Context, string, string) (string, error) {
	if tmux.respawn == "" {
		return "Claude\n❯ ", nil
	}
	tmux.ready = true
	if tmux.literal == "continue the task" {
		if tmux.submitted {
			// Submitted text remains in scrollback while the active composer is
			// empty. Submit proof must inspect the composer, not the whole pane.
			return "❯ continue the task\nWorking\n❯ ", nil
		}
		return "Claude\n❯ continue the task", nil
	}
	return "Claude\n❯ ", nil
}

func (tmux *delayedThenTmux) SendKey(_ context.Context, _, _, key string) error {
	if key != "Enter" {
		return nil
	}
	if tmux.literal == "/exit" {
		tmux.dead = true
	} else if tmux.literal == "continue the task" {
		tmux.submitted = true
	}
	return nil
}

type promptReadyProc struct{ tmux *delayedThenTmux }

func (proc promptReadyProc) PIDs() ([]int, error) {
	if !proc.tmux.ready {
		return nil, errors.New("Claude has not reached its prompt")
	}
	return []int{801}, nil
}

type respawnPIDTmux struct {
	delayedThenTmux
	oldPID int
	newPID int
}

func (tmux *respawnPIDTmux) ListPanes(context.Context, string) ([]Pane, error) {
	pid := tmux.oldPID
	if tmux.respawn != "" {
		pid = tmux.newPID
	}
	return []Pane{{ID: "%7", Dead: tmux.dead, PID: pid}}, nil
}

type respawnPromptProc struct{ tmux *respawnPIDTmux }

func (proc respawnPromptProc) PIDs() ([]int, error) {
	if !proc.tmux.ready {
		return nil, errors.New("Claude has not reached its prompt")
	}
	return []int{801}, nil
}
func (respawnPromptProc) Cmdline(int) ([]string, error) { return []string{"claude"}, nil }
func (respawnPromptProc) Environ(int) (map[string]string, error) {
	return map[string]string{}, nil
}
func (proc respawnPromptProc) Stat(pid int) (gather.ProcStat, error) {
	if pid == 801 {
		return gather.ProcStat{ParentPID: proc.tmux.newPID}, nil
	}
	return gather.ProcStat{ParentPID: 1}, nil
}
func (promptReadyProc) Cmdline(int) ([]string, error) { return []string{"claude"}, nil }
func (promptReadyProc) Environ(int) (map[string]string, error) {
	return map[string]string{}, nil
}
func (promptReadyProc) Stat(pid int) (gather.ProcStat, error) {
	if pid == 801 {
		return gather.ProcStat{ParentPID: 700}, nil
	}
	return gather.ProcStat{ParentPID: 1}, nil
}

func TestSessionFromCrumbUsesPaneSpecificIdentity(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe-1.%7"), []byte("/transcripts/pane.jsonl\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe-1"), []byte("/transcripts/socket.jsonl\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	id, path, err := SessionFromCrumb(dir, "probe-1", "%7")
	if err != nil {
		t.Fatal(err)
	}
	if id != "pane" || path != "/transcripts/pane.jsonl" {
		t.Fatalf("identity = %q/%q", id, path)
	}
}

func TestTranscriptCWDReadsARecordBeforeTheSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.jsonl")
	content := strings.Join([]string{
		`{"type":"summary","message":"not a cwd"}`,
		`{"cwd":"/jail/project","message":"start"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := TranscriptCWD(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/jail/project" {
		t.Fatalf("cwd = %q", got)
	}
}

func TestRunRefusesAnOverlappingPaneReload(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".probe-1.%7.reloadlock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	_, err = Run(context.Background(), Request{Engine: pfmengine.Claude, SocketPath: "/tmp/probe-1", Pane: "%7", Account: 2, AccountIDs: []int{2}}, Options{SIDDir: dir, Delay: -1}, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "already in flight") {
		t.Fatalf("overlap error = %v", err)
	}
}

func TestClaudeRunUnsetsInheritedIdentity(t *testing.T) {
	run := claudeRun(Request{
		Account:          2,
		AccountConfigDir: "/jail/home/.cc/2",
		SessionID:        "11111111-1111-4111-8111-111111111111",
	})
	for _, variable := range []string{"CLAUDE_CODE_SESSION_ID", "CLAUDE_CONFIG_DIR", "FORCE_PROMPT_CACHING_5M"} {
		if !strings.Contains(run, variable) {
			t.Fatalf("run %q does not mention %s", run, variable)
		}
	}
	if !strings.Contains(run, "claude --resume") {
		t.Fatalf("run %q has no resume", run)
	}
}

func TestCodexRunUsesTheSelectedHomeAndRosterPolicy(t *testing.T) {
	run := engineRun(Request{
		Engine:      "cx",
		Account:     9,
		AccountHome: "/jail/codex/9",
		CodexBinary: "/opt/codex safe",
		CodexYolo:   false,
		SessionID:   "019ff700-0000-7000-8000-000000000001",
	})
	for _, want := range []string{
		"CODEX_HOME='/jail/codex/9'",
		"'/opt/codex safe' --sandbox workspace-write",
		"resume '019ff700-0000-7000-8000-000000000001'",
	} {
		if !strings.Contains(run, want) {
			t.Fatalf("run %q lacks %q", run, want)
		}
	}
	if strings.Contains(run, "CLAUDE_CONFIG_DIR=") {
		t.Fatalf("Codex reload inherited a Claude config assignment: %q", run)
	}
}

func TestOpencodeRunIsNotMisroutedToClaude(t *testing.T) {
	if run := engineRun(Request{Engine: pfmengine.Opencode}); run != "" {
		t.Fatalf("engineRun(OpenCode)=%q, want an explicit unsupported result", run)
	}
}

func TestRunRefusesOpencodeBeforeExitingThePane(t *testing.T) {
	tmux := &fakeReloadTmux{}
	_, err := Run(
		context.Background(),
		Request{
			Engine: pfmengine.Opencode, SocketPath: "/tmp/ox-session", Pane: "%7",
			PanePID: 700, Account: 1, AccountIDs: []int{1}, CWD: "/work",
		},
		Options{SIDDir: t.TempDir(), Delay: -1, Poll: -1, ExitTries: 1},
		tmux, nil, nil,
	)
	if err == nil || !strings.Contains(err.Error(), "OpenCode") || tmux.literal != "" {
		t.Fatalf("OpenCode reload error=%v literal=%q, want refusal before /exit", err, tmux.literal)
	}
}

func TestRunGracefullyExitsThenRespawnsTheSamePane(t *testing.T) {
	tmux := &fakeReloadTmux{}
	result, err := Run(
		context.Background(),
		Request{
			Engine:           pfmengine.Claude,
			SocketPath:       "/tmp/tmux-1000/probe-reload",
			Pane:             "%7",
			PanePID:          700,
			SessionID:        "11111111-1111-4111-8111-111111111111",
			CWD:              "/jail/project",
			Account:          2,
			AccountIDs:       []int{2},
			AccountConfigDir: "/jail/home/.cc/2",
			Cache1H:          false,
		},
		Options{SIDDir: t.TempDir(), Delay: -1, Poll: -1, ExitTries: 2},
		tmux,
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Fresh || tmux.literal != "/exit" {
		t.Fatalf("result=%+v literal=%q", result, tmux.literal)
	}
	for _, want := range []string{
		"CLAUDE_CONFIG_DIR=",
		"FORCE_PROMPT_CACHING_5M=1",
		"claude --resume",
		"11111111-1111-4111-8111-111111111111",
	} {
		if !strings.Contains(tmux.respawn, want) {
			t.Fatalf("respawn %q lacks %q", tmux.respawn, want)
		}
	}
}

func TestRunWaitsForTheRebornPromptBeforeCheckingClaudeAndSubmittingThen(t *testing.T) {
	tmux := &delayedThenTmux{}
	_, err := Run(
		context.Background(),
		Request{
			Engine:     pfmengine.Claude,
			SocketPath: "/tmp/tmux-1000/probe-reload-then",
			Pane:       "%7",
			PanePID:    700,
			SessionID:  "11111111-1111-4111-8111-111111111111",
			CWD:        "/jail/project",
			Account:    1,
			AccountIDs: []int{1},
			Cache1H:    true,
			Then:       "continue the task",
		},
		Options{
			SIDDir:    t.TempDir(),
			Delay:     -1,
			Poll:      -1,
			ExitTries: 2,
			ThenTries: 2,
		},
		tmux,
		promptReadyProc{tmux: tmux},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !tmux.ready || !tmux.submitted {
		t.Fatalf("ready=%t submitted=%t", tmux.ready, tmux.submitted)
	}
	if len(tmux.displays) != 0 {
		t.Fatalf("successful --then displayed a failure: %q", tmux.displays)
	}
}

func TestRunRefreshesThePanePIDAfterRespawnBeforeSubmittingThen(t *testing.T) {
	tmux := &respawnPIDTmux{oldPID: 700, newPID: 900}
	_, err := Run(
		context.Background(),
		Request{
			Engine:     pfmengine.Claude,
			SocketPath: "/tmp/tmux-1000/probe-reload-then-pid",
			Pane:       "%7",
			PanePID:    tmux.oldPID,
			SessionID:  "11111111-1111-4111-8111-111111111111",
			CWD:        "/jail/project",
			Account:    2,
			AccountIDs: []int{2},
			Then:       "continue the task",
		},
		Options{SIDDir: t.TempDir(), Delay: -1, Poll: -1, ExitTries: 2, ThenTries: 2},
		tmux,
		respawnPromptProc{tmux: tmux},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !tmux.submitted {
		t.Fatal("--then was not submitted after the pane process changed")
	}
}

func TestClaudeLiveUsesThePaneProcessPIDNotTheTmuxPaneID(t *testing.T) {
	proc := fakeReloadProc{
		pids: []int{801},
		argv: map[int][]string{801: {"claude"}},
		stat: map[int]gather.ProcStat{801: {ParentPID: 700}},
	}
	live, err := claudeLive(proc, 700)
	if err != nil || !live {
		t.Fatalf("claudeLive() = %v, %v", live, err)
	}
}

func TestClaudeLiveIgnoresAProcessThatExitsDuringTheProcScan(t *testing.T) {
	proc := fakeReloadProc{
		pids:   []int{800, 801},
		argv:   map[int][]string{801: {"claude"}},
		cmdErr: map[int]error{800: os.ErrNotExist},
		stat:   map[int]gather.ProcStat{801: {ParentPID: 700}},
	}
	live, err := claudeLive(proc, 700)
	if err != nil || !live {
		t.Fatalf("claudeLive() = %v, %v", live, err)
	}
}

func TestFailedThenWritesTheRecoverableSentinel(t *testing.T) {
	dir := t.TempDir()
	tmux := &fakeReloadTmux{}
	request := Request{
		SocketPath: "/tmp/tmux-1000/probe-reload",
		Pane:       "%7",
		Then:       "continue the task",
	}
	if err := failThen(context.Background(), request, dir, tmux, "input box missing"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "probe-reload.then-failed"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "continue the task\n" || len(tmux.displays) != 1 {
		t.Fatalf("sentinel=%q displays=%q", content, tmux.displays)
	}
}
