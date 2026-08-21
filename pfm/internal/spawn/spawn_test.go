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
	// startupModals is how many full-screen overlays stand between boot and
	// the composer, each cleared by one Escape. A real Codex has at least one
	// (hooks review, trust, plugin notices) and they swallow every keystroke
	// sent to them — the state no stub modelled the first time around.
	startupModals   int
	stuckModal      bool
	modalAfterReads int
	reads           int
	// dropsEnters is how many of the prompt's Enters the composer swallows
	// before one takes, and deafComposer swallows every one of them. This is
	// the live failure that made a chat sit there holding its orders: the
	// keystroke was sent, the engine never took it, and nothing checked.
	dropsEnters  int
	deafComposer bool

	sessions []SessionSpec
	keys     []string
	composer string
	stage    string
	name     string
}

// fakeStatusLine is the idle-composer status row, whose token meter is the
// half of readiness a modal cannot fake.
const fakeStatusLine = "  019f · ~/work/alpha · Full Access · Context 0% used · 0 in · 0 out\n"

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
	// modalAfterReads reproduces the exact live failure: Codex paints its
	// composer first and raises its startup modal a beat LATER, so the
	// composer is visible for a flash before the screen is stolen.
	if fake.modalAfterReads > 0 {
		fake.reads++
		if fake.reads > fake.modalAfterReads {
			fake.modalAfterReads = 0
			fake.startupModals++
		}
	}
	if fake.startupModals > 0 || fake.stuckModal {
		// No composer glyph anywhere: this screen eats keystrokes.
		// The selection cursor is the SAME glyph the composer uses — the live
		// false positive this fake exists to reproduce.
		return "codex\n  Hooks\n  1 hook needs review before it can run.\n" +
			"› 2. Trust all and continue\n" +
			"  Press enter to confirm or esc to go back\n", nil
	}
	switch fake.stage {
	case "offered":
		return "codex\n› " + fake.composer +
			"\n  /rename  rename the current thread\n" + fakeStatusLine, nil
	case "prompt":
		// The rename modal paints over the composer, exactly as the real one
		// does — no composer glyph on screen while it is up.
		return "codex\n▌ Name thread\n▌\n▌ Type a name and press Enter\n", nil
	case "empty":
		return "codex\n▌ Name thread\n▌ Type a name and press Enter\n" +
			"Thread name cannot be empty.\n", nil
	default:
		header := "codex"
		if fake.name != "" {
			header = "• Session renamed to " + fake.name + ".\ncodex · " + fake.name
		}
		return header + "\n› " + fake.composer + "\n" + fakeStatusLine, nil
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
		if fake.stage == "composer" && fake.name != "" &&
			(fake.deafComposer || fake.dropsEnters > 0) {
			// A busy engine reading its input in bursts drops the newline that
			// arrives glued to the text.
			if fake.dropsEnters > 0 {
				fake.dropsEnters--
			}
			return nil
		}
		switch fake.stage {
		case "offered":
			// The real rename modal opens PRE-FILLED with the thread's
			// current name, so a spawn that types straight into it appends.
			fake.composer = fake.name
			if fake.opensPrompt {
				fake.stage = "prompt"
			} else {
				fake.composer = ""
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
		if fake.startupModals > 0 {
			fake.startupModals--
			return nil
		}
		fake.composer = ""
		fake.stage = "composer"
	}
	return nil
}

// collapseClears folds a run of BSpace presses into one "clear" token: the
// count is a bound on the field's length, not a contract.
func collapseClears(keys []string) []string {
	collapsed := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "key:BSpace" {
			if len(collapsed) > 0 && collapsed[len(collapsed)-1] == "clear" {
				continue
			}
			collapsed = append(collapsed, "clear")
			continue
		}
		collapsed = append(collapsed, key)
	}
	return collapsed
}

func testTimings() Timings {
	return Timings{
		Poll:  time.Millisecond,
		Boot:  200 * time.Millisecond,
		Step:  200 * time.Millisecond,
		Typed: time.Nanosecond,
	}
}

func codexRequest() Request {
	return Request{
		Engine:  "cx",
		Name:    "_KILL codex worker",
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
	if fake.name != "_KILL codex worker" {
		t.Fatalf("thread name = %q", fake.name)
	}
	want := []string{
		"literal:/rename",
		"key:Enter",
		"clear",
		"literal:_KILL codex worker",
		"key:Enter",
		"literal:read the incident report",
		"key:Enter",
	}
	if got := collapseClears(fake.keys); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("keystrokes\n got: %v\nwant: %v", got, want)
	}
}

