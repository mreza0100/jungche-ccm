package hide

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hostops/cc-fleet/internal/index"
	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/shared"
	"hostops/cc-fleet/internal/store"
)

const (
	defaultExitDelay    = 1500 * time.Millisecond
	defaultPollEvery    = time.Second
	defaultPollAttempts = 20
)

type indexRefresher struct {
	database *store.Store
}

func (refresher indexRefresher) Refresh(ctx context.Context) error {
	indexer, err := index.New(refresher.database)
	if err != nil {
		return err
	}
	_, err = indexer.Run(ctx, index.Options{})
	return err
}

// NewFinisher constructs the hidden internal subcommand worker.
func NewFinisher(
	database *store.Store,
	dependencies Dependencies,
) (*Finisher, error) {
	if database == nil {
		return nil, errors.New("hide finisher store is nil")
	}
	resolved, err := paths.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve hide finisher paths: %w", err)
	}
	tmux := dependencies.Tmux
	if tmux == nil {
		tmux = CommandTmux{}
	}
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	refresher := dependencies.Refresher
	if refresher == nil {
		refresher = indexRefresher{database: database}
	}
	delay := dependencies.Delay
	if delay == 0 {
		delay = defaultExitDelay
	}
	pollEvery := dependencies.PollEvery
	if pollEvery == 0 {
		pollEvery = defaultPollEvery
	}
	pollAttempts := dependencies.PollAttempts
	if pollAttempts == 0 {
		pollAttempts = defaultPollAttempts
	}
	return &Finisher{
		database:     database,
		tmux:         tmux,
		refresher:    refresher,
		now:          now,
		delay:        delay,
		pollEvery:    pollEvery,
		pollAttempts: pollAttempts,
		paths: resolvedPaths{
			home:      resolved.Home,
			sidDir:    resolved.SIDDir,
			codexRoot: resolved.CodexRoot,
			tmuxDir:   resolved.TmuxDir,
		},
	}, nil
}

// Run performs the delayed graceful close, fallback kill, cleanup, refresh,
// post-exit hide, and teammate reap.
func (finisher *Finisher) Run(
	ctx context.Context,
	args ExitArgs,
) error {
	if args.Engine != ClaudeEngine && args.Engine != CodexEngine {
		return fmt.Errorf("unknown hide-exit engine %q", args.Engine)
	}
	if args.ID == "" || args.SocketPath == "" || args.PaneID == "" {
		return errors.New("hide-exit requires id, socket, and pane")
	}
	if err := waitContext(ctx, finisher.delay); err != nil {
		return err
	}

	command := "/exit"
	if args.Engine == CodexEngine {
		command = "/quit"
	}
	_ = finisher.tmux.SendLine(ctx, args.SocketPath, args.PaneID, command)
	for attempt := 0; attempt < finisher.pollAttempts; attempt++ {
		if !finisher.tmux.PaneExists(ctx, args.SocketPath, args.PaneID) {
			break
		}
		if err := waitContext(ctx, finisher.pollEvery); err != nil {
			return err
		}
	}
	_ = finisher.tmux.KillPane(ctx, args.SocketPath, args.PaneID)

	var cleanupErrors []error
	for _, crumb := range []string{
		filepath.Join(finisher.paths.sidDir, args.SocketName+"."+args.PaneID),
		filepath.Join(finisher.paths.sidDir, args.SocketName),
	} {
		if err := os.Remove(crumb); err != nil && !errors.Is(err, fs.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, err)
		}
	}

	if err := finisher.refresher.Refresh(ctx); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("refresh post-exit index: %w", err))
	} else if err := finisher.recordPostExitHide(ctx, args); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if args.Engine == ClaudeEngine {
		if err := finisher.reapTeammates(ctx, args.ID); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	return errors.Join(cleanupErrors...)
}

