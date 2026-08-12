package spawn

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeCodex is a Codex TUI as far as this package can tell: it draws a
// composer, offers /rename only when that exact text is typed, opens its
// rename prompt only on Enter, and refuses an empty name — the four states the
// choreography navigates. The flags turn each capability off so the abort
// paths are driven by a TUI that behaves differently, not by a stubbed
// timeout.
type fakeCodex struct {
	mutex sync.Mutex

	offersRename bool
	opensPrompt  bool
	refusesEmpty bool

	sessions []SessionSpec
	keys     []string
	composer string
	stage    string
	name     string
}

func newFakeCodex() *fakeCodex {
	return &fakeCodex{offersRename: true, opensPrompt: true, stage: "composer"}
}

func (fake *fakeCodex) NewSession(_ context.Context, spec SessionSpec) error {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.sessions = append(fake.sessions, spec)
	return nil
}

func (fake *fakeCodex) Capture(_ context.Context, _, _ string) (string, error) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	switch fake.stage {
	case "offered":
		return "codex\n> " + fake.composer +
			"\n  rename  rename the current thread\n", nil
	case "prompt":
		return "codex\nRename thread\nType a name and press Enter\n> " +
			fake.composer + "\n", nil
	case "empty":
		return "codex\nRename thread\nType a name and press Enter\n" +
			"Thread name cannot be empty.\n", nil
	default:
		header := "codex"
		if fake.name != "" {
			header = "codex · " + fake.name
		}
		return header + "\n> " + fake.composer + "\n", nil
	}
}

func (fake *fakeCodex) SendLiteral(_ context.Context, _, _, text string) error {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.keys = append(fake.keys, "literal:"+text)
	fake.composer += text
	if fake.stage == "composer" &&
		fake.offersRename &&
		strings.HasPrefix(fake.composer, codexRenameCommand) {
		fake.stage = "offered"
	}
	return nil
}

func (fake *fakeCodex) SendKey(_ context.Context, _, _, key string) error {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.keys = append(fake.keys, "key:"+key)
	switch key {
	case "Enter":
		switch fake.stage {
		case "offered":
			fake.composer = ""
			if fake.opensPrompt {
				fake.stage = "prompt"
			} else {
				fake.stage = "composer"
			}
		case "prompt":
			if fake.composer == "" && fake.refusesEmpty {
				fake.stage = "empty"
				return nil
			}
			fake.name = fake.composer
			fake.composer = ""
			fake.stage = "composer"
		default:
			fake.composer = ""
		}
	case "BSpace":
		runes := []rune(fake.composer)
		if len(runes) > 0 {
			fake.composer = string(runes[:len(runes)-1])
		}
	case "Escape":
		fake.composer = ""
		fake.stage = "composer"
	}
	return nil
}

func testTimings() Timings {
	return Timings{
		Poll:  time.Millisecond,
		Boot:  200 * time.Millisecond,
		Step:  200 * time.Millisecond,
		Typed: 0,
	}
}

func codexRequest() Request {
	return Request{
		Engine:  "cx",
		Name:    "_HIDE codex worker",
		Socket:  "cx-1-2-3",
		CWD:     "/work/alpha",
		Run:     "codex",
		Prompt:  "read the incident report",
		Timings: testTimings(),
	}
}

// TestCodexThreadIsRenamedThenPrompted is the whole point of the Codex path:
// the name lands through the engine's own rename UI BEFORE the first prompt
// starts a turn.
func TestCodexThreadIsRenamedThenPrompted(t *testing.T) {
	fake := newFakeCodex()
	result, err := Run(context.Background(), fake, codexRequest())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Named || !result.Prompted || len(result.Warnings) != 0 {
		t.Fatalf("result = %#v", result)
	}
	if fake.name != "_HIDE codex worker" {
		t.Fatalf("thread name = %q", fake.name)
	}
	want := []string{
		"literal:/rename",
		"key:Enter",
		"literal:_HIDE codex worker",
		"key:Enter",
		"literal:read the incident report",
		"key:Enter",
	}
	if strings.Join(fake.keys, "|") != strings.Join(want, "|") {
		t.Fatalf("keystrokes\n got: %v\nwant: %v", fake.keys, want)
	}
}

