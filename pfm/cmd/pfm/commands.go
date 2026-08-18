package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"hostops/pfm/internal/action"
	"hostops/pfm/internal/compose"
	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/heal"
	"hostops/pfm/internal/hide"
	fleetindex "hostops/pfm/internal/index"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/shared"
	pfmstats "hostops/pfm/internal/stats"
	"hostops/pfm/internal/store"
	"hostops/pfm/internal/ui"
)

func runLS(
	args []string,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) int {
	flags := newFlagSet(
		"ls",
		"usage: pfm ls [-a|--all] [--plain|--tsv] [id] | pfm ls --hidden [--tsv]",
		stderr,
	)
	var all bool
	var hidden bool
	flags.BoolVar(&all, "a", false, "include hidden, background, and uncapped rows")
	flags.BoolVar(&all, "all", false, "include hidden, background, and uncapped rows")
	flags.BoolVar(&hidden, "H", false, "list the hidden ledger")
	flags.BoolVar(&hidden, "hidden", false, "list the hidden ledger")
	plain := flags.Bool("plain", false, "render a noninteractive list")
	tsv := flags.Bool("tsv", false, "render stable tab-separated rows")
	noSky := flags.Bool("no-sky", false, "disable the interactive sky widget")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() > 1 ||
		boolCount(*plain, *tsv) > 1 {
		flags.Usage()
		return 2
	}
	if hidden {
		if flags.NArg() != 0 || all || *plain {
			flags.Usage()
			return 2
		}
		// The hidden ledger is already a stable three-column TSV contract.
		// Accepting --tsv makes that format explicit for scripts instead of
		// returning an empty success or rejecting a harmless format request.
		return runHidden(nil, stdout, stderr, runtime)
	}
	if flags.NArg() == 1 {
		if all || *plain || *tsv {
			flags.Usage()
			return 2
		}
		return openID(context.Background(), flags.Arg(0), stdout, stderr, runtime)
	}

	view := compose.DefaultView
	if all {
		view = compose.AllView
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "pfm ls: %v\n", err)
		return 1
	}
	defer database.Close()

	ctx := context.Background()
	rotation := 0
	query := ""
	cache1H := initialCache1H()
	forceFull := false
	for {
		request := scanRequest{
			View:      view,
			Rotation:  rotation,
			Query:     query,
			Cache1H:   cache1H,
			ForceFull: forceFull,
			NoSky:     *noSky,
			Runtime:   &runtime,
		}
		var scan scanResult
		var outcome ui.Outcome
		if *plain || *tsv {
			scan, err = scanFleet(ctx, database, request, stderr)
			if err != nil {
				fmt.Fprintf(stderr, "pfm ls: %v\n", err)
				return 1
			}
			var picker ui.Picker
			if *plain {
				picker = ui.PlainPicker{Writer: stdout}
			} else {
				picker = ui.TSVPicker{Writer: stdout}
			}
			outcome, err = picker.Pick(ctx, scan.Snapshot)
		} else {
			scan, err = scanFleetCached(ctx, database, request)
			if err != nil {
				fmt.Fprintf(stderr, "pfm ls: %v\n", err)
				return 1
			}
			applier, err := hideApplier(ctx, database, runtime)
			if err != nil {
				fmt.Fprintf(stderr, "pfm ls: %v\n", err)
				return 1
			}
			scan.Snapshot.ApplyHide = applier
			scan.Snapshot.StatsSampler = pfmstats.NewSampler(
				scan.Paths.ProcRoot,
				scan.Paths.CgroupRoot,
			)
			refreshContext, refreshCancel := context.WithCancel(ctx)
			updates := make(chan ui.Snapshot, 1)
			// Bubble Tea owns the tty for as long as Pick runs: a probe warning
			// the background refresh raises mid-frame corrupts the alt-screen,
			// so it is buffered here and flushed only once Pick has released
			// the terminal.
			var warnings bufferedWarnings
			go streamFleetRefreshes(
				refreshContext,
				database,
				request,
				warnings.add,
				stderr,
				updates,
			)
			outcome, err = (ui.BubblePicker{Updates: updates}).Pick(
				ctx,
				scan.Snapshot,
			)
			refreshCancel()
			warnings.flush(stderr)
		}
		if err != nil {
			fmt.Fprintf(stderr, "pfm ls: %v\n", err)
			return 1
		}
		if *plain || *tsv {
			return 0
		}
		// Every ⌃X already landed the moment it was typed, so there is nothing
		// here to apply and no exit key that can lose one. What remains is the
		// receipt, printed once the picker has released the terminal.
		reportHides(outcome.HideChanges, stderr)
		// A ⌃S account switch IS still a pending intent, and backing out with
		// Esc/⌃C must not write it. Nor does a non-positive PrimaryAccount ever
		// mean a deliberate choice — see primaryWriteback.
		if account, should := primaryWriteback(
			outcome.Kind,
			outcome.PrimaryAccount,
			readPrimaryAccount(scan.Paths, runtime.Config),
		); should {
			if err := writePrimaryAccount(scan.Paths, runtime.Config, account); err != nil {
				fmt.Fprintf(stderr, "pfm ls: save primary account: %v\n", err)
				return 1
			}
		}
		rotation = outcome.Rotation
		query = outcome.Query
		cache1H = outcome.Cache1H
		switch outcome.Kind {
		case ui.OutcomeReload:
			forceFull = true
			continue
		case ui.OutcomeReboot:
			row, err := rebootRow(ctx, scan.Paths, outcome.Row, stderr)
			if err != nil {
				fmt.Fprintf(stderr, "pfm ls: %v\n", err)
				return 1
			}
			return openRow(ctx, row, outcome.PrimaryAccount, cache1H, stdout, stderr, runtime)
		case ui.OutcomeSelected:
			return openRow(
				ctx,
				outcome.Row,
				outcome.PrimaryAccount,
				cache1H,
				stdout,
				stderr,
				runtime,
			)
		case ui.OutcomeCancelled, ui.OutcomeNone:
			return 0
		default:
			fmt.Fprintf(stderr, "pfm ls: unsupported picker outcome %d\n", outcome.Kind)
			return 1
		}
	}
}

func boolCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

func openID(
	ctx context.Context,
	id string,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) int {
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat open: %v\n", err)
		return 1
	}
	defer database.Close()
	scan, err := scanFleet(ctx, database, scanRequest{View: compose.AllView, Runtime: &runtime}, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat open: %v\n", err)
		return 1
	}
	for _, row := range scan.Output.Rows {
		if row.ID == id {
			return openRow(
				ctx,
				row,
				readPrimaryAccount(scan.Paths, runtime.Config),
				initialCache1H(),
				stdout,
				stderr,
				runtime,
			)
		}
	}
	fmt.Fprintf(stderr, "pfm chat open: chat %q is not indexed\n", id)
	return 1
}

func openRow(
	ctx context.Context,
	row compose.Row,
	primary int,
	cache1H bool,
	stdout, stderr io.Writer,
	runtime commandRuntime,
) int {
	resolved := runtime.Paths
	if row.Kind != compose.LiveClaude &&
		row.Kind != compose.LiveCodex &&
		row.Kind != compose.LiveSplit {
		if info, statErr := os.Stat(row.CWD); statErr != nil || !info.IsDir() {
			if currentDir, cwdErr := os.Getwd(); cwdErr == nil {
				row.CWD = currentDir
			}
		}
	}
	// The Codex projection repair rides the resume path itself: a wedged
	// thread is repaired in the same breath that opens it, with no shell
	// helper in the run string to be missing, unexecutable, or stale.
	executor, err := action.New(action.Dependencies{
		Stderr: stderr,
		Heal: func(ctx context.Context, threadID string) string {
			return heal.Thread(ctx, resolved.CodexRoot, threadID)
		},
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat open: %v\n", err)
		return 1
	}
	line, err := executor.Open(ctx, action.Request{
		Row:            row,
		PrimaryAccount: primary,
		Cache1H:        cache1H,
		Bunker:         inBunker(),
		Home:           resolved.Home,
		FreshSocket:    freshSocket(row.Kind),
		CurrentTMUX:    os.Getenv("TMUX"),
		Config:         runtime.Config,
	})
	if err != nil {
		fmt.Fprintf(stderr, "pfm chat open: %v\n", err)
		return 1
	}
	if line != "" {
		if err := dispatchAction(stdout, line); err != nil {
			fmt.Fprintf(stderr, "pfm chat open: execute action: %v\n", err)
			return 1
		}
	}
	return 0
}

func freshSocket(kind compose.Kind) string {
	if value := os.Getenv(testFreshSocketEnv); value != "" {
		return value
	}
	prefix := "cc"
	if kind == compose.ResumeCodex || kind == compose.NewCodex {
		prefix = "cx"
	}
	var randomBytes [2]byte
	_, _ = rand.Read(randomBytes[:])
	return fmt.Sprintf(
		"%s-%d-%d-%d",
		prefix,
		time.Now().Unix(),
		os.Getpid(),
		binary.BigEndian.Uint16(randomBytes[:]),
	)
}

func initialCache1H() bool {
	if os.Getenv("CC_ARM_1H") == "1" {
		return true
	}
	return os.Getenv("ENABLE_PROMPT_CACHING_1H") == "1" &&
		os.Getenv("CLAUDECODE") == ""
}

// reportHides is the receipt for what ⌃X did while the picker was open. A
// hide that ENDED a running chat is worth saying out loud — it is the one
// picker keystroke that destroys something.
func reportHides(changes []ui.HideChange, stderr io.Writer) {
	hidden, killed := 0, 0
	for _, change := range changes {
		if !change.Hidden {
			continue
		}
		hidden++
		if change.Live && change.Socket != "" {
			killed++
			fmt.Fprintf(stderr, "pfm ls: ended %s (%s)\n", change.Name, change.Socket)
		}
	}
	if killed > 0 {
		fmt.Fprintf(stderr, "pfm ls: hid %d, ended %d\n", hidden, killed)
	}
}

// hideApplier performs a picker ⌃X the instant it is typed: the store write,
// and the kill when the row is live. Hiding a running chat ENDS it — a chat
// that has left the list is a chat nobody can reach to stop.
//
// It reports failure by returning it, never by writing to stderr: Bubble Tea
// owns the terminal for as long as the picker is open.
func hideApplier(
	ctx context.Context,
	database *store.Store,
	runtime commandRuntime,
) (func(ui.HideChange) error, error) {
	manager, err := hide.New(database, hideDependencies(runtime))
	if err != nil {
		return nil, err
	}
	return func(change ui.HideChange) error {
		if !change.Hidden {
			return manager.Unhide(ctx, change.ID)
		}
		// The picker was showing the row, so it vouches for the engine: a live
		// agent whose transcript the index has not seen yet still hides.
		if _, err := manager.Hide(ctx, hide.Request{
			ID:     change.ID,
			Engine: change.Engine,
		}); err != nil {
			return err
		}
		if change.Live && change.Socket != "" {
			return killChatServer(ctx, runtime.Paths, change.Socket)
		}
		return nil
	}, nil
}

func hideDependencies(runtime commandRuntime) hide.Dependencies {
	return hide.Dependencies{
		Paths:       runtime.Paths,
		ClaudeRoots: append([]string(nil), runtime.Paths.ClaudeRoots...),
		ConfigPath:  runtime.Config.Path,
	}
}

// killChatServer ends one chat's tmux server and removes every handle that
// would otherwise keep pointing at it — the socket file, and the sid crumbs
// that resolve a chat to its server. It is the single kill sequence: ⌃O
// reboots a chat through it, ⌃X ends one through it.
func killChatServer(
	ctx context.Context,
	resolved paths.Values,
	socket string,
) error {
	tmux := action.CommandTmux{TmuxDir: resolved.TmuxDir}
	if err := tmux.KillServer(ctx, socket); err != nil {
		// Killing a corpse fails loudly for no reason — a socket file outlives
		// its server. The goal is "not running", so ask whether it is rather
		// than reporting a failure the user cannot act on.
		if tmux.SocketAlive(ctx, socket) {
			return err
		}
	}
	_ = os.Remove(filepath.Join(resolved.TmuxDir, socket))
	entries, _ := os.ReadDir(resolved.SIDDir)
	for _, entry := range entries {
		if name, _, ok := gather.ParseCrumbName(entry.Name()); ok &&
			name == socket {
			_ = os.Remove(filepath.Join(resolved.SIDDir, entry.Name()))
		}
	}
	state := shared.Open(ctx, resolved)
	clearErr := state.ClearBranchSeat(ctx, socket)
	closeErr := state.Close()
	if clearErr != nil || closeErr != nil {
		return fmt.Errorf(
			"clear branch marker after ending %s: %w",
			socket,
			errors.Join(clearErr, closeErr),
		)
	}
	return nil
}

func rebootRow(
	ctx context.Context,
	resolved paths.Values,
	row compose.Row,
	stderr io.Writer,
) (compose.Row, error) {
	if row.Kind == compose.LiveSplit || row.ID == "" {
		return compose.Row{}, errors.New("a split live row cannot be rebooted as one chat")
	}
	if err := killChatServer(ctx, resolved, row.Socket); err != nil {
		fmt.Fprintf(stderr, "pfm: reboot kill-server %s: %v\n", row.Socket, err)
	}
	if row.Kind == compose.LiveCodex {
		row.Kind = compose.ResumeCodex
	} else {
		row.Kind = compose.ResumeClaude
	}
	row.Socket = ""
	row.PaneID = ""
	row.SessionName = ""
	row.WindowName = ""
	fmt.Fprintf(stderr, "pfm: rebooting %s in a fresh server\n", row.ID)
	return row, nil
}

func runIndex(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	flags := newFlagSet(
		"index",
		"usage: pfm index [--full] [--progress]",
		stderr,
	)
	full := flags.Bool("full", false, "reparse every indexed file")
	progress := flags.Bool("progress", false, "report start and elapsed time to stderr")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "pfm index: %v\n", err)
		return 1
	}
	defer database.Close()
	indexer, err := fleetindex.NewWithPaths(database, runtime.Paths)
	if err != nil {
		fmt.Fprintf(stderr, "pfm index: %v\n", err)
		return 1
	}
	started := time.Now()
	if *progress {
		fmt.Fprintln(stderr, "pfm index: scanning")
	}
	counters, err := indexer.Run(context.Background(), fleetindex.Options{Full: *full})
	if err != nil {
		fmt.Fprintf(stderr, "pfm index: %v\n", err)
		return 1
	}
	if *full {
		_ = database.SetMeta(
			context.Background(),
			"last_full_index_at",
			strconv.FormatInt(time.Now().Unix(), 10),
		)
	}
	fmt.Fprintln(stdout, formatCounters(counters))
	if *progress {
		fmt.Fprintf(stderr, "pfm index: done in %s\n", time.Since(started).Round(time.Millisecond))
	}
	return 0
}

