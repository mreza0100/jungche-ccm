package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"hostops/pfm/internal/compose"
	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/hide"
	fleetindex "hostops/pfm/internal/index"
	"hostops/pfm/internal/naming"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/shared"
	"hostops/pfm/internal/store"
	"hostops/pfm/internal/ui"
)

const (
	testFreshSocketEnv = "PFM_TEST_FRESH_SOCKET"
	testNowNSEnv       = "PFM_TEST_NOW_NS"
	codexAvailableEnv  = "PFM_CODEX_AVAILABLE"
	// fleetRefreshInterval is the cadence while somebody is driving the picker.
	// One pass is expensive — a tmux fork+exec per live socket plus a whole
	// store read, ~2.6 CPU-seconds on a busy box — so paying it on a fixed
	// clock forever is what let an abandoned picker hold half a core.
	fleetRefreshInterval = 5 * time.Second
	// fleetRefreshGrowth stretches the interval by 10% after every pass nobody
	// interrupted. The decay is gentle where it is felt (5s → 5.5s → 6.05s, all
	// still a live list) and steep where it pays (roughly 49 minutes untouched
	// to reach the ceiling below).
	fleetRefreshGrowth = 1.1
	// fleetRefreshMaxInterval bounds the decay. Unbounded, 1.1^n reaches a
	// refresh a day inside a shift, and a picker that has quietly stopped
	// refreshing looks exactly like a fleet where nothing is happening — the
	// screen would assert a truth it stopped checking. Five minutes keeps an
	// abandoned picker under 1% of a core while still bounding how stale the
	// frame in front of you can be.
	fleetRefreshMaxInterval = 5 * time.Minute
)

// refreshCadence is one refresh stream's backoff state. It starts at
// fleetRefreshInterval and stretches by fleetRefreshGrowth after each pass
// that nobody interrupted, so a picker being driven stays prompt and an
// abandoned one decays toward costing nothing.
type refreshCadence struct {
	activity  *ui.ActivityClock
	lastStamp int64
	interval  time.Duration
}

func newRefreshCadence(activity *ui.ActivityClock) *refreshCadence {
	return &refreshCadence{
		activity:  activity,
		lastStamp: activity.StampNS(),
		interval:  fleetRefreshInterval,
	}
}

// next reports how long to wait before the next pass. Any interaction since
// the previous call snaps the cadence back to fleetRefreshInterval; otherwise
// it grows, capped at fleetRefreshMaxInterval.
//
// A nil clock — every non-interactive caller — never backs off. An absent
// presence signal is a claim about US, not about the user, and must never be
// spent as evidence that nobody is there.
func (cadence *refreshCadence) next() time.Duration {
	if cadence.activity == nil {
		return fleetRefreshInterval
	}
	if stamp := cadence.activity.StampNS(); stamp != cadence.lastStamp {
		cadence.lastStamp = stamp
		cadence.interval = fleetRefreshInterval
		return cadence.interval
	}
	grown := time.Duration(float64(cadence.interval) * fleetRefreshGrowth)
	if grown > fleetRefreshMaxInterval {
		grown = fleetRefreshMaxInterval
	}
	cadence.interval = grown
	return cadence.interval
}

// gatherWarn reports one tmux probe warning raised during a gather pass.
// scanFleet's callers — plain, tsv, check, and every one-shot command — print
// immediately through printWarn; the interactive picker instead buffers
// through bufferedWarnings, because Bubble Tea owns the tty for as long as it
// runs and a warning written straight to stderr mid-refresh lands on top of
// its alt-screen frame.
type gatherWarn func(warning string)

// printWarn reports a warning immediately, matching every non-interactive
// caller's existing behavior.
func printWarn(stderr io.Writer) gatherWarn {
	return func(warning string) {
		fmt.Fprintf(stderr, "pfm: tmux probe warning: %s\n", warning)
	}
}

// bufferedWarnings collects gather warnings raised from the background
// refresh goroutine while an interactive picker owns the terminal (runLS),
// releasing them to stderr only once flush is called after Pick returns.
type bufferedWarnings struct {
	mu       sync.Mutex
	warnings []string
}

