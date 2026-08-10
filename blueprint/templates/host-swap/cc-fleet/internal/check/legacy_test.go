package check

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hostops/cc-fleet/internal/paths"
)

func TestLegacyRunnerMasksFZFAndInstrumentsFallback(t *testing.T) {
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
	scriptPath := filepath.Join(root, "legacy.zsh")
	script := `cc-ls() {
  local -a rows=()
  rows+=($'project\t7\t0\tR\tchat-7\t/work/project\t\e[2m↻\e[0m project        │ name                           │     7p │   12B │ now')
  if ! command -v fzf >/dev/null; then
    printf '%s\n' "${rows[@]}" | sort -t$'\t' -k1,1 -k3,3nr | cut -f7
    echo "cc-ls: fzf not found"
    return 1
  fi
	  return 99
	}
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(LegacyOutputEnv, "")
	t.Setenv(LegacyScriptEnv, scriptPath)
	output, err := runLegacy(context.Background(), paths.Values{
		Home:        home,
		ClaudeRoots: []string{claude},
		CodexRoot:   codex,
		SIDDir:      sid,
		TmuxDir:     tmuxDir,
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
	if string(content) != script {
		t.Fatal("legacy reference script was modified")
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
