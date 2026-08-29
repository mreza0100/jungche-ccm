package reload

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"hostops/pfm/internal/action"
	pfmconfig "hostops/pfm/internal/config"
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
func (tmux *fakeReloadTmux) Capture(context.Context, string, string) (string, error) {
	if tmux.literal == "/exit" {
		return "Claude\n❯ /exit", nil
	}
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
	marker    string
}

type delayedExitRenderTmux struct {
	fakeReloadTmux
	exitRendered     bool
	earlySubmitCount int
}

type neverExitRenderTmux struct {
	fakeReloadTmux
	enterCount int
}

func (tmux *neverExitRenderTmux) Capture(context.Context, string, string) (string, error) {
	return "Codex\n› ", nil
}

func (tmux *neverExitRenderTmux) SendKey(_ context.Context, _, _, key string) error {
	if key == "Enter" && tmux.literal == "/exit" {
		tmux.enterCount++
	}
	return nil
}

func (tmux *delayedExitRenderTmux) Capture(context.Context, string, string) (string, error) {
	if tmux.literal == "/exit" {
		tmux.exitRendered = true
		return "Codex\n› /exit", nil
	}
	return "Codex\n› ", nil
}

func (tmux *delayedExitRenderTmux) SendKey(_ context.Context, _, _, key string) error {
	if key != "Enter" || tmux.literal != "/exit" {
		return nil
	}
	if !tmux.exitRendered {
		tmux.earlySubmitCount++
		return nil
	}
	tmux.dead = true
	return nil
}

func (tmux *delayedThenTmux) Capture(context.Context, string, string) (string, error) {
	marker := tmux.marker
	if marker == "" {
		marker = "❯"
	}
	if tmux.respawn == "" {
		if tmux.literal == "/exit" {
			return "Chat\n" + marker + " /exit", nil
		}
		return "Chat\n" + marker + " ", nil
	}
	tmux.ready = true
	draft := tmux.literal
	if draft == "" || draft == "/exit" {
		return "Chat\n" + marker + " ", nil
	}
	if tmux.submitted {
		// Submitted text remains in scrollback while the active composer is
		// empty. Submit proof must inspect the composer, not the whole pane.
		return marker + " " + draft + "\nWorking\n" + wrapComposer(marker, "", 60), nil
	}
	return "Chat\n" + wrapComposer(marker, draft, 60), nil
}

// wrapComposer renders a draft the way Claude and Codex actually draw one: the
// ❯/› marker on the FIRST line only, continuation lines indented beneath it,
// the block framed by the input box's horizontal rules.
//
// The fixture this replaced echoed one hardcoded 17-character prompt onto a
// single line, so it read the same whether the composer reader handled wrapping
// or not — green against correct code and against the one-line reader that
// could never confirm a real steer. It breaks lines at a fixed width, MID-word,
// which is the harsher of the two real shapes (Claude breaks at word boundaries
// until a single token is wider than the box — a path or a URL, the substance
// of most steers).
func wrapComposer(marker, text string, width int) string {
	rule := strings.Repeat("─", width)
	runes := []rune(text)
	rows := []string{}
	for start := 0; start < len(runes); start += width {
		end := start + width
		if end > len(runes) {
			end = len(runes)
		}
		rows = append(rows, string(runes[start:end]))
	}
	if len(rows) == 0 {
		rows = []string{""}
	}
	block := []string{rule, marker + " " + rows[0]}
	for _, row := range rows[1:] {
		block = append(block, "  "+row)
	}
	return strings.Join(append(block, rule), "\n")
}