func (buffer *bufferedWarnings) add(warning string) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	buffer.warnings = append(buffer.warnings, warning)
}

// flush prints every warning collected so far and clears the buffer, so a
// caller that flushes between picker frames — the reload loop in runLS — never
// prints the same warning twice.
func (buffer *bufferedWarnings) flush(stderr io.Writer) {
	buffer.mu.Lock()
	pending := buffer.warnings
	buffer.warnings = nil
	buffer.mu.Unlock()
	for _, warning := range pending {
		fmt.Fprintf(stderr, "pfm: tmux probe warning: %s\n", warning)
	}
}

type scanRequest struct {
	View      compose.View
	Rotation  int
	Query     string
	ReadOnly  bool
	Cache1H   bool
	ForceFull bool
	NoSky     bool
	Runtime   *commandRuntime
}

type scanResult struct {
	Output   compose.Output
	Snapshot ui.Snapshot
	Live     gather.Snapshot
	Counters fleetindex.Counters
	Paths    paths.Values
}

type fleetData struct {
	transcripts  []store.Transcript
	rollouts     []store.Rollout
	cxNames      map[string]string
	hidden       []store.Hidden
	cachedCounts *store.CachedCounts
}

type scanEnvironment struct {
	paths      paths.Values
	currentDir string
	nowNS      int64
	primary    int
	config     pfmconfig.Config
}

type indexRunner interface {
	Run(context.Context, fleetindex.Options) (fleetindex.Counters, error)
}

type refreshDependencies struct {
	newIndexer func(*store.Store) (indexRunner, error)
	// activity is the picker's presence clock. Nil — every non-interactive
	// caller and every existing stream test — reads as permanently active and
	// holds the loop at fleetRefreshInterval, exactly as before the backoff.
	activity *ui.ActivityClock
}

func scanFleet(
	ctx context.Context,
	database *store.Store,
	request scanRequest,
	stderr io.Writer,
) (scanResult, error) {
	environment, err := resolveScanEnvironment(request)
	if err != nil {
		return scanResult{}, err
	}
	indexer, err := fleetindex.NewWithPaths(database, environment.paths)
	if err != nil {
		return scanResult{}, err
	}
	counters, err := indexer.Run(ctx, fleetindex.Options{
		PriorityCWD: environment.currentDir,
	})
	if err != nil {
		return scanResult{}, err
	}
	data, err := loadFleetData(ctx, database)
	if err != nil {
		return scanResult{}, err
	}
	live, err := gatherFleet(
		ctx,
		environment.paths,
		environment.config,
		data,
		request.ReadOnly,
		printWarn(stderr),
		stderr,
	)
	if err != nil {
		return scanResult{}, err
	}
	result := composeFleet(environment, request, data, live)
	result.Counters = counters
	result.Live = live
	return result, nil
}

// resolveRowEngine looks id up in a compose pass over CURRENT database state
// plus a live gather — the picker's own source of truth for what exists right
// now — and reports the engine and rollout path of the row that carries it.
// It finds exactly the ids the picker displays, including a live agent row
// and a live Codex pane the index has not caught up with; an id nothing
// composes returns "", "", which leaves an ordinary hide free to refuse it as
// unindexed. Errors from the pass itself are swallowed the same way: a
// failed vouch attempt falls through to that same refusal rather than
// replacing the hide's own error.
//
// The rollout path lets hide.Manager resolve an UNINDEXED Codex lineage
// member to its root through the file's own session_meta header
// (resolveUnindexedCodexParent) instead of hiding under the member's own id
// — the id compose never carries once a full lineage IS indexed, since a
// Codex row is always keyed on its lineage root, never a member.
//
// This deliberately skips the indexer scanFleet runs: a caller resolving one
// id for a hide has no business reconciling the whole filesystem index, and
// a delta run can prune a transcript row whose file is not there YET — the
// exact row a hide right after spawning a chat is racing to catch.
func resolveRowEngine(
	ctx context.Context,
	database *store.Store,
	id string,
	stderr io.Writer,
	runtimes ...commandRuntime,
) (string, string) {
	request := scanRequest{View: compose.AllView}
	if len(runtimes) != 0 {
		request.Runtime = &runtimes[0]
	}
	environment, err := resolveScanEnvironment(request)
	if err != nil {
		return "", ""
	}
	data, err := loadFleetData(ctx, database)
	if err != nil {
		return "", ""
	}
	live, err := gatherFleet(ctx, environment.paths, environment.config, data, false, printWarn(stderr), stderr)
	if err != nil {
		return "", ""
	}
	result := composeFleet(environment, request, data, live)
	for _, row := range result.Output.Rows {
		if row.ID == id {
			return compose.EngineForKind(row.Kind), row.Path
		}
	}
	return "", ""
}

