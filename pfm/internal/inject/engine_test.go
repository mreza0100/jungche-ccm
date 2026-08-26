package inject

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
	literalBuffer string
	literals      []string
	pasted        bool
	windowName    string
	paneCommand   string
	keys          []string
	dead          bool
	busyUntilEsc  bool
	stashClears   bool
	submitOnEnter bool
	submitCapture string
	proofCapture  string
	submitted     bool
	postCaptures  int
	inMode        bool
	modeAfterEsc  bool
	cancelModes   int
	killLiteral   bool
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

func TestScheduleAfterCurrentTurnDoesNotQueueACompactAsModelInput(t *testing.T) {
	fake := &fakeTmux{capture: "Working (10s)\n› Ask Codex to do anything"}
	spawner := &fakeSpawner{}
	engine := newTestEngineWith(t, "cx-scheduled-compact", fake, spawner)
	result, err := engine.ScheduleAfterCurrentTurn(context.Background(), Request{
		Target:  "chat",
		Message: "/compact preserve the live findings",
		Then:    []string{"resume after real compaction"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Status != "scheduled" || result.Typed || result.Steers != 1 {
		t.Fatalf("scheduled compact result = %+v", result)
	}
	if len(fake.literals) != 0 || len(fake.keys) != 0 {
		t.Fatalf("scheduler typed into the busy model turn: literals=%q keys=%q", fake.literals, fake.keys)
	}
	calls := spawner.spawned()
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].Steers, []string{
		"/compact preserve the live findings",
		"resume after real compaction",
	}) {
		t.Fatalf("detached compact chain = %+v", calls)
	}
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
	if fake.submitted && fake.proofCapture != "" {
		fake.postCaptures++
		if fake.postCaptures > 1 {
			return fake.proofCapture, nil
		}
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
	fake.literalBuffer += text
	fake.literals = append(fake.literals, text)
	if fake.killLiteral {
		return nil
	}
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
		if fake.modeAfterEsc {
			fake.inMode = true
			fake.modeAfterEsc = false
		}
		if fake.busyUntilEsc {
			fake.capture = "turn interrupted\n❯ "
			fake.busyUntilEsc = false
		}
	case "C-s":
		if fake.stashClears {
			fake.capture = "draft stashed\n❯ "
		}
	case "Enter":
		if fake.submitOnEnter && fake.literalBuffer != "" {
			if fake.submitCapture != "" {
				fake.capture = strings.ReplaceAll(
					fake.submitCapture,
					"{{MESSAGE}}",
					fake.literalBuffer,
				)
			} else {
				fake.capture = "USER: " + fake.literalBuffer + "\n❯ "
			}
			fake.literal = ""
			fake.literalBuffer = ""
			fake.submitted = true
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
	fake.cancelModes++
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
			Poll:              time.Nanosecond,
			EnterGap:          time.Nanosecond,
			EnterSettle:       time.Nanosecond,
			ProofSettle:       time.Nanosecond,
			BusyTries:         2,
			InterruptTries:    2,
			StashTries:        2,
			SettleTries:       2,
			EnterTries:        2,
			LockTimeout:       time.Second,
			LockPoll:          time.Nanosecond,
			LockMaxHold:       time.Second,
			LockRoot:          t.TempDir(),
			ThenLogRoot:       t.TempDir(),
			Sender:            &Sender{Session: "sender", Label: "Operator", UUID: "1234567890"},
			ClaudeAutoFileMax: ClaudeAutoFileMax,
			CodexAutoFileMax:  CodexAutoFileMax,
			CommandChunkRunes: CommandChunkRunes,
			CommandChunkGap:   time.Nanosecond,
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
			name:   "busy codex vitest output is not a composer draft",
			socket: "cx-1-2-3",
			capture: "Working (2s · 9 tokens) · esc to interrupt\n" +
				"└ ❯ src/tests/integration/gate-transaction-crash-safety.db.test.ts (3 tests | 1\n" +
				"› ",
			configure: func(fake *fakeTmux) {
				fake.submitOnEnter = true
			},
			request:    Request{Target: "chat", Message: "queue after vitest output"},
			wantCode:   0,
			wantStatus: "queued",
			wantTyped:  true,
			wantNoKey:  "C-s",
		},
		{
			name:   "busy claude agent overlay row is not a composer draft",
			socket: "cc-1-2-3",
			capture: "Working (2s · 9 tokens)\n" +
				"❯ ● qa-cortex  Verifying 6-bugs.md is untracked",
			configure: func(fake *fakeTmux) {
				fake.submitOnEnter = true
			},
			request:    Request{Target: "chat", Message: "queue after agent overlay"},
			wantCode:   0,
			wantStatus: "queued",
			wantTyped:  true,
			wantNoKey:  "C-s",
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
				fake.stashClears = true
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
				!strings.Contains(result.Proof, "to reply: chat_inject sender <message>") {
				t.Fatalf("proof lacks mandatory sender signature: %q", result.Proof)
			}
		})
	}
}

