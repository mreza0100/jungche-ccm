package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"hostops/pfm/internal/archive"
	"hostops/pfm/internal/hide"
	"hostops/pfm/internal/paths"
)

// hideStoreAdapter exposes the hide manager as the archive's hidden-chat
// source. The manager stays the ONE writer of a hide (shared store plus
// carrier file); archive asks it questions and asks it to retire rows, and
// never writes that pair itself.
type hideStoreAdapter struct {
	manager *hide.Manager
}

func (adapter hideStoreAdapter) Hidden(
	ctx context.Context,
) ([]archive.HiddenChat, error) {
	rows, err := adapter.manager.Hidden(ctx)
	if err != nil {
		return nil, err
	}
	chats := make([]archive.HiddenChat, 0, len(rows))
	for _, row := range rows {
		chats = append(chats, archive.HiddenChat{
			ID:     row.ID,
			Engine: row.Engine,
		})
	}
	return chats, nil
}

func (adapter hideStoreAdapter) Unhide(ctx context.Context, id string) error {
	return adapter.manager.Unhide(ctx, id)
}

// runArchive moves chats out of both engines' sight, reversibly.
//
// The default is a DRY RUN, and the whole design is reversibility: every move
// is recorded in the manifest, and --restore puts one back exactly where it
// came from. Nothing here deletes anything, ever.
func runArchive(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"archive",
		"usage: pfm archive [--apply] [--subagents [--older-than DAYS]] [--restore id]",
		stderr,
	)
	apply := flags.Bool("apply", false, "perform the moves instead of planning them")
	subagents := flags.Bool(
		"subagents",
		false,
		"archive sidechain transcripts instead of hidden chats",
	)
	olderThan := flags.Int(
		"older-than",
		2,
		"with --subagents, the age in days a transcript must reach",
	)
	restore := flags.String("restore", "", "put one archived chat back")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *olderThan < 0 ||
		(*restore != "" && (*apply || *subagents)) {
		flags.Usage()
		return 2
	}

	resolved, err := paths.Resolve()
	if err != nil {
		fmt.Fprintf(stderr, "pfm archive: %v\n", err)
		return 1
	}
	if *restore != "" {
		row, err := archive.Restore(resolved.ArchiveDir, *restore)
		if err != nil {
			fmt.Fprintf(stderr, "pfm archive: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "restored %s -> %s\n", row.ID, row.Original)
		return 0
	}

	database, manager, code := openHideManager(stderr)
	if code != 0 {
		return code
	}
	defer database.Close()
	runner, err := archive.New(archive.Dependencies{
		Paths: resolved,
		Hides: hideStoreAdapter{manager: manager},
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm archive: %v\n", err)
		return 1
	}
	report, err := runner.Run(context.Background(), archive.Options{
		Apply:     *apply,
		Subagents: *subagents,
		OlderThan: time.Duration(*olderThan) * 24 * time.Hour,
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm archive: %v\n", err)
		return 1
	}
	printArchiveReport(report, *apply, *subagents, resolved, stdout)
	return 0
}

func printArchiveReport(
	report archive.Report,
	apply, subagents bool,
	resolved paths.Values,
	stdout io.Writer,
) {
	mode := "hidden chats"
	if subagents {
		mode = "sidechain transcripts"
	}
	fmt.Fprintf(stdout, "mode: %s\n", mode)
	for _, move := range report.Moves {
		switch {
		case move.Failed != "":
			fmt.Fprintf(stdout, "  FAILED %s -> %s: %s\n", move.Source, move.Target, move.Failed)
		case apply:
			fmt.Fprintf(stdout, "  moved  %s -> %s\n", move.Source, move.Target)
		default:
			fmt.Fprintf(stdout, "  plan   %s -> %s\n", move.Source, move.Target)
		}
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(
		stdout,
		"archive: %d file(s), %s   live (skipped): %d   orphaned hides: %d\n",
		len(report.Moves),
		archive.FormatBytes(report.Bytes),
		len(report.Live),
		len(report.Orphans),
	)
	if subagents {
		fmt.Fprintf(stdout, "younger than the age gate (left alone): %d\n", report.Young)
	}
	if !apply {
		fmt.Fprintln(
			stdout,
			"dry run — nothing moved. Re-run with --apply.",
		)
		return
	}
	if len(report.SidecarBackups) > 0 {
		fmt.Fprintf(
			stdout,
			"sidecars backed up: %d file(s) under %s/_sidecar-backups\n",
			len(report.SidecarBackups),
			resolved.ArchiveDir,
		)
	}
	if !subagents {
		fmt.Fprintf(
			stdout,
			"hides retired: %d   history.jsonl lines dropped: %d   codex index rows dropped: %d\n",
			report.Unhidden,
			report.HistoryPruned,
			report.IndexPruned,
		)
	}
	fmt.Fprintf(stdout, "manifest: %s\n", archive.ManifestPath(resolved.ArchiveDir))
	fmt.Fprintln(stdout, "put one back with: pfm archive --restore <id>")
}
