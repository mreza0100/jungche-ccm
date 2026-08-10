package gather

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"hostops/cc-fleet/internal/resolve"
)

// CodexThreadResolver names the conversation behind a live codex process that
// holds no rollout file descriptor. It receives the process's exported
// CODEX_THREAD_ID, its pane directory, and its start time in epoch seconds,
// and returns the thread id with the rollout path the Codex state store
// records for it. An empty id means the process could not be identified and
// must not become a live row.
type CodexThreadResolver func(
	exported string,
	cwd string,
	birth int64,
) (id string, rolloutPath string)

// DetectCodex maps live codex processes to panes using pid ancestry. It sees
// only sessions that hold a rollout file descriptor.
func DetectCodex(proc ProcFS, codexRoot string, panes []Pane) ([]LiveCodex, error) {
	return DetectCodexThreads(proc, codexRoot, panes, nil)
}

// DetectCodexThreads is DetectCodex plus state-store identity, so a Codex
// session that writes no rollout file — the normal shape of a paginated
// thread since Codex 0.146.1 — is still a live chat instead of a missing one.
func DetectCodexThreads(
	proc ProcFS,
	codexRoot string,
	panes []Pane,
	identify CodexThreadResolver,
) ([]LiveCodex, error) {
	pids, err := proc.PIDs()
	if err != nil {
		return nil, fmt.Errorf("list processes for Codex scan: %w", err)
	}
	sort.Ints(pids)
	paneByPID := panesByPID(panes)
	sessionsRoot := filepath.Join(codexRoot, "sessions")

	live := make([]LiveCodex, 0)
	for _, pid := range pids {
		cmdline, err := proc.Cmdline(pid)
		if err != nil || len(cmdline) == 0 || filepath.Base(cmdline[0]) != "codex" {
			continue
		}
		links, err := proc.FDLinks(pid)
		if err != nil {
			continue
		}
		sort.Slice(links, func(left, right int) bool {
			return links[left].FD < links[right].FD
		})

		rolloutPath := ""
		for _, link := range links {
			if isRolloutUnder(sessionsRoot, link.Target) {
				rolloutPath = filepath.Clean(link.Target)
				break
			}
		}
		if rolloutPath == "" && identify == nil {
			continue
		}

		pane, found := paneForProcess(proc, pid, paneByPID)
		if !found {
			continue
		}
		threadID := ""
		if identify != nil {
			threadID = codexThreadEnv(proc, pid)
		}
		if rolloutPath == "" {
			id, path := identify(threadID, pane.CurrentPath, processBirth(proc, pid))
			if id == "" {
				continue
			}
			threadID = id
			rolloutPath = path
		}
		live = append(live, LiveCodex{
			PID:         pid,
			PanePID:     pane.PID,
			Socket:      pane.Socket,
			PaneID:      pane.PaneID,
			RolloutPath: rolloutPath,
			ThreadID:    threadID,
		})
	}
	sort.Slice(live, func(left, right int) bool {
		if live[left].Socket != live[right].Socket {
			return live[left].Socket < live[right].Socket
		}
		return live[left].PID < live[right].PID
	})
	return live, nil
}

// codexThreadEnv reads the identity a Codex session exports into the shells
// it spawns. It is absent from many sessions, which is why the caller still
// needs the directory and birth-time fallback.
func codexThreadEnv(proc ProcFS, pid int) string {
	environment, err := proc.Environ(pid)
	if err != nil {
		return ""
	}
	return environment[resolve.CodexThreadEnv]
}

func processBirth(proc ProcFS, pid int) int64 {
	birther, supported := proc.(ProcBirth)
	if !supported {
		return 0
	}
	birth, err := birther.Birth(pid)
	if err != nil {
		return 0
	}
	return birth
}

func isRolloutUnder(root, target string) bool {
	if !strings.HasPrefix(filepath.Base(target), "rollout-") ||
		filepath.Ext(target) != ".jsonl" {
		return false
	}
	relative, err := filepath.Rel(root, filepath.Clean(target))
	return err == nil &&
		relative != "." &&
		relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func panesByPID(panes []Pane) map[int]Pane {
	paneByPID := make(map[int]Pane, len(panes))
	for _, pane := range panes {
		paneByPID[pane.PID] = pane
	}
	return paneByPID
}

func paneForProcess(proc ProcFS, pid int, paneByPID map[int]Pane) (Pane, bool) {
	current := pid
	for depth := 0; depth <= 4; depth++ {
		if pane, found := paneByPID[current]; found {
			return pane, true
		}
		if depth == 4 {
			break
		}
		stat, err := proc.Stat(current)
		if err != nil || stat.ParentPID <= 1 || stat.ParentPID == current {
			break
		}
		current = stat.ParentPID
	}
	return Pane{}, false
}
