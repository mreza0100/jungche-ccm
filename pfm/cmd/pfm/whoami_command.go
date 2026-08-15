package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"hostops/pfm/internal/compose"
	"hostops/pfm/internal/resolve"
	"hostops/pfm/internal/store"
)

// runWhoami prints THIS chat's own tmux session name — its identity, and the
// address another chat injects to. The stdout contract is chat.sh's whoami
// (chat.sh:482-484): one bare session name and nothing else, so an existing
// caller can switch to this binary without reading differently. --json adds
// the engine identity for callers that want more than the handle.
func runWhoami(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("whoami", "usage: pfm whoami [--json]", stderr)
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
		fmt.Fprintf(stderr, "pfm whoami: %v\n", err)
		return 1
	}
	ctx := context.Background()
	identity, err := identifier.Identify(ctx)
	if err != nil {
		seat, found := codexSeatIdentity(ctx)
		if !found {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		identity = seat
	}
	if *asJSON {
		encoded, err := json.Marshal(identity)
		if err != nil {
			fmt.Fprintf(stderr, "pfm whoami: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", encoded)
		return 0
	}
	fmt.Fprintf(stdout, "%s\n", identity.Session)
	return 0
}

// codexSeatIdentity answers for a codex seat whose turns run in `codex
// app-server` instead of in its own pane. That server is reparented to init and
// serves every seat from one process, so a tool shell it spawns has no tmux
// anywhere in its ancestry — the walk has nothing to find, and the seat's
// messages went out UNSIGNED though the seat itself is plainly addressable.
//
// The shell does carry CODEX_THREAD_ID, and the fleet already binds a thread to
// the socket hosting it: this is that lookup, and nothing more. It runs only
// after the tmux rungs fail, because CODEX_THREAD_ID is INHERITED — a process
// with a pane of its own must never be renamed by an id it merely inherited.
func codexSeatIdentity(ctx context.Context) (resolve.Identity, bool) {
	thread := os.Getenv(resolve.CodexThreadEnv)
	if thread == "" || os.Getenv(resolve.ClaudeSessionEnv) != "" {
		return resolve.Identity{}, false
	}
	database, err := store.Open(store.WithWarningWriter(io.Discard))
	if err != nil {
		return resolve.Identity{}, false
	}
	defer database.Close()
	scan, err := scanFleet(
		ctx,
		database,
		scanRequest{View: compose.AllView, ReadOnly: true},
		io.Discard,
	)
	if err != nil {
		return resolve.Identity{}, false
	}
	for _, row := range scan.Output.Rows {
		if row.ID != thread || row.Socket == "" {
			continue
		}
		// A seat with no live socket is no identity: better to say the sender
		// is underivable than to hand back a handle nobody can reply to.
		return resolve.Identity{
			Session:    row.Socket,
			SocketName: row.Socket,
			Engine:     resolve.CodexEngine,
			ID:         thread,
			Source:     "codex-thread",
			Recovered:  true,
		}, true
	}
	return resolve.Identity{}, false
}
