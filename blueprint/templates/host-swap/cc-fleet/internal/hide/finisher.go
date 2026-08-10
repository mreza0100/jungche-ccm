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

func (finisher *Finisher) reapTeammates(
	ctx context.Context,
	id string,
) error {
	var reapErrors []error
	detachedPath := filepath.Join(
		finisher.paths.home,
		".claude",
		".cc-new-children",
		id,
	)
	if file, err := os.Open(detachedPath); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			socket := strings.TrimSpace(scanner.Text())
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
		if err := scanner.Err(); err != nil {
			reapErrors = append(reapErrors, err)
		}
		_ = file.Close()
	} else if !errors.Is(err, fs.ErrNotExist) {
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
	if file, err := os.Open(panePath); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			socket, pane, found := strings.Cut(scanner.Text(), "\t")
			if !found || pane == "" {
				continue
			}
			_ = finisher.tmux.KillPane(
				ctx,
				filepath.Join(finisher.paths.tmuxDir, socket),
				pane,
			)
		}
		if err := scanner.Err(); err != nil {
			reapErrors = append(reapErrors, err)
		}
		_ = file.Close()
	} else if !errors.Is(err, fs.ErrNotExist) {
		reapErrors = append(reapErrors, err)
	}
	if err := os.Remove(panePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		reapErrors = append(reapErrors, err)
	}
	return errors.Join(reapErrors...)
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
