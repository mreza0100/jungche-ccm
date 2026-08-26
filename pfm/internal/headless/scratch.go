package headless

import (
	"fmt"

	"hostops/pfm/internal/paths"
)

// preparedScratchDir resolves the directory writePreparedExchange and
// writePreparedCapture use for their disposable per-run files (a prepared
// exchange, a live pane capture) when the caller supplies none — as
// options.TempDir is in every real chat-status command, only tests inject a
// directory directly.
//
// It goes through internal/paths — the same package cmd/pfm's launch command
// uses for its own disposable status file (SIDDir, via os.CreateTemp) — so
// the PFM_SID_DIR jail override the rest of the fleet honors keeps working
// here too. A bare filepath.Join("tmp", "chat-status") would instead resolve
// CWD-relative: whatever directory the operator happens to be standing in
// when they run a read-only status command gets a tmp/ dropped into it.
//
// A path that fails to resolve is an error, never a second-choice guess —
// silently falling back to something else is how the CWD bug came to exist.
func preparedScratchDir(directory string) (string, error) {
	if directory != "" {
		return directory, nil
	}
	resolved, err := paths.Resolve()
	if err != nil {
		return "", fmt.Errorf("resolve scratch directory: %w", err)
	}
	return resolved.SIDDir, nil
}