func TestInjectBodyAboveFormerAbsoluteCapUsesAutoFile(t *testing.T) {
	fake := &fakeTmux{capture: "› ", submitOnEnter: true}
	engine := newTestEngine(t, "cx-former-absolute-cap", fake)
	body := "former absolute cap payload\n" + strings.Repeat("x", 1<<20)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: body,
	})
	if err != nil || result.Code != 0 || result.AutoFilePath == "" ||
		!strings.Contains(result.Message, "AUTO-FILE") || len(fake.literals) != 1 {
		t.Fatalf("former absolute cap result=%+v literals=%d err=%v", result, len(fake.literals), err)
	}
	stored, readErr := os.ReadFile(result.AutoFilePath)
	if readErr != nil || string(stored) != body {
		t.Fatalf("former absolute cap body=%d bytes err=%v", len(stored), readErr)
	}
}

func TestInjectAutoFileBoundaryAndKillerBody(t *testing.T) {
	tests := []struct {
		name      string
		socket    string
		threshold int
	}{
		{name: "claude", socket: "cc-auto-file", threshold: 720},
		{name: "codex", socket: "cx-auto-file", threshold: 900},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			underFake := &fakeTmux{capture: "conversation\n❯ ", submitOnEnter: true}
			if strings.HasPrefix(test.socket, "cx-") {
				underFake.capture = "conversation\n› "
			}
			under := newTestEngine(t, test.socket, underFake)
			under.options.DisableSignature = true
			underBody := strings.Repeat("u", test.threshold-1)
			underResult, err := under.Inject(context.Background(), Request{
				Target: "chat", Message: underBody,
			})
			if err != nil || underResult.Code != 0 ||
				len(underFake.literals) == 0 ||
				underFake.literals[0] != underBody ||
				strings.Contains(underResult.Message, "AUTO-FILE") {
				t.Fatalf("one-under delivery result=%+v literals=%q err=%v", underResult, underFake.literals, err)
			}

			overFake := &fakeTmux{capture: underFake.capture, submitOnEnter: true}
			over := newTestEngine(t, test.socket, overFake)
			over.options.DisableSignature = true
			overBody := strings.Repeat("o", test.threshold+1)
			overResult, err := over.Inject(context.Background(), Request{
				Target: "chat", Message: overBody,
			})
			if err != nil || overResult.Code != 0 ||
				!strings.Contains(overResult.Message, "AUTO-FILE") ||
				len(overFake.literals) == 0 ||
				strings.Contains(overFake.literals[0], overBody) ||
				!strings.Contains(overFake.literals[0], "read ") ||
				!strings.Contains(overFake.literals[0], " fully") {
				t.Fatalf("one-over delivery result=%+v literals=%q err=%v", overResult, overFake.literals, err)
			}
			files, globErr := filepath.Glob(filepath.Join(
				os.Getenv("PFM_HOME"), ".local", "state", "pfm", "inject-bodies", "*.md",
			))
			if globErr != nil || len(files) != 1 {
				t.Fatalf("canonical body files=%q err=%v", files, globErr)
			}
			stored, readErr := os.ReadFile(files[0])
			if readErr != nil || string(stored) != overBody ||
				!strings.Contains(overResult.Message, files[0]) {
				t.Fatalf("stored body=%d bytes err=%v receipt=%q", len(stored), readErr, overResult.Message)
			}

			killerFake := &fakeTmux{capture: overFake.capture, submitOnEnter: true}
			killer := newTestEngine(t, test.socket+"-killer", killerFake)
			killer.options.DisableSignature = true
			killerBody := "killer caption\n" + strings.Repeat("k", 8<<10)
			killerResult, err := killer.Inject(context.Background(), Request{
				Target: "chat", Message: killerBody,
			})
			if err != nil || killerResult.Code != 0 ||
				len(killerFake.literals) == 0 ||
				len([]rune(killerFake.literals[0])) >= test.threshold ||
				!strings.Contains(killerResult.Message, "AUTO-FILE") {
				t.Fatalf("killer body was not reduced to a safe pointer: result=%+v literal=%q err=%v", killerResult, killerFake.literals, err)
			}
		})
	}
}

