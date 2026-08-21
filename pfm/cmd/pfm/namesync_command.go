package main

import (
	"context"
	"fmt"
	"io"

	fleetindex "hostops/pfm/internal/index"
	"hostops/pfm/internal/store"
)

// runNameSync converges every live chat's tmux WINDOW name — the fleet's DNS
// record. chat.sh resolves codex chats by it, the terminal tab renders it, and
// a person picking a pane reads it.
//
// A codex window follows its thread's indexed name; a claude window follows
// the 🔖 label its own statusline renders. Both halves are computed and
// applied by the same gather pass the picker runs, so there is exactly ONE
// writer of a window name however this command is reached — a systemd path
// unit on a codex rename, a timer, or a picker refresh.
func runNameSync(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	flags := newFlagSet("name-sync", "usage: pfm name-sync [--dry-run]", stderr)
	dryRun := flags.Bool("dry-run", false, "report the renames without applying them")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}
	defer database.Close()
	ctx := context.Background()

	// A delta index first: a codex rename lands in session_index.jsonl or the
	// thread store, and the name a window converges on is read from the index.
	// Without this pass the sync would converge yesterday's names.
	indexer, err := fleetindex.NewWithCodexRoots(database, runtime.Paths, codexHomes(runtime.Config))
	if err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}
	if _, err := indexer.Run(ctx, fleetindex.Options{}); err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}

	environment, err := resolveScanEnvironment(scanRequest{Runtime: &runtime})
	if err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}
	data, err := loadFleetData(ctx, database)
	if err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}
	// ReadOnly is what makes --dry-run a dry run: the gather pass applies the
	// renames it plans, and only a read-only pass plans without applying.
	live, err := gatherFleet(
		ctx,
		environment.paths,
		environment.config,
		data,
		*dryRun,
		printWarn(stderr),
		stderr,
	)
	if err != nil {
		fmt.Fprintf(stderr, "pfm name-sync: %v\n", err)
		return 1
	}
	if !*dryRun {
		rememberCodexPaneBindings(ctx, database, live, runtime, stderr)
	}
	verb := "renamed"
	if *dryRun {
		verb = "would rename"
	}
	for _, rename := range live.Renames {
		fmt.Fprintf(
			stdout,
			"%s %s %s: %s -> %s\n",
			verb,
			rename.Socket,
			rename.WindowID,
			rename.CurrentName,
			rename.TargetName,
		)
	}
	fmt.Fprintf(stdout, "windows converged: %d\n", len(live.Renames))
	return 0
}
