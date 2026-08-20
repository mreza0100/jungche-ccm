//go:build linux

package resolve

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type linuxProcFS struct{ root string }

func newNativeProcFS(root string) ProcFS {
	if root == "" {
		root = "/proc"
	}
	return linuxProcFS{root: root}
}

func (proc linuxProcFS) Environ(pid int) (map[string]string, error) {
	content, err := os.ReadFile(proc.path(pid, "environ"))
	if err != nil {
		return nil, err
	}
	environment := make(map[string]string)
	for _, entry := range strings.Split(string(content), "\x00") {
		key, value, found := strings.Cut(entry, "=")
		if found {
			environment[key] = value
		}
	}
	return environment, nil
}

func (proc linuxProcFS) Stat(pid int) (ProcStat, error) {
	content, err := os.ReadFile(proc.path(pid, "stat"))
	if err != nil {
		return ProcStat{}, err
	}
	closeParen := strings.LastIndex(string(content), ") ")
	if closeParen < 0 {
		return ProcStat{}, errors.New("malformed proc stat")
	}
	fields := strings.Fields(string(content[closeParen+2:]))
	if len(fields) < 2 {
		return ProcStat{}, errors.New("short proc stat")
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return ProcStat{}, err
	}
	return ProcStat{ParentPID: parent}, nil
}

func (proc linuxProcFS) path(pid int, element string) string {
	return filepath.Join(proc.root, strconv.Itoa(pid), element)
}
