package inject

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"hostops/pfm/internal/resolve"
)

// fakeSelf answers "who am I" without asking the machine. Resolving the target
// "self" for real needs a live tmux seat, so a test that leans on the ambient
// environment passes on a developer box that happens to be running inside a
// chat and fails in the container — which is exactly what this one did before
// the fence caught it. Supplying the identity makes the test measure the code.
type fakeSelf struct {
	identity resolve.Identity
}

func (fake fakeSelf) Identify(context.Context) (resolve.Identity, error) {
	return fake.identity, nil
}

// The two repros below pin the SAME defect from its two opposite ends.
//
// waitForSettledTurn rides out "the turn the primary started" using nothing but
// a busy bit. That bit has no identity: it reads true while the compaction runs
// AND while the caller's own turn runs AND while the post-compaction session
// resumes on its own. So the waiter latches onto whichever turn happened to be
// busy when it woke up, and then races whichever idle arrives first.
//
// Which way it loses depends on timing alone:
//
//   - the caller stops promptly -> the waiter latches the caller's turn, sees
//     the idle BEFORE compaction even starts, and delivers the steer EARLY,
//     into a pane that is about to be compacted out from under it.
//   - the caller keeps working -> the brief idle that follows the compaction is
//     shorter than the stability window, so the waiter misses its one true window
//     and delivers LATE, into work that already resumed.
//
// One bug, two faces. A fix that only moves the delivery point earlier or later
// trades one face for the other; the waiter has to identify the turn instead of
// counting on a coincidence.

// paneFrame is one observation of the pane plus the phase it belongs to, so a
// test can assert WHERE the waiter made its decision and not merely that it
// eventually returned.
type paneFrame struct {
	phase   string
	capture string
}

const (
	phaseCaller     = "caller-turn"      // the caller's own turn, still running
	phaseGap        = "pre-compact-idle" // idle, compaction has NOT started
	phaseCompacting = "compacting"       // the compaction turn is running
	phaseDone       = "compaction-done"  // idle, compaction provably finished
	phaseResumed    = "resumed-work"     // the session resumed on its own
	phaseLate       = "long-after"       // idle again, far too late to be useful
)

// Real captures. busyPattern (guards.go:13) keys on "esc to interrupt"; the
// compaction receipt is what Claude Code prints once compaction has actually
// happened, which is the only positive evidence in the pane that the turn the
// waiter was told to ride out is the turn that ran.
const (
	captureBusy    = "working on it\n  esc to interrupt\n"
	captureIdle    = "conversation\n❯ "
	captureReceipt = "Compacted (ctrl+o to see full summary)\n❯ "
)

// paneScript replays a fixed sequence of frames and remembers the last one it
// handed out, so the waiter's decision point is observable.
type paneScript struct {
	*fakeTmux
	mu     sync.Mutex
	frames []paneFrame
	served int
}

func (script *paneScript) Capture(
	_ context.Context,
	_, _ string,
	_ bool,
	_ int,
) (string, error) {
	script.mu.Lock()
	defer script.mu.Unlock()
	index := script.served
	if index >= len(script.frames) {
		index = len(script.frames) - 1
	}
	script.served++
	return script.frames[index].capture, nil
}

// decidedIn names the phase the waiter was looking at when it stopped waiting.
func (script *paneScript) decidedIn() string {
	script.mu.Lock()
	defer script.mu.Unlock()
	index := script.served - 1
	if index < 0 {
		index = 0
	}
	if index >= len(script.frames) {
		index = len(script.frames) - 1
	}
	return script.frames[index].phase
}

func newScriptedEngine(t *testing.T, frames []paneFrame) (*Engine, *paneScript) {
	t.Helper()
	fake := &fakeTmux{capture: captureIdle}
	engine := newTestEngine(t, "cc-1-2-3", fake)
	script := &paneScript{fakeTmux: fake, frames: frames}
	engine.tmux = script
	// Every wait is unbounded in tries and free in wall-clock, so the test
	// measures the waiter's DECISION and never the machine it runs on.
	engine.options.ThenMin = time.Nanosecond
	engine.options.Poll = time.Nanosecond
	engine.options.ThenIdlePoll = time.Nanosecond
	engine.options.ThenSettle = time.Nanosecond
	engine.options.ThenBusyTries = 200
	engine.options.ThenIdleTries = 200
	engine.options.ThenIdleStable = 3
	return engine, script
}

