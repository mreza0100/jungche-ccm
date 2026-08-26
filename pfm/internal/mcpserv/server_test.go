package mcpserv

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"hostops/pfm/internal/paths"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var originalTestHome = os.Getenv("HOME")

type protocolClient struct {
	clientSession *mcp.ClientSession
	serverSession *mcp.ServerSession
}

func connectInMemory(
	t *testing.T,
	server *mcp.Server,
) protocolClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "pfm-test",
		Version: "test",
	}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
	})
	return protocolClient{
		clientSession: clientSession,
		serverSession: serverSession,
	}
}

func callTool[T any](
	t *testing.T,
	session *mcp.ClientSession,
	name string,
	arguments any,
) T {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if result.IsError {
		t.Fatalf("%s returned tool error: %#v", name, result.Content)
	}
	content, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output T
	if err := json.Unmarshal(content, &output); err != nil {
		t.Fatalf("%s structured output %s: %v", name, content, err)
	}
	return output
}

func TestChatStatusSummaryUsesCanonicalCommandAndReturnsField(t *testing.T) {
	var gotArgs []string
	service := newService("test", &backend{dispatch: func(_ context.Context, args []string, stdout, _ io.Writer) int {
		gotArgs = append([]string(nil), args...)
		_, _ = io.WriteString(stdout, `{"name":"seat","state":"idle","idle_seconds":2,"engine":"cc","summary":"done","summary_cached":true}`+"\n")
		return 0
	}})
	protocol := connectInMemory(t, service.Server())
	output := callTool[StatusOutput](t, protocol.clientSession, "chat_status", StatusInput{
		Target: "seat", Summary: true, Engine: "codex", Model: "test-model",
	})
	if strings.Join(gotArgs, " ") != "chat status seat --json --summary --engine codex --model test-model" {
		t.Fatalf("dispatch args=%q", gotArgs)
	}
	if output.Summary != "done" || !output.SummaryCached {
		t.Fatalf("status output=%+v", output)
	}
}

func setupBackendFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	claude := filepath.Join(root, "claude")
	codex := filepath.Join(root, "codex")
	sid := filepath.Join(root, "sid")
	proc := filepath.Join(root, "proc")
	tmux := filepath.Join(root, "tmux-"+strconv.Itoa(os.Getuid()))
	for _, directory := range []string{
		home, claude, codex, sid, proc, tmux,
		filepath.Join(claude, "project-alpha"),
		filepath.Join(claude, "project-beta"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeJSONL(t, filepath.Join(claude, "project-alpha", "alpha.jsonl"), []any{
		map[string]any{
			"type": "user", "cwd": "/work/alpha",
			"message": map[string]any{"content": "alpha unique prompt"},
		},
		map[string]any{
			"type": "assistant",
			"message": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": "alpha answer"},
			}},
		},
	})
	writeJSONL(t, filepath.Join(claude, "project-beta", "beta.jsonl"), []any{
		map[string]any{
			"type": "user", "cwd": "/work/beta",
			"message": map[string]any{"content": "beta unique prompt"},
		},
		map[string]any{
			"type":    "assistant",
			"message": map[string]any{"content": "beta answer"},
		},
	})
	t.Setenv("HOME", home)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_TMPDIR", root)
	t.Setenv(paths.EnvHome, home)
	t.Setenv(paths.EnvDB, filepath.Join(root, "state", "fleet.db"))
	t.Setenv(paths.EnvSIDDir, sid)
	t.Setenv(paths.EnvClaudeRoots, claude)
	t.Setenv(paths.EnvCodexRoot, codex)
	t.Setenv(paths.EnvTmuxDir, tmux)
	t.Setenv(paths.EnvProcRoot, proc)
	t.Setenv("PFM_CODEX_AVAILABLE", "0")
	return root
}

