package resolve

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type resolveJail struct {
	root       string
	tmuxDir    string
	sidDir     string
	home       string
	sockets    []string
	scriptNext int
}

func newResolveJail(t *testing.T) *resolveJail {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux binary is not installed")
	}
	root, err := os.MkdirTemp("/tmp", "ccr")
	if err != nil {
		t.Fatal(err)
	}
	jail := &resolveJail{
		root:    root,
		tmuxDir: filepath.Join(root, "tmux-"+strconv.Itoa(os.Getuid())),
		sidDir:  filepath.Join(root, "sid"),
		home:    filepath.Join(root, "home"),
	}
	for _, directory := range []string{
		jail.tmuxDir,
		jail.sidDir,
		jail.home,
		filepath.Join(root, "claude"),
		filepath.Join(root, "codex"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", jail.home)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_TMPDIR", jail.root)
	t.Setenv("PFM_HOME", jail.home)
	t.Setenv("PFM_DB", filepath.Join(root, "fleet.db"))
	t.Setenv("PFM_SID_DIR", jail.sidDir)
	t.Setenv("PFM_CLAUDE_ROOTS", filepath.Join(root, "claude"))
	t.Setenv("PFM_CODEX_ROOT", filepath.Join(root, "codex"))
	t.Setenv("PFM_TMUX_DIR", jail.tmuxDir)
	t.Cleanup(func() {
		for _, socket := range jail.sockets {
			_ = jail.command("-L", socket, "kill-server").Run()
		}
		if err := os.RemoveAll(jail.root); err != nil {
			t.Errorf("remove resolve jail: %v", err)
		}
	})
	return jail
}

func (jail *resolveJail) command(arguments ...string) *exec.Cmd {
	command := exec.Command("tmux", arguments...)
	command.Env = append(
		os.Environ(),
		"HOME="+jail.home,
		"TMUX=",
		"TMUX_TMPDIR="+jail.root,
	)
	return command
}

