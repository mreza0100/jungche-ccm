package inject

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/resolve"
)

type fakeIdentifier struct {
	identity resolve.Identity
	err      error
}

func (fake fakeIdentifier) Identify(context.Context) (resolve.Identity, error) {
	return fake.identity, fake.err
}

func newSignatureEngine(
	t *testing.T,
	socket string,
	tmux *fakeTmux,
	identifier SelfIdentifier,
) *Engine {
	t.Helper()
	return newSignatureEngineWith(t, socket, tmux, identifier, &fakeSpawner{})
}

func newSignatureEngineWith(
	t *testing.T,
	socket string,
	tmux *fakeTmux,
	identifier SelfIdentifier,
	spawner ThenSpawner,
) *Engine {
	t.Helper()
	t.Setenv("PFM_HOME", t.TempDir())
	t.Setenv("PFM_TMUX_DIR", filepath.Join(t.TempDir(), "no-tmux"))
	t.Setenv("PFM_PROC_ROOT", filepath.Join(t.TempDir(), "no-proc"))
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CHAT_INJECT_SOCKET", "")
	clearStatedSender(t)
	engine, err := New(Dependencies{
		Resolver: fakeResolver{
			socket: filepath.Join("/tmp", "tmux-jail", socket),
			target: "%1",
		},
		Tmux:       tmux,
		Spawner:    spawner,
		Identifier: identifier,
		Options: Options{
			Poll:        time.Nanosecond,
			EnterGap:    time.Nanosecond,
			EnterSettle: time.Nanosecond,
			ProofSettle: time.Nanosecond,
			BusyTries:   2,
			StashTries:  2,
			SettleTries: 2,
			EnterTries:  2,
			LockTimeout: time.Second,
			LockPoll:    time.Nanosecond,
			LockMaxHold: time.Second,
			LockRoot:    t.TempDir(),
			ThenLogRoot: t.TempDir(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

// clearStatedSender keeps an ambient CHAT_SENDER_* out of the jail: a
// developer running the suite from inside a chat would otherwise sign every
// fixture with their own identity.
func clearStatedSender(t *testing.T) {
	t.Helper()
	t.Setenv(SenderSessionEnv, "")
	t.Setenv(SenderLabelEnv, "")
	t.Setenv(SenderIDEnv, "")
}

// TestSignatureUsesTheSenderStatedByTheSpawningChat is the fix for the defect
// this rung exists for: a --then waiter is detached with setsid, and a codex
// tool shell carries no $TMUX and no session id, so the waiter can derive
// NOTHING and every steer it delivered went out UNSIGNED. The chat resolves
// its identity while it still can and states it to the waiter.
func TestSignatureUsesTheSenderStatedByTheSpawningChat(t *testing.T) {
	fake := &fakeTmux{capture: "conversation\n❯ ", submitOnEnter: true}
	engine := newSignatureEngine(
		t,
		"cc-1-2-3",
		fake,
		fakeIdentifier{err: resolve.ErrNoTmux},
	)
	t.Setenv(SenderSessionEnv, "cx-1700000000-1-1")
	t.Setenv(SenderLabelEnv, "WAVE_ORCHESTRATOR")
	t.Setenv(SenderIDEnv, "019ffd1e-300f-7872")
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "dispatch from a detached waiter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Unsigned {
		t.Fatalf("Inject() = %+v", result)
	}
	for _, want := range []string{
		"sid 019ffd1e",
		"to reply: /chat:inject cx-1700000000-1-1 <message>",
		"🔖 WAVE_ORCHESTRATOR",
	} {
		if !strings.Contains(lastLiteral(fake), want) {
			t.Fatalf("typed text %q lacks %q", lastLiteral(fake), want)
		}
	}
}

// TestSignatureStatesAnUnderivableIdentity covers chat.sh:648-657: when every
// derivation source is empty the footer says so out loud, because a silently
// dropped footer is indistinguishable from a signed message at the recipient.
func TestSignatureStatesAnUnderivableIdentity(t *testing.T) {
	fake := &fakeTmux{capture: "conversation\n❯ ", submitOnEnter: true}
	engine := newSignatureEngine(
		t,
		"cc-1-2-3",
		fake,
		fakeIdentifier{err: resolve.ErrNoTmux},
	)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "who is speaking",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || !result.Unsigned {
		t.Fatalf("Inject() = %+v", result)
	}
	want := "UNSIGNED — sender identity underivable; " +
		"no tmux session (ambient or via ancestry); no session id"
	if !strings.Contains(lastLiteral(fake), want) {
		t.Fatalf("typed text %q lacks %q", lastLiteral(fake), want)
	}
	if !strings.Contains(result.Proof, want) {
		t.Fatalf("proof %q lacks the unsigned marker", result.Proof)
	}
}

// TestSignatureUsesAncestryRecoveredIdentity covers chat.sh:65-96: an inject
// from a codex-origin shell — no $TMUX, no session id in this process — still
// signs, because the handle is recovered from the sender's own process chain.
func TestSignatureUsesAncestryRecoveredIdentity(t *testing.T) {
	fake := &fakeTmux{capture: "conversation\n❯ ", submitOnEnter: true}
	engine := newSignatureEngine(t, "cc-1-2-3", fake, fakeIdentifier{
		identity: resolve.Identity{
			Session:    "cx-1700000000-1-1",
			SocketPath: "/tmp/tmux-jail/cx-1700000000-1-1",
			SocketName: "cx-1700000000-1-1",
			Pane:       "%3",
			Engine:     resolve.CodexEngine,
			ID:         "thread-xyz",
			Source:     "ancestry",
			Recovered:  true,
		},
	})
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "recovered sender",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Unsigned {
		t.Fatalf("Inject() = %+v", result)
	}
	for _, want := range []string{
		"sid thread-x",
		"to reply: /chat:inject cx-1700000000-1-1 <message>",
	} {
		if !strings.Contains(lastLiteral(fake), want) {
			t.Fatalf("typed text %q lacks %q", lastLiteral(fake), want)
		}
	}
	if strings.Contains(lastLiteral(fake), "UNSIGNED") {
		t.Fatalf("recovered identity still signed UNSIGNED: %q", lastLiteral(fake))
	}
}

// TestSignatureCodexLabelFallsBackToTheWindowName covers chat.sh:104-121: a
// codex chat has no 🔖 statusline, so its label is the tmux window name — the
// human thread name — carried BARE so a reply by that label resolves.
func TestSignatureCodexLabelFallsBackToTheWindowName(t *testing.T) {
	fake := &fakeTmux{
		capture:       "codex conversation\n› ",
		windowName:    "Fleet Porter",
		submitOnEnter: true,
	}
	engine := newSignatureEngine(t, "cx-1-2-3", fake, fakeIdentifier{
		identity: resolve.Identity{
			Session:    "cx-1700000000-1-1",
			SocketPath: "/tmp/tmux-jail/cx-1700000000-1-1",
			SocketName: "cx-1700000000-1-1",
			Pane:       "%3",
			Engine:     resolve.CodexEngine,
		},
	})
	if _, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "labelled by window name",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(lastLiteral(fake), "🔖 Fleet Porter") {
		t.Fatalf("typed text %q lacks the codex window-name label", lastLiteral(fake))
	}

	// A real 🔖 statusline always wins — the Claude path is unchanged.
	labelled := &fakeTmux{
		capture:       "🥇 │ 🔖 Founder │ status\nconversation\n❯ ",
		windowName:    "Fleet Porter",
		submitOnEnter: true,
	}
	engine = newSignatureEngine(t, "cc-1-2-3", labelled, fakeIdentifier{
		identity: resolve.Identity{
			Session:    "cc-1700000000-1-1",
			SocketPath: "/tmp/tmux-jail/cc-1700000000-1-1",
			SocketName: "cc-1700000000-1-1",
			Engine:     resolve.ClaudeEngine,
		},
	})
	if _, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "labelled by statusline",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(lastLiteral(labelled), "🔖 Founder") ||
		strings.Contains(lastLiteral(labelled), "Fleet Porter") {
		t.Fatalf("statusline label lost to the window name: %q", lastLiteral(labelled))
	}
}

// TestCaptureLabelAcceptsTheRetiredMedal keeps 🍀 in the label anchor: the
// account is gone, but chats labelled while it was live still render it.
func TestCaptureLabelAcceptsTheRetiredMedal(t *testing.T) {
	for _, medal := range []string{"🥇", "🥈", "🥉", "🍀"} {
		line := medal + " │ 🔖 Founder │ ctx 12%"
		if label := captureLabel("noise\n" + line + "\nmore"); label != "Founder" {
			t.Fatalf("captureLabel(%q) = %q, want Founder", line, label)
		}
	}
	if label := captureLabel("🔖 Founder │ no medal here"); label != "" {
		t.Fatalf("medal-less line produced a label: %q", label)
	}
}

// lastLiteral is the text most recently typed into the fake pane; the fake
// clears its pending literal when a submit lands, so the history is what a
// signature assertion must read.
func lastLiteral(fake *fakeTmux) string {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.literals) == 0 {
		return ""
	}
	return fake.literals[len(fake.literals)-1]
}
