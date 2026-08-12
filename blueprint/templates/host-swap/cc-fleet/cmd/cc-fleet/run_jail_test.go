package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubCodex is a Codex TUI reduced to the four states cc-fleet's rename
// choreography navigates, driven over a REAL tty inside a REAL tmux server:
// canonical mode is turned off so a partial "/rename" is seen before Enter
// (exactly how the engine's own slash-command popup behaves), and every state
// repaints the screen so a marker that should be gone really leaves the
// capture.
const stubCodex = `#!/usr/bin/env bash
stty -icanon -echo min 1 time 0 2>/dev/null
stage=composer; buf=""; name=""
status='  019f · ~/work · Full Access · Context 0% used · 0 in · 0 out
'
modals="${CX_STUB_MODALS:-1}"
render() {
  printf '\033[2J\033[H'
  if [ "$modals" -gt 0 ]; then
    # The selection cursor is the same glyph the composer draws, and the
    # status line is gone: the exact screen that fooled the first fix.
    printf 'codex\n  Hooks\n  1 hook needs review before it can run.\n'
    printf '\u203a 2. Trust all and continue\n'
    printf '  Press enter to confirm or esc to go back\n'
    return
  fi
  case "$stage" in
    offered) printf 'codex\n› %s\n  /rename  rename the current thread\n%s' "$buf" "$status" ;;
    prompt)  printf 'codex\n| Name thread\n| Type a name and press Enter\n' ;;
    *)       if [ -n "$name" ]; then printf '* Session renamed to %s.\n' "$name"; fi
             printf 'codex\n› %s\n%s' "$buf" "$status" ;;
  esac
}
render
while IFS= read -r -N1 ch; do
  if [ "$modals" -gt 0 ]; then
    # A startup overlay: ONLY Escape gets out of it, everything else vanishes
    # into it exactly as the real hooks/trust modals swallow keystrokes.
    [ "$ch" = $'\033' ] && modals=$((modals - 1))
    render
    continue
  fi
  case "$ch" in
    $'\n'|$'\r')
      case "$stage" in
        offered) stage=prompt; buf="" ;;
        prompt)  name="$buf"; printf '%s' "$buf" > "$CX_STUB_NAME"; stage=composer; buf="" ;;
        *)       printf '%s\n' "$buf" >> "$CX_STUB_PROMPT"; buf="" ;;
      esac ;;
    $'\177'|$'\b')
      buf="${buf%?}" ;;
    *)
      buf="$buf$ch"
      if [ "$stage" = composer ] && [ "$buf" = "/rename" ] &&
         [ -z "$CX_STUB_NO_RENAME" ]; then stage=offered; fi ;;
  esac
  render
done
`

// stubClaude records the argv it was launched with and then holds the pane
// open, which is all the Claude route needs: its name travels on the command
// line, so nothing is ever typed into it.
const stubClaude = `#!/usr/bin/env bash
printf '%s\n' "$*" > "$CC_STUB_ARGV"
printf 'claude ready\n'
while IFS= read -r _; do :; done
`

type runJail struct {
	root    string
	tmuxDir string
	binDir  string
}