func formatCounters(counters fleetindex.Counters) string {
	return fmt.Sprintf(
		"files=%d skipped=%d delta=%d full=%d deleted=%d touched=%d bytes=%d cx_names=%t",
		counters.FilesSeen,
		counters.FilesSkipped,
		counters.DeltaParsed,
		counters.FullParsed,
		counters.Deleted,
		counters.RowsTouched,
		counters.BytesRead,
		counters.CxNamesReloaded,
	)
}

func runRevive(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	flags := newFlagSet("revive", "usage: pfm revive", stderr)
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	database, err := store.Open(store.WithWarningWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "pfm revive: %v\n", err)
		return 1
	}
	defer database.Close()
	scan, err := scanFleet(
		context.Background(),
		database,
		scanRequest{View: compose.AllView, Runtime: &runtime},
		stderr,
	)
	if err != nil {
		fmt.Fprintf(stderr, "pfm revive: %v\n", err)
		return 1
	}
	rows := make([]compose.Row, 0)
	for _, row := range scan.Output.Rows {
		if row.Kind == compose.ResumeClaude || row.Kind == compose.ResumeCodex {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "pfm revive: no resumable chats")
		return 0
	}
	snapshot := scan.Snapshot
	snapshot.Rows = rows
	snapshot.View = compose.AllView
	_, err = (ui.PlainPicker{Writer: stdout}).Pick(context.Background(), snapshot)
	if err != nil {
		fmt.Fprintf(stderr, "pfm revive: %v\n", err)
		return 1
	}
	return 0
}

