package seat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	pfmengine "hostops/pfm/internal/engine"
)

// ProcessRecord is one observed /proc parent relation and sanitized executable
// identity. The structured gate event never carries raw process arguments.
type ProcessRecord struct {
	PID            int    `json:"pid"`
	ParentPID      int    `json:"parent_pid"`
	ProcessGroupID int    `json:"process_group_id,omitempty"`
	SessionID      int    `json:"session_id,omitempty"`
	StartTicks     uint64 `json:"start_ticks"`
	Command        string `json:"command,omitempty"`
}

// ProcessTreeVerification records both visibility proofs RULING-10 requires:
// the exact tmux pane root was resolved and read, and the /proc parent
// relation was actually enumerated before the exact two-process Codex
// bootstrap chain was called clean.
type ProcessTreeVerification struct {
	PaneRootResolved    bool            `json:"pane_root_resolved"`
	PaneRootPID         int             `json:"pane_root_pid,omitempty"`
	RootReadable        bool            `json:"root_readable"`
	Root                ProcessRecord   `json:"root"`
	ProcessesEnumerated int             `json:"processes_enumerated"`
	RelationsEnumerated int             `json:"relations_enumerated"`
	Descendants         []ProcessRecord `json:"descendants"`
	Clean               bool            `json:"clean"`
}

// ProcessTree inspects the Linux proc filesystem. Root is injectable so tests
// prove the empty and descendant cases without observing the host's live
// process state.
type ProcessTree interface {
	Inspect(context.Context, int) (ProcessTreeVerification, error)
}

// ProcTree is the production ProcessTree. It uses only procfs reads: the gate
// must not create another process beneath the seat it is trying to inspect.
type ProcTree struct {
	Root string
}

func (tree ProcTree) Inspect(
	ctx context.Context,
	rootPID int,
) (ProcessTreeVerification, error) {
	verification := ProcessTreeVerification{
		PaneRootPID: rootPID,
		Descendants: make([]ProcessRecord, 0),
	}
	if rootPID <= 0 {
		return verification, fmt.Errorf("pane root pid must be positive, got %d", rootPID)
	}

	root := filepath.Clean(tree.Root)
	if tree.Root == "" || !filepath.IsAbs(root) {
		return verification, fmt.Errorf("proc root must be an absolute path, got %q", tree.Root)
	}
	rootRecord, err := readProcessRecord(root, rootPID, true)
	if err != nil {
		return verification, fmt.Errorf("read exact pane root process %d: %w", rootPID, err)
	}
	verification.RootReadable = true
	verification.Root = rootRecord

	pids, err := listProcessPIDs(root)
	if err != nil {
		return verification, fmt.Errorf("enumerate proc process relation: %w", err)
	}
	relations := make(map[int]ProcessRecord)
	for _, pid := range pids {
		if err := ctx.Err(); err != nil {
			return verification, err
		}
		if pid <= 0 {
			continue
		}
		verification.ProcessesEnumerated++
		record, err := readProcessRecord(root, pid, false)
		if errors.Is(err, fs.ErrNotExist) {
			// A process may exit between ReadDir and its stat read. It cannot be
			// a live external child at the instant the snapshot is evaluated.
			continue
		}
		if err != nil {
			return verification, fmt.Errorf("enumerate proc relation for pid %d: %w", pid, err)
		}
		if _, duplicate := relations[pid]; duplicate {
			return verification, fmt.Errorf("proc relation enumerated pid %d twice", pid)
		}
		relations[pid] = record
		verification.RelationsEnumerated++
	}
	if verification.ProcessesEnumerated == 0 || verification.RelationsEnumerated == 0 {
		return verification, errors.New("proc relation enumeration returned no process relations")
	}
	if enumeratedRoot, found := relations[rootPID]; !found {
		return verification, fmt.Errorf(
			"exact pane root process %d was readable but absent from enumerated proc relation",
			rootPID,
		)
	} else if enumeratedRoot.ParentPID != rootRecord.ParentPID ||
		enumeratedRoot.StartTicks != rootRecord.StartTicks {
		return verification, fmt.Errorf(
			"pane root process %d changed during proc enumeration",
			rootPID,
		)
	}

	children := make(map[int][]int)
	for pid, record := range relations {
		children[record.ParentPID] = append(children[record.ParentPID], pid)
	}
	for parent := range children {
		sort.Ints(children[parent])
	}
	queue := append([]int(nil), children[rootPID]...)
	seen := map[int]bool{rootPID: true}
	for len(queue) != 0 {
		pid := queue[0]
		queue = queue[1:]
		if seen[pid] {
			return verification, fmt.Errorf("cycle in proc descendant relation at pid %d", pid)
		}
		seen[pid] = true
		record := relations[pid]
		argv, readErr := readCmdline(root, pid)
		if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
			return verification, fmt.Errorf("read descendant process %d command line: %w", pid, readErr)
		}
		record.Command = commandShape(argv)
		verification.Descendants = append(verification.Descendants, record)
		queue = append(queue, children[pid]...)
	}
	sort.Slice(verification.Descendants, func(left, right int) bool {
		return verification.Descendants[left].PID < verification.Descendants[right].PID
	})
	return verification, nil
}

