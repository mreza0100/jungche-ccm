package check

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"hostops/cc-fleet/internal/paths"
)

const (
	// LegacyOutputEnv supplies a recorded legacy output file for jailed tests.
	LegacyOutputEnv = "CC_FLEET_CHECK_LEGACY_OUTPUT"
	// LegacyScriptEnv overrides the read-only reference script location.
	LegacyScriptEnv = "CC_FLEET_LEGACY_ZSH"

	// LegacyScriptName is the picker this checker shadow-runs.
	LegacyScriptName = "cc-fleet.zsh"
	// legacyPortableName is the sh library the picker sources by self-location.
	legacyPortableName = "cc-portable.sh"
	// legacyDatabaseName is cc-db.sh's state store, addressed as
	// ${CC_FLEET_DB:-$HOME/.cc/fleet.db}.
	legacyDatabaseName = "fleet.db"
)

// legacySiblingPattern finds every bundle file the picker reaches through its
// own directory: "$CC_FLEET_HOME/cc-db.sh", ${(q)CC_FLEET_HOME}/cc-agent-open.sh
// and the like. The shadow copy must carry each one, because the copied script
// resolves CC_FLEET_HOME to the shadow root, not to the bundle it came from.
var legacySiblingPattern = regexp.MustCompile(
	`CC_FLEET_HOME\}?/([A-Za-z0-9._-]+)`,
)

// legacySourceLinePattern reads the source line install.sh writes into ~/.zshrc:
//
//	[[ -r "/path/to/cc-fleet.zsh" ]] && source "/path/to/cc-fleet.zsh"
var legacySourceLinePattern = regexp.MustCompile(
	`source[ \t]+"?([^"\n]*` + regexp.QuoteMeta(LegacyScriptName) + `)"?`,
)

// LegacyOutput runs the reference cc-ls fallback or reads a recorded test
// fixture. The real runner mirrors every legacy-writable path into a temporary
// tree and blocks all mutating tmux verbs.
func LegacyOutput(ctx context.Context, resolved paths.Values) (string, error) {
	if fixture := os.Getenv(LegacyOutputEnv); fixture != "" {
		content, err := os.ReadFile(fixture)
		if err != nil {
			return "", fmt.Errorf("read recorded legacy output: %w", err)
		}
		return string(content), nil
	}
	return runLegacy(ctx, resolved)
}

// ResolveLegacyScript locates the live cc-fleet.zsh whose behaviour this
// checker is the parity contract for. The tree is a template that ships inside
// the bundle it checks, so the script sits BESIDE the Go tree in a clone and is
// reachable through the installed link farm on a provisioned box. Each link of
// the chain is a fact that cannot go stale silently:
//
//  1. $CC_FLEET_LEGACY_ZSH — the explicit answer. Taken as given; an unreadable
//     path is an error rather than a silent fall-through to a different script.
//  2. ../cc-fleet.zsh beside this Go tree's own source directory. This is the
//     clone's layout (host-swap/cc-fleet.zsh next to host-swap/cc-fleet/), and
//     it is right whenever the checker is run from a checkout.
//  3. ../cc-fleet.zsh beside the running binary, then the binary's own
//     directory. cc-fleet.dev is built into the tree root, so the first form
//     covers a binary copied out with the tree; the second covers one dropped
//     into the bundle directly.
//  4. The installed link farm. install.sh symlinks cc-portable.sh into
//     ~/.claude/bin, so resolving that link names the bundle directory, whose
//     sibling is the script the shell actually sources.
//  5. ~/.zshrc's source line, which install.sh writes with the clone's absolute
//     path. Last because it is the only step that parses text, and first in
//     authority about what the founder's shell really loads — the two agree on
//     every installed box, and this catches a clone the earlier steps missed.
func ResolveLegacyScript(home string) (string, error) {
	if override := os.Getenv(LegacyScriptEnv); override != "" {
		if readable(override) {
			return override, nil
		}
		return "", fmt.Errorf(
			"%s names %q, which is not a readable file",
			LegacyScriptEnv,
			override,
		)
	}
	candidates := make([]string, 0, 5)
	if treeDir := treeSourceDir(); treeDir != "" {
		candidates = append(
			candidates,
			filepath.Join(filepath.Dir(treeDir), LegacyScriptName),
		)
	}
	if binaryDir := executableDir(); binaryDir != "" {
		candidates = append(
			candidates,
			filepath.Join(filepath.Dir(binaryDir), LegacyScriptName),
			filepath.Join(binaryDir, LegacyScriptName),
		)
	}
	if home != "" {
		if installed := installedBundleScript(home); installed != "" {
			candidates = append(candidates, installed)
		}
		if sourced := zshrcLegacyScript(filepath.Join(home, ".zshrc")); sourced != "" {
			candidates = append(candidates, sourced)
		}
	}
	for _, candidate := range candidates {
		if readable(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf(
		"locate the live %s: set %s, or keep it beside the cc-fleet tree "+
			"(tried %s)",
		LegacyScriptName,
		LegacyScriptEnv,
		strings.Join(candidates, ", "),
	)
}

// treeSourceDir is this package's own directory as recorded at compile time.
// It resolves to the clone whenever the binary was built from one; a -trimpath
// build records a relative path instead, and that case is declined rather than
// resolved against an unrelated working directory.
func treeSourceDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok || !filepath.IsAbs(file) {
		return ""
	}
	// <tree>/internal/check/legacy.go -> <tree>
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func executableDir() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	return filepath.Dir(executable)
}

// installedBundleScript names the picker through the link farm install.sh
// builds. It links cc-portable.sh — a sourced library, not a command — into
// ~/.claude/bin precisely so the bundle has one published address per file;
// resolving that link names the clone, and the picker is its sibling.
func installedBundleScript(home string) string {
	link := filepath.Join(home, ".claude", "bin", legacyPortableName)
	bundle, err := filepath.EvalSymlinks(link)
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(bundle), LegacyScriptName)
}

