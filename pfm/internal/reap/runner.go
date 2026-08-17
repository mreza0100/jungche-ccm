package reap

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/shared"
)

const (
	// deadAfter is how long a socket file with no server behind it must have
	// sat there before it counts as a corpse rather than a server still
	// starting up. It mirrors the picker's own corpse sweep.
	deadAfter = time.Hour
	// vsctMaxIdle is how long a detached plain-terminal bunker may sit unused.
	vsctMaxIdle = 7 * 24 * time.Hour
	// defaultBusyRecent is how recently a transcript must have been written
	// for its chat to count as working. A running turn writes continuously.
	defaultBusyRecent = time.Minute
	// probeTimeout bounds every tmux call. A wedged server answers its socket
	// and then says nothing, and an unbounded probe waits on it forever — the
	// sweep hangs and the operator sees a tool that "does not run" rather than
	// one socket it could not read. On timeout the socket is reported unread,
	// which is a SKIP, never a kill.
	probeTimeout = 5 * time.Second
)

// KillServerFunc ends one chat's tmux server and every handle pointing at it.
// The reaper takes it as a dependency rather than spelling the sequence again:
// the picker's ⌃X and ⌃O already own that sequence, and two spellings of "end
// this chat" is how one of them starts leaving crumbs behind (K3).
type KillServerFunc func(ctx context.Context, socket string) error

// Dependencies are the reaper's collaborators. Every one of them has a real
// default; a test supplies its own.
type Dependencies struct {
	Paths      paths.Values
	Tmux       Tmux
	Proc       gather.ProcFS
	Busy       BusyProbe
	KillServer KillServerFunc
	Now        func() time.Time
}

// Runner performs one sweep.
type Runner struct {
	paths      paths.Values
	tmux       Tmux
	proc       gather.ProcFS
	busy       BusyProbe
	killServer KillServerFunc
	now        func() time.Time
}

// Options are one sweep's knobs.
type Options struct {
	// Apply performs the reap. Without it nothing is killed, moved, or
	// removed — the sweep only classifies.
	Apply bool
	// BusyRecent is the transcript-write window that counts as working.
	BusyRecent time.Duration
	// Self is the caller's own socket, which is never a candidate.
	Self string
}

// Report is one sweep's result.
type Report struct {
	Decisions []Decision
	AgentsOK  bool
	BusyError string

	Keep        int
	Orphans     int
	DeadFiles   int
	Killed      int
	KeepKB      int64
	OrphanKB    int64
	FreedKB     int64
	AvailBefore int64
	AvailAfter  int64
}

// New fills the real implementations for anything the caller omitted.
func New(dependencies Dependencies) (*Runner, error) {
	resolved := dependencies.Paths
	if resolved.TmuxDir == "" {
		var err error
		resolved, err = paths.Resolve()
		if err != nil {
			return nil, fmt.Errorf("resolve reap paths: %w", err)
		}
	}
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	tmux := dependencies.Tmux
	if tmux == nil {
		tmux = CommandTmux{TmuxDir: resolved.TmuxDir, Now: now}
	}
	proc := dependencies.Proc
	if proc == nil {
		proc = gather.NewProcFS(resolved.ProcRoot)
	}
	busy := dependencies.Busy
	if busy == nil {
		busy = NewClaudeAgents(resolved)
	}
	if dependencies.KillServer == nil {
		return nil, errors.New("reap needs a kill-server implementation")
	}
	return &Runner{
		paths:      resolved,
		tmux:       tmux,
		proc:       proc,
		busy:       busy,
		killServer: dependencies.KillServer,
		now:        now,
	}, nil
}

