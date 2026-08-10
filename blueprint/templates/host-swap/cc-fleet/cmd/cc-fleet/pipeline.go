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

	"hostops/cc-fleet/internal/compose"
	"hostops/cc-fleet/internal/gather"
	fleetindex "hostops/cc-fleet/internal/index"
	"hostops/cc-fleet/internal/naming"
	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/resolve"
	"hostops/cc-fleet/internal/store"
	"hostops/cc-fleet/internal/ui"
)

const (
	testFreshSocketEnv = "CC_FLEET_TEST_FRESH_SOCKET"
	testNowNSEnv       = "CC_FLEET_TEST_NOW_NS"
	codexAvailableEnv  = "CC_FLEET_CODEX_AVAILABLE"
)

type scanRequest struct {
	View      compose.View
	Rotation  int
	Query     string
	ReadOnly  bool
	Cache1H   bool
	ForceFull bool
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
}

type indexRunner interface {
	Run(context.Context, fleetindex.Options) (fleetindex.Counters, error)
}

type refreshDependencies struct {
	newIndexer func(*store.Store) (indexRunner, error)
}

func scanFleet(
	ctx context.Context,
	database *store.Store,
	request scanRequest,
	stderr io.Writer,
) (scanResult, error) {
	environment, err := resolveScanEnvironment()
	if err != nil {
		return scanResult{}, err
	}
	indexer, err := fleetindex.New(database)
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
		data,
		request.ReadOnly,
		stderr,
	)
	if err != nil {
		return scanResult{}, err
	}
	result, err := composeFleet(
		ctx,
		database,
		environment,
		request,
		data,
		live,
		!request.ReadOnly,
	)
	if err != nil {
		return scanResult{}, err
	}
	result.Counters = counters
	result.Live = live
	return result, nil
}

func scanFleetCached(
	ctx context.Context,
	database *store.Store,
	request scanRequest,
) (scanResult, error) {
	environment, err := resolveScanEnvironment()
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
	result, err := composeFleet(
		ctx,
		database,
		environment,
		request,
		data,
		gather.Snapshot{},
		false,
	)
	if err != nil {
		return scanResult{}, err
	}
	result.Snapshot.Refreshing = true
	return result, nil
}

