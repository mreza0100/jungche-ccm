package hide

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hostops/cc-fleet/internal/gather"
	"hostops/cc-fleet/internal/paths"
	"hostops/cc-fleet/internal/store"
)

// New constructs a manager from the already-open fleet store.
func New(database *store.Store, dependencies Dependencies) (*Manager, error) {
	if database == nil {
		return nil, errors.New("hide store is nil")
	}
	resolved, err := paths.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve hide paths: %w", err)
	}
	proc := dependencies.ProcFS
	if proc == nil {
		proc = gather.RealProcFS{Root: resolved.ProcRoot}
	}
	tmux := dependencies.Tmux
	if tmux == nil {
		tmux = CommandTmux{}
	}
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	spawner := dependencies.Spawner
	if spawner == nil {
		spawner = CommandSpawner{Executable: dependencies.Executable}
	}
	return &Manager{
		database: database,
		proc:     proc,
		tmux:     tmux,
		spawner:  spawner,
		now:      now,
		paths: resolvedPaths{
			home:      resolved.Home,
			sidDir:    resolved.SIDDir,
			codexRoot: resolved.CodexRoot,
			tmuxDir:   resolved.TmuxDir,
		},
	}, nil
}

// Environment reads the three caller values used by --self.
func Environment() SelfEnvironment {
	return SelfEnvironment{
		TMUX:            os.Getenv("TMUX"),
		TMUXPane:        os.Getenv("TMUX_PANE"),
		ClaudeSessionID: os.Getenv("CLAUDE_CODE_SESSION_ID"),
	}
}

// Hide records a permanent hide and optionally starts the detached exit
// finisher. The hide lifts only on an explicit unhide, so no prompt baseline
// is recorded.
func (manager *Manager) Hide(
	ctx context.Context,
	request Request,
) (Target, error) {
	var target Target
	var err error
	switch {
	case request.Self:
		target, err = manager.IdentifySelf(ctx, request.Environment)
	case request.ID != "":
		target, err = manager.lookupTarget(ctx, request.ID)
	default:
		err = errors.New("hide requires --self or an id")
	}
	if err != nil {
		return Target{}, err
	}
	if request.Exit && (target.SocketPath == "" || target.PaneID == "") {
		return Target{}, errors.New("--exit requires a live --self tmux pane")
	}

	if err := manager.database.Hide(ctx, store.Hidden{
		ID:       target.ID,
		Engine:   target.Engine,
		HiddenAt: manager.now().Unix(),
	}); err != nil {
		return Target{}, err
	}

	if request.Exit {
		if err := manager.spawner.Spawn(ctx, ExitArgs{
			Engine:     target.Engine,
			ID:         target.ID,
			DataPath:   target.DataPath,
			SocketPath: target.SocketPath,
			SocketName: target.SocketName,
			PaneID:     target.PaneID,
		}); err != nil {
			return Target{}, err
		}
	}
	return target, nil
}

// Unhide removes one hide through the store's non-fatal busy policy.
func (manager *Manager) Unhide(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("unhide id is empty")
	}
	if _, found, err := manager.database.Transcript(ctx, id); err != nil {
		return err
	} else if !found {
		if lineage, found, err := manager.database.CodexLineage(
			ctx,
			id,
		); err != nil {
			return err
		} else if found {
			id = lineage.RootID
		}
	}
	return manager.database.Unhide(ctx, id)
}

// Hidden returns every current hide in stable ID order.
func (manager *Manager) Hidden(ctx context.Context) ([]store.Hidden, error) {
	return manager.database.HiddenChats(ctx)
}

func (manager *Manager) lookupTarget(
	ctx context.Context,
	id string,
) (Target, error) {
	transcript, found, err := manager.database.Transcript(ctx, id)
	if err != nil {
		return Target{}, err
	}
	if found {
		return Target{
			Engine:   ClaudeEngine,
			ID:       id,
			DataPath: transcript.Path,
		}, nil
	}
	lineage, found, err := manager.database.CodexLineage(ctx, id)
	if err != nil {
		return Target{}, err
	}
	if found {
		return Target{
			Engine:   CodexEngine,
			ID:       lineage.RootID,
			DataPath: lineage.Newest.Path,
		}, nil
	}
	return Target{}, fmt.Errorf("chat %q is not indexed", id)
}

func (manager *Manager) codexTarget(
	ctx context.Context,
	id, fallbackPath string,
) (Target, error) {
	lineage, found, err := manager.database.CodexLineage(ctx, id)
	if err != nil {
		return Target{}, err
	}
	if !found {
		return Target{
			Engine:   CodexEngine,
			ID:       id,
			DataPath: fallbackPath,
		}, nil
	}
	path := lineage.Newest.Path
	if path == "" {
		path = fallbackPath
	}
	return Target{
		Engine:   CodexEngine,
		ID:       lineage.RootID,
		DataPath: path,
	}, nil
}

func parseTMUX(value string) (socketPath, socketName string) {
	if comma := strings.IndexByte(value, ','); comma >= 0 {
		value = value[:comma]
	}
	if value == "" {
		return "", ""
	}
	return value, filepath.Base(value)
}
