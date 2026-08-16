package inject

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"hostops/pfm/internal/resolve"
)

type fakeResolver struct {
	socket string
	target string
	code   int
	detail string
}

func (fake fakeResolver) Resolve(
	_ context.Context,
	_ resolve.Kind,
	_ string,
) (resolve.Outcome, error) {
	stdout := ""
	if fake.code == 0 {
		stdout = fake.socket + "\t" + fake.target + "\n"
	}
	return resolve.Outcome{
		Code:   fake.code,
		Stdout: stdout,
		Stderr: fake.detail,
	}, nil
}

type fakeTmux struct {
	mu            sync.Mutex
	capture       string
	styled        string
	literal       string
	literals      []string
	pasted        bool
	windowName    string
	paneCommand   string
	keys          []string
	dead          bool
	busyUntilEsc  bool
	stashClears   bool
	submitOnEnter bool
	inMode        bool
}

// fakeSpawner records the detached --then waiter instead of starting one.
type fakeSpawner struct {
	mu    sync.Mutex
	calls []SteerSpawn
	err   error
}

func (fake *fakeSpawner) Spawn(_ context.Context, request SteerSpawn) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.err != nil {
		return fake.err
	}
	fake.calls = append(fake.calls, request)
	return nil
}

func (fake *fakeSpawner) spawned() []SteerSpawn {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return append([]SteerSpawn(nil), fake.calls...)
}

func (fake *fakeTmux) Capture(
	_ context.Context,
	_, _ string,
	styled bool,
	_ int,
) (string, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.dead {
		return "", errors.New("dead")
	}
	if styled && fake.styled != "" {
		return fake.styled, nil
	}
	return fake.capture, nil
}

func (fake *fakeTmux) SendLiteral(
	_ context.Context,
	_, _ string,
	text string,
) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.dead {
		return errors.New("dead")
	}
	fake.literal = text
	fake.literals = append(fake.literals, text)
	marker := "❯"
	if strings.Contains(fake.capture, "›") &&
		!strings.Contains(fake.capture, "❯") {
		marker = "›"
	}
	fake.capture += "\n" + marker + " " + text
	return nil
}

func (fake *fakeTmux) SendPaste(
	ctx context.Context,
	socketPath, target, text string,
) error {
	fake.mu.Lock()
	fake.pasted = true
	fake.mu.Unlock()
	return fake.SendLiteral(ctx, socketPath, target, text)
}

func (fake *fakeTmux) SendKey(
	_ context.Context,
	_, _ string,
	key string,
) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.dead {
		return errors.New("dead")
	}
	fake.keys = append(fake.keys, key)
	switch key {
	case "Escape":
		if fake.busyUntilEsc {
			fake.capture = "turn interrupted\n❯ "
			fake.busyUntilEsc = false
		}
	case "C-s":
		if fake.stashClears {
			fake.capture = "draft stashed\n❯ "
		}
	case "Enter":
		if fake.submitOnEnter && fake.literal != "" {
			fake.capture = "USER: " + fake.literal + "\n❯ "
			fake.literal = ""
		}
	}
	return nil
}

func (fake *fakeTmux) CancelCopyMode(
	context.Context,
	string,
	string,
) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.inMode = false
	return nil
}

func (fake *fakeTmux) PaneInMode(
	context.Context,
	string,
	string,
) (bool, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.inMode, nil
}

func (fake *fakeTmux) PaneCommand(
	context.Context,
	string,
	string,
) (string, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.paneCommand, nil
}

func (*fakeTmux) CurrentSession(context.Context, string) (string, error) {
	return "sender-session", nil
}

func (fake *fakeTmux) WindowName(
	context.Context,
	string,
	string,
) (string, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.windowName, nil
}

func newTestEngine(t *testing.T, socket string, tmux *fakeTmux) *Engine {
	t.Helper()
	return newTestEngineWith(t, socket, tmux, &fakeSpawner{})
}