// pruneOrphanedHides reports, and only with confirm deletes, the hides doctor
// counts as orphaned_hidden. A hide cannot be recovered once deleted, so the
// dry run is the default and the count is always printed.
func pruneOrphanedHides(
	ctx context.Context,
	database *store.Store,
	confirm bool,
	stdout, stderr io.Writer,
) int {
	orphans, err := database.OrphanedHides(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "pfm archive: %v\n", err)
		return 1
	}
	if !confirm {
		for _, orphan := range orphans {
			fmt.Fprintf(
				stdout,
				"would prune\t%s\t%s\t%d\n",
				orphan.ID,
				orphan.Engine,
				orphan.HiddenAt,
			)
		}
		fmt.Fprintf(
			stdout,
			"pfm archive: %d orphaned hide(s); re-run with --yes to delete\n",
			len(orphans),
		)
		return 0
	}
	deleted, err := database.DeleteOrphanedHides(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "pfm archive: %v\n", err)
		return 1
	}
	for _, orphan := range orphans {
		fmt.Fprintf(
			stdout,
			"pruned\t%s\t%s\t%d\n",
			orphan.ID,
			orphan.Engine,
			orphan.HiddenAt,
		)
	}
	fmt.Fprintf(
		stdout,
		"pfm archive: pruned %d orphaned hide(s)\n",
		deleted,
	)
	return 0
}
