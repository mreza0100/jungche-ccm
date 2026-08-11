// Package gather probes live tmux and process state without indexing files.
package gather

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"hostops/cc-fleet/internal/paths"

	"golang.org/x/sync/errgroup"
)

// CodexNameResolver resolves the indexed display name for a rollout path.
type CodexNameResolver func(rolloutPath string) string

// CodexIDNameResolver resolves the indexed display name for a Codex thread id.
// It is the only naming rung a session that writes no rollout file has, and
// the index behind it owns the precedence: the thread's own name, then the
// session_index name, then its first prompt.
type CodexIDNameResolver func(threadID string) string

// Dependencies supplies probe interfaces and process-local policy.
type Dependencies struct {
	ProcFS      ProcFS
	Tmux        TmuxClient
	TmuxBinary  string
	TmuxTmpDir  string
	Now         func() time.Time
	CodexName   CodexNameResolver
	CodexIDName CodexIDNameResolver
	CodexThread CodexThreadResolver
	ReadOnly    bool
}

// Gatherer creates immutable live-state snapshots.
type Gatherer struct {
	paths       paths.Values
	proc        ProcFS
	tmux        TmuxClient
	now         func() time.Time
	codexName   CodexNameResolver
	codexIDName CodexIDNameResolver
	codexThread CodexThreadResolver
	readOnly    bool
}

// New resolves paths and fills real implementations for omitted interfaces.
func New(dependencies Dependencies) (*Gatherer, error) {
	resolved, err := paths.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve gather paths: %w", err)
	}
	proc := dependencies.ProcFS
	if proc == nil {
		proc = RealProcFS{Root: resolved.ProcRoot}
	}
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	tmux := dependencies.Tmux
	if tmux == nil {
		tmuxTmpDir := dependencies.TmuxTmpDir
		if tmuxTmpDir == "" {
			tmuxTmpDir = os.Getenv("TMUX_TMPDIR")
			if tmuxTmpDir == "" {
				tmuxTmpDir = "/tmp"
			}
		}
		tmux = CommandTmux{
			Binary:     dependencies.TmuxBinary,
			TmuxTmpDir: tmuxTmpDir,
		}
	}
	return &Gatherer{
		paths:       resolved,
		proc:        proc,
		tmux:        tmux,
		now:         now,
		codexName:   dependencies.CodexName,
		codexIDName: dependencies.CodexIDName,
		codexThread: dependencies.CodexThread,
		readOnly:    dependencies.ReadOnly,
	}, nil
}

// Gather probes live state. Pane-dependent filesystem and ProcFS probes run
// concurrently after the one-command-per-socket tmux snapshot is available.
func (gatherer *Gatherer) Gather(ctx context.Context) (Snapshot, error) {
	var tmuxProbe TmuxProbe
	var err error
	if gatherer.readOnly {
		tmuxProbe, err = ProbeTmuxReadOnly(
			ctx,
			gatherer.paths.TmuxDir,
			gatherer.tmux,
			gatherer.now(),
		)
	} else {
		tmuxProbe, err = ProbeTmux(
			ctx,
			gatherer.paths.TmuxDir,
			gatherer.tmux,
			gatherer.now(),
		)
	}
	if err != nil {
		return Snapshot{}, err
	}

	var crumbs CrumbProbe
	var codex []LiveCodex
	var claudeProcesses []ClaudeProcess
	var agents []Agent
	var cacheSockets []string
	group, _ := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		if gatherer.readOnly {
			crumbs, err = ReadCrumbsReadOnly(gatherer.paths.SIDDir, tmuxProbe.Panes)
		} else {
			crumbs, err = ReadCrumbs(gatherer.paths.SIDDir, tmuxProbe.Panes)
		}
		return err
	})
	group.Go(func() error {
		var err error
		codex, err = DetectCodexThreads(
			gatherer.proc,
			gatherer.paths.CodexRoot,
			tmuxProbe.Panes,
			gatherer.codexThread,
		)
		return err
	})
	group.Go(func() error {
		var err error
		agents, err = DetectAgents(
			gatherer.proc,
			gatherer.paths.Home,
			tmuxProbe.Panes,
		)
		return err
	})
	group.Go(func() error {
		var err error
		claudeProcesses, err = DetectClaudeProcesses(
			gatherer.proc,
			tmuxProbe.Panes,
		)
		return err
	})
	group.Go(func() error {
		var err error
		cacheSockets, err = DetectCache1H(gatherer.proc, tmuxProbe.Panes)
		return err
	})
	if err := group.Wait(); err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		Panes:           append([]Pane(nil), tmuxProbe.Panes...),
		Crumbs:          append([]Crumb(nil), crumbs.Crumbs...),
		Codex:           append([]LiveCodex(nil), codex...),
		ClaudeProcesses: append([]ClaudeProcess(nil), claudeProcesses...),
		Agents:          append([]Agent(nil), agents...),
		Cache1HSockets:  append([]string(nil), cacheSockets...),
		Renames: computeWindowRenames(
			tmuxProbe.Panes,
			codex,
			gatherer.codexName,
			gatherer.codexIDName,
		),
		CorpseSwept: append([]string(nil), tmuxProbe.CorpseSwept...),
		StaleSwept:  append([]string(nil), crumbs.StaleSwept...),
		Warnings:    append([]string(nil), tmuxProbe.ProbeWarnings...),
	}, nil
}