func scanFleetCached(
	ctx context.Context,
	database *store.Store,
	request scanRequest,
) (scanResult, error) {
	environment, err := resolveScanEnvironment(request)
	if err != nil {
		return scanResult{}, err
	}
	var data fleetData
	if request.View == compose.DefaultView {
		data, err = loadDefaultFleetData(ctx, database)
	} else {
		data, err = loadFleetData(ctx, database)
	}
	if err != nil {
		return scanResult{}, err
	}
	result := composeFleet(environment, request, data, gather.Snapshot{})
	result.Snapshot.Refreshing = true
	return result, nil
}

func resolveScanEnvironment(request scanRequest) (scanEnvironment, error) {
	var resolved paths.Values
	var machine pfmconfig.Config
	if request.Runtime != nil {
		resolved = request.Runtime.Paths
		machine = request.Runtime.Config
	} else {
		var err error
		resolved, err = paths.Resolve()
		if err != nil {
			return scanEnvironment{}, err
		}
		machine = pfmconfig.Defaults(resolved.Home, resolved.ClaudeRoots)
	}
	currentDir, err := os.Getwd()
	if err != nil {
		return scanEnvironment{}, fmt.Errorf("read current directory: %w", err)
	}
	nowNS := time.Now().UnixNano()
	if value := os.Getenv(testNowNSEnv); value != "" {
		parsed, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil {
			return scanEnvironment{}, fmt.Errorf("%s: %w", testNowNSEnv, parseErr)
		}
		nowNS = parsed
	}
	return scanEnvironment{
		paths:      resolved,
		currentDir: currentDir,
		nowNS:      nowNS,
		primary:    readPrimaryAccount(resolved, machine),
		config:     machine,
	}, nil
}

func loadFleetData(ctx context.Context, database *store.Store) (fleetData, error) {
	transcripts, err := database.Transcripts(ctx)
	if err != nil {
		return fleetData{}, err
	}
	rollouts, err := database.Rollouts(ctx)
	if err != nil {
		return fleetData{}, err
	}
	cxNames, err := database.CxNames(ctx)
	if err != nil {
		return fleetData{}, err
	}
	hidden, err := database.HiddenChats(ctx)
	if err != nil {
		return fleetData{}, err
	}
	return fleetData{
		transcripts: transcripts,
		rollouts:    rollouts,
		cxNames:     cxNames,
		hidden:      hidden,
	}, nil
}

func loadDefaultFleetData(
	ctx context.Context,
	database *store.Store,
) (fleetData, error) {
	transcripts, rollouts, counts, err := database.DefaultCandidates(
		ctx,
		30,
		15,
	)
	if err != nil {
		return fleetData{}, err
	}
	cxNames, err := database.CxNames(ctx)
	if err != nil {
		return fleetData{}, err
	}
	hidden, err := database.HiddenChats(ctx)
	if err != nil {
		return fleetData{}, err
	}
	return fleetData{
		transcripts:  transcripts,
		rollouts:     rollouts,
		cxNames:      cxNames,
		hidden:       hidden,
		cachedCounts: &counts,
	}, nil
}