func resolveScanEnvironment() (scanEnvironment, error) {
	resolved, err := paths.Resolve()
	if err != nil {
		return scanEnvironment{}, err
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
		primary:    readPrimaryAccount(resolved.Home),
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
	data fleetData,
	readOnly bool,
	stderr io.Writer,
) (gather.Snapshot, error) {
	codexNamesByPath := make(map[string]string, len(data.rollouts))
	for _, rollout := range data.rollouts {
		codexNamesByPath[filepath.Clean(rollout.Path)] = naming.CxName(
			rollout.ID,
			rollout.SessionID,
			rollout.ParentThread,
			data.cxNames,
			rollout.FirstPrompt,
		)
	}
	tmuxClient := gather.CommandTmux{
		TmuxTmpDir: filepath.Dir(resolved.TmuxDir),
	}
	gatherer, err := gather.New(gather.Dependencies{
		Tmux:       tmuxClient,
		TmuxTmpDir: filepath.Dir(resolved.TmuxDir),
		CodexName: func(rolloutPath string) string {
			return codexNamesByPath[filepath.Clean(rolloutPath)]
		},
		CodexThread: codexThreadResolver(ctx, resolved.CodexRoot),
		ReadOnly:    readOnly,
	})
	if err != nil {
		return gather.Snapshot{}, err
	}
	live, err := gatherer.Gather(ctx)
	if err != nil {
		return gather.Snapshot{}, err
	}
	for _, warning := range live.Warnings {
		fmt.Fprintf(stderr, "cc-fleet: tmux probe warning: %s\n", warning)
	}
	if !readOnly {
		for _, rename := range live.Renames {
			if err := tmuxClient.RenameWindow(ctx, rename); err != nil {
				fmt.Fprintf(stderr, "cc-fleet: %v\n", err)
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

// codexThreadResolver identifies a live Codex session that holds no rollout
// file descriptor. The state store is read on the first such process, so an
// ordinary scan never pays for the query.
func codexThreadResolver(
	ctx context.Context,
	codexRoot string,
) gather.CodexThreadResolver {
	candidates := sync.OnceValue(func() []resolve.CodexThread {
		files, err := store.CodexStateFiles(codexRoot)
		if err != nil {
			return nil
		}
		threads, err := store.ReadCodexThreads(ctx, files)
		if err != nil {
			return nil
		}
		rows := make([]resolve.CodexThread, 0, len(threads))
		for _, thread := range threads {
			if !thread.Listed() {
				continue
			}
			rows = append(rows, resolve.CodexThread{
				ID:          thread.ID,
				CWD:         thread.CWD,
				CreatedAt:   thread.CreatedAt,
				RolloutPath: thread.RolloutPath,
			})
		}
		return rows
	})
	return func(exported, cwd string, birth int64) (string, string) {
		thread, err := resolve.CodexThreadID(exported, cwd, birth, candidates())
		if err != nil {
			return "", ""
		}
		return thread.ID, thread.RolloutPath
	}
}

func composeFleet(
	ctx context.Context,
	database *store.Store,
	environment scanEnvironment,
	request scanRequest,
	data fleetData,
	live gather.Snapshot,
	applyIntents bool,
) (scanResult, error) {
	output := compose.Compose(compose.Input{
		Snapshot:     live,
		Transcripts:  data.transcripts,
		Rollouts:     data.rollouts,
		CxNames:      data.cxNames,
		Hidden:       data.hidden,
		AccountRoots: accountRoots(environment.paths.ClaudeRoots),
		Options: compose.Options{
			View:           request.View,
			CurrentDir:     environment.currentDir,
			CurrentSocket:  currentSocket(),
			PrimaryAccount: environment.primary,
			CodexAvailable: codexAvailable(environment.paths.CodexRoot),
			Rotation:       request.Rotation,
			NowNS:          environment.nowNS,
		},
	})
	if data.cachedCounts != nil {
		output.HiddenCount = data.cachedCounts.Hidden
		output.SuppressedCount = data.cachedCounts.Suppressed
	}
	if applyIntents {
		if err := applyComposeIntents(ctx, database, data.hidden, output); err != nil {
			return scanResult{}, err
		}
	}
	snapshot := ui.Snapshot{
		Rows:            output.Rows,
		View:            request.View,
		HiddenCount:     output.HiddenCount,
		SuppressedCount: output.SuppressedCount,
		PrimaryAccount:  environment.primary,
		Cache1H:         request.Cache1H,
		Rotation:        request.Rotation,
		NowNS:           environment.nowNS,
		InitialQuery:    request.Query,
	}
	return scanResult{
		Output:   output,
		Snapshot: snapshot,
		Paths:    environment.paths,
	}, nil
}

func streamFleetRefreshes(
	ctx context.Context,
	database *store.Store,
	request scanRequest,
	stderr io.Writer,
	updates chan<- ui.Snapshot,
) {
	streamFleetRefreshesWith(
		ctx,
		database,
		request,
		stderr,
		updates,
		refreshDependencies{},
	)
}

func streamFleetRefreshesWith(
	ctx context.Context,
	database *store.Store,
	request scanRequest,
	stderr io.Writer,
	updates chan<- ui.Snapshot,
	dependencies refreshDependencies,
) {
	defer close(updates)
	environment, err := resolveScanEnvironment()
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet refresh: %v\n", err)
		return
	}
	var data fleetData
	if request.View == compose.DefaultView {
		data, err = loadDefaultFleetData(ctx, database)
	} else {
		data, err = loadFleetData(ctx, database)
	}
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet refresh: %v\n", err)
		return
	}
	live, err := gatherFleet(
		ctx,
		environment.paths,
		data,
		request.ReadOnly,
		stderr,
	)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet refresh gather: %v\n", err)
		return
	}
	data, err = enrichLiveFleetData(ctx, database, data, live)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet refresh live cache: %v\n", err)
		return
	}
	if !sendRefresh(ctx, database, environment, request, data, live, true, updates) {
		return
	}

	newIndexer := dependencies.newIndexer
	if newIndexer == nil {
		newIndexer = func(database *store.Store) (indexRunner, error) {
			return fleetindex.New(database)
		}
	}
	indexer, err := newIndexer(database)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet refresh index: %v\n", err)
		return
	}
	if !request.ForceFull {
		if _, err := indexer.Run(ctx, fleetindex.Options{
			PriorityCWD:  environment.currentDir,
			PriorityOnly: true,
		}); err != nil {
			fmt.Fprintf(stderr, "cc-fleet refresh project index: %v\n", err)
			return
		}
		data, err = loadFleetData(ctx, database)
		if err != nil {
			fmt.Fprintf(stderr, "cc-fleet refresh: %v\n", err)
			return
		}
		if !sendRefresh(
			ctx,
			database,
			environment,
			request,
			data,
			live,
			true,
			updates,
		) {
			return
		}
	}

	if _, err := indexer.Run(ctx, fleetindex.Options{
		Full:        request.ForceFull,
		PriorityCWD: environment.currentDir,
	}); err != nil {
		fmt.Fprintf(stderr, "cc-fleet refresh full index: %v\n", err)
		return
	}
	data, err = loadFleetData(ctx, database)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet refresh: %v\n", err)
		return
	}
	request.ForceFull = false
	result, err := composeFleet(
		ctx,
		database,
		environment,
		request,
		data,
		live,
		!request.ReadOnly,
	)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet refresh compose: %v\n", err)
		return
	}
	result.Snapshot.Refreshing = false
	select {
	case updates <- result.Snapshot:
	case <-ctx.Done():
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
	database *store.Store,
	environment scanEnvironment,
	request scanRequest,
	data fleetData,
	live gather.Snapshot,
	refreshing bool,
	updates chan<- ui.Snapshot,
) bool {
	result, err := composeFleet(
		ctx,
		database,
		environment,
		request,
		data,
		live,
		!request.ReadOnly,
	)
	if err != nil {
		return false
	}
	result.Snapshot.Refreshing = refreshing
	select {
	case updates <- result.Snapshot:
		return true
	case <-ctx.Done():
		return false
	}
}