func writeJSONL(t *testing.T, path string, records []any) {
	t.Helper()
	var content bytes.Buffer
	encoder := json.NewEncoder(&content)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, content.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMCPHandshakeAndAllToolsOverJailedStdio(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}
	jail := newStdioJail(t)
	binary := buildFleetBinary(t, jail.root)
	command := exec.Command(binary, "--config", writeEnabledMCPConfig(t, jail.root), "mcp")
	command.Env = jail.environment()
	var serverStderr bytes.Buffer
	command.Stderr = &serverStderr
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "stdio-test",
		Version: "test",
	}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, &mcp.CommandTransport{
		Command:           command,
		TerminateDuration: 2 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("stdio handshake: %v\nstderr:\n%s", err, serverStderr.String())
	}
	defer session.Close()

	var toolNames []string
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		toolNames = append(toolNames, tool.Name)
	}
	sort.Strings(toolNames)
	wantTools := ToolNames()
	sort.Strings(wantTools)
	if !reflect.DeepEqual(toolNames, wantTools) {
		t.Fatalf("tools = %q, want %q", toolNames, wantTools)
	}

	listed := callTool[LSOutput](t, session, "chat_ls", LSInput{All: true})
	if listed.Count != 1 {
		t.Fatalf("chat_ls = %+v", listed)
	}
	wantRow := ChatRow{
		Session: "fixture-session",
		ID:      "fixture",
		Engine:  "cc",
		State:   "idle",
		Dir:     "/work/fixture",
		Project: "fixture",
		Name:    "Fixture Chat",
		Account: 1,
		Kind:    "live-claude",
		Socket:  jail.socket,
		Pane:    jail.pane,
	}
	if !reflect.DeepEqual(listed.Rows[0], wantRow) {
		t.Fatalf("chat_ls row = %+v, want %+v", listed.Rows[0], wantRow)
	}

	resolved := callTool[ResolveOutput](t, session, "chat_resolve", ResolveInput{
		Kind: "label",
		Name: "Fixture Label",
	})
	if resolved.Status != "ok" ||
		resolved.Code != 0 ||
		resolved.SocketPath != filepath.Join(jail.tmuxDir, jail.socket) ||
		resolved.Pane != jail.pane {
		t.Fatalf("chat_resolve = %+v", resolved)
	}

	captured := callTool[CaptureOutput](t, session, "chat_capture", CaptureInput{
		Target:    "Fixture Label",
		TailLines: 10,
	})
	if captured.Status != "ok" ||
		!strings.Contains(captured.Text, "🔖 Fixture Label") ||
		!strings.Contains(captured.Text, "❯") {
		t.Fatalf("chat_capture = %+v", captured)
	}

	found := callTool[FindOutput](t, session, "chat_find", FindInput{
		Excerpt: "secret middle",
	})
	if found.Count != 1 ||
		found.Candidates[0].ID != "fixture" ||
		!found.Candidates[0].Confirmed {
		t.Fatalf("chat_find = %+v", found)
	}

	read := callTool[ReadOutput](t, session, "chat_read", ReadInput{
		Source:   "fixture",
		LastN:    2,
		MaxBytes: 1000,
	})
	wantTurns := []Turn{
		{Role: "assistant", Text: "Assistant visible"},
		{Role: "user", Text: "Final prompt"},
	}
	if !reflect.DeepEqual(read.Turns, wantTurns) ||
		read.Count != 2 ||
		!read.Truncated {
		t.Fatalf("chat_read = %+v, want turns %+v", read, wantTurns)
	}

	injected := callTool[InjectOutput](t, session, "chat_inject", InjectInput{
		Target:  "Fixture Label",
		Message: "hello fixture",
	})
	if injected.Status != "delivered" ||
		injected.Code != 0 ||
		!injected.Typed ||
		!strings.Contains(injected.Proof, "USER:hello fixture") {
		t.Fatalf("chat_inject = %+v\nstderr:\n%s", injected, serverStderr.String())
	}
	after := callTool[CaptureOutput](t, session, "chat_capture", CaptureInput{
		Target: "Fixture Label",
	})
	if !strings.Contains(after.Text, "USER:hello fixture") {
		t.Fatalf("post-inject capture = %+v", after)
	}

	selectorBefore := callTool[CaptureOutput](
		t,
		session,
		"chat_capture",
		CaptureInput{Target: jail.selectorSession},
	)
	selector := callTool[InjectOutput](t, session, "chat_inject", InjectInput{
		Target:  jail.selectorSession,
		Message: "must not type",
	})
	selectorAfter := callTool[CaptureOutput](
		t,
		session,
		"chat_capture",
		CaptureInput{Target: jail.selectorSession},
	)
	if selector.Code != 6 ||
		selector.Typed ||
		selector.Status != "refused" ||
		selectorBefore.Text != selectorAfter.Text ||
		strings.Contains(selectorAfter.Text, "MISTYPED") {
		t.Fatalf(
			"selector guard result=%+v before=%q after=%q",
			selector,
			selectorBefore.Text,
			selectorAfter.Text,
		)
	}

	busy := callTool[InjectOutput](t, session, "chat_inject", InjectInput{
		Target:  jail.busySession,
		Message: "wait until idle",
	})
	busyAfter := callTool[CaptureOutput](
		t,
		session,
		"chat_capture",
		CaptureInput{Target: jail.busySession},
	)
	if busy.Code != 0 || busy.Status != "queued" ||
		!busy.Typed || !busy.Busy ||
		!strings.Contains(busyAfter.Text, "QUEUED:wait until idle") ||
		strings.Contains(busyAfter.Text, "MISTYPED") ||
		strings.Contains(busyAfter.Text, "CONTROL-S-ERROR") {
		t.Fatalf(
			"busy queue result=%+v after=%q",
			busy,
			busyAfter.Text,
		)
	}

	numberedDraft := callTool[InjectOutput](
		t,
		session,
		"chat_inject",
		InjectInput{
			Target:  jail.codexSession,
			Message: " next",
		},
	)
	if numberedDraft.Code != 6 || numberedDraft.Typed ||
		!strings.Contains(numberedDraft.Message, "draft that will not stash") {
		t.Fatalf("numbered Codex draft result=%+v", numberedDraft)
	}
	codexLongWithDraft := callTool[InjectOutput](
		t,
		session,
		"chat_inject",
		InjectInput{
			Target:  jail.codexSession,
			Message: strings.Repeat("x", 1501),
		},
	)
	if codexLongWithDraft.Code != 6 ||
		codexLongWithDraft.Typed ||
		codexLongWithDraft.Status != "refused" ||
		!strings.Contains(codexLongWithDraft.Message, "draft that will not stash") {
		t.Fatalf("Codex long-body draft guard result=%+v", codexLongWithDraft)
	}

	oversizeBody := strings.Repeat("x", 1<<20)
	oversize := callTool[InjectOutput](t, session, "chat_inject", InjectInput{
		Target:  "Fixture Label",
		Message: oversizeBody,
	})
	if oversize.Code != 0 || !oversize.Typed || oversize.Status != "delivered" ||
		oversize.AutoFilePath == "" ||
		!strings.Contains(oversize.Message, "AUTO-FILE") ||
		!strings.Contains(oversize.Proof, "read "+oversize.AutoFilePath+" fully") {
		t.Fatalf("oversize chat_inject = %+v", oversize)
	}
	storedOversize, err := os.ReadFile(oversize.AutoFilePath)
	if err != nil || string(storedOversize) != oversizeBody {
		t.Fatalf("oversize body bytes=%d err=%v", len(storedOversize), err)
	}
}

