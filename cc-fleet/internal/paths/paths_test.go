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
	sharedDB := filepath.Join(testRoot, "shared", "fleet.db")

	t.Setenv(EnvHome, home)
	t.Setenv(EnvDB, db)
	t.Setenv(EnvSharedDB, sharedDB)
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
		SharedDB:    sharedDB,
		SIDDir:      sidDir,
		ClaudeRoots: claudeRoots,
		CodexRoot:   codexRoot,
		TmuxDir:     tmuxDir,
		Home:        home,
		// The carrier and the archive have no override of their own: both are
		// defined relative to Home, and jailing Home jails them.
		HiddenCarrier: filepath.Join(home, ".claude", ".cc-ls-hidden"),
		ArchiveDir:    filepath.Join(home, ".claude-archive"),
		ProcRoot:      procRoot,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Resolve() = %#v, want %#v", got, want)
	}
}

// The shared state store is the fleet's, not this binary's: it defaults beside
// the zsh half's ~/.cc/fleet.db and NEVER to the private cache's directory.
// CC_FLEET_DB is cc-db.sh's own override name for that shared file (cc-db.sh:41),
// so the two must not resolve to the same place when only CC_FLEET_DB is set.
func TestResolveSharedStoreDefaultsBesideTheZshHalf(t *testing.T) {
	testRoot := t.TempDir()
	home := filepath.Join(testRoot, "home")
	t.Setenv(EnvHome, home)
	t.Setenv(EnvDB, filepath.Join(testRoot, "cache", "fleet.db"))
	t.Setenv(EnvSharedDB, "")

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := filepath.Join(home, ".cc", "fleet.db")
	if got.SharedDB != want {
		t.Fatalf("Resolve().SharedDB = %q, want %q", got.SharedDB, want)
	}
	if got.SharedDB == got.DB {
		t.Fatalf("shared store and private cache resolved to the same file %q", got.DB)
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
