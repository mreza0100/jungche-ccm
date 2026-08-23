// Package paths centralizes filesystem defaults and their test-jail overrides.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const (
	EnvDB          = "PFM_DB"
	EnvSharedDB    = "PFM_SHARED_DB"
	EnvSIDDir      = "PFM_SID_DIR"
	EnvClaudeRoots = "PFM_CLAUDE_ROOTS"
	EnvCodexRoot   = "PFM_CODEX_ROOT"
	// EnvOpencodeRoot jails OpenCode's data home (~/.local/share/opencode),
	// the directory holding its SQLite session store opencode.db.
	EnvOpencodeRoot = "PFM_OPENCODE_ROOT"
	EnvTmuxDir      = "PFM_TMUX_DIR"
	EnvHome         = "PFM_HOME"
	// EnvRealHome lets the rare test that MUST see the operator's own
	// machine — building against the real module cache, probing a live
	// config — opt back in by name. Everything else running under `go
	// test` is refused the real home rather than handed it silently.
	EnvRealHome   = "PFM_TEST_REAL_HOME"
	EnvProcRoot   = "PFM_PROC_ROOT"
	EnvCgroupRoot = "PFM_CGROUP_ROOT"
	// EnvTmuxConf pins the config a chat's tmux server is born with. Unset —
	// the way a real chat runs — the server loads ~/.tmux.conf like every other
	// terminal on the machine, because a chat IS a terminal the user lives in:
	// one that ignores their config wears tmux's default green status bar at
	// the bottom while every other window wears theirs on top. Jails set it to
	// /dev/null so a machine's real config can never steer a fixture.
	EnvTmuxConf   = "PFM_TMUX_CONF"
	defaultTmpDir = "/tmp"
)

// TmuxConfigArguments returns the `-f <config>` a chat server is created with,
// or nothing at all so tmux loads the user's own config.
func TmuxConfigArguments() []string {
	if config := os.Getenv(EnvTmuxConf); config != "" {
		return []string{"-f", config}
	}
	return nil
}

// Values contains the filesystem locations used by pfm.
//
// DB is this binary's own derived cache (transcripts, rollouts, names) and
// nothing else reads it. SharedDB is the authoritative operator state: kills,
// teammates, and the primary account.
type Values struct {
	DB          string
	SharedDB    string
	SIDDir      string
	ClaudeRoots []string
	CodexRoot   string
	// OpenCodeRoot is OpenCode's data home: the directory holding its SQLite
	// session store (opencode.db). It is the OpenCode twin of CodexRoot.
	OpenCodeRoot string
	TmuxDir      string
	Home         string
	// ArchiveDir is ~/.claude-archive: where archived transcripts and rollouts
	// go, with the manifest that puts them back. It is defined relative to Home.
	ArchiveDir string
	ProcRoot   string
	CgroupRoot string
}

// EnvOr returns a non-empty environment override, or fallback otherwise.
func EnvOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// Resolve returns the standard host paths with all K4 test-jail overrides
// applied. It only computes pathnames; it does not access the filesystem.
func Resolve() (Values, error) {
	home := os.Getenv(EnvHome)
	if home == "" {
		// A test that never set up its jail would otherwise resolve to the
		// OPERATOR'S OWN home: the fleet.db their live chats are indexed in,
		// the ~/.claude/projects their transcripts live in. That is not a
		// hypothetical — one `go test ./...` run outside the fence has written
		// fixture transcripts into a real account and held write transactions
		// on a real fleet.db until the TUI could no longer open it.
		//
		// Resolving is silent by design: it computes pathnames and touches
		// nothing, so a jailed run and an escaped one are byte-identical here
		// and stay indistinguishable until something WRITES. This is the last
		// place the difference is visible, so a missing jail is an error here
		// rather than a surprise several layers down.
		if testing.Testing() && os.Getenv(EnvRealHome) == "" {
			return Values{}, fmt.Errorf(
				"refusing to resolve the operator's real home directory inside a test: "+
					"point %s at a temporary directory (see internal/testjail), or set %s=1 "+
					"if this test genuinely must read the host",
				EnvHome, EnvRealHome,
			)
		}
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return Values{}, fmt.Errorf("resolve home directory: %w", err)
		}
	}

	var claudeRoots []string
	if value := os.Getenv(EnvClaudeRoots); value != "" {
		claudeRoots = filepath.SplitList(value)
	}

	tmuxBase := EnvOr("TMUX_TMPDIR", defaultTmpDir)

	return Values{
		DB: EnvOr(EnvDB, filepath.Join(home, ".local", "state", "pfm", "fleet.db")),
		// The shared database defaults to $HOME/.cc/fleet.db. PFM_DB already
		// overrides the private cache, so the shared handle gets a distinct name.
		SharedDB:    EnvOr(EnvSharedDB, filepath.Join(home, ".cc", "fleet.db")),
		SIDDir:      EnvOr(EnvSIDDir, filepath.Join(defaultTmpDir, "cc-sid")),
		ClaudeRoots: claudeRoots,
		CodexRoot:   EnvOr(EnvCodexRoot, filepath.Join(home, ".codex")),
		OpenCodeRoot: EnvOr(
			EnvOpencodeRoot,
			filepath.Join(home, ".local", "share", "opencode"),
		),
		TmuxDir:    EnvOr(EnvTmuxDir, filepath.Join(tmuxBase, "tmux-"+strconv.Itoa(os.Getuid()))),
		Home:       home,
		ArchiveDir: filepath.Join(home, ".claude-archive"),
		ProcRoot:   EnvOr(EnvProcRoot, "/proc"),
		CgroupRoot: EnvOr(EnvCgroupRoot, "/sys/fs/cgroup"),
	}, nil
}
