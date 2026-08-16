package inject

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/resolve"
)

// TestCompactFocusRuleRefusesBeforeAnyKey covers chat.sh:591-625: a /compact
// primary without a steer, an over-long /compact focus, and a steer that is
// itself a /compact all die at the caller, before a single key is sent.
func TestCompactFocusRuleRefusesBeforeAnyKey(t *testing.T) {
	tests := []struct {
		name     string
		request  Request
		wantCode int
		wantText string
	}{
		{
			name:     "steerless compact refused",
			request:  Request{Target: "chat", Message: "/compact hold: read /tmp/x.md"},
			wantCode: 6,
			wantText: "requires a then steer",
		},
		{
			name: "long compact focus refused",
			request: Request{
				Target:  "chat",
				Message: "/compact " + strings.Repeat("f", CompactFocusMax),
				Then:    []string{"carry on"},
			},
			wantCode: 6,
			wantText: "a body this long is typed as a PASTE",
		},
		{
			name: "compact steering into compact refused",
			request: Request{
				Target:  "chat",
				Message: "/compact hold: read /tmp/x.md",
				Then:    []string{" /compact again"},
			},
			wantCode: 6,
			wantText: "must not itself start with /compact",
		},
		{
			name: "empty steer refused",
			request: Request{
				Target:  "chat",
				Message: "plain work",
				Then:    []string{"  "},
			},
			wantCode: 6,
			wantText: "must be non-empty",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeTmux{capture: "conversation\n❯ ", submitOnEnter: true}
			spawner := &fakeSpawner{}
			engine := newTestEngineWith(t, "cc-1-2-3", fake, spawner)
			result, err := engine.Inject(context.Background(), test.request)
			if err != nil {
				t.Fatal(err)
			}
			if result.Code != test.wantCode || result.Status != "refused" {
				t.Fatalf("Inject() = %+v", result)
			}
			if !strings.Contains(result.Message, test.wantText) {
				t.Fatalf("diagnostic %q lacks %q", result.Message, test.wantText)
			}
			if fake.literal != "" || len(fake.keys) != 0 {
				t.Fatalf("refusal touched the pane: literal=%q keys=%q", fake.literal, fake.keys)
			}
			if len(spawner.spawned()) != 0 {
				t.Fatalf("refusal armed a waiter: %+v", spawner.spawned())
			}
		})
	}
}