func TestChatKeysMCPRejectsUnknownNamesBeforeResolution(t *testing.T) {
	setupBackendFixture(t)
	resolved, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewConfigured("test", nil, Runtime{Paths: resolved})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	protocol := connectInMemory(t, service.Server())
	result, err := protocol.clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "chat_keys",
		Arguments: KeysInput{Target: "not-used", Keys: []string{"Esc"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("unknown key result = %#v, want tool error", result)
	}
	content, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "not a tmux key") {
		t.Fatalf("unknown key error = %s", content)
	}
}

type stdioJail struct {
	root            string
	home            string
	claude          string
	codex           string
	sid             string
	proc            string
	tmuxBase        string
	tmuxDir         string
	database        string
	socket          string
	session         string
	pane            string
	transcript      string
	selectorSocket  string
	selectorSession string
	busySocket      string
	busySession     string
	codexSocket     string
	codexSession    string
	sockets         []string
}

func newStdioJail(t *testing.T) *stdioJail {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "ccfm")
	if err != nil {
		t.Fatal(err)
	}
	jail := &stdioJail{
		root:            root,
		home:            filepath.Join(root, "h"),
		claude:          filepath.Join(root, "c"),
		codex:           filepath.Join(root, "x"),
		sid:             filepath.Join(root, "s"),
		proc:            filepath.Join(root, "p"),
		tmuxBase:        filepath.Join(root, "t"),
		database:        filepath.Join(root, "d", "fleet.db"),
		socket:          "cc-1700000000-1-1",
		session:         "fixture-session",
		selectorSocket:  "cc-1700000001-1-2",
		selectorSession: "selector-session",
		busySocket:      "cc-1700000002-1-3",
		busySession:     "busy-session",
		codexSocket:     "cx-1700000003-1-4",
		codexSession:    "codex-draft-session",
	}
	jail.tmuxDir = filepath.Join(
		jail.tmuxBase,
		"tmux-"+strconv.Itoa(os.Getuid()),
	)
	project := filepath.Join(jail.claude, "project")
	for _, directory := range []string{
		jail.home,
		project,
		jail.codex,
		jail.sid,
		jail.proc,
		jail.tmuxDir,
		filepath.Dir(jail.database),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	jail.transcript = filepath.Join(project, "fixture.jsonl")
	writeJSONL(t, jail.transcript, []any{
		map[string]any{
			"type":        "custom-title",
			"customTitle": "Fixture Chat",
		},
		map[string]any{
			"type": "user", "cwd": "/work/fixture",
			"message": map[string]any{"content": "secret middle prompt"},
		},
		map[string]any{
			"type": "assistant",
			"message": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": "Assistant visible"},
				map[string]any{"type": "tool_result", "content": "killed tool"},
			}},
		},
		map[string]any{
			"type": "user", "cwd": "/work/fixture",
			"message": map[string]any{"content": "[Request interrupted by user]"},
		},
		map[string]any{
			"type": "user", "cwd": "/work/fixture",
			"message": map[string]any{"content": "Final prompt"},
		},
	})
	uiPath := filepath.Join(root, "ui.py")
	ui := `import os, sys, tty
tty.setraw(0)
mode = sys.argv[1]
label = sys.argv[2]
marker = sys.argv[3]
initial = sys.argv[4]
sys.stdout.write("🔖 " + label + " │ 🥇\n")
if mode == "selector":
    sys.stdout.write("❯ 1. Allow\n  2. Deny\n")
elif mode == "busy":
    sys.stdout.write("Working (2s · 9 tokens)\n" + marker + " ")
else:
    sys.stdout.write(marker + " " + initial)
sys.stdout.flush()
buf = bytearray(initial.encode("utf-8"))
while True:
    ch = os.read(0, 1)
    if not ch:
        break
    if mode == "selector":
        sys.stdout.write("\nMISTYPED:" + repr(ch) + "\n")
        sys.stdout.flush()
        continue
    if mode == "busy" and ch == b"\x13":
        sys.stdout.write("\nCONTROL-S-ERROR\n")
        sys.stdout.flush()
        continue
    if ch == b"\x13":
        if marker == "❯":
            buf.clear()
            sys.stdout.write("\r\x1b[2K" + marker + " ")
            sys.stdout.flush()
    elif ch in (b"\r", b"\n"):
        if buf:
            text = bytes(buf).decode("utf-8", "replace")
            if mode == "busy":
                sys.stdout.write("\nQUEUED:" + text + "\nPress up to edit queued messages\n" + marker + " ")
            else:
                sys.stdout.write("\nUSER:" + text + "\n" + marker + " ")
            sys.stdout.flush()
            buf.clear()
    elif ch == b"\x1b":
        buf.clear()
        sys.stdout.write("\r\x1b[2K" + marker + " ")
        sys.stdout.flush()
    else:
        buf.extend(ch)
        sys.stdout.buffer.write(ch)
        sys.stdout.flush()
`
	if err := os.WriteFile(uiPath, []byte(ui), 0o700); err != nil {
		t.Fatal(err)
	}
	jail.startUI(t, jail.socket, jail.session, uiPath, "normal", "Fixture Label", "❯", "")
	jail.startUI(
		t,
		jail.selectorSocket,
		jail.selectorSession,
		uiPath,
		"selector",
		"Selector Label",
		"❯",
		"",
	)
	jail.startUI(
		t,
		jail.busySocket,
		jail.busySession,
		uiPath,
		"busy",
		"Busy Label",
		"❯",
		"",
	)
	jail.startUI(
		t,
		jail.codexSocket,
		jail.codexSession,
		uiPath,
		"normal",
		"Codex Draft",
		"›",
		"1. draft",
	)
	paneCommand := exec.Command(
		"tmux",
		"-L", jail.socket,
		"display-message",
		"-p",
		"-t", jail.session,
		"#{pane_id}",
	)
	paneCommand.Env = jail.environment()
	output, err := paneCommand.Output()
	if err != nil {
		t.Fatal(err)
	}
	jail.pane = strings.TrimSpace(string(output))
	if err := os.WriteFile(
		filepath.Join(jail.sid, jail.socket+"."+jail.pane),
		[]byte(jail.transcript),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, socket := range jail.sockets {
			kill := exec.Command("tmux", "-L", socket, "kill-server")
			kill.Env = jail.environment()
			_ = kill.Run()
		}
		_ = os.RemoveAll(root)
	})
	return jail
}

