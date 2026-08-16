package hide

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/store"
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
		target, err = manager.lookupTarget(ctx, request.ID, request.Engine, request.RolloutPath)
	default:
		err = errors.New("hide requires --self or an id")
	}
	if err != nil {
		return Target{}, err
	}
	if request.SocketName != "" || request.PaneID != "" {
		if request.SocketName == "" || request.PaneID == "" {
			return Target{}, errors.New("resolved live address requires socket and pane")
		}
		if filepath.Base(request.SocketName) != request.SocketName {
			return Target{}, errors.New("resolved socket name must not contain a path")
		}
		target.SocketName = request.SocketName
		target.SocketPath = filepath.Join(manager.paths.tmuxDir, request.SocketName)
		target.PaneID = request.PaneID
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

// HideCleared records a prompt-baseline hide only when id is already an
// indexed Claude fleet chat. The SessionEnd hook supplies the id directly;
// no tmux or cwd guess may turn an ordinary bare Claude session into a fleet
// hide.
func (manager *Manager) HideCleared(
	ctx context.Context,
	id string,
) (Target, bool, error) {
	if id == "" {
		return Target{}, false, nil
	}
	transcript, found, err := manager.database.Transcript(ctx, id)
	if err != nil || !found {
		return Target{}, false, err
	}
	baseline := transcript.PromptCount
	if err := manager.database.Hide(ctx, store.Hidden{
		ID: id, Engine: ClaudeEngine, HiddenAt: manager.now().Unix(),
		BaselinePrompts: &baseline,
	}); err != nil {
		return Target{}, false, err
	}
	return Target{
		Engine: ClaudeEngine, ID: id, DataPath: transcript.Path,
	}, true, nil
}

// Unhide removes one hide through the store's non-fatal busy policy.
//
// A Codex hide is not always keyed on the id this call receives: the live
// process can expose a resumed CHILD rollout id, while every id
// the picker shows the user is the lineage ROOT — compose keys every Codex
// row on it (composer.rolloutRow). Resolving id to the root and unhiding
// only that key would leave a child-keyed hide standing: the row would come
// right back hidden (composer.hiddenMatch, store.codexLineageHidden both
// check every member). So every id in the lineage is unhidden, root and
// members alike; the shared store's Unhide is a safe no-op for an id that
// carries no hide.
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
			return manager.unhideLineage(ctx, lineage)
		}
	}
	return manager.database.Unhide(ctx, id)
}

// unhideLineage clears every id in a Codex resume lineage, root and members
// alike — see Unhide's own comment for why the root alone is not enough.
func (manager *Manager) unhideLineage(
	ctx context.Context,
	lineage store.CodexLineage,
) error {
	if err := manager.database.Unhide(ctx, lineage.RootID); err != nil {
		return err
	}
	for _, member := range lineage.MemberIDs {
		if member == lineage.RootID {
			continue
		}
		if err := manager.database.Unhide(ctx, member); err != nil {
			return err
		}
	}
	return nil
}

// Hidden returns every current hide in stable ID order.
func (manager *Manager) Hidden(ctx context.Context) ([]store.Hidden, error) {
	return manager.database.HiddenChats(ctx)
}

// lookupTarget resolves the chat a hide is aimed at. engine is the caller's
// own answer to "which engine?", supplied only by a caller holding the row;
// rolloutPath is that same caller's own answer to "which file?" for a Codex
// id, used only to resolve an UNINDEXED lineage member to its root before
// falling back to naming the member's own (wrong) id.
func (manager *Manager) lookupTarget(
	ctx context.Context,
	id, engine, rolloutPath string,
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
	if engine == CodexEngine {
		if target, resolved := manager.resolveUnindexedCodexParent(ctx, rolloutPath); resolved {
			return target, nil
		}
	}
	if engine != "" {
		// A LIVE agent, almost always: its row is composed straight from the
		// running process, so the picker shows it long before its transcript
		// reaches the index — and the index is what the two lookups above ask.
		// Refusing here is what made ⌃X on an agent row a silent no-op. The
		// shared hidden store is keyed by uuid alone and derives the engine
		// back out of the index, so the agent's own session uuid is the whole
		// key this hide needs.
		return Target{Engine: engine, ID: id}, nil
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
		if target, resolved := manager.resolveUnindexedCodexParent(ctx, fallbackPath); resolved {
			return target, nil
		}
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

// resolveUnindexedCodexParent reads rolloutPath's own session_meta header —
// the shared shape identifyCodexSelf and the CLI's own row lookup both hit —
// for the conversation it resumes from: the ONLY place that link exists
// before the indexer has parsed the file into the fleet database. It
// answers found=true only when THAT parent id itself resolves to a real,
// already-indexed lineage; a parent that is ALSO unindexed (a deeper resume
// chain the indexer has not reached at all) falls through to the caller's
// own fallback rather than guessing further.
func (manager *Manager) resolveUnindexedCodexParent(
	ctx context.Context,
	rolloutPath string,
) (Target, bool) {
	parent := readCodexLineageParent(rolloutPath)
	if parent == "" {
		return Target{}, false
	}
	lineage, found, err := manager.database.CodexLineage(ctx, parent)
	if err != nil || !found {
		return Target{}, false
	}
	return Target{
		Engine:   CodexEngine,
		ID:       lineage.RootID,
		DataPath: lineage.Newest.Path,
	}, true
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