// TestCodexWithoutARenameCommandStaysUnnamed is the version-drift guard: a
// Codex build that does not offer /rename must leave the composer EMPTY (the
// typed command cleared, never submitted) and report the chat unnamed.
func TestCodexWithoutARenameCommandStaysUnnamed(t *testing.T) {
	fake := newFakeCodex()
	fake.offersRename = false
	request := codexRequest()
	request.Prompt = ""
	result, err := Run(context.Background(), fake, request)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Named {
		t.Fatal("unnamed chat reported as named")
	}
	if len(result.Warnings) != 1 ||
		!strings.Contains(result.Warnings[0], "did not offer /rename") {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
	if fake.composer != "" {
		t.Fatalf("composer still holds %q — it would ride along with the next prompt",
			fake.composer)
	}
	for _, key := range fake.keys {
		if key == "key:Enter" {
			t.Fatalf("an unoffered /rename was submitted to the model: %v", fake.keys)
		}
	}
}

// TestCodexRenamePromptNeverOpensStaysUnnamed covers the middle step failing:
// the command exists but its prompt never appears, so the NAME must not be
// typed into whatever is on screen.
func TestCodexRenamePromptNeverOpensStaysUnnamed(t *testing.T) {
	fake := newFakeCodex()
	fake.opensPrompt = false
	request := codexRequest()
	request.Prompt = ""
	result, err := Run(context.Background(), fake, request)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Named {
		t.Fatal("unnamed chat reported as named")
	}
	if len(result.Warnings) != 1 ||
		!strings.Contains(result.Warnings[0], "never asked for a thread name") {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
	for _, key := range fake.keys {
		if key == "literal:_HIDE codex worker" {
			t.Fatalf("the name was typed with no prompt open: %v", fake.keys)
		}
	}
}

// TestCodexRefusingAnEmptyNameIsReportedAsSuch covers the last rename failure
// Codex itself can raise, so the warning names the real cause instead of
// blaming the prompt for never closing.
func TestCodexRefusingAnEmptyNameIsReportedAsSuch(t *testing.T) {
	fake := newFakeCodex()
	fake.refusesEmpty = true
	request := codexRequest()
	request.Name = ""
	request.Prompt = ""
	result, err := Run(context.Background(), fake, request)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Named {
		t.Fatal("an empty name was reported as landed")
	}
	if len(result.Warnings) != 1 ||
		!strings.Contains(result.Warnings[0], "refused the name as empty") {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
}

// TestClaudeNeedsNoChoreography: Claude takes its name on the command line, so
// the spawn must type NOTHING into it.
func TestClaudeNeedsNoChoreography(t *testing.T) {
	fake := newFakeCodex()
	result, err := Run(context.Background(), fake, Request{
		Engine:  "cc",
		Name:    "worker",
		Socket:  "cc-1-2-3",
		CWD:     "/work/alpha",
		Run:     "claude --name worker",
		Prompt:  "audit the firewall",
		Timings: testTimings(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Named || !result.Prompted {
		t.Fatalf("result = %#v", result)
	}
	if len(fake.keys) != 0 {
		t.Fatalf("keys typed into a Claude chat: %v", fake.keys)
	}
	if len(fake.sessions) != 1 {
		t.Fatalf("sessions = %#v", fake.sessions)
	}
	session := fake.sessions[0]
	if session.Socket != "cc-1-2-3" || session.Session != "cc-1-2-3" ||
		session.Window != "worker" || session.CWD != "/work/alpha" {
		t.Fatalf("session spec = %#v", session)
	}
	if session.Width != 220 || session.Height != 50 {
		t.Fatalf("headless geometry = %dx%d, want 220x50", session.Width, session.Height)
	}
}

// deadPane is a server whose session never comes up — the chat died at birth.
type deadPane struct{ fakeCodex }

func (fake *deadPane) Capture(context.Context, string, string) (string, error) {
	return "", errNoSession{}
}

type errNoSession struct{}

func (errNoSession) Error() string { return "no server running on socket" }

func TestChatThatDiesAtBirthIsReportedAsSuch(t *testing.T) {
	fake := &deadPane{}
	_, err := Run(context.Background(), fake, codexRequest())
	if err == nil {
		t.Fatal("Run() reported a dead chat as healthy")
	}
	if !strings.Contains(err.Error(), "died at birth") {
		t.Fatalf("error = %v", err)
	}
}

func TestWindowName(t *testing.T) {
	for _, testCase := range []struct{ in, want string }{
		{"_HIDE worker 3", "_HIDE worker 3"},
		{"a:b.c", "a-b-c"},
		{"  spaced   out\t", "spaced out"},
		{"", "chat"},
		{strings.Repeat("x", 60), strings.Repeat("x", 40)},
	} {
		if got := WindowName(testCase.in); got != testCase.want {
			t.Fatalf("WindowName(%q) = %q, want %q", testCase.in, got, testCase.want)
		}
	}
}