// Run probes the fleet, classifies every socket, and — only with Apply —
// performs the actions the plan asked for.
func (runner *Runner) Run(
	ctx context.Context,
	options Options,
) (Report, error) {
	input := Input{
		Self:        options.Self,
		Apply:       options.Apply,
		DeadAfter:   deadAfter,
		VSCTMaxIdle: vsctMaxIdle,
		BusyRecent:  options.BusyRecent,
	}
	if input.BusyRecent <= 0 {
		input.BusyRecent = defaultBusyRecent
	}

	var report Report
	busyIDs, err := runner.busy.BusySessions(ctx)
	input.AgentsOK = err == nil
	input.BusyIDs = busyIDs
	if err != nil {
		report.BusyError = err.Error()
	}
	report.AgentsOK = input.AgentsOK

	tree, err := NewProcessTree(runner.proc)
	if err != nil {
		return Report{}, err
	}

	sockets, err := runner.probeSockets(ctx, tree)
	if err != nil {
		return Report{}, err
	}
	state := shared.Open(ctx, runner.paths)
	branchSeats, branchErr := state.BranchSeats(ctx)
	closeErr := state.Close()
	if branchErr != nil || closeErr != nil {
		return Report{}, fmt.Errorf("read detached branch seats: %w", errors.Join(branchErr, closeErr))
	}
	for index := range sockets {
		_, sockets[index].DetachedFork = branchSeats[sockets[index].Name]
	}
	input.Sockets = sockets
	input.RecentIDs = runner.recentSessions(sockets, input.BusyRecent)

	sessionCtx, cancelSessions := context.WithTimeout(ctx, probeTimeout)
	sessions, err := runner.tmux.Sessions(sessionCtx, "vsct")
	cancelSessions()
	if err != nil {
		return Report{}, fmt.Errorf("list bunker sessions: %w", err)
	}
	input.VSCT = sessions

	decisions := Plan(input)
	if options.Apply {
		report.AvailBefore = runner.availableKB()
		decisions = runner.apply(ctx, decisions)
		report.AvailAfter = runner.availableKB()
	}
	report.Decisions = decisions
	for _, decision := range decisions {
		switch decision.State {
		case StateOrphan, StateFork:
			report.Orphans++
			report.OrphanKB += decision.RSSKB
		case StateKilled:
			report.Killed++
			report.FreedKB += decision.RSSKB
		case StateDead:
			report.DeadFiles++
		case StateSkip:
			report.Keep++
			report.KeepKB += decision.RSSKB
		default:
			report.Keep++
			report.KeepKB += decision.RSSKB
		}
	}
	return report, nil
}

// probeSockets reads every chat socket in the jail's socket directory.
func (runner *Runner) probeSockets(
	ctx context.Context,
	tree *ProcessTree,
) ([]Socket, error) {
	entries, err := os.ReadDir(runner.paths.TmuxDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read tmux socket directory: %w", err)
	}

	type socketFile struct {
		name    string
		modTime time.Time
	}
	files := make([]socketFile, 0, len(entries))
	for _, entry := range entries {
		if !gather.IsChatSocketName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSocket == 0 {
			continue
		}
		files = append(files, socketFile{name: entry.Name(), modTime: info.ModTime()})
	}
	sort.Slice(files, func(left, right int) bool {
		return files[left].name < files[right].name
	})

	allPanes := make([]gather.Pane, 0)
	sockets := make([]Socket, 0, len(files))
	for _, file := range files {
		socket := Socket{Name: file.name, Age: runner.now().Sub(file.modTime)}
		panes, err := runner.listPanes(ctx, file.name)
		switch {
		case err == nil:
			socket.HasServer = len(panes) > 0
		case errors.Is(err, gather.ErrServerGone):
			socket.HasServer = false
		default:
			socket.ProbeFailed = true
			socket.ProbeError = err.Error()
		}
		panePIDs := make([]int, 0, len(panes))
		for _, pane := range panes {
			allPanes = append(allPanes, pane)
			panePIDs = append(panePIDs, pane.PID)
			if pane.Attached {
				socket.Attached = true
			}
			if socket.Label == "" && pane.WindowName != "" {
				socket.Label = pane.WindowName
			}
			if socket.CWD == "" {
				socket.CWD = pane.CurrentPath
			}
		}
		socket.RSSKB = tree.SubtreeRSSKB(panePIDs...)
		socket.Foreign = tree.ForeignProcesses(panePIDs)
		sockets = append(sockets, socket)
	}

	// Crumbs are read AFTER every pane is known, because a pane crumb is only
	// meaningful beside the pane list that says whether that pane still exists.
	crumbs, err := gather.ReadCrumbsReadOnly(runner.paths.SIDDir, allPanes)
	if err != nil {
		return nil, fmt.Errorf("read session crumbs: %w", err)
	}
	byName := make(map[string]int, len(sockets))
	for index, socket := range sockets {
		byName[socket.Name] = index
	}
	for _, crumb := range crumbs.Crumbs {
		index, found := byName[crumb.Socket]
		if !found {
			continue
		}
		sockets[index].HasCrumb = true
		if uuid := sessionIDFromPath(crumb.TranscriptPath); uuid != "" {
			sockets[index].CrumbUUIDs = append(sockets[index].CrumbUUIDs, uuid)
			sockets[index].transcripts = append(
				sockets[index].transcripts,
				crumb.TranscriptPath,
			)
		}
	}
	return sockets, nil
}