func repeatFrame(phase, capture string, count int) []paneFrame {
	frames := make([]paneFrame, 0, count)
	for range count {
		frames = append(frames, paneFrame{phase: phase, capture: capture})
	}
	return frames
}

// TestThenWaiterDoesNotDeliverBeforeTheCompactionRuns is the EARLY-landing
// repro. The caller ends its turn promptly, so the pane goes idle while the
// queued /compact is still waiting its turn. A waiter that only knows "was
// busy, now idle" reads that gap as the compaction having finished and fires
// the steer into a pane that is about to be compacted — the steer is consumed
// by the pre-compaction session and vanishes with it.
func TestThenWaiterDoesNotDeliverBeforeTheCompactionRuns(t *testing.T) {
	var frames []paneFrame
	frames = append(frames, repeatFrame(phaseCaller, captureBusy, 2)...)
	frames = append(frames, repeatFrame(phaseGap, captureIdle, 6)...)
	frames = append(frames, repeatFrame(phaseCompacting, captureBusy, 3)...)
	frames = append(frames, repeatFrame(phaseDone, captureReceipt, 6)...)

	engine, script := newScriptedEngine(t, frames)
	engine.waitForSettledTurn(context.Background(), "", "chat", true)

	if got := script.decidedIn(); got != phaseDone {
		t.Fatalf(
			"waiter released in phase %q, want %q: it delivered the steer "+
				"before the compaction ever ran, so the steer dies with the "+
				"pre-compaction session",
			got, phaseDone,
		)
	}
}

// TestThenWaiterDoesNotDeliverIntoResumedWork is the LATE-landing repro, and
// the one that actually happened. The caller does not stop after queueing the
// compaction, so the idle between its turn and the compaction is a single
// blink — shorter than the stability window. The waiter misses it, misses the
// equally brief idle right after the compaction, and finally releases once the
// resumed session goes quiet: the steer lands on top of work already in
// progress, which is the collision this pair of tests exists to prevent.
func TestThenWaiterDoesNotDeliverIntoResumedWork(t *testing.T) {
	var frames []paneFrame
	frames = append(frames, repeatFrame(phaseCaller, captureBusy, 4)...)
	frames = append(frames, paneFrame{phase: phaseGap, capture: captureIdle})
	frames = append(frames, repeatFrame(phaseCompacting, captureBusy, 3)...)
	frames = append(frames, paneFrame{phase: phaseDone, capture: captureReceipt})
	frames = append(frames, repeatFrame(phaseResumed, captureBusy, 8)...)
	frames = append(frames, repeatFrame(phaseLate, captureReceipt, 6)...)

	engine, script := newScriptedEngine(t, frames)
	engine.waitForSettledTurn(context.Background(), "", "chat", true)

	if got := script.decidedIn(); got != phaseDone {
		t.Fatalf(
			"waiter released in phase %q, want %q: it slept through the one "+
				"idle that followed the compaction and delivered the steer "+
				"into work that had already resumed",
			got, phaseDone,
		)
	}
}