func (jail *stdioJail) startUI(
	t *testing.T,
	socket, session, script, mode, label, marker, initial string,
) {
	t.Helper()
	command := exec.Command(
		"tmux",
		"-f", "/dev/null",
		"-L", socket,
		"new-session",
		"-d",
		"-s", session,
		"-n", "fixture",
		"python3", script, mode, label, marker, initial,
	)
	command.Env = jail.environment()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start jailed tmux %q: %v: %s", socket, err, output)
	}
	jail.sockets = append(jail.sockets, socket)
}

func (jail *stdioJail) environment() []string {
	return append(os.Environ(),
		"HOME="+jail.home,
		"TMUX=",
		"TMUX_TMPDIR="+jail.tmuxBase,
		paths.EnvHome+"="+jail.home,
		paths.EnvDB+"="+jail.database,
		paths.EnvSIDDir+"="+jail.sid,
		paths.EnvClaudeRoots+"="+jail.claude,
		paths.EnvCodexRoot+"="+jail.codex,
		paths.EnvTmuxDir+"="+jail.tmuxDir,
		paths.EnvProcRoot+"="+jail.proc,
		"PFM_CODEX_AVAILABLE=0",
	)
}

func buildFleetBinary(t *testing.T, destination string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(destination, "pfm-test")
	command := exec.Command(
		"go",
		"build",
		"-trimpath",
		"-o",
		binary,
		"./cmd/pfm",
	)
	command.Dir = root
	command.Env = append(
		os.Environ(),
		"HOME="+originalTestHome,
		"CGO_ENABLED=0",
		"GOTOOLCHAIN=local",
		"GOTELEMETRY=off",
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build pfm: %v\n%s", err, output)
	}
	return binary
}

