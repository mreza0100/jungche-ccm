package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"hostops/cc-fleet/internal/resolve"
)

// runWhoami prints THIS chat's own tmux session name — its identity, and the
// address another chat injects to. The stdout contract is chat.sh's whoami
// (chat.sh:482-484): one bare session name and nothing else, so an existing
// caller can switch to this binary without reading differently. --json adds
// the engine identity for callers that want more than the handle.
func runWhoami(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("whoami", "usage: cc-fleet whoami [--json]", stderr)
	asJSON := flags.Bool("json", false, "print the full identity as JSON")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	identifier, err := resolve.NewWhoami(resolve.WhoamiDependencies{})
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet whoami: %v\n", err)
		return 1
	}
	identity, err := identifier.Identify(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *asJSON {
		encoded, err := json.Marshal(identity)
		if err != nil {
			fmt.Fprintf(stderr, "cc-fleet whoami: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", encoded)
		return 0
	}
	fmt.Fprintf(stdout, "%s\n", identity.Session)
	return 0
}
