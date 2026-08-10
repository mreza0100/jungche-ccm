package check

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"hostops/cc-fleet/internal/paths"
)

// legacyFixtureScript mirrors the live picker's two load-bearing shapes: it
// self-locates its bundle directory and sources cc-portable.sh from it, and it
// prints its rows through `cut -f7` when fzf is missing.
const legacyFixtureScript = `_ccfs="${(%):-%N}"
CC_FLEET_HOME="${CC_FLEET_HOME:-${_ccfs:h:A}}"
[[ -r "$CC_FLEET_HOME/cc-portable.sh" ]] && source "$CC_FLEET_HOME/cc-portable.sh"
cc-ls() {
  local -a rows=()
  rows+=($'project\t'"$(cc_fixture_prompts)"$'\t0\tR\tchat-7\t/work/project\t\e[2m↻\e[0m project        │ name                           │     7p │   12B │ now')
  if ! command -v fzf >/dev/null; then
    printf '%s\n' "${rows[@]}" | sort -t$'\t' -k1,1 -k3,3nr | cut -f7
    echo "cc-ls: fzf not found"
    return 1
  fi
  return 99
}
`

const legacyFixturePortable = "cc_fixture_prompts() { printf '%s' 7; }\n"

func writeLegacyFixture(t *testing.T, bundle string) string {
	t.Helper()
	if err := os.MkdirAll(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(bundle, LegacyScriptName)
	if err := os.WriteFile(
		scriptPath,
		[]byte(legacyFixtureScript),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(bundle, legacyPortableName),
		[]byte(legacyFixturePortable),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	return scriptPath
}

func TestLegacyRunnerMasksFZFAndInstrumentsFallback(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is not installed")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	claude := filepath.Join(root, "claude")
	codex := filepath.Join(root, "codex")
	sid := filepath.Join(root, "sid")
	tmuxDir := filepath.Join(root, "tmux", "tmux-1000")
	for _, directory := range []string{
		filepath.Join(home, ".claude"),
		claude,
		codex,
		sid,
		tmuxDir,
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	scriptPath := writeLegacyFixture(t, filepath.Join(root, "bundle"))
	t.Setenv(LegacyOutputEnv, "")
	t.Setenv(LegacyScriptEnv, scriptPath)
	output, err := runLegacy(context.Background(), paths.Values{
		Home:          home,
		ClaudeRoots:   []string{claude},
		CodexRoot:     codex,
		SIDDir:        sid,
		TmuxDir:       tmuxDir,
		SharedDB:      filepath.Join(root, "absent-fleet.db"),
		HiddenCarrier: filepath.Join(home, ".claude", ".cc-ls-hidden"),
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ParseLegacy(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("legacy runner returned %d rows: %q", len(rows), output)
	}
	// The prompt count comes from the SIBLING library. A shadow copy that lost
	// cc-portable.sh would still print a row, just a wrong one.
	if got := rows[0].Tuple; got.ID != "chat-7" ||
		got.Kind != "resume-claude" ||
		got.Project != "project" ||
		got.Prompts != 7 {
		t.Fatalf("legacy runner tuple = %#v", got)
	}
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != legacyFixtureScript {
		t.Fatal("legacy reference script was modified")
	}
}

func TestResolveLegacyScriptPrefersTheExplicitOverride(t *testing.T) {
	root := t.TempDir()
	scriptPath := writeLegacyFixture(t, filepath.Join(root, "bundle"))
	t.Setenv(LegacyScriptEnv, scriptPath)
	resolved, err := ResolveLegacyScript(filepath.Join(root, "home"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved != scriptPath {
		t.Fatalf("ResolveLegacyScript() = %q, want %q", resolved, scriptPath)
	}

	// An override that names nothing readable is an error, never a silent fall
	// through to a different script — the whole point of the variable is that
	// the operator chose which picker is the reference.
	t.Setenv(LegacyScriptEnv, filepath.Join(root, "absent", LegacyScriptName))
	if _, err := ResolveLegacyScript(filepath.Join(root, "home")); err == nil ||
		!strings.Contains(err.Error(), LegacyScriptEnv) {
		t.Fatalf("ResolveLegacyScript(missing override) error = %v", err)
	}
}

func TestResolveLegacyScriptFindsTheTreeSibling(t *testing.T) {
	t.Setenv(LegacyScriptEnv, "")
	resolved, err := ResolveLegacyScript("")
	if err != nil {
		t.Skipf("no live bundle beside this tree: %v", err)
	}
	sibling := filepath.Join(filepath.Dir(treeSourceDir()), LegacyScriptName)
	if resolved != sibling {
		t.Fatalf("ResolveLegacyScript() = %q, want the tree sibling %q",
			resolved, sibling)
	}
}

func TestResolveLegacyScriptFollowsTheInstalledLinkFarm(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "clone")
	scriptPath := writeLegacyFixture(t, bundle)
	home := filepath.Join(root, "home")
	binDir := filepath.Join(home, ".claude", "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// install.sh links the sourced library, never the picker itself, so the
	// picker has to be found as that link's sibling.
	if err := os.Symlink(
		filepath.Join(bundle, legacyPortableName),
		filepath.Join(binDir, legacyPortableName),
	); err != nil {
		t.Fatal(err)
	}
	t.Setenv(LegacyScriptEnv, "")
	if resolved := installedBundleScript(home); resolved != scriptPath {
		t.Fatalf("installed link farm resolved to %q, want %q",
			resolved, scriptPath)
	}
}

func TestResolveLegacyScriptReadsTheZshrcSourceLine(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "clone")
	scriptPath := writeLegacyFixture(t, bundle)
	zshrc := filepath.Join(root, ".zshrc")
	if err := os.WriteFile(zshrc, []byte(strings.Join([]string{
		"# a commented line must not win",
		`# source "/nowhere/cc-fleet.zsh"`,
		"export PATH=$PATH:/usr/local/bin",
		`[[ -r "` + scriptPath + `" ]] && source "` + scriptPath + `"`,
	}, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := zshrcLegacyScript(zshrc); got != scriptPath {
		t.Fatalf("zshrcLegacyScript() = %q, want %q", got, scriptPath)
	}
	if got := zshrcLegacyScript(filepath.Join(root, "absent")); got != "" {
		t.Fatalf("zshrcLegacyScript(absent) = %q, want empty", got)
	}
}

func TestCopyLegacySiblingsCarriesEverySelfLocatedFile(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "bundle")
	shadow := filepath.Join(root, "shadow")
	for _, directory := range []string{bundle, shadow} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{legacyPortableName, "cc-db.sh"} {
		if err := os.WriteFile(
			filepath.Join(bundle, name),
			[]byte("# "+name+"\n"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	script := `source "$CC_FLEET_HOME/cc-portable.sh"
typeset -g CC_DB="$CC_FLEET_HOME/cc-db.sh"
bash ${(q)CC_FLEET_HOME}/cc-absent.sh
`
	if err := copyLegacySiblings(script, bundle, shadow); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{legacyPortableName, "cc-db.sh"} {
		if _, err := os.Stat(filepath.Join(shadow, name)); err != nil {
			t.Fatalf("shadow is missing sibling %s: %v", name, err)
		}
	}
	// A named file the bundle does not contain is skipped, not fatal: the
	// picker guards each of those with its own readability test.
	if _, err := os.Stat(filepath.Join(shadow, "cc-absent.sh")); err == nil {
		t.Fatal("shadow invented a sibling the bundle does not have")
	}

	// A script that names no cc-portable.sh would run without the GNU/BSD seam
	// and list nothing at all — refuse rather than report a false parity.
	if err := copyLegacySiblings("cc-ls() { : }\n", bundle, shadow); err == nil {
		t.Fatal("copyLegacySiblings accepted a script with no portable seam")
	}
}

func TestShadowTmuxWrapperAllowsOnlyReadOnlyVerbs(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeWrappers(binDir); err != nil {
		t.Fatal(err)
	}
	// The wrapper delegates to a recorder instead of the real tmux, so the test
	// observes exactly which invocations would have reached the live servers.
	log := filepath.Join(root, "invocations")
	recorder := filepath.Join(root, "tmux-recorder")
	if err := os.WriteFile(recorder, []byte(
		"#!/bin/sh\nprintf '%s\\n' \"$*\" >> "+shellQuote(log)+"\n",
	), 0o700); err != nil {
		t.Fatal(err)
	}
	wrapper, err := os.ReadFile(filepath.Join(binDir, "tmux"))
	if err != nil {
		t.Fatal(err)
	}
	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is not installed")
	}
	patched := strings.ReplaceAll(
		string(wrapper),
		shellQuote(realTmux),
		shellQuote(recorder),
	)
	if patched == string(wrapper) {
		t.Fatal("tmux wrapper does not delegate to the resolved tmux binary")
	}
	probe := filepath.Join(root, "tmux-probe")
	if err := os.WriteFile(probe, []byte(patched), 0o700); err != nil {
		t.Fatal(err)
	}

	allowed := [][]string{
		{"-L", "cc-1-2-3", "list-panes", "-a", "-F", "#{session_name}"},
		{"-L", "cc-1-2-3", "list-sessions", "-F", "#{session_name}"},
		{"-L", "cc-1-2-3", "has-session", "-t", "=chat"},
		{"-L", "cc-1-2-3", "display-message", "-p", "#{pane_id}"},
	}
	blocked := [][]string{
		{"-L", "cc-1-2-3", "kill-server"},
		{"-L", "cc-1-2-3", "kill-session", "-t", "=chat"},
		{"-L", "cc-1-2-3", "new-session", "-s", "chat"},
		{"-L", "cc-1-2-3", "new-window", "-n", "chat"},
		{"-L", "cc-1-2-3", "rename-window", "-t", "%1", "chat"},
		{"-L", "cc-1-2-3", "select-window", "-t", "%1"},
		{"-L", "cc-1-2-3", "set-option", "-g", "status", "off"},
		{"-L", "cc-1-2-3", "attach-session", "-t", "=chat"},
		// Verbs the old blocklist never named. An allowlist has to stop these
		// without being taught about them one incident at a time.
		{"-L", "cc-1-2-3", "send-keys", "-t", "%1", "rm -rf /", "Enter"},
		{"-L", "cc-1-2-3", "kill-pane", "-t", "%1"},
		{"-L", "cc-1-2-3", "respawn-pane", "-k", "-t", "%1"},
		{"-L", "cc-1-2-3", "run-shell", "touch /tmp/pwned"},
		{"-L", "cc-1-2-3", "source-file", "/tmp/evil.conf"},
		{"-L", "cc-1-2-3", "swap-window", "-s", "1", "-t", "2"},
		{"-L", "cc-1-2-3", "unlink-window", "-t", "%1"},
		{"-L", "cc-1-2-3", "load-buffer", "/etc/passwd"},
		// A socket NAMED like a verb must not be read as one.
		{"-L", "kill-server", "list-panes", "-a"},
	}
	for _, arguments := range append(append([][]string(nil), allowed...), blocked...) {
		command := exec.Command(probe, arguments...)
		if err := command.Run(); err != nil {
			t.Fatalf("tmux wrapper %v exited %v", arguments, err)
		}
	}
	recorded, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("tmux wrapper reached the binary %d times: %v", 0, err)
	}
	lines := strings.Split(strings.TrimSuffix(string(recorded), "\n"), "\n")
	if len(lines) != len(allowed)+1 {
		t.Fatalf("tmux wrapper forwarded %d invocations, want %d: %q",
			len(lines), len(allowed)+1, lines)
	}
	for _, line := range lines {
		if !strings.Contains(line, "list-") &&
			!strings.Contains(line, "has-session") &&
			!strings.Contains(line, "display-message") {
			t.Fatalf("tmux wrapper forwarded a mutating invocation: %q", line)
		}
	}
}

func TestShadowLegacyScriptRedirectsAllDirectCleanup(t *testing.T) {
	source := `local siddir=/tmp/cc-sid
(( ${#corpse} )) && rm -f "$s" 2>/dev/null
rm -f "$siddir/$sock"
[[ "$fd" == "$HOME/.codex/sessions/"*/rollout-*.jsonl ]]
idx="$HOME/.codex/session_index.jsonl"
store="$HOME/.claude/projects"
`
	shadow := shadowLegacyScript(
		source,
		"/jailed/sid",
		"/jailed/claude/projects",
		"/jailed/codex",
	)
	if strings.Contains(shadow, `rm -f "$s"`) {
		t.Fatalf("shadow retained real socket removal: %s", shadow)
	}
	if !strings.Contains(shadow, "siddir='/jailed/sid'") ||
		!strings.Contains(shadow, `rm -f "$siddir/$sock"`) ||
		!strings.Contains(shadow, `"/jailed/codex/sessions/"*`) ||
		!strings.Contains(shadow, `idx="/jailed/codex/session_index.jsonl"`) ||
		!strings.Contains(shadow, `store="/jailed/claude/projects"`) {
		t.Fatalf("shadow did not redirect SID cleanup: %s", shadow)
	}
}

// TestLiveLegacyScriptStillCarriesEveryShadowAnchor fails the moment the live
// picker renames one of the strings the shadow rewrite depends on. Each anchor
// silently stops matching when it drifts, and the shadow then runs against the
// founder's real /tmp/cc-sid and real transcripts instead of the jail.
func TestLiveLegacyScriptStillCarriesEveryShadowAnchor(t *testing.T) {
	t.Setenv(LegacyScriptEnv, "")
	scriptPath, err := ResolveLegacyScript(homeForTest())
	if err != nil {
		t.Skipf("no live picker to check: %v", err)
	}
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	script := string(content)
	for _, anchor := range []string{
		"siddir=/tmp/cc-sid",
		"$HOME/.codex/sessions",
		"$HOME/.codex/session_index.jsonl",
		"$HOME/.claude/projects",
		`(( ${#corpse} )) && rm -f "$s" 2>/dev/null`,
		`source "$CC_FLEET_HOME/cc-portable.sh"`,
	} {
		if !strings.Contains(script, anchor) {
			t.Fatalf("live picker %s no longer contains the shadow anchor %q",
				scriptPath, anchor)
		}
	}
}

func homeForTest() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