func TestMCPStressSequentialCallsNoLeaks(t *testing.T) {
	setupBackendFixture(t)
	service := newFixtureService(t)
	defer service.Close()
	client := connectInMemory(t, service.Server())
	_ = callTool[FindOutput](t, client.clientSession, "chat_find", FindInput{
		Excerpt: "alpha unique",
	})
	runtime.GC()
	beforeGoroutines := runtime.NumGoroutine()
	beforeFDs := openFDs(t)
	beforeRSS := rssBytes(t)
	started := time.Now()
	for iteration := 0; iteration < 200; iteration++ {
		output := callTool[FindOutput](
			t,
			client.clientSession,
			"chat_find",
			FindInput{Excerpt: "alpha unique"},
		)
		if output.Count != 1 || output.Candidates[0].ID != "alpha" {
			t.Fatalf("iteration %d cross-talk: %+v", iteration, output)
		}
	}
	elapsed := time.Since(started)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	afterGoroutines := runtime.NumGoroutine()
	afterFDs := openFDs(t)
	afterRSS := rssBytes(t)
	if delta := afterGoroutines - beforeGoroutines; delta > 4 {
		t.Fatalf("goroutine leak: before=%d after=%d", beforeGoroutines, afterGoroutines)
	}
	if delta := afterFDs - beforeFDs; delta > 2 {
		t.Fatalf("fd leak: before=%d after=%d", beforeFDs, afterFDs)
	}
	if delta := afterRSS - beforeRSS; delta > 32<<20 {
		t.Fatalf("RSS grew by %d bytes", delta)
	}
	if os.Getenv("PFM_STRESS_STRICT") == "1" && elapsed > 5*time.Second {
		t.Fatalf("strict sequential MCP stress took %s", elapsed)
	}
	t.Logf(
		"STRESS mcp_sequential calls=200 elapsed_ms=%d goroutines=%d->%d fds=%d->%d rss_bytes=%d->%d",
		elapsed.Milliseconds(),
		beforeGoroutines,
		afterGoroutines,
		beforeFDs,
		afterFDs,
		beforeRSS,
		afterRSS,
	)
}