func zshrcLegacyScript(zshrc string) string {
	content, err := os.ReadFile(zshrc)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		match := legacySourceLinePattern.FindStringSubmatch(line)
		if match == nil || !filepath.IsAbs(match[1]) {
			continue
		}
		sourced := match[1]
		// Post-cutover the shell sources <bundle>/cc-fleet/shim/cc-fleet.zsh.
		// The shim is the Go engine's wrapper, not the legacy oracle this
		// checker diffs against — that oracle stays at <bundle>/cc-fleet.zsh,
		// beside the shim's tree root.
		shimDir := filepath.Dir(sourced)
		if filepath.Base(shimDir) == "shim" {
			treeRoot := filepath.Dir(shimDir)
			return filepath.Join(filepath.Dir(treeRoot), LegacyScriptName)
		}
		return sourced
	}
	return ""
}

func readable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func runLegacy(ctx context.Context, resolved paths.Values) (string, error) {
	scriptPath, err := ResolveLegacyScript(resolved.Home)
	if err != nil {
		return "", err
	}
	bundleDir := filepath.Dir(scriptPath)
	if canonical, resolveErr := filepath.EvalSymlinks(scriptPath); resolveErr == nil {
		bundleDir = filepath.Dir(canonical)
	}
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", fmt.Errorf("read legacy script %q: %w", scriptPath, err)
	}

	root, err := os.MkdirTemp("", "cc-fleet-check-")
	if err != nil {
		return "", fmt.Errorf("create legacy shadow: %w", err)
	}
	defer os.RemoveAll(root)
	shadowHome := filepath.Join(root, "home")
	shadowSID := filepath.Join(root, "sid")
	binDir := filepath.Join(root, "bin")
	runtimeDir := filepath.Join(root, "tmp")
	for _, directory := range []string{
		filepath.Join(shadowHome, ".claude"),
		filepath.Join(shadowHome, ".cc"),
		filepath.Join(shadowHome, ".codex"),
		shadowSID,
		binDir,
		runtimeDir,
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return "", fmt.Errorf("create legacy shadow directory: %w", err)
		}
	}

	if len(resolved.ClaudeRoots) > 0 {
		if err := os.Symlink(
			resolved.ClaudeRoots[0],
			filepath.Join(shadowHome, ".claude", "projects"),
		); err != nil {
			return "", fmt.Errorf("link legacy Claude store: %w", err)
		}
	}
	if err := os.Symlink(
		filepath.Join(shadowHome, ".claude"),
		filepath.Join(shadowHome, ".cc", "1"),
	); err != nil {
		return "", fmt.Errorf("link legacy account 1: %w", err)
	}
	for index := 1; index < len(resolved.ClaudeRoots) && index < 3; index++ {
		accountDir := filepath.Join(shadowHome, ".cc", fmt.Sprint(index+1))
		if err := os.MkdirAll(accountDir, 0o700); err != nil {
			return "", err
		}
		if err := os.Symlink(
			resolved.ClaudeRoots[index],
			filepath.Join(accountDir, "projects"),
		); err != nil {
			return "", fmt.Errorf("link legacy account %d: %w", index+1, err)
		}
	}
	codexLinks := []string{"sessions", "session_index.jsonl"}
	// state_<N>.sqlite is where a Codex thread's identity now lives, and a
	// paginated thread may write no rollout file at all. Link the generations in
	// so a reference script that reads the store sees the real one instead of an
	// empty shadow .codex, which would read as "this box has no Codex threads".
	states, err := filepath.Glob(filepath.Join(resolved.CodexRoot, "state_*.sqlite"))
	if err != nil {
		return "", fmt.Errorf("list legacy Codex state files: %w", err)
	}
	for _, state := range states {
		codexLinks = append(codexLinks, filepath.Base(state))
	}
	for _, name := range codexLinks {
		source := filepath.Join(resolved.CodexRoot, name)
		if _, err := os.Stat(source); err == nil {
			if err := os.Symlink(source, filepath.Join(shadowHome, ".codex", name)); err != nil {
				return "", fmt.Errorf("link legacy Codex %s: %w", name, err)
			}
		}
	}
	// The hide set travels in HiddenCarrier, and its .adopted stamp is what
	// tells the picker the file already holds everything the database does.
	// Copy both, or the shadow re-runs the one-time adoption merge against a
	// database it must not be allowed to reach.
	if resolved.HiddenCarrier != "" {
		carrier := filepath.Base(resolved.HiddenCarrier)
		for _, name := range []string{carrier, carrier + ".at", carrier + ".adopted"} {
			if err := copyRegular(
				filepath.Join(filepath.Dir(resolved.HiddenCarrier), name),
				filepath.Join(shadowHome, ".claude", name),
			); err != nil {
				return "", err
			}
		}
	}
	if err := copyRegular(
		filepath.Join(resolved.Home, ".claude-primary"),
		filepath.Join(shadowHome, ".claude-primary"),
	); err != nil {
		return "", err
	}
	if err := copyDirectoryFiles(resolved.SIDDir, shadowSID); err != nil {
		return "", err
	}

	claudeRoot := ""
	if len(resolved.ClaudeRoots) > 0 {
		claudeRoot = resolved.ClaudeRoots[0]
		if canonical, resolveErr := filepath.EvalSymlinks(claudeRoot); resolveErr == nil {
			claudeRoot = canonical
		}
	}
	shadowScript := shadowLegacyScript(
		string(script),
		shadowSID,
		claudeRoot,
		resolved.CodexRoot,
	)
	// The copied script self-locates: CC_FLEET_HOME becomes the shadow root, so
	// every sibling it sources or execs must sit beside the copy. cc-portable.sh
	// is the one it SOURCES — without it the picker loses cc_find_meta and lists
	// nothing at all, silently.
	scriptCopy := filepath.Join(root, "legacy.zsh")
	if err := copyLegacySiblings(string(script), bundleDir, root); err != nil {
		return "", err
	}
	if err := os.WriteFile(scriptCopy, []byte(shadowScript), 0o600); err != nil {
		return "", fmt.Errorf("write legacy shadow script: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(shadowHome, ".zshrc"),
		[]byte("source "+zshQuote(scriptCopy)+"\n"),
		0o600,
	); err != nil {
		return "", fmt.Errorf("write shadow zshrc: %w", err)
	}
	if err := writeWrappers(binDir); err != nil {
		return "", err
	}
	shadowDatabase := filepath.Join(root, legacyDatabaseName)
	seedLegacyDatabase(ctx, resolved.SharedDB, shadowDatabase)

	command := exec.CommandContext(ctx, "zsh", "-ic", "cc-ls -a")
	command.Dir = root
	command.Env = shadowEnvironment(
		resolved,
		shadowHome,
		runtimeDir,
		binDir,
		shadowDatabase,
	)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	var exitError *exec.ExitError
	if err != nil && !(errors.As(err, &exitError) && exitError.ExitCode() == 1) {
		return "", fmt.Errorf(
			"run legacy fallback: %w: %s",
			err,
			strings.TrimSpace(stderr.String()),
		)
	}
	if stdout.Len() == 0 && stderr.Len() > 0 {
		return "", fmt.Errorf("legacy fallback emitted no rows: %s", strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func shadowLegacyScript(script, sidDir, claudeRoot, codexRoot string) string {
	script = strings.ReplaceAll(
		script,
		"siddir=/tmp/cc-sid",
		"siddir="+zshQuote(sidDir),
	)
	// ProcFS reports canonical fd targets under the real Codex root. Point the
	// disposable script's read-only prefix tests at that same root; a shadow
	// HOME symlink would otherwise make every live rollout look unrelated.
	realCodex := zshDoubleQuoteBody(filepath.Clean(codexRoot))
	script = strings.ReplaceAll(
		script,
		"$HOME/.codex/sessions",
		realCodex+"/sessions",
	)
	script = strings.ReplaceAll(
		script,
		"$HOME/.codex/session_index.jsonl",
		realCodex+"/session_index.jsonl",
	)
	if claudeRoot != "" {
		script = strings.ReplaceAll(
			script,
			"$HOME/.claude/projects",
			zshDoubleQuoteBody(filepath.Clean(claudeRoot)),
		)
	}
	// cc-ls's one direct mutation outside siddir is its old-corpse cleanup.
	// The shadow copy probes the same socket but deliberately cannot unlink it.
	script = strings.ReplaceAll(
		script,
		`(( ${#corpse} )) && rm -f "$s" 2>/dev/null`,
		`(( ${#corpse} )) && :`,
	)
	return script
}

// copyLegacySiblings mirrors every bundle file the script addresses through its
// own directory into the shadow root. The list is READ OUT OF THE SCRIPT rather
// than hard-coded, so a new sibling arrives with the script that needs it.
// A named file that the bundle does not actually contain is skipped: the picker
// already guards each one with a readability test.
func copyLegacySiblings(script, bundleDir, shadowRoot string) error {
	if bundleDir == "" {
		return nil
	}
	seen := make(map[string]struct{})
	for _, match := range legacySiblingPattern.FindAllStringSubmatch(script, -1) {
		name := match[1]
		if name == "" || name == "." || name == ".." ||
			strings.ContainsRune(name, filepath.Separator) {
			continue
		}
		if _, already := seen[name]; already {
			continue
		}
		seen[name] = struct{}{}
		if err := copyRegular(
			filepath.Join(bundleDir, name),
			filepath.Join(shadowRoot, name),
		); err != nil {
			return err
		}
	}
	if _, carried := seen[legacyPortableName]; !carried {
		return fmt.Errorf(
			"legacy script names no %s sibling: the shadow copy would run "+
				"without the GNU/BSD seam and list nothing",
			legacyPortableName,
		)
	}
	if _, copied := os.Stat(
		filepath.Join(shadowRoot, legacyPortableName),
	); copied != nil {
		return fmt.Errorf(
			"copy legacy %s from %q: %w",
			legacyPortableName,
			bundleDir,
			copied,
		)
	}
	return nil
}

// seedLegacyDatabase gives the shadow picker a private copy of the fleet's
// shared state store. The copy matters twice: its `chat` table is the picker's
// transcript name/prompt cache, and a cold shadow would re-parse gigabytes of
// transcripts with jq on every check — and the picker WRITES that cache back,
// so it must never be pointed at the live database.
//
// The snapshot is taken with sqlite3's own backup over a mode=ro connection: a
// plain file copy of a WAL database can tear, and this way the live store is
// opened read-only by construction rather than by promise. Failure is not an
// error — the picker degrades to a cold parse, which is slow, not wrong.
func seedLegacyDatabase(ctx context.Context, source, shadowDatabase string) {
	if source == "" || !readable(source) {
		return
	}
	sqlite, err := exec.LookPath("sqlite3")
	if err != nil {
		return
	}
	command := exec.CommandContext(
		ctx,
		sqlite,
		"-readonly",
		"file:"+source+"?mode=ro",
		".backup "+sqliteQuote(shadowDatabase),
	)
	if err := command.Run(); err != nil {
		_ = os.Remove(shadowDatabase)
	}
}

func sqliteQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func zshDoubleQuoteBody(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`$`, `\$`,
		"`", "\\`",
	)
	return replacer.Replace(value)
}

func copyDirectoryFiles(sourceDir, destinationDir string) error {
	entries, err := os.ReadDir(sourceDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy SID source: %w", err)
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		if err := copyRegular(
			filepath.Join(sourceDir, entry.Name()),
			filepath.Join(destinationDir, entry.Name()),
		); err != nil {
			return err
		}
	}
	return nil
}

func copyRegular(source, destination string) error {
	content, err := os.ReadFile(source)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy shadow input %q: %w", source, err)
	}
	if err := os.WriteFile(destination, content, 0o600); err != nil {
		return fmt.Errorf("write legacy shadow input %q: %w", destination, err)
	}
	return nil
}

func writeWrappers(binDir string) error {
	realCut, err := exec.LookPath("cut")
	if err != nil {
		return fmt.Errorf("find cut for legacy shadow: %w", err)
	}
	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("find tmux for legacy shadow: %w", err)
	}
	cutScript := fmt.Sprintf(`#!/bin/sh
if [ "$#" -eq 1 ] && [ "$1" = "-f7" ]; then
  tmp="$(mktemp)"
  trap 'rm -f "$tmp"' EXIT
  cat > "$tmp"
  kind="$(awk -F '	' 'NR == 1 { print $4 }' "$tmp")"
  case "$kind" in
    L|R|A|X)
      awk -F '	' 'BEGIN { OFS="\t" } {
        identity=$5
        if ($4 == "L" && $8 != "" && index($7, "⬢") == 0) identity=$8
        print "%s", $4, $5, identity, $6, $7
      }' "$tmp"
      exit 0
      ;;
  esac
  exec %s "$@" < "$tmp"
fi
exec %s "$@"
`, legacyRowMarker, shellQuote(realCut), shellQuote(realCut))
	// The shadow probes LIVE tmux servers, so this wrapper is the only thing
	// between a reference script and the founder's running fleet. It is an
	// ALLOWLIST, not a blocklist: the script it guards is 1,500 lines and grows,
	// and a blocklist silently stops covering the verb it never heard of. The
	// command is the first non-option word after tmux's own value-taking flags;
	// anything not on the read-only list is answered with success and no action.
	tmuxScript := fmt.Sprintf(`#!/bin/sh
verb=""
skip=0
for arg in "$@"; do
  if [ "$skip" = 1 ]; then skip=0; continue; fi
  case "$arg" in
    -L|-S|-f|-c|-T) skip=1; continue ;;
    -*) continue ;;
  esac
  verb="$arg"
  break
done
case "$verb" in
  list-panes|list-windows|list-sessions|list-clients|list-commands|list-keys|\
  lsp|lsw|ls|lsc|has-session|has|display-message|display|show-options|show|\
  show-window-options|showw|show-environment|showenv|info|server-info)
    exec %s "$@"
    ;;
esac
exit 0
`, shellQuote(realTmux))
	for name, content := range map[string]string{
		"cut":  cutScript,
		"tmux": tmuxScript,
	} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(content), 0o700); err != nil {
			return fmt.Errorf("write legacy %s wrapper: %w", name, err)
		}
	}
	return nil
}

func shadowEnvironment(
	resolved paths.Values,
	shadowHome, runtimeDir, binDir, shadowDatabase string,
) []string {
	filteredPath := make([]string, 0)
	for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
		if directory == "" {
			continue
		}
		info, err := os.Stat(filepath.Join(directory, "fzf"))
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			continue
		}
		filteredPath = append(filteredPath, directory)
	}
	values := []string{
		"HOME=" + shadowHome,
		"ZDOTDIR=" + shadowHome,
		"TMPDIR=" + runtimeDir,
		"TMUX=",
		"TMUX_PANE=",
		"TMUX_TMPDIR=" + filepath.Dir(resolved.TmuxDir),
		// cc-db.sh's own override. Without it the shadow picker would write its
		// name cache and its hide sync into the live ~/.cc/fleet.db.
		"CC_FLEET_DB=" + shadowDatabase,
		"PATH=" + strings.Join(
			append([]string{binDir}, filteredPath...),
			string(os.PathListSeparator),
		),
	}
	for _, name := range []string{"LANG", "LC_ALL", "TERM", "COLORTERM"} {
		if value := os.Getenv(name); value != "" {
			values = append(values, name+"="+value)
		}
	}
	return values
}

func zshQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