func (jail *resolveJail) script(t *testing.T, label string) string {
	t.Helper()
	jail.scriptNext++
	path := filepath.Join(
		jail.root,
		fmt.Sprintf("p%03d.sh", jail.scriptNext),
	)
	content := "#!/bin/sh\nprintf '%s\\n' " +
		shellSingleQuote("🥇 │ 🔖 "+label+" │ status") +
		"\nexec sleep 120\n"
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (jail *resolveJail) start(
	t *testing.T,
	socket, session, window, label string,
) {
	t.Helper()
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
		window,
		jail.script(t, label),
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start %s: %v: %s", socket, err, output)
	}
	jail.sockets = append(jail.sockets, socket)
	deadline := time.Now().Add(5 * time.Second)
	for {
		output, err := jail.command(
			"-L",
			socket,
			"capture-pane",
			"-p",
			"-t",
			session,
		).CombinedOutput()
		if err == nil && strings.Contains(string(output), label) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"wait for %s label %q: %v: %s",
				socket,
				label,
				err,
				output,
			)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (jail *resolveJail) split(
	t *testing.T,
	socket, session, label string,
) {
	t.Helper()
	command := jail.command(
		"-L",
		socket,
		"split-window",
		"-d",
		"-t",
		session,
		jail.script(t, label),
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("split %s: %v: %s", socket, err, output)
	}
	if output, err := jail.command(
		"-L",
		socket,
		"select-layout",
		"-t",
		session,
		"tiled",
	).CombinedOutput(); err != nil {
		t.Fatalf("tile %s: %v: %s", socket, err, output)
	}
}

func TestJailedTmuxResolveContract(t *testing.T) {
	jail := newResolveJail(t)
	jail.start(t, "cc-100-1-1", "alpha-session", "alpha", "Alpha Label")
	jail.start(
		t,
		"cx-200-1-1",
		"cx-session",
		"abcdefghijklmnopqrstuvwx",
		"Codex Label",
	)
	jail.start(t, "cc-300-1-1", "other-session", "other", "Other Label")
	time.Sleep(100 * time.Millisecond)

	resolver, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	label, err := resolver.Resolve(ctx, Label, "alpha label")
	if err != nil {
		t.Fatal(err)
	}
	if label.Code != 0 {
		t.Logf("resolve diagnostics: %s", resolveDiagnostics(t, resolver))
	}
	assertTarget(t, label, "cc-100-1-1", "%")

	session, err := resolver.Resolve(ctx, Session, "alpha-session")
	if err != nil {
		t.Fatal(err)
	}
	assertTarget(t, session, "cc-100-1-1", "%")

	cx, err := resolver.Resolve(
		ctx,
		CxWindow,
		"abcdefghijklmnopqrstuvwxyz-more",
	)
	if err != nil {
		t.Fatal(err)
	}
	assertTarget(t, cx, "cx-200-1-1", "%")

	miss, err := resolver.Resolve(ctx, Label, "Alpha")
	if err != nil || miss.Code != 1 || miss.Stdout != "" || miss.Stderr != "" {
		t.Fatalf("miss = %#v, err=%v", miss, err)
	}

	jail.start(t, "cc-400-1-1", "ambiguous", "ambiguous", "Alpha Label")
	time.Sleep(50 * time.Millisecond)
	ambiguous, err := resolver.Resolve(ctx, Label, "Alpha Label")
	if err != nil || ambiguous.Code != 2 ||
		ambiguous.Stdout != "" ||
		!strings.Contains(ambiguous.Stderr, "ambiguous") {
		t.Fatalf("ambiguous = %#v, err=%v", ambiguous, err)
	}
}

func TestJailedTmuxNewestServerTieBreak(t *testing.T) {
	jail := newResolveJail(t)
	const uuid = "77777777-7777-4777-8777-777777777777"
	jail.start(t, "cc-100-1-1", "old", "old", "One Chat")
	jail.start(t, "cc-900-1-1", "new", "new", "One Chat")
	for _, socket := range []string{"cc-100-1-1", "cc-900-1-1"} {
		if err := os.WriteFile(
			filepath.Join(jail.sidDir, socket),
			[]byte(filepath.Join(jail.root, uuid+".jsonl")),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	resolver, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := resolver.Resolve(context.Background(), Label, "one chat")
	if err != nil || outcome.Code != 0 {
		t.Logf("resolve diagnostics: %s", resolveDiagnostics(t, resolver))
		t.Fatalf("tie outcome = %#v, err=%v", outcome, err)
	}
	if !strings.Contains(outcome.Stdout, "cc-900-1-1\t%") ||
		!strings.Contains(outcome.Stderr, "newest") ||
		!strings.Contains(outcome.Stderr, uuid) {
		t.Fatalf("tie outcome = %#v", outcome)
	}
}

func resolveDiagnostics(t *testing.T, resolver *Resolver) string {
	t.Helper()
	panes, err := resolver.allPanes(context.Background())
	if err != nil {
		return err.Error()
	}
	var output strings.Builder
	for _, pane := range panes {
		capture, captureErr := resolver.tmux.CapturePane(
			context.Background(),
			pane.SocketPath,
			pane.PaneID,
		)
		fmt.Fprintf(
			&output,
			"\n%+v capture=%q err=%v",
			pane,
			capture,
			captureErr,
		)
	}
	return output.String()
}

func TestStressResolveSixtySocketsTwoHundredPanes(t *testing.T) {
	strict := os.Getenv("PFM_STRESS_STRICT") == "1"
	jail := newResolveJail(t)
	const socketCount = 60
	const paneCount = 200
	createdPanes := 0
	for index := 0; index < socketCount; index++ {
		prefix := "cc"
		if index%3 == 0 {
			prefix = "cx"
		}
		socket := fmt.Sprintf("%s-%d-1-1", prefix, 1000+index)
		session := fmt.Sprintf("stress-%02d", index)
		window := fmt.Sprintf("window-%02d", index)
		label := fmt.Sprintf("Fleet %03d", createdPanes)
		if index == socketCount-1 {
			label = "Stress Target"
		}
		if index == 57 {
			window = "abcdefghijklmnopqrstuvwx"
		}
		jail.start(t, socket, session, window, label)
		createdPanes++
		wanted := 3
		if index < 20 {
			wanted = 4
		}
		switch index {
		case 57, 59:
			wanted = 1
		case 58:
			wanted = 7
		}
		for pane := 1; pane < wanted; pane++ {
			jail.split(
				t,
				socket,
				session,
				fmt.Sprintf("Fleet %03d", createdPanes),
			)
			createdPanes++
		}
	}
	if createdPanes != paneCount {
		t.Fatalf("created %d panes, want %d", createdPanes, paneCount)
	}
	time.Sleep(200 * time.Millisecond)
	resolver, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	queries := []struct {
		kind Kind
		name string
	}{
		{kind: Label, name: "Stress Target"},
		{kind: Session, name: "stress-59"},
		{kind: CxWindow, name: "abcdefghijklmnopqrstuvwxyz-long"},
	}
	for _, query := range queries {
		started := time.Now()
		outcome, err := resolver.Resolve(ctx, query.kind, query.name)
		elapsed := time.Since(started)
		if err != nil || outcome.Code != 0 {
			t.Fatalf(
				"%s stress outcome=%#v err=%v elapsed=%s",
				query.kind,
				outcome,
				err,
				elapsed,
			)
		}
		limit := 2 * time.Minute
		if strict {
			limit = 15 * time.Second
		}
		if elapsed > limit {
			t.Fatalf(
				"%s resolve took %s, want under %s (strict=%t)",
				query.kind,
				elapsed,
				limit,
				strict,
			)
		}
		t.Logf(
			"STRESS resolve kind=%s sockets=%d panes=%d elapsed=%s strict=%t",
			query.kind,
			socketCount,
			paneCount,
			elapsed,
			strict,
		)
	}

	for _, query := range []string{
		"",
		strings.Repeat("x", 500),
		"🧭🚀",
		"Stress",
	} {
		outcome, err := resolver.Resolve(ctx, Label, query)
		if err != nil || outcome.Code != 1 || outcome.Stdout != "" {
			t.Fatalf("adversarial %q = %#v, err=%v", query, outcome, err)
		}
	}
	t.Log("STRESS adversarial=4 false_matches=0")
}

func assertTarget(
	t *testing.T,
	outcome Outcome,
	socket, targetPrefix string,
) {
	t.Helper()
	if outcome.Code != 0 || outcome.Stderr != "" {
		t.Fatalf("outcome = %#v", outcome)
	}
	fields := strings.Split(strings.TrimSuffix(outcome.Stdout, "\n"), "\t")
	if len(fields) != 2 ||
		filepath.Base(fields[0]) != socket ||
		!strings.HasPrefix(fields[1], targetPrefix) {
		t.Fatalf("target = %#v", outcome)
	}
}

// TestPaneOwnersSurvivesAStrippedEnvironment pins the delimiter choice in the
// pane format.
//
// tmux hands a control character in a format string back as "_" unless the
// CALLER's environment either carries a UTF-8 locale or merely DEFINES $TMUX —
// an empty $TMUX is enough. Both hold for a normal fleet call and neither holds
// in the tool shell a chat engine spawns from a scrubbed environment, which is
// the one place ancestry recovery has to work: there a tab made every row
// unsplittable, recovery answered "not in tmux", and codex-origin messages went
// out UNSIGNED.
//
// The second half runs the tab against an environment stripped of both, so the
// trap is proven live and a rewrite back to a tab cannot pass quietly.
func TestPaneOwnersSurvivesAStrippedEnvironment(t *testing.T) {
	jail := newResolveJail(t)
	jail.start(t, "cc-500-1-1", "locale-session", "locale", "Locale Label")

	// Cleared only AFTER the session is up: the readiness probe reads an emoji
	// label back off the pane, which a stripped client would mangle too.
	t.Setenv("LANG", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_CTYPE", "")

	socketPath := filepath.Join(jail.tmuxDir, "cc-500-1-1")
	owners, err := CommandPaneOwners{TmuxDir: jail.tmuxDir}.PaneOwners(
		context.Background(),
		socketPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(owners) == 0 {
		t.Fatal("PaneOwners() returned no rows for a live server")
	}
	for _, owner := range owners {
		if owner.PanePID <= 0 || !strings.HasPrefix(owner.PaneID, "%") {
			t.Fatalf("PaneOwners() row = %#v", owner)
		}
	}

	stripped := exec.Command(
		"tmux",
		"-S",
		socketPath,
		"list-panes",
		"-a",
		"-F",
		"#{pane_pid}\t#{pane_id}",
	)
	stripped.Env = []string{
		"HOME=" + jail.home,
		"PATH=" + os.Getenv("PATH"),
	}
	tabbed, err := stripped.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(tabbed), "\t") {
		t.Fatalf(
			"a tab survived a stripped caller (%q) — this test no longer guards anything",
			string(tabbed),
		)
	}
}