func gatherFleet(
	ctx context.Context,
	resolved paths.Values,
	machine pfmconfig.Config,
	data fleetData,
	readOnly bool,
	warn gatherWarn,
	stderr io.Writer,
) (gather.Snapshot, error) {
	codexNamesByPath, codexNamesByID := naming.CodexNameIndex(
		store.CodexThreads(data.rollouts),
		data.cxNames,
	)
	tmuxClient := gather.CommandTmux{
		TmuxTmpDir: filepath.Dir(resolved.TmuxDir),
	}
	gatherer, err := gather.New(gather.Dependencies{
		Tmux:       tmuxClient,
		TmuxTmpDir: filepath.Dir(resolved.TmuxDir),
		CodexName: func(rolloutPath string) string {
			return codexNamesByPath[filepath.Clean(rolloutPath)]
		},
		CodexIDName: func(threadID string) string {
			return codexNamesByID[threadID]
		},
		CodexThread:  store.NewCodexThreadResolver(ctx, resolved.CodexRoot),
		ClaudeBinary: machine.Claude.Binary,
		CodexBinary:  machine.Codex.Binary,
		ReadOnly:     readOnly,
	})
	if err != nil {
		return gather.Snapshot{}, err
	}
	live, err := gatherer.Gather(ctx)
	if err != nil {
		return gather.Snapshot{}, err
	}
	for _, warning := range live.Warnings {
		warn(warning)
	}
	if !readOnly {
		for _, rename := range live.Renames {
			if err := tmuxClient.RenameWindow(ctx, rename); err != nil {
				fmt.Fprintf(stderr, "pfm: %v\n", err)
				continue
			}
			for index := range live.Panes {
				if live.Panes[index].Socket == rename.Socket &&
					live.Panes[index].WindowID == rename.WindowID {
					live.Panes[index].WindowName = rename.TargetName
				}
			}
		}
	}
	return live, nil
}

func composeFleet(
	environment scanEnvironment,
	request scanRequest,
	data fleetData,
	live gather.Snapshot,
) scanResult {
	output := compose.Compose(compose.Input{
		Snapshot:     live,
		Transcripts:  data.transcripts,
		Rollouts:     data.rollouts,
		CxNames:      data.cxNames,
		Hidden:       data.hidden,
		AccountRoots: accountRoots(environment.config.Accounts),
		Options: compose.Options{
			View:           request.View,
			CurrentDir:     environment.currentDir,
			CurrentSocket:  currentSocket(),
			PrimaryAccount: environment.primary,
			CodexAvailable: codexAvailable(environment.paths.CodexRoot, environment.config.Codex.Binary),
			Rotation:       request.Rotation,
			NowNS:          environment.nowNS,
		},
	})
	if data.cachedCounts != nil {
		output.HiddenCount = data.cachedCounts.Hidden
		output.SuppressedCount = data.cachedCounts.Suppressed
	}
	snapshot := ui.Snapshot{
		Rows:            output.Rows,
		View:            request.View,
		HiddenCount:     output.HiddenCount,
		SuppressedCount: output.SuppressedCount,
		PrimaryAccount:  environment.primary,
		AccountIDs:      environment.config.AccountIDs(),
		Cache1H:         request.Cache1H,
		Rotation:        request.Rotation,
		NowNS:           environment.nowNS,
		InitialQuery:    request.Query,
		NoSky:           request.NoSky,
	}
	return scanResult{
		Output:   output,
		Snapshot: snapshot,
		Paths:    environment.paths,
	}
}

func streamFleetRefreshes(
	ctx context.Context,
	database *store.Store,
	request scanRequest,
	warn gatherWarn,
	stderr io.Writer,
	updates chan<- ui.Snapshot,
	activity *ui.ActivityClock,
) {
	streamFleetRefreshesWith(
		ctx,
		database,
		request,
		warn,
		stderr,
		updates,
		refreshDependencies{activity: activity},
	)
}

