package inject

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// injectTmuxJail runs real tmux servers on a scratch TMUX_TMPDIR so no test
// ever reaches a live fleet socket.
type injectTmuxJail struct {
	root    string
	home    string
	tmuxDir string
	sockets []string
}

func newInjectTmuxJail(t *testing.T) *injectTmuxJail {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux binary is not installed")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}
	root, err := os.MkdirTemp("/tmp", "ccij")
	if err != nil {
		t.Fatal(err)
	}
	jail := &injectTmuxJail{
		root:    root,
		home:    filepath.Join(root, "home"),
		tmuxDir: filepath.Join(root, "tmux-"+strconv.Itoa(os.Getuid())),
	}
	for _, directory := range []string{jail.home, jail.tmuxDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", jail.home)
	t.Setenv("TMUX", "")
	// The waiter exports CHAT_INJECT_SOCKET the way chat.sh does; claiming it
	// here restores the caller's value when the test ends.
	t.Setenv("CHAT_INJECT_SOCKET", "")
	t.Setenv("TMUX_TMPDIR", jail.root)
	t.Setenv("TMPDIR", jail.root)
	t.Setenv("PFM_HOME", jail.home)
	t.Setenv("PFM_TMUX_DIR", jail.tmuxDir)
	t.Setenv("PFM_PROC_ROOT", filepath.Join(root, "proc"))
	t.Cleanup(func() {
		for _, socket := range jail.sockets {
			_ = jail.command("-L", socket, "kill-server").Run()
		}
		if err := os.RemoveAll(jail.root); err != nil {
			t.Errorf("remove inject tmux jail: %v", err)
		}
	})
	return jail
}

func (jail *injectTmuxJail) command(arguments ...string) *exec.Cmd {
	command := exec.Command("tmux", arguments...)
	command.Env = append(
		os.Environ(),
		"HOME="+jail.home,
		"TMUX=",
		"TMUX_TMPDIR="+jail.root,
	)
	return command
}

// busyThenIdleUI is a fake chat TUI: it renders the busy spinner detail chat.sh
// keys on, then clears to an idle composer after a delay, and echoes a
// submitted line as "USER:<text>". A line typed while it is still busy is
// echoed as "BUSY-TYPED:", so a premature steer is visible in the capture.
const busyThenIdleUI = `import os, select, sys, time, tty
tty.setraw(0)
delay = float(sys.argv[1])
start = time.time()
sys.stdout.write("Working (2s · 9 tokens)\r\n")
sys.stdout.flush()
idle = False
buf = bytearray()
while True:
    if not idle and time.time() - start >= delay:
        idle = True
        sys.stdout.write("\x1b[2J\x1b[H❯ ")
        sys.stdout.flush()
    ready, _, _ = select.select([0], [], [], 0.05)
    if not ready:
        continue
    ch = os.read(0, 1)
    if not ch:
        break
    if ch == b"\x13":
        buf.clear()
        sys.stdout.write("\r\x1b[2K❯ ")
        sys.stdout.flush()
    elif ch in (b"\r", b"\n"):
        if buf:
            text = bytes(buf).decode("utf-8", "replace")
            tag = "USER:" if idle else "BUSY-TYPED:"
            sys.stdout.write("\r\n" + tag + text + "\r\n❯ ")
            sys.stdout.flush()
            buf.clear()
    elif ch == b"\x1b":
        buf.clear()
        sys.stdout.write("\r\x1b[2K❯ ")
        sys.stdout.flush()
    else:
        buf.extend(ch)
        sys.stdout.buffer.write(ch)
        sys.stdout.flush()
`

func (jail *injectTmuxJail) startBusyPane(
	t *testing.T,
	socket, session string,
	busyFor time.Duration,
) string {
	t.Helper()
	script := filepath.Join(jail.root, "ui.py")
	if err := os.WriteFile(script, []byte(busyThenIdleUI), 0o700); err != nil {
		t.Fatal(err)
	}
	command := jail.command(
		"-f",
		"/dev/null",
		"-L",
		socket,
		"new-session",
		"-d",
		"-s",
		session,
		"-n",
		"steer-window",
		"python3",
		script,
		strconv.FormatFloat(busyFor.Seconds(), 'f', 2, 64),
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start jailed pane: %v: %s", err, output)
	}
	jail.sockets = append(jail.sockets, socket)
	output, err := jail.command("-L", socket, "list-panes", "-F", "#{pane_id}").Output()
	if err != nil {
		t.Fatal(err)
	}
	pane := strings.TrimSpace(string(output))
	// The pane is only busy once its UI has rendered the spinner detail line;
	// asserting before that would measure the fixture's startup, not the guard.
	deadline := time.Now().Add(10 * time.Second)
	for {
		capture, err := CommandTmux{}.Capture(
			context.Background(),
			filepath.Join(jail.tmuxDir, socket),
			pane,
			false,
			0,
		)
		if err == nil && IsBusy(capture) {
			return pane
		}
		if time.Now().After(deadline) {
			t.Fatalf("jailed pane never became busy: %q", capture)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestJailedThenWaiterDeliversAfterIdleExactlyOnce is the --then contract in a
// real tmux server: the waiter rides out the busy turn (chat.sh:1062-1077) and
// the steer lands once, after idle, with the submit confirmed.
func TestJailedThenWaiterDeliversAfterIdleExactlyOnce(t *testing.T) {
	jail := newInjectTmuxJail(t)
	socket := "cc-1700000900-1-1"
	pane := jail.startBusyPane(t, socket, "steer-session", 2500*time.Millisecond)
	socketPath := filepath.Join(jail.tmuxDir, socket)

	engine, err := New(Dependencies{
		Resolver: fakeResolver{socket: socketPath, target: pane},
		Tmux:     CommandTmux{},
		Spawner:  &fakeSpawner{},
		Options: Options{
			Poll:            50 * time.Millisecond,
			EnterGap:        50 * time.Millisecond,
			EnterSettle:     200 * time.Millisecond,
			ProofSettle:     150 * time.Millisecond,
			BusyTries:       2,
			InterruptTries:  2,
			StashTries:      2,
			SettleTries:     10,
			EnterTries:      6,
			LockTimeout:     5 * time.Second,
			LockPoll:        20 * time.Millisecond,
			LockMaxHold:     30 * time.Second,
			LockRoot:        filepath.Join(jail.root, "chat-inject-locks"),
			ThenLogRoot:     jail.root,
			ThenMin:         100 * time.Millisecond,
			ThenBusyTries:   10,
			ThenIdlePoll:    100 * time.Millisecond,
			ThenIdleTries:   80,
			ThenIdleStable:  3,
			ThenSettle:      100 * time.Millisecond,
			Sender:          &Sender{Session: "sender", Label: "Founder", UUID: "abcdef123456"},
			CodexInlineMax:  CodexInlineMax,
			AbsoluteByteMax: AbsoluteMessageMax,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// The pane is busy right now: an inject must refuse instead of typing.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	busy, err := engine.Inject(ctx, Request{
		Target:  "steer-session",
		Message: "too early",
	})
	if err != nil {
		t.Fatal(err)
	}
	if busy.Code != 7 || busy.Typed {
		t.Fatalf("busy pane accepted an inject: %+v", busy)
	}

	steer := "post compact steer landed"
	result, err := engine.DeliverThen(ctx, socketPath, pane, []string{steer})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Status != "delivered" || !result.Typed {
		t.Fatalf("DeliverThen() = %+v", result)
	}
	capture, err := CommandTmux{}.Capture(ctx, socketPath, pane, false, FullScrollback)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(capture, "BUSY-TYPED:") {
		t.Fatalf("steer landed while the pane was busy:\n%s", capture)
	}
	if occurrences := strings.Count(capture, "USER:"+steer); occurrences != 1 {
		t.Fatalf("steer landed %d times, want exactly once:\n%s", occurrences, capture)
	}
	if !strings.Contains(capture, "to reply: /chat:inject sender <message>") {
		t.Fatalf("steer was delivered unsigned:\n%s", capture)
	}
	if !strings.Contains(result.Proof, "USER:"+steer) {
		t.Fatalf("delivery proof missing the steer: %q", result.Proof)
	}
}

// TestJailedCaptureReturnsWholeScrollback covers chat.sh:1215-1219: capture
// reads the entire retained buffer, not only the visible fold.
func TestJailedCaptureReturnsWholeScrollback(t *testing.T) {
	jail := newInjectTmuxJail(t)
	socket := "cc-1700000901-1-1"
	session := "scrollback-session"
	command := jail.command(
		"-f",
		"/dev/null",
		"-L",
		socket,
		"new-session",
		"-d",
		"-s",
		session,
		"-x",
		"80",
		"-y",
		"10",
		"sh",
		"-c",
		"i=1; while [ $i -le 60 ]; do echo scrollrow-$i; i=$((i+1)); done; sleep 60",
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start scrollback pane: %v: %s", err, output)
	}
	jail.sockets = append(jail.sockets, socket)
	socketPath := filepath.Join(jail.tmuxDir, socket)
	engine, err := New(Dependencies{
		Resolver: fakeResolver{socket: socketPath, target: session},
		Tmux:     CommandTmux{},
		Spawner:  &fakeSpawner{},
		Options:  Options{LockRoot: filepath.Join(jail.root, "locks")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	deadline := time.Now().Add(10 * time.Second)
	for {
		_, text, code, detail, captureErr := engine.Capture(ctx, session, 0)
		if captureErr != nil {
			t.Fatal(captureErr)
		}
		if code != 0 {
			t.Fatalf("Capture() code=%d detail=%q", code, detail)
		}
		if strings.Contains(text, "scrollrow-1\n") &&
			strings.Contains(text, "scrollrow-60") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("capture never reached the top of the scrollback:\n%s", text)
		}
		time.Sleep(100 * time.Millisecond)
	}
	_, tailed, _, _, err := engine.Capture(ctx, session, 5)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(tailed, "scrollrow-1\n") ||
		!strings.Contains(tailed, "scrollrow-60") {
		t.Fatalf("tail bound was not applied after the capture:\n%s", tailed)
	}
}
