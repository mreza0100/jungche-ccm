package inject

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"hostops/cc-fleet/internal/resolve"
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
	windowName    string
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
	t.Setenv("CC_FLEET_HOME", t.TempDir())
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
			Sender:          &Sender{Session: "sender", Label: "Founder", UUID: "1234567890"},
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
	}{
		{
			name:       "claude selector aborts before any key",
			socket:     "cc-1-2-3",
			capture:    "Question\n❯ 1. Allow\n  2. Deny\n",
			request:    Request{Target: "chat", Message: "do work"},
			wantCode:   4,
			wantStatus: "refused",
		},
		{
			name:       "codex selector aborts before any key",
			socket:     "cx-1-2-3",
			capture:    "› 1. Allow\n  2. Deny\nenter to confirm\n",
			request:    Request{Target: "chat", Message: "do work"},
			wantCode:   4,
			wantStatus: "refused",
		},
		{
			name:     "busy refuses without typing",
			socket:   "cc-1-2-3",
			capture:  "Working (2s · 9 tokens)",
			request:  Request{Target: "chat", Message: "do work"},
			wantCode: 7, wantStatus: "refused",
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
			wantCode:   5,
			wantStatus: "refused",
			wantKey:    "C-s",
		},
		{
			name:       "dead pane refuses",
			socket:     "cc-1-2-3",
			capture:    "❯ ",
			configure:  func(fake *fakeTmux) { fake.dead = true },
			request:    Request{Target: "chat", Message: "next"},
			wantCode:   1,
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

func TestInjectAdversarialGuardStatesNeverMistypeOnRefusal(t *testing.T) {
	states := []struct {
		socket  string
		capture string
		dead    bool
		code    int
	}{
		{"cc-1-2-3", "❯ 1. Allow\n2. Deny", false, 4},
		{"cx-1-2-3", "› 1. Allow\n2. Deny", false, 4},
		{"cc-1-2-3", "Working (1s · 3 tokens)", false, 7},
		{"cc-1-2-3", "❯ draft", false, 5},
		{"cc-1-2-3", "❯ ", true, 1},
	}
	for iteration := 0; iteration < 100; iteration++ {
		state := states[iteration%len(states)]
		fake := &fakeTmux{capture: state.capture, dead: state.dead}
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
