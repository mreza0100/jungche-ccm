package gather

import (
	"fmt"
	"sort"
)

// DetectClaudeProcesses maps every live Claude process to a gathered pane.
// Compose uses this fact to decide whether a socket-scoped crumb is still
// trustworthy after its original pane exits.
func DetectClaudeProcesses(
	proc ProcFS,
	panes []Pane,
	binaries ...string,
) ([]ClaudeProcess, error) {
	pids, err := proc.PIDs()
	if err != nil {
		return nil, fmt.Errorf("list processes for Claude scan: %w", err)
	}
	sort.Ints(pids)
	paneByPID := panesByPID(panes)
	processes := make([]ClaudeProcess, 0)
	for _, pid := range pids {
		cmdline, err := proc.Cmdline(pid)
		if err != nil || !isClaudeCommand(cmdline, binaries...) {
			continue
		}
		pane, found := paneForProcess(proc, pid, paneByPID)
		if !found {
			continue
		}
		processes = append(processes, ClaudeProcess{
			PID:     pid,
			PanePID: pane.PID,
			Socket:  pane.Socket,
			PaneID:  pane.PaneID,
			TTY:     pane.TTY,
		})
	}
	sort.Slice(processes, func(left, right int) bool {
		if processes[left].Socket != processes[right].Socket {
			return processes[left].Socket < processes[right].Socket
		}
		return processes[left].PID < processes[right].PID
	})
	return processes, nil
}
