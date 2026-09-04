package main

import (
	"fmt"
	"io"

	pfmconfig "hostops/pfm/internal/config"
)

// runInternalTmuxTitles prints the tmux server options the shim must apply for
// the machine's tmux.titles policy, one per line:
//
//	<option-name> <value…>
//
// The option name is the first word and the value is the whole rest of the
// line, so the shim never re-spells set-titles-string and the format cannot
// drift between the shell path and the two Go spawn paths.
//
// A DISABLED policy prints nothing at all: pfm sets neither option and
// whatever the host put on the outer terminal before tmux started stands.
// Every failure is a one-line stderr reason with a nonzero exit and NO stdout
// — the shim applies nothing on nonzero, which leaves the host's title alone
// rather than seizing it on a config it could not read.
func runInternalTmuxTitles(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: pfm internal tmux-titles")
		return 2
	}
	if runtime.ConfigError != nil {
		fmt.Fprintf(stderr, "pfm internal tmux-titles: config unreadable: %v\n", runtime.ConfigError)
		return 1
	}
	for _, option := range pfmconfig.TmuxTitlesOrDefault(&runtime.Config.Tmux.Titles).Options() {
		// Options() speaks the argv tmux itself takes ("set-option", "-g",
		// name, value); the shim's line protocol carries only the name and
		// the value, because the shim already knows it is setting a global
		// server option.
		fmt.Fprintf(stdout, "%s %s\n", option[len(option)-2], option[len(option)-1])
	}
	return 0
}
