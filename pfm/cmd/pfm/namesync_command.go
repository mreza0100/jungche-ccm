package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"hostops/pfm/internal/gather"
	fleetindex "hostops/pfm/internal/index"
	"hostops/pfm/internal/inject"
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
	indexer, err := fleetindex.NewWithRoots(database, runtime.Paths, runtime.Paths.Roots)
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
		database,
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
		reconcileCodexPanes(ctx, database, live, runtime, printWarn(stderr))
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
	if *dryRun {
		// A dry run applied nothing, so it has nothing to verify. It reports
		// the PLAN, and says so — a plan counted as an outcome is exactly the
		// lie this command used to tell.
		fmt.Fprintf(stdout, "windows planned: %d\n", len(live.Renames))
		return 0
	}
	converged, unverified := verifyRenames(ctx, runtime, live.Renames, stderr)
	fmt.Fprintf(stdout, "windows converged: %d\n", converged)
	if unverified != 0 {
		fmt.Fprintf(stdout, "windows unverified: %d\n", unverified)
		return 1
	}
	return 0
}

// verifyRenames reads every renamed window's name BACK off its server and
// counts only a match as converged.
//
// An attempt is not an outcome. `windows converged: N` used to count the
// renames this pass planned, so a window whose name a second writer took back
// — or one whose server died between the plan and the rename — was reported as
// converged while the fleet still could not address it by that name. Each
// unverified window is named with the value actually read, and the command
// exits non-zero so a scheduler run that achieved nothing is not silent.
func verifyRenames(
	ctx context.Context,
	runtime commandRuntime,
	renames []gather.WindowRename,
	stderr io.Writer,
) (converged, unverified int) {
	reader := inject.CommandTmux{}
	for _, rename := range renames {
		socketPath := filepath.Join(runtime.Paths.TmuxDir, rename.Socket)
		actual, err := reader.WindowName(ctx, socketPath, rename.WindowID)
		if err != nil {
			unverified++
			fmt.Fprintf(
				stderr,
				"pfm name-sync: window %s %s: wanted %q, could not be read back after rename: %v\n",
				rename.Socket, rename.WindowID, rename.TargetName, err,
			)
			continue
		}
		if actual != rename.TargetName {
			unverified++
			fmt.Fprintf(
				stderr,
				"pfm name-sync: window %s %s: wanted %q, reads %q after rename\n",
				rename.Socket, rename.WindowID, rename.TargetName, actual,
			)
			continue
		}
		converged++
	}
	return converged, unverified
}
