package shim

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"hostops/pfm/internal/testjail"
)

func TestShimSyntaxAndEvalProtocol(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	shimPath := embeddedShimPath(t)
	if output, err := exec.Command(zsh, "-n", shimPath).CombinedOutput(); err != nil {
		t.Fatalf("zsh -n: %v: %s", err, output)
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	fake := `#!/bin/sh
case "$1:$2" in
  open:good) printf '%s\n' 'print -r -- evaluated' ;;
  open:bad) printf '%s\n' 'print -r -- must-not-eval'; exit 7 ;;
  ls:--tsv) printf '%s\n' 'literal tsv output' ;;
  *) exit 0 ;;
esac
`
	if err := os.WriteFile(
		filepath.Join(binDir, "pfm"),
		[]byte(fake),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	script := "source " + quoteZsh(shimPath) + `
cc-open good
cc-ls --tsv
cc-open bad
print -r -- "rc=$?"
`
	command := exec.Command(zsh, "-c", script)
	command.Env = append(os.Environ(), "HOME="+home)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run shim protocol: %v: %s", err, output)
	}
	got := string(output)
	for _, wanted := range []string{
		"evaluated\n",
		"literal tsv output\n",
		"rc=7\n",
	} {
		if !strings.Contains(got, wanted) {
			t.Fatalf("shim output %q does not contain %q", got, wanted)
		}
	}
	if strings.Contains(got, "must-not-eval") {
		t.Fatalf("shim evaluated output from failed binary: %q", got)
	}
}

// TestShimLaunchPosture drives the shell surface the Go action protocol emits
// into, and fixtures what a launched chat actually receives: the autonomy flags
// in argv, an endpoint-free environment, and the account the fleet's own state
// store names.
func TestShimLaunchPosture(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	shimPath := embeddedShimPath(t)

	home := t.TempDir()
	fakeBin := filepath.Join(home, "fake-bin")
	for _, directory := range []string{
		filepath.Join(home, ".local", "bin"),
		fakeBin,
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeShimFile(t, filepath.Join(home, ".local", "bin", "pfm"), `#!/bin/sh
case "$1 $2" in
  "internal primary-get") cat "$SHIM_DB_STATE" 2>/dev/null || printf '1\n' ;;
  "internal primary-set")
    case "$3" in 1|2) ;; *) printf 'account must be 1-2\n' >&2; exit 2 ;; esac
    printf '%s\n' "$3" > "$SHIM_DB_STATE"
    printf 'primary-set %s\n' "$3" >> "$SHIM_DB_LOG" ;;
  "autonomy ") printf 'on\n' ;;
esac
`)
	// The launched engine records its argv and the environment it was born with.
	writeShimFile(t, filepath.Join(fakeBin, "claude"), `#!/bin/sh
{
  for argument in "$@"; do printf 'arg=%s\n' "$argument"; done
  for name in CLAUDE_CONFIG_DIR CLAUDE_CODE_SESSION_ID CLAUDECODE \
    CLAUDE_CODE_CHILD_SESSION ANTHROPIC_BASE_URL ANTHROPIC_AUTH_TOKEN \
    ANTHROPIC_MODEL ANTHROPIC_SMALL_FAST_MODEL CLAUDE_CODE_AUTO_COMPACT_WINDOW \
    CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC \
    CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK \
    CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY; do
    eval "value=\${$name-unset}"
    printf 'env %s=%s\n' "$name" "$value"
  done
} > "$SHIM_CLAUDE_RESULT"
`)
	writeShimFile(t, filepath.Join(fakeBin, "tmux"), `#!/bin/sh
for argument in "$@"; do printf '%s\n' "$argument"; done > "$SHIM_TMUX_RESULT"
`)
	claudeResult := filepath.Join(home, "claude-result")
	tmuxResult := filepath.Join(home, "tmux-result")
	dbState := filepath.Join(home, "db-state")
	dbLog := filepath.Join(home, "db-log")
	writeShimFile(t, dbState, "2\n")

	script := "source " + quoteZsh(shimPath) + `
_cc_run 2 0 --model 'x y' --verbose
cc --model z
cc-swap 1
print -r -- "cc3=$(typeset -f cc3 > /dev/null 2>&1 && print yes || print no)"
print -r -- "primary=$(_cc_primary)"
`
	command := exec.Command(zsh, "-c", script)
	command.Env = append(
		os.Environ(),
		"HOME="+home,
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"TMUX=",
		"SHIM_CLAUDE_RESULT="+claudeResult,
		"SHIM_TMUX_RESULT="+tmuxResult,
		"SHIM_DB_STATE="+dbState,
		"SHIM_DB_LOG="+dbLog,
		// A chat launched from inside another chat inherits all of these.
		// CLAUDE_CODE_CHILD_SESSION is the quiet one: left in place it marks the
		// newborn a subordinate and silently disables transcript saving.
		"CLAUDE_CODE_SESSION_ID=parent-session",
		"CLAUDECODE=1",
		"CLAUDE_CODE_CHILD_SESSION=1",
		"ANTHROPIC_BASE_URL=http://127.0.0.1:9/proxy",
		"ANTHROPIC_AUTH_TOKEN=poison",
		"ANTHROPIC_MODEL=poison",
		"ANTHROPIC_SMALL_FAST_MODEL=poison",
		"CLAUDE_CODE_AUTO_COMPACT_WINDOW=poison",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
		"CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK=1",
		"CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run shim launch posture: %v: %s", err, output)
	}

	// Bare launch: both autonomy flags lead argv, and each caller argument
	// survives as ONE word. Joining the flags into a single argv element is what
	// made claude reject an unknown option and the chat die at birth.
	launched := readShimFile(t, claudeResult)
	wantArgv := "arg=--allow-dangerously-skip-permissions\n" +
		"arg=--dangerously-skip-permissions\n" +
		"arg=--model\narg=x y\narg=--verbose\n"
	if !strings.HasPrefix(launched, wantArgv) {
		t.Fatalf("launched argv = %q, want prefix %q", launched, wantArgv)
	}
	if !strings.Contains(launched, "env CLAUDE_CONFIG_DIR="+home+"/.cc/2\n") {
		t.Fatalf("launched without account 2's config dir: %q", launched)
	}
	for _, name := range []string{
		"CLAUDE_CODE_SESSION_ID",
		"CLAUDECODE",
		"CLAUDE_CODE_CHILD_SESSION",
		"ANTHROPIC_BASE_URL",
		"ANTHROPIC_AUTH_TOKEN",
		"ANTHROPIC_MODEL",
		"ANTHROPIC_SMALL_FAST_MODEL",
		"CLAUDE_CODE_AUTO_COMPACT_WINDOW",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC",
		"CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK",
		"CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY",
	} {
		if !strings.Contains(launched, "env "+name+"=unset\n") {
			t.Fatalf("launched still carrying %s: %q", name, launched)
		}
	}

	// tmux launch: the run string strips the same endpoint variables, carries
	// the autonomy flags, and lands on the account the state store names.
	runString := readShimFile(t, tmuxResult)
	for _, wanted := range []string{
		"-u CLAUDE_CODE_SESSION_ID",
		"-u CLAUDECODE",
		"-u CLAUDE_CODE_CHILD_SESSION",
		"-u ANTHROPIC_BASE_URL",
		"-u CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY",
		"claude --allow-dangerously-skip-permissions --dangerously-skip-permissions --model z",
		"CLAUDE_CONFIG_DIR=" + home + "/.cc/2",
	} {
		if !strings.Contains(runString, wanted) {
			t.Fatalf("tmux run string %q lacks %q", runString, wanted)
		}
	}

	// The pfm store validates and records the primary account; the shim never
	// writes ~/.claude-primary itself.
	if log := readShimFile(t, dbLog); log != "primary-set 1\n" {
		t.Fatalf("pfm primary-set log = %q", log)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude-primary")); !os.IsNotExist(err) {
		t.Fatalf("shim wrote ~/.claude-primary behind pfm: %v", err)
	}
	got := string(output)
	// The roster is two accounts, so there is no cc3 launcher to reach a third.
	if !strings.Contains(got, "cc3=no\n") ||
		!strings.Contains(got, "primary=1\n") {
		t.Fatalf("shim roster output = %q", got)
	}
}

// A fleet chat owns a bare terminal. Replacing that shell with the tmux
// client makes terminal teardown structural: when the harness closes the
// server, the client exits and there is no parent shell left to reveal.
func TestBareFleetLaunchExecsTheTerminalOwner(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script is not installed")
	}
	home := t.TempDir()
	fakeBin := filepath.Join(home, "fake-bin")
	if err := os.MkdirAll(filepath.Join(home, ".local", "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fakeBin, 0o700); err != nil {
		t.Fatal(err)
	}
	writeShimFile(t, filepath.Join(home, ".local", "bin", "pfm"), "#!/bin/sh\nexit 0\n")
	writeShimFile(t, filepath.Join(fakeBin, "tmux"), "#!/bin/sh\nprintf '%s\\n' \"$$\" > \"$SHIM_TMUX_PID\"\n")
	shellPID := filepath.Join(home, "shell.pid")
	tmuxPID := filepath.Join(home, "tmux.pid")
	driver := filepath.Join(home, "driver.zsh")
	writeShimFile(t, driver,
		"source "+quoteZsh(embeddedShimPath(t))+"\n"+
			"print -r -- \"$$\" > \"$SHIM_SHELL_PID\"\n"+
			"cc\n",
	)
	command := testjail.PTYCommand(zsh, "-fi", driver)
	command.Env = append(
		os.Environ(),
		"HOME="+home,
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"TMUX=",
		"SHIM_SHELL_PID="+shellPID,
		"SHIM_TMUX_PID="+tmuxPID,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("drive interactive shim: %v: %s", err, output)
	}
	want := strings.TrimSpace(readShimFile(t, shellPID))
	got := strings.TrimSpace(readShimFile(t, tmuxPID))
	if got != want {
		t.Fatalf("tmux pid=%s, shell pid=%s: bare launch forked and left an outer terminal shell", got, want)
	}
}

// TestShimAutoOpenDefersDisarmsAndWhitelists drives the auto-open hook the way a
// terminal profile does: an environment variable set on a fresh interactive
// shell. The hook exists because the source line lands at the BOTTOM of
// ~/.zshrc, so a profile that called `cc` from ~/.zshrc itself reached
// /usr/bin/cc — the C compiler — and every new terminal opened on "clang: error:
// no input files" instead of a chat. Owning the hook here is what makes the
// ordering unbreakable, so these are the properties that must hold.
func TestShimAutoOpenDefersDisarmsAndWhitelists(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	shimPath := embeddedShimPath(t)

	home := t.TempDir()
	fakeBin := filepath.Join(home, "fake-bin")
	for _, directory := range []string{filepath.Join(home, ".local", "bin"), fakeBin} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeShimFile(t, filepath.Join(home, ".local", "bin", "pfm"), `#!/bin/sh
case "$1" in
  internal) printf '1\n' ;;
  ls) printf 'picker\n' >> "$SHIM_AUTO_LOG" ;;
esac
`)
	// The launch path ends in tmux; recording its argv proves WHICH account the
	// whitelist picked, not merely that something launched.
	writeShimFile(t, filepath.Join(fakeBin, "tmux"), `#!/bin/sh
printf 'launch %s\n' "$*" >> "$SHIM_AUTO_LOG"
`)
	// A value that is neither whitelisted nor a command must reach NOTHING. This
	// records the day `eval` or a bare "$_cc_auto_what" creeps back in.
	writeShimFile(t, filepath.Join(fakeBin, "rm"), `#!/bin/sh
printf 'EVALUATED %s\n' "$*" >> "$SHIM_AUTO_LOG"
`)

	cases := []struct {
		name    string
		environ []string
		want    []string
		absent  []string
	}{{
		name:    "bare truthy value opens the picker",
		environ: []string{"CC_AUTO_OPEN=1"},
		want:    []string{"picker"},
	}, {
		name:    "cc opens a fresh chat on the primary account",
		environ: []string{"CC_AUTO_OPEN=cc"},
		want:    []string{"launch ", "-u CLAUDE_CONFIG_DIR"},
	}, {
		name:    "cc2 opens a fresh chat on account 2",
		environ: []string{"CC_AUTO_OPEN=cc2"},
		want:    []string{"launch ", "CLAUDE_CONFIG_DIR=" + home + "/.cc/2"},
	}, {
		name:    "the retired spelling still works",
		environ: []string{"VSCODE_AUTO_CC=1"},
		want:    []string{"picker"},
	}, {
		name:    "an unlisted value falls to the picker and is never run",
		environ: []string{"CC_AUTO_OPEN=rm -rf /"},
		want:    []string{"picker"},
		absent:  []string{"EVALUATED"},
	}, {
		name:    "an unset variable opens nothing",
		environ: nil,
		absent:  []string{"picker", "launch "},
	}, {
		name:    "a shell inside a chat opens nothing",
		environ: []string{"CC_AUTO_OPEN=1", "CLAUDECODE=1"},
		absent:  []string{"picker", "launch "},
	}}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			log := filepath.Join(t.TempDir(), "log")
			writeShimFile(t, log, "")
			// Three prompts, so a hook that failed to disarm would fire again:
			// the command RETURNS when the chat exits, and a still-registered
			// hook is a terminal that reopens whatever was just closed, forever.
			got := runAutoOpenShell(t, zsh, shimPath, home, fakeBin, log, testCase.environ,
				"print -r -- one\nprint -r -- two\nprint -r -- three\n")
			for _, wanted := range testCase.want {
				if !strings.Contains(got, wanted) {
					t.Fatalf("auto-open log %q lacks %q", got, wanted)
				}
			}
			for _, unwanted := range testCase.absent {
				if strings.Contains(got, unwanted) {
					t.Fatalf("auto-open log %q contains %q", got, unwanted)
				}
			}
			if lines := strings.Count(got, "\n"); lines > 1 {
				t.Fatalf("auto-open fired %d times, want at most once: %q", lines, got)
			}
		})
	}

	// Both spellings are exported by the terminal profile, so they are INHERITED.
	// Unsetting them before arming is what stops a shell opened inside the chat
	// from opening a chat inside the chat.
	writeShimFile(t, filepath.Join(home, "inherit-log"), "")
	inherited := runAutoOpenShell(t, zsh, shimPath, home, fakeBin,
		filepath.Join(home, "inherit-log"),
		[]string{"CC_AUTO_OPEN=1", "VSCODE_AUTO_CC=1"},
		`print -r -- "leaked=${CC_AUTO_OPEN-no}${VSCODE_AUTO_CC-no}" >> "$SHIM_AUTO_LOG"`+"\n")
	if !strings.Contains(inherited, "leaked=nono") {
		t.Fatalf("auto-open left its variable in the environment: %q", inherited)
	}
}

// runAutoOpenShell sources the shim in a REAL interactive zsh — the hook is a
// precmd, so only a shell that actually draws prompts exercises it — and returns
// what the fake launchers recorded.
func runAutoOpenShell(
	t *testing.T,
	zsh, shimPath, home, fakeBin, log string,
	environ []string,
	body string,
) string {
	t.Helper()
	command := exec.Command(zsh, "-f", "-i", "+m")
	command.Stdin = strings.NewReader("source " + quoteZsh(shimPath) + "\n" + body)
	command.Env = append([]string{
		"HOME=" + home,
		"PATH=" + fakeBin + ":/usr/bin:/bin",
		"TERM=dumb",
		"SHIM_AUTO_LOG=" + log,
	}, environ...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run auto-open shell: %v: %s", err, output)
	}
	return readShimFile(t, log)
}

func writeShimFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func readShimFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func embeddedShimPath(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Clean(filepath.Join(
		cwd,
		"..",
		"internal",
		"installer",
		"assets",
		"shim",
		"pfm.zsh",
	))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("locate embedded installer shim source %s: %v", path, err)
	}
	return path
}

func quoteZsh(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