func newTestEngineWith(
	t *testing.T,
	socket string,
	tmux *fakeTmux,
	spawner ThenSpawner,
) *Engine {
	t.Helper()
	if tmux.paneCommand == "" {
		tmux.paneCommand = "claude"
		if strings.HasPrefix(socket, "cx-") {
			tmux.paneCommand = "codex"
		}
	}
	t.Setenv("PFM_HOME", t.TempDir())
	t.Setenv("TMUX", "")
	t.Setenv("CHAT_INJECT_SOCKET", "")
	clearStatedSender(t)
	engine, err := New(Dependencies{
		Resolver: fakeResolver{
			socket: filepath.Join("/tmp", "tmux-jail", socket),
			target: "%1",
		},
		Tmux:    tmux,
		Spawner: spawner,
		Options: Options{
			Poll:            time.Nanosecond,
			EnterGap:        time.Nanosecond,
			EnterSettle:     time.Nanosecond,
			ProofSettle:     time.Nanosecond,
			BusyTries:       2,
			InterruptTries:  2,
			StashTries:      2,
			SettleTries:     2,
			EnterTries:      2,
			LockTimeout:     time.Second,
			LockPoll:        time.Nanosecond,
			LockMaxHold:     time.Second,
			LockRoot:        t.TempDir(),
			ThenLogRoot:     t.TempDir(),
			Sender:          &Sender{Session: "sender", Label: "Operator", UUID: "1234567890"},
			CodexInlineMax:  CodexInlineMax,
			AbsoluteByteMax: AbsoluteMessageMax,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestInjectGuardAndDeliveryMatrix(t *testing.T) {
	tests := []struct {
		name       string
		socket     string
		capture    string
		configure  func(*fakeTmux)
		request    Request
		wantCode   int
		wantStatus string
		wantTyped  bool
		wantKey    string
		wantNoKey  string
	}{
		{
			name:       "claude selector aborts before any key",
			socket:     "cc-1-2-3",
			capture:    "Question\n❯ 1. Allow\n  2. Deny\n",
			request:    Request{Target: "chat", Message: "do work"},
			wantCode:   6,
			wantStatus: "refused",
		},
		{
			name:       "codex selector aborts before any key",
			socket:     "cx-1-2-3",
			capture:    "› 1. Allow\n  2. Deny\nenter to confirm\n",
			request:    Request{Target: "chat", Message: "do work"},
			wantCode:   6,
			wantStatus: "refused",
		},
		{
			name:    "busy claude queues without interrupting",
			socket:  "cc-1-2-3",
			capture: "Working (2s · 9 tokens)\n❯ ",
			configure: func(fake *fakeTmux) {
				fake.submitOnEnter = true
			},
			request:    Request{Target: "chat", Message: "queue this"},
			wantCode:   0,
			wantStatus: "queued",
			wantTyped:  true,
			wantNoKey:  "C-s",
		},
		{
			name:       "busy claude draft refuses without typing",
			socket:     "cc-1-2-3",
			capture:    "Working (2s · 9 tokens)\n❯ existing draft",
			request:    Request{Target: "chat", Message: "do work"},
			wantCode:   6,
			wantStatus: "refused",
		},
		{
			name:    "busy codex queues without interrupting",
			socket:  "cx-1-2-3",
			capture: "Working (2s · 9 tokens) · esc to interrupt\n› ",
			configure: func(fake *fakeTmux) {
				fake.submitOnEnter = true
			},
			request:    Request{Target: "chat", Message: "queue this"},
			wantCode:   0,
			wantStatus: "queued",
			wantTyped:  true,
		},
		{
			name:    "busy codex dim placeholder is not a draft",
			socket:  "cx-1-2-3",
			capture: "Working (2s · 9 tokens) · esc to interrupt\n› \x1b[2mAsk Codex to do anything\x1b[0m",
			configure: func(fake *fakeTmux) {
				fake.submitOnEnter = true
			},
			request:    Request{Target: "chat", Message: "queue after placeholder"},
			wantCode:   0,
			wantStatus: "queued",
			wantTyped:  true,
		},
		{
			name:    "force now interrupts then delivers",
			socket:  "cc-1-2-3",
			capture: "Working (2s · 9 tokens)",
			configure: func(fake *fakeTmux) {
				fake.busyUntilEsc = true
				fake.submitOnEnter = true
			},
			request: Request{
				Target: "chat", Message: "do work", ForceNow: true,
			},
			wantCode: 0, wantStatus: "delivered", wantTyped: true, wantKey: "Escape",
		},
		{
			name:   "idle composer delivers signed text",
			socket: "cc-1-2-3",
			capture: "conversation\n" +
				"❯ ",
			configure: func(fake *fakeTmux) {
				fake.submitOnEnter = true
			},
			request:  Request{Target: "chat", Message: "do work"},
			wantCode: 0, wantStatus: "delivered", wantTyped: true, wantKey: "C-s",
		},
		{
			name:    "numbered codex draft is not a selector",
			socket:  "cx-1-2-3",
			capture: "conversation\n› 1. do this\n",
			configure: func(fake *fakeTmux) {
				fake.submitOnEnter = true
			},
			request:  Request{Target: "chat", Message: "next"},
			wantCode: 0, wantStatus: "delivered", wantTyped: true,
		},
		{
			name:       "unstashable claude draft aborts",
			socket:     "cc-1-2-3",
			capture:    "❯ target draft",
			request:    Request{Target: "chat", Message: "next"},
			wantCode:   6,
			wantStatus: "refused",
			wantKey:    "C-s",
		},
		{
			name:       "dead pane refuses",
			socket:     "cc-1-2-3",
			capture:    "❯ ",
			configure:  func(fake *fakeTmux) { fake.dead = true },
			request:    Request{Target: "chat", Message: "next"},
			wantCode:   3,
			wantStatus: "refused",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeTmux{capture: test.capture}
			if test.configure != nil {
				test.configure(fake)
			}
			engine := newTestEngine(t, test.socket, fake)
			before := fake.capture
			result, err := engine.Inject(context.Background(), test.request)
			if err != nil {
				t.Fatal(err)
			}
			if result.Code != test.wantCode ||
				result.Status != test.wantStatus ||
				result.Typed != test.wantTyped {
				t.Fatalf("Inject() = %+v", result)
			}
			if !test.wantTyped && fake.literal != "" {
				t.Fatalf("refusal typed literal %q", fake.literal)
			}
			if test.wantKey != "" && !contains(fake.keys, test.wantKey) {
				t.Fatalf("keys = %q, want %q", fake.keys, test.wantKey)
			}
			if test.wantNoKey != "" && contains(fake.keys, test.wantNoKey) {
				t.Fatalf("keys = %q, must not contain %q", fake.keys, test.wantNoKey)
			}
			if test.wantCode == 4 && fake.capture != before {
				t.Fatalf("selector refusal changed capture: %q -> %q", before, fake.capture)
			}
			if test.wantTyped &&
				!strings.Contains(result.Proof, "to reply: /chat:inject sender <message>") {
				t.Fatalf("proof lacks mandatory sender signature: %q", result.Proof)
			}
		})
	}
}

func TestInjectCodexAndAbsoluteCapsPrecedeTyping(t *testing.T) {
	fake := &fakeTmux{capture: "› "}
	engine := newTestEngine(t, "cx-1-2-3", fake)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: strings.Repeat("x", CodexInlineMax+1),
	})
	if err != nil || result.Code != 6 || result.Typed || len(fake.keys) != 0 {
		t.Fatalf("Codex cap result=%+v keys=%q err=%v", result, fake.keys, err)
	}

	result, err = engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: strings.Repeat("x", 1<<20),
	})
	if err != nil || result.Code != 6 || result.Typed || len(fake.keys) != 0 {
		t.Fatalf("absolute cap result=%+v keys=%q err=%v", result, fake.keys, err)
	}
}