// recordPostExitHide re-asserts the hide after the post-exit index refresh so
// the chat stays hidden whatever the exit flush wrote, keeping the original
// hide time.
func (finisher *Finisher) recordPostExitHide(
	ctx context.Context,
	args ExitArgs,
) error {
	hiddenAt := finisher.now().Unix()
	if hidden, exists, err := finisher.database.Hidden(ctx, args.ID); err != nil {
		return err
	} else if exists {
		hiddenAt = hidden.HiddenAt
	}
	if err := finisher.database.Hide(ctx, store.Hidden{
		ID:       args.ID,
		Engine:   args.Engine,
		HiddenAt: hiddenAt,
	}); err != nil {
		return fmt.Errorf("record post-exit hide: %w", err)
	}
	return nil
}

// reapTeammates takes down the chats this chat spawned, reading them from the
// shared store's `children` table — the same rows chat.sh writes with `cc-db.sh
// child-add new` for a detached teammate (chat.sh:1362) and `child-add pane`
// for one sharing this server (chat.sh:435). The flat files under
// ~/.claude/.cc-{new,pane}-children are the fallback the children helper
// describes; the live fleet stopped writing them, and reading them alone is why
// teammates outlived their orchestrator.
//
// A detached teammate owns its socket, so it dies by kill-server. A pane
// teammate shares this chat's server, so it dies by kill-pane and never by
// kill-server, which would take this chat's neighbours down with it
// (chat.sh:428-431).
func (finisher *Finisher) reapTeammates(
	ctx context.Context,
	id string,
) error {
	var reapErrors []error
	state := finisher.database.Shared()

	detachedPath := filepath.Join(
		finisher.paths.home,
		".claude",
		".cc-new-children",
		id,
	)
	detached, err := finisher.children(ctx, shared.KindNew, id, detachedPath)
	if err != nil {
		reapErrors = append(reapErrors, err)
	}
	for _, socket := range detached {
		socket = strings.TrimSpace(socket)
		if socket == "" {
			continue
		}
		socketPath := filepath.Join(finisher.paths.tmuxDir, socket)
		_ = finisher.tmux.KillServer(ctx, socketPath)
		if err := os.Remove(socketPath); err != nil &&
			!errors.Is(err, fs.ErrNotExist) {
			reapErrors = append(reapErrors, err)
		}
	}
	if err := state.ClearChildren(ctx, shared.KindNew, id); err != nil {
		reapErrors = append(reapErrors, err)
	}
	if err := os.Remove(detachedPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		reapErrors = append(reapErrors, err)
	}

	panePath := filepath.Join(
		finisher.paths.home,
		".claude",
		".cc-pane-children",
		id,
	)
	panes, err := finisher.children(ctx, shared.KindPane, id, panePath)
	if err != nil {
		reapErrors = append(reapErrors, err)
	}
	for _, value := range panes {
		socket, pane, found := strings.Cut(value, "\t")
		if !found || pane == "" {
			continue
		}
		_ = finisher.tmux.KillPane(
			ctx,
			filepath.Join(finisher.paths.tmuxDir, socket),
			pane,
		)
	}
	if err := state.ClearChildren(ctx, shared.KindPane, id); err != nil {
		reapErrors = append(reapErrors, err)
	}
	if err := os.Remove(panePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		reapErrors = append(reapErrors, err)
	}
	return errors.Join(reapErrors...)
}

// children resolves one kind of teammate for a chat: the shared table's rows,
// or the flat file when the table has nothing to say.
//
// "Nothing to say" covers both an unreachable table and a table with no row for
// this chat. The second case is the one that matters in practice: a chat that
// registered its teammates through the old flat-file path and exits after the
// cutover has a file and no rows, and reading only the table would leave its
// teammates running forever. A chat registered through the table has rows and
// no file, so the file is never consulted for it.
func (finisher *Finisher) children(
	ctx context.Context,
	kind, id, flatPath string,
) ([]string, error) {
	values, _, err := finisher.database.Shared().Children(ctx, kind, id)
	if err != nil {
		return nil, err
	}
	if len(values) > 0 {
		return values, nil
	}
	return readChildFile(flatPath)
}

// readChildFile reads one flat teammate file, whose lines are exactly the
// values cc-db.sh's fallback appends (cc-db.sh:283-284).
func readChildFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var values []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		values = append(values, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
