package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"hostops/pfm/internal/reap"
)

// runReap sweeps the chat socket graveyard: the tmux servers (and their
// ~0.5-1 GB chat processes) that outlive the terminal tabs they were opened
// in, plus the socket files crashed servers leave behind.
//
// The default is a DRY RUN. A wrongly kept socket costs memory; a wrongly
// killed one costs a chat nobody can get back, so killing has to be asked for.
func runReap(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	flags := newFlagSet(
		"reap",
		"usage: pfm reap [--apply] [--busy-recent SECONDS]",
		stderr,
	)
	apply := flags.Bool("apply", false, "kill unattached orphans and remove dead socket files")
	busyRecent := flags.Int(
		"busy-recent",
		60,
		"seconds of transcript writes that count as a working chat",
	)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *busyRecent < 0 {
		flags.Usage()
		return 2
	}

	resolved := runtime.Paths
	configDirs := make([]string, 0, len(runtime.Config.Accounts))
	for _, account := range runtime.Config.Accounts {
		configDirs = append(configDirs, account.ConfigDir)
	}
	ctx := context.Background()
	runner, err := reap.New(reap.Dependencies{
		Paths:        resolved,
		Busy:         reap.NewClaudeAgentsConfigured(resolved, runtime.Config.Claude.Binary, configDirs),
		ClaudeBinary: runtime.Config.Claude.Binary,
		CodexBinary:  runtime.Config.Codex.Binary,
		KillServer: func(ctx context.Context, socket string) error {
			return killChatServer(ctx, resolved, socket)
		},
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm reap: %v\n", err)
		return 1
	}
	report, err := runner.Run(ctx, reap.Options{
		Apply:      *apply,
		BusyRecent: time.Duration(*busyRecent) * time.Second,
		Self:       currentSocket(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm reap: %v\n", err)
		return 1
	}
	printReapReport(report, *apply, stdout, stderr)
	return 0
}

func printReapReport(
	report reap.Report,
	apply bool,
	stdout, stderr io.Writer,
) {
	if !report.AgentsOK {
		fmt.Fprintf(
			stderr,
			"pfm reap: busy state unknown (%s) — every chat with a breadcrumb is skipped\n",
			report.BusyError,
		)
	}
	fmt.Fprintf(stdout, "%-38s %-6s %8s  %s\n", "SOCKET", "STATE", "RAM(MB)", "LABEL [cwd]")
	for _, decision := range report.Decisions {
		detail := decision.Label
		if decision.CWD != "" {
			detail = fmt.Sprintf("%s [%s]", detail, decision.CWD)
		}
		if decision.Reason != "" {
			detail = fmt.Sprintf("%s — %s", decision.Reason, detail)
		}
		fmt.Fprintf(
			stdout,
			"%-38s %-6s %8d  %s\n",
			decision.Socket,
			decision.State,
			decision.RSSKB/1024,
			detail,
		)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(
		stdout,
		"KEEP: %d (~%d MB)   ORPHAN: %d (~%d MB summed RSS)   dead files: %d\n",
		report.Keep,
		report.KeepKB/1024,
		report.Orphans,
		report.OrphanKB/1024,
		report.DeadFiles,
	)
	if !apply {
		fmt.Fprintln(
			stdout,
			"dry run — nothing changed. Re-run with --apply to reap.",
		)
		return
	}
	fmt.Fprintf(
		stdout,
		"reaped %d socket(s), ~%d MB summed RSS\n",
		report.Killed,
		report.FreedKB/1024,
	)
	if report.AvailBefore > 0 || report.AvailAfter > 0 {
		fmt.Fprintf(
			stdout,
			"memory available: %d MB before, %d MB after (the honest reclaim)\n",
			report.AvailBefore/1024,
			report.AvailAfter/1024,
		)
	}
}
