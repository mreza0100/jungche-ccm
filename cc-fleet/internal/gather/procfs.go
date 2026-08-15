package gather

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// FDLink is one numeric process file descriptor and its symlink target.
type FDLink struct {
	FD     int
	Target string
}

// ProcStat contains the process relationship and birth fields gather needs.
type ProcStat struct {
	ParentPID int
	StartTime uint64
}

// ProcFS abstracts all /proc access used by gather.
type ProcFS interface {
	PIDs() ([]int, error)
	Cmdline(pid int) ([]string, error)
	Environ(pid int) (map[string]string, error)
	FDLinks(pid int) ([]FDLink, error)
	Stat(pid int) (ProcStat, error)
}

// ProcBirth is the optional ProcFS extension that reports when a process was
// created, in epoch seconds. Only Codex thread identification needs a wall
// clock, so a ProcFS that cannot supply one stays usable everywhere else.
type ProcBirth interface {
	Birth(pid int) (int64, error)
}

// ProcMemory is the optional ProcFS extension that reports a process's
// resident set size in kilobytes. Only the reaper needs it — the RAM a socket
// holds is the whole reason to reap one — so a ProcFS without it stays usable
// everywhere else and the reaper reports no RAM rather than refusing to run.
type ProcMemory interface {
	RSSKB(pid int) (int64, error)
}

// RealProcFS reads a Linux proc filesystem. Root defaults to /proc.
type RealProcFS struct {
	Root string
}

func (proc RealProcFS) root() string {
	if proc.Root == "" {
		return "/proc"
	}
	return proc.Root
}

// PIDs returns numeric process directories in ascending order.
func (proc RealProcFS) PIDs() ([]int, error) {
	entries, err := os.ReadDir(proc.root())
	if err != nil {
		return nil, fmt.Errorf("read proc root: %w", err)
	}
	pids := make([]int, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	sort.Ints(pids)
	return pids, nil
}

// Cmdline returns NUL-delimited argv.
func (proc RealProcFS) Cmdline(pid int) ([]string, error) {
	content, err := os.ReadFile(proc.path(pid, "cmdline"))
	if err != nil {
		return nil, err
	}
	return splitNUL(content), nil
}

// Environ returns the process environment keyed before the first equals sign.
func (proc RealProcFS) Environ(pid int) (map[string]string, error) {
	content, err := os.ReadFile(proc.path(pid, "environ"))
	if err != nil {
		return nil, err
	}
	environment := make(map[string]string)
	for _, entry := range splitNUL(content) {
		key, value, found := strings.Cut(entry, "=")
		if found {
			environment[key] = value
		}
	}
	return environment, nil
}

// FDLinks returns readable fd links in numeric order. Descriptors that vanish
// during the walk are ignored.
func (proc RealProcFS) FDLinks(pid int) ([]FDLink, error) {
	directory := proc.path(pid, "fd")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	links := make([]FDLink, 0, len(entries))
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry.Name())
		if err != nil || fd < 0 {
			continue
		}
		target, err := os.Readlink(filepath.Join(directory, entry.Name()))
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			continue
		}
		if err != nil {
			continue
		}
		links = append(links, FDLink{FD: fd, Target: target})
	}
	sort.Slice(links, func(left, right int) bool {
		return links[left].FD < links[right].FD
	})
	return links, nil
}

// Stat reads parent pid and kernel start ticks from /proc/<pid>/stat.
func (proc RealProcFS) Stat(pid int) (ProcStat, error) {
	content, err := os.ReadFile(proc.path(pid, "stat"))
	if err != nil {
		return ProcStat{}, err
	}
	closeParen := strings.LastIndex(string(content), ") ")
	if closeParen < 0 {
		return ProcStat{}, fmt.Errorf("malformed proc stat for pid %d", pid)
	}
	fields := strings.Fields(string(content[closeParen+2:]))
	if len(fields) <= 19 {
		return ProcStat{}, fmt.Errorf("short proc stat for pid %d", pid)
	}
	parentPID, err := strconv.Atoi(fields[1])
	if err != nil {
		return ProcStat{}, fmt.Errorf("parse parent pid for %d: %w", pid, err)
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return ProcStat{}, fmt.Errorf("parse start time for %d: %w", pid, err)
	}
	return ProcStat{ParentPID: parentPID, StartTime: startTime}, nil
}

// Birth returns the process start time in epoch seconds. The kernel stamps
// the /proc/<pid> directory when it creates the process, so its modification
// time is the birth moment without the boot-time arithmetic /proc/<pid>/stat
// ticks would need.
func (proc RealProcFS) Birth(pid int) (int64, error) {
	info, err := os.Stat(filepath.Join(proc.root(), strconv.Itoa(pid)))
	if err != nil {
		return 0, err
	}
	return info.ModTime().Unix(), nil
}

// RSSKB returns a process's resident set size in kilobytes, read from
// /proc/<pid>/statm — the cheapest of the three files that carry it (one line,
// no parsing beyond a field split) and the one whose page counts scale with
// the kernel's own page size rather than assuming 4 KiB.
func (proc RealProcFS) RSSKB(pid int) (int64, error) {
	content, err := os.ReadFile(proc.path(pid, "statm"))
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(content))
	if len(fields) < 2 {
		return 0, fmt.Errorf("malformed proc statm for pid %d", pid)
	}
	pages, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse resident pages for %d: %w", pid, err)
	}
	return pages * int64(os.Getpagesize()) / 1024, nil
}

func (proc RealProcFS) path(pid int, element string) string {
	return filepath.Join(proc.root(), strconv.Itoa(pid), element)
}

func splitNUL(content []byte) []string {
	parts := strings.Split(string(content), "\x00")
	for len(parts) != 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