// TestThenWaiterStillReleasesWithoutACompactionReceipt keeps the fix honest for
// every non-compaction primary (a reload steer, a plain queued message): those
// panes never print a receipt, and a waiter that insisted on one would hang
// until its bound expired and strand the chain. It must still ride out the
// turn — and it must still refuse to mistake the caller's own turn for it.
func TestThenWaiterStillReleasesWithoutACompactionReceipt(t *testing.T) {
	// The pre-compaction gap is deliberately LONGER than the stability window,
	// so a waiter that counts steady idle before it has seen the primary's turn
	// begin releases here and the test catches it. The trailing frames are busy
	// for the same reason in the other direction: a waiter that bounds out
	// without ever deciding ends up in resumed work, not in a phase that would
	// have let it pass by accident.
	var frames []paneFrame
	frames = append(frames, repeatFrame(phaseCaller, captureBusy, 2)...)
	frames = append(frames, repeatFrame(phaseGap, captureIdle, 5)...)
	frames = append(frames, repeatFrame(phaseCompacting, captureBusy, 3)...)
	frames = append(frames, repeatFrame(phaseDone, captureIdle, 3)...)
	frames = append(frames, repeatFrame(phaseResumed, captureBusy, 8)...)

	engine, script := newScriptedEngine(t, frames)
	engine.waitForSettledTurn(context.Background(), "", "chat", true)

	if got := script.decidedIn(); got != phaseDone {
		t.Fatalf(
			"waiter released in phase %q, want %q: with no receipt to key on "+
				"it must still ride out the turn that started AFTER the "+
				"caller's own turn ended",
			got, phaseDone,
		)
	}
}

// TestCompactionReceiptNeedsToAppear guards the one way the receipt rule could
// itself become a coincidence detector: a receipt still sitting in the pane
// from a PREVIOUS compaction is not evidence that this one ran. Only a receipt
// that appears while the waiter is watching counts.
func TestCompactionReceiptNeedsToAppear(t *testing.T) {
	var frames []paneFrame
	// The pane already shows an older receipt when the waiter wakes up.
	frames = append(frames, repeatFrame(phaseCaller, captureBusy, 2)...)
	frames = append(frames, repeatFrame(phaseGap, captureReceipt, 4)...)
	frames = append(frames, repeatFrame(phaseCompacting, captureBusy, 3)...)
	frames = append(frames, repeatFrame(phaseDone, captureReceipt, 6)...)

	engine, script := newScriptedEngine(t, frames)
	if !strings.Contains(frames[2].capture, "Compacted") {
		t.Fatalf("fixture no longer shows a stale receipt during the gap")
	}
	engine.waitForSettledTurn(context.Background(), "", "chat", true)

	if got := script.decidedIn(); got != phaseDone {
		t.Fatalf(
			"waiter released in phase %q, want %q: it accepted a receipt "+
				"left over from an earlier compaction as proof that this "+
				"compaction had run",
			got, phaseDone,
		)
	}
}

// TestThenWaiterDoesNotBurnTheBudgetWaitingForATurnThatAlreadyRan guards the
// bound on the new "wait for the primary's turn to start" step. If the turn
// began and ended inside ThenMin — or the pane simply never reports busy — that
// step has nothing left to observe, and waiting out the full idle budget would
// hold the steer for minutes and strand the chain. It must give up quickly,
// deliver anyway, and say the boundary was never observed rather than let a
// guess pass for proof.
func TestThenWaiterDoesNotBurnTheBudgetWaitingForATurnThatAlreadyRan(t *testing.T) {
	frames := repeatFrame(phaseDone, captureIdle, 4)

	engine, script := newScriptedEngine(t, frames)
	engine.options.ThenBusyTries = 3
	engine.options.ThenIdleTries = 500
	engine.options.ThenIdleStable = 2

	observed := engine.waitForSettledTurn(context.Background(), "", "chat", true)

	if observed {
		t.Fatal(
			"waiter claimed it observed a turn boundary when the pane never " +
				"went busy — that is a guess reported as proof",
		)
	}
	const budget = 20
	if script.served > budget {
		t.Fatalf(
			"waiter sampled the pane %d times before giving up, want at most "+
				"%d: it burned the idle budget waiting for a turn that was "+
				"never going to start, stranding the steer",
			script.served, budget,
		)
	}
}