func streamFleetRefreshesWith(
	ctx context.Context,
	database *store.Store,
	request scanRequest,
	warn gatherWarn,
	stderr io.Writer,
	updates chan<- ui.Snapshot,
	dependencies refreshDependencies,
) {
	defer close(updates)
	environment, err := resolveScanEnvironment(request)
	if err != nil {
		fmt.Fprintf(stderr, "pfm refresh: %v\n", err)
		return
	}
	var data fleetData
	if request.View == compose.DefaultView {
		data, err = loadDefaultFleetData(ctx, database)
	} else {
		data, err = loadFleetData(ctx, database)
	}
	if err != nil {
		fmt.Fprintf(stderr, "pfm refresh: %v\n", err)
		return
	}
	live, err := gatherFleet(
		ctx,
		environment.paths,
		environment.config,
		data,
		request.ReadOnly,
		warn,
		stderr,
	)
	if err != nil {
		fmt.Fprintf(stderr, "pfm refresh gather: %v\n", err)
		return
	}
	data, err = enrichLiveFleetData(ctx, database, data, live)
	if err != nil {
		fmt.Fprintf(stderr, "pfm refresh live cache: %v\n", err)
		return
	}
	if !sendRefresh(ctx, environment, request, data, live, true, updates) {
		return
	}
	if !request.ReadOnly {
		rememberCodexPaneBindings(ctx, database, live, commandRuntime{
			Config: environment.config,
			Paths:  environment.paths,
		}, stderr)
	}

	newIndexer := dependencies.newIndexer
	if newIndexer == nil {
		newIndexer = func(database *store.Store) (indexRunner, error) {
			return fleetindex.NewWithPaths(database, environment.paths)
		}
	}
	indexer, err := newIndexer(database)
	if err != nil {
		fmt.Fprintf(stderr, "pfm refresh index: %v\n", err)
		return
	}
	if !request.ForceFull {
		if _, err := indexer.Run(ctx, fleetindex.Options{
			PriorityCWD:  environment.currentDir,
			PriorityOnly: true,
		}); err != nil {
			fmt.Fprintf(stderr, "pfm refresh project index: %v\n", err)
			return
		}
		data, err = loadFleetData(ctx, database)
		if err != nil {
			fmt.Fprintf(stderr, "pfm refresh: %v\n", err)
			return
		}
		if !sendRefresh(ctx, environment, request, data, live, true, updates) {
			return
		}
	}

	if request.ForceFull {
		if _, err := indexer.Run(ctx, fleetindex.Options{
			Full:        true,
			PriorityCWD: environment.currentDir,
		}); err != nil {
			fmt.Fprintf(stderr, "pfm refresh full index: %v\n", err)
			return
		}
	}
	data, err = loadFleetData(ctx, database)
	if err != nil {
		fmt.Fprintf(stderr, "pfm refresh: %v\n", err)
		return
	}
	request.ForceFull = false
	result := composeFleet(environment, request, data, live)
	result.Snapshot.Refreshing = false
	select {
	case updates <- result.Snapshot:
	case <-ctx.Done():
		return
	}

	cadence := newRefreshCadence(dependencies.activity)
	timer := time.NewTimer(cadence.interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		// Rearm BEFORE the pass, never after it. The body below leaves through
		// several `continue`s on transient errors, and a Reset parked at the
		// bottom would be skipped by every one of them — the stream would go
		// permanently silent on the first gather hiccup, which reads on screen
		// as a fleet that simply stopped changing.
		timer.Reset(cadence.next())
		environment, err = resolveScanEnvironment(request)
		if err != nil {
			fmt.Fprintf(stderr, "pfm refresh: %v\n", err)
			continue
		}
		if request.View == compose.DefaultView {
			data, err = loadDefaultFleetData(ctx, database)
		} else {
			data, err = loadFleetData(ctx, database)
		}
		if err != nil {
			fmt.Fprintf(stderr, "pfm refresh: %v\n", err)
			continue
		}
		live, err = gatherFleet(
			ctx,
			environment.paths,
			environment.config,
			data,
			request.ReadOnly,
			warn,
			stderr,
		)
		if err != nil {
			fmt.Fprintf(stderr, "pfm refresh gather: %v\n", err)
			continue
		}
		data, err = enrichLiveFleetData(ctx, database, data, live)
		if err != nil {
			fmt.Fprintf(stderr, "pfm refresh live cache: %v\n", err)
			continue
		}
		if !request.ReadOnly {
			rememberCodexPaneBindings(ctx, database, live, commandRuntime{
				Config: environment.config,
				Paths:  environment.paths,
			}, stderr)
		}
		if !sendRefresh(ctx, environment, request, data, live, true, updates) {
			return
		}
		if _, err = indexer.Run(ctx, fleetindex.Options{
			PriorityCWD:  environment.currentDir,
			PriorityOnly: true,
		}); err != nil {
			fmt.Fprintf(stderr, "pfm refresh index: %v\n", err)
			continue
		}
		data, err = loadFleetData(ctx, database)
		if err != nil {
			fmt.Fprintf(stderr, "pfm refresh: %v\n", err)
			continue
		}
		if !sendRefresh(ctx, environment, request, data, live, false, updates) {
			return
		}
	}
}