// TestLongProseAutoFilePreservesBodySignatureAndProof covers the other side
// of the unlimited-inject contract: plain prose remains durable file-backed
// transport, while only its short signed pointer crosses the TUI boundary.
func TestLongProseAutoFilePreservesBodySignatureAndProof(t *testing.T) {
	body := "long prose payload\n" + strings.Repeat("byte-exact body ", 400)
	fake := &fakeTmux{
		capture:       "conversation\n❯ ",
		submitOnEnter: true,
		submitCapture: "USER: {{MESSAGE}}\n❯ ",
	}
	engine := newTestEngine(t, "cc-long-prose", fake)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Status != "delivered" ||
		result.AutoFilePath == "" || result.Unsigned {
		t.Fatalf("long prose result=%+v err=%v", result, err)
	}
	stored, err := os.ReadFile(result.AutoFilePath)
	if err != nil || string(stored) != body {
		t.Fatalf("canonical body changed: bytes=%d err=%v", len(stored), err)
	}
	if len(fake.literals) != 1 {
		t.Fatalf("auto-file pointer used unexpected transport chunks: %d", len(fake.literals))
	}
	pointer := fake.literals[0]
	if strings.Contains(pointer, body) ||
		!strings.Contains(pointer, "long prose payload — read "+result.AutoFilePath+" fully") ||
		!strings.Contains(pointer, "to reply: chat_inject sender <message>") {
		t.Fatalf("pointer changed body/signature semantics: %q", pointer)
	}
	if !strings.Contains(result.Message, "AUTO-FILE") ||
		!strings.Contains(result.Message, result.AutoFilePath) ||
		!strings.Contains(result.Proof, "long prose payload — read "+result.AutoFilePath+" fully") {
		t.Fatalf("auto-file receipt/proof lacks pointer evidence: result=%+v", result)
	}
}

