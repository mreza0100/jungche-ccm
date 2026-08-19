package main

import (
	"fmt"
	"io"
	"os"

	"hostops/pfm/internal/policy"
)

// runAutonomy prints the resolved permission posture — "on" or "off" — as the single word the zsh
// shim tests. It is the one reader of the posture that the shell can call, so the shell and every Go
// launch path answer from the same resolver instead of each carrying its own copy of the default.
func runAutonomy(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("autonomy", "usage: pfm autonomy [--path]", stderr)
	showPath := flags.Bool("path", false, "print the config file path instead of the posture")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// Prompted mode is the safe answer, but the reason must not vanish: a shell reading a bare
		// "off" cannot tell a deliberate setting from a broken lookup.
		fmt.Fprintf(stderr, "pfm autonomy: resolve home: %v\n", err)
		fmt.Fprintln(stdout, "off")
		return 1
	}
	if *showPath {
		fmt.Fprintln(stdout, policy.ConfigPath(home))
		return 0
	}
	if policy.Autonomy(home) {
		fmt.Fprintln(stdout, "on")
		return 0
	}
	fmt.Fprintln(stdout, "off")
	return 0
}