func (tmux *delayedThenTmux) SendKey(_ context.Context, _, _, key string) error {
	if key != "Enter" {
		return nil
	}
	switch {
	case tmux.literal == "/exit":
		tmux.dead = true
	case tmux.literal != "":
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
	run, err := claudeRun(Request{
		Account:   2,
		Machine:   reloadTestMachine("", ""),
		SessionID: "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, variable := range []string{"CLAUDE_CODE_SESSION_ID", "CLAUDE_CONFIG_DIR", "FORCE_PROMPT_CACHING_5M"} {
		if !strings.Contains(run, variable) {
			t.Fatalf("run %q does not mention %s", run, variable)
		}
	}
	if !strings.Contains(run, "claude '--resume'") {
		t.Fatalf("run %q has no resume", run)
	}
}

// reloadTestMachine is a two-account roster whose account 2 is explicit, so a
// reload line has a config dir to state.
func reloadTestMachine(systemPrompt, home string) pfmconfig.Config {
	return pfmconfig.Config{
		Claude: pfmconfig.ClaudePrefs{PermissionMode: pfmconfig.PermissionBypass, SystemPrompt: systemPrompt},
		Accounts: []pfmconfig.Account{
			{ID: 1, ConfigDir: filepath.Join(home, ".claude"), Implicit: true},
			{ID: 2, ConfigDir: filepath.Join(home, ".cc", "2")},
		},
	}
}

// A reloaded chat is a RESUME, and a resume carries the same prompt material a
// fresh launch would: the reload constructor used to be a fourth independent
// spawn site with no idea the fleet had a configured system prompt, so a
// rebooted seat silently reverted to the CLI's own.
func TestClaudeRunCarriesTheConfiguredSystemPrompt(t *testing.T) {
	home := "/jail/home"
	professor, err := claudeRun(Request{
		Account:   2,
		Home:      home,
		Machine:   reloadTestMachine(pfmconfig.SystemPromptProfessor, home),
		SessionID: "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantFile := " --system-prompt-file " + action.Quote(action.ProfessorPromptPath(home))
	if !strings.Contains(professor, wantFile) {
		t.Fatalf("reloaded chat lost the professor prompt: %q lacks %q", professor, wantFile)
	}
	if !strings.Contains(professor, "--dangerously-skip-permissions") {
		t.Fatalf("reloaded chat lost the configured autonomy posture: %q", professor)
	}

	lean, err := claudeRun(Request{
		Account:   2,
		Home:      home,
		Machine:   reloadTestMachine(pfmconfig.SystemPromptLean, home),
		SessionID: "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(lean, " CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT=1 ") {
		t.Fatalf("reloaded chat lost the lean prompt arm: %q", lean)
	}

	production, err := claudeRun(Request{
		Account:   2,
		Home:      home,
		Machine:   reloadTestMachine(pfmconfig.SystemPromptProduction, home),
		SessionID: "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(production, "--system-prompt-file") ||
		strings.Contains(production, "CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT=1") {
		t.Fatalf("production mode invented prompt material: %q", production)
	}
	if !strings.Contains(production, " -u CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT ") {
		t.Fatalf("reload no longer strips an inherited lean arm: %q", production)
	}
}

func TestCodexRunUsesTheSelectedHomeAndRosterPolicy(t *testing.T) {
	run, err := engineRun(Request{
		Engine:      "cx",
		Account:     9,
		AccountHome: "/jail/codex/9",
		CodexBinary: "/opt/codex safe",
		CodexYolo:   false,
		SessionID:   "019ff700-0000-7000-8000-000000000001",
	})
	if err != nil {
		t.Fatal(err)
	}
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
	run, err := engineRun(Request{Engine: pfmengine.Opencode})
	if err != nil {
		t.Fatal(err)
	}
	if run != "" {
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
			Engine:     pfmengine.Claude,
			SocketPath: "/tmp/tmux-1000/probe-reload",
			Pane:       "%7",
			PanePID:    700,
			SessionID:  "11111111-1111-4111-8111-111111111111",
			CWD:        "/jail/project",
			Account:    2,
			AccountIDs: []int{2},
			Machine:    reloadTestMachine("", "/jail/home"),
			Cache1H:    false,
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
		"claude '--resume'",
		"11111111-1111-4111-8111-111111111111",
	} {
		if !strings.Contains(tmux.respawn, want) {
			t.Fatalf("respawn %q lacks %q", tmux.respawn, want)
		}
	}
}

func TestRunWaitsForExitTextToRenderBeforeSubmitting(t *testing.T) {
	tmux := &delayedExitRenderTmux{}
	_, err := Run(
		context.Background(),
		Request{
			Engine: pfmengine.Codex, SocketPath: "/tmp/tmux-1000/probe-reload-render", Pane: "%7",
			PanePID: 700, SessionID: "019ff700-0000-7000-8000-000000000001", CWD: "/jail/project",
			Account: 1, AccountIDs: []int{1}, AccountHome: "/jail/codex/1",
		},
		Options{SIDDir: t.TempDir(), Delay: -1, Poll: -1, ExitTries: 2},
		tmux,
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if tmux.earlySubmitCount != 0 || !tmux.exitRendered || tmux.respawn == "" {
		t.Fatalf(
			"early submits=%d rendered=%t respawn=%q",
			tmux.earlySubmitCount, tmux.exitRendered, tmux.respawn,
		)
	}
}

func TestRunRefusesBlindExitWhenTextNeverRenders(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	tmux := &neverExitRenderTmux{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := Run(
		ctx,
		Request{
			Engine: pfmengine.Codex, SocketPath: filepath.Join(t.TempDir(), "fake-codex-socket"), Pane: "%7",
			PanePID: 700, SessionID: "019ff700-0000-7000-8000-000000000001", CWD: "/jail/project",
			Account: 1, AccountIDs: []int{1}, AccountHome: "/jail/codex/1",
		},
		Options{SIDDir: t.TempDir(), Delay: -1, Poll: -1, ExitTries: 1},
		tmux,
		nil,
		nil,
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("never-rendered /exit error=%v, want context deadline while waiting for visible text", err)
	}
	if tmux.enterCount != 0 || tmux.respawn != "" {
		t.Fatalf("blind exit mutation: enters=%d respawn=%q", tmux.enterCount, tmux.respawn)
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

func TestDeliverThenRecognizesTheCodexComposerMarker(t *testing.T) {
	tmux := &delayedThenTmux{marker: "›"}
	tmux.respawn = "codex"
	proc := fakeReloadProc{
		pids: []int{801},
		argv: map[int][]string{801: {"codex"}},
		stat: map[int]gather.ProcStat{801: {ParentPID: 700}},
	}
	err := deliverThen(
		context.Background(),
		Request{
			Engine: pfmengine.Codex, SocketPath: "/tmp/tmux-1000/probe-codex-then", Pane: "%7",
			PanePID: 700, Then: "continue the task",
		},
		Options{ThenTries: 2},
		tmux,
		proc,
		io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !tmux.ready || !tmux.submitted {
		t.Fatalf("ready=%t submitted=%t", tmux.ready, tmux.submitted)
	}
}

func TestLastComposerLineFindsCodexCommandAbovePopupWhitespace(t *testing.T) {
	capture := "Codex\n› /exit\n" + strings.Repeat("\n", 30)
	if got := lastComposerLine(capture); !strings.Contains(got, "/exit") {
		t.Fatalf("lastComposerLine()=%q, want the visible Codex command", got)
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

// TestDeliverThenSubmitsAPromptThatWrapsAcrossComposerLines reproduces the
// defect the operator hit on every account switch: the steer landed in the
// composer and pfm refused to press Enter, so a human had to.
//
// deliverThen proves delivery by the prompt's TAIL — the half that proves
// nothing was truncated in transit — and read that proof off the composer's
// MARKER line alone. Claude prints the marker on the first line of a wrapped
// draft, so the tail of any prompt longer than one row was unreachable and the
// proof could never be satisfied. Every real steer is longer than one row.
func TestDeliverThenSubmitsAPromptThatWrapsAcrossComposerLines(t *testing.T) {
	const then = "Continue the wave: read the refine checkpoint end to end, " +
		"execute the remaining round, and write the zero-gap spec to the queue " +
		"path before presenting the user gate."
	tmux := &delayedThenTmux{}
	tmux.respawn = "claude"
	proc := fakeReloadProc{
		pids: []int{801},
		argv: map[int][]string{801: {"claude"}},
		stat: map[int]gather.ProcStat{801: {ParentPID: 700}},
	}
	err := deliverThen(
		context.Background(),
		Request{
			Engine: pfmengine.Claude, SocketPath: "/tmp/tmux-1000/probe-wrapped-then", Pane: "%7",
			PanePID: 700, Then: then,
		},
		Options{ThenTries: 2},
		tmux,
		proc,
		io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !tmux.submitted {
		t.Fatal("a wrapped --then prompt was never submitted — Enter was withheld from a prompt that had fully landed")
	}
}

// TestComposerTextReadsAWrappedDraftAndStopsAtTheBoxRule pins the render taken
// from a live pane at the moment of a refusal: marker plus non-breaking space
// on line one, continuations indented beneath, the box rule closing the block,
// status rows below it that must stay OUT of the read.
func TestComposerTextReadsAWrappedDraftAndStopsAtTheBoxRule(t *testing.T) {
	rule := strings.Repeat("─", 40)
	capture := strings.Join([]string{
		"Chat",
		rule,
		"❯ \u00a0Continue the reload-then reproduction. This prompt is",
		"  deliberately long enough to wrap across several rendered",
		"  composer lines. END OF REPRO PROMPT MARKER.",
		rule,
		"  bypass permissions on (shift+tab to cycle)",
	}, "\n")
	got := composerText(capture)
	if !strings.Contains(got, "END OF REPRO PROMPT MARKER.") {
		t.Fatalf("composerText lost the wrapped tail: %q", got)
	}
	if strings.Contains(got, "bypass permissions") {
		t.Fatalf("composerText read past the box rule into the status rows: %q", got)
	}
}