func computeWindowRenames(
	panes []Pane,
	codex []LiveCodex,
	resolveRollout CodexNameResolver,
	resolveID CodexIDNameResolver,
) []WindowRename {
	if resolveRollout == nil && resolveID == nil {
		return nil
	}
	paneByTarget := make(map[string]Pane, len(panes))
	for _, pane := range panes {
		paneByTarget[pane.Socket+"\x00"+pane.PaneID] = pane
	}
	seenWindows := make(map[string]struct{})
	planned := make([]WindowRename, 0)
	for _, live := range codex {
		pane, found := paneByTarget[live.Socket+"\x00"+live.PaneID]
		if !found {
			continue
		}
		target := clipRunes(codexWindowName(live, resolveRollout, resolveID), 24)
		if target == "" || pane.WindowName == target || pane.WindowID == "" {
			continue
		}
		key := pane.Socket + "\x00" + pane.WindowID
		if _, duplicate := seenWindows[key]; duplicate {
			continue
		}
		seenWindows[key] = struct{}{}
		planned = append(planned, WindowRename{
			Socket:      pane.Socket,
			SessionName: pane.SessionName,
			WindowID:    pane.WindowID,
			CurrentName: pane.WindowName,
			TargetName:  target,
		})
	}

	// Two windows converging on ONE name is an ambiguity, not a rename: the
	// window name is how the fleet addresses a chat, so two chats answering to
	// it is worse than either keeping the name it has. Skip BOTH, exactly as
	// the zsh half leaves a window with two differently-labeled panes alone.
	windowsByTarget := make(map[string]int, len(planned))
	for _, rename := range planned {
		windowsByTarget[rename.TargetName]++
	}
	renames := make([]WindowRename, 0, len(planned))
	for _, rename := range planned {
		if windowsByTarget[rename.TargetName] > 1 {
			continue
		}
		renames = append(renames, rename)
	}
	sort.Slice(renames, func(left, right int) bool {
		if renames[left].Socket != renames[right].Socket {
			return renames[left].Socket < renames[right].Socket
		}
		return renames[left].WindowID < renames[right].WindowID
	})
	return renames
}

// codexWindowName walks the naming rungs for one live Codex pane: the rollout
// file it holds, then the thread id it was identified by. A resumed or
// paginated session has only the second — keying the rename on the rollout
// path alone left every such window carrying the name it was born with, or,
// worse, a sibling's.
func codexWindowName(
	live LiveCodex,
	resolveRollout CodexNameResolver,
	resolveID CodexIDNameResolver,
) string {
	if resolveRollout != nil && live.RolloutPath != "" {
		if name := resolveRollout(live.RolloutPath); name != "" {
			return name
		}
	}
	if resolveID != nil && live.ThreadID != "" {
		return resolveID(live.ThreadID)
	}
	return ""
}

func clipRunes(value string, limit int) string {
	count := 0
	for index := range value {
		if count == limit {
			return value[:index]
		}
		count++
	}
	return value
}