// TestSelfCompactScheduleTellsTheCallerToStop covers the result half of the
// stop rule at the layer that actually writes it, so BOTH callers — the
// chat_self_compact MCP tool and `pfm chat inject --then self "/compact …"` —
// are covered by one assertion instead of one of them being quietly missed.
// That miss was real: the notice first shipped in the MCP handler only, which
// left the CLI path (the one that caused the collision) saying nothing.
func TestSelfCompactScheduleTellsTheCallerToStop(t *testing.T) {
	fake := &fakeTmux{capture: "Working (10s)\n› Ask Codex to do anything"}
	engine := newTestEngineWith(t, "cx-self-compact", fake, &fakeSpawner{})
	engine.whoami = fakeSelf{identity: resolve.Identity{
		Session:    "cx-self-compact",
		SocketPath: filepath.Join("/tmp", "tmux-jail", "cx-self-compact"),
		Pane:       "%1",
		Engine:     "codex",
		Source:     "test",
	}}

	result, err := engine.ScheduleAfterCurrentTurn(context.Background(), Request{
		Target:  "self",
		Message: "/compact hold the wave state",
		Then:    []string{"resume the wave"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 {
		t.Fatalf("scheduled self-compaction = %+v", result)
	}
	if !strings.Contains(result.Message, "STOP NOW") {
		t.Fatalf(
			"a queued SELF-compaction does not tell the caller to stop; the "+
				"waiter needs this caller's turn to end so it can tell the "+
				"compaction apart from it.\ngot: %s",
			result.Message,
		)
	}
}

// TestCompactOfAnotherChatDoesNotTellTheCallerToStop keeps the notice from
// becoming noise on the shape it does not apply to. When the target is somebody
// else's pane, the waiter watches THAT pane; what this caller does next cannot
// blur a boundary it is not part of, so telling it to stop would be wrong.
func TestCompactOfAnotherChatDoesNotTellTheCallerToStop(t *testing.T) {
	fake := &fakeTmux{capture: "Working (10s)\n› Ask Codex to do anything"}
	engine := newTestEngineWith(t, "cx-other-compact", fake, &fakeSpawner{})

	result, err := engine.ScheduleAfterCurrentTurn(context.Background(), Request{
		Target:  "chat",
		Message: "/compact hold the wave state",
		Then:    []string{"resume the wave"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Message, "STOP NOW") {
		t.Fatalf(
			"compacting ANOTHER chat told this caller to stop working; the "+
				"rule applies only to a chat compacting itself.\ngot: %s",
			result.Message,
		)
	}
}

// TestNonSelfWaiterDoesNotWaitOutATurnItDidNotStart pins the boundary of the
// caller-yield rule, and a regression the rule nearly introduced.
//
// Requiring an idle observation BEFORE the primary's turn is correct only when
// the pane being watched is the pane that asked for the wait — a chat compacting
// itself, whose own turn is still running when the waiter wakes. For any other
// target nothing else owns that pane, so its first busy already IS the primary's
// turn. Demanding a prior idle there waits out the primary, then waits again for
// a second turn that never comes, and finally releases on the bounded fallback:
// seconds late, and carrying a "no turn boundary observed" warning that is
// simply false. This is the shape every `--then` steer to another chat takes, so
// the regression would have been the common case, not the exotic one.
func TestNonSelfWaiterDoesNotWaitOutATurnItDidNotStart(t *testing.T) {
	// The pane is already busy with the primary's own turn on the first sample.
	var frames []paneFrame
	frames = append(frames, repeatFrame(phaseCompacting, captureBusy, 3)...)
	frames = append(frames, repeatFrame(phaseDone, captureIdle, 6)...)
	frames = append(frames, repeatFrame(phaseLate, captureBusy, 6)...)

	engine, script := newScriptedEngine(t, frames)
	observed := engine.waitForSettledTurn(context.Background(), "", "chat", false)

	if !observed {
		t.Fatal(
			"waiter reported no turn boundary for a non-self target whose " +
				"turn it watched start and finish — a false warning on the " +
				"most common --then shape",
		)
	}
	if got := script.decidedIn(); got != phaseDone {
		t.Fatalf(
			"waiter released in phase %q, want %q: it treated the primary's "+
				"own turn as somebody else's and waited for a second turn "+
				"that was never going to come",
			got, phaseDone,
		)
	}
}
