package paths

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveOverrides(t *testing.T) {
	testRoot := t.TempDir()
	t.Setenv("TMUX_TMPDIR", filepath.Join(testRoot, "t"))
	home := filepath.Join(testRoot, "home")
	db := filepath.Join(testRoot, "fleet.db")
	sidDir := filepath.Join(testRoot, "sid")
	claudeRoots := []string{
		filepath.Join(testRoot, "claude-1"),
		filepath.Join(testRoot, "claude-2"),
	}
	codexRoot := filepath.Join(testRoot, "codex")
	tmuxDir := filepath.Join(testRoot, "tmux")
	procRoot := filepath.Join(testRoot, "proc")

	t.Setenv(EnvHome, home)
	t.Setenv(EnvDB, db)
	t.Setenv(EnvSIDDir, sidDir)
	t.Setenv(EnvClaudeRoots, strings.Join(claudeRoots, string(os.PathListSeparator)))
	t.Setenv(EnvCodexRoot, codexRoot)
	t.Setenv(EnvTmuxDir, tmuxDir)
	t.Setenv(EnvProcRoot, procRoot)

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := Values{
		DB:          db,
		SIDDir:      sidDir,
		ClaudeRoots: claudeRoots,
		CodexRoot:   codexRoot,
		TmuxDir:     tmuxDir,
		Home:        home,
		ProcRoot:    procRoot,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Resolve() = %#v, want %#v", got, want)
	}
}

func TestResolveUsesScratchTmuxBase(t *testing.T) {
	testRoot := t.TempDir()
	t.Setenv(EnvHome, filepath.Join(testRoot, "home"))
	t.Setenv(EnvDB, filepath.Join(testRoot, "fleet.db"))
	t.Setenv(EnvSIDDir, filepath.Join(testRoot, "sid"))
	t.Setenv(EnvClaudeRoots, filepath.Join(testRoot, "claude"))
	t.Setenv(EnvCodexRoot, filepath.Join(testRoot, "codex"))
	t.Setenv(EnvTmuxDir, "")
	t.Setenv(EnvProcRoot, filepath.Join(testRoot, "proc"))
	t.Setenv("TMUX_TMPDIR", testRoot)

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := filepath.Join(testRoot, "tmux-"+strconvUID())
	if got.TmuxDir != want {
		t.Fatalf("Resolve().TmuxDir = %q, want %q", got.TmuxDir, want)
	}
}
