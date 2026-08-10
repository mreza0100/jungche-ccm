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
	"time"

	"hostops/cc-fleet/internal/action"
	"hostops/cc-fleet/internal/compose"
	"hostops/cc-fleet/internal/gather"
	fleetindex "hostops/cc-fleet/internal/index"
	"hostops/cc-fleet/internal/naming"
	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/shared"
	"hostops/cc-fleet/internal/store"
	"hostops/cc-fleet/internal/ui"
)

const (
	testFreshSocketEnv = "CC_FLEET_TEST_FRESH_SOCKET"
	testNowNSEnv       = "CC_FLEET_TEST_NOW_NS"
	codexAvailableEnv  = "CC_FLEET_CODEX_AVAILABLE"
	dbScriptEnv        = "CC_FLEET_DB_SCRIPT"
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
	result := composeFleet(environment, request, data, live)
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
	result := composeFleet(environment, request, data, gather.Snapshot{})
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
		primary:    readPrimaryAccount(resolved),
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
		CodexThread: store.NewCodexThreadResolver(ctx, resolved.CodexRoot),
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
	}
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
	if !sendRefresh(ctx, environment, request, data, live, true, updates) {
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
		if !sendRefresh(ctx, environment, request, data, live, true, updates) {
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
	result := composeFleet(environment, request, data, live)
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

// readPrimaryAccount resolves the primary Claude account the way cc-db.sh's
// primary-get does (cc-db.sh:262-267): the shared database's meta row is the
// value, ~/.claude-primary is the mirror consulted only when the database has
// none, and anything off the roster reads as account 1.
//
// Reading the mirror alone is how the picker came up showing a different
// account from the one the launchers used: primary-set writes both, but a
// database restored without the file, or a file left behind by a rollback,
// makes them disagree, and only one of the two is authoritative.
func readPrimaryAccount(values paths.Values) int {
	account, found := shared.PrimaryAccount(context.Background(), values)
	if !found || account < 1 || account > action.MaxAccount {
		return 1
	}
	return account
}

// ccDBScript is the fleet state store's CLI, as install.sh symlinks it into
// place. CC_FLEET_DB_SCRIPT points a test jail at a stand-in.
func ccDBScript(home string) string {
	if value := os.Getenv(dbScriptEnv); value != "" {
		return value
	}
	return filepath.Join(home, ".claude", "bin", "cc-db.sh")
}

// writePrimaryAccount routes the choice through cc-db.sh primary-set, which
// validates it against the roster and mirrors it into ~/.claude-primary for the
// statusline (cc-db.sh:268-275). Writing that file behind cc-db.sh's back would
// leave the database saying one account and the file another.
//
// When cc-db.sh is absent the legacy file is written directly instead: that is
// cc-db.sh's own fallback, and the picker must never be down because a state
// store is missing.
func writePrimaryAccount(home string, account int) error {
	if account < 1 || account > action.MaxAccount {
		return fmt.Errorf("primary account must be 1-%d", action.MaxAccount)
	}
	script := ccDBScript(home)
	if _, err := os.Stat(script); err == nil {
		command := exec.Command(
			"bash",
			script,
			"primary-set",
			strconv.Itoa(account),
		)
		// HOME steers cc-db.sh at the same tree cc-fleet resolved; CC_FLEET_DB
		// is dropped because cc-db.sh reads that same name for ITS OWN database
		// (cc-db.sh:41), and CC_FLEET_HOME because cc-db.sh reads it as its
		// bundle directory (cc-db.sh:36-38) — a jail's value would make it
		// source cc-portable.sh from the jail.
		command.Env = append(
			environWithout("HOME", paths.EnvDB, paths.EnvHome),
			"HOME="+home,
		)
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf(
				"cc-db.sh primary-set %d: %w: %s",
				account,
				err,
				strings.TrimSpace(string(output)),
			)
		}
		return nil
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

// environWithout copies the environment minus the named variables, so a child
// process cannot be steered by a name this binary reads for its own purposes.
func environWithout(names ...string) []string {
	dropped := make(map[string]struct{}, len(names))
	for _, name := range names {
		dropped[name] = struct{}{}
	}
	environment := os.Environ()
	kept := make([]string, 0, len(environment))
	for _, entry := range environment {
		name := entry
		if equals := strings.IndexByte(entry, '='); equals >= 0 {
			name = entry[:equals]
		}
		if _, found := dropped[name]; found {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
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
