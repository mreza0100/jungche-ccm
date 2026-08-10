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
	EnvSIDDir      = "CC_FLEET_SID_DIR"
	EnvClaudeRoots = "CC_FLEET_CLAUDE_ROOTS"
	EnvCodexRoot   = "CC_FLEET_CODEX_ROOT"
	EnvTmuxDir     = "CC_FLEET_TMUX_DIR"
	EnvHome        = "CC_FLEET_HOME"
	EnvProcRoot    = "CC_FLEET_PROC_ROOT"
	defaultTmpDir  = "/tmp"
)

// Values contains the filesystem locations used by cc-fleet.
type Values struct {
	DB          string
	SIDDir      string
	ClaudeRoots []string
	CodexRoot   string
	TmuxDir     string
	Home        string
	ProcRoot    string
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
		DB:          EnvOr(EnvDB, filepath.Join(home, ".local", "state", "cc-fleet", "fleet.db")),
		SIDDir:      EnvOr(EnvSIDDir, filepath.Join(defaultTmpDir, "cc-sid")),
		ClaudeRoots: claudeRoots,
		CodexRoot:   EnvOr(EnvCodexRoot, filepath.Join(home, ".codex")),
		TmuxDir:     EnvOr(EnvTmuxDir, filepath.Join(tmuxBase, "tmux-"+strconv.Itoa(os.Getuid()))),
		Home:        home,
		ProcRoot:    EnvOr(EnvProcRoot, "/proc"),
	}, nil
}
