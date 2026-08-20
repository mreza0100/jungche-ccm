//go:build !linux && !darwin

package resolve

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type fallbackProcFS struct{ root string }

func newNativeProcFS(root string) ProcFS {
	if root == "" {
		root = "/proc"
	}
	return fallbackProcFS{root: root}
}

func (proc fallbackProcFS) Environ(pid int) (map[string]string, error) {
	content, err := os.ReadFile(filepath.Join(proc.root, strconv.Itoa(pid), "environ"))
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, entry := range strings.Split(string(content), "\x00") {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			result[key] = value
		}
	}
	return result, nil
}

func (proc fallbackProcFS) Stat(pid int) (ProcStat, error) {
	content, err := os.ReadFile(filepath.Join(proc.root, strconv.Itoa(pid), "stat"))
	if err != nil {
		return ProcStat{}, err
	}
	closeParen := strings.LastIndex(string(content), ") ")
	if closeParen < 0 {
		return ProcStat{}, os.ErrInvalid
	}
	fields := strings.Fields(string(content[closeParen+2:]))
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return ProcStat{}, err
	}
	return ProcStat{ParentPID: parent}, nil
}
