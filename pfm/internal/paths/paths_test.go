package paths

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	pfmengine "hostops/pfm/internal/engine"
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
	cgroupRoot := filepath.Join(testRoot, "cgroup")
	sharedDB := filepath.Join(testRoot, "shared", "fleet.db")

	t.Setenv(EnvHome, home)
	t.Setenv(EnvDB, db)
	t.Setenv(EnvSharedDB, sharedDB)
	t.Setenv(EnvSIDDir, sidDir)
	t.Setenv(EnvClaudeRoots, strings.Join(claudeRoots, string(os.PathListSeparator)))
	t.Setenv(EnvCodexRoot, codexRoot)
	opencodeRoot := filepath.Join(testRoot, "opencode")
	t.Setenv(EnvOpencodeRoot, opencodeRoot)
	t.Setenv(EnvTmuxDir, tmuxDir)
	t.Setenv(EnvProcRoot, procRoot)
	t.Setenv(EnvCgroupRoot, cgroupRoot)

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := Values{
		DB:       db,
		SharedDB: sharedDB,
		SIDDir:   sidDir,
		Roots: map[pfmengine.ID][]string{
			pfmengine.Claude:   claudeRoots,
			pfmengine.Codex:    {codexRoot},
			pfmengine.Opencode: {opencodeRoot},
		},
		TmuxDir: tmuxDir,
		Home:    home,
		// The carrier and the archive have no override of their own: both are
		// defined relative to Home, and jailing Home jails them.
		ArchiveDir: filepath.Join(home, ".claude-archive"),
		ProcRoot:   procRoot,
		CgroupRoot: cgroupRoot,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Resolve() = %#v, want %#v", got, want)
	}
}

// The shared state store defaults to ~/.cc/fleet.db, never the private cache's
// directory, and the two overrides must not collide.
func TestResolveSharedStoreDefaultsOutsideThePrivateCache(t *testing.T) {
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

func TestResolveDoesNotFabricateClaudeAccountRoots(t *testing.T) {
	testRoot := t.TempDir()
	home := filepath.Join(testRoot, "home")
	t.Setenv(EnvHome, home)
	t.Setenv(EnvClaudeRoots, "")

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	wantClaude := pfmengine.MustLookup(pfmengine.Claude).DefaultRoots(home)
	if !reflect.DeepEqual(got.Roots[pfmengine.Claude], wantClaude) {
		t.Fatalf("Resolve().Roots[cc] = %#v, want %#v", got.Roots[pfmengine.Claude], wantClaude)
	}
}

// The OpenCode root defaults under Home and jails with it; the explicit
// override wins over the default.
func TestResolveOpencodeRootDefaultsUnderHome(t *testing.T) {
	testRoot := t.TempDir()
	home := filepath.Join(testRoot, "home")
	t.Setenv(EnvHome, home)
	t.Setenv(EnvOpencodeRoot, "")

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := filepath.Join(home, ".local", "share", "opencode")
	if roots := got.Roots[pfmengine.Opencode]; len(roots) != 1 || roots[0] != want {
		t.Fatalf("Resolve().Roots[ox] = %#v, want %q", roots, want)
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