// TestCompactWithSteerArmsWaiterOnlyAfterConfirmedSubmit covers chat.sh:922-951:
// the chain is armed after the primary submit is CONFIRMED, never before, and
// an unconfirmed submit arms nothing.
func TestCompactWithSteerArmsWaiterOnlyAfterConfirmedSubmit(t *testing.T) {
	fake := &fakeTmux{capture: "conversation\n❯ ", submitOnEnter: true}
	spawner := &fakeSpawner{}
	engine := newTestEngineWith(t, "cc-1-2-3", fake, spawner)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "/compact hold: read /tmp/hold.md — keep the lock scheme",
		Then:    []string{"resume the port", "then run the tests"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Status != "delivered" || result.Steers != 2 {
		t.Fatalf("Inject() = %+v", result)
	}
	spawned := spawner.spawned()
	if len(spawned) != 1 {
		t.Fatalf("spawned %d waiters, want exactly one", len(spawned))
	}
	if spawned[0].Target != "%1" ||
		len(spawned[0].Steers) != 2 ||
		spawned[0].Steers[0] != "resume the port" ||
		spawned[0].Steers[1] != "then run the tests" {
		t.Fatalf("waiter = %+v", spawned[0])
	}
	if !strings.HasPrefix(filepath.Base(spawned[0].LogPath), "chat-then-") ||
		!strings.HasSuffix(spawned[0].LogPath, ".log") {
		t.Fatalf("steer log path = %q", spawned[0].LogPath)
	}
	if !strings.Contains(result.Message, "2 then steer(s) queued") {
		t.Fatalf("banner does not report the queued steers: %q", result.Message)
	}

	// A /compact travels bare: a footer would corrupt the harness command's
	// argument (chat.sh:659-668).
	if strings.Contains(fake.literal, "to reply:") {
		t.Fatalf("harness command was signed: %q", fake.literal)
	}

	unconfirmed := &fakeTmux{capture: "conversation\n❯ "}
	spawner = &fakeSpawner{}
	engine = newTestEngineWith(t, "cc-1-2-3", unconfirmed, spawner)
	result, err = engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "/compact hold: read /tmp/hold.md",
		Then:    []string{"resume the port"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 6 || result.Steers != 0 || len(spawner.spawned()) != 0 {
		t.Fatalf(
			"unconfirmed submit armed a steer: result=%+v spawned=%+v",
			result,
			spawner.spawned(),
		)
	}
}

// TestWaiterCarriesTheSenderTheChatResolved covers the hop identity is lost
// at: the waiter runs detached, so it must be TOLD who armed it. The primary
// here is a /compact — exempt from signing — which is exactly the case that
// hid the defect: the chat never needed its own identity for the message it
// typed, only for the steer a severed process would deliver minutes later.
func TestWaiterCarriesTheSenderTheChatResolved(t *testing.T) {
	fake := &fakeTmux{
		capture:       "codex conversation\n› ",
		windowName:    "WAVE_ORCHESTRATOR",
		submitOnEnter: true,
	}
	spawner := &fakeSpawner{}
	engine := newSignatureEngineWith(t, "cc-1-2-3", fake, fakeIdentifier{
		identity: resolve.Identity{
			Session:    "cx-1700000000-1-1",
			SocketPath: "/tmp/tmux-jail/cx-1700000000-1-1",
			SocketName: "cx-1700000000-1-1",
			Pane:       "%3",
			Engine:     resolve.CodexEngine,
			ID:         "019ffd1e-300f",
			Source:     "ancestry",
			Recovered:  true,
		},
	}, spawner)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "/compact hold: read /tmp/hold.md",
		Then:    []string{"resume the wave"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Steers != 1 {
		t.Fatalf("Inject() = %+v", result)
	}
	spawned := spawner.spawned()
	if len(spawned) != 1 {
		t.Fatalf("spawned %d waiters, want exactly one", len(spawned))
	}
	sender := spawned[0].Sender
	if sender.Session != "cx-1700000000-1-1" ||
		sender.Label != "WAVE_ORCHESTRATOR" ||
		sender.UUID != "019ffd1e-300f" {
		t.Fatalf("waiter carries sender %+v", sender)
	}
}

// TestCommandThenSpawnerStatesTheSenderToTheWaiter proves the identity survives
// the process boundary: the detached waiter derives nothing, so the three
// CHAT_SENDER_* names are the whole of what it knows about who armed it.
func TestCommandThenSpawnerStatesTheSenderToTheWaiter(t *testing.T) {
	scratch := t.TempDir()
	dump := filepath.Join(scratch, "environment.txt")
	stub := filepath.Join(scratch, "setsid-stub")
	script := "#!/bin/sh\nenv > " + dump + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	// An inherited definition must lose to the one this spawn states, or a
	// chain hop signs as whoever spawned the hop before it.
	t.Setenv(SenderSessionEnv, "cc-STALE-HOP")
	t.Setenv(SenderLabelEnv, "STALE_LABEL")
	spawner := CommandThenSpawner{
		Executable: filepath.Join(scratch, "pfm"),
		Setsid:     stub,
	}
	err := spawner.Spawn(context.Background(), SteerSpawn{
		SocketPath: filepath.Join(scratch, "cx-1-2-3"),
		Target:     "%3",
		Steers:     []string{"resume the wave"},
		LogPath:    filepath.Join(scratch, "steer.log"),
		Sender: Sender{
			Session: "cx-1700000000-1-1",
			UUID:    "019ffd1e-300f",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	environment := string(raw)
	for _, want := range []string{
		SenderSessionEnv + "=cx-1700000000-1-1",
		SenderIDEnv + "=019ffd1e-300f",
	} {
		if !strings.Contains(environment, want) {
			t.Fatalf("waiter environment lacks %q", want)
		}
	}
	// The label was not resolved by this chat, so nothing is stated for it:
	// an inherited one would sign this message with another chat's name.
	for _, unwanted := range []string{
		"cc-STALE-HOP",
		"STALE_LABEL",
		SenderLabelEnv + "=",
	} {
		if strings.Contains(environment, unwanted) {
			t.Fatalf("waiter environment still carries %q", unwanted)
		}
	}
}

// TestSteerSpawnFailureIsReportedNotSwallowed keeps a failed arming visible:
// the primary landed, the follow-up did not, and the caller is told which.
func TestSteerSpawnFailureIsReportedNotSwallowed(t *testing.T) {
	fake := &fakeTmux{capture: "conversation\n❯ ", submitOnEnter: true}
	spawner := &fakeSpawner{err: errors.New("no setsid")}
	engine := newTestEngineWith(t, "cc-1-2-3", fake, spawner)
	result, err := engine.Inject(context.Background(), Request{
		Target:  "chat",
		Message: "/compact hold: read /tmp/hold.md",
		Then:    []string{"resume"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 6 ||
		result.Steers != 0 ||
		!strings.Contains(result.Message, "could NOT arm") {
		t.Fatalf("Inject() = %+v", result)
	}
}

// TestLockNamespaceMatchesChatShell proves a Go inject and a chat.sh inject
// into the same pane contend for the SAME lock directory (chat.sh:144-172).
func TestLockNamespaceMatchesChatShell(t *testing.T) {
	root := t.TempDir()
	key := "/tmp/tmux-1000/cc-1-2-3:%5"
	want := "_tmp_tmux-1000_cc-1-2-3_%5.lock"
	if got := lockDirName(key); got != want {
		t.Fatalf("lockDirName(%q) = %q, want %q", key, got, want)
	}
	lock, err := acquireTargetLock(root, key, time.Second, time.Millisecond, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	owner, readErr := os.ReadFile(filepath.Join(root, want, "owner"))
	if readErr != nil {
		t.Fatalf("owner file: %v", readErr)
	}
	fields := strings.Fields(string(owner))
	if len(fields) != 2 || fields[0] != strconv.Itoa(os.Getpid()) {
		t.Fatalf("owner file = %q, want \"<pid> <epoch>\"", owner)
	}
	if _, err := acquireTargetLock(
		root,
		key,
		20*time.Millisecond,
		time.Millisecond,
		time.Minute,
	); err == nil {
		t.Fatal("second holder acquired a held lock")
	}
	lock.release()
	second, err := acquireTargetLock(root, key, time.Second, time.Millisecond, time.Minute)
	if err != nil {
		t.Fatalf("lock not released: %v", err)
	}
	second.release()

	// A lock whose owner process is gone is STOLEN, not waited on.
	stalePath := filepath.Join(root, want)
	if err := os.MkdirAll(stalePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(stalePath, "owner"),
		[]byte("2147483646 1\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	stolen, err := acquireTargetLock(root, key, time.Second, time.Millisecond, time.Minute)
	if err != nil {
		t.Fatalf("stale lock was not stolen: %v", err)
	}
	stolen.release()
}
