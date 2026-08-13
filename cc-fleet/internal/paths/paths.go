// Package paths centralizes filesystem defaults and their test-jail overrides.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const (
	EnvDB          = "CC_FLEET_DB"
	EnvSharedDB    = "CC_FLEET_SHARED_DB"
	EnvSIDDir      = "CC_FLEET_SID_DIR"
	EnvClaudeRoots = "CC_FLEET_CLAUDE_ROOTS"
	EnvCodexRoot   = "CC_FLEET_CODEX_ROOT"
	EnvTmuxDir     = "CC_FLEET_TMUX_DIR"
	EnvHome        = "CC_FLEET_HOME"
	EnvProcRoot    = "CC_FLEET_PROC_ROOT"
	// EnvTmuxConf pins the config a chat's tmux server is born with. Unset —
	// the way a real chat runs — the server loads ~/.tmux.conf like every other
	// terminal on the machine, because a chat IS a terminal the user lives in:
	// one that ignores their config wears tmux's default green status bar at
	// the bottom while every other window wears theirs on top. Jails set it to
	// /dev/null so a machine's real config can never steer a fixture.
	EnvTmuxConf   = "CC_FLEET_TMUX_CONF"
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

// Values contains the filesystem locations used by cc-fleet.
//
// DB is this binary's own derived cache (transcripts, rollouts, names) and
// nothing else reads it. SharedDB and HiddenCarrier are the fleet's state,
// co-owned with the zsh half through cc-db.sh: user-visible decisions — hides,
// teammates, the primary account — live there and nowhere else.
type Values struct {
	DB          string
	SharedDB    string
	SIDDir      string
	ClaudeRoots []string
	CodexRoot   string
	TmuxDir     string
	Home        string
	// HiddenCarrier is ~/.claude/.cc-ls-hidden: the hidden set as it stands
	// between picker runs (cc-db.sh:110-115). It has no environment override
	// of its own because it is defined relative to Home, which EnvHome jails.
	HiddenCarrier string
	ProcRoot      string
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
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return Values{}, fmt.Errorf("resolve home directory: %w", err)
		}
	}

	claudeRoots := []string{
		filepath.Join(home, ".cc", "1", "projects"),
		filepath.Join(home, ".cc", "2", "projects"),
		filepath.Join(home, ".cc", "3", "projects"),
	}
	if value := os.Getenv(EnvClaudeRoots); value != "" {
		claudeRoots = filepath.SplitList(value)
	}

	tmuxBase := EnvOr("TMUX_TMPDIR", defaultTmpDir)

	return Values{
		DB: EnvOr(EnvDB, filepath.Join(home, ".local", "state", "cc-fleet", "fleet.db")),
		// cc-db.sh:41 reads the same file as $HOME/.cc/fleet.db. Its own
		// override is named CC_FLEET_DB, which this binary already spends on
		// its private cache, so the shared handle gets a distinct name.
		SharedDB:      EnvOr(EnvSharedDB, filepath.Join(home, ".cc", "fleet.db")),
		SIDDir:        EnvOr(EnvSIDDir, filepath.Join(defaultTmpDir, "cc-sid")),
		ClaudeRoots:   claudeRoots,
		CodexRoot:     EnvOr(EnvCodexRoot, filepath.Join(home, ".codex")),
		TmuxDir:       EnvOr(EnvTmuxDir, filepath.Join(tmuxBase, "tmux-"+strconv.Itoa(os.Getuid()))),
		Home:          home,
		HiddenCarrier: filepath.Join(home, ".claude", ".cc-ls-hidden"),
		ProcRoot:      EnvOr(EnvProcRoot, "/proc"),
	}, nil
}
