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
	"strings"

	"hostops/cc-fleet/internal/paths"
)

const (
	// LegacyOutputEnv supplies a recorded legacy output file for jailed tests.
	LegacyOutputEnv = "CC_FLEET_CHECK_LEGACY_OUTPUT"
	// LegacyScriptEnv overrides the read-only reference script location.
	LegacyScriptEnv = "CC_FLEET_CHECK_LEGACY_SCRIPT"
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

func runLegacy(ctx context.Context, resolved paths.Values) (string, error) {
	scriptPath := os.Getenv(LegacyScriptEnv)
	if scriptPath == "" {
		scriptPath = filepath.Join(
			resolved.Home,
			"work",
			"host-ops",
			"oldbox",
			"scripts",
			"cc-fleet.zsh",
		)
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
	for _, name := range []string{"sessions", "session_index.jsonl"} {
		source := filepath.Join(resolved.CodexRoot, name)
		if _, err := os.Stat(source); err == nil {
			if err := os.Symlink(source, filepath.Join(shadowHome, ".codex", name)); err != nil {
				return "", fmt.Errorf("link legacy Codex %s: %w", name, err)
			}
		}
	}
	for _, name := range []string{".cc-ls-hidden", ".cc-ls-hidden.at"} {
		if err := copyRegular(
			filepath.Join(resolved.Home, ".claude", name),
			filepath.Join(shadowHome, ".claude", name),
		); err != nil {
			return "", err
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
	scriptCopy := filepath.Join(root, "legacy.zsh")
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

	command := exec.CommandContext(ctx, "zsh", "-ic", "cc-ls -a")
	command.Dir = root
	command.Env = shadowEnvironment(resolved, shadowHome, runtimeDir, binDir)
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
	tmuxScript := fmt.Sprintf(`#!/bin/sh
for arg in "$@"; do
  case "$arg" in
    attach|attach-session|kill-server|kill-session|new-session|new-window|rename-window|select-window|set|set-option|set-window-option|setw)
      exit 0
      ;;
  esac
done
exec %s "$@"
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
	shadowHome, runtimeDir, binDir string,
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