func TestPersistBodyPrunesExpiredMarkdownByAge(t *testing.T) {
	engine := newTestEngine(t, "cc-body-prune", &fakeTmux{capture: "❯ "})
	now := time.Date(2031, 2, 3, 4, 5, 6, 7, time.UTC)
	engine.options.Now = func() time.Time { return now }
	engine.options.BodyMaxAge = 24 * time.Hour
	if err := os.MkdirAll(engine.options.BodyRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(engine.options.BodyRoot, "stale.md")
	fresh := filepath.Join(engine.options.BodyRoot, "fresh.md")
	nonBody := filepath.Join(engine.options.BodyRoot, "stale.txt")
	for _, path := range []string{stale, fresh, nonBody} {
		if err := os.WriteFile(path, []byte(path), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := now.Add(-48 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(nonBody, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(fresh, now, now); err != nil {
		t.Fatal(err)
	}
	path, warnings, err := engine.persistBody("exact body", "cc-1:%2")
	if err != nil || len(warnings) != 0 {
		t.Fatalf("persist body path=%q warnings=%q err=%v", path, warnings, err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expired markdown survived: %v", err)
	}
	for _, kept := range []string{fresh, nonBody} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("non-expired/non-body file %q was pruned: %v", kept, err)
		}
	}
	stored, err := os.ReadFile(path)
	if err != nil || string(stored) != "exact body" ||
		!strings.Contains(filepath.Base(path), "cc-1--2") {
		t.Fatalf("canonical file path=%q body=%q err=%v", path, stored, err)
	}
}

func TestPrepareForResumeUsesSignedAutoFilePointer(t *testing.T) {
	engine := newTestEngine(t, "cc-resume-auto-file", &fakeTmux{capture: "❯ "})
	body := "resume caption\n" + strings.Repeat("r", 8<<10)
	prepared, err := engine.PrepareForResume(
		context.Background(),
		"11111111-2222-4333-8444-555555555555",
		"cc",
		body,
	)
	if err != nil || prepared.AutoFilePath == "" ||
		!strings.Contains(prepared.Message, "resume caption — read ") ||
		!strings.Contains(prepared.Message, " fully\n\n— sid 12345678") {
		t.Fatalf("resume preparation=%+v err=%v", prepared, err)
	}
	stored, err := os.ReadFile(prepared.AutoFilePath)
	if err != nil || string(stored) != body {
		t.Fatalf("resume canonical body=%d bytes err=%v", len(stored), err)
	}
}

func TestInjectShortExplicitFileUsesPasteTransport(t *testing.T) {
	fake := &fakeTmux{capture: "› ", submitOnEnter: true}
	engine := newTestEngine(t, "cx-1-2-3", fake)
	message := strings.Repeat("file body ", 20)
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

// The render-settle probe is advisory. Terminal echo can disappear while the
// TUI still accepted the literal bytes; leaving those bytes in the composer
// without an Enter is the worst possible failure because every retry stacks a
// second body on top of the first.
func TestRenderSettleMissStillFallsThroughToEnter(t *testing.T) {
	fake := &fakeTmux{
		capture:       "conversation\n❯ ",
		killLiteral:   true,
		submitOnEnter: true,
	}
	engine := newTestEngine(t, "cc-1-2-3", fake)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "terminal echo is deliberately killed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || !contains(fake.keys, "Enter") {
		t.Fatalf("render miss stranded typed text: result=%+v keys=%q", result, fake.keys)
	}
}

// tmux reports the foreground descendant, not the pane launcher. Codex and
// Claude legitimately show node/python/sleep while a tool runs, but their
// busy composers remain safe queue surfaces.
func TestBusyQueueIgnoresForegroundDescendant(t *testing.T) {
	fake := &fakeTmux{
		capture:       "Working (600s · 99 tokens)\n❯ ",
		paneCommand:   "python3",
		submitOnEnter: true,
	}
	engine := newTestEngine(t, "cc-1-2-3", fake)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "queue while the descendant owns the foreground",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Status != "queued" || !result.Typed {
		t.Fatalf("busy descendant was refused: %+v", result)
	}
	if contains(fake.keys, "Escape") {
		t.Fatalf("ordinary queue interrupted the pane: keys=%q", fake.keys)
	}
}

func TestDeliveryProofClassificationMatrix(t *testing.T) {
	tests := []struct {
		name          string
		socket        string
		paneCommand   string
		initial       string
		afterSubmit   string
		fileBacked    bool
		wantStatus    string
		wantProofText string
	}{
		{
			name:          "claude idle message is past the composer",
			socket:        "cc-proof-idle",
			paneCommand:   "claude",
			initial:       "conversation\n❯ ",
			afterSubmit:   "conversation\nUSER: {{MESSAGE}}\n❯ ",
			wantStatus:    "delivered",
			wantProofText: "USER: proof body",
		},
		{
			name:        "claude busy queue indicator",
			socket:      "cc-proof-queue",
			paneCommand: "claude",
			initial:     "Working (2s · 9 tokens)\n❯ ",
			afterSubmit: "Working (2s · 9 tokens)\nQUEUED: {{MESSAGE}}\n" +
				"Press up to edit queued messages\n❯ ",
			wantStatus:    "queued",
			wantProofText: "Press up to edit queued messages",
		},
		{
			name:          "codex idle message is past the composer",
			socket:        "cx-proof-idle",
			paneCommand:   "codex",
			initial:       "conversation\n› ",
			afterSubmit:   "conversation\nUSER: {{MESSAGE}}\n› ",
			wantStatus:    "delivered",
			wantProofText: "USER: proof body",
		},
		{
			name:        "codex busy queue follows process not socket spelling",
			socket:      "cc-proof-codex-queue",
			paneCommand: "codex",
			initial:     "Working (2s · 9 tokens) · esc to interrupt\n› ",
			afterSubmit: "Working (2s · 9 tokens)\nPending message\n" +
				"{{MESSAGE}}\n› ",
			wantStatus:    "queued",
			wantProofText: "Pending message",
		},
		{
			name:          "file-backed body is past the composer",
			socket:        "cx-proof-file",
			paneCommand:   "codex",
			initial:       "conversation\n› ",
			afterSubmit:   "conversation\nUSER: {{MESSAGE}}\n› ",
			fileBacked:    true,
			wantStatus:    "delivered",
			wantProofText: "USER: proof body",
		},
		{
			name:          "cleared composer without pane evidence is honest",
			socket:        "cc-proof-unproven",
			paneCommand:   "claude",
			initial:       "conversation\n❯ ",
			afterSubmit:   "conversation unchanged\n❯ ",
			wantStatus:    "delivered_unproven",
			wantProofText: "conversation unchanged",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeTmux{
				capture:       test.initial,
				paneCommand:   test.paneCommand,
				submitOnEnter: true,
				submitCapture: test.afterSubmit,
			}
			engine := newTestEngine(t, test.socket, fake)
			result, err := engine.Inject(context.Background(), Request{
				Target:     "chat",
				Message:    "proof body",
				FileBacked: test.fileBacked,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Code != 0 || result.Status != test.wantStatus {
				t.Fatalf("Inject() = %+v", result)
			}
			if !strings.Contains(result.Proof, test.wantProofText) {
				t.Fatalf("proof %q lacks %q", result.Proof, test.wantProofText)
			}
			if test.wantStatus == "delivered_unproven" &&
				!strings.Contains(result.Message, "delivered-unproven") {
				t.Fatalf("unproven receipt = %q", result.Message)
			}
		})
	}
}

func TestBlindDeliveryProofStatesMissingComposerLine(t *testing.T) {
	fake := &fakeTmux{
		capture:       "conversation\n❯ ",
		submitOnEnter: true,
		submitCapture: "response started\n❯ ",
		proofCapture:  "terminal redraw without a visible prompt",
	}
	engine := newTestEngine(t, "cc-proof-blind", fake)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "blind proof body",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Status != "delivered_unproven" ||
		!strings.Contains(result.Message, "no composer line") {
		t.Fatalf("blind proof receipt = %+v", result)
	}
}

func TestSuccessReceiptNamesForceAndStashedDraft(t *testing.T) {
	tests := []struct {
		name    string
		fake    *fakeTmux
		request Request
		want    string
	}{
		{
			name: "force now",
			fake: &fakeTmux{
				capture:       "Working (3s · 4 tokens)",
				busyUntilEsc:  true,
				submitOnEnter: true,
			},
			request: Request{Target: "chat", Message: "urgent", ForceNow: true},
			want:    "FORCE-NOW: interrupted",
		},
		{
			name: "stashed draft",
			fake: &fakeTmux{
				capture:       "conversation\n❯ existing draft",
				stashClears:   true,
				submitOnEnter: true,
			},
			request: Request{Target: "chat", Message: "next"},
			want:    "stashed and restored",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := newTestEngine(t, "cc-receipt", test.fake)
			result, err := engine.Inject(context.Background(), test.request)
			if err != nil {
				t.Fatal(err)
			}
			if result.Code != 0 || !strings.Contains(result.Message, test.want) {
				t.Fatalf("receipt = %+v, want %q", result, test.want)
			}
		})
	}
}

func TestInjectCarriesSuccessfulResolverNote(t *testing.T) {
	fake := &fakeTmux{capture: "conversation\n❯ ", submitOnEnter: true}
	engine := newTestEngine(t, "cc-resolve-note", fake)
	engine.resolver = fakeResolver{
		socket: "/tmp/tmux-jail/cc-resolve-note",
		target: "%1",
		detail: "resolved 🔖 label 'QA' → pane '%1'",
	}
	result, err := engine.Inject(context.Background(), Request{
		Target:  "QA",
		Message: "next",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.ResolutionNote != "resolved 🔖 label 'QA' → pane '%1'" {
		t.Fatalf("resolution note lost: %+v", result)
	}
}

func TestResolvePreservesAmbiguousVerdict(t *testing.T) {
	engine := newTestEngine(t, "cx-ambiguous", &fakeTmux{})
	engine.resolver = fakeResolver{code: 2, detail: "two live panes match"}
	_, code, detail, err := engine.Resolve(context.Background(), "duplicate")
	if err != nil || code != CodeAmbiguous || detail != "two live panes match" {
		t.Fatalf("Resolve() code=%d detail=%q err=%v", code, detail, err)
	}
}

func TestSecondEnterDefenseCoversCodexAndBusyQueues(t *testing.T) {
	tests := []struct {
		name    string
		socket  string
		capture string
	}{
		{name: "idle codex", socket: "cx-enter", capture: "conversation\n› "},
		{name: "busy claude queue", socket: "cc-enter", capture: "Working (4s · 2 tokens)\n❯ "},
		{name: "busy codex queue", socket: "cx-enter", capture: "Working (4s · 2 tokens)\n› "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeTmux{capture: test.capture, submitOnEnter: true}
			engine := newTestEngine(t, test.socket, fake)
			result, err := engine.Inject(context.Background(), Request{
				Target:  "chat",
				Message: "submit defensively",
			})
			if err != nil || result.Code != 0 {
				t.Fatalf("delivery result=%+v err=%v", result, err)
			}
			enters := 0
			for _, key := range fake.keys {
				if key == "Enter" {
					enters++
				}
			}
			if enters < 2 {
				t.Fatalf("keys=%q, want swallowed-Enter defense", fake.keys)
			}
		})
	}
}

func TestBusyPaneStashesExistingDraftBeforeQueueing(t *testing.T) {
	fake := &fakeTmux{
		capture:       "Working (7s · 8 tokens)\n❯ existing draft",
		stashClears:   true,
		submitOnEnter: true,
	}
	engine := newTestEngine(t, "cc-busy-draft", fake)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "queue after preserving the draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Status != "queued" || !result.DraftStashed ||
		!contains(fake.keys, "C-s") {
		t.Fatalf("busy draft was not preserved before queueing: result=%+v keys=%q", result, fake.keys)
	}
}

func TestCopyModeIsCancelledAgainAfterOverlayDismissal(t *testing.T) {
	fake := &fakeTmux{
		capture:       "Restore the code\n❯ ",
		inMode:        true,
		modeAfterEsc:  true,
		submitOnEnter: true,
	}
	engine := newTestEngine(t, "cc-copy-mode", fake)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "deliver after overlays",
	})
	if err != nil || result.Code != 0 {
		t.Fatalf("delivery result=%+v err=%v", result, err)
	}
	if fake.cancelModes != 2 || fake.inMode {
		t.Fatalf("copy mode cancellation count=%d inMode=%t", fake.cancelModes, fake.inMode)
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