// listPanes probes one socket under a deadline, so one unresponsive server
// cannot stall the whole sweep.
func (runner *Runner) listPanes(
	ctx context.Context,
	socket string,
) ([]gather.Pane, error) {
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	panes, err := runner.tmux.ListPanes(probeCtx, socket)
	if err != nil && probeCtx.Err() != nil && ctx.Err() == nil {
		return nil, fmt.Errorf(
			"socket %s did not answer within %s",
			socket,
			probeTimeout,
		)
	}
	return panes, err
}

// recentSessions is the second busy signal: a session that was idle when the
// snapshot was taken can start work while detached — a queued --then steer, a
// /loop turn — and a running turn writes its transcript the whole time.
func (runner *Runner) recentSessions(
	sockets []Socket,
	window time.Duration,
) map[string]struct{} {
	recent := make(map[string]struct{})
	cutoff := runner.now().Add(-window)
	for _, socket := range sockets {
		for index, path := range socket.transcripts {
			info, err := os.Stat(path)
			if err != nil || !info.ModTime().After(cutoff) {
				continue
			}
			if index < len(socket.CrumbUUIDs) {
				recent[socket.CrumbUUIDs[index]] = struct{}{}
			}
		}
	}
	return recent
}

// apply performs the plan. Every kill re-verifies the socket at kill time:
// the fleet spawns and attaches chats while a sweep is running, and the
// verdict was formed seconds ago.
func (runner *Runner) apply(
	ctx context.Context,
	decisions []Decision,
) []Decision {
	applied := make([]Decision, 0, len(decisions))
	for _, decision := range decisions {
		switch decision.Action {
		case ActionRemoveSocketFile:
			path := filepath.Join(runner.paths.TmuxDir, decision.Socket)
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				decision.State = StateSkip
				decision.Reason = fmt.Sprintf("remove socket file: %v", err)
			}
		case ActionKillServer:
			panes, err := runner.listPanes(ctx, decision.Socket)
			if err != nil && !errors.Is(err, gather.ErrServerGone) {
				decision.State = StateSkip
				decision.Reason = fmt.Sprintf("re-probe before kill: %v", err)
				break
			}
			attached := false
			for _, pane := range panes {
				if pane.Attached {
					attached = true
				}
			}
			if attached {
				decision.State = StateSkip
				decision.Reason = "attached since the sweep started"
				break
			}
			if err := runner.killServer(ctx, decision.Socket); err != nil {
				decision.State = StateSkip
				decision.Reason = fmt.Sprintf("kill-server: %v", err)
				break
			}
			decision.State = StateKilled
		case ActionKillSession:
			name := strings.TrimPrefix(decision.Socket, "vsct:")
			if err := runner.tmux.KillSession(ctx, "vsct", name); err != nil {
				decision.State = StateSkip
				decision.Reason = fmt.Sprintf("kill-session: %v", err)
				break
			}
			decision.State = StateKilled
		}
		decision.Action = ActionNone
		applied = append(applied, decision)
	}
	return applied
}

// availableKB reads the machine's own available memory, which is the only
// honest measure of what a reap reclaimed — summed RSS double-counts the
// runtime pages several node processes share.
func (runner *Runner) availableKB() int64 {
	content, err := os.ReadFile(filepath.Join(runner.paths.ProcRoot, "meminfo"))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(line, "MemAvailable:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return value
	}
	return 0
}

// sessionIDFromPath reads the session id out of a transcript pathname.
func sessionIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