func TestMCPStressConcurrentEightClientsNoCrossTalk(t *testing.T) {
	setupBackendFixture(t)
	service := newFixtureService(t)
	defer service.Close()
	clients := make([]protocolClient, 8)
	for index := range clients {
		clients[index] = connectInMemory(t, service.Server())
	}
	started := time.Now()
	var wait sync.WaitGroup
	errors := make(chan error, 8)
	for worker := range clients {
		wait.Add(1)
		go func() {
			defer wait.Done()
			query := "alpha unique"
			want := "alpha"
			if worker%2 == 1 {
				query = "beta unique"
				want = "beta"
			}
			for iteration := 0; iteration < 25; iteration++ {
				ctx, cancel := context.WithTimeout(
					context.Background(),
					15*time.Second,
				)
				result, err := clients[worker].clientSession.CallTool(
					ctx,
					&mcp.CallToolParams{
						Name: "chat_find",
						Arguments: FindInput{
							Excerpt: query,
						},
					},
				)
				cancel()
				if err != nil {
					errors <- fmt.Errorf("worker %d: %w", worker, err)
					return
				}
				content, _ := json.Marshal(result.StructuredContent)
				var output FindOutput
				if err := json.Unmarshal(content, &output); err != nil ||
					output.Count != 1 ||
					output.Candidates[0].ID != want {
					errors <- fmt.Errorf(
						"worker %d cross-talk: %s err=%v",
						worker,
						content,
						err,
					)
					return
				}
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	elapsed := time.Since(started)
	if os.Getenv("PFM_STRESS_STRICT") == "1" && elapsed > 8*time.Second {
		t.Fatalf("strict concurrent MCP stress took %s", elapsed)
	}
	t.Logf(
		"STRESS mcp_concurrent clients=8 calls=200 elapsed_ms=%d cross_talk=0",
		elapsed.Milliseconds(),
	)
}

func TestMCPAdversarialUnknownAndHugeArguments(t *testing.T) {
	setupBackendFixture(t)
	service := newFixtureService(t)
	defer service.Close()
	client := connectInMemory(t, service.Server())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	unknown, unknownErr := client.clientSession.CallTool(
		ctx,
		&mcp.CallToolParams{Name: "unknown_tool"},
	)
	if unknownErr == nil && (unknown == nil || !unknown.IsError) {
		t.Fatalf("unknown tool returned success: result=%+v error=%v", unknown, unknownErr)
	}
	huge := callTool[InjectOutput](t, client.clientSession, "chat_inject", InjectInput{
		Target:  "missing",
		Message: strings.Repeat("x", 1<<20),
	})
	if huge.Code != 4 || huge.Typed || huge.Status != "refused" ||
		!strings.Contains(huge.Message, "matched no live chat") {
		t.Fatalf("huge injection = %+v", huge)
	}
}

func TestMCPMalformedFrameReturnsJSONRPCError(t *testing.T) {
	root := setupBackendFixture(t)
	binary := buildFleetBinary(t, root)
	command := exec.Command(binary, "--config", writeEnabledMCPConfig(t, root), "mcp")
	command.Env = os.Environ()
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = stdin.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
	}()
	if _, err := io.WriteString(
		stdin,
		"{malformed json-rpc}\n"+
			`{"jsonrpc":"2.0","id":7,"method":"unknown/method"}`+"\n",
	); err != nil {
		t.Fatal(err)
	}
	lineChannel := make(chan []string, 1)
	go func() {
		reader := bufio.NewReader(stdout)
		lines := make([]string, 0, 2)
		for len(lines) < 2 {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				break
			}
			var envelope struct {
				Method string `json:"method"`
				Error  any    `json:"error"`
			}
			if json.Unmarshal([]byte(line), &envelope) == nil &&
				envelope.Method != "" &&
				envelope.Error == nil {
				// The SDK may publish tools/list_changed between request
				// responses. Notifications have no request ID and are not a
				// malformed-frame response.
				continue
			}
			lines = append(lines, line)
		}
		lineChannel <- lines
	}()
	select {
	case lines := <-lineChannel:
		if len(lines) != 2 {
			t.Fatalf("malformed frame responses = %q", lines)
		}
		for index, line := range lines {
			var response struct {
				JSONRPC string `json:"jsonrpc"`
				Error   any    `json:"error"`
			}
			if err := json.Unmarshal([]byte(line), &response); err != nil ||
				response.JSONRPC != "2.0" ||
				response.Error == nil {
				t.Fatalf(
					"malformed frame response[%d] = %q parsed=%+v err=%v stderr=%s",
					index,
					line,
					response,
					err,
					stderr.String(),
				)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("server did not answer malformed frame; stderr=%s", stderr.String())
	}
}

func writeEnabledMCPConfig(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "mcp-enabled.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"mcp":{"servers":{"chat":{"enabled":true}}}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestChatReadBudgetsAndJunkFilter(t *testing.T) {
	root := setupBackendFixture(t)
	path := filepath.Join(root, "claude", "project-alpha", "budget.jsonl")
	writeJSONL(t, path, []any{
		map[string]any{
			"type": "user", "cwd": "/work/alpha",
			"message": map[string]any{"content": "<system-reminder>killed"},
		},
		map[string]any{
			"type": "user", "cwd": "/work/alpha",
			"message": map[string]any{"content": "first visible"},
		},
		map[string]any{
			"type":    "assistant",
			"message": map[string]any{"content": strings.Repeat("z", 100)},
		},
		map[string]any{
			"type": "user", "cwd": "/work/alpha",
			"message": map[string]any{"content": "last visible"},
		},
	})
	service := newFixtureService(t)
	defer service.Close()
	client := connectInMemory(t, service.Server())
	output := callTool[ReadOutput](t, client.clientSession, "chat_read", ReadInput{
		Source:   "budget",
		LastN:    3,
		MaxBytes: 30,
	})
	if output.Bytes > 30 || !output.Truncated {
		t.Fatalf("budget output = %+v", output)
	}
	for _, turn := range output.Turns {
		if strings.Contains(turn.Text, "system-reminder") {
			t.Fatalf("junk prompt leaked: %+v", output)
		}
	}
}

func openFDs(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("cannot count fds: %v", err)
	}
	return len(entries)
}

func rssBytes(t *testing.T) int64 {
	t.Helper()
	content, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		t.Skipf("cannot read RSS: %v", err)
	}
	fields := strings.Fields(string(content))
	if len(fields) < 2 {
		t.Fatalf("malformed statm: %q", content)
	}
	pages, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return pages * int64(os.Getpagesize())
}