func rememberCodexPaneBindings(
	ctx context.Context,
	database *store.Store,
	live gather.Snapshot,
	runtime commandRuntime,
	stderr io.Writer,
) {
	manager, err := hide.New(database, hideDependencies(runtime))
	if err != nil {
		fmt.Fprintf(stderr, "pfm refresh Codex clear binding: %v\n", err)
		return
	}
	for _, process := range live.Codex {
		threadID := process.ThreadID
		if threadID == "" {
			threadID = rolloutIDFromPath(process.RolloutPath)
		}
		if err := manager.SeedCodexPane(
			ctx,
			process.Socket,
			process.PaneID,
			threadID,
		); err != nil {
			fmt.Fprintf(stderr, "pfm refresh Codex clear binding for %s %s: %v\n", process.Socket, process.PaneID, err)
		}
	}
}

func enrichLiveFleetData(
	ctx context.Context,
	database *store.Store,
	data fleetData,
	live gather.Snapshot,
) (fleetData, error) {
	transcriptIDs := make(map[string]struct{}, len(data.transcripts))
	for _, transcript := range data.transcripts {
		transcriptIDs[transcript.UUID] = struct{}{}
	}
	wantedTranscripts := make(map[string]struct{})
	for _, crumb := range live.Crumbs {
		id := strings.TrimSuffix(
			filepath.Base(crumb.TranscriptPath),
			filepath.Ext(crumb.TranscriptPath),
		)
		if id != "" {
			wantedTranscripts[id] = struct{}{}
		}
	}
	for _, agent := range live.Agents {
		if agent.SessionID != "" {
			wantedTranscripts[agent.SessionID] = struct{}{}
		}
	}
	for id := range wantedTranscripts {
		if _, found := transcriptIDs[id]; found {
			continue
		}
		transcript, found, err := database.Transcript(ctx, id)
		if err != nil {
			return fleetData{}, err
		}
		if found {
			data.transcripts = append(data.transcripts, transcript)
			transcriptIDs[id] = struct{}{}
		}
	}

	rolloutIDs := make(map[string]struct{}, len(data.rollouts))
	for _, rollout := range data.rollouts {
		rolloutIDs[rollout.ID] = struct{}{}
	}
	for _, process := range live.Codex {
		id := rolloutIDFromPath(process.RolloutPath)
		if id == "" {
			continue
		}
		if _, found := rolloutIDs[id]; found {
			continue
		}
		family, err := database.RolloutLineage(ctx, id)
		if err != nil {
			return fleetData{}, err
		}
		for _, rollout := range family {
			if _, found := rolloutIDs[rollout.ID]; found {
				continue
			}
			data.rollouts = append(data.rollouts, rollout)
			rolloutIDs[id] = struct{}{}
			rolloutIDs[rollout.ID] = struct{}{}
		}
	}
	return data, nil
}

func rolloutIDFromPath(path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	rest := strings.TrimPrefix(stem, "rollout-")
	if len(rest) > 20 &&
		rest[4] == '-' &&
		rest[7] == '-' &&
		rest[10] == 'T' &&
		rest[13] == '-' &&
		rest[16] == '-' &&
		rest[19] == '-' {
		return rest[20:]
	}
	return rest
}