func TestInjectFileBackedCodexUsesPasteTransportPastInlineCap(t *testing.T) {
	fake := &fakeTmux{capture: "› ", submitOnEnter: true}
	engine := newTestEngine(t, "cx-1-2-3", fake)
	message := strings.Repeat("file body ", CodexInlineMax)
	result, err := engine.Inject(context.Background(), Request{
		Target:     "chat",
		Message:    message,
		FileBacked: true,
	})
	if err != nil || result.Code != 0 || result.Status != "delivered" {
		t.Fatalf("file-backed result=%+v err=%v", result, err)
	}
	if !fake.pasted {
		t.Fatal("file-backed delivery used inline send-keys instead of paste transport")
	}
}

func TestInjectAdversarialGuardStatesNeverMistypeOnRefusal(t *testing.T) {
	states := []struct {
		socket      string
		capture     string
		paneCommand string
		dead        bool
		code        int
	}{
		{"cc-1-2-3", "❯ 1. Allow\n2. Deny", "claude", false, 6},
		{"cx-1-2-3", "› 1. Allow\n2. Deny", "codex", false, 6},
		{"cc-1-2-3", "Working (1s · 3 tokens)", "python3", false, 7},
		{"cc-1-2-3", "❯ draft", "claude", false, 6},
		{"cc-1-2-3", "❯ ", "claude", true, 3},
	}
	for iteration := 0; iteration < 100; iteration++ {
		state := states[iteration%len(states)]
		fake := &fakeTmux{
			capture:     state.capture,
			paneCommand: state.paneCommand,
			dead:        state.dead,
		}
		engine := newTestEngine(t, state.socket, fake)
		before := fake.capture
		result, err := engine.Inject(context.Background(), Request{
			Target:  "chat",
			Message: "must never mistype",
		})
		if err != nil {
			t.Fatalf("iteration %d: %v", iteration, err)
		}
		if result.Code != state.code || result.Typed || fake.literal != "" {
			t.Fatalf("iteration %d: result=%+v literal=%q", iteration, result, fake.literal)
		}
		if result.Code == 4 && fake.capture != before {
			t.Fatalf("iteration %d selector capture changed", iteration)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