// listProcessPIDs enumerates visible pids from the proc root, or from the
// kernel when there is none.
func listProcessPIDs(root string) ([]int, error) {
	if !procRootUsable(root) {
		return nativeProcessPIDs()
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	pids := make([]int, 0, len(entries))
	for _, entry := range entries {
		if pid, convErr := strconv.Atoi(entry.Name()); convErr == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func readProcessRecord(root string, pid int, requireCommand bool) (ProcessRecord, error) {
	record, _, err := readProcessIdentity(root, pid, requireCommand)
	return record, err
}

func readProcessIdentity(
	root string,
	pid int,
	requireCommand bool,
) (ProcessRecord, string, error) {
	// A proc root that EXISTS is always read — that is how the jail feeds this
	// deterministic fixtures. Without one, the kernel answers directly: macOS
	// has no /proc, and a jailer that could not identify a process would refuse
	// to contain it.
	if !procRootUsable(root) {
		return nativeProcessIdentity(pid, requireCommand)
	}
	content, err := os.ReadFile(filepath.Join(root, strconv.Itoa(pid), "stat"))
	if err != nil {
		return ProcessRecord{}, "", err
	}
	record, state, err := parseProcStat(pid, content)
	if err != nil {
		return ProcessRecord{}, "", err
	}
	if requireCommand {
		argv, readErr := readCmdline(root, pid)
		err = readErr
		if err != nil {
			return ProcessRecord{}, "", err
		}
		if len(argv) == 0 {
			return ProcessRecord{}, "", errors.New("process command line is empty")
		}
		record.Command = commandShape(argv)
	}
	return record, state, nil
}

func parseProcStat(wantPID int, content []byte) (ProcessRecord, string, error) {
	line := strings.TrimSpace(string(content))
	closeParen := strings.LastIndex(line, ") ")
	if closeParen < 0 {
		return ProcessRecord{}, "", errors.New("malformed proc stat")
	}
	openParen := strings.IndexByte(line, '(')
	if openParen < 1 || openParen > closeParen {
		return ProcessRecord{}, "", errors.New("malformed proc stat command")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line[:openParen]))
	if err != nil || pid != wantPID {
		return ProcessRecord{}, "", fmt.Errorf("proc stat pid is %q, want %d", line[:openParen], wantPID)
	}
	fields := strings.Fields(line[closeParen+2:])
	if len(fields) <= 19 {
		return ProcessRecord{}, "", errors.New("short proc stat")
	}
	parentPID, err := strconv.Atoi(fields[1])
	if err != nil || parentPID < 0 {
		return ProcessRecord{}, "", fmt.Errorf("parse proc parent pid %q", fields[1])
	}
	processGroupID, err := strconv.Atoi(fields[2])
	if err != nil || processGroupID < 0 {
		return ProcessRecord{}, "", fmt.Errorf("parse proc process group id %q", fields[2])
	}
	sessionID, err := strconv.Atoi(fields[3])
	if err != nil || sessionID < 0 {
		return ProcessRecord{}, "", fmt.Errorf("parse proc session id %q", fields[3])
	}
	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return ProcessRecord{}, "", fmt.Errorf("parse proc start ticks %q", fields[19])
	}
	return ProcessRecord{
		PID:            pid,
		ParentPID:      parentPID,
		ProcessGroupID: processGroupID,
		SessionID:      sessionID,
		StartTicks:     startTicks,
	}, fields[0], nil
}

func readCmdline(root string, pid int) ([]string, error) {
	content, err := os.ReadFile(filepath.Join(root, strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(content, []byte{0})
	argv := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) != 0 {
			argv = append(argv, string(part))
		}
	}
	return argv, nil
}

// commandShape reduces a command line to executable identity before it enters
// a verification, error, or structured event. MCP transports may carry
// credentials in trailing arguments; the gate must never persist them.
func commandShape(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	command := filepath.Base(argv[0])
	binary := pfmengine.MustLookup(pfmengine.Codex).Binary
	if command == "node" && len(argv) >= 2 && filepath.Base(argv[1]) == binary {
		return "node:" + binary
	}
	return command
}

func validateProcessTreeVerification(verification ProcessTreeVerification) error {
	switch {
	case !verification.PaneRootResolved || verification.PaneRootPID <= 0:
		return errors.New("exact tmux pane root process was not resolved")
	case !verification.RootReadable:
		return fmt.Errorf("exact pane root process %d was not readable", verification.PaneRootPID)
	case verification.Root.PID != verification.PaneRootPID:
		return fmt.Errorf(
			"read pane root pid %d does not match resolved pid %d",
			verification.Root.PID,
			verification.PaneRootPID,
		)
	case verification.Root.Command == "":
		return fmt.Errorf("exact pane root process %d has no readable command line", verification.PaneRootPID)
	case verification.Root.Command != "node:"+pfmengine.MustLookup(pfmengine.Codex).Binary:
		return fmt.Errorf(
			"exact pane root process %d is command shape %q, not the Codex Node launcher",
			verification.PaneRootPID,
			verification.Root.Command,
		)
	case verification.ProcessesEnumerated == 0 || verification.RelationsEnumerated == 0:
		return errors.New("proc descendant relation was not enumerated")
	}
	// The installed Codex launcher is a Node executable whose next argument is
	// the codex shim. It starts one direct native codex child. That exact bootstrap
	// edge is legitimate; a sibling, a deeper child, or a differently named
	// executable is external to the TUI and includes MCP transports.
	allowedNativePID := 0
	for _, descendant := range verification.Descendants {
		if allowedNativePID == 0 && descendant.ParentPID == verification.PaneRootPID &&
			descendant.Command == pfmengine.MustLookup(pfmengine.Codex).Binary {
			allowedNativePID = descendant.PID
			continue
		}
		return fmt.Errorf(
			"pane root process %d has external/MCP descendant pid %d command %q",
			verification.PaneRootPID,
			descendant.PID,
			descendant.Command,
		)
	}
	if allowedNativePID == 0 {
		return fmt.Errorf(
			"pane root process %d has no direct native Codex child",
			verification.PaneRootPID,
		)
	}
	return nil
}

func cloneProcessTreeVerification(
	verification ProcessTreeVerification,
) ProcessTreeVerification {
	descendants := make([]ProcessRecord, len(verification.Descendants))
	copy(descendants, verification.Descendants)
	verification.Descendants = descendants
	return verification
}