func sendRefresh(
	ctx context.Context,
	environment scanEnvironment,
	request scanRequest,
	data fleetData,
	live gather.Snapshot,
	refreshing bool,
	updates chan<- ui.Snapshot,
) bool {
	result := composeFleet(environment, request, data, live)
	result.Snapshot.Refreshing = refreshing
	select {
	case updates <- result.Snapshot:
		return true
	case <-ctx.Done():
		return false
	}
}

func accountRoots(accounts []pfmconfig.Account) []compose.AccountRoot {
	roots := make([]compose.AccountRoot, 0, len(accounts))
	for _, account := range accounts {
		path := account.ProjectDir
		if resolved, err := filepath.EvalSymlinks(account.ProjectDir); err == nil {
			path = resolved
		} else if absolute, err := filepath.Abs(account.ProjectDir); err == nil {
			path = absolute
		}
		roots = append(roots, compose.AccountRoot{
			Account: account.ID,
			Path:    filepath.Clean(path),
		})
	}
	return roots
}

// readPrimaryAccount resolves the shared database's meta row first, then the
// ~/.claude-primary mirror, and maps anything off the roster to the first
// configured account.
//
// Reading the mirror alone is how the picker came up showing a different
// account from the one the launchers used: primary-set writes both, but a
// database restored without the file, or a file left behind by a rollback,
// makes them disagree, and only one of the two is authoritative.
func readPrimaryAccount(
	values paths.Values,
	configs ...pfmconfig.Config,
) int {
	machine := pfmconfig.Defaults(values.Home, values.ClaudeRoots)
	if len(configs) != 0 {
		machine = configs[0]
	}
	account, found := shared.PrimaryAccount(context.Background(), values)
	if found {
		if _, exists := machine.Account(account); exists {
			return account
		}
	}
	if len(machine.Accounts) != 0 {
		return machine.Accounts[0].ID
	}
	return 1
}

// writePrimaryAccount validates the operator-facing roster before committing
// the shared state row and statusline mirror as one reported operation.
func writePrimaryAccount(
	values paths.Values,
	machine pfmconfig.Config,
	account int,
) error {
	if _, found := machine.Account(account); !found {
		return fmt.Errorf("primary account %d is not in the configured roster", account)
	}
	return shared.SetPrimaryAccount(
		context.Background(),
		values,
		account,
		time.Now().Unix(),
	)
}

// primaryWriteback decides whether an ls session's picker outcome is worth
// persisting. Zero (and anything non-positive) is never a real account — it
// is the zero value ui.Outcome carries before a picker has ever reported a
// deliberate choice — so it means "nothing to save", not "save account 0".
// Treating it as a real value sent it straight into writePrimaryAccount's
// roster check, which rejected it and aborted the whole `pfm ls` run before
// the picker's actual selection (or reload) ever executed. A cancelled
// picker (Esc/⌃C) never writes either: a ⌃S account switch is only a
// pending intent until the picker exits deliberately. An outcome that
// already matches the persisted primary has nothing new to write.
func primaryWriteback(kind ui.OutcomeKind, account, current int) (int, bool) {
	if kind == ui.OutcomeCancelled || account <= 0 || account == current {
		return 0, false
	}
	return account, true
}

func currentSocket() string {
	value := os.Getenv("TMUX")
	if comma := strings.IndexByte(value, ','); comma >= 0 {
		value = value[:comma]
	}
	if value == "" {
		return ""
	}
	return filepath.Base(value)
}

func inBunker() bool {
	return currentSocket() == "vsct"
}

func codexAvailable(codexRoot string, binaries ...string) bool {
	switch os.Getenv(codexAvailableEnv) {
	case "1":
		return true
	case "0":
		return false
	}
	binary := "codex"
	if len(binaries) != 0 && binaries[0] != "" {
		binary = binaries[0]
	}
	if _, err := exec.LookPath(binary); err == nil {
		return true
	}
	info, err := os.Stat(codexRoot)
	return err == nil && info.IsDir()
}
