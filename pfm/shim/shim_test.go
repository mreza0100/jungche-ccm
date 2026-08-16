package shim

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShimSyntaxAndEvalProtocol(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	shimPath := bundleShimPath(t)
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
	shimPath := bundleShimPath(t)

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
esac
`)
	// The launched engine records its argv and the environment it was born with.
	writeShimFile(t, filepath.Join(fakeBin, "claude"), `#!/bin/sh
{
  for argument in "$@"; do printf 'arg=%s\n' "$argument"; done
  for name in CLAUDE_CONFIG_DIR ANTHROPIC_BASE_URL ANTHROPIC_AUTH_TOKEN \
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

func bundleShimPath(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Clean(filepath.Join(
		cwd,
		"..",
		"..",
		"blueprint",
		"templates",
		"host-swap",
		"shim",
		"pfm.zsh",
	))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("locate bundle shim %s: %v", path, err)
	}
	return path
}

func quoteZsh(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
