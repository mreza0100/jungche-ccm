package action

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// RealProcesses reads the small /proc subset needed by the stray sweep.
type RealProcesses struct {
	Root string
}

func (processes RealProcesses) Processes(
	ctx context.Context,
) ([]Process, error) {
	root := processes.Root
	if root == "" {
		root = "/proc"
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	result := make([]Process, 0)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 || !entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, entry.Name(), "cmdline"))
		if err != nil || len(content) == 0 {
			continue
		}
		argv := splitNUL(content)
		tty := ""
		if target, err := os.Readlink(
			filepath.Join(root, entry.Name(), "fd", "0"),
		); err == nil {
			tty = strings.TrimPrefix(target, "/dev/")
		}
		result = append(result, Process{PID: pid, Argv: argv, TTY: tty})
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].PID < result[right].PID
	})
	return result, nil
}

func (RealProcesses) Terminate(pid int) error {
	err := syscall.Kill(pid, syscall.SIGTERM)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func splitNUL(content []byte) []string {
	fields := bytes.Split(content, []byte{0})
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) != 0 {
			values = append(values, string(field))
		}
	}
	return values
}
