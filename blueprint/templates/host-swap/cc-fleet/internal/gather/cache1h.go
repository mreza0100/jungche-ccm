package gather

import (
	"fmt"
	"sort"
)

// DetectCache1H returns sockets containing a Claude process explicitly born
// with ENABLE_PROMPT_CACHING_1H=1.
func DetectCache1H(proc ProcFS, panes []Pane) ([]string, error) {
	pids, err := proc.PIDs()
	if err != nil {
		return nil, fmt.Errorf("list processes for cache scan: %w", err)
	}
	sort.Ints(pids)
	paneByPID := panesByPID(panes)
	sockets := make(map[string]struct{})

	for _, pid := range pids {
		cmdline, err := proc.Cmdline(pid)
		if err != nil || !isClaudeCommand(cmdline) {
			continue
		}
		environment, err := proc.Environ(pid)
		if err != nil || environment["ENABLE_PROMPT_CACHING_1H"] != "1" {
			continue
		}
		pane, found := paneForProcess(proc, pid, paneByPID)
		if found {
			sockets[pane.Socket] = struct{}{}
		}
	}

	result := make([]string, 0, len(sockets))
	for socket := range sockets {
		result = append(result, socket)
	}
	sort.Strings(result)
	return result, nil
}