func newRunJail(t *testing.T) *runJail {
	t.Helper()
	// /tmp, not t.TempDir(): a tmux socket path must stay inside the ~100
	// byte sun_path limit, which the test-name-derived TempDir blows past.
	root, err := os.MkdirTemp("/tmp", "ccfrun")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	jail := &runJail{
		root:    root,
		tmuxDir: filepath.Join(root, "tmux"),
		binDir:  filepath.Join(root, "bin"),
	}
	for _, directory := range []string{
		jail.tmuxDir,
		jail.binDir,
		filepath.Join(root, "home"),
		filepath.Join(root, "claude"),
		filepath.Join(root, "codex"),
		filepath.Join(root, "sid"),
		filepath.Join(root, "work"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(name, body string) {
		if err := os.WriteFile(
			filepath.Join(jail.binDir, name),
			[]byte(body),
			0o700,
		); err != nil {
			t.Fatal(err)
		}
	}
	write("codex", stubCodex)
	write("claude", stubClaude)

	// The engine stubs go on PATH BEFORE anything drives the CLI, and every
	// self-identifying variable is cleared: a spawn made from inside this
	// chat must not inherit its tmux server, pane, or chat identity.
	t.Setenv("PATH", jail.binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("TMUX_TMPDIR", root)
	t.Setenv("CC_FLEET_HOME", filepath.Join(root, "home"))
	t.Setenv("CC_FLEET_DB", filepath.Join(root, "fleet.db"))
	t.Setenv("CC_FLEET_SID_DIR", filepath.Join(root, "sid"))
	t.Setenv("CC_FLEET_CLAUDE_ROOTS", filepath.Join(root, "claude"))
	t.Setenv("CC_FLEET_CODEX_ROOT", filepath.Join(root, "codex"))
	t.Setenv("CC_FLEET_TMUX_DIR", jail.tmuxDir)
	t.Setenv("CC_FLEET_PROC_ROOT", filepath.Join(root, "proc"))
	t.Setenv("CX_STUB_NAME", filepath.Join(root, "cx-name"))
	t.Setenv("CX_STUB_PROMPT", filepath.Join(root, "cx-prompt"))
	t.Setenv("CC_STUB_ARGV", filepath.Join(root, "cc-argv"))
	return jail
}

// killSockets takes down every server this test started, so a jail never
// leaves a tmux server behind holding the temp directory open.
func (jail *runJail) killSockets(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(jail.tmuxDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		_ = exec.Command(
			"tmux",
			"-S", filepath.Join(jail.tmuxDir, entry.Name()),
			"kill-server",
		).Run()
	}
}

func (jail *runJail) read(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(jail.root, name))
	if err != nil {
		return ""
	}
	return string(content)
}

// await polls for engine-side state. `run` returns the moment the keystrokes
// are delivered — the engine writes its file a beat later — so a bare read
// races the pane and would flake.
func (jail *runJail) await(t *testing.T, name, want string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	got := ""
	for time.Now().Before(deadline) {
		got = jail.read(t, name)
		if strings.Contains(got, want) {
			return got
		}
		time.Sleep(50 * time.Millisecond)
	}
	return got
}

// TestRunSpawnsANamedHeadlessCodexChat drives the CLI against a real tmux
// server and a stub engine: the session is created detached, the thread is
// renamed through the engine's own UI, and only then is the first prompt
// delivered.
func TestRunSpawnsANamedHeadlessCodexChat(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	jail := newRunJail(t)
	defer jail.killSockets(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"run",
		"--engine", "codex",
		"--name", "_HIDE codex worker",
		"--cwd", filepath.Join(jail.root, "work"),
		"read the incident report",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got := jail.await(t, "cx-name", "_HIDE"); got != "_HIDE codex worker" {
		t.Fatalf("thread name = %q, want %q (stderr=%q)",
			got, "_HIDE codex worker", stderr.String())
	}
	got := jail.await(t, "cx-prompt", "incident")
	if strings.TrimSpace(got) != "read the incident report" {
		t.Fatalf("delivered prompt = %q", got)
	}
	report := stdout.String()
	if !strings.Contains(report, "\tnamed\t") ||
		!strings.Contains(report, "hidden by its _HIDE name") {
		t.Fatalf("run report = %q", report)
	}
	if !strings.Contains(report, "attach: tmux -L cx-") {
		t.Fatalf("run report has no attach line: %q", report)
	}

	entries, err := os.ReadDir(jail.tmuxDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("jailed tmux sockets = %v (err=%v)", entries, err)
	}
	if !strings.HasPrefix(entries[0].Name(), "cx-") {
		t.Fatalf("codex chat born on socket %q", entries[0].Name())
	}
}

// TestRunReportsACodexBuildThatCannotBeRenamed is the version-drift drill: a
// Codex that does not offer /rename must leave a WORKING chat that says so —
// non-zero exit, UNNAMED in the report — and the abandoned command must never
// reach the model as a prompt.
func TestRunReportsACodexBuildThatCannotBeRenamed(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	jail := newRunJail(t)
	defer jail.killSockets(t)
	t.Setenv("CX_STUB_NO_RENAME", "1")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"run",
		"--engine", "cx",
		"--name", "worker",
		"--cwd", filepath.Join(jail.root, "work"),
		"read the incident report",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run exit=%d, want 1 (stdout=%q)", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "\tUNNAMED\t") {
		t.Fatalf("run report = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "did not offer /rename") {
		t.Fatalf("run warnings = %q", stderr.String())
	}
	if got := jail.read(t, "cx-name"); got != "" {
		t.Fatalf("a thread name landed anyway: %q", got)
	}
	prompts := jail.await(t, "cx-prompt", "incident")
	if strings.TrimSpace(prompts) != "read the incident report" {
		t.Fatalf("delivered prompts = %q — the work must still be delivered, "+
			"and the abandoned /rename must not be", prompts)
	}
}

// TestRunSpawnsAHeadlessClaudeChatWithItsNameOnTheCommandLine proves the other
// route: Claude is named by its own flag, carries the autonomy flags, and is
// never typed into.
func TestRunSpawnsAHeadlessClaudeChatWithItsNameOnTheCommandLine(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	jail := newRunJail(t)
	defer jail.killSockets(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"run",
		"--name", "worker 7",
		"--cwd", filepath.Join(jail.root, "work"),
		"audit the firewall",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	argv := jail.await(t, "cc-argv", "--name")
	for _, want := range []string{
		"--name worker 7",
		"audit the firewall",
		"--allow-dangerously-skip-permissions",
		"--dangerously-skip-permissions",
	} {
		if !strings.Contains(argv, want) {
			t.Fatalf("claude argv = %q, want it to contain %q", argv, want)
		}
	}
	if jail.read(t, "cx-name") != "" {
		t.Fatal("the Claude route drove the Codex rename UI")
	}
	report := stdout.String()
	if !strings.Contains(report, "cc\tworker 7\t") ||
		!strings.Contains(report, "\tlisted\n") {
		t.Fatalf("run report = %q", report)
	}
}
