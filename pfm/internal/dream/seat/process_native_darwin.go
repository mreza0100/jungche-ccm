//go:build darwin

package seat

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// procRootUsable reports whether a real proc tree is there to read. The jail
// fixtures point the seat at one; a live macOS never has one.
func procRootUsable(root string) bool {
	info, err := os.Stat(root)
	return err == nil && info.IsDir()
}

// nativeProcessIdentity answers the same question /proc/<pid>/stat answers, on
// a kernel with no /proc.
//
// Every field has a direct equivalent: the process group and session come from
// getpgid/getsid, which are POSIX and exact, rather than from parsing a line.
// StartTicks is NOT the Linux unit — there it is kernel ticks since boot, here
// microseconds since the epoch — and that is safe because the field is only
// ever compared against a value captured the same way, to prove a recycled PID
// is not the process that was captured.
func nativeProcessIdentity(pid int, requireCommand bool) (ProcessRecord, string, error) {
	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return ProcessRecord{}, "", processGoneOr(err, pid, "read kern.proc.pid")
	}
	if int(process.Proc.P_pid) != pid {
		return ProcessRecord{}, "", fs.ErrNotExist
	}
	// getsid races the walk exactly as the kinfo read does: a member that exits
	// between the two calls must read as GONE, not as a jail that cannot be
	// inspected. Returning a hard error here refused the whole cleanup.
	sessionID, err := unix.Getsid(pid)
	if err != nil {
		return ProcessRecord{}, "", processGoneOr(err, pid, "read session id")
	}
	started := process.Proc.P_starttime
	record := ProcessRecord{
		PID:            pid,
		ParentPID:      int(process.Eproc.Ppid),
		ProcessGroupID: int(process.Eproc.Pgid),
		SessionID:      sessionID,
		StartTicks:     uint64(started.Sec)*1_000_000 + uint64(started.Usec),
	}
	if requireCommand {
		argv, cmdErr := nativeProcessArgv(pid)
		if cmdErr != nil {
			return ProcessRecord{}, "", cmdErr
		}
		if len(argv) == 0 {
			return ProcessRecord{}, "", errors.New("process command line is empty")
		}
		record.Command = commandShape(argv)
	}
	return record, darwinProcessState(process.Proc.P_stat), nil
}

// processGoneOr normalizes the several errnos macOS uses for "that pid is not
// there any more" into the not-exist shape a /proc read would have produced.
//
// A pid that exits between the listing and the read is the normal case during a
// sweep, and macOS reports it inconsistently: ESRCH when the pid is unknown,
// EINVAL when the sysctl name no longer resolves, and EIO when the kernel had
// the entry but it went away mid-copy. All three mean the same thing here — it
// cannot be a live member of the group.
func processGoneOr(err error, pid int, action string) error {
	if errors.Is(err, unix.ESRCH) ||
		errors.Is(err, unix.EINVAL) ||
		errors.Is(err, unix.EIO) ||
		errors.Is(err, unix.ENOENT) {
		return fs.ErrNotExist
	}
	return fmt.Errorf("%s for %d: %w", action, pid, err)
}

// darwinProcessState maps a BSD p_stat to the Linux state letter the jail
// compares against. Only "Z" is load-bearing — the kill loop waits until every
// surviving member is a zombie — so that mapping is the one that must be right.
func darwinProcessState(state int8) string {
	switch state {
	case 1: // SIDL — being created
		return "I"
	case 2: // SRUN
		return "R"
	case 3: // SSLEEP
		return "S"
	case 4: // SSTOP
		return "T"
	case 5: // SZOMB
		return "Z"
	default:
		return "S"
	}
}

// nativeProcessPIDs lists every visible pid.
func nativeProcessPIDs() ([]int, error) {
	processes, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, fmt.Errorf("list processes via kern.proc.all: %w", err)
	}
	pids := make([]int, 0, len(processes))
	for index := range processes {
		if pid := int(processes[index].Proc.P_pid); pid > 0 {
			pids = append(pids, pid)
		}
	}
	sort.Ints(pids)
	return pids, nil
}

// nativeProcessArgv decodes one process's argv from kern.procargs2.
//
// This duplicates the decode in internal/gather DELIBERATELY, and it must stay
// duplicated. internal/dream/isolation_test.go forbids every dream package from
// importing gather, and the seat may cross the host boundary only into action,
// spawn, headless, transcript and paths. Sharing this would either breach that
// boundary or widen it, and the boundary is the more valuable of the two rules:
// it is what keeps the dreamer independent of the fleet's host surface. Anyone
// tempted to "fix" the duplication by importing gather should read that test
// first.
//
// The buffer layout is Apple's, and the order matters:
//
//	int32 argc | exec_path NUL | NUL padding | argc×(argv NUL) | env NUL...
//
// argv entries may legitimately be EMPTY strings, so the argc count is what
// separates argv from the environment.
func nativeProcessArgv(pid int) ([]string, error) {
	buffer, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return nil, processGoneOr(err, pid, "read kern.procargs2")
	}
	if len(buffer) < 4 {
		return nil, fmt.Errorf("short kern.procargs2 buffer for %d: %d bytes", pid, len(buffer))
	}
	argc := int(binary.LittleEndian.Uint32(buffer[:4]))
	if argc < 0 {
		return nil, fmt.Errorf("negative argc for %d", pid)
	}
	fields := strings.Split(string(buffer[4:]), "\x00")
	index := 0
	if index < len(fields) {
		index++ // exec_path, which argv[0] repeats
	}
	for index < len(fields) && fields[index] == "" {
		index++ // alignment padding between exec_path and argv
	}
	argv := make([]string, 0, argc)
	for counted := 0; counted < argc && index < len(fields); counted++ {
		argv = append(argv, fields[index])
		index++
	}
	return argv, nil
}