func applyComposeIntents(
	ctx context.Context,
	database *store.Store,
	hidden []store.Hidden,
	output compose.Output,
) error {
	hiddenByID := make(map[string]store.Hidden, len(hidden))
	for _, row := range hidden {
		hiddenByID[row.ID] = row
	}
	for _, update := range output.BaselineUpdates {
		hiddenAt := time.Now().Unix()
		if existing, found := hiddenByID[update.ID]; found {
			hiddenAt = existing.HiddenAt
		}
		baseline := update.BaselinePrompts
		if err := database.Hide(ctx, store.Hidden{
			ID:              update.ID,
			Engine:          update.Engine,
			HiddenAt:        hiddenAt,
			BaselinePrompts: &baseline,
		}); err != nil {
			return err
		}
	}
	for _, id := range output.UnhideIDs {
		if err := database.Unhide(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func accountRoots(values []string) []compose.AccountRoot {
	roots := make([]compose.AccountRoot, 0, len(values))
	for index, value := range values {
		path := value
		if resolved, err := filepath.EvalSymlinks(value); err == nil {
			path = resolved
		} else if absolute, err := filepath.Abs(value); err == nil {
			path = absolute
		}
		roots = append(roots, compose.AccountRoot{
			Account: index%3 + 1,
			Path:    filepath.Clean(path),
		})
	}
	return roots
}

func readPrimaryAccount(home string) int {
	content, err := os.ReadFile(filepath.Join(home, ".claude-primary"))
	if err != nil {
		return 1
	}
	account, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil || account < 1 || account > 3 {
		return 1
	}
	return account
}

func writePrimaryAccount(home string, account int) error {
	if account < 1 || account > 3 {
		return fmt.Errorf("primary account must be 1, 2, or 3")
	}
	path := filepath.Join(home, ".claude-primary")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".claude-primary.tmp-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := fmt.Fprintf(file, "%d\n", account); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
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

func codexAvailable(codexRoot string) bool {
	switch os.Getenv(codexAvailableEnv) {
	case "1":
		return true
	case "0":
		return false
	}
	if _, err := exec.LookPath("codex"); err == nil {
		return true
	}
	info, err := os.Stat(codexRoot)
	return err == nil && info.IsDir()
}