// TestCodexBootsThroughStartupModals is the regression for the bug a stub
// without overlays could never catch: a real Codex boots into a full-screen
// hooks/trust modal that SWALLOWS keystrokes, so a spawn that starts typing at
// the first settled screen loses both the rename and the first prompt. The
// modals must be dismissed and the composer seen before anything is typed.
func TestCodexBootsThroughStartupModals(t *testing.T) {
	fake := newFakeCodex()
	fake.startupModals = 2
	result, err := Run(context.Background(), fake, codexRequest())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Named || !result.Prompted || len(result.Warnings) != 0 {
		t.Fatalf("result = %#v", result)
	}
	if fake.name != "_KILL codex worker" {
		t.Fatalf("thread name = %q", fake.name)
	}
	escapes := 0
	for index, key := range fake.keys {
		if key != "key:Escape" {
			// Every keystroke that is not one of the dismissals must come
			// AFTER the last of them: nothing may be typed into a modal.
			if escapes < 2 {
				t.Fatalf("keystroke %d (%s) was typed before the modals cleared: %v",
					index, key, fake.keys)
			}
			continue
		}
		escapes++
	}
	if escapes != 2 {
		t.Fatalf("dismissals = %d, want 2: %v", escapes, fake.keys)
	}
}

// TestCodexComposerFlashBeforeAModalIsNotReadiness is the live bug itself: the
// composer appeared, the modal painted over it a beat later, and the rename
// went into the modal. One sighting of a composer is not a ready chat.
func TestCodexComposerFlashBeforeAModalIsNotReadiness(t *testing.T) {
	fake := newFakeCodex()
	fake.modalAfterReads = 2
	result, err := Run(context.Background(), fake, codexRequest())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Named || !result.Prompted || len(result.Warnings) != 0 {
		t.Fatalf("result = %#v (warnings must be empty — the modal was survivable)", result)
	}
	if fake.name != "_KILL codex worker" {
		t.Fatalf("thread name = %q", fake.name)
	}
}

// TestCodexStuckAtAStartupScreenTypesNothing: when the overlay will not clear,
// the chat is left strictly alone — an unnamed, unprompted, LIVE chat the user
// is told to go clear by hand beats a name and a prompt fed to a modal.
func TestCodexStuckAtAStartupScreenTypesNothing(t *testing.T) {
	fake := newFakeCodex()
	fake.stuckModal = true
	result, err := Run(context.Background(), fake, codexRequest())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Named || result.Prompted {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Warnings) != 1 ||
		!strings.Contains(result.Warnings[0], "startup screen") {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
	for _, key := range fake.keys {
		if key != "key:Escape" {
			t.Fatalf("something was typed into a startup modal: %v", fake.keys)
		}
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
		if key == "literal:_KILL codex worker" {
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
		{"_KILL worker 3", "_KILL worker 3"},
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

// TestCodexPromptIsResentUntilItLeavesTheComposer is the failure this package
// shipped with: the prompt was typed, one Enter was sent, the engine dropped
// it, and pfm reported a working chat that had never been asked anything.
func TestCodexPromptIsResentUntilItLeavesTheComposer(t *testing.T) {
	fake := newFakeCodex()
	fake.dropsEnters = 2
	result, err := Run(context.Background(), fake, codexRequest())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Prompted || len(result.Warnings) != 0 {
		t.Fatalf("result = %#v, want a delivered prompt and no warning", result)
	}
	if fake.composer != "" {
		t.Fatalf("composer still holds %q", fake.composer)
	}
}

// TestCodexPromptThatNeverSubmitsIsReportedUndelivered: when the composer will
// not let go of the text, the chat is still up — and the report says plainly
// that the prompt is sitting in it.
func TestCodexPromptThatNeverSubmitsIsReportedUndelivered(t *testing.T) {
	fake := newFakeCodex()
	fake.deafComposer = true
	result, err := Run(context.Background(), fake, codexRequest())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Named {
		t.Fatalf("result = %#v, want the rename to have landed", result)
	}
	if result.Prompted {
		t.Fatal("Prompted = true for a prompt the composer never released")
	}
	if len(result.Warnings) != 1 ||
		!strings.Contains(result.Warnings[0], "not delivered") {
		t.Fatalf("warnings = %q, want one naming the undelivered prompt", result.Warnings)
	}
}
