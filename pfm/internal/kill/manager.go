package kill

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	pfmengine "hostops/pfm/internal/engine"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/store"
)

// New constructs a manager from the already-open fleet store.
func New(database *store.Store, dependencies Dependencies) (*Manager, error) {
	if database == nil {
		return nil, errors.New("kill store is nil")
	}
	resolved := dependencies.Paths
	if resolved.Home == "" {
		var err error
		resolved, err = paths.Resolve()
		if err != nil {
			return nil, fmt.Errorf("resolve kill paths: %w", err)
		}
	}
	proc := dependencies.ProcFS
	if proc == nil {
		proc = gather.NewProcFS(resolved.ProcRoot)
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
		spawner = CommandSpawner{
			Executable: dependencies.Executable,
			ConfigPath: dependencies.ConfigPath,
		}
	}
	codexRoots := dependencies.CodexRoots
	if codexRoots == nil {
		codexRoots = []string{resolved.CodexRoot}
	} else {
		codexRoots = append([]string{}, codexRoots...)
	}
	return &Manager{
		database: database,
		proc:     proc,
		tmux:     tmux,
		spawner:  spawner,
		now:      now,
		paths: resolvedPaths{
			home:       resolved.Home,
			sidDir:     resolved.SIDDir,
			codexRoots: codexRoots,
			tmuxDir:    resolved.TmuxDir,
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

// Kill records a permanent kill and optionally starts the detached exit
// finisher. The kill lifts only on an explicit unkill, so no prompt baseline
// is recorded.
func (manager *Manager) Kill(
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
		err = errors.New("kill requires --self or an id")
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

	if err := manager.database.Kill(ctx, store.Killed{
		ID:       target.ID,
		Engine:   target.Engine,
		KilledAt: manager.now().Unix(),
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

// KillCleared records a prompt-baseline kill only when id is already an
// indexed Claude fleet chat. The SessionEnd hook supplies the id directly;
// no tmux or cwd guess may turn an ordinary bare Claude session into a fleet
// kill.
func (manager *Manager) KillCleared(
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
	if err := manager.database.Kill(ctx, store.Killed{
		ID: id, Engine: string(pfmengine.Claude), KilledAt: manager.now().Unix(),
		BaselinePrompts: &baseline,
	}); err != nil {
		return Target{}, false, err
	}
	return Target{
		Engine: string(pfmengine.Claude), ID: id, DataPath: transcript.Path,
	}, true, nil
}

// CodexPaneBinding returns the last Codex thread observed in one immutable
// tmux pane. Codex starts a new thread in the same pane for /clear, so this is
// the only unambiguous way for its deferred SessionStart(source=clear) hook to
// identify the completed thread without guessing from a shared cwd.
func (manager *Manager) CodexPaneBinding(
	ctx context.Context,
	socket, pane string,
) (string, bool, error) {
	key, ok := codexPaneBindingKey(socket, pane)
	if !ok {
		return "", false, nil
	}
	return manager.database.Meta(ctx, key)
}

// BindCodexPane records the current thread for one live Codex pane in pfm's
// private derived cache. The shared store remains reserved for operator state.
func (manager *Manager) BindCodexPane(
	ctx context.Context,
	socket, pane, threadID string,
) error {
	key, ok := codexPaneBindingKey(socket, pane)
	if !ok || threadID == "" {
		return nil
	}
	current, found, err := manager.database.Meta(ctx, key)
	if err != nil || found && current == threadID {
		return err
	}
	return manager.database.SetMeta(ctx, key, threadID)
}

// CodexProcessBinding returns the last thread observed by SessionStart hooks
// from one Codex TUI process. Codex keeps that process alive across /clear,
// including when no tmux pane exists.
func (manager *Manager) CodexProcessBinding(
	ctx context.Context,
	parent string,
) (string, bool, error) {
	key, ok := codexProcessBindingKey(parent)
	if !ok {
		return "", false, nil
	}
	return manager.database.Meta(ctx, key)
}

// BindCodexProcess records the current thread for one Codex TUI process in
// pfm's private derived cache.
func (manager *Manager) BindCodexProcess(
	ctx context.Context,
	parent, threadID string,
) error {
	key, ok := codexProcessBindingKey(parent)
	if !ok || threadID == "" {
		return nil
	}
	current, found, err := manager.database.Meta(ctx, key)
	if err != nil || found && current == threadID {
		return err
	}
	return manager.database.SetMeta(ctx, key, threadID)
}

// SeedCodexPane records a live-scan identity only when no hook binding exists.
// A Codex process cannot rewrite its own inherited CODEX_THREAD_ID after
// /clear, so later scans may still report the completed id and must never
// overwrite the authoritative SessionStart payload.
func (manager *Manager) SeedCodexPane(
	ctx context.Context,
	socket, pane, threadID string,
) error {
	key, ok := codexPaneBindingKey(socket, pane)
	if !ok || threadID == "" {
		return nil
	}
	_, found, err := manager.database.Meta(ctx, key)
	if err != nil || found {
		return err
	}
	return manager.database.SetMeta(ctx, key, threadID)
}

// KillClearedCodex records a prompt-baseline kill on the visible lineage root
// for an already indexed Codex thread. It never guesses an id: callers must
// supply the hook-observed previous id from a pane/process binding or
// inherited CODEX_THREAD_ID.
func (manager *Manager) KillClearedCodex(
	ctx context.Context,
	id string,
) (Target, bool, error) {
	if id == "" {
		return Target{}, false, nil
	}
	lineage, found, err := manager.database.CodexLineage(ctx, id)
	if err != nil || !found {
		return Target{}, false, err
	}
	baseline := lineage.PromptCount
	if err := manager.database.Kill(ctx, store.Killed{
		ID: lineage.RootID, Engine: string(pfmengine.Codex), KilledAt: manager.now().Unix(),
		BaselinePrompts: &baseline,
	}); err != nil {
		return Target{}, false, err
	}
	return Target{
		Engine: string(pfmengine.Codex), ID: lineage.RootID, DataPath: lineage.Newest.Path,
	}, true, nil
}

func codexPaneBindingKey(socket, pane string) (string, bool) {
	socket = filepath.Base(strings.TrimSpace(socket))
	pane = strings.TrimSpace(pane)
	if socket == "" || socket == "." || pane == "" {
		return "", false
	}
	address := base64.RawURLEncoding.EncodeToString([]byte(socket + "\x00" + pane))
	return "codex_clear_pane_" + address, true
}

func codexProcessBindingKey(parent string) (string, bool) {
	parent = strings.TrimSpace(parent)
	pid, err := strconv.ParseUint(parent, 10, 64)
	if err != nil || pid == 0 {
		return "", false
	}
	return "codex_clear_process_" + parent, true
}

// Unkill removes one kill through the store's non-fatal busy policy.
//
// A Codex kill is not always keyed on the id this call receives: the live
// process can expose a resumed CHILD rollout id, while every id
// the picker shows the user is the lineage ROOT — compose keys every Codex
// row on it (composer.rolloutRow). Resolving id to the root and unhiding
// only that key would leave a child-keyed kill standing: the row would come
// right back killed (composer.killedMatch, store.codexLineageKilled both
// check every member). So every id in the lineage is unkilled, root and
// members alike; the shared store's Unkill is a safe no-op for an id that
// carries no kill.
func (manager *Manager) Unkill(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("unkill id is empty")
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
			return manager.unkillLineage(ctx, lineage)
		}
	}
	return manager.database.Unkill(ctx, id)
}

// unkillLineage clears every id in a Codex resume lineage, root and members
// alike — see Unkill's own comment for why the root alone is not enough.
func (manager *Manager) unkillLineage(
	ctx context.Context,
	lineage store.CodexLineage,
) error {
	if err := manager.database.Unkill(ctx, lineage.RootID); err != nil {
		return err
	}
	for _, member := range lineage.MemberIDs {
		if member == lineage.RootID {
			continue
		}
		if err := manager.database.Unkill(ctx, member); err != nil {
			return err
		}
	}
	return nil
}

// Killed returns every current kill in stable ID order.
func (manager *Manager) Killed(ctx context.Context) ([]store.Killed, error) {
	return manager.database.KilledChats(ctx)
}

// lookupTarget resolves the chat a kill is aimed at. engine is the caller's
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
			Engine:   string(pfmengine.Claude),
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
			Engine:   string(pfmengine.Codex),
			ID:       lineage.RootID,
			DataPath: lineage.Newest.Path,
		}, nil
	}
	if engine == string(pfmengine.Codex) {
		if target, resolved := manager.resolveUnindexedCodexParent(ctx, rolloutPath); resolved {
			return target, nil
		}
	}
	if engine != "" {
		// A LIVE agent, almost always: its row is composed straight from the
		// running process, so the picker shows it long before its transcript
		// reaches the index — and the index is what the two lookups above ask.
		// Refusing here is what made ⌃X on an agent row a silent no-op. The
		// shared killed store is keyed by uuid alone and derives the engine
		// back out of the index, so the agent's own session uuid is the whole
		// key this kill needs.
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
			Engine:   string(pfmengine.Codex),
			ID:       id,
			DataPath: fallbackPath,
		}, nil
	}
	path := lineage.Newest.Path
	if path == "" {
		path = fallbackPath
	}
	return Target{
		Engine:   string(pfmengine.Codex),
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
		Engine:   string(pfmengine.Codex),
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
