package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"hostops/pfm/internal/codexlaunch"
	"hostops/pfm/internal/deps"
)

// runInternalCodexLaunch is the one runtime appendix door used by shell and
// argv launchers. Exec preserves the pane's native Codex process identity.
func runInternalCodexLaunch(args []string, stderr io.Writer, runtime commandRuntime) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: pfm internal codex-launch BINARY [arguments...]")
		return 2
	}
	binary, err := deps.Resolve(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "resolve Codex launcher: %v\n", err)
		return 1
	}
	prepared, err := codexlaunch.Prepare(context.Background(), binary, runtime.Paths.Home, args[1:], codexlaunch.ReadDeveloper)
	if err != nil {
		fmt.Fprintf(stderr, "pfm Codex appendix: %v\n", err)
		return 1
	}
	if err := launchExec(binary, append([]string{binary}, prepared...), os.Environ()); err != nil {
		fmt.Fprintf(stderr, "launch Codex: %v\n", err)
		return 1
	}
	return 0
}
